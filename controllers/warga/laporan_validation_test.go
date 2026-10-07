package warga

import (
	"bytes"
	"encoding/json"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
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

func validateCoordinate(latStr, lngStr string) (float64, float64, string) {
	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil || math.IsNaN(lat) || math.IsInf(lat, 0) || lat < -90 || lat > 90 {
		return 0, 0, "Latitude tidak valid (harus berada di antara -90 dan 90)"
	}

	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil || math.IsNaN(lng) || math.IsInf(lng, 0) || lng < -180 || lng > 180 {
		return 0, 0, "Longitude tidak valid (harus berada di antara -180 dan 180)"
	}

	return lat, lng, ""
}

func TestCoordinateValidation(t *testing.T) {
	testCases := []struct {
		name        string
		latStr      string
		lngStr      string
		expectError bool
	}{
		{
			name:        "Valid Indramayu Coordinate",
			latStr:      "-6.3400",
			lngStr:      "108.3300",
			expectError: false,
		},
		{
			name:        "Valid Boundary Coordinate",
			latStr:      "90.0",
			lngStr:      "180.0",
			expectError: false,
		},
		{
			name:        "Valid Negative Boundary",
			latStr:      "-90.0",
			lngStr:      "-180.0",
			expectError: false,
		},
		{
			name:        "Latitude Too Low (< -90)",
			latStr:      "-90.0001",
			lngStr:      "108.3300",
			expectError: true,
		},
		{
			name:        "Latitude Too High (> 90)",
			latStr:      "90.0001",
			lngStr:      "108.3300",
			expectError: true,
		},
		{
			name:        "Longitude Too Low (< -180)",
			latStr:      "-6.3400",
			lngStr:      "-180.0001",
			expectError: true,
		},
		{
			name:        "Longitude Too High (> 180)",
			latStr:      "-6.3400",
			lngStr:      "180.0001",
			expectError: true,
		},
		{
			name:        "Non-numeric Latitude",
			latStr:      "invalid_lat",
			lngStr:      "108.3300",
			expectError: true,
		},
		{
			name:        "Non-numeric Longitude",
			latStr:      "-6.3400",
			lngStr:      "invalid_lng",
			expectError: true,
		},
		{
			name:        "NaN Coordinate",
			latStr:      "NaN",
			lngStr:      "108.3300",
			expectError: true,
		},
		{
			name:        "Infinity Coordinate",
			latStr:      "+Inf",
			lngStr:      "108.3300",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, errMsg := validateCoordinate(tc.latStr, tc.lngStr)
			if tc.expectError && errMsg == "" {
				t.Errorf("[%s] expected error, got nil", tc.name)
			}
			if !tc.expectError && errMsg != "" {
				t.Errorf("[%s] expected success, got error: %s", tc.name, errMsg)
			}
		})
	}
}

