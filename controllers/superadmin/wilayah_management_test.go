package superadmin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/middlewares"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
)

func setupWilayahRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	api := r.Group("/api")
	api.Use(middlewares.AuthMiddleware())

	// Warga route
	warga := api.Group("/warga")
	warga.Use(middlewares.RequireRole("warga"))
	{
		warga.GET("/wilayah", GetAllWilayah)
	}

	// Superadmin route
	superadmin := api.Group("/superadmin")
	superadmin.Use(middlewares.RequireRole("super_admin"))
	{
		superadmin.GET("/wilayah", GetAllWilayah)
		superadmin.POST("/wilayah", CreateWilayah)
		superadmin.GET("/wilayah/:id", ShowWilayah)
		superadmin.PUT("/wilayah/:id", UpdateWilayah)
		superadmin.DELETE("/wilayah/:id", DeleteWilayah)
	}
	return r
}

// 1. Authorization Matrix Test for All Roles
func TestWilayahManagement_AuthorizationMatrix_AllRoles(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for Wilayah Management tests")
	}

	t.Setenv("JWT_SECRET", "superadmin_test_secret_key_1234567890")
	r := setupWilayahRouter()

	now := time.Now().UnixNano()
	superadminUser := models.User{
		Name:         "Superadmin Wilayah Auth",
		Email:        fmt.Sprintf("superwil_%d@roadis.local", now),
		Password:     "hashedpassword123",
		Role:         models.RoleSuperAdmin,
		TokenVersion: 1,
	}
	config.DB.Create(&superadminUser)
	defer config.DB.Unscoped().Delete(&superadminUser)

	adminPuUser := models.User{
		Name:         "Admin PU Wilayah Auth",
		Email:        fmt.Sprintf("puwil_%d@roadis.local", now),
		Password:     "hashedpassword123",
		Role:         models.RoleAdminPu,
		TokenVersion: 1,
	}
	config.DB.Create(&adminPuUser)
	defer config.DB.Unscoped().Delete(&adminPuUser)

	adminPemdesUser := models.User{
		Name:         "Admin Pemdes Wilayah Auth",
		Email:        fmt.Sprintf("pemdeswil_%d@roadis.local", now),
		Password:     "hashedpassword123",
		Role:         models.RoleAdminPemdes,
		TokenVersion: 1,
	}
	config.DB.Create(&adminPemdesUser)
	defer config.DB.Unscoped().Delete(&adminPemdesUser)

	wargaUser := models.User{
		Name:         "Warga Wilayah Auth",
		Email:        fmt.Sprintf("wargawil_%d@roadis.local", now),
		Password:     "hashedpassword123",
		Role:         models.RoleWarga,
		TokenVersion: 1,
	}
	config.DB.Create(&wargaUser)
	defer config.DB.Unscoped().Delete(&wargaUser)

	tokenSuper, _ := utils.GenerateToken(superadminUser.ID, superadminUser.Email, superadminUser.Role, nil, 1)
	tokenPU, _ := utils.GenerateToken(adminPuUser.ID, adminPuUser.Email, adminPuUser.Role, nil, 1)
	tokenPemdes, _ := utils.GenerateToken(adminPemdesUser.ID, adminPemdesUser.Email, adminPemdesUser.Role, nil, 1)
	tokenWarga, _ := utils.GenerateToken(wargaUser.ID, wargaUser.Email, wargaUser.Role, nil, 1)

	// A. Superadmin -> 200 OK
	t.Run("Superadmin allowed GET /api/superadmin/wilayah", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/superadmin/wilayah", nil)
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	// B. Admin PU -> 403 Forbidden
	t.Run("Admin PU forbidden on /api/superadmin/wilayah", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/superadmin/wilayah", nil)
		req.Header.Set("Authorization", "Bearer "+tokenPU)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for Admin PU, got %d", w.Code)
		}
	})

	// C. Admin Pemdes -> 403 Forbidden
	t.Run("Admin Pemdes forbidden on /api/superadmin/wilayah", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/superadmin/wilayah", nil)
		req.Header.Set("Authorization", "Bearer "+tokenPemdes)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for Admin Pemdes, got %d", w.Code)
		}
	})

	// D. Warga forbidden on superadmin route -> 403 Forbidden
	t.Run("Warga forbidden on /api/superadmin/wilayah", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/superadmin/wilayah", nil)
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for Warga on superadmin route, got %d", w.Code)
		}
	})

	// E. Warga allowed on /api/warga/wilayah -> 200 OK
	t.Run("Warga allowed on /api/warga/wilayah", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/warga/wilayah", nil)
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK for Warga on /api/warga/wilayah, got %d: %s", w.Code, w.Body.String())
		}
	})

	// F. Unauthenticated -> 401 Unauthorized
	t.Run("Unauthenticated rejected with 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/superadmin/wilayah", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized without token, got %d", w.Code)
		}
	})
}

