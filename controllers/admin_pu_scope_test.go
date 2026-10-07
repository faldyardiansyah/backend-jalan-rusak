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

type be7Fixtures struct {
	desaA      models.Wilayah
	desaB      models.Wilayah
	wargaA     models.User
	wargaB     models.User
	pemdesA    models.User
	adminPU    models.User
	superAdmin models.User

	tokenPemdesA string
	tokenPU      string
	tokenSA      string
	tokenWargaA  string

	lapDesaA     models.LaporanKerusakan
	lapDesaB     models.LaporanKerusakan
	lapKabupaten models.LaporanKerusakan
	lapProvinsi  models.LaporanKerusakan
	lapNasional  models.LaporanKerusakan
	lapDeleted   models.LaporanKerusakan
}

func setupBE7Fixtures(t *testing.T) (*be7Fixtures, func()) {
	if !ensureDBForLaporanTest(t) {
		t.Skip("Database not available")
	}

	f := &be7Fixtures{}
	nowNano := time.Now().UnixNano()

	// 1. Wilayah
	f.desaA = models.Wilayah{Nama: fmt.Sprintf("Desa A BE7 %d", nowNano), Tipe: "desa"}
	config.DB.Create(&f.desaA)
	f.desaB = models.Wilayah{Nama: fmt.Sprintf("Desa B BE7 %d", nowNano), Tipe: "desa"}
	config.DB.Create(&f.desaB)

	// 2. Users
	f.wargaA = models.User{Name: "Warga A BE7", Email: fmt.Sprintf("warga_a_%d@roadis.id", nowNano), Role: models.RoleWarga}
	config.DB.Create(&f.wargaA)
	f.wargaB = models.User{Name: "Warga B BE7", Email: fmt.Sprintf("warga_b_%d@roadis.id", nowNano), Role: models.RoleWarga}
	config.DB.Create(&f.wargaB)

	f.pemdesA = models.User{Name: "Pemdes A BE7", Email: fmt.Sprintf("pemdes_a_%d@roadis.id", nowNano), Role: models.RoleAdminPemdes, WilayahID: &f.desaA.ID}
	config.DB.Create(&f.pemdesA)

	f.adminPU = models.User{Name: "Admin PU BE7", Email: fmt.Sprintf("pu_%d@roadis.id", nowNano), Role: models.RoleAdminPu}
	config.DB.Create(&f.adminPU)

	f.superAdmin = models.User{Name: "Super Admin BE7", Email: fmt.Sprintf("sa_%d@roadis.id", nowNano), Role: models.RoleSuperAdmin}
	config.DB.Create(&f.superAdmin)

	// 3. Tokens
	f.tokenPemdesA, _ = utils.GenerateToken(f.pemdesA.ID, f.pemdesA.Email, f.pemdesA.Role, nil)
	f.tokenPU, _ = utils.GenerateToken(f.adminPU.ID, f.adminPU.Email, f.adminPU.Role, nil)
	f.tokenSA, _ = utils.GenerateToken(f.superAdmin.ID, f.superAdmin.Email, f.superAdmin.Role, nil)
	f.tokenWargaA, _ = utils.GenerateToken(f.wargaA.ID, f.wargaA.Email, f.wargaA.Role, nil)

	// 4. Reports for each authority
	f.lapDesaA = models.LaporanKerusakan{
		UserID: f.wargaA.ID, WilayahID: f.desaA.ID, JenisJalan: "desa", Judul: "Jalan Rusak Desa A BE7",
		Deskripsi: "Deskripsi", Latitude: -6.3400, Longitude: 108.3300, ImageURL: "https://foto.jpg",
		TipeKerusakan: "lubang", Status: "menunggu",
	}
	config.DB.Create(&f.lapDesaA)

	f.lapDesaB = models.LaporanKerusakan{
		UserID: f.wargaB.ID, WilayahID: f.desaB.ID, JenisJalan: "desa", Judul: "Jalan Rusak Desa B BE7",
		Deskripsi: "Deskripsi", Latitude: -6.3450, Longitude: 108.3350, ImageURL: "https://foto.jpg",
		TipeKerusakan: "lubang", Status: "menunggu",
	}
	config.DB.Create(&f.lapDesaB)

	f.lapKabupaten = models.LaporanKerusakan{
		UserID: f.wargaA.ID, WilayahID: f.desaA.ID, JenisJalan: "kabupaten", Judul: "Jalan Rusak Kabupaten BE7",
		Deskripsi: "Deskripsi", Latitude: -6.3500, Longitude: 108.3400, ImageURL: "https://foto.jpg",
		TipeKerusakan: "retak", Status: "menunggu",
	}
	config.DB.Create(&f.lapKabupaten)

	f.lapProvinsi = models.LaporanKerusakan{
		UserID: f.wargaA.ID, WilayahID: f.desaA.ID, JenisJalan: "provinsi", Judul: "Jalan Rusak Provinsi BE7",
		Deskripsi: "Deskripsi", Latitude: -6.3550, Longitude: 108.3450, ImageURL: "https://foto.jpg",
		TipeKerusakan: "ambles", Status: "menunggu",
	}
	config.DB.Create(&f.lapProvinsi)

	f.lapNasional = models.LaporanKerusakan{
		UserID: f.wargaA.ID, WilayahID: f.desaA.ID, JenisJalan: "nasional", Judul: "Jalan Rusak Nasional BE7",
		Deskripsi: "Deskripsi", Latitude: -6.3600, Longitude: 108.3500, ImageURL: "https://foto.jpg",
		TipeKerusakan: "retak", Status: "menunggu",
	}
	config.DB.Create(&f.lapNasional)

	f.lapDeleted = models.LaporanKerusakan{
		UserID: f.wargaA.ID, WilayahID: f.desaA.ID, JenisJalan: "kabupaten", Judul: "Jalan Deleted Kabupaten BE7",
		Deskripsi: "Deskripsi", Latitude: -6.3650, Longitude: 108.3550, ImageURL: "https://foto.jpg",
		TipeKerusakan: "lubang", Status: "menunggu",
		Model: gorm.Model{DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}},
	}
	config.DB.Create(&f.lapDeleted)

	teardown := func() {
		config.DB.Unscoped().Delete(&f.lapDesaA)
		config.DB.Unscoped().Delete(&f.lapDesaB)
		config.DB.Unscoped().Delete(&f.lapKabupaten)
		config.DB.Unscoped().Delete(&f.lapProvinsi)
		config.DB.Unscoped().Delete(&f.lapNasional)
		config.DB.Unscoped().Delete(&f.lapDeleted)
		config.DB.Unscoped().Where("user_id IN ?", []uint{f.wargaA.ID, f.wargaB.ID, f.pemdesA.ID, f.adminPU.ID, f.superAdmin.ID}).Delete(&models.UserPreference{})
		config.DB.Unscoped().Delete(&f.wargaA)
		config.DB.Unscoped().Delete(&f.wargaB)
		config.DB.Unscoped().Delete(&f.pemdesA)
		config.DB.Unscoped().Delete(&f.adminPU)
		config.DB.Unscoped().Delete(&f.superAdmin)
		config.DB.Unscoped().Delete(&f.desaA)
		config.DB.Unscoped().Delete(&f.desaB)
	}

	return f, teardown
}

