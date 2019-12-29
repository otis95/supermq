package service

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/578157900/supermq/message"

	glog "github.com/Sirupsen/logrus"
)

// RouteType designates the router type
type RouteType int

// Type of Route
const (
	// This route we learned from speaking to other routes.此集群路由的连接来自其他的集群服务
	Implicit RouteType = iota
	// This route was explicitly configured.这个集群路由的连接是依靠配置文件
	Explicit
)

type route struct {
	remoteID     string    //路由id
	didSolicit   bool      //是服务连接其他集群，还是其他集群连接此服务
	retry        bool      //是否重试
	routeType    RouteType //类型
	url          *url.URL  //地址
	authRequired bool      //是否有验证
	tlsRequired  bool      //是否有tls加密
}

type Info struct {
	ID                string   `json:"server_id"`
	Version           string   `json:"version"`
	Host              string   `json:"host"`
	Port              int      `json:"port"`
	AuthRequired      bool     `json:"auth_required"`
	SSLRequired       bool     `json:"ssl_required"` // DEPRECATED: ssl json used for older clients
	TLSRequired       bool     `json:"tls_required"`
	TLSVerify         bool     `json:"tls_verify"`
	MaxPayload        int      `json:"max_payload"`
	IP                string   `json:"ip,omitempty"`
	ClientConnectURLs []string `json:"connect_urls,omitempty"` // Contains URLs a client can connect to.
}

func (this *Server) StartRouting(info Info) {
	this.route_client = make(map[uint64]*service)

	info.ClientConnectURLs = getClientConnectURLs(info.Host, info.Port)

	info_byte, _ := json.Marshal(info)

	this.routeInfo = info
	this.routeInfoJSON = info_byte
	this.routes = make(map[string]*route)

	ch := make(chan struct{})
	go this.routeAcceptLoop(ch)
	<-ch

	//本服务去连接其他集群服务
	this.solicitRoutes()
}

func (this *Server) routeAcceptLoop(ch chan struct{}) {
	hp := net.JoinHostPort(this.routeInfo.Host, strconv.Itoa(this.routeInfo.Port))

	glog.Infof("Listening for route connections on %s", hp)

	l, e := net.Listen("tcp", hp)
	defer l.Close()

	if e != nil {
		// We need to close this channel to avoid a deadlock
		close(ch)
		glog.Errorf("Error listening on router port: %d - %v", this.routeInfo.Port, e)
		return
	}
	// Setup state that can enable shutdown
	this.routeListener = l

	// Let them know we are up
	close(ch)

	var tempDelay time.Duration // how long to sleep on accept failure

	for {
		conn, err := this.routeListener.Accept()

		if err != nil {
			select {
			case <-this.quit:
				return

			default:
			}

			// Borrowed from go1.3.3/src/pkg/net/http/server.go:1699
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				glog.Errorf("router-server/ListenAndServe: Accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return
		}

		go this.createRoute(conn, nil)
	}
}

func (this *Server) createRoute(conn net.Conn, rURL *url.URL) (svc *service, err error) {

	defer func() {
		if err != nil {
			conn.Close()
		}
	}()

	err = this.checkConfiguration()
	if err != nil {
		return nil, err
	}

	didSolicit := rURL != nil

	r := &route{didSolicit: didSolicit}
	for _, route := range this.Routes {
		if rURL != nil && (strings.ToLower(rURL.Host) == strings.ToLower(route.Host)) {
			r.routeType = Explicit
		}
	}

	if didSolicit {
		r.url = rURL
	}

	svc = &service{
		id:     atomic.AddUint64(&gsvcid, 1),
		client: false,

		keepAlive:      minKeepAlive,
		connectTimeout: this.ConnectTimeout,
		ackTimeout:     this.AckTimeout,
		timeoutRetries: this.TimeoutRetries,

		conn:      conn,
		topicsMgr: this.topicsMgr,

		server: this,
		route:  r,
		topics: make(map[string]struct{}),
	}

	conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(this.ConnectTimeout)))

	req := message.NewRouteMessage()

	req.SetInfo(this.routeInfoJSON)

	if err = writeMessage(conn, req); err != nil {
		return nil, err
	}

	if err := svc.start(); err != nil {
		svc.stop()
		return nil, err
	}

	this.mu.Lock()
	this.route_client[svc.id] = svc
	this.mu.Unlock()

	return svc, err
}

//此服务主动连接
func (this *Server) connectToRoute(rURL *url.URL, tryForEver bool) {
	attempts := 0
	for rURL != nil {
		glog.Infof("Trying to connect to route on %s", rURL.Host)
		conn, err := net.DialTimeout("tcp", rURL.Host, 1*time.Second)
		if err != nil {
			glog.Debugf("Error trying to connect to route: %v", err)

			if !tryForEver {
				attempts++
				if attempts > 15 {
					return
				}
			}
			select {
			case <-this.quit:
				return
			case <-time.After(1 * time.Second):
				continue
			}
		}
		this.createRoute(conn, rURL)
		return
	}
}

//连接其他集群
func (this *Server) solicitRoutes() {
	for _, r := range this.Routes {
		go func(rURL *url.URL) { this.connectToRoute(rURL, true) }(r)
	}
}

