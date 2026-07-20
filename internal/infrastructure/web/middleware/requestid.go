package middleware

import (
	"crypto/rand"
	"fmt"

	"github.com/labstack/echo/v4"
)

func RequestID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			reqID := c.Request().Header.Get("X-Request-ID")
			if reqID == "" {
				reqID = c.Request().Header.Get("X-Correlation-ID")
			}
			if reqID == "" {
				b := make([]byte, 8)
				_, _ = rand.Read(b)
				reqID = fmt.Sprintf("%x", b)
			}

			c.Response().Header().Set("X-Request-ID", reqID)
			c.Set("request_id", reqID)
			return next(c)
		}
	}
}
