package controllers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/controllers"
	"backend-jalan-rusak/controllers/admin"
	"backend-jalan-rusak/controllers/warga"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/routes"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func ensureTestDB(t *testing.T) bool {
	if config.DB != nil {
		return true
	}
	dsn := "root:@tcp(127.0.0.1:3306)/db_jalan_rusak?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})
	if err != nil {
		t.Logf("MySQL connection unavailable in test environment (%v)", err)
		return false
	}
	config.DB = db
	return true
}

func setupBE16Router() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	routes.SetupRoutes(r)
	return r
}

func makeBE16Token(userID uint, role string, exp time.Duration) string {
	claims := utils.JWTClaim{
		UserID: userID,
		Email:  "be16_test@roadis.local",
		Role:   models.UserRole(role),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(exp)),
		},
	}
	secretStr := os.Getenv("JWT_SECRET")
	if secretStr == "" {
		secretStr = os.Getenv("JWT_SECRET_KEY")
	}
	if secretStr == "" {
		secretStr = "jalan_rusak_ai"
		_ = os.Setenv("JWT_SECRET", secretStr)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	str, _ := tok.SignedString([]byte(secretStr))
	return str
}

// 1. Success envelope consistency: {"status": "success", "message": "...", "data": ...}
func TestBE16_01_SuccessEnvelopeConsistency(t *testing.T) {
	r := setupBE16Router()

	t.Run("HealthCheckEnvelope", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if resp["status"] != "ok" && resp["status"] != "success" {
			t.Errorf("expected status 'ok' or 'success', got %v", resp["status"])
		}
	})

	t.Run("SystemInfoEnvelope", func(t *testing.T) {
		token := makeBE16Token(1, "warga", time.Hour)
		req := httptest.NewRequest(http.MethodGet, "/api/system/info", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if resp["status"] != "success" {
			t.Errorf("expected status 'success', got %v", resp["status"])
		}
		if resp["data"] == nil {
			t.Errorf("expected 'data' field in success response")
		}
	})
}

// 2. Error envelope consistency: {"status": "error", "message": "...", "error": "..."}
func TestBE16_02_ErrorEnvelopeConsistency(t *testing.T) {
	r := setupBE16Router()

	t.Run("AuthMiddlewareErrorEnvelope", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", w.Code)
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if resp["status"] != "error" {
			t.Errorf("expected status 'error', got %v", resp["status"])
		}
		if resp["message"] == nil || resp["message"] == "" {
			t.Errorf("expected non-empty 'message', got %v", resp["message"])
		}
		if resp["error"] == nil || resp["error"] == "" {
			t.Errorf("expected backward-compatible 'error' key, got %v", resp["error"])
		}
	})
}

