package controllers_test

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
	adminController "backend-jalan-rusak/controllers/admin"
	wargaController "backend-jalan-rusak/controllers/warga"
	"backend-jalan-rusak/middlewares"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/routes"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func init() {
	gin.SetMode(gin.TestMode)
	_ = os.Setenv("JWT_SECRET", "be17_integration_secret_key_1234567890")
}

func ensureBE17DB(t *testing.T) bool {
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
		t.Logf("MySQL connection unavailable in integration test environment (%v)", err)
		return false
	}
	_ = db.AutoMigrate(
		&models.User{},
		&models.Wilayah{},
		&models.LaporanKerusakan{},
		&models.RiwayatChat{},
		&models.Notifikasi{},
		&models.UserPreference{},
	)
	config.DB = db
	return true
}

func setupBE17Router() *gin.Engine {
	r := gin.New()
	r.Use(middlewares.CORSMiddleware())
	routes.SetupRoutes(r)
	return r
}


// 1. TestBE17_01_AuthLifecycle
func TestBE17_01_AuthLifecycle(t *testing.T) {
	if !ensureBE17DB(t) {
		t.Skip("MySQL not available for Auth Lifecycle integration test")
	}
	r := setupBE17Router()
	nano := time.Now().UnixNano()
	email := fmt.Sprintf("be17_auth_%d@roadis.local", nano)
	password := "Secret123456"

	// Step 1: Register
	regPayload, _ := json.Marshal(map[string]string{
		"name":     "Warga E2E",
		"email":    email,
		"password": password,
	})
	wReg := httptest.NewRecorder()
	reqReg := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBuffer(regPayload))
	reqReg.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wReg, reqReg)

	if wReg.Code != http.StatusCreated {
		t.Fatalf("Register failed: %d, body: %s", wReg.Code, wReg.Body.String())
	}

	var user models.User
	if err := config.DB.Where("email = ?", email).First(&user).Error; err != nil {
		t.Fatalf("User not saved in DB: %v", err)
	}
	defer config.DB.Unscoped().Delete(&user)

	// Step 2: Login
	loginPayload, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
	})
	wLogin := httptest.NewRecorder()
	reqLogin := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBuffer(loginPayload))
	reqLogin.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wLogin, reqLogin)

	if wLogin.Code != http.StatusOK {
		t.Fatalf("Login failed: %d, body: %s", wLogin.Code, wLogin.Body.String())
	}
	var loginResp map[string]interface{}
	_ = json.Unmarshal(wLogin.Body.Bytes(), &loginResp)
	token, ok := loginResp["token"].(string)
	if !ok || token == "" {
		t.Fatalf("Expected token string in login response, got: %v", loginResp["token"])
	}

	// Step 3: Access Protected Endpoint
	wProf := httptest.NewRecorder()
	reqProf := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	reqProf.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wProf, reqProf)
	if wProf.Code != http.StatusOK {
		t.Fatalf("Access protected endpoint failed: %d, body: %s", wProf.Code, wProf.Body.String())
	}

	// Step 4: Logout All (Revoke Token)
	wLogout := httptest.NewRecorder()
	reqLogout := httptest.NewRequest(http.MethodPost, "/api/settings/logout-all", nil)
	reqLogout.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wLogout, reqLogout)
	if wLogout.Code != http.StatusOK {
		t.Fatalf("Logout all failed: %d, body: %s", wLogout.Code, wLogout.Body.String())
	}

	// Step 5: Verify Old Token is Rejected (401)
	wReject := httptest.NewRecorder()
	reqReject := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	reqReject.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wReject, reqReject)
	if wReject.Code != http.StatusUnauthorized {
		t.Fatalf("Expected old token to be rejected 401 after logout-all, got: %d", wReject.Code)
	}
}

