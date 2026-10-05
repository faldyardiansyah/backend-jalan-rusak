package controllers

import (
	"math"
	"net/http"
	"strconv"
	"strings"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"

	"github.com/gin-gonic/gin"
)

func getUserIDFromContext(c *gin.Context) (uint, bool) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "User tidak terautentikasi",
		})
		return 0, false
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
		return 0, false
	}

	return userID, true
}

func GetNotifikasiUser(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")

	page, errPage := strconv.Atoi(pageStr)
	if errPage != nil || page < 1 {
		page = 1
	}

	limit, errLimit := strconv.Atoi(limitStr)
	if errLimit != nil || limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offset := (page - 1) * limit

	var totalData int64
	if err := config.DB.Model(&models.Notifikasi{}).
		Where("user_id = ?", userID).
		Count(&totalData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menghitung data notifikasi",
		})
		return
	}

	var unreadCount int64
	if err := config.DB.Model(&models.Notifikasi{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&unreadCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menghitung notifikasi belum dibaca",
		})
		return
	}

	totalPages := 0
	if totalData > 0 {
		totalPages = int(math.Ceil(float64(totalData) / float64(limit)))
	}

	listNotifikasi := make([]models.Notifikasi, 0)
	if err := config.DB.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&listNotifikasi).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil data notifikasi",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":       "success",
		"message":      "Data notifikasi berhasil diambil",
		"data":         listNotifikasi,
		"unread_count": unreadCount,
		"meta": gin.H{
			"page":         page,
			"limit":        limit,
			"total":        totalData,
			"unread_count": unreadCount,
			"total_pages":  totalPages,
		},
	})
}

func MarkNotifikasiRead(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	notifID := c.Param("id")
	id, errID := strconv.ParseUint(notifID, 10, 32)
	if errID != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "ID notifikasi tidak valid",
		})
		return
	}

	var notifikasi models.Notifikasi
	if err := config.DB.Where("deleted_at IS NULL").First(&notifikasi, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Notifikasi tidak ditemukan",
		})
		return
	}

	// IDOR Protection: User hanya dapat menandai notifikasi miliknya sendiri
	if notifikasi.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"status":  "error",
			"message": "Anda tidak memiliki akses ke notifikasi ini",
		})
		return
	}

	// Idempotent: jika sudah true, langsung kembalikan sukses
	if notifikasi.IsRead {
		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "Notifikasi sudah ditandai sebagai dibaca",
			"data":    notifikasi,
		})
		return
	}

	notifikasi.IsRead = true
	if err := config.DB.Save(&notifikasi).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal memperbarui status notifikasi",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Notifikasi berhasil ditandai sebagai dibaca",
		"data":    notifikasi,
	})
}

func MarkAllNotifikasiRead(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	// Menggunakan batch UPDATE query langsung di database tanpa memuat data ke memory
	// GORM otomatis menerapkan deleted_at IS NULL dari gorm.Model
	if err := config.DB.Model(&models.Notifikasi{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Update("is_read", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal memperbarui status notifikasi",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Semua notifikasi berhasil ditandai sebagai dibaca",
	})
}

func DeleteNotifikasi(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	notifID := c.Param("id")
	id, errID := strconv.ParseUint(notifID, 10, 32)
	if errID != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "ID notifikasi tidak valid",
		})
		return
	}

	var notifikasi models.Notifikasi
	if err := config.DB.Where("deleted_at IS NULL").First(&notifikasi, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Notifikasi tidak ditemukan",
		})
		return
	}

	// IDOR Protection: User hanya dapat menghapus notifikasi miliknya sendiri
	if notifikasi.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"status":  "error",
			"message": "Anda tidak memiliki akses ke notifikasi ini",
		})
		return
	}

	// Soft delete menggunakan GORM (mengisi deleted_at, bukan physical delete)
	if err := config.DB.Delete(&notifikasi).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menghapus notifikasi",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Notifikasi berhasil dihapus",
	})
}

// KirimNotifikasiChatWarga mengirimkan notifikasi ke admin yang berwenang saat warga mengirim pesan baru
func KirimNotifikasiChatWarga(laporan models.LaporanKerusakan) {
	var adminTujuan []models.User

	switch strings.ToLower(laporan.JenisJalan) {
	case "desa":
		config.DB.
			Where("role = ? AND wilayah_id = ?", models.RoleAdminPemdes, laporan.WilayahID).
			Find(&adminTujuan)
	case "kabupaten":
		config.DB.
			Where("role = ?", models.RoleAdminPu).
			Find(&adminTujuan)
	}

	laporanIDStr := strconv.FormatUint(uint64(laporan.ID), 10)
	for _, admin := range adminTujuan {
		config.DB.Create(&models.Notifikasi{
			UserID:    admin.ID,
			LaporanID: laporan.ID,
			Judul:     "Pesan Baru Masuk",
			Pesan:     "Warga mengirim pesan pada laporan #" + laporanIDStr,
		})
	}
}

// KirimNotifikasiBalasanAdmin mengirimkan notifikasi ke warga pemilik laporan saat admin membalas pesan
func KirimNotifikasiBalasanAdmin(laporan models.LaporanKerusakan) {
	laporanIDStr := strconv.FormatUint(uint64(laporan.ID), 10)
	config.DB.Create(&models.Notifikasi{
		UserID:    laporan.UserID,
		LaporanID: laporan.ID,
		Judul:     "Balasan Pesan",
		Pesan:     "Admin membalas pesan pada laporan #" + laporanIDStr,
	})
}