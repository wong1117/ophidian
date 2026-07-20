package web

import (
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/ophidian/ophidian/internal/infrastructure/web/handlers"
	"github.com/ophidian/ophidian/internal/infrastructure/web/middleware"
)

type ServerDeps struct {
	MissionHandler *handlers.MissionHandler
	ReconHandler   *handlers.ReconHandler
	ExploitHandler *handlers.ExploitHandler
	AIHandler      *handlers.AIHandler
	ReportHandler  *handlers.ReportHandler
	HealthHandler  *handlers.HealthHandler
	Metrics        middleware.MetricsCollector
}

type Server struct {
	Echo *echo.Echo
	deps ServerDeps
}

func NewServer(deps ServerDeps) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(middleware.RequestID())
	e.Use(middleware.Logging())
	e.Use(middleware.ErrorHandler())
	e.Use(middleware.Metrics(deps.Metrics))
	e.Use(echomw.CORS())
	e.Use(echomw.Recover())

	if deps.HealthHandler == nil {
		deps.HealthHandler = handlers.NewHealthHandler()
	}

	s := &Server{Echo: e, deps: deps}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	e := s.Echo

	e.GET("/health", s.deps.HealthHandler.Health)
	e.GET("/ready", s.deps.HealthHandler.Ready)

	api := e.Group("/api/v1")

	if s.deps.MissionHandler != nil {
		m := api.Group("/missions")
		m.POST("", s.deps.MissionHandler.Create)
		m.GET("", s.deps.MissionHandler.List)
		m.GET("/:id", s.deps.MissionHandler.Get)
		m.POST("/:id/start", s.deps.MissionHandler.Start)
		m.POST("/:id/abort", s.deps.MissionHandler.Abort)
	}

	if s.deps.ReconHandler != nil {
		r := api.Group("/recon")
		r.POST("/passive", s.deps.ReconHandler.StartPassive)
		r.POST("/active", s.deps.ReconHandler.StartActive)
		r.GET("/:id", s.deps.ReconHandler.GetResults)
	}

	if s.deps.ExploitHandler != nil {
		x := api.Group("/exploit")
		x.POST("/match", s.deps.ExploitHandler.Match)
		x.POST("/execute", s.deps.ExploitHandler.Execute)
		x.GET("/sessions", s.deps.ExploitHandler.ListSessions)
	}

	if s.deps.AIHandler != nil {
		a := api.Group("/ai")
		a.POST("/plan", s.deps.AIHandler.GeneratePlan)
		a.GET("/plan/:id", s.deps.AIHandler.GetPlan)
		a.POST("/correlate", s.deps.AIHandler.Correlate)
	}

	if s.deps.ReportHandler != nil {
		rp := api.Group("/report")
		rp.POST("/generate", s.deps.ReportHandler.Generate)
		rp.GET("/export/:format", s.deps.ReportHandler.Export)
	}
}

func (s *Server) Start(addr string) error {
	return s.Echo.Start(addr)
}