// =========================================================================
// 1. AUTHENTICATION & ROLE AUTHORIZATION
// =========================================================================
func TestBE7_AdminPU_AuthAndRole(t *testing.T) {
	f, cleanup := setupBE7Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// Unauthenticated request -> 401
	t.Run("Unauthenticated request to /api/admin/laporan returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/laporan", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	// Warga access to /api/admin/* -> 403
	t.Run("Warga access to /api/admin/laporan returns 403 Forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/laporan", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})

	// Admin PU access to /api/superadmin/* -> 403
	t.Run("Admin PU access to /api/superadmin/users returns 403 Forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/superadmin/users", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPU)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})

	// Fail-closed test on unrecognized role: active user in DB with unknown role -> 403
	t.Run("Unknown role fails closed on admin endpoints with 403", func(t *testing.T) {
		unknownUser := models.User{
			Name:  "Unknown Role User",
			Email: fmt.Sprintf("unknown_%d@roadis.id", time.Now().UnixNano()),
			Role:  "guest_role",
		}
		config.DB.Create(&unknownUser)
		defer config.DB.Unscoped().Delete(&unknownUser)

		tokenUnknown, _ := utils.GenerateToken(unknownUser.ID, unknownUser.Email, unknownUser.Role, nil)
		endpoints := []string{"/api/admin/laporan", "/api/admin/dashboard", "/api/admin/map/laporan", "/api/admin/chat"}
		for _, ep := range endpoints {
			req := httptest.NewRequest(http.MethodGet, ep, nil)
			req.Header.Set("Authorization", "Bearer "+tokenUnknown)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusForbidden {
				t.Errorf("[%s] expected 403 for unknown role, got %d", ep, w.Code)
			}
		}
	})
}

