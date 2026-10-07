package warga

import (
	"encoding/json"
	"math"
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
	"gorm.io/gorm"
)

// filterWargaMapSimulasi mensimulasikan query dan filter koordinat pada GetAllLaporanPeta
func filterWargaMapSimulasi(laporanList []models.LaporanKerusakan) []LaporanResponse {
	responseData := make([]LaporanResponse, 0)

	for _, lap := range laporanList {
		// Soft deleted tidak boleh muncul
		if lap.DeletedAt.Valid {
			continue
		}

		// Koordinat invalid tidak boleh muncul
		if !utils.IsValidCoordinate(lap.Latitude, lap.Longitude) {
			continue
		}

		responseData = append(responseData, FormatLaporanToResponse(lap))
	}

	return responseData
}

func TestWargaMap_ValidAndInvalidFiltering(t *testing.T) {
	laporanList := []models.LaporanKerusakan{
		// 1. Laporan valid -> HARUS MUNCUL
		{
			Model:     gorm.Model{ID: 1},
			Judul:     "Jalan Rusak Desa",
			Latitude:  -6.3265,
			Longitude: 108.3241,
			Status:    "menunggu",
		},
		// 2. Laporan valid kabupaten -> HARUS MUNCUL (Warga melihat peta publik)
		{
			Model:     gorm.Model{ID: 2},
			Judul:     "Jalan Rusak Kabupaten",
			Latitude:  -6.3300,
			Longitude: 108.3300,
			Status:    "proses",
		},
		// 3. Laporan soft-deleted -> TIDAK BOLEH MUNCUL
		{
			Model:     gorm.Model{ID: 3, DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}},
			Judul:     "Laporan Terhapus",
			Latitude:  -6.3265,
			Longitude: 108.3241,
			Status:    "selesai",
		},
		// 4. Laporan dengan Latitude out of range (95.0) -> TIDAK BOLEH MUNCUL
		{
			Model:     gorm.Model{ID: 4},
			Judul:     "Laporan Invalid Lat",
			Latitude:  95.0,
			Longitude: 108.3241,
			Status:    "menunggu",
		},
		// 5. Laporan dengan Longitude out of range (-190.0) -> TIDAK BOLEH MUNCUL
		{
			Model:     gorm.Model{ID: 5},
			Judul:     "Laporan Invalid Lng",
			Latitude:  -6.3265,
			Longitude: -190.0,
			Status:    "menunggu",
		},
		// 6. Laporan dengan Latitude NaN -> TIDAK BOLEH MUNCUL
		{
			Model:     gorm.Model{ID: 6},
			Judul:     "Laporan NaN",
			Latitude:  math.NaN(),
			Longitude: 108.3241,
			Status:    "menunggu",
		},
		// 7. Laporan dengan Longitude Inf -> TIDAK BOLEH MUNCUL
		{
			Model:     gorm.Model{ID: 7},
			Judul:     "Laporan Inf",
			Latitude:  -6.3265,
			Longitude: math.Inf(1),
			Status:    "menunggu",
		},
	}

	result := filterWargaMapSimulasi(laporanList)

	if len(result) != 2 {
		t.Fatalf("expected 2 visible reports for Warga Map, got %d", len(result))
	}

	if result[0].ID != 1 || result[1].ID != 2 {
		t.Errorf("expected IDs 1 and 2, got %d and %d", result[0].ID, result[1].ID)
	}
}

