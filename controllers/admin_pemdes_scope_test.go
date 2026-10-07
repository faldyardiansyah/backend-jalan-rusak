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

// Fixture setup helper for BE-6 Admin Pemdes Scope Tests
type be6Fixtures struct {
	desaA      models.Wilayah
	desaB      models.Wilayah
	wargaA     models.User
	wargaB     models.User
	pemdesA    models.User
	pemdesB    models.User
	adminPU    models.User
	superAdmin models.User

	tokenPemdesA string
	tokenPemdesB string
	tokenPU      string
	tokenSA      string
	tokenWargaA  string

	lapDesaA      models.LaporanKerusakan
	lapDesaB      models.LaporanKerusakan
	lapKabupaten  models.LaporanKerusakan
	lapProvinsi   models.LaporanKerusakan
	lapNasional   models.LaporanKerusakan
	lapDeletedA   models.LaporanKerusakan
}

func setupBE6Fixtures(t *testing.T) (*be6Fixtures, func()) {
	if !ensureDBForLaporanTest(t) {
		t.Skip("Database not available")
	}

	f := &be6Fixtures{}
	nowNano := time.Now().UnixNano()

	// 1. Wilayah
	f.desaA = models.Wilayah{Nama: fmt.Sprintf("Desa A BE6 %d", nowNano), Tipe: "desa"}
	config.DB.Create(&f.desaA)
	f.desaB = models.Wilayah{Nama: fmt.Sprintf("Desa B BE6 %d", nowNano), Tipe: "desa"}
	config.DB.Create(&f.desaB)

	// 2. Users
	f.wargaA = models.User{Name: "Warga A BE6", Email: fmt.Sprintf("warga_a_%d@roadis.id", nowNano), Role: models.RoleWarga}
	config.DB.Create(&f.wargaA)
	f.wargaB = models.User{Name: "Warga B BE6", Email: fmt.Sprintf("warga_b_%d@roadis.id", nowNano), Role: models.RoleWarga}
	config.DB.Create(&f.wargaB)

	f.pemdesA = models.User{Name: "Pemdes A BE6", Email: fmt.Sprintf("pemdes_a_%d@roadis.id", nowNano), Role: models.RoleAdminPemdes, WilayahID: &f.desaA.ID}
	config.DB.Create(&f.pemdesA)
	f.pemdesB = models.User{Name: "Pemdes B BE6", Email: fmt.Sprintf("pemdes_b_%d@roadis.id", nowNano), Role: models.RoleAdminPemdes, WilayahID: &f.desaB.ID}
	config.DB.Create(&f.pemdesB)

	f.adminPU = models.User{Name: "Admin PU BE6", Email: fmt.Sprintf("pu_%d@roadis.id", nowNano), Role: models.RoleAdminPu}
	config.DB.Create(&f.adminPU)
	f.superAdmin = models.User{Name: "Super Admin BE6", Email: fmt.Sprintf("sa_%d@roadis.id", nowNano), Role: models.RoleSuperAdmin}
	config.DB.Create(&f.superAdmin)

	// 3. Tokens
	f.tokenPemdesA, _ = utils.GenerateToken(f.pemdesA.ID, f.pemdesA.Email, f.pemdesA.Role, nil)
	f.tokenPemdesB, _ = utils.GenerateToken(f.pemdesB.ID, f.pemdesB.Email, f.pemdesB.Role, nil)
	f.tokenPU, _ = utils.GenerateToken(f.adminPU.ID, f.adminPU.Email, f.adminPU.Role, nil)
	f.tokenSA, _ = utils.GenerateToken(f.superAdmin.ID, f.superAdmin.Email, f.superAdmin.Role, nil)
	f.tokenWargaA, _ = utils.GenerateToken(f.wargaA.ID, f.wargaA.Email, f.wargaA.Role, nil)

	// 4. Reports
	f.lapDesaA = models.LaporanKerusakan{
		UserID: f.wargaA.ID, WilayahID: f.desaA.ID, JenisJalan: "desa", Judul: "Jalan Rusak Desa A",
		Deskripsi: "Deskripsi", Latitude: -6.3400, Longitude: 108.3300, ImageURL: "https://foto.jpg",
		TipeKerusakan: "lubang", Status: "menunggu",
	}
	config.DB.Create(&f.lapDesaA)

	f.lapDesaB = models.LaporanKerusakan{
		UserID: f.wargaB.ID, WilayahID: f.desaB.ID, JenisJalan: "desa", Judul: "Jalan Rusak Desa B",
		Deskripsi: "Deskripsi", Latitude: -6.3450, Longitude: 108.3350, ImageURL: "https://foto.jpg",
		TipeKerusakan: "lubang", Status: "menunggu",
	}
	config.DB.Create(&f.lapDesaB)

	f.lapKabupaten = models.LaporanKerusakan{
		UserID: f.wargaA.ID, WilayahID: f.desaA.ID, JenisJalan: "kabupaten", Judul: "Jalan Rusak Kabupaten di Desa A",
		Deskripsi: "Deskripsi", Latitude: -6.3500, Longitude: 108.3400, ImageURL: "https://foto.jpg",
		TipeKerusakan: "retak", Status: "menunggu",
	}
	config.DB.Create(&f.lapKabupaten)

	f.lapProvinsi = models.LaporanKerusakan{
		UserID: f.wargaA.ID, WilayahID: f.desaA.ID, JenisJalan: "provinsi", Judul: "Jalan Rusak Provinsi di Desa A",
		Deskripsi: "Deskripsi", Latitude: -6.3550, Longitude: 108.3450, ImageURL: "https://foto.jpg",
		TipeKerusakan: "ambles", Status: "menunggu",
	}
	config.DB.Create(&f.lapProvinsi)

	f.lapNasional = models.LaporanKerusakan{
		UserID: f.wargaA.ID, WilayahID: f.desaA.ID, JenisJalan: "nasional", Judul: "Jalan Rusak Nasional di Desa A",
		Deskripsi: "Deskripsi", Latitude: -6.3600, Longitude: 108.3500, ImageURL: "https://foto.jpg",
		TipeKerusakan: "retak", Status: "menunggu",
	}
	config.DB.Create(&f.lapNasional)

	f.lapDeletedA = models.LaporanKerusakan{
		UserID: f.wargaA.ID, WilayahID: f.desaA.ID, JenisJalan: "desa", Judul: "Jalan Deleted Desa A",
		Deskripsi: "Deskripsi", Latitude: -6.3650, Longitude: 108.3550, ImageURL: "https://foto.jpg",
		TipeKerusakan: "lubang", Status: "menunggu",
		Model: gorm.Model{DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}},
	}
	config.DB.Create(&f.lapDeletedA)

	teardown := func() {
		config.DB.Unscoped().Delete(&f.lapDesaA)
		config.DB.Unscoped().Delete(&f.lapDesaB)
		config.DB.Unscoped().Delete(&f.lapKabupaten)
		config.DB.Unscoped().Delete(&f.lapProvinsi)
		config.DB.Unscoped().Delete(&f.lapNasional)
		config.DB.Unscoped().Delete(&f.lapDeletedA)
		config.DB.Unscoped().Where("user_id IN ?", []uint{f.wargaA.ID, f.wargaB.ID, f.pemdesA.ID, f.pemdesB.ID, f.adminPU.ID, f.superAdmin.ID}).Delete(&models.UserPreference{})
		config.DB.Unscoped().Delete(&f.wargaA)
		config.DB.Unscoped().Delete(&f.wargaB)
		config.DB.Unscoped().Delete(&f.pemdesA)
		config.DB.Unscoped().Delete(&f.pemdesB)
		config.DB.Unscoped().Delete(&f.adminPU)
		config.DB.Unscoped().Delete(&f.superAdmin)
		config.DB.Unscoped().Delete(&f.desaA)
		config.DB.Unscoped().Delete(&f.desaB)
	}

	return f, teardown
}

