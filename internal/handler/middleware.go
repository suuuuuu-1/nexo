package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/suuuuu/nexo/internal/model"
	"github.com/suuuuu/nexo/internal/service"
)

const (
	// 鉴权中间件将用户上下文写入 Gin Context，Handler 不直接解析 JWT。
	contextUserID = "user_id"
	contextRole   = "role"
)

// AuthRequired 校验 Bearer Token，并从数据库确认用户仍然处于 ACTIVE 状态。
// 这样被禁用的用户即使持有未过期 Token，也不能继续访问受保护接口。
func AuthRequired(userService *service.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}

		claims, err := userService.ParseToken(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		current, err := userService.GetByID(c.Request.Context(), claims.UserID)
		if err != nil || current.Status != model.StatusActive {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user is not active"})
			return
		}

		c.Set(contextUserID, current.ID)
		c.Set(contextRole, string(current.Role))
		c.Next()
	}
}

// RequireRoles 限制当前路由允许访问的角色。
// 角色权限集中在路由组上，避免在每个 Handler 内重复判断。
func RequireRoles(roles ...model.Role) gin.HandlerFunc {
	allowed := make(map[model.Role]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}
	return func(c *gin.Context) {
		value, exists := c.Get(contextRole)
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		role, ok := value.(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid role"})
			return
		}
		if _, ok := allowed[model.Role(role)]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "permission denied"})
			return
		}
		c.Next()
	}
}

// CurrentUserID 从请求上下文读取已通过鉴权的用户 ID。
func CurrentUserID(c *gin.Context) (string, bool) {
	value, exists := c.Get(contextUserID)
	if !exists {
		return "", false
	}
	id, ok := value.(string)
	return id, ok
}