// 2. TestBE17_02_WargaLifecycle
func TestBE17_02_WargaLifecycle(t *testing.T) {
	if !ensureBE17DB(t) {
		t.Skip("MySQL not available for Warga Lifecycle integration test")
	}
	r := setupBE17Router()
	nano := time.Now().UnixNano()

	// Seed Wilayah
	wilayah := models.Wilayah{
		Nama: fmt.Sprintf("Desa Warga %d", nano),
		Tipe: "desa",
	}
	config.DB.Create(&wilayah)
	defer config.DB.Unscoped().Delete(&wilayah)

	// Seed Warga User
	wargaUser := models.User{
		Name:         "Warga Lifecycle",
		Email:        fmt.Sprintf("warga_lc_%d@roadis.local", nano),
		Password:     "hashedpassword123",
		Role:         models.RoleWarga,
		TokenVersion: 1,
	}
	config.DB.Create(&wargaUser)
	defer config.DB.Unscoped().Delete(&wargaUser)

	token, _ := utils.GenerateToken(wargaUser.ID, wargaUser.Email, wargaUser.Role, nil, 1)

	// A. Get Wilayah
	wWil := httptest.NewRecorder()
	reqWil := httptest.NewRequest(http.MethodGet, "/api/warga/wilayah", nil)
	reqWil.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wWil, reqWil)
	if wWil.Code != http.StatusOK {
		t.Fatalf("Get wilayah failed: %d", wWil.Code)
	}

	// B. Create Laporan via Multipart
	origUploader := wargaController.LaporanUploader
	wargaController.LaporanUploader = func(file *multipart.FileHeader) (string, error) {
		return "https://res.cloudinary.com/roadis/image/upload/v12345/test_laporan.jpg", nil
	}
	defer func() { wargaController.LaporanUploader = origUploader }()

	bodyBuf := &bytes.Buffer{}
	mpWriter := multipart.NewWriter(bodyBuf)
	_ = mpWriter.WriteField("judul", "Lubang Besar di Poros Desa")
	_ = mpWriter.WriteField("deskripsi", "Jalan berlubang cukup dalam")
	_ = mpWriter.WriteField("tipe_kerusakan", "Lubang")
	_ = mpWriter.WriteField("latitude", "-6.3265")
	_ = mpWriter.WriteField("longitude", "108.3221")
	_ = mpWriter.WriteField("wilayah_id", fmt.Sprintf("%d", wilayah.ID))
	_ = mpWriter.WriteField("jenis_jalan", "desa")

	part, _ := mpWriter.CreateFormFile("foto", "foto.jpg")
	// Minimal valid JPEG header bytes
	jpegHeader := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01}
	_, _ = part.Write(jpegHeader)
	_ = mpWriter.Close()

	wLap := httptest.NewRecorder()
	reqLap := httptest.NewRequest(http.MethodPost, "/api/warga/laporan", bodyBuf)
	reqLap.Header.Set("Authorization", "Bearer "+token)
	reqLap.Header.Set("Content-Type", mpWriter.FormDataContentType())
	r.ServeHTTP(wLap, reqLap)

	if wLap.Code != http.StatusOK {
		t.Fatalf("Create Laporan failed: %d, body: %s", wLap.Code, wLap.Body.String())
	}

	var createdLap models.LaporanKerusakan
	if err := config.DB.Where("user_id = ? AND wilayah_id = ?", wargaUser.ID, wilayah.ID).First(&createdLap).Error; err != nil {
		t.Fatalf("Laporan not found in DB: %v", err)
	}
	defer config.DB.Unscoped().Delete(&createdLap)

	// C. Get Riwayat Laporan
	wRiw := httptest.NewRecorder()
	reqRiw := httptest.NewRequest(http.MethodGet, "/api/warga/laporan", nil)
	reqRiw.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wRiw, reqRiw)
	if wRiw.Code != http.StatusOK {
		t.Fatalf("Get Riwayat Laporan failed: %d", wRiw.Code)
	}

	// D. Get Detail Laporan
	wDet := httptest.NewRecorder()
	reqDet := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/warga/laporan/%d", createdLap.ID), nil)
	reqDet.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wDet, reqDet)
	if wDet.Code != http.StatusOK {
		t.Fatalf("Get Detail Laporan failed: %d", wDet.Code)
	}

	// E. Send Chat on Own Report
	chatPayload, _ := json.Marshal(map[string]string{"pesan": "Mohon segera diperbaiki pak"})
	wChat := httptest.NewRecorder()
	reqChat := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/warga/laporan/%d/chat", createdLap.ID), bytes.NewBuffer(chatPayload))
	reqChat.Header.Set("Authorization", "Bearer "+token)
	reqChat.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wChat, reqChat)
	if wChat.Code != http.StatusOK {
		t.Fatalf("Send Chat failed: %d, body: %s", wChat.Code, wChat.Body.String())
	}
}

// 3. TestBE17_03_PemdesLifecycle
func TestBE17_03_PemdesLifecycle(t *testing.T) {
	if !ensureBE17DB(t) {
		t.Skip("MySQL not available for Pemdes Lifecycle integration test")
	}
	r := setupBE17Router()
	nano := time.Now().UnixNano()

	wilayahDesa := models.Wilayah{Nama: fmt.Sprintf("Desa Pemdes %d", nano), Tipe: "desa"}
	wilayahLain := models.Wilayah{Nama: fmt.Sprintf("Desa Lain %d", nano), Tipe: "desa"}
	config.DB.Create(&wilayahDesa)
	config.DB.Create(&wilayahLain)
	defer config.DB.Unscoped().Delete(&wilayahDesa)
	defer config.DB.Unscoped().Delete(&wilayahLain)

	adminPemdes := models.User{
		Name:         "Admin Pemdes E2E",
		Email:        fmt.Sprintf("pemdes_%d@roadis.local", nano),
		Password:     "hashedpassword123",
		Role:         models.RoleAdminPemdes,
		WilayahID:    &wilayahDesa.ID,
		TokenVersion: 1,
	}
	config.DB.Create(&adminPemdes)
	defer config.DB.Unscoped().Delete(&adminPemdes)

	token, _ := utils.GenerateToken(adminPemdes.ID, adminPemdes.Email, adminPemdes.Role, adminPemdes.WilayahID, 1)

	// Laporan Desa Sendiri
	lapOwn := models.LaporanKerusakan{
		Judul:         "Jalan Rusak Desa Sendiri",
		JenisJalan:    "desa",
		WilayahID:     wilayahDesa.ID,
		UserID:        adminPemdes.ID,
		Status:        "menunggu",
		Latitude:      -6.32,
		Longitude:     108.32,
		TipeKerusakan: "Lubang",
	}
	config.DB.Create(&lapOwn)
	defer config.DB.Unscoped().Delete(&lapOwn)

	// Laporan Wilayah Lain
	lapForeign := models.LaporanKerusakan{
		Judul:         "Jalan Rusak Desa Sebelah",
		JenisJalan:    "desa",
		WilayahID:     wilayahLain.ID,
		UserID:        adminPemdes.ID,
		Status:        "menunggu",
		Latitude:      -6.33,
		Longitude:     108.33,
		TipeKerusakan: "Lubang",
	}
	config.DB.Create(&lapForeign)
	defer config.DB.Unscoped().Delete(&lapForeign)

	// A. Dashboard Stats
	wDash := httptest.NewRecorder()
	reqDash := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard", nil)
	reqDash.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wDash, reqDash)
	if wDash.Code != http.StatusOK {
		t.Fatalf("Pemdes Dashboard failed: %d", wDash.Code)
	}

	// B. List Laporan (should only include own wilayah)
	wList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/admin/laporan", nil)
	reqList.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wList, reqList)
	if wList.Code != http.StatusOK {
		t.Fatalf("Pemdes List failed: %d", wList.Code)
	}
	if strings.Contains(wList.Body.String(), "Jalan Rusak Desa Sebelah") {
		t.Errorf("Pemdes received foreign wilayah reports in list: %s", wList.Body.String())
	}

	// C. Update Own Report (Allowed)
	upOwn, _ := json.Marshal(map[string]string{"status": "proses"})
	wUpOwn := httptest.NewRecorder()
	reqUpOwn := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/laporan/%d/status", lapOwn.ID), bytes.NewBuffer(upOwn))
	reqUpOwn.Header.Set("Authorization", "Bearer "+token)
	reqUpOwn.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wUpOwn, reqUpOwn)
	if wUpOwn.Code != http.StatusOK {
		t.Fatalf("Expected 200 on update own report, got %d: %s", wUpOwn.Code, wUpOwn.Body.String())
	}

	// D. Update Foreign Report (Must be 403 Forbidden)
	upFor, _ := json.Marshal(map[string]string{"status": "proses"})
	wUpFor := httptest.NewRecorder()
	reqUpFor := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/laporan/%d/status", lapForeign.ID), bytes.NewBuffer(upFor))
	reqUpFor.Header.Set("Authorization", "Bearer "+token)
	reqUpFor.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wUpFor, reqUpFor)
	if wUpFor.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 on update foreign report, got %d", wUpFor.Code)
	}
}

