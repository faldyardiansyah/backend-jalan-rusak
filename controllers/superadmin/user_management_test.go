package superadmin

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
		t.Logf("MySQL connection unavailable in test (%v)", err)
		return false
	}

	config.DB = db
	return true
}

func setupSuperadminRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	api := r.Group("/api")
	api.Use(middlewares.AuthMiddleware())
	superadmin := api.Group("/superadmin")
	superadmin.Use(middlewares.RequireRole("super_admin"))
	{
		superadmin.GET("/users", GetAllUsers)
		superadmin.POST("/users", CreateUser)
		superadmin.GET("/users/:id", ShowUser)
		superadmin.PUT("/users/:id", UpdateUser)
		superadmin.DELETE("/users/:id", DeleteUser)
	}
	return r
}

// 1. Authorization Matrix Test for All Roles
func TestUserManagement_AuthorizationMatrix_AllRoles(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for User Management tests")
	}

	t.Setenv("JWT_SECRET", "superadmin_test_secret_key_1234567890")
	r := setupSuperadminRouter()

	// Fixtures
	now := time.Now().UnixNano()
	superadminUser := models.User{
		Name:         "Super Admin Live",
		Email:        fmt.Sprintf("super_%d@roadis.local", now),
		Password:     "hashedpassword123",
		Role:         models.RoleSuperAdmin,
		TokenVersion: 1,
	}
	config.DB.Create(&superadminUser)
	defer config.DB.Unscoped().Delete(&superadminUser)

	adminPuUser := models.User{
		Name:         "Admin PU Live",
		Email:        fmt.Sprintf("pu_%d@roadis.local", now),
		Password:     "hashedpassword123",
		Role:         models.RoleAdminPu,
		TokenVersion: 1,
	}
	config.DB.Create(&adminPuUser)
	defer config.DB.Unscoped().Delete(&adminPuUser)

	adminPemdesUser := models.User{
		Name:         "Admin Pemdes Live",
		Email:        fmt.Sprintf("pemdes_%d@roadis.local", now),
		Password:     "hashedpassword123",
		Role:         models.RoleAdminPemdes,
		TokenVersion: 1,
	}
	config.DB.Create(&adminPemdesUser)
	defer config.DB.Unscoped().Delete(&adminPemdesUser)

	wargaUser := models.User{
		Name:         "Warga Live",
		Email:        fmt.Sprintf("warga_%d@roadis.local", now),
		Password:     "hashedpassword123",
		Role:         models.RoleWarga,
		TokenVersion: 1,
	}
	config.DB.Create(&wargaUser)
	defer config.DB.Unscoped().Delete(&wargaUser)

	tokenSuper, _ := utils.GenerateToken(superadminUser.ID, superadminUser.Email, superadminUser.Role, nil, 1)
	tokenPU, _ := utils.GenerateToken(adminPuUser.ID, adminPuUser.Email, adminPuUser.Role, nil, 1)
	tokenPemdes, _ := utils.GenerateToken(adminPemdesUser.ID, adminPemdesUser.Email, adminPemdesUser.Role, nil, 1)
	tokenWarga, _ := utils.GenerateToken(wargaUser.ID, wargaUser.Email, wargaUser.Role, nil, 1)

	// Test A: Superadmin -> 200 OK
	t.Run("Superadmin allowed GET /api/superadmin/users", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/superadmin/users", nil)
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK for superadmin, got %d: %s", w.Code, w.Body.String())
		}
	})

	// Test B: Admin PU -> 403 Forbidden
	t.Run("Admin PU forbidden on /api/superadmin/users", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/superadmin/users", nil)
		req.Header.Set("Authorization", "Bearer "+tokenPU)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for Admin PU, got %d", w.Code)
		}
	})

	// Test C: Admin Pemdes -> 403 Forbidden
	t.Run("Admin Pemdes forbidden on /api/superadmin/users", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/superadmin/users", nil)
		req.Header.Set("Authorization", "Bearer "+tokenPemdes)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for Admin Pemdes, got %d", w.Code)
		}
	})

	// Test D: Warga -> 403 Forbidden
	t.Run("Warga forbidden on /api/superadmin/users", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/superadmin/users", nil)
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for Warga, got %d", w.Code)
		}
	})

	// Test E: Unauthenticated -> 401 Unauthorized
	t.Run("Unauthenticated rejected with 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/superadmin/users", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized without token, got %d", w.Code)
		}
	})
}

