package warga

import (
	"fmt"
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
	if judul == "" || len(judul) > 150 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Judul wajib diisi dan maksimal 150 karakter",
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
	if tipeKerusakan == "" || len(tipeKerusakan) > 150 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Tipe kerusakan wajib diisi dan maksimal 150 karakter",
		})
		return
	}

	lat, err := strconv.ParseFloat(c.PostForm("latitude"), 64)
	if err != nil || math.IsNaN(lat) || math.IsInf(lat, 0) || lat < -90 || lat > 90 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Latitude tidak valid",
		})
		return
	}

	lng, err := strconv.ParseFloat(c.PostForm("longitude"), 64)
	if err != nil || math.IsNaN(lng) || math.IsInf(lng, 0) || lng < -180 || lng > 180 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Longitude tidak valid",
		})
		return
	}

	wilayahIDStr := c.PostForm("wilayah_id")
	var wilayahID uint

	if wilayahIDStr != "" {
		id, err := strconv.ParseUint(wilayahIDStr, 10, 32)
		if err != nil || id == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Wilayah ID tidak valid",
			})
			return
		}
		wilayahID = uint(id)
	}

	fileHeader, err := c.FormFile("foto")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Foto laporan wajib diunggah",
		})
		return
	}

	if err := utils.ValidateImageFile(fileHeader); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": err.Error(),
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

	namaWilayahOSM, jenisJalanOSM, errOSM := utils.ReverseGeocodeOSM(lat, lng)

	fmt.Printf(
		"[ROADIS OSM] lat=%f lng=%f wilayah=%q jenis_jalan=%q error=%v\n",
		lat,
		lng,
		namaWilayahOSM,
		jenisJalanOSM,
		errOSM,
	)

	if wilayahID == 0 && errOSM == nil {
		wilayah, err := utils.FindWilayahByNama(config.DB, namaWilayahOSM)
		if err == nil {
			wilayahID = wilayah.ID
		}
	}

	if wilayahID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Wilayah tidak ditemukan, silakan pilih manual",
		})
		return
	}

	var targetWilayah models.Wilayah
	if err := config.DB.Where(
		"id = ? AND deleted_at IS NULL",
		wilayahID,
	).First(&targetWilayah).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Wilayah tidak valid atau sudah tidak aktif",
		})
		return
	}

	jenisJalan := strings.ToLower(strings.TrimSpace(jenisJalanOSM))
	switch jenisJalan {
	case "desa", "kabupaten", "provinsi", "nasional":
	case "belum_pasti", "":
		jenisJalan = "belum_pasti"
	default:
		jenisJalan = "belum_pasti"
	}

	imageURL, err := LaporanUploader(fileHeader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal upload foto",
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

	if err := config.DB.Create(&laporan).Error; err != nil {
		_ = utils.DeleteCloudinary(imageURL)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menyimpan laporan",
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

	jenisJalan := strings.ToLower(strings.TrimSpace(laporan.JenisJalan))

	var adminTujuan []models.User

	if err := config.DB.
		Where("role = ?", models.RoleSuperAdmin).
		Find(&adminTujuan).Error; err != nil {
		config.DB.Logger.Error(
			config.DB.Statement.Context,
			"Gagal mengambil Superadmin: %v",
			err,
		)
		return
	}

	var adminTeknis []models.User

	switch jenisJalan {
	case "nasional", "provinsi", "kabupaten":
		if err := config.DB.
			Where("role = ?", models.RoleAdminPu).
			Find(&adminTeknis).Error; err != nil {
			config.DB.Logger.Error(
				config.DB.Statement.Context,
				"Gagal mengambil admin PU: %v",
				err,
			)
		}

	case "desa":
		if laporan.WilayahID > 0 {
			if err := config.DB.
				Where(
					"role = ? AND wilayah_id = ?",
					models.RoleAdminPemdes,
					laporan.WilayahID,
				).
				Find(&adminTeknis).Error; err != nil {
				config.DB.Logger.Error(
					config.DB.Statement.Context,
					"Gagal mengambil admin Pemdes: %v",
					err,
				)
			}
		}
	}

	semuaAdmin := append(adminTujuan, adminTeknis...)
	seen := make(map[uint]bool)

	pengirim := "Warga"
	if nama := strings.TrimSpace(laporan.User.Name); nama != "" {
		pengirim = nama
	}

	for _, admin := range semuaAdmin {
		if admin.ID == 0 || seen[admin.ID] {
			continue
		}
		seen[admin.ID] = true

		notifikasi := models.Notifikasi{
			UserID:    admin.ID,
			LaporanID: laporan.ID,
			Judul:     "Laporan Baru Masuk",
			Pesan:     "Laporan baru telah dikirimkan oleh " + pengirim,
		}

		if err := config.DB.Create(&notifikasi).Error; err != nil {
			config.DB.Logger.Error(
				config.DB.Statement.Context,
				"Gagal membuat notifikasi laporan: %v",
				err,
			)
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

func CekJalan(c *gin.Context) {
	latStr := strings.TrimSpace(c.Query("latitude"))
	lngStr := strings.TrimSpace(c.Query("longitude"))

	lat, errLat := strconv.ParseFloat(latStr, 64)
	lng, errLng := strconv.ParseFloat(lngStr, 64)

	if errLat != nil || errLng != nil ||
		math.IsNaN(lat) || math.IsInf(lat, 0) ||
		math.IsNaN(lng) || math.IsInf(lng, 0) ||
		lat < -90 || lat > 90 ||
		lng < -180 || lng > 180 {

		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Latitude atau longitude tidak valid",
			"data":    nil,
		})
		return
	}

	// Memeriksa apakah terdapat jalan dalam radius 20 meter.
	hasil, err := utils.ValidasiJarakJalanOSM(lat, lng)

	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"status":  "error",
			"message": "Gagal memeriksa jalan dari OpenStreetMap",
			"data":    nil,
		})
		return
	}

	if !hasil.Terdeteksi {
		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "Jalan tidak ditemukan dalam radius 20 meter",
			"data":    hasil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": hasil.Keterangan,
		"data":    hasil,
	})
}