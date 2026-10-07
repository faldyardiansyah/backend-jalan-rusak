package superadmin

import (
	"net/http"
	"strconv"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
)

func DeleteLaporanSpam(c *gin.Context) {
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

	var laporan models.LaporanKerusakan
	if err := config.DB.Where("id = ? AND deleted_at IS NULL", id).First(&laporan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Laporan kerusakan tidak ditemukan",
		})
		return
	}

	if err := config.DB.Delete(&laporan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal menghapus laporan kerusakan",
		})
		return
	}

	if laporan.ImageURL != "" {
		_ = utils.DeleteCloudinary(laporan.ImageURL)
	}

	if laporan.FotoBukti != "" {
		_ = utils.DeleteCloudinary(laporan.FotoBukti)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Laporan '" + laporan.Judul + "' berhasil dihapus (ditandai spam)",
	})
}