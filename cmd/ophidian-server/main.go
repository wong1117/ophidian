package main

import (
	"log"
	"net/http"
	"net/http/pprof"
	"runtime"

	"github.com/labstack/echo/v4"
	"github.com/ophidian/ophidian/internal/infrastructure/web"
	"github.com/ophidian/ophidian/internal/infrastructure/web/handlers"
)

func main() {
	runtime.SetBlockProfileRate(1)
	runtime.SetMutexProfileFraction(1)

	server := web.NewServer(web.ServerDeps{
		HealthHandler:  handlers.NewHealthHandler(),
		MissionHandler: handlers.NewMissionHandler(nil, nil, nil),
		ReconHandler:   handlers.NewReconHandler(nil),
		ExploitHandler: handlers.NewExploitHandler(nil, nil, nil),
		AIHandler:      handlers.NewAIHandler(nil, nil),
		ReportHandler:  handlers.NewReportHandler(nil),
	})

	registerPprof(server.Echo)

	if err := server.Start(":8443"); err != nil {
		log.Fatal(err)
	}
}

func registerPprof(e *echo.Echo) {
	pprofGroup := e.Group("/debug/pprof")
	pprofGroup.GET("/", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	pprofGroup.GET("/heap", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	pprofGroup.GET("/goroutine", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	pprofGroup.GET("/block", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	pprofGroup.GET("/mutex", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	pprofGroup.GET("/threadcreate", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	pprofGroup.GET("/cmdline", echo.WrapHandler(http.HandlerFunc(pprof.Cmdline)))
	pprofGroup.GET("/profile", echo.WrapHandler(http.HandlerFunc(pprof.Profile)))
	pprofGroup.GET("/symbol", echo.WrapHandler(http.HandlerFunc(pprof.Symbol)))
	pprofGroup.GET("/trace", echo.WrapHandler(http.HandlerFunc(pprof.Trace)))

	getGroup := e.Group("/debug/pprof")
	getGroup.GET("/allocs", echo.WrapHandler(http.HandlerFunc(pprof.Handler("allocs").ServeHTTP)))
}
