package controllers

import (
	"net/http"
	"os"
	"strings"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UpdateSettingsInput struct {
	NotificationSoundEnabled  *bool   `json:"notification_sound_enabled"`
	NotificationReportEnabled *bool   `json:"notification_report_enabled"`
	NotificationStatusEnabled *bool   `json:"notification_status_enabled"`
	NotificationChatEnabled   *bool   `json:"notification_chat_enabled"`
	Theme                     *string `json:"theme"`
	DisplayDensity            *string `json:"display_density"`
	MapDefaultView            *string `json:"map_default_view"`
	MapShowLabels             *bool   `json:"map_show_labels"`
	ReportDisplayPreference   *string `json:"report_display_preference"`
}

func formatPreferencesResponse(pref models.UserPreference) gin.H {
	theme := pref.Theme
	if theme == "" {
		theme = "light"
	}
	density := pref.DisplayDensity
	if density == "" {
		density = "comfortable"
	}
	mapView := pref.MapDefaultView
	if mapView == "" {
		mapView = "standard"
	}
	reportDisplay := pref.ReportDisplayPreference
	if reportDisplay == "" {
		reportDisplay = "comfortable"
	}

	return gin.H{
		"notification_sound_enabled":  pref.SoundEnabled(),
		"notification_report_enabled": pref.ReportEnabled(),
		"notification_status_enabled": pref.StatusEnabled(),
		"notification_chat_enabled":   pref.ChatEnabled(),
		"theme":                       theme,
		"display_density":             density,
		"map_default_view":            mapView,
		"map_show_labels":             pref.ShowLabels(),
		"report_display_preference":   reportDisplay,
	}
}

// GET /api/settings
func GetSettings(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	var user models.User
	if err := config.DB.Preload("Wilayah").Where("deleted_at IS NULL").First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Pengguna tidak ditemukan",
		})
		return
	}

	// Ambil preferensi pengguna atau inisialisasi default secara idempoten
	var pref models.UserPreference
	err := config.DB.Where("user_id = ?", user.ID).First(&pref).Error
	if err != nil {
		pref = models.NewDefaultUserPreference(user.ID)
		if errCreate := config.DB.FirstOrCreate(&pref, models.UserPreference{UserID: user.ID}).Error; errCreate != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Gagal memuat preferensi pengguna",
			})
			return
		}
	}

	var wilayahData interface{} = nil
	if user.WilayahID != nil && user.Wilayah.ID != 0 {
		wilayahData = gin.H{
			"id":   user.Wilayah.ID,
			"nama": user.Wilayah.Nama,
			"tipe": user.Wilayah.Tipe,
		}
	}

	statusAkun := "active"
	if user.DeletedAt.Valid {
		statusAkun = "inactive"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Pengaturan berhasil diambil",
		"data": gin.H{
			"account": gin.H{
				"id":      user.ID,
				"name":    user.Name,
				"email":   user.Email,
				"role":    user.Role,
				"wilayah": wilayahData,
				"status":  statusAkun,
			},
			"preferences": formatPreferencesResponse(pref),
			"security": gin.H{
				"last_login_at":      user.LastLoginAt,
				"has_active_session": true,
			},
		},
	})
}