// =========================================================================
// 2. REPORT LIST SCOPE & FILTERING (READ ALL AUTHORITIES)
// =========================================================================
func TestBE7_AdminPU_ReportListScope(t *testing.T) {
	f, cleanup := setupBE7Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// Admin PU default view: sees all authorities
	t.Run("Admin PU default list sees reports across all authorities", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/laporan?page=1&limit=100", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPU)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var resp struct {
			Data []models.LaporanKerusakan `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		seenTypes := make(map[string]bool)
		for _, item := range resp.Data {
			seenTypes[item.JenisJalan] = true
		}

		if !seenTypes["desa"] || !seenTypes["kabupaten"] || !seenTypes["provinsi"] || !seenTypes["nasional"] {
			t.Errorf("Admin PU should see all authorities (desa, kabupaten, provinsi, nasional), seen: %v", seenTypes)
		}
	})

	// Authority filters
	authorities := []string{"desa", "kabupaten", "provinsi", "nasional"}
	for _, auth := range authorities {
		t.Run(fmt.Sprintf("Admin PU filters by jenis_jalan=%s", auth), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan?jenis_jalan=%s&page=1&limit=100", auth), nil)
			req.Header.Set("Authorization", "Bearer "+f.tokenPU)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200 OK for %s, got %d", auth, w.Code)
			}

			var resp struct {
				Data []models.LaporanKerusakan `json:"data"`
			}
			_ = json.Unmarshal(w.Body.Bytes(), &resp)

			for _, item := range resp.Data {
				if item.JenisJalan != auth {
					t.Errorf("Filter breach: expected only %s, found %s", auth, item.JenisJalan)
				}
			}
		})
	}

	// Filter with all returns all authorities
	t.Run("Admin PU filters by jenis_jalan=all returns all authorities", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/laporan?jenis_jalan=all&page=1&limit=100", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPU)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
	})
}

// =========================================================================
// 3. REPORT DETAIL ACCESS & IDOR (READ ALL AUTHORITIES)
// =========================================================================
func TestBE7_AdminPU_ReportDetailAccess(t *testing.T) {
	f, cleanup := setupBE7Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// Admin PU can view detail of all road types: Desa, Kabupaten, Provinsi, Nasional
	testDetails := []struct {
		name     string
		reportID uint
		expected int
	}{
		{"Desa report", f.lapDesaA.ID, http.StatusOK},
		{"Desa B report", f.lapDesaB.ID, http.StatusOK},
		{"Kabupaten report", f.lapKabupaten.ID, http.StatusOK},
		{"Provinsi report", f.lapProvinsi.ID, http.StatusOK},
		{"Nasional report", f.lapNasional.ID, http.StatusOK},
		{"Soft-deleted report", f.lapDeleted.ID, http.StatusNotFound},
		{"Non-existent report", 999999, http.StatusNotFound},
	}

	for _, td := range testDetails {
		t.Run("Admin PU reads "+td.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d", td.reportID), nil)
			req.Header.Set("Authorization", "Bearer "+f.tokenPU)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != td.expected {
				t.Errorf("[%s] expected %d, got %d: %s", td.name, td.expected, w.Code, w.Body.String())
			}
		})
	}

	// Malformed IDs
	t.Run("Admin PU reads malformed ID -> 400 Bad Request", func(t *testing.T) {
		for _, badID := range []string{"abc", "0"} {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%s", badID), nil)
			req.Header.Set("Authorization", "Bearer "+f.tokenPU)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for bad ID '%s', got %d", badID, w.Code)
			}
		}
	})
}

// =========================================================================
// 4. STATUS UPDATE SCOPE (WRITE SCOPE: KABUPATEN ONLY)
// =========================================================================
func TestBE7_AdminPU_StatusUpdateScope(t *testing.T) {
	f, cleanup := setupBE7Fixtures(t)
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

	// 1. Kabupaten report -> 200 ALLOWED
	t.Run("Admin PU updates Kabupaten report -> 200 OK", func(t *testing.T) {
		w := sendUpdateReq(f.tokenPU, f.lapKabupaten.ID, "proses")
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK updating Kabupaten road, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 2. Desa report -> 403 FORBIDDEN
	t.Run("Admin PU updates Desa report -> 403 Forbidden", func(t *testing.T) {
		w := sendUpdateReq(f.tokenPU, f.lapDesaA.ID, "proses")
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden updating Desa road, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 3. Provinsi report -> 403 FORBIDDEN
	t.Run("Admin PU updates Provinsi report -> 403 Forbidden", func(t *testing.T) {
		w := sendUpdateReq(f.tokenPU, f.lapProvinsi.ID, "proses")
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden updating Provinsi road, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 4. Nasional report -> 403 FORBIDDEN
	t.Run("Admin PU updates Nasional report -> 403 Forbidden", func(t *testing.T) {
		w := sendUpdateReq(f.tokenPU, f.lapNasional.ID, "proses")
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden updating Nasional road, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 5. BE-5 Lifecycle Guards: selesai without evidence rejected
	t.Run("Admin PU updating status to selesai without evidence is rejected", func(t *testing.T) {
		w := sendUpdateReq(f.tokenPU, f.lapKabupaten.ID, "selesai")
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request updating to selesai without evidence, got %d", w.Code)
		}
	})
}

// =========================================================================
// 5. MAP SCOPE (MONITOR ALL AUTHORITIES)
// =========================================================================
func TestBE7_AdminPU_MapScope(t *testing.T) {
	f, cleanup := setupBE7Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// Admin PU default map: sees all authorities
	t.Run("Admin PU default map returns markers across all authorities", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/map/laporan", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPU)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var resp struct {
			Data []adminController.MapPoint `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		seenMapTypes := make(map[string]bool)
		for _, pt := range resp.Data {
			seenMapTypes[pt.JenisJalan] = true
		}

		if !seenMapTypes["desa"] || !seenMapTypes["kabupaten"] || !seenMapTypes["provinsi"] || !seenMapTypes["nasional"] {
			t.Errorf("Admin PU should see all road markers on map, seen: %v", seenMapTypes)
		}
	})

	// Filter by authority on map
	t.Run("Admin PU filters map by jenis_jalan=kabupaten", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/map/laporan?jenis_jalan=kabupaten", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPU)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp struct {
			Data []adminController.MapPoint `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		for _, pt := range resp.Data {
			if pt.JenisJalan != "kabupaten" {
				t.Errorf("Map filter breach: expected only kabupaten, got %s", pt.JenisJalan)
			}
		}
	})
}

// =========================================================================
// 6. DASHBOARD SCOPE (SCOPED TO KABUPATEN ONLY)
// =========================================================================
func TestBE7_AdminPU_DashboardScope(t *testing.T) {
	f, cleanup := setupBE7Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+f.tokenPU)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			TotalLaporan int64 `json:"total_laporan"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	// Admin PU dashboard must count ONLY active kabupaten reports
	var expectedKabupatenCount int64
	config.DB.Model(&models.LaporanKerusakan{}).
		Where("jenis_jalan = ? AND deleted_at IS NULL", "kabupaten").
		Count(&expectedKabupatenCount)

	if resp.Data.TotalLaporan != expectedKabupatenCount {
		t.Errorf("DASHBOARD SCOPE ERROR: expected %d kabupaten reports, got %d", expectedKabupatenCount, resp.Data.TotalLaporan)
	}
}

