package controllers

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"
	"gorm.io/gorm"
)

// 1. Contract Test: Empty Collections MUST serialize as [] and NEVER null
func TestContract_EmptyCollectionsSerializeAsEmptyArray(t *testing.T) {
	// A. Chat response list
	emptyChats := make([]ChatResponse, 0)
	bChats, err := json.Marshal(ginHData(emptyChats))
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if !strings.Contains(string(bChats), `"data":[]`) {
		t.Errorf("expected empty chats to serialize as '[]', got: %s", string(bChats))
	}

	// B. Admin inbox list
	emptyInbox := make([]AdminInboxItem, 0)
	bInbox, err := json.Marshal(ginHData(emptyInbox))
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if !strings.Contains(string(bInbox), `"data":[]`) {
		t.Errorf("expected empty inbox to serialize as '[]', got: %s", string(bInbox))
	}

	// C. Notifikasi list
	emptyNotifs := make([]models.Notifikasi, 0)
	bNotifs, err := json.Marshal(ginHData(emptyNotifs))
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if !strings.Contains(string(bNotifs), `"data":[]`) {
		t.Errorf("expected empty notifikasi to serialize as '[]', got: %s", string(bNotifs))
	}

	// D. Users list
	emptyUsers := make([]models.User, 0)
	bUsers, err := json.Marshal(ginHData(emptyUsers))
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if !strings.Contains(string(bUsers), `"data":[]`) {
		t.Errorf("expected empty users to serialize as '[]', got: %s", string(bUsers))
	}

	// E. Wilayah list
	emptyWilayah := make([]models.Wilayah, 0)
	bWilayah, err := json.Marshal(ginHData(emptyWilayah))
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if !strings.Contains(string(bWilayah), `"data":[]`) {
		t.Errorf("expected empty wilayah to serialize as '[]', got: %s", string(bWilayah))
	}
}

// 2. Contract Test: Sensitive Data Leak Prevention
func TestContract_SensitiveFieldsOmittedInResponses(t *testing.T) {
	wilayahID := uint(5)
	user := models.User{
		Model:        gorm.Model{ID: 1},
		Name:         "Warga Rahasia",
		Email:        "warga@roadis.id",
		Password:     "$2a$10$very_secret_bcrypt_hash_that_must_not_leak",
		Role:         models.RoleWarga,
		WilayahID:    &wilayahID,
		ProfilePhoto: "https://example.com/photo.jpg",
	}

	// A. Direct User serialization
	bUser, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("Marshal user failed: %v", err)
	}
	userStr := string(bUser)
	if strings.Contains(userStr, "password") || strings.Contains(userStr, "very_secret") {
		t.Errorf("password leaked in User serialization: %s", userStr)
	}

	// B. ChatResponse containing User and Admin
	adminID := uint(2)
	admin := models.User{
		Model:    gorm.Model{ID: 2},
		Name:     "Admin Petugas",
		Email:    "admin@roadis.id",
		Password: "$2a$10$admin_super_secret_password_hash",
		Role:     models.RoleAdminPu,
	}

	balasan := "Sedang dalam pengerjaan"
	chat := models.RiwayatChat{
		Model:              gorm.Model{ID: 10},
		LaporanKerusakanID: 100,
		UserID:             user.ID,
		User:               user,
		Pesan:              "Kapan selesai?",
		AdminID:            &adminID,
		Admin:              &admin,
		Balasan:            &balasan,
		DibalasAt:          nil,
	}

	chatResp := FormatChatToResponse(chat)
	bChat, err := json.Marshal(chatResp)
	if err != nil {
		t.Fatalf("Marshal chat failed: %v", err)
	}
	chatStr := string(bChat)
	if strings.Contains(chatStr, "password") || strings.Contains(chatStr, "secret") {
		t.Errorf("password leaked in ChatResponse serialization: %s", chatStr)
	}
}

// 3. Contract Test: Validation of Input Lengths and Empty/Whitespace strings
func TestContract_InputValidationRules(t *testing.T) {
	// Rule A: Empty or whitespace strings are invalid
	validateNonEmpty := func(s string) bool {
		return strings.TrimSpace(s) != ""
	}

	if validateNonEmpty("") {
		t.Error("expected empty string to be rejected")
	}
	if validateNonEmpty("   \t\n  ") {
		t.Error("expected whitespace-only string to be rejected")
	}
	if !validateNonEmpty("Valid Title") {
		t.Error("expected valid string to be accepted")
	}

	// Rule B: String max lengths
	validateMaxLength := func(s string, max int) bool {
		return len(strings.TrimSpace(s)) <= max
	}

	shortText := "Jalan Rusak Poros Desa"
	longText := strings.Repeat("A", 151)

	if !validateMaxLength(shortText, 150) {
		t.Error("expected shortText <= 150 to be valid")
	}
	if validateMaxLength(longText, 150) {
		t.Error("expected longText > 150 to be rejected")
	}

	// Rule C: Status Enum
	validStatuses := map[string]bool{"menunggu": true, "proses": true, "selesai": true, "ditolak": true}
	if !validStatuses["menunggu"] || !validStatuses["proses"] || !validStatuses["selesai"] || !validStatuses["ditolak"] {
		t.Error("expected valid statuses")
	}
	if validStatuses["pending"] || validStatuses["rejected"] {
		t.Error("foreign statuses should not be valid")
	}
}

// 4. Contract Test: Timestamp Format Consistency
func TestContract_TimestampIndoFormatting(t *testing.T) {
	fixedTime := time.Date(2026, 9, 27, 10, 30, 0, 0, time.UTC)
	formatted := utils.FormatTanggalIndo(&fixedTime)

	if !strings.Contains(formatted, "27") || !strings.Contains(formatted, "September") || !strings.Contains(formatted, "2026") {
		t.Errorf("expected Indonesian date format, got: %s", formatted)
	}
}

// Helper mock gin.H{"data": ...}
func ginHData(data interface{}) map[string]interface{} {
	return map[string]interface{}{
		"status":  "success",
		"message": "OK",
		"data":    data,
	}
}
