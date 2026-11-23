package admin

import (
	"fmt"
	"net/http"
	"time"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TrenItem struct {
	Name      string `json:"name"`
	Count     int64  `json:"count"`
	Month     int    `json:"month,omitempty"`
	Year      int    `json:"year,omitempty"`
	IsCurrent bool   `json:"is_current"`
}

// HitungTrenBulanan menghitung agregasi laporan per bulan untuk trailing 6 bulan berakhir pada bulan berjalan
func HitungTrenBulanan(createdTimes []time.Time, now time.Time) []TrenItem {
	currentYear := now.Year()
	currentMonth := now.Month()

	monthNames := []string{"", "Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}

	type bucketDef struct {
		start time.Time
		end   time.Time
		name  string
		month int
		year  int
		isCur bool
	}

	buckets := make([]bucketDef, 6)
	for i := 0; i < 6; i++ {
		start := time.Date(currentYear, currentMonth-time.Month(5-i), 1, 0, 0, 0, 0, now.Location())
		end := time.Date(currentYear, currentMonth-time.Month(4-i), 1, 0, 0, 0, 0, now.Location())
		buckets[i] = bucketDef{
			start: start,
			end:   end,
			name:  monthNames[start.Month()],
			month: int(start.Month()),
			year:  start.Year(),
			isCur: i == 5,
		}
	}

	result := make([]TrenItem, 6)
	for i, b := range buckets {
		var cnt int64
		for _, t := range createdTimes {
			if (t.Equal(b.start) || t.After(b.start)) && t.Before(b.end) {
				cnt++
			}
		}
		result[i] = TrenItem{
			Name:      b.name,
			Count:     cnt,
			Month:     b.month,
			Year:      b.year,
			IsCurrent: b.isCur,
		}
	}

	return result
}

// HitungTrenMingguan menghitung agregasi laporan per jendela 7 hari untuk 6 minggu terakhir
func HitungTrenMingguan(createdTimes []time.Time, now time.Time) []TrenItem {
	type weekBucketDef struct {
		start time.Time
		end   time.Time
		name  string
		isCur bool
	}

	buckets := make([]weekBucketDef, 6)
	for i := 0; i < 6; i++ {
		start := now.Add(-time.Duration(6-i) * 7 * 24 * time.Hour)
		end := now.Add(-time.Duration(5-i) * 7 * 24 * time.Hour)
		name := fmt.Sprintf("Mg -%d", 5-i)
		if i == 5 {
			name = "Mg Ini"
		}
		buckets[i] = weekBucketDef{
			start: start,
			end:   end,
			name:  name,
			isCur: i == 5,
		}
	}

	result := make([]TrenItem, 6)
	for i, b := range buckets {
		var cnt int64
		for _, t := range createdTimes {
			isMatch := false
			if i == len(buckets)-1 {
				isMatch = (t.Equal(b.start) || t.After(b.start)) && (t.Before(b.end) || t.Equal(b.end))
			} else {
				isMatch = (t.Equal(b.start) || t.After(b.start)) && t.Before(b.end)
			}
			if isMatch {
				cnt++
			}
		}
		result[i] = TrenItem{
			Name:      b.name,
			Count:     cnt,
			IsCurrent: b.isCur,
		}
	}

	return result
}

func GetDashboardStats(c *gin.Context) {
	roleVal, existsRole := c.Get("role")
	userIDVal, existsUserID := c.Get("user_id")

	if !existsRole || !existsUserID {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "User tidak terautentikasi",
		})
		return
	}

	role, ok := roleVal.(string)
	if !ok || role == "" {
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

	var totalLaporan int64
	var totalMenunggu int64
	var totalProses int64
	var totalSelesai int64
	var totalDitolak int64

	if config.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Koneksi database tidak tersedia",
		})
		return
	}

	baseQuery := config.DB.
		Model(&models.LaporanKerusakan{}).
		Where("laporan_kerusakan.deleted_at IS NULL")

	switch role {
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

		baseQuery = baseQuery.Where(
			"laporan_kerusakan.wilayah_id = ? AND laporan_kerusakan.jenis_jalan = ?",
			*adminUser.WilayahID,
			"desa",
		)

	case string(models.RoleAdminPu):
		baseQuery = baseQuery.Where("laporan_kerusakan.jenis_jalan = ?", "kabupaten")

	case string(models.RoleSuperAdmin):
		// Superadmin menghitung seluruh laporan tanpa filter wilayah dan jenis jalan

	default:
		c.JSON(http.StatusForbidden, gin.H{
			"status":  "error",
			"message": "Akses tidak diizinkan",
		})
		return
	}

	if err := baseQuery.Count(&totalLaporan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menghitung total laporan",
		})
		return
	}

	if err := baseQuery.Session(&gorm.Session{}).
		Where("laporan_kerusakan.status = ?", "menunggu").
		Count(&totalMenunggu).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menghitung laporan menunggu",
		})
		return
	}

	if err := baseQuery.Session(&gorm.Session{}).
		Where("laporan_kerusakan.status = ?", "proses").
		Count(&totalProses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menghitung laporan proses",
		})
		return
	}

	if err := baseQuery.Session(&gorm.Session{}).
		Where("laporan_kerusakan.status = ?", "selesai").
		Count(&totalSelesai).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menghitung laporan selesai",
		})
		return
	}

	if err := baseQuery.Session(&gorm.Session{}).
		Where("laporan_kerusakan.status = ?", "ditolak").
		Count(&totalDitolak).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menghitung laporan ditolak",
		})
		return
	}

	now := time.Now()
	sixMonthsAgoStart := time.Date(now.Year(), now.Month()-5, 1, 0, 0, 0, 0, now.Location())

	var createdTimes []time.Time
	_ = baseQuery.Session(&gorm.Session{}).
		Where("laporan_kerusakan.created_at >= ?", sixMonthsAgoStart).
		Pluck("laporan_kerusakan.created_at", &createdTimes)

	trenBulanan := HitungTrenBulanan(createdTimes, now)
	trenMingguan := HitungTrenMingguan(createdTimes, now)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Dashboard stats berhasil diambil",
		"data": gin.H{
			"total_laporan":  totalLaporan,
			"total_menunggu": totalMenunggu,
			"total_proses":   totalProses,
			"total_selesai":  totalSelesai,
			"total_ditolak":  totalDitolak,
			"tren_bulanan":   trenBulanan,
			"tren_mingguan":  trenMingguan,
		},
	})
}
