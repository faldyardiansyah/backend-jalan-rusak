package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"backend-jalan-rusak/models"

	"github.com/gin-gonic/gin"
)

// filterLaporanListSimulasi mensimulasikan logic filtering GetAllLaporan
func filterLaporanListSimulasi(
	role string,
	adminWilayahID *uint,
	lap models.LaporanKerusakan,
	statusFilter string,
	jenisJalanFilter string,
	searchKeyword string,
) (bool, string) {
	if lap.DeletedAt.Valid {
		return false, "deleted"
	}

	jenisJalanLower := strings.ToLower(lap.JenisJalan)

	switch role {
	case string(models.RoleAdminPemdes):
		if adminWilayahID == nil {
			return false, "unassigned_wilayah"
		}
		if jenisJalanLower != "desa" {
			return false, "jenis_jalan_mismatch"
		}
		if lap.WilayahID != *adminWilayahID {
			return false, "wilayah_mismatch"
		}

	case string(models.RoleAdminPu):
		// PU-5.1: Admin PU dapat melihat semua kewenangan; jika difilter, sesuaikan
		if jenisJalanFilter != "" && strings.ToLower(jenisJalanFilter) != "all" {
			if jenisJalanLower != strings.ToLower(jenisJalanFilter) {
				return false, "jenis_jalan_filter_mismatch"
			}
		}

	case string(models.RoleSuperAdmin):
		if jenisJalanFilter != "" && strings.ToLower(jenisJalanFilter) != "all" {
			if jenisJalanLower != strings.ToLower(jenisJalanFilter) {
				return false, "jenis_jalan_filter_mismatch"
			}
		}

	default:
		return false, "forbidden"
	}

	if statusFilter != "" && !strings.EqualFold(lap.Status, statusFilter) {
		return false, "status_mismatch"
	}

	if searchKeyword != "" {
		k := strings.ToLower(searchKeyword)
		if !strings.Contains(strings.ToLower(lap.Judul), k) &&
			!strings.Contains(strings.ToLower(lap.Deskripsi), k) {
			return false, "search_mismatch"
		}
	}

	return true, "ok"
}

func TestLaporanListScope_AdminPU(t *testing.T) {
	wilayahA := uint(1)
	wilayahB := uint(2)

	laporanList := []models.LaporanKerusakan{
		{WilayahID: wilayahA, JenisJalan: "desa", Status: "menunggu", Judul: "Jalan Rusak Desa"},
		{WilayahID: wilayahA, JenisJalan: "kabupaten", Status: "proses", Judul: "Jalan Rusak Kabupaten"},
		{WilayahID: wilayahB, JenisJalan: "provinsi", Status: "selesai", Judul: "Jalan Rusak Provinsi"},
		{WilayahID: wilayahA, JenisJalan: "nasional", Status: "menunggu", Judul: "Jalan Rusak Nasional"},
	}

	// 1. Tanpa filter jenis_jalan: Admin PU melihat 4 laporan (SEMUA kewenangan)
	var allPU []models.LaporanKerusakan
	for _, lap := range laporanList {
		ok, _ := filterLaporanListSimulasi(string(models.RoleAdminPu), nil, lap, "", "", "")
		if ok {
			allPU = append(allPU, lap)
		}
	}
	if len(allPU) != 4 {
		t.Fatalf("expected 4 reports for Admin PU without jenis_jalan filter, got %d", len(allPU))
	}

	// 2. Filter jenis_jalan = "all": tetap 4 laporan
	var allParamPU []models.LaporanKerusakan
	for _, lap := range laporanList {
		ok, _ := filterLaporanListSimulasi(string(models.RoleAdminPu), nil, lap, "", "all", "")
		if ok {
			allParamPU = append(allParamPU, lap)
		}
	}
	if len(allParamPU) != 4 {
		t.Fatalf("expected 4 reports for Admin PU with jenis_jalan=all, got %d", len(allParamPU))
	}

	// 3. Filter jenis_jalan = "kabupaten": hanya 1 laporan
	var kabPU []models.LaporanKerusakan
	for _, lap := range laporanList {
		ok, _ := filterLaporanListSimulasi(string(models.RoleAdminPu), nil, lap, "", "kabupaten", "")
		if ok {
			kabPU = append(kabPU, lap)
		}
	}
	if len(kabPU) != 1 || kabPU[0].JenisJalan != "kabupaten" {
		t.Fatalf("expected 1 kabupaten report for Admin PU, got %d", len(kabPU))
	}

	// 4. Filter jenis_jalan = "desa": hanya 1 laporan
	var desaPU []models.LaporanKerusakan
	for _, lap := range laporanList {
		ok, _ := filterLaporanListSimulasi(string(models.RoleAdminPu), nil, lap, "", "desa", "")
		if ok {
			desaPU = append(desaPU, lap)
		}
	}
	if len(desaPU) != 1 || desaPU[0].JenisJalan != "desa" {
		t.Fatalf("expected 1 desa report for Admin PU, got %d", len(desaPU))
	}

	// 5. Filter jenis_jalan = "provinsi": hanya 1 laporan
	var provPU []models.LaporanKerusakan
	for _, lap := range laporanList {
		ok, _ := filterLaporanListSimulasi(string(models.RoleAdminPu), nil, lap, "", "provinsi", "")
		if ok {
			provPU = append(provPU, lap)
		}
	}
	if len(provPU) != 1 || provPU[0].JenisJalan != "provinsi" {
		t.Fatalf("expected 1 provinsi report for Admin PU, got %d", len(provPU))
	}

	// 6. Filter jenis_jalan = "nasional": hanya 1 laporan
	var nasPU []models.LaporanKerusakan
	for _, lap := range laporanList {
		ok, _ := filterLaporanListSimulasi(string(models.RoleAdminPu), nil, lap, "", "nasional", "")
		if ok {
			nasPU = append(nasPU, lap)
		}
	}
	if len(nasPU) != 1 || nasPU[0].JenisJalan != "nasional" {
		t.Fatalf("expected 1 nasional report for Admin PU, got %d", len(nasPU))
	}
}