// 4. TestBE17_04_PULifecycle
func TestBE17_04_PULifecycle(t *testing.T) {
	if !ensureBE17DB(t) {
		t.Skip("MySQL not available for PU Lifecycle integration test")
	}
	r := setupBE17Router()
	nano := time.Now().UnixNano()

	wilayah := models.Wilayah{Nama: fmt.Sprintf("Wilayah PU %d", nano), Tipe: "kabupaten"}
	config.DB.Create(&wilayah)
	defer config.DB.Unscoped().Delete(&wilayah)

	adminPU := models.User{
		Name:         "Admin PU E2E",
		Email:        fmt.Sprintf("pu_%d@roadis.local", nano),
		Password:     "hashedpassword123",
		Role:         models.RoleAdminPu,
		TokenVersion: 1,
	}
	config.DB.Create(&adminPU)
	defer config.DB.Unscoped().Delete(&adminPU)

	token, _ := utils.GenerateToken(adminPU.ID, adminPU.Email, adminPU.Role, nil, 1)

	// Reports of different road types
	lapKab := models.LaporanKerusakan{
		Judul: "Jalan Kabupaten", JenisJalan: "kabupaten", WilayahID: wilayah.ID, UserID: adminPU.ID, Status: "menunggu", Latitude: -6.32, Longitude: 108.32,
	}
	lapDesa := models.LaporanKerusakan{
		Judul: "Jalan Desa", JenisJalan: "desa", WilayahID: wilayah.ID, UserID: adminPU.ID, Status: "menunggu", Latitude: -6.33, Longitude: 108.33,
	}
	config.DB.Create(&lapKab)
	config.DB.Create(&lapDesa)
	defer config.DB.Unscoped().Delete(&lapKab)
	defer config.DB.Unscoped().Delete(&lapDesa)

	// A. Read Detail Desa Report (PU-5.1: Read is Allowed)
	wReadDesa := httptest.NewRecorder()
	reqReadDesa := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d", lapDesa.ID), nil)
	reqReadDesa.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wReadDesa, reqReadDesa)
	if wReadDesa.Code != http.StatusOK {
		t.Fatalf("Admin PU should be allowed to view detail of desa report, got: %d", wReadDesa.Code)
	}

	// B. Update Desa Report (Forbidden: PU can only update kabupaten)
	upDesa, _ := json.Marshal(map[string]string{"status": "proses"})
	wUpDesa := httptest.NewRecorder()
	reqUpDesa := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/laporan/%d/status", lapDesa.ID), bytes.NewBuffer(upDesa))
	reqUpDesa.Header.Set("Authorization", "Bearer "+token)
	reqUpDesa.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wUpDesa, reqUpDesa)
	if wUpDesa.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 when Admin PU updates desa report, got %d", wUpDesa.Code)
	}

	// C. Update Kabupaten Report (Allowed: 200 OK)
	upKab, _ := json.Marshal(map[string]string{"status": "proses"})
	wUpKab := httptest.NewRecorder()
	reqUpKab := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/laporan/%d/status", lapKab.ID), bytes.NewBuffer(upKab))
	reqUpKab.Header.Set("Authorization", "Bearer "+token)
	reqUpKab.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wUpKab, reqUpKab)
	if wUpKab.Code != http.StatusOK {
		t.Fatalf("Expected 200 when Admin PU updates kabupaten report, got %d: %s", wUpKab.Code, wUpKab.Body.String())
	}

	// D. Default List: Admin PU sees reports across all authorities (contains both lapKab and lapDesa)
	wList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/admin/laporan?limit=100", nil)
	reqList.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wList, reqList)
	if wList.Code != http.StatusOK {
		t.Fatalf("Expected 200 for Admin PU report list, got %d", wList.Code)
	}
	var respList struct {
		Data []models.LaporanKerusakan `json:"data"`
	}
	_ = json.Unmarshal(wList.Body.Bytes(), &respList)
	foundKab, foundDesa := false, false
	for _, lap := range respList.Data {
		if lap.ID == lapKab.ID {
			foundKab = true
		}
		if lap.ID == lapDesa.ID {
			foundDesa = true
		}
	}
	if !foundKab || !foundDesa {
		t.Errorf("Expected Admin PU default list to contain both Kabupaten and Desa reports, foundKab=%v, foundDesa=%v", foundKab, foundDesa)
	}

	// E. Filtered List: Admin PU filters by jenis_jalan=desa
	wFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/admin/laporan?jenis_jalan=desa&limit=100", nil)
	reqFilter.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wFilter, reqFilter)
	if wFilter.Code != http.StatusOK {
		t.Fatalf("Expected 200 for Admin PU filtered report list, got %d", wFilter.Code)
	}

	// F. Dashboard Stats: Admin PU only counts kabupaten
	wDash := httptest.NewRecorder()
	reqDash := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard", nil)
	reqDash.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wDash, reqDash)
	if wDash.Code != http.StatusOK {
		t.Fatalf("Expected 200 for Admin PU dashboard, got %d", wDash.Code)
	}

	// G. Map Reports: Admin PU monitors all authorities
	wMap := httptest.NewRecorder()
	reqMap := httptest.NewRequest(http.MethodGet, "/api/admin/map/laporan", nil)
	reqMap.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wMap, reqMap)
	if wMap.Code != http.StatusOK {
		t.Fatalf("Expected 200 for Admin PU map, got %d", wMap.Code)
	}
}

