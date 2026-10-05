package controllers

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
	"backend-jalan-rusak/middlewares"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func setupProfileRoutes(r *gin.Engine) {
	public := r.Group("/api")
	{
		public.POST("/login", Login)
	}
	api := r.Group("/api")
	api.Use(middlewares.AuthMiddleware())
	{
		api.GET("/profile", GetProfile)
		api.PUT("/profile", UpdateProfile)
		api.PUT("/profile/password", ChangePassword)
		api.PUT("/profile/avatar", UploadAvatar)
		api.DELETE("/profile/avatar", DeleteAvatar)
	}
}

// Helper untuk inisialisasi database jika MySQL lokal tersedia
func ensureTestDB(t *testing.T) bool {
	if config.DB != nil {
		_ = config.DB.AutoMigrate(&models.UserPreference{})
		return true
	}

	dsn := "root:@tcp(127.0.0.1:3306)/db_jalan_rusak?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})
	if err != nil {
		t.Logf("MySQL connection unavailable in test environment (%v), falling back to mock tests", err)
		return false
	}

	err = db.AutoMigrate(
		&models.User{},
		&models.LaporanKerusakan{},
		&models.RiwayatChat{},
		&models.Wilayah{},
		&models.Notifikasi{},
		&models.UserPreference{},
	)
	if err != nil {
		t.Logf("AutoMigrate error in test: %v", err)
	}

	config.DB = db
	return true
}

func init() {
	gin.SetMode(gin.TestMode)
	_ = os.Setenv("JWT_SECRET", "profile_test_secret_key_1234567890123456")
}

// ============================================================
// A. GET PROFILE TESTS
// ============================================================

// 1. GET Profile: No token -> 401 Unauthorized
func TestProfile_GetProfile_NoToken_Returns401(t *testing.T) {
	r := gin.New()
	r.Use(middlewares.AuthMiddleware())
	r.GET("/api/profile", GetProfile)

	req, _ := http.NewRequest(http.MethodGet, "/api/profile", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized without token, got %d", w.Code)
	}
}

