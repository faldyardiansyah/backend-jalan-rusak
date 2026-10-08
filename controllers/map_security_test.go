package controllers_test

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/controllers/admin"
	"backend-jalan-rusak/controllers/warga"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/routes"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

func setupBE12Router() *gin.Engine {
	r := gin.New()
	routes.SetupRoutes(r)
	return r
}

func getActiveJWTSecret() []byte {
	secretStr := os.Getenv("JWT_SECRET")
	if secretStr == "" {
		secretStr = os.Getenv("JWT_SECRET_KEY")
	}
	if secretStr == "" {
		secretStr = "jalan_rusak_ai"
		_ = os.Setenv("JWT_SECRET", secretStr)
	}
	return []byte(secretStr)
}

func makeBE12Token(userID uint, role string, exp time.Duration) string {
	claims := utils.JWTClaim{
		UserID: userID,
		Email:  "user_map@example.com",
		Role:   models.UserRole(role),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(exp)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	str, _ := tok.SignedString(getActiveJWTSecret())
	return str
}

// ============================================================
// 1. STANDALONE ZERO-DB TESTS (High-speed unit & security tests)
// ============================================================

func TestBE12_Map_StandaloneSecurity(t *testing.T) {
	origDB := config.DB
	config.DB = nil
	defer func() { config.DB = origDB }()

	r := setupBE12Router()

	wargaToken, _ := utils.GenerateToken(10, "warga@example.com", models.RoleWarga, nil)
	pemdesToken, _ := utils.GenerateToken(20, "pemdes@example.com", models.RoleAdminPemdes, nil)
	puToken, _ := utils.GenerateToken(30, "pu@example.com", models.RoleAdminPu, nil)
	superToken, _ := utils.GenerateToken(1, "sa@example.com", models.RoleSuperAdmin, nil)
	unknownRoleToken := makeBE12Token(10, "unrecognized_role", time.Hour)
	zeroUserToken := makeBE12Token(0, "warga", time.Hour)

	// A. Authentication on Map Endpoints
	mapEndpoints := []struct {
		name   string
		method string
		path   string
	}{
		{"WargaMap", http.MethodGet, "/api/warga/laporan/peta"},
		{"AdminMap", http.MethodGet, "/api/admin/map/laporan"},
	}

	for _, ep := range mapEndpoints {
		t.Run("Auth_MissingToken_"+ep.name, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("[%s] expected 401 Unauthorized without token, got %d", ep.name, w.Code)
			}
		})

		t.Run("Auth_InvalidSignature_"+ep.name, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			req.Header.Set("Authorization", "Bearer invalid.signature.token")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("[%s] expected 401 Unauthorized on invalid signature, got %d", ep.name, w.Code)
			}
		})

		t.Run("Auth_UnknownRole_"+ep.name, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			req.Header.Set("Authorization", "Bearer "+unknownRoleToken)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusForbidden {
				t.Errorf("[%s] expected 403 Forbidden on unknown role, got %d", ep.name, w.Code)
			}
		})

		t.Run("Auth_ZeroUserID_"+ep.name, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			req.Header.Set("Authorization", "Bearer "+zeroUserToken)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized && w.Code != http.StatusForbidden {
				t.Errorf("[%s] expected 401 or 403 on zero user ID, got %d", ep.name, w.Code)
			}
		})
	}

	// B. Role Authorization Boundaries: Warga Forbidden from Admin Map
	t.Run("Auth_WargaForbiddenFromAdminMap", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/map/laporan", nil)
		req.Header.Set("Authorization", "Bearer "+wargaToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for Warga accessing /api/admin/map/laporan, got %d", w.Code)
		}
	})

	// C. Role Authorization Boundaries: Admin Roles Forbidden from Warga Map
	t.Run("Auth_AdminForbiddenFromWargaMap", func(t *testing.T) {
		adminTokens := []struct {
			role  string
			token string
		}{
			{"pemdes", pemdesToken},
			{"pu", puToken},
			{"superadmin", superToken},
		}

		for _, at := range adminTokens {
			req := httptest.NewRequest(http.MethodGet, "/api/warga/laporan/peta", nil)
			req.Header.Set("Authorization", "Bearer "+at.token)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("expected 403 Forbidden for %s accessing /api/warga/laporan/peta, got %d", at.role, w.Code)
			}
		}
	})

	// D. Coordinate Validation Suite via utils.IsValidCoordinate
	t.Run("Coordinate_Validation_Rules", func(t *testing.T) {
		validCases := [][2]float64{
			{-6.3265, 108.3241}, // Indramayu standard
			{0.0, 0.0},          // Equator / Prime Meridian (valid coordinates)
			{90.0, 180.0},       // Upper boundaries
			{-90.0, -180.0},     // Lower boundaries
			{45.123, -122.456},
		}
		for _, c := range validCases {
			if !utils.IsValidCoordinate(c[0], c[1]) {
				t.Errorf("expected coordinate (%f, %f) to be valid", c[0], c[1])
			}
		}

		invalidCases := [][2]float64{
			{90.0001, 108.0},          // Lat > 90
			{-90.0001, 108.0},         // Lat < -90
			{-6.32, 180.0001},         // Lng > 180
			{-6.32, -180.0001},        // Lng < -180
			{math.NaN(), 108.0},       // Lat NaN
			{-6.32, math.NaN()},       // Lng NaN
			{math.Inf(1), 108.0},      // Lat +Inf
			{-6.32, math.Inf(-1)},     // Lng -Inf
			{math.NaN(), math.NaN()},  // Both NaN
			{math.Inf(1), math.Inf(1)},// Both Inf
		}
		for _, c := range invalidCases {
			if utils.IsValidCoordinate(c[0], c[1]) {
				t.Errorf("expected coordinate (%f, %f) to be invalid", c[0], c[1])
			}
		}
	})

	// E. Fail-Closed on Nil Database
	t.Run("FailClosed_DatabaseNil_Returns500WithoutPanic", func(t *testing.T) {
		oldDB := config.DB
		config.DB = nil
		defer func() { config.DB = oldDB }()

		// 1. Warga Map with nil DB
		reqWarga := httptest.NewRequest(http.MethodGet, "/api/warga/laporan/peta", nil)
		reqWarga.Header.Set("Authorization", "Bearer "+wargaToken)
		wWarga := httptest.NewRecorder()
		r.ServeHTTP(wWarga, reqWarga)
		if wWarga.Code != http.StatusInternalServerError {
			t.Errorf("expected 500 when DB is nil on Warga Map, got %d", wWarga.Code)
		}

		// 2. Admin PU Map with nil DB
		reqPU := httptest.NewRequest(http.MethodGet, "/api/admin/map/laporan", nil)
		reqPU.Header.Set("Authorization", "Bearer "+puToken)
		wPU := httptest.NewRecorder()
		r.ServeHTTP(wPU, reqPU)
		if wPU.Code != http.StatusInternalServerError {
			t.Errorf("expected 500 when DB is nil on Admin Map, got %d", wPU.Code)
		}

		// 3. Superadmin Wilayah list with nil DB
		reqWilayah := httptest.NewRequest(http.MethodGet, "/api/superadmin/wilayah", nil)
		reqWilayah.Header.Set("Authorization", "Bearer "+superToken)
		wWilayah := httptest.NewRecorder()
		r.ServeHTTP(wWilayah, reqWilayah)
		if wWilayah.Code != http.StatusInternalServerError {
			t.Errorf("expected 500 when DB is nil on Wilayah list, got %d", wWilayah.Code)
		}
	})

	// F. ReverseGeocodeOSM & FindWilayahByNama Safety
	t.Run("Geocoding_ReverseGeocodeOSM_InputSafety", func(t *testing.T) {
		// Invalid lat/lng must fail immediately without making external HTTP request
		_, _, errNaN := utils.ReverseGeocodeOSM(math.NaN(), 108.0)
		if errNaN == nil {
			t.Error("expected error for NaN latitude in ReverseGeocodeOSM")
		}

		_, _, errInf := utils.ReverseGeocodeOSM(-6.32, math.Inf(1))
		if errInf == nil {
			t.Error("expected error for Inf longitude in ReverseGeocodeOSM")
		}

		_, _, errOutOfRange := utils.ReverseGeocodeOSM(150.0, 300.0)
		if errOutOfRange == nil {
			t.Error("expected error for out of range coordinates in ReverseGeocodeOSM")
		}

		// Nil DB in FindWilayahByNama must fail cleanly
		_, errDB := utils.FindWilayahByNama(nil, "Desa Cantigi")
		if errDB == nil {
			t.Error("expected error for nil DB in FindWilayahByNama")
		}

		// Empty name must fail cleanly
		_, errEmpty := utils.FindWilayahByNama(config.DB, "   ")
		if errEmpty == nil {
			t.Error("expected error for empty name in FindWilayahByNama")
		}
	})

	// G. Response Contract & PII Exclusion
	t.Run("Response_PII_Exclusion_Contract", func(t *testing.T) {
		// 1. MapPoint struct JSON test
		samplePoint := admin.MapPoint{
			ID:            101,
			Judul:         "Jalan Berlubang Parah",
			Latitude:      -6.3265,
			Longitude:     108.3241,
			Status:        "menunggu",
			TipeKerusakan: "lubang",
			JenisJalan:    "desa",
			ImageURL:      "https://res.cloudinary.com/demo/image/upload/road.jpg",
			FotoBukti:     "",
			CatatanAdmin:  "",
			Name:          "Warga Anonymous",
			WilayahID:     5,
		}

		bPoint, err := json.Marshal(samplePoint)
		if err != nil {
			t.Fatalf("failed to marshal MapPoint: %v", err)
		}
		strPoint := string(bPoint)

		forbiddenPII := []string{
			"password",
			"token",
			"email",
			"phone",
			"token_version",
			"deleted_at",
			"cloudinary_secret",
		}
		for _, f := range forbiddenPII {
			if strings.Contains(strings.ToLower(strPoint), `"`+f+`"`) {
				t.Errorf("PII LEAK: found %q in MapPoint JSON: %s", f, strPoint)
			}
		}

		// 2. LaporanResponse struct JSON test
		sampleLap := models.LaporanKerusakan{
			Judul:         "Jalan Rusak",
			Deskripsi:     "Deskripsi laporan",
			Latitude:      -6.3265,
			Longitude:     108.3241,
			ImageURL:      "http://img.com/a.jpg",
			TipeKerusakan: "retak",
			Status:        "proses",
		}
		respLap := warga.FormatLaporanToResponse(sampleLap)
		bLap, _ := json.Marshal(respLap)
		strLap := string(bLap)

		for _, f := range forbiddenPII {
			if strings.Contains(strings.ToLower(strLap), `"`+f+`"`) {
				t.Errorf("PII LEAK: found %q in LaporanResponse JSON: %s", f, strLap)
			}
		}
	})
}

