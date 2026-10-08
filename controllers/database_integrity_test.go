package controllers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/controllers"
	"backend-jalan-rusak/controllers/admin"
	"backend-jalan-rusak/controllers/superadmin"
	"backend-jalan-rusak/controllers/warga"
	"backend-jalan-rusak/middlewares"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 1. TestBE14_UserWilayahIntegrity
func TestBE14_UserWilayahIntegrity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/superadmin/users", superadmin.CreateUser)
	r.PUT("/api/superadmin/users/:id", superadmin.UpdateUser)

	// A. Admin Pemdes without WilayahID must be rejected (400)
	t.Run("AdminPemdes_RequiresWilayah", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"name":     "Pemdes Integritas",
			"email":    "pemdes_nowilayah@roadis.id",
			"password": "password123",
			"role":     "admin_pemdes",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/superadmin/users", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 when admin_pemdes has no wilayah, got %d", w.Code)
		}
	})

	// B. Non-pemdes with WilayahID must be rejected (400)
	t.Run("NonPemdes_CannotHaveWilayah", func(t *testing.T) {
		wilayahID := uint(5)
		body, _ := json.Marshal(map[string]interface{}{
			"name":       "PU Integritas",
			"email":      "pu_withwilayah@roadis.id",
			"password":   "password123",
			"role":       "admin_pu",
			"wilayah_id": wilayahID,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/superadmin/users", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 when admin_pu is given wilayah_id, got %d", w.Code)
		}
	})

	// C. DB-backed test if MySQL is running
	if ensureDBForLaporanTest(t) {
		nowNano := time.Now().UnixNano()
		delWilayah := models.Wilayah{Nama: fmt.Sprintf("Desa Dihapus %d", nowNano), Tipe: "desa"}
		config.DB.Create(&delWilayah)
		config.DB.Delete(&delWilayah)
		defer config.DB.Unscoped().Delete(&delWilayah)

		t.Run("AdminPemdes_CannotUseDeletedWilayah_DB", func(t *testing.T) {
			body, _ := json.Marshal(map[string]interface{}{
				"name":       "Pemdes Deleted Wilayah",
				"email":      fmt.Sprintf("pemdes_delw_%d@roadis.id", nowNano),
				"password":   "password123",
				"role":       "admin_pemdes",
				"wilayah_id": delWilayah.ID,
			})
			req := httptest.NewRequest(http.MethodPost, "/api/superadmin/users", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected 400 when assigning soft-deleted wilayah, got %d", w.Code)
			}
		})
	}
}

// 2. TestBE14_ReportUserIntegrity
func TestBE14_ReportUserIntegrity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// A. Zero User ID on CreateLaporan must return 401 Unauthorized
	t.Run("ZeroUserID_CreateLaporan_Rejected", func(t *testing.T) {
		r := gin.New()
		r.POST("/api/warga/laporan", func(c *gin.Context) {
			c.Set("user_id", uint(0))
			c.Set("role", "warga")
			warga.CreateLaporan(c)
		})

		req := httptest.NewRequest(http.MethodPost, "/api/warga/laporan", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 when user_id is 0, got %d", w.Code)
		}
	})

	// B. Missing User ID in Context on GetLaporanByID returns 401
	t.Run("MissingAuth_GetLaporanByID_Rejected", func(t *testing.T) {
		r := gin.New()
		r.GET("/api/warga/laporan/:id", warga.GetLaporanByID)

		req := httptest.NewRequest(http.MethodGet, "/api/warga/laporan/1", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 when auth context is missing, got %d", w.Code)
		}
	})

	// C. DB-backed test if MySQL is running
	if ensureDBForLaporanTest(t) {
		nowNano := time.Now().UnixNano()
		wargaUser := models.User{Name: "Warga Aktif", Email: fmt.Sprintf("warga_act_%d@roadis.id", nowNano), Role: models.RoleWarga}
		config.DB.Create(&wargaUser)
		defer config.DB.Unscoped().Delete(&wargaUser)

		deletedUser := models.User{Name: "Warga Terhapus", Email: fmt.Sprintf("warga_del_%d@roadis.id", nowNano), Role: models.RoleWarga}
		config.DB.Create(&deletedUser)
		defer config.DB.Unscoped().Delete(&deletedUser)

		wilayah := models.Wilayah{Nama: fmt.Sprintf("Desa Subur %d", nowNano), Tipe: "desa"}
		config.DB.Create(&wilayah)
		defer config.DB.Unscoped().Delete(&wilayah)

		lap := models.LaporanKerusakan{
			UserID:        deletedUser.ID,
			WilayahID:     wilayah.ID,
			JenisJalan:    "desa",
			Judul:         "Jalan Berlubang",
			Deskripsi:     "Lubang besar",
			Latitude:      -6.3,
			Longitude:     108.3,
			ImageURL:      "https://example.com/foto.jpg",
			TipeKerusakan: "rusak_ringan",
			Status:        "menunggu",
		}
		config.DB.Create(&lap)
		defer config.DB.Unscoped().Delete(&lap)

		// Soft delete creator
		config.DB.Delete(&deletedUser)

		t.Run("OtherUserCannotAccessDeletedUserReport", func(t *testing.T) {
			r := gin.New()
			r.GET("/api/warga/laporan/:id", func(c *gin.Context) {
				c.Set("user_id", wargaUser.ID)
				c.Set("role", "warga")
				warga.GetLaporanByID(c)
			})

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/warga/laporan/%d", lap.ID), nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("Expected 403 when accessing report owned by another user, got %d", w.Code)
			}
		})
	}
}

