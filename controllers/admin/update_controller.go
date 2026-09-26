package admin

import (
	"net/http"
	"strings"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
)

func UpdateStatusLaporan(c *gin.Context) {
	id := c.Param("id")

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
	if err := config.DB.First(&laporan, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Laporan tidak ditemukan",
		})
		return
	}

	// ini buat validasi hak aksesnya
	jenisJalanLower := strings.ToLower(laporan.JenisJalan)

	if role == models.RoleAdminPemdes {
		var adminUser models.User
		config.DB.First(&adminUser, userID)

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
		uploadedURL, errUpload := utils.UploadCloudinary(fileHeader)
		if errUpload != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Gagal mengupload foto bukti",
			})
			return
		}
		newFotoBukti = uploadedURL
	}

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
		if laporan.FotoBukti != "" {
			_ = utils.DeleteCloudinary(laporan.FotoBukti)
		}
		laporan.FotoBukti = newFotoBukti
	}

	// Simpan perubahan ke database
	if err := config.DB.Save(&laporan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal menyimpan laporan",
		})
		return
	}

	// Notifikasi otomatis ke warga jika status berubah
	if status != "" {
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
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Laporan berhasil diperbarui",
		"data":    laporan,
	})
}
