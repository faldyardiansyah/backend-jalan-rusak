package admin

import (
	"net/http"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetDashboardStats(c *gin.Context) {
	roleVal, _ := c.Get("role")
	userIDVal, _ := c.Get("user_id")

	role := roleVal.(string)
	userID := userIDVal.(uint)

	var totalLaporan int64
	var totalMenunggu int64
	var totalProses int64
	var totalSelesai int64
	var totalDitolak int64

	baseQuery := config.DB.
		Model(&models.LaporanKerusakan{}).
		Where("laporan_kerusakan.deleted_at IS NULL")

	if role == string(models.RoleAdminPemdes) {
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
	}

	baseQuery.Count(&totalLaporan)

	queryMenunggu := baseQuery.Session(&gorm.Session{})
	queryMenunggu.
		Where("laporan_kerusakan.status = ?", "menunggu").
		Count(&totalMenunggu)

	queryProses := baseQuery.Session(&gorm.Session{})
	queryProses.
		Where("laporan_kerusakan.status = ?", "proses").
		Count(&totalProses)

	querySelesai := baseQuery.Session(&gorm.Session{})
	querySelesai.
		Where("laporan_kerusakan.status = ?", "selesai").
		Count(&totalSelesai)

	queryDitolak := baseQuery.Session(&gorm.Session{})
	queryDitolak.
		Where("laporan_kerusakan.status = ?", "ditolak").
		Count(&totalDitolak)

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