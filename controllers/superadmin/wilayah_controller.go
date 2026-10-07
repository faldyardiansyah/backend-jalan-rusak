package superadmin

import (
	"net/http"
	"strings"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"

	"github.com/gin-gonic/gin"
)

func GetAllWilayah(c *gin.Context) {
	wilayahList := make([]models.Wilayah, 0)
	if err := config.DB.Where("deleted_at IS NULL").Order("nama ASC").Find(&wilayahList).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil data wilayah",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   wilayahList,
	})
}

func ShowWilayah(c *gin.Context) {
	id := c.Param("id")

	var wilayah models.Wilayah
	if err := config.DB.Where("deleted_at IS NULL").First(&wilayah, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status": "error",
			"error":  "Wilayah tidak ditemukan",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   wilayah,
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

	nama := strings.TrimSpace(input.Nama)
	if nama == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Nama wilayah tidak boleh kosong",
		})
		return
	}
	if len(nama) > 250 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Nama wilayah maksimal 250 karakter",
		})
		return
	}

	tipe := strings.ToLower(strings.TrimSpace(input.Tipe))
	if tipe != "desa" && tipe != "kabupaten" && tipe != "provinsi" && tipe != "nasional" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Tipe harus salah satu dari: desa, kabupaten, provinsi, nasional",
		})
		return
	}

	// Cek duplicate wilayah (case-insensitive)
	var existing models.Wilayah
	if err := config.DB.Where("LOWER(nama) = ? AND LOWER(tipe) = ? AND deleted_at IS NULL", strings.ToLower(nama), tipe).First(&existing).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Wilayah dengan nama dan tipe tersebut sudah ada",
		})
		return
	}

	wilayah := models.Wilayah{Nama: nama, Tipe: tipe}

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
	if err := config.DB.Where("deleted_at IS NULL").First(&wilayah, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Wilayah tidak ditemukan"})
		return
	}

	var input CreateWilayahInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid: " + err.Error()})
		return
	}

	nama := strings.TrimSpace(input.Nama)
	if nama == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Nama wilayah tidak boleh kosong",
		})
		return
	}
	if len(nama) > 250 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Nama wilayah maksimal 250 karakter",
		})
		return
	}

	tipe := strings.ToLower(strings.TrimSpace(input.Tipe))
	if tipe != "desa" && tipe != "kabupaten" && tipe != "provinsi" && tipe != "nasional" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Tipe harus salah satu dari: desa, kabupaten, provinsi, nasional",
		})
		return
	}

	// Cek duplicate wilayah jika nama atau tipe berubah
	var existing models.Wilayah
	if err := config.DB.Where("LOWER(nama) = ? AND LOWER(tipe) = ? AND id != ? AND deleted_at IS NULL", strings.ToLower(nama), tipe, wilayah.ID).First(&existing).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Wilayah dengan nama dan tipe tersebut sudah ada",
		})
		return
	}

	wilayah.Nama = nama
	wilayah.Tipe = tipe

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
	if err := config.DB.Where("deleted_at IS NULL").First(&wilayah, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Wilayah tidak ditemukan"})
		return
	}

	var jumlahUser int64
	config.DB.Model(&models.User{}).Where("wilayah_id = ? AND deleted_at IS NULL", wilayah.ID).Count(&jumlahUser)

	var jumlahLaporan int64
	config.DB.Model(&models.LaporanKerusakan{}).Where("wilayah_id = ? AND deleted_at IS NULL", wilayah.ID).Count(&jumlahLaporan)

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
