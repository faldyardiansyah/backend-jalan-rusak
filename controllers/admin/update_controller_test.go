package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

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

func validateStatusEnum(status string) (string, bool) {
	if status == "" {
		return "", true
	}
	s := strings.ToLower(strings.TrimSpace(status))
	if s != "menunggu" && s != "proses" && s != "selesai" && s != "ditolak" {
		return s, false
	}
	return s, true
}

func validateFotoBuktiRequirement(statusLower string, existingFotoBukti string, hasNewFileUpload bool) bool {
	if statusLower == "selesai" {
		if !hasNewFileUpload && existingFotoBukti == "" {
			return false // Ditolak: bukti perbaikan wajib ada
		}
	}
	return true
}

func validateCatatanAdminRequirement(statusLower string, catatanAdmin string) (bool, string) {
	if statusLower == "ditolak" {
		if strings.TrimSpace(catatanAdmin) == "" {
			return false, "Catatan admin / alasan penolakan wajib diisi saat menolak laporan"
		}
	}
	return true, "OK"
}

func TestStatusValidation(t *testing.T) {
	validCases := []string{"menunggu", "proses", "selesai", "ditolak", "MENUNGGU", "PROSES", "SELESAI", "DITOLAK"}
	for _, status := range validCases {
		_, ok := validateStatusEnum(status)
		if !ok {
			t.Errorf("expected status %q to be valid, but was rejected", status)
		}
	}

	invalidCases := []string{"batal", "pending", "done", "random_status", "123"}
	for _, status := range invalidCases {
		_, ok := validateStatusEnum(status)
		if ok {
			t.Errorf("expected status %q to be invalid, but was accepted", status)
		}
	}
}

func TestFotoBuktiRequirementOnSelesai(t *testing.T) {
	testCases := []struct {
		name              string
		status            string
		existingFotoBukti string
		hasNewFileUpload  bool
		expectedAllowed   bool
	}{
		{
			name:              "Selesai without existing photo and without upload -> REJECTED",
			status:            "selesai",
			existingFotoBukti: "",
			hasNewFileUpload:  false,
			expectedAllowed:   false,
		},
		{
			name:              "Selesai with existing photo and no new upload -> ALLOWED",
			status:            "selesai",
			existingFotoBukti: "https://res.cloudinary.com/demo/image/upload/bukti1.jpg",
			hasNewFileUpload:  false,
			expectedAllowed:   true,
		},
		{
			name:              "Selesai with new photo upload -> ALLOWED",
			status:            "selesai",
			existingFotoBukti: "",
			hasNewFileUpload:  true,
			expectedAllowed:   true,
		},
		{
			name:              "Status proses without photo -> ALLOWED",
			status:            "proses",
			existingFotoBukti: "",
			hasNewFileUpload:  false,
			expectedAllowed:   true,
		},
		{
			name:              "Status menunggu without photo -> ALLOWED",
			status:            "menunggu",
			existingFotoBukti: "",
			hasNewFileUpload:  false,
			expectedAllowed:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			allowed := validateFotoBuktiRequirement(tc.status, tc.existingFotoBukti, tc.hasNewFileUpload)
			if allowed != tc.expectedAllowed {
				t.Errorf("[%s] expected allowed=%v, got %v", tc.name, tc.expectedAllowed, allowed)
			}
		})
	}
}

func TestCatatanAdminRequirementOnDitolak(t *testing.T) {
	testCases := []struct {
		name            string
		status          string
		catatanAdmin    string
		expectedAllowed bool
	}{
		{
			name:            "Ditolak with empty catatan_admin -> REJECTED (400)",
			status:          "ditolak",
			catatanAdmin:    "",
			expectedAllowed: false,
		},
		{
			name:            "Ditolak with whitespace-only catatan_admin -> REJECTED (400)",
			status:          "ditolak",
			catatanAdmin:    "   \t\n  ",
			expectedAllowed: false,
		},
		{
			name:            "Ditolak with valid catatan_admin -> ALLOWED",
			status:          "ditolak",
			catatanAdmin:    "Bukan kewenangan jalan kabupaten",
			expectedAllowed: true,
		},
		{
			name:            "Menunggu with empty catatan_admin -> ALLOWED",
			status:          "menunggu",
			catatanAdmin:    "",
			expectedAllowed: true,
		},
		{
			name:            "Proses with empty catatan_admin -> ALLOWED",
			status:          "proses",
			catatanAdmin:    "",
			expectedAllowed: true,
		},
		{
			name:            "Selesai with empty catatan_admin -> ALLOWED",
			status:          "selesai",
			catatanAdmin:    "",
			expectedAllowed: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			allowed, _ := validateCatatanAdminRequirement(tc.status, tc.catatanAdmin)
			if allowed != tc.expectedAllowed {
				t.Errorf("[%s] expected allowed=%v, got %v", tc.name, tc.expectedAllowed, allowed)
			}
		})
	}
}

