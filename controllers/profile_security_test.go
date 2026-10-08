package controllers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/controllers"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/routes"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func init() {
	gin.SetMode(gin.TestMode)
	_ = os.Setenv("JWT_SECRET", "be11_profile_test_secret_key_123456789012")
}

func setupBE11Router() *gin.Engine {
	r := gin.New()
	routes.SetupRoutes(r)
	return r
}

func createMultipartProfileFile(fieldName, filename string, content []byte) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(content); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPut, "/api/profile/avatar", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

func createMultipartLegacyPhotoRequest(fieldName, filename string, content []byte) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(content); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPut, "/api/profile/photo", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

// ============================================================
// 1. STANDALONE ZERO-DB TESTS (High-speed unit security tests)
// ============================================================

func TestBE11_Profile_StandaloneSecurity(t *testing.T) {
	r := setupBE11Router()
	secretStr := os.Getenv("JWT_SECRET")
	if secretStr == "" {
		secretStr = os.Getenv("JWT_SECRET_KEY")
	}
	if secretStr == "" {
		secretStr = "be11_profile_test_secret_key_123456789012"
		_ = os.Setenv("JWT_SECRET", secretStr)
	}
	secret := []byte(secretStr)

	makeToken := func(userID uint, role string, exp time.Duration) string {
		claims := utils.JWTClaim{
			UserID: userID,
			Email:  "user@example.com",
			Role:   models.UserRole(role),
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(exp)),
			},
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		str, _ := tok.SignedString(secret)
		return str
	}

	wargaToken, _ := utils.GenerateToken(10, "user@example.com", models.RoleWarga, nil)
	unknownRoleToken := makeToken(10, "attacker_role", time.Hour)
	zeroUserToken := makeToken(0, "warga", time.Hour)

	// A. Authentication & Role Validation Across All Profile/Settings Endpoints
	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/profile"},
		{http.MethodPut, "/api/profile"},
		{http.MethodPut, "/api/profile/password"},
		{http.MethodPut, "/api/profile/avatar"},
		{http.MethodDelete, "/api/profile/avatar"},
		{http.MethodPut, "/api/profile/photo"},
		{http.MethodGet, "/api/settings"},
		{http.MethodPut, "/api/settings"},
		{http.MethodPost, "/api/settings/logout-all"},
	}

	for _, ep := range endpoints {
		t.Run("Auth_MissingToken_"+ep.method+"_"+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("[%s %s] expected 401 Unauthorized without token, got %d", ep.method, ep.path, w.Code)
			}
		})

		t.Run("Auth_InvalidSignature_"+ep.method+"_"+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			req.Header.Set("Authorization", "Bearer invalid.signature.token")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("[%s %s] expected 401 Unauthorized on invalid signature, got %d", ep.method, ep.path, w.Code)
			}
		})

		t.Run("Auth_UnknownRole_"+ep.method+"_"+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			req.Header.Set("Authorization", "Bearer "+unknownRoleToken)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusForbidden {
				t.Errorf("[%s %s] expected 403 Forbidden on unknown role, got %d", ep.method, ep.path, w.Code)
			}
		})

		t.Run("Auth_ZeroUserID_"+ep.method+"_"+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			req.Header.Set("Authorization", "Bearer "+zeroUserToken)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("[%s %s] expected 401 Unauthorized on zero user ID, got %d", ep.method, ep.path, w.Code)
			}
		})
	}

	// B. Avatar & Central ValidateImageFile Validation (Zero DB needed)
	t.Run("Avatar_CentralValidation_Rules", func(t *testing.T) {
		// 1. Nil file header -> error
		if err := utils.ValidateImageFile(nil); err == nil {
			t.Error("expected error for nil file header")
		}

		// Helper create header
		createHeader := func(filename string, content []byte) *multipart.FileHeader {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			part, _ := writer.CreateFormFile("avatar", filename)
			_, _ = part.Write(content)
			_ = writer.Close()

			req, _ := http.NewRequest(http.MethodPost, "/", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			_ = req.ParseMultipartForm(10 << 20)
			return req.MultipartForm.File["avatar"][0]
		}

		// 2. Zero-byte file -> rejected
		fhEmpty := createHeader("empty.jpg", []byte{})
		if err := utils.ValidateImageFile(fhEmpty); err == nil {
			t.Error("expected error for zero-byte file")
		}

		// 3. Fake extension (txt content with .jpg ext) -> rejected by MIME sniffing
		fhFake := createHeader("fake.jpg", []byte("this is plain text, not an image"))
		if err := utils.ValidateImageFile(fhFake); err == nil {
			t.Error("expected error for fake extension with text content")
		}

		// 4. Disallowed extensions (pdf, svg, exe) -> rejected
		fhPDF := createHeader("doc.pdf", []byte("%PDF-1.4\n%%EOF"))
		if err := utils.ValidateImageFile(fhPDF); err == nil {
			t.Error("expected error for pdf extension")
		}

		fhSVG := createHeader("vector.svg", []byte("<svg></svg>"))
		if err := utils.ValidateImageFile(fhSVG); err == nil {
			t.Error("expected error for svg extension")
		}

		fhEXE := createHeader("virus.exe", []byte("MZ\x90\x00"))
		if err := utils.ValidateImageFile(fhEXE); err == nil {
			t.Error("expected error for exe extension")
		}

		// 5. Oversized (> 2 MB with custom limit) -> rejected
		oversizedBytes := make([]byte, 2*1024*1024+100)
		copy(oversizedBytes, []byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00\x01\x01\x01\x00`\x00`\x00\x00"))
		fhOversized := createHeader("large.jpg", oversizedBytes)
		if err := utils.ValidateImageFile(fhOversized, 2*1024*1024); err == nil {
			t.Error("expected error for oversized file exceeding 2MB")
		}

		// 6. Valid JPG header -> passes
		validJPG := []byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00\x01\x01\x01\x00`\x00`\x00\x00\xFF\xDB\x00C\x00")
		fhJPG := createHeader("valid.jpg", validJPG)
		if err := utils.ValidateImageFile(fhJPG, 2*1024*1024); err != nil {
			t.Errorf("expected valid JPG to pass, got: %v", err)
		}

		// 7. Valid PNG header -> passes
		validPNG := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15c4")
		fhPNG := createHeader("valid.png", validPNG)
		if err := utils.ValidateImageFile(fhPNG, 2*1024*1024); err != nil {
			t.Errorf("expected valid PNG to pass, got: %v", err)
		}

		// 8. Valid WEBP header -> passes
		validWEBP := []byte("RIFF\x1a\x00\x00\x00WEBPVP8 \x0e\x00\x00\x00\x30\x01\x00\x9d\x01\x2a\x01\x00\x01\x00\x02\x00\x34\x25")
		fhWEBP := createHeader("valid.webp", validWEBP)
		if err := utils.ValidateImageFile(fhWEBP, 2*1024*1024); err != nil {
			t.Errorf("expected valid WEBP to pass, got: %v", err)
		}
	})

	// C. Phone Validation Utility Rules
	t.Run("Phone_Validation_Rules", func(t *testing.T) {
		validPhones := []string{
			"081234567890",
			"+6281234567890",
			"6281234567890",
			"0812-3456-7890",
			"+62 812 3456 7890",
		}
		for _, p := range validPhones {
			if !controllers.IsValidIndonesianPhone(p) {
				t.Errorf("expected phone %s to be valid", p)
			}
		}

		invalidPhones := []string{
			"",
			"123",
			"abcdefghij",
			"+123456789012",
			"0812",
			"+6281234567890123456789012345", // too long
			"08123456<script>",
		}
		for _, p := range invalidPhones {
			if controllers.IsValidIndonesianPhone(p) {
				t.Errorf("expected phone %s to be invalid", p)
			}
		}
	})

	// D. Nil Safety and Fail-Closed on Handlers
	t.Run("FailClosed_DatabaseNil_Returns500WithoutPanic", func(t *testing.T) {
		oldDB := config.DB
		config.DB = nil
		defer func() { config.DB = oldDB }()

		// GET /api/profile
		reqProfile := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
		reqProfile.Header.Set("Authorization", "Bearer "+wargaToken)
		wProfile := httptest.NewRecorder()
		r.ServeHTTP(wProfile, reqProfile)
		if wProfile.Code != http.StatusInternalServerError {
			t.Errorf("expected 500 when DB is nil, got %d", wProfile.Code)
		}

		// PUT /api/profile
		bName, _ := json.Marshal(map[string]string{"name": "Valid Name"})
		reqUpProfile := httptest.NewRequest(http.MethodPut, "/api/profile", bytes.NewBuffer(bName))
		reqUpProfile.Header.Set("Authorization", "Bearer "+wargaToken)
		reqUpProfile.Header.Set("Content-Type", "application/json")
		wUpProfile := httptest.NewRecorder()
		r.ServeHTTP(wUpProfile, reqUpProfile)
		if wUpProfile.Code != http.StatusInternalServerError {
			t.Errorf("expected 500 when DB is nil, got %d", wUpProfile.Code)
		}

		// PUT /api/profile/password
		bPwd, _ := json.Marshal(map[string]string{"current_password": "old", "new_password": "newpassword123"})
		reqPwd := httptest.NewRequest(http.MethodPut, "/api/profile/password", bytes.NewBuffer(bPwd))
		reqPwd.Header.Set("Authorization", "Bearer "+wargaToken)
		reqPwd.Header.Set("Content-Type", "application/json")
		wPwd := httptest.NewRecorder()
		r.ServeHTTP(wPwd, reqPwd)
		if wPwd.Code != http.StatusInternalServerError {
			t.Errorf("expected 500 when DB is nil, got %d", wPwd.Code)
		}

		// GET /api/settings
		reqSettings := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
		reqSettings.Header.Set("Authorization", "Bearer "+wargaToken)
		wSettings := httptest.NewRecorder()
		r.ServeHTTP(wSettings, reqSettings)
		if wSettings.Code != http.StatusInternalServerError {
			t.Errorf("expected 500 when DB is nil, got %d", wSettings.Code)
		}

		// PUT /api/settings
		bSet, _ := json.Marshal(map[string]string{"theme": "dark"})
		reqUpSettings := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewBuffer(bSet))
		reqUpSettings.Header.Set("Authorization", "Bearer "+wargaToken)
		reqUpSettings.Header.Set("Content-Type", "application/json")
		wUpSettings := httptest.NewRecorder()
		r.ServeHTTP(wUpSettings, reqUpSettings)
		if wUpSettings.Code != http.StatusInternalServerError {
			t.Errorf("expected 500 when DB is nil, got %d", wUpSettings.Code)
		}

		// POST /api/settings/logout-all
		reqLogout := httptest.NewRequest(http.MethodPost, "/api/settings/logout-all", nil)
		reqLogout.Header.Set("Authorization", "Bearer "+wargaToken)
		wLogout := httptest.NewRecorder()
		r.ServeHTTP(wLogout, reqLogout)
		if wLogout.Code != http.StatusInternalServerError {
			t.Errorf("expected 500 when DB is nil, got %d", wLogout.Code)
		}

		// PUT /api/profile/photo
		reqPhoto, _ := createMultipartLegacyPhotoRequest("foto", "test.jpg", []byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00\x01\x01\x01\x00`\x00`\x00\x00\xFF\xDB\x00C\x00"))
		reqPhoto.Header.Set("Authorization", "Bearer "+wargaToken)
		wPhoto := httptest.NewRecorder()
		r.ServeHTTP(wPhoto, reqPhoto)
		if wPhoto.Code != http.StatusInternalServerError {
			t.Errorf("expected 500 when DB is nil on legacy photo, got %d", wPhoto.Code)
		}
	})
}

// ============================================================
// 2. LIVE INTEGRATION TESTS (Full Lifecycle, Revocation, DB)
// ============================================================

func TestBE11_Profile_Integration(t *testing.T) {
	if config.DB == nil {
		t.Skip("MySQL DB not available for BE-11 Integration test")
	}

	r := setupBE11Router()

	// Setup 2 test users (Warga A and Warga B)
	pwdA := "passwordA123"
	hashA, _ := bcrypt.GenerateFromPassword([]byte(pwdA), bcrypt.DefaultCost)
	phoneA := "08111111111"
	userA := models.User{
		Name:         "User A Warga",
		Email:        fmt.Sprintf("user_a_%d@roadis.local", time.Now().UnixNano()),
		Password:     string(hashA),
		Role:         models.RoleWarga,
		Phone:        &phoneA,
		TokenVersion: 1,
	}
	config.DB.Create(&userA)
	defer config.DB.Unscoped().Delete(&userA)

	pwdB := "passwordB123"
	hashB, _ := bcrypt.GenerateFromPassword([]byte(pwdB), bcrypt.DefaultCost)
	userB := models.User{
		Name:         "User B Warga",
		Email:        fmt.Sprintf("user_b_%d@roadis.local", time.Now().UnixNano()),
		Password:     string(hashB),
		Role:         models.RoleWarga,
		TokenVersion: 1,
	}
	config.DB.Create(&userB)
	defer config.DB.Unscoped().Delete(&userB)

	tokenA, _ := utils.GenerateToken(userA.ID, userA.Email, userA.Role, nil, userA.TokenVersion)
	tokenB, _ := utils.GenerateToken(userB.ID, userB.Email, userB.Role, nil, userB.TokenVersion)

	// A. Profile Ownership & IDOR Protection (Self Scoped)
	t.Run("Profile_Ownership_Isolation", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
		req.Header.Set("Authorization", "Bearer "+tokenA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp struct {
			Data struct {
				ID    uint   `json:"id"`
				Name  string `json:"name"`
				Email string `json:"email"`
			} `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		if resp.Data.ID != userA.ID || resp.Data.Email != userA.Email {
			t.Errorf("expected User A data, got ID %d, Email %s", resp.Data.ID, resp.Data.Email)
		}
	})

	// B. Privilege Field Injection Protection
	t.Run("Profile_PrivilegeFieldInjection_Ignored", func(t *testing.T) {
		// Attempt to inject role=super_admin and wilayah_id=99
		payload := map[string]interface{}{
			"name":          "Updated Name",
			"role":          "super_admin",
			"wilayah_id":    99,
			"token_version": 999,
			"email":         "hacker@roadis.local",
		}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPut, "/api/profile", bytes.NewBuffer(b))
		req.Header.Set("Authorization", "Bearer "+tokenA)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var refreshedUser models.User
		config.DB.First(&refreshedUser, userA.ID)

		if refreshedUser.Role != models.RoleWarga {
			t.Errorf("PRIVILEGE ESCALATION: role changed to %s!", refreshedUser.Role)
		}
		if refreshedUser.Email != userA.Email {
			t.Errorf("EMAIL INJECTION: email changed to %s!", refreshedUser.Email)
		}
		if refreshedUser.Name != "Updated Name" {
			t.Errorf("expected name to update to 'Updated Name', got %s", refreshedUser.Name)
		}
	})

	// C. Password Change, Token Revocation, and Login Verification
	t.Run("PasswordChange_TokenRevocation_Lifecycle", func(t *testing.T) {
		newPwd := "super_new_pwd_456"
		payload := map[string]string{
			"current_password": pwdA,
			"new_password":     newPwd,
		}
		b, _ := json.Marshal(payload)
		reqChange := httptest.NewRequest(http.MethodPut, "/api/profile/password", bytes.NewBuffer(b))
		reqChange.Header.Set("Authorization", "Bearer "+tokenA)
		reqChange.Header.Set("Content-Type", "application/json")
		wChange := httptest.NewRecorder()
		r.ServeHTTP(wChange, reqChange)

		if wChange.Code != http.StatusOK {
			t.Fatalf("expected 200 OK on password change, got %d: %s", wChange.Code, wChange.Body.String())
		}

		// 1. Old token must now be revoked (token version mismatch)
		reqOldToken := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
		reqOldToken.Header.Set("Authorization", "Bearer "+tokenA)
		wOldToken := httptest.NewRecorder()
		r.ServeHTTP(wOldToken, reqOldToken)

		if wOldToken.Code != http.StatusUnauthorized {
			t.Errorf("CRITICAL: Old token was NOT revoked! Expected 401 Unauthorized, got %d", wOldToken.Code)
		}

		// 2. Login with old password must fail
		loginOldPayload := map[string]string{
			"email":    userA.Email,
			"password": pwdA,
		}
		bLoginOld, _ := json.Marshal(loginOldPayload)
		reqLoginOld := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBuffer(bLoginOld))
		reqLoginOld.Header.Set("Content-Type", "application/json")
		wLoginOld := httptest.NewRecorder()
		r.ServeHTTP(wLoginOld, reqLoginOld)

		if wLoginOld.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized for old password login, got %d", wLoginOld.Code)
		}

		// 3. Login with new password must succeed
		loginNewPayload := map[string]string{
			"email":    userA.Email,
			"password": newPwd,
		}
		bLoginNew, _ := json.Marshal(loginNewPayload)
		reqLoginNew := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBuffer(bLoginNew))
		reqLoginNew.Header.Set("Content-Type", "application/json")
		wLoginNew := httptest.NewRecorder()
		r.ServeHTTP(wLoginNew, reqLoginNew)

		if wLoginNew.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for new password login, got %d: %s", wLoginNew.Code, wLoginNew.Body.String())
		}

		var loginResp struct {
			Token string `json:"token"`
		}
		_ = json.Unmarshal(wLoginNew.Body.Bytes(), &loginResp)
		if loginResp.Token == "" {
			t.Fatal("expected new JWT token in login response")
		}

		// 4. Access with new token succeeds
		reqNewToken := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
		reqNewToken.Header.Set("Authorization", "Bearer "+loginResp.Token)
		wNewToken := httptest.NewRecorder()
		r.ServeHTTP(wNewToken, reqNewToken)

		if wNewToken.Code != http.StatusOK {
			t.Errorf("expected 200 OK with newly issued token, got %d", wNewToken.Code)
		}
	})

	// D. Legacy Profile Photo Endpoint Hardening (/api/profile/photo)
	t.Run("Legacy_ProfilePhoto_Endpoint_Hardened", func(t *testing.T) {
		// Mock uploader & deleter
		oldUploader := controllers.AvatarUploader
		oldDeleter := controllers.AvatarDeleter
		defer func() {
			controllers.AvatarUploader = oldUploader
			controllers.AvatarDeleter = oldDeleter
		}()

		uploadedPhotoURL := "https://res.cloudinary.com/test/image/upload/legacy_profile.jpg"
		controllers.AvatarUploader = func(fh *multipart.FileHeader) (string, error) {
			return uploadedPhotoURL, nil
		}
		controllers.AvatarDeleter = func(url string) error {
			return nil
		}

		// Valid photo upload via legacy endpoint
		validJPG := []byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00\x01\x01\x01\x00`\x00`\x00\x00\xFF\xDB\x00C\x00")
		reqPhoto, _ := createMultipartLegacyPhotoRequest("foto", "myphoto.jpg", validJPG)
		reqPhoto.Header.Set("Authorization", "Bearer "+tokenB)
		wPhoto := httptest.NewRecorder()
		r.ServeHTTP(wPhoto, reqPhoto)

		if wPhoto.Code != http.StatusOK {
			t.Fatalf("expected 200 OK on legacy photo upload, got %d: %s", wPhoto.Code, wPhoto.Body.String())
		}

		var refreshedUserB models.User
		config.DB.First(&refreshedUserB, userB.ID)

		if refreshedUserB.ProfilePhoto != uploadedPhotoURL {
			t.Errorf("expected ProfilePhoto to update to %s, got %s", uploadedPhotoURL, refreshedUserB.ProfilePhoto)
		}
		if refreshedUserB.AvatarURL == nil || *refreshedUserB.AvatarURL != uploadedPhotoURL {
			t.Errorf("expected AvatarURL to be synchronized, got %v", refreshedUserB.AvatarURL)
		}

		// Invalid file (e.g. PDF) via legacy endpoint must be rejected with 400
		pdfContent := []byte("%PDF-1.4\n%%EOF")
		reqInvalid, _ := createMultipartLegacyPhotoRequest("foto", "doc.pdf", pdfContent)
		reqInvalid.Header.Set("Authorization", "Bearer "+tokenB)
		wInvalid := httptest.NewRecorder()
		r.ServeHTTP(wInvalid, reqInvalid)

		if wInvalid.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for PDF on legacy photo endpoint, got %d", wInvalid.Code)
		}
	})

	// E. Soft-Deleted User Protection
	t.Run("SoftDeleted_User_BlockedFromProfileAndSettings", func(t *testing.T) {
		deletedUser := models.User{
			Name:         "Deleted User",
			Email:        fmt.Sprintf("del_%d@roadis.local", time.Now().UnixNano()),
			Role:         models.RoleWarga,
			TokenVersion: 1,
		}
		config.DB.Create(&deletedUser)
		delToken, _ := utils.GenerateToken(deletedUser.ID, deletedUser.Email, deletedUser.Role, nil, 1)

		// Soft delete user
		config.DB.Delete(&deletedUser)

		// Profile access must be rejected 401
		req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
		req.Header.Set("Authorization", "Bearer "+delToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized for soft-deleted user, got %d", w.Code)
		}

		// Settings access must be rejected 401
		reqSet := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
		reqSet.Header.Set("Authorization", "Bearer "+delToken)
		wSet := httptest.NewRecorder()
		r.ServeHTTP(wSet, reqSet)

		if wSet.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized for soft-deleted user settings, got %d", wSet.Code)
		}
	})
}
