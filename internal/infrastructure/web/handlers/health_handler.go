package handlers

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type HealthHandler struct {
	startedAt time.Time
}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{startedAt: time.Now()}
}

type HealthResponse struct {
	Status    string `json:"status"`
	Uptime    string `json:"uptime"`
	Timestamp string `json:"timestamp"`
}

func (h *HealthHandler) Health(c echo.Context) error {
	return c.JSON(http.StatusOK, HealthResponse{
		Status:    "ok",
		Uptime:    time.Since(h.startedAt).String(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

type ReadinessResponse struct {
	Ready      bool              `json:"ready"`
	Components map[string]string `json:"components"`
}

func (h *HealthHandler) Ready(c echo.Context) error {
	return c.JSON(http.StatusOK, ReadinessResponse{
		Ready:      true,
		Components: map[string]string{"status": "healthy"},
	})
}
