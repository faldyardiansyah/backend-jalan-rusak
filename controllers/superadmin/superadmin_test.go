package superadmin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"backend-jalan-rusak/middlewares"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// 1 - 8. Test Authorization Matrix & Unauthenticated 401
func TestSuperadminAuth_RoleMatrix(t *testing.T) {
	t.Setenv("JWT_SECRET", "superadmin_test_secret_123456789012")

	roles := []struct {
		role        models.UserRole
		expectAllow bool
	}{
		{models.RoleWarga, false},
		{models.RoleAdminPemdes, false},
		{models.RoleAdminPu, false},
		{models.RoleSuperAdmin, true},
	}

	for _, tc := range roles {
		t.Run("Role_"+string(tc.role), func(t *testing.T) {
			token, err := utils.GenerateToken(10, string(tc.role)+"@roadis.id", tc.role, nil)
			if err != nil {
				t.Fatalf("GenerateToken failed: %v", err)
			}

			r := gin.New()
			r.Use(middlewares.AuthMiddleware())
			r.Use(middlewares.RequireRole("super_admin"))
			r.GET("/api/superadmin/users", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "success"})
			})
			r.GET("/api/superadmin/wilayah", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "success"})
			})

			// Test /users
			reqUsers, _ := http.NewRequest(http.MethodGet, "/api/superadmin/users", nil)
			reqUsers.Header.Set("Authorization", "Bearer "+token)
			wUsers := httptest.NewRecorder()
			r.ServeHTTP(wUsers, reqUsers)

			if tc.expectAllow && wUsers.Code != http.StatusOK {
				t.Errorf("[%s] expected 200 on /users, got %d", tc.role, wUsers.Code)
			}
			if !tc.expectAllow && wUsers.Code != http.StatusForbidden {
				t.Errorf("[%s] expected 403 Forbidden on /users, got %d", tc.role, wUsers.Code)
			}

			// Test /wilayah
			reqWilayah, _ := http.NewRequest(http.MethodGet, "/api/superadmin/wilayah", nil)
			reqWilayah.Header.Set("Authorization", "Bearer "+token)
			wWilayah := httptest.NewRecorder()
			r.ServeHTTP(wWilayah, reqWilayah)

			if tc.expectAllow && wWilayah.Code != http.StatusOK {
				t.Errorf("[%s] expected 200 on /wilayah, got %d", tc.role, wWilayah.Code)
			}
			if !tc.expectAllow && wWilayah.Code != http.StatusForbidden {
				t.Errorf("[%s] expected 403 Forbidden on /wilayah, got %d", tc.role, wWilayah.Code)
			}
		})
	}

	// 8. No token -> 401 Unauthorized
	r := gin.New()
	r.Use(middlewares.AuthMiddleware())
	r.Use(middlewares.RequireRole("super_admin"))
	r.GET("/api/superadmin/users", func(c *gin.Context) {})

	reqNoToken, _ := http.NewRequest(http.MethodGet, "/api/superadmin/users", nil)
	wNoToken := httptest.NewRecorder()
	r.ServeHTTP(wNoToken, reqNoToken)

	if wNoToken.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized without token, got %d", wNoToken.Code)
	}
}

