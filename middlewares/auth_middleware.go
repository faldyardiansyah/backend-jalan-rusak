package middlewares

import (
	"net/http"
	"strings"

	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		authHeader := c.GetHeader("Authorization")

		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Unauthorized",
			})
			c.Abort()
			return
		}

		// Memotong Bearer untuk mengambil JWT
		tokenString := strings.Replace(authHeader, "Bearer ", "", 1)

		// Validasi token
		claims, err := utils.ValidateToken(tokenString)

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Token tidak valid atau kadaluarsa",
			})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)


		c.Set("role", string(claims.Role))

		c.Next()
	}
}