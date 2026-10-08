package middlewares

import (
	"errors"
	"net/http"
	"strings"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		authHeader := c.GetHeader("Authorization")

		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Unauthorized: Token tidak ditemukan",
				"error":   "Unauthorized: Token tidak ditemukan",
			})
			c.Abort()
			return
		}

		// Memotong dan memvalidasi prefix Bearer
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") || strings.TrimSpace(parts[1]) == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Format Authorization header tidak valid (harus 'Bearer <token>')",
				"error":   "Format Authorization header tidak valid (harus 'Bearer <token>')",
			})
			c.Abort()
			return
		}

		tokenString := strings.TrimSpace(parts[1])

		// Validasi token
		claims, err := utils.ValidateToken(tokenString)

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Token tidak valid atau kadaluarsa",
				"error":   "Token tidak valid atau kadaluarsa",
			})
			c.Abort()
			return
		}

		// Role validity check pada klaim token (fail-closed jika role asing atau tidak sah)
		tokenRoleStr := strings.ToLower(string(claims.Role))
		if tokenRoleStr != string(models.RoleWarga) &&
			tokenRoleStr != string(models.RoleAdminPemdes) &&
			tokenRoleStr != string(models.RoleAdminPu) &&
			tokenRoleStr != string(models.RoleSuperAdmin) {
			c.JSON(http.StatusForbidden, gin.H{
				"status":  "error",
				"message": "Role tidak valid atau tidak memiliki hak akses",
				"error":   "Role tidak valid atau tidak memiliki hak akses",
			})
			c.Abort()
			return
		}

		// Verifikasi keberadaan pengguna dan status aktif jika terhubung ke database
		if config.DB != nil {
			var user models.User
			err := config.DB.Select("id, role, wilayah_id, token_version").
				Where("id = ? AND deleted_at IS NULL", claims.UserID).
				Take(&user).Error

			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					c.JSON(http.StatusUnauthorized, gin.H{
						"status":  "error",
						"message": "Pengguna tidak ditemukan atau telah dinonaktifkan",
						"error":   "Pengguna tidak ditemukan atau telah dinonaktifkan",
					})
				} else {
					c.JSON(http.StatusUnauthorized, gin.H{
						"status":  "error",
						"message": "Gagal memverifikasi akun pengguna: kendala basis data",
						"error":   "Gagal memverifikasi akun pengguna: kendala basis data",
					})
				}
				c.Abort()
				return
			}

			// Verifikasi sesi aktif / token version revocation
			if claims.TokenVersion > 0 && user.TokenVersion > 0 && claims.TokenVersion != user.TokenVersion {
				c.JSON(http.StatusUnauthorized, gin.H{
					"status":  "error",
					"message": "Sesi telah berakhir atau telah dikeluarkan (token revoked)",
					"error":   "Sesi telah berakhir atau telah dikeluarkan (token revoked)",
				})
				c.Abort()
				return
			}

			// Sinkronisasi data otoritatif dari basis data
			claims.Role = user.Role
			claims.WilayahID = user.WilayahID
		}

		// Role validity check (fail-closed jika role asing atau tidak sah)
		roleStr := strings.ToLower(string(claims.Role))
		if roleStr != string(models.RoleWarga) &&
			roleStr != string(models.RoleAdminPemdes) &&
			roleStr != string(models.RoleAdminPu) &&
			roleStr != string(models.RoleSuperAdmin) {
			c.JSON(http.StatusForbidden, gin.H{
				"status":  "error",
				"message": "Role tidak valid atau tidak memiliki hak akses",
				"error":   "Role tidak valid atau tidak memiliki hak akses",
			})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("role", string(claims.Role))
		c.Set("wilayah_id", claims.WilayahID)

		c.Next()
	}
}
