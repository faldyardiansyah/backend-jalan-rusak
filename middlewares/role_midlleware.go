package middlewares

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {

		// Ambil role dari AuthMiddleware
		role, exists := c.Get("role")

		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Akses tidak diizinkan",
			})
			c.Abort()
			return
		}

		// Pastikan role berupa string
		roleStr, ok := role.(string)

		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Role tidak valid",
			})
			c.Abort()
			return
		}

		// Cocokkan role user dengan role yang diizinkan
		for _, allowedRole := range allowedRoles {
			if roleStr == allowedRole {
				c.Next()
				return
			}
		}

		// Role tidak memiliki akses
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Akses tidak diizinkan: Anda tidak memiliki hak akses ini",
		})
		c.Abort()
	}
}