// 2, 3, 4, 5. GET Profile: Valid token -> Own profile, password & hash excluded, wilayah correctness
func TestProfile_GetProfile_ValidToken_And_Exclusions(t *testing.T) {
	hasDB := ensureTestDB(t)
	if !hasDB {
		t.Skip("MySQL DB is required for live GetProfile handler test")
	}

	// Buat user test di DB
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	phone := "081234567890"
	avatar := "https://res.cloudinary.com/demo/image/upload/v1/test_avatar.jpg"
	now := time.Now().Truncate(time.Second)

	testUser := models.User{
		Name:              "Budi Santoso Pemdes",
		Email:             fmt.Sprintf("budi_pemdes_%d@roadis.local", time.Now().UnixNano()),
		Password:          string(hashedPassword),
		Role:              models.RoleAdminPemdes,
		Phone:             &phone,
		AvatarURL:         &avatar,
		LastLoginAt:       &now,
		PasswordChangedAt: &now,
	}
	if err := config.DB.Create(&testUser).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	defer config.DB.Unscoped().Delete(&testUser)

	token, err := utils.GenerateToken(testUser.ID, testUser.Email, testUser.Role, testUser.WilayahID)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	r := gin.New()
	setupProfileRoutes(r)

	req, _ := http.NewRequest(http.MethodGet, "/api/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	bodyStr := w.Body.String()

	// 3. Password tidak muncul
	if strings.Contains(bodyStr, `"password":`) {
		t.Errorf("profile response MUST NOT contain password field: %s", bodyStr)
	}
	// 4. Hash tidak muncul
	if strings.Contains(bodyStr, string(hashedPassword)) || strings.Contains(bodyStr, "$2a$") {
		t.Errorf("profile response MUST NOT contain password hash: %s", bodyStr)
	}

	var resp struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Data    struct {
			ID          uint            `json:"id"`
			Name        string          `json:"name"`
			Email       string          `json:"email"`
			Phone       *string         `json:"phone"`
			Role        string          `json:"role"`
			WilayahID   *uint           `json:"wilayah_id"`
			Wilayah     *models.Wilayah `json:"wilayah"`
			AvatarURL   *string         `json:"avatar_url"`
			LastLoginAt *time.Time      `json:"last_login_at"`
		} `json:"data"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal profile response failed: %v", err)
	}

	// 2. Valid token -> own profile
	if resp.Data.ID != testUser.ID {
		t.Errorf("expected profile ID %d, got %d", testUser.ID, resp.Data.ID)
	}
	if resp.Data.Email != testUser.Email {
		t.Errorf("expected email %s, got %s", testUser.Email, resp.Data.Email)
	}
	if resp.Data.Phone == nil || *resp.Data.Phone != phone {
		t.Errorf("expected phone %s, got %v", phone, resp.Data.Phone)
	}
	if resp.Data.AvatarURL == nil || *resp.Data.AvatarURL != avatar {
		t.Errorf("expected avatar_url %s, got %v", avatar, resp.Data.AvatarURL)
	}

	// 5. Wilayah null jika tidak ada wilayah_id
	if testUser.WilayahID == nil && resp.Data.Wilayah != nil {
		t.Errorf("expected null wilayah for user without wilayah_id, got %v", resp.Data.Wilayah)
	}
}

// ============================================================
// B. UPDATE PROFILE TESTS
// ============================================================

// 1, 2. Update Profile: Name & Phone Update
func TestProfile_UpdateProfile_NameAndPhoneSuccess(t *testing.T) {
	hasDB := ensureTestDB(t)
	if !hasDB {
		t.Skip("MySQL DB is required for live UpdateProfile test")
	}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	testUser := models.User{
		Name:     "Initial Name",
		Email:    fmt.Sprintf("update_test_%d@roadis.local", time.Now().UnixNano()),
		Password: string(hashedPassword),
		Role:     models.RoleWarga,
	}
	config.DB.Create(&testUser)
	defer config.DB.Unscoped().Delete(&testUser)

	token, _ := utils.GenerateToken(testUser.ID, testUser.Email, testUser.Role, nil)

	r := gin.New()
	setupProfileRoutes(r)

	payload := map[string]interface{}{
		"name":  "  Nama Baru Terverifikasi  ",
		"phone": " +62 812-3456-7890 ",
	}
	jsonPayload, _ := json.Marshal(payload)

	req, _ := http.NewRequest(http.MethodPut, "/api/profile", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	// Verify DB state
	var updated models.User
	config.DB.First(&updated, testUser.ID)
	if updated.Name != "Nama Baru Terverifikasi" {
		t.Errorf("expected trimmed name 'Nama Baru Terverifikasi', got '%s'", updated.Name)
	}
	if updated.Phone == nil || *updated.Phone != "+62 812-3456-7890" {
		t.Errorf("expected updated phone, got %v", updated.Phone)
	}
}

// 3. Update Profile: Invalid empty name -> Reject 400
func TestProfile_UpdateProfile_EmptyName_Rejected(t *testing.T) {
	hasDB := ensureTestDB(t)
	if !hasDB {
		t.Skip("MySQL DB is required for test")
	}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	testUser := models.User{
		Name:     "Initial Name",
		Email:    fmt.Sprintf("empty_name_%d@roadis.local", time.Now().UnixNano()),
		Password: string(hashedPassword),
		Role:     models.RoleWarga,
	}
	config.DB.Create(&testUser)
	defer config.DB.Unscoped().Delete(&testUser)

	token, _ := utils.GenerateToken(testUser.ID, testUser.Email, testUser.Role, nil)

	r := gin.New()
	setupProfileRoutes(r)

	// Case A: Whitespace only
	payload := map[string]interface{}{
		"name": "    ",
	}
	jsonPayload, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPut, "/api/profile", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for whitespace name, got %d", w.Code)
	}

	// Case B: Empty string
	payloadB := map[string]interface{}{
		"name": "",
	}
	jsonPayloadB, _ := json.Marshal(payloadB)
	reqB, _ := http.NewRequest(http.MethodPut, "/api/profile", bytes.NewBuffer(jsonPayloadB))
	reqB.Header.Set("Authorization", "Bearer "+token)
	reqB.Header.Set("Content-Type", "application/json")
	wB := httptest.NewRecorder()
	r.ServeHTTP(wB, reqB)

	if wB.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for empty name, got %d", wB.Code)
	}
}

// 4. Update Profile: Invalid phone -> Reject 400
func TestProfile_UpdateProfile_InvalidPhone_Rejected(t *testing.T) {
	hasDB := ensureTestDB(t)
	if !hasDB {
		t.Skip("MySQL DB is required for test")
	}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	testUser := models.User{
		Name:     "Initial Name",
		Email:    fmt.Sprintf("invalid_phone_%d@roadis.local", time.Now().UnixNano()),
		Password: string(hashedPassword),
		Role:     models.RoleWarga,
	}
	config.DB.Create(&testUser)
	defer config.DB.Unscoped().Delete(&testUser)

	token, _ := utils.GenerateToken(testUser.ID, testUser.Email, testUser.Role, nil)

	r := gin.New()
	setupProfileRoutes(r)

	invalidPhones := []string{
		"123",                    // too short
		"abcde12345",             // contains letters
		"+18001234567",           // not Indonesian prefix
		"0812345678901234567890", // too long
		"0812@#$%",               // invalid characters
	}

	for _, badPhone := range invalidPhones {
		payload := map[string]interface{}{
			"name":  "Valid Name",
			"phone": badPhone,
		}
		jsonPayload, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPut, "/api/profile", bytes.NewBuffer(jsonPayload))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for bad phone '%s', got %d: %s", badPhone, w.Code, w.Body.String())
		}
	}
}

// 5, 6, 7, 8. Update Profile: Immutability of Role, Wilayah, Email, ID, and Self-Only
func TestProfile_UpdateProfile_ImmutableFieldsProtected(t *testing.T) {
	hasDB := ensureTestDB(t)
	if !hasDB {
		t.Skip("MySQL DB is required for test")
	}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	originalEmail := fmt.Sprintf("immutable_%d@roadis.local", time.Now().UnixNano())
	testUser := models.User{
		Name:      "Original User",
		Email:     originalEmail,
		Password:  string(hashedPassword),
		Role:      models.RoleWarga,
		WilayahID: nil,
	}
	config.DB.Create(&testUser)
	defer config.DB.Unscoped().Delete(&testUser)

	token, _ := utils.GenerateToken(testUser.ID, testUser.Email, testUser.Role, nil)

	r := gin.New()
	setupProfileRoutes(r)

	// Malicious payload attempting to escalate role, change email, change wilayah, hijack ID
	maliciousPayload := map[string]interface{}{
		"name":       "Updated Name",
		"id":         999,
		"user_id":    999,
		"role":       "super_admin",
		"email":      "hacker@roadis.local",
		"wilayah_id": 1,
		"password":   "hacked_password",
	}
	jsonPayload, _ := json.Marshal(maliciousPayload)

	req, _ := http.NewRequest(http.MethodPut, "/api/profile", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var after models.User
	config.DB.First(&after, testUser.ID)

	// 5. User hanya update dirinya sendiri
	if after.ID != testUser.ID {
		t.Errorf("user ID altered!")
	}
	// 6. Role tidak dapat diubah
	if after.Role != models.RoleWarga {
		t.Errorf("CRITICAL SECURITY: Role was changed to %s!", after.Role)
	}
	// 7. Wilayah tidak dapat diubah
	if after.WilayahID != nil {
		t.Errorf("CRITICAL SECURITY: WilayahID was changed to %v!", after.WilayahID)
	}
	// 8. Email tidak dapat diubah lewat endpoint profile
	if after.Email != originalEmail {
		t.Errorf("CRITICAL SECURITY: Email was changed to %s!", after.Email)
	}
	// Password tidak dapat diubah via PUT /api/profile
	if after.Password != string(hashedPassword) {
		t.Errorf("CRITICAL SECURITY: Password was altered via PUT /api/profile!")
	}
}

// ============================================================
// C. CHANGE PASSWORD TESTS
// ============================================================

// 1 - 7. Change Password test suite
func TestProfile_ChangePassword_FullSecurityMatrix(t *testing.T) {
	hasDB := ensureTestDB(t)
	if !hasDB {
		t.Skip("MySQL DB is required for test")
	}

	initialPassword := "initial_secret_123"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(initialPassword), bcrypt.DefaultCost)
	testUser := models.User{
		Name:     "Password Test User",
		Email:    fmt.Sprintf("pwd_test_%d@roadis.local", time.Now().UnixNano()),
		Password: string(hashedPassword),
		Role:     models.RoleWarga,
	}
	config.DB.Create(&testUser)
	defer config.DB.Unscoped().Delete(&testUser)

	token, _ := utils.GenerateToken(testUser.ID, testUser.Email, testUser.Role, nil)

	r := gin.New()
	setupProfileRoutes(r)

	// 2. Current password salah -> REJECT 400
	payloadWrongCurrent := map[string]string{
		"current_password": "wrong_password_here",
		"new_password":     "new_valid_password_123",
	}
	bWrong, _ := json.Marshal(payloadWrongCurrent)
	reqWrong, _ := http.NewRequest(http.MethodPut, "/api/profile/password", bytes.NewBuffer(bWrong))
	reqWrong.Header.Set("Authorization", "Bearer "+token)
	reqWrong.Header.Set("Content-Type", "application/json")
	wWrong := httptest.NewRecorder()
	r.ServeHTTP(wWrong, reqWrong)

	if wWrong.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for incorrect current password, got %d", wWrong.Code)
	}

	// 3. New password invalid: < 6 characters -> REJECT 400
	payloadShort := map[string]string{
		"current_password": initialPassword,
		"new_password":     "12345",
	}
	bShort, _ := json.Marshal(payloadShort)
	reqShort, _ := http.NewRequest(http.MethodPut, "/api/profile/password", bytes.NewBuffer(bShort))
	reqShort.Header.Set("Authorization", "Bearer "+token)
	reqShort.Header.Set("Content-Type", "application/json")
	wShort := httptest.NewRecorder()
	r.ServeHTTP(wShort, reqShort)

	if wShort.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for short new password, got %d", wShort.Code)
	}

	// 3. New password invalid: only whitespace -> REJECT 400
	payloadSpaces := map[string]string{
		"current_password": initialPassword,
		"new_password":     "      ",
	}
	bSpaces, _ := json.Marshal(payloadSpaces)
	reqSpaces, _ := http.NewRequest(http.MethodPut, "/api/profile/password", bytes.NewBuffer(bSpaces))
	reqSpaces.Header.Set("Authorization", "Bearer "+token)
	reqSpaces.Header.Set("Content-Type", "application/json")
	wSpaces := httptest.NewRecorder()
	r.ServeHTTP(wSpaces, reqSpaces)

	if wSpaces.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for whitespace new password, got %d", wSpaces.Code)
	}

	// 1. Current password benar & new password valid -> SUCCESS 200
	newPassword := "brand_new_secret_456"
	payloadValid := map[string]string{
		"current_password": initialPassword,
		"new_password":     newPassword,
	}
	bValid, _ := json.Marshal(payloadValid)
	reqValid, _ := http.NewRequest(http.MethodPut, "/api/profile/password", bytes.NewBuffer(bValid))
	reqValid.Header.Set("Authorization", "Bearer "+token)
	reqValid.Header.Set("Content-Type", "application/json")
	wValid := httptest.NewRecorder()
	r.ServeHTTP(wValid, reqValid)

	if wValid.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for valid password change, got %d: %s", wValid.Code, wValid.Body.String())
	}

	// 7. Password tidak muncul di response
	respStr := wValid.Body.String()
	if strings.Contains(respStr, newPassword) || strings.Contains(respStr, initialPassword) || strings.Contains(respStr, "$2a$") {
		t.Errorf("password or hash leaked in response: %s", respStr)
	}

	// 4 & 5 & 6. Verify DB changes
	var afterUser models.User
	config.DB.First(&afterUser, testUser.ID)

	// 4. Password baru berhasil diverifikasi dengan bcrypt
	if err := bcrypt.CompareHashAndPassword([]byte(afterUser.Password), []byte(newPassword)); err != nil {
		t.Errorf("new password verification failed with bcrypt: %v", err)
	}

	// 6. Hash lama tidak dipakai / password lama tidak bisa dipakai
	if err := bcrypt.CompareHashAndPassword([]byte(afterUser.Password), []byte(initialPassword)); err == nil {
		t.Errorf("old password should NO LONGER match updated hash!")
	}

	// 5. PasswordChangedAt berubah
	if afterUser.PasswordChangedAt == nil {
		t.Errorf("expected PasswordChangedAt to be set, got nil")
	} else if time.Since(*afterUser.PasswordChangedAt) > 10*time.Second {
		t.Errorf("PasswordChangedAt timestamp not recent: %v", *afterUser.PasswordChangedAt)
	}
}

// ============================================================
// D. AVATAR TESTS
// ============================================================

// Helper membuat multipart body untuk avatar
func createMultipartAvatarRequest(fieldName, filename string, content []byte) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		return nil, err
	}
	_, err = part.Write(content)
	if err != nil {
		return nil, err
	}
	writer.Close()

	req, err := http.NewRequest(http.MethodPut, "/api/profile/avatar", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

// 1 - 7. Avatar File Validation (JPG, PNG, WEBP allowed; PDF, SVG, Oversized rejected; 401 Unauthorized)
func TestProfile_AvatarUpload_Validation(t *testing.T) {
	hasDB := ensureTestDB(t)
	if !hasDB {
		t.Skip("MySQL DB is required for test")
	}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	testUser := models.User{
		Name:     "Avatar Test User",
		Email:    fmt.Sprintf("avatar_val_%d@roadis.local", time.Now().UnixNano()),
		Password: string(hashedPassword),
		Role:     models.RoleWarga,
	}
	config.DB.Create(&testUser)
	defer config.DB.Unscoped().Delete(&testUser)

	token, _ := utils.GenerateToken(testUser.ID, testUser.Email, testUser.Role, nil)

	r := gin.New()
	setupProfileRoutes(r)

	// Mock AvatarUploader & AvatarDeleter untuk test
	oldUploader := AvatarUploader
	oldDeleter := AvatarDeleter
	defer func() {
		AvatarUploader = oldUploader
		AvatarDeleter = oldDeleter
	}()

	AvatarUploader = func(fileHeader *multipart.FileHeader) (string, error) {
		return "https://res.cloudinary.com/test/image/upload/profile_avatars/new_uploaded_avatar.jpg", nil
	}
	AvatarDeleter = func(fileURL string) error {
		return nil
	}

	// 7. Unauthorized -> 401
	reqUnauth, _ := createMultipartAvatarRequest("avatar", "test.jpg", []byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00\x01\x01\x01\x00`\x00`\x00\x00"))
	wUnauth := httptest.NewRecorder()
	r.ServeHTTP(wUnauth, reqUnauth)
	if wUnauth.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized without token, got %d", wUnauth.Code)
	}

	// 1. JPG -> Success
	jpgBytes := []byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00\x01\x01\x01\x00`\x00`\x00\x00\xFF\xDB\x00C\x00")
	reqJPG, _ := createMultipartAvatarRequest("avatar", "avatar.jpg", jpgBytes)
	reqJPG.Header.Set("Authorization", "Bearer "+token)
	wJPG := httptest.NewRecorder()
	r.ServeHTTP(wJPG, reqJPG)
	if wJPG.Code != http.StatusOK {
		t.Errorf("expected 200 OK for JPG, got %d: %s", wJPG.Code, wJPG.Body.String())
	}

	// 2. PNG -> Success
	pngBytes := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15c4")
	reqPNG, _ := createMultipartAvatarRequest("avatar", "avatar.png", pngBytes)
	reqPNG.Header.Set("Authorization", "Bearer "+token)
	wPNG := httptest.NewRecorder()
	r.ServeHTTP(wPNG, reqPNG)
	if wPNG.Code != http.StatusOK {
		t.Errorf("expected 200 OK for PNG, got %d: %s", wPNG.Code, wPNG.Body.String())
	}

	// 3. WEBP -> Success
	// RIFF....WEBPVP8
	webpBytes := []byte("RIFF\x1a\x00\x00\x00WEBPVP8 \x0e\x00\x00\x00\x30\x01\x00\x9d\x01\x2a\x01\x00\x01\x00\x02\x00\x34\x25")
	reqWEBP, _ := createMultipartAvatarRequest("avatar", "avatar.webp", webpBytes)
	reqWEBP.Header.Set("Authorization", "Bearer "+token)
	wWEBP := httptest.NewRecorder()
	r.ServeHTTP(wWEBP, reqWEBP)
	if wWEBP.Code != http.StatusOK {
		t.Errorf("expected 200 OK for WEBP, got %d: %s", wWEBP.Code, wWEBP.Body.String())
	}

	// 4. PDF -> Reject 400
	pdfBytes := []byte("%PDF-1.4\n%...\n%%EOF")
	reqPDF, _ := createMultipartAvatarRequest("avatar", "document.pdf", pdfBytes)
	reqPDF.Header.Set("Authorization", "Bearer "+token)
	wPDF := httptest.NewRecorder()
	r.ServeHTTP(wPDF, reqPDF)
	if wPDF.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for PDF, got %d", wPDF.Code)
	}

	// 5. SVG -> Reject 400
	svgBytes := []byte("<?xml version=\"1.0\"?><svg xmlns=\"http://www.w3.org/2000/svg\"></svg>")
	reqSVG, _ := createMultipartAvatarRequest("avatar", "vector.svg", svgBytes)
	reqSVG.Header.Set("Authorization", "Bearer "+token)
	wSVG := httptest.NewRecorder()
	r.ServeHTTP(wSVG, reqSVG)
	if wSVG.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for SVG, got %d", wSVG.Code)
	}

	// 6. Oversized (> 2MB) -> Reject 400
	oversizedBytes := make([]byte, 2*1024*1024+1024) // 2MB + 1KB
	copy(oversizedBytes, jpgBytes)
	reqOversized, _ := createMultipartAvatarRequest("avatar", "large.jpg", oversizedBytes)
	reqOversized.Header.Set("Authorization", "Bearer "+token)
	wOversized := httptest.NewRecorder()
	r.ServeHTTP(wOversized, reqOversized)
	if wOversized.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for oversized file, got %d", wOversized.Code)
	}
}

// 8. Avatar Replacement: existing avatar replaced, old deleted after DB success, failure keeps old
func TestProfile_AvatarUpload_ReplacementFlow(t *testing.T) {
	hasDB := ensureTestDB(t)
	if !hasDB {
		t.Skip("MySQL DB is required for test")
	}

	initialAvatar := "https://res.cloudinary.com/test/image/upload/profile_avatars/old_avatar.jpg"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	testUser := models.User{
		Name:         "Replacement User",
		Email:        fmt.Sprintf("repl_%d@roadis.local", time.Now().UnixNano()),
		Password:     string(hashedPassword),
		Role:         models.RoleWarga,
		AvatarURL:    &initialAvatar,
		ProfilePhoto: initialAvatar,
	}
	config.DB.Create(&testUser)
	defer config.DB.Unscoped().Delete(&testUser)

	token, _ := utils.GenerateToken(testUser.ID, testUser.Email, testUser.Role, nil)

	r := gin.New()
	setupProfileRoutes(r)

	oldUploader := AvatarUploader
	oldDeleter := AvatarDeleter
	defer func() {
		AvatarUploader = oldUploader
		AvatarDeleter = oldDeleter
	}()

	deletedURLs := make([]string, 0)
	AvatarDeleter = func(fileURL string) error {
		deletedURLs = append(deletedURLs, fileURL)
		return nil
	}

	// Sub-test A: Upload failure -> Profile tidak berubah
	AvatarUploader = func(fileHeader *multipart.FileHeader) (string, error) {
		return "", fmt.Errorf("Cloudinary upload failed (network timeout)")
	}

	jpgBytes := []byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00\x01\x01\x01\x00`\x00`\x00\x00\xFF\xDB\x00C\x00")
	reqFail, _ := createMultipartAvatarRequest("avatar", "new.jpg", jpgBytes)
	reqFail.Header.Set("Authorization", "Bearer "+token)
	wFail := httptest.NewRecorder()
	r.ServeHTTP(wFail, reqFail)

	if wFail.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on upload failure, got %d", wFail.Code)
	}

	var unchangedUser models.User
	config.DB.First(&unchangedUser, testUser.ID)
	if unchangedUser.AvatarURL == nil || *unchangedUser.AvatarURL != initialAvatar {
		t.Errorf("avatar must NOT change on upload failure! Got %v", unchangedUser.AvatarURL)
	}

	// Sub-test B: Upload success -> Simpan baru & hapus avatar lama
	newAvatarURL := "https://res.cloudinary.com/test/image/upload/profile_avatars/brand_new_avatar.jpg"
	AvatarUploader = func(fileHeader *multipart.FileHeader) (string, error) {
		return newAvatarURL, nil
	}

	reqSuccess, _ := createMultipartAvatarRequest("avatar", "new.jpg", jpgBytes)
	reqSuccess.Header.Set("Authorization", "Bearer "+token)
	wSuccess := httptest.NewRecorder()
	r.ServeHTTP(wSuccess, reqSuccess)

	if wSuccess.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on avatar replacement, got %d: %s", wSuccess.Code, wSuccess.Body.String())
	}

	var updatedUser models.User
	config.DB.First(&updatedUser, testUser.ID)
	if updatedUser.AvatarURL == nil || *updatedUser.AvatarURL != newAvatarURL {
		t.Errorf("expected new avatar %s, got %v", newAvatarURL, updatedUser.AvatarURL)
	}

	// Beri jeda kecil untuk goroutine delete
	time.Sleep(50 * time.Millisecond)
	foundOldDeleted := false
	for _, u := range deletedURLs {
		if u == initialAvatar {
			foundOldDeleted = true
			break
		}
	}
	if !foundOldDeleted {
		t.Errorf("expected old avatar %s to be deleted via Cloudinary deleter", initialAvatar)
	}
}

// 9, 10. Delete Avatar: removes avatar and is idempotent (delete twice is safe)
func TestProfile_AvatarDelete_Idempotent(t *testing.T) {
	hasDB := ensureTestDB(t)
	if !hasDB {
		t.Skip("MySQL DB is required for test")
	}

	initialAvatar := "https://res.cloudinary.com/test/image/upload/profile_avatars/to_delete.jpg"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	testUser := models.User{
		Name:         "Delete Avatar User",
		Email:        fmt.Sprintf("del_av_%d@roadis.local", time.Now().UnixNano()),
		Password:     string(hashedPassword),
		Role:         models.RoleWarga,
		AvatarURL:    &initialAvatar,
		ProfilePhoto: initialAvatar,
	}
	config.DB.Create(&testUser)
	defer config.DB.Unscoped().Delete(&testUser)

	token, _ := utils.GenerateToken(testUser.ID, testUser.Email, testUser.Role, nil)

	r := gin.New()
	setupProfileRoutes(r)

	oldDeleter := AvatarDeleter
	defer func() { AvatarDeleter = oldDeleter }()
	deletedCount := 0
	AvatarDeleter = func(fileURL string) error {
		deletedCount++
		return nil
	}

	// 9. First Delete: avatar exists -> deleted, sets AvatarURL = nil, user remains in DB
	reqDel1, _ := http.NewRequest(http.MethodDelete, "/api/profile/avatar", nil)
	reqDel1.Header.Set("Authorization", "Bearer "+token)
	wDel1 := httptest.NewRecorder()
	r.ServeHTTP(wDel1, reqDel1)

	if wDel1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on first delete, got %d: %s", wDel1.Code, wDel1.Body.String())
	}

	var afterDel1 models.User
	if err := config.DB.First(&afterDel1, testUser.ID).Error; err != nil {
		t.Fatalf("user must NOT be deleted from DB: %v", err)
	}
	if afterDel1.AvatarURL != nil {
		t.Errorf("expected AvatarURL to be nil, got %v", *afterDel1.AvatarURL)
	}
	if afterDel1.ProfilePhoto != "" {
		t.Errorf("expected ProfilePhoto to be empty string, got %s", afterDel1.ProfilePhoto)
	}

	// 10. Second Delete (Idempotent): avatar is already nil -> returns 200 OK safely
	reqDel2, _ := http.NewRequest(http.MethodDelete, "/api/profile/avatar", nil)
	reqDel2.Header.Set("Authorization", "Bearer "+token)
	wDel2 := httptest.NewRecorder()
	r.ServeHTTP(wDel2, reqDel2)

	if wDel2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on idempotent second delete, got %d: %s", wDel2.Code, wDel2.Body.String())
	}
}

// ============================================================
// E. LAST LOGIN TESTS
// ============================================================

// 1, 2. Last Login: updated on success login, NOT updated on failed login
func TestProfile_LastLogin_LoginIntegration(t *testing.T) {
	hasDB := ensureTestDB(t)
	if !hasDB {
		t.Skip("MySQL DB is required for test")
	}

	rawPassword := "test_login_pass_123"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.DefaultCost)
	testUser := models.User{
		Name:        "Last Login User",
		Email:       fmt.Sprintf("login_test_%d@roadis.local", time.Now().UnixNano()),
		Password:    string(hashedPassword),
		Role:        models.RoleWarga,
		LastLoginAt: nil,
	}
	config.DB.Create(&testUser)
	defer config.DB.Unscoped().Delete(&testUser)

	r := gin.New()
	setupProfileRoutes(r)

	// 2. Failed Login: wrong password -> LastLoginAt remains nil
	failPayload := map[string]string{
		"email":    testUser.Email,
		"password": "wrong_password_xyz",
	}
	bFail, _ := json.Marshal(failPayload)
	reqFail, _ := http.NewRequest(http.MethodPost, "/api/login", bytes.NewBuffer(bFail))
	reqFail.Header.Set("Content-Type", "application/json")
	wFail := httptest.NewRecorder()
	r.ServeHTTP(wFail, reqFail)

	if wFail.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for bad login, got %d", wFail.Code)
	}

	var checkFailUser models.User
	config.DB.First(&checkFailUser, testUser.ID)
	if checkFailUser.LastLoginAt != nil {
		t.Errorf("LastLoginAt must NOT be updated when password is wrong!")
	}

	// 1. Successful Login -> LastLoginAt is updated
	successPayload := map[string]string{
		"email":    testUser.Email,
		"password": rawPassword,
	}
	bSuccess, _ := json.Marshal(successPayload)
	reqSuccess, _ := http.NewRequest(http.MethodPost, "/api/login", bytes.NewBuffer(bSuccess))
	reqSuccess.Header.Set("Content-Type", "application/json")
	wSuccess := httptest.NewRecorder()
	r.ServeHTTP(wSuccess, reqSuccess)

	if wSuccess.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for successful login, got %d: %s", wSuccess.Code, wSuccess.Body.String())
	}

	var checkSuccessUser models.User
	config.DB.First(&checkSuccessUser, testUser.ID)
	if checkSuccessUser.LastLoginAt == nil {
		t.Errorf("expected LastLoginAt to be populated after successful login")
	} else if time.Since(*checkSuccessUser.LastLoginAt) > 10*time.Second {
		t.Errorf("expected recent LastLoginAt timestamp, got %v", *checkSuccessUser.LastLoginAt)
	}
}

// ============================================================
// F. IDOR TESTS
// ============================================================

// 1. Verify NO /api/profile/:id route exists
func TestProfile_IDOR_NoUserIDRoute(t *testing.T) {
	r := gin.New()
	setupProfileRoutes(r)

	// Test GET /api/profile/123 -> Must return 404 Not Found (or 401 if caught by /profile wildcard, but should not route to profile)
	req, _ := http.NewRequest(http.MethodGet, "/api/profile/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Since /api/profile/:id is NOT registered, Gin returns 404 or 401 (if caught by middleware)
	if w.Code == http.StatusOK {
		t.Errorf("CRITICAL SECURITY VULNERABILITY: /api/profile/:id is accessible!")
	}
}

// 2. Request cannot specify another user (Strictly self-only)
func TestProfile_IDOR_RequestCannotTargetOtherUser(t *testing.T) {
	hasDB := ensureTestDB(t)
	if !hasDB {
		t.Skip("MySQL DB is required for test")
	}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	userA := models.User{
		Name:     "User A (Victim)",
		Email:    fmt.Sprintf("user_a_%d@roadis.local", time.Now().UnixNano()),
		Password: string(hashedPassword),
		Role:     models.RoleWarga,
	}
	userB := models.User{
		Name:     "User B (Attacker)",
		Email:    fmt.Sprintf("user_b_%d@roadis.local", time.Now().UnixNano()),
		Password: string(hashedPassword),
		Role:     models.RoleWarga,
	}
	config.DB.Create(&userA)
	config.DB.Create(&userB)
	defer func() {
		config.DB.Unscoped().Delete(&userA)
		config.DB.Unscoped().Delete(&userB)
	}()

	// Attacker has token for User B
	tokenB, _ := utils.GenerateToken(userB.ID, userB.Email, userB.Role, nil)

	r := gin.New()
	setupProfileRoutes(r)

	// Attacker tries to update User A by passing User A's ID in body / query
	maliciousPayload := map[string]interface{}{
		"id":      userA.ID,
		"user_id": userA.ID,
		"name":    "Hacked Name",
	}
	bMalicious, _ := json.Marshal(maliciousPayload)
	req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("/api/profile?user_id=%d&id=%d", userA.ID, userA.ID), bytes.NewBuffer(bMalicious))
	req.Header.Set("Authorization", "Bearer "+tokenB)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for attacker self-update, got %d", w.Code)
	}

	// Verify Victim (User A) is UNTOUCHED
	var victimCheck models.User
	config.DB.First(&victimCheck, userA.ID)
	if victimCheck.Name != "User A (Victim)" {
		t.Errorf("CRITICAL IDOR: User B was able to modify User A's name to '%s'!", victimCheck.Name)
	}

	// Verify Attacker (User B) updated their OWN profile
	var attackerCheck models.User
	config.DB.First(&attackerCheck, userB.ID)
	if attackerCheck.Name != "Hacked Name" {
		t.Errorf("expected User B's own name to be updated, got '%s'", attackerCheck.Name)
	}
}
