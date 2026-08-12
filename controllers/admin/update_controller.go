package admin

import (
	"net/http"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
)

func UpdateStatusLaporan(c *gin.Context){
	id := c.Param("id")
	roleVal, _ := c.Get("role")
	userIDVal, _ := c.Get("user_id")
	role := roleVal.(models.UserRole)
	userID := userIDVal.(uint)

	var laporan models.LaporanKerusakan
	if err := config.DB.First(&laporan, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Laporan kerusakan tidak ditemukan",
		})
		return
	}

	// untuk admin pemdes ini cuman bole update laporan dari wilayahnya saja
	if role == models.RoleAdminPemdes {
		var adminUser models.User
		config.DB.First(&adminUser, userID)

		if adminUser.WilayahID == nil || laporan.WilayahID != *adminUser.WilayahID || laporan.JenisJalan != "desa" {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Anda tidak memiliki akses untuk mengubah laporan ini",
			})
			return
		}
	}

	// ini buat admin_pu dia bisa keduanya
	if role == models.RoleAdminPu && laporan.JenisJalan != "kabupaten" {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Anda tidak memiliki akses untuk mengubah laporan ini",
		})
		return
	}

	status := c.PostForm("status")
	ditugaskanKe := c.PostForm("ditugaskan_ke")
	catatanAdmin := c.PostForm("catatan_admin")

	if status != "" {
		if status != "menunggu" && status != "proses" && status != "selesai" && status != "ditolak" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Status tidak valid",
			})
			return
		}
		laporan.Status = status
	}

	if ditugaskanKe != "" {
		laporan.DitugaskanKe = ditugaskanKe
	}
	if catatanAdmin != "" {
		laporan.CatatanAdmin = catatanAdmin
	}
	
	fileHeader, err := c.FormFile("foto_bukti")
	if err == nil {
		fotoBukti, errUpload := utils.UploadCloudinary(fileHeader)
		if errUpload != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Gagal upload foto bukti",
			})
			return
		}
		laporan.FotoBukti = fotoBukti
	}

	// notifikasi
	if status != "" {
		config.DB.Create(&models.Notifikasi{
			UserID:    laporan.UserID,
			LaporanID: laporan.ID,
			Judul : "Status Laporan Berubah",
			Pesan : "Status laporan telah diubah oleh admin",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Laporan kerusakan berhasil diperbarui",
		"data":    laporan,
	})
}