// 5. TestBE17_05_SuperadminLifecycle
func TestBE17_05_SuperadminLifecycle(t *testing.T) {
	if !ensureBE17DB(t) {
		t.Skip("MySQL not available for Superadmin Lifecycle integration test")
	}
	r := setupBE17Router()
	nano := time.Now().UnixNano()

	superUser := models.User{
		Name:         "Superadmin E2E",
		Email:        fmt.Sprintf("super_%d@roadis.local", nano),
		Password:     "hashedpassword123",
		Role:         models.RoleSuperAdmin,
		TokenVersion: 1,
	}
	config.DB.Create(&superUser)
	defer config.DB.Unscoped().Delete(&superUser)

	token, _ := utils.GenerateToken(superUser.ID, superUser.Email, superUser.Role, nil, 1)

	// A. Create Wilayah
	wilPayload, _ := json.Marshal(map[string]string{
		"nama": fmt.Sprintf("Wilayah Super %d", nano),
		"tipe": "desa",
	})
	wWil := httptest.NewRecorder()
	reqWil := httptest.NewRequest(http.MethodPost, "/api/superadmin/wilayah", bytes.NewBuffer(wilPayload))
	reqWil.Header.Set("Authorization", "Bearer "+token)
	reqWil.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wWil, reqWil)
	if wWil.Code != http.StatusCreated {
		t.Fatalf("Create Wilayah failed: %d", wWil.Code)
	}

	var createdWil models.Wilayah
	_ = config.DB.Where("nama = ?", fmt.Sprintf("Wilayah Super %d", nano)).First(&createdWil)
	defer config.DB.Unscoped().Delete(&createdWil)

	// B. Create User
	userPayload, _ := json.Marshal(map[string]interface{}{
		"name":       "Staff Pemdes Baru",
		"email":      fmt.Sprintf("staff_%d@roadis.local", nano),
		"password":   "secret123456",
		"role":       "admin_pemdes",
		"wilayah_id": createdWil.ID,
	})
	wUser := httptest.NewRecorder()
	reqUser := httptest.NewRequest(http.MethodPost, "/api/superadmin/users", bytes.NewBuffer(userPayload))
	reqUser.Header.Set("Authorization", "Bearer "+token)
	reqUser.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wUser, reqUser)
	if wUser.Code != http.StatusOK {
		t.Fatalf("Create User failed: %d, body: %s", wUser.Code, wUser.Body.String())
	}

	var createdUser models.User
	_ = config.DB.Where("email = ?", fmt.Sprintf("staff_%d@roadis.local", nano)).First(&createdUser)
	defer config.DB.Unscoped().Delete(&createdUser)

	// C. Self Delete Guard (Superadmin cannot delete self)
	wSelfDel := httptest.NewRecorder()
	reqSelfDel := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/superadmin/users/%d", superUser.ID), nil)
	reqSelfDel.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wSelfDel, reqSelfDel)
	if wSelfDel.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 on self delete, got %d", wSelfDel.Code)
	}

	// D. Wilayah Dependency Guard (Cannot delete wilayah while user is attached)
	wWilDel := httptest.NewRecorder()
	reqWilDel := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/superadmin/wilayah/%d", createdWil.ID), nil)
	reqWilDel.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wWilDel, reqWilDel)
	if wWilDel.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 on deleting referenced wilayah, got %d", wWilDel.Code)
	}
}

