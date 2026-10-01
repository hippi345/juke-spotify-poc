package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const contextUserIDKey = "auth_user_id"
const contextRoleKey = "auth_role"

// Middleware validates Bearer token when present and stores claims on context.
func Middleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if h == "" {
			c.Next()
			return
		}
		const prefix = "Bearer "
		if !strings.HasPrefix(h, prefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header"})
			return
		}
		claims, err := ParseToken(secret, strings.TrimPrefix(h, prefix))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		c.Set(contextUserIDKey, claims.UserID)
		c.Set(contextRoleKey, claims.Role)
		c.Next()
	}
}

// UserIDFromContext returns authenticated user id or 0.
func UserIDFromContext(c *gin.Context) uint {
	v, ok := c.Get(contextUserIDKey)
	if !ok {
		return 0
	}
	id, ok := v.(uint)
	if !ok {
		return 0
	}
	return id
}

// RoleFromContext returns role or empty string.
func RoleFromContext(c *gin.Context) string {
	v, ok := c.Get(contextRoleKey)
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// RequireRole aborts unless the request has the given role.
func RequireRole(c *gin.Context, role string) bool {
	if RoleFromContext(c) != role {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return false
	}
	return true
}
