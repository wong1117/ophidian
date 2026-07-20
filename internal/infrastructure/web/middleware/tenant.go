package middleware

import (
	"github.com/labstack/echo/v4"
	domainTenant "github.com/ophidian/ophidian/internal/domain/tenant"
)

func TenantContext() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			tenantID := c.Request().Header.Get("X-Tenant-ID")
			if tenantID == "" {
				tenantID = c.QueryParam("tenant_id")
			}
			if tenantID != "" {
				c.Set("tenant_id", tenantID)
				ctx := domainTenant.WithTenant(c.Request().Context(), tenantID)
				c.SetRequest(c.Request().WithContext(ctx))
			}
			return next(c)
		}
	}
}
