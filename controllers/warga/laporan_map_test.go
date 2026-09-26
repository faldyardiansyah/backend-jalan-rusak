package warga

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// filterWargaMapSimulasi mensimulasikan query dan filter koordinat pada GetAllLaporanPeta
func filterWargaMapSimulasi(laporanList []models.LaporanKerusakan) []LaporanResponse {
	responseData := make([]LaporanResponse, 0)

	for _, lap := range laporanList {
		// Soft deleted tidak boleh muncul
		if lap.DeletedAt.Valid {
			continue
		}

		// Koordinat invalid tidak boleh muncul
		if !utils.IsValidCoordinate(lap.Latitude, lap.Longitude) {
			continue
		}

		responseData = append(responseData, FormatLaporanToResponse(lap))
	}

	return responseData
}

func TestWargaMap_ValidAndInvalidFiltering(t *testing.T) {
	laporanList := []models.LaporanKerusakan{
		// 1. Laporan valid -> HARUS MUNCUL
		{
			Model:     gorm.Model{ID: 1},
			Judul:     "Jalan Rusak Desa",
			Latitude:  -6.3265,
			Longitude: 108.3241,
			Status:    "menunggu",
		},
		// 2. Laporan valid kabupaten -> HARUS MUNCUL (Warga melihat peta publik)
		{
			Model:     gorm.Model{ID: 2},
			Judul:     "Jalan Rusak Kabupaten",
			Latitude:  -6.3300,
			Longitude: 108.3300,
			Status:    "proses",
		},
		// 3. Laporan soft-deleted -> TIDAK BOLEH MUNCUL
		{
			Model:     gorm.Model{ID: 3, DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}},
			Judul:     "Laporan Terhapus",
			Latitude:  -6.3265,
			Longitude: 108.3241,
			Status:    "selesai",
		},
		// 4. Laporan dengan Latitude out of range (95.0) -> TIDAK BOLEH MUNCUL
		{
			Model:     gorm.Model{ID: 4},
			Judul:     "Laporan Invalid Lat",
			Latitude:  95.0,
			Longitude: 108.3241,
			Status:    "menunggu",
		},
		// 5. Laporan dengan Longitude out of range (-190.0) -> TIDAK BOLEH MUNCUL
		{
			Model:     gorm.Model{ID: 5},
			Judul:     "Laporan Invalid Lng",
			Latitude:  -6.3265,
			Longitude: -190.0,
			Status:    "menunggu",
		},
		// 6. Laporan dengan Latitude NaN -> TIDAK BOLEH MUNCUL
		{
			Model:     gorm.Model{ID: 6},
			Judul:     "Laporan NaN",
			Latitude:  math.NaN(),
			Longitude: 108.3241,
			Status:    "menunggu",
		},
		// 7. Laporan dengan Longitude Inf -> TIDAK BOLEH MUNCUL
		{
			Model:     gorm.Model{ID: 7},
			Judul:     "Laporan Inf",
			Latitude:  -6.3265,
			Longitude: math.Inf(1),
			Status:    "menunggu",
		},
	}

	result := filterWargaMapSimulasi(laporanList)

	if len(result) != 2 {
		t.Fatalf("expected 2 visible reports for Warga Map, got %d", len(result))
	}

	if result[0].ID != 1 || result[1].ID != 2 {
		t.Errorf("expected IDs 1 and 2, got %d and %d", result[0].ID, result[1].ID)
	}
}

func TestWargaMap_EmptyDatabaseReturnsEmptyArray(t *testing.T) {
	emptyList := []models.LaporanKerusakan{}
	result := filterWargaMapSimulasi(emptyList)

	resp := gin.H{
		"status":  "success",
		"message": "Semua laporan berhasil diambil",
		"data":    result,
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	jsonStr := string(b)
	if !strings.Contains(jsonStr, `"data":[]`) {
		t.Errorf("expected '\"data\":[]' in response, got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"status":"success"`) {
		t.Errorf("expected '\"status\":\"success\"', got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"message":"Semua laporan berhasil diambil"`) {
		t.Errorf("expected message, got: %s", jsonStr)
	}

	// Pastikan tidak ada field AI palsu pada LaporanResponse
	forbiddenFields := []string{"severity", "severity_score", "confidence", "priority", "model_version"}
	for _, field := range forbiddenFields {
		if strings.Contains(jsonStr, `"`+field+`"`) {
			t.Errorf("found forbidden AI field %q in Warga Map JSON: %s", field, jsonStr)
		}
	}
}