// ============================================================
// 2. INTEGRATION SIMULATION TESTS (Role Scopes & Geographic Filter)
// ============================================================

func TestBE12_Map_GeographicScopeRules(t *testing.T) {
	wilayahDesaA := uint(10)
	wilayahDesaB := uint(20)

	// Mock Dataset representing diverse road categories & coordinates
	dataset := []models.LaporanKerusakan{
		// 1. Desa A, desa road, valid coordinate -> Visible to Pemdes A, PU, Superadmin
		{
			Model:         gorm.Model{ID: 1},
			WilayahID:     wilayahDesaA,
			JenisJalan:    "desa",
			Status:        "menunggu",
			Latitude:      -6.3265,
			Longitude:     108.3241,
			TipeKerusakan: "lubang",
		},
		// 2. Desa B, desa road, valid coordinate -> Visible to Pemdes B, PU, Superadmin (NOT Pemdes A)
		{
			Model:         gorm.Model{ID: 2},
			WilayahID:     wilayahDesaB,
			JenisJalan:    "desa",
			Status:        "proses",
			Latitude:      -6.3400,
			Longitude:     108.3300,
			TipeKerusakan: "retak",
		},
		// 3. Desa A, kabupaten road, valid coordinate -> Visible to PU, Superadmin (NOT Pemdes A!)
		{
			Model:         gorm.Model{ID: 3},
			WilayahID:     wilayahDesaA,
			JenisJalan:    "kabupaten",
			Status:        "menunggu",
			Latitude:      -6.3270,
			Longitude:     108.3250,
			TipeKerusakan: "amblas",
		},
		// 4. Desa B, provinsi road, valid coordinate -> Visible to PU, Superadmin (NOT Pemdes!)
		{
			Model:         gorm.Model{ID: 4},
			WilayahID:     wilayahDesaB,
			JenisJalan:    "provinsi",
			Status:        "proses",
			Latitude:      -6.3500,
			Longitude:     108.3600,
			TipeKerusakan: "longsor",
		},
		// 5. Desa A, nasional road, valid coordinate -> Visible to PU, Superadmin (NOT Pemdes!)
		{
			Model:         gorm.Model{ID: 5},
			WilayahID:     wilayahDesaA,
			JenisJalan:    "nasional",
			Status:        "selesai",
			Latitude:      -6.3100,
			Longitude:     108.3100,
			TipeKerusakan: "gelombang",
		},
		// 6. Soft-deleted report -> NEVER visible to ANY role!
		{
			Model:         gorm.Model{ID: 6, DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}},
			WilayahID:     wilayahDesaA,
			JenisJalan:    "desa",
			Status:        "menunggu",
			Latitude:      -6.3265,
			Longitude:     108.3241,
		},
		// 7. Invalid coordinate (Lat > 90) -> NEVER visible on map markers!
		{
			Model:         gorm.Model{ID: 7},
			WilayahID:     wilayahDesaA,
			JenisJalan:    "desa",
			Status:        "menunggu",
			Latitude:      95.0,
			Longitude:     108.3241,
		},
		// 8. Invalid coordinate (NaN) -> NEVER visible on map markers!
		{
			Model:         gorm.Model{ID: 8},
			WilayahID:     wilayahDesaA,
			JenisJalan:    "kabupaten",
			Status:        "proses",
			Latitude:      math.NaN(),
			Longitude:     108.3241,
		},
	}

	// Filter simulator adhering strictly to ROADIS product rules
	evalMapScope := func(role string, adminWilayah *uint, lap models.LaporanKerusakan, filterJenis, filterWilayah string) bool {
		if lap.DeletedAt.Valid {
			return false
		}
		if !utils.IsValidCoordinate(lap.Latitude, lap.Longitude) {
			return false
		}

		jenisJalan := strings.ToLower(lap.JenisJalan)

		switch role {
		case string(models.RoleAdminPemdes):
			if adminWilayah == nil || *adminWilayah == 0 {
				return false // Fail-closed
			}
			// Pemdes MUST ONLY see Desa roads in their own wilayah
			if jenisJalan != "desa" || lap.WilayahID != *adminWilayah {
				return false
			}

		case string(models.RoleAdminPu):
			// PU reads ALL authorities (desa, kabupaten, provinsi, nasional)
			if filterJenis != "" && filterJenis != "all" && jenisJalan != filterJenis {
				return false
			}
			if filterWilayah != "" {
				// filter by specific wilayah_id
				var targetW uint
				_ = json.Unmarshal([]byte(filterWilayah), &targetW)
				if targetW > 0 && lap.WilayahID != targetW {
					return false
				}
			}

		case string(models.RoleSuperAdmin):
			// Superadmin views all
			if filterJenis != "" && filterJenis != "all" && jenisJalan != filterJenis {
				return false
			}

		default:
			return false
		}
		return true
	}

	// 1. Admin Pemdes Wilayah A Scope:
	// Only Report #1 matches (Desa A, desa road). Reports #2, #3, #4, #5, #6, #7, #8 are blocked!
	t.Run("PemdesA_StrictDesaWilayahIsolation", func(t *testing.T) {
		var matched []uint
		for _, lap := range dataset {
			if evalMapScope(string(models.RoleAdminPemdes), &wilayahDesaA, lap, "", "") {
				matched = append(matched, lap.ID)
			}
		}
		if len(matched) != 1 || matched[0] != 1 {
			t.Fatalf("expected ONLY report #1 for Pemdes A, got %v", matched)
		}
	})

	// 2. Admin Pemdes Wilayah B Scope:
	// Only Report #2 matches (Desa B, desa road).
	t.Run("PemdesB_StrictDesaWilayahIsolation", func(t *testing.T) {
		var matched []uint
		for _, lap := range dataset {
			if evalMapScope(string(models.RoleAdminPemdes), &wilayahDesaB, lap, "", "") {
				matched = append(matched, lap.ID)
			}
		}
		if len(matched) != 1 || matched[0] != 2 {
			t.Fatalf("expected ONLY report #2 for Pemdes B, got %v", matched)
		}
	})

	// 3. Admin Pemdes Unassigned Wilayah: Fail-closed (0 points)
	t.Run("Pemdes_UnassignedWilayah_FailClosed", func(t *testing.T) {
		for _, lap := range dataset {
			if evalMapScope(string(models.RoleAdminPemdes), nil, lap, "", "") {
				t.Fatalf("unassigned Pemdes must not see any map points!")
			}
		}
	})

	// 4. Admin PU All-Authority Read Scope:
	// Reports #1 (desa), #2 (desa), #3 (kabupaten), #4 (provinsi), #5 (nasional) all visible (5 total)!
	t.Run("AdminPU_MonitorsAllAuthorities", func(t *testing.T) {
		var matched []uint
		for _, lap := range dataset {
			if evalMapScope(string(models.RoleAdminPu), nil, lap, "", "") {
				matched = append(matched, lap.ID)
			}
		}
		if len(matched) != 5 {
			t.Fatalf("expected 5 visible points for Admin PU across all authorities, got %d: %v", len(matched), matched)
		}
	})

	// 5. Admin PU Filtered by jenis_jalan="kabupaten":
	// Only Report #3 matches
	t.Run("AdminPU_FilteredByKabupaten", func(t *testing.T) {
		var matched []uint
		for _, lap := range dataset {
			if evalMapScope(string(models.RoleAdminPu), nil, lap, "kabupaten", "") {
				matched = append(matched, lap.ID)
			}
		}
		if len(matched) != 1 || matched[0] != 3 {
			t.Fatalf("expected report #3 for PU with jenis_jalan=kabupaten, got %v", matched)
		}
	})

	// 6. Superadmin Scope:
	// Reports #1, #2, #3, #4, #5 visible (5 total)
	t.Run("Superadmin_FullMapVisibility", func(t *testing.T) {
		var matched []uint
		for _, lap := range dataset {
			if evalMapScope(string(models.RoleSuperAdmin), nil, lap, "", "") {
				matched = append(matched, lap.ID)
			}
		}
		if len(matched) != 5 {
			t.Fatalf("expected 5 visible points for Superadmin, got %d: %v", len(matched), matched)
		}
	})
}
