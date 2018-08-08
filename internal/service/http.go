package service

import (
	"net/http/pprof"
	"runtime"
	ppf "runtime/pprof"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func StartHTTPServer(sv *Server, url string) {

	gin.SetMode("release")

	router := gin.Default()

	router.Use(func(ctx *gin.Context) {
		ctx.Set("server", sv)
	})

	router.GET("/ping", ping)

	//pprof
	router.GET("/pprof", status)
	router.GET("/debug/pprof/", func(ctx *gin.Context) { pprof.Index(ctx.Writer, ctx.Request) })
	router.GET("/debug/pprof/cmdline", func(ctx *gin.Context) { pprof.Cmdline(ctx.Writer, ctx.Request) })
	router.GET("/debug/pprof/symbol", func(ctx *gin.Context) { pprof.Symbol(ctx.Writer, ctx.Request) })
	router.POST("/debug/pprof/symbol", func(ctx *gin.Context) { pprof.Symbol(ctx.Writer, ctx.Request) })
	router.GET("/debug/pprof/profile", func(ctx *gin.Context) { pprof.Profile(ctx.Writer, ctx.Request) })
	router.GET("/debug/pprof/heap", func(ctx *gin.Context) { pprof.Handler("heap").ServeHTTP(ctx.Writer, ctx.Request) })
	router.GET("/debug/pprof/goroutine", func(ctx *gin.Context) { pprof.Handler("goroutine").ServeHTTP(ctx.Writer, ctx.Request) })
	router.GET("/debug/pprof/block", func(ctx *gin.Context) { pprof.Handler("block").ServeHTTP(ctx.Writer, ctx.Request) })
	router.GET("/debug/pprof/threadcreate", func(ctx *gin.Context) { pprof.Handler("threadcreate").ServeHTTP(ctx.Writer, ctx.Request) })

	router.Run(url)
}

func ping(ctx *gin.Context) {
	ctx.String(200, "Welcome to mqtt server.Timestamp: %s", strconv.FormatInt(time.Now().UnixNano(), 10))
}

func status(ctx *gin.Context) {
	profiles := ppf.Profiles()

	pprof_map := make(map[string]interface{})

	for _, p := range profiles {
		if p.Name() == "goroutine" || p.Name() == "threadcreate" {
			pprof_map[p.Name()] = p.Count()
		}
	}

	//mem
	s := new(runtime.MemStats)
	runtime.ReadMemStats(s)
	pprof_map["Mem"] = s.Sys
	ctx.JSON(200, pprof_map)
}
