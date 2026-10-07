package controllers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"backend-jalan-rusak/config"
	adminController "backend-jalan-rusak/controllers/admin"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/routes"
	"backend-jalan-rusak/utils"
	wargaController "backend-jalan-rusak/controllers/warga"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func init() {
	gin.SetMode(gin.TestMode)
	_ = os.Setenv("JWT_SECRET", "be5_test_secret_key_123456789012")
}

func ensureDBForLaporanTest(t *testing.T) bool {
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
		t.Logf("MySQL connection unavailable for laporan test (%v)", err)
		return false
	}

	config.DB = db
	return true
}

func createMultipartForm(fieldName, fileName string, fileBytes []byte, fields map[string]string) (*http.Request, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if fileName != "" && fileBytes != nil {
		part, _ := writer.CreateFormFile(fieldName, fileName)
		_, _ = part.Write(fileBytes)
	}

	for k, v := range fields {
		_ = writer.WriteField(k, v)
	}

	_ = writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, writer.FormDataContentType()
}

// =========================================================================
// AREA 19 TEST MATRIX A: CREATE LAPORAN
// =========================================================================
func TestLaporan_CreateMatrix(t *testing.T) {
	if !ensureDBForLaporanTest(t) {
		t.Skip("Database not available")
	}

	// Setup router
	r := gin.New()
	routes.SetupRoutes(r)

	// Fixtures
	wilayahAktif := models.Wilayah{Nama: "Desa Aktif BE5", Tipe: "desa"}
	config.DB.Create(&wilayahAktif)
	defer config.DB.Unscoped().Delete(&wilayahAktif)

	wilayahDeleted := models.Wilayah{
		Nama: "Desa Terhapus BE5",
		Tipe: "desa",
		Model: gorm.Model{DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}},
	}
	config.DB.Create(&wilayahDeleted)
	defer config.DB.Unscoped().Delete(&wilayahDeleted)

	wargaUser := models.User{
		Name:     "Warga Pelapor BE5",
		Email:    fmt.Sprintf("warga_be5_%d@roadis.id", time.Now().UnixNano()),
		Password: "password",
		Role:     models.RoleWarga,
	}
	config.DB.Create(&wargaUser)
	defer config.DB.Unscoped().Delete(&wargaUser)

	tokenWarga, _ := utils.GenerateToken(wargaUser.ID, wargaUser.Email, wargaUser.Role, nil)
	validJPG := []byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00\x01\x01\x01\x00`\x00`\x00\x00\xFF\xDB\x00C\x00")

	origUploader := wargaController.LaporanUploader
	wargaController.LaporanUploader = func(fh *multipart.FileHeader) (string, error) {
		return "https://mock.cloudinary/be5_foto.jpg", nil
	}
	defer func() { wargaController.LaporanUploader = origUploader }()

	// 1. Valid report creation
	t.Run("Valid report created with status menunggu and correct ownership", func(t *testing.T) {
		fields := map[string]string{
			"judul":          "Lubang Besar Jalan Desa",
			"deskripsi":      "Jalan rusak parah di pertigaan",
			"tipe_kerusakan": "rusak berat",
			"latitude":       "-6.3400",
			"longitude":      "108.3300",
			"wilayah_id":     strconv.Itoa(int(wilayahAktif.ID)),
			"jenis_jalan":    "desa",
		}
		req, contentType := createMultipartForm("foto", "jalan.jpg", validJPG, fields)
		req.URL.Path = "/api/warga/laporan"
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", contentType)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp struct {
			Status string                        `json:"status"`
			Data   wargaController.LaporanResponse `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		if resp.Data.Status != "menunggu" {
			t.Errorf("expected status 'menunggu', got '%s'", resp.Data.Status)
		}
		if resp.Data.UserID != wargaUser.ID {
			t.Errorf("expected user_id %d, got %d", wargaUser.ID, resp.Data.UserID)
		}

		// Cleanup created report
		defer config.DB.Unscoped().Delete(&models.LaporanKerusakan{}, resp.Data.ID)
	})

	// 2. Unauthenticated request -> 401
	t.Run("Unauthenticated request rejected with 401", func(t *testing.T) {
		fields := map[string]string{
			"judul":          "Lubang",
			"deskripsi":      "Deskripsi",
			"tipe_kerusakan": "rusak",
			"latitude":       "-6.3400",
			"longitude":      "108.3300",
			"wilayah_id":     strconv.Itoa(int(wilayahAktif.ID)),
		}
		req, contentType := createMultipartForm("foto", "jalan.jpg", validJPG, fields)
		req.URL.Path = "/api/warga/laporan"
		req.Header.Set("Content-Type", contentType)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	// 3. Wrong role (Admin PU calling warga endpoint) -> 403
	t.Run("Wrong role rejected with 403", func(t *testing.T) {
		adminPUUser := models.User{
			Name:     "Admin PU Test",
			Email:    fmt.Sprintf("pu_be5_%d@roadis.id", time.Now().UnixNano()),
			Password: "password",
			Role:     models.RoleAdminPu,
		}
		config.DB.Create(&adminPUUser)
		defer config.DB.Unscoped().Delete(&adminPUUser)

		tokenPU, _ := utils.GenerateToken(adminPUUser.ID, adminPUUser.Email, adminPUUser.Role, nil)
		fields := map[string]string{
			"judul":          "Lubang",
			"deskripsi":      "Deskripsi",
			"tipe_kerusakan": "rusak",
			"latitude":       "-6.3400",
			"longitude":      "108.3300",
			"wilayah_id":     strconv.Itoa(int(wilayahAktif.ID)),
		}
		req, contentType := createMultipartForm("foto", "jalan.jpg", validJPG, fields)
		req.URL.Path = "/api/warga/laporan"
		req.Header.Set("Authorization", "Bearer "+tokenPU)
		req.Header.Set("Content-Type", contentType)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %d", w.Code)
		}
	})

	// 4. Invalid non-existent wilayah_id -> 400
	t.Run("Invalid non-existent wilayah_id rejected with 400", func(t *testing.T) {
		fields := map[string]string{
			"judul":          "Lubang",
			"deskripsi":      "Deskripsi",
			"tipe_kerusakan": "rusak",
			"latitude":       "-6.3400",
			"longitude":      "108.3300",
			"wilayah_id":     "999999",
		}
		req, contentType := createMultipartForm("foto", "jalan.jpg", validJPG, fields)
		req.URL.Path = "/api/warga/laporan"
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", contentType)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for non-existent wilayah_id, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 5. Soft-deleted wilayah_id -> 400
	t.Run("Soft-deleted wilayah_id rejected with 400", func(t *testing.T) {
		fields := map[string]string{
			"judul":          "Lubang",
			"deskripsi":      "Deskripsi",
			"tipe_kerusakan": "rusak",
			"latitude":       "-6.3400",
			"longitude":      "108.3300",
			"wilayah_id":     strconv.Itoa(int(wilayahDeleted.ID)),
		}
		req, contentType := createMultipartForm("foto", "jalan.jpg", validJPG, fields)
		req.URL.Path = "/api/warga/laporan"
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", contentType)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for soft-deleted wilayah_id, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 6. Invalid coordinate (latitude > 90) -> 400
	t.Run("Invalid coordinate rejected with 400", func(t *testing.T) {
		fields := map[string]string{
			"judul":          "Lubang",
			"deskripsi":      "Deskripsi",
			"tipe_kerusakan": "rusak",
			"latitude":       "95.0",
			"longitude":      "108.3300",
			"wilayah_id":     strconv.Itoa(int(wilayahAktif.ID)),
		}
		req, contentType := createMultipartForm("foto", "jalan.jpg", validJPG, fields)
		req.URL.Path = "/api/warga/laporan"
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", contentType)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for invalid coordinate, got %d", w.Code)
		}
	})

	// 7. Malicious internal fields injection attempt
	t.Run("Malicious internal fields ignored on creation", func(t *testing.T) {
		fields := map[string]string{
			"judul":          "Percobaan Tampering",
			"deskripsi":      "Mencoba injeksi status dan user_id",
			"tipe_kerusakan": "retak",
			"latitude":       "-6.3400",
			"longitude":      "108.3300",
			"wilayah_id":     strconv.Itoa(int(wilayahAktif.ID)),
			"user_id":        "9999",
			"status":         "SELESAI",
			"priority":       "TINGGI",
			"foto_bukti":     "https://hacker.com/fake.jpg",
			"catatan_admin":  "Hacked",
		}
		req, contentType := createMultipartForm("foto", "jalan.jpg", validJPG, fields)
		req.URL.Path = "/api/warga/laporan"
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", contentType)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp struct {
			Data wargaController.LaporanResponse `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		if resp.Data.UserID != wargaUser.ID {
			t.Errorf("CRITICAL IDOR: user_id was tampered! Expected %d, got %d", wargaUser.ID, resp.Data.UserID)
		}
		if resp.Data.Status != "menunggu" {
			t.Errorf("CRITICAL LIFECYCLE TAMPER: status was tampered! Expected 'menunggu', got '%s'", resp.Data.Status)
		}

		// Verify DB directly
		var inDB models.LaporanKerusakan
		config.DB.First(&inDB, resp.Data.ID)
		defer config.DB.Unscoped().Delete(&inDB)

		if inDB.FotoBukti != "" {
			t.Errorf("foto_bukti was illegally injected: %s", inDB.FotoBukti)
		}
		if inDB.CatatanAdmin != "" {
			t.Errorf("catatan_admin was illegally injected: %s", inDB.CatatanAdmin)
		}
	})
}

