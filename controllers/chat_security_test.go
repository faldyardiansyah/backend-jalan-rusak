package controllers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/controllers"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/routes"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

type chatSecurityFixtures struct {
	desaA      models.Wilayah
	desaB      models.Wilayah
	wargaA     models.User
	wargaB     models.User
	pemdesA    models.User
	pemdesB    models.User
	adminPU    models.User
	superAdmin models.User
	deletedUser models.User

	tokenPemdesA    string
	tokenPemdesB    string
	tokenPU         string
	tokenSA         string
	tokenWargaA     string
	tokenWargaB     string
	tokenDeletedUser string

	lapDesaA     models.LaporanKerusakan
	lapDesaB     models.LaporanKerusakan
	lapKabupaten models.LaporanKerusakan
	lapProvinsi  models.LaporanKerusakan
	lapNasional  models.LaporanKerusakan
	lapDeleted   models.LaporanKerusakan

	chatDesaA     models.RiwayatChat
	chatDesaB     models.RiwayatChat
	chatKabupaten models.RiwayatChat
	chatDeleted   models.RiwayatChat
}

func setupChatSecurityFixtures(t *testing.T) (*chatSecurityFixtures, func()) {
	if !ensureDBForLaporanTest(t) {
		t.Skip("Database not available")
	}

	f := &chatSecurityFixtures{}
	nowNano := time.Now().UnixNano()

	// 1. Wilayah
	f.desaA = models.Wilayah{Nama: fmt.Sprintf("Desa A Chat %d", nowNano), Tipe: "desa"}
	config.DB.Create(&f.desaA)
	f.desaB = models.Wilayah{Nama: fmt.Sprintf("Desa B Chat %d", nowNano), Tipe: "desa"}
	config.DB.Create(&f.desaB)

	// 2. Users
	f.wargaA = models.User{Name: "Warga A Chat", Email: fmt.Sprintf("warga_a_%d@chat.test", nowNano), Role: models.RoleWarga, TokenVersion: 1}
	config.DB.Create(&f.wargaA)
	f.wargaB = models.User{Name: "Warga B Chat", Email: fmt.Sprintf("warga_b_%d@chat.test", nowNano), Role: models.RoleWarga, TokenVersion: 1}
	config.DB.Create(&f.wargaB)

	f.pemdesA = models.User{Name: "Pemdes A Chat", Email: fmt.Sprintf("pemdes_a_%d@chat.test", nowNano), Role: models.RoleAdminPemdes, WilayahID: &f.desaA.ID, TokenVersion: 1}
	config.DB.Create(&f.pemdesA)
	f.pemdesB = models.User{Name: "Pemdes B Chat", Email: fmt.Sprintf("pemdes_b_%d@chat.test", nowNano), Role: models.RoleAdminPemdes, WilayahID: &f.desaB.ID, TokenVersion: 1}
	config.DB.Create(&f.pemdesB)

	f.adminPU = models.User{Name: "Admin PU Chat", Email: fmt.Sprintf("pu_%d@chat.test", nowNano), Role: models.RoleAdminPu, TokenVersion: 1}
	config.DB.Create(&f.adminPU)
	f.superAdmin = models.User{Name: "Super Admin Chat", Email: fmt.Sprintf("sa_%d@chat.test", nowNano), Role: models.RoleSuperAdmin, TokenVersion: 1}
	config.DB.Create(&f.superAdmin)

	f.deletedUser = models.User{
		Name:         "Deleted User",
		Email:        fmt.Sprintf("deleted_%d@chat.test", nowNano),
		Role:         models.RoleWarga,
		TokenVersion: 1,
		Model:        gorm.Model{DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}},
	}
	config.DB.Create(&f.deletedUser)

	// 3. Tokens
	f.tokenPemdesA, _ = utils.GenerateToken(f.pemdesA.ID, f.pemdesA.Email, f.pemdesA.Role, nil)
	f.tokenPemdesB, _ = utils.GenerateToken(f.pemdesB.ID, f.pemdesB.Email, f.pemdesB.Role, nil)
	f.tokenPU, _ = utils.GenerateToken(f.adminPU.ID, f.adminPU.Email, f.adminPU.Role, nil)
	f.tokenSA, _ = utils.GenerateToken(f.superAdmin.ID, f.superAdmin.Email, f.superAdmin.Role, nil)
	f.tokenWargaA, _ = utils.GenerateToken(f.wargaA.ID, f.wargaA.Email, f.wargaA.Role, nil)
	f.tokenWargaB, _ = utils.GenerateToken(f.wargaB.ID, f.wargaB.Email, f.wargaB.Role, nil)
	f.tokenDeletedUser, _ = utils.GenerateToken(f.deletedUser.ID, f.deletedUser.Email, f.deletedUser.Role, nil)

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
		UserID: f.wargaA.ID, WilayahID: f.desaA.ID, JenisJalan: "kabupaten", Judul: "Jalan Rusak Kabupaten",
		Deskripsi: "Deskripsi", Latitude: -6.3500, Longitude: 108.3400, ImageURL: "https://foto.jpg",
		TipeKerusakan: "retak", Status: "proses",
	}
	config.DB.Create(&f.lapKabupaten)

	f.lapProvinsi = models.LaporanKerusakan{
		UserID: f.wargaA.ID, WilayahID: f.desaA.ID, JenisJalan: "provinsi", Judul: "Jalan Rusak Provinsi",
		Deskripsi: "Deskripsi", Latitude: -6.3550, Longitude: 108.3450, ImageURL: "https://foto.jpg",
		TipeKerusakan: "amblas", Status: "menunggu",
	}
	config.DB.Create(&f.lapProvinsi)

	f.lapNasional = models.LaporanKerusakan{
		UserID: f.wargaA.ID, WilayahID: f.desaA.ID, JenisJalan: "nasional", Judul: "Jalan Rusak Nasional",
		Deskripsi: "Deskripsi", Latitude: -6.3600, Longitude: 108.3500, ImageURL: "https://foto.jpg",
		TipeKerusakan: "lubang", Status: "menunggu",
	}
	config.DB.Create(&f.lapNasional)

	f.lapDeleted = models.LaporanKerusakan{
		UserID: f.wargaA.ID, WilayahID: f.desaA.ID, JenisJalan: "desa", Judul: "Jalan Rusak Deleted",
		Deskripsi: "Deskripsi", Latitude: -6.3650, Longitude: 108.3550, ImageURL: "https://foto.jpg",
		TipeKerusakan: "lubang", Status: "menunggu",
		Model: gorm.Model{DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}},
	}
	config.DB.Create(&f.lapDeleted)

	// 5. Chats
	f.chatDesaA = models.RiwayatChat{LaporanKerusakanID: f.lapDesaA.ID, UserID: f.wargaA.ID, Pesan: "Pesan Warga Desa A"}
	config.DB.Create(&f.chatDesaA)

	f.chatDesaB = models.RiwayatChat{LaporanKerusakanID: f.lapDesaB.ID, UserID: f.wargaB.ID, Pesan: "Pesan Warga Desa B"}
	config.DB.Create(&f.chatDesaB)

	f.chatKabupaten = models.RiwayatChat{LaporanKerusakanID: f.lapKabupaten.ID, UserID: f.wargaA.ID, Pesan: "Pesan Warga Kabupaten"}
	config.DB.Create(&f.chatKabupaten)

	f.chatDeleted = models.RiwayatChat{
		LaporanKerusakanID: f.lapDesaA.ID,
		UserID:             f.wargaA.ID,
		Pesan:              "Pesan Chat Terhapus",
		Model:              gorm.Model{DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}},
	}
	config.DB.Create(&f.chatDeleted)

	teardown := func() {
		config.DB.Unscoped().Delete(&f.chatDesaA)
		config.DB.Unscoped().Delete(&f.chatDesaB)
		config.DB.Unscoped().Delete(&f.chatKabupaten)
		config.DB.Unscoped().Delete(&f.chatDeleted)
		config.DB.Unscoped().Delete(&f.lapDesaA)
		config.DB.Unscoped().Delete(&f.lapDesaB)
		config.DB.Unscoped().Delete(&f.lapKabupaten)
		config.DB.Unscoped().Delete(&f.lapProvinsi)
		config.DB.Unscoped().Delete(&f.lapNasional)
		config.DB.Unscoped().Delete(&f.lapDeleted)
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
// 1. AUTHENTICATION TESTS
// =========================================================================
func TestBE9_ChatSecurity_Authentication(t *testing.T) {
	f, cleanup := setupChatSecurityFixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// A. Missing token -> 401
	t.Run("Missing token rejected with 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/warga/laporan/%d/chat", f.lapDesaA.ID), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for missing token, got %d", w.Code)
		}
	})

	// B. Invalid JWT -> 401
	t.Run("Invalid JWT format/signature rejected with 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/warga/laporan/%d/chat", f.lapDesaA.ID), nil)
		req.Header.Set("Authorization", "Bearer invalid.jwt.signature")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for invalid JWT, got %d", w.Code)
		}
	})

	// C. Expired JWT -> 401
	t.Run("Expired JWT rejected with 401", func(t *testing.T) {
		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			secret = os.Getenv("JWT_SECRET_KEY")
		}
		expiredClaims := utils.JWTClaim{
			UserID:       f.wargaA.ID,
			Email:        f.wargaA.Email,
			Role:         f.wargaA.Role,
			TokenVersion: 1,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			},
		}
		expiredTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
		expToken, _ := expiredTokenObj.SignedString([]byte(secret))

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/warga/laporan/%d/chat", f.lapDesaA.ID), nil)
		req.Header.Set("Authorization", "Bearer "+expToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for expired token, got %d", w.Code)
		}
	})

	// D. Soft-deleted user -> 401
	t.Run("Soft-deleted user rejected with 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/warga/laporan/%d/chat", f.lapDesaA.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenDeletedUser)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for soft-deleted user, got %d", w.Code)
		}
	})

	// E. Token version mismatch -> 401
	t.Run("Token version mismatch (revoked token) rejected with 401", func(t *testing.T) {
		config.DB.Model(&f.wargaA).Update("token_version", 2)
		defer config.DB.Model(&f.wargaA).Update("token_version", 1)

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/warga/laporan/%d/chat", f.lapDesaA.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for token_version mismatch, got %d", w.Code)
		}
	})

	// F. Unknown role -> 403
	t.Run("Unknown role token rejected with 403", func(t *testing.T) {
		unknownRoleToken, _ := utils.GenerateToken(f.wargaA.ID, f.wargaA.Email, "hacker_role", nil)
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/warga/laporan/%d/chat", f.lapDesaA.ID), nil)
		req.Header.Set("Authorization", "Bearer "+unknownRoleToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for unknown role, got %d", w.Code)
		}
	})
}