// =========================================================================
// A. AUTHENTICATION & ROLE ACCESS
// =========================================================================
func TestBE6_AdminPemdes_AuthAndRole(t *testing.T) {
	f, cleanup := setupBE6Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// 1. Unauthenticated -> 401
	t.Run("Unauthenticated request to /api/admin/laporan returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/laporan", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	// 2. Warga access to /api/admin/* -> 403
	t.Run("Warga access to /api/admin/laporan returns 403 Forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/laporan", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})

	// 3. Admin Pemdes access to /api/superadmin/* -> 403
	t.Run("Admin Pemdes access to /api/superadmin/users returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/superadmin/users", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})

	// 4. Admin Pemdes without assigned wilayah -> 403 Forbidden (fail-closed)
	t.Run("Admin Pemdes without WilayahID fails closed with 403", func(t *testing.T) {
		unassignedAdmin := models.User{
			Name:  "Unassigned Admin",
			Email: fmt.Sprintf("unassigned_%d@roadis.id", time.Now().UnixNano()),
			Role:  models.RoleAdminPemdes,
		}
		config.DB.Create(&unassignedAdmin)
		defer config.DB.Unscoped().Delete(&unassignedAdmin)

		tokenUnassigned, _ := utils.GenerateToken(unassignedAdmin.ID, unassignedAdmin.Email, unassignedAdmin.Role, nil)

		endpoints := []string{"/api/admin/laporan", "/api/admin/dashboard", "/api/admin/map/laporan", "/api/admin/chat"}
		for _, ep := range endpoints {
			req := httptest.NewRequest(http.MethodGet, ep, nil)
			req.Header.Set("Authorization", "Bearer "+tokenUnassigned)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusForbidden {
				t.Errorf("[%s] expected 403 Forbidden for unassigned Pemdes admin, got %d", ep, w.Code)
			}
		}
	})
}

