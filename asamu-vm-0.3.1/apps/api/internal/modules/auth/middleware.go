package auth

import (
	"net/http"
	"strings"

	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func Middleware(service *Service, required bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
		if raw == "" {
			if required {
				httpx.Fail(c, httpx.NewError(http.StatusUnauthorized, "AUTH_REQUIRED", "请先登录"))
				return
			}
			c.Next()
			return
		}
		claims, err := service.TokenManager().Parse(raw)
		if err != nil {
			if required {
				httpx.Fail(c, httpx.NewError(http.StatusUnauthorized, "INVALID_ACCESS_TOKEN", "登录状态已失效"))
				return
			}
			c.Next()
			return
		}
		id, err := uuid.Parse(claims.Subject)
		if err != nil {
			httpx.Fail(c, httpx.NewError(http.StatusUnauthorized, "INVALID_ACCESS_TOKEN", "登录状态无效"))
			return
		}
		if err := service.ValidateAccess(c.Request.Context(), id, claims.TokenVersion); err != nil {
			if required {
				httpx.Fail(c, err)
				return
			}
			c.Next()
			return
		}
		c.Set("user_id", id)
		c.Set("roles", claims.Roles)
		c.Set("permissions", claims.Permissions)
		c.Next()
	}
}
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		values, _ := c.Get("permissions")
		permissions, _ := values.([]string)
		for _, candidate := range permissions {
			if candidate == "*" || candidate == permission {
				c.Next()
				return
			}
		}
		httpx.Fail(c, httpx.NewError(http.StatusForbidden, "PERMISSION_DENIED", "当前账号没有执行此操作的权限"))
	}
}
func RequireAnyRole(roles ...string) gin.HandlerFunc {
	accepted := map[string]bool{}
	for _, role := range roles {
		accepted[role] = true
	}
	return func(c *gin.Context) {
		values, _ := c.Get("roles")
		current, _ := values.([]string)
		for _, role := range current {
			if accepted[role] || role == "super_admin" {
				c.Next()
				return
			}
		}
		httpx.Fail(c, httpx.NewError(http.StatusForbidden, "ROLE_REQUIRED", "当前角色不能执行此操作"))
	}
}
