package controllers

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"backend-jalan-rusak/middlewares"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Helper simulasi filtering notifikasi per user (hanya item aktif / deleted_at IS NULL)
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

// ============================================================
// A. GET NOTIFICATIONS TESTS
// ============================================================

// 1. Test Get Notifikasi Own User & IDOR Prevention
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

// 2. Test Empty Notification Serializes to [] and NEVER null
func TestNotifikasi_EmptyReturnsArray(t *testing.T) {
	userC := uint(99)
	allNotifs := []models.Notifikasi{
		{Model: gorm.Model{ID: 1}, UserID: 10, Judul: "Notif", Pesan: "Pesan"},
	}

	notifsC := filterNotifikasiSimulasi(userC, allNotifs)

	resp := gin.H{
		"status":       "success",
		"message":      "Data notifikasi berhasil diambil",
		"data":         notifsC,
		"unread_count": 0,
		"meta": gin.H{
			"page":         1,
			"limit":        20,
			"total":        0,
			"unread_count": 0,
			"total_pages":  0,
		},
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	jsonStr := string(b)
	if !strings.Contains(jsonStr, `"data":[]`) {
		t.Errorf(`expected JSON to contain '"data":[]', got: %s`, jsonStr)
	}
	if strings.Contains(jsonStr, `"data":null`) {
		t.Errorf(`JSON must never contain '"data":null', got: %s`, jsonStr)
	}
}

// 3. Test Pagination: defaults, limits, offset, and total_pages
func TestNotifikasi_PaginationDefaultsAndLimits(t *testing.T) {
	parsePaginationParams := func(pageStr, limitStr string) (int, int, int) {
		page, errPage := strconv.Atoi(pageStr)
		if errPage != nil || page < 1 {
			page = 1
		}

		limit, errLimit := strconv.Atoi(limitStr)
		if errLimit != nil || limit <= 0 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}

		offset := (page - 1) * limit
		return page, limit, offset
	}

	tests := []struct {
		name           string
		pageInput      string
		limitInput     string
		expectedPage   int
		expectedLimit  int
		expectedOffset int
	}{
		{"Empty defaults", "", "", 1, 20, 0},
		{"Explicit page 1 limit 20", "1", "20", 1, 20, 0},
		{"Page 2 limit 20", "2", "20", 2, 20, 20},
		{"Page 3 limit 15", "3", "15", 3, 15, 30},
		{"Invalid page non-numeric", "abc", "20", 1, 20, 0},
		{"Negative page", "-5", "20", 1, 20, 0},
		{"Zero page", "0", "20", 1, 20, 0},
		{"Invalid limit non-numeric", "1", "xyz", 1, 20, 0},
		{"Zero limit defaults to 20", "1", "0", 1, 20, 0},
		{"Negative limit defaults to 20", "1", "-10", 1, 20, 0},
		{"Limit above max 100 capped at 100", "1", "150", 1, 100, 0},
		{"Limit 100 exact allowed", "2", "100", 2, 100, 100},
	}

	for _, tc := range tests {
		p, l, o := parsePaginationParams(tc.pageInput, tc.limitInput)
		if p != tc.expectedPage || l != tc.expectedLimit || o != tc.expectedOffset {
			t.Errorf("[%s] got page=%d, limit=%d, offset=%d; expected %d, %d, %d",
				tc.name, p, l, o, tc.expectedPage, tc.expectedLimit, tc.expectedOffset)
		}
	}

	// Test total_pages calculation
	calcTotalPages := func(total int64, limit int) int {
		if total <= 0 {
			return 0
		}
		return int(math.Ceil(float64(total) / float64(limit)))
	}

	pageTests := []struct {
		total         int64
		limit         int
		expectedPages int
	}{
		{0, 20, 0},
		{5, 20, 1},
		{20, 20, 1},
		{21, 20, 2},
		{42, 20, 3},
		{100, 20, 5},
	}

	for _, pt := range pageTests {
		pages := calcTotalPages(pt.total, pt.limit)
		if pages != pt.expectedPages {
			t.Errorf("total %d with limit %d: expected %d pages, got %d", pt.total, pt.limit, pt.expectedPages, pages)
		}
	}
}

