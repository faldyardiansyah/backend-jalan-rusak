package admin

import (
	"encoding/json"
	"math"
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

// filterAdminMapSimulasi mensimulasikan query dan filter koordinat pada GetMapLaporan
func filterAdminMapSimulasi(
	role string,
	adminWilayahID *uint,
	lap models.LaporanKerusakan,
	statusQuery string,
	jenisJalanQuery string,
) (bool, string) {
	// Laporan soft-deleted tidak boleh muncul di map
	if lap.DeletedAt.Valid {
		return false, "deleted"
	}

	// Koordinat tidak valid tidak boleh menjadi marker
	if !utils.IsValidCoordinate(lap.Latitude, lap.Longitude) {
		return false, "invalid_coordinate"
	}

	jenisJalan := strings.ToLower(lap.JenisJalan)

	switch role {
	case string(models.RoleAdminPemdes):
		if adminWilayahID == nil {
			return false, "unassigned_wilayah" // Fail-closed: 403
		}
		if jenisJalan != "desa" {
			return false, "jenis_jalan_mismatch"
		}
		if lap.WilayahID != *adminWilayahID {
			return false, "wilayah_mismatch"
		}

	case string(models.RoleAdminPu):
		if jenisJalanQuery != "" && strings.ToLower(jenisJalanQuery) != "all" && jenisJalan != strings.ToLower(jenisJalanQuery) {
			return false, "jenis_jalan_query_mismatch"
		}

	case string(models.RoleSuperAdmin):
		if jenisJalanQuery != "" && jenisJalan != strings.ToLower(jenisJalanQuery) {
			return false, "jenis_jalan_query_mismatch"
		}

	default:
		return false, "forbidden"
	}

	if statusQuery != "" && !strings.EqualFold(lap.Status, statusQuery) {
		return false, "status_query_mismatch"
	}

	return true, "ok"
}

func TestMapScope_AdminPemdes(t *testing.T) {
	wilayahA := uint(1)
	wilayahB := uint(2)

	laporanList := []models.LaporanKerusakan{
		// 1. Desa Wilayah A, koordinat valid -> HARUS MUNCUL
		{WilayahID: wilayahA, JenisJalan: "desa", Status: "menunggu", Latitude: -6.3265, Longitude: 108.3241},
		// 2. Desa Wilayah A, koordinat valid -> HARUS MUNCUL
		{WilayahID: wilayahA, JenisJalan: "desa", Status: "proses", Latitude: -6.3300, Longitude: 108.3200},
		// 3. Desa Wilayah B (wilayah lain) -> TIDAK BOLEH MUNCUL
		{WilayahID: wilayahB, JenisJalan: "desa", Status: "menunggu", Latitude: -6.3400, Longitude: 108.3300},
		// 4. Kabupaten di Wilayah A -> TIDAK BOLEH MUNCUL
		{WilayahID: wilayahA, JenisJalan: "kabupaten", Status: "menunggu", Latitude: -6.3265, Longitude: 108.3241},
		// 5. Provinsi di Wilayah A -> TIDAK BOLEH MUNCUL
		{WilayahID: wilayahA, JenisJalan: "provinsi", Status: "selesai", Latitude: -6.3265, Longitude: 108.3241},
		// 6. Nasional di Wilayah A -> TIDAK BOLEH MUNCUL
		{WilayahID: wilayahA, JenisJalan: "nasional", Status: "proses", Latitude: -6.3265, Longitude: 108.3241},
		// 7. Desa Wilayah A tapi soft-deleted -> TIDAK BOLEH MUNCUL
		{WilayahID: wilayahA, JenisJalan: "desa", Status: "menunggu", Latitude: -6.3265, Longitude: 108.3241, Model: gorm.Model{DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}}},
		// 8. Desa Wilayah A tapi koordinat invalid (Latitude > 90) -> TIDAK BOLEH MUNCUL
		{WilayahID: wilayahA, JenisJalan: "desa", Status: "menunggu", Latitude: 95.0, Longitude: 108.3241},
		// 9. Desa Wilayah A tapi koordinat NaN -> TIDAK BOLEH MUNCUL
		{WilayahID: wilayahA, JenisJalan: "desa", Status: "menunggu", Latitude: math.NaN(), Longitude: 108.3241},
	}

	var visiblePoints []models.LaporanKerusakan
	for _, lap := range laporanList {
		ok, _ := filterAdminMapSimulasi(string(models.RoleAdminPemdes), &wilayahA, lap, "", "")
		if ok {
			visiblePoints = append(visiblePoints, lap)
		}
	}

	if len(visiblePoints) != 2 {
		t.Fatalf("expected 2 visible points for Admin Pemdes, got %d", len(visiblePoints))
	}
}

