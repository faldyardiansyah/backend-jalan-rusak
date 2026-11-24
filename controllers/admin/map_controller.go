package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
)

type MapPoint struct {
	ID            uint      `json:"id"`
	Judul         string    `json:"judul"`
	Latitude      float64   `json:"latitude"`
	Longitude     float64   `json:"longitude"`
	Status        string    `json:"status"`
	Severity      string    `json:"severity"`
	TipeKerusakan string    `json:"tipe_kerusakan"`
	JenisJalan    string    `json:"jenis_jalan"`
	ImageURL      string    `json:"image_url"`
	FotoBukti     string    `json:"foto_bukti"`
	CatatanAdmin  string    `json:"catatan_admin"`
	Name          string    `json:"name"`
	WilayahID     uint      `json:"wilayah_id"`
	WilayahNama   string    `json:"wilayah_nama"`
	CreatedAt     time.Time `json:"created_at"`
}

func GetMapLaporan(c *gin.Context) {
	severity := strings.ToLower(strings.TrimSpace(c.Query("severity")))
	if severity != "" && severity != "all" {
		switch severity {
		case "ringan", "sedang", "berat", "belum_dinilai":
			// valid
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Parameter severity tidak valid. Gunakan 'ringan', 'sedang', 'berat', atau 'belum_dinilai'",
			})
			return
		}
	}

	status := strings.ToLower(strings.TrimSpace(c.Query("status")))
	if status != "" && status != "all" {
		switch status {
		case "menunggu", "proses", "selesai", "ditolak":
			// valid
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Parameter status tidak valid. Gunakan 'menunggu', 'proses', 'selesai', atau 'ditolak'",
			})
			return
		}
	}

	jenisJalan := strings.ToLower(strings.TrimSpace(c.Query("jenis_jalan")))
	if jenisJalan != "" && jenisJalan != "all" {
		switch jenisJalan {
		case "desa", "kabupaten", "provinsi", "nasional":
			// valid
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Parameter jenis_jalan tidak valid. Gunakan 'desa', 'kabupaten', 'provinsi', atau 'nasional'",
			})
			return
		}
	}

	wIDStr := strings.TrimSpace(c.Query("wilayah_id"))
	var filterWilayahID uint
	if wIDStr != "" && wIDStr != "all" {
		wID, err := strconv.ParseUint(wIDStr, 10, 32)
		if err != nil || wID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Parameter wilayah_id tidak valid",
			})
			return
		}
		filterWilayahID = uint(wID)
	}

	if config.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Koneksi database tidak tersedia",
		})
		return
	}

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

	query := config.DB.
		Table("laporan_kerusakan").
		Select(`
			laporan_kerusakan.id,
			laporan_kerusakan.judul,
			laporan_kerusakan.latitude,
			laporan_kerusakan.longitude,
			laporan_kerusakan.status,
			COALESCE(NULLIF(laporan_kerusakan.severity, ''), 'belum_dinilai') as severity,
			laporan_kerusakan.tipe_kerusakan,
			laporan_kerusakan.jenis_jalan,
			laporan_kerusakan.image_url,
			laporan_kerusakan.foto_bukti,
			laporan_kerusakan.catatan_admin,
			COALESCE(user.name, 'Warga') as name,
			laporan_kerusakan.wilayah_id,
			COALESCE(wilayah.nama, '') as wilayah_nama,
			laporan_kerusakan.created_at
		`).
		Joins("LEFT JOIN user ON user.id = laporan_kerusakan.user_id AND user.deleted_at IS NULL").
		Joins("LEFT JOIN wilayah ON wilayah.id = laporan_kerusakan.wilayah_id AND wilayah.deleted_at IS NULL").
		Where("laporan_kerusakan.deleted_at IS NULL")

	switch role {
	case string(models.RoleAdminPemdes):
		var adminUser models.User

		if err := config.DB.Where("id = ? AND deleted_at IS NULL", userID).First(&adminUser).Error; err != nil {
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

		if filterWilayahID > 0 && filterWilayahID != *adminUser.WilayahID {
			c.JSON(http.StatusForbidden, gin.H{
				"status":  "error",
				"message": "Admin Pemdes tidak dapat mengakses wilayah di luar kewenangannya",
			})
			return
		}

		if jenisJalan != "" && jenisJalan != "all" && jenisJalan != "desa" {
			c.JSON(http.StatusForbidden, gin.H{
				"status":  "error",
				"message": "Admin Pemdes hanya memiliki kewenangan pada jalan desa",
			})
			return
		}

		query = query.Where(
			"laporan_kerusakan.wilayah_id = ? AND laporan_kerusakan.jenis_jalan = ?",
			*adminUser.WilayahID,
			"desa",
		)

	case string(models.RoleAdminPu):
		// PU-5.1: Admin PU memonitor seluruh kewenangan jalan (Desa, Kabupaten, Provinsi, Nasional)
		if jenisJalan != "" && jenisJalan != "all" {
			query = query.Where("laporan_kerusakan.jenis_jalan = ?", jenisJalan)
		}
		if filterWilayahID > 0 {
			query = query.Where("laporan_kerusakan.wilayah_id = ?", filterWilayahID)
		}

	case string(models.RoleSuperAdmin):
		// Superadmin melihat seluruh laporan aktif
		if jenisJalan != "" && jenisJalan != "all" {
			query = query.Where("laporan_kerusakan.jenis_jalan = ?", jenisJalan)
		}
		if filterWilayahID > 0 {
			query = query.Where("laporan_kerusakan.wilayah_id = ?", filterWilayahID)
		}

	default:
		c.JSON(http.StatusForbidden, gin.H{
			"status":  "error",
			"message": "Akses tidak diizinkan",
		})
		return
	}

	// Filter berdasarkan status jika dikirim
	if status != "" && status != "all" {
		query = query.Where(
			"laporan_kerusakan.status = ?",
			status,
		)
	}

	// Filter berdasarkan severity jika dikirim
	if severity != "" && severity != "all" {
		switch severity {
		case "ringan", "sedang", "berat":
			query = query.Where("laporan_kerusakan.severity = ?", severity)
		case "belum_dinilai":
			query = query.Where("laporan_kerusakan.severity = ? OR laporan_kerusakan.severity IS NULL OR laporan_kerusakan.severity = ''", "belum_dinilai")
		}
	}

	var rawPoints []MapPoint

	// Ambil data laporan
	if err := query.Scan(&rawPoints).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil data laporan untuk peta",
		})
		return
	}

	validPoints := make([]MapPoint, 0)
	for _, p := range rawPoints {
		if !utils.IsValidCoordinate(p.Latitude, p.Longitude) || (p.Latitude == 0 && p.Longitude == 0) {
			continue
		}
		if p.Severity == "" {
			p.Severity = "belum_dinilai"
		}
		validPoints = append(validPoints, p)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"total":  len(validPoints),
		"data":   validPoints,
	})
}