// =========================================================================
// AREA 19 TEST MATRIX B & D: READ & IDOR OWNERSHIP ACCESS
// =========================================================================
func TestLaporan_ReadAndIDORMatrix(t *testing.T) {
	if !ensureDBForLaporanTest(t) {
		t.Skip("Database not available")
	}

	r := gin.New()
	routes.SetupRoutes(r)

	// Wilayah Desa A & Desa B
	desaA := models.Wilayah{Nama: "Desa A BE5", Tipe: "desa"}
	config.DB.Create(&desaA)
	defer config.DB.Unscoped().Delete(&desaA)

	desaB := models.Wilayah{Nama: "Desa B BE5", Tipe: "desa"}
	config.DB.Create(&desaB)
	defer config.DB.Unscoped().Delete(&desaB)

	// Users
	wargaA := models.User{Name: "Warga A", Email: fmt.Sprintf("wargaA_%d@roadis.id", time.Now().UnixNano()), Role: models.RoleWarga}
	config.DB.Create(&wargaA)
	defer config.DB.Unscoped().Delete(&wargaA)

	wargaB := models.User{Name: "Warga B", Email: fmt.Sprintf("wargaB_%d@roadis.id", time.Now().UnixNano()), Role: models.RoleWarga}
	config.DB.Create(&wargaB)
	defer config.DB.Unscoped().Delete(&wargaB)

	pemdesA := models.User{Name: "Pemdes A", Email: fmt.Sprintf("pemdesA_%d@roadis.id", time.Now().UnixNano()), Role: models.RoleAdminPemdes, WilayahID: &desaA.ID}
	config.DB.Create(&pemdesA)
	defer config.DB.Unscoped().Delete(&pemdesA)

	adminPU := models.User{Name: "Admin PU", Email: fmt.Sprintf("pu_%d@roadis.id", time.Now().UnixNano()), Role: models.RoleAdminPu}
	config.DB.Create(&adminPU)
	defer config.DB.Unscoped().Delete(&adminPU)

	superAdmin := models.User{Name: "Super Admin", Email: fmt.Sprintf("sa_%d@roadis.id", time.Now().UnixNano()), Role: models.RoleSuperAdmin}
	config.DB.Create(&superAdmin)
	defer config.DB.Unscoped().Delete(&superAdmin)

	tokenWargaA, _ := utils.GenerateToken(wargaA.ID, wargaA.Email, wargaA.Role, nil)
	tokenWargaB, _ := utils.GenerateToken(wargaB.ID, wargaB.Email, wargaB.Role, nil)
	tokenPemdesA, _ := utils.GenerateToken(pemdesA.ID, pemdesA.Email, pemdesA.Role, nil)
	tokenPU, _ := utils.GenerateToken(adminPU.ID, adminPU.Email, adminPU.Role, nil)
	tokenSA, _ := utils.GenerateToken(superAdmin.ID, superAdmin.Email, superAdmin.Role, nil)

	// Laporan di Desa A milik Warga A
	lapDesaA := models.LaporanKerusakan{
		UserID: wargaA.ID, WilayahID: desaA.ID, JenisJalan: "desa", Judul: "Desa A Road",
		Deskripsi: "Rusak", Latitude: -6.34, Longitude: 108.33, ImageURL: "https://foto.jpg",
		TipeKerusakan: "lubang", Status: "menunggu",
	}
	config.DB.Create(&lapDesaA)
	defer config.DB.Unscoped().Delete(&lapDesaA)

	// Laporan di Desa B milik Warga B
	lapDesaB := models.LaporanKerusakan{
		UserID: wargaB.ID, WilayahID: desaB.ID, JenisJalan: "desa", Judul: "Desa B Road",
		Deskripsi: "Rusak", Latitude: -6.35, Longitude: 108.34, ImageURL: "https://foto.jpg",
		TipeKerusakan: "lubang", Status: "menunggu",
	}
	config.DB.Create(&lapDesaB)
	defer config.DB.Unscoped().Delete(&lapDesaB)

	// Laporan Kabupaten
	lapKab := models.LaporanKerusakan{
		UserID: wargaA.ID, WilayahID: desaA.ID, JenisJalan: "kabupaten", Judul: "Kabupaten Road",
		Deskripsi: "Rusak", Latitude: -6.36, Longitude: 108.35, ImageURL: "https://foto.jpg",
		TipeKerusakan: "lubang", Status: "menunggu",
	}
	config.DB.Create(&lapKab)
	defer config.DB.Unscoped().Delete(&lapKab)

	// Laporan Soft-Deleted
	lapDeleted := models.LaporanKerusakan{
		UserID: wargaA.ID, WilayahID: desaA.ID, JenisJalan: "desa", Judul: "Deleted Road",
		Deskripsi: "Rusak", Latitude: -6.37, Longitude: 108.36, ImageURL: "https://foto.jpg",
		TipeKerusakan: "lubang", Status: "menunggu",
		Model: gorm.Model{DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}},
	}
	config.DB.Create(&lapDeleted)
	defer config.DB.Unscoped().Delete(&lapDeleted)

	// 1. Warga A reads own report on /api/warga/laporan/:id -> 200 OK
	t.Run("Warga A reads own report -> 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/warga/laporan/%d", lapDesaA.ID), nil)
		req.Header.Set("Authorization", "Bearer "+tokenWargaA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 2. IDOR: Warga B attempts reading Warga A report on /api/warga/laporan/:id -> 403 Forbidden
	t.Run("Warga B cross-reads Warga A report -> 403 Forbidden (IDOR Guard)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/warga/laporan/%d", lapDesaA.ID), nil)
		req.Header.Set("Authorization", "Bearer "+tokenWargaB)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 3. Warga reads non-existent report -> 404 Not Found
	t.Run("Warga reads non-existent report -> 404 Not Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/warga/laporan/999999", nil)
		req.Header.Set("Authorization", "Bearer "+tokenWargaA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found, got %d", w.Code)
		}
	})

	// 4. Warga reads soft-deleted report -> 404 Not Found
	t.Run("Warga reads soft-deleted report -> 404 Not Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/warga/laporan/%d", lapDeleted.ID), nil)
		req.Header.Set("Authorization", "Bearer "+tokenWargaA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found, got %d", w.Code)
		}
	})

	// 5. Admin Pemdes Desa A reads Desa A report -> 200 OK
	t.Run("Admin Pemdes Desa A reads own Desa report -> 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d", lapDesaA.ID), nil)
		req.Header.Set("Authorization", "Bearer "+tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
	})

	// 6. Admin Pemdes Desa A cross-reads Desa B report -> 404 Not Found (scoped query)
	t.Run("Admin Pemdes Desa A cross-reads Desa B report -> 404 Not Found (Scope Guard)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d", lapDesaB.ID), nil)
		req.Header.Set("Authorization", "Bearer "+tokenPemdesA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found, got %d", w.Code)
		}
	})

	// 7. Admin PU reads Desa and Kabupaten reports -> 200 OK (all authorities monitoring)
	t.Run("Admin PU reads Desa report -> 200 OK (Monitoring requirement)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d", lapDesaA.ID), nil)
		req.Header.Set("Authorization", "Bearer "+tokenPU)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
	})

	// 8. Superadmin reads any report -> 200 OK
	t.Run("Superadmin reads any report -> 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d", lapDesaB.ID), nil)
		req.Header.Set("Authorization", "Bearer "+tokenSA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
	})

	// 9. Admin reads soft-deleted report -> 404 Not Found
	t.Run("Admin reads soft-deleted report -> 404 Not Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d", lapDeleted.ID), nil)
		req.Header.Set("Authorization", "Bearer "+tokenSA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found, got %d", w.Code)
		}
	})

	// 10. Invalid parameter IDs (string 'abc', '0') -> 400 Bad Request
	t.Run("Invalid parameter IDs rejected with 400", func(t *testing.T) {
		for _, invalidID := range []string{"abc", "0", "-5"} {
			reqW := httptest.NewRequest(http.MethodGet, "/api/warga/laporan/"+invalidID, nil)
			reqW.Header.Set("Authorization", "Bearer "+tokenWargaA)
			wW := httptest.NewRecorder()
			r.ServeHTTP(wW, reqW)
			if wW.Code != http.StatusBadRequest {
				t.Errorf("warga expected 400 for id '%s', got %d", invalidID, wW.Code)
			}

			reqA := httptest.NewRequest(http.MethodGet, "/api/admin/laporan/"+invalidID, nil)
			reqA.Header.Set("Authorization", "Bearer "+tokenSA)
			wA := httptest.NewRecorder()
			r.ServeHTTP(wA, reqA)
			if wA.Code != http.StatusBadRequest {
				t.Errorf("admin expected 400 for id '%s', got %d", invalidID, wA.Code)
			}
		}
	})
}

