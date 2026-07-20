package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
	domainRBAC "github.com/ophidian/ophidian/internal/domain/rbac"
)

func RequirePermission(policy domainRBAC.Policy, userProvider func(c echo.Context) (*domainRBAC.User, error), resource, action string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, err := userProvider(c)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
			}

			granted, err := policy.Evaluate(c.Request().Context(), user, resource, action)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "authorization check failed")
			}
			if !granted {
				return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions")
			}

			c.Set("rbac_user", user)
			return next(c)
		}
	}
}

func RequireRole(userProvider func(c echo.Context) (*domainRBAC.User, error), roleName string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, err := userProvider(c)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
			}

			for _, r := range user.Roles {
				if r == roleName {
					c.Set("rbac_user", user)
					return next(c)
				}
			}
			return echo.NewHTTPError(http.StatusForbidden, "role required: "+roleName)
		}
	}
}
