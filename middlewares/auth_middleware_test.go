package middlewares

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestRouter() *gin.Engine {
	r := gin.New()
	return r
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	r := setupTestRouter()
	r.Use(AuthMiddleware())
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestAuthMiddleware_MalformedHeader(t *testing.T) {
	r := setupTestRouter()
	r.Use(AuthMiddleware())
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	testCases := []string{
		"Basic abcdef",
		"Bearer",
		"Bearer ",
		"InvalidPrefix 12345",
	}

	for _, tc := range testCases {
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", tc)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("for header %q, expected status %d, got %d", tc, http.StatusUnauthorized, w.Code)
		}
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "test_secret_for_middleware_test_12345")

	var wilayahID uint = 99
	userID := uint(5)
	email := "pemdes@roadis.id"
	role := models.RoleAdminPemdes

	token, err := utils.GenerateToken(userID, email, role, &wilayahID)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	r := setupTestRouter()
	r.Use(AuthMiddleware())

	var ctxUserID uint
	var ctxEmail string
	var ctxRole string
	var ctxWilayahID *uint

	r.GET("/protected", func(c *gin.Context) {
		uidVal, _ := c.Get("user_id")
		ctxUserID = uidVal.(uint)

		emVal, _ := c.Get("email")
		ctxEmail = emVal.(string)

		roVal, _ := c.Get("role")
		ctxRole = roVal.(string)

		wVal, _ := c.Get("wilayah_id")
		ctxWilayahID = wVal.(*uint)

		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	if ctxUserID != userID {
		t.Errorf("expected context user_id %d, got %d", userID, ctxUserID)
	}
	if ctxEmail != email {
		t.Errorf("expected context email %s, got %s", email, ctxEmail)
	}
	if ctxRole != string(role) {
		t.Errorf("expected context role %s, got %s", role, ctxRole)
	}
	if ctxWilayahID == nil || *ctxWilayahID != wilayahID {
		t.Errorf("expected context wilayah_id %d, got %v", wilayahID, ctxWilayahID)
	}
}

func TestRequireRole_Matrix(t *testing.T) {
	t.Setenv("JWT_SECRET", "test_secret_for_role_matrix_12345")

	type testCase struct {
		name         string
		role         models.UserRole
		allowedRoles []string
		expectedCode int
	}

	cases := []testCase{
		{
			name:         "Warga access warga endpoint",
			role:         models.RoleWarga,
			allowedRoles: []string{"warga"},
			expectedCode: http.StatusOK,
		},
		{
			name:         "Warga access admin endpoint -> 403",
			role:         models.RoleWarga,
			allowedRoles: []string{"admin_pemdes", "admin_pu", "super_admin"},
			expectedCode: http.StatusForbidden,
		},
		{
			name:         "Admin Pemdes access admin endpoint",
			role:         models.RoleAdminPemdes,
			allowedRoles: []string{"admin_pemdes", "admin_pu", "super_admin"},
			expectedCode: http.StatusOK,
		},
		{
			name:         "Admin Pemdes access warga endpoint -> 403",
			role:         models.RoleAdminPemdes,
			allowedRoles: []string{"warga"},
			expectedCode: http.StatusForbidden,
		},
		{
			name:         "Admin PU access admin endpoint",
			role:         models.RoleAdminPu,
			allowedRoles: []string{"admin_pemdes", "admin_pu", "super_admin"},
			expectedCode: http.StatusOK,
		},
		{
			name:         "Admin PU access superadmin endpoint -> 403",
			role:         models.RoleAdminPu,
			allowedRoles: []string{"super_admin"},
			expectedCode: http.StatusForbidden,
		},
		{
			name:         "Super Admin access superadmin endpoint",
			role:         models.RoleSuperAdmin,
			allowedRoles: []string{"super_admin"},
			expectedCode: http.StatusOK,
		},
		{
			name:         "Super Admin access admin endpoint",
			role:         models.RoleSuperAdmin,
			allowedRoles: []string{"admin_pemdes", "admin_pu", "super_admin"},
			expectedCode: http.StatusOK,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token, err := utils.GenerateToken(1, "user@roadis.id", tc.role, nil)
			if err != nil {
				t.Fatalf("GenerateToken failed: %v", err)
			}

			r := setupTestRouter()
			r.Use(AuthMiddleware())
			r.Use(RequireRole(tc.allowedRoles...))
			r.GET("/endpoint", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			req, _ := http.NewRequest(http.MethodGet, "/endpoint", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.expectedCode {
				t.Errorf("[%s] expected HTTP %d, got %d", tc.name, tc.expectedCode, w.Code)
			}
		})
	}
}

func TestRequireRole_Unauthenticated(t *testing.T) {
	r := setupTestRouter()
	// No AuthMiddleware, directly RequireRole
	r.Use(RequireRole("warga"))
	r.GET("/endpoint", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req, _ := http.NewRequest(http.MethodGet, "/endpoint", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected HTTP 401 when no auth middleware ran, got %d", w.Code)
	}
}

func ensureDBForMiddlewareTest(t *testing.T) bool {
	if config.DB != nil {
		_ = config.DB.AutoMigrate(&models.User{})
		return true
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		os.Getenv("DB_USER"), os.Getenv("DB_PASS"),
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)
	if os.Getenv("DB_HOST") == "" {
		dsn = "root:@tcp(127.0.0.1:3306)/db_jalan_rusak?charset=utf8mb4&parseTime=True&loc=Local"
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})
	if err != nil {
		t.Logf("MySQL connection unavailable for middleware test: %v", err)
		return false
	}

	_ = db.AutoMigrate(
		&models.User{},
		&models.LaporanKerusakan{},
		&models.RiwayatChat{},
		&models.Wilayah{},
		&models.Notifikasi{},
		&models.UserPreference{},
	)
	config.DB = db
	return true
}

func generateExpiredToken(userID uint, email string, role models.UserRole, secret string) (string, error) {
	claims := utils.JWTClaim{
		UserID:       userID,
		Email:        email,
		Role:         role,
		TokenVersion: 1,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// TestAuthMiddleware_SecurityScenarios covers minimal scenarios A through H:
// A. User aktif + JWT valid -> 200
// B. User aktif + token version berbeda -> 401
// C. User soft-deleted + JWT lama masih valid -> 401 (SEC-01)
// D. User tidak ditemukan -> 401
// E. JWT invalid -> 401
// F. JWT expired -> 401
// G. Missing Authorization header -> 401
// H. LogoutAll revocation flow -> 401
func TestAuthMiddleware_SecurityScenarios(t *testing.T) {
	testSecret := "security_hardening_test_secret_12345"
	t.Setenv("JWT_SECRET", testSecret)

	r := setupTestRouter()
	r.Use(AuthMiddleware())
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "user_id": c.MustGet("user_id")})
	})

	// Scenario E: JWT invalid signature / malformed
	t.Run("Scenario E: JWT invalid signature/format -> 401", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer invalid.malformed.token")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for invalid JWT, got %d", w.Code)
		}
	})

	// Scenario F: JWT expired
	t.Run("Scenario F: JWT expired -> 401", func(t *testing.T) {
		expToken, err := generateExpiredToken(99, "exp@roadis.id", models.RoleWarga, testSecret)
		if err != nil {
			t.Fatalf("failed to generate expired token: %v", err)
		}

		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+expToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for expired JWT, got %d", w.Code)
		}
	})

	// Scenario G: Missing Authorization header
	t.Run("Scenario G: Missing Authorization header -> 401", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for missing auth header, got %d", w.Code)
		}
	})

	// Database-dependent scenarios (A, B, C, D, H)
	if !ensureDBForMiddlewareTest(t) {
		t.Skip("MySQL not available for live DB auth middleware tests")
	}

	// Scenario A: User aktif + JWT valid -> 200
	t.Run("Scenario A: User aktif + JWT valid -> 200", func(t *testing.T) {
		activeUser := models.User{
			Name:         "Active User Test",
			Email:        fmt.Sprintf("active_%d@roadis.id", time.Now().UnixNano()),
			Role:         models.RoleWarga,
			TokenVersion: 1,
		}
		if err := config.DB.Create(&activeUser).Error; err != nil {
			t.Fatalf("failed to create active user: %v", err)
		}
		defer config.DB.Unscoped().Delete(&activeUser)

		token, err := utils.GenerateToken(activeUser.ID, activeUser.Email, activeUser.Role, nil, activeUser.TokenVersion)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 for active user with valid token, got %d: %s", w.Code, w.Body.String())
		}
	})

	// Scenario B: User aktif + token version berbeda -> 401
	t.Run("Scenario B: User aktif + token version berbeda -> 401", func(t *testing.T) {
		activeUser := models.User{
			Name:         "Version Mismatch User",
			Email:        fmt.Sprintf("mismatch_%d@roadis.id", time.Now().UnixNano()),
			Role:         models.RoleWarga,
			TokenVersion: 2,
		}
		if err := config.DB.Create(&activeUser).Error; err != nil {
			t.Fatalf("failed to create user: %v", err)
		}
		defer config.DB.Unscoped().Delete(&activeUser)

		// Token generated with outdated version 1 (while user has version 2)
		oldToken, err := utils.GenerateToken(activeUser.ID, activeUser.Email, activeUser.Role, nil, 1)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+oldToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for token version mismatch, got %d: %s", w.Code, w.Body.String())
		}
	})

	// Scenario C: User soft-deleted + JWT lama masih valid secara cryptographic -> 401 (SEC-01)
	t.Run("Scenario C: User soft-deleted + valid JWT -> 401 (SEC-01)", func(t *testing.T) {
		delUser := models.User{
			Name:         "Soft Deleted User",
			Email:        fmt.Sprintf("softdel_%d@roadis.id", time.Now().UnixNano()),
			Role:         models.RoleWarga,
			TokenVersion: 1,
		}
		if err := config.DB.Create(&delUser).Error; err != nil {
			t.Fatalf("failed to create user: %v", err)
		}
		defer config.DB.Unscoped().Delete(&delUser)

		// Token dibuat saat user masih aktif
		validToken, err := utils.GenerateToken(delUser.ID, delUser.Email, delUser.Role, nil, delUser.TokenVersion)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		// Soft-delete user
		if err := config.DB.Delete(&delUser).Error; err != nil {
			t.Fatalf("failed to soft delete user: %v", err)
		}

		// Pastikan user benar-benar ter-soft-delete
		var check models.User
		if err := config.DB.Where("id = ? AND deleted_at IS NULL", delUser.ID).First(&check).Error; err == nil {
			t.Fatal("expected user to be soft-deleted, but record still found with deleted_at IS NULL")
		}

		// Kirim request dengan token yang secara cryptographic valid
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+validToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("CRITICAL SEC-01 FAILING: expected 401 for soft-deleted user, got %d: %s", w.Code, w.Body.String())
		}
	})

	// Scenario D: User tidak ditemukan -> 401
	t.Run("Scenario D: User tidak ditemukan di DB -> 401", func(t *testing.T) {
		nonExistentID := uint(99999999)
		ghostToken, err := utils.GenerateToken(nonExistentID, "ghost@roadis.id", models.RoleWarga, nil, 1)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+ghostToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for non-existent user, got %d: %s", w.Code, w.Body.String())
		}
	})

	// Scenario H: LogoutAll -> token lama tetap ditolak
	t.Run("Scenario H: LogoutAll session revocation -> 401", func(t *testing.T) {
		logoutUser := models.User{
			Name:         "LogoutAll Test User",
			Email:        fmt.Sprintf("logout_%d@roadis.id", time.Now().UnixNano()),
			Role:         models.RoleWarga,
			TokenVersion: 1,
		}
		if err := config.DB.Create(&logoutUser).Error; err != nil {
			t.Fatalf("failed to create user: %v", err)
		}
		defer config.DB.Unscoped().Delete(&logoutUser)

		initialToken, err := utils.GenerateToken(logoutUser.ID, logoutUser.Email, logoutUser.Role, nil, 1)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		// 1. Verifikasi token awal bekerja
		req1, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req1.Header.Set("Authorization", "Bearer "+initialToken)
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)
		if w1.Code != http.StatusOK {
			t.Fatalf("expected initial token to return 200, got %d", w1.Code)
		}

		// 2. Simulasi LogoutAll: increment token_version di DB
		config.DB.Model(&models.User{}).Where("id = ?", logoutUser.ID).Update("token_version", gorm.Expr("token_version + 1"))

		// 3. Verifikasi token lama kini ditolak (401)
		req2, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req2.Header.Set("Authorization", "Bearer "+initialToken)
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		if w2.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 after LogoutAll revocation, got %d: %s", w2.Code, w2.Body.String())
		}
	})
}
