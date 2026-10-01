package controllers

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
	"golang.org/x/crypto/bcrypt"
)

func setupSettingsTestRouter() *gin.Engine {
	r := gin.New()
	public := r.Group("/api")
	{
		public.GET("/health", HealthCheck)
	}

	api := r.Group("/api")
	api.Use(middlewares.AuthMiddleware())
	{
		api.GET("/settings", GetSettings)
		api.PUT("/settings", UpdateSettings)
		api.POST("/settings/logout-all", LogoutAll)
		api.GET("/system/info", GetSystemInfo)
	}
	return r
}

func createTestUserWithWilayah(t *testing.T, role models.UserRole) (models.User, string, func()) {
	t.Helper()
	nowNano := time.Now().UnixNano()

	// Buat wilayah jika admin_pemdes
	var wilayah models.Wilayah
	var wilayahID *uint
	if role == models.RoleAdminPemdes {
		wilayah = models.Wilayah{
			Nama: fmt.Sprintf("Desa Test %d", nowNano),
			Tipe: "desa",
		}
		if err := config.DB.Create(&wilayah).Error; err != nil {
			t.Fatalf("Failed to create test wilayah: %v", err)
		}
		wilayahID = &wilayah.ID
	}

	hashed, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	now := time.Now()
	user := models.User{
		Name:              fmt.Sprintf("User Test %d", nowNano),
		Email:             fmt.Sprintf("user_%d@roadis.local", nowNano),
		Password:          string(hashed),
		Role:              role,
		WilayahID:         wilayahID,
		TokenVersion:      1,
		LastLoginAt:       &now,
		PasswordChangedAt: &now,
	}
	if err := config.DB.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	token, err := utils.GenerateToken(user.ID, user.Email, user.Role, user.WilayahID, user.TokenVersion)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	cleanup := func() {
		config.DB.Unscoped().Where("user_id = ?", user.ID).Delete(&models.UserPreference{})
		config.DB.Unscoped().Delete(&user)
		if wilayah.ID != 0 {
			config.DB.Unscoped().Delete(&wilayah)
		}
	}

	return user, token, cleanup
}

func TestHealthCheck(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for HealthCheck test")
	}

	r := setupSettingsTestRouter()
	req, _ := http.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var res map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}

	if res["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", res["status"])
	}
	if res["service"] != "roadis-api" {
		t.Errorf("expected service 'roadis-api', got %v", res["service"])
	}
	if res["database"] != "connected" {
		t.Errorf("expected database 'connected', got %v", res["database"])
	}

	// Pastikan tidak membocorkan credential atau host
	bodyStr := w.Body.String()
	if strings.Contains(bodyStr, "password") || strings.Contains(bodyStr, "JWT_SECRET") || strings.Contains(bodyStr, "root") {
		t.Errorf("health check response leaked sensitive data: %s", bodyStr)
	}
}

func TestSettings_Unauthenticated(t *testing.T) {
	r := setupSettingsTestRouter()

	// 1. GET /api/settings tanpa token -> 401
	{
		req, _ := http.NewRequest(http.MethodGet, "/api/settings", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("GET /api/settings expected 401, got %d", w.Code)
		}
	}

	// 2. PUT /api/settings tanpa token -> 401
	{
		payload := `{"notification_sound_enabled": false}`
		req, _ := http.NewRequest(http.MethodPut, "/api/settings", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("PUT /api/settings expected 401, got %d", w.Code)
		}
	}

	// 3. POST /api/settings/logout-all tanpa token -> 401
	{
		req, _ := http.NewRequest(http.MethodPost, "/api/settings/logout-all", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("POST /api/settings/logout-all expected 401, got %d", w.Code)
		}
	}

	// 4. GET /api/system/info tanpa token -> 401
	{
		req, _ := http.NewRequest(http.MethodGet, "/api/system/info", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("GET /api/system/info expected 401, got %d", w.Code)
		}
	}
}

