package main

import (
	"io"
	"net"
	"net/http"
	"net/url"

	glog "github.com/Sirupsen/logrus"
	"golang.org/x/net/websocket"
)

func AddWebsocketHandler(urlPattern string, uri string) error {
	glog.Debugf("AddWebsocketHandler urlPattern=%s, uri=%s", urlPattern, uri)
	u, err := url.Parse(uri)
	if err != nil {
		glog.Errorf("surgemq/main: %v", err)
		return err
	}

	h := func(ws *websocket.Conn) {
		WebsocketTcpProxy(ws, u.Scheme, u.Host)
	}
	http.Handle(urlPattern, websocket.Handler(h))
	return nil
}

/* start a listener that proxies websocket <-> tcp */
func ListenAndServeWebsocket(addr string) error {
	glog.Info("ListenAndServeWebsocket start......")
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		glog.Infof("start websocket %v Fail", addr)
	} else {
		glog.Infof("start websocket %v Success", addr)
	}
	glog.Info("ListenAndServeWebsocket end......")
	return err
}

/* starts an HTTPS listener */
func ListenAndServeWebsocketSecure(addr string, cert string, key string) error {
	glog.Info("ListenAndServeWebsocketSecure start......")
	err := http.ListenAndServeTLS(addr, cert, key, nil)
	if err != nil {
		glog.Infof("start TLS websocket %v Fail", addr)
	} else {
		glog.Infof("start TLS websocket %v Success", addr)
	}
	glog.Info("ListenAndServeWebsocketSecure end......")
	return err
}

/* copy from websocket to writer, this copies the binary frames as is */
func io_copy_ws(src *websocket.Conn, dst io.Writer) (int, error) {
	var buffer []byte
	count := 0
	for {
		err := websocket.Message.Receive(src, &buffer)
		if err != nil {
			return count, err
		}
		n := len(buffer)
		count += n
		i, err := dst.Write(buffer)
		if err != nil || i < 1 {
			return count, err
		}
	}
	return count, nil
}

/* copy from reader to websocket, this copies the binary frames as is */
func io_ws_copy(src io.Reader, dst *websocket.Conn) (int, error) {
	buffer := make([]byte, 2048)
	count := 0
	for {
		n, err := src.Read(buffer)
		if err != nil || n < 1 {
			return count, err
		}
		count += n
		err = websocket.Message.Send(dst, buffer[0:n])
		if err != nil {
			return count, err
		}
	}
	return count, nil
}

/* handler that proxies websocket <-> unix domain socket */
func WebsocketTcpProxy(ws *websocket.Conn, nettype string, host string) error {
	client, err := net.Dial(nettype, host)
	if err != nil {
		return err
	}
	defer client.Close()
	defer ws.Close()
	chDone := make(chan bool)

	go func() {
		io_ws_copy(client, ws)
		chDone <- true
	}()
	go func() {
		io_copy_ws(ws, client)
		chDone <- true
	}()
	<-chDone
	return nil
}
