package middlewares

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// fungsinya ambil role dari authmiddleware
		userRole := c.GetString("role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Akses tidak diizinkan"})
			c.Abort()
			return
		}

		roleStr, ok := Role.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Role tidak valid"})
			c.Abort()
			return
		}

		// untuk pencocokan role user dengan role yang diizinkan
		for _, allowedRole := range allowedRoles {
			if roleStr == allowedRole {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "Akses tidak diizinkan: Anda tidak memiliki hak akses ini"})
		c.Abort()
	}
}