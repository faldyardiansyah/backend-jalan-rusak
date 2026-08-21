package superadmin

import (
	"net/http"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
)

func DeleteLaporanSpam(c *gin.Context) {
	id := c.Param("id")

	var laporan models.LaporanKerusakan
	if err := config.DB.First(&laporan, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Laporan kerusakan tidak ditemukan",
		})
		return
	}

	if laporan.ImageURL != "" {
		_ = utils.DeleteCloudinary(laporan.ImageURL)
	}

	if laporan.FotoBukti != "" {
		_ = utils.DeleteCloudinary(laporan.FotoBukti)
	}

	if err := config.DB.Delete(&laporan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal menghapus laporan kerusakan",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Laporan '" + laporan.Judul + "' berhasil dihapus (ditandai spam)",
	})
}