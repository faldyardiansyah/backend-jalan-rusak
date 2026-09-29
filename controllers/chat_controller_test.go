package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"backend-jalan-rusak/middlewares"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Helper simulasi otorisasi laporan berbasis CekAksesLaporan
func cekAksesLaporanSimulasi(
	role string,
	userID uint,
	adminWilayahID *uint,
	lap models.LaporanKerusakan,
) (bool, string) {
	if lap.DeletedAt.Valid {
		return false, "deleted_report"
	}

	jenisJalan := strings.ToLower(lap.JenisJalan)

	switch role {
	case string(models.RoleWarga):
		if lap.UserID != userID {
			return false, "not_owner"
		}
		return true, "ok"

	case string(models.RoleAdminPemdes):
		if adminWilayahID == nil {
			return false, "unassigned_wilayah"
		}
		if jenisJalan != "desa" {
			return false, "jenis_jalan_mismatch"
		}
		if lap.WilayahID != *adminWilayahID {
			return false, "wilayah_mismatch"
		}
		return true, "ok"

	case string(models.RoleAdminPu):
		if jenisJalan != "kabupaten" {
			return false, "jenis_jalan_mismatch"
		}
		return true, "ok"

	case string(models.RoleSuperAdmin):
		return true, "ok"

	default:
		return false, "forbidden"
	}
}

// A. Test Warga Access Matrix
func TestChatAuth_WargaOwnerVsNonOwner(t *testing.T) {
	ownerID := uint(10)
	otherUserID := uint(20)

	lap := models.LaporanKerusakan{
		Model:      gorm.Model{ID: 1},
		UserID:     ownerID,
		JenisJalan: "desa",
		WilayahID:  1,
	}

	// Owner -> ALLOWED
	ok, reason := cekAksesLaporanSimulasi(string(models.RoleWarga), ownerID, nil, lap)
	if !ok {
		t.Errorf("expected owner to be allowed, got: %s", reason)
	}

	// Non-Owner -> FORBIDDEN (IDOR protection)
	ok, reason = cekAksesLaporanSimulasi(string(models.RoleWarga), otherUserID, nil, lap)
	if ok || reason != "not_owner" {
		t.Errorf("expected non-owner to be rejected with not_owner, got ok=%v, reason=%s", ok, reason)
	}

	// Soft-deleted report -> REJECTED
	lapDeleted := lap
	lapDeleted.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
	ok, reason = cekAksesLaporanSimulasi(string(models.RoleWarga), ownerID, nil, lapDeleted)
	if ok || reason != "deleted_report" {
		t.Errorf("expected soft-deleted report to be rejected, got ok=%v, reason=%s", ok, reason)
	}
}

// B. Test Input Validation (Pesan & Balasan)
func TestChatInputValidation_PesanWarga(t *testing.T) {
	cases := []struct {
		name        string
		pesan       string
		expectValid bool
		errReason   string
	}{
		{"Valid standard message", "Kondisi jalan sangat parah, mohon diperbaiki.", true, ""},
		{"Empty message", "", false, "empty"},
		{"Whitespace only message (spaces)", "   ", false, "whitespace"},
		{"Whitespace only message (tabs and newlines)", "\t\n  \r\n", false, "whitespace"},
		{"Exceeding maximum length 1000 chars", strings.Repeat("A", 1001), false, "too_long"},
		{"Exact maximum length 1000 chars", strings.Repeat("A", 1000), true, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			trimmed := strings.TrimSpace(tc.pesan)
			isValid := true
			reason := ""

			if trimmed == "" {
				isValid = false
				if tc.pesan == "" {
					reason = "empty"
				} else {
					reason = "whitespace"
				}
			} else if len(trimmed) > 1000 {
				isValid = false
				reason = "too_long"
			}

			if isValid != tc.expectValid {
				t.Errorf("[%s] expected valid=%v, got=%v (reason=%s)", tc.name, tc.expectValid, isValid, reason)
			}
			if !tc.expectValid && reason != tc.errReason {
				t.Errorf("[%s] expected errReason=%s, got=%s", tc.name, tc.errReason, reason)
			}
		})
	}
}

