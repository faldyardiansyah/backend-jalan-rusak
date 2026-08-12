package admin

import (
	"net/http"
	"strconv"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"
	
	"github.com/gin-gonic/gin"
)

func GetAllLaporan(c *gin.Context) {
	roleVal, _ := c.Get("role")
	userIDVal, _ := c.Get("user_id")
	role := roleVal.(string)
	userID := userIDVal.(uint)

	statusFilter := c.Query("status")
	searchKeyword := c.Query("search")
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)
	offset := (page - 1) * limit

	var listLaporan []models.LaporanKerusakan
	var totalData int64

	query := config.DB.Model(&models.LaporanKerusakan{}).
			Preload("User").
			Joins("JOIN user ON user.id = laporan_kerusakan.user_id").
			Where("laporan_kerusakan.deleted_at IS NULL")

	if role == "admin_pemdes" {
		var adminUser models.User
		config.DB.First(&adminUser, userID)
		query = query.Where("user.wilayah_id = ?", adminUser.WilayahID)
	}

	if statusFilter != "" {
		query = query.Where("laporan_kerusakan.status = ?", statusFilter)
	}

	if searchKeyword != "" {
		likePattern := "%" + searchKeyword + "%"
		query = query.Where("laporan_kerusakan.judul LIKE ? OR laporan_kerusakan.deskripsi LIKE ? OR user.name LIKE ?", likePattern, likePattern, likePattern)
	}

	query.Count(&totalData)

	err := query.Order("laporan_kerusakan.created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&listLaporan).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal mengambil laporan kerusakan",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Laporan kerusakan berhasil diambil",
		"data":    listLaporan,
		"total":   totalData,
	})
}

func GetDetailLaporan(c *gin.Context) {
	id := c.Param("id")

	var laporan models.LaporanKerusakan
	if err := config.DB.Preload("User").First(&laporan, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Laporan kerusakan tidak ditemukan",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Detail laporan berhasil diambil",
		"data":    laporan,
	})
}