// 6. TestBE17_06_ReportLifecycle
func TestBE17_06_ReportLifecycle(t *testing.T) {
	if !ensureBE17DB(t) {
		t.Skip("MySQL not available for Report Lifecycle integration test")
	}
	r := setupBE17Router()
	nano := time.Now().UnixNano()

	wilayah := models.Wilayah{Nama: fmt.Sprintf("Wilayah Life %d", nano), Tipe: "kabupaten"}
	config.DB.Create(&wilayah)
	defer config.DB.Unscoped().Delete(&wilayah)

	adminPU := models.User{
		Name:         "PU Lifecycle",
		Email:        fmt.Sprintf("pu_life_%d@roadis.local", nano),
		Password:     "hashedpassword123",
		Role:         models.RoleAdminPu,
		TokenVersion: 1,
	}
	config.DB.Create(&adminPU)
	defer config.DB.Unscoped().Delete(&adminPU)

	token, _ := utils.GenerateToken(adminPU.ID, adminPU.Email, adminPU.Role, nil, 1)

	laporan := models.LaporanKerusakan{
		Judul:         "Laporan Full Lifecycle",
		JenisJalan:    "kabupaten",
		WilayahID:     wilayah.ID,
		UserID:        adminPU.ID,
		Status:        "menunggu",
		Latitude:      -6.32,
		Longitude:     108.32,
		TipeKerusakan: "Amblas",
	}
	config.DB.Create(&laporan)
	defer config.DB.Unscoped().Delete(&laporan)

	// Step 1: Transisi MENUNGGU -> PROSES
	upProses, _ := json.Marshal(map[string]string{"status": "proses", "ditugaskan_ke": "Tim Reaksi Cepat"})
	wP := httptest.NewRecorder()
	reqP := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/laporan/%d/status", laporan.ID), bytes.NewBuffer(upProses))
	reqP.Header.Set("Authorization", "Bearer "+token)
	reqP.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wP, reqP)
	if wP.Code != http.StatusOK {
		t.Fatalf("Update to proses failed: %d", wP.Code)
	}

	// Step 2: Negative: PROSES -> SELESAI tanpa bukti foto
	upSelesaiNoBukti, _ := json.Marshal(map[string]string{"status": "selesai"})
	wSNo := httptest.NewRecorder()
	reqSNo := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/laporan/%d/status", laporan.ID), bytes.NewBuffer(upSelesaiNoBukti))
	reqSNo.Header.Set("Authorization", "Bearer "+token)
	reqSNo.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wSNo, reqSNo)
	if wSNo.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 when finishing report without evidence photo, got %d", wSNo.Code)
	}

	// Step 3: PROSES -> SELESAI dengan foto bukti (simulasi form-data)
	origBuktiUploader := adminController.BuktiUploader
	adminController.BuktiUploader = func(file *multipart.FileHeader) (string, error) {
		return "https://res.cloudinary.com/roadis/image/upload/v12345/bukti_selesai.jpg", nil
	}
	defer func() { adminController.BuktiUploader = origBuktiUploader }()

	bodyBuf := &bytes.Buffer{}
	mpWriter := multipart.NewWriter(bodyBuf)
	_ = mpWriter.WriteField("status", "selesai")
	part, _ := mpWriter.CreateFormFile("foto_bukti", "bukti.jpg")
	_, _ = part.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01})
	_ = mpWriter.Close()

	wS := httptest.NewRecorder()
	reqS := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/laporan/%d/status", laporan.ID), bodyBuf)
	reqS.Header.Set("Authorization", "Bearer "+token)
	reqS.Header.Set("Content-Type", mpWriter.FormDataContentType())
	r.ServeHTTP(wS, reqS)
	if wS.Code != http.StatusOK {
		t.Fatalf("Update to selesai with evidence photo failed: %d, body: %s", wS.Code, wS.Body.String())
	}

	// Step 4: Terminal state guard: SELESAI cannot be modified anymore
	upPostSelesai, _ := json.Marshal(map[string]string{"status": "proses"})
	wPost := httptest.NewRecorder()
	reqPost := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/laporan/%d/status", laporan.ID), bytes.NewBuffer(upPostSelesai))
	reqPost.Header.Set("Authorization", "Bearer "+token)
	reqPost.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wPost, reqPost)
	if wPost.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 when modifying completed report, got %d", wPost.Code)
	}
}

// 7. TestBE17_07_ChatLifecycle
func TestBE17_07_ChatLifecycle(t *testing.T) {
	if !ensureBE17DB(t) {
		t.Skip("MySQL not available for Chat Lifecycle integration test")
	}
	r := setupBE17Router()
	nano := time.Now().UnixNano()

	wilayah := models.Wilayah{Nama: fmt.Sprintf("Wilayah Chat %d", nano), Tipe: "desa"}
	config.DB.Create(&wilayah)
	defer config.DB.Unscoped().Delete(&wilayah)

	warga := models.User{Name: "Warga Chat", Email: fmt.Sprintf("wc_%d@roadis.local", nano), Password: "hash", Role: models.RoleWarga}
	pemdes := models.User{Name: "Pemdes Chat", Email: fmt.Sprintf("pc_%d@roadis.local", nano), Password: "hash", Role: models.RoleAdminPemdes, WilayahID: &wilayah.ID}
	otherWarga := models.User{Name: "Other Warga", Email: fmt.Sprintf("ow_%d@roadis.local", nano), Password: "hash", Role: models.RoleWarga}
	config.DB.Create(&warga)
	config.DB.Create(&pemdes)
	config.DB.Create(&otherWarga)
	defer config.DB.Unscoped().Delete(&warga)
	defer config.DB.Unscoped().Delete(&pemdes)
	defer config.DB.Unscoped().Delete(&otherWarga)

	laporan := models.LaporanKerusakan{
		Judul: "Laporan Chat E2E", JenisJalan: "desa", WilayahID: wilayah.ID, UserID: warga.ID, Status: "menunggu",
	}
	config.DB.Create(&laporan)
	defer config.DB.Unscoped().Delete(&laporan)

	tokenWarga, _ := utils.GenerateToken(warga.ID, warga.Email, warga.Role, nil, 1)
	tokenPemdes, _ := utils.GenerateToken(pemdes.ID, pemdes.Email, pemdes.Role, pemdes.WilayahID, 1)
	tokenOther, _ := utils.GenerateToken(otherWarga.ID, otherWarga.Email, otherWarga.Role, nil, 1)

	// A. Other Warga IDOR check on Chat (Must be 403 Forbidden)
	pOther, _ := json.Marshal(map[string]string{"pesan": "Hacker chat"})
	wIdor := httptest.NewRecorder()
	reqIdor := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/warga/laporan/%d/chat", laporan.ID), bytes.NewBuffer(pOther))
	reqIdor.Header.Set("Authorization", "Bearer "+tokenOther)
	reqIdor.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wIdor, reqIdor)
	if wIdor.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 on IDOR chat attempt, got %d", wIdor.Code)
	}

	// B. Owner Warga sends chat
	pWarga, _ := json.Marshal(map[string]string{"pesan": "Kapan mulai dikerjakan?"})
	wSend := httptest.NewRecorder()
	reqSend := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/warga/laporan/%d/chat", laporan.ID), bytes.NewBuffer(pWarga))
	reqSend.Header.Set("Authorization", "Bearer "+tokenWarga)
	reqSend.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wSend, reqSend)
	if wSend.Code != http.StatusOK {
		t.Fatalf("Owner chat failed: %d", wSend.Code)
	}

	var chatRecord models.RiwayatChat
	config.DB.Where("laporan_kerusakan_id = ? AND user_id = ?", laporan.ID, warga.ID).First(&chatRecord)
	defer config.DB.Unscoped().Delete(&chatRecord)

	// C. Admin Pemdes replies
	pReply, _ := json.Marshal(map[string]string{"balasan": "Tim kami sedang meluncur ke lokasi"})
	wReply := httptest.NewRecorder()
	reqReply := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/chat/%d", chatRecord.ID), bytes.NewBuffer(pReply))
	reqReply.Header.Set("Authorization", "Bearer "+tokenPemdes)
	reqReply.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wReply, reqReply)
	if wReply.Code != http.StatusOK {
		t.Fatalf("Admin reply failed: %d, body: %s", wReply.Code, wReply.Body.String())
	}
}

