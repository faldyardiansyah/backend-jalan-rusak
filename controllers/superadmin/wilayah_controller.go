package superadmin

import (
	"net/http"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"

	"github.com/gin-gonic/gin"
)

func GetAllWilayah(c *gin.Context) {
	var wilayahList []models.Wilayah
	config.DB.Order("nama ASC").Find(&wilayahList)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   wilayahList,
	})
}

type CreateWilayahInput struct {
	Nama string `json:"nama" binding:"required"`
	Tipe string `json:"tipe" binding:"required"`
}

func CreateWilayah(c *gin.Context) {
	var input CreateWilayahInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid: " + err.Error()})
		return
	}

	if input.Tipe != "desa"  && input.Tipe != "kabupaten" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Tipe harus salah satu dari: desa dan kabupaten",
		})
		return
	}

	wilayah := models.Wilayah{Nama: input.Nama, Tipe: input.Tipe}

	if err := config.DB.Create(&wilayah).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menambahkan wilayah"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":  "success",
		"message": "Wilayah berhasil ditambahkan",
		"data":    wilayah,
	})
}

func UpdateWilayah(c *gin.Context) {
	id := c.Param("id")

	var wilayah models.Wilayah
	if err := config.DB.First(&wilayah, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Wilayah tidak ditemukan"})
		return
	}

	var input CreateWilayahInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid: " + err.Error()})
		return
	}

	wilayah.Nama = input.Nama
	wilayah.Tipe = input.Tipe

	if err := config.DB.Save(&wilayah).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui wilayah"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Wilayah berhasil diperbarui",
		"data":    wilayah,
	})
}

func DeleteWilayah(c *gin.Context) {
	id := c.Param("id")

	var wilayah models.Wilayah
	if err := config.DB.First(&wilayah, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Wilayah tidak ditemukan"})
		return
	}

	var jumlahUser int64
	config.DB.Model(&models.User{}).Where("wilayah_id = ?", id).Count(&jumlahUser)

	var jumlahLaporan int64
	config.DB.Model(&models.LaporanKerusakan{}).Where("wilayah_id = ?", id).Count(&jumlahLaporan)

	if jumlahUser > 0 || jumlahLaporan > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Wilayah tidak dapat dihapus karena masih digunakan oleh user atau laporan",
		})
		return
	}

	if err := config.DB.Delete(&wilayah).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus wilayah"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Wilayah berhasil dihapus",
	})
}