// =========================================================================
// B. REPORT LIST & SCOPE ISOLATION (AREA 2, 3, 12, 14)
// =========================================================================
func TestBE6_AdminPemdes_ReportListScope(t *testing.T) {
	f, cleanup := setupBE6Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// Admin Pemdes Desa A calls GET /api/admin/laporan
	t.Run("Admin Pemdes Desa A sees only Desa A reports with jenis_jalan desa", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/laporan?page=1&limit=50", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp struct {
			Data []models.LaporanKerusakan `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		// Assert all returned reports strictly belong to Desa A and are desa roads
		for _, lap := range resp.Data {
			if lap.WilayahID != f.desaA.ID {
				t.Errorf("SCOPE BREACH: Found report with WilayahID=%d in Pemdes Desa A (%d) list", lap.WilayahID, f.desaA.ID)
			}
			if lap.JenisJalan != "desa" {
				t.Errorf("SCOPE BREACH: Found report with jenis_jalan='%s' in Pemdes list", lap.JenisJalan)
			}
		}
	})

	// Parameter tampering attempt: Admin Pemdes Desa A tries ?wilayah_id=DesaB
	t.Run("Tampering attempt with ?wilayah_id=DesaB is strictly ignored by backend", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan?wilayah_id=%d", f.desaB.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp struct {
			Data []models.LaporanKerusakan `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		for _, lap := range resp.Data {
			if lap.WilayahID == f.desaB.ID {
				t.Errorf("SECURITY BYPASS: Admin Pemdes A accessed Desa B via ?wilayah_id query parameter tampering!")
			}
		}
	})

	// Parameter tampering attempt: Admin Pemdes Desa A tries ?jenis_jalan=kabupaten
	t.Run("Tampering attempt with ?jenis_jalan=kabupaten is strictly ignored by backend", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/laporan?jenis_jalan=kabupaten", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp struct {
			Data []models.LaporanKerusakan `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		for _, lap := range resp.Data {
			if lap.JenisJalan != "desa" {
				t.Errorf("SECURITY BYPASS: Admin Pemdes A accessed non-desa road '%s' via ?jenis_jalan tampering!", lap.JenisJalan)
			}
		}
	})
}

// =========================================================================
// C. REPORT DETAIL / IDOR PROTECTION (AREA 4)
// =========================================================================
func TestBE6_AdminPemdes_ReportDetailIDOR(t *testing.T) {
	f, cleanup := setupBE6Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// 1. Own report in Desa A -> 200 OK
	t.Run("Admin Pemdes Desa A reads own Desa report -> 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d", f.lapDesaA.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
	})

	// 2. Report in Desa B -> 404 Not Found (IDOR protected, hides foreign existence)
	t.Run("Admin Pemdes Desa A reads Desa B report -> 404 Not Found (IDOR Guard)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d", f.lapDesaB.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found for foreign desa, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 3. Kabupaten road report in Desa A -> 404 Not Found
	t.Run("Admin Pemdes Desa A reads Kabupaten road in Desa A -> 404 Not Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d", f.lapKabupaten.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found for non-desa road, got %d", w.Code)
		}
	})

	// 4. Provinsi and Nasional road reports in Desa A -> 404 Not Found
	t.Run("Admin Pemdes Desa A reads Provinsi / Nasional roads -> 404 Not Found", func(t *testing.T) {
		for _, lapID := range []uint{f.lapProvinsi.ID, f.lapNasional.ID} {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d", lapID), nil)
			req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				t.Errorf("expected 404 Not Found for report %d, got %d", lapID, w.Code)
			}
		}
	})

	// 5. Soft-deleted report -> 404 Not Found
	t.Run("Admin Pemdes Desa A reads soft-deleted report -> 404 Not Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d", f.lapDeletedA.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found for soft-deleted report, got %d", w.Code)
		}
	})
}

// =========================================================================
// D. UPDATE STATUS SCOPE (AREA 5)
// =========================================================================
func TestBE6_AdminPemdes_UpdateStatusScope(t *testing.T) {
	f, cleanup := setupBE6Fixtures(t)
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

	// 1. Own Desa report -> 200 OK
	t.Run("Admin Pemdes Desa A updates own Desa report -> 200 OK", func(t *testing.T) {
		w := sendUpdateReq(f.tokenPemdesA, f.lapDesaA.ID, "proses")
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK for own desa update, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 2. Foreign Desa B report -> 403 Forbidden
	t.Run("Admin Pemdes Desa A updates foreign Desa B report -> 403 Forbidden", func(t *testing.T) {
		w := sendUpdateReq(f.tokenPemdesA, f.lapDesaB.ID, "proses")
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for cross-desa update, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 3. Kabupaten road report in Desa A -> 403 Forbidden
	t.Run("Admin Pemdes Desa A updates Kabupaten road report -> 403 Forbidden", func(t *testing.T) {
		w := sendUpdateReq(f.tokenPemdesA, f.lapKabupaten.ID, "proses")
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for kabupaten road update by pemdes, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 4. Provinsi & Nasional road reports in Desa A -> 403 Forbidden
	t.Run("Admin Pemdes Desa A updates Provinsi & Nasional road reports -> 403 Forbidden", func(t *testing.T) {
		for _, repID := range []uint{f.lapProvinsi.ID, f.lapNasional.ID} {
			w := sendUpdateReq(f.tokenPemdesA, repID, "proses")
			if w.Code != http.StatusForbidden {
				t.Errorf("expected 403 Forbidden for non-desa road report %d update, got %d", repID, w.Code)
			}
		}
	})
}

// =========================================================================
// E. MAP SCOPE (AREA 6)
// =========================================================================
func TestBE6_AdminPemdes_MapScope(t *testing.T) {
	f, cleanup := setupBE6Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/map/laporan", nil)
	req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data []adminController.MapPoint `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	for _, pt := range resp.Data {
		if pt.WilayahID != f.desaA.ID {
			t.Errorf("MAP SCOPE BREACH: Point with WilayahID=%d appeared on Pemdes Desa A map", pt.WilayahID)
		}
		if pt.JenisJalan != "desa" {
			t.Errorf("MAP SCOPE BREACH: Point with jenis_jalan='%s' appeared on Pemdes Desa A map", pt.JenisJalan)
		}
	}
}