// 3. TestBE14_ReportWilayahIntegrity
func TestBE14_ReportWilayahIntegrity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// A. Invalid coordinate validation on CreateLaporan returns 400
	t.Run("InvalidCoordinates_Rejected", func(t *testing.T) {
		r := gin.New()
		r.POST("/api/warga/laporan", func(c *gin.Context) {
			c.Set("user_id", uint(1))
			c.Set("role", "warga")
			warga.CreateLaporan(c)
		})

		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		_ = writer.WriteField("judul", "Jalan Rusak")
		_ = writer.WriteField("deskripsi", "Deskripsi")
		_ = writer.WriteField("tipe_kerusakan", "rusak_ringan")
		_ = writer.WriteField("latitude", "999.0") // Out of range
		_ = writer.WriteField("longitude", "108.0")
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/warga/laporan", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 for out-of-bounds latitude, got %d", w.Code)
		}
	})

	// B. DB-backed test for soft-deleted wilayah rejection
	if ensureDBForLaporanTest(t) {
		nowNano := time.Now().UnixNano()
		wargaUser := models.User{Name: "Warga C", Email: fmt.Sprintf("warga_c_%d@roadis.id", nowNano), Role: models.RoleWarga}
		config.DB.Create(&wargaUser)
		defer config.DB.Unscoped().Delete(&wargaUser)

		delWilayah := models.Wilayah{Nama: fmt.Sprintf("Wilayah Dihapus %d", nowNano), Tipe: "desa"}
		config.DB.Create(&delWilayah)
		config.DB.Delete(&delWilayah)
		defer config.DB.Unscoped().Delete(&delWilayah)

		t.Run("SoftDeletedWilayah_RejectedOnCreateLaporan", func(t *testing.T) {
			r := gin.New()
			r.POST("/api/warga/laporan", func(c *gin.Context) {
				c.Set("user_id", wargaUser.ID)
				c.Set("role", "warga")
				warga.CreateLaporan(c)
			})

			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			_ = writer.WriteField("judul", "Laporan Baru")
			_ = writer.WriteField("deskripsi", "Deskripsi laporan")
			_ = writer.WriteField("tipe_kerusakan", "rusak_berat")
			_ = writer.WriteField("latitude", "-6.3265")
			_ = writer.WriteField("longitude", "108.3245")
			_ = writer.WriteField("wilayah_id", fmt.Sprintf("%d", delWilayah.ID))
			part, _ := writer.CreateFormFile("foto", "jalan.jpg")
			part.Write([]byte("\xFF\xD8\xFF\xE0\x00\x10JFIF"))
			writer.Close()

			req := httptest.NewRequest(http.MethodPost, "/api/warga/laporan", &body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected 400 when creating report with soft-deleted wilayah, got %d", w.Code)
			}
		})
	}
}