func TestWargaMap_EmptyDatabaseReturnsEmptyArray(t *testing.T) {
	emptyList := []models.LaporanKerusakan{}
	result := filterWargaMapSimulasi(emptyList)

	resp := gin.H{
		"status":  "success",
		"message": "Semua laporan berhasil diambil",
		"data":    result,
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	jsonStr := string(b)
	if !strings.Contains(jsonStr, `"data":[]`) {
		t.Errorf("expected '\"data\":[]' in response, got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"status":"success"`) {
		t.Errorf("expected '\"status\":\"success\"', got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"message":"Semua laporan berhasil diambil"`) {
		t.Errorf("expected message, got: %s", jsonStr)
	}

	// Pastikan tidak ada field AI palsu pada LaporanResponse
	forbiddenFields := []string{"severity", "severity_score", "confidence", "priority", "model_version"}
	for _, field := range forbiddenFields {
		if strings.Contains(jsonStr, `"`+field+`"`) {
			t.Errorf("found forbidden AI field %q in Warga Map JSON: %s", field, jsonStr)
		}
	}
}

// 1. Negative Security Test: Memastikan response serializer peta Warga tidak membocorkan PII/User object
func TestWargaMap_PrivacyHardening_NoSensitiveDataLeakage(t *testing.T) {
	phone := "08123456789"
	avatar := "https://example.com/avatar.jpg"
	lapWithUser := models.LaporanKerusakan{
		Model:         gorm.Model{ID: 10, CreatedAt: time.Now()},
		UserID:        5,
		User: models.User{
			Model:        gorm.Model{ID: 5},
			Name:         "Warga Rahasia",
			Email:        "rahasia@roadis.local",
			Role:         models.RoleWarga,
			Phone:        &phone,
			AvatarURL:    &avatar,
			ProfilePhoto: avatar,
			TokenVersion: 2,
		},
		Judul:         "Jalan Berlubang di Sukaurip",
		Deskripsi:     "Lubang cukup dalam di pertigaan",
		Latitude:      -6.3400,
		Longitude:     108.3300,
		ImageURL:      "https://example.com/jalan.jpg",
		TipeKerusakan: "Lubang",
		Status:        "menunggu",
	}

	respItem := FormatLaporanToResponse(lapWithUser)

	wrapper := gin.H{
		"status":  "success",
		"message": "Semua laporan berhasil diambil",
		"data":    []LaporanResponse{respItem},
	}

	b, err := json.Marshal(wrapper)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	jsonStr := string(b)

	// A. NEGATIVE SECURITY ASSERTIONS: Data pribadi/sensitif MUST NOT EXIST
	forbiddenSensitiveFields := []string{
		`"user"`,
		`"email"`,
		`"password"`,
		`"phone"`,
		`"avatar_url"`,
		`"profile_photo"`,
		`"token_version"`,
		`"deleted_at"`,
		`rahasia@roadis.local`,
		`08123456789`,
		`Warga Rahasia`,
	}

	for _, field := range forbiddenSensitiveFields {
		if strings.Contains(jsonStr, field) {
			t.Errorf("SECURITY LEAK DETECTED: found forbidden sensitive data %s in Warga Map JSON: %s", field, jsonStr)
		}
	}

	// B. FUNCTIONAL ASSERTIONS: Field fungsional peta MUST EXIST
	requiredFunctionalFields := []string{
		`"id":10`,
		`"user_id":5`,
		`"judul":"Jalan Berlubang di Sukaurip"`,
		`"deskripsi":"Lubang cukup dalam di pertigaan"`,
		`"latitude":-6.34`,
		`"longitude":108.33`,
		`"image_url":"https://example.com/jalan.jpg"`,
		`"tipe_kerusakan":"Lubang"`,
		`"status":"menunggu"`,
		`"waktu_laporan"`,
	}

	for _, reqField := range requiredFunctionalFields {
		if !strings.Contains(jsonStr, reqField) {
			t.Errorf("MISSING FUNCTIONAL FIELD: expected %s in Warga Map JSON: %s", reqField, jsonStr)
		}
	}
}

// 2. HTTP Endpoint Privacy & Role Authorization Test
func TestWargaMap_HTTP_PrivacyAndAuthorization(t *testing.T) {
	hasDB := ensureTestDB(t)
	if !hasDB {
		t.Skip("MySQL not available for HTTP integration test")
	}

	t.Setenv("JWT_SECRET", "test_secret_for_warga_map_test_1234567890")
	gin.SetMode(gin.TestMode)

	// Fixture data
	now := time.Now().UnixNano()
	wilayah := models.Wilayah{
		Nama: "Desa Map Test " + utils.FormatTanggalIndo(nil),
		Tipe: "desa",
	}
	config.DB.Create(&wilayah)
	defer config.DB.Unscoped().Delete(&wilayah)

	phone := "08987654321"
	testWarga := models.User{
		Name:     "Pelapor Privacy",
		Email:    "pelapor_privacy@roadis.local",
		Password: "hashedpassword123",
		Role:     models.RoleWarga,
		Phone:    &phone,
	}
	config.DB.Create(&testWarga)
	defer config.DB.Unscoped().Delete(&testWarga)

	testLaporan := models.LaporanKerusakan{
		UserID:        testWarga.ID,
		WilayahID:     wilayah.ID,
		Judul:         "Uji Peta Privasi " + string(rune(now%100)),
		Deskripsi:     "Deskripsi privasi",
		Latitude:      -6.3456,
		Longitude:     108.3344,
		ImageURL:      "https://example.com/test.jpg",
		TipeKerusakan: "retak",
		Status:        "menunggu",
	}
	config.DB.Create(&testLaporan)
	defer config.DB.Unscoped().Delete(&testLaporan)

	r := gin.New()
	api := r.Group("/api")
	api.Use(middlewares.AuthMiddleware())
	wargaGroup := api.Group("/warga")
	wargaGroup.Use(middlewares.RequireRole("warga"))
	wargaGroup.GET("/laporan/peta", GetAllLaporanPeta)

	tokenWarga, err := utils.GenerateToken(testWarga.ID, testWarga.Email, testWarga.Role, nil)
	if err != nil {
		t.Fatalf("failed to generate token warga: %v", err)
	}

	// A. Valid Warga request -> 200 OK & no PII in response
	t.Run("Warga access GET /api/warga/laporan/peta succeeds without PII", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/warga/laporan/peta", nil)
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		body := w.Body.String()

		// Negative security checks on HTTP response
		forbidden := []string{
			`"email"`,
			`"password"`,
			`"phone"`,
			`"avatar_url"`,
			`"profile_photo"`,
			`"token_version"`,
			`pelapor_privacy@roadis.local`,
			`08987654321`,
			`Pelapor Privacy`,
			`"user":{`,
		}
		for _, f := range forbidden {
			if strings.Contains(body, f) {
				t.Errorf("SECURITY LEAK in HTTP response: found %s in %s", f, body)
			}
		}

		// Functional checks on HTTP response
		if !strings.Contains(body, `"status":"success"`) {
			t.Errorf("expected success status, got: %s", body)
		}
		if !strings.Contains(body, `-6.3456`) || !strings.Contains(body, `108.3344`) {
			t.Errorf("expected coordinates in response, got: %s", body)
		}
	})

	// B. Unauthenticated request -> 401 Unauthorized
	t.Run("Unauthenticated request rejected with 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/warga/laporan/peta", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	// C. Non-warga role (e.g. Admin Pemdes) -> 403 Forbidden
	t.Run("Non-warga role rejected with 403", func(t *testing.T) {
		testAdmin := models.User{
			Name:     "Admin Pemdes Test",
			Email:    "admin_pemdes_map_test@roadis.local",
			Password: "hashedpassword123",
			Role:     models.RoleAdminPemdes,
		}
		config.DB.Create(&testAdmin)
		defer config.DB.Unscoped().Delete(&testAdmin)

		tokenAdmin, err := utils.GenerateToken(testAdmin.ID, testAdmin.Email, testAdmin.Role, nil)
		if err != nil {
			t.Fatalf("failed to generate token admin: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/warga/laporan/peta", nil)
		req.Header.Set("Authorization", "Bearer "+tokenAdmin)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %d (body: %s)", w.Code, w.Body.String())
		}
	})
}
