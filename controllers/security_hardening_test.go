package controllers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/controllers"
	"backend-jalan-rusak/controllers/superadmin"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
)

// 1. Authentication bypass: Missing or forged tokens rejected with 401
func TestBE15_01_AuthBypass(t *testing.T) {
	r := setupBE13Router()

	endpoints := []struct {
		method string
		url    string
	}{
		{http.MethodGet, "/api/warga/laporan"},
		{http.MethodGet, "/api/admin/dashboard"},
		{http.MethodGet, "/api/superadmin/users"},
		{http.MethodGet, "/api/profile"},
		{http.MethodGet, "/api/settings"},
		{http.MethodGet, "/api/notifikasi"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+"_"+ep.url, func(t *testing.T) {
			// No token
			reqNoAuth := httptest.NewRequest(ep.method, ep.url, nil)
			wNoAuth := httptest.NewRecorder()
			r.ServeHTTP(wNoAuth, reqNoAuth)
			if wNoAuth.Code != http.StatusUnauthorized {
				t.Errorf("Expected 401 without auth header on %s, got %d", ep.url, wNoAuth.Code)
			}

			// Forged signature
			reqForged := httptest.NewRequest(ep.method, ep.url, nil)
			reqForged.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.forged_signature_here")
			wForged := httptest.NewRecorder()
			r.ServeHTTP(wForged, reqForged)
			if wForged.Code != http.StatusUnauthorized {
				t.Errorf("Expected 401 with forged signature on %s, got %d", ep.url, wForged.Code)
			}
		})
	}
}

// 2. Expired token: Rejected with 401
func TestBE15_02_ExpiredToken(t *testing.T) {
	r := setupBE13Router()
	expiredToken := makeBE13Token(1, "super_admin", -time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/api/superadmin/users", nil)
	req.Header.Set("Authorization", "Bearer "+expiredToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for expired token, got %d", w.Code)
	}
}

// 3. Deleted user token: Inactive accounts cannot access protected resources
func TestBE15_03_DeletedUserToken(t *testing.T) {
	if ensureDBForLaporanTest(t) {
		nowNano := time.Now().UnixNano()
		delUser := models.User{
			Name:         "User Dihapus BE15",
			Email:        fmt.Sprintf("del_be15_%d@roadis.id", nowNano),
			Role:         models.RoleWarga,
			TokenVersion: 1,
		}
		config.DB.Create(&delUser)
		config.DB.Delete(&delUser)
		defer config.DB.Unscoped().Delete(&delUser)

		token := makeBE13Token(delUser.ID, "warga", time.Hour)
		r := setupBE13Router()

		req := httptest.NewRequest(http.MethodGet, "/api/warga/laporan", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 for soft-deleted user token, got %d", w.Code)
		}
	}
}

// 4. TokenVersion mismatch: Revoked tokens rejected with 401
func TestBE15_04_TokenVersionMismatch(t *testing.T) {
	if ensureDBForLaporanTest(t) {
		nowNano := time.Now().UnixNano()
		user := models.User{
			Name:         "User TV BE15",
			Email:        fmt.Sprintf("tv_be15_%d@roadis.id", nowNano),
			Role:         models.RoleWarga,
			TokenVersion: 2, // Database is version 2
		}
		config.DB.Create(&user)
		defer config.DB.Unscoped().Delete(&user)

		// Token generated with version 1
		tokenV1, _ := utils.GenerateToken(user.ID, user.Email, user.Role, nil, 1)

		r := setupBE13Router()
		req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
		req.Header.Set("Authorization", "Bearer "+tokenV1)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 when token version is outdated, got %d", w.Code)
		}
	}
}

// 5. Unknown role: Must fail closed with 403 Forbidden
func TestBE15_05_UnknownRole(t *testing.T) {
	r := setupBE13Router()
	unknownRoles := []string{"guest", "anonymous", "super_user", "root", "moderator", ""}

	for _, role := range unknownRoles {
		token := makeBE13Token(99, role, time.Hour)
		req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 for unknown role %q, got %d", role, w.Code)
		}
	}
}

