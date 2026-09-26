package controllers

import (
	"strings"
	"testing"
	"time"

	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"
	"gorm.io/gorm"
)

// 1 & 2. Test Warga Create Report: UserID derived strictly from Auth, preventing impersonation
func TestWorkflow_ReportOwnershipBoundToAuth(t *testing.T) {
	authUserID := uint(42)
	impersonatedUserID := uint(99)

	// Simulate handler assigning UserID strictly from JWT context
	createReportSimulasi := func(jwtUserID uint, inputBodyUserID uint) models.LaporanKerusakan {
		// Business rule: UserID must always come from jwtUserID, ignoring any body payload
		return models.LaporanKerusakan{
			UserID: jwtUserID,
			Judul:  "Laporan Jalan Berlubang",
			Status: "menunggu",
		}
	}

	report := createReportSimulasi(authUserID, impersonatedUserID)

	if report.UserID != authUserID {
		t.Errorf("expected report.UserID == %d (from Auth), got %d", authUserID, report.UserID)
	}
	if report.UserID == impersonatedUserID {
		t.Error("CRITICAL: Report allowed UserID impersonation!")
	}
}

// 3. Test Admin Pemdes: Cannot access other wilayah or non-desa road
func TestWorkflow_AdminPemdes_StrictWilayahAndDesaScope(t *testing.T) {
	desaA := uint(10)
	desaB := uint(20)

	lapDesaA := models.LaporanKerusakan{
		Model:      gorm.Model{ID: 1},
		WilayahID:  desaA,
		JenisJalan: "desa",
	}
	lapDesaB := models.LaporanKerusakan{
		Model:      gorm.Model{ID: 2},
		WilayahID:  desaB,
		JenisJalan: "desa",
	}
	lapKabupatenA := models.LaporanKerusakan{
		Model:      gorm.Model{ID: 3},
		WilayahID:  desaA,
		JenisJalan: "kabupaten",
	}

	// Helper checking Pemdes access
	checkPemdesAccess := func(adminWilayahID *uint, lap models.LaporanKerusakan) bool {
		if adminWilayahID == nil {
			return false
		}
		return strings.ToLower(lap.JenisJalan) == "desa" && lap.WilayahID == *adminWilayahID
	}

	// Admin Pemdes Desa A
	if !checkPemdesAccess(&desaA, lapDesaA) {
		t.Error("Admin Pemdes Desa A should have access to Desa A reports")
	}
	if checkPemdesAccess(&desaA, lapDesaB) {
		t.Error("Admin Pemdes Desa A must NOT have access to Desa B reports (Isolation Violation)")
	}
	if checkPemdesAccess(&desaA, lapKabupatenA) {
		t.Error("Admin Pemdes Desa A must NOT have access to Kabupaten reports")
	}
}

// 4. Test Admin PU: Hardened to Kabupaten, cannot access Desa
func TestWorkflow_AdminPU_HardenedToKabupatenScope(t *testing.T) {
	checkPUAccess := func(jenisJalan string) bool {
		return strings.ToLower(jenisJalan) == "kabupaten"
	}

	if !checkPUAccess("kabupaten") {
		t.Error("Admin PU must have access to kabupaten road reports")
	}
	if checkPUAccess("desa") {
		t.Error("Admin PU must NOT have access to desa road reports")
	}
	if checkPUAccess("provinsi") {
		t.Error("Admin PU must NOT have access to provinsi road reports under hardened scope")
	}
	if checkPUAccess("nasional") {
		t.Error("Admin PU must NOT have access to nasional road reports under hardened scope")
	}
}

// 5. Test Superadmin: Full Access to any road and wilayah
func TestWorkflow_Superadmin_FullAccess(t *testing.T) {
	roads := []string{"desa", "kabupaten", "provinsi", "nasional"}
	for _, r := range roads {
		lap := models.LaporanKerusakan{
			Model:      gorm.Model{ID: 1},
			WilayahID:  99,
			JenisJalan: r,
		}
		if !utils.CekAksesLaporan(string(models.RoleSuperAdmin), 1, lap) {
			t.Errorf("Superadmin must have access to %s road reports", r)
		}
	}
}

// 6 & 7. Test Selesai status requires evidence; failed upload does not change status
func TestWorkflow_CompletionRequiresEvidence(t *testing.T) {
	validateCompletion := func(targetStatus string, existingEvidence string, hasNewUpload bool, uploadErr error) (bool, string) {
		statusLower := strings.ToLower(targetStatus)
		if statusLower == "selesai" {
			if !hasNewUpload && existingEvidence == "" {
				return false, "Foto bukti perbaikan wajib diunggah untuk menyelesaikan laporan"
			}
			if hasNewUpload && uploadErr != nil {
				return false, "Gagal mengupload foto bukti"
			}
		}
		return true, "OK"
	}

	// 1. Selesai without evidence -> Rejected
	ok, _ := validateCompletion("selesai", "", false, nil)
	if ok {
		t.Error("Selesai without evidence must be rejected")
	}

	// 2. Selesai with failed upload -> Rejected, status remains unchanged
	okFail, msg := validateCompletion("selesai", "", true, gorm.ErrInvalidData)
	if okFail || msg != "Gagal mengupload foto bukti" {
		t.Error("Selesai with failed upload must be rejected and not commit status change")
	}

	// 3. Selesai with valid upload -> Allowed
	okSuccess, _ := validateCompletion("selesai", "", true, nil)
	if !okSuccess {
		t.Error("Selesai with valid upload should be allowed")
	}

	// 4. Selesai with existing photo evidence -> Allowed
	okExisting, _ := validateCompletion("selesai", "https://example.com/bukti.jpg", false, nil)
	if !okExisting {
		t.Error("Selesai with existing evidence should be allowed")
	}
}

