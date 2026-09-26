package controllers

import (
	"net/http"
	"strings"
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

type AdminInboxItem struct {
	LaporanID            uint   `json:"laporan_id"`
	JudulLaporan         string `json:"judul_laporan"`
	JenisJalan           string `json:"jenis_jalan"`
	StatusLaporan        string `json:"status_laporan"`
	WilayahID            uint   `json:"wilayah_id"`
	NamaWilayah          string `json:"nama_wilayah"`
	UserID               uint   `json:"user_id"`
	NamaWarga            string `json:"nama_warga"`
	ProfilePhoto         string `json:"profile_photo"`
	IsiPesanTerakhir     string `json:"isi_pesan_terakhir"`
	WaktuPesanTerakhir   string `json:"waktu_pesan_terakhir"`
	MenungguBalasanAdmin bool   `json:"menunggu_balasan_admin"`
	TotalPesan           int    `json:"total_pesan"`
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

func getAuthContext(c *gin.Context) (string, uint, bool) {
	roleVal, existsRole := c.Get("role")
	userIDVal, existsUserID := c.Get("user_id")

	if !existsRole || !existsUserID {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "User tidak terautentikasi",
		})
		return "", 0, false
	}

	role, ok := roleVal.(string)
	if !ok || role == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Role tidak valid",
		})
		return "", 0, false
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
		return "", 0, false
	}

	return role, userID, true
}

func GetChatByLaporanID(c *gin.Context) {
	laporanID := c.Param("id")
	roleStr, userID, ok := getAuthContext(c)
	if !ok {
		return
	}

	var laporan models.LaporanKerusakan
	if err := config.DB.Where("deleted_at IS NULL").First(&laporan, laporanID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Laporan tidak ditemukan",
		})
		return
	}

	if !utils.CekAksesLaporan(roleStr, userID, laporan) {
		c.JSON(http.StatusForbidden, gin.H{
			"status":  "error",
			"message": "Anda tidak memiliki akses ke laporan ini",
		})
		return
	}

	var listChat []models.RiwayatChat
	if err := config.DB.Preload("User").Preload("Admin").
		Where("laporan_kerusakan_id = ? AND deleted_at IS NULL", laporan.ID).
		Order("created_at ASC").
		Find(&listChat).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil riwayat chat",
		})
		return
	}

	responseData := make([]ChatResponse, 0)
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
	roleStr, userID, ok := getAuthContext(c)
	if !ok {
		return
	}

	if roleStr != string(models.RoleWarga) {
		c.JSON(http.StatusForbidden, gin.H{
			"status":  "error",
			"message": "Hanya warga yang dapat mengirim pesan melalui endpoint ini",
		})
		return
	}

	var input struct {
		Pesan string `json:"pesan" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Pesan tidak boleh kosong",
		})
		return
	}

	trimmedPesan := strings.TrimSpace(input.Pesan)
	if trimmedPesan == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Pesan tidak boleh hanya berisi spasi kosong",
		})
		return
	}

	if len(trimmedPesan) > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Pesan terlalu panjang (maksimal 1000 karakter)",
		})
		return
	}

	var laporan models.LaporanKerusakan
	if err := config.DB.Where("deleted_at IS NULL").First(&laporan, laporanID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Laporan tidak ditemukan",
		})
		return
	}

	if !utils.CekAksesLaporan(string(models.RoleWarga), userID, laporan) {
		c.JSON(http.StatusForbidden, gin.H{
			"status":  "error",
			"message": "Ini bukan laporan milik anda",
		})
		return
	}

	newChat := models.RiwayatChat{
		LaporanKerusakanID: laporan.ID,
		UserID:             userID,
		Pesan:              trimmedPesan,
	}

	if err := config.DB.Create(&newChat).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengirim pesan",
		})
		return
	}

	if err := config.DB.Preload("User").First(&newChat, newChat.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal memuat pesan yang dikirim",
		})
		return
	}

	KirimNotifikasiChatWarga(laporan)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Pesan berhasil dikirim",
		"data":    FormatChatToResponse(newChat),
	})
}

func ReplyPesanAdmin(c *gin.Context) {
	chatID := c.Param("chat_id")
	roleStr, adminID, ok := getAuthContext(c)
	if !ok {
		return
	}

	var input struct {
		Balasan string `json:"balasan" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Balasan tidak boleh kosong"})
		return
	}

	trimmedBalasan := strings.TrimSpace(input.Balasan)
	if trimmedBalasan == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Balasan tidak boleh hanya berisi spasi kosong"})
		return
	}

	if len(trimmedBalasan) > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Balasan terlalu panjang (maksimal 1000 karakter)"})
		return
	}

	var chat models.RiwayatChat
	if err := config.DB.Where("deleted_at IS NULL").First(&chat, chatID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data chat tidak ditemukan"})
		return
	}

	var laporan models.LaporanKerusakan
	if err := config.DB.Where("deleted_at IS NULL").First(&laporan, chat.LaporanKerusakanID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Laporan terkait chat ini tidak ditemukan"})
		return
	}

	if !utils.CekAksesLaporan(roleStr, adminID, laporan) {
		c.JSON(http.StatusForbidden, gin.H{"status": "error", "message": "Anda tidak memiliki wewenang membalas chat di wilayah/jenis jalan ini"})
		return
	}

	now := time.Now()
	chat.AdminID = &adminID
	chat.Balasan = &trimmedBalasan
	chat.DibalasAt = &now

	if err := config.DB.Save(&chat).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan balasan chat"})
		return
	}

	if err := config.DB.Preload("User").Preload("Admin").First(&chat, chat.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat balasan chat"})
		return
	}

	KirimNotifikasiBalasanAdmin(laporan)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Balasan berhasil dikirim oleh Admin",
		"data":    FormatChatToResponse(chat),
	})
}

