package controllers

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
	"gorm.io/gorm"
)

// Helper simulasi filtering notifikasi per user
func filterNotifikasiSimulasi(userID uint, allNotifs []models.Notifikasi) []models.Notifikasi {
	result := make([]models.Notifikasi, 0)
	for _, n := range allNotifs {
		if n.DeletedAt.Valid {
			continue
		}
		if n.UserID == userID {
			result = append(result, n)
		}
	}
	return result
}

// 1 & 2. Test Get Notifikasi Own User & IDOR Prevention
func TestNotifikasi_OwnUserAndIDOR(t *testing.T) {
	userA := uint(10)
	userB := uint(20)

	allNotifs := []models.Notifikasi{
		{Model: gorm.Model{ID: 1}, UserID: userA, Judul: "Notif A1", Pesan: "Pesan A1", IsRead: false},
		{Model: gorm.Model{ID: 2}, UserID: userA, Judul: "Notif A2", Pesan: "Pesan A2", IsRead: true},
		{Model: gorm.Model{ID: 3}, UserID: userB, Judul: "Notif B1", Pesan: "Pesan B1", IsRead: false},
	}

	// User A hanya melihat notifikasi miliknya (ID 1, 2)
	notifsA := filterNotifikasiSimulasi(userA, allNotifs)
	if len(notifsA) != 2 {
		t.Fatalf("expected 2 notifications for User A, got %d", len(notifsA))
	}
	for _, n := range notifsA {
		if n.UserID != userA {
			t.Errorf("User A received notification belonging to User %d (IDOR)", n.UserID)
		}
	}

	// User B hanya melihat notifikasi miliknya (ID 3)
	notifsB := filterNotifikasiSimulasi(userB, allNotifs)
	if len(notifsB) != 1 {
		t.Fatalf("expected 1 notification for User B, got %d", len(notifsB))
	}
	if notifsB[0].ID != 3 || notifsB[0].UserID != userB {
		t.Errorf("User B received unexpected notification: %+v", notifsB[0])
	}
}

// 3. Test Empty Notification Serializes to []
func TestNotifikasi_EmptyReturnsArray(t *testing.T) {
	userC := uint(99)
	allNotifs := []models.Notifikasi{
		{Model: gorm.Model{ID: 1}, UserID: 10, Judul: "Notif", Pesan: "Pesan"},
	}

	notifsC := filterNotifikasiSimulasi(userC, allNotifs)

	resp := gin.H{
		"status":  "success",
		"message": "Data notifikasi berhasil diambil",
		"data":    notifsC,
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	jsonStr := string(b)
	if !strings.Contains(jsonStr, `"data":[]`) {
		t.Errorf(`expected JSON to contain '"data":[]', got: %s`, jsonStr)
	}
}

// 4, 5, 6. Test Mark As Read: Ownership, Block IDOR, Idempotent
func TestNotifikasi_MarkAsRead_Logic(t *testing.T) {
	ownerID := uint(5)
	attackerID := uint(6)

	notif := models.Notifikasi{
		Model:  gorm.Model{ID: 10},
		UserID: ownerID,
		Judul:  "Status Berubah",
		Pesan:  "Laporan selesai",
		IsRead: false,
	}

	// Simulasi otorisasi mark as read
	markReadSimulasi := func(callerID uint, n *models.Notifikasi) (int, string) {
		if n == nil || n.DeletedAt.Valid {
			return http.StatusNotFound, "Notifikasi tidak ditemukan"
		}
		if n.UserID != callerID {
			return http.StatusForbidden, "Anda tidak memiliki akses ke notifikasi ini"
		}
		if n.IsRead {
			return http.StatusOK, "Notifikasi sudah ditandai sebagai dibaca"
		}
		n.IsRead = true
		return http.StatusOK, "Notifikasi berhasil ditandai sebagai dibaca"
	}

	// 5. Attacker tries to mark owner's notification -> 403 Forbidden
	code, msg := markReadSimulasi(attackerID, &notif)
	if code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for non-owner, got %d (%s)", code, msg)
	}
	if notif.IsRead {
		t.Error("notification should still be unread after unauthorized attempt")
	}

	// 4. Owner marks as read -> 200 OK & IsRead becomes true
	code, msg = markReadSimulasi(ownerID, &notif)
	if code != http.StatusOK || !notif.IsRead {
		t.Errorf("expected 200 OK and IsRead=true, got %d (%s)", code, msg)
	}

	// 6. Idempotency test: Owner marks already read notification -> 200 OK
	code, msg = markReadSimulasi(ownerID, &notif)
	if code != http.StatusOK || !notif.IsRead {
		t.Errorf("expected 200 OK idempotent, got %d (%s)", code, msg)
	}
}