// =========================================================================
// 2. AUTHORIZATION & CROSS-ROLE ISOLATION TESTS
// =========================================================================
func TestBE9_ChatSecurity_CrossRoleIsolation(t *testing.T) {
	f, cleanup := setupChatSecurityFixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// A. Warga ownership
	t.Run("Warga can read chat of own report -> 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/warga/laporan/%d/chat", f.lapDesaA.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 for Warga own report chat, got %d", w.Code)
		}
	})

	t.Run("Warga cannot read chat of foreign report -> 403 (IDOR guard)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/warga/laporan/%d/chat", f.lapDesaB.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for foreign report chat read, got %d", w.Code)
		}
	})

	t.Run("Warga can send chat to own report -> 200", func(t *testing.T) {
		body := bytes.NewBufferString(`{"pesan":"Pertanyaan tindak lanjut perbaikan"}`)
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/warga/laporan/%d/chat", f.lapDesaA.ID), body)
		req.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 for Warga own report message, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("Warga cannot send chat to foreign report -> 403 (IDOR guard)", func(t *testing.T) {
		body := bytes.NewBufferString(`{"pesan":"Pesan ilegal di laporan lain"}`)
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/warga/laporan/%d/chat", f.lapDesaB.ID), body)
		req.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for sending chat to foreign report, got %d", w.Code)
		}
	})

	t.Run("Warga cannot call admin chat reply endpoint -> 403", func(t *testing.T) {
		body := bytes.NewBufferString(`{"balasan":"Palsukan balasan admin"}`)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/chat/%d", f.chatDesaA.ID), body)
		req.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for Warga attempting admin chat reply, got %d", w.Code)
		}
	})

	// B. Admin Pemdes Scope
	t.Run("Admin Pemdes Desa A reads own Desa report chat -> 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d/chat", f.lapDesaA.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 for Pemdes reading own Desa chat, got %d", w.Code)
		}
	})

	t.Run("Admin Pemdes Desa A reads foreign Desa B report chat -> 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d/chat", f.lapDesaB.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for Pemdes reading foreign Desa chat, got %d", w.Code)
		}
	})

	t.Run("Admin Pemdes Desa A reads Kabupaten report chat -> 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d/chat", f.lapKabupaten.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for Pemdes reading Kabupaten chat, got %d", w.Code)
		}
	})

	t.Run("Admin Pemdes Desa A reads Provinsi report chat -> 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d/chat", f.lapProvinsi.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for Pemdes reading Provinsi chat, got %d", w.Code)
		}
	})

	t.Run("Admin Pemdes Desa A reads Nasional report chat -> 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d/chat", f.lapNasional.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for Pemdes reading Nasional chat, got %d", w.Code)
		}
	})

	t.Run("Admin Pemdes Desa A replies to own Desa chat -> 200", func(t *testing.T) {
		body := bytes.NewBufferString(`{"balasan":"Tim desa segera meninjau"}`)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/chat/%d", f.chatDesaA.ID), body)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 for Pemdes replying to own Desa chat, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("Admin Pemdes Desa A replies to foreign Desa B chat -> 403", func(t *testing.T) {
		body := bytes.NewBufferString(`{"balasan":"Balasan ilegal"}`)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/chat/%d", f.chatDesaB.ID), body)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for Pemdes replying to foreign Desa chat, got %d", w.Code)
		}
	})

	t.Run("Admin Pemdes Desa A replies to Kabupaten chat -> 403", func(t *testing.T) {
		body := bytes.NewBufferString(`{"balasan":"Balasan ilegal pemdes di kabupaten"}`)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/chat/%d", f.chatKabupaten.ID), body)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for Pemdes replying to Kabupaten chat, got %d", w.Code)
		}
	})

	// C. Admin PU Scope
	t.Run("Admin PU reads Kabupaten report chat -> 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d/chat", f.lapKabupaten.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPU)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 for PU reading Kabupaten chat, got %d", w.Code)
		}
	})

	t.Run("Admin PU reads Desa report chat -> 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d/chat", f.lapDesaA.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPU)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for PU reading Desa chat, got %d", w.Code)
		}
	})

	t.Run("Admin PU reads Provinsi report chat -> 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d/chat", f.lapProvinsi.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPU)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for PU reading Provinsi chat, got %d", w.Code)
		}
	})

	t.Run("Admin PU reads Nasional report chat -> 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d/chat", f.lapNasional.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenPU)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for PU reading Nasional chat, got %d", w.Code)
		}
	})

	t.Run("Admin PU replies to Kabupaten chat -> 200", func(t *testing.T) {
		body := bytes.NewBufferString(`{"balasan":"Dinas PU telah mengagendakan pengaspalan"}`)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/chat/%d", f.chatKabupaten.ID), body)
		req.Header.Set("Authorization", "Bearer "+f.tokenPU)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 for PU replying to Kabupaten chat, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("Admin PU replies to Desa chat -> 403", func(t *testing.T) {
		body := bytes.NewBufferString(`{"balasan":"PU balas desa"}`)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/chat/%d", f.chatDesaA.ID), body)
		req.Header.Set("Authorization", "Bearer "+f.tokenPU)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for PU replying to Desa chat, got %d", w.Code)
		}
	})

	// D. Superadmin Scope
	t.Run("Superadmin reads all chats (Desa, Kabupaten, Provinsi, Nasional) -> 200", func(t *testing.T) {
		reports := []models.LaporanKerusakan{f.lapDesaA, f.lapKabupaten, f.lapProvinsi, f.lapNasional}
		for _, rep := range reports {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d/chat", rep.ID), nil)
			req.Header.Set("Authorization", "Bearer "+f.tokenSA)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("expected 200 for Superadmin reading chat on road %s, got %d", rep.JenisJalan, w.Code)
			}
		}
	})

	t.Run("Superadmin replies to chat on any road -> 200", func(t *testing.T) {
		body := bytes.NewBufferString(`{"balasan":"Balasan dari Superadmin"}`)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/chat/%d", f.chatDesaA.ID), body)
		req.Header.Set("Authorization", "Bearer "+f.tokenSA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 for Superadmin replying to chat, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("Admin role attempting to send message as Warga -> 403", func(t *testing.T) {
		body := bytes.NewBufferString(`{"pesan":"Admin coba kirim pesan warga"}`)
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/warga/laporan/%d/chat", f.lapDesaA.ID), body)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for admin calling warga send chat endpoint, got %d", w.Code)
		}
	})
}