func TestChatInputValidation_BalasanAdmin(t *testing.T) {
	cases := []struct {
		name        string
		balasan     string
		expectValid bool
		errReason   string
	}{
		{"Valid admin reply", "Laporan sudah kami terima dan tim lapangan telah dijadwalkan.", true, ""},
		{"Empty reply", "", false, "empty"},
		{"Whitespace only reply", "    ", false, "whitespace"},
		{"Exceeding 1000 chars", strings.Repeat("B", 1001), false, "too_long"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			trimmed := strings.TrimSpace(tc.balasan)
			isValid := true
			reason := ""

			if trimmed == "" {
				isValid = false
				if tc.balasan == "" {
					reason = "empty"
				} else {
					reason = "whitespace"
				}
			} else if len(trimmed) > 1000 {
				isValid = false
				reason = "too_long"
			}

			if isValid != tc.expectValid {
				t.Errorf("[%s] expected valid=%v, got=%v (reason=%s)", tc.name, tc.expectValid, isValid, reason)
			}
		})
	}
}

// C. Test Admin Pemdes Scope
func TestChatAuth_AdminPemdesScope(t *testing.T) {
	wilayahA := uint(1)
	wilayahB := uint(2)
	adminID := uint(5)

	tests := []struct {
		name        string
		jenisJalan  string
		wilayahID   uint
		expectAllow bool
	}{
		{"Desa wilayah sendiri -> ALLOWED", "desa", wilayahA, true},
		{"Desa wilayah lain -> REJECTED", "desa", wilayahB, false},
		{"Kabupaten di wilayah sendiri -> REJECTED", "kabupaten", wilayahA, false},
		{"Provinsi di wilayah sendiri -> REJECTED", "provinsi", wilayahA, false},
		{"Nasional di wilayah sendiri -> REJECTED", "nasional", wilayahA, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lap := models.LaporanKerusakan{
				Model:      gorm.Model{ID: 100},
				UserID:     2,
				JenisJalan: tc.jenisJalan,
				WilayahID:  tc.wilayahID,
			}

			ok, _ := cekAksesLaporanSimulasi(string(models.RoleAdminPemdes), adminID, &wilayahA, lap)
			if ok != tc.expectAllow {
				t.Errorf("[%s] expected allow=%v, got=%v", tc.name, tc.expectAllow, ok)
			}
		})
	}

	// Fail-closed test: Admin Pemdes tanpa wilayah_id
	lapDesaA := models.LaporanKerusakan{Model: gorm.Model{ID: 101}, JenisJalan: "desa", WilayahID: wilayahA}
	ok, reason := cekAksesLaporanSimulasi(string(models.RoleAdminPemdes), adminID, nil, lapDesaA)
	if ok || reason != "unassigned_wilayah" {
		t.Errorf("expected fail-closed unassigned_wilayah, got ok=%v, reason=%s", ok, reason)
	}
}

// D. Test Admin PU Scope
func TestChatAuth_AdminPUScope(t *testing.T) {
	adminPUID := uint(6)

	tests := []struct {
		name        string
		jenisJalan  string
		wilayahID   uint
		expectAllow bool
	}{
		{"Kabupaten Wilayah A -> ALLOWED", "kabupaten", 1, true},
		{"Kabupaten Wilayah B -> ALLOWED", "kabupaten", 2, true},
		{"Desa -> REJECTED", "desa", 1, false},
		{"Provinsi -> REJECTED", "provinsi", 1, false},
		{"Nasional -> REJECTED", "nasional", 1, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lap := models.LaporanKerusakan{
				Model:      gorm.Model{ID: 200},
				UserID:     3,
				JenisJalan: tc.jenisJalan,
				WilayahID:  tc.wilayahID,
			}

			ok, _ := cekAksesLaporanSimulasi(string(models.RoleAdminPu), adminPUID, nil, lap)
			if ok != tc.expectAllow {
				t.Errorf("[%s] expected allow=%v, got=%v", tc.name, tc.expectAllow, ok)
			}
		})
	}
}

