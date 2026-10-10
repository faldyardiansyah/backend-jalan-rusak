package middlewares

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// IsValidOrigin memvalidasi format string origin sesuai RFC 6454 (scheme://host[:port] tanpa path, query, atau fragment).
func IsValidOrigin(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "*" {
		return false
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if u.Host == "" {
		return false
	}
	// Origin tidak boleh memiliki path (misal: /api, /path, atau trailing slash /), query, fragment, atau userinfo
	if u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return false
	}
	return true
}

// ValidateCORSConfig memvalidasi kesiapan konfigurasi CORS.
// Pada mode production (APP_ENV=production), allowlist wajib dikonfigurasi dan setiap origin harus valid.
func ValidateCORSConfig() error {
	appEnv := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))

	envOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if envOrigins == "" {
		envOrigins = os.Getenv("ALLOWED_ORIGINS")
	}

	if appEnv == "production" {
		if strings.TrimSpace(envOrigins) == "" {
			return errors.New("CORS configuration error: CORS_ALLOWED_ORIGINS must be set in production mode")
		}

		parts := strings.Split(envOrigins, ",")
		validCount := 0
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed == "" {
				continue
			}
			if !IsValidOrigin(trimmed) {
				return fmt.Errorf("CORS configuration error: invalid origin format '%s' in CORS_ALLOWED_ORIGINS (must be scheme://host[:port] without path)", trimmed)
			}
			validCount++
		}

		if validCount == 0 {
			return errors.New("CORS configuration error: CORS_ALLOWED_ORIGINS does not contain any valid origin")
		}
	} else {
		// Pada mode development, jika ada origin yang dikonfigurasi, validasi formatnya
		if strings.TrimSpace(envOrigins) != "" {
			for _, p := range strings.Split(envOrigins, ",") {
				trimmed := strings.TrimSpace(p)
				if trimmed != "" && !IsValidOrigin(trimmed) {
					return fmt.Errorf("CORS configuration error: invalid origin format '%s' (must be scheme://host[:port] without path)", trimmed)
				}
			}
		}
	}

	return nil
}

// isOriginAllowed memeriksa apakah origin yang dikirim termasuk dalam daftar yang diizinkan.
func isOriginAllowed(origin string) bool {
	if !IsValidOrigin(origin) {
		return false
	}
	normalized := strings.ToLower(strings.TrimSpace(origin))

	// 1. Periksa konfigurasi origin dari environment variable (CORS_ALLOWED_ORIGINS / ALLOWED_ORIGINS)
	envOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if envOrigins == "" {
		envOrigins = os.Getenv("ALLOWED_ORIGINS")
	}
	if envOrigins != "" {
		for _, o := range strings.Split(envOrigins, ",") {
			trimmed := strings.TrimSpace(o)
			if IsValidOrigin(trimmed) && strings.ToLower(trimmed) == normalized {
				return true
			}
		}
	}

	// 2. Pada mode non-production (development & test), izinkan origin dev frontend lokal
	if strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV"))) != "production" {
		devOrigins := []string{
			"http://localhost:5173",
			"http://127.0.0.1:5173",
			"http://localhost:3000",
			"http://127.0.0.1:3000",
			"http://localhost:8080",
			"http://127.0.0.1:8080",
		}
		for _, dev := range devOrigins {
			if dev == normalized {
				return true
			}
		}
	}

	return false
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Selalu terapkan security headers
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Set("X-Frame-Options", "DENY")
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")

		if origin != "" {
			if isOriginAllowed(origin) {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
				c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
				c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")
				c.Writer.Header().Set("Vary", "Origin")

				if c.Request.Method == "OPTIONS" {
					c.AbortWithStatus(http.StatusNoContent)
					return
				}
			} else {
				// Origin tidak dikenal / tidak diizinkan: tolak preflight, jangan kirim header CORS allow
				if c.Request.Method == "OPTIONS" {
					c.AbortWithStatus(http.StatusForbidden)
					return
				}
			}
		} else {
			// Request tanpa origin (misal: mobile app native, cURL, server-to-server)
			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
		}

		c.Next()
	}
}