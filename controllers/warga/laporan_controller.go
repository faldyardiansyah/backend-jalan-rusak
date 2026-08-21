package warga

import (
	"net/http"
	"strconv"
	"strings"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
)

type LaporanResponse struct {
	ID            uint        `json:"id"`
	UserID        uint        `json:"user_id"`
	Judul         string      `json:"judul"`
	Deskripsi     string      `json:"deskripsi"`
	Latitude      float64     `json:"latitude"`
	Longitude     float64     `json:"longitude"`
	ImageURL      string      `json:"image_url"`
	TipeKerusakan string      `json:"tipe_kerusakan"`
	Status        string      `json:"status"`
	WaktuLaporan  string      `json:"waktu_laporan"`
	User          models.User `json:"user,omitempty"`
}

func FormatLaporanToResponse(lap models.LaporanKerusakan) LaporanResponse {
	return LaporanResponse{
		ID:            lap.ID,
		UserID:        lap.UserID,
		Judul:         lap.Judul,
		Deskripsi:     lap.Deskripsi,
		Latitude:      lap.Latitude,
		Longitude:     lap.Longitude,
		ImageURL:      lap.ImageURL,
		TipeKerusakan: lap.TipeKerusakan,
		Status:        lap.Status,
		WaktuLaporan:  utils.FormatTanggalIndo(&lap.CreatedAt),
		User:          lap.User,
	}
}

func CreateLaporan(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "User ID tidak ditemukan",
		})
		return
	}

	userID := userIDVal.(uint)
	judul := c.PostForm("judul")
	deskripsi := c.PostForm("deskripsi")
	tipeKerusakan := c.PostForm("tipe_kerusakan")

	latStr := c.PostForm("latitude")
	lngStr := c.PostForm("longitude")
	wilayahIDStr := c.PostForm("wilayah_id")

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Latitude tidak valid",
		})
		return
	}

	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Longitude tidak valid",
		})
		return
	}

	fileHeader, err := c.FormFile("foto")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "foto laporan wajib di unggah"})
		return
	}

	imageURL, err := utils.UploadCloudinary(fileHeader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal upload foto"})
		return
	}

	// Ambil data cadangan dari OSM (Nama Wilayah & Deteksi Jenis Jalan)
	namaWilayahOSM, jenisJalanOSM, errOSM := utils.ReverseGeocodeOSM(lat, lng)

	// memnetukan jenis jalan
	jenisJalan := c.PostForm("jenis_jalan")
	if jenisJalan == "" {
		jenisJalan = jenisJalanOSM
	}
	if jenisJalan == "" {
		jenisJalan = "desa"
	}

	// memenentukan wilayah id berdasarkan inputan wilayah
	var wilayahID uint
	if wilayahIDStr != "" {
		id, _ := strconv.Atoi(wilayahIDStr)
		wilayahID = uint(id)
	}

	if wilayahID == 0 && errOSM == nil {
		wilayah, errFind := utils.FindWilayahByNama(config.DB, namaWilayahOSM)
		if errFind == nil {
			wilayahID = wilayah.ID
		}
	}

	if wilayahID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Wilayah tidak ditemukan, mohon pilih manual nyak",
		})
		return
	}

	laporan := models.LaporanKerusakan{
		UserID:        userID,
		Judul:         judul,
		JenisJalan:    jenisJalan,
		WilayahID:     wilayahID,
		Deskripsi:     deskripsi,
		Latitude:      lat,
		Longitude:     lng,
		ImageURL:      imageURL,
		TipeKerusakan: tipeKerusakan,
		Status:        "menunggu",
	}

	// Simpan laporan ke database
	result := config.DB.Create(&laporan)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menyimpan laporan",
			"error":   result.Error.Error(),
		})
		return
	}

	config.DB.
		Preload("User").
		Preload("Wilayah").
		First(&laporan, laporan.ID)

	KirimNotifikasiLaporanBaru(laporan)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Laporan berhasil dikirim",
		"data":    FormatLaporanToResponse(laporan),
	})
}

func KirimNotifikasiLaporanBaru(laporan models.LaporanKerusakan) {
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

	for _, admin := range adminTujuan {
		config.DB.Create(&models.Notifikasi{
			UserID:    admin.ID,
			LaporanID: laporan.ID,
			Judul:     "Laporan Baru Masuk",
			Pesan:     "Laporan baru telah dikirimkan oleh " + laporan.User.Name,
		})
	}
}

func GetRiwayatLaporan(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")

	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User tidak terautentikasi",
		})
		return
	}

	userID := userIDVal.(uint)

	var listLaporan []models.LaporanKerusakan

	config.DB.
		Preload("User").
		Preload("Wilayah").
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&listLaporan)

	var responseData []LaporanResponse

	for _, lap := range listLaporan {
		responseData = append(responseData, FormatLaporanToResponse(lap))
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Riwayat laporan berhasil diambil",
		"data":    responseData,
	})
}

func GetAllLaporanPeta(c *gin.Context) {
	var listLaporan []models.LaporanKerusakan

	config.DB.
		Preload("User").
		Preload("Wilayah").
		Find(&listLaporan)

	var responseData []LaporanResponse

	for _, lap := range listLaporan {
		responseData = append(responseData, FormatLaporanToResponse(lap))
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Semua laporan berhasil diambil",
		"data":    responseData,
	})
}