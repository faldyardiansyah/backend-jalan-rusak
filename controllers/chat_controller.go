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
	WaktuKirim         string       `json:"waktu_kirim"`
	AdminID            *uint        `json:"admin_id"`
	Admin              *models.User `json:"admin,omitempty"`
	Balasan            *string      `json:"balasan"`
	WaktuBalas         string       `json:"waktu_balas"`
}

func FormatChatToResponse(chat models.RiwayatChat) ChatResponse {
	waktuBalas := ""

	if chat.DibalasAt != nil {
		waktuBalas = utils.FormatTanggalIndo(chat.DibalasAt)
	}

	return ChatResponse{
		ID:                 chat.ID,
		LaporanKerusakanID: chat.LaporanKerusakanID,
		UserID:             chat.UserID,
		User:               chat.User,
		Pesan:              chat.Pesan,
		WaktuKirim:         utils.FormatTanggalIndo(&chat.CreatedAt),
		AdminID:            chat.AdminID,
		Admin:              chat.Admin,
		Balasan:            chat.Balasan,
		WaktuBalas:         waktuBalas,
	}
}

func GetChatByLaporanID(c *gin.Context) {
	laporanID := c.Param("id")
	roleVal, existsRole := c.Get("role")
	userIDVal, existsUserID := c.Get("user_id")

	// kondisi dimana jika user tidak terauntentikasi
	if !existsRole || !existsUserID {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "User tidak terautentikasi",
		})
		return
	}

	roleStr, ok := roleVal.(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Role tidak valid",
		})
		return
	}

	// kondisi buat user id tidak valid
	userID, ok := userIDVal.(uint)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "User ID tidak valid",
		})
		return
	}

	// ini fungsi buat ambil data laoran
	var laporan models.LaporanKerusakan
	if err := config.DB.First(&laporan, laporanID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Laporan tidak ditemukan",
		})
		return
	}

	// ini buat cek hak aksesnya
	if roleStr == string(models.RoleWarga) && laporan.UserID != userID {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "Anda tidak memiliki akses ke laporan ini",
		})
		return
	}

	// mengambil riwayat chat
	var listChat []models.RiwayatChat
	config.DB.Preload("User").Preload("Admin").
		Where("laporan_kerusakan_id = ?", laporanID).
		Order("created_at ASC").
		Find(&listChat)

	var responseData []ChatResponse
	for _, chat := range listChat {
		responseData = append(responseData, FormatChatToResponse(chat))
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Riwayat chat berhasil diambil",
		"data":    responseData,
	})
}

func SendPesanWarga(c *gin.Context) {
	laporanID := c.Param("id")
	userIDVal, _ := c.Get("user_id")
	userID := userIDVal.(uint)

	var input struct {
		pesan string `json:"pesan" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Pesan tidak boleh kosong",
		})
		return
	}

	// ambil data laporan
	var laporan models.LaporanKerusakan
	if err := config.DB.First(&laporan, laporanID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Laporan tidak ditemukan",
		})
		return
	}

	// cek buat user id nya 
	if !utils.CekAksesLaporan(string(models.RoleWarga), userID, laporan) {
		c.JSON(http.StatusForbidden, gin.H{
			"status":  "error",
			"message": "Ini bukan laporan milik anda",
		})
		return
	}

	// buat chat baru
	newChat := models.RiwayatChat{
		LaporanKerusakanID: laporan.ID,
		UserID:             userID,
		Pesan:              input.pesan,
	}

	config.DB.Create(&newChat)
	config.DB.Preload("User").First(&newChat, newChat.ID)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Pesan berhasil dikirim",
		"data":    FormatChatToResponse(newChat),
	})
}

func ReplyPesanAdmin(c *gin.Context) {
	chatID := c.Param("chat_id")
	roleVal, _ := c.Get("role")
	adminIDVal, _ := c.Get("user_id")

	roleStr := roleVal.(string)
	adminID := adminIDVal.(uint)

	var input struct {
		Balasan string `json:"balasan" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Balasan tidak boleh kosong"})
		return
	}

	// Cari data chat
	var chat models.RiwayatChat
	if err := config.DB.First(&chat, chatID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data chat tidak ditemukan"})
		return
	}

	// Ambil data laporannya untuk dicek hak aksesnya
	var laporan models.LaporanKerusakan
	config.DB.First(&laporan, chat.LaporanKerusakanID)

	// 3. Validasi: Apakah admin ini berhak membalas laporan tersebut?
	if !utils.CekAksesLaporan(roleStr, adminID, laporan) {
		c.JSON(http.StatusForbidden, gin.H{"status": "error", "message": "Anda tidak memiliki wewenang membalas chat di wilayah/jenis jalan ini"})
		return
	}

	now := time.Now()
	chat.AdminID = &adminID
	chat.Balasan = &input.Balasan
	chat.DibalasAt = &now

	config.DB.Save(&chat)
	config.DB.Preload("User").Preload("Admin").First(&chat, chat.ID)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Balasan berhasil dikirim oleh Admin",
		"data":    FormatChatToResponse(chat),
	})
}