//发送本地的topics给其他集群
func (s *Server) sendLocalSubsToRoute(route *service) {
	topics := s.topicsMgr.Topics()
	if len(topics) > 0 {
		msg := message.NewSubscribeMessage()

		for _, topic := range topics {
			msg.AddTopic([]byte(topic), 0x00)
		}

		writeMessage(route.conn, msg)
	}
	glog.Debugf("Route sent local subscriptions")
}

//添加路由
func (this *Server) addRoute(client *service) (bool, bool) {
	id := client.route.remoteID
	sendInfo := false

	this.mu.Lock()
	_, exists := this.routes[id]
	if !exists {
		this.routes[id] = client.route

		//只有2个服务集群 就不需要通知其他集群了
		sendInfo = len(this.routes) > 1
	}

	this.mu.Unlock()
	return !exists, sendInfo
}

//通知所有集群服务，新连接的集群信息
func (this *Server) forwardNewRouteInfoToKnownServers(info Info) {
	this.mu.Lock()
	defer this.mu.Unlock()

	b, _ := json.Marshal(info)

	req := message.NewRouteMessage()

	req.SetInfo(b)

	for _, r := range this.route_client {
		r.mu.Lock()
		if r.route.remoteID != info.ID {
			writeMessage(r.conn, req)
		}
		r.mu.Unlock()
	}
}

//集群信息是否在配置文件中
func (s *Server) hasThisRouteConfigured(info Info) bool {
	urlToCheckExplicit := strings.ToLower(net.JoinHostPort(info.Host, strconv.Itoa(info.Port)))
	for _, ri := range s.Routes {
		if strings.ToLower(ri.Host) == urlToCheckExplicit {
			return true
		}
	}
	return false
}

//若集群会通知其他服务，集群机器的信息。连接其他集群
func (this *Server) processImplicitRoute(info Info) {
	remoteID := info.ID

	this.mu.Lock()
	defer this.mu.Unlock()

	// Don't connect to ourself
	if remoteID == this.routeInfo.ID {
		return
	}
	// Check if this route already exists
	if _, exists := this.routes[remoteID]; exists {
		return
	}
	// Check if we have this route as a configured route
	if this.hasThisRouteConfigured(info) {
		return
	}

	// Initiate the connection, using info.IP instead of info.URL here...
	r, err := url.Parse(info.IP)
	if err != nil {
		glog.Debugf("Error parsing URL from INFO: %v\n", err)
		return
	}
	go func() { this.connectToRoute(r, false) }()
}

//发送心跳包
func (this *service) processPingTimer() {
	this.mu.Lock()
	this.ptmr = nil
	this.mu.Unlock()

	glog.Infof("%s Ping Timer", this.cid())

	// Send PING
	req := message.NewPingreqMessage()
	_, err := this.writeMessage(req)
	if err != nil {
		glog.Errorf("Error on Client Ping Flush, error %s", err)
		this.stop()
	} else {
		// Reset to fire again if all OK.
		this.setPingTimer()
	}
}

//开启心跳 10S一个，防止长连接失效
func (this *service) setPingTimer() {
	this.mu.Lock()
	if this.server == nil {
		return
	}
	d := 10 * time.Second
	this.ptmr = time.AfterFunc(d, this.processPingTimer)
	this.mu.Unlock()
}

//暂停清空心跳
func (this *service) clearPingTimer() {
	this.mu.Lock()
	defer this.mu.Unlock()
	if this.ptmr == nil {
		return
	}
	this.ptmr.Stop()
	this.ptmr = nil
}

//获取此服务的所有url，能通知client去连接。暂时未使用到。
func getClientConnectURLs(host string, port int) []string {

	sPort := strconv.Itoa(port)
	urls := make([]string, 0, 1)

	ipAddr, err := net.ResolveIPAddr("ip", host)
	// If the host is "any" (0.0.0.0 or ::), get specific IPs from available
	// interfaces.
	if err == nil && ipAddr.IP.IsUnspecified() {
		var ip net.IP
		ifaces, _ := net.Interfaces()
		for _, i := range ifaces {
			addrs, _ := i.Addrs()
			for _, addr := range addrs {
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				// Skip non global unicast addresses
				if !ip.IsGlobalUnicast() || ip.IsUnspecified() {
					ip = nil
					continue
				}
				urls = append(urls, net.JoinHostPort(ip.String(), sPort))
			}
		}
	}
	if err != nil || len(urls) == 0 {
		// We are here if s.opts.Host is not "0.0.0.0" nor "::", or if for some
		// reason we could not add any URL in the loop above.
		// We had a case where a Windows VM was hosed and would have err == nil
		// and not add any address in the array in the loop above, and we
		// ended-up returning 0.0.0.0, which is problematic for Windows clients.
		// Check for 0.0.0.0 or :: specifically, and ignore if that's the case.
		if host == "0.0.0.0" || host == "::" {
			fmt.Printf("Address %q can not be resolved properly\n", host)
		} else {
			urls = append(urls, net.JoinHostPort(host, sPort))
		}
	}
	return urls
}
