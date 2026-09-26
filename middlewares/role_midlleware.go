package middlewares

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {

		// Ambil role dari AuthMiddleware
		role, exists := c.Get("role")

		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Akses tidak diizinkan: Autentikasi diperlukan",
			})
			c.Abort()
			return
		}

		// Pastikan role berupa string non-kosong
		roleStr, ok := role.(string)

		if !ok || roleStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Akses tidak diizinkan: Role tidak valid atau kosong",
			})
			c.Abort()
			return
		}

		// Cocokkan role user dengan role yang diizinkan (case-insensitive)
		for _, allowedRole := range allowedRoles {
			if strings.EqualFold(roleStr, allowedRole) {
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