// 8. TestBE17_08_NotificationLifecycle
func TestBE17_08_NotificationLifecycle(t *testing.T) {
	if !ensureBE17DB(t) {
		t.Skip("MySQL not available for Notification Lifecycle integration test")
	}
	r := setupBE17Router()
	nano := time.Now().UnixNano()

	userA := models.User{Name: "User Notif A", Email: fmt.Sprintf("na_%d@roadis.local", nano), Password: "hash", Role: models.RoleWarga}
	userB := models.User{Name: "User Notif B", Email: fmt.Sprintf("nb_%d@roadis.local", nano), Password: "hash", Role: models.RoleWarga}
	config.DB.Create(&userA)
	config.DB.Create(&userB)
	defer config.DB.Unscoped().Delete(&userA)
	defer config.DB.Unscoped().Delete(&userB)

	notifA := models.Notifikasi{UserID: userA.ID, Judul: "Notif A", Pesan: "Pesan A", IsRead: false}
	config.DB.Create(&notifA)
	defer config.DB.Unscoped().Delete(&notifA)

	tokenA, _ := utils.GenerateToken(userA.ID, userA.Email, userA.Role, nil, 1)
	tokenB, _ := utils.GenerateToken(userB.ID, userB.Email, userB.Role, nil, 1)

	// A. User B attempts to read User A's notification (Must be 403 or 404)
	wIdor := httptest.NewRecorder()
	reqIdor := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/notifikasi/%d/read", notifA.ID), nil)
	reqIdor.Header.Set("Authorization", "Bearer "+tokenB)
	r.ServeHTTP(wIdor, reqIdor)
	if wIdor.Code != http.StatusNotFound && wIdor.Code != http.StatusForbidden {
		t.Fatalf("Expected 404/403 on IDOR read notification, got %d", wIdor.Code)
	}

	// B. User A marks notification as read
	wRead := httptest.NewRecorder()
	reqRead := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/notifikasi/%d/read", notifA.ID), nil)
	reqRead.Header.Set("Authorization", "Bearer "+tokenA)
	r.ServeHTTP(wRead, reqRead)
	if wRead.Code != http.StatusOK {
		t.Fatalf("Mark notifikasi read failed: %d", wRead.Code)
	}

	// C. User A reads all notif
	wReadAll := httptest.NewRecorder()
	reqReadAll := httptest.NewRequest(http.MethodPut, "/api/notifikasi/read-all", nil)
	reqReadAll.Header.Set("Authorization", "Bearer "+tokenA)
	r.ServeHTTP(wReadAll, reqReadAll)
	if wReadAll.Code != http.StatusOK {
		t.Fatalf("Mark all read failed: %d", wReadAll.Code)
	}

	// D. User A deletes notification
	wDel := httptest.NewRecorder()
	reqDel := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/notifikasi/%d", notifA.ID), nil)
	reqDel.Header.Set("Authorization", "Bearer "+tokenA)
	r.ServeHTTP(wDel, reqDel)
	if wDel.Code != http.StatusOK {
		t.Fatalf("Delete notification failed: %d", wDel.Code)
	}
}