// =========================================================================
// 7. CHAT SCOPE (KABUPATEN ONLY)
// =========================================================================
func TestBE7_AdminPU_ChatScope(t *testing.T) {
	f, cleanup := setupBE7Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// Create chats on different authority reports
	chatKab := models.RiwayatChat{LaporanKerusakanID: f.lapKabupaten.ID, UserID: f.wargaA.ID, Pesan: "Chat di Kabupaten BE7"}
	config.DB.Create(&chatKab)
	defer config.DB.Unscoped().Delete(&chatKab)

	chatDesa := models.RiwayatChat{LaporanKerusakanID: f.lapDesaA.ID, UserID: f.wargaA.ID, Pesan: "Chat di Desa BE7"}
	config.DB.Create(&chatDesa)
	defer config.DB.Unscoped().Delete(&chatDesa)

	chatProv := models.RiwayatChat{LaporanKerusakanID: f.lapProvinsi.ID, UserID: f.wargaA.ID, Pesan: "Chat di Provinsi BE7"}
	config.DB.Create(&chatProv)
	defer config.DB.Unscoped().Delete(&chatProv)

	chatNas := models.RiwayatChat{LaporanKerusakanID: f.lapNasional.ID, UserID: f.wargaA.ID, Pesan: "Chat di Nasional BE7"}
	config.DB.Create(&chatNas)
	defer config.DB.Unscoped().Delete(&chatNas)

	// 1. Admin PU Inbox: contains only Kabupaten chats
	t.Run("Admin PU inbox contains ONLY kabupaten chats", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/chat", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPU)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp struct {
			Data []struct {
				JenisJalan string `json:"jenis_jalan"`
			} `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		for _, item := range resp.Data {
			if item.JenisJalan != "kabupaten" {
				t.Errorf("CHAT INBOX BREACH: Found non-kabupaten chat ('%s') in PU inbox", item.JenisJalan)
			}
		}
	})

	// 2. Read chat by report ID
	t.Run("Admin PU reads chat on Kabupaten report -> 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d/chat", f.lapKabupaten.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPU)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK reading kabupaten chat, got %d", w.Code)
		}
	})

	t.Run("Admin PU reads chat on Desa report -> 403 Forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d/chat", f.lapDesaA.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPU)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden reading desa chat, got %d", w.Code)
		}
	})

	t.Run("Admin PU reads chat on Provinsi/Nasional reports -> 403 Forbidden", func(t *testing.T) {
		for _, lapID := range []uint{f.lapProvinsi.ID, f.lapNasional.ID} {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d/chat", lapID), nil)
			req.Header.Set("Authorization", "Bearer "+f.tokenPU)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("expected 403 Forbidden reading non-kabupaten chat %d, got %d", lapID, w.Code)
			}
		}
	})

	// 3. Reply chat
	t.Run("Admin PU replies to Kabupaten chat -> 200 OK", func(t *testing.T) {
		body := bytes.NewBufferString(`{"balasan":"Admin PU merespons chat kabupaten"}`)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/chat/%d", chatKab.ID), body)
		req.Header.Set("Authorization", "Bearer "+f.tokenPU)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK replying to kabupaten chat, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("Admin PU replies to Desa chat -> 403 Forbidden", func(t *testing.T) {
		body := bytes.NewBufferString(`{"balasan":"Balasan ilegal desa"}`)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/chat/%d", chatDesa.ID), body)
		req.Header.Set("Authorization", "Bearer "+f.tokenPU)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden replying to desa chat, got %d", w.Code)
		}
	})

	t.Run("Admin PU replies to Provinsi/Nasional chats -> 403 Forbidden", func(t *testing.T) {
		for _, cID := range []uint{chatProv.ID, chatNas.ID} {
			body := bytes.NewBufferString(`{"balasan":"Balasan ilegal"}`)
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/chat/%d", cID), body)
			req.Header.Set("Authorization", "Bearer "+f.tokenPU)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("expected 403 Forbidden replying to chat %d, got %d", cID, w.Code)
			}
		}
	})
}

// =========================================================================
// 8. NOTIFICATIONS SCOPE & IDOR
// =========================================================================
func TestBE7_AdminPU_NotificationsScope(t *testing.T) {
	f, cleanup := setupBE7Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	notifPU := models.Notifikasi{UserID: f.adminPU.ID, Judul: "Notif PU", Pesan: "Pesan PU"}
	config.DB.Create(&notifPU)
	defer config.DB.Unscoped().Delete(&notifPU)

	notifOther := models.Notifikasi{UserID: f.pemdesA.ID, Judul: "Notif Pemdes", Pesan: "Pesan Pemdes"}
	config.DB.Create(&notifOther)
	defer config.DB.Unscoped().Delete(&notifOther)

	// Admin PU only gets own notifications
	t.Run("Admin PU gets only own notifications", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/notifikasi", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPU)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp struct {
			Data []models.Notifikasi `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		for _, notif := range resp.Data {
			if notif.UserID != f.adminPU.ID {
				t.Errorf("NOTIFICATION LEAK: Admin PU received notification belonging to UserID=%d", notif.UserID)
			}
		}
	})

	// Admin PU cannot mark read another user's notification -> 403
	t.Run("Admin PU cannot mark read another user's notification -> 403 Forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/notifikasi/%d/read", notifOther.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPU)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %d", w.Code)
		}
	})
}

