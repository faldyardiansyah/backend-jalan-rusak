package routes_test

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
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func init() {
	gin.SetMode(gin.TestMode)
	_ = os.Setenv("JWT_SECRET", "routes_integration_secret_12345678")
}

func ensureDBForRoutesTest(t *testing.T) bool {
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
		t.Logf("MySQL not available for routes integration test: %v", err)
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

func TestHTTPIntegration_ProfileEndpoints(t *testing.T) {
	hasDB := ensureDBForRoutesTest(t)
	if !hasDB {
		t.Skip("MySQL DB is required for live routes HTTP integration test")
	}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("currentPassword123"), bcrypt.DefaultCost)
	testUser := models.User{
		Name:     "Integration Test User",
		Email:    fmt.Sprintf("integration_%d@roadis.local", time.Now().UnixNano()),
		Password: string(hashedPassword),
		Role:     models.RoleWarga,
	}
	config.DB.Create(&testUser)
	defer config.DB.Unscoped().Delete(&testUser)

	token, err := utils.GenerateToken(testUser.ID, testUser.Email, testUser.Role, nil)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	r := gin.New()
	routes.SetupRoutes(r)

	// 1. GET /api/profile
	t.Run("GET /api/profile", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/profile", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("GET /api/profile expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 2. PUT /api/profile
	t.Run("PUT /api/profile", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"name":  "Nama Terupdate Integrasi",
			"phone": "081298765432",
		})
		req, _ := http.NewRequest(http.MethodPut, "/api/profile", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("PUT /api/profile expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 3. PUT /api/profile/password
	t.Run("PUT /api/profile/password", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"current_password": "currentPassword123",
			"new_password":     "newPassword456",
		})
		req, _ := http.NewRequest(http.MethodPut, "/api/profile/password", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("PUT /api/profile/password expected 200, got %d: %s", w.Code, w.Body.String())
		}

		// Password change increments TokenVersion; refresh token for subsequent tests
		config.DB.First(&testUser, testUser.ID)
		newToken, errTok := utils.GenerateToken(testUser.ID, testUser.Email, testUser.Role, nil, testUser.TokenVersion)
		if errTok == nil {
			token = newToken
		}
	})

	// 4. PUT /api/profile/avatar
	t.Run("PUT /api/profile/avatar", func(t *testing.T) {
		oldUploader := controllers.AvatarUploader
		defer func() { controllers.AvatarUploader = oldUploader }()
		controllers.AvatarUploader = func(fileHeader *multipart.FileHeader) (string, error) {
			return "https://res.cloudinary.com/test/image/upload/profile_avatars/integration_avatar.jpg", nil
		}

		b := &bytes.Buffer{}
		writer := multipart.NewWriter(b)
		part, _ := writer.CreateFormFile("avatar", "test.jpg")
		part.Write([]byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00\x01\x01\x01\x00`\x00`\x00\x00\xFF\xDB\x00C\x00"))
		writer.Close()

		req, _ := http.NewRequest(http.MethodPut, "/api/profile/avatar", b)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("PUT /api/profile/avatar expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 5. DELETE /api/profile/avatar
	t.Run("DELETE /api/profile/avatar", func(t *testing.T) {
		oldDeleter := controllers.AvatarDeleter
		defer func() { controllers.AvatarDeleter = oldDeleter }()
		controllers.AvatarDeleter = func(fileURL string) error {
			return nil
		}

		req, _ := http.NewRequest(http.MethodDelete, "/api/profile/avatar", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("DELETE /api/profile/avatar expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})
}