// 9. TestBE17_09_ProfileSettingsLifecycle
func TestBE17_09_ProfileSettingsLifecycle(t *testing.T) {
	if !ensureBE17DB(t) {
		t.Skip("MySQL not available for Profile/Settings Lifecycle integration test")
	}
	r := setupBE17Router()
	nano := time.Now().UnixNano()

	user := models.User{
		Name:         "User Profile E2E",
		Email:        fmt.Sprintf("prof_%d@roadis.local", nano),
		Password:     "hashedpassword123",
		Role:         models.RoleWarga,
		TokenVersion: 1,
	}
	config.DB.Create(&user)
	defer func() {
		config.DB.Unscoped().Where("user_id = ?", user.ID).Delete(&models.UserPreference{})
		config.DB.Unscoped().Delete(&user)
	}()

	token, _ := utils.GenerateToken(user.ID, user.Email, user.Role, nil, 1)

	// A. Get Profile
	wGetProf := httptest.NewRecorder()
	reqGetProf := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	reqGetProf.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wGetProf, reqGetProf)
	if wGetProf.Code != http.StatusOK {
		t.Fatalf("Get profile failed: %d", wGetProf.Code)
	}

	// B. Update Profile (Name & Phone)
	phone := "081234567890"
	name := "User Profile Updated"
	upPayload, _ := json.Marshal(map[string]string{"name": name, "phone": phone})
	wUpProf := httptest.NewRecorder()
	reqUpProf := httptest.NewRequest(http.MethodPut, "/api/profile", bytes.NewBuffer(upPayload))
	reqUpProf.Header.Set("Authorization", "Bearer "+token)
	reqUpProf.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wUpProf, reqUpProf)
	if wUpProf.Code != http.StatusOK {
		t.Fatalf("Update profile failed: %d", wUpProf.Code)
	}

	// C. Get Settings & Idempotent Preference Creation
	wGetSet := httptest.NewRecorder()
	reqGetSet := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	reqGetSet.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wGetSet, reqGetSet)
	if wGetSet.Code != http.StatusOK {
		t.Fatalf("Get settings failed: %d", wGetSet.Code)
	}

	// D. Update Settings
	theme := "dark"
	sound := true
	setPayload, _ := json.Marshal(map[string]interface{}{"theme": theme, "notification_sound_enabled": sound})
	wUpSet := httptest.NewRecorder()
	reqUpSet := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewBuffer(setPayload))
	reqUpSet.Header.Set("Authorization", "Bearer "+token)
	reqUpSet.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wUpSet, reqUpSet)
	if wUpSet.Code != http.StatusOK {
		t.Fatalf("Update settings failed: %d", wUpSet.Code)
	}
}

// 10. TestBE17_10_MapLifecycle
func TestBE17_10_MapLifecycle(t *testing.T) {
	if !ensureBE17DB(t) {
		t.Skip("MySQL not available for Map Lifecycle integration test")
	}
	r := setupBE17Router()
	nano := time.Now().UnixNano()

	wilayah := models.Wilayah{Nama: fmt.Sprintf("Wilayah Map %d", nano), Tipe: "desa"}
	config.DB.Create(&wilayah)
	defer config.DB.Unscoped().Delete(&wilayah)

	user := models.User{Name: "User Map", Email: fmt.Sprintf("map_%d@roadis.local", nano), Password: "hash", Role: models.RoleWarga}
	config.DB.Create(&user)
	defer config.DB.Unscoped().Delete(&user)

	// Seed one valid coordinate and one invalid coordinate
	lapValid := models.LaporanKerusakan{
		Judul: "Titik Valid", JenisJalan: "desa", WilayahID: wilayah.ID, UserID: user.ID, Status: "menunggu", Latitude: -6.3265, Longitude: 108.3221,
	}
	lapInvalid := models.LaporanKerusakan{
		Judul: "Titik Invalid", JenisJalan: "desa", WilayahID: wilayah.ID, UserID: user.ID, Status: "menunggu", Latitude: 999.0, Longitude: 999.0,
	}
	config.DB.Create(&lapValid)
	config.DB.Create(&lapInvalid)
	defer config.DB.Unscoped().Delete(&lapValid)
	defer config.DB.Unscoped().Delete(&lapInvalid)

	token, _ := utils.GenerateToken(user.ID, user.Email, user.Role, nil, 1)

	// Warga Map Query
	wMap := httptest.NewRecorder()
	reqMap := httptest.NewRequest(http.MethodGet, "/api/warga/laporan/peta", nil)
	reqMap.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wMap, reqMap)

	if wMap.Code != http.StatusOK {
		t.Fatalf("Warga map query failed: %d", wMap.Code)
	}
	bodyStr := wMap.Body.String()
	if !strings.Contains(bodyStr, "Titik Valid") {
		t.Errorf("Expected valid coordinate point to be included in map response: %s", bodyStr)
	}
	if strings.Contains(bodyStr, "Titik Invalid") {
		t.Errorf("Invalid coordinate point must be excluded from map points: %s", bodyStr)
	}
}

// 11. TestBE17_11_FileUploadLifecycle
func TestBE17_11_FileUploadLifecycle(t *testing.T) {
	// A. Valid image extensions sniffing test
	validJpeg := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01}
	validPng := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D}
	fakeExe := []byte("MZ\x90\x00\x03\x00\x00\x00\x04\x00\x00\x00\xFF\xFF")

	checkMime := func(b []byte, expected string) bool {
		mime := http.DetectContentType(b)
		return strings.HasPrefix(mime, expected)
	}

	if !checkMime(validJpeg, "image/jpeg") {
		t.Errorf("expected JPEG detection")
	}
	if !checkMime(validPng, "image/png") {
		t.Errorf("expected PNG detection")
	}
	if checkMime(fakeExe, "image/") {
		t.Errorf("executable should not be detected as image")
	}
}

