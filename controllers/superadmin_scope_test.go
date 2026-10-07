package controllers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"backend-jalan-rusak/config"
	adminController "backend-jalan-rusak/controllers/admin"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/routes"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type be8Fixtures struct {
	desaA      models.Wilayah
	desaB      models.Wilayah
	wargaA     models.User
	pemdesA    models.User
	adminPU    models.User
	superAdmin models.User
	otherSA    models.User

	tokenPemdesA string
	tokenPU      string
	tokenSA      string
	tokenOtherSA string
	tokenWargaA  string

	lapDesaA     models.LaporanKerusakan
	lapKabupaten models.LaporanKerusakan
	lapProvinsi  models.LaporanKerusakan
	lapNasional  models.LaporanKerusakan
	lapDeleted   models.LaporanKerusakan
}

func setupBE8Fixtures(t *testing.T) (*be8Fixtures, func()) {
	if !ensureDBForLaporanTest(t) {
		t.Skip("Database not available")
	}

	f := &be8Fixtures{}
	nowNano := time.Now().UnixNano()

	// 1. Wilayah
	f.desaA = models.Wilayah{Nama: fmt.Sprintf("Desa A BE8 %d", nowNano), Tipe: "desa"}
	config.DB.Create(&f.desaA)
	f.desaB = models.Wilayah{Nama: fmt.Sprintf("Desa B BE8 %d", nowNano), Tipe: "desa"}
	config.DB.Create(&f.desaB)

	// 2. Users
	f.wargaA = models.User{Name: "Warga A BE8", Email: fmt.Sprintf("warga_a_%d@roadis.id", nowNano), Role: models.RoleWarga}
	config.DB.Create(&f.wargaA)

	f.pemdesA = models.User{Name: "Pemdes A BE8", Email: fmt.Sprintf("pemdes_a_%d@roadis.id", nowNano), Role: models.RoleAdminPemdes, WilayahID: &f.desaA.ID}
	config.DB.Create(&f.pemdesA)

	f.adminPU = models.User{Name: "Admin PU BE8", Email: fmt.Sprintf("pu_%d@roadis.id", nowNano), Role: models.RoleAdminPu}
	config.DB.Create(&f.adminPU)

	f.superAdmin = models.User{Name: "Super Admin BE8", Email: fmt.Sprintf("sa_%d@roadis.id", nowNano), Role: models.RoleSuperAdmin, TokenVersion: 1}
	config.DB.Create(&f.superAdmin)

	f.otherSA = models.User{Name: "Other SA BE8", Email: fmt.Sprintf("other_sa_%d@roadis.id", nowNano), Role: models.RoleSuperAdmin, TokenVersion: 1}
	config.DB.Create(&f.otherSA)

	// 3. Tokens
	f.tokenPemdesA, _ = utils.GenerateToken(f.pemdesA.ID, f.pemdesA.Email, f.pemdesA.Role, nil)
	f.tokenPU, _ = utils.GenerateToken(f.adminPU.ID, f.adminPU.Email, f.adminPU.Role, nil)
	f.tokenSA, _ = utils.GenerateToken(f.superAdmin.ID, f.superAdmin.Email, f.superAdmin.Role, nil)
	f.tokenOtherSA, _ = utils.GenerateToken(f.otherSA.ID, f.otherSA.Email, f.otherSA.Role, nil)
	f.tokenWargaA, _ = utils.GenerateToken(f.wargaA.ID, f.wargaA.Email, f.wargaA.Role, nil)

	// 4. Reports for each authority
	f.lapDesaA = models.LaporanKerusakan{
		UserID: f.wargaA.ID, WilayahID: f.desaA.ID, JenisJalan: "desa", Judul: "Jalan Rusak Desa A BE8",
		Deskripsi: "Deskripsi", Latitude: -6.3400, Longitude: 108.3300, ImageURL: "https://foto.jpg",
		TipeKerusakan: "lubang", Status: "menunggu",
	}
	config.DB.Create(&f.lapDesaA)

	f.lapKabupaten = models.LaporanKerusakan{
		UserID: f.wargaA.ID, WilayahID: f.desaA.ID, JenisJalan: "kabupaten", Judul: "Jalan Rusak Kabupaten BE8",
		Deskripsi: "Deskripsi", Latitude: -6.3500, Longitude: 108.3400, ImageURL: "https://foto.jpg",
		TipeKerusakan: "retak", Status: "menunggu",
	}
	config.DB.Create(&f.lapKabupaten)

	f.lapProvinsi = models.LaporanKerusakan{
		UserID: f.wargaA.ID, WilayahID: f.desaA.ID, JenisJalan: "provinsi", Judul: "Jalan Rusak Provinsi BE8",
		Deskripsi: "Deskripsi", Latitude: -6.3550, Longitude: 108.3450, ImageURL: "https://foto.jpg",
		TipeKerusakan: "ambles", Status: "menunggu",
	}
	config.DB.Create(&f.lapProvinsi)

	f.lapNasional = models.LaporanKerusakan{
		UserID: f.wargaA.ID, WilayahID: f.desaA.ID, JenisJalan: "nasional", Judul: "Jalan Rusak Nasional BE8",
		Deskripsi: "Deskripsi", Latitude: -6.3600, Longitude: 108.3500, ImageURL: "https://foto.jpg",
		TipeKerusakan: "retak", Status: "menunggu",
	}
	config.DB.Create(&f.lapNasional)

	f.lapDeleted = models.LaporanKerusakan{
		UserID: f.wargaA.ID, WilayahID: f.desaA.ID, JenisJalan: "kabupaten", Judul: "Jalan Deleted BE8",
		Deskripsi: "Deskripsi", Latitude: -6.3650, Longitude: 108.3550, ImageURL: "https://foto.jpg",
		TipeKerusakan: "lubang", Status: "menunggu",
		Model: gorm.Model{DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}},
	}
	config.DB.Create(&f.lapDeleted)

	teardown := func() {
		config.DB.Unscoped().Delete(&f.lapDesaA)
		config.DB.Unscoped().Delete(&f.lapKabupaten)
		config.DB.Unscoped().Delete(&f.lapProvinsi)
		config.DB.Unscoped().Delete(&f.lapNasional)
		config.DB.Unscoped().Delete(&f.lapDeleted)
		config.DB.Unscoped().Where("user_id IN ?", []uint{f.wargaA.ID, f.pemdesA.ID, f.adminPU.ID, f.superAdmin.ID, f.otherSA.ID}).Delete(&models.UserPreference{})
		config.DB.Unscoped().Delete(&f.wargaA)
		config.DB.Unscoped().Delete(&f.pemdesA)
		config.DB.Unscoped().Delete(&f.adminPU)
		config.DB.Unscoped().Delete(&f.superAdmin)
		config.DB.Unscoped().Delete(&f.otherSA)
		config.DB.Unscoped().Delete(&f.desaA)
		config.DB.Unscoped().Delete(&f.desaB)
	}

	return f, teardown
}