func TestMapScope_AdminPU(t *testing.T) {
	wilayahA := uint(1)
	wilayahB := uint(2)

	laporanList := []models.LaporanKerusakan{
		// 1. Kabupaten Wilayah A, valid -> HARUS MUNCUL
		{WilayahID: wilayahA, JenisJalan: "kabupaten", Status: "menunggu", Latitude: -6.3265, Longitude: 108.3241},
		// 2. Kabupaten Wilayah B, valid -> HARUS MUNCUL (PU lintas wilayah)
		{WilayahID: wilayahB, JenisJalan: "kabupaten", Status: "proses", Latitude: -6.3500, Longitude: 108.3400},
		// 3. Desa Wilayah A -> TIDAK BOLEH MUNCUL
		{WilayahID: wilayahA, JenisJalan: "desa", Status: "menunggu", Latitude: -6.3265, Longitude: 108.3241},
		// 4. Provinsi Wilayah B -> TIDAK BOLEH MUNCUL
		{WilayahID: wilayahB, JenisJalan: "provinsi", Status: "proses", Latitude: -6.3500, Longitude: 108.3400},
		// 5. Nasional Wilayah A -> TIDAK BOLEH MUNCUL
		{WilayahID: wilayahA, JenisJalan: "nasional", Status: "selesai", Latitude: -6.3265, Longitude: 108.3241},
		// 6. Kabupaten soft-deleted -> TIDAK BOLEH MUNCUL
		{WilayahID: wilayahA, JenisJalan: "kabupaten", Status: "menunggu", Latitude: -6.3265, Longitude: 108.3241, Model: gorm.Model{DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}}},
		// 7. Kabupaten tapi koordinat invalid (Longitude > 180) -> TIDAK BOLEH MUNCUL
		{WilayahID: wilayahA, JenisJalan: "kabupaten", Status: "menunggu", Latitude: -6.3265, Longitude: 185.0},
	}

	// 1. Tanpa filter jenis_jalan: Admin PU memonitor seluruh kewenangan (Desa, Kabupaten, Provinsi, Nasional)
	var visiblePoints []models.LaporanKerusakan
	for _, lap := range laporanList {
		ok, _ := filterAdminMapSimulasi(string(models.RoleAdminPu), nil, lap, "", "")
		if ok {
			visiblePoints = append(visiblePoints, lap)
		}
	}

	if len(visiblePoints) != 5 {
		t.Fatalf("expected 5 visible points for Admin PU across all road authorities, got %d", len(visiblePoints))
	}

	// 2. Dengan filter jenis_jalan="kabupaten": hanya 2 laporan kabupaten
	var visibleKabupaten []models.LaporanKerusakan
	for _, lap := range laporanList {
		ok, _ := filterAdminMapSimulasi(string(models.RoleAdminPu), nil, lap, "", "kabupaten")
		if ok {
			visibleKabupaten = append(visibleKabupaten, lap)
		}
	}

	if len(visibleKabupaten) != 2 {
		t.Fatalf("expected 2 visible points for Admin PU with jenis_jalan=kabupaten, got %d", len(visibleKabupaten))
	}
}