func TestEmptyAdminLaporanResponse_SerializesToArray(t *testing.T) {
	listLaporan := make([]models.LaporanKerusakan, 0)

	type ResponseWrapper struct {
		Status  string                    `json:"status"`
		Message string                    `json:"message"`
		Data    []models.LaporanKerusakan `json:"data"`
		Total   int64                     `json:"total"`
	}

	wrapped := ResponseWrapper{
		Status:  "success",
		Message: "Laporan kerusakan berhasil diambil",
		Data:    listLaporan,
		Total:   0,
	}

	bytes, err := json.Marshal(wrapped)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	jsonStr := string(bytes)
	expectedSub := `"data":[]`
	if !strings.Contains(jsonStr, expectedSub) {
		t.Errorf("expected JSON to contain %q, got %s", expectedSub, jsonStr)
	}
}

func parsePagination(pageStr, limitStr string) (int, int, int) {
	page, errPage := strconv.Atoi(pageStr)
	if errPage != nil || page < 1 {
		page = 1
	}
	limit, errLimit := strconv.Atoi(limitStr)
	if errLimit != nil || limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit
	return page, limit, offset
}

func TestPaginationParsing(t *testing.T) {
	cases := []struct {
		name           string
		pageStr        string
		limitStr       string
		expectedPage   int
		expectedLimit  int
		expectedOffset int
	}{
		{
			name:           "Standard valid pagination",
			pageStr:        "2",
			limitStr:       "15",
			expectedPage:   2,
			expectedLimit:  15,
			expectedOffset: 15,
		},
		{
			name:           "Page 0 -> defaults to 1",
			pageStr:        "0",
			limitStr:       "10",
			expectedPage:   1,
			expectedLimit:  10,
			expectedOffset: 0,
		},
		{
			name:           "Negative page -> defaults to 1",
			pageStr:        "-5",
			limitStr:       "10",
			expectedPage:   1,
			expectedLimit:  10,
			expectedOffset: 0,
		},
		{
			name:           "Limit 0 -> defaults to 10",
			pageStr:        "1",
			limitStr:       "0",
			expectedPage:   1,
			expectedLimit:  10,
			expectedOffset: 0,
		},
		{
			name:           "Negative limit -> defaults to 10",
			pageStr:        "1",
			limitStr:       "-20",
			expectedPage:   1,
			expectedLimit:  10,
			expectedOffset: 0,
		},
		{
			name:           "Malformed non-numeric strings -> defaults",
			pageStr:        "abc",
			limitStr:       "xyz",
			expectedPage:   1,
			expectedLimit:  10,
			expectedOffset: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, l, o := parsePagination(tc.pageStr, tc.limitStr)
			if p != tc.expectedPage {
				t.Errorf("[%s] expected page %d, got %d", tc.name, tc.expectedPage, p)
			}
			if l != tc.expectedLimit {
				t.Errorf("[%s] expected limit %d, got %d", tc.name, tc.expectedLimit, l)
			}
			if o != tc.expectedOffset {
				t.Errorf("[%s] expected offset %d, got %d", tc.name, tc.expectedOffset, o)
			}
		})
	}
}

