package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"backend-jalan-rusak/config"
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
		{
			name:           "Invalid status parameter",
			queryString:    "?status=unregistered_status",
			expectedCode:   http.StatusBadRequest,
			expectedErrMsg: "Parameter status tidak valid",
		},
		{
			name:           "Invalid jenis_jalan parameter",
			queryString:    "?jenis_jalan=tol_layang",
			expectedCode:   http.StatusBadRequest,
			expectedErrMsg: "Parameter jenis_jalan tidak valid",
		},
		{
			name:           "Invalid wilayah_id parameter",
			queryString:    "?wilayah_id=abc_not_a_number",
			expectedCode:   http.StatusBadRequest,
			expectedErrMsg: "Parameter wilayah_id tidak valid",
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

func TestPrioritySortingAndActiveStatus(t *testing.T) {
	timeOld := time.Date(2026, 6, 1, 10, 0, 0, 0, time.Local)
	timeNew := time.Date(2026, 6, 20, 10, 0, 0, 0, time.Local)

	lapBeratOld := models.LaporanKerusakan{Judul: "Berat Old", Severity: "berat", Status: "menunggu"}
	lapBeratOld.ID = 1
	lapBeratOld.CreatedAt = timeOld

	lapBelumDinilaiOld := models.LaporanKerusakan{Judul: "Belum Dinilai Old", Severity: "belum_dinilai", Status: "menunggu"}
	lapBelumDinilaiOld.ID = 2
	lapBelumDinilaiOld.CreatedAt = timeOld

	lapSedangNew := models.LaporanKerusakan{Judul: "Sedang New", Severity: "sedang", Status: "proses"}
	lapSedangNew.ID = 3
	lapSedangNew.CreatedAt = timeNew

	lapRinganOld := models.LaporanKerusakan{Judul: "Ringan Old", Severity: "ringan", Status: "menunggu"}
	lapRinganOld.ID = 4
	lapRinganOld.CreatedAt = timeOld

	lapBeratNew := models.LaporanKerusakan{Judul: "Berat New", Severity: "berat", Status: "proses"}
	lapBeratNew.ID = 5
	lapBeratNew.CreatedAt = timeNew

	lapSelesaiBerat := models.LaporanKerusakan{Judul: "Selesai Berat", Severity: "berat", Status: "selesai"}
	lapSelesaiBerat.ID = 6
	lapSelesaiBerat.CreatedAt = timeOld

	list := []models.LaporanKerusakan{lapRinganOld, lapSedangNew, lapBelumDinilaiOld, lapBeratNew, lapBeratOld, lapSelesaiBerat}

	// 1. Filter aktif
	var activeOnly []models.LaporanKerusakan
	for _, l := range list {
		if l.Status == "menunggu" || l.Status == "proses" {
			activeOnly = append(activeOnly, l)
		}
	}
	if len(activeOnly) != 5 {
		t.Fatalf("expected 5 active reports, got %d", len(activeOnly))
	}

	// 2. Sort priority simulation:
	// Rank: berat (1) -> belum_dinilai (2) -> sedang (3) -> ringan (4)
	// Tie-breaker: CreatedAt ASC -> ID ASC
	getRank := func(sev string) int {
		switch strings.ToLower(sev) {
		case "berat":
			return 1
		case "belum_dinilai", "":
			return 2
		case "sedang":
			return 3
		case "ringan":
			return 4
		default:
			return 5
		}
	}

	for i := 0; i < len(activeOnly); i++ {
		for j := i + 1; j < len(activeOnly); j++ {
			rankI := getRank(activeOnly[i].Severity)
			rankJ := getRank(activeOnly[j].Severity)
			if rankI > rankJ ||
				(rankI == rankJ && activeOnly[i].CreatedAt.After(activeOnly[j].CreatedAt)) ||
				(rankI == rankJ && activeOnly[i].CreatedAt.Equal(activeOnly[j].CreatedAt) && activeOnly[i].ID > activeOnly[j].ID) {
				activeOnly[i], activeOnly[j] = activeOnly[j], activeOnly[i]
			}
		}
	}

	// Expected:
	// 1. ID 1 (Berat, old)
	// 2. ID 5 (Berat, new)
	// 3. ID 2 (Belum Dinilai, old)
	// 4. ID 3 (Sedang, new)
	// 5. ID 4 (Ringan, old)
	expectedIDs := []uint{1, 5, 2, 3, 4}
	for idx, expectedID := range expectedIDs {
		if activeOnly[idx].ID != expectedID {
			t.Fatalf("position %d: expected report ID %d, got %d (%s)", idx, expectedID, activeOnly[idx].ID, activeOnly[idx].Judul)
		}
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

func TestGetAllLaporan_LiveDatabaseQuery(t *testing.T) {
	if !ensureTestDB(t) {
		return
	}
	gin.SetMode(gin.TestMode)

	// Persiapkan wilayah dan user pengujian di db_jalan_rusak_test
	nowNano := time.Now().UnixNano()
	wilayahA := models.Wilayah{Nama: fmt.Sprintf("Desa Filter A %d", nowNano), Tipe: "desa"}
	wilayahB := models.Wilayah{Nama: fmt.Sprintf("Desa Filter B %d", nowNano), Tipe: "desa"}
	config.DB.Create(&wilayahA)
	config.DB.Create(&wilayahB)

	adminPemdes := models.User{
		Name:      "Admin Pemdes Filter Test",
		Email:     fmt.Sprintf("pemdes_filter_%d@example.com", nowNano),
		Password:  "password123",
		Role:      models.RoleAdminPemdes,
		WilayahID: &wilayahA.ID,
	}
	adminPU := models.User{
		Name:     "Admin PU Filter Test",
		Email:    fmt.Sprintf("pu_filter_%d@example.com", nowNano),
		Password: "password123",
		Role:     models.RoleAdminPu,
	}
	superAdmin := models.User{
		Name:     "Super Admin Filter Test",
		Email:    fmt.Sprintf("super_filter_%d@example.com", nowNano),
		Password: "password123",
		Role:     models.RoleSuperAdmin,
	}
	wargaUser := models.User{
		Name:     "Warga Filter Test",
		Email:    fmt.Sprintf("warga_filter_%d@example.com", nowNano),
		Password: "password123",
		Role:     models.RoleWarga,
	}
	config.DB.Create(&adminPemdes)
	config.DB.Create(&adminPU)
	config.DB.Create(&superAdmin)
	config.DB.Create(&wargaUser)

	baseTime := time.Now().Truncate(time.Hour)

	// Buat 4 laporan dengan kombinasi atribut untuk pengujian live
	lap1 := models.LaporanKerusakan{
		UserID:        wargaUser.ID,
		WilayahID:     wilayahA.ID,
		JenisJalan:    "desa",
		Judul:         fmt.Sprintf("Lubang Jalan Desa A %d", nowNano),
		Deskripsi:     "Deskripsi lubang dalam",
		Latitude:      -6.31,
		Longitude:     108.31,
		ImageURL:      "https://example.com/1.jpg",
		TipeKerusakan: "Lubang",
		Status:        "menunggu",
		Severity:      "ringan",
	}
	lap1.CreatedAt = baseTime.Add(-3 * 24 * time.Hour) // 3 hari lalu
	config.DB.Create(&lap1)

	lap2 := models.LaporanKerusakan{
		UserID:        wargaUser.ID,
		WilayahID:     wilayahA.ID,
		JenisJalan:    "desa",
		Judul:         fmt.Sprintf("Retak Jalan Desa A %d", nowNano),
		Deskripsi:     "Deskripsi retak rambut",
		Latitude:      -6.32,
		Longitude:     108.32,
		ImageURL:      "https://example.com/2.jpg",
		TipeKerusakan: "Retak",
		Status:        "proses",
		Severity:      "sedang",
	}
	lap2.CreatedAt = baseTime.Add(-2 * 24 * time.Hour) // 2 hari lalu
	config.DB.Create(&lap2)

	lap3 := models.LaporanKerusakan{
		UserID:        wargaUser.ID,
		WilayahID:     wilayahB.ID,
		JenisJalan:    "desa",
		Judul:         fmt.Sprintf("Amblas Jalan Desa B %d", nowNano),
		Deskripsi:     "Deskripsi jalan amblas parah",
		Latitude:      -6.33,
		Longitude:     108.33,
		ImageURL:      "https://example.com/3.jpg",
		TipeKerusakan: "Amblas",
		Status:        "menunggu",
		Severity:      "berat",
	}
	lap3.CreatedAt = baseTime.Add(-1 * 24 * time.Hour) // 1 hari lalu
	config.DB.Create(&lap3)

	lap4 := models.LaporanKerusakan{
		UserID:        wargaUser.ID,
		WilayahID:     wilayahB.ID,
		JenisJalan:    "kabupaten",
		Judul:         fmt.Sprintf("Jalan Kabupaten Bergelombang %d", nowNano),
		Deskripsi:     "Deskripsi jalan kabupaten",
		Latitude:      -6.34,
		Longitude:     108.34,
		ImageURL:      "https://example.com/4.jpg",
		TipeKerusakan: "Bergelombang",
		Status:        "selesai",
		Severity:      "", // Belum dinilai / kosong
	}
	lap4.CreatedAt = baseTime // hari ini
	config.DB.Create(&lap4)

	t.Cleanup(func() {
		config.DB.Unscoped().Delete(&models.LaporanKerusakan{}, "id IN ?", []uint{lap1.ID, lap2.ID, lap3.ID, lap4.ID})
		config.DB.Unscoped().Delete(&models.User{}, "id IN ?", []uint{adminPemdes.ID, adminPU.ID, superAdmin.ID, wargaUser.ID})
		config.DB.Unscoped().Delete(&models.Wilayah{}, "id IN ?", []uint{wilayahA.ID, wilayahB.ID})
	})

	// Helper untuk memanggil GetAllLaporan via Gin TestContext
	callAPI := func(role string, userID uint, queryStr string) (*httptest.ResponseRecorder, struct {
		Status string                   `json:"status"`
		Data   []models.LaporanKerusakan `json:"data"`
		Total  int64                    `json:"total"`
	}) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest(http.MethodGet, "/api/admin/laporan"+queryStr, nil)
		c.Request = req
		c.Set("role", role)
		c.Set("user_id", userID)

		GetAllLaporan(c)

		var resp struct {
			Status string                   `json:"status"`
			Data   []models.LaporanKerusakan `json:"data"`
			Total  int64                    `json:"total"`
		}
		if w.Code == http.StatusOK {
			_ = json.Unmarshal(w.Body.Bytes(), &resp)
		}
		return w, resp
	}

	t.Run("1. Filter Status (menunggu vs proses)", func(t *testing.T) {
		w, resp := callAPI("super_admin", superAdmin.ID, "?status=menunggu")
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		for _, item := range resp.Data {
			if item.Status != "menunggu" {
				t.Errorf("expected status 'menunggu', got '%s'", item.Status)
			}
		}
	})

	t.Run("2. Filter Severity (sedang vs belum_dinilai)", func(t *testing.T) {
		_, respSedang := callAPI("super_admin", superAdmin.ID, "?severity=sedang")
		for _, item := range respSedang.Data {
			if item.Severity != "sedang" {
				t.Errorf("expected severity 'sedang', got '%s'", item.Severity)
			}
		}

		_, respBelum := callAPI("super_admin", superAdmin.ID, "?severity=belum_dinilai")
		for _, item := range respBelum.Data {
			if item.Severity != "belum_dinilai" && item.Severity != "" {
				t.Errorf("expected severity 'belum_dinilai', got '%s'", item.Severity)
			}
		}
	})

	t.Run("3. Filter Jenis Jalan (kabupaten vs desa)", func(t *testing.T) {
		_, respKab := callAPI("admin_pu", adminPU.ID, "?jenis_jalan=kabupaten")
		for _, item := range respKab.Data {
			if item.JenisJalan != "kabupaten" {
				t.Errorf("expected jenis_jalan 'kabupaten', got '%s'", item.JenisJalan)
			}
		}
	})

	t.Run("4. Filter Wilayah (wilayah_id)", func(t *testing.T) {
		q := fmt.Sprintf("?wilayah_id=%d", wilayahA.ID)
		_, respWilA := callAPI("super_admin", superAdmin.ID, q)
		for _, item := range respWilA.Data {
			if item.WilayahID != wilayahA.ID {
				t.Errorf("expected wilayah_id %d, got %d", wilayahA.ID, item.WilayahID)
			}
		}
	})

	t.Run("5. Filter Tanggal (start_date & end_date)", func(t *testing.T) {
		startDate := lap2.CreatedAt.Format("2006-01-02")
		endDate := lap3.CreatedAt.Format("2006-01-02")
		q := fmt.Sprintf("?start_date=%s&end_date=%s", startDate, endDate)
		_, respDate := callAPI("super_admin", superAdmin.ID, q)
		for _, item := range respDate.Data {
			if item.CreatedAt.Before(lap2.CreatedAt.Truncate(24 * time.Hour)) {
				t.Errorf("item created_at %v is before start_date %v", item.CreatedAt, startDate)
			}
		}
	})

	t.Run("6. Pencarian (Judul, Tipe Kerusakan, Numeric ID)", func(t *testing.T) {
		// Cari berdasarkan tipe kerusakan
		_, respTipe := callAPI("super_admin", superAdmin.ID, "?search=Amblas")
		foundAmblas := false
		for _, item := range respTipe.Data {
			if item.ID == lap3.ID {
				foundAmblas = true
			}
		}
		if !foundAmblas {
			t.Errorf("search=Amblas should return report lap3 (ID %d)", lap3.ID)
		}

		// Cari berdasarkan ID langsung (numerik)
		qID := fmt.Sprintf("?search=%d", lap1.ID)
		_, respID := callAPI("super_admin", superAdmin.ID, qID)
		foundLap1 := false
		for _, item := range respID.Data {
			if item.ID == lap1.ID {
				foundLap1 = true
				break
			}
		}
		if !foundLap1 {
			t.Errorf("search=%d should return report lap1 (ID %d)", lap1.ID, lap1.ID)
		}

		// Cari berdasarkan ID berawalan tagar (?search=#ID)
		qIDHash := fmt.Sprintf("?search=#%d", lap1.ID)
		_, respIDHash := callAPI("super_admin", superAdmin.ID, qIDHash)
		foundLap1Hash := false
		for _, item := range respIDHash.Data {
			if item.ID == lap1.ID {
				foundLap1Hash = true
				break
			}
		}
		if !foundLap1Hash {
			t.Errorf("search=#%d should return report lap1 (ID %d)", lap1.ID, lap1.ID)
		}
	})

	t.Run("7. Sorting Deterministic (newest/terbaru vs oldest/terlama)", func(t *testing.T) {
		_, respNewest := callAPI("super_admin", superAdmin.ID, "?sort_by=newest")
		if len(respNewest.Data) >= 2 {
			if respNewest.Data[0].CreatedAt.Before(respNewest.Data[1].CreatedAt) {
				t.Errorf("sort_by=newest should be DESC")
			}
		}

		_, respOldest := callAPI("super_admin", superAdmin.ID, "?sort=terlama")
		if len(respOldest.Data) >= 2 {
			if respOldest.Data[0].CreatedAt.After(respOldest.Data[1].CreatedAt) {
				t.Errorf("sort=terlama should be ASC")
			}
		}
	})

	t.Run("8. Pagination dan Total Data", func(t *testing.T) {
		_, respPage1 := callAPI("super_admin", superAdmin.ID, "?limit=2&page=1")
		_, respPage2 := callAPI("super_admin", superAdmin.ID, "?limit=2&page=2")

		if len(respPage1.Data) > 2 {
			t.Errorf("limit=2 should return at most 2 items, got %d", len(respPage1.Data))
		}
		if respPage1.Total < 4 {
			t.Errorf("totalData should be at least 4, got %d", respPage1.Total)
		}
		if len(respPage1.Data) > 0 && len(respPage2.Data) > 0 {
			if respPage1.Data[0].ID == respPage2.Data[0].ID {
				t.Errorf("page 1 and page 2 must return different reports")
			}
		}
	})

	t.Run("9. Kombinasi Beberapa Filter", func(t *testing.T) {
		q := fmt.Sprintf("?status=menunggu&jenis_jalan=desa&severity=ringan&wilayah_id=%d", wilayahA.ID)
		_, respCombo := callAPI("super_admin", superAdmin.ID, q)
		if len(respCombo.Data) != 1 || respCombo.Data[0].ID != lap1.ID {
			t.Errorf("kombinasi filter harus menghasilkan tepat laporan lap1, didapat: %d items", len(respCombo.Data))
		}
	})

	t.Run("10. Scope Admin Pemdes dan IDOR Guard", func(t *testing.T) {
		// Admin Pemdes Desa A hanya bisa melihat laporan di desanya yang bertipe desa
		_, respPemdes := callAPI("admin_pemdes", adminPemdes.ID, "")
		for _, item := range respPemdes.Data {
			if item.WilayahID != wilayahA.ID || item.JenisJalan != "desa" {
				t.Errorf("Admin Pemdes tidak boleh melihat laporan di luar wilayah desanya: %+v", item)
			}
		}

		// Admin Pemdes mencoba memfilter ke wilayah lain -> 403 Forbidden
		qTamperWilayah := fmt.Sprintf("?wilayah_id=%d", wilayahB.ID)
		wTamper, _ := callAPI("admin_pemdes", adminPemdes.ID, qTamperWilayah)
		if wTamper.Code != http.StatusForbidden {
			t.Errorf("Admin Pemdes memfilter wilayah lain harus 403 Forbidden, got %d", wTamper.Code)
		}

		// Admin Pemdes mencoba memfilter ke jenis jalan kabupaten -> 403 Forbidden
		wTamperJalan, _ := callAPI("admin_pemdes", adminPemdes.ID, "?jenis_jalan=kabupaten")
		if wTamperJalan.Code != http.StatusForbidden {
			t.Errorf("Admin Pemdes memfilter jenis jalan selain desa harus 403 Forbidden, got %d", wTamperJalan.Code)
		}
	})

	t.Run("11. Role Warga Ditolak (403)", func(t *testing.T) {
		wWarga, _ := callAPI("warga", wargaUser.ID, "")
		if wWarga.Code != http.StatusForbidden {
			t.Errorf("Role warga harus 403 Forbidden, got %d", wWarga.Code)
		}
	})
}