// 2. User Create Audit & Validation
func TestUserManagement_CreateUser_ValidationAndSecurity(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for User Management tests")
	}

	t.Setenv("JWT_SECRET", "superadmin_test_secret_key_1234567890")
	r := setupSuperadminRouter()

	now := time.Now().UnixNano()
	superadminUser := models.User{
		Name:         "Super Admin Creator",
		Email:        fmt.Sprintf("creator_%d@roadis.local", now),
		Password:     "hashedpassword123",
		Role:         models.RoleSuperAdmin,
		TokenVersion: 1,
	}
	config.DB.Create(&superadminUser)
	defer config.DB.Unscoped().Delete(&superadminUser)

	wilayah := models.Wilayah{
		Nama: fmt.Sprintf("Desa Create Test %d", now),
		Tipe: "desa",
	}
	config.DB.Create(&wilayah)
	defer config.DB.Unscoped().Delete(&wilayah)

	tokenSuper, _ := utils.GenerateToken(superadminUser.ID, superadminUser.Email, superadminUser.Role, nil, 1)

	// A. Valid User Creation
	t.Run("Valid User Creation and Password Hashing", func(t *testing.T) {
		targetEmail := fmt.Sprintf("new_user_%d@roadis.local", time.Now().UnixNano())
		payload := map[string]interface{}{
			"name":     "Petugas Baru",
			"email":    targetEmail,
			"password": "secretpassword123",
			"role":     "admin_pu",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/superadmin/users", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		respStr := w.Body.String()
		// Sensitive field exclusion
		if strings.Contains(respStr, "secretpassword123") {
			t.Errorf("plaintext password leaked in create response: %s", respStr)
		}
		if strings.Contains(respStr, `"password":`) {
			t.Errorf("password field must not be present in response JSON: %s", respStr)
		}
		if strings.Contains(respStr, `"token_version":`) {
			t.Errorf("token_version must not be present in response JSON: %s", respStr)
		}

		// Verify in DB
		var created models.User
		if err := config.DB.Where("email = ?", targetEmail).First(&created).Error; err != nil {
			t.Fatalf("created user not found in DB: %v", err)
		}
		defer config.DB.Unscoped().Delete(&created)

		if err := bcrypt.CompareHashAndPassword([]byte(created.Password), []byte("secretpassword123")); err != nil {
			t.Errorf("password was not properly hashed with bcrypt: %v", err)
		}
	})

	// B. Duplicate Email Validation
	t.Run("Duplicate email rejected with 400", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":     "Duplicate Attempt",
			"email":    superadminUser.Email, // Email already taken
			"password": "secretpassword123",
			"role":     "admin_pu",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/superadmin/users", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for duplicate email, got %d: %s", w.Code, w.Body.String())
		}
	})

	// C. Invalid Email Format
	t.Run("Invalid email format rejected with 400", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":     "Invalid Email",
			"email":    "not-an-email",
			"password": "secretpassword123",
			"role":     "admin_pu",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/superadmin/users", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid email, got %d", w.Code)
		}
	})

	// D. Invalid Role
	t.Run("Invalid role rejected with 400", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":     "Invalid Role",
			"email":    fmt.Sprintf("invrole_%d@roadis.local", time.Now().UnixNano()),
			"password": "secretpassword123",
			"role":     "hacker_role",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/superadmin/users", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid role, got %d", w.Code)
		}
	})

	// E. Admin Pemdes without Wilayah
	t.Run("Admin Pemdes without Wilayah rejected with 400", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":     "Pemdes No Wilayah",
			"email":    fmt.Sprintf("pemdesnowil_%d@roadis.local", time.Now().UnixNano()),
			"password": "secretpassword123",
			"role":     "admin_pemdes",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/superadmin/users", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for Admin Pemdes without wilayah, got %d", w.Code)
		}
	})

	// F. Non-Pemdes assigned Wilayah
	t.Run("Non-Pemdes with Wilayah rejected with 400", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":       "PU With Wilayah",
			"email":      fmt.Sprintf("puwithwil_%d@roadis.local", time.Now().UnixNano()),
			"password":   "secretpassword123",
			"role":       "admin_pu",
			"wilayah_id": wilayah.ID,
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/superadmin/users", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for non-pemdes with wilayah, got %d", w.Code)
		}
	})
}

