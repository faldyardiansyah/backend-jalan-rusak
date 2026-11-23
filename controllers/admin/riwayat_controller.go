package admin

import (
	"net/http"
	"strconv"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetRiwayatStatusLaporan mengambil catatan audit transisi status laporan.
// Hak akses:
// - Admin Pemdes: hanya laporan jalan desa dalam wilayah kewenangannya.
// - Admin PU & Superadmin: dapat melihat riwayat laporan yang dapat mereka akses.
// - Warga: dilarang (endpoint admin).
// Hasil diurutkan secara kronologis (paling lama ke terbaru, created_at ASC).
func GetRiwayatStatusLaporan(c *gin.Context) {
	idStr := c.Param("id")
	id, errID := strconv.ParseUint(idStr, 10, 32)
	if errID != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "ID laporan tidak valid",
			"error":   "ID laporan tidak valid",
		})
		return
	}

	roleVal, existsRole := c.Get("role")
	userIDVal, existsUserID := c.Get("user_id")
	if !existsRole || !existsUserID {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "User tidak terautentikasi",
			"error":   "User tidak terautentikasi",
		})
		return
	}

	var role string
	if rStr, ok := roleVal.(string); ok {
		role = rStr
	} else if rEnum, ok := roleVal.(models.UserRole); ok {
		role = string(rEnum)
	}

	var userID uint
	switch v := userIDVal.(type) {
	case uint:
		userID = v
	case float64:
		userID = uint(v)
	case int:
		userID = uint(v)
	}

	if userID == 0 || role == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Data autentikasi pengguna tidak valid",
		})
		return
	}

	if config.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Koneksi database tidak tersedia",
		})
		return
	}

	// Cek eksistensi dan otorisasi hak akses laporan
	var laporan models.LaporanKerusakan
	query := config.DB.Model(&models.LaporanKerusakan{}).
		Where("id = ? AND deleted_at IS NULL", id)

	switch role {
	case "admin_pemdes":
		var adminUser models.User
		if err := config.DB.First(&adminUser, userID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"status":  "error",
				"message": "Data admin tidak ditemukan",
			})
			return
		}

		if adminUser.WilayahID == nil || *adminUser.WilayahID == 0 {
			c.JSON(http.StatusForbidden, gin.H{
				"status":  "error",
				"message": "Admin Pemdes belum memiliki wilayah",
			})
			return
		}

		query = query.Where("wilayah_id = ? AND jenis_jalan = ?", *adminUser.WilayahID, "desa")
	case "admin_pu", "super_admin":
		// Admin PU dan Superadmin dapat melihat riwayat laporan
	default:
		c.JSON(http.StatusForbidden, gin.H{
			"status":  "error",
			"message": "Akses tidak diizinkan",
		})
		return
	}

	if err := query.First(&laporan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Laporan tidak ditemukan",
		})
		return
	}

	// Jika tabel riwayat belum aktif di database, kembalikan array kosong dengan 200 OK
	if !config.DB.Migrator().HasTable(&models.RiwayatStatusLaporan{}) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "Riwayat status laporan berhasil diambil (tabel audit belum aktif)",
			"data":    []models.RiwayatStatusLaporan{},
			"total":   0,
		})
		return
	}

	var riwayatList []models.RiwayatStatusLaporan
	err := config.DB.
		Where("laporan_id = ?", id).
		Preload("User", func(db *gorm.DB) *gorm.DB {
			return db.Select("id, name, email, role")
		}).
		Order("created_at ASC").
		Find(&riwayatList).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil riwayat status laporan: " + err.Error(),
		})
		return
	}

	// Pastikan data tidak bernilai nil agar serialize menjadi [] di JSON
	if riwayatList == nil {
		riwayatList = []models.RiwayatStatusLaporan{}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Riwayat status laporan berhasil diambil",
		"data":    riwayatList,
		"total":   len(riwayatList),
	})
}