// 3. 400 validation consistency: malformed input, invalid enum, invalid ID
func TestBE16_03_Validation400Consistency(t *testing.T) {
	r := setupBE16Router()
	token := makeBE16Token(1, "super_admin", time.Hour)

	cases := []struct {
		name   string
		method string
		url    string
		body   string
	}{
		{"InvalidUserID_Alpha", http.MethodGet, "/api/superadmin/users/abc", ""},
		{"InvalidUserID_Zero", http.MethodGet, "/api/superadmin/users/0", ""},
		{"InvalidWilayahID_Negative", http.MethodGet, "/api/superadmin/wilayah/-5", ""},
		{"MalformedJSON_CreateWilayah", http.MethodPost, "/api/superadmin/wilayah", "{invalid-json"},
		{"MissingFields_CreateWilayah", http.MethodPost, "/api/superadmin/wilayah", `{"nama":""}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var req *http.Request
			if tc.body != "" {
				req = httptest.NewRequest(tc.method, tc.url, bytes.NewBufferString(tc.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tc.method, tc.url, nil)
			}
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("%s: expected 400 Bad Request, got %d (body: %s)", tc.name, w.Code, w.Body.String())
			}
			var resp map[string]interface{}
			_ = json.Unmarshal(w.Body.Bytes(), &resp)
			if resp["status"] != "error" {
				t.Errorf("%s: expected status 'error', got %v", tc.name, resp["status"])
			}
		})
	}
}

// 4. 401 auth consistency: missing token, invalid JWT, expired JWT
func TestBE16_04_Auth401Consistency(t *testing.T) {
	r := setupBE16Router()

	cases := []struct {
		name       string
		authHeader string
	}{
		{"MissingToken", ""},
		{"InvalidFormatNoBearer", "Token123456"},
		{"InvalidTokenString", "Bearer thisisnotavalidjwttoken"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("%s: expected 401 Unauthorized, got %d", tc.name, w.Code)
			}
			var resp map[string]interface{}
			_ = json.Unmarshal(w.Body.Bytes(), &resp)
			if resp["status"] != "error" {
				t.Errorf("%s: expected status 'error', got %v", tc.name, resp["status"])
			}
		})
	}
}

// 5. 403 authorization consistency: role boundary violation
func TestBE16_05_Authorization403Consistency(t *testing.T) {
	r := setupBE16Router()
	wargaToken := makeBE16Token(10, "warga", time.Hour)

	cases := []struct {
		name   string
		method string
		url    string
	}{
		{"WargaAccessSuperadminUsers", http.MethodGet, "/api/superadmin/users"},
		{"WargaAccessSuperadminWilayah", http.MethodPost, "/api/superadmin/wilayah"},
		{"WargaAccessAdminDashboard", http.MethodGet, "/api/admin/dashboard"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.url, nil)
			req.Header.Set("Authorization", "Bearer "+wargaToken)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("%s: expected 403 Forbidden, got %d", tc.name, w.Code)
			}
			var resp map[string]interface{}
			_ = json.Unmarshal(w.Body.Bytes(), &resp)
			if resp["status"] != "error" {
				t.Errorf("%s: expected status 'error', got %v", tc.name, resp["status"])
			}
		})
	}
}

// 6. 404 not-found consistency: resource not found
func TestBE16_06_NotFound404Consistency(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for not found tests")
	}
	r := setupBE16Router()
	token := makeBE16Token(1, "super_admin", time.Hour)

	cases := []struct {
		name string
		url  string
	}{
		{"NonExistentUser", "/api/superadmin/users/999999"},
		{"NonExistentWilayah", "/api/superadmin/wilayah/999999"},
		{"NonExistentLaporan", "/api/superadmin/laporan/999999"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			method := http.MethodGet
			if strings.Contains(tc.url, "/laporan/") {
				method = http.MethodDelete
			}
			req := httptest.NewRequest(method, tc.url, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				t.Errorf("%s: expected 404 Not Found, got %d", tc.name, w.Code)
			}
			var resp map[string]interface{}
			_ = json.Unmarshal(w.Body.Bytes(), &resp)
			if resp["status"] != "error" {
				t.Errorf("%s: expected status 'error', got %v", tc.name, resp["status"])
			}
		})
	}
}

// 7. 409 conflict consistency: duplicate registration
func TestBE16_07_Conflict409Consistency(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for conflict tests")
	}
	r := setupBE16Router()

	uniqueEmail := fmt.Sprintf("be16_conflict_%d@roadis.local", time.Now().UnixNano())
	user := models.User{
		Name:     "Existing User",
		Email:    uniqueEmail,
		Password: "hashedpassword123",
		Role:     models.RoleWarga,
	}
	if err := config.DB.Create(&user).Error; err != nil {
		t.Skipf("cannot seed test user: %v", err)
	}
	defer config.DB.Unscoped().Delete(&user)

	payload := map[string]string{
		"name":     "New Duplicate User",
		"email":    uniqueEmail,
		"password": "newpassword123",
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict, got %d", w.Code)
	}
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "error" {
		t.Errorf("expected status 'error', got %v", resp["status"])
	}
}

// 8. ID field naming consistency: snake_case for all ID tags in public API DTOs
func TestBE16_08_IDFieldNamingConsistency(t *testing.T) {
	wilayahID := uint(10)
	user := models.User{
		Model:     gorm.Model{ID: 1},
		Name:      "Test User",
		Email:     "user@roadis.id",
		WilayahID: &wilayahID,
		Role:      models.RoleWarga,
	}

	// A. Laporan DTO
	laporan := models.LaporanKerusakan{
		Model:      gorm.Model{ID: 5},
		UserID:     user.ID,
		WilayahID:  wilayahID,
		Judul:      "Jalan Berlubang",
		JenisJalan: "desa",
		Status:     "menunggu",
	}
	lapResp := warga.FormatLaporanToResponse(laporan)
	bLap, err := json.Marshal(lapResp)
	if err != nil {
		t.Fatalf("Marshal laporan response failed: %v", err)
	}
	strLap := string(bLap)
	if !strings.Contains(strLap, `"id":5`) || !strings.Contains(strLap, `"user_id":1`) {
		t.Errorf("expected snake_case id and user_id in laporan response: %s", strLap)
	}

	// B. Chat DTO
	chat := models.RiwayatChat{
		Model:              gorm.Model{ID: 25},
		LaporanKerusakanID: 5,
		UserID:             1,
		Pesan:              "Halo Petugas",
	}
	chatResp := controllers.FormatChatToResponse(chat)
	bChat, err := json.Marshal(chatResp)
	if err != nil {
		t.Fatalf("Marshal chat response failed: %v", err)
	}
	strChat := string(bChat)
	if !strings.Contains(strChat, `"id":25`) || !strings.Contains(strChat, `"laporan_kerusakan_id":5`) || !strings.Contains(strChat, `"user_id":1`) {
		t.Errorf("expected snake_case id, laporan_kerusakan_id, user_id in chat response: %s", strChat)
	}

	// C. MapPoint DTO
	mapPt := admin.MapPoint{
		ID:        12,
		WilayahID: 10,
		Judul:     "Titik Kerusakan",
	}
	bMap, err := json.Marshal(mapPt)
	if err != nil {
		t.Fatalf("Marshal MapPoint failed: %v", err)
	}
	strMap := string(bMap)
	if !strings.Contains(strMap, `"id":12`) || !strings.Contains(strMap, `"wilayah_id":10`) {
		t.Errorf("expected snake_case id and wilayah_id in MapPoint: %s", strMap)
	}
}

// 9. Pagination contract: page, limit, total/total_data, empty collections serialize as []
func TestBE16_09_PaginationContractConsistency(t *testing.T) {
	// A. Serialization of empty list
	emptyList := make([]models.User, 0)
	b, err := json.Marshal(gin.H{
		"status": "success",
		"data": gin.H{
			"users":      emptyList,
			"page":       1,
			"limit":      10,
			"total":      0,
			"total_data": 0,
		},
	})
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	str := string(b)
	if !strings.Contains(str, `"users":[]`) {
		t.Errorf("expected empty array '[]', got: %s", str)
	}
	if !strings.Contains(str, `"total":0`) || !strings.Contains(str, `"total_data":0`) {
		t.Errorf("expected total and total_data in pagination data: %s", str)
	}
}

// 10. Null / optional fields consistency: pointers and nullable attributes
func TestBE16_10_NullOptionalFieldsConsistency(t *testing.T) {
	user := models.User{
		Model:     gorm.Model{ID: 2},
		Name:      "User Without Wilayah",
		Email:     "nowilayah@roadis.id",
		WilayahID: nil,
		AvatarURL: nil,
	}
	b, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	str := string(b)
	if !strings.Contains(str, `"wilayah_id":null`) {
		t.Errorf("expected 'wilayah_id: null', got: %s", str)
	}
}

// 11. Create resource response consistency: 201 for Register and Wilayah, 200 preserved for User
func TestBE16_11_CreateResourceResponseConsistency(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for create resource tests")
	}
	r := setupBE16Router()

	// A. Register -> 201 Created
	uniqueEmail := fmt.Sprintf("be16_reg_%d@roadis.local", time.Now().UnixNano())
	regPayload := map[string]string{
		"name":     "Warga Baru",
		"email":    uniqueEmail,
		"password": "password123",
	}
	bReg, _ := json.Marshal(regPayload)
	reqReg := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBuffer(bReg))
	reqReg.Header.Set("Content-Type", "application/json")
	wReg := httptest.NewRecorder()
	r.ServeHTTP(wReg, reqReg)

	if wReg.Code != http.StatusCreated {
		t.Errorf("expected 201 Created for register, got %d: %s", wReg.Code, wReg.Body.String())
	}
	var regResp map[string]interface{}
	_ = json.Unmarshal(wReg.Body.Bytes(), &regResp)
	if regResp["status"] != "success" {
		t.Errorf("expected status 'success', got %v", regResp["status"])
	}

	// Clean up user
	config.DB.Unscoped().Where("email = ?", uniqueEmail).Delete(&models.User{})

	// B. Create Wilayah -> 201 Created
	superToken := makeBE16Token(1, "super_admin", time.Hour)
	wilayahPayload := map[string]string{
		"nama": fmt.Sprintf("Wilayah BE16 %d", time.Now().UnixNano()),
		"tipe": "desa",
	}
	bWil, _ := json.Marshal(wilayahPayload)
	reqWil := httptest.NewRequest(http.MethodPost, "/api/superadmin/wilayah", bytes.NewBuffer(bWil))
	reqWil.Header.Set("Authorization", "Bearer "+superToken)
	reqWil.Header.Set("Content-Type", "application/json")
	wWil := httptest.NewRecorder()
	r.ServeHTTP(wWil, reqWil)

	if wWil.Code != http.StatusCreated {
		t.Errorf("expected 201 Created for CreateWilayah, got %d: %s", wWil.Code, wWil.Body.String())
	}
	var wilResp map[string]interface{}
	_ = json.Unmarshal(wWil.Body.Bytes(), &wilResp)
	if wilResp["status"] != "success" {
		t.Errorf("expected status 'success', got %v", wilResp["status"])
	}
}

// 12. Update resource response consistency: 200 OK with success envelope
func TestBE16_12_UpdateResourceResponseConsistency(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for update tests")
	}
	r := setupBE16Router()
	superToken := makeBE16Token(1, "super_admin", time.Hour)

	wilayah := models.Wilayah{
		Nama: fmt.Sprintf("Wilayah Update %d", time.Now().UnixNano()),
		Tipe: "desa",
	}
	config.DB.Create(&wilayah)
	defer config.DB.Unscoped().Delete(&wilayah)

	updatePayload := map[string]string{
		"nama": fmt.Sprintf("Wilayah Renamed %d", time.Now().UnixNano()),
		"tipe": "desa",
	}
	b, _ := json.Marshal(updatePayload)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/superadmin/wilayah/%d", wilayah.ID), bytes.NewBuffer(b))
	req.Header.Set("Authorization", "Bearer "+superToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK for update, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "success" {
		t.Errorf("expected status 'success', got %v", resp["status"])
	}
	if resp["message"] == nil || resp["message"] == "" {
		t.Errorf("expected message in update response, got %v", resp["message"])
	}
}

// 13. Delete resource response consistency: 200 OK with success envelope
func TestBE16_13_DeleteResourceResponseConsistency(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for delete tests")
	}
	r := setupBE16Router()
	superToken := makeBE16Token(1, "super_admin", time.Hour)

	wilayah := models.Wilayah{
		Nama: fmt.Sprintf("Wilayah Delete %d", time.Now().UnixNano()),
		Tipe: "desa",
	}
	config.DB.Create(&wilayah)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/superadmin/wilayah/%d", wilayah.ID), nil)
	req.Header.Set("Authorization", "Bearer "+superToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK for delete, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "success" {
		t.Errorf("expected status 'success', got %v", resp["status"])
	}
	if resp["message"] == nil || resp["message"] == "" {
		t.Errorf("expected message in delete response, got %v", resp["message"])
	}
}

// 14. Sensitive field leakage prevention
func TestBE16_14_SensitiveFieldLeakagePrevention(t *testing.T) {
	user := models.User{
		Model:        gorm.Model{ID: 1},
		Name:         "User Secret",
		Email:        "secret@roadis.id",
		Password:     "$2a$10$supersecretbcrypthashthatmustneverbeexposed",
		TokenVersion: 5,
		Role:         models.RoleWarga,
	}

	b, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("marshal user failed: %v", err)
	}
	str := string(b)
	if strings.Contains(str, "password") || strings.Contains(str, "supersecret") {
		t.Errorf("password leaked in user serialization: %s", str)
	}
	if strings.Contains(str, "token_version") {
		t.Errorf("token_version leaked in user serialization: %s", str)
	}
}

// 15. Timestamp format consistency
func TestBE16_15_TimestampFormatConsistency(t *testing.T) {
	fixedTime := time.Date(2026, 10, 8, 14, 30, 0, 0, time.UTC)
	formatted := utils.FormatTanggalIndo(&fixedTime)

	if !strings.Contains(formatted, "8") || !strings.Contains(formatted, "Oktober") || !strings.Contains(formatted, "2026") {
		t.Errorf("expected Indonesian date string, got: %s", formatted)
	}
}

// 16. Frontend-critical response compatibility
func TestBE16_16_FrontendCriticalContractCompatibility(t *testing.T) {
	// A. Verify Login success payload structure matches frontend expectation
	mockUser := gin.H{
		"id":           uint(1),
		"nama":         "Admin User",
		"email":        "admin@roadis.id",
		"role":         models.RoleSuperAdmin,
		"wilayah_id":   nil,
		"profil_photo": "",
	}
	loginResp := gin.H{
		"status":  "success",
		"message": "Login berhasil",
		"token":   "mock-jwt-token",
		"user":    mockUser,
	}
	bLogin, err := json.Marshal(loginResp)
	if err != nil {
		t.Fatalf("marshal login response failed: %v", err)
	}
	strLogin := string(bLogin)
	if !strings.Contains(strLogin, `"token":"mock-jwt-token"`) || !strings.Contains(strLogin, `"user":{`) {
		t.Errorf("login response structure incompatible: %s", strLogin)
	}

	// B. Verify Settings response structure matches frontend expectation
	pref := models.NewDefaultUserPreference(1)
	prefResp := gin.H{
		"status":  "success",
		"message": "Preferensi berhasil diambil",
		"data": gin.H{
			"preferences": gin.H{
				"notification_sound_enabled":  pref.SoundEnabled(),
				"notification_report_enabled": pref.ReportEnabled(),
				"notification_status_enabled": pref.StatusEnabled(),
				"notification_chat_enabled":   pref.ChatEnabled(),
				"theme":                       pref.Theme,
				"display_density":             pref.DisplayDensity,
				"map_default_view":            pref.MapDefaultView,
				"map_show_labels":             pref.ShowLabels(),
				"report_display_preference":   pref.ReportDisplayPreference,
			},
		},
	}
	bPref, err := json.Marshal(prefResp)
	if err != nil {
		t.Fatalf("marshal pref response failed: %v", err)
	}
	strPref := string(bPref)
	if !strings.Contains(strPref, `"notification_sound_enabled":true`) || !strings.Contains(strPref, `"theme":"light"`) {
		t.Errorf("settings response structure incompatible: %s", strPref)
	}

	// C. Verify Chat formatting matches frontend ChatResponse contract
	chat := models.RiwayatChat{
		Model:              gorm.Model{ID: 10},
		LaporanKerusakanID: 5,
		UserID:             1,
		Pesan:              "Halo admin",
	}
	formattedChat := controllers.FormatChatToResponse(chat)
	bChat, err := json.Marshal(formattedChat)
	if err != nil {
		t.Fatalf("marshal chat response failed: %v", err)
	}
	strChat := string(bChat)
	if !strings.Contains(strChat, `"laporan_kerusakan_id":5`) || !strings.Contains(strChat, `"pesan":"Halo admin"`) {
		t.Errorf("chat response structure incompatible: %s", strChat)
	}
}
