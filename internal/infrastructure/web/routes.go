package web

import (
	"github.com/labstack/echo/v4"
	"github.com/ophidian/ophidian/internal/infrastructure/web/handlers"
)

func RegisterRoutes(e *echo.Echo, mh *handlers.MissionHandler, rh *handlers.ReconHandler,
	eh *handlers.ExploitHandler, ah *handlers.AIHandler, rph *handlers.ReportHandler) {

	api := e.Group("/api/v1")

	missions := api.Group("/missions")
	missions.POST("", mh.Create)
	missions.GET("", mh.List)
	missions.GET("/:id", mh.Get)
	missions.POST("/:id/start", mh.Start)
	missions.POST("/:id/abort", mh.Abort)

	recon := api.Group("/recon")
	recon.POST("/passive", rh.StartPassive)
	recon.POST("/active", rh.StartActive)
	recon.GET("/:id", rh.GetResults)

	exploit := api.Group("/exploit")
	exploit.POST("/match", eh.Match)
	exploit.POST("/execute", eh.Execute)
	exploit.GET("/sessions", eh.ListSessions)

	ai := api.Group("/ai")
	ai.POST("/plan", ah.GeneratePlan)
	ai.GET("/plan/:id", ah.GetPlan)
	ai.POST("/correlate", ah.Correlate)

	report := api.Group("/report")
	report.POST("/generate", rph.Generate)
	report.GET("/export/:format", rph.Export)
}
