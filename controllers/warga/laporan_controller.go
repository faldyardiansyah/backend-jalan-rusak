package warga

import (
	"net/http"
	"strconv"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
)

// 1. Struct DTO untuk mengatur output JSON ke Frontend
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
	WaktuLaporan  string      `json:"waktu_laporan"` // <--- Tanggal Indonesia
	User          models.User `json:"user,omitempty"`
}

// 2. Helper untuk mengubah Model Database -> Response DTO Berformat Tanggal
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
		WaktuLaporan:  utils.FormatTanggalIndo(&lap.CreatedAt, "Asia/Jakarta"), // Menggunakan helper tanggal
		User:          lap.User,
	}
}

// ini buat warga untuk ngelaporin jalannya
func CreateLaporan(c *gin.Context) {
	// ini buat ngambil data dari authmiddleware
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	userID := userIDVal.(uint)

	// ngambil datanya
	judul := c.PostFrom("judul")
	deskripsi := c.PostFrom("deskripsi")
	tipeKerusakan := c.PostFrom("tipe_kerusakan")
	jenisJalan := c.PostFrom("jenis_jalan")
	latStr := c.PostFrom("latitude")
	lngStr := c.PostFrom("longitude")

	if judul == "" || deskripsi == "" || tipeKerusakan == "" || jenisJalan == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Judul, deskripsi, tipe kerusakan, dan jenis jalan harus diisi",
		})
		return
	}

	// convert string latitude & longitude ke float64
	lat, _ := strconv.ParseFloat(latStr, 64)
	lng, _ := strconv.ParseFloat(lngStr, 64)

	// buat ambil foto dari form nya
	fileHeader, err := c.FormFile("foto")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "foto laporan wajib di unggah",
		})
		return
	}

	// upload foto ke cloudinary
	imageURL, err := utils.UploadCloudinary(fileHeader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal mengunggah foto ke cloudinary: " + err.Error(),
		})
		return
	}

	var WilayahID uint 
	namaWilayah, errOsm := utils.ReverseGeocodeOSM(lat, lng)
	if errOsm != nil {
		wilayah, errFind := utils.FindWilayahByNama(config.DB, namaWilayah)
		if errFind != nil {
			wilayahID = wilayah.ID
		}
	}

	if wilayahID == 0 {
		wilayahIDStr := c.PostFrom("wilayah_id")
		id, _ := strconv.Atoi(wilayahIDStr)
		wilayahID = uint(id)
	}

	if wilayahID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Wilayah tidak ditemukan, mohon pilih wilayah secara manual di map",
		})
		return
	}

	// buat menyimpan laporan ke database mysql 
	laporan := models.LaporanKerusakan{
		UserID:        userID, // <-- Diperbaiki: UserID (Kapital)
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

	if err := config.DB.Create(&laporan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal menyimpan laporan ke database",
		})
		return
	}

	// Preload relasi User untuk kelengkapan response
	config.DB.Preload("User").First(&laporan, laporan.ID)

	KirimNotifikasiLaporanBaru(laporan)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Laporan berhasil dikirim",
		"data":    FormatLaporanToResponse(laporan), // <-- Menggunakan DTO Berformat Tanggal
	})
}

// ini buat notifikasi
func KirimNotifikasiLaporanBaru(laporan models.LaporanKerusakan) {
	var adminTujuan []models.User

	switch laporan.JenisJalan {
	case "desa":
		config.DB.Where("role = ? AND wilayah_id = ?", models.RoleAdminPemdes, laporan.WilayahID).Find(&adminTujuan)
	case "kabupaten":
		config.DB.Where("role = ?", models.RoleAdminPU).Find(&adminTujuan)
	case "provinsi":
		config.DB.Where("role = ?", models.RoleSuperadmin).Find(&adminTujuan)
	}

	for _, admin := range adminTujuan {
		config.DB.Create(&models.Notifikasi{
			UserID:    admin.ID,
			LaporanID: laporan.ID,
			Judul : "Laporan Baru Masuk",
			Pesan : "Laporan baru telah dikirimkan oleh " + laporan.User.Nama + "untuk wilayah " + laporan.Wilayah.NamaWilayah,
		})
	}
}

// buat ini itu buat mengambil riwayat
func GetRiwayatLaporan(c *gin.Context) {
	userIDVal, _ := c.Get("User_id")
	userID := userIDVal.(uint)

	var listLaporan []models.LaporanKerusakan
	
	// Ditambahkan Preload("User") dan Order("created_at desc") agar laporan terbaru muncul di paling atas
	config.DB.Preload("User").
		Where("user_id = ?", userID).
		Order("created_at desc").
		Find(&listLaporan)

	// Loop dan konversi tiap data laporan ke format Response DTO
	var responseData []LaporanResponse
	for _, lap := range listLaporan {
		responseData = append(responseData, FormatLaporanToResponse(lap))
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Riwayat laporan berhasil diambil",
		"data":    responseData, // <-- Mengirim daftar laporan dengan tanggal berformat
	})
}

// mapnya si warga
func GetAllLaporanPeta(c *gin.Context) {
	var listLaporan []models.LaporanKerusakan
	config.DB.Where("delete_at IS NULL").Find(&listLaporan)

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