// 2. Create Wilayah Validation & Security
func TestWilayahManagement_CreateWilayah_ValidationAndSecurity(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for Wilayah Management tests")
	}

	t.Setenv("JWT_SECRET", "superadmin_test_secret_key_1234567890")
	r := setupWilayahRouter()

	now := time.Now().UnixNano()
	superadminUser := models.User{
		Name:         "Superadmin Creator Wilayah",
		Email:        fmt.Sprintf("supercreator_%d@roadis.local", now),
		Password:     "hashedpassword123",
		Role:         models.RoleSuperAdmin,
		TokenVersion: 1,
	}
	config.DB.Create(&superadminUser)
	defer config.DB.Unscoped().Delete(&superadminUser)

	tokenSuper, _ := utils.GenerateToken(superadminUser.ID, superadminUser.Email, superadminUser.Role, nil, 1)

	// A. Valid Create
	t.Run("Valid Wilayah Creation returns 201", func(t *testing.T) {
		namaWilayah := fmt.Sprintf("Desa Anyar %d", time.Now().UnixNano())
		payload := map[string]string{
			"nama": namaWilayah,
			"tipe": "desa",
		}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/superadmin/wilayah", bytes.NewBuffer(b))
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
		}

		var created models.Wilayah
		if err := config.DB.Where("nama = ?", namaWilayah).First(&created).Error; err != nil {
			t.Fatalf("created wilayah not found in DB: %v", err)
		}
		defer config.DB.Unscoped().Delete(&created)

		if created.Tipe != "desa" {
			t.Errorf("expected tipe 'desa', got '%s'", created.Tipe)
		}
	})

	// B. Duplicate Name + Tipe
	t.Run("Duplicate nama and tipe rejected with 400", func(t *testing.T) {
		existing := models.Wilayah{
			Nama: fmt.Sprintf("Desa Duplikat %d", time.Now().UnixNano()),
			Tipe: "desa",
		}
		config.DB.Create(&existing)
		defer config.DB.Unscoped().Delete(&existing)

		// Coba buat dengan nama dan tipe yang sama persis
		payload := map[string]string{
			"nama": existing.Nama,
			"tipe": "desa",
		}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/superadmin/wilayah", bytes.NewBuffer(b))
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for duplicate wilayah, got %d: %s", w.Code, w.Body.String())
		}

		// Coba case-insensitive duplicate (huruf kapital / kecil)
		payloadLower := map[string]string{
			"nama": strings.ToLower(existing.Nama),
			"tipe": "DESA",
		}
		bLower, _ := json.Marshal(payloadLower)
		reqLower := httptest.NewRequest(http.MethodPost, "/api/superadmin/wilayah", bytes.NewBuffer(bLower))
		reqLower.Header.Set("Authorization", "Bearer "+tokenSuper)
		reqLower.Header.Set("Content-Type", "application/json")
		wLower := httptest.NewRecorder()
		r.ServeHTTP(wLower, reqLower)

		if wLower.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for case-insensitive duplicate wilayah, got %d", wLower.Code)
		}
	})

	// C. Invalid Tipe
	t.Run("Invalid tipe rejected with 400", func(t *testing.T) {
		payload := map[string]string{
			"nama": "Wilayah Tipe Ngawur",
			"tipe": "kecamatan_tidak_valid",
		}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/superadmin/wilayah", bytes.NewBuffer(b))
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid tipe, got %d: %s", w.Code, w.Body.String())
		}
	})

	// D. Empty Name
	t.Run("Empty or whitespace nama rejected with 400", func(t *testing.T) {
		payload := map[string]string{
			"nama": "   ",
			"tipe": "desa",
		}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/superadmin/wilayah", bytes.NewBuffer(b))
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for whitespace nama, got %d: %s", w.Code, w.Body.String())
		}
	})

	// E. Oversized Name (> 250 chars)
	t.Run("Oversized nama (> 250 chars) rejected with 400", func(t *testing.T) {
		longName := strings.Repeat("a", 251)
		payload := map[string]string{
			"nama": longName,
			"tipe": "desa",
		}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/superadmin/wilayah", bytes.NewBuffer(b))
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for oversized nama, got %d", w.Code)
		}
	})
}