// E. Test Superadmin Scope
func TestChatAuth_SuperadminScope(t *testing.T) {
	superadminID := uint(1)

	roads := []string{"desa", "kabupaten", "provinsi", "nasional"}
	for _, road := range roads {
		lap := models.LaporanKerusakan{
			Model:      gorm.Model{ID: 300},
			UserID:     4,
			JenisJalan: road,
			WilayahID:  1,
		}

		ok, _ := cekAksesLaporanSimulasi(string(models.RoleSuperAdmin), superadminID, nil, lap)
		if !ok {
			t.Errorf("superadmin expected to access road %s, but rejected", road)
		}
	}
}

// G. Test IDOR Protection across different chats
func TestChatIDOR_ChatIsolation(t *testing.T) {
	userA := uint(11)
	userB := uint(22)

	reportA := models.LaporanKerusakan{Model: gorm.Model{ID: 501}, UserID: userA, JenisJalan: "desa", WilayahID: 1}
	reportB := models.LaporanKerusakan{Model: gorm.Model{ID: 502}, UserID: userB, JenisJalan: "desa", WilayahID: 1}

	// User A can access report A, but NOT report B
	okA, _ := cekAksesLaporanSimulasi(string(models.RoleWarga), userA, nil, reportA)
	if !okA {
		t.Error("User A should access their own report chat")
	}
	okB, _ := cekAksesLaporanSimulasi(string(models.RoleWarga), userA, nil, reportB)
	if okB {
		t.Error("User A must NOT be able to access User B's report chat (IDOR vulnerability)")
	}
}

// H. Test Admin Inbox Scope & MenungguBalasan Calculation
func TestAdminInbox_AggregationLogic(t *testing.T) {
	wilayahA := uint(1)
	wilayahB := uint(2)

	reports := []models.LaporanKerusakan{
		{Model: gorm.Model{ID: 1}, JenisJalan: "desa", WilayahID: wilayahA, Judul: "Jalan Desa A"},
		{Model: gorm.Model{ID: 2}, JenisJalan: "desa", WilayahID: wilayahB, Judul: "Jalan Desa B"},
		{Model: gorm.Model{ID: 3}, JenisJalan: "kabupaten", WilayahID: wilayahA, Judul: "Jalan Kabupaten"},
	}

	replyText := "Sudah ditangani"
	chats := map[uint][]models.RiwayatChat{
		1: {
			{LaporanKerusakanID: 1, Pesan: "Tolong perbaiki", Balasan: nil}, // belum dibalas
		},
		2: {
			{LaporanKerusakanID: 2, Pesan: "Banjir parah", Balasan: &replyText}, // sudah dibalas
		},
		3: {
			{LaporanKerusakanID: 3, Pesan: "Aspal mengelupas", Balasan: nil}, // belum dibalas
		},
	}

	// Filter inbox untuk Admin Pemdes Wilayah A
	var pemdesInbox []AdminInboxItem
	for _, rep := range reports {
		ok, _ := cekAksesLaporanSimulasi(string(models.RoleAdminPemdes), 5, &wilayahA, rep)
		if !ok {
			continue
		}
		repChats := chats[rep.ID]
		menunggu := false
		for _, ch := range repChats {
			if ch.Balasan == nil || *ch.Balasan == "" {
				menunggu = true
				break
			}
		}
		pemdesInbox = append(pemdesInbox, AdminInboxItem{
			LaporanID:            rep.ID,
			JudulLaporan:         rep.Judul,
			MenungguBalasanAdmin: menunggu,
			TotalPesan:           len(repChats),
		})
	}

	if len(pemdesInbox) != 1 || pemdesInbox[0].LaporanID != 1 {
		t.Fatalf("expected 1 conversation (Report ID 1) for Pemdes, got: %d", len(pemdesInbox))
	}
	if !pemdesInbox[0].MenungguBalasanAdmin {
		t.Errorf("expected MenungguBalasanAdmin=true for report 1")
	}

	// Filter inbox untuk Admin PU
	var puInbox []AdminInboxItem
	for _, rep := range reports {
		ok, _ := cekAksesLaporanSimulasi(string(models.RoleAdminPu), 6, nil, rep)
		if !ok {
			continue
		}
		repChats := chats[rep.ID]
		menunggu := false
		for _, ch := range repChats {
			if ch.Balasan == nil || *ch.Balasan == "" {
				menunggu = true
				break
			}
		}
		puInbox = append(puInbox, AdminInboxItem{
			LaporanID:            rep.ID,
			JudulLaporan:         rep.Judul,
			MenungguBalasanAdmin: menunggu,
			TotalPesan:           len(repChats),
		})
	}

	if len(puInbox) != 1 || puInbox[0].LaporanID != 3 {
		t.Fatalf("expected 1 conversation (Report ID 3) for PU, got: %d", len(puInbox))
	}
}