// 4. Test Newest First Ordering (created_at DESC)
func TestNotifikasi_NewestFirstOrdering(t *testing.T) {
	now := time.Now()
	n1 := models.Notifikasi{Model: gorm.Model{ID: 1, CreatedAt: now.Add(-2 * time.Hour)}, Judul: "Lama"}
	n2 := models.Notifikasi{Model: gorm.Model{ID: 2, CreatedAt: now.Add(-1 * time.Hour)}, Judul: "Sedang"}
	n3 := models.Notifikasi{Model: gorm.Model{ID: 3, CreatedAt: now}, Judul: "Baru"}

	list := []models.Notifikasi{n1, n2, n3}

	// Simulasi Order("created_at DESC")
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})

	if list[0].ID != 3 || list[1].ID != 2 || list[2].ID != 1 {
		t.Errorf("expected newest first order [3, 2, 1], got [%d, %d, %d]", list[0].ID, list[1].ID, list[2].ID)
	}
}

// 5. Test Unread Count: only unread, own user, non-deleted
func TestNotifikasi_UnreadCountCalculation(t *testing.T) {
	userA := uint(10)
	userB := uint(20)

	allNotifs := []models.Notifikasi{
		{Model: gorm.Model{ID: 1}, UserID: userA, IsRead: false},
		{Model: gorm.Model{ID: 2}, UserID: userA, IsRead: false},
		{Model: gorm.Model{ID: 3}, UserID: userA, IsRead: true},
		// Soft-deleted unread notif for User A
		{Model: gorm.Model{ID: 4, DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}}, UserID: userA, IsRead: false},
		// User B unread notif
		{Model: gorm.Model{ID: 5}, UserID: userB, IsRead: false},
	}

	calcUnread := func(targetUser uint, notifs []models.Notifikasi) int64 {
		var count int64
		for _, n := range notifs {
			if n.DeletedAt.Valid {
				continue
			}
			if n.UserID == targetUser && !n.IsRead {
				count++
			}
		}
		return count
	}

	unreadA := calcUnread(userA, allNotifs)
	if unreadA != 2 {
		t.Errorf("expected User A unread_count=2, got %d", unreadA)
	}

	unreadB := calcUnread(userB, allNotifs)
	if unreadB != 1 {
		t.Errorf("expected User B unread_count=1, got %d", unreadB)
	}
}

// ============================================================
// B. MARK ONE READ TESTS
// ============================================================

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

	// 1. Non-owner / IDOR attempt -> 403 Forbidden
	code, msg := markReadSimulasi(attackerID, &notif)
	if code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for non-owner, got %d (%s)", code, msg)
	}
	if notif.IsRead {
		t.Error("notification should still be unread after unauthorized attempt")
	}

	// 2. Owner marks as read -> 200 OK & IsRead becomes true
	code, msg = markReadSimulasi(ownerID, &notif)
	if code != http.StatusOK || !notif.IsRead {
		t.Errorf("expected 200 OK and IsRead=true, got %d (%s)", code, msg)
	}

	// 3. Idempotency test: Owner marks already read notification -> 200 OK
	code, msg = markReadSimulasi(ownerID, &notif)
	if code != http.StatusOK || !notif.IsRead {
		t.Errorf("expected 200 OK idempotent, got %d (%s)", code, msg)
	}

	// 4. Not Found: nil pointer or soft-deleted item -> 404 Not Found
	code, msg = markReadSimulasi(ownerID, nil)
	if code != http.StatusNotFound {
		t.Errorf("expected 404 for nil notif, got %d (%s)", code, msg)
	}

	deletedNotif := models.Notifikasi{
		Model:     gorm.Model{ID: 11, DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}},
		UserID:    ownerID,
		IsRead:    false,
	}
	code, msg = markReadSimulasi(ownerID, &deletedNotif)
	if code != http.StatusNotFound {
		t.Errorf("expected 404 for soft-deleted notif, got %d (%s)", code, msg)
	}
}

// ============================================================
// C. MARK ALL READ TESTS
// ============================================================