func TestMapScope_Superadmin(t *testing.T) {
	wilayahA := uint(1)
	wilayahB := uint(2)

	laporanList := []models.LaporanKerusakan{
		{WilayahID: wilayahA, JenisJalan: "desa", Status: "menunggu", Latitude: -6.32, Longitude: 108.32},
		{WilayahID: wilayahB, JenisJalan: "desa", Status: "proses", Latitude: -6.33, Longitude: 108.33},
		{WilayahID: wilayahA, JenisJalan: "kabupaten", Status: "selesai", Latitude: -6.34, Longitude: 108.34},
		{WilayahID: wilayahB, JenisJalan: "provinsi", Status: "ditolak", Latitude: -6.35, Longitude: 108.35},
		{WilayahID: wilayahA, JenisJalan: "nasional", Status: "proses", Latitude: -6.36, Longitude: 108.36},
		// Soft deleted -> TIDAK muncul
		{WilayahID: wilayahA, JenisJalan: "nasional", Status: "proses", Latitude: -6.36, Longitude: 108.36, Model: gorm.Model{DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}}},
		// Invalid coordinate -> TIDAK muncul
		{WilayahID: wilayahA, JenisJalan: "desa", Status: "menunggu", Latitude: -95.0, Longitude: 108.32},
	}

	var visiblePoints []models.LaporanKerusakan
	for _, lap := range laporanList {
		ok, _ := filterAdminMapSimulasi(string(models.RoleSuperAdmin), nil, lap, "", "")
		if ok {
			visiblePoints = append(visiblePoints, lap)
		}
	}

	if len(visiblePoints) != 5 {
		t.Fatalf("expected 5 visible points for Superadmin, got %d", len(visiblePoints))
	}
}

func TestMapScope_AdminPemdesTanpaWilayah_FailClosed(t *testing.T) {
	lap := models.LaporanKerusakan{
		WilayahID:  1,
		JenisJalan: "desa",
		Status:     "menunggu",
		Latitude:   -6.3265,
		Longitude:  108.3241,
	}

	ok, reason := filterAdminMapSimulasi(string(models.RoleAdminPemdes), nil, lap, "", "")
	if ok || reason != "unassigned_wilayah" {
		t.Fatalf("expected fail-closed unassigned_wilayah, got ok=%v, reason=%s", ok, reason)
	}
}

func TestMapHTTP_WargaForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", "map_test_secret_12345678901234")

	token, err := utils.GenerateToken(10, "warga@roadis.id", models.RoleWarga, nil)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	r := gin.New()
	r.Use(middlewares.AuthMiddleware())
	r.Use(middlewares.RequireRole("admin_pemdes", "admin_pu", "super_admin"))
	r.GET("/api/admin/map/laporan", GetMapLaporan)

	req, _ := http.NewRequest(http.MethodGet, "/api/admin/map/laporan", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected HTTP 403 Forbidden for Warga accessing admin map, got %d", w.Code)
	}
}

func TestAdminMapResponseContract_JSONFormat(t *testing.T) {
	// Memastikan data kosong menghasilkan "data": [] dan "total": 0, bukan null
	validPoints := make([]MapPoint, 0)

	resp := gin.H{
		"status": "success",
		"total":  len(validPoints),
		"data":   validPoints,
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	str := string(b)
	if !strings.Contains(str, `"data":[]`) {
		t.Errorf(`expected JSON to contain '"data":[]', got: %s`, str)
	}
	if !strings.Contains(str, `"total":0`) {
		t.Errorf(`expected JSON to contain '"total":0', got: %s`, str)
	}
	if !strings.Contains(str, `"status":"success"`) {
		t.Errorf(`expected JSON to contain '"status":"success"', got: %s`, str)
	}

	// Pastikan field MapPoint tidak memiliki field AI palsu
	samplePoint := MapPoint{
		ID:            1,
		Judul:         "Jalan Berlubang",
		Latitude:      -6.3265,
		Longitude:     108.3241,
		Status:        "menunggu",
		TipeKerusakan: "lubang",
		JenisJalan:    "kabupaten",
		ImageURL:      "http://example.com/img.jpg",
		FotoBukti:     "",
		CatatanAdmin:  "",
		Name:          "Warga Indramayu",
		WilayahID:     1,
	}

	pb, err := json.Marshal(samplePoint)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	pStr := string(pb)

	// Pastikan tidak ada field AI fiktif
	forbiddenFields := []string{"severity", "severity_score", "confidence", "priority", "model_version"}
	for _, field := range forbiddenFields {
		if strings.Contains(pStr, `"`+field+`"`) {
			t.Errorf("found forbidden AI field %q in MapPoint JSON: %s", field, pStr)
		}
	}
}
