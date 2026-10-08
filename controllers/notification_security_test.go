package controllers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/controllers"
	"backend-jalan-rusak/controllers/warga"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/routes"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

type notifSecurityFixtures struct {
	desaA      models.Wilayah
	desaB      models.Wilayah
	wargaA     models.User
	wargaB     models.User
	pemdesA    models.User
	pemdesB    models.User
	adminPU    models.User
	superAdmin models.User
	deletedUser models.User

	tokenWargaA     string
	tokenWargaB     string
	tokenPemdesA    string
	tokenPemdesB    string
	tokenPU         string
	tokenSA         string
	tokenDeletedUser string

	lapDesaA     models.LaporanKerusakan
	lapKabupaten models.LaporanKerusakan

	notifWargaA1 models.Notifikasi
	notifWargaA2 models.Notifikasi
	notifWargaB1 models.Notifikasi
	notifPemdesA models.Notifikasi
	notifDeleted models.Notifikasi
}

func setupNotifSecurityFixtures(t *testing.T) (*notifSecurityFixtures, func()) {
	if !ensureDBForLaporanTest(t) {
		t.Skip("Database not available")
	}

	f := &notifSecurityFixtures{}
	nowNano := time.Now().UnixNano()

	// 1. Wilayah
	f.desaA = models.Wilayah{Nama: fmt.Sprintf("Desa A Notif %d", nowNano), Tipe: "desa"}
	config.DB.Create(&f.desaA)
	f.desaB = models.Wilayah{Nama: fmt.Sprintf("Desa B Notif %d", nowNano), Tipe: "desa"}
	config.DB.Create(&f.desaB)

	// 2. Users
	f.wargaA = models.User{Name: "Warga A Notif", Email: fmt.Sprintf("warga_a_%d@notif.test", nowNano), Role: models.RoleWarga, TokenVersion: 1}
	config.DB.Create(&f.wargaA)
	f.wargaB = models.User{Name: "Warga B Notif", Email: fmt.Sprintf("warga_b_%d@notif.test", nowNano), Role: models.RoleWarga, TokenVersion: 1}
	config.DB.Create(&f.wargaB)

	f.pemdesA = models.User{Name: "Pemdes A Notif", Email: fmt.Sprintf("pemdes_a_%d@notif.test", nowNano), Role: models.RoleAdminPemdes, WilayahID: &f.desaA.ID, TokenVersion: 1}
	config.DB.Create(&f.pemdesA)
	f.pemdesB = models.User{Name: "Pemdes B Notif", Email: fmt.Sprintf("pemdes_b_%d@notif.test", nowNano), Role: models.RoleAdminPemdes, WilayahID: &f.desaB.ID, TokenVersion: 1}
	config.DB.Create(&f.pemdesB)

	f.adminPU = models.User{Name: "PU Notif", Email: fmt.Sprintf("pu_%d@notif.test", nowNano), Role: models.RoleAdminPu, TokenVersion: 1}
	config.DB.Create(&f.adminPU)
	f.superAdmin = models.User{Name: "SA Notif", Email: fmt.Sprintf("sa_%d@notif.test", nowNano), Role: models.RoleSuperAdmin, TokenVersion: 1}
	config.DB.Create(&f.superAdmin)

	f.deletedUser = models.User{
		Name:         "Deleted User Notif",
		Email:        fmt.Sprintf("deleted_%d@notif.test", nowNano),
		Role:         models.RoleWarga,
		TokenVersion: 1,
		Model:        gorm.Model{DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}},
	}
	config.DB.Create(&f.deletedUser)

	// 3. Tokens
	f.tokenWargaA, _ = utils.GenerateToken(f.wargaA.ID, f.wargaA.Email, f.wargaA.Role, nil)
	f.tokenWargaB, _ = utils.GenerateToken(f.wargaB.ID, f.wargaB.Email, f.wargaB.Role, nil)
	f.tokenPemdesA, _ = utils.GenerateToken(f.pemdesA.ID, f.pemdesA.Email, f.pemdesA.Role, nil)
	f.tokenPemdesB, _ = utils.GenerateToken(f.pemdesB.ID, f.pemdesB.Email, f.pemdesB.Role, nil)
	f.tokenPU, _ = utils.GenerateToken(f.adminPU.ID, f.adminPU.Email, f.adminPU.Role, nil)
	f.tokenSA, _ = utils.GenerateToken(f.superAdmin.ID, f.superAdmin.Email, f.superAdmin.Role, nil)
	f.tokenDeletedUser, _ = utils.GenerateToken(f.deletedUser.ID, f.deletedUser.Email, f.deletedUser.Role, nil)

	// 4. Reports
	f.lapDesaA = models.LaporanKerusakan{
		UserID: f.wargaA.ID, WilayahID: f.desaA.ID, JenisJalan: "desa", Judul: "Laporan Desa A",
		Deskripsi: "Deskripsi", Latitude: -6.3400, Longitude: 108.3300, ImageURL: "https://foto.jpg",
		TipeKerusakan: "lubang", Status: "menunggu",
	}
	config.DB.Create(&f.lapDesaA)

	f.lapKabupaten = models.LaporanKerusakan{
		UserID: f.wargaA.ID, WilayahID: f.desaA.ID, JenisJalan: "kabupaten", Judul: "Laporan Kabupaten",
		Deskripsi: "Deskripsi", Latitude: -6.3450, Longitude: 108.3350, ImageURL: "https://foto.jpg",
		TipeKerusakan: "retak", Status: "menunggu",
	}
	config.DB.Create(&f.lapKabupaten)

	// 5. Notifications
	f.notifWargaA1 = models.Notifikasi{UserID: f.wargaA.ID, LaporanID: f.lapDesaA.ID, Judul: "Notif A1", Pesan: "Pesan A1", IsRead: false}
	config.DB.Create(&f.notifWargaA1)

	f.notifWargaA2 = models.Notifikasi{UserID: f.wargaA.ID, LaporanID: f.lapDesaA.ID, Judul: "Notif A2", Pesan: "Pesan A2", IsRead: true}
	config.DB.Create(&f.notifWargaA2)

	f.notifWargaB1 = models.Notifikasi{UserID: f.wargaB.ID, LaporanID: f.lapDesaA.ID, Judul: "Notif B1", Pesan: "Pesan B1", IsRead: false}
	config.DB.Create(&f.notifWargaB1)

	f.notifPemdesA = models.Notifikasi{UserID: f.pemdesA.ID, LaporanID: f.lapDesaA.ID, Judul: "Notif Pemdes A", Pesan: "Pesan Pemdes A", IsRead: false}
	config.DB.Create(&f.notifPemdesA)

	f.notifDeleted = models.Notifikasi{
		UserID:    f.wargaA.ID,
		LaporanID: f.lapDesaA.ID,
		Judul:     "Notif Deleted",
		Pesan:     "Pesan Deleted",
		Model:     gorm.Model{DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}},
	}
	config.DB.Create(&f.notifDeleted)

	teardown := func() {
		config.DB.Unscoped().Delete(&f.notifWargaA1)
		config.DB.Unscoped().Delete(&f.notifWargaA2)
		config.DB.Unscoped().Delete(&f.notifWargaB1)
		config.DB.Unscoped().Delete(&f.notifPemdesA)
		config.DB.Unscoped().Delete(&f.notifDeleted)
		config.DB.Unscoped().Delete(&f.lapDesaA)
		config.DB.Unscoped().Delete(&f.lapKabupaten)
		config.DB.Unscoped().Where("user_id IN ?", []uint{f.wargaA.ID, f.wargaB.ID, f.pemdesA.ID, f.pemdesB.ID, f.adminPU.ID, f.superAdmin.ID, f.deletedUser.ID}).Delete(&models.UserPreference{})
		config.DB.Unscoped().Where("user_id IN ?", []uint{f.wargaA.ID, f.wargaB.ID, f.pemdesA.ID, f.pemdesB.ID, f.adminPU.ID, f.superAdmin.ID, f.deletedUser.ID}).Delete(&models.Notifikasi{})
		config.DB.Unscoped().Delete(&f.wargaA)
		config.DB.Unscoped().Delete(&f.wargaB)
		config.DB.Unscoped().Delete(&f.pemdesA)
		config.DB.Unscoped().Delete(&f.pemdesB)
		config.DB.Unscoped().Delete(&f.adminPU)
		config.DB.Unscoped().Delete(&f.superAdmin)
		config.DB.Unscoped().Delete(&f.deletedUser)
		config.DB.Unscoped().Delete(&f.desaA)
		config.DB.Unscoped().Delete(&f.desaB)
	}

	return f, teardown
}