func TestLaporanListScope_AdminPemdes_Regression(t *testing.T) {
	wilayahA := uint(1)
	wilayahB := uint(2)

	laporanList := []models.LaporanKerusakan{
		{WilayahID: wilayahA, JenisJalan: "desa", Status: "menunggu"},
		{WilayahID: wilayahB, JenisJalan: "desa", Status: "menunggu"},
		{WilayahID: wilayahA, JenisJalan: "kabupaten", Status: "menunggu"},
		{WilayahID: wilayahA, JenisJalan: "provinsi", Status: "menunggu"},
		{WilayahID: wilayahA, JenisJalan: "nasional", Status: "menunggu"},
	}

	var visiblePemdes []models.LaporanKerusakan
	for _, lap := range laporanList {
		ok, _ := filterLaporanListSimulasi(string(models.RoleAdminPemdes), &wilayahA, lap, "", "", "")
		if ok {
			visiblePemdes = append(visiblePemdes, lap)
		}
	}

	if len(visiblePemdes) != 1 {
		t.Fatalf("expected exactly 1 report for Admin Pemdes (only their village desa), got %d", len(visiblePemdes))
	}
}

func TestGetAllLaporan_ParameterValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queryString    string
		expectedCode   int
		expectedErrMsg string
	}{
		{
			name:           "Invalid start_date format",
			queryString:    "?start_date=24-06-2026",
			expectedCode:   http.StatusBadRequest,
			expectedErrMsg: "Format start_date tidak valid",
		},
		{
			name:           "Invalid end_date format",
			queryString:    "?end_date=2026/06/24",
			expectedCode:   http.StatusBadRequest,
			expectedErrMsg: "Format end_date tidak valid",
		},
		{
			name:           "start_date exceeds end_date",
			queryString:    "?start_date=2026-06-30&end_date=2026-06-01",
			expectedCode:   http.StatusBadRequest,
			expectedErrMsg: "start_date tidak boleh melebihi end_date",
		},
		{
			name:           "Invalid sort_by parameter",
			queryString:    "?sort_by=random_column",
			expectedCode:   http.StatusBadRequest,
			expectedErrMsg: "Parameter sort_by tidak valid",
		},
		{
			name:           "Invalid severity parameter",
			queryString:    "?severity=extreme_critical",
			expectedCode:   http.StatusBadRequest,
			expectedErrMsg: "Parameter severity tidak valid",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req := httptest.NewRequest(http.MethodGet, "/api/admin/laporan"+tc.queryString, nil)
			c.Request = req
			c.Set("role", "admin_pu")
			c.Set("user_id", uint(1))

			GetAllLaporan(c)

			if w.Code != tc.expectedCode {
				t.Fatalf("expected status %d, got %d. Body: %s", tc.expectedCode, w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.expectedErrMsg) {
				t.Fatalf("expected body to contain %q, got: %s", tc.expectedErrMsg, w.Body.String())
			}
		})
	}
}

