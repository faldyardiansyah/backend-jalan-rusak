package admin

import (
	"strings"
	"testing"

	"backend-jalan-rusak/models"
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

	if statusFilter != "" && strings.ToLower(lap.Status) != strings.ToLower(statusFilter) {
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