func TestSettings_AdminPemdesGetDefaultAndPersistence(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for Settings tests")
	}

	user, token, cleanup := createTestUserWithWilayah(t, models.RoleAdminPemdes)
	defer cleanup()

	r := setupSettingsTestRouter()

	// 1. GET /api/settings pertama kali -> cek seluruh default value sesuai kontrak
	{
		req, _ := http.NewRequest(http.MethodGet, "/api/settings", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var res struct {
			Status string `json:"status"`
			Data   struct {
				Account struct {
					ID        uint   `json:"id"`
					Name      string `json:"name"`
					Email     string `json:"email"`
					Role      string `json:"role"`
					WilayahID *uint  `json:"wilayah_id"`
					Wilayah   *struct {
						ID   uint   `json:"id"`
						Nama string `json:"nama"`
						Tipe string `json:"tipe"`
					} `json:"wilayah"`
					Status string `json:"status"`
				} `json:"account"`
				Preferences struct {
					NotificationSoundEnabled  bool   `json:"notification_sound_enabled"`
					NotificationReportEnabled bool   `json:"notification_report_enabled"`
					NotificationStatusEnabled bool   `json:"notification_status_enabled"`
					NotificationChatEnabled   bool   `json:"notification_chat_enabled"`
					Theme                     string `json:"theme"`
					DisplayDensity            string `json:"display_density"`
					MapDefaultView            string `json:"map_default_view"`
					MapShowLabels             bool   `json:"map_show_labels"`
					ReportDisplayPreference   string `json:"report_display_preference"`
				} `json:"preferences"`
				Security struct {
					LastLoginAt      *time.Time `json:"last_login_at"`
					HasActiveSession bool       `json:"has_active_session"`
				} `json:"security"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode json: %v", err)
		}

		// Account data checks
		if res.Data.Account.Email != user.Email {
			t.Errorf("expected email %s, got %s", user.Email, res.Data.Account.Email)
		}
		if res.Data.Account.Role != string(models.RoleAdminPemdes) {
			t.Errorf("expected role %s, got %s", models.RoleAdminPemdes, res.Data.Account.Role)
		}
		if res.Data.Account.Status != "active" {
			t.Errorf("expected status 'active', got %s", res.Data.Account.Status)
		}
		if res.Data.Account.Wilayah == nil || res.Data.Account.Wilayah.ID == 0 {
			t.Errorf("expected non-nil wilayah for admin_pemdes")
		}

		// Default Preferences checks (sesuai spesifikasi Section 4)
		if !res.Data.Preferences.NotificationSoundEnabled {
			t.Errorf("expected default notification_sound_enabled true, got false")
		}
		if !res.Data.Preferences.NotificationReportEnabled {
			t.Errorf("expected default notification_report_enabled true, got false")
		}
		if !res.Data.Preferences.NotificationStatusEnabled {
			t.Errorf("expected default notification_status_enabled true, got false")
		}
		if !res.Data.Preferences.NotificationChatEnabled {
			t.Errorf("expected default notification_chat_enabled true, got false")
		}
		if res.Data.Preferences.Theme != "light" {
			t.Errorf("expected default theme 'light', got %s", res.Data.Preferences.Theme)
		}
		if res.Data.Preferences.DisplayDensity != "comfortable" {
			t.Errorf("expected default display_density 'comfortable', got %s", res.Data.Preferences.DisplayDensity)
		}
		if res.Data.Preferences.MapDefaultView != "standard" {
			t.Errorf("expected default map_default_view 'standard', got %s", res.Data.Preferences.MapDefaultView)
		}
		if !res.Data.Preferences.MapShowLabels {
			t.Errorf("expected default map_show_labels true, got false")
		}
		if res.Data.Preferences.ReportDisplayPreference != "comfortable" {
			t.Errorf("expected default report_display_preference 'comfortable', got %s", res.Data.Preferences.ReportDisplayPreference)
		}

		// Security checks
		if !res.Data.Security.HasActiveSession {
			t.Errorf("expected has_active_session true")
		}
	}

	// 2. Cek database: memastikan row dibuat idempotently (tepat 1 record)
	var count int64
	config.DB.Model(&models.UserPreference{}).Where("user_id = ?", user.ID).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly 1 preference record, found %d", count)
	}

	// 3. Panggil GET sekali lagi -> memastikan idempotent (tidak membuat duplicate row)
	{
		req, _ := http.NewRequest(http.MethodGet, "/api/settings", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("second GET expected 200, got %d", w.Code)
		}

		config.DB.Model(&models.UserPreference{}).Where("user_id = ?", user.ID).Count(&count)
		if count != 1 {
			t.Fatalf("expected count still 1 after second GET, found %d", count)
		}
	}

	// 4. PUT /api/settings -> update preferensi notifikasi dan tampilan
	{
		payload := `{
			"notification_sound_enabled": false,
			"notification_report_enabled": false,
			"notification_status_enabled": false,
			"notification_chat_enabled": false,
			"theme": "dark",
			"display_density": "compact",
			"map_default_view": "satellite",
			"map_show_labels": false,
			"report_display_preference": "compact"
		}`
		req, _ := http.NewRequest(http.MethodPut, "/api/settings", bytes.NewBufferString(payload))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("PUT /api/settings expected 200, got %d: %s", w.Code, w.Body.String())
		}
	}

	// 5. GET /api/settings lagi -> pastikan seluruh nilai tersimpan persisten
	{
		req, _ := http.NewRequest(http.MethodGet, "/api/settings", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var res struct {
			Data struct {
				Preferences struct {
					NotificationSoundEnabled  bool   `json:"notification_sound_enabled"`
					NotificationReportEnabled bool   `json:"notification_report_enabled"`
					NotificationStatusEnabled bool   `json:"notification_status_enabled"`
					NotificationChatEnabled   bool   `json:"notification_chat_enabled"`
					Theme                     string `json:"theme"`
					DisplayDensity            string `json:"display_density"`
					MapDefaultView            string `json:"map_default_view"`
					MapShowLabels             bool   `json:"map_show_labels"`
					ReportDisplayPreference   string `json:"report_display_preference"`
				} `json:"preferences"`
			} `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &res)

		p := res.Data.Preferences
		if p.NotificationSoundEnabled != false {
			t.Errorf("expected sound false, got true")
		}
		if p.NotificationReportEnabled != false {
			t.Errorf("expected report false, got true")
		}
		if p.NotificationStatusEnabled != false {
			t.Errorf("expected status false, got true")
		}
		if p.NotificationChatEnabled != false {
			t.Errorf("expected chat false, got true")
		}
		if p.Theme != "dark" {
			t.Errorf("expected theme dark, got %s", p.Theme)
		}
		if p.DisplayDensity != "compact" {
			t.Errorf("expected display_density compact, got %s", p.DisplayDensity)
		}
		if p.MapDefaultView != "satellite" {
			t.Errorf("expected map_default_view satellite, got %s", p.MapDefaultView)
		}
		if p.MapShowLabels != false {
			t.Errorf("expected map_show_labels false, got true")
		}
		if p.ReportDisplayPreference != "compact" {
			t.Errorf("expected report_display_preference compact, got %s", p.ReportDisplayPreference)
		}
	}
}

func TestSettings_PartialUpdatePreservesOtherValues(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for PartialUpdate test")
	}

	_, token, cleanup := createTestUserWithWilayah(t, models.RoleAdminPemdes)
	defer cleanup()

	r := setupSettingsTestRouter()

	// Langkah 1: Set theme = dark dan density = compact
	{
		payload := `{"theme": "dark", "display_density": "compact", "notification_sound_enabled": false}`
		req, _ := http.NewRequest(http.MethodPut, "/api/settings", bytes.NewBufferString(payload))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("setup PUT expected 200, got %d", w.Code)
		}
	}

	// Langkah 2: Hanya ubah map_default_view menjadi satellite (field lain tidak dikirim)
	{
		partialPayload := `{"map_default_view": "satellite"}`
		req, _ := http.NewRequest(http.MethodPut, "/api/settings", bytes.NewBufferString(partialPayload))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("partial PUT expected 200, got %d", w.Code)
		}

		var res struct {
			Data struct {
				Preferences struct {
					NotificationSoundEnabled bool   `json:"notification_sound_enabled"`
					Theme                    string `json:"theme"`
					DisplayDensity           string `json:"display_density"`
					MapDefaultView           string `json:"map_default_view"`
				} `json:"preferences"`
			} `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &res)

		// Verifikasi field yang diubah berubah
		if res.Data.Preferences.MapDefaultView != "satellite" {
			t.Errorf("expected map_default_view satellite, got %s", res.Data.Preferences.MapDefaultView)
		}
		// Verifikasi field lama TIDAK ter-reset ke default
		if res.Data.Preferences.Theme != "dark" {
			t.Errorf("PARTIAL UPDATE LEAK: theme was reset from dark to %s", res.Data.Preferences.Theme)
		}
		if res.Data.Preferences.DisplayDensity != "compact" {
			t.Errorf("PARTIAL UPDATE LEAK: display_density was reset from compact to %s", res.Data.Preferences.DisplayDensity)
		}
		if res.Data.Preferences.NotificationSoundEnabled != false {
			t.Errorf("PARTIAL UPDATE LEAK: notification_sound_enabled was reset to true")
		}
	}
}

func TestSettings_EnumValidations(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for EnumValidations test")
	}

	_, token, cleanup := createTestUserWithWilayah(t, models.RoleAdminPemdes)
	defer cleanup()

	r := setupSettingsTestRouter()

	testCases := []struct {
		name    string
		payload string
		field   string
	}{
		{
			name:    "Invalid Theme -> 400",
			payload: `{"theme": "neon_blue"}`,
			field:   "theme",
		},
		{
			name:    "Invalid DisplayDensity -> 400",
			payload: `{"display_density": "super_wide"}`,
			field:   "display_density",
		},
		{
			name:    "Invalid MapDefaultView -> 400",
			payload: `{"map_default_view": "hybrid_3d"}`,
			field:   "map_default_view",
		},
		{
			name:    "Invalid ReportDisplayPreference -> 400",
			payload: `{"report_display_preference": "ultra_dense"}`,
			field:   "report_display_preference",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPut, "/api/settings", bytes.NewBufferString(tc.payload))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("[%s] expected 400, got %d: %s", tc.name, w.Code, w.Body.String())
			}

			var res map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &res)
			if res["status"] != "error" {
				t.Errorf("expected error status, got %v", res["status"])
			}
		})
	}
}

func TestSettings_SecurityPrivilegeEscalationPrevented(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for Security tests")
	}

	user, token, cleanup := createTestUserWithWilayah(t, models.RoleAdminPemdes)
	defer cleanup()

	r := setupSettingsTestRouter()

	// Upaya jahat: mengirim payload berbahaya mencoba mengubah role menjadi super_admin,
	// wilayah_id menjadi 999, email menjadi hacker@roadis.id, password, dan user_id
	maliciousPayload := `{
		"user_id": 99999,
		"notification_sound_enabled": false,
		"role": "super_admin",
		"wilayah_id": 9999,
		"email": "hacked_email@roadis.id",
		"name": "Hacked Name",
		"password": "new_plaintext_password"
	}`

	req, _ := http.NewRequest(http.MethodPut, "/api/settings", bytes.NewBufferString(maliciousPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (graceful ignore of unknown fields), got %d: %s", w.Code, w.Body.String())
	}

	// Verifikasi di database: data user tidak berubah sama sekali!
	var refreshedUser models.User
	config.DB.First(&refreshedUser, user.ID)

	if refreshedUser.Role != models.RoleAdminPemdes {
		t.Fatalf("PRIVILEGE ESCALATION LEAK: user role changed to %s!", refreshedUser.Role)
	}
	if refreshedUser.Email != user.Email {
		t.Fatalf("INTEGRITY LEAK: user email changed to %s!", refreshedUser.Email)
	}
	if refreshedUser.Name != user.Name {
		t.Fatalf("INTEGRITY LEAK: user name changed to %s!", refreshedUser.Name)
	}
	if refreshedUser.WilayahID == nil || *refreshedUser.WilayahID != *user.WilayahID {
		t.Fatalf("INTEGRITY LEAK: user wilayah_id changed!")
	}

	// Verifikasi response tidak membocorkan password
	bodyStr := w.Body.String()
	if strings.Contains(bodyStr, "password") || strings.Contains(bodyStr, "new_plaintext_password") {
		t.Errorf("SECURITY LEAK: password appeared in response: %s", bodyStr)
	}
}

func TestSettings_InvalidPayload(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for InvalidPayload tests")
	}

	_, token, cleanup := createTestUserWithWilayah(t, models.RoleAdminPemdes)
	defer cleanup()

	r := setupSettingsTestRouter()

	// Malformed JSON -> 400
	payload := `{"theme": `
	req, _ := http.NewRequest(http.MethodPut, "/api/settings", bytes.NewBufferString(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for malformed json, got %d", w.Code)
	}
}

func TestSettings_IDORProtection(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for IDOR test")
	}

	userA, tokenA, cleanupA := createTestUserWithWilayah(t, models.RoleAdminPemdes)
	defer cleanupA()

	_, tokenB, cleanupB := createTestUserWithWilayah(t, models.RoleAdminPemdes)
	defer cleanupB()

	r := setupSettingsTestRouter()

	// User A set preference theme to dark
	{
		payload := `{"theme": "dark", "notification_sound_enabled": false}`
		req, _ := http.NewRequest(http.MethodPut, "/api/settings", bytes.NewBufferString(payload))
		req.Header.Set("Authorization", "Bearer "+tokenA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("User A PUT expected 200, got %d", w.Code)
		}
	}

	// User B set preference theme to light
	{
		payload := `{"theme": "light", "notification_sound_enabled": true}`
		req, _ := http.NewRequest(http.MethodPut, "/api/settings", bytes.NewBufferString(payload))
		req.Header.Set("Authorization", "Bearer "+tokenB)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("User B PUT expected 200, got %d", w.Code)
		}
	}

	// User A GET settings -> harus tetap dark dan false
	{
		req, _ := http.NewRequest(http.MethodGet, "/api/settings", nil)
		req.Header.Set("Authorization", "Bearer "+tokenA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var res struct {
			Data struct {
				Account struct {
					ID uint `json:"id"`
				} `json:"account"`
				Preferences struct {
					NotificationSoundEnabled bool   `json:"notification_sound_enabled"`
					Theme                    string `json:"theme"`
				} `json:"preferences"`
			} `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &res)

		if res.Data.Account.ID != userA.ID {
			t.Errorf("IDOR LEAK: expected User A ID %d, got %d", userA.ID, res.Data.Account.ID)
		}
		if res.Data.Preferences.Theme != "dark" {
			t.Errorf("IDOR LEAK: User A theme was overwritten by User B!")
		}
		if res.Data.Preferences.NotificationSoundEnabled != false {
			t.Errorf("IDOR LEAK: User A sound was overwritten by User B!")
		}
	}
}