// 3. User Read Audit & Sensitive Data Exclusion
func TestUserManagement_ReadUsers_PrivacyAndExclusions(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for User Management tests")
	}

	t.Setenv("JWT_SECRET", "superadmin_test_secret_key_1234567890")
	r := setupSuperadminRouter()

	now := time.Now().UnixNano()
	superadminUser := models.User{
		Name:         "Super Admin Reader",
		Email:        fmt.Sprintf("reader_%d@roadis.local", now),
		Password:     "hashedpassword123",
		Role:         models.RoleSuperAdmin,
		TokenVersion: 1,
	}
	config.DB.Create(&superadminUser)
	defer config.DB.Unscoped().Delete(&superadminUser)

	targetUser := models.User{
		Name:         "Target User Info",
		Email:        fmt.Sprintf("target_%d@roadis.local", now),
		Password:     "supersecret123",
		Role:         models.RoleWarga,
		TokenVersion: 5,
	}
	config.DB.Create(&targetUser)
	defer config.DB.Unscoped().Delete(&targetUser)

	tokenSuper, _ := utils.GenerateToken(superadminUser.ID, superadminUser.Email, superadminUser.Role, nil, 1)

	// A. List Users: No password or token_version in response
	t.Run("GET /users omits sensitive fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/superadmin/users?page=1&limit=10", nil)
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		bodyStr := w.Body.String()
		if strings.Contains(bodyStr, "supersecret123") || strings.Contains(bodyStr, `"password":`) {
			t.Errorf("password leaked in user list JSON: %s", bodyStr)
		}
		if strings.Contains(bodyStr, `"token_version":`) {
			t.Errorf("token_version leaked in user list JSON: %s", bodyStr)
		}
	})

	// B. Show User: No password or token_version in response
	t.Run("GET /users/:id returns user without credentials", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/superadmin/users/%d", targetUser.ID), nil)
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		bodyStr := w.Body.String()
		if strings.Contains(bodyStr, "supersecret123") || strings.Contains(bodyStr, `"password":`) {
			t.Errorf("password leaked in show user JSON: %s", bodyStr)
		}
		if strings.Contains(bodyStr, `"token_version":`) {
			t.Errorf("token_version leaked in show user JSON: %s", bodyStr)
		}
	})

	// C. Nonexistent User returns 404
	t.Run("GET /users/:id with invalid ID returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/superadmin/users/99999999", nil)
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 for nonexistent user, got %d", w.Code)
		}
	})
}

// 4. User Update Audit & Token Invalidation
func TestUserManagement_UpdateUser_ValidationAndSecurity(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for User Management tests")
	}

	t.Setenv("JWT_SECRET", "superadmin_test_secret_key_1234567890")
	r := setupSuperadminRouter()

	now := time.Now().UnixNano()
	superadminUser := models.User{
		Name:         "Super Admin Updater",
		Email:        fmt.Sprintf("updater_%d@roadis.local", now),
		Password:     "hashedpassword123",
		Role:         models.RoleSuperAdmin,
		TokenVersion: 1,
	}
	config.DB.Create(&superadminUser)
	defer config.DB.Unscoped().Delete(&superadminUser)

	targetUser := models.User{
		Name:         "User To Update",
		Email:        fmt.Sprintf("toupdate_%d@roadis.local", now),
		Password:     "oldpassword123",
		Role:         models.RoleAdminPu,
		TokenVersion: 1,
	}
	config.DB.Create(&targetUser)
	defer config.DB.Unscoped().Delete(&targetUser)

	tokenSuper, _ := utils.GenerateToken(superadminUser.ID, superadminUser.Email, superadminUser.Role, nil, 1)

	// A. Valid Update & Password Hash & Token Invalidation
	t.Run("Update password hashes properly and increments token_version", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":     "Updated Name PU",
			"password": "newfreshpassword123",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/superadmin/users/%d", targetUser.ID), bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var updated models.User
		config.DB.First(&updated, targetUser.ID)

		if updated.Name != "Updated Name PU" {
			t.Errorf("expected name to be updated, got: %s", updated.Name)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(updated.Password), []byte("newfreshpassword123")); err != nil {
			t.Errorf("new password was not hashed properly: %v", err)
		}
		if updated.TokenVersion <= targetUser.TokenVersion {
			t.Errorf("expected token_version to increment on password update, got %d", updated.TokenVersion)
		}
	})

	// B. Duplicate Email on Update
	t.Run("Updating to existing email rejected with 400", func(t *testing.T) {
		payload := map[string]interface{}{
			"email": superadminUser.Email,
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/superadmin/users/%d", targetUser.ID), bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for duplicate email update, got %d", w.Code)
		}
	})

	// C. Non-Pemdes assigned Wilayah on Update
	t.Run("Assigning wilayah to non-pemdes on update rejected with 400", func(t *testing.T) {
		dummyWilayahID := uint(1)
		payload := map[string]interface{}{
			"wilayah_id": &dummyWilayahID,
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/superadmin/users/%d", targetUser.ID), bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 when assigning wilayah to PU role, got %d", w.Code)
		}
	})
}