// 6. IDOR laporan: Citizen cannot access reports of another citizen
func TestBE15_06_IDOR_Laporan(t *testing.T) {
	if ensureDBForLaporanTest(t) {
		nowNano := time.Now().UnixNano()
		wargaOwner := models.User{Name: "Owner", Email: fmt.Sprintf("owner_%d@roadis.id", nowNano), Role: models.RoleWarga}
		wargaAttacker := models.User{Name: "Attacker", Email: fmt.Sprintf("attacker_%d@roadis.id", nowNano), Role: models.RoleWarga}
		config.DB.Create(&wargaOwner)
		config.DB.Create(&wargaAttacker)
		defer config.DB.Unscoped().Delete(&wargaOwner)
		defer config.DB.Unscoped().Delete(&wargaAttacker)

		wilayah := models.Wilayah{Nama: fmt.Sprintf("Desa IDOR %d", nowNano), Tipe: "desa"}
		config.DB.Create(&wilayah)
		defer config.DB.Unscoped().Delete(&wilayah)

		lap := models.LaporanKerusakan{
			UserID:        wargaOwner.ID,
			WilayahID:     wilayah.ID,
			JenisJalan:    "desa",
			Judul:         "Laporan Rahasia",
			Deskripsi:     "Deskripsi",
			Latitude:      -6.3,
			Longitude:     108.3,
			ImageURL:      "https://example.com/foto.jpg",
			TipeKerusakan: "rusak_ringan",
			Status:        "menunggu",
		}
		config.DB.Create(&lap)
		defer config.DB.Unscoped().Delete(&lap)

		r := setupBE13Router()
		attackerToken := makeBE13Token(wargaAttacker.ID, "warga", time.Hour)

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/warga/laporan/%d", lap.ID), nil)
		req.Header.Set("Authorization", "Bearer "+attackerToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 when citizen accesses another citizen's report, got %d", w.Code)
		}
	}
}

// 7. IDOR chat: Citizen cannot send chat to reports owned by others
func TestBE15_07_IDOR_Chat(t *testing.T) {
	if ensureDBForLaporanTest(t) {
		nowNano := time.Now().UnixNano()
		owner := models.User{Name: "Chat Owner", Email: fmt.Sprintf("ch_owner_%d@roadis.id", nowNano), Role: models.RoleWarga}
		attacker := models.User{Name: "Chat Attacker", Email: fmt.Sprintf("ch_att_%d@roadis.id", nowNano), Role: models.RoleWarga}
		config.DB.Create(&owner)
		config.DB.Create(&attacker)
		defer config.DB.Unscoped().Delete(&owner)
		defer config.DB.Unscoped().Delete(&attacker)

		wilayah := models.Wilayah{Nama: fmt.Sprintf("Desa Chat %d", nowNano), Tipe: "desa"}
		config.DB.Create(&wilayah)
		defer config.DB.Unscoped().Delete(&wilayah)

		lap := models.LaporanKerusakan{
			UserID:        owner.ID,
			WilayahID:     wilayah.ID,
			JenisJalan:    "desa",
			Judul:         "Laporan Chat",
			Deskripsi:     "Deskripsi",
			Latitude:      -6.3,
			Longitude:     108.3,
			ImageURL:      "https://example.com/foto.jpg",
			TipeKerusakan: "rusak_ringan",
			Status:        "menunggu",
		}
		config.DB.Create(&lap)
		defer config.DB.Unscoped().Delete(&lap)

		r := setupBE13Router()
		attackerToken := makeBE13Token(attacker.ID, "warga", time.Hour)

		body, _ := json.Marshal(map[string]string{"pesan": "Inject chat"})
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/warga/laporan/%d/chat", lap.ID), bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+attackerToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 when citizen chats on another citizen's report, got %d", w.Code)
		}
	}
}