// =========================================================================
// 9. PROFILE & SETTINGS SCOPE
// =========================================================================
func TestBE7_AdminPU_ProfileAndSettingsScope(t *testing.T) {
	f, cleanup := setupBE7Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// Profile update cannot change role or inject wilayah_id
	t.Run("Admin PU profile update cannot alter role or inject wilayah", func(t *testing.T) {
		body := bytes.NewBufferString(fmt.Sprintf(`{
			"name": "Updated PU Name",
			"role": "super_admin",
			"wilayah_id": %d
		}`, f.desaA.ID))
		req := httptest.NewRequest(http.MethodPut, "/api/profile", body)
		req.Header.Set("Authorization", "Bearer "+f.tokenPU)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var refreshed models.User
		config.DB.First(&refreshed, f.adminPU.ID)

		if refreshed.Role != models.RoleAdminPu {
			t.Errorf("CRITICAL PRIVILEGE ESCALATION: Role altered to %s", refreshed.Role)
		}
		if refreshed.WilayahID != nil {
			t.Errorf("ILLEGAL WILAYAH INJECTION: WilayahID altered to %v", refreshed.WilayahID)
		}
	})

	// Settings update is scoped to own user
	t.Run("Admin PU settings update is scoped to caller", func(t *testing.T) {
		body := bytes.NewBufferString(`{"theme":"dark","user_id":9999}`)
		req := httptest.NewRequest(http.MethodPut, "/api/settings", body)
		req.Header.Set("Authorization", "Bearer "+f.tokenPU)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var pref models.UserPreference
		config.DB.Where("user_id = ?", f.adminPU.ID).First(&pref)
		if pref.Theme != "dark" {
			t.Errorf("expected theme dark for PU, got %s", pref.Theme)
		}
	})
}

