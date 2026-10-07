package admin

import (
	"net/http"
	"strconv"
	"strings"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
)

var (
	// BuktiUploader dan BuktiDeleter menggunakan fungsi Cloudinary existing, dapat di-override pada unit test
	BuktiUploader = utils.UploadCloudinary
	BuktiDeleter  = utils.DeleteCloudinary
)

func UpdateStatusLaporan(c *gin.Context) {
	idStr := c.Param("id")
	id, errID := strconv.ParseUint(idStr, 10, 32)
	if errID != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "ID laporan tidak valid",
			"error":   "ID laporan tidak valid",
		})
		return
	}

	roleVal, _ := c.Get("role")
	userIDVal, _ := c.Get("user_id")

	var role models.UserRole
	if rStrl, ok := roleVal.(string); ok {
		role = models.UserRole(rStrl)
	} else if rEnum, ok := roleVal.(models.UserRole); ok {
		role = rEnum
	}

	// ini itu biar konversinya aman
	var userID uint
	switch v := userIDVal.(type) {
	case uint:
		userID = v
	case float64:
		userID = uint(v)
	case int:
		userID = uint(v)
	}

	// ini buat cari data laporan di databasenya
	var laporan models.LaporanKerusakan
	if err := config.DB.Where("id = ? AND deleted_at IS NULL", id).First(&laporan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Laporan tidak ditemukan",
		})
		return
	}

	// ini buat validasi hak aksesnya
	jenisJalanLower := strings.ToLower(laporan.JenisJalan)

	if role != models.RoleAdminPemdes && role != models.RoleAdminPu && role != models.RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"status":  "error",
			"message": "Akses tidak diizinkan",
			"error":   "Akses tidak diizinkan",
		})
		return
	}

	if role == models.RoleAdminPemdes {
		var adminUser models.User
		if err := config.DB.First(&adminUser, userID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"status":  "error",
				"message": "Data admin tidak ditemukan",
				"error":   "Data admin tidak ditemukan",
			})
			return
		}

		if adminUser.WilayahID == nil || laporan.WilayahID != *adminUser.WilayahID || jenisJalanLower != "desa" {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Anda tidak memiliki akses untuk mengubah laporan ini",
			})
			return
		}
	}

	if role == models.RoleAdminPu && jenisJalanLower != "kabupaten" {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Anda tidak memiliki akses untuk mengubah laporan ini",
		})
		return
	}

	type UpdateReq struct {
		Status       string `json:"status" form:"status"`
		DitugaskanKe string `json:"ditugaskan_ke" form:"ditugaskan_ke"`
		CatatanAdmin string `json:"catatan_admin" form:"catatan_admin"`
	}

	var req UpdateReq
	if strings.Contains(c.ContentType(), "application/json") {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Format JSON tidak valid: " + err.Error(),
			})
			return
		}
	} else {
		_ = c.ShouldBind(&req)
	}

	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = strings.TrimSpace(c.PostForm("status"))
	}
	ditugaskanKe := strings.TrimSpace(req.DitugaskanKe)
	if ditugaskanKe == "" {
		ditugaskanKe = strings.TrimSpace(c.PostForm("ditugaskan_ke"))
	}
	catatanAdmin := strings.TrimSpace(req.CatatanAdmin)
	if catatanAdmin == "" {
		catatanAdmin = strings.TrimSpace(c.PostForm("catatan_admin"))
	}

	if len(ditugaskanKe) > 150 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Petugas yang ditugaskan maksimal 150 karakter",
		})
		return
	}

	// Validasi enum status jika dikirim
	var statusLower string
	if status != "" {
		statusLower = strings.ToLower(status)
		if statusLower != "menunggu" && statusLower != "proses" && statusLower != "selesai" && statusLower != "ditolak" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Status tidak valid",
			})
			return
		}
	}

	oldStatus := strings.ToLower(laporan.Status)
	statusChanged := statusLower != "" && statusLower != oldStatus

	// Validasi transisi status (State Machine Guard)
	if statusChanged {
		if oldStatus == "selesai" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Laporan yang sudah selesai tidak dapat diubah statusnya",
				"error":   "Laporan yang sudah selesai tidak dapat diubah statusnya",
			})
			return
		}
		if oldStatus == "ditolak" && statusLower == "selesai" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Laporan yang ditolak tidak dapat langsung diselesaikan",
				"error":   "Laporan yang ditolak tidak dapat langsung diselesaikan",
			})
			return
		}
	}

	// Validasi catatan admin jika status baru adalah "ditolak"
	if statusLower == "ditolak" && catatanAdmin == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Catatan admin / alasan penolakan wajib diisi saat menolak laporan",
		})
		return
	}

	// Validasi foto bukti jika status baru adalah "selesai"
	fileHeader, errFile := c.FormFile("foto_bukti")
	if statusLower == "selesai" {
		if errFile != nil && laporan.FotoBukti == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Foto bukti perbaikan wajib diunggah untuk menyelesaikan laporan",
			})
			return
		}
	}

	// Upload foto bukti baru jika ada file yang diunggah
	var newFotoBukti string
	if errFile == nil {
		if errVal := utils.ValidateImageFile(fileHeader); errVal != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": errVal.Error(),
				"error":   errVal.Error(),
			})
			return
		}

		uploadedURL, errUpload := BuktiUploader(fileHeader)
		if errUpload != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Gagal mengupload foto bukti",
			})
			return
		}
		newFotoBukti = uploadedURL
	}

	oldFotoBukti := laporan.FotoBukti

	// Terapkan perubahan ke entitas laporan
	if statusLower != "" {
		laporan.Status = statusLower
	}

	if ditugaskanKe != "" {
		laporan.DitugaskanKe = ditugaskanKe
	}

	if catatanAdmin != "" {
		laporan.CatatanAdmin = catatanAdmin
	}

	if newFotoBukti != "" {
		laporan.FotoBukti = newFotoBukti
	}

	// Simpan perubahan ke database
	if err := config.DB.Save(&laporan).Error; err != nil {
		if newFotoBukti != "" {
			_ = BuktiDeleter(newFotoBukti)
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal menyimpan laporan",
		})
		return
	}

	// Hapus foto bukti lama hanya jika database save berhasil
	if newFotoBukti != "" && oldFotoBukti != "" {
		_ = BuktiDeleter(oldFotoBukti)
	}

	// Notifikasi otomatis ke warga jika status berubah
	if statusChanged {
		pesanNotif := "Laporan \"" + laporan.Judul + "\" statusnya diperbarui menjadi: " + strings.ToUpper(laporan.Status)

		if catatanAdmin != "" {
			pesanNotif += ". Catatan admin: " + catatanAdmin
		}

		config.DB.Create(&models.Notifikasi{
			UserID:    laporan.UserID,
			LaporanID: laporan.ID,
			Judul:     "Status Laporan Berubah",
			Pesan:     pesanNotif,
		})

		// Notifikasi otomatis ke admin berwenang jika diupdate oleh pihak lain
		var adminTujuan []models.User
		switch strings.ToLower(laporan.JenisJalan) {
		case "desa":
			config.DB.
				Where("role = ? AND wilayah_id = ? AND id != ?", models.RoleAdminPemdes, laporan.WilayahID, userID).
				Find(&adminTujuan)
		case "kabupaten":
			config.DB.
				Where("role = ? AND id != ?", models.RoleAdminPu, userID).
				Find(&adminTujuan)
		}

		for _, admin := range adminTujuan {
			config.DB.Create(&models.Notifikasi{
				UserID:    admin.ID,
				LaporanID: laporan.ID,
				Judul:     "Status Laporan Berubah",
				Pesan:     pesanNotif,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Laporan berhasil diperbarui",
		"data":    laporan,
	})
}