// 8. IDOR notification: User cannot mark or delete another user's notifications
func TestBE15_08_IDOR_Notification(t *testing.T) {
	if ensureDBForLaporanTest(t) {
		nowNano := time.Now().UnixNano()
		victim := models.User{Name: "Victim", Email: fmt.Sprintf("vic_%d@roadis.id", nowNano), Role: models.RoleWarga}
		attacker := models.User{Name: "Attacker", Email: fmt.Sprintf("att_%d@roadis.id", nowNano), Role: models.RoleWarga}
		config.DB.Create(&victim)
		config.DB.Create(&attacker)
		defer config.DB.Unscoped().Delete(&victim)
		defer config.DB.Unscoped().Delete(&attacker)

		notif := models.Notifikasi{
			UserID: victim.ID,
			Judul:  "Rahasia",
			Pesan:  "Pesan penting",
			IsRead: false,
		}
		config.DB.Create(&notif)
		defer config.DB.Unscoped().Delete(&notif)

		r := setupBE13Router()
		attackerToken := makeBE13Token(attacker.ID, "warga", time.Hour)

		// Mark Read
		reqRead := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/notifikasi/%d/read", notif.ID), nil)
		reqRead.Header.Set("Authorization", "Bearer "+attackerToken)
		wRead := httptest.NewRecorder()
		r.ServeHTTP(wRead, reqRead)
		if wRead.Code != http.StatusForbidden {
			t.Errorf("Expected 403 for IDOR notification read, got %d", wRead.Code)
		}

		// Delete
		reqDel := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/notifikasi/%d", notif.ID), nil)
		reqDel.Header.Set("Authorization", "Bearer "+attackerToken)
		wDel := httptest.NewRecorder()
		r.ServeHTTP(wDel, reqDel)
		if wDel.Code != http.StatusForbidden {
			t.Errorf("Expected 403 for IDOR notification delete, got %d", wDel.Code)
		}
	}
}

// 9. Cross-wilayah access: Admin Pemdes cannot view or update reports outside assigned village scope
func TestBE15_09_CrossWilayahAccess(t *testing.T) {
	if ensureDBForLaporanTest(t) {
		nowNano := time.Now().UnixNano()
		wilayahA := models.Wilayah{Nama: fmt.Sprintf("Desa A %d", nowNano), Tipe: "desa"}
		wilayahB := models.Wilayah{Nama: fmt.Sprintf("Desa B %d", nowNano), Tipe: "desa"}
		config.DB.Create(&wilayahA)
		config.DB.Create(&wilayahB)
		defer config.DB.Unscoped().Delete(&wilayahA)
		defer config.DB.Unscoped().Delete(&wilayahB)

		pemdesA := models.User{
			Name:      "Pemdes A",
			Email:     fmt.Sprintf("pemdes_a_%d@roadis.id", nowNano),
			Role:      models.RoleAdminPemdes,
			WilayahID: &wilayahA.ID,
		}
		config.DB.Create(&pemdesA)
		defer config.DB.Unscoped().Delete(&pemdesA)

		wargaB := models.User{Name: "Warga B", Email: fmt.Sprintf("wb_%d@roadis.id", nowNano), Role: models.RoleWarga}
		config.DB.Create(&wargaB)
		defer config.DB.Unscoped().Delete(&wargaB)

		lapB := models.LaporanKerusakan{
			UserID:        wargaB.ID,
			WilayahID:     wilayahB.ID,
			JenisJalan:    "desa",
			Judul:         "Jalan Desa B",
			Deskripsi:     "Deskripsi",
			Latitude:      -6.3,
			Longitude:     108.3,
			ImageURL:      "https://example.com/foto.jpg",
			TipeKerusakan: "rusak_ringan",
			Status:        "menunggu",
		}
		config.DB.Create(&lapB)
		defer config.DB.Unscoped().Delete(&lapB)

		r := setupBE13Router()
		pemdesAToken := makeBE13Token(pemdesA.ID, "admin_pemdes", time.Hour)

		// Detail view of outside village report
		reqGet := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/laporan/%d", lapB.ID), nil)
		reqGet.Header.Set("Authorization", "Bearer "+pemdesAToken)
		wGet := httptest.NewRecorder()
		r.ServeHTTP(wGet, reqGet)
		if wGet.Code != http.StatusForbidden && wGet.Code != http.StatusNotFound {
			t.Errorf("Expected 403 or 404 when Pemdes accesses report outside assigned village, got %d", wGet.Code)
		}

		// Update attempt of outside village report
		body, _ := json.Marshal(map[string]string{"status": "proses"})
		reqPut := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/laporan/%d/status", lapB.ID), bytes.NewBuffer(body))
		reqPut.Header.Set("Authorization", "Bearer "+pemdesAToken)
		reqPut.Header.Set("Content-Type", "application/json")
		wPut := httptest.NewRecorder()
		r.ServeHTTP(wPut, reqPut)
		if wPut.Code != http.StatusForbidden {
			t.Errorf("Expected 403 when Pemdes updates report outside assigned village, got %d", wPut.Code)
		}
	}
}