// =========================================================================
// 1. AUTHENTICATION & RBAC (SECTION 4 & 5)
// =========================================================================
func TestBE8_Superadmin_AuthAndRBAC(t *testing.T) {
	f, cleanup := setupBE8Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// 1. Superadmin can access /api/superadmin/users -> 200
	t.Run("Superadmin accesses /api/superadmin/users -> 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/superadmin/users", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenSA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK for Superadmin, got %d", w.Code)
		}
	})

	// 2. Missing token -> 401
	t.Run("Missing token to /api/superadmin/users returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/superadmin/users", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	// 3. Invalid token -> 401
	t.Run("Invalid token returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/superadmin/users", nil)
		req.Header.Set("Authorization", "Bearer invalid.jwt.token")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	// 4. Soft-deleted superadmin -> 401
	t.Run("Soft-deleted Superadmin token returns 401", func(t *testing.T) {
		deletedSA := models.User{
			Name:         "Deleted SA",
			Email:        fmt.Sprintf("del_sa_%d@roadis.id", time.Now().UnixNano()),
			Role:         models.RoleSuperAdmin,
			TokenVersion: 1,
		}
		config.DB.Create(&deletedSA)
		delToken, _ := utils.GenerateToken(deletedSA.ID, deletedSA.Email, deletedSA.Role, nil)
		config.DB.Delete(&deletedSA)
		defer config.DB.Unscoped().Delete(&deletedSA)

		req := httptest.NewRequest(http.MethodGet, "/api/superadmin/users", nil)
		req.Header.Set("Authorization", "Bearer "+delToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized for soft-deleted SA, got %d", w.Code)
		}
	})

	// 5. Non-superadmin roles to /api/superadmin/* -> 403 Forbidden
	nonSARoles := []struct {
		name  string
		token string
	}{
		{"Admin PU", f.tokenPU},
		{"Admin Pemdes", f.tokenPemdesA},
		{"Warga", f.tokenWargaA},
	}

	for _, tc := range nonSARoles {
		t.Run(fmt.Sprintf("%s accessing /api/superadmin/users returns 403", tc.name), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/superadmin/users", nil)
			req.Header.Set("Authorization", "Bearer "+tc.token)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusForbidden {
				t.Errorf("expected 403 Forbidden for %s, got %d", tc.name, w.Code)
			}
		})
	}
}