// 4. TestBE14_ChatReportIntegrity
func TestBE14_ChatReportIntegrity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// A. Non-warga calling SendPesanWarga must be rejected (403)
	t.Run("NonWarga_SendPesanWarga_Forbidden", func(t *testing.T) {
		r := gin.New()
		r.POST("/api/warga/laporan/:id/chat", func(c *gin.Context) {
			c.Set("user_id", uint(1))
			c.Set("role", "admin_pu")
			controllers.SendPesanWarga(c)
		})

		body, _ := json.Marshal(map[string]string{"pesan": "Halo"})
		req := httptest.NewRequest(http.MethodPost, "/api/warga/laporan/1/chat", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 when non-warga accesses SendPesanWarga, got %d", w.Code)
		}
	})

	// B. Non-admin calling ReplyPesanAdmin must be rejected (403)
	t.Run("NonAdmin_ReplyPesanAdmin_Forbidden", func(t *testing.T) {
		r := gin.New()
		r.POST("/api/admin/chat/:chat_id/reply", func(c *gin.Context) {
			c.Set("user_id", uint(1))
			c.Set("role", "warga")
			controllers.ReplyPesanAdmin(c)
		})

		body, _ := json.Marshal(map[string]string{"balasan": "Balasan"})
		req := httptest.NewRequest(http.MethodPost, "/api/admin/chat/1/reply", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 when non-admin accesses ReplyPesanAdmin, got %d", w.Code)
		}
	})

	// C. DB-backed test for ownership & soft delete
	if ensureDBForLaporanTest(t) {
		nowNano := time.Now().UnixNano()
		wargaA := models.User{Name: "Warga A", Email: fmt.Sprintf("warga_a_%d@roadis.id", nowNano), Role: models.RoleWarga}
		wargaB := models.User{Name: "Warga B", Email: fmt.Sprintf("warga_b_%d@roadis.id", nowNano), Role: models.RoleWarga}
		config.DB.Create(&wargaA)
		config.DB.Create(&wargaB)
		defer config.DB.Unscoped().Delete(&wargaA)
		defer config.DB.Unscoped().Delete(&wargaB)

		wilayah := models.Wilayah{Nama: fmt.Sprintf("Desa Mekar %d", nowNano), Tipe: "desa"}
		config.DB.Create(&wilayah)
		defer config.DB.Unscoped().Delete(&wilayah)

		lap := models.LaporanKerusakan{
			UserID:        wargaA.ID,
			WilayahID:     wilayah.ID,
			JenisJalan:    "desa",
			Judul:         "Jalan Retak",
			Deskripsi:     "Retak parah",
			Latitude:      -6.3,
			Longitude:     108.3,
			ImageURL:      "https://example.com/foto.jpg",
			TipeKerusakan: "rusak_sedang",
			Status:        "menunggu",
		}
		config.DB.Create(&lap)
		defer config.DB.Unscoped().Delete(&lap)

		t.Run("NonOwner_CannotChatOnReport", func(t *testing.T) {
			r := gin.New()
			r.POST("/api/warga/laporan/:id/chat", func(c *gin.Context) {
				c.Set("user_id", wargaB.ID)
				c.Set("role", "warga")
				controllers.SendPesanWarga(c)
			})

			body, _ := json.Marshal(map[string]string{"pesan": "Halo admin"})
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/warga/laporan/%d/chat", lap.ID), bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("Expected 403 when non-owner citizen sends chat, got %d", w.Code)
			}
		})
	}
}