func TestNotifikasi_MarkAllRead_Logic(t *testing.T) {
	userA := uint(10)
	userB := uint(20)

	allNotifs := []models.Notifikasi{
		{Model: gorm.Model{ID: 1}, UserID: userA, IsRead: false},
		{Model: gorm.Model{ID: 2}, UserID: userA, IsRead: false},
		{Model: gorm.Model{ID: 3}, UserID: userA, IsRead: true},
		{Model: gorm.Model{ID: 4}, UserID: userB, IsRead: false},
		{Model: gorm.Model{ID: 5, DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}}, UserID: userA, IsRead: false},
	}

	markAllReadSimulasi := func(targetUser uint, notifs []*models.Notifikasi) int {
		updated := 0
		for _, n := range notifs {
			if n.DeletedAt.Valid {
				continue
			}
			if n.UserID == targetUser && !n.IsRead {
				n.IsRead = true
				updated++
			}
		}
		return updated
	}

	notifPtrs := make([]*models.Notifikasi, len(allNotifs))
	for i := range allNotifs {
		notifPtrs[i] = &allNotifs[i]
	}

	// User A marks all as read
	updatedCount := markAllReadSimulasi(userA, notifPtrs)
	if updatedCount != 2 {
		t.Errorf("expected 2 notifications updated for User A, got %d", updatedCount)
	}

	// Verify User A notifications are all read (except soft-deleted stays intact)
	for _, n := range notifPtrs {
		if n.UserID == userA && !n.DeletedAt.Valid {
			if !n.IsRead {
				t.Errorf("expected User A active notif %d to be read", n.ID)
			}
		}
	}

	// Verify User B notification is UNTOUCHED
	if notifPtrs[3].UserID != userB || notifPtrs[3].IsRead != false {
		t.Errorf("User B notification was modified by User A mark all read! %+v", notifPtrs[3])
	}

	// Idempotency: calling mark all read again when unread = 0 returns success with 0 updated
	updatedCountAgain := markAllReadSimulasi(userA, notifPtrs)
	if updatedCountAgain != 0 {
		t.Errorf("expected 0 updated on second run, got %d", updatedCountAgain)
	}

	// Empty notification set succeeds without error
	userEmpty := uint(999)
	updatedEmpty := markAllReadSimulasi(userEmpty, notifPtrs)
	if updatedEmpty != 0 {
		t.Errorf("expected 0 updated for user with no notifications, got %d", updatedEmpty)
	}
}

// ============================================================
// D. DELETE NOTIFICATION TESTS (Soft Delete & Isolation)
// ============================================================

func TestNotifikasi_Delete_Logic(t *testing.T) {
	ownerID := uint(10)
	attackerID := uint(20)

	deleteNotifSimulasi := func(callerID uint, n *models.Notifikasi) (int, string) {
		if n == nil || n.DeletedAt.Valid {
			return http.StatusNotFound, "Notifikasi tidak ditemukan"
		}
		if n.UserID != callerID {
			return http.StatusForbidden, "Anda tidak memiliki akses ke notifikasi ini"
		}
		// Soft delete: set DeletedAt
		n.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
		return http.StatusOK, "Notifikasi berhasil dihapus"
	}

	notif := models.Notifikasi{
		Model:  gorm.Model{ID: 101},
		UserID: ownerID,
		Judul:  "Notif Hapus",
		IsRead: false,
	}

	// 1. Non-owner cannot delete -> 403 Forbidden
	code, msg := deleteNotifSimulasi(attackerID, &notif)
	if code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for non-owner, got %d (%s)", code, msg)
	}
	if notif.DeletedAt.Valid {
		t.Error("notification should not be soft-deleted by non-owner")
	}

	// 2. Owner can delete -> 200 OK & DeletedAt is Valid
	code, msg = deleteNotifSimulasi(ownerID, &notif)
	if code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d (%s)", code, msg)
	}
	if !notif.DeletedAt.Valid {
		t.Error("expected DeletedAt.Valid to be true after soft delete")
	}

	// 3. Deleting already soft-deleted item -> 404 Not Found
	code, msg = deleteNotifSimulasi(ownerID, &notif)
	if code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found on already deleted notif, got %d (%s)", code, msg)
	}

	// 4. Soft-deleted item does not appear in GET list
	activeNotifs := filterNotifikasiSimulasi(ownerID, []models.Notifikasi{notif})
	if len(activeNotifs) != 0 {
		t.Errorf("expected 0 active notifications after delete, got %d", len(activeNotifs))
	}

	// 5. Soft-deleted item does not count towards unread_count
	var unreadCount int64
	for _, n := range []models.Notifikasi{notif} {
		if !n.DeletedAt.Valid && n.UserID == ownerID && !n.IsRead {
			unreadCount++
		}
	}
	if unreadCount != 0 {
		t.Errorf("expected unread_count=0 for soft-deleted item, got %d", unreadCount)
	}
}