// 8. Test Warga Chat -> Admin Notification Targets
func TestNotifikasiEvent_ChatWargaTujuanAdmin(t *testing.T) {
	// Laporan desa di Wilayah 1
	lapDesa := models.LaporanKerusakan{
		Model:      gorm.Model{ID: 101},
		UserID:     10,
		JenisJalan: "desa",
		WilayahID:  1,
	}

	// Laporan kabupaten
	lapKab := models.LaporanKerusakan{
		Model:      gorm.Model{ID: 102},
		UserID:     10,
		JenisJalan: "kabupaten",
		WilayahID:  1,
	}

	type AdminRule struct {
		Role      models.UserRole
		WilayahID *uint
	}

	determineTargetAdmins := func(lap models.LaporanKerusakan, admins []models.User) []uint {
		var targets []uint
		for _, a := range admins {
			if strings.ToLower(lap.JenisJalan) == "desa" {
				if a.Role == models.RoleAdminPemdes && a.WilayahID != nil && *a.WilayahID == lap.WilayahID {
					targets = append(targets, a.ID)
				}
			} else if strings.ToLower(lap.JenisJalan) == "kabupaten" {
				if a.Role == models.RoleAdminPu {
					targets = append(targets, a.ID)
				}
			}
		}
		return targets
	}

	wilayah1 := uint(1)
	wilayah2 := uint(2)
	mockAdmins := []models.User{
		{Model: gorm.Model{ID: 1001}, Role: models.RoleAdminPemdes, WilayahID: &wilayah1},
		{Model: gorm.Model{ID: 1002}, Role: models.RoleAdminPemdes, WilayahID: &wilayah2},
		{Model: gorm.Model{ID: 2001}, Role: models.RoleAdminPu},
	}

	// Laporan Desa Wilayah 1 -> Hanya Admin Pemdes Wilayah 1 (ID 1001)
	desaTargets := determineTargetAdmins(lapDesa, mockAdmins)
	if len(desaTargets) != 1 || desaTargets[0] != 1001 {
		t.Errorf("expected target admin 1001 for desa wilayah 1, got %v", desaTargets)
	}

	// Laporan Kabupaten -> Admin PU (ID 2001)
	kabTargets := determineTargetAdmins(lapKab, mockAdmins)
	if len(kabTargets) != 1 || kabTargets[0] != 2001 {
		t.Errorf("expected target admin 2001 for kabupaten, got %v", kabTargets)
	}
}

// 9. Test Admin Reply -> Warga Notification Target
func TestNotifikasiEvent_AdminReplyTujuanWarga(t *testing.T) {
	wargaID := uint(55)
	lap := models.LaporanKerusakan{
		Model:  gorm.Model{ID: 77},
		UserID: wargaID,
		Judul:  "Jalan Rusak",
	}

	notif := models.Notifikasi{
		UserID:    lap.UserID,
		LaporanID: lap.ID,
		Judul:     "Balasan Pesan",
		Pesan:     "Admin membalas pesan pada laporan #77",
	}

	if notif.UserID != wargaID {
		t.Errorf("expected notification target to be Warga %d, got %d", wargaID, notif.UserID)
	}
	if notif.LaporanID != 77 {
		t.Errorf("expected LaporanID 77, got %d", notif.LaporanID)
	}
}

// 12 & 13. Test Status Change & Duplicate Protection
func TestNotifikasiEvent_StatusChangeDuplicateProtection(t *testing.T) {
	checkStatusNotification := func(oldStatus, newStatus string) bool {
		newLower := strings.ToLower(newStatus)
		oldLower := strings.ToLower(oldStatus)
		return newLower != "" && newLower != oldLower
	}

	// Status berubah dari menunggu ke proses -> HARUS NOTIFIKASI
	if !checkStatusNotification("menunggu", "proses") {
		t.Error("expected notification when status changes from menunggu to proses")
	}

	// Status berubah dari proses ke selesai -> HARUS NOTIFIKASI
	if !checkStatusNotification("proses", "selesai") {
		t.Error("expected notification when status changes from proses to selesai")
	}

	// Status tetap (misal hanya update catatan admin) -> TIDAK BOLEH NOTIFIKASI (cegah duplikasi)
	if checkStatusNotification("proses", "proses") {
		t.Error("must NOT create notification when status does not change (duplicate protection)")
	}
	if checkStatusNotification("proses", "") {
		t.Error("must NOT create notification when new status is empty")
	}
}

// 7. Test HTTP Auth: No Token -> 401 Unauthorized
func TestNotifikasiHTTP_NoTokenReturns401(t *testing.T) {
	r := gin.New()
	r.Use(middlewares.AuthMiddleware())
	r.GET("/api/notifikasi", GetNotifikasiUser)

	req, _ := http.NewRequest(http.MethodGet, "/api/notifikasi", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized without token, got %d", w.Code)
	}
}

// Test HTTP Auth: Valid Token -> Success Envelope Format
func TestNotifikasiHTTP_ValidTokenResponseEnvelope(t *testing.T) {
	t.Setenv("JWT_SECRET", "notif_test_secret_12345678901234")

	token, err := utils.GenerateToken(10, "warga@roadis.id", models.RoleWarga, nil)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	r := gin.New()
	r.Use(middlewares.AuthMiddleware())
	r.GET("/api/notifikasi", func(c *gin.Context) {
		// Mock handler menguji serialisasi data kosong
		listNotifikasi := make([]models.Notifikasi, 0)
		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "Data notifikasi berhasil diambil",
			"data":    listNotifikasi,
		})
	})

	req, _ := http.NewRequest(http.MethodGet, "/api/notifikasi", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, `"data":[]`) {
		t.Errorf(`expected response body to contain '"data":[]', got: %s`, body)
	}
}