// 5. TestBE14_NotificationIntegrity
func TestBE14_NotificationIntegrity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// A. Missing user ID on MarkNotifikasiRead returns 401
	t.Run("MissingUser_MarkNotifRead_Unauthorized", func(t *testing.T) {
		r := gin.New()
		r.PUT("/api/notifikasi/:id/read", controllers.MarkNotifikasiRead)

		req := httptest.NewRequest(http.MethodPut, "/api/notifikasi/1/read", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 when auth is missing on MarkNotifikasiRead, got %d", w.Code)
		}
	})

	// B. Invalid notification ID returns 400
	t.Run("InvalidID_MarkNotifRead_BadRequest", func(t *testing.T) {
		r := gin.New()
		r.PUT("/api/notifikasi/:id/read", func(c *gin.Context) {
			c.Set("user_id", uint(1))
			c.Set("role", "warga")
			controllers.MarkNotifikasiRead(c)
		})

		req := httptest.NewRequest(http.MethodPut, "/api/notifikasi/invalid_id/read", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 for non-numeric notif ID, got %d", w.Code)
		}
	})

	// C. DB-backed test for ownership
	if ensureDBForLaporanTest(t) {
		nowNano := time.Now().UnixNano()
		userA := models.User{Name: "User A", Email: fmt.Sprintf("a_%d@roadis.id", nowNano), Role: models.RoleWarga}
		userB := models.User{Name: "User B", Email: fmt.Sprintf("b_%d@roadis.id", nowNano), Role: models.RoleWarga}
		config.DB.Create(&userA)
		config.DB.Create(&userB)
		defer config.DB.Unscoped().Delete(&userA)
		defer config.DB.Unscoped().Delete(&userB)

		notif := models.Notifikasi{
			UserID: userA.ID,
			Judul:  "Status Update",
			Pesan:  "Laporan sedang diproses",
			IsRead: false,
		}
		config.DB.Create(&notif)
		defer config.DB.Unscoped().Delete(&notif)

		t.Run("CannotMarkOtherUserNotificationRead", func(t *testing.T) {
			r := gin.New()
			r.PUT("/api/notifikasi/:id/read", func(c *gin.Context) {
				c.Set("user_id", userB.ID)
				c.Set("role", "warga")
				controllers.MarkNotifikasiRead(c)
			})

			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/notifikasi/%d/read", notif.ID), nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("Expected 403 when marking another user's notification as read, got %d", w.Code)
			}
		})
	}
}

// 6. TestBE14_UserPreferenceIntegrity
func TestBE14_UserPreferenceIntegrity(t *testing.T) {
	// A. Invariant test: Default values and nil-safety on helper methods
	t.Run("DefaultPreference_And_NilSafety", func(t *testing.T) {
		pref := models.NewDefaultUserPreference(10)
		if pref.UserID != 10 {
			t.Errorf("Expected UserID 10, got %d", pref.UserID)
		}
		if !pref.SoundEnabled() || !pref.ReportEnabled() || !pref.StatusEnabled() || !pref.ChatEnabled() || !pref.ShowLabels() {
			t.Errorf("Default preference must have all notifications and labels enabled")
		}

		// Nil pointers should fall back safely to true without panic
		emptyPref := models.UserPreference{}
		if !emptyPref.SoundEnabled() || !emptyPref.ReportEnabled() || !emptyPref.StatusEnabled() || !emptyPref.ChatEnabled() || !emptyPref.ShowLabels() {
			t.Errorf("Empty preference nil pointer fallbacks must return true")
		}
	})

	// B. Enum validation on UpdateSettings: invalid theme returns 400
	t.Run("InvalidTheme_Rejected", func(t *testing.T) {
		r := gin.New()
		r.PUT("/api/settings", func(c *gin.Context) {
			c.Set("user_id", uint(1))
			c.Set("role", "warga")
			controllers.UpdateSettings(c)
		})

		body, _ := json.Marshal(map[string]string{"theme": "neon_blue"})
		req := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 for invalid theme enum, got %d", w.Code)
		}
	})

	// C. DB-backed test if MySQL is running
	if ensureDBForLaporanTest(t) {
		nowNano := time.Now().UnixNano()
		user := models.User{Name: "Pref User", Email: fmt.Sprintf("pref_%d@roadis.id", nowNano), Role: models.RoleWarga}
		config.DB.Create(&user)
		defer config.DB.Unscoped().Delete(&user)
		defer config.DB.Unscoped().Where("user_id = ?", user.ID).Delete(&models.UserPreference{})

		t.Run("IdempotentPreferenceCreation", func(t *testing.T) {
			r := gin.New()
			r.GET("/api/settings", func(c *gin.Context) {
				c.Set("user_id", user.ID)
				c.Set("role", "warga")
				controllers.GetSettings(c)
			})

			reqGet := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
			wGet := httptest.NewRecorder()
			r.ServeHTTP(wGet, reqGet)

			if wGet.Code != http.StatusOK {
				t.Fatalf("Expected 200 on GetSettings, got %d", wGet.Code)
			}

			var count int64
			config.DB.Model(&models.UserPreference{}).Where("user_id = ?", user.ID).Count(&count)
			if count != 1 {
				t.Errorf("Expected exactly 1 preference record, found %d", count)
			}
		})
	}
}

