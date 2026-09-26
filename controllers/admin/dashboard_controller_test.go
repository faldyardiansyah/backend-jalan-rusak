package admin

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

// filterLaporanSimulasi memvalidasi filtering scope dashboard sesuai logika GetDashboardStats
func filterLaporanSimulasi(
	role string,
	adminWilayahID *uint,
	lap models.LaporanKerusakan,
) (bool, string) {
	// Laporan soft-deleted tidak boleh dihitung
	if lap.DeletedAt.Valid {
		return false, "deleted"
	}

	jenisJalan := strings.ToLower(lap.JenisJalan)

	switch role {
	case string(models.RoleAdminPemdes):
		if adminWilayahID == nil {
			return false, "unassigned_wilayah" // Fail-closed: 403 Forbidden
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

func hitungStatistikSimulasi(
	role string,
	adminWilayahID *uint,
	laporanList []models.LaporanKerusakan,
) (map[string]int64, error) {
	stats := map[string]int64{
		"total_laporan":  0,
		"total_menunggu": 0,
		"total_proses":   0,
		"total_selesai":  0,
		"total_ditolak":  0,
	}

	for _, lap := range laporanList {
		match, reason := filterLaporanSimulasi(role, adminWilayahID, lap)
		if reason == "unassigned_wilayah" || reason == "forbidden" {
			return nil, http.ErrAbortHandler
		}
		if match {
			stats["total_laporan"]++
			switch strings.ToLower(lap.Status) {
			case "menunggu":
				stats["total_menunggu"]++
			case "proses":
				stats["total_proses"]++
			case "selesai":
				stats["total_selesai"]++
			case "ditolak":
				stats["total_ditolak"]++
			}
		}
	}

	return stats, nil
}

func TestDashboardScope_AdminPemdes(t *testing.T) {
	wilayahA := uint(1)
	wilayahB := uint(2)

	laporanList := []models.LaporanKerusakan{
		// 1. Desa Wilayah A (harus dihitung)
		{WilayahID: wilayahA, JenisJalan: "desa", Status: "menunggu"},
		// 2. Desa Wilayah A (harus dihitung)
		{WilayahID: wilayahA, JenisJalan: "desa", Status: "proses"},
		// 3. Desa Wilayah B (wilayah lain -> TIDAK boleh dihitung)
		{WilayahID: wilayahB, JenisJalan: "desa", Status: "menunggu"},
		// 4. Kabupaten di Wilayah A (bukan desa -> TIDAK boleh dihitung)
		{WilayahID: wilayahA, JenisJalan: "kabupaten", Status: "menunggu"},
		// 5. Provinsi di Wilayah A (bukan desa -> TIDAK boleh dihitung)
		{WilayahID: wilayahA, JenisJalan: "provinsi", Status: "selesai"},
		// 6. Nasional di Wilayah A (bukan desa -> TIDAK boleh dihitung)
		{WilayahID: wilayahA, JenisJalan: "nasional", Status: "proses"},
		// 7. Desa Wilayah A tapi soft-deleted (TIDAK boleh dihitung)
		{WilayahID: wilayahA, JenisJalan: "desa", Status: "selesai", Model: gorm.Model{DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}}},
	}

	stats, err := hitungStatistikSimulasi(string(models.RoleAdminPemdes), &wilayahA, laporanList)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats["total_laporan"] != 2 {
		t.Errorf("expected total_laporan 2, got %d", stats["total_laporan"])
	}
	if stats["total_menunggu"] != 1 {
		t.Errorf("expected total_menunggu 1, got %d", stats["total_menunggu"])
	}
	if stats["total_proses"] != 1 {
		t.Errorf("expected total_proses 1, got %d", stats["total_proses"])
	}
	if stats["total_selesai"] != 0 {
		t.Errorf("expected total_selesai 0, got %d", stats["total_selesai"])
	}
}

func TestDashboardScope_AdminPU(t *testing.T) {
	wilayahA := uint(1)
	wilayahB := uint(2)

	laporanList := []models.LaporanKerusakan{
		// 1. Kabupaten Wilayah A (harus dihitung)
		{WilayahID: wilayahA, JenisJalan: "kabupaten", Status: "menunggu"},
		// 2. Kabupaten Wilayah B (harus dihitung, PU lintas wilayah)
		{WilayahID: wilayahB, JenisJalan: "kabupaten", Status: "selesai"},
		// 3. Desa Wilayah A (bukan kabupaten -> TIDAK dihitung)
		{WilayahID: wilayahA, JenisJalan: "desa", Status: "menunggu"},
		// 4. Provinsi Wilayah B (bukan kabupaten -> TIDAK dihitung)
		{WilayahID: wilayahB, JenisJalan: "provinsi", Status: "proses"},
		// 5. Nasional Wilayah A (bukan kabupaten -> TIDAK dihitung)
		{WilayahID: wilayahA, JenisJalan: "nasional", Status: "proses"},
		// 6. Kabupaten soft-deleted (TIDAK dihitung)
		{WilayahID: wilayahA, JenisJalan: "kabupaten", Status: "menunggu", Model: gorm.Model{DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}}},
	}

	stats, err := hitungStatistikSimulasi(string(models.RoleAdminPu), nil, laporanList)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats["total_laporan"] != 2 {
		t.Errorf("expected total_laporan 2, got %d", stats["total_laporan"])
	}
	if stats["total_menunggu"] != 1 {
		t.Errorf("expected total_menunggu 1, got %d", stats["total_menunggu"])
	}
	if stats["total_selesai"] != 1 {
		t.Errorf("expected total_selesai 1, got %d", stats["total_selesai"])
	}
	if stats["total_proses"] != 0 {
		t.Errorf("expected total_proses 0, got %d", stats["total_proses"])
	}
}

func TestDashboardScope_Superadmin(t *testing.T) {
	wilayahA := uint(1)
	wilayahB := uint(2)

	laporanList := []models.LaporanKerusakan{
		{WilayahID: wilayahA, JenisJalan: "desa", Status: "menunggu"},
		{WilayahID: wilayahB, JenisJalan: "desa", Status: "proses"},
		{WilayahID: wilayahA, JenisJalan: "kabupaten", Status: "selesai"},
		{WilayahID: wilayahB, JenisJalan: "provinsi", Status: "ditolak"},
		{WilayahID: wilayahA, JenisJalan: "nasional", Status: "proses"},
		// soft-deleted (tetap tidak dihitung)
		{WilayahID: wilayahA, JenisJalan: "desa", Status: "selesai", Model: gorm.Model{DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}}},
	}

	stats, err := hitungStatistikSimulasi(string(models.RoleSuperAdmin), nil, laporanList)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats["total_laporan"] != 5 {
		t.Errorf("expected total_laporan 5, got %d", stats["total_laporan"])
	}
	if stats["total_menunggu"] != 1 {
		t.Errorf("expected total_menunggu 1, got %d", stats["total_menunggu"])
	}
	if stats["total_proses"] != 2 {
		t.Errorf("expected total_proses 2, got %d", stats["total_proses"])
	}
	if stats["total_selesai"] != 1 {
		t.Errorf("expected total_selesai 1, got %d", stats["total_selesai"])
	}
	if stats["total_ditolak"] != 1 {
		t.Errorf("expected total_ditolak 1, got %d", stats["total_ditolak"])
	}
}