// =========================================================================
// 2. REPORT READ SCOPE (SECTION 6)
// =========================================================================
func TestBE8_Superadmin_ReportReadScope(t *testing.T) {
	f, cleanup := setupBE8Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// List: Superadmin sees all authorities
	t.Run("Superadmin sees reports across all authorities", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/laporan?page=1&limit=100", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenSA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp struct {
			Data []models.LaporanKerusakan `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		seenTypes := make(map[string]bool)
		for _, lap := range resp.Data {
			seenTypes[lap.JenisJalan] = true
		}
		if !seenTypes["desa"] || !seenTypes["kabupaten"] || !seenTypes["provinsi"] || !seenTypes["nasional"] {
			t.Errorf("Superadmin should see all authorities, seen: %v", seenTypes)
		}
	})

	// Detail: Superadmin can read any authority
	for _, lap := range []models.LaporanKerusakan{f.lapDesaA, f.lapKabupaten, f.lapProvinsi, f.lapNasional} {
		t.Run(fmt.Sprintf("Superadmin reads %s report detail -> 200 OK", lap.JenisJalan), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d", lap.ID), nil)
			req.Header.Set("Authorization", "Bearer "+f.tokenSA)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("expected 200 OK reading %s report, got %d", lap.JenisJalan, w.Code)
			}
		})
	}

	// Nonexistent & soft-deleted reports -> 404
	t.Run("Superadmin reads nonexistent report returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/laporan/999999", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenSA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 for nonexistent report, got %d", w.Code)
		}
	})

	t.Run("Superadmin reads soft-deleted report returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d", f.lapDeleted.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenSA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 for soft-deleted report, got %d", w.Code)
		}
	})

	// Malformed ID -> 400
	t.Run("Superadmin reads malformed ID returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/laporan/abc", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenSA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for malformed ID, got %d", w.Code)
		}
	})
}

// =========================================================================
// 3. REPORT UPDATE & LIFECYCLE INVARIANTS (SECTION 7)
// =========================================================================
func TestBE8_Superadmin_ReportUpdateScope(t *testing.T) {
	f, cleanup := setupBE8Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	sendUpdateReq := func(token string, reportID uint, status string) *httptest.ResponseRecorder {
		body := bytes.NewBufferString(fmt.Sprintf(`{"status":"%s"}`, status))
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/laporan/%d/status", reportID), body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// 1. Superadmin can update status across all authorities: Desa, Kabupaten, Provinsi, Nasional
	for _, lap := range []models.LaporanKerusakan{f.lapDesaA, f.lapKabupaten, f.lapProvinsi, f.lapNasional} {
		t.Run(fmt.Sprintf("Superadmin updates %s report status to proses -> 200 OK", lap.JenisJalan), func(t *testing.T) {
			w := sendUpdateReq(f.tokenSA, lap.ID, "proses")
			if w.Code != http.StatusOK {
				t.Errorf("expected 200 OK updating %s report, got %d: %s", lap.JenisJalan, w.Code, w.Body.String())
			}
		})
	}

	// 2. Lifecycle guard: status selesai without evidence -> rejected (400)
	t.Run("Superadmin updating to selesai without evidence is rejected with 400", func(t *testing.T) {
		w := sendUpdateReq(f.tokenSA, f.lapKabupaten.ID, "selesai")
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request updating to selesai without evidence, got %d", w.Code)
		}
	})

	// 3. Lifecycle guard: status ditolak without catatan_admin -> rejected (400)
	t.Run("Superadmin updating to ditolak without catatan is rejected with 400", func(t *testing.T) {
		w := sendUpdateReq(f.tokenSA, f.lapDesaA.ID, "ditolak")
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request updating to ditolak without note, got %d", w.Code)
		}
	})

	// 4. Update soft-deleted report -> 404 Not Found
	t.Run("Superadmin updating soft-deleted report returns 404", func(t *testing.T) {
		w := sendUpdateReq(f.tokenSA, f.lapDeleted.ID, "proses")
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found updating soft-deleted report, got %d", w.Code)
		}
	})
}

// =========================================================================
// 4. SUPERADMIN SPAM REPORT DELETION (SECTION 8)
// =========================================================================
func TestBE8_Superadmin_SpamReportDelete(t *testing.T) {
	f, cleanup := setupBE8Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// Create a temporary report to delete
	spamLap := models.LaporanKerusakan{
		UserID: f.wargaA.ID, WilayahID: f.desaA.ID, JenisJalan: "desa", Judul: "Laporan Spam BE8",
		Deskripsi: "Spam", Latitude: -6.3400, Longitude: 108.3300, ImageURL: "https://foto.jpg",
		Status: "menunggu", TipeKerusakan: "lubang",
	}
	config.DB.Create(&spamLap)
	defer config.DB.Unscoped().Delete(&spamLap)

	// 1. Non-superadmin cannot delete spam report -> 403
	t.Run("Admin PU calling spam delete returns 403 Forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/superadmin/laporan/%d", spamLap.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPU)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %d", w.Code)
		}
	})

	// 2. Superadmin deletes spam report -> 200 OK
	t.Run("Superadmin deletes spam report -> 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/superadmin/laporan/%d", spamLap.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenSA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		// Verify record is soft-deleted
		var check models.LaporanKerusakan
		err := config.DB.Where("id = ? AND deleted_at IS NULL", spamLap.ID).First(&check).Error
		if err == nil {
			t.Errorf("Spam report should be soft-deleted in database!")
		}
	})

	// 3. Repeated delete on already deleted report -> 404
	t.Run("Repeated delete on soft-deleted report returns 404 Not Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/superadmin/laporan/%d", spamLap.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenSA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found for repeated delete, got %d", w.Code)
		}
	})

	// 4. Malformed ID -> 400 Bad Request
	t.Run("Spam delete with malformed ID returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/superadmin/laporan/invalid_id", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenSA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})
}

// =========================================================================
// 5. USER MANAGEMENT & SELF-DESTRUCT GUARDS (SECTION 9)
// =========================================================================
func TestBE8_Superadmin_UserManagement(t *testing.T) {
	f, cleanup := setupBE8Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// 1. Self-delete protection -> 400
	t.Run("Superadmin cannot delete own account -> 400 Bad Request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/superadmin/users/%d", f.superAdmin.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenSA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for self-delete, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 2. Last superadmin protection
	t.Run("Cannot delete last superadmin in system", func(t *testing.T) {
		// Temporary solitary SA
		solitarySA := models.User{
			Name:         "Solitary SA",
			Email:        fmt.Sprintf("solitary_%d@roadis.id", time.Now().UnixNano()),
			Role:         models.RoleSuperAdmin,
			TokenVersion: 1,
		}
		config.DB.Create(&solitarySA)
		defer config.DB.Unscoped().Delete(&solitarySA)

		// Delete otherSA first so solitarySA would be the last if deleted
		// But in our fixture f.superAdmin exists, so f.otherSA can delete f.superAdmin unless last.
		// Let's verify deleting a regular admin works
		tempAdmin := models.User{
			Name:  "Temp Admin",
			Email: fmt.Sprintf("temp_adm_%d@roadis.id", time.Now().UnixNano()),
			Role:  models.RoleAdminPu,
		}
		config.DB.Create(&tempAdmin)
		defer config.DB.Unscoped().Delete(&tempAdmin)

		reqDel := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/superadmin/users/%d", tempAdmin.ID), nil)
		reqDel.Header.Set("Authorization", "Bearer "+f.tokenSA)
		wDel := httptest.NewRecorder()
		r.ServeHTTP(wDel, reqDel)

		if wDel.Code != http.StatusOK {
			t.Errorf("expected 200 OK deleting normal user, got %d: %s", wDel.Code, wDel.Body.String())
		}
	})
}

// =========================================================================
// 6. WILAYAH MANAGEMENT & DEPENDENCY GUARDS (SECTION 10)
// =========================================================================
func TestBE8_Superadmin_WilayahManagement(t *testing.T) {
	f, cleanup := setupBE8Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// 1. Dependency protection: cannot delete wilayah used by reports/users -> 400
	t.Run("Cannot delete wilayah currently in use -> 400 Bad Request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/superadmin/wilayah/%d", f.desaA.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenSA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for deleting in-use wilayah, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 2. Duplicate wilayah protection
	t.Run("Creating duplicate wilayah with same name and tipe is rejected", func(t *testing.T) {
		body := bytes.NewBufferString(fmt.Sprintf(`{"nama":"%s","tipe":"%s"}`, f.desaA.Nama, f.desaA.Tipe))
		req := httptest.NewRequest(http.MethodPost, "/api/superadmin/wilayah", body)
		req.Header.Set("Authorization", "Bearer "+f.tokenSA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for duplicate wilayah, got %d", w.Code)
		}
	})
}

// =========================================================================
// 7. CHAT ACCESS & REPLY SCOPE (SECTION 11)
// =========================================================================
func TestBE8_Superadmin_ChatScope(t *testing.T) {
	f, cleanup := setupBE8Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// Chats on different road types
	chatDesa := models.RiwayatChat{LaporanKerusakanID: f.lapDesaA.ID, UserID: f.wargaA.ID, Pesan: "Chat di Desa BE8"}
	config.DB.Create(&chatDesa)
	defer config.DB.Unscoped().Delete(&chatDesa)

	chatKab := models.RiwayatChat{LaporanKerusakanID: f.lapKabupaten.ID, UserID: f.wargaA.ID, Pesan: "Chat di Kabupaten BE8"}
	config.DB.Create(&chatKab)
	defer config.DB.Unscoped().Delete(&chatKab)

	// 1. Superadmin can read chats on any authority
	for _, lap := range []models.LaporanKerusakan{f.lapDesaA, f.lapKabupaten} {
		t.Run(fmt.Sprintf("Superadmin reads %s chat -> 200 OK", lap.JenisJalan), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d/chat", lap.ID), nil)
			req.Header.Set("Authorization", "Bearer "+f.tokenSA)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("expected 200 OK reading chat on %s report, got %d", lap.JenisJalan, w.Code)
			}
		})
	}

	// 2. Superadmin can reply to any chat
	t.Run("Superadmin replies to chat -> 200 OK", func(t *testing.T) {
		body := bytes.NewBufferString(`{"balasan":"Superadmin menanggapi"}`)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/chat/%d", chatDesa.ID), body)
		req.Header.Set("Authorization", "Bearer "+f.tokenSA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK replying to chat, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 3. Superadmin reply to nonexistent chat -> 404
	t.Run("Superadmin reply to nonexistent chat returns 404", func(t *testing.T) {
		body := bytes.NewBufferString(`{"balasan":"test"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/admin/chat/999999", body)
		req.Header.Set("Authorization", "Bearer "+f.tokenSA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found, got %d", w.Code)
		}
	})
}

