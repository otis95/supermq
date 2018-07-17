// Copyright (c) 2014 The SurgeMQ Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strings"
	"syscall"

	"github.com/578157900/supermq/applog"
	"github.com/578157900/supermq/config"

	"supermq/internal/service"

	log "github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

//*******************************************//
var (
	TimeStamp  string //Build Time
	Version    string //Publish Version
	SVNVersion string //SVN Version
)

/******************************************/
//mqtt:wss			121.42.35.23:8084	64		1
//mqtt:ws			121.42.35.23:8083	64		0
//mqtt:tcp			121.42.35.23:1883	1024	4
//dashboard:http	11883				512		6
/******************************************/
var (
	keepAlive        int    = service.DefaultKeepAlive
	connectTimeout   int    = service.DefaultConnectTimeout
	ackTimeout       int    = service.DefaultAckTimeout
	timeoutRetries   int    = service.DefaultTimeoutRetries
	authenticator    string = service.DefaultAuthenticator
	sessionsProvider string = service.DefaultSessionsProvider
	topicsProvider   string = service.DefaultTopicsProvider
	wsAddr           string // HTTPS websocket address eg. :8083
	wssAddr          string // HTTPS websocket address, eg. :8084
	wssCertPath      string // path to HTTPS public key
	wssKeyPath       string // path to HTTPS private key
)

func run(c *cli.Context) error {

	conf, err := config.ReadConfig(c.String("conf"))
	if err != nil {
		log.Error("read from conf fail!", c.String("conf"))
		return err
	}
	fmt.Println("conf =  ", conf)

	fmt.Println("runtime.GOOS = ", runtime.GOOS)

	//Auto Daily Logger
	if conf.LogToFile {
		var logger *applog.AutoDailyLoger
		if runtime.GOOS == "windows" {
			logger = applog.NewAutoDailyLoger(conf.LogDirWin, conf.LogPrefix, conf.LogLevel)
		} else {
			logger = applog.NewAutoDailyLoger(conf.LogDirLinux, conf.LogPrefix, conf.LogLevel)
		}
		logger.Start()
		defer logger.Stop()
	} else {
		switch conf.LogLevel {
		case "debug":
			log.SetLevel(log.DebugLevel)
		case "info":
			log.SetLevel(log.InfoLevel)
		case "warn":
			log.SetLevel(log.WarnLevel)
		case "error":
			log.SetLevel(log.ErrorLevel)
		case "panic":
			log.SetLevel(log.PanicLevel)
		default:
			log.SetLevel(log.InfoLevel)
		}
	}
	// create mqtt server
	svr := &service.Server{
		KeepAlive:        service.DefaultKeepAlive,
		ConnectTimeout:   connectTimeout,
		AckTimeout:       ackTimeout,
		TimeoutRetries:   timeoutRetries,
		SessionsProvider: sessionsProvider,
		TopicsProvider:   topicsProvider,
	}
	for _, s := range conf.Routes {
		u, err := url.Parse(s)
		if err != nil {
			panic(err)
		}
		svr.Routes = append(svr.Routes, u)
	}

	if conf.RouteModel {
		go func() {
			svr.StartRouting(
				service.Info{
					ID:   conf.RouteId,
					Host: conf.Host,
					Port: conf.Port,
					IP:   conf.Ip},
			)
		}()
	}
	/* mqtt:tcp	121.42.35.23:1883 MQTT listener */
	go func() {
		err = svr.ListenAndServe(conf.MQTTServer)
		if err != nil {
			log.Errorf("surgemq/main TCP: %v", err)
		}
	}()

	wsAddr := conf.WSMQTTServer
	wssAddr := conf.WSSMQTTServer
	wssCertPath := conf.CertPath
	wssKeyPath := conf.KeyPath
	log.Info("wsAddr = ", wsAddr)
	log.Info("wssAddr = ", wssAddr)
	log.Info("wssCertPath = ", wssCertPath)
	log.Info("wssKeyPath = ", wssKeyPath)
	// create websocket mqtt server
	if len(wsAddr) > 0 || len(wssAddr) > 0 {
		addr := conf.MQTTServer
		AddWebsocketHandler("/mqtt", addr)
		/* start a plain websocket listener */
		if len(wsAddr) > 0 {
			go ListenAndServeWebsocket(wsAddr)
		}
		/* start a secure websocket listener */
		if len(wssAddr) > 0 && len(wssCertPath) > 0 && len(wssKeyPath) > 0 {
			go ListenAndServeWebsocketSecure(wssAddr, wssCertPath, wssKeyPath)
		}
	}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	log.Infof("signal received signal %v", <-ch)
	if err = svr.Close(); err != nil {
		log.Error("Couldn't shutdown server", err)
	}
	//集群关闭
	return nil
}

type ContextHook struct{}

func (hook ContextHook) Levels() []log.Level {
	return log.AllLevels
}

func (hook ContextHook) Fire(entry *log.Entry) error {
	pc := make([]uintptr, 3, 3)
	cnt := runtime.Callers(7, pc)
	for i := 0; i < cnt; i++ {
		fu := runtime.FuncForPC(pc[i] - 1)
		name := fu.Name()
		if !strings.Contains(name, "surgemq/vendor/github.com/Sirupsen/logrus") {
			file, line := fu.FileLine(pc[i] - 1)
			flieName := path.Base(file)
			entry.Data["File"] = flieName
			funcNames := strings.SplitAfterN(name, ".", strings.LastIndex(name, "."))
			length := len(funcNames)
			entry.Data["Func"] = funcNames[length-1]
			entry.Data["Line"] = line
			break
		}
	}
	return nil
}

func main() {
	//log.AddHook(ContextHook{})
	app := cli.NewApp()
	app.Name = "surpermq"
	app.Usage = "surpermq: mqtt server"
	app.Copyright = "lowan@lowan-cn.com "
	app.Version = Version + " SVNVersion@" + SVNVersion + " build@" + TimeStamp
	app.Action = run
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "conf,c",
			Usage:  "Set conf path here",
			Value:  "appserver.conf",
			EnvVar: "APP_CONF",
		},
	}
	app.Run(os.Args)
}
