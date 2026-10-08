package controllers

import (
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
)

const (
	MaxChatAttachmentSizeBytes = 5 * 1024 * 1024 // 5 MB
)

var allowedMimeTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

type LampiranBalasan struct {
	URL      string `json:"url"`
	Nama     string `json:"nama"`
	MimeType string `json:"mime_type"`
}

type ChatResponse struct {
	ID                 uint             `json:"id"`
	LaporanKerusakanID uint             `json:"laporan_kerusakan_id"`
	UserID             uint             `json:"user_id"`
	User               models.User      `json:"user"`
	Pesan              string           `json:"pesan"`
	WaktuKirim         string           `json:"waktu_kirim"`
	AdminID            *uint            `json:"admin_id"`
	Admin              *models.User     `json:"admin,omitempty"`
	Balasan            *string          `json:"balasan"`
	WaktuBalas         string           `json:"waktu_balas"`
	LampiranBalasan    *LampiranBalasan `json:"lampiran_balasan,omitempty"`
	LampiranBalasanURL *string          `json:"lampiran_balasan_url,omitempty"`
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

	var lampiran *LampiranBalasan
	if chat.LampiranBalasanURL != nil && *chat.LampiranBalasanURL != "" {
		nama := ""
		if chat.LampiranBalasanNama != nil {
			nama = *chat.LampiranBalasanNama
		}
		mime := ""
		if chat.LampiranBalasanMimeType != nil {
			mime = *chat.LampiranBalasanMimeType
		}
		lampiran = &LampiranBalasan{
			URL:      *chat.LampiranBalasanURL,
			Nama:     nama,
			MimeType: mime,
		}
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
		LampiranBalasan:    lampiran,
		LampiranBalasanURL: chat.LampiranBalasanURL,
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

	if role != string(models.RoleWarga) &&
		role != string(models.RoleAdminPemdes) &&
		role != string(models.RoleAdminPu) &&
		role != string(models.RoleSuperAdmin) {
		c.JSON(http.StatusForbidden, gin.H{
			"status":  "error",
			"message": "Role tidak dikenal atau tidak memiliki hak akses",
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

	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "User ID tidak valid",
		})
		return "", 0, false
	}

	return role, userID, true
}

func GetChatByLaporanID(c *gin.Context) {
	laporanIDStr := c.Param("id")
	laporanID, errID := strconv.ParseUint(laporanIDStr, 10, 32)
	if errID != nil || laporanID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "ID laporan tidak valid",
		})
		return
	}

	roleStr, userID, ok := getAuthContext(c)
	if !ok {
		return
	}

	if config.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Koneksi database tidak tersedia",
		})
		return
	}

	var laporan models.LaporanKerusakan
	if err := config.DB.Where("id = ? AND deleted_at IS NULL", laporanID).First(&laporan).Error; err != nil {
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
	laporanIDStr := c.Param("id")
	laporanID, errID := strconv.ParseUint(laporanIDStr, 10, 32)
	if errID != nil || laporanID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "ID laporan tidak valid",
		})
		return
	}

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

	if config.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Koneksi database tidak tersedia",
		})
		return
	}

	var laporan models.LaporanKerusakan
	if err := config.DB.Where("id = ? AND deleted_at IS NULL", laporanID).First(&laporan).Error; err != nil {
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
	chatIDStr := c.Param("chat_id")
	chatID, errID := strconv.ParseUint(chatIDStr, 10, 32)
	if errID != nil || chatID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "ID chat tidak valid",
		})
		return
	}

	roleStr, adminID, ok := getAuthContext(c)
	if !ok {
		return
	}

	if roleStr != string(models.RoleAdminPemdes) && roleStr != string(models.RoleAdminPu) && roleStr != string(models.RoleSuperAdmin) {
		c.JSON(http.StatusForbidden, gin.H{
			"status":  "error",
			"message": "Anda tidak memiliki wewenang membalas chat",
		})
		return
	}

	if config.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Koneksi database tidak tersedia",
		})
		return
	}

	var chat models.RiwayatChat
	if err := config.DB.Where("id = ? AND deleted_at IS NULL", chatID).First(&chat).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data chat tidak ditemukan"})
		return
	}

	var laporan models.LaporanKerusakan
	if err := config.DB.Where("id = ? AND deleted_at IS NULL", chat.LaporanKerusakanID).First(&laporan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Laporan terkait chat ini tidak ditemukan"})
		return
	}

	if !utils.CekAksesLaporan(roleStr, adminID, laporan) {
		c.JSON(http.StatusForbidden, gin.H{"status": "error", "message": "Anda tidak memiliki wewenang membalas chat di wilayah/jenis jalan ini"})
		return
	}

	contentType := c.ContentType()
	var balasanText string
	var fileHeader *multipart.FileHeader
	var hasFile bool

	if strings.Contains(contentType, "application/json") {
		var input struct {
			Balasan string `json:"balasan"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format JSON tidak valid"})
			return
		}
		balasanText = input.Balasan
	} else {
		balasanText = c.PostForm("balasan")
		var errFile error
		fileHeader, errFile = c.FormFile("lampiran")
		if errFile == nil && fileHeader != nil {
			hasFile = true
		}
	}

	trimmedBalasan := strings.TrimSpace(balasanText)
	if !hasFile && trimmedBalasan == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Balasan atau lampiran gambar tidak boleh kosong"})
		return
	}

	if len(trimmedBalasan) > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Balasan terlalu panjang (maksimal 1000 karakter)"})
		return
	}

	var uploadedURL string
	var mimeType string
	var fileName string

	if hasFile {
		if err := utils.ValidateImageFile(fileHeader); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Gagal membaca file lampiran"})
			return
		}
		buffer := make([]byte, 512)
		n, _ := file.Read(buffer)
		file.Close()

		detectedMime := http.DetectContentType(buffer[:n])
		detectedMime = strings.Split(detectedMime, ";")[0]
		mimeType = strings.TrimSpace(detectedMime)
		fileName = filepath.Base(fileHeader.Filename)

		var errUpload error
		uploadedURL, errUpload = utils.UploadCloudinary(fileHeader)
		if errUpload != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengunggah lampiran gambar ke server"})
			return
		}
	}

	oldAttachmentURL := ""
	if chat.LampiranBalasanURL != nil {
		oldAttachmentURL = *chat.LampiranBalasanURL
	}

	now := time.Now()
	chat.AdminID = &adminID
	if trimmedBalasan != "" {
		chat.Balasan = &trimmedBalasan
	}
	chat.DibalasAt = &now

	if hasFile && uploadedURL != "" {
		chat.LampiranBalasanURL = &uploadedURL
		chat.LampiranBalasanNama = &fileName
		chat.LampiranBalasanMimeType = &mimeType
	}

	if err := config.DB.Save(&chat).Error; err != nil {
		if hasFile && uploadedURL != "" {
			_ = utils.DeleteCloudinary(uploadedURL)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan balasan chat"})
		return
	}

	// Clean up overwritten old attachment from Cloudinary if successfully replaced
	if hasFile && oldAttachmentURL != "" && oldAttachmentURL != uploadedURL {
		_ = utils.DeleteCloudinary(oldAttachmentURL)
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
	if config.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Koneksi database tidak tersedia",
		})
		return
	}

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
		if adminUser.WilayahID == nil || *adminUser.WilayahID == 0 {
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
		hasBalasanText := lastChat.Balasan != nil && *lastChat.Balasan != ""
		hasBalasanAttachment := lastChat.LampiranBalasanURL != nil && *lastChat.LampiranBalasanURL != ""

		if hasBalasanText || hasBalasanAttachment {
			if lastChat.DibalasAt != nil {
				waktuPesanTerakhir = utils.FormatTanggalIndo(lastChat.DibalasAt)
			}
			if hasBalasanText {
				isiPesanTerakhir = *lastChat.Balasan
			} else {
				isiPesanTerakhir = "📎 Lampiran gambar"
			}
		}

		menungguBalasan := false
		for _, ch := range repChats {
			chatAnswered := (ch.Balasan != nil && *ch.Balasan != "") || (ch.LampiranBalasanURL != nil && *ch.LampiranBalasanURL != "")
			if !chatAnswered {
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
