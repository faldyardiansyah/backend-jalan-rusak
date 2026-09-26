package admin

import (
	"encoding/json"
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
