package middleware

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/ophidian/ophidian/internal/domain/common"
)

var statusMap = map[error]int{
	common.ErrInvalidID:          http.StatusBadRequest,
	common.ErrInvalidTarget:      http.StatusBadRequest,
	common.ErrInvalidScope:       http.StatusBadRequest,
	common.ErrInvalidState:       http.StatusUnprocessableEntity,
	common.ErrInvalidTransition:  http.StatusUnprocessableEntity,
	common.ErrMissionNotFound:    http.StatusNotFound,
	common.ErrTaskNotFound:       http.StatusNotFound,
	common.ErrPlanNotFound:       http.StatusNotFound,
	common.ErrSessionNotFound:    http.StatusNotFound,
	common.ErrRoEViolation:       http.StatusForbidden,
	common.ErrUnauthorized:       http.StatusUnauthorized,
	common.ErrForbidden:          http.StatusForbidden,
	common.ErrTimeout:            http.StatusGatewayTimeout,
	common.ErrCircuitOpen:        http.StatusServiceUnavailable,
	common.ErrDuplicateEvent:     http.StatusConflict,
	common.ErrConcurrencyConflict: http.StatusConflict,
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code"`
}

func ErrorHandler() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)
			if err == nil {
				return nil
			}

			var httpErr *echo.HTTPError
			if errors.As(err, &httpErr) {
				return c.JSON(httpErr.Code, ErrorResponse{
					Error:   http.StatusText(httpErr.Code),
					Message: httpErr.Message.(string),
					Code:    httpErr.Code,
				})
			}

			code := http.StatusInternalServerError
			for domainErr, httpCode := range statusMap {
				if errors.Is(err, domainErr) {
					code = httpCode
					break
				}
			}

			return c.JSON(code, ErrorResponse{
				Error:   http.StatusText(code),
				Message: err.Error(),
				Code:    code,
			})
		}
	}
}