// =========================================================================
// 10. COMPREHENSIVE CROSS-AUTHORITY MATRIX
// =========================================================================
func TestBE7_Comprehensive_CrossAuthorityMatrix(t *testing.T) {
	f, cleanup := setupBE7Fixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// Matrix PU:
	// Read:   Desa(200), Kab(200), Prov(200), Nas(200)
	// Update: Desa(403), Kab(200), Prov(403), Nas(403)
	// Chat:   Desa(403), Kab(200), Prov(403), Nas(403)

	matrix := []struct {
		name       string
		reportID   uint
		readExp    int
		updateExp  int
		chatExp    int
	}{
		{"Desa", f.lapDesaA.ID, http.StatusOK, http.StatusForbidden, http.StatusForbidden},
		{"Kabupaten", f.lapKabupaten.ID, http.StatusOK, http.StatusOK, http.StatusOK},
		{"Provinsi", f.lapProvinsi.ID, http.StatusOK, http.StatusForbidden, http.StatusForbidden},
		{"Nasional", f.lapNasional.ID, http.StatusOK, http.StatusForbidden, http.StatusForbidden},
	}

	for _, item := range matrix {
		t.Run(fmt.Sprintf("Matrix %s", item.name), func(t *testing.T) {
			// Read
			reqRead := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d", item.reportID), nil)
			reqRead.Header.Set("Authorization", "Bearer "+f.tokenPU)
			wRead := httptest.NewRecorder()
			r.ServeHTTP(wRead, reqRead)
			if wRead.Code != item.readExp {
				t.Errorf("[%s Read] expected %d, got %d", item.name, item.readExp, wRead.Code)
			}

			// Update
			bodyUp := bytes.NewBufferString(`{"status":"proses"}`)
			reqUp := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/laporan/%d/status", item.reportID), bodyUp)
			reqUp.Header.Set("Authorization", "Bearer "+f.tokenPU)
			reqUp.Header.Set("Content-Type", "application/json")
			wUp := httptest.NewRecorder()
			r.ServeHTTP(wUp, reqUp)
			if wUp.Code != item.updateExp {
				t.Errorf("[%s Update] expected %d, got %d: %s", item.name, item.updateExp, wUp.Code, wUp.Body.String())
			}

			// Chat Read
			reqChat := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d/chat", item.reportID), nil)
			reqChat.Header.Set("Authorization", "Bearer "+f.tokenPU)
			wChat := httptest.NewRecorder()
			r.ServeHTTP(wChat, reqChat)
			if wChat.Code != item.chatExp {
				t.Errorf("[%s Chat Read] expected %d, got %d", item.name, item.chatExp, wChat.Code)
			}
		})
	}
}