// 3. Read Wilayah (List and Detail)
func TestWilayahManagement_ReadWilayah_ListAndDetail(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for Wilayah Management tests")
	}

	t.Setenv("JWT_SECRET", "superadmin_test_secret_key_1234567890")
	r := setupWilayahRouter()

	now := time.Now().UnixNano()
	superadminUser := models.User{
		Name:         "Superadmin Reader Wilayah",
		Email:        fmt.Sprintf("superread_%d@roadis.local", now),
		Password:     "hashedpassword123",
		Role:         models.RoleSuperAdmin,
		TokenVersion: 1,
	}
	config.DB.Create(&superadminUser)
	defer config.DB.Unscoped().Delete(&superadminUser)

	targetWilayah := models.Wilayah{
		Nama: fmt.Sprintf("Desa Target Baca %d", now),
		Tipe: "desa",
	}
	config.DB.Create(&targetWilayah)
	defer config.DB.Unscoped().Delete(&targetWilayah)

	tokenSuper, _ := utils.GenerateToken(superadminUser.ID, superadminUser.Email, superadminUser.Role, nil, 1)

	// A. List Wilayah: Array format
	t.Run("GET /api/superadmin/wilayah returns array", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/superadmin/wilayah", nil)
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		if !strings.Contains(w.Body.String(), targetWilayah.Nama) {
			t.Errorf("target wilayah not found in list response: %s", w.Body.String())
		}
	})

	// B. Detail Wilayah: Show
	t.Run("GET /api/superadmin/wilayah/:id returns detail", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/superadmin/wilayah/%d", targetWilayah.ID), nil)
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		if !strings.Contains(w.Body.String(), targetWilayah.Nama) {
			t.Errorf("target wilayah nama missing from detail response: %s", w.Body.String())
		}
	})

	// C. Nonexistent Wilayah returns 404
	t.Run("GET nonexistent wilayah returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/superadmin/wilayah/99999999", nil)
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})
}

// 4. Update Wilayah Validation
func TestWilayahManagement_UpdateWilayah_Validation(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for Wilayah Management tests")
	}

	t.Setenv("JWT_SECRET", "superadmin_test_secret_key_1234567890")
	r := setupWilayahRouter()

	now := time.Now().UnixNano()
	superadminUser := models.User{
		Name:         "Superadmin Updater Wilayah",
		Email:        fmt.Sprintf("superupd_%d@roadis.local", now),
		Password:     "hashedpassword123",
		Role:         models.RoleSuperAdmin,
		TokenVersion: 1,
	}
	config.DB.Create(&superadminUser)
	defer config.DB.Unscoped().Delete(&superadminUser)

	wilayah1 := models.Wilayah{
		Nama: fmt.Sprintf("Desa Wilayah Satu %d", now),
		Tipe: "desa",
	}
	config.DB.Create(&wilayah1)
	defer config.DB.Unscoped().Delete(&wilayah1)

	wilayah2 := models.Wilayah{
		Nama: fmt.Sprintf("Desa Wilayah Dua %d", now),
		Tipe: "desa",
	}
	config.DB.Create(&wilayah2)
	defer config.DB.Unscoped().Delete(&wilayah2)

	tokenSuper, _ := utils.GenerateToken(superadminUser.ID, superadminUser.Email, superadminUser.Role, nil, 1)

	// A. Valid Update
	t.Run("Valid update modifies name and tipe", func(t *testing.T) {
		newNama := fmt.Sprintf("Desa Wilayah Terupdate %d", time.Now().UnixNano())
		payload := map[string]string{
			"nama": newNama,
			"tipe": "kabupaten",
		}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/superadmin/wilayah/%d", wilayah1.ID), bytes.NewBuffer(b))
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var check models.Wilayah
		config.DB.First(&check, wilayah1.ID)
		if check.Nama != newNama || check.Tipe != "kabupaten" {
			t.Errorf("wilayah was not properly updated in DB: %+v", check)
		}
	})

	// B. Duplicate Update (collision with existing wilayah2)
	t.Run("Updating to duplicate name and tipe on another record rejected with 400", func(t *testing.T) {
		payload := map[string]string{
			"nama": wilayah2.Nama,
			"tipe": wilayah2.Tipe,
		}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/superadmin/wilayah/%d", wilayah1.ID), bytes.NewBuffer(b))
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for duplicate update, got %d: %s", w.Code, w.Body.String())
		}
	})

	// C. Update Nonexistent Wilayah returns 404
	t.Run("Update nonexistent wilayah returns 404", func(t *testing.T) {
		payload := map[string]string{
			"nama": "Gak Ada",
			"tipe": "desa",
		}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPut, "/api/superadmin/wilayah/99999999", bytes.NewBuffer(b))
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})
}