// =========================================================================
// AREA 19 TEST MATRIX C: STATUS TRANSITIONS & COMPLETION/REJECTION RULES
// =========================================================================
func TestLaporan_StatusTransitionLifecycleMatrix(t *testing.T) {
	if !ensureDBForLaporanTest(t) {
		t.Skip("Database not available")
	}

	r := gin.New()
	routes.SetupRoutes(r)

	desa := models.Wilayah{Nama: "Desa Lifecycle", Tipe: "desa"}
	config.DB.Create(&desa)
	defer config.DB.Unscoped().Delete(&desa)

	warga := models.User{Name: "Warga L", Email: fmt.Sprintf("wl_%d@roadis.id", time.Now().UnixNano()), Role: models.RoleWarga}
	config.DB.Create(&warga)
	defer config.DB.Unscoped().Delete(&warga)

	pemdes := models.User{Name: "Pemdes L", Email: fmt.Sprintf("pl_%d@roadis.id", time.Now().UnixNano()), Role: models.RoleAdminPemdes, WilayahID: &desa.ID}
	config.DB.Create(&pemdes)
	defer config.DB.Unscoped().Delete(&pemdes)

	adminPU := models.User{Name: "PU L", Email: fmt.Sprintf("pul_%d@roadis.id", time.Now().UnixNano()), Role: models.RoleAdminPu}
	config.DB.Create(&adminPU)
	defer config.DB.Unscoped().Delete(&adminPU)

	tokenPemdes, _ := utils.GenerateToken(pemdes.ID, pemdes.Email, pemdes.Role, nil)
	tokenPU, _ := utils.GenerateToken(adminPU.ID, adminPU.Email, adminPU.Role, nil)

	// Helper update status request
	sendUpdate := func(token string, reportID uint, status, catatanAdmin string, hasEvidence bool) *httptest.ResponseRecorder {
		fields := map[string]string{}
		if status != "" {
			fields["status"] = status
		}
		if catatanAdmin != "" {
			fields["catatan_admin"] = catatanAdmin
		}

		var fileBytes []byte
		var fileName string
		if hasEvidence {
			fileName = "bukti.jpg"
			fileBytes = []byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00\x01\x01\x01\x00`\x00`\x00\x00\xFF\xDB\x00C\x00")
		}

		origUploader := adminController.BuktiUploader
		adminController.BuktiUploader = func(fh *multipart.FileHeader) (string, error) {
			return "https://mock.cloudinary/bukti_lifecycle.jpg", nil
		}
		defer func() { adminController.BuktiUploader = origUploader }()

		req, contentType := createMultipartForm("foto_bukti", fileName, fileBytes, fields)
		req.Method = http.MethodPut
		req.URL.Path = fmt.Sprintf("/api/admin/laporan/%d/status", reportID)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", contentType)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// 1. MENUNGGU -> PROSES -> ALLOWED
	t.Run("menunggu -> proses is allowed", func(t *testing.T) {
		lap := models.LaporanKerusakan{
			UserID: warga.ID, WilayahID: desa.ID, JenisJalan: "desa", Judul: "Desa Menunggu",
			Status: "menunggu", Latitude: -6.34, Longitude: 108.33, ImageURL: "https://foto.jpg", TipeKerusakan: "rusak",
		}
		config.DB.Create(&lap)
		defer config.DB.Unscoped().Delete(&lap)

		w := sendUpdate(tokenPemdes, lap.ID, "proses", "", false)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 2. PROSES -> SELESAI without evidence -> REJECTED 400
	t.Run("proses -> selesai without evidence rejected with 400", func(t *testing.T) {
		lap := models.LaporanKerusakan{
			UserID: warga.ID, WilayahID: desa.ID, JenisJalan: "desa", Judul: "Desa Proses",
			Status: "proses", Latitude: -6.34, Longitude: 108.33, ImageURL: "https://foto.jpg", TipeKerusakan: "rusak",
		}
		config.DB.Create(&lap)
		defer config.DB.Unscoped().Delete(&lap)

		w := sendUpdate(tokenPemdes, lap.ID, "selesai", "", false)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for completion without evidence, got %d", w.Code)
		}
	})

	// 3. PROSES -> SELESAI with evidence -> ALLOWED 200
	t.Run("proses -> selesai with evidence allowed with 200", func(t *testing.T) {
		lap := models.LaporanKerusakan{
			UserID: warga.ID, WilayahID: desa.ID, JenisJalan: "desa", Judul: "Desa Selesai",
			Status: "proses", Latitude: -6.34, Longitude: 108.33, ImageURL: "https://foto.jpg", TipeKerusakan: "rusak",
		}
		config.DB.Create(&lap)
		defer config.DB.Unscoped().Delete(&lap)

		w := sendUpdate(tokenPemdes, lap.ID, "selesai", "", true)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 for completion with evidence, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 4. MENUNGGU -> DITOLAK without note -> REJECTED 400
	t.Run("rejection without note rejected with 400", func(t *testing.T) {
		lap := models.LaporanKerusakan{
			UserID: warga.ID, WilayahID: desa.ID, JenisJalan: "desa", Judul: "Desa Tolak",
			Status: "menunggu", Latitude: -6.34, Longitude: 108.33, ImageURL: "https://foto.jpg", TipeKerusakan: "rusak",
		}
		config.DB.Create(&lap)
		defer config.DB.Unscoped().Delete(&lap)

		w := sendUpdate(tokenPemdes, lap.ID, "ditolak", "", false)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for rejection without note, got %d", w.Code)
		}
	})

	// 5. MENUNGGU -> DITOLAK with note -> ALLOWED 200
	t.Run("rejection with note allowed with 200", func(t *testing.T) {
		lap := models.LaporanKerusakan{
			UserID: warga.ID, WilayahID: desa.ID, JenisJalan: "desa", Judul: "Desa Tolak Valid",
			Status: "menunggu", Latitude: -6.34, Longitude: 108.33, ImageURL: "https://foto.jpg", TipeKerusakan: "rusak",
		}
		config.DB.Create(&lap)
		defer config.DB.Unscoped().Delete(&lap)

		w := sendUpdate(tokenPemdes, lap.ID, "ditolak", "Laporan tidak valid", false)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 for rejection with note, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 6. INVALID TRANSITIONS FROM SELESAI
	t.Run("Transitions away from selesai are rejected with 400", func(t *testing.T) {
		for _, targetStatus := range []string{"menunggu", "proses", "ditolak"} {
			lap := models.LaporanKerusakan{
				UserID: warga.ID, WilayahID: desa.ID, JenisJalan: "desa", Judul: "Completed Report",
				Status: "selesai", FotoBukti: "https://foto.jpg", Latitude: -6.34, Longitude: 108.33, ImageURL: "https://foto.jpg", TipeKerusakan: "rusak",
			}
			config.DB.Create(&lap)

			w := sendUpdate(tokenPemdes, lap.ID, targetStatus, "Catatan", false)
			config.DB.Unscoped().Delete(&lap)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for illegal transition selesai -> %s, got %d", targetStatus, w.Code)
			}
		}
	})

	// 7. INVALID TRANSITION DITOLAK -> SELESAI
	t.Run("ditolak -> selesai rejected with 400", func(t *testing.T) {
		lap := models.LaporanKerusakan{
			UserID: warga.ID, WilayahID: desa.ID, JenisJalan: "desa", Judul: "Rejected Report",
			Status: "ditolak", CatatanAdmin: "Alasan", Latitude: -6.34, Longitude: 108.33, ImageURL: "https://foto.jpg", TipeKerusakan: "rusak",
		}
		config.DB.Create(&lap)
		defer config.DB.Unscoped().Delete(&lap)

		w := sendUpdate(tokenPemdes, lap.ID, "selesai", "", true)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for ditolak -> selesai transition, got %d", w.Code)
		}
	})

	// 8. Admin PU updating non-kabupaten report -> 403 Forbidden
	t.Run("Admin PU updating non-kabupaten report rejected with 403", func(t *testing.T) {
		lap := models.LaporanKerusakan{
			UserID: warga.ID, WilayahID: desa.ID, JenisJalan: "desa", Judul: "Desa Report",
			Status: "menunggu", Latitude: -6.34, Longitude: 108.33, ImageURL: "https://foto.jpg", TipeKerusakan: "rusak",
		}
		config.DB.Create(&lap)
		defer config.DB.Unscoped().Delete(&lap)

		w := sendUpdate(tokenPU, lap.ID, "proses", "", false)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for Admin PU updating Desa report, got %d", w.Code)
		}
	})
}

// =========================================================================
// AREA 19 TEST MATRIX E: DELETE SPAM BY SUPERADMIN
// =========================================================================
func TestLaporan_DeleteSpamMatrix(t *testing.T) {
	if !ensureDBForLaporanTest(t) {
		t.Skip("Database not available")
	}

	r := gin.New()
	routes.SetupRoutes(r)

	desa := models.Wilayah{Nama: "Desa Del", Tipe: "desa"}
	config.DB.Create(&desa)
	defer config.DB.Unscoped().Delete(&desa)

	warga := models.User{Name: "Warga Del", Email: fmt.Sprintf("wdel_%d@roadis.id", time.Now().UnixNano()), Role: models.RoleWarga}
	config.DB.Create(&warga)
	defer config.DB.Unscoped().Delete(&warga)

	superAdmin := models.User{Name: "SA Del", Email: fmt.Sprintf("sadel_%d@roadis.id", time.Now().UnixNano()), Role: models.RoleSuperAdmin}
	config.DB.Create(&superAdmin)
	defer config.DB.Unscoped().Delete(&superAdmin)

	tokenWarga, _ := utils.GenerateToken(warga.ID, warga.Email, warga.Role, nil)
	tokenSA, _ := utils.GenerateToken(superAdmin.ID, superAdmin.Email, superAdmin.Role, nil)

	// 1. Superadmin deletes spam report -> 200 OK & soft deleted
	t.Run("Superadmin deletes spam report -> 200 OK", func(t *testing.T) {
		lap := models.LaporanKerusakan{
			UserID: warga.ID, WilayahID: desa.ID, JenisJalan: "desa", Judul: "Spam Report",
			Status: "menunggu", Latitude: -6.34, Longitude: 108.33, ImageURL: "https://foto.jpg", TipeKerusakan: "spam",
		}
		config.DB.Create(&lap)
		defer config.DB.Unscoped().Delete(&lap)

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/superadmin/laporan/%d", lap.ID), nil)
		req.Header.Set("Authorization", "Bearer "+tokenSA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		// Verify soft-deleted
		var check models.LaporanKerusakan
		err := config.DB.First(&check, lap.ID).Error
		if err == nil {
			t.Errorf("expected report to be soft deleted, but was still found in active query")
		}
	})

	// 2. Non-superadmin cannot delete report -> 403 Forbidden
	t.Run("Non-superadmin delete attempt rejected with 403", func(t *testing.T) {
		lap := models.LaporanKerusakan{
			UserID: warga.ID, WilayahID: desa.ID, JenisJalan: "desa", Judul: "Normal Report",
			Status: "menunggu", Latitude: -6.34, Longitude: 108.33, ImageURL: "https://foto.jpg", TipeKerusakan: "rusak",
		}
		config.DB.Create(&lap)
		defer config.DB.Unscoped().Delete(&lap)

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/superadmin/laporan/%d", lap.ID), nil)
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for warga delete, got %d", w.Code)
		}
	})

	// 3. Deleting non-existent report -> 404 Not Found
	t.Run("Deleting non-existent report -> 404 Not Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/superadmin/laporan/999999", nil)
		req.Header.Set("Authorization", "Bearer "+tokenSA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found, got %d", w.Code)
		}
	})

	// 4. Invalid delete ID -> 400 Bad Request
	t.Run("Invalid delete ID string rejected with 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/superadmin/laporan/invalid_id", nil)
		req.Header.Set("Authorization", "Bearer "+tokenSA)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for invalid ID, got %d", w.Code)
		}
	})
}