func GetAdminInbox(c *gin.Context) {
	roleStr, userID, ok := getAuthContext(c)
	if !ok {
		return
	}

	query := config.DB.Table("riwayat_chat").
		Joins("JOIN laporan_kerusakan ON laporan_kerusakan.id = riwayat_chat.laporan_kerusakan_id").
		Where("riwayat_chat.deleted_at IS NULL").
		Where("laporan_kerusakan.deleted_at IS NULL")

	switch roleStr {
	case string(models.RoleAdminPemdes):
		var adminUser models.User
		if err := config.DB.First(&adminUser, userID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"status":  "error",
				"message": "Data admin tidak ditemukan",
			})
			return
		}
		if adminUser.WilayahID == nil {
			c.JSON(http.StatusForbidden, gin.H{
				"status":  "error",
				"message": "Admin Pemdes belum memiliki wilayah",
			})
			return
		}
		query = query.Where("laporan_kerusakan.wilayah_id = ? AND laporan_kerusakan.jenis_jalan = ?", *adminUser.WilayahID, "desa")

	case string(models.RoleAdminPu):
		query = query.Where("laporan_kerusakan.jenis_jalan = ?", "kabupaten")

	case string(models.RoleSuperAdmin):
		// Superadmin melihat seluruh percakapan

	default:
		c.JSON(http.StatusForbidden, gin.H{
			"status":  "error",
			"message": "Akses tidak diizinkan",
		})
		return
	}

	var reportIDs []uint
	if err := query.Distinct("riwayat_chat.laporan_kerusakan_id").Pluck("riwayat_chat.laporan_kerusakan_id", &reportIDs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil daftar percakapan",
		})
		return
	}

	inboxList := make([]AdminInboxItem, 0)
	if len(reportIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "Daftar percakapan berhasil diambil",
			"data":    inboxList,
		})
		return
	}

	var reports []models.LaporanKerusakan
	if err := config.DB.Preload("User").Preload("Wilayah").Where("id IN ?", reportIDs).Find(&reports).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil detail laporan percakapan",
		})
		return
	}

	var allChats []models.RiwayatChat
	if err := config.DB.Where("laporan_kerusakan_id IN ? AND deleted_at IS NULL", reportIDs).Order("created_at ASC").Find(&allChats).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil riwayat pesan",
		})
		return
	}

	chatsByReport := make(map[uint][]models.RiwayatChat)
	for _, ch := range allChats {
		chatsByReport[ch.LaporanKerusakanID] = append(chatsByReport[ch.LaporanKerusakanID], ch)
	}

	for _, rep := range reports {
		repChats := chatsByReport[rep.ID]
		if len(repChats) == 0 {
			continue
		}

		totalPesan := len(repChats)
		lastChat := repChats[totalPesan-1]

		isiPesanTerakhir := lastChat.Pesan
		waktuPesanTerakhir := utils.FormatTanggalIndo(&lastChat.CreatedAt)
		if lastChat.Balasan != nil && *lastChat.Balasan != "" && lastChat.DibalasAt != nil {
			isiPesanTerakhir = *lastChat.Balasan
			waktuPesanTerakhir = utils.FormatTanggalIndo(lastChat.DibalasAt)
		}

		menungguBalasan := false
		for _, ch := range repChats {
			if ch.Balasan == nil || *ch.Balasan == "" {
				menungguBalasan = true
				break
			}
		}

		namaWilayah := ""
		if rep.Wilayah.Nama != "" {
			namaWilayah = rep.Wilayah.Nama
		}

		inboxList = append(inboxList, AdminInboxItem{
			LaporanID:            rep.ID,
			JudulLaporan:         rep.Judul,
			JenisJalan:           rep.JenisJalan,
			StatusLaporan:        rep.Status,
			WilayahID:            rep.WilayahID,
			NamaWilayah:          namaWilayah,
			UserID:               rep.UserID,
			NamaWarga:            rep.User.Name,
			ProfilePhoto:         rep.User.ProfilePhoto,
			IsiPesanTerakhir:     isiPesanTerakhir,
			WaktuPesanTerakhir:   waktuPesanTerakhir,
			MenungguBalasanAdmin: menungguBalasan,
			TotalPesan:           totalPesan,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Daftar percakapan berhasil diambil",
		"data":    inboxList,
	})
}