// 5. Delete Wilayah & Dependency Protections
func TestWilayahManagement_DeleteWilayah_IntegrityAndProtections(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for Wilayah Management tests")
	}

	t.Setenv("JWT_SECRET", "superadmin_test_secret_key_1234567890")
	r := setupWilayahRouter()

	now := time.Now().UnixNano()
	superadminUser := models.User{
		Name:         "Superadmin Deleter Wilayah",
		Email:        fmt.Sprintf("superdelwil_%d@roadis.local", now),
		Password:     "hashedpassword123",
		Role:         models.RoleSuperAdmin,
		TokenVersion: 1,
	}
	config.DB.Create(&superadminUser)
	defer config.DB.Unscoped().Delete(&superadminUser)

	tokenSuper, _ := utils.GenerateToken(superadminUser.ID, superadminUser.Email, superadminUser.Role, nil, 1)

	// A. Deletion blocked when used by active User
	t.Run("Deletion blocked when wilayah is assigned to active User -> 400", func(t *testing.T) {
		wilayahUser := models.Wilayah{
			Nama: fmt.Sprintf("Desa Terpakai User %d", time.Now().UnixNano()),
			Tipe: "desa",
		}
		config.DB.Create(&wilayahUser)
		defer config.DB.Unscoped().Delete(&wilayahUser)

		pemdes := models.User{
			Name:         "Admin Pemdes Dependent",
			Email:        fmt.Sprintf("pemdesdep_%d@roadis.local", time.Now().UnixNano()),
			Password:     "hashedpassword123",
			Role:         models.RoleAdminPemdes,
			WilayahID:    &wilayahUser.ID,
			TokenVersion: 1,
		}
		config.DB.Create(&pemdes)
		defer config.DB.Unscoped().Delete(&pemdes)

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/superadmin/wilayah/%d", wilayahUser.ID), nil)
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 when deleting wilayah assigned to user, got %d: %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "digunakan") {
			t.Errorf("expected dependency error message, got: %s", w.Body.String())
		}
	})

	// B. Deletion blocked when used by active Laporan
	t.Run("Deletion blocked when wilayah is assigned to active Laporan -> 400", func(t *testing.T) {
		wilayahLap := models.Wilayah{
			Nama: fmt.Sprintf("Desa Terpakai Laporan %d", time.Now().UnixNano()),
			Tipe: "desa",
		}
		config.DB.Create(&wilayahLap)
		defer config.DB.Unscoped().Delete(&wilayahLap)

		laporan := models.LaporanKerusakan{
			UserID:        superadminUser.ID,
			WilayahID:     wilayahLap.ID,
			Judul:         "Jalan Rusak Dep",
			Deskripsi:     "Deskripsi",
			Latitude:      -6.33,
			Longitude:     108.33,
			ImageURL:      "https://example.com/foto.jpg",
			TipeKerusakan: "retak",
			Status:        "menunggu",
		}
		config.DB.Create(&laporan)
		defer config.DB.Unscoped().Delete(&laporan)

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/superadmin/wilayah/%d", wilayahLap.ID), nil)
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 when deleting wilayah assigned to laporan, got %d: %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "digunakan") {
			t.Errorf("expected dependency error message, got: %s", w.Body.String())
		}
	})

	// C. Valid Deletion of Unused Wilayah (Soft Delete)
	t.Run("Valid delete of unused wilayah soft-deletes and returns 200", func(t *testing.T) {
		wilayahUnused := models.Wilayah{
			Nama: fmt.Sprintf("Desa Kosong %d", time.Now().UnixNano()),
			Tipe: "desa",
		}
		config.DB.Create(&wilayahUnused)
		defer config.DB.Unscoped().Delete(&wilayahUnused)

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/superadmin/wilayah/%d", wilayahUnused.ID), nil)
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for unused wilayah deletion, got %d: %s", w.Code, w.Body.String())
		}

		// Verify record is soft deleted in DB
		var checkActive models.Wilayah
		if err := config.DB.Where("id = ? AND deleted_at IS NULL", wilayahUnused.ID).First(&checkActive).Error; err == nil {
			t.Errorf("wilayah must be soft-deleted and not found with deleted_at IS NULL")
		}

		// Verify subsequent GET returns 404
		reqGet := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/superadmin/wilayah/%d", wilayahUnused.ID), nil)
		reqGet.Header.Set("Authorization", "Bearer "+tokenSuper)
		wGet := httptest.NewRecorder()
		r.ServeHTTP(wGet, reqGet)

		if wGet.Code != http.StatusNotFound {
			t.Errorf("expected 404 for deleted wilayah query, got %d", wGet.Code)
		}
	})

	// D. Deleting Nonexistent Wilayah returns 404
	t.Run("Deleting nonexistent wilayah returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/superadmin/wilayah/99999999", nil)
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})
}
