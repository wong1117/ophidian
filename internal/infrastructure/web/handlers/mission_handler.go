package handlers

import (
	"net/http"
	"github.com/labstack/echo/v4"
)

type MissionHandler struct{}

func NewMissionHandler() *MissionHandler {
	return &MissionHandler{}
}

func (h *MissionHandler) Create(c echo.Context) error {
	return c.JSON(http.StatusNotImplemented, map[string]string{"error": "not implemented"})
}

func (h *MissionHandler) Get(c echo.Context) error {
	return c.JSON(http.StatusNotImplemented, map[string]string{"error": "not implemented"})
}

func (h *MissionHandler) List(c echo.Context) error {
	return c.JSON(http.StatusNotImplemented, map[string]string{"error": "not implemented"})
}

func (h *MissionHandler) Start(c echo.Context) error {
	return c.JSON(http.StatusNotImplemented, map[string]string{"error": "not implemented"})
}

func (h *MissionHandler) Abort(c echo.Context) error {
	return c.JSON(http.StatusNotImplemented, map[string]string{"error": "not implemented"})
}
