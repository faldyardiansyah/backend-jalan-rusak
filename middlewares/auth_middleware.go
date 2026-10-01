package middlewares

import (
	"net/http"
	"strings"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		authHeader := c.GetHeader("Authorization")

		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Unauthorized: Token tidak ditemukan",
			})
			c.Abort()
			return
		}

		// Memotong dan memvalidasi prefix Bearer
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") || strings.TrimSpace(parts[1]) == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Format Authorization header tidak valid (harus 'Bearer <token>')",
			})
			c.Abort()
			return
		}

		tokenString := strings.TrimSpace(parts[1])

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
		c.Set("wilayah_id", claims.WilayahID)

		// Verifikasi sesi aktif / token version revocation jika terhubung ke database
		if claims.TokenVersion > 0 && config.DB != nil {
			var currentVersion uint
			err := config.DB.Model(&models.User{}).
				Select("token_version").
				Where("id = ? AND deleted_at IS NULL", claims.UserID).
				Scan(&currentVersion).Error

			if err == nil && currentVersion > 0 && claims.TokenVersion != currentVersion {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Sesi telah berakhir atau telah dikeluarkan (token revoked)",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}