// =========================================================================
// F. DASHBOARD SCOPE (AREA 7)
// =========================================================================
func TestBE6_AdminPemdes_DashboardScope(t *testing.T) {
	f, cleanup := setupBE6Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			TotalLaporan  int64 `json:"total_laporan"`
			TotalMenunggu int64 `json:"total_menunggu"`
			TotalProses   int64 `json:"total_proses"`
			TotalSelesai  int64 `json:"total_selesai"`
			TotalDitolak  int64 `json:"total_ditolak"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	// In our fixture, Desa A only has 1 active desa road report (lapDesaA).
	// lapKabupaten, lapProvinsi, lapNasional, lapDesaB, and lapDeletedA MUST NOT be included!
	var expectedDesaACount int64
	config.DB.Model(&models.LaporanKerusakan{}).
		Where("wilayah_id = ? AND jenis_jalan = ? AND deleted_at IS NULL", f.desaA.ID, "desa").
		Count(&expectedDesaACount)

	if resp.Data.TotalLaporan != expectedDesaACount {
		t.Errorf("DASHBOARD SCOPE ERROR: expected total_laporan=%d, got %d", expectedDesaACount, resp.Data.TotalLaporan)
	}
}

// =========================================================================
// G. CHAT SCOPE (AREA 8)
// =========================================================================
func TestBE6_AdminPemdes_ChatScope(t *testing.T) {
	f, cleanup := setupBE6Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// Create chats on reports
	chatDesaA := models.RiwayatChat{LaporanKerusakanID: f.lapDesaA.ID, UserID: f.wargaA.ID, Pesan: "Chat di Desa A"}
	config.DB.Create(&chatDesaA)
	defer config.DB.Unscoped().Delete(&chatDesaA)

	chatDesaB := models.RiwayatChat{LaporanKerusakanID: f.lapDesaB.ID, UserID: f.wargaB.ID, Pesan: "Chat di Desa B"}
	config.DB.Create(&chatDesaB)
	defer config.DB.Unscoped().Delete(&chatDesaB)

	chatKabupaten := models.RiwayatChat{LaporanKerusakanID: f.lapKabupaten.ID, UserID: f.wargaA.ID, Pesan: "Chat di Kabupaten"}
	config.DB.Create(&chatKabupaten)
	defer config.DB.Unscoped().Delete(&chatKabupaten)

	// 1. GetAdminInbox for Pemdes Desa A: only includes Desa A chat
	t.Run("Admin Pemdes Desa A inbox contains only Desa A chats", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/chat", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp struct {
			Data []struct {
				LaporanID  uint   `json:"laporan_id"`
				JenisJalan string `json:"jenis_jalan"`
				WilayahID  uint   `json:"wilayah_id"`
			} `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		for _, item := range resp.Data {
			if item.WilayahID != f.desaA.ID {
				t.Errorf("CHAT INBOX SCOPE BREACH: Found chat from WilayahID=%d in Pemdes Desa A inbox", item.WilayahID)
			}
			if item.JenisJalan != "desa" {
				t.Errorf("CHAT INBOX SCOPE BREACH: Found non-desa road '%s' chat in Pemdes inbox", item.JenisJalan)
			}
		}
	})

	// 2. Read chat on own report -> 200 OK
	t.Run("Admin Pemdes Desa A reads chat on own Desa A report -> 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d/chat", f.lapDesaA.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	// 3. Read chat on foreign report -> 403 Forbidden
	t.Run("Admin Pemdes Desa A reads chat on foreign Desa B report -> 403 Forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d/chat", f.lapDesaB.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for foreign chat access, got %d", w.Code)
		}
	})

	// 4. Reply chat on foreign report -> 403 Forbidden
	t.Run("Admin Pemdes Desa A replies to foreign Desa B chat -> 403 Forbidden", func(t *testing.T) {
		body := bytes.NewBufferString(`{"balasan":"Balasan ilegal"}`)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/chat/%d", chatDesaB.ID), body)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for replying to foreign chat, got %d", w.Code)
		}
	})

	// 5. Reply chat on own report -> 200 OK (A. Valid related report lookup)
	t.Run("Admin Pemdes Desa A replies to own Desa A chat -> 200 OK", func(t *testing.T) {
		body := bytes.NewBufferString(`{"balasan":"Siap, laporan ditindaklanjuti"}`)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/chat/%d", chatDesaA.ID), body)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK replying to own chat, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 6. Reply chat with nonexistent report -> 404 Not Found (B. Nonexistent report)
	config.DB.Exec("SET FOREIGN_KEY_CHECKS = 0")
	chatNonexistentReport := models.RiwayatChat{LaporanKerusakanID: 999999, UserID: f.wargaA.ID, Pesan: "Chat tanpa laporan"}
	config.DB.Create(&chatNonexistentReport)
	config.DB.Exec("SET FOREIGN_KEY_CHECKS = 1")
	defer func() {
		if chatNonexistentReport.ID > 0 {
			config.DB.Unscoped().Delete(&chatNonexistentReport)
		}
	}()

	t.Run("Reply chat with nonexistent report returns 404 Not Found", func(t *testing.T) {
		body := bytes.NewBufferString(`{"balasan":"Balasan test"}`)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/chat/%d", chatNonexistentReport.ID), body)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found when related report does not exist, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 7. Reply chat with soft-deleted report -> 404 Not Found (C. Soft-deleted report)
	chatSoftDeletedReport := models.RiwayatChat{LaporanKerusakanID: f.lapDeletedA.ID, UserID: f.wargaA.ID, Pesan: "Chat laporan soft deleted"}
	config.DB.Create(&chatSoftDeletedReport)
	defer config.DB.Unscoped().Delete(&chatSoftDeletedReport)

	t.Run("Reply chat with soft-deleted report returns 404 Not Found", func(t *testing.T) {
		body := bytes.NewBufferString(`{"balasan":"Balasan test"}`)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/chat/%d", chatSoftDeletedReport.ID), body)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found when related report is soft-deleted, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 8. Admin Pemdes replies to non-desa roads (Kabupaten/Provinsi/Nasional) -> 403 Forbidden (D. Scope)
	chatProvinsi := models.RiwayatChat{LaporanKerusakanID: f.lapProvinsi.ID, UserID: f.wargaA.ID, Pesan: "Chat jalan provinsi"}
	config.DB.Create(&chatProvinsi)
	defer config.DB.Unscoped().Delete(&chatProvinsi)

	chatNasional := models.RiwayatChat{LaporanKerusakanID: f.lapNasional.ID, UserID: f.wargaA.ID, Pesan: "Chat jalan nasional"}
	config.DB.Create(&chatNasional)
	defer config.DB.Unscoped().Delete(&chatNasional)

	t.Run("Admin Pemdes Desa A replies to Kabupaten/Provinsi/Nasional chat -> 403 Forbidden", func(t *testing.T) {
		for _, cID := range []uint{chatKabupaten.ID, chatProvinsi.ID, chatNasional.ID} {
			body := bytes.NewBufferString(`{"balasan":"Balasan non desa"}`)
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/chat/%d", cID), body)
			req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("expected 403 Forbidden for non-desa chat %d, got %d", cID, w.Code)
			}
		}
	})

	// 9. Admin PU replies: Kabupaten allowed (200), Desa forbidden (403)
	t.Run("Admin PU can reply to Kabupaten chat -> 200 OK", func(t *testing.T) {
		body := bytes.NewBufferString(`{"balasan":"PU merespons"}`)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/chat/%d", chatKabupaten.ID), body)
		req.Header.Set("Authorization", "Bearer "+f.tokenPU)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK for PU replying to kabupaten chat, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("Admin PU cannot reply to Desa chat -> 403 Forbidden", func(t *testing.T) {
		body := bytes.NewBufferString(`{"balasan":"PU merespons desa"}`)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/chat/%d", chatDesaA.ID), body)
		req.Header.Set("Authorization", "Bearer "+f.tokenPU)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for PU replying to desa chat, got %d", w.Code)
		}
	})

	// 10. Superadmin can reply to chats -> 200 OK
	t.Run("Superadmin can reply to chat -> 200 OK", func(t *testing.T) {
		body := bytes.NewBufferString(`{"balasan":"Superadmin merespons"}`)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/chat/%d", chatDesaA.ID), body)
		req.Header.Set("Authorization", "Bearer "+f.tokenSA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK for Superadmin replying to chat, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// =========================================================================
// H, I, J. NOTIFICATIONS, PROFILE, AND SETTINGS SCOPE (AREA 9, 10, 11)
// =========================================================================
func TestBE6_AdminPemdes_SelfResourceScope(t *testing.T) {
	f, cleanup := setupBE6Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// Notifications
	notifPemdesA := models.Notifikasi{UserID: f.pemdesA.ID, Judul: "Notif A", Pesan: "Pesan A"}
	config.DB.Create(&notifPemdesA)
	defer config.DB.Unscoped().Delete(&notifPemdesA)

	notifPemdesB := models.Notifikasi{UserID: f.pemdesB.ID, Judul: "Notif B", Pesan: "Pesan B"}
	config.DB.Create(&notifPemdesB)
	defer config.DB.Unscoped().Delete(&notifPemdesB)

	// 1. Get Notifications: only own notifications
	t.Run("Pemdes A gets only own notifications", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/notifikasi", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp struct {
			Data []models.Notifikasi `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		for _, notif := range resp.Data {
			if notif.UserID != f.pemdesA.ID {
				t.Errorf("NOTIFICATION IDOR: Found notif belonging to UserID=%d in Pemdes A notifs", notif.UserID)
			}
		}
	})

	// 2. Mark Read foreign notification -> 403 Forbidden
	t.Run("Pemdes A cannot mark read Pemdes B notification -> 403 Forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/notifikasi/%d/read", notifPemdesB.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for marking foreign notification read, got %d", w.Code)
		}
	})

	// 3. Profile update: cannot alter role, email, or wilayah_id
	t.Run("Pemdes A profile update preserves role and wilayah_id", func(t *testing.T) {
		body := bytes.NewBufferString(`{
			"name": "Updated Name Pemdes A",
			"role": "super_admin",
			"wilayah_id": 999
		}`)
		req := httptest.NewRequest(http.MethodPut, "/api/profile", body)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		// Verify in DB directly
		var refreshed models.User
		config.DB.First(&refreshed, f.pemdesA.ID)

		if refreshed.Role != models.RoleAdminPemdes {
			t.Errorf("CRITICAL PRIVILEGE ESCALATION: Role was altered via profile update to %s!", refreshed.Role)
		}
		if refreshed.WilayahID == nil || *refreshed.WilayahID != f.desaA.ID {
			t.Errorf("CRITICAL SCOPE TAMPERING: WilayahID was altered via profile update to %v!", refreshed.WilayahID)
		}
	})

	// 4. Settings update: scoped strictly to caller
	t.Run("Pemdes A settings update is scoped to own user", func(t *testing.T) {
		body := bytes.NewBufferString(`{
			"theme": "dark",
			"user_id": 999
		}`)
		req := httptest.NewRequest(http.MethodPut, "/api/settings", body)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var prefPemdesA models.UserPreference
		config.DB.Where("user_id = ?", f.pemdesA.ID).First(&prefPemdesA)
		if prefPemdesA.Theme != "dark" {
			t.Errorf("expected theme dark for Pemdes A, got %s", prefPemdesA.Theme)
		}
	})
}

// =========================================================================
// K. SOFT-DELETED WILAYAH BEHAVIOR (AREA 13)
// =========================================================================
func TestBE6_AdminPemdes_SoftDeletedWilayah(t *testing.T) {
	if !ensureDBForLaporanTest(t) {
		t.Skip("Database not available")
	}

	nowNano := time.Now().UnixNano()
	// Create a wilayah and soft-delete it
	desaSoft := models.Wilayah{Nama: fmt.Sprintf("Desa SoftDel BE6 %d", nowNano), Tipe: "desa"}
	config.DB.Create(&desaSoft)

	userSoft := models.User{
		Name:      "Pemdes SoftDel BE6",
		Email:     fmt.Sprintf("pemdes_soft_%d@roadis.id", nowNano),
		Role:      models.RoleAdminPemdes,
		WilayahID: &desaSoft.ID,
	}
	config.DB.Create(&userSoft)

	tokenSoft, _ := utils.GenerateToken(userSoft.ID, userSoft.Email, userSoft.Role, nil)

	// Now soft delete the wilayah
	config.DB.Delete(&desaSoft)

	defer func() {
		config.DB.Unscoped().Delete(&userSoft)
		config.DB.Unscoped().Delete(&desaSoft)
	}()

	r := gin.New()
	routes.SetupRoutes(r)

	// Test endpoints do not crash (500) and respond gracefully
	testCases := []struct {
		name     string
		method   string
		url      string
		expected int
	}{
		{"Dashboard", http.MethodGet, "/api/admin/dashboard", http.StatusOK},
		{"Laporan List", http.MethodGet, "/api/admin/laporan", http.StatusOK},
		{"Map Laporan", http.MethodGet, "/api/admin/map/laporan", http.StatusOK},
		{"Chat Inbox", http.MethodGet, "/api/admin/chat", http.StatusOK},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.url, nil)
			req.Header.Set("Authorization", "Bearer "+tokenSoft)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.expected {
				t.Errorf("[%s] expected status %d with soft-deleted wilayah, got %d: %s", tc.name, tc.expected, w.Code, w.Body.String())
			}
		})
	}
}

// =========================================================================
// L. PARAMETER TAMPERING & MALFORMED ID (AREA 15)
// =========================================================================
func TestBE6_AdminPemdes_ParameterTamperingAndEdgeCases(t *testing.T) {
	f, cleanup := setupBE6Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	testCases := []struct {
		name       string
		method     string
		url        string
		body       string
		token      string
		expectedIn []int // Expected HTTP status codes (e.g. 400 or 404, never 500)
	}{
		// Report Detail ID edge cases
		{"Detail with string ID", http.MethodGet, "/api/admin/laporan/invalid_id", "", f.tokenPemdesA, []int{http.StatusBadRequest, http.StatusNotFound}},
		{"Detail with ID 0", http.MethodGet, "/api/admin/laporan/0", "", f.tokenPemdesA, []int{http.StatusBadRequest, http.StatusNotFound}},
		{"Detail with negative ID", http.MethodGet, "/api/admin/laporan/-1", "", f.tokenPemdesA, []int{http.StatusBadRequest, http.StatusNotFound}},
		{"Detail with very large ID", http.MethodGet, "/api/admin/laporan/999999999", "", f.tokenPemdesA, []int{http.StatusNotFound}},

		// Chat ID edge cases
		{"Chat detail with string ID", http.MethodGet, "/api/admin/laporan/invalid_id/chat", "", f.tokenPemdesA, []int{http.StatusBadRequest, http.StatusNotFound}},
		{"Chat detail with ID 0", http.MethodGet, "/api/admin/laporan/0/chat", "", f.tokenPemdesA, []int{http.StatusBadRequest, http.StatusNotFound}},
		{"Chat detail with negative ID", http.MethodGet, "/api/admin/laporan/-1/chat", "", f.tokenPemdesA, []int{http.StatusBadRequest, http.StatusNotFound}},
		{"Reply chat with string ID", http.MethodPut, "/api/admin/chat/invalid_id", `{"balasan":"test"}`, f.tokenPemdesA, []int{http.StatusBadRequest, http.StatusNotFound}},
		{"Reply chat with ID 0", http.MethodPut, "/api/admin/chat/0", `{"balasan":"test"}`, f.tokenPemdesA, []int{http.StatusBadRequest, http.StatusNotFound}},

		// Notification ID edge cases
		{"Notification mark read string ID", http.MethodPut, "/api/notifikasi/invalid_id/read", "", f.tokenPemdesA, []int{http.StatusBadRequest, http.StatusNotFound}},
		{"Notification mark read ID 0", http.MethodPut, "/api/notifikasi/0/read", "", f.tokenPemdesA, []int{http.StatusBadRequest, http.StatusNotFound}},
		{"Notification delete string ID", http.MethodDelete, "/api/notifikasi/invalid_id", "", f.tokenPemdesA, []int{http.StatusBadRequest, http.StatusNotFound}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var bodyBuf *bytes.Buffer
			if tc.body != "" {
				bodyBuf = bytes.NewBufferString(tc.body)
			} else {
				bodyBuf = bytes.NewBuffer(nil)
			}
			req := httptest.NewRequest(tc.method, tc.url, bodyBuf)
			req.Header.Set("Authorization", "Bearer "+tc.token)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			matched := false
			for _, code := range tc.expectedIn {
				if w.Code == code {
					matched = true
					break
				}
			}
			if !matched {
				t.Errorf("[%s] expected one of %v, got %d: %s", tc.name, tc.expectedIn, w.Code, w.Body.String())
			}
		})
	}
}

// =========================================================================
// M. CROSS-ROLE REGRESSION (AREA 17)
// =========================================================================
func TestBE6_CrossRole_Regression(t *testing.T) {
	f, cleanup := setupBE6Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// 1. Admin PU READ access to all authorities (Desa, Kabupaten, Provinsi, Nasional)
	t.Run("Admin PU can read all authority reports", func(t *testing.T) {
		reports := []uint{f.lapDesaA.ID, f.lapDesaB.ID, f.lapKabupaten.ID, f.lapProvinsi.ID, f.lapNasional.ID}
		for _, repID := range reports {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d", repID), nil)
			req.Header.Set("Authorization", "Bearer "+f.tokenPU)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Admin PU failed to read report ID %d: got %d: %s", repID, w.Code, w.Body.String())
			}
		}
	})

	// 2. Admin PU UPDATE authority: Kabupaten allowed, Desa / Provinsi / Nasional FORBIDDEN
	t.Run("Admin PU update scope: Kabupaten allowed, non-kabupaten forbidden", func(t *testing.T) {
		// Update Kabupaten -> 200
		bodyKab := bytes.NewBufferString(`{"status":"proses"}`)
		reqKab := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/laporan/%d/status", f.lapKabupaten.ID), bodyKab)
		reqKab.Header.Set("Authorization", "Bearer "+f.tokenPU)
		reqKab.Header.Set("Content-Type", "application/json")
		wKab := httptest.NewRecorder()
		r.ServeHTTP(wKab, reqKab)
		if wKab.Code != http.StatusOK {
			t.Errorf("Admin PU expected 200 updating Kabupaten road, got %d: %s", wKab.Code, wKab.Body.String())
		}

		// Update Desa -> 403
		bodyDesa := bytes.NewBufferString(`{"status":"proses"}`)
		reqDesa := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/laporan/%d/status", f.lapDesaA.ID), bodyDesa)
		reqDesa.Header.Set("Authorization", "Bearer "+f.tokenPU)
		reqDesa.Header.Set("Content-Type", "application/json")
		wDesa := httptest.NewRecorder()
		r.ServeHTTP(wDesa, reqDesa)
		if wDesa.Code != http.StatusForbidden {
			t.Errorf("Admin PU expected 403 updating Desa road, got %d: %s", wDesa.Code, wDesa.Body.String())
		}

		// Update Provinsi -> 403
		bodyProv := bytes.NewBufferString(`{"status":"proses"}`)
		reqProv := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/laporan/%d/status", f.lapProvinsi.ID), bodyProv)
		reqProv.Header.Set("Authorization", "Bearer "+f.tokenPU)
		reqProv.Header.Set("Content-Type", "application/json")
		wProv := httptest.NewRecorder()
		r.ServeHTTP(wProv, reqProv)
		if wProv.Code != http.StatusForbidden {
			t.Errorf("Admin PU expected 403 updating Provinsi road, got %d: %s", wProv.Code, wProv.Body.String())
		}
	})

	// 3. Super Admin full access
	t.Run("Superadmin has full access to reports", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d", f.lapDesaA.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenSA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Superadmin expected 200 reading Desa report, got %d", w.Code)
		}
	})

	// 4. Warga cannot access admin endpoints
	t.Run("Warga cannot access admin endpoints", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("Warga expected 403 accessing admin dashboard, got %d", w.Code)
		}
	})
}

// =========================================================================
// N. DATA MUTATION IMMUTABILITY (AREA 18)
// =========================================================================
func TestBE6_AdminPemdes_DataMutationImmutability(t *testing.T) {
	f, cleanup := setupBE6Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// Attempt mass assignment injection during PUT /api/admin/laporan/:id/status
	maliciousPayload := `{
		"status": "proses",
		"user_id": 9999,
		"wilayah_id": 9999,
		"jenis_jalan": "provinsi",
		"tipe_kerusakan": "jembatan_runtuh",
		"role": "super_admin"
	}`

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/laporan/%d/status", f.lapDesaA.ID), bytes.NewBufferString(maliciousPayload))
	req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	// Verify database record has NOT mutated immutable fields
	var refreshed models.LaporanKerusakan
	config.DB.First(&refreshed, f.lapDesaA.ID)

	if refreshed.UserID != f.wargaA.ID {
		t.Errorf("IMMUTABILITY VIOLATION: UserID altered from %d to %d!", f.wargaA.ID, refreshed.UserID)
	}
	if refreshed.WilayahID != f.desaA.ID {
		t.Errorf("IMMUTABILITY VIOLATION: WilayahID altered from %d to %d!", f.desaA.ID, refreshed.WilayahID)
	}
	if refreshed.JenisJalan != "desa" {
		t.Errorf("IMMUTABILITY VIOLATION: JenisJalan altered from desa to %s!", refreshed.JenisJalan)
	}
	if refreshed.TipeKerusakan != "lubang" {
		t.Errorf("IMMUTABILITY VIOLATION: TipeKerusakan altered from lubang to %s!", refreshed.TipeKerusakan)
	}
}