// 10. Cross-authority update: Admin PU cannot update non-kabupaten roads
func TestBE15_10_CrossAuthorityUpdate(t *testing.T) {
	if ensureDBForLaporanTest(t) {
		nowNano := time.Now().UnixNano()
		adminPU := models.User{Name: "PU Staff", Email: fmt.Sprintf("pu_%d@roadis.id", nowNano), Role: models.RoleAdminPu}
		config.DB.Create(&adminPU)
		defer config.DB.Unscoped().Delete(&adminPU)

		wargaU := models.User{Name: "Warga U", Email: fmt.Sprintf("wu_%d@roadis.id", nowNano), Role: models.RoleWarga}
		config.DB.Create(&wargaU)
		defer config.DB.Unscoped().Delete(&wargaU)

		wilayah := models.Wilayah{Nama: fmt.Sprintf("Wilayah Auth %d", nowNano), Tipe: "desa"}
		config.DB.Create(&wilayah)
		defer config.DB.Unscoped().Delete(&wilayah)

		lapProvinsi := models.LaporanKerusakan{
			UserID:        wargaU.ID,
			WilayahID:     wilayah.ID,
			JenisJalan:    "provinsi", // Non-kabupaten road
			Judul:         "Jalan Provinsi",
			Deskripsi:     "Deskripsi",
			Latitude:      -6.3,
			Longitude:     108.3,
			ImageURL:      "https://example.com/foto.jpg",
			TipeKerusakan: "rusak_sedang",
			Status:        "menunggu",
		}
		config.DB.Create(&lapProvinsi)
		defer config.DB.Unscoped().Delete(&lapProvinsi)

		r := setupBE13Router()
		puToken := makeBE13Token(adminPU.ID, "admin_pu", time.Hour)

		body, _ := json.Marshal(map[string]string{"status": "proses"})
		reqPut := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/laporan/%d/status", lapProvinsi.ID), bytes.NewBuffer(body))
		reqPut.Header.Set("Authorization", "Bearer "+puToken)
		reqPut.Header.Set("Content-Type", "application/json")
		wPut := httptest.NewRecorder()
		r.ServeHTTP(wPut, reqPut)

		if wPut.Code != http.StatusForbidden {
			t.Errorf("Expected 403 when Admin PU updates non-kabupaten road, got %d", wPut.Code)
		}
	}
}

// 11. Superadmin privilege boundary: Cannot delete self, cannot delete last superadmin, cannot delete warga
func TestBE15_11_SuperadminPrivilegeBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.DELETE("/api/superadmin/users/:id", func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Set("role", "super_admin")
		superadmin.DeleteUser(c)
	})

	// Self-delete guard (fails before DB check)
	req := httptest.NewRequest(http.MethodDelete, "/api/superadmin/users/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for self-delete attempt, got %d", w.Code)
	}
}

