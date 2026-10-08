package controllers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/routes"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupBE13Router() *gin.Engine {
	r := gin.New()
	routes.SetupRoutes(r)
	return r
}

func makeBE13Token(userID uint, role string, exp time.Duration) string {
	claims := utils.JWTClaim{
		UserID: userID,
		Email:  "test_be13@roadis.local",
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

// 1. Path Parameter: Invalid IDs
func TestBE13_InvalidIDs(t *testing.T) {
	r := setupBE13Router()
	superToken := makeBE13Token(1, "super_admin", time.Hour)
	wargaToken := makeBE13Token(10, "warga", time.Hour)
	puToken := makeBE13Token(2, "admin_pu", time.Hour)

	invalidIDValues := []string{"abc", "-1", "0", "1.5", "%201%20", "@#$"}

	endpoints := []struct {
		name   string
		method string
		pathF  string
		token  string
	}{
		{"SuperadminShowUser", http.MethodGet, "/api/superadmin/users/%s", superToken},
		{"SuperadminUpdateUser", http.MethodPut, "/api/superadmin/users/%s", superToken},
		{"SuperadminDeleteUser", http.MethodDelete, "/api/superadmin/users/%s", superToken},
		{"SuperadminShowWilayah", http.MethodGet, "/api/superadmin/wilayah/%s", superToken},
		{"SuperadminUpdateWilayah", http.MethodPut, "/api/superadmin/wilayah/%s", superToken},
		{"SuperadminDeleteWilayah", http.MethodDelete, "/api/superadmin/wilayah/%s", superToken},
		{"SuperadminDeleteSpam", http.MethodDelete, "/api/superadmin/laporan/%s", superToken},
		{"WargaGetLaporan", http.MethodGet, "/api/warga/laporan/%s", wargaToken},
		{"WargaGetChat", http.MethodGet, "/api/warga/laporan/%s/chat", wargaToken},
		{"WargaSendChat", http.MethodPost, "/api/warga/laporan/%s/chat", wargaToken},
		{"AdminGetLaporan", http.MethodGet, "/api/admin/laporan/%s", puToken},
		{"AdminUpdateStatus", http.MethodPut, "/api/admin/laporan/%s/status", puToken},
		{"AdminReplyChat", http.MethodPut, "/api/admin/chat/%s", puToken},
		{"NotifikasiMarkRead", http.MethodPut, "/api/notifikasi/%s/read", wargaToken},
		{"NotifikasiDelete", http.MethodDelete, "/api/notifikasi/%s", wargaToken},
	}

	for _, ep := range endpoints {
		for _, invID := range invalidIDValues {
			testName := fmt.Sprintf("%s_%s", ep.name, invID)
			t.Run(testName, func(t *testing.T) {
				url := fmt.Sprintf(ep.pathF, invID)
				req := httptest.NewRequest(ep.method, url, bytes.NewBufferString("{}"))
				req.Header.Set("Authorization", "Bearer "+ep.token)
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				if w.Code != http.StatusBadRequest {
					t.Errorf("[%s] expected 400 Bad Request for ID '%s', got %d: %s", ep.name, invID, w.Code, w.Body.String())
				}
			})
		}
	}
}

// 2. Path Parameter: Overflow IDs
func TestBE13_OverflowIDs(t *testing.T) {
	r := setupBE13Router()
	superToken := makeBE13Token(1, "super_admin", time.Hour)
	wargaToken := makeBE13Token(10, "warga", time.Hour)
	puToken := makeBE13Token(2, "admin_pu", time.Hour)

	overflowID := "99999999999999999999999999999999999999999999999999"

	endpoints := []struct {
		name   string
		method string
		pathF  string
		token  string
	}{
		{"SuperadminShowUser", http.MethodGet, "/api/superadmin/users/%s", superToken},
		{"SuperadminShowWilayah", http.MethodGet, "/api/superadmin/wilayah/%s", superToken},
		{"WargaGetLaporan", http.MethodGet, "/api/warga/laporan/%s", wargaToken},
		{"AdminGetLaporan", http.MethodGet, "/api/admin/laporan/%s", puToken},
		{"NotifikasiMarkRead", http.MethodPut, "/api/notifikasi/%s/read", wargaToken},
	}

	for _, ep := range endpoints {
		t.Run("Overflow_"+ep.name, func(t *testing.T) {
			url := fmt.Sprintf(ep.pathF, overflowID)
			req := httptest.NewRequest(ep.method, url, nil)
			req.Header.Set("Authorization", "Bearer "+ep.token)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("[%s] expected 400 Bad Request on integer overflow ID, got %d: %s", ep.name, w.Code, w.Body.String())
			}
		})
	}
}

// 3. Enum Validation
func TestBE13_InvalidEnums(t *testing.T) {
	r := setupBE13Router()
	superToken := makeBE13Token(1, "super_admin", time.Hour)
	puToken := makeBE13Token(2, "admin_pu", time.Hour)

	// A. Role Enum
	t.Run("CreateUser_InvalidRoleEnum", func(t *testing.T) {
		body := `{"name":"Tester","email":"enum_test@roadis.local","password":"password123","role":"superuser"}`
		req := httptest.NewRequest(http.MethodPost, "/api/superadmin/users", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+superToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for invalid role enum, got %d: %s", w.Code, w.Body.String())
		}
	})

	// B. Wilayah Tipe Enum
	t.Run("CreateWilayah_InvalidTipeEnum", func(t *testing.T) {
		body := `{"nama":"Galaksi Bimasakti","tipe":"galaksi"}`
		req := httptest.NewRequest(http.MethodPost, "/api/superadmin/wilayah", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+superToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for invalid wilayah tipe enum, got %d: %s", w.Code, w.Body.String())
		}
	})

	// C. Status Enum
	t.Run("UpdateLaporan_InvalidStatusEnum", func(t *testing.T) {
		// Mock laporan update with status 'approved'
		body := `{"status":"approved"}`
		req := httptest.NewRequest(http.MethodPut, "/api/admin/laporan/1/status", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+puToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		// It either returns 400 (invalid status) or 404 (not found). It must NOT accept 'approved'.
		if w.Code == http.StatusOK {
			t.Errorf("status 'approved' must not be accepted, got %d", w.Code)
		}
	})
}

// 4. Malformed JSON
func TestBE13_MalformedJSON(t *testing.T) {
	r := setupBE13Router()
	superToken := makeBE13Token(1, "super_admin", time.Hour)
	wargaToken := makeBE13Token(10, "warga", time.Hour)

	malformedBodies := []string{
		`{broken json`,
		`{"test":`,
		`{"theme": 12345, "name": 12345, "pesan": 12345, "email": 12345, "nama": 12345}`,
		``,
	}

	endpoints := []struct {
		name   string
		method string
		path   string
		token  string
	}{
		{"Register", http.MethodPost, "/api/register", ""},
		{"Login", http.MethodPost, "/api/login", ""},
		{"CreateUser", http.MethodPost, "/api/superadmin/users", superToken},
		{"CreateWilayah", http.MethodPost, "/api/superadmin/wilayah", superToken},
		{"UpdateProfile", http.MethodPut, "/api/profile", wargaToken},
		{"UpdateSettings", http.MethodPut, "/api/settings", wargaToken},
		{"SendChat", http.MethodPost, "/api/warga/laporan/1/chat", wargaToken},
	}

	for _, ep := range endpoints {
		for i, mal := range malformedBodies {
			t.Run(fmt.Sprintf("%s_Malformed_%d", ep.name, i), func(t *testing.T) {
				req := httptest.NewRequest(ep.method, ep.path, bytes.NewBufferString(mal))
				if ep.token != "" {
					req.Header.Set("Authorization", "Bearer "+ep.token)
				}
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				if w.Code != http.StatusBadRequest {
					t.Errorf("[%s] expected 400 Bad Request on malformed JSON, got %d: %s", ep.name, w.Code, w.Body.String())
				}
			})
		}
	}
}

// 5. Error Envelope Consistency
func TestBE13_ErrorEnvelope(t *testing.T) {
	r := setupBE13Router()
	wargaToken := makeBE13Token(10, "warga", time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/api/warga/laporan/abc", nil)
	req.Header.Set("Authorization", "Bearer "+wargaToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected valid JSON error envelope: %v", err)
	}

	// Envelope must have message or error
	_, hasMessage := resp["message"]
	_, hasError := resp["error"]
	if !hasMessage && !hasError {
		t.Errorf("error envelope missing both 'message' and 'error' keys: %v", resp)
	}
}

// 6. Record Not Found Handling (404)
func TestBE13_GORMNotFound(t *testing.T) {
	r := setupBE13Router()
	superToken := makeBE13Token(1, "super_admin", time.Hour)

	// Valid numeric ID that definitely doesn't exist
	nonExistentID := "999999999"

	endpoints := []struct {
		name   string
		method string
		path   string
	}{
		{"ShowUser", http.MethodGet, "/api/superadmin/users/" + nonExistentID},
		{"ShowWilayah", http.MethodGet, "/api/superadmin/wilayah/" + nonExistentID},
	}

	for _, ep := range endpoints {
		t.Run("NotFound_"+ep.name, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			req.Header.Set("Authorization", "Bearer "+superToken)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// When DB is available, non-existent record returns 404.
			// When DB is nil (standalone), returns 500.
			// Neither should ever return 200 OK.
			if w.Code == http.StatusOK {
				t.Errorf("[%s] non-existent record must not return 200 OK", ep.name)
			}
		})
	}
}

// 7. Database Error Safety: No SQL / Table Leaks
func TestBE13_DBErrorSafety(t *testing.T) {
	r := setupBE13Router()
	wargaToken := makeBE13Token(10, "warga", time.Hour)

	// Save original DB and set to nil to simulate DB outage
	origDB := config.DB
	config.DB = nil
	defer func() { config.DB = origDB }()

	req := httptest.NewRequest(http.MethodGet, "/api/notifikasi", nil)
	req.Header.Set("Authorization", "Bearer "+wargaToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body := w.Body.String()
	sensitivePatterns := []string{"SELECT", "FROM", "WHERE", "SQL", "gorm", "table", "mysql", "connectex"}
	for _, pat := range sensitivePatterns {
		if strings.Contains(strings.ToUpper(body), pat) {
			t.Errorf("database error response leaks internal DB keyword '%s': %s", pat, body)
		}
	}
}

// 8. Nil Safety: No Panics When DB is Nil
func TestBE13_NilSafety(t *testing.T) {
	r := setupBE13Router()
	superToken := makeBE13Token(1, "super_admin", time.Hour)
	wargaToken := makeBE13Token(10, "warga", time.Hour)
	puToken := makeBE13Token(2, "admin_pu", time.Hour)

	origDB := config.DB
	config.DB = nil
	defer func() { config.DB = origDB }()

	endpoints := []struct {
		name   string
		method string
		path   string
		token  string
		body   string
	}{
		{"Register", http.MethodPost, "/api/register", "", `{"name":"A","email":"a@b.com","password":"secret"}`},
		{"Login", http.MethodPost, "/api/login", "", `{"email":"a@b.com","password":"secret"}`},
		{"WargaPeta", http.MethodGet, "/api/warga/laporan/peta", wargaToken, ""},
		{"WargaRiwayat", http.MethodGet, "/api/warga/laporan/riwayat", wargaToken, ""},
		{"AdminDashboard", http.MethodGet, "/api/admin/dashboard", puToken, ""},
		{"AdminLaporan", http.MethodGet, "/api/admin/laporan", puToken, ""},
		{"AdminMap", http.MethodGet, "/api/admin/map/laporan", puToken, ""},
		{"AdminInbox", http.MethodGet, "/api/admin/chat", puToken, ""},
		{"SuperadminUsers", http.MethodGet, "/api/superadmin/users", superToken, ""},
		{"SuperadminWilayah", http.MethodGet, "/api/superadmin/wilayah", superToken, ""},
		{"GetNotifikasi", http.MethodGet, "/api/notifikasi", wargaToken, ""},
		{"GetProfile", http.MethodGet, "/api/profile", wargaToken, ""},
		{"GetSettings", http.MethodGet, "/api/settings", wargaToken, ""},
	}

	for _, ep := range endpoints {
		t.Run("NilSafety_"+ep.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("[%s] PANIC occurred when config.DB == nil: %v", ep.name, r)
				}
			}()

			var req *http.Request
			if ep.body != "" {
				req = httptest.NewRequest(ep.method, ep.path, bytes.NewBufferString(ep.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(ep.method, ep.path, nil)
			}
			if ep.token != "" {
				req.Header.Set("Authorization", "Bearer "+ep.token)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// Graceful error response (500), never panic or 200
			if w.Code != http.StatusInternalServerError {
				t.Errorf("[%s] expected 500 Internal Server Error when DB is nil, got %d: %s", ep.name, w.Code, w.Body.String())
			}
		})
	}
}

// 9. Unknown Role Fail-Closed
func TestBE13_UnknownRole(t *testing.T) {
	r := setupBE13Router()
	unknownRoleToken := makeBE13Token(10, "rogue_admin", time.Hour)

	endpoints := []struct {
		name   string
		method string
		path   string
	}{
		{"WargaRoute", http.MethodGet, "/api/warga/laporan"},
		{"AdminRoute", http.MethodGet, "/api/admin/dashboard"},
		{"SuperadminRoute", http.MethodGet, "/api/superadmin/users"},
		{"ProfileRoute", http.MethodGet, "/api/profile"},
		{"NotifikasiRoute", http.MethodGet, "/api/notifikasi"},
	}

	for _, ep := range endpoints {
		t.Run("FailClosed_"+ep.name, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			req.Header.Set("Authorization", "Bearer "+unknownRoleToken)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("[%s] expected 403 Forbidden for unknown role claim, got %d: %s", ep.name, w.Code, w.Body.String())
			}
		})
	}
}

// 10. Validation Before Side Effect
func TestBE13_ValidationBeforeSideEffect(t *testing.T) {
	r := setupBE13Router()
	wargaToken := makeBE13Token(10, "warga", time.Hour)

	// A. Missing photo field in CreateLaporan must fail before any DB/Cloudinary action
	t.Run("CreateLaporan_MissingPhotoFailsBeforeSideEffect", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("judul", "Lubang Besar")
		_ = writer.WriteField("deskripsi", "Jalan rusak parah")
		_ = writer.WriteField("tipe_kerusakan", "lubang")
		_ = writer.WriteField("latitude", "-6.3265")
		_ = writer.WriteField("longitude", "108.3245")
		_ = writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/warga/laporan", body)
		req.Header.Set("Authorization", "Bearer "+wargaToken)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request when photo missing, got %d: %s", w.Code, w.Body.String())
		}
	})

	// B. Invalid file format in CreateLaporan must fail before any DB/Cloudinary action
	t.Run("CreateLaporan_PDFRejectedBeforeSideEffect", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("judul", "Lubang Besar")
		_ = writer.WriteField("deskripsi", "Jalan rusak parah")
		_ = writer.WriteField("tipe_kerusakan", "lubang")
		_ = writer.WriteField("latitude", "-6.3265")
		_ = writer.WriteField("longitude", "108.3245")

		part, _ := writer.CreateFormFile("foto", "document.pdf")
		_, _ = part.Write([]byte("%PDF-1.4 file content"))
		_ = writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/warga/laporan", body)
		req.Header.Set("Authorization", "Bearer "+wargaToken)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for PDF file upload, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// 11. Pagination Security
func TestBE13_Pagination(t *testing.T) {
	r := setupBE13Router()
	wargaToken := makeBE13Token(10, "warga", time.Hour)

	paginationQueries := []string{
		"?page=0&limit=0",
		"?page=-5&limit=-10",
		"?page=abc&limit=xyz",
		"?page=1&limit=9999999",
	}

	for _, q := range paginationQueries {
		t.Run("Pagination_Notifikasi_"+q, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/notifikasi"+q, nil)
			req.Header.Set("Authorization", "Bearer "+wargaToken)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// When DB is available or nil, pagination query normalization must never crash
			if w.Code == http.StatusBadRequest {
				t.Logf("Pagination was rejected with 400 or normalized")
			}
		})
	}
}

// 12. Error Privacy: No Hash / Secret Leakage
func TestBE13_ErrorPrivacy(t *testing.T) {
	r := setupBE13Router()
	superToken := makeBE13Token(1, "super_admin", time.Hour)

	// Send an intentionally malformed request to Superadmin user endpoint
	req := httptest.NewRequest(http.MethodPost, "/api/superadmin/users", bytes.NewBufferString(`{"role":"invalid"}`))
	req.Header.Set("Authorization", "Bearer "+superToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	respBody := w.Body.String()
	secrets := []string{"$2a$", "JWT_SECRET", "cloudinary://", "password_hash"}
	for _, s := range secrets {
		if strings.Contains(respBody, s) {
			t.Errorf("error response leaked sensitive token/credential string '%s': %s", s, respBody)
		}
	}
}
