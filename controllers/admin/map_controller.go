package admin

import (
	"net/http"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
)

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

func GetMapLaporan(c *gin.Context) {
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

		query = query.Where(
			"laporan_kerusakan.wilayah_id = ? AND laporan_kerusakan.jenis_jalan = ?",
			*adminUser.WilayahID,
			"desa",
		)

	case string(models.RoleAdminPu):
		query = query.Where("laporan_kerusakan.jenis_jalan = ?", "kabupaten")

	case string(models.RoleSuperAdmin):
		// Superadmin melihat seluruh laporan aktif
		if jenisJalan := c.Query("jenis_jalan"); jenisJalan != "" {
			query = query.Where("laporan_kerusakan.jenis_jalan = ?", jenisJalan)
		}

	default:
		c.JSON(http.StatusForbidden, gin.H{
			"status":  "error",
			"message": "Akses tidak diizinkan",
		})
		return
	}

	// Filter berdasarkan status jika dikirim
	if status := c.Query("status"); status != "" {
		query = query.Where(
			"laporan_kerusakan.status = ?",
			status,
		)
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
		if !utils.IsValidCoordinate(p.Latitude, p.Longitude) {
			continue
		}
		validPoints = append(validPoints, p)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"total":  len(validPoints),
		"data":   validPoints,
	})
}