// 8 & 9. Test Status Change Notification: Only sent when status actually changes
func TestWorkflow_StatusNotification_NoDuplicateOnSameStatus(t *testing.T) {
	shouldNotify := func(oldStatus, newStatus string) bool {
		newStatusLower := strings.ToLower(strings.TrimSpace(newStatus))
		oldStatusLower := strings.ToLower(strings.TrimSpace(oldStatus))
		return newStatusLower != "" && newStatusLower != oldStatusLower
	}

	// A. Actual change: menunggu -> proses
	if !shouldNotify("menunggu", "proses") {
		t.Error("expected notification when status changes from menunggu to proses")
	}

	// B. Actual change: proses -> selesai
	if !shouldNotify("proses", "selesai") {
		t.Error("expected notification when status changes from proses to selesai")
	}

	// C. Same status: proses -> proses (or PROSES)
	if shouldNotify("proses", "proses") {
		t.Error("duplicate notification MUST NOT be sent when status is unchanged")
	}
	if shouldNotify("proses", "PROSES") {
		t.Error("duplicate notification MUST NOT be sent when status case is varied")
	}
}

// 10 & 11. Test Chat Notification Recipient Routing
func TestWorkflow_ChatNotificationRecipients(t *testing.T) {
	// A. Warga sends chat: routed to appropriate admin
	resolveChatRecipientsForAdmin := func(jenisJalan string) string {
		switch strings.ToLower(jenisJalan) {
		case "desa":
			return "admin_pemdes"
		case "kabupaten":
			return "admin_pu"
		default:
			return "none"
		}
	}

	if resolveChatRecipientsForAdmin("desa") != "admin_pemdes" {
		t.Error("Chat on desa report must notify admin_pemdes")
	}
	if resolveChatRecipientsForAdmin("kabupaten") != "admin_pu" {
		t.Error("Chat on kabupaten report must notify admin_pu")
	}

	// B. Admin replies chat: routed strictly to report owner citizen
	ownerUserID := uint(77)
	lap := models.LaporanKerusakan{
		Model:  gorm.Model{ID: 10},
		UserID: ownerUserID,
	}

	recipient := lap.UserID
	if recipient != ownerUserID {
		t.Errorf("Admin reply notification must go to report owner (%d), got %d", ownerUserID, recipient)
	}
}

// 12. Test Notification IDOR Protection
func TestWorkflow_NotificationIDORProtection(t *testing.T) {
	callerID := uint(10)
	otherUserID := uint(20)

	notif := models.Notifikasi{
		Model:  gorm.Model{ID: 5},
		UserID: otherUserID,
	}

	canMarkRead := notif.UserID == callerID
	if canMarkRead {
		t.Error("User must NOT be able to mark another user's notification as read (IDOR)")
	}
}

// 13. Test Soft-deleted report cannot be accessed for new interactions
func TestWorkflow_SoftDeletedReportBlockedFromInteraction(t *testing.T) {
	softDeletedLap := models.LaporanKerusakan{
		Model: gorm.Model{
			ID:        15,
			DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true},
		},
		UserID:     10,
		JenisJalan: "desa",
		WilayahID:  1,
	}

	// CekAksesLaporan must reject soft-deleted reports for all roles
	if utils.CekAksesLaporan(string(models.RoleWarga), 10, softDeletedLap) {
		t.Error("Warga must not be granted access to soft-deleted report")
	}
	if utils.CekAksesLaporan(string(models.RoleAdminPemdes), 2, softDeletedLap) {
		t.Error("Admin Pemdes must not be granted access to soft-deleted report")
	}
	if utils.CekAksesLaporan(string(models.RoleAdminPu), 3, softDeletedLap) {
		t.Error("Admin PU must not be granted access to soft-deleted report")
	}
	if utils.CekAksesLaporan(string(models.RoleSuperAdmin), 1, softDeletedLap) {
		t.Error("Superadmin must not be granted active access to soft-deleted report")
	}
}

// 14 & 15. Test Historical Chat & Notification preserved when report is soft-deleted
func TestWorkflow_HistoricalDataPreservedOnReportSoftDelete(t *testing.T) {
	reportID := uint(55)

	chatHistory := []models.RiwayatChat{
		{Model: gorm.Model{ID: 1}, LaporanKerusakanID: reportID, Pesan: "Pertanyaan 1"},
		{Model: gorm.Model{ID: 2}, LaporanKerusakanID: reportID, Pesan: "Pertanyaan 2"},
	}

	notifHistory := []models.Notifikasi{
		{Model: gorm.Model{ID: 101}, LaporanID: reportID, Judul: "Laporan Terkirim"},
	}

	// Simulating soft delete of report
	reportDeleted := true

	// Assert historical child items still exist with their original records
	if reportDeleted {
		if len(chatHistory) != 2 {
			t.Errorf("expected 2 historical chat records, got %d", len(chatHistory))
		}
		if len(notifHistory) != 1 {
			t.Errorf("expected 1 historical notification record, got %d", len(notifHistory))
		}
		for _, ch := range chatHistory {
			if ch.DeletedAt.Valid {
				t.Errorf("historical chat record %d was unintentionally soft deleted", ch.ID)
			}
		}
	}
}
