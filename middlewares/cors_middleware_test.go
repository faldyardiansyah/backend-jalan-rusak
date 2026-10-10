package middlewares_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"backend-jalan-rusak/middlewares"
	"backend-jalan-rusak/routes"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupCORSTestRouter() *gin.Engine {
	r := gin.New()
	r.Use(middlewares.CORSMiddleware())
	r.GET("/test-endpoint", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	return r
}

// 1. Allowed Origin in Development Mode (e.g. Vite dev on localhost:5173)
func TestCORS_AllowedOrigin_Development(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

	r := setupCORSTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/test-endpoint", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	allowOrigin := w.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin != "http://localhost:5173" {
		t.Errorf("expected Access-Control-Allow-Origin to be 'http://localhost:5173', got '%s'", allowOrigin)
	}

	credentials := w.Header().Get("Access-Control-Allow-Credentials")
	if credentials != "true" {
		t.Errorf("expected Access-Control-Allow-Credentials to be 'true', got '%s'", credentials)
	}

	vary := w.Header().Get("Vary")
	if vary != "Origin" {
		t.Errorf("expected Vary header to be 'Origin', got '%s'", vary)
	}
}

// 2. Alternative Allowed Dev Origins
func TestCORS_AlternativeDevOrigins(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

	r := setupCORSTestRouter()
	validOrigins := []string{
		"http://127.0.0.1:5173",
		"http://localhost:3000",
		"http://127.0.0.1:3000",
		"http://localhost:8080",
		"http://127.0.0.1:8080",
	}

	for _, origin := range validOrigins {
		t.Run(origin, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test-endpoint", nil)
			req.Header.Set("Origin", origin)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200 OK for %s, got %d", origin, w.Code)
			}
			if w.Header().Get("Access-Control-Allow-Origin") != origin {
				t.Errorf("expected Allow-Origin %s, got %s", origin, w.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

// 3. Untrusted / Malicious Origin must NOT be allowed (No reflection, No credentials)
func TestCORS_UntrustedOrigin_Rejected(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

	r := setupCORSTestRouter()
	untrustedOrigins := []string{
		"https://evil.com",
		"http://attacker.example.org",
		"http://localhost.attacker.com",
		"http://evil-localhost:5173",
	}

	for _, origin := range untrustedOrigins {
		t.Run(origin, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test-endpoint", nil)
			req.Header.Set("Origin", origin)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			allowOrigin := w.Header().Get("Access-Control-Allow-Origin")
			if allowOrigin != "" {
				t.Errorf("expected NO Access-Control-Allow-Origin for untrusted %s, got '%s'", origin, allowOrigin)
			}

			credentials := w.Header().Get("Access-Control-Allow-Credentials")
			if credentials == "true" {
				t.Errorf("expected NO Access-Control-Allow-Credentials for untrusted %s, got '%s'", origin, credentials)
			}
		})
	}
}

// 4. Preflight OPTIONS for Allowed Origin
func TestCORS_Preflight_AllowedOrigin(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

	r := setupCORSTestRouter()
	req := httptest.NewRequest(http.MethodOptions, "/test-endpoint", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content for allowed preflight, got %d", w.Code)
	}

	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("expected Allow-Origin header on preflight, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Errorf("expected Access-Control-Allow-Methods header on preflight")
	}
	if w.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Errorf("expected Access-Control-Allow-Headers header on preflight")
	}
}

// 5. Preflight OPTIONS for Untrusted Origin must return 403 Forbidden without CORS allow headers
func TestCORS_Preflight_UntrustedOrigin_Forbidden(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

	r := setupCORSTestRouter()
	req := httptest.NewRequest(http.MethodOptions, "/test-endpoint", nil)
	req.Header.Set("Origin", "https://malicious-website.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for untrusted preflight, got %d", w.Code)
	}

	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("untrusted preflight must NOT return Access-Control-Allow-Origin, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

// 6. Request without Origin header (e.g. native mobile app, cURL, server-to-server)
func TestCORS_NoOriginHeader_NormalProcessing(t *testing.T) {
	t.Setenv("APP_ENV", "development")

	r := setupCORSTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/test-endpoint", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("request without Origin must NOT have Access-Control-Allow-Origin, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Errorf("request without Origin must NOT have Access-Control-Allow-Credentials")
	}

	// Security headers must still be set
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("expected X-Content-Type-Options: nosniff")
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("expected X-Frame-Options: DENY")
	}
	if w.Header().Get("X-XSS-Protection") != "1; mode=block" {
		t.Errorf("expected X-XSS-Protection: 1; mode=block")
	}
}

// 7. Environment Configured Origin (CORS_ALLOWED_ORIGINS / ALLOWED_ORIGINS)
func TestCORS_EnvironmentConfiguredOrigin(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://roadis.indramayukab.go.id, https://admin.roadis.indramayukab.go.id")

	r := setupCORSTestRouter()

	// A. Configured Production Origin 1 -> Allowed
	t.Run("Configured Production Origin 1", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test-endpoint", nil)
		req.Header.Set("Origin", "https://roadis.indramayukab.go.id")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if w.Header().Get("Access-Control-Allow-Origin") != "https://roadis.indramayukab.go.id" {
			t.Errorf("expected configured origin to be allowed, got '%s'", w.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	// B. Configured Production Origin 2 -> Allowed
	t.Run("Configured Production Origin 2", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test-endpoint", nil)
		req.Header.Set("Origin", "https://admin.roadis.indramayukab.go.id")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if w.Header().Get("Access-Control-Allow-Origin") != "https://admin.roadis.indramayukab.go.id" {
			t.Errorf("expected configured origin to be allowed, got '%s'", w.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	// C. Unconfigured Origin under Production -> Blocked
	t.Run("Unconfigured Origin in Production", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test-endpoint", nil)
		req.Header.Set("Origin", "https://attacker.org")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("unconfigured origin must be blocked in production, got '%s'", w.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	// D. Localhost in Production -> Blocked because not in explicit allowlist
	t.Run("Localhost in Production without explicit env entry", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test-endpoint", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("localhost origin must be blocked in production if not in CORS_ALLOWED_ORIGINS, got '%s'", w.Header().Get("Access-Control-Allow-Origin"))
		}
	})
}

// 8. Origin Format Validation (RFC 6454 scheme://host[:port] without path/query/fragment)
func TestCORS_OriginFormatValidation(t *testing.T) {
	validOrigins := []string{
		"http://localhost",
		"http://localhost:5173",
		"https://example.com",
		"https://example.com:8443",
		"https://roadis.indramayukab.go.id",
		"http://127.0.0.1:3000",
	}

	for _, o := range validOrigins {
		if !middlewares.IsValidOrigin(o) {
			t.Errorf("expected '%s' to be recognized as VALID origin format", o)
		}
	}

	invalidOrigins := []string{
		"",
		"   ",
		"*",
		"example.com",                  // missing scheme
		"ftp://example.com",            // non-http scheme
		"https://",                     // empty host
		"https://example.com/api",      // contains path
		"https://example.com/",         // trailing slash path
		"https://example.com?query=1",  // contains query
		"https://example.com#section",  // contains fragment
		"https://user:pass@example.com",// contains credentials
	}

	for _, o := range invalidOrigins {
		if middlewares.IsValidOrigin(o) {
			t.Errorf("expected '%s' to be recognized as INVALID origin format", o)
		}
	}
}

// 9. Production Guard: ValidateCORSConfig fail-closed checks
func TestCORS_ValidateCORSConfig_ProductionGuard(t *testing.T) {
	t.Run("Production with valid allowlist -> PASS", func(t *testing.T) {
		t.Setenv("APP_ENV", "production")
		t.Setenv("CORS_ALLOWED_ORIGINS", "https://frontend.roadis.local, https://admin.roadis.local")

		if err := middlewares.ValidateCORSConfig(); err != nil {
			t.Errorf("expected valid production allowlist to pass, got error: %v", err)
		}
	})

	t.Run("Production with empty allowlist -> FAIL (must return clear error)", func(t *testing.T) {
		t.Setenv("APP_ENV", "production")
		t.Setenv("CORS_ALLOWED_ORIGINS", "")
		t.Setenv("ALLOWED_ORIGINS", "")

		err := middlewares.ValidateCORSConfig()
		if err == nil {
			t.Errorf("expected error when CORS_ALLOWED_ORIGINS is empty in production, got nil")
		}
	})

	t.Run("Production with whitespace-only allowlist -> FAIL", func(t *testing.T) {
		t.Setenv("APP_ENV", "production")
		t.Setenv("CORS_ALLOWED_ORIGINS", "   ,   ")

		err := middlewares.ValidateCORSConfig()
		if err == nil {
			t.Errorf("expected error when CORS_ALLOWED_ORIGINS is only whitespace/commas, got nil")
		}
	})

	t.Run("Production with invalid origin containing path -> FAIL", func(t *testing.T) {
		t.Setenv("APP_ENV", "production")
		t.Setenv("CORS_ALLOWED_ORIGINS", "https://valid.roadis.local, https://invalid.roadis.local/api")

		err := middlewares.ValidateCORSConfig()
		if err == nil {
			t.Errorf("expected error when an origin in CORS_ALLOWED_ORIGINS contains a path, got nil")
		}
	})

	t.Run("Production with wildcard origin -> FAIL", func(t *testing.T) {
		t.Setenv("APP_ENV", "production")
		t.Setenv("CORS_ALLOWED_ORIGINS", "*")

		err := middlewares.ValidateCORSConfig()
		if err == nil {
			t.Errorf("expected error when CORS_ALLOWED_ORIGINS is wildcard '*', got nil")
		}
	})

	t.Run("Development mode without production config -> PASS", func(t *testing.T) {
		t.Setenv("APP_ENV", "development")
		t.Setenv("CORS_ALLOWED_ORIGINS", "")
		t.Setenv("ALLOWED_ORIGINS", "")

		if err := middlewares.ValidateCORSConfig(); err != nil {
			t.Errorf("expected development mode to pass without production allowlist, got error: %v", err)
		}
	})

	t.Run("Development mode with invalid origin format -> FAIL", func(t *testing.T) {
		t.Setenv("APP_ENV", "development")
		t.Setenv("CORS_ALLOWED_ORIGINS", "https://invalid.roadis.local/path")

		err := middlewares.ValidateCORSConfig()
		if err == nil {
			t.Errorf("expected development mode to reject invalid origin format, got nil")
		}
	})
}

// 10. Full Application Router Integration Verification
func TestCORS_FullAppRouterIntegration(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

	r := gin.New()
	r.Use(middlewares.CORSMiddleware())
	routes.SetupRoutes(r)

	// Test public health endpoint through full CORS middleware
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/health, got %d", w.Code)
	}

	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("expected full app router to apply CORS allow origin for http://localhost:5173")
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("expected credentials true")
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("expected X-Frame-Options DENY")
	}
}