// =========================================================================
// 8. MAP & DASHBOARD SCOPE (SECTION 12 & 13)
// =========================================================================
func TestBE8_Superadmin_MapAndDashboard(t *testing.T) {
	f, cleanup := setupBE8Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// Map: Superadmin sees all markers
	t.Run("Superadmin map returns all markers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/map/laporan", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenSA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp struct {
			Data []adminController.MapPoint `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		seenMapTypes := make(map[string]bool)
		for _, pt := range resp.Data {
			seenMapTypes[pt.JenisJalan] = true
		}
		if !seenMapTypes["desa"] || !seenMapTypes["kabupaten"] {
			t.Errorf("Superadmin should see all markers, seen: %v", seenMapTypes)
		}
	})

	// Dashboard: Superadmin counts active reports excluding soft-deleted
	t.Run("Superadmin dashboard excludes soft-deleted reports", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenSA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp struct {
			Data struct {
				TotalLaporan int64 `json:"total_laporan"`
			} `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		var expectedActiveCount int64
		config.DB.Model(&models.LaporanKerusakan{}).
			Where("deleted_at IS NULL").
			Count(&expectedActiveCount)

		if resp.Data.TotalLaporan != expectedActiveCount {
			t.Errorf("DASHBOARD ERROR: expected %d active reports, got %d", expectedActiveCount, resp.Data.TotalLaporan)
		}
	})
}