// 5. User Delete Audit & Protections
func TestUserManagement_DeleteUser_SecurityAndProtections(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for User Management tests")
	}

	t.Setenv("JWT_SECRET", "superadmin_test_secret_key_1234567890")
	r := setupSuperadminRouter()

	now := time.Now().UnixNano()
	superadminUser := models.User{
		Name:         "Super Admin Deleter",
		Email:        fmt.Sprintf("deleter_%d@roadis.local", now),
		Password:     "hashedpassword123",
		Role:         models.RoleSuperAdmin,
		TokenVersion: 1,
	}
	config.DB.Create(&superadminUser)
	defer config.DB.Unscoped().Delete(&superadminUser)

	adminToDelete := models.User{
		Name:         "Admin PU To Delete",
		Email:        fmt.Sprintf("delpu_%d@roadis.local", now),
		Password:     "hashedpassword123",
		Role:         models.RoleAdminPu,
		TokenVersion: 1,
	}
	config.DB.Create(&adminToDelete)
	defer config.DB.Unscoped().Delete(&adminToDelete)

	wargaToDelete := models.User{
		Name:         "Warga Protected",
		Email:        fmt.Sprintf("wargaprot_%d@roadis.local", now),
		Password:     "hashedpassword123",
		Role:         models.RoleWarga,
		TokenVersion: 1,
	}
	config.DB.Create(&wargaToDelete)
	defer config.DB.Unscoped().Delete(&wargaToDelete)

	tokenSuper, _ := utils.GenerateToken(superadminUser.ID, superadminUser.Email, superadminUser.Role, nil, 1)

	// A. Self-Deletion Protection
	t.Run("Superadmin cannot delete themselves -> 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/superadmin/users/%d", superadminUser.ID), nil)
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 on self-delete attempt, got %d: %s", w.Code, w.Body.String())
		}
	})

	// B. Citizen (Warga) Deletion Protection
	t.Run("Cannot delete warga via superadmin user management -> 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/superadmin/users/%d", wargaToDelete.ID), nil)
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 when attempting to delete warga, got %d: %s", w.Code, w.Body.String())
		}
	})

	// C. Valid Deletion of Admin & Soft Delete Verification
	t.Run("Valid delete soft-deletes user and blocks future access", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/superadmin/users/%d", adminToDelete.ID), nil)
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 on valid delete, got %d: %s", w.Code, w.Body.String())
		}

		// Verify record is soft deleted in DB
		var checkActive models.User
		if err := config.DB.Where("id = ? AND deleted_at IS NULL", adminToDelete.ID).First(&checkActive).Error; err == nil {
			t.Errorf("user must be soft-deleted and not found with deleted_at IS NULL")
		}

		// Verify GET /users/:id returns 404
		reqGet := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/superadmin/users/%d", adminToDelete.ID), nil)
		reqGet.Header.Set("Authorization", "Bearer "+tokenSuper)
		wGet := httptest.NewRecorder()
		r.ServeHTTP(wGet, reqGet)

		if wGet.Code != http.StatusNotFound {
			t.Errorf("expected 404 when querying soft-deleted user, got %d", wGet.Code)
		}
	})
}