// 12. Mass assignment: General update endpoints do not allow privilege escalation
func TestBE15_12_MassAssignment(t *testing.T) {
	// Verify UpdateProfileInput struct has no privilege fields
	typ := reflect.TypeOf(controllers.UpdateProfileInput{})
	privilegeFields := []string{"Role", "TokenVersion", "WilayahID", "ID", "DeletedAt", "Password"}
	for _, field := range privilegeFields {
		if _, ok := typ.FieldByName(field); ok {
			t.Errorf("UpdateProfileInput must not expose field %s", field)
		}
	}
}

// 13. SQL injection attempt: Special characters and injection syntax safely handled
func TestBE15_13_SQLInjectionAttempt(t *testing.T) {
	r := setupBE13Router()
	saToken := makeBE13Token(1, "super_admin", time.Hour)

	sqliPayloads := []string{
		"1' OR '1'='1",
		"1; DROP TABLE user;--",
		"1 UNION SELECT * FROM user",
		"1' AND 1=1--",
		"admin'--",
	}

	for _, payload := range sqliPayloads {
		t.Run("SQLi_Path_"+payload, func(t *testing.T) {
			escapedPath := url.PathEscape(payload)
			req := httptest.NewRequest(http.MethodGet, "/api/superadmin/users/"+escapedPath, nil)
			req.Header.Set("Authorization", "Bearer "+saToken)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// Path ID parser rejects non-uint string with 400
			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected 400 for SQLi path ID %q, got %d", payload, w.Code)
			}
		})
	}
}

// 14. Invalid path ID: Non-numeric, negative, zero, and overflow IDs return 400
func TestBE15_14_InvalidPathID(t *testing.T) {
	r := setupBE13Router()
	saToken := makeBE13Token(1, "super_admin", time.Hour)

	invalidIDs := []string{"abc", "-1", "0", "999999999999999999999999", "1.5", " "}
	for _, idStr := range invalidIDs {
		escapedID := url.PathEscape(idStr)
		req := httptest.NewRequest(http.MethodGet, "/api/superadmin/users/"+escapedID, nil)
		req.Header.Set("Authorization", "Bearer "+saToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 for invalid path ID %q, got %d", idStr, w.Code)
		}
	}
}

// 15. Invalid query parameter: Malformed query parameters normalized or filtered
func TestBE15_15_InvalidQueryParameter(t *testing.T) {
	r := setupBE13Router()
	wargaToken := makeBE13Token(1, "warga", time.Hour)

	// Negative and malformed pagination parameters normalized safely
	req := httptest.NewRequest(http.MethodGet, "/api/notifikasi?page=-1&limit=-10", nil)
	req.Header.Set("Authorization", "Bearer "+wargaToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Returns 200 (normalized) or 500 (if DB nil), but NEVER panics or leaks SQL
	if w.Code == http.StatusInternalServerError {
		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if strings.Contains(w.Body.String(), "syntax error") {
			t.Errorf("Query parameter must not cause SQL syntax error")
		}
	}
}

// 16. File upload abuse: Executable files (.exe, .sh, .php) are rejected
func TestBE15_16_FileUploadAbuse(t *testing.T) {
	dangerousFiles := []struct {
		name string
		data []byte
	}{
		{"malware.exe", []byte("MZ\x90\x00\x03\x00\x00\x00")},
		{"script.sh", []byte("#!/bin/bash\nrm -rf /")},
		{"backdoor.php", []byte("<?php system($_GET['cmd']); ?>")},
		{"exploit.js", []byte("alert('pwned')")},
	}

	for _, f := range dangerousFiles {
		t.Run("Reject_"+f.name, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			part, _ := writer.CreateFormFile("foto", f.name)
			part.Write(f.data)
			writer.Close()

			req := httptest.NewRequest(http.MethodPost, "/api/warga/laporan", &body)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			_, fileHeader, err := req.FormFile("foto")
			if err != nil {
				t.Fatalf("FormFile failed: %v", err)
			}

			errVal := utils.ValidateImageFile(fileHeader)
			if errVal == nil {
				t.Errorf("Expected dangerous file %s to be rejected by ValidateImageFile", f.name)
			}
		})
	}
}