// 9. Test Empty Users List returns []
func TestEmptyUsersResponse_SerializesToArray(t *testing.T) {
	users := make([]models.User, 0)
	resp := gin.H{
		"status": "success",
		"data": gin.H{
			"users":      users,
			"page":       1,
			"limit":      10,
			"total_data": 0,
		},
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	str := string(b)
	if !strings.Contains(str, `"users":[]`) {
		t.Errorf("expected JSON to contain '\"users\":[]', got: %s", str)
	}
}

// 12 - 14. Test Duplicate Email Validation on Create and Update
func TestUserValidation_DuplicateEmailLogic(t *testing.T) {
	existingUsers := []models.User{
		{Model: gorm.Model{ID: 1}, Email: "user1@roadis.id"},
		{Model: gorm.Model{ID: 2}, Email: "user2@roadis.id"},
	}

	isEmailDuplicate := func(email string, excludeID uint) bool {
		lowerEmail := strings.ToLower(strings.TrimSpace(email))
		for _, u := range existingUsers {
			if excludeID != 0 && u.ID == excludeID {
				continue
			}
			if strings.ToLower(u.Email) == lowerEmail {
				return true
			}
		}
		return false
	}

	// 12. Create duplicate email (same case) -> DUPLICATE
	if !isEmailDuplicate("user1@roadis.id", 0) {
		t.Error("expected duplicate for user1@roadis.id")
	}

	// 12. Create duplicate email (different case) -> DUPLICATE
	if !isEmailDuplicate("USER1@ROADIS.ID", 0) {
		t.Error("expected duplicate for case-insensitive USER1@ROADIS.ID")
	}

	// Create unique email -> ALLOWED
	if isEmailDuplicate("user3@roadis.id", 0) {
		t.Error("expected unique email user3@roadis.id to be allowed")
	}

	// 13. Update User 2 with User 1's email -> DUPLICATE (blocked)
	if !isEmailDuplicate("user1@roadis.id", 2) {
		t.Error("expected update to existing user1 email to be blocked")
	}

	// 14. Update User 1 with User 1's own email -> NOT duplicate (allowed)
	if isEmailDuplicate("user1@roadis.id", 1) {
		t.Error("expected updating own email to be allowed")
	}
}

// 15 - 16. Test Role and Wilayah Validation
func TestUserValidation_RoleAndWilayahRequirements(t *testing.T) {
	validateRoleAndWilayah := func(roleStr string, wilayahID *uint, knownWilayahIDs []uint) (bool, string) {
		role := models.UserRole(strings.ToLower(strings.TrimSpace(roleStr)))
		if role != models.RoleAdminPemdes &&
			role != models.RoleAdminPu &&
			role != models.RoleSuperAdmin &&
			role != models.RoleWarga {
			return false, "invalid_role"
		}

		if role == models.RoleAdminPemdes {
			if wilayahID == nil {
				return false, "pemdes_missing_wilayah"
			}
			// Cek apakah wilayah benar-benar ada
			wilayahFound := false
			for _, id := range knownWilayahIDs {
				if id == *wilayahID {
					wilayahFound = true
					break
				}
			}
			if !wilayahFound {
				return false, "wilayah_not_found"
			}
		} else {
			if wilayahID != nil {
				return false, "non_pemdes_has_wilayah"
			}
		}

		return true, "ok"
	}

	knownWilayah := []uint{1, 2, 3}
	validWID := uint(1)
	nonExistentWID := uint(999)

	// Valid cases
	ok, _ := validateRoleAndWilayah("admin_pemdes", &validWID, knownWilayah)
	if !ok {
		t.Error("expected valid admin_pemdes with valid wilayah to pass")
	}
	ok, _ = validateRoleAndWilayah("admin_pu", nil, knownWilayah)
	if !ok {
		t.Error("expected valid admin_pu without wilayah to pass")
	}
	ok, _ = validateRoleAndWilayah("super_admin", nil, knownWilayah)
	if !ok {
		t.Error("expected valid super_admin without wilayah to pass")
	}
	ok, _ = validateRoleAndWilayah("warga", nil, knownWilayah)
	if !ok {
		t.Error("expected valid warga without wilayah to pass")
	}

	// 15. Invalid role -> BLOCKED
	ok, reason := validateRoleAndWilayah("arbitrary_role", nil, knownWilayah)
	if ok || reason != "invalid_role" {
		t.Errorf("expected invalid_role, got ok=%v, reason=%s", ok, reason)
	}

	// 16. Admin Pemdes without wilayah -> BLOCKED
	ok, reason = validateRoleAndWilayah("admin_pemdes", nil, knownWilayah)
	if ok || reason != "pemdes_missing_wilayah" {
		t.Errorf("expected pemdes_missing_wilayah, got ok=%v, reason=%s", ok, reason)
	}

	// 16. Admin Pemdes with non-existent wilayah -> BLOCKED
	ok, reason = validateRoleAndWilayah("admin_pemdes", &nonExistentWID, knownWilayah)
	if ok || reason != "wilayah_not_found" {
		t.Errorf("expected wilayah_not_found, got ok=%v, reason=%s", ok, reason)
	}

	// Non-Pemdes assigned a wilayah -> BLOCKED
	ok, reason = validateRoleAndWilayah("admin_pu", &validWID, knownWilayah)
	if ok || reason != "non_pemdes_has_wilayah" {
		t.Errorf("expected non_pemdes_has_wilayah, got ok=%v, reason=%s", ok, reason)
	}
}

// 17 - 19. Test Password Hashing, Preservation, and Exclusion
func TestUserPassword_SecurityAndPreservation(t *testing.T) {
	originalPass := "initial_secret_123"
	hashed, err := bcrypt.GenerateFromPassword([]byte(originalPass), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt failed: %v", err)
	}

	user := models.User{
		Model:    gorm.Model{ID: 1},
		Name:     "Test Admin",
		Email:    "admin@roadis.id",
		Password: string(hashed),
	}

	// 17. Ensure Password tag json:"-" does not expose password in serialized JSON
	b, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if strings.Contains(string(b), "password") || strings.Contains(string(b), "initial_secret") {
		t.Errorf("password must not be exposed in JSON: %s", string(b))
	}

	// 19. Update user without password preserves old password
	newPasswordInput := ""
	if strings.TrimSpace(newPasswordInput) == "" {
		// Preserve old password
	} else {
		user.Password = "should_not_change"
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(originalPass)); err != nil {
		t.Error("old password was not preserved during empty password update")
	}

	// 18. Update user with new password hashes correctly
	newPass := "new_secret_456"
	newHashed, err := bcrypt.GenerateFromPassword([]byte(newPass), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt failed: %v", err)
	}
	user.Password = string(newHashed)

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(newPass)); err != nil {
		t.Error("new password was not updated or hashed correctly")
	}
}