// 12. TestBE17_12_DatabaseRelationshipLifecycle
func TestBE17_12_DatabaseRelationshipLifecycle(t *testing.T) {
	if !ensureBE17DB(t) {
		t.Skip("MySQL not available for Database Relationship integration test")
	}
	nano := time.Now().UnixNano()

	wilayah := models.Wilayah{Nama: fmt.Sprintf("Rel Wil %d", nano), Tipe: "desa"}
	config.DB.Create(&wilayah)
	defer config.DB.Unscoped().Delete(&wilayah)

	user := models.User{
		Name:      "User Rel",
		Email:     fmt.Sprintf("rel_%d@roadis.local", nano),
		Password:  "hash",
		Role:      models.RoleAdminPemdes,
		WilayahID: &wilayah.ID,
	}
	config.DB.Create(&user)
	defer config.DB.Unscoped().Delete(&user)

	var loadedUser models.User
	if err := config.DB.Preload("Wilayah").First(&loadedUser, user.ID).Error; err != nil {
		t.Fatalf("Preload Wilayah failed: %v", err)
	}
	if loadedUser.Wilayah.ID != wilayah.ID {
		t.Errorf("Expected associated wilayah ID %d, got %d", wilayah.ID, loadedUser.Wilayah.ID)
	}
}

// 13. TestBE17_13_ResponseContractLifecycle
func TestBE17_13_ResponseContractLifecycle(t *testing.T) {
	r := setupBE17Router()

	// A. Health Check Contract
	wH := httptest.NewRecorder()
	reqH := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	r.ServeHTTP(wH, reqH)
	if wH.Code != http.StatusOK {
		t.Fatalf("Health check failed: %d", wH.Code)
	}
	var respH map[string]interface{}
	_ = json.Unmarshal(wH.Body.Bytes(), &respH)
	if respH["service"] != "roadis-api" {
		t.Errorf("expected service: roadis-api, got: %v", respH["service"])
	}

	// B. CORS & Security Headers Check
	wCors := httptest.NewRecorder()
	reqCors := httptest.NewRequest(http.MethodOptions, "/api/health", nil)
	reqCors.Header.Set("Origin", "http://localhost:5173")
	r.ServeHTTP(wCors, reqCors)
	if wCors.Code != http.StatusNoContent {
		t.Errorf("Expected 204 No Content for CORS preflight, got: %d", wCors.Code)
	}
	if wCors.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("Expected dynamic origin in CORS response, got: %v", wCors.Header().Get("Access-Control-Allow-Origin"))
	}
	if wCors.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("Expected nosniff header, got: %v", wCors.Header().Get("X-Content-Type-Options"))
	}
	if wCors.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("Expected DENY frame options, got: %v", wCors.Header().Get("X-Frame-Options"))
	}

	// C. Auth Failure Contract
	wErr := httptest.NewRecorder()
	reqErr := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	r.ServeHTTP(wErr, reqErr)
	if wErr.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", wErr.Code)
	}
	var respErr map[string]interface{}
	_ = json.Unmarshal(wErr.Body.Bytes(), &respErr)
	if respErr["status"] != "error" || respErr["message"] == nil || respErr["error"] == nil {
		t.Errorf("Error envelope contract violation: %v", respErr)
	}
}

// 14. TestBE17_14_CrossRoleSecurityMatrix
func TestBE17_14_CrossRoleSecurityMatrix(t *testing.T) {
	if !ensureBE17DB(t) {
		t.Skip("MySQL not available for CrossRoleSecurityMatrix integration test")
	}
	r := setupBE17Router()
	nano := time.Now().UnixNano()

	wilayahDesa := models.Wilayah{Nama: fmt.Sprintf("Matrix Desa %d", nano), Tipe: "desa"}
	config.DB.Create(&wilayahDesa)
	defer config.DB.Unscoped().Delete(&wilayahDesa)

	uWarga := models.User{
		Name:         "Matrix Warga",
		Email:        fmt.Sprintf("warga_%d@roadis.local", nano),
		Password:     "secret123",
		Role:         models.RoleWarga,
		TokenVersion: 1,
	}
	uPemdes := models.User{
		Name:         "Matrix Pemdes",
		Email:        fmt.Sprintf("pemdes_%d@roadis.local", nano),
		Password:     "secret123",
		Role:         models.RoleAdminPemdes,
		WilayahID:    &wilayahDesa.ID,
		TokenVersion: 1,
	}
	uPU := models.User{
		Name:         "Matrix PU",
		Email:        fmt.Sprintf("pu_%d@roadis.local", nano),
		Password:     "secret123",
		Role:         models.RoleAdminPu,
		TokenVersion: 1,
	}
	config.DB.Create(&uWarga)
	config.DB.Create(&uPemdes)
	config.DB.Create(&uPU)
	defer config.DB.Unscoped().Delete(&uWarga)
	defer config.DB.Unscoped().Delete(&uPemdes)
	defer config.DB.Unscoped().Delete(&uPU)

	tokenWarga, _ := utils.GenerateToken(uWarga.ID, uWarga.Email, uWarga.Role, nil, 1)
	tokenPemdes, _ := utils.GenerateToken(uPemdes.ID, uPemdes.Email, uPemdes.Role, uPemdes.WilayahID, 1)
	tokenPU, _ := utils.GenerateToken(uPU.ID, uPU.Email, uPU.Role, nil, 1)

	// Matrix A: Superadmin User Management Routes must reject non-superadmin with 403
	checkForbidden := func(roleToken, url string) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+roleToken)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 for token on %s, got: %d", url, w.Code)
		}
	}

	checkForbidden(tokenWarga, "/api/superadmin/users")
	checkForbidden(tokenPemdes, "/api/superadmin/users")
	checkForbidden(tokenPU, "/api/superadmin/users")

	checkForbidden(tokenWarga, "/api/superadmin/wilayah")
	checkForbidden(tokenPemdes, "/api/superadmin/wilayah")
	checkForbidden(tokenPU, "/api/superadmin/wilayah")

	// Matrix B: Admin Dashboard & Admin Laporan must reject Warga with 403
	checkForbidden(tokenWarga, "/api/admin/dashboard")
	checkForbidden(tokenWarga, "/api/admin/laporan")
}