// =========================================================================
// 3. ID VALIDATION & MALFORMED PARAMETERS
// =========================================================================
func TestBE9_ChatSecurity_IDValidation(t *testing.T) {
	f, cleanup := setupChatSecurityFixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	malformedIDs := []struct {
		idStr string
		desc  string
	}{
		{"abc", "Alphabetic ID"},
		{"-1", "Negative ID"},
		{"0", "Zero ID"},
		{"%201%20", "Whitespace-padded ID"},
		{"99999999999999999999999999999999999", "Huge integer out of range"},
	}

	for _, tc := range malformedIDs {
		t.Run(fmt.Sprintf("Read chat with %s returns 400", tc.desc), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/warga/laporan/%s/chat", tc.idStr), nil)
			req.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("[%s] expected 400 Bad Request, got %d", tc.desc, w.Code)
			}
		})

		t.Run(fmt.Sprintf("Send chat with %s returns 400", tc.desc), func(t *testing.T) {
			body := bytes.NewBufferString(`{"pesan":"Halo admin"}`)
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/warga/laporan/%s/chat", tc.idStr), body)
			req.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("[%s] expected 400 Bad Request, got %d", tc.desc, w.Code)
			}
		})

		t.Run(fmt.Sprintf("Reply chat with %s returns 400", tc.desc), func(t *testing.T) {
			body := bytes.NewBufferString(`{"balasan":"Halo warga"}`)
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/chat/%s", tc.idStr), body)
			req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("[%s] expected 400 Bad Request, got %d", tc.desc, w.Code)
			}
		})
	}

	t.Run("Nonexistent valid report ID returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/warga/laporan/999999/chat", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 for nonexistent report ID, got %d", w.Code)
		}
	})

	t.Run("Nonexistent valid chat ID returns 404 on reply", func(t *testing.T) {
		body := bytes.NewBufferString(`{"balasan":"Balasan untuk chat ghaib"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/admin/chat/999999", body)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 for nonexistent chat ID, got %d", w.Code)
		}
	})
}

// =========================================================================
// 4. SOFT-DELETE & ORPHAN INTEGRITY TESTS
// =========================================================================
func TestBE9_ChatSecurity_SoftDeleteIntegrity(t *testing.T) {
	f, cleanup := setupChatSecurityFixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// A. Chat on soft-deleted report cannot be read
	t.Run("Reading chat of soft-deleted report returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/warga/laporan/%d/chat", f.lapDeleted.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 for soft-deleted report chat read, got %d", w.Code)
		}
	})

	// B. Sending chat to soft-deleted report returns 404
	t.Run("Sending chat to soft-deleted report returns 404", func(t *testing.T) {
		body := bytes.NewBufferString(`{"pesan":"Pesan ke laporan terhapus"}`)
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/warga/laporan/%d/chat", f.lapDeleted.ID), body)
		req.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 for sending chat to soft-deleted report, got %d", w.Code)
		}
	})

	// C. Replying to a soft-deleted chat returns 404
	t.Run("Replying to soft-deleted chat returns 404", func(t *testing.T) {
		body := bytes.NewBufferString(`{"balasan":"Balasan ke chat terhapus"}`)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/chat/%d", f.chatDeleted.ID), body)
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 for replying to soft-deleted chat, got %d", w.Code)
		}
	})

	// D. Soft-deleted chats are excluded from GetChatByLaporanID
	t.Run("Soft-deleted chat is omitted from chat list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/warga/laporan/%d/chat", f.lapDesaA.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp struct {
			Data []struct {
				ID uint `json:"id"`
			} `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		for _, c := range resp.Data {
			if c.ID == f.chatDeleted.ID {
				t.Errorf("SOFT-DELETE LEAK: Soft-deleted chat ID %d appeared in chat list", f.chatDeleted.ID)
			}
		}
	})

	// E. Soft-deleted chat or report excluded from admin inbox
	t.Run("Admin inbox excludes soft-deleted reports and chats", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/chat", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokenSA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp struct {
			Data []struct {
				LaporanID uint `json:"laporan_id"`
			} `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		for _, item := range resp.Data {
			if item.LaporanID == f.lapDeleted.ID {
				t.Errorf("SOFT-DELETE LEAK: Soft-deleted report ID %d appeared in inbox", f.lapDeleted.ID)
			}
		}
	})
}

// =========================================================================
// 5. MESSAGE CONTENT VALIDATION
// =========================================================================
func TestBE9_ChatSecurity_MessageContentValidation(t *testing.T) {
	f, cleanup := setupChatSecurityFixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	invalidMessages := []struct {
		name    string
		payload string
	}{
		{"Empty message", `{"pesan":""}`},
		{"Spaces only", `{"pesan":"   "}`},
		{"Whitespace newlines", `{"pesan":"\n\t  \r\n"}`},
		{"Too long > 1000 chars", fmt.Sprintf(`{"pesan":"%s"}`, strings.Repeat("M", 1001))},
		{"Malformed JSON", `{"pesan":`},
	}

	for _, tc := range invalidMessages {
		t.Run("Send chat with "+tc.name+" returns 400", func(t *testing.T) {
			body := bytes.NewBufferString(tc.payload)
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/warga/laporan/%d/chat", f.lapDesaA.ID), body)
			req.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("[%s] expected 400 Bad Request, got %d: %s", tc.name, w.Code, w.Body.String())
			}
		})
	}

	invalidReplies := []struct {
		name    string
		payload string
	}{
		{"Empty reply", `{"balasan":""}`},
		{"Spaces only", `{"balasan":"    "}`},
		{"Too long > 1000 chars", fmt.Sprintf(`{"balasan":"%s"}`, strings.Repeat("R", 1001))},
		{"Malformed JSON", `{"balasan":`},
	}

	for _, tc := range invalidReplies {
		t.Run("Reply chat with "+tc.name+" returns 400", func(t *testing.T) {
			body := bytes.NewBufferString(tc.payload)
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/chat/%d", f.chatDesaA.ID), body)
			req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("[%s] expected 400 Bad Request, got %d: %s", tc.name, w.Code, w.Body.String())
			}
		})
	}
}

// =========================================================================
// 6. ATTACHMENT & FILE SECURITY IN CHAT REPLY
// =========================================================================
func createMultipartChatRequest(url, balasan, filename string, fileBytes []byte) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if balasan != "" {
		_ = writer.WriteField("balasan", balasan)
	}

	if filename != "" {
		part, err := writer.CreateFormFile("lampiran", filename)
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(part, bytes.NewReader(fileBytes)); err != nil {
			return nil, err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	req := httptest.NewRequest(http.MethodPut, url, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

func TestBE9_ChatSecurity_AttachmentValidation(t *testing.T) {
	f, cleanup := setupChatSecurityFixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	replyURL := fmt.Sprintf("/api/admin/chat/%d", f.chatDesaA.ID)

	// A. Oversized attachment (> 5MB)
	t.Run("Oversized attachment > 5MB is rejected with 400", func(t *testing.T) {
		oversizedBytes := make([]byte, 5*1024*1024+10)
		req, err := createMultipartChatRequest(replyURL, "Balasan teks", "large.jpg", oversizedBytes)
		if err != nil {
			t.Fatalf("create request error: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for oversized attachment, got %d", w.Code)
		}
	})

	// B. Disallowed extension (.exe, .pdf, .txt)
	t.Run("Disallowed extension (.pdf) is rejected with 400", func(t *testing.T) {
		req, err := createMultipartChatRequest(replyURL, "Balasan teks", "document.pdf", []byte("%PDF-1.4 file content"))
		if err != nil {
			t.Fatalf("create request error: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for disallowed extension, got %d", w.Code)
		}
	})

	// C. Fake extension (text content named .jpg)
	t.Run("Fake extension (text named .jpg) is rejected with 400", func(t *testing.T) {
		req, err := createMultipartChatRequest(replyURL, "Balasan teks", "fake.jpg", []byte("Plain text spoofing image"))
		if err != nil {
			t.Fatalf("create request error: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for fake image extension, got %d", w.Code)
		}
	})

	// D. Zero-byte empty file
	t.Run("Zero-byte empty file is rejected with 400", func(t *testing.T) {
		req, err := createMultipartChatRequest(replyURL, "Balasan teks", "empty.jpg", []byte{})
		if err != nil {
			t.Fatalf("create request error: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for zero-byte file, got %d", w.Code)
		}
	})

	// E. Corrupt binary data with .png extension
	t.Run("Corrupt binary with .png extension is rejected with 400", func(t *testing.T) {
		req, err := createMultipartChatRequest(replyURL, "Balasan teks", "corrupt.png", []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55})
		if err != nil {
			t.Fatalf("create request error: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+f.tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for corrupt binary, got %d", w.Code)
		}
	})
}

// =========================================================================
// 7. NOTIFICATION RECIPIENT ISOLATION
// =========================================================================
func TestBE9_ChatSecurity_NotificationIsolation(t *testing.T) {
	f, cleanup := setupChatSecurityFixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// Clean existing notifications
	config.DB.Where("1 = 1").Delete(&models.Notifikasi{})

	// 1. Warga sends message on Desa A report
	t.Run("Warga message on Desa report creates notification only for Pemdes Desa A", func(t *testing.T) {
		body := bytes.NewBufferString(`{"pesan":"Lampu jalan di desa A juga mati"}`)
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/warga/laporan/%d/chat", f.lapDesaA.ID), body)
		req.Header.Set("Authorization", "Bearer "+f.tokenWargaA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var notifs []models.Notifikasi
		config.DB.Where("laporan_id = ?", f.lapDesaA.ID).Find(&notifs)

		if len(notifs) == 0 {
			t.Errorf("expected at least 1 notification generated for Pemdes Desa A")
		}

		for _, n := range notifs {
			if n.UserID != f.pemdesA.ID {
				t.Errorf("NOTIFICATION RECIPIENT LEAK: Notification sent to UserID %d instead of Pemdes A (%d)", n.UserID, f.pemdesA.ID)
			}
			if n.UserID == f.wargaA.ID {
				t.Errorf("NOTIFICATION RECIPIENT LEAK: Notification sent back to sender Warga A")
			}
			if n.UserID == f.pemdesB.ID {
				t.Errorf("NOTIFICATION RECIPIENT LEAK: Notification sent to foreign Pemdes B")
			}
			if n.UserID == f.adminPU.ID {
				t.Errorf("NOTIFICATION RECIPIENT LEAK: Notification sent to Admin PU on Desa report")
			}
		}
	})

	// 2. Admin Pemdes replies to Desa A chat
	t.Run("Admin Pemdes reply creates notification only for Warga A (report owner)", func(t *testing.T) {
		config.DB.Where("1 = 1").Delete(&models.Notifikasi{})

		body := bytes.NewBufferString(`{"balasan":"Sudah dikoordinasikan dengan PLN dan RT"}`)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/chat/%d", f.chatDesaA.ID), body)
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
			t.Errorf("expected notification generated for report owner Warga A")
		}

		for _, n := range notifs {
			if n.UserID != f.wargaA.ID {
				t.Errorf("NOTIFICATION RECIPIENT LEAK: Notification sent to UserID %d instead of report owner (%d)", n.UserID, f.wargaA.ID)
			}
			if n.UserID == f.pemdesA.ID {
				t.Errorf("NOTIFICATION RECIPIENT LEAK: Notification sent back to replying admin")
			}
			if n.UserID == f.wargaB.ID {
				t.Errorf("NOTIFICATION RECIPIENT LEAK: Notification sent to random citizen Warga B")
			}
		}
	})
}

// =========================================================================
// 8. FAIL-CLOSED BEHAVIOR & DEFENSE-IN-DEPTH
// =========================================================================
func TestBE9_ChatSecurity_FailClosedBehavior(t *testing.T) {
	f, cleanup := setupChatSecurityFixtures(t)
	defer cleanup()

	r := gin.New()
	routes.SetupRoutes(r)

	// A. Admin Pemdes without assigned wilayah
	t.Run("Admin Pemdes without Wilayah fails closed with 403 on inbox", func(t *testing.T) {
		pemdesNoWilayah := models.User{
			Name:         "Pemdes Tanpa Wilayah",
			Email:        fmt.Sprintf("no_wilayah_%d@test.id", time.Now().UnixNano()),
			Role:         models.RoleAdminPemdes,
			WilayahID:    nil,
			TokenVersion: 1,
		}
		config.DB.Create(&pemdesNoWilayah)
		defer config.DB.Unscoped().Delete(&pemdesNoWilayah)

		token, _ := utils.GenerateToken(pemdesNoWilayah.ID, pemdesNoWilayah.Email, pemdesNoWilayah.Role, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/admin/chat", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for Pemdes without Wilayah accessing inbox, got %d", w.Code)
		}
	})

	// B. CekAksesLaporan fails closed on zero-value report ID
	t.Run("CekAksesLaporan fails closed on zero-value report struct", func(t *testing.T) {
		zeroReport := models.LaporanKerusakan{}
		if utils.CekAksesLaporan(string(models.RoleWarga), f.wargaA.ID, zeroReport) {
			t.Errorf("FAIL-CLOSED BREACH: CekAksesLaporan returned true for zero-value report on Warga")
		}
		if utils.CekAksesLaporan(string(models.RoleAdminPemdes), f.pemdesA.ID, zeroReport) {
			t.Errorf("FAIL-CLOSED BREACH: CekAksesLaporan returned true for zero-value report on Pemdes")
		}
		if utils.CekAksesLaporan(string(models.RoleAdminPu), f.adminPU.ID, zeroReport) {
			t.Errorf("FAIL-CLOSED BREACH: CekAksesLaporan returned true for zero-value report on PU")
		}
		if utils.CekAksesLaporan(string(models.RoleSuperAdmin), f.superAdmin.ID, zeroReport) {
			t.Errorf("FAIL-CLOSED BREACH: CekAksesLaporan returned true for zero-value report on Superadmin")
		}
	})

	// C. CekAksesLaporan fails closed on zero-value user ID
	t.Run("CekAksesLaporan fails closed on userID 0", func(t *testing.T) {
		if utils.CekAksesLaporan(string(models.RoleWarga), 0, f.lapDesaA) {
			t.Errorf("FAIL-CLOSED BREACH: CekAksesLaporan returned true for userID 0 on Warga")
		}
		if utils.CekAksesLaporan(string(models.RoleAdminPemdes), 0, f.lapDesaA) {
			t.Errorf("FAIL-CLOSED BREACH: CekAksesLaporan returned true for userID 0 on Pemdes")
		}
	})
}

// =========================================================================
// 9. STANDALONE SECURITY & VALIDATION SUITE (Zero-dependency & High-Speed)
// =========================================================================
func TestBE9_ChatSecurity_StandaloneSuite(t *testing.T) {
	origDB := config.DB
	config.DB = nil
	defer func() { config.DB = origDB }()

	_ = os.Setenv("JWT_SECRET", "be9_standalone_test_secret_key_1234")
	secret := []byte("be9_standalone_test_secret_key_1234")

	r := gin.New()
	routes.SetupRoutes(r)

	wargaToken, _ := utils.GenerateToken(10, "warga@roadis.local", models.RoleWarga, nil)
	pemdesToken, _ := utils.GenerateToken(20, "pemdes@roadis.local", models.RoleAdminPemdes, nil)
	puToken, _ := utils.GenerateToken(30, "pu@roadis.local", models.RoleAdminPu, nil)
	superToken, _ := utils.GenerateToken(1, "sa@roadis.local", models.RoleSuperAdmin, nil)

	// 1. Authentication boundaries
	t.Run("Auth_MissingTokenReturns401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/warga/laporan/1/chat", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("Auth_InvalidSignatureReturns401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/warga/laporan/1/chat", nil)
		req.Header.Set("Authorization", "Bearer invalid.signature.token")
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

		req := httptest.NewRequest(http.MethodGet, "/api/warga/laporan/1/chat", nil)
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
			Email:  "intruder@roadis.local",
			Role:   "anonymous_hacker",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tStr, _ := tok.SignedString(secret)

		req := httptest.NewRequest(http.MethodGet, "/api/warga/laporan/1/chat", nil)
		req.Header.Set("Authorization", "Bearer "+tStr)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for unknown role, got %d", w.Code)
		}
	})

	// 2. Cross-role route guards
	t.Run("RoleGuard_WargaAttemptingAdminInboxReturns403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/chat", nil)
		req.Header.Set("Authorization", "Bearer "+wargaToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})

	t.Run("RoleGuard_WargaAttemptingAdminReplyReturns403", func(t *testing.T) {
		body := bytes.NewBufferString(`{"balasan":"Palsu"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/admin/chat/1", body)
		req.Header.Set("Authorization", "Bearer "+wargaToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})

	t.Run("RoleGuard_AdminAttemptingSendPesanWargaReturns403", func(t *testing.T) {
		body := bytes.NewBufferString(`{"pesan":"Pesan warga dari pemdes"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/warga/laporan/1/chat", body)
		req.Header.Set("Authorization", "Bearer "+pemdesToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})

	// 3. ID Validation
	malformedIDs := []string{"abc", "-1", "0", "%201%20", "99999999999999999999999999999999999"}
	for _, idStr := range malformedIDs {
		t.Run("IDValidation_ReadChat_"+idStr, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/warga/laporan/"+idStr+"/chat", nil)
			req.Header.Set("Authorization", "Bearer "+wargaToken)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("[%s] expected 400 Bad Request, got %d", idStr, w.Code)
			}
		})

		t.Run("IDValidation_SendChat_"+idStr, func(t *testing.T) {
			body := bytes.NewBufferString(`{"pesan":"Testing malformed ID"}`)
			req := httptest.NewRequest(http.MethodPost, "/api/warga/laporan/"+idStr+"/chat", body)
			req.Header.Set("Authorization", "Bearer "+wargaToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("[%s] expected 400 Bad Request, got %d", idStr, w.Code)
			}
		})

		t.Run("IDValidation_ReplyChat_"+idStr, func(t *testing.T) {
			body := bytes.NewBufferString(`{"balasan":"Testing malformed ID"}`)
			req := httptest.NewRequest(http.MethodPut, "/api/admin/chat/"+idStr, body)
			req.Header.Set("Authorization", "Bearer "+pemdesToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("[%s] expected 400 Bad Request, got %d", idStr, w.Code)
			}
		})
	}

	// 4. Message validation (SendPesanWarga)
	invalidWargaMessages := []struct {
		name    string
		payload string
	}{
		{"Empty pesan", `{"pesan":""}`},
		{"Whitespace spaces", `{"pesan":"   "}`},
		{"Whitespace newlines", `{"pesan":"\n\t  \r\n"}`},
		{"Too long > 1000 chars", fmt.Sprintf(`{"pesan":"%s"}`, strings.Repeat("M", 1001))},
		{"Malformed JSON", `{"pesan":`},
	}
	for _, tc := range invalidWargaMessages {
		t.Run("MessageValidation_Warga_"+tc.name, func(t *testing.T) {
			body := bytes.NewBufferString(tc.payload)
			req := httptest.NewRequest(http.MethodPost, "/api/warga/laporan/1/chat", body)
			req.Header.Set("Authorization", "Bearer "+wargaToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("[%s] expected 400 Bad Request, got %d: %s", tc.name, w.Code, w.Body.String())
			}
		})
	}

	// 5. Fail-Closed CekAksesLaporan Matrix
	t.Run("FailClosed_ZeroValueReport", func(t *testing.T) {
		zeroReport := models.LaporanKerusakan{}
		if utils.CekAksesLaporan(string(models.RoleWarga), 10, zeroReport) {
			t.Errorf("CekAksesLaporan must reject zero-value report for Warga")
		}
		if utils.CekAksesLaporan(string(models.RoleAdminPemdes), 20, zeroReport) {
			t.Errorf("CekAksesLaporan must reject zero-value report for Pemdes")
		}
		if utils.CekAksesLaporan(string(models.RoleAdminPu), 30, zeroReport) {
			t.Errorf("CekAksesLaporan must reject zero-value report for PU")
		}
		if utils.CekAksesLaporan(string(models.RoleSuperAdmin), 1, zeroReport) {
			t.Errorf("CekAksesLaporan must reject zero-value report for Superadmin")
		}
	})

	t.Run("FailClosed_ZeroUserID", func(t *testing.T) {
		validReport := models.LaporanKerusakan{Model: gorm.Model{ID: 1}, JenisJalan: "desa", WilayahID: 100, UserID: 10}
		if utils.CekAksesLaporan(string(models.RoleWarga), 0, validReport) {
			t.Errorf("CekAksesLaporan must reject userID 0 for Warga")
		}
		if utils.CekAksesLaporan(string(models.RoleAdminPemdes), 0, validReport) {
			t.Errorf("CekAksesLaporan must reject userID 0 for Pemdes")
		}
		if utils.CekAksesLaporan(string(models.RoleAdminPu), 0, validReport) {
			t.Errorf("CekAksesLaporan must reject userID 0 for PU")
		}
		if utils.CekAksesLaporan(string(models.RoleSuperAdmin), 0, validReport) {
			t.Errorf("CekAksesLaporan must reject userID 0 for Superadmin")
		}
	})

	t.Run("FailClosed_SoftDeletedReport", func(t *testing.T) {
		deletedReport := models.LaporanKerusakan{
			Model:      gorm.Model{ID: 1, DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}},
			JenisJalan: "desa",
			WilayahID:  100,
			UserID:     10,
		}
		if utils.CekAksesLaporan(string(models.RoleWarga), 10, deletedReport) {
			t.Errorf("CekAksesLaporan must reject soft-deleted report for Warga")
		}
		if utils.CekAksesLaporan(string(models.RoleAdminPemdes), 20, deletedReport) {
			t.Errorf("CekAksesLaporan must reject soft-deleted report for Pemdes")
		}
		if utils.CekAksesLaporan(string(models.RoleAdminPu), 30, deletedReport) {
			t.Errorf("CekAksesLaporan must reject soft-deleted report for PU")
		}
		if utils.CekAksesLaporan(string(models.RoleSuperAdmin), 1, deletedReport) {
			t.Errorf("CekAksesLaporan must reject soft-deleted report for Superadmin")
		}
	})

	// 6. Role Scope Matrix in CekAksesLaporan
	t.Run("RoleScope_WargaOwnership", func(t *testing.T) {
		lapWargaA := models.LaporanKerusakan{Model: gorm.Model{ID: 10}, UserID: 100, JenisJalan: "desa", WilayahID: 1}
		if !utils.CekAksesLaporan(string(models.RoleWarga), 100, lapWargaA) {
			t.Errorf("Warga owner must have access")
		}
		if utils.CekAksesLaporan(string(models.RoleWarga), 200, lapWargaA) {
			t.Errorf("Warga non-owner must be rejected (IDOR vulnerability)")
		}
	})

	t.Run("RoleScope_AdminPU_StrictKabupatenOnly", func(t *testing.T) {
		lapKab := models.LaporanKerusakan{Model: gorm.Model{ID: 20}, UserID: 100, JenisJalan: "kabupaten", WilayahID: 1}
		lapDesa := models.LaporanKerusakan{Model: gorm.Model{ID: 21}, UserID: 100, JenisJalan: "desa", WilayahID: 1}
		lapProv := models.LaporanKerusakan{Model: gorm.Model{ID: 22}, UserID: 100, JenisJalan: "provinsi", WilayahID: 1}
		lapNas := models.LaporanKerusakan{Model: gorm.Model{ID: 23}, UserID: 100, JenisJalan: "nasional", WilayahID: 1}

		if !utils.CekAksesLaporan(string(models.RoleAdminPu), 30, lapKab) {
			t.Errorf("PU must access kabupaten road chat")
		}
		if utils.CekAksesLaporan(string(models.RoleAdminPu), 30, lapDesa) {
			t.Errorf("PU must NOT access desa road chat")
		}
		if utils.CekAksesLaporan(string(models.RoleAdminPu), 30, lapProv) {
			t.Errorf("PU must NOT access provinsi road chat")
		}
		if utils.CekAksesLaporan(string(models.RoleAdminPu), 30, lapNas) {
			t.Errorf("PU must NOT access nasional road chat")
		}
	})

	t.Run("RoleScope_Superadmin_AllRoads", func(t *testing.T) {
		roads := []string{"desa", "kabupaten", "provinsi", "nasional"}
		for i, r := range roads {
			lap := models.LaporanKerusakan{Model: gorm.Model{ID: uint(30 + i)}, UserID: 100, JenisJalan: r, WilayahID: 1}
			if !utils.CekAksesLaporan(string(models.RoleSuperAdmin), 1, lap) {
				t.Errorf("Superadmin must access chat for road %s", r)
			}
		}
	})

	// 7. Notification helper safety on edge cases
	t.Run("Notification_NilSafetyAndZeroGuards", func(t *testing.T) {
		// Calling with zero ID or nil DB must not panic
		controllers.KirimNotifikasiChatWarga(models.LaporanKerusakan{})
		controllers.KirimNotifikasiBalasanAdmin(models.LaporanKerusakan{})
	})

	// 8. Attachment validation directly via utils.ValidateImageFile
	t.Run("Attachment_ValidAndInvalidFormats", func(t *testing.T) {
		validJPG := []byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00\x01\x01\x01\x00`\x00`\x00\x00\xFF\xDB\x00C\x00")
		validPNG := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15c4")
		validWEBP := []byte("RIFF\x1a\x00\x00\x00WEBPVP8 \x0e\x00\x00\x00\x30\x01\x00\x9d\x01\x2a\x01\x00\x01\x00\x02\x00\x34\x25")
		plainText := []byte("This is text")
		pdfBytes := []byte("%PDF-1.4 header")
		corruptBin := []byte{0x00, 0x11, 0x22, 0x33, 0x44}

		testCases := []struct {
			name        string
			filename    string
			content     []byte
			expectError bool
		}{
			{"Valid JPEG", "bukti.jpg", validJPG, false},
			{"Valid PNG", "bukti.png", validPNG, false},
			{"Valid WEBP", "bukti.webp", validWEBP, false},
			{"Disallowed PDF extension", "dokumen.pdf", pdfBytes, true},
			{"Fake JPEG extension", "fake.jpg", plainText, true},
			{"Corrupt binary as PNG", "corrupt.png", corruptBin, true},
			{"Empty zero-byte file", "empty.jpg", []byte{}, true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)
				part, _ := writer.CreateFormFile("lampiran", tc.filename)
				_, _ = part.Write(tc.content)
				writer.Close()

				req := httptest.NewRequest(http.MethodPost, "/", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				_ = req.ParseMultipartForm(int64(len(tc.content) + 4096))

				var header *multipart.FileHeader
				if req.MultipartForm != nil && len(req.MultipartForm.File["lampiran"]) > 0 {
					header = req.MultipartForm.File["lampiran"][0]
				}

				err := utils.ValidateImageFile(header)
				if (err != nil) != tc.expectError {
					t.Errorf("[%s] expected error=%v, got err=%v", tc.name, tc.expectError, err)
				}
			})
		}
	})

	// Suppress unused variables warnings if any
	_ = puToken
	_ = superToken
}

