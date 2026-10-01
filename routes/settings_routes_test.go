package routes_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/routes"

	"github.com/gin-gonic/gin"
)

func TestHTTPIntegration_SettingsEndpoints(t *testing.T) {
	if !ensureDBForRoutesTest(t) {
		t.Skip("MySQL not available for live routes HTTP integration test")
	}

	r := gin.New()
	routes.SetupRoutes(r)

	// 1. GET /api/health (public)
	t.Run("GET /api/health", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/health", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var res map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &res)
		if res["status"] != "ok" || res["database"] != "connected" {
			t.Fatalf("unexpected health response: %v", res)
		}
	})

	// 2. Login as Admin Pemdes
	var token string
	t.Run("POST /api/login (Admin Pemdes)", func(t *testing.T) {
		loginPayload := map[string]string{
			"email":    "adminpemdes@gmail.com",
			"password": "12345678",
		}
		body, _ := json.Marshal(loginPayload)
		req, _ := http.NewRequest(http.MethodPost, "/api/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("login failed (%d): %s", w.Code, w.Body.String())
		}

		var loginRes struct {
			Token string `json:"token"`
		}
		json.Unmarshal(w.Body.Bytes(), &loginRes)
		if loginRes.Token == "" {
			t.Fatalf("expected non-empty token")
		}
		token = loginRes.Token
	})

	if token == "" {
		t.Fatal("cannot proceed with authenticated routes test without token")
	}

	// 3. GET /api/settings initial
	t.Run("GET /api/settings initial", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/settings", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var res struct {
			Data struct {
				Account struct {
					Email  string `json:"email"`
					Role   string `json:"role"`
					Status string `json:"status"`
				} `json:"account"`
				Preferences struct {
					NotificationSoundEnabled  bool   `json:"notification_sound_enabled"`
					NotificationReportEnabled bool   `json:"notification_report_enabled"`
					Theme                     string `json:"theme"`
					DisplayDensity            string `json:"display_density"`
					MapDefaultView            string `json:"map_default_view"`
					ReportDisplayPreference   string `json:"report_display_preference"`
				} `json:"preferences"`
				Security struct {
					HasActiveSession bool `json:"has_active_session"`
				} `json:"security"`
			} `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &res)
		if res.Data.Account.Role != string(models.RoleAdminPemdes) {
			t.Errorf("expected role admin_pemdes, got %s", res.Data.Account.Role)
		}
		if res.Data.Account.Status != "active" {
			t.Errorf("expected status active, got %s", res.Data.Account.Status)
		}
		if !res.Data.Security.HasActiveSession {
			t.Errorf("expected has_active_session true")
		}
	})

	// 4. Persistence Test: Update multiple preferences and verify
	t.Run("PUT /api/settings update preferences", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]interface{}{
			"notification_sound_enabled":  false,
			"notification_report_enabled": false,
			"theme":                       "dark",
			"display_density":             "compact",
			"map_default_view":            "satellite",
			"map_show_labels":             false,
			"report_display_preference":   "compact",
		})
		req, _ := http.NewRequest(http.MethodPut, "/api/settings", bytes.NewBuffer(payload))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("PUT expected 200, got %d: %s", w.Code, w.Body.String())
		}

		// GET to verify persistence
		reqGet, _ := http.NewRequest(http.MethodGet, "/api/settings", nil)
		reqGet.Header.Set("Authorization", "Bearer "+token)
		wGet := httptest.NewRecorder()
		r.ServeHTTP(wGet, reqGet)

		var resGet struct {
			Data struct {
				Preferences struct {
					NotificationSoundEnabled  bool   `json:"notification_sound_enabled"`
					NotificationReportEnabled bool   `json:"notification_report_enabled"`
					Theme                     string `json:"theme"`
					DisplayDensity            string `json:"display_density"`
					MapDefaultView            string `json:"map_default_view"`
					MapShowLabels             bool   `json:"map_show_labels"`
					ReportDisplayPreference   string `json:"report_display_preference"`
				} `json:"preferences"`
			} `json:"data"`
		}
		json.Unmarshal(wGet.Body.Bytes(), &resGet)
		p := resGet.Data.Preferences
		if p.NotificationSoundEnabled != false || p.Theme != "dark" || p.MapDefaultView != "satellite" {
			t.Errorf("unexpected preferences after update: %+v", p)
		}
	})

	// 5. POST /api/settings/logout-all and verify token revocation
	t.Run("POST /api/settings/logout-all and token revocation", func(t *testing.T) {
		reqLogout, _ := http.NewRequest(http.MethodPost, "/api/settings/logout-all", nil)
		reqLogout.Header.Set("Authorization", "Bearer "+token)
		wLogout := httptest.NewRecorder()
		r.ServeHTTP(wLogout, reqLogout)

		if wLogout.Code != http.StatusOK {
			t.Fatalf("logout-all expected 200, got %d: %s", wLogout.Code, wLogout.Body.String())
		}

		// Old token must now be rejected with 401 Unauthorized
		reqWithOldToken, _ := http.NewRequest(http.MethodGet, "/api/settings", nil)
		reqWithOldToken.Header.Set("Authorization", "Bearer "+token)
		wWithOldToken := httptest.NewRecorder()
		r.ServeHTTP(wWithOldToken, reqWithOldToken)

		if wWithOldToken.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for revoked token, got %d: %s", wWithOldToken.Code, wWithOldToken.Body.String())
		}

		// Re-login to get fresh token with new token_version
		loginPayload := map[string]string{
			"email":    "adminpemdes@gmail.com",
			"password": "12345678",
		}
		body, _ := json.Marshal(loginPayload)
		reqReLogin, _ := http.NewRequest(http.MethodPost, "/api/login", bytes.NewBuffer(body))
		reqReLogin.Header.Set("Content-Type", "application/json")
		wReLogin := httptest.NewRecorder()
		r.ServeHTTP(wReLogin, reqReLogin)

		if wReLogin.Code != http.StatusOK {
			t.Fatalf("re-login failed: %s", wReLogin.Body.String())
		}

		var reLoginRes struct {
			Token string `json:"token"`
		}
		json.Unmarshal(wReLogin.Body.Bytes(), &reLoginRes)

		// Fresh token must succeed (200 OK)
		reqNewToken, _ := http.NewRequest(http.MethodGet, "/api/settings", nil)
		reqNewToken.Header.Set("Authorization", "Bearer "+reLoginRes.Token)
		wNewToken := httptest.NewRecorder()
		r.ServeHTTP(wNewToken, reqNewToken)

		if wNewToken.Code != http.StatusOK {
			t.Fatalf("new token expected 200, got %d: %s", wNewToken.Code, wNewToken.Body.String())
		}

		// Update token variable for subsequent tests
		token = reLoginRes.Token
	})

	// 6. GET /api/system/info
	t.Run("GET /api/system/info", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/system/info", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var res struct {
			Data struct {
				Application string `json:"application"`
				Version     string `json:"version"`
			} `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &res)
		if res.Data.Application != "ROADIS" {
			t.Errorf("expected application ROADIS, got %s", res.Data.Application)
		}
	})

	// Cleanup test preference for seeded user
	var seededAdmin models.User
	if err := config.DB.Where("email = ?", "adminpemdes@gmail.com").First(&seededAdmin).Error; err == nil {
		config.DB.Unscoped().Where("user_id = ?", seededAdmin.ID).Delete(&models.UserPreference{})
	}
}
