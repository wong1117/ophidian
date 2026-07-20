package middleware

import (
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

type MetricsCollector interface {
	IncrementCounter(name string, value int64)
	RecordDuration(name string, d time.Duration)
}

func Metrics(collector MetricsCollector) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if collector == nil {
				return next(c)
			}

			start := time.Now()
			err := next(c)

			status := c.Response().Status
			collector.IncrementCounter("http.requests.total", 1)
			collector.IncrementCounter("http.requests."+strconv.Itoa(status), 1)
			collector.RecordDuration("http.requests.duration", time.Since(start))

			if err != nil {
				collector.IncrementCounter("http.requests.errors", 1)
			}

			return err
		}
	}
}