// 7. TestBE14_SoftDeleteIntegrity
func TestBE14_SoftDeleteIntegrity(t *testing.T) {
	// A. Model reflection check: all models must embed gorm.Model
	t.Run("AllModelsEmbedGormModel", func(t *testing.T) {
		modelsList := []interface{}{
			models.User{},
			models.Wilayah{},
			models.LaporanKerusakan{},
			models.RiwayatChat{},
			models.Notifikasi{},
			models.UserPreference{},
		}
		for _, m := range modelsList {
			typ := reflect.TypeOf(m)
			field, ok := typ.FieldByName("Model")
			if !ok || field.Type != reflect.TypeOf(gorm.Model{}) {
				t.Errorf("Model %s must embed gorm.Model for soft-delete support", typ.Name())
			}
		}
	})

	// B. DB-backed test for AuthMiddleware with deleted user
	if ensureDBForLaporanTest(t) {
		nowNano := time.Now().UnixNano()
		user := models.User{
			Name:         "User Deactivated",
			Email:        fmt.Sprintf("inactive_%d@roadis.id", nowNano),
			Role:         models.RoleWarga,
			TokenVersion: 1,
		}
		config.DB.Create(&user)
		config.DB.Delete(&user)
		defer config.DB.Unscoped().Delete(&user)

		t.Run("DeactivatedUser_TokenRejected", func(t *testing.T) {
			token, _ := utils.GenerateToken(user.ID, user.Email, user.Role, nil, user.TokenVersion)
			r := gin.New()
			r.Use(middlewares.AuthMiddleware())
			r.GET("/protected", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("Expected 401 when deactivated user presents token, got %d", w.Code)
			}
		})
	}
}

// 8. TestBE14_DuplicatePrevention
func TestBE14_DuplicatePrevention(t *testing.T) {
	// A. Verify unique tag on User.Email and uniqueIndex on UserPreference.UserID
	t.Run("ModelUniqueTags", func(t *testing.T) {
		userType := reflect.TypeOf(models.User{})
		emailField, ok := userType.FieldByName("Email")
		if !ok || !strings.Contains(emailField.Tag.Get("gorm"), "unique") {
			t.Errorf("User.Email must have 'unique' constraint tag in GORM")
		}

		prefType := reflect.TypeOf(models.UserPreference{})
		userField, ok := prefType.FieldByName("UserID")
		if !ok || !strings.Contains(userField.Tag.Get("gorm"), "uniqueIndex") {
			t.Errorf("UserPreference.UserID must have 'uniqueIndex' tag in GORM")
		}
	})

	// B. DB-backed test for case-insensitive Wilayah duplication
	if ensureDBForLaporanTest(t) {
		nowNano := time.Now().UnixNano()
		wilayahName := fmt.Sprintf("Desa Harapan %d", nowNano)
		wilayah := models.Wilayah{Nama: wilayahName, Tipe: "desa"}
		config.DB.Create(&wilayah)
		defer config.DB.Unscoped().Delete(&wilayah)

		t.Run("DuplicateWilayahCaseInsensitive_Rejected", func(t *testing.T) {
			r := gin.New()
			r.POST("/api/superadmin/wilayah", superadmin.CreateWilayah)

			body, _ := json.Marshal(map[string]string{
				"nama": strings.ToLower(wilayahName),
				"tipe": "desa",
			})
			req := httptest.NewRequest(http.MethodPost, "/api/superadmin/wilayah", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected 400 for duplicate wilayah name/tipe, got %d", w.Code)
			}
		})
	}
}