func TestDeterministicSortingLogic(t *testing.T) {
	timeBase := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	timeLater := timeBase.Add(2 * time.Hour)

	lap1 := models.LaporanKerusakan{Judul: "Laporan 1"}
	lap1.ID = 1
	lap1.CreatedAt = timeBase

	lap2 := models.LaporanKerusakan{Judul: "Laporan 2 (same time)"}
	lap2.ID = 2
	lap2.CreatedAt = timeBase

	lap3 := models.LaporanKerusakan{Judul: "Laporan 3 (later)"}
	lap3.ID = 3
	lap3.CreatedAt = timeLater

	laporanList := []models.LaporanKerusakan{lap1, lap2, lap3}

	// 1. Newest sorting simulation: created_at DESC, id DESC
	// Expected order: ID 3, ID 2, ID 1
	sortNewest := make([]models.LaporanKerusakan, len(laporanList))
	copy(sortNewest, laporanList)
	for i := 0; i < len(sortNewest); i++ {
		for j := i + 1; j < len(sortNewest); j++ {
			if sortNewest[i].CreatedAt.Before(sortNewest[j].CreatedAt) ||
				(sortNewest[i].CreatedAt.Equal(sortNewest[j].CreatedAt) && sortNewest[i].ID < sortNewest[j].ID) {
				sortNewest[i], sortNewest[j] = sortNewest[j], sortNewest[i]
			}
		}
	}

	if sortNewest[0].ID != 3 || sortNewest[1].ID != 2 || sortNewest[2].ID != 1 {
		t.Fatalf("newest sorting incorrect: got IDs [%d, %d, %d]", sortNewest[0].ID, sortNewest[1].ID, sortNewest[2].ID)
	}

	// 2. Oldest sorting simulation: created_at ASC, id ASC
	// Expected order: ID 1, ID 2, ID 3
	sortOldest := make([]models.LaporanKerusakan, len(laporanList))
	copy(sortOldest, laporanList)
	for i := 0; i < len(sortOldest); i++ {
		for j := i + 1; j < len(sortOldest); j++ {
			if sortOldest[i].CreatedAt.After(sortOldest[j].CreatedAt) ||
				(sortOldest[i].CreatedAt.Equal(sortOldest[j].CreatedAt) && sortOldest[i].ID > sortOldest[j].ID) {
				sortOldest[i], sortOldest[j] = sortOldest[j], sortOldest[i]
			}
		}
	}

	if sortOldest[0].ID != 1 || sortOldest[1].ID != 2 || sortOldest[2].ID != 3 {
		t.Fatalf("oldest sorting incorrect: got IDs [%d, %d, %d]", sortOldest[0].ID, sortOldest[1].ID, sortOldest[2].ID)
	}
}

func TestDateFilterInclusiveEndOfDay(t *testing.T) {
	loc := time.Local
	dateLayout := "2006-01-02"

	parsedEnd, err := time.ParseInLocation(dateLayout, "2026-06-25", loc)
	if err != nil {
		t.Fatalf("failed to parse end date: %v", err)
	}

	endOfDayExclusive := parsedEnd.AddDate(0, 0, 1)

	// Laporan on 2026-06-25 at 23:59:59 should be included
	lapLate := time.Date(2026, 6, 25, 23, 59, 59, 999999999, loc)
	if !lapLate.Before(endOfDayExclusive) {
		t.Fatalf("report at end of day should be before exclusive next day mark")
	}

	// Laporan on 2026-06-26 at 00:00:00 should NOT be included
	lapNextDay := time.Date(2026, 6, 26, 0, 0, 0, 0, loc)
	if lapNextDay.Before(endOfDayExclusive) {
		t.Fatalf("report on next day should not be before exclusive next day mark")
	}
}