func TestSettings_LogoutAllSessions(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for LogoutAllSessions test")
	}

	userA, tokenA, cleanupA := createTestUserWithWilayah(t, models.RoleAdminPemdes)
	defer cleanupA()

	_, tokenB, cleanupB := createTestUserWithWilayah(t, models.RoleAdminPemdes)
	defer cleanupB()

	r := setupSettingsTestRouter()

	// 1. User A panggil GET /api/settings dengan token aktif -> 200 OK
	{
		req, _ := http.NewRequest(http.MethodGet, "/api/settings", nil)
		req.Header.Set("Authorization", "Bearer "+tokenA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("initial GET expected 200, got %d", w.Code)
		}
	}

	// 2. User A panggil POST /api/settings/logout-all -> 200 OK
	{
		req, _ := http.NewRequest(http.MethodPost, "/api/settings/logout-all", nil)
		req.Header.Set("Authorization", "Bearer "+tokenA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("logout-all expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var res map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &res)
		if res["status"] != "success" {
			t.Errorf("expected status success, got %v", res["status"])
		}
	}

	// 3. User A mencoba menggunakan token lama -> HARUS 401 Unauthorized (token revoked)
	{
		req, _ := http.NewRequest(http.MethodGet, "/api/settings", nil)
		req.Header.Set("Authorization", "Bearer "+tokenA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("revoked token expected 401 Unauthorized, got %d: %s", w.Code, w.Body.String())
		}
	}

	// 4. Verifikasi bahwa logout-all User A TIDAK membatalkan token User B (sesi User B tetap valid)
	{
		req, _ := http.NewRequest(http.MethodGet, "/api/settings", nil)
		req.Header.Set("Authorization", "Bearer "+tokenB)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("User B token should remain valid (200), got %d: %s", w.Code, w.Body.String())
		}
	}

	// 5. User A login kembali dan memperoleh token baru -> token baru bekerja normal (200 OK)
	{
		var refreshedUser models.User
		config.DB.First(&refreshedUser, userA.ID)
		newToken, err := utils.GenerateToken(refreshedUser.ID, refreshedUser.Email, refreshedUser.Role, refreshedUser.WilayahID, refreshedUser.TokenVersion)
		if err != nil {
			t.Fatalf("GenerateToken failed for new session: %v", err)
		}

		req, _ := http.NewRequest(http.MethodGet, "/api/settings", nil)
		req.Header.Set("Authorization", "Bearer "+newToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("new token expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}
	}
}

func TestSettings_SystemInfo(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for SystemInfo test")
	}

	_, token, cleanup := createTestUserWithWilayah(t, models.RoleAdminPemdes)
	defer cleanup()

	r := setupSettingsTestRouter()

	req, _ := http.NewRequest(http.MethodGet, "/api/system/info", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var res struct {
		Status string `json:"status"`
		Data   struct {
			Application string `json:"application"`
			Version     string `json:"version"`
			Environment string `json:"environment"`
			Description string `json:"description"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}

	if res.Status != "success" {
		t.Errorf("expected status 'success', got %s", res.Status)
	}
	if res.Data.Application != "ROADIS" {
		t.Errorf("expected application 'ROADIS', got %s", res.Data.Application)
	}
	if res.Data.Version == "" {
		t.Errorf("expected non-empty version")
	}
	if !strings.Contains(res.Data.Description, "YOLOv11") {
		t.Errorf("expected description to describe ROADIS YOLOv11")
	}

	// Pastikan tidak membocorkan credential
	bodyStr := w.Body.String()
	if strings.Contains(bodyStr, "password") || strings.Contains(bodyStr, "JWT_SECRET") {
		t.Errorf("system info leaked sensitive secrets: %s", bodyStr)
	}
}
