package admin

import (
	"net/http"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"

	"github.com/gin-gonic/gin"
)

func GetDashboardStats(c *gin.Context) {
	roleVal, _ := c.Get("role")
	userIDVal, _ := c.Get("user_id")
	role := roleVal.(string)
	userID := userIDVal.(uint)

	var totalLaporan, totalMenunggu, totalProses, totalSelesai, totalDitolak int64

	baseQuery := config.DB.Model(&models.LaporanKerusakan{}).
		joins({"JOIN users ON users.id = laporan_kerusakan.user_id"}).
		Where("laporan_kerusakan.delete_at IS NULL")
	
	if role == "admin_pemdes" {
		var adminUser models.User
		config.DB.First(&adminUser, userID)
		baseQuery = baseQuery.Where("users.domisili = ?", adminUser.Domisili)
	}

	baseQuery.Count(&totalLaporan)

	config.DB.Model(&models.LaporanKerusakan{}).
		joins({"JOIN users ON users.id = laporan_kerusakan.user_id"}).
		Where("laporan_kerusakan.delete_at IS NULL").
		Where("laporan_kerusakan.status = ?", "menunggu").
		Count(&totalMenunggu)

	config.DB.Model(&models.LaporanKerusakan{}).
		joins({"JOIN users ON users.id = laporan_kerusakan.user_id"}).
		Where("laporan_kerusakan.delete_at IS NULL").
		Where("laporan_kerusakan.status = ?", "proses").
		Count(&totalProses)

	config.DB.Model(&models.LaporanKerusakan{}).
		joins({"JOIN users ON users.id = laporan_kerusakan.user_id"}).
		Where("laporan_kerusakan.delete_at IS NULL").
		Where("laporan_kerusakan.status = ?", "selesai").
		Count(&totalSelesai)
	
	config.DB.Model(&models.LaporanKerusakan{}).
		joins({"JOIN users ON users.id = laporan_kerusakan.user_id"}).
		Where("laporan_kerusakan.delete_at IS NULL").
		Where("laporan_kerusakan.status = ?", "ditolak").
		Count(&totalDitolak)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Dashboard stats berhasil diambil",
		"data": gin.H{
			"total_laporan": totalLaporan,
			"total_menunggu": totalMenunggu,
			"total_proses": totalProses,
			"total_selesai": totalSelesai,
			"total_ditolak": totalDitolak,
		},
	})
}