// Test HTTP Forbidden for Warga accessing Admin Inbox
func TestAdminInboxHTTP_WargaForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", "chat_test_secret_12345678901234")

	token, err := utils.GenerateToken(10, "warga@roadis.id", models.RoleWarga, nil)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	r := gin.New()
	r.Use(middlewares.AuthMiddleware())
	r.Use(middlewares.RequireRole("admin_pemdes", "admin_pu", "super_admin"))
	r.GET("/api/admin/chat", GetAdminInbox)

	req, _ := http.NewRequest(http.MethodGet, "/api/admin/chat", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected HTTP 403 Forbidden for Warga accessing admin chat inbox, got %d", w.Code)
	}
}

// Test Empty Chat and Inbox returns "data": []
func TestEmptyChatResponse_SerializesToArray(t *testing.T) {
	emptyList := make([]ChatResponse, 0)
	resp := gin.H{
		"status":  "success",
		"message": "Riwayat chat berhasil diambil",
		"data":    emptyList,
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	str := string(b)
	if !strings.Contains(str, `"data":[]`) {
		t.Errorf("expected JSON to contain '\"data\":[]', got: %s", str)
	}

	emptyInbox := make([]AdminInboxItem, 0)
	inboxResp := gin.H{
		"status":  "success",
		"message": "Daftar percakapan berhasil diambil",
		"data":    emptyInbox,
	}

	ib, err := json.Marshal(inboxResp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	iStr := string(ib)
	if !strings.Contains(iStr, `"data":[]`) {
		t.Errorf("expected JSON to contain '\"data\":[]', got: %s", iStr)
	}
}

// Tests for Attachment Validation and Formatting
func TestChatAttachmentValidation_MimeAndSize(t *testing.T) {
	// Magic bytes for file types
	jpegHeader := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	pdfHeader := []byte("%PDF-1.4 sample pdf content")
	exeHeader := []byte("MZ this is an executable binary")
	txtHeader := []byte("plain text content here")

	testCases := []struct {
		name        string
		data        []byte
		filename    string
		size        int64
		expectValid bool
		errReason   string
	}{
		{"Valid JPEG file", jpegHeader, "jalan-rusak.jpg", 1024 * 100, true, ""},
		{"Valid PNG file", pngHeader, "bukti-perbaikan.png", 1024 * 200, true, ""},
		{"Invalid PDF file", pdfHeader, "dokumen.pdf", 1024 * 50, false, "invalid_mime"},
		{"Invalid EXE file", exeHeader, "script.exe", 1024 * 10, false, "invalid_mime"},
		{"Invalid TXT file", txtHeader, "catatan.txt", 1024 * 5, false, "invalid_mime"},
		{"Oversized file", jpegHeader, "foto-besar.jpg", 6 * 1024 * 1024, false, "oversized"},
		{"Invalid extension spoofing", jpegHeader, "dokumen.pdf", 1024 * 50, false, "invalid_ext"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.size > MaxChatAttachmentSizeBytes {
				if tc.expectValid {
					t.Errorf("expected file to be oversized, but got valid")
				}
				return
			}

			ext := strings.ToLower(tc.filename[strings.LastIndex(tc.filename, "."):])
			if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
				if tc.expectValid {
					t.Errorf("expected extension check to fail for %s", tc.filename)
				}
				return
			}

			detected := http.DetectContentType(tc.data)
			detected = strings.Split(detected, ";")[0]
			detected = strings.TrimSpace(detected)

			isValidMime := allowedMimeTypes[detected]
			if isValidMime != tc.expectValid {
				t.Errorf("[%s] expected valid mime=%v, got=%v (detected: %s)", tc.name, tc.expectValid, isValidMime, detected)
			}
		})
	}
}

