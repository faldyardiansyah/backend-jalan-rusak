package admin

import (
	"net/http"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
)

func UpdateStatusLaporan(c *gin.Context) {
	id := c.Param("id")

	var laporan models.LaporanKerusakan
	if err := config.DB.First(&laporan, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Laporan kerusakan tidak ditemukan",
		})
		return
	}

	status := c.PostForm("status")
	ditugaskanKe := c.PostForm("ditugaskan_ke")
	catatanAdmin := c.PostForm("catatan_admin")

	if status != "" {
		if status != "menunggu" && status != "proses" && status != "selesai" && status != "ditolak" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Status harus dari: menunggu, proses, selesai, ditolak",
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
		fotoBukti, err := utils.UploadCloudinary(fileHeader)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Gagal mengunggah ke Cloudinary",
			})
			return
		}
		laporan.FotoBukti = fotoBuktiURL
	}

	if err := config.DB.Save(&laporan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal memperbarui laporan kerusakan",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Laporan kerusakan berhasil diperbarui",
		"data":    laporan,
	})
}