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

	// ini itu biar konversinya amn 
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
	_ = c.ShouldBind(&req)

	status := req.Status
	if status == "" {
		status = c.PostForm("status")
	}
	ditugaskanKe := req.DitugaskanKe
	if ditugaskanKe == "" {
		ditugaskanKe = c.PostForm("ditugaskan_ke")
	}
	catatanAdmin := req.CatatanAdmin
	if catatanAdmin == "" {
		catatanAdmin = c.PostForm("catatan_admin")
	}


	// ini buat validasi enum statusnya
	if status != "" {
		statusLower := strings.ToLower(status)
		if statusLower != "menunggu" && statusLower != "proses" && statusLower != "selesai" && statusLower != "ditolak" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Status tidak valid",
			})
			return
		}
		laporan.Status = statusLower
	}

	if ditugaskanKe != "" {
		laporan.DitugaskanKe = ditugaskanKe
	}

	if catatanAdmin != "" {
		laporan.CatatanAdmin = catatanAdmin
	}

	// ini buat upload bukti foto 
	fileHeader, err := c.FormFile("foto_bukti")
	if err == nil {
		fotoBukti, err := utils.UploadCloudinary(fileHeader)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Gagal mengupload foto bukti",
			})
			return
		}
		laporan.FotoBukti = fotoBukti
	}

	// ini buat simpan perubahan ke db
	if err := config.DB.Save(&laporan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal menyimpan laporan",
		})
		return
	}

	//  ini itu buat notifikasi otomatis ke warga nya
	if status != "" {
		config.DB.Create(&models.Notifikasi{
			UserID: laporan.UserID,
			LaporanID: laporan.ID,
			Judul: "Status Laporan Berubah",
			Pesan: "Status laporan Anda telah diubah menjadi " + laporan.Status,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Laporan berhasil diperbarui",
		"data":    laporan,
	})
}