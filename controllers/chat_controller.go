package controllers

import (
	"net/http"
	"time"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
)

type ChatResponse struct {
	ID                 uint         `json:"id"`
	LaporanKerusakanID uint         `json:"laporan_kerusakan_id"`
	UserID             uint         `json:"user_id"`
	User               models.User  `json:"user"`
	Pesan              string       `json:"pesan"`
	WaktuKirim         string       `json:"waktu_kirim"` // <--- Hari, Tgl, Jam Indonesia
	AdminID            *uint        `json:"admin_id"`
	Admin              *models.User `json:"admin,omitempty"`
	Balasan            *string      `json:"balasan"`
	WaktuBalas         string       `json:"waktu_balas"` // <--- Hari, Tgl, Jam Indonesia
}

// Helper untuk mengonversi Model RiwayatChat ke ChatResponse
func FormatChatToResponse(chat models.RiwayatChat) ChatResponse {
	return ChatResponse{
		ID:                 chat.ID,
		LaporanKerusakanID: chat.LaporanKerusakanID,
		UserID:             chat.UserID,
		User:               chat.User,
		Pesan:              chat.Pesan,
		WaktuKirim:         utils.FormatTanggalIndo(&chat.CreatedAt, "Asia/Jakarta"),
		AdminID:            chat.AdminID,
		Admin:              chat.Admin,
		Balasan:            chat.Balasan,
		WaktuBalas:         utils.FormatTanggalIndo(chat.DibalasAt, "Asia/Jakarta"),
	}
}

// 1. GET /api/laporan/:id/chat -> Ambil riwayat chat 1 laporan
func GetChatByLaporanID(c *gin.Context) {
	laporanID := c.Param("id")

	var listChat []models.RiwayatChat

	err := config.DB.
		Preload("User").
		Preload("Admin").
		Where("laporan_kerusakan_id = ?", laporanID).
		Order("created_at asc").
		Find(&listChat).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil riwayat chat",
		})
		return
	}

	var responseData []ChatResponse
	for _, item := range listChat {
		responseData = append(responseData, FormatChatToResponse(item))
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"total":  len(responseData),
		"data":   responseData,
	})
}

// 2. POST /api/laporan/:id/chat -> Warga kirim pesan/pertanyaan
func SendPesanWarga(c *gin.Context) {
	laporanID := c.Param("id")
	userIDVal, _ := c.Get("user_id")
	userID := userIDVal.(uint)

	var input struct {
		Pesan string `json:"pesan" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Pesan wajib diisi",
		})
		return
	}

	var laporan models.LaporanKerusakan
	if err := config.DB.First(&laporan, laporanID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Laporan tidak ditemukan",
		})
		return
	}

	newChat := models.RiwayatChat{
		LaporanKerusakanID: laporan.ID,
		UserID:             userID,
		Pesan:              input.Pesan,
	}

	if err := config.DB.Create(&newChat).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengirim pesan",
		})
		return
	}

	config.DB.Preload("User").First(&newChat, newChat.ID)

	c.JSON(http.StatusCreated, gin.H{
		"status":  "success",
		"message": "Pesan berhasil dikirim",
		"data":    FormatChatToResponse(newChat),
	})
}

// 3. PUT /api/admin/chat/:chat_id -> Admin mengisi balasan
func ReplyPesanAdmin(c *gin.Context) {
	chatID := c.Param("chat_id")
	adminIDVal, _ := c.Get("user_id")
	adminID := adminIDVal.(uint)

	var input struct {
		Balasan string `json:"balasan" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Balasan tidak boleh kosong",
		})
		return
	}

	var chat models.RiwayatChat
	if err := config.DB.First(&chat, chatID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Data chat tidak ditemukan",
		})
		return
	}

	now := time.Now()
	chat.AdminID = &adminID
	chat.Balasan = &input.Balasan
	chat.DibalasAt = &now

	if err := config.DB.Save(&chat).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menyimpan balasan admin",
		})
		return
	}

	config.DB.Preload("User").Preload("Admin").First(&chat, chat.ID)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Balasan berhasil dikirim oleh Admin",
		"data":    FormatChatToResponse(chat),
	})
}