package admin

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"backend-jalan-rusak/models"
)

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