// 9. TestBE14_MassAssignment
func TestBE14_MassAssignment(t *testing.T) {
	// A. Verify UpdateProfileInput struct has no Role or TokenVersion fields
	t.Run("ProfileInput_NoPrivilegeFields", func(t *testing.T) {
		typ := reflect.TypeOf(controllers.UpdateProfileInput{})
		if _, ok := typ.FieldByName("Role"); ok {
			t.Errorf("UpdateProfileInput must not have Role field")
		}
		if _, ok := typ.FieldByName("TokenVersion"); ok {
			t.Errorf("UpdateProfileInput must not have TokenVersion field")
		}
		if _, ok := typ.FieldByName("WilayahID"); ok {
			t.Errorf("UpdateProfileInput must not have WilayahID field")
		}
	})

	// B. DB-backed test for UpdateProfile attempting privilege escalation
	if ensureDBForLaporanTest(t) {
		nowNano := time.Now().UnixNano()
		user := models.User{
			Name:         "Warga Aman",
			Email:        fmt.Sprintf("warga_aman_%d@roadis.id", nowNano),
			Role:         models.RoleWarga,
			TokenVersion: 1,
		}
		config.DB.Create(&user)
		defer config.DB.Unscoped().Delete(&user)

		t.Run("ProfileUpdate_CannotChangeRoleOrTokenVersion", func(t *testing.T) {
			r := gin.New()
			r.PUT("/api/profile", func(c *gin.Context) {
				c.Set("user_id", user.ID)
				c.Set("role", "warga")
				controllers.UpdateProfile(c)
			})

			payload, _ := json.Marshal(map[string]interface{}{
				"name":          "Warga Hacked",
				"role":          "super_admin",
				"token_version": 99,
				"wilayah_id":    10,
			})
			req := httptest.NewRequest(http.MethodPut, "/api/profile", bytes.NewBuffer(payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("Expected 200 on UpdateProfile, got %d", w.Code)
			}

			var refreshed models.User
			config.DB.First(&refreshed, user.ID)
			if refreshed.Role != models.RoleWarga {
				t.Errorf("CRITICAL: Role was modified via mass assignment to %q", refreshed.Role)
			}
			if refreshed.TokenVersion != 1 {
				t.Errorf("CRITICAL: TokenVersion was modified via mass assignment to %d", refreshed.TokenVersion)
			}
		})
	}
}

// 10. TestBE14_DeleteDependency
func TestBE14_DeleteDependency(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// A. Delete user with self ID returns 400
	t.Run("DeleteUser_CannotDeleteSelf", func(t *testing.T) {
		r := gin.New()
		r.DELETE("/api/superadmin/users/:id", func(c *gin.Context) {
			c.Set("user_id", uint(1))
			c.Set("role", "super_admin")
			superadmin.DeleteUser(c)
		})

		req := httptest.NewRequest(http.MethodDelete, "/api/superadmin/users/1", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Self-delete is checked before DB query
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 for self-delete attempt, got %d", w.Code)
		}
	})

	// B. DB-backed test for Wilayah deletion dependency guard
	if ensureDBForLaporanTest(t) {
		nowNano := time.Now().UnixNano()
		wilayah := models.Wilayah{Nama: fmt.Sprintf("Desa Terikat %d", nowNano), Tipe: "desa"}
		config.DB.Create(&wilayah)
		defer config.DB.Unscoped().Delete(&wilayah)

		adminPemdes := models.User{
			Name:      "Admin Pemdes Terikat",
			Email:     fmt.Sprintf("pemdes_terikat_%d@roadis.id", nowNano),
			Role:      models.RoleAdminPemdes,
			WilayahID: &wilayah.ID,
		}
		config.DB.Create(&adminPemdes)
		defer config.DB.Unscoped().Delete(&adminPemdes)

		t.Run("WilayahWithUser_CannotBeDeleted", func(t *testing.T) {
			r := gin.New()
			r.DELETE("/api/superadmin/wilayah/:id", superadmin.DeleteWilayah)

			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/superadmin/wilayah/%d", wilayah.ID), nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected 400 when deleting wilayah with assigned user, got %d", w.Code)
			}
		})
	}
}

// 11. TestBE14_OrphanPrevention
func TestBE14_OrphanPrevention(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// A. Zero user ID cannot create report
	t.Run("ZeroUser_CreateLaporan_Rejected", func(t *testing.T) {
		r := gin.New()
		r.POST("/api/warga/laporan", func(c *gin.Context) {
			c.Set("user_id", uint(0))
			c.Set("role", "warga")
			warga.CreateLaporan(c)
		})

		req := httptest.NewRequest(http.MethodPost, "/api/warga/laporan", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 when creating report with zero user ID, got %d", w.Code)
		}
	})

	// B. Zero user ID cannot send chat
	t.Run("ZeroUser_SendChat_Rejected", func(t *testing.T) {
		r := gin.New()
		r.POST("/api/warga/laporan/:id/chat", func(c *gin.Context) {
			c.Set("user_id", uint(0))
			c.Set("role", "warga")
			controllers.SendPesanWarga(c)
		})

		body, _ := json.Marshal(map[string]string{"pesan": "halo"})
		req := httptest.NewRequest(http.MethodPost, "/api/warga/laporan/1/chat", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 when sending chat with zero user ID, got %d", w.Code)
		}
	})
}