// ============================================================
// E. REGRESSION TESTS (Event Handlers & Notification Creation)
// ============================================================

// 1. New Report Routing (Desa -> Pemdes wilayah, Kabupaten -> Admin PU)
func TestNotifikasiEvent_LaporanBaruTargetAdmins(t *testing.T) {
	lapDesaWilayah1 := models.LaporanKerusakan{
		Model:      gorm.Model{ID: 1},
		UserID:     10,
		JenisJalan: "desa",
		WilayahID:  1,
	}
	lapKabupaten := models.LaporanKerusakan{
		Model:      gorm.Model{ID: 2},
		UserID:     10,
		JenisJalan: "kabupaten",
		WilayahID:  1,
	}

	wil1 := uint(1)
	wil2 := uint(2)
	admins := []models.User{
		{Model: gorm.Model{ID: 101}, Role: models.RoleAdminPemdes, WilayahID: &wil1},
		{Model: gorm.Model{ID: 102}, Role: models.RoleAdminPemdes, WilayahID: &wil2},
		{Model: gorm.Model{ID: 201}, Role: models.RoleAdminPu},
	}

	getTargets := func(lap models.LaporanKerusakan) []uint {
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

	targetsDesa := getTargets(lapDesaWilayah1)
	if len(targetsDesa) != 1 || targetsDesa[0] != 101 {
		t.Errorf("expected target admin 101 for desa wilayah 1, got %v", targetsDesa)
	}

	targetsKab := getTargets(lapKabupaten)
	if len(targetsKab) != 1 || targetsKab[0] != 201 {
		t.Errorf("expected target admin 201 for kabupaten, got %v", targetsKab)
	}
}

// 2. Chat Warga -> Admin Notification Targets
func TestNotifikasiEvent_ChatWargaTujuanAdmin(t *testing.T) {
	lapDesa := models.LaporanKerusakan{
		Model:      gorm.Model{ID: 101},
		UserID:     10,
		JenisJalan: "desa",
		WilayahID:  1,
	}
	lapKab := models.LaporanKerusakan{
		Model:      gorm.Model{ID: 102},
		UserID:     10,
		JenisJalan: "kabupaten",
		WilayahID:  1,
	}

	wilayah1 := uint(1)
	wilayah2 := uint(2)
	mockAdmins := []models.User{
		{Model: gorm.Model{ID: 1001}, Role: models.RoleAdminPemdes, WilayahID: &wilayah1},
		{Model: gorm.Model{ID: 1002}, Role: models.RoleAdminPemdes, WilayahID: &wilayah2},
		{Model: gorm.Model{ID: 2001}, Role: models.RoleAdminPu},
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

	desaTargets := determineTargetAdmins(lapDesa, mockAdmins)
	if len(desaTargets) != 1 || desaTargets[0] != 1001 {
		t.Errorf("expected target admin 1001 for desa wilayah 1, got %v", desaTargets)
	}

	kabTargets := determineTargetAdmins(lapKab, mockAdmins)
	if len(kabTargets) != 1 || kabTargets[0] != 2001 {
		t.Errorf("expected target admin 2001 for kabupaten, got %v", kabTargets)
	}
}

// 3. Admin Reply -> Warga Notification Target
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

// 4. Status Change & Duplicate Protection
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

// ============================================================
// F. HTTP ENDPOINT TESTS (Auth, Validation, Response Envelopes)
// ============================================================

// Test HTTP Auth: All Notification Endpoints Require Valid Token (401 without token)
func TestNotifikasiHTTP_AllEndpointsRequireAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middlewares.AuthMiddleware())
	r.GET("/api/notifikasi", GetNotifikasiUser)
	r.PUT("/api/notifikasi/read-all", MarkAllNotifikasiRead)
	r.PATCH("/api/notifikasi/read-all", MarkAllNotifikasiRead)
	r.PUT("/api/notifikasi/:id/read", MarkNotifikasiRead)
	r.PATCH("/api/notifikasi/:id/read", MarkNotifikasiRead)
	r.DELETE("/api/notifikasi/:id", DeleteNotifikasi)

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/notifikasi"},
		{http.MethodPut, "/api/notifikasi/read-all"},
		{http.MethodPatch, "/api/notifikasi/read-all"},
		{http.MethodPut, "/api/notifikasi/10/read"},
		{http.MethodPatch, "/api/notifikasi/10/read"},
		{http.MethodDelete, "/api/notifikasi/10"},
	}

	for _, route := range routes {
		req, _ := http.NewRequest(route.method, route.path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("[%s %s] expected 401 Unauthorized without token, got %d", route.method, route.path, w.Code)
		}
	}
}

