package middleware

import (
	"github.com/gin-gonic/gin"
)

// InstanceID adds X-Juke-Instance on every response (for load-balancer smoke tests).
func InstanceID(id string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if id != "" {
			c.Set("instance_id", id)
			c.Header("X-Juke-Instance", id)
		}
		c.Next()
	}
}
