package warga

import (
	"math"
	"net/http"
	"strconv"
	"strings"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
)

var (
	// LaporanUploader menggunakan utils.UploadCloudinary existing, dapat di-override pada unit test
	LaporanUploader = utils.UploadCloudinary
)

type LaporanResponse struct {
	ID            uint    `json:"id"`
	UserID        uint    `json:"user_id"`
	Judul         string  `json:"judul"`
	Deskripsi     string  `json:"deskripsi"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	ImageURL      string  `json:"image_url"`
	TipeKerusakan string  `json:"tipe_kerusakan"`
	Status        string  `json:"status"`
	WaktuLaporan  string  `json:"waktu_laporan"`
	FotoBukti     string  `json:"foto_bukti,omitempty"`
	CatatanAdmin  string  `json:"catatan_admin,omitempty"`
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
		FotoBukti:     lap.FotoBukti,
		CatatanAdmin:  lap.CatatanAdmin,
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

	var userID uint
	switch v := userIDVal.(type) {
	case uint:
		userID = v
	case float64:
		userID = uint(v)
	case int:
		userID = uint(v)
	}

	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "User ID tidak valid",
		})
		return
	}
	judul := strings.TrimSpace(c.PostForm("judul"))
	if judul == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Judul laporan tidak boleh kosong",
		})
		return
	}
	if len(judul) > 150 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Judul laporan maksimal 150 karakter",
		})
		return
	}

	deskripsi := strings.TrimSpace(c.PostForm("deskripsi"))
	if deskripsi == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Deskripsi laporan tidak boleh kosong",
		})
		return
	}

	tipeKerusakan := strings.TrimSpace(c.PostForm("tipe_kerusakan"))
	if tipeKerusakan == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Tipe kerusakan tidak boleh kosong",
		})
		return
	}
	if len(tipeKerusakan) > 150 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Tipe kerusakan maksimal 150 karakter",
		})
		return
	}

	latStr := c.PostForm("latitude")
	lngStr := c.PostForm("longitude")
	wilayahIDStr := c.PostForm("wilayah_id")

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil || math.IsNaN(lat) || math.IsInf(lat, 0) || lat < -90 || lat > 90 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Latitude tidak valid (harus berada di antara -90 dan 90)",
		})
		return
	}

	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil || math.IsNaN(lng) || math.IsInf(lng, 0) || lng < -180 || lng > 180 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Longitude tidak valid (harus berada di antara -180 dan 180)",
		})
		return
	}

	fileHeader, err := c.FormFile("foto")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "foto laporan wajib di unggah",
			"error":   "foto laporan wajib di unggah",
		})
		return
	}

	if errVal := utils.ValidateImageFile(fileHeader); errVal != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": errVal.Error(),
			"error":   errVal.Error(),
		})
		return
	}

	// Menentukan wilayah id berdasarkan inputan wilayah atau OSM
	var wilayahID uint
	if wilayahIDStr != "" {
		id, err := strconv.ParseUint(wilayahIDStr, 10, 32)
		if err != nil || id == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Wilayah ID tidak valid",
				"error":   "Wilayah ID tidak valid",
			})
			return
		}
		wilayahID = uint(id)
	}

	// Ambil data cadangan dari OSM (Nama Wilayah & Deteksi Jenis Jalan)
	namaWilayahOSM, jenisJalanOSM, errOSM := utils.ReverseGeocodeOSM(lat, lng)

	if wilayahID == 0 && errOSM == nil {
		wilayah, errFind := utils.FindWilayahByNama(config.DB, namaWilayahOSM)
		if errFind == nil {
			wilayahID = wilayah.ID
		}
	}

	if wilayahID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Wilayah tidak ditemukan, mohon pilih manual",
			"error":   "Wilayah tidak ditemukan, mohon pilih manual",
		})
		return
	}

	if config.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Koneksi database tidak tersedia",
			"error":   "Koneksi database tidak tersedia",
		})
		return
	}

	// Validasi bahwa WilayahID merujuk ke wilayah aktif (tidak soft-deleted)
	var targetWilayah models.Wilayah
	if err := config.DB.Where("id = ? AND deleted_at IS NULL", wilayahID).First(&targetWilayah).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Wilayah tidak valid atau sudah tidak aktif",
			"error":   "Wilayah tidak valid atau sudah tidak aktif",
		})
		return
	}

	// Menentukan jenis jalan
	jenisJalan := strings.ToLower(strings.TrimSpace(c.PostForm("jenis_jalan")))
	if jenisJalan == "" {
		jenisJalan = strings.ToLower(strings.TrimSpace(jenisJalanOSM))
	}
	if jenisJalan == "" {
		jenisJalan = "desa"
	}
	if jenisJalan != "desa" && jenisJalan != "kabupaten" && jenisJalan != "provinsi" && jenisJalan != "nasional" {
		jenisJalan = "desa"
	}

	// Upload foto setelah semua validasi input dan wilayah berhasil
	imageURL, err := LaporanUploader(fileHeader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal upload foto",
			"error":   "Gagal upload foto",
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
		_ = utils.DeleteCloudinary(imageURL)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menyimpan laporan",
			"error":   "Terjadi kesalahan pada sistem saat menyimpan laporan",
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

func GetLaporanByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "ID laporan tidak valid",
			"error":   "ID laporan tidak valid",
		})
		return
	}

	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "User ID tidak ditemukan",
			"error":   "User ID tidak ditemukan",
		})
		return
	}

	var userID uint
	switch v := userIDVal.(type) {
	case uint:
		userID = v
	case float64:
		userID = uint(v)
	case int:
		userID = uint(v)
	}

	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "User ID tidak valid",
			"error":   "User ID tidak valid",
		})
		return
	}

	if config.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Koneksi database tidak tersedia",
			"error":   "Koneksi database tidak tersedia",
		})
		return
	}

	var laporan models.LaporanKerusakan
	if err := config.DB.Where("id = ? AND deleted_at IS NULL", id).First(&laporan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Laporan tidak ditemukan",
			"error":   "Laporan tidak ditemukan",
		})
		return
	}

	if laporan.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"status":  "error",
			"message": "Anda tidak memiliki akses ke laporan ini",
			"error":   "Anda tidak memiliki akses ke laporan ini",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Detail laporan berhasil diambil",
		"data":    FormatLaporanToResponse(laporan),
	})
}

func KirimNotifikasiLaporanBaru(laporan models.LaporanKerusakan) {
	if config.DB == nil || laporan.ID == 0 {
		return
	}

	var adminTujuan []models.User

	switch strings.ToLower(laporan.JenisJalan) {
	case "desa":
		if laporan.WilayahID == 0 {
			return
		}
		config.DB.
			Where("role = ? AND wilayah_id = ?", models.RoleAdminPemdes, laporan.WilayahID).
			Find(&adminTujuan)
	case "kabupaten":
		config.DB.
			Where("role = ?", models.RoleAdminPu).
			Find(&adminTujuan)
	}

	pengirim := "Warga"
	if strings.TrimSpace(laporan.User.Name) != "" {
		pengirim = strings.TrimSpace(laporan.User.Name)
	}

	for _, admin := range adminTujuan {
		if admin.ID > 0 {
			config.DB.Create(&models.Notifikasi{
				UserID:    admin.ID,
				LaporanID: laporan.ID,
				Judul:     "Laporan Baru Masuk",
				Pesan:     "Laporan baru telah dikirimkan oleh " + pengirim,
			})
		}
	}
}

func GetRiwayatLaporan(c *gin.Context) {
	if config.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Koneksi database tidak tersedia",
			"error":   "Koneksi database tidak tersedia",
		})
		return
	}

	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"error":   "User tidak terautentikasi",
			"message": "User tidak terautentikasi",
		})
		return
	}

	var userID uint
	switch v := userIDVal.(type) {
	case uint:
		userID = v
	case float64:
		userID = uint(v)
	case int:
		userID = uint(v)
	}

	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"error":   "User ID tidak valid",
			"message": "User ID tidak valid",
		})
		return
	}

	var listLaporan []models.LaporanKerusakan

	if err := config.DB.
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Order("created_at DESC").
		Find(&listLaporan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"error":   "Gagal mengambil riwayat laporan",
			"message": "Gagal mengambil riwayat laporan",
		})
		return
	}

	responseData := make([]LaporanResponse, 0)

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
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "User tidak terautentikasi",
		})
		return
	}

	var userID uint
	switch v := userIDVal.(type) {
	case uint:
		userID = v
	case float64:
		userID = uint(v)
	case int:
		userID = uint(v)
	}

	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "User ID tidak valid",
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

	query := config.DB.Model(&models.LaporanKerusakan{}).
		Where("deleted_at IS NULL")

	if status := strings.ToLower(strings.TrimSpace(c.Query("status"))); status != "" {
		query = query.Where("status = ?", status)
	}

	if jenisJalan := strings.ToLower(strings.TrimSpace(c.Query("jenis_jalan"))); jenisJalan != "" && jenisJalan != "all" {
		query = query.Where("jenis_jalan = ?", jenisJalan)
	}

	if wilayahIDStr := strings.TrimSpace(c.Query("wilayah_id")); wilayahIDStr != "" {
		if wID, err := strconv.ParseUint(wilayahIDStr, 10, 32); err == nil && wID > 0 {
			query = query.Where("wilayah_id = ?", uint(wID))
		}
	}

	var listLaporan []models.LaporanKerusakan
	if err := query.Find(&listLaporan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil data laporan untuk peta",
		})
		return
	}

	responseData := make([]LaporanResponse, 0)
	for _, lap := range listLaporan {
		if !utils.IsValidCoordinate(lap.Latitude, lap.Longitude) {
			continue
		}
		responseData = append(responseData, FormatLaporanToResponse(lap))
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Semua laporan berhasil diambil",
		"data":    responseData,
	})
}
