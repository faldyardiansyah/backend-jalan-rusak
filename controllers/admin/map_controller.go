package admin

import (
	"net/http"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"

	"github.com/gin-gonic/gin"
)

func GetMapLaporan(c *gin.Context) {
	roleVal, _ := c.Get("role")
	userIDVal, _ := c.Get("user_id")

	role := roleVal.(string)
	userID := userIDVal.(uint)

	type MapPoint struct {
		ID            uint    `json:"id"`
		Judul         string  `json:"judul"`
		Latitude      float64 `json:"latitude"`
		Longitude     float64 `json:"longitude"`
		Status        string  `json:"status"`
		TipeKerusakan string  `json:"tipe_kerusakan"`
		JenisJalan    string  `json:"jenis_jalan"`
		ImageURL      string  `json:"image_url"`
		FotoBukti     string  `json:"foto_bukti"`
		CatatanAdmin  string  `json:"catatan_admin"`
		Name          string  `json:"name"`
		WilayahID     uint    `json:"wilayah_id"`
	}

	var mapPoints []MapPoint

	query := config.DB.
		Table("laporan_kerusakan").
		Select(`
			laporan_kerusakan.id,
			laporan_kerusakan.judul,
			laporan_kerusakan.latitude,
			laporan_kerusakan.longitude,
			laporan_kerusakan.status,
			laporan_kerusakan.tipe_kerusakan,
			laporan_kerusakan.jenis_jalan,
			laporan_kerusakan.image_url,
			laporan_kerusakan.foto_bukti,
			laporan_kerusakan.catatan_admin,
			user.name,
			laporan_kerusakan.wilayah_id
		`).
		Joins("JOIN user ON user.id = laporan_kerusakan.user_id").
		Where("laporan_kerusakan.deleted_at IS NULL")

	// Admin Pemdes hanya melihat laporan
	// sesuai wilayahnya dan jenis jalan desa
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
				"message": "Akun admin ini belum di-set wilayahnya",
			})
			return
		}

		query = query.Where(
			"laporan_kerusakan.wilayah_id = ? AND laporan_kerusakan.jenis_jalan = ?",
			*adminUser.WilayahID,
			"desa",
		)
	}

	// Admin PU dan Super Admin tidak difilter berdasarkan wilayah

	// Filter berdasarkan jenis jalan jika dikirim
	if jenisJalan := c.Query("jenis_jalan"); jenisJalan != "" {
		query = query.Where(
			"laporan_kerusakan.jenis_jalan = ?",
			jenisJalan,
		)
	}

	// Filter berdasarkan status jika dikirim
	if status := c.Query("status"); status != "" {
		query = query.Where(
			"laporan_kerusakan.status = ?",
			status,
		)
	}

	// Ambil data laporan
	if err := query.Scan(&mapPoints).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil data laporan untuk peta",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"total":  len(mapPoints),
		"data":   mapPoints,
	})
}