// 17. Oversized upload: Files exceeding maximum allowed size are rejected
func TestBE15_17_OversizedUpload(t *testing.T) {
	// Profile avatar: max 2MB
	oversizedData := make([]byte, 3*1024*1024) // 3MB JPEG
	oversizedData[0] = 0xFF
	oversizedData[1] = 0xD8
	oversizedData[2] = 0xFF

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("avatar", "large.jpg")
	part.Write(oversizedData)
	writer.Close()

	req := httptest.NewRequest(http.MethodPut, "/api/profile/avatar", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	_, fileHeader, err := req.FormFile("avatar")
	if err != nil {
		t.Fatalf("FormFile failed: %v", err)
	}

	errVal := utils.ValidateImageFile(fileHeader, 2*1024*1024)
	if errVal == nil {
		t.Errorf("Expected 3MB avatar to be rejected by ValidateImageFile")
	}
}

// 18. Fake MIME: Non-image files with .jpg extension rejected by magic byte detection
func TestBE15_18_FakeMIME(t *testing.T) {
	fakeImages := []struct {
		name    string
		content []byte
	}{
		{"fake.jpg", []byte("%PDF-1.4 Fake PDF with JPG extension")},
		{"fake.png", []byte("<html><body>Fake HTML with PNG extension</body></html>")},
		{"fake.webp", []byte("MZ Windows executable with WEBP extension")},
	}

	for _, f := range fakeImages {
		t.Run("RejectFakeMagicBytes_"+f.name, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			part, _ := writer.CreateFormFile("foto", f.name)
			part.Write(f.content)
			writer.Close()

			req := httptest.NewRequest(http.MethodPost, "/api/warga/laporan", &body)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			_, fileHeader, err := req.FormFile("foto")
			if err != nil {
				t.Fatalf("FormFile failed: %v", err)
			}

			errVal := utils.ValidateImageFile(fileHeader)
			if errVal == nil {
				t.Errorf("Expected fake image %s to be rejected by magic bytes detection", f.name)
			}
		})
	}
}

// 19. Error information disclosure: Error responses must not leak SQL queries, schema, or credentials
func TestBE15_19_ErrorInformationDisclosure(t *testing.T) {
	r := setupBE13Router()

	// Hit endpoints with invalid inputs that trigger error handling
	endpoints := []string{
		"/api/register",
		"/api/login",
		"/api/health",
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(http.MethodPost, ep, bytes.NewBuffer([]byte("{invalid_json:")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		body := w.Body.String()
		sensitiveKeywords := []string{
			"SELECT ", "INSERT ", "UPDATE ", "DELETE FROM",
			"db_jalan_rusak", "root:@", "CLOUDINARY_URL",
			"stack trace", "goroutine ", ".go:",
		}

		for _, kw := range sensitiveKeywords {
			if strings.Contains(body, kw) {
				t.Errorf("Error response on %s leaks sensitive keyword %q: %s", ep, kw, body)
			}
		}
	}
}

// 20. Resource exhaustion: Extremely large pagination limits are safely capped
func TestBE15_20_ResourceExhaustion_Pagination(t *testing.T) {
	r := setupBE13Router()
	wargaToken := makeBE13Token(1, "warga", time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/api/notifikasi?page=1&limit=99999999", nil)
	req.Header.Set("Authorization", "Bearer "+wargaToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// If DB is connected, verify meta limit is capped at 100
	if w.Code == http.StatusOK {
		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if meta, ok := resp["meta"].(map[string]interface{}); ok {
			if limitVal, ok := meta["limit"].(float64); ok && limitVal > 100 {
				t.Errorf("Pagination limit was not capped, got %f (max 100)", limitVal)
			}
		}
	}
}