func createAdminMultipartRequest(laporanID string, fieldName, filename string, content []byte, formFields map[string]string) (*http.Request, error) {
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

	req := httptest.NewRequest(http.MethodPut, "/api/admin/laporan/"+laporanID+"/status", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

func TestUpdateStatusLaporan_FileUploadSecurityAndScope(t *testing.T) {
	hasDB := ensureTestDB(t)
	if !hasDB {
		t.Skip("Database not available for integration test")
	}

	gin.SetMode(gin.TestMode)

	// Fixtures
	now := time.Now().UnixNano()
	wilayah := models.Wilayah{
		Nama: "Desa Uji BE22 " + strconv.FormatInt(now, 10),
		Tipe: "desa",
	}
	config.DB.Create(&wilayah)
	defer config.DB.Unscoped().Delete(&wilayah)

	adminPemdes := models.User{
		Name:      "Admin Pemdes Test",
		Email:     fmt.Sprintf("pemdes_%d@roadis.local", now),
		Role:      models.RoleAdminPemdes,
		WilayahID: &wilayah.ID,
	}
	config.DB.Create(&adminPemdes)
	defer config.DB.Unscoped().Delete(&adminPemdes)

	adminPU := models.User{
		Name:  "Admin PU Test",
		Email: fmt.Sprintf("pu_%d@roadis.local", now),
		Role:  models.RoleAdminPu,
	}
	config.DB.Create(&adminPU)
	defer config.DB.Unscoped().Delete(&adminPU)

	laporanDesa := models.LaporanKerusakan{
		UserID:        adminPemdes.ID,
		WilayahID:     wilayah.ID,
		Judul:         "Jalan Desa Rusak",
		Deskripsi:     "Deskripsi",
		Latitude:      -6.34,
		Longitude:     108.33,
		ImageURL:      "https://example.com/foto.jpg",
		TipeKerusakan: "sedang",
		JenisJalan:    "desa",
		Status:        "menunggu",
		FotoBukti:     "",
	}
	config.DB.Create(&laporanDesa)
	defer config.DB.Unscoped().Delete(&laporanDesa)

	laporanKab := models.LaporanKerusakan{
		UserID:        adminPU.ID,
		WilayahID:     wilayah.ID,
		Judul:         "Jalan Kabupaten Rusak",
		Deskripsi:     "Deskripsi",
		Latitude:      -6.34,
		Longitude:     108.33,
		ImageURL:      "https://example.com/foto.jpg",
		TipeKerusakan: "sedang",
		JenisJalan:    "kabupaten",
		Status:        "menunggu",
		FotoBukti:     "",
	}
	config.DB.Create(&laporanKab)
	defer config.DB.Unscoped().Delete(&laporanKab)

	validJPG := []byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00\x01\x01\x01\x00`\x00`\x00\x00\xFF\xDB\x00C\x00")
	plainText := []byte("Not an image")
	pdfBytes := []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%EOF")
	corruptBytes := []byte{0x00, 0x01, 0x02, 0x03}

	laporanDesaID := strconv.Itoa(int(laporanDesa.ID))
	laporanKabID := strconv.Itoa(int(laporanKab.ID))

	// 1. Missing foto_bukti on status selesai -> 400
	t.Run("Missing foto_bukti on status selesai rejected with 400", func(t *testing.T) {
		req, _ := createAdminMultipartRequest(laporanDesaID, "", "", nil, map[string]string{
			"status": "selesai",
		})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: laporanDesaID}}
		c.Set("user_id", adminPemdes.ID)
		c.Set("role", string(models.RoleAdminPemdes))

		UpdateStatusLaporan(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 2. Oversized foto_bukti (>5MB) on status selesai -> 400, uploader NOT called
	t.Run("Oversized foto_bukti (>5MB) rejected with 400 and uploader NOT called", func(t *testing.T) {
		uploaderCalled := false
		origUploader := BuktiUploader
		BuktiUploader = func(fh *multipart.FileHeader) (string, error) {
			uploaderCalled = true
			return "https://mock.cloudinary/bukti.jpg", nil
		}
		defer func() { BuktiUploader = origUploader }()

		oversized := make([]byte, utils.MaxUploadImageSizeBytes+1)
		copy(oversized, validJPG)
		req, _ := createAdminMultipartRequest(laporanDesaID, "foto_bukti", "huge.jpg", oversized, map[string]string{
			"status": "selesai",
		})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: laporanDesaID}}
		c.Set("user_id", adminPemdes.ID)
		c.Set("role", string(models.RoleAdminPemdes))

		UpdateStatusLaporan(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
		if uploaderCalled {
			t.Errorf("expected BuktiUploader NOT to be called for oversized file")
		}
	})

	// 3. Disallowed extension (.pdf) on status selesai -> 400, uploader NOT called
	t.Run("Disallowed extension (.pdf) rejected with 400 and uploader NOT called", func(t *testing.T) {
		uploaderCalled := false
		origUploader := BuktiUploader
		BuktiUploader = func(fh *multipart.FileHeader) (string, error) {
			uploaderCalled = true
			return "https://mock.cloudinary/bukti.jpg", nil
		}
		defer func() { BuktiUploader = origUploader }()

		req, _ := createAdminMultipartRequest(laporanDesaID, "foto_bukti", "document.pdf", pdfBytes, map[string]string{
			"status": "selesai",
		})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: laporanDesaID}}
		c.Set("user_id", adminPemdes.ID)
		c.Set("role", string(models.RoleAdminPemdes))

		UpdateStatusLaporan(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
		if uploaderCalled {
			t.Errorf("expected BuktiUploader NOT to be called for .pdf")
		}
	})

	// 4. Fake image extension (.jpg with text) on status selesai -> 400, uploader NOT called
	t.Run("Fake image extension (.jpg with text) rejected with 400 and uploader NOT called", func(t *testing.T) {
		uploaderCalled := false
		origUploader := BuktiUploader
		BuktiUploader = func(fh *multipart.FileHeader) (string, error) {
			uploaderCalled = true
			return "https://mock.cloudinary/bukti.jpg", nil
		}
		defer func() { BuktiUploader = origUploader }()

		req, _ := createAdminMultipartRequest(laporanDesaID, "foto_bukti", "fake.jpg", plainText, map[string]string{
			"status": "selesai",
		})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: laporanDesaID}}
		c.Set("user_id", adminPemdes.ID)
		c.Set("role", string(models.RoleAdminPemdes))

		UpdateStatusLaporan(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
		if uploaderCalled {
			t.Errorf("expected BuktiUploader NOT to be called for fake .jpg")
		}
	})

	// 5. Corrupt file on status selesai -> 400, uploader NOT called
	t.Run("Corrupt binary rejected with 400 and uploader NOT called", func(t *testing.T) {
		uploaderCalled := false
		origUploader := BuktiUploader
		BuktiUploader = func(fh *multipart.FileHeader) (string, error) {
			uploaderCalled = true
			return "https://mock.cloudinary/bukti.jpg", nil
		}
		defer func() { BuktiUploader = origUploader }()

		req, _ := createAdminMultipartRequest(laporanDesaID, "foto_bukti", "corrupt.jpg", corruptBytes, map[string]string{
			"status": "selesai",
		})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: laporanDesaID}}
		c.Set("user_id", adminPemdes.ID)
		c.Set("role", string(models.RoleAdminPemdes))

		UpdateStatusLaporan(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
		if uploaderCalled {
			t.Errorf("expected BuktiUploader NOT to be called for corrupt file")
		}
	})

	// 6. Valid JPG <= 5MB on status selesai -> 200, uploader called
	t.Run("Valid JPEG <= 5MB on status selesai accepted and uploader called", func(t *testing.T) {
		uploaderCalled := false
		origUploader := BuktiUploader
		BuktiUploader = func(fh *multipart.FileHeader) (string, error) {
			uploaderCalled = true
			return "https://mock.cloudinary/bukti.jpg", nil
		}
		defer func() { BuktiUploader = origUploader }()

		req, _ := createAdminMultipartRequest(laporanDesaID, "foto_bukti", "valid.jpg", validJPG, map[string]string{
			"status": "selesai",
		})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: laporanDesaID}}
		c.Set("user_id", adminPemdes.ID)
		c.Set("role", string(models.RoleAdminPemdes))

		UpdateStatusLaporan(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if !uploaderCalled {
			t.Errorf("expected BuktiUploader TO be called for valid JPG")
		}

		// Verify DB status updated
		var refreshed models.LaporanKerusakan
		config.DB.First(&refreshed, laporanDesa.ID)
		if refreshed.Status != "selesai" {
			t.Errorf("expected status selesai, got %s", refreshed.Status)
		}
		if refreshed.FotoBukti != "https://mock.cloudinary/bukti.jpg" {
			t.Errorf("expected FotoBukti updated, got %s", refreshed.FotoBukti)
		}
	})

	// 7. Status selesai with existing photo and no new upload -> 200
	t.Run("Status selesai with existing photo and no new upload allowed", func(t *testing.T) {
		req, _ := createAdminMultipartRequest(laporanDesaID, "", "", nil, map[string]string{
			"status": "selesai",
		})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: laporanDesaID}}
		c.Set("user_id", adminPemdes.ID)
		c.Set("role", string(models.RoleAdminPemdes))

		UpdateStatusLaporan(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 8. Scope regression: Admin PU cannot update non-kabupaten laporan -> 403
	t.Run("Scope regression: Admin PU forbidden on Desa laporan", func(t *testing.T) {
		req, _ := createAdminMultipartRequest(laporanDesaID, "", "", nil, map[string]string{
			"status": "proses",
		})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: laporanDesaID}}
		c.Set("user_id", adminPU.ID)
		c.Set("role", string(models.RoleAdminPu))

		UpdateStatusLaporan(c)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for Admin PU on Desa laporan, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 9. Scope regression: Admin PU can update Kabupaten laporan -> 200
	t.Run("Scope regression: Admin PU allowed on Kabupaten laporan", func(t *testing.T) {
		req, _ := createAdminMultipartRequest(laporanKabID, "", "", nil, map[string]string{
			"status": "proses",
		})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: laporanKabID}}
		c.Set("user_id", adminPU.ID)
		c.Set("role", string(models.RoleAdminPu))

		UpdateStatusLaporan(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK for Admin PU on Kabupaten laporan, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 10. Scope regression: Admin Pemdes cannot update Kabupaten laporan -> 403
	t.Run("Scope regression: Admin Pemdes forbidden on Kabupaten laporan", func(t *testing.T) {
		req, _ := createAdminMultipartRequest(laporanKabID, "", "", nil, map[string]string{
			"status": "proses",
		})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: laporanKabID}}
		c.Set("user_id", adminPemdes.ID)
		c.Set("role", string(models.RoleAdminPemdes))

		UpdateStatusLaporan(c)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for Admin Pemdes on Kabupaten laporan, got %d: %s", w.Code, w.Body.String())
		}
	})
}