// =========================================================================
// 1. INTEGRATION TESTS (WITH DATABASE FIXTURES)
// =========================================================================
func TestBE10_Notifikasi_Integration(t *testing.T) {
	f, cleanup := setupNotifSecurityFixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// A. Ownership & Isolation on List
	t.Run("Warga A only sees own notifications", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/notifikasi", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp struct {
			Data []models.Notifikasi `json:"data"`
			UnreadCount int64        `json:"unread_count"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		for _, n := range resp.Data {
			if n.UserID != f.wargaA.ID {
				t.Errorf("IDOR: Warga A received notification of User %d", n.UserID)
			}
			if n.ID == f.notifDeleted.ID {
				t.Errorf("SOFT-DELETE LEAK: Soft-deleted notification appeared in list")
			}
		}
	})

	// B. IDOR on Mark Read
	t.Run("Warga A cannot mark read Warga B notification -> 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/notifikasi/%d/read", f.notifWargaB1.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for marking foreign notification, got %d", w.Code)
		}
	})

	// C. Superadmin cannot mark read personal notification of another user -> 403
	t.Run("Superadmin cannot mark read another user's personal notification -> 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/notifikasi/%d/read", f.notifWargaA1.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenSA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for superadmin on foreign personal notification, got %d", w.Code)
		}
	})

	// D. Own Mark Read -> 200 & Idempotent
	t.Run("Warga A marks own notification as read -> 200 and idempotent", func(t *testing.T) {
		req1 := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/notifikasi/%d/read", f.notifWargaA1.ID), nil)
		req1.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)

		if w1.Code != http.StatusOK {
			t.Fatalf("expected 200 for marking own notification read, got %d", w1.Code)
		}

		// Second call (idempotent)
		req2 := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/notifikasi/%d/read", f.notifWargaA1.ID), nil)
		req2.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)

		if w2.Code != http.StatusOK {
			t.Errorf("expected 200 for idempotent mark read, got %d", w2.Code)
		}
	})

	// E. Mark All Read -> isolates to authenticated user
	t.Run("Mark all read only affects authenticated user", func(t *testing.T) {
		// Ensure Warga B has unread notification
		var beforeB models.Notifikasi
		config.DB.First(&beforeB, f.notifWargaB1.ID)
		if beforeB.IsRead {
			t.Fatalf("precondition failed: Warga B notification should be unread")
		}

		req := httptest.NewRequest(http.MethodPut, "/api/notifikasi/read-all", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 for mark all read, got %d", w.Code)
		}

		// Verify Warga B's notification remains unread!
		var afterB models.Notifikasi
		config.DB.First(&afterB, f.notifWargaB1.ID)
		if afterB.IsRead {
			t.Errorf("CROSS-USER CONTAMINATION: Warga B's notification was marked read by Warga A!")
		}
	})

	// F. IDOR on Delete
	t.Run("Warga A cannot delete Warga B notification -> 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/notifikasi/%d", f.notifWargaB1.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for deleting foreign notification, got %d", w.Code)
		}
	})

	// G. Own Delete -> 200 and soft-deletes
	t.Run("Warga A deletes own notification -> 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/notifikasi/%d", f.notifWargaA2.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 for deleting own notification, got %d", w.Code)
		}

		// Subsequent mark read must return 404
		reqCheck := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/notifikasi/%d/read", f.notifWargaA2.ID), nil)
		reqCheck.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
		wCheck := httptest.NewRecorder()
		r.ServeHTTP(wCheck, reqCheck)

		if wCheck.Code != http.StatusNotFound {
			t.Errorf("expected 404 for deleted notification, got %d", wCheck.Code)
		}
	})

	// H. Notification Creation via Report Status Update
	t.Run("Report status update creates notification for owner citizen", func(t *testing.T) {
		// Clean existing notifs
		config.DB.Where("1 = 1").Delete(&models.Notifikasi{})

		body := bytes.NewBufferString(`{"status":"proses","catatan_admin":"Sedang diperbaiki tim"}`)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/laporan/%d/status", f.lapDesaA.ID), body)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var notifs []models.Notifikasi
		config.DB.Where("laporan_id = ?", f.lapDesaA.ID).Find(&notifs)

		if len(notifs) == 0 {
			t.Errorf("expected notification created upon status change")
		}

		for _, n := range notifs {
			if n.UserID == 0 {
				t.Errorf("ZERO-VALUE RECIPIENT: Notification created with UserID 0")
			}
			if n.UserID == f.pemdesA.ID {
				t.Errorf("RECIPIENT LEAK: Updating admin received notification of their own update")
			}
		}
	})
}

// =========================================================================
// 2. STANDALONE NOTIFICATION SECURITY SUITE (Unconditional Zero-DB Runner)
// =========================================================================
func TestBE10_Notifikasi_StandaloneSecurity(t *testing.T) {
	origDB := config.DB
	config.DB = nil
	defer func() { config.DB = origDB }()

	_ = os.Setenv("JWT_SECRET", "be10_standalone_secret_key_123456789")
	secret := []byte("be10_standalone_secret_key_123456789")

	r := gin.New()
	routes.SetupRoutes(r)

	wargaToken, _ := utils.GenerateToken(10, "warga@roadis.local", models.RoleWarga, nil)
	pemdesToken, _ := utils.GenerateToken(20, "pemdes@roadis.local", models.RoleAdminPemdes, nil)
	puToken, _ := utils.GenerateToken(30, "pu@roadis.local", models.RoleAdminPu, nil)
	superToken, _ := utils.GenerateToken(1, "sa@roadis.local", models.RoleSuperAdmin, nil)

	// A. Authentication
	t.Run("Auth_MissingTokenReturns401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/notifikasi", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("Auth_InvalidSignatureReturns401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/notifikasi", nil)
		req.Header.Set("Authorization", "Bearer invalid.jwt.token")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("Auth_ExpiredTokenReturns401", func(t *testing.T) {
		claims := utils.JWTClaim{
			UserID: 10,
			Email:  "warga@roadis.local",
			Role:   models.RoleWarga,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			},
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tStr, _ := tok.SignedString(secret)

		req := httptest.NewRequest(http.MethodGet, "/api/notifikasi", nil)
		req.Header.Set("Authorization", "Bearer "+tStr)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for expired token, got %d", w.Code)
		}
	})

	t.Run("Auth_UnknownRoleReturns403", func(t *testing.T) {
		claims := utils.JWTClaim{
			UserID: 99,
			Email:  "unknown@roadis.local",
			Role:   "anonymous_intruder",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tStr, _ := tok.SignedString(secret)

		req := httptest.NewRequest(http.MethodGet, "/api/notifikasi", nil)
		req.Header.Set("Authorization", "Bearer "+tStr)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for unknown role on notifications, got %d", w.Code)
		}
	})

	// B. ID Validation (Malformed Parameters)
	malformedIDs := []struct {
		idStr string
		desc  string
	}{
		{"abc", "Alphabetic ID"},
		{"-1", "Negative ID"},
		{"0", "Zero ID"},
		{"%201%20", "Whitespace padded ID"},
		{"99999999999999999999999999999999999", "Integer overflow"},
	}

	for _, tc := range malformedIDs {
		t.Run("IDValidation_MarkRead_"+tc.desc, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/api/notifikasi/"+tc.idStr+"/read", nil)
			req.Header.Set("Authorization", "Bearer "+wargaToken)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("[%s] expected 400 Bad Request, got %d: %s", tc.desc, w.Code, w.Body.String())
			}
		})

		t.Run("IDValidation_Delete_"+tc.desc, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/api/notifikasi/"+tc.idStr, nil)
			req.Header.Set("Authorization", "Bearer "+wargaToken)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("[%s] expected 400 Bad Request, got %d: %s", tc.desc, w.Code, w.Body.String())
			}
		})
	}

	// C. Fail-Closed & Nil Pointer Safety on Creation Helpers
	t.Run("FailClosed_NotificationHelpers_NoPanic", func(t *testing.T) {
		// Zero report struct
		controllers.KirimNotifikasiChatWarga(models.LaporanKerusakan{})
		controllers.KirimNotifikasiBalasanAdmin(models.LaporanKerusakan{})
		warga.KirimNotifikasiLaporanBaru(models.LaporanKerusakan{})

		// Zero report ID
		controllers.KirimNotifikasiChatWarga(models.LaporanKerusakan{Model: gorm.Model{ID: 0}})
		controllers.KirimNotifikasiBalasanAdmin(models.LaporanKerusakan{Model: gorm.Model{ID: 0}})
		warga.KirimNotifikasiLaporanBaru(models.LaporanKerusakan{Model: gorm.Model{ID: 0}})

		// Zero user ID on reply
		controllers.KirimNotifikasiBalasanAdmin(models.LaporanKerusakan{Model: gorm.Model{ID: 10}, UserID: 0})

		// Zero wilayah ID on desa report
		controllers.KirimNotifikasiChatWarga(models.LaporanKerusakan{Model: gorm.Model{ID: 10}, JenisJalan: "desa", WilayahID: 0})
		warga.KirimNotifikasiLaporanBaru(models.LaporanKerusakan{Model: gorm.Model{ID: 10}, JenisJalan: "desa", WilayahID: 0})
	})

	// D. Pagination Edge Cases
	t.Run("Pagination_QueryParamHandling", func(t *testing.T) {
		// Negative or malformed pagination queries must not panic and use safe fallbacks
		req := httptest.NewRequest(http.MethodGet, "/api/notifikasi?page=-1&limit=-10", nil)
		req.Header.Set("Authorization", "Bearer "+wargaToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		// Should execute cleanly (200 or 500 depending on DB connectivity, but never panic)
		if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
			t.Errorf("unexpected status %d", w.Code)
		}
	})

	// E. Role Whitelist in Context
	t.Run("AllValidRolesAllowedAuthentication", func(t *testing.T) {
		tokens := []string{wargaToken, pemdesToken, puToken, superToken}
		for _, tok := range tokens {
			req := httptest.NewRequest(http.MethodGet, "/api/notifikasi", nil)
			req.Header.Set("Authorization", "Bearer "+tok)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code == http.StatusForbidden || w.Code == http.StatusUnauthorized {
				t.Errorf("valid role was rejected with %d", w.Code)
			}
		}
	})
}