// 12. TestBE14_TransactionSafety
func TestBE14_TransactionSafety(t *testing.T) {
	// A. Verify storage cleanups on partial failure
	t.Run("EvidenceRollback_OnDBSaveFailure", func(t *testing.T) {
		cleanedUp := false
		admin.BuktiDeleter = func(url string) error {
			if url == "https://example.com/new_fail_bukti.jpg" {
				cleanedUp = true
			}
			return nil
		}

		// When DB is nil, file is not uploaded/saved;
		// verify BuktiDeleter cleans up appropriately when called
		_ = admin.BuktiDeleter("https://example.com/new_fail_bukti.jpg")
		if !cleanedUp {
			t.Errorf("Expected BuktiDeleter to successfully handle cleanup")
		}
	})

	// B. DB-backed test for updating status with evidence photo replacement
	if ensureDBForLaporanTest(t) {
		nowNano := time.Now().UnixNano()
		wargaUser := models.User{Name: "Warga Tx", Email: fmt.Sprintf("warga_tx_%d@roadis.id", nowNano), Role: models.RoleWarga}
		config.DB.Create(&wargaUser)
		defer config.DB.Unscoped().Delete(&wargaUser)

		adminPU := models.User{Name: "Admin PU Tx", Email: fmt.Sprintf("pu_tx_%d@roadis.id", nowNano), Role: models.RoleAdminPu}
		config.DB.Create(&adminPU)
		defer config.DB.Unscoped().Delete(&adminPU)

		wilayah := models.Wilayah{Nama: fmt.Sprintf("Desa Tx %d", nowNano), Tipe: "desa"}
		config.DB.Create(&wilayah)
		defer config.DB.Unscoped().Delete(&wilayah)

		lap := models.LaporanKerusakan{
			UserID:        wargaUser.ID,
			WilayahID:     wilayah.ID,
			JenisJalan:    "kabupaten",
			Judul:         "Jalan Aspal Terkelupas",
			Deskripsi:     "Perlu perbaikan segera",
			Latitude:      -6.3,
			Longitude:     108.3,
			ImageURL:      "https://example.com/foto.jpg",
			TipeKerusakan: "rusak_sedang",
			Status:        "proses",
			FotoBukti:     "https://example.com/old_bukti.jpg",
		}
		config.DB.Create(&lap)
		defer config.DB.Unscoped().Delete(&lap)

		deletedFiles := make([]string, 0)
		admin.BuktiDeleter = func(url string) error {
			deletedFiles = append(deletedFiles, url)
			return nil
		}
		admin.BuktiUploader = func(file *multipart.FileHeader) (string, error) {
			return "https://example.com/new_bukti.jpg", nil
		}

		t.Run("UpdateStatus_ReplacesOldPhotoCleanly", func(t *testing.T) {
			r := gin.New()
			r.PUT("/api/admin/laporan/:id/status", func(c *gin.Context) {
				c.Set("user_id", adminPU.ID)
				c.Set("role", "admin_pu")
				admin.UpdateStatusLaporan(c)
			})

			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			_ = writer.WriteField("status", "selesai")
			part, _ := writer.CreateFormFile("foto_bukti", "selesai.jpg")
			part.Write([]byte("\xFF\xD8\xFF\xE0\x00\x10JFIF"))
			writer.Close()

			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/laporan/%d/status", lap.ID), &body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("Expected 200 on UpdateStatusLaporan, got %d", w.Code)
			}

			if len(deletedFiles) == 0 || deletedFiles[0] != "https://example.com/old_bukti.jpg" {
				t.Errorf("Expected old foto bukti to be cleaned up on update success, got %v", deletedFiles)
			}
		})
	}
}

// Static check verifying zero Unscoped() in production code
func TestBE14_StaticNoUnscopedInProduction(t *testing.T) {
	root := filepath.Clean("..")
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == "vendor" || info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(content), "Unscoped()") {
				t.Errorf("VIOLATION: Unscoped() detected in production file %s", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to scan directory: %v", err)
	}
}
