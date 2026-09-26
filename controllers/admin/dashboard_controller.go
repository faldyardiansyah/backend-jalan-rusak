package admin

import (
	"net/http"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetDashboardStats(c *gin.Context) {
	roleVal, existsRole := c.Get("role")
	userIDVal, existsUserID := c.Get("user_id")

	if !existsRole || !existsUserID {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "User tidak terautentikasi",
		})
		return
	}

	role, ok := roleVal.(string)
	if !ok || role == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Role tidak valid",
		})
		return
	}

	var userID uint
	switch v := userIDVal.(type) {
	case uint:
		userID = v
	case float64:
		userID = uint(v)
	case int:
		userID = uint(v)
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "User ID tidak valid",
		})
		return
	}

	var totalLaporan int64
	var totalMenunggu int64
	var totalProses int64
	var totalSelesai int64
	var totalDitolak int64

	baseQuery := config.DB.
		Model(&models.LaporanKerusakan{}).
		Where("laporan_kerusakan.deleted_at IS NULL")

	switch role {
	case string(models.RoleAdminPemdes):
		var adminUser models.User

		if err := config.DB.First(&adminUser, userID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"status":  "error",
				"message": "Data admin tidak ditemukan",
			})
			return
		}

		if adminUser.WilayahID == nil {
			c.JSON(http.StatusForbidden, gin.H{
				"status":  "error",
				"message": "Admin Pemdes belum memiliki wilayah",
			})
			return
		}

		baseQuery = baseQuery.Where(
			"laporan_kerusakan.wilayah_id = ? AND laporan_kerusakan.jenis_jalan = ?",
			*adminUser.WilayahID,
			"desa",
		)

	case string(models.RoleAdminPu):
		baseQuery = baseQuery.Where("laporan_kerusakan.jenis_jalan = ?", "kabupaten")

	case string(models.RoleSuperAdmin):
		// Superadmin menghitung seluruh laporan tanpa filter wilayah dan jenis jalan

	default:
		c.JSON(http.StatusForbidden, gin.H{
			"status":  "error",
			"message": "Akses tidak diizinkan",
		})
		return
	}

	if err := baseQuery.Count(&totalLaporan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menghitung total laporan",
		})
		return
	}

	if err := baseQuery.Session(&gorm.Session{}).
		Where("laporan_kerusakan.status = ?", "menunggu").
		Count(&totalMenunggu).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menghitung laporan menunggu",
		})
		return
	}

	if err := baseQuery.Session(&gorm.Session{}).
		Where("laporan_kerusakan.status = ?", "proses").
		Count(&totalProses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menghitung laporan proses",
		})
		return
	}

	if err := baseQuery.Session(&gorm.Session{}).
		Where("laporan_kerusakan.status = ?", "selesai").
		Count(&totalSelesai).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menghitung laporan selesai",
		})
		return
	}

	if err := baseQuery.Session(&gorm.Session{}).
		Where("laporan_kerusakan.status = ?", "ditolak").
		Count(&totalDitolak).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menghitung laporan ditolak",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Dashboard stats berhasil diambil",
		"data": gin.H{
			"total_laporan":  totalLaporan,
			"total_menunggu": totalMenunggu,
			"total_proses":   totalProses,
			"total_selesai":  totalSelesai,
			"total_ditolak":  totalDitolak,
		},
	})
}