// 6. IDOR & Privilege Escalation Resistance
func TestUserManagement_IDOR_And_CrossRoleAttacks(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for User Management tests")
	}

	t.Setenv("JWT_SECRET", "superadmin_test_secret_key_1234567890")
	r := setupSuperadminRouter()

	now := time.Now().UnixNano()
	adminPU := models.User{
		Name:         "Attacker PU",
		Email:        fmt.Sprintf("attpu_%d@roadis.local", now),
		Password:     "hashedpassword123",
		Role:         models.RoleAdminPu,
		TokenVersion: 1,
	}
	config.DB.Create(&adminPU)
	defer config.DB.Unscoped().Delete(&adminPU)

	victimUser := models.User{
		Name:         "Victim Superadmin",
		Email:        fmt.Sprintf("victim_%d@roadis.local", now),
		Password:     "hashedpassword123",
		Role:         models.RoleSuperAdmin,
		TokenVersion: 1,
	}
	config.DB.Create(&victimUser)
	defer config.DB.Unscoped().Delete(&victimUser)

	tokenAttacker, _ := utils.GenerateToken(adminPU.ID, adminPU.Email, adminPU.Role, nil, 1)

	// Attempt PUT /api/superadmin/users/:id from non-superadmin -> 403 Forbidden
	t.Run("IDOR PUT attempt by non-superadmin blocked with 403", func(t *testing.T) {
		payload := map[string]interface{}{"name": "Hacked Name"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/superadmin/users/%d", victimUser.ID), bytes.NewBuffer(b))
		req.Header.Set("Authorization", "Bearer "+tokenAttacker)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for IDOR PUT, got %d", w.Code)
		}
	})

	// Attempt DELETE /api/superadmin/users/:id from non-superadmin -> 403 Forbidden
	t.Run("IDOR DELETE attempt by non-superadmin blocked with 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/superadmin/users/%d", victimUser.ID), nil)
		req.Header.Set("Authorization", "Bearer "+tokenAttacker)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for IDOR DELETE, got %d", w.Code)
		}
	})
}

// 7. Soft-Delete Login and List Exclusion
func TestUserManagement_SoftDelete_LoginAndListExclusion(t *testing.T) {
	if !ensureTestDB(t) {
		t.Skip("MySQL not available for User Management tests")
	}

	t.Setenv("JWT_SECRET", "superadmin_test_secret_key_1234567890")
	r := setupSuperadminRouter()

	now := time.Now().UnixNano()
	superadminUser := models.User{
		Name:         "Super Admin Softdel Test",
		Email:        fmt.Sprintf("superdel_%d@roadis.local", now),
		Password:     "hashedpassword123",
		Role:         models.RoleSuperAdmin,
		TokenVersion: 1,
	}
	config.DB.Create(&superadminUser)
	defer config.DB.Unscoped().Delete(&superadminUser)

	tokenSuper, _ := utils.GenerateToken(superadminUser.ID, superadminUser.Email, superadminUser.Role, nil, 1)

	// Create user with known password
	plainPassword := "loginsecret123"
	hashedPass, _ := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	testUser := models.User{
		Name:         "User To Soft Delete",
		Email:        fmt.Sprintf("softdeltest_%d@roadis.local", now),
		Password:     string(hashedPass),
		Role:         models.RoleAdminPu,
		TokenVersion: 1,
	}
	config.DB.Create(&testUser)
	defer config.DB.Unscoped().Delete(&testUser)

	// Soft-delete the user
	config.DB.Delete(&testUser)

	// 1. Verify soft-deleted user does NOT appear in GET /api/superadmin/users
	t.Run("Soft-deleted user not in GetAllUsers list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/superadmin/users?search="+testUser.Email, nil)
		req.Header.Set("Authorization", "Bearer "+tokenSuper)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		if strings.Contains(w.Body.String(), testUser.Email) {
			t.Errorf("soft-deleted user email %s found in GetAllUsers list: %s", testUser.Email, w.Body.String())
		}
	})

	// 2. Verify soft-deleted user CANNOT authenticate via AuthMiddleware
	t.Run("Soft-deleted user token rejected with 401 in AuthMiddleware", func(t *testing.T) {
		oldToken, _ := utils.GenerateToken(testUser.ID, testUser.Email, testUser.Role, nil, 1)
		req := httptest.NewRequest(http.MethodGet, "/api/superadmin/users", nil)
		req.Header.Set("Authorization", "Bearer "+oldToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized for soft-deleted user token, got %d", w.Code)
		}
	})
}
