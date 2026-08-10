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
	role := roleVal.(models.UserRole)
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

	query := config.DB.Table("laporan_kerusakan").
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
			users.name,
			laporan_kerusakan.wilayah_id
		`).Joins("JOIN user ON user.id = laporan_kerusakan.user_id").
		Where("laporan_kerusakan.delete_at IS NULL")

		if role == "models.RoleAdminPemdes" {
			var adminUser models.User
			config.DB.First(&adminUser, userID)

			if adminUser.WilayahID == nil {
				c.JSON(http.StatusForbidden, gin.H{
					"error" : "Akun admin ini belom di set wilayahnya",
				})
				return 
			}

			query = query 
		}
}
