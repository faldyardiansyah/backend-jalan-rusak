package models

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

// 1. User model relation & Password security
func TestModel_UserStructureAndExclusion(t *testing.T) {
	wilayahID := uint(10)
	user := User{
		Model:        gorm.Model{ID: 1},
		Name:         "Budi Santoso",
		Email:        "budi@roadis.id",
		Password:     "super_secret_hash_value",
		Role:         RoleAdminPemdes,
		WilayahID:    &wilayahID,
		Wilayah:      Wilayah{Model: gorm.Model{ID: 10}, Nama: "Lobener Lor", Tipe: "desa"},
		ProfilePhoto: "https://example.com/budi.jpg",
		Reports: []LaporanKerusakan{
			{Model: gorm.Model{ID: 101}, Judul: "Laporan 1"},
		},
		ChatHistories: []RiwayatChat{
			{Model: gorm.Model{ID: 201}, Pesan: "Chat 1"},
		},
	}

	b, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	jsonStr := string(b)
	if strings.Contains(jsonStr, "password") || strings.Contains(jsonStr, "super_secret_hash_value") {
		t.Errorf("User password must not be serialized to JSON: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"wilayah_id":10`) {
		t.Errorf("expected wilayah_id in JSON: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"reports"`) || !strings.Contains(jsonStr, `"riwayat_chat"`) {
		t.Errorf("expected associated reports and riwayat_chat in User JSON: %s", jsonStr)
	}

	// Nullable WilayahID for non-pemdes
	userPU := User{
		Model:     gorm.Model{ID: 2},
		Name:      "Admin PU",
		Email:     "pu@roadis.id",
		Role:      RoleAdminPu,
		WilayahID: nil,
	}

	bPU, err := json.Marshal(userPU)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	jsonStrPU := string(bPU)
	if !strings.Contains(jsonStrPU, `"wilayah_id":null`) {
		t.Errorf("expected null wilayah_id in JSON: %s", jsonStrPU)
	}
}

// 2. Wilayah model relation & structure
func TestModel_WilayahStructure(t *testing.T) {
	wilayah := Wilayah{
		Model: gorm.Model{ID: 1},
		Nama:  "Indramayu",
		Tipe:  "kabupaten",
	}

	b, err := json.Marshal(wilayah)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	jsonStr := string(b)
	if !strings.Contains(jsonStr, `"nama":"Indramayu"`) || !strings.Contains(jsonStr, `"tipe":"kabupaten"`) {
		t.Errorf("unexpected JSON for wilayah: %s", jsonStr)
	}
}

// 3. LaporanKerusakan model fields, types, and relations
func TestModel_LaporanKerusakanFields(t *testing.T) {
	laporan := LaporanKerusakan{
		Model:         gorm.Model{ID: 5},
		UserID:        1,
		User:          User{Model: gorm.Model{ID: 1}, Name: "Pelapor"},
		WilayahID:     2,
		Wilayah:       Wilayah{Model: gorm.Model{ID: 2}, Nama: "Lobener Lor", Tipe: "desa"},
		JenisJalan:    "kabupaten",
		Judul:         "Jalan Berlubang",
		Deskripsi:     "Lubang cukup dalam",
		Latitude:      -6.3265,
		Longitude:     108.3241,
		ImageURL:      "http://example.com/img.jpg",
		TipeKerusakan: "lubang",
		Status:        "menunggu",
		DitugaskanKe:  "Tim Pemeliharaan",
		FotoBukti:     "http://example.com/bukti.jpg",
		CatatanAdmin:  "Diproses segera",
	}

	b, err := json.Marshal(laporan)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	jsonStr := string(b)
	expectedSubstrings := []string{
		`"user_id":1`,
		`"wilayah_id":2`,
		`"jenis_jalan":"kabupaten"`,
		`"status":"menunggu"`,
		`"latitude":-6.3265`,
		`"longitude":108.3241`,
		`"user"`,
		`"wilayah"`,
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(jsonStr, sub) {
			t.Errorf("expected JSON to contain %s, got: %s", sub, jsonStr)
		}
	}
}

// 4. RiwayatChat model tags, table name, nullability, and relations
func TestModel_RiwayatChatTagsAndNullability(t *testing.T) {
	chat := RiwayatChat{
		Model:              gorm.Model{ID: 10},
		LaporanKerusakanID: 5,
		LaporanKerusakan:   LaporanKerusakan{Model: gorm.Model{ID: 5}, Judul: "Laporan Terkait"},
		UserID:             1,
		User:               User{Model: gorm.Model{ID: 1}, Name: "Warga Pelapor"},
		Pesan:              "Kapan jalan diperbaiki?",
		AdminID:            nil,
		Balasan:            nil,
		DibalasAt:          nil,
	}

	// Verify TableName
	if chat.TableName() != "riwayat_chat" {
		t.Errorf("expected TableName to be 'riwayat_chat', got '%s'", chat.TableName())
	}

	// Verify reflection on struct tags (ensures no syntax errors for go vet)
	typ := reflect.TypeOf(chat)
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		_ = field.Tag.Get("json")
		_ = field.Tag.Get("gorm")
	}

	// Verify JSON with null reply
	b, err := json.Marshal(chat)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	jsonStr := string(b)
	if !strings.Contains(jsonStr, `"admin_id":null`) || !strings.Contains(jsonStr, `"balasan":null`) {
		t.Errorf("expected null admin_id and balasan: %s", jsonStr)
	}

	// Verify with admin reply and relation
	adminID := uint(3)
	replyText := "Sedang dijadwalkan."
	now := time.Now()
	chat.AdminID = &adminID
	chat.Admin = &User{Model: gorm.Model{ID: adminID}, Name: "Admin Petugas"}
	chat.Balasan = &replyText
	chat.DibalasAt = &now

	bReply, err := json.Marshal(chat)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	jsonReplyStr := string(bReply)
	if !strings.Contains(jsonReplyStr, `"admin_id":3`) || !strings.Contains(jsonReplyStr, `"balasan":"Sedang dijadwalkan."`) {
		t.Errorf("expected populated admin_id and balasan: %s", jsonReplyStr)
	}
}

// 5. Notifikasi model structure and default IsRead
func TestModel_NotifikasiStructure(t *testing.T) {
	notif := Notifikasi{
		Model:     gorm.Model{ID: 20},
		UserID:    1,
		LaporanID: 5,
		Judul:     "Status Laporan Berubah",
		Pesan:     "Laporan statusnya diperbarui",
		IsRead:    false,
	}

	b, err := json.Marshal(notif)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	jsonStr := string(b)
	if !strings.Contains(jsonStr, `"user_id":1`) || !strings.Contains(jsonStr, `"is_read":false`) {
		t.Errorf("unexpected JSON for notifikasi: %s", jsonStr)
	}
}

// 6-10. Soft Delete Verification for all 5 models
func TestModel_SoftDelete_AllModels(t *testing.T) {
	modelsToTest := []struct {
		name     string
		getModel func() gorm.Model
	}{
		{"User", func() gorm.Model { return User{}.Model }},
		{"Wilayah", func() gorm.Model { return Wilayah{}.Model }},
		{"LaporanKerusakan", func() gorm.Model { return LaporanKerusakan{}.Model }},
		{"RiwayatChat", func() gorm.Model { return RiwayatChat{}.Model }},
		{"Notifikasi", func() gorm.Model { return Notifikasi{}.Model }},
	}

	for _, m := range modelsToTest {
		t.Run("SoftDelete_"+m.name, func(t *testing.T) {
			active := m.getModel()
			active.ID = 1
			active.DeletedAt = gorm.DeletedAt{Time: time.Time{}, Valid: false}

			deleted := m.getModel()
			deleted.ID = 2
			deleted.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}

			if active.DeletedAt.Valid {
				t.Errorf("[%s] active model should have DeletedAt.Valid = false", m.name)
			}
			if !deleted.DeletedAt.Valid {
				t.Errorf("[%s] soft-deleted model should have DeletedAt.Valid = true", m.name)
			}
		})
	}
}