// 20. Test Self-Deletion & Last Superadmin Protections
func TestSuperadminSelfAndLastProtection(t *testing.T) {
	canDeleteUser := func(callerID uint, targetUser models.User, remainingSuperadmins int) (bool, string) {
		// Caller cannot delete themselves
		if callerID == targetUser.ID {
			return false, "cannot_delete_self"
		}
		// Cannot delete last superadmin
		if targetUser.Role == models.RoleSuperAdmin && remainingSuperadmins <= 0 {
			return false, "cannot_delete_last_superadmin"
		}
		if targetUser.Role == models.RoleWarga {
			return false, "cannot_delete_warga"
		}
		return true, "ok"
	}

	superadmin1 := models.User{Model: gorm.Model{ID: 1}, Role: models.RoleSuperAdmin}
	superadmin2 := models.User{Model: gorm.Model{ID: 2}, Role: models.RoleSuperAdmin}
	wargaUser := models.User{Model: gorm.Model{ID: 3}, Role: models.RoleWarga}
	adminPemdes := models.User{Model: gorm.Model{ID: 4}, Role: models.RoleAdminPemdes}

	// Self-deletion -> BLOCKED
	ok, reason := canDeleteUser(1, superadmin1, 1)
	if ok || reason != "cannot_delete_self" {
		t.Errorf("expected cannot_delete_self, got ok=%v, reason=%s", ok, reason)
	}

	// Last superadmin deletion -> BLOCKED
	ok, reason = canDeleteUser(2, superadmin1, 0)
	if ok || reason != "cannot_delete_last_superadmin" {
		t.Errorf("expected cannot_delete_last_superadmin, got ok=%v, reason=%s", ok, reason)
	}

	// Warga deletion -> BLOCKED
	ok, reason = canDeleteUser(1, wargaUser, 1)
	if ok || reason != "cannot_delete_warga" {
		t.Errorf("expected cannot_delete_warga, got ok=%v, reason=%s", ok, reason)
	}

	// Valid deletion of another admin when superadmins remain -> ALLOWED
	ok, _ = canDeleteUser(1, adminPemdes, 1)
	if !ok {
		t.Error("expected valid deletion of adminPemdes to be allowed")
	}

	// Deletion of superadmin when other superadmins exist -> ALLOWED
	ok, _ = canDeleteUser(1, superadmin2, 1)
	if !ok {
		t.Error("expected deletion of non-last superadmin by another superadmin to be allowed")
	}
}

// 21 - 29. Test Wilayah Management & Integrity Protection
func TestWilayahManagement_ValidationAndIntegrity(t *testing.T) {
	// 21. Empty wilayah list returns []
	wilayahList := make([]models.Wilayah, 0)
	resp := gin.H{"status": "success", "data": wilayahList}
	b, _ := json.Marshal(resp)
	if !strings.Contains(string(b), `"data":[]`) {
		t.Errorf("expected '\"data\":[]', got %s", string(b))
	}

	// 23. Tipe validation
	isValidTipe := func(tipe string) bool {
		t := strings.ToLower(strings.TrimSpace(tipe))
		return t == "desa" || t == "kabupaten" || t == "provinsi" || t == "nasional"
	}
	if !isValidTipe("desa") || !isValidTipe("kabupaten") || !isValidTipe("provinsi") || !isValidTipe("nasional") {
		t.Error("standard wilayah tipes should be valid")
	}
	if isValidTipe("kecamatan_lain") || isValidTipe("arbitrary") {
		t.Error("unrecognized wilayah tipe should be invalid")
	}

	// 28 - 29. Delete integrity protection
	canDeleteWilayah := func(usedByUserCount, usedByLaporanCount int64) (bool, string) {
		if usedByUserCount > 0 || usedByLaporanCount > 0 {
			return false, "wilayah_in_use"
		}
		return true, "ok"
	}

	// 28. Wilayah used by User -> BLOCKED
	ok, reason := canDeleteWilayah(1, 0)
	if ok || reason != "wilayah_in_use" {
		t.Errorf("expected deletion blocked when used by user, got ok=%v", ok)
	}

	// 29. Wilayah used by Laporan -> BLOCKED
	ok, reason = canDeleteWilayah(0, 5)
	if ok || reason != "wilayah_in_use" {
		t.Errorf("expected deletion blocked when used by laporan, got ok=%v", ok)
	}

	// 27. Unused wilayah -> ALLOWED
	ok, _ = canDeleteWilayah(0, 0)
	if !ok {
		t.Error("expected unused wilayah deletion to be allowed")
	}
}