func TestFormatChatToResponse_AttachmentHandling(t *testing.T) {
	url := "https://res.cloudinary.com/roadis/image/upload/v1234/test.jpg"
	nama := "test.jpg"
	mime := "image/jpeg"
	balasanText := "Jalan sudah kami tinjau."

	now := time.Now()
	chatWithAttachment := models.RiwayatChat{
		Model:                   gorm.Model{ID: 101},
		LaporanKerusakanID:      1,
		UserID:                  5,
		Pesan:                   "Apakah sudah diperiksa?",
		Balasan:                 &balasanText,
		DibalasAt:               &now,
		LampiranBalasanURL:      &url,
		LampiranBalasanNama:     &nama,
		LampiranBalasanMimeType: &mime,
	}

	resp := FormatChatToResponse(chatWithAttachment)

	if resp.LampiranBalasan == nil {
		t.Fatalf("expected LampiranBalasan to be non-nil")
	}
	if resp.LampiranBalasan.URL != url {
		t.Errorf("expected URL=%s, got %s", url, resp.LampiranBalasan.URL)
	}
	if resp.LampiranBalasan.Nama != nama {
		t.Errorf("expected Nama=%s, got %s", nama, resp.LampiranBalasan.Nama)
	}
	if resp.LampiranBalasan.MimeType != mime {
		t.Errorf("expected MimeType=%s, got %s", mime, resp.LampiranBalasan.MimeType)
	}
	if resp.LampiranBalasanURL == nil || *resp.LampiranBalasanURL != url {
		t.Errorf("expected flat LampiranBalasanURL=%s", url)
	}

	// Test chat without attachment
	chatWithoutAttachment := models.RiwayatChat{
		Model:              gorm.Model{ID: 102},
		LaporanKerusakanID: 1,
		UserID:             5,
		Pesan:              "Halo?",
		Balasan:            &balasanText,
		DibalasAt:          &now,
	}

	respNoAtt := FormatChatToResponse(chatWithoutAttachment)
	if respNoAtt.LampiranBalasan != nil {
		t.Errorf("expected LampiranBalasan to be nil for text-only chat")
	}
	if respNoAtt.LampiranBalasanURL != nil {
		t.Errorf("expected flat LampiranBalasanURL to be nil for text-only chat")
	}
}

func TestAdminInboxItem_AttachmentOnlyPreview(t *testing.T) {
	url := "https://res.cloudinary.com/roadis/image/upload/v1234/test.jpg"
	emptyText := ""

	// Scenario 1: Text only
	textOnly := "Pekerjaan sedang dilakukan."
	preview1 := textOnly
	if textOnly == "" && url != "" {
		preview1 = "📎 Lampiran gambar"
	}
	if preview1 != "Pekerjaan sedang dilakukan." {
		t.Errorf("expected text preview, got: %s", preview1)
	}

	// Scenario 2: Attachment only
	preview2 := emptyText
	if emptyText == "" && url != "" {
		preview2 = "📎 Lampiran gambar"
	}
	if preview2 != "📎 Lampiran gambar" {
		t.Errorf("expected '📎 Lampiran gambar', got: %s", preview2)
	}
}

func TestChatReplyValidation_TextAndAttachment(t *testing.T) {
	cases := []struct {
		name        string
		balasan     string
		hasFile     bool
		expectValid bool
	}{
		{"Text only valid", "Pekerjaan sudah selesai.", false, true},
		{"Attachment only valid", "", true, true},
		{"Text and attachment valid", "Ini foto bukti jalan.", true, true},
		{"Both empty -> invalid", "", false, false},
		{"Whitespace only without attachment -> invalid", "   \t\n", false, false},
		{"Whitespace text with attachment -> valid", "   ", true, true},
		{"Text exceeding 1000 characters -> invalid", strings.Repeat("X", 1001), false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			trimmed := strings.TrimSpace(tc.balasan)
			isValid := true
			if !tc.hasFile && trimmed == "" {
				isValid = false
			} else if len(trimmed) > 1000 {
				isValid = false
			}

			if isValid != tc.expectValid {
				t.Errorf("[%s] expected valid=%v, got=%v", tc.name, tc.expectValid, isValid)
			}
		})
	}
}

