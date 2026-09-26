package warga

import (
	"encoding/json"
	"math"
	"strconv"
	"testing"
)

func validateCoordinate(latStr, lngStr string) (float64, float64, string) {
	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil || math.IsNaN(lat) || math.IsInf(lat, 0) || lat < -90 || lat > 90 {
		return 0, 0, "Latitude tidak valid (harus berada di antara -90 dan 90)"
	}

	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil || math.IsNaN(lng) || math.IsInf(lng, 0) || lng < -180 || lng > 180 {
		return 0, 0, "Longitude tidak valid (harus berada di antara -180 dan 180)"
	}

	return lat, lng, ""
}

func TestCoordinateValidation(t *testing.T) {
	testCases := []struct {
		name        string
		latStr      string
		lngStr      string
		expectError bool
	}{
		{
			name:        "Valid Indramayu Coordinate",
			latStr:      "-6.3400",
			lngStr:      "108.3300",
			expectError: false,
		},
		{
			name:        "Valid Boundary Coordinate",
			latStr:      "90.0",
			lngStr:      "180.0",
			expectError: false,
		},
		{
			name:        "Valid Negative Boundary",
			latStr:      "-90.0",
			lngStr:      "-180.0",
			expectError: false,
		},
		{
			name:        "Latitude Too Low (< -90)",
			latStr:      "-90.0001",
			lngStr:      "108.3300",
			expectError: true,
		},
		{
			name:        "Latitude Too High (> 90)",
			latStr:      "90.0001",
			lngStr:      "108.3300",
			expectError: true,
		},
		{
			name:        "Longitude Too Low (< -180)",
			latStr:      "-6.3400",
			lngStr:      "-180.0001",
			expectError: true,
		},
		{
			name:        "Longitude Too High (> 180)",
			latStr:      "-6.3400",
			lngStr:      "180.0001",
			expectError: true,
		},
		{
			name:        "Non-numeric Latitude",
			latStr:      "invalid_lat",
			lngStr:      "108.3300",
			expectError: true,
		},
		{
			name:        "Non-numeric Longitude",
			latStr:      "-6.3400",
			lngStr:      "invalid_lng",
			expectError: true,
		},
		{
			name:        "NaN Coordinate",
			latStr:      "NaN",
			lngStr:      "108.3300",
			expectError: true,
		},
		{
			name:        "Infinity Coordinate",
			latStr:      "+Inf",
			lngStr:      "108.3300",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, errMsg := validateCoordinate(tc.latStr, tc.lngStr)
			if tc.expectError && errMsg == "" {
				t.Errorf("[%s] expected error, got nil", tc.name)
			}
			if !tc.expectError && errMsg != "" {
				t.Errorf("[%s] expected success, got error: %s", tc.name, errMsg)
			}
		})
	}
}

func TestEmptyLaporanResponse_SerializesToArray(t *testing.T) {
	// Memastikan respon kosong mengembalikan [] dan bukan null
	responseData := make([]LaporanResponse, 0)

	type ResponseWrapper struct {
		Status  string            `json:"status"`
		Message string            `json:"message"`
		Data    []LaporanResponse `json:"data"`
	}

	wrapped := ResponseWrapper{
		Status:  "success",
		Message: "Riwayat laporan berhasil diambil",
		Data:    responseData,
	}

	bytes, err := json.Marshal(wrapped)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	jsonStr := string(bytes)
	expectedSub := `"data":[]`
	if !containsString(jsonStr, expectedSub) {
		t.Errorf("expected JSON to contain %q, got %s", expectedSub, jsonStr)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