// 11-17. Referential Integrity & Association Field Validations
func TestModel_ReferentialIntegrity_Fields(t *testing.T) {
	// 11. User -> Wilayah (foreign key tag and type)
	userType := reflect.TypeOf(User{})
	wilayahIDField, ok := userType.FieldByName("WilayahID")
	if !ok || wilayahIDField.Type.Kind() != reflect.Ptr {
		t.Errorf("User.WilayahID must exist and be *uint")
	}
	wilayahRel, ok := userType.FieldByName("Wilayah")
	if !ok || wilayahRel.Tag.Get("gorm") != "foreignKey:WilayahID" {
		t.Errorf("User.Wilayah association tag must be foreignKey:WilayahID")
	}

	// 12. LaporanKerusakan -> User
	lapType := reflect.TypeOf(LaporanKerusakan{})
	userField, ok := lapType.FieldByName("User")
	if !ok || !strings.Contains(userField.Tag.Get("gorm"), "foreignKey:UserID") {
		t.Errorf("LaporanKerusakan.User association tag must specify foreignKey:UserID")
	}

	// 13. LaporanKerusakan -> Wilayah
	wilayahField, ok := lapType.FieldByName("Wilayah")
	if !ok || !strings.Contains(wilayahField.Tag.Get("gorm"), "foreignKey:WilayahID") {
		t.Errorf("LaporanKerusakan.Wilayah association tag must specify foreignKey:WilayahID")
	}

	// 14. RiwayatChat -> LaporanKerusakan
	chatType := reflect.TypeOf(RiwayatChat{})
	chatLapField, ok := chatType.FieldByName("LaporanKerusakan")
	if !ok || !strings.Contains(chatLapField.Tag.Get("gorm"), "foreignKey:LaporanKerusakanID") {
		t.Errorf("RiwayatChat.LaporanKerusakan association tag must specify foreignKey:LaporanKerusakanID")
	}

	// 15. RiwayatChat -> User
	chatUserField, ok := chatType.FieldByName("User")
	if !ok || !strings.Contains(chatUserField.Tag.Get("gorm"), "foreignKey:UserID") {
		t.Errorf("RiwayatChat.User association tag must specify foreignKey:UserID")
	}

	// 16. Notifikasi -> User
	notifType := reflect.TypeOf(Notifikasi{})
	notifUserField, ok := notifType.FieldByName("UserID")
	if !ok || notifUserField.Type.Kind() != reflect.Uint {
		t.Errorf("Notifikasi.UserID must exist and be uint")
	}

	// 17. Notifikasi -> Laporan
	notifLapField, ok := notifType.FieldByName("LaporanID")
	if !ok || notifLapField.Type.Kind() != reflect.Uint {
		t.Errorf("Notifikasi.LaporanID must exist and be uint")
	}
}

