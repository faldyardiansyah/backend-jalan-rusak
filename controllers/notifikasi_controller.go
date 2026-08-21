package controllers

import (
	"net/http"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"

	"github.com/gin-gonic/gin"
)

func GetNotifikasiUser(c *gin.Context) {
	userIDVal, _ := c.Get("user_id")
	userID := userIDVal.(uint)

	var listNotifikasi []models.Notifikasi
	if err := config.DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&listNotifikasi).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"message": "Gagal mengambil data notifikasi",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"message": "Data notifikasi berhasil diambil",
		"data": listNotifikasi,
	})
}