func TestEmptyLaporanResponse_SerializesToArray(t *testing.T) {
	// Memastikan respon kosong mengembalikan [] dan bukan null
	responseData := make([]LaporanResponse, 0)

	type ResponseWrapper struct {
		Status  string            `json:"status"`
		Message string            `json:"message"`
		Data    []LaporanResponse `json:"data"`
	}

	wrapped := ResponseWrapper{
		Status:  "success",
		Message: "Riwayat laporan berhasil diambil",
		Data:    responseData,
	}

	bytes, err := json.Marshal(wrapped)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	jsonStr := string(bytes)
	expectedSub := `"data":[]`
	if !containsString(jsonStr, expectedSub) {
		t.Errorf("expected JSON to contain %q, got %s", expectedSub, jsonStr)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func createMultipartRequest(fieldName, filename string, content []byte, formFields map[string]string) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for k, v := range formFields {
		_ = writer.WriteField(k, v)
	}

	if fieldName != "" {
		part, err := writer.CreateFormFile(fieldName, filename)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(content); err != nil {
			return nil, err
		}
	}

	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/warga/laporan", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

func TestCreateLaporan_FileValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	validJPG := []byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00\x01\x01\x01\x00`\x00`\x00\x00\xFF\xDB\x00C\x00")
	plainText := []byte("This is plain text and not a photo")
	pdfBytes := []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%EOF")
	corruptBytes := []byte{0x00, 0x01, 0x02, 0x03, 0x04}

	baseFields := map[string]string{
		"latitude":       "-6.3400",
		"longitude":      "108.3300",
		"judul":          "Jalan Rusak Parah",
		"deskripsi":      "Lubang besar di tengah jalan",
		"tipe_kerusakan": "berat",
		"wilayah_id":     "1",
	}

	t.Run("Missing foto field rejected with 400", func(t *testing.T) {
		uploaderCalled := false
		origUploader := LaporanUploader
		LaporanUploader = func(fh *multipart.FileHeader) (string, error) {
			uploaderCalled = true
			return "https://mock.cloudinary/img.jpg", nil
		}
		defer func() { LaporanUploader = origUploader }()

		r := gin.New()
		r.POST("/api/warga/laporan", func(c *gin.Context) {
			c.Set("user_id", uint(1))
			CreateLaporan(c)
		})

		req, _ := createMultipartRequest("", "", nil, baseFields)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
		if uploaderCalled {
			t.Errorf("expected LaporanUploader NOT to be called")
		}
	})

	t.Run("Oversized foto (>5MB) rejected with 400 and uploader NOT called", func(t *testing.T) {
		uploaderCalled := false
		origUploader := LaporanUploader
		LaporanUploader = func(fh *multipart.FileHeader) (string, error) {
			uploaderCalled = true
			return "https://mock.cloudinary/img.jpg", nil
		}
		defer func() { LaporanUploader = origUploader }()

		r := gin.New()
		r.POST("/api/warga/laporan", func(c *gin.Context) {
			c.Set("user_id", uint(1))
			CreateLaporan(c)
		})

		oversized := make([]byte, utils.MaxUploadImageSizeBytes+1)
		copy(oversized, validJPG)
		req, _ := createMultipartRequest("foto", "huge.jpg", oversized, baseFields)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
		if uploaderCalled {
			t.Errorf("expected LaporanUploader NOT to be called for oversized file")
		}
	})

	t.Run("Disallowed extension (.pdf) rejected with 400 and uploader NOT called", func(t *testing.T) {
		uploaderCalled := false
		origUploader := LaporanUploader
		LaporanUploader = func(fh *multipart.FileHeader) (string, error) {
			uploaderCalled = true
			return "https://mock.cloudinary/img.jpg", nil
		}
		defer func() { LaporanUploader = origUploader }()

		r := gin.New()
		r.POST("/api/warga/laporan", func(c *gin.Context) {
			c.Set("user_id", uint(1))
			CreateLaporan(c)
		})

		req, _ := createMultipartRequest("foto", "document.pdf", pdfBytes, baseFields)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
		if uploaderCalled {
			t.Errorf("expected LaporanUploader NOT to be called for .pdf")
		}
	})

	t.Run("Fake image extension (.jpg with text content) rejected with 400 and uploader NOT called", func(t *testing.T) {
		uploaderCalled := false
		origUploader := LaporanUploader
		LaporanUploader = func(fh *multipart.FileHeader) (string, error) {
			uploaderCalled = true
			return "https://mock.cloudinary/img.jpg", nil
		}
		defer func() { LaporanUploader = origUploader }()

		r := gin.New()
		r.POST("/api/warga/laporan", func(c *gin.Context) {
			c.Set("user_id", uint(1))
			CreateLaporan(c)
		})

		req, _ := createMultipartRequest("foto", "fake.jpg", plainText, baseFields)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
		if uploaderCalled {
			t.Errorf("expected LaporanUploader NOT to be called for fake .jpg")
		}
	})

	t.Run("Corrupt binary image rejected with 400 and uploader NOT called", func(t *testing.T) {
		uploaderCalled := false
		origUploader := LaporanUploader
		LaporanUploader = func(fh *multipart.FileHeader) (string, error) {
			uploaderCalled = true
			return "https://mock.cloudinary/img.jpg", nil
		}
		defer func() { LaporanUploader = origUploader }()

		r := gin.New()
		r.POST("/api/warga/laporan", func(c *gin.Context) {
			c.Set("user_id", uint(1))
			CreateLaporan(c)
		})

		req, _ := createMultipartRequest("foto", "corrupt.jpg", corruptBytes, baseFields)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
		if uploaderCalled {
			t.Errorf("expected LaporanUploader NOT to be called for corrupt file")
		}
	})

	t.Run("Valid JPEG image <= 5MB accepted and uploader called", func(t *testing.T) {
		hasDB := ensureTestDB(t)
		if !hasDB {
			t.Skip("Database not available")
		}
		var testUser models.User
		if err := config.DB.First(&testUser).Error; err != nil {
			t.Skip("No test user in DB")
		}

		uploaderCalled := false
		origUploader := LaporanUploader
		LaporanUploader = func(fh *multipart.FileHeader) (string, error) {
			uploaderCalled = true
			return "https://mock.cloudinary/img.jpg", nil
		}
		defer func() { LaporanUploader = origUploader }()

		r := gin.New()
		r.POST("/api/warga/laporan", func(c *gin.Context) {
			c.Set("user_id", testUser.ID)
			CreateLaporan(c)
		})

		req, _ := createMultipartRequest("foto", "valid.jpg", validJPG, baseFields)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if !uploaderCalled {
			t.Errorf("expected LaporanUploader TO be called for valid JPG")
		}
	})
}