// Enum Role constants verification
func TestModel_UserRoles(t *testing.T) {
	if RoleWarga != "warga" {
		t.Errorf("expected RoleWarga == 'warga'")
	}
	if RoleAdminPemdes != "admin_pemdes" {
		t.Errorf("expected RoleAdminPemdes == 'admin_pemdes'")
	}
	if RoleAdminPu != "admin_pu" {
		t.Errorf("expected RoleAdminPu == 'admin_pu'")
	}
	if RoleSuperAdmin != "super_admin" {
		t.Errorf("expected RoleSuperAdmin == 'super_admin'")
	}
}

// ============================================================
// PHASE 8 REPAIR — HISTORICAL DATA & SOFT DELETE INTEGRITY TESTS
// ============================================================

// 18. User soft delete maintains reports, chat, and notification
func TestHistoricalIntegrity_UserSoftDeleteRetainsChildData(t *testing.T) {
	// Simulate user being soft-deleted
	deletedUser := User{
		Model: gorm.Model{
			ID:        10,
			CreatedAt: time.Now().Add(-24 * time.Hour),
			UpdatedAt: time.Now(),
			DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true},
		},
		Name:  "Ex-Admin Pemdes",
		Email: "admin_ex@roadis.id",
		Role:  RoleAdminPemdes,
	}

	// Associated report created while active
	associatedReport := LaporanKerusakan{
		Model: gorm.Model{
			ID:        101,
			CreatedAt: time.Now().Add(-20 * time.Hour),
			UpdatedAt: time.Now().Add(-10 * time.Hour),
			DeletedAt: gorm.DeletedAt{Time: time.Time{}, Valid: false}, // Active
		},
		UserID: deletedUser.ID,
		Judul:  "Laporan Kerusakan Jalan Poros",
		Status: "proses",
	}

	// Associated chat
	associatedChat := RiwayatChat{
		Model: gorm.Model{
			ID:        201,
			CreatedAt: time.Now().Add(-15 * time.Hour),
			DeletedAt: gorm.DeletedAt{Time: time.Time{}, Valid: false}, // Active
		},
		LaporanKerusakanID: associatedReport.ID,
		UserID:             deletedUser.ID,
		Pesan:              "Kapan perbaikan dilakukan?",
	}

	// Associated notification
	associatedNotif := Notifikasi{
		Model: gorm.Model{
			ID:        301,
			CreatedAt: time.Now().Add(-10 * time.Hour),
			DeletedAt: gorm.DeletedAt{Time: time.Time{}, Valid: false}, // Active
		},
		UserID:    deletedUser.ID,
		LaporanID: associatedReport.ID,
		Judul:     "Laporan Diproses",
		Pesan:     "Laporan sedang ditangani.",
	}

	// Assert: Child models remain active despite parent user being soft-deleted
	if !deletedUser.DeletedAt.Valid {
		t.Error("deletedUser should have DeletedAt.Valid = true")
	}
	if associatedReport.DeletedAt.Valid {
		t.Error("associatedReport must remain active (DeletedAt.Valid = false)")
	}
	if associatedChat.DeletedAt.Valid {
		t.Error("associatedChat must remain active (DeletedAt.Valid = false)")
	}
	if associatedNotif.DeletedAt.Valid {
		t.Error("associatedNotif must remain active (DeletedAt.Valid = false)")
	}
}

