package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"

	"github.com/gin-gonic/gin"
)

func GetAllLaporan(c *gin.Context) {
	roleVal, existsRole := c.Get("role")
	userIDVal, existsUserID := c.Get("user_id")
	if !existsRole || !existsUserID {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "User tidak terautentikasi",
		})
		return
	}

	role, okRole := roleVal.(string)
	if !okRole || role == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Role tidak valid",
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
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "User ID tidak valid",
		})
		return
	}

	statusFilter := strings.TrimSpace(c.Query("status"))
	severityFilter := strings.ToLower(strings.TrimSpace(c.Query("severity")))
	searchKeyword := strings.TrimSpace(c.Query("search"))
	jenisJalanFilter := strings.TrimSpace(c.Query("jenis_jalan"))
	startDateStr := strings.TrimSpace(c.Query("start_date"))
	endDateStr := strings.TrimSpace(c.Query("end_date"))
	sortByParam := strings.ToLower(strings.TrimSpace(c.Query("sort_by")))
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")

	page, errPage := strconv.Atoi(pageStr)
	if errPage != nil || page < 1 {
		page = 1
	}
	limit, errLimit := strconv.Atoi(limitStr)
	if errLimit != nil || limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	// Validasi filter tanggal jika diberikan
	const dateLayout = "2006-01-02"
	var startDate, endDate time.Time
	var hasStartDate, hasEndDate bool

	if startDateStr != "" {
		parsedStart, err := time.ParseInLocation(dateLayout, startDateStr, time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Format start_date tidak valid. Gunakan format YYYY-MM-DD",
			})
			return
		}
		startDate = parsedStart
		hasStartDate = true
	}

	if endDateStr != "" {
		parsedEnd, err := time.ParseInLocation(dateLayout, endDateStr, time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Format end_date tidak valid. Gunakan format YYYY-MM-DD",
			})
			return
		}
		endDate = parsedEnd
		hasEndDate = true
	}

	if hasStartDate && hasEndDate && startDate.After(endDate) {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "start_date tidak boleh melebihi end_date",
		})
		return
	}

	// Validasi dan penentuan klausul sorting deterministik
	var orderClause string
	switch sortByParam {
	case "", "newest":
		orderClause = "laporan_kerusakan.created_at DESC, laporan_kerusakan.id DESC"
	case "oldest":
		orderClause = "laporan_kerusakan.created_at ASC, laporan_kerusakan.id ASC"
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Parameter sort_by tidak valid. Gunakan 'newest' atau 'oldest'",
		})
		return
	}

	if severityFilter != "" && severityFilter != "all" {
		switch severityFilter {
		case "ringan", "sedang", "berat", "belum_dinilai":
			// valid
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Parameter severity tidak valid. Gunakan 'ringan', 'sedang', 'berat', atau 'belum_dinilai'",
			})
			return
		}
	}

	if config.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Koneksi database tidak tersedia",
		})
		return
	}

	listLaporan := make([]models.LaporanKerusakan, 0)
	var totalData int64

	query := config.DB.Model(&models.LaporanKerusakan{}).
		Preload("User").
		Preload("Wilayah").
		Joins("LEFT JOIN user ON user.id = laporan_kerusakan.user_id AND user.deleted_at IS NULL").
		Where("laporan_kerusakan.deleted_at IS NULL")

	switch role {
	case "admin_pemdes":
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
	case "admin_pu":
		// PU-5.1: Admin PU dapat melihat laporan dari SEMUA kewenangan jalan
		if jenisJalanFilter != "" && strings.ToLower(jenisJalanFilter) != "all" {
			query = query.Where("laporan_kerusakan.jenis_jalan = ?", strings.ToLower(jenisJalanFilter))
		}
	case "super_admin":
		// Superadmin melihat seluruh laporan aktif dengan filter opsional jenis_jalan
		if jenisJalanFilter != "" && strings.ToLower(jenisJalanFilter) != "all" {
			query = query.Where("laporan_kerusakan.jenis_jalan = ?", strings.ToLower(jenisJalanFilter))
		}
	default:
		c.JSON(http.StatusForbidden, gin.H{
			"status":  "error",
			"message": "Akses tidak diizinkan",
		})
		return
	}

	if statusFilter != "" && strings.ToLower(statusFilter) != "all" {
		query = query.Where("laporan_kerusakan.status = ?", statusFilter)
	}

	if severityFilter != "" && severityFilter != "all" {
		switch severityFilter {
		case "ringan", "sedang", "berat":
			query = query.Where("laporan_kerusakan.severity = ?", severityFilter)
		case "belum_dinilai":
			query = query.Where("laporan_kerusakan.severity = ? OR laporan_kerusakan.severity IS NULL OR laporan_kerusakan.severity = ''", "belum_dinilai")
		}
	}

	if searchKeyword != "" {
		likePattern := "%" + searchKeyword + "%"
		query = query.Where("laporan_kerusakan.judul LIKE ? OR laporan_kerusakan.deskripsi LIKE ? OR user.name LIKE ?", likePattern, likePattern, likePattern)
	}

	if hasStartDate {
		query = query.Where("laporan_kerusakan.created_at >= ?", startDate)
	}

	if hasEndDate {
		// Menjangkau keseluruhan tanggal akhir secara inklusif (hingga akhir hari)
		endOfDayExclusive := endDate.AddDate(0, 0, 1)
		query = query.Where("laporan_kerusakan.created_at < ?", endOfDayExclusive)
	}

	query.Count(&totalData)

	err := query.Order(orderClause).
		Limit(limit).
		Offset(offset).
		Find(&listLaporan).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil laporan kerusakan",
			"error":   "Gagal mengambil laporan kerusakan",
		})
		return
	}

	for i := range listLaporan {
		if listLaporan[i].Severity == "" {
			listLaporan[i].Severity = "belum_dinilai"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Laporan kerusakan berhasil diambil",
		"data":    listLaporan,
		"total":   totalData,
	})
}

func GetLaporanByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "ID laporan tidak valid",
		})
		return
	}

	roleVal, existsRole := c.Get("role")
	userIDVal, existsUserID := c.Get("user_id")
	if !existsRole || !existsUserID {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "User tidak terautentikasi",
		})
		return
	}

	role, okRole := roleVal.(string)
	if !okRole || role == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Role tidak valid",
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
	default:
		c.JSON(http.StatusBadRequest, gin.H{
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

	var laporan models.LaporanKerusakan
	query := config.DB.Model(&models.LaporanKerusakan{}).
		Preload("User").
		Preload("Wilayah").
		Where("laporan_kerusakan.id = ? AND laporan_kerusakan.deleted_at IS NULL", id)

	switch role {
	case "admin_pemdes":
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
	case "admin_pu", "super_admin":
		// admin_pu dan super_admin: dapat melihat semua jenis jalan pada view detail laporan
	default:
		c.JSON(http.StatusForbidden, gin.H{
			"status":  "error",
			"message": "Akses tidak diizinkan",
		})
		return
	}

	if err := query.First(&laporan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Laporan tidak ditemukan",
		})
		return
	}

	if laporan.Severity == "" {
		laporan.Severity = "belum_dinilai"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Detail laporan berhasil diambil",
		"data":    laporan,
	})
}