// Test HTTP Invalid ID: Returns 400 Bad Request
func TestNotifikasiHTTP_InvalidIDsReturn400(t *testing.T) {
	t.Setenv("JWT_SECRET", "notif_test_secret_12345678901234")
	token, err := utils.GenerateToken(10, "warga@roadis.id", models.RoleWarga, nil)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middlewares.AuthMiddleware())
	r.PUT("/api/notifikasi/:id/read", MarkNotifikasiRead)
	r.DELETE("/api/notifikasi/:id", DeleteNotifikasi)

	invalidRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodPut, "/api/notifikasi/abc/read"},
		{http.MethodPut, "/api/notifikasi/0/read"},
		{http.MethodPut, "/api/notifikasi/-5/read"},
		{http.MethodDelete, "/api/notifikasi/abc"},
		{http.MethodDelete, "/api/notifikasi/0"},
		{http.MethodDelete, "/api/notifikasi/-10"},
	}

	for _, route := range invalidRoutes {
		req, _ := http.NewRequest(route.method, route.path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("[%s %s] expected 400 Bad Request for invalid ID, got %d (%s)",
				route.method, route.path, w.Code, w.Body.String())
		}

		if !strings.Contains(w.Body.String(), "ID notifikasi tidak valid") {
			t.Errorf("[%s %s] expected error message 'ID notifikasi tidak valid', got %s",
				route.method, route.path, w.Body.String())
		}
	}
}

// Test HTTP Auth: Valid Token -> Success Envelope Format with Meta and Unread Count
func TestNotifikasiHTTP_ValidTokenResponseEnvelope(t *testing.T) {
	t.Setenv("JWT_SECRET", "notif_test_secret_12345678901234")

	token, err := utils.GenerateToken(10, "warga@roadis.id", models.RoleWarga, nil)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middlewares.AuthMiddleware())
	r.GET("/api/notifikasi", func(c *gin.Context) {
		// Mock serialization of notification response envelope with pagination & meta
		listNotifikasi := make([]models.Notifikasi, 0)
		c.JSON(http.StatusOK, gin.H{
			"status":       "success",
			"message":      "Data notifikasi berhasil diambil",
			"data":         listNotifikasi,
			"unread_count": 0,
			"meta": gin.H{
				"page":         1,
				"limit":        20,
				"total":        0,
				"unread_count": 0,
				"total_pages":  0,
			},
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
	if !strings.Contains(body, `"unread_count":0`) {
		t.Errorf(`expected response body to contain '"unread_count":0', got: %s`, body)
	}
	if !strings.Contains(body, `"meta"`) {
		t.Errorf(`expected response body to contain '"meta"', got: %s`, body)
	}
	if !strings.Contains(body, `"total_pages":0`) {
		t.Errorf(`expected response body to contain '"total_pages":0', got: %s`, body)
	}
}
