package handlers

import (
	"net/http"
	"github.com/labstack/echo/v4"
)

type ReconHandler struct{}

func NewReconHandler() *ReconHandler {
	return &ReconHandler{}
}

func (h *ReconHandler) StartPassive(c echo.Context) error {
	return c.JSON(http.StatusNotImplemented, map[string]string{"error": "not implemented"})
}

func (h *ReconHandler) StartActive(c echo.Context) error {
	return c.JSON(http.StatusNotImplemented, map[string]string{"error": "not implemented"})
}

func (h *ReconHandler) GetResults(c echo.Context) error {
	return c.JSON(http.StatusNotImplemented, map[string]string{"error": "not implemented"})
}