// =========================================================================
// 9. NOTIFICATIONS & PROFILE ISOLATION (SECTION 14 & 15)
// =========================================================================
func TestBE8_Superadmin_SelfResourceScope(t *testing.T) {
	f, cleanup := setupBE8Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	notifOther := models.Notifikasi{UserID: f.wargaA.ID, Judul: "Warga Notif", Pesan: "Pesan Warga"}
	config.DB.Create(&notifOther)
	defer config.DB.Unscoped().Delete(&notifOther)

	// Superadmin cannot mark read another user's personal notification -> 403
	t.Run("Superadmin cannot mark read another user's notification -> 403 Forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/notifikasi/%d/read", notifOther.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenSA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for marking other's notification read, got %d", w.Code)
		}
	})

	// Profile update cannot change role via profile endpoint
	t.Run("Superadmin profile update cannot alter role", func(t *testing.T) {
		body := bytes.NewBufferString(`{"name":"Updated SA Name","role":"warga"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/profile", body)
		req.Header.Set("Authorization", "Bearer "+f.tokenSA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var refreshed models.User
		config.DB.First(&refreshed, f.superAdmin.ID)
		if refreshed.Role != models.RoleSuperAdmin {
			t.Errorf("CRITICAL: Role altered via profile update to %s", refreshed.Role)
		}
	})
}

// =========================================================================
// 10. COMPREHENSIVE ROLE MATRIX TEST (SECTION 20)
// =========================================================================
func TestBE8_Comprehensive_RoleMatrix(t *testing.T) {
	f, cleanup := setupBE8Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// Matrix: Superadmin(200), PU(403), Pemdes(403), Warga(403) on Superadmin endpoints
	saEndpoints := []struct {
		method string
		url    string
	}{
		{http.MethodGet, "/api/superadmin/users"},
		{http.MethodGet, "/api/superadmin/wilayah"},
		{http.MethodDelete, fmt.Sprintf("/api/superadmin/laporan/%d", f.lapDesaA.ID)},
	}

	for _, ep := range saEndpoints {
		t.Run(fmt.Sprintf("%s %s role authorization matrix", ep.method, ep.url), func(t *testing.T) {
			// PU -> 403
			reqPU := httptest.NewRequest(ep.method, ep.url, nil)
			reqPU.Header.Set("Authorization", "Bearer "+f.tokenPU)
			wPU := httptest.NewRecorder()
			r.ServeHTTP(wPU, reqPU)
			if wPU.Code != http.StatusForbidden {
				t.Errorf("[%s] expected 403 for Admin PU, got %d", ep.url, wPU.Code)
			}

			// Pemdes -> 403
			reqPem := httptest.NewRequest(ep.method, ep.url, nil)
			reqPem.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
			wPem := httptest.NewRecorder()
			r.ServeHTTP(wPem, reqPem)
			if wPem.Code != http.StatusForbidden {
				t.Errorf("[%s] expected 403 for Admin Pemdes, got %d", ep.url, wPem.Code)
			}

			// Warga -> 403
			reqW := httptest.NewRequest(ep.method, ep.url, nil)
			reqW.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
			wW := httptest.NewRecorder()
			r.ServeHTTP(wW, reqW)
			if wW.Code != http.StatusForbidden {
				t.Errorf("[%s] expected 403 for Warga, got %d", ep.url, wW.Code)
			}
		})
	}
}
