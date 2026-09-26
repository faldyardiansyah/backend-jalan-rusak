package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
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