func TestDashboardScope_AdminPemdesTanpaWilayah(t *testing.T) {
	// Fail-closed: Admin Pemdes tanpa wilayah_id tidak boleh menghitung data apa pun
	laporanList := []models.LaporanKerusakan{
		{WilayahID: 1, JenisJalan: "desa", Status: "menunggu"},
	}

	_, err := hitungStatistikSimulasi(string(models.RoleAdminPemdes), nil, laporanList)
	if err == nil {
		t.Fatal("expected error for admin pemdes without wilayah, got nil")
	}
}

func TestDashboardScope_EmptyDatabase(t *testing.T) {
	laporanList := []models.LaporanKerusakan{}
	wilayahA := uint(1)

	stats, err := hitungStatistikSimulasi(string(models.RoleAdminPemdes), &wilayahA, laporanList)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats["total_laporan"] != 0 || stats["total_menunggu"] != 0 || stats["total_proses"] != 0 || stats["total_selesai"] != 0 || stats["total_ditolak"] != 0 {
		t.Errorf("expected all 0 stats, got %v", stats)
	}
}

func TestDashboardHTTP_WargaForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", "dashboard_test_secret_1234567890")

	// Token role warga
	token, err := utils.GenerateToken(10, "warga@roadis.id", models.RoleWarga, nil)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	r := gin.New()
	r.Use(middlewares.AuthMiddleware())
	r.Use(middlewares.RequireRole("admin_pemdes", "admin_pu", "super_admin"))
	r.GET("/api/admin/dashboard", GetDashboardStats)

	req, _ := http.NewRequest(http.MethodGet, "/api/admin/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected HTTP 403 Forbidden for Warga accessing dashboard, got %d", w.Code)
	}
}

func TestDashboardResponseContract_JSONFormat(t *testing.T) {
	type DashboardData struct {
		TotalLaporan  int64 `json:"total_laporan"`
		TotalMenunggu int64 `json:"total_menunggu"`
		TotalProses   int64 `json:"total_proses"`
		TotalSelesai  int64 `json:"total_selesai"`
		TotalDitolak  int64 `json:"total_ditolak"`
	}

	type ResponseWrapper struct {
		Status  string        `json:"status"`
		Message string        `json:"message"`
		Data    DashboardData `json:"data"`
	}

	resp := ResponseWrapper{
		Status:  "success",
		Message: "Dashboard stats berhasil diambil",
		Data: DashboardData{
			TotalLaporan:  10,
			TotalMenunggu: 3,
			TotalProses:   4,
			TotalSelesai:  2,
			TotalDitolak:  1,
		},
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	str := string(b)
	expectedKeys := []string{
		`"status":"success"`,
		`"message":"Dashboard stats berhasil diambil"`,
		`"total_laporan":10`,
		`"total_menunggu":3`,
		`"total_proses":4`,
		`"total_selesai":2`,
		`"total_ditolak":1`,
	}

	for _, key := range expectedKeys {
		if !strings.Contains(str, key) {
			t.Errorf("expected JSON to contain %q, got: %s", key, str)
		}
	}
}