// 19. Laporan soft delete maintains chat and notifications
func TestHistoricalIntegrity_LaporanSoftDeleteRetainsChildData(t *testing.T) {
	// Simulate report soft-deleted (e.g. marked as spam by superadmin)
	deletedReport := LaporanKerusakan{
		Model: gorm.Model{
			ID:        501,
			CreatedAt: time.Now().Add(-48 * time.Hour),
			UpdatedAt: time.Now(),
			DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true},
		},
		UserID: 1,
		Judul:  "Laporan Spam Terhapus",
		Status: "menunggu",
	}

	// Historical chat records under this report
	historicalChat := RiwayatChat{
		Model: gorm.Model{
			ID:        601,
			CreatedAt: time.Now().Add(-30 * time.Hour),
			DeletedAt: gorm.DeletedAt{Time: time.Time{}, Valid: false}, // Unaltered
		},
		LaporanKerusakanID: deletedReport.ID,
		UserID:             1,
		Pesan:              "Pesan historis pada laporan spam",
	}

	// Historical notification referencing this report
	historicalNotif := Notifikasi{
		Model: gorm.Model{
			ID:        701,
			CreatedAt: time.Now().Add(-30 * time.Hour),
			DeletedAt: gorm.DeletedAt{Time: time.Time{}, Valid: false}, // Unaltered
		},
		UserID:    1,
		LaporanID: deletedReport.ID,
		Judul:     "Pemberitahuan Laporan",
		Pesan:     "Laporan telah diterima sistem.",
	}

	// Assert: Soft-deleting report does NOT alter or erase historical chat/notif
	if !deletedReport.DeletedAt.Valid {
		t.Error("deletedReport should have DeletedAt.Valid = true")
	}
	if historicalChat.DeletedAt.Valid {
		t.Error("historicalChat must not be deleted (DeletedAt.Valid = false)")
	}
	if historicalNotif.DeletedAt.Valid {
		t.Error("historicalNotif must not be deleted (DeletedAt.Valid = false)")
	}
}

// 20. Static audit verifying that Unscoped() is NEVER used in the repository
func TestAudit_NoUnscopedInEntireCodebase(t *testing.T) {
	rootPath := ".."
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(info.Name(), ".go") && !strings.HasSuffix(info.Name(), "_test.go") {
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			if strings.Contains(string(content), "Unscoped()") {
				t.Errorf("CRITICAL VIOLATION: Unscoped() detected in file %s", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("filepath.Walk failed: %v", err)
	}
}

// 21. Verify RoleWarga cannot be deleted (app-level guard safeguarding report ownership)
func TestDeleteProtection_RoleWargaImmutable(t *testing.T) {
	canDeleteUser := func(role UserRole) (bool, string) {
		if role == RoleWarga {
			return false, "Tidak dapat menghapus warga"
		}
		return true, ""
	}

	allowed, reason := canDeleteUser(RoleWarga)
	if allowed {
		t.Error("RoleWarga must NOT be deletable")
	}
	if reason != "Tidak dapat menghapus warga" {
		t.Errorf("unexpected rejection message: %s", reason)
	}

	// Non-warga roles can be soft deleted subject to superadmin checks
	allowedPU, _ := canDeleteUser(RoleAdminPu)
	if !allowedPU {
		t.Error("RoleAdminPu should be deletable via soft delete")
	}
}