// PUT /api/settings
func UpdateSettings(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	var input UpdateSettingsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Format request tidak valid: " + err.Error(),
		})
		return
	}

	// Validasi Enum yang ketat (HTTP 400 jika nilai tidak sah)
	if input.Theme != nil {
		themeVal := strings.ToLower(strings.TrimSpace(*input.Theme))
		if themeVal != "light" && themeVal != "dark" && themeVal != "system" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Theme harus salah satu dari: light, dark, system",
			})
			return
		}
		*input.Theme = themeVal
	}

	if input.DisplayDensity != nil {
		densityVal := strings.ToLower(strings.TrimSpace(*input.DisplayDensity))
		if densityVal != "comfortable" && densityVal != "compact" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Display density harus salah satu dari: comfortable, compact",
			})
			return
		}
		*input.DisplayDensity = densityVal
	}

	if input.MapDefaultView != nil {
		mapVal := strings.ToLower(strings.TrimSpace(*input.MapDefaultView))
		if mapVal != "standard" && mapVal != "satellite" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Map default view harus salah satu dari: standard, satellite",
			})
			return
		}
		*input.MapDefaultView = mapVal
	}

	if input.ReportDisplayPreference != nil {
		reportVal := strings.ToLower(strings.TrimSpace(*input.ReportDisplayPreference))
		if reportVal != "comfortable" && reportVal != "compact" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Report display preference harus salah satu dari: comfortable, compact",
			})
			return
		}
		*input.ReportDisplayPreference = reportVal
	}

	var user models.User
	if err := config.DB.Preload("Wilayah").Where("deleted_at IS NULL").First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Pengguna tidak ditemukan",
		})
		return
	}

	// Cari preference yang sudah ada atau siapkan default
	var pref models.UserPreference
	if err := config.DB.Where("user_id = ?", user.ID).First(&pref).Error; err != nil {
		pref = models.NewDefaultUserPreference(user.ID)
		if errCreate := config.DB.Create(&pref).Error; errCreate != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Gagal membuat preferensi pengguna",
			})
			return
		}
	}

	// Partial update: hanya update field yang dikirim pada request body
	if input.NotificationSoundEnabled != nil {
		pref.NotificationSoundEnabled = input.NotificationSoundEnabled
	}
	if input.NotificationReportEnabled != nil {
		pref.NotificationReportEnabled = input.NotificationReportEnabled
	}
	if input.NotificationStatusEnabled != nil {
		pref.NotificationStatusEnabled = input.NotificationStatusEnabled
	}
	if input.NotificationChatEnabled != nil {
		pref.NotificationChatEnabled = input.NotificationChatEnabled
	}
	if input.Theme != nil {
		pref.Theme = *input.Theme
	}
	if input.DisplayDensity != nil {
		pref.DisplayDensity = *input.DisplayDensity
	}
	if input.MapDefaultView != nil {
		pref.MapDefaultView = *input.MapDefaultView
	}
	if input.MapShowLabels != nil {
		pref.MapShowLabels = input.MapShowLabels
	}
	if input.ReportDisplayPreference != nil {
		pref.ReportDisplayPreference = *input.ReportDisplayPreference
	}

	if errSave := config.DB.Save(&pref).Error; errSave != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal memperbarui preferensi pengguna",
		})
		return
	}

	var wilayahData interface{} = nil
	if user.WilayahID != nil && user.Wilayah.ID != 0 {
		wilayahData = gin.H{
			"id":   user.Wilayah.ID,
			"nama": user.Wilayah.Nama,
			"tipe": user.Wilayah.Tipe,
		}
	}

	statusAkun := "active"
	if user.DeletedAt.Valid {
		statusAkun = "inactive"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Pengaturan berhasil diperbarui",
		"data": gin.H{
			"account": gin.H{
				"id":      user.ID,
				"name":    user.Name,
				"email":   user.Email,
				"role":    user.Role,
				"wilayah": wilayahData,
				"status":  statusAkun,
			},
			"preferences": formatPreferencesResponse(pref),
			"security": gin.H{
				"last_login_at":      user.LastLoginAt,
				"has_active_session": true,
			},
		},
	})
}

// POST /api/settings/logout-all
func LogoutAll(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	// Invalidate semua token aktif milik user dengan increment token_version di database
	if err := config.DB.Model(&models.User{}).
		Where("id = ? AND deleted_at IS NULL", userID).
		Update("token_version", gorm.Expr("token_version + 1")).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengeluarkan semua sesi",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Semua sesi berhasil dikeluarkan.",
	})
}

// GET /api/health
func HealthCheck(c *gin.Context) {
	dbStatus := "connected"
	if config.DB != nil {
		sqlDB, err := config.DB.DB()
		if err != nil || sqlDB.Ping() != nil {
			dbStatus = "disconnected"
		}
	} else {
		dbStatus = "disconnected"
	}

	version := os.Getenv("APP_VERSION")
	if version == "" {
		version = "1.0.0"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"service":  "roadis-api",
		"version":  version,
		"database": dbStatus,
	})
}

// GET /api/system/info
func GetSystemInfo(c *gin.Context) {
	version := os.Getenv("APP_VERSION")
	if version == "" {
		version = "1.0.0"
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"application": "ROADIS",
			"version":     version,
			"environment": env,
			"description": "Sistem Deteksi dan Pemetaan Kerusakan Jalan Berbasis YOLOv11 dan GIS dengan Klasifikasi Kewenangan Jalan untuk Optimasi Routing Pengaduan Infrastruktur di Kabupaten Indramayu.",
		},
	})
}
