package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"

	"github.com/gin-gonic/gin"
)

func TestRiwayatStatus_TransactionalRecording(t *testing.T) {
	if !ensureTestDB(t) {
		return
	}
	gin.SetMode(gin.TestMode)

	// Persiapkan master wilayah & user
	wilayahDesa := models.Wilayah{
		Nama: fmt.Sprintf("Desa Riwayat Test %d", time.Now().UnixNano()),
		Tipe: "desa",
	}
	config.DB.Create(&wilayahDesa)

	adminPemdes := models.User{
		Name:      "Admin Pemdes Riwayat Test",
		Email:     fmt.Sprintf("admin_riwayat_%d@example.com", time.Now().UnixNano()),
		Password:  "password123",
		Role:      models.RoleAdminPemdes,
		WilayahID: &wilayahDesa.ID,
	}
	config.DB.Create(&adminPemdes)

	pelapor := models.User{
		Name:     "Warga Pelapor Riwayat",
		Email:    fmt.Sprintf("warga_riwayat_%d@example.com", time.Now().UnixNano()),
		Password: "password123",
		Role:     models.RoleWarga,
	}
	config.DB.Create(&pelapor)

	laporan := models.LaporanKerusakan{
		UserID:        pelapor.ID,
		WilayahID:     wilayahDesa.ID,
		JenisJalan:    "desa",
		Judul:         "Jalan Rusak Riwayat Test",
		Deskripsi:     "Lubang uji coba riwayat status",
		Latitude:      -6.3264,
		Longitude:     108.3242,
		ImageURL:      "https://example.com/foto.jpg",
		TipeKerusakan: "Lubang",
		Status:        "menunggu",
		Severity:      "sedang",
	}
	config.DB.Create(&laporan)

	t.Run("Status berubah menghasilkan tepat satu entri riwayat", func(t *testing.T) {
		// Update status: menunggu -> proses
		body, _ := json.Marshal(map[string]string{
			"status":        "proses",
			"catatan_admin": "Petugas telah dijadwalkan ke lokasi",
		})
		req, _ := http.NewRequest(http.MethodPut, "/api/admin/laporan/"+strconv.Itoa(int(laporan.ID))+"/status", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(laporan.ID))}}
		c.Set("user_id", adminPemdes.ID)
		c.Set("role", string(models.RoleAdminPemdes))

		UpdateStatusLaporan(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		// Verifikasi tabel riwayat di database
		var riwayat []models.RiwayatStatusLaporan
		config.DB.Where("laporan_id = ?", laporan.ID).Find(&riwayat)
		if len(riwayat) != 1 {
			t.Fatalf("Harus ada tepat satu entri riwayat setelah status diubah, didapat: %d", len(riwayat))
		}
		if riwayat[0].StatusLama != "menunggu" {
			t.Errorf("expected StatusLama 'menunggu', got '%s'", riwayat[0].StatusLama)
		}
		if riwayat[0].StatusBaru != "proses" {
			t.Errorf("expected StatusBaru 'proses', got '%s'", riwayat[0].StatusBaru)
		}
		if riwayat[0].UserID == nil || *riwayat[0].UserID != adminPemdes.ID {
			t.Errorf("expected UserID %d, got %v", adminPemdes.ID, riwayat[0].UserID)
		}
		if riwayat[0].Role != string(models.RoleAdminPemdes) {
			t.Errorf("expected Role '%s', got '%s'", models.RoleAdminPemdes, riwayat[0].Role)
		}
		if riwayat[0].Catatan != "Petugas telah dijadwalkan ke lokasi" {
			t.Errorf("expected Catatan match, got '%s'", riwayat[0].Catatan)
		}
	})

	t.Run("Update tanpa perubahan status tidak menghasilkan riwayat tambahan", func(t *testing.T) {
		var countBefore int64
		config.DB.Model(&models.RiwayatStatusLaporan{}).Where("laporan_id = ?", laporan.ID).Count(&countBefore)

		// Update hanya catatan dan ditugaskan_ke tanpa mengubah status (tetap "proses")
		body, _ := json.Marshal(map[string]string{
			"status":        "proses",
			"ditugaskan_ke": "Tim Pemeliharaan 2",
			"catatan_admin": "Pembaruan catatan penugasan saja",
		})
		req, _ := http.NewRequest(http.MethodPut, "/api/admin/laporan/"+strconv.Itoa(int(laporan.ID))+"/status", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(laporan.ID))}}
		c.Set("user_id", adminPemdes.ID)
		c.Set("role", string(models.RoleAdminPemdes))

		UpdateStatusLaporan(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var countAfter int64
		config.DB.Model(&models.RiwayatStatusLaporan{}).Where("laporan_id = ?", laporan.ID).Count(&countAfter)
		if countAfter != countBefore {
			t.Errorf("Jumlah riwayat tidak boleh bertambah jika status tidak berubah. Sebelum: %d, Sesudah: %d", countBefore, countAfter)
		}
	})
}

func TestRiwayatStatus_ScopeAndAuthorization(t *testing.T) {
	if !ensureTestDB(t) {
		return
	}
	gin.SetMode(gin.TestMode)

	wilayahDesaA := models.Wilayah{Nama: fmt.Sprintf("Desa A %d", time.Now().UnixNano()), Tipe: "desa"}
	wilayahDesaB := models.Wilayah{Nama: fmt.Sprintf("Desa B %d", time.Now().UnixNano()), Tipe: "desa"}
	config.DB.Create(&wilayahDesaA)
	config.DB.Create(&wilayahDesaB)

	adminPemdesA := models.User{
		Name:      "Admin Pemdes Desa A",
		Email:     fmt.Sprintf("pemdesA_%d@example.com", time.Now().UnixNano()),
		Password:  "password123",
		Role:      models.RoleAdminPemdes,
		WilayahID: &wilayahDesaA.ID,
	}
	adminPU := models.User{
		Name:     "Admin PU",
		Email:    fmt.Sprintf("adminpu_%d@example.com", time.Now().UnixNano()),
		Password: "password123",
		Role:     models.RoleAdminPu,
	}
	superAdmin := models.User{
		Name:     "Super Admin",
		Email:    fmt.Sprintf("super_%d@example.com", time.Now().UnixNano()),
		Password: "password123",
		Role:     models.RoleSuperAdmin,
	}
	wargaUser := models.User{
		Name:     "Warga Biasa",
		Email:    fmt.Sprintf("warga_%d@example.com", time.Now().UnixNano()),
		Password: "password123",
		Role:     models.RoleWarga,
	}
	config.DB.Create(&adminPemdesA)
	config.DB.Create(&adminPU)
	config.DB.Create(&superAdmin)
	config.DB.Create(&wargaUser)

	laporanDesaB := models.LaporanKerusakan{
		UserID:        wargaUser.ID,
		WilayahID:     wilayahDesaB.ID,
		JenisJalan:    "desa",
		Judul:         "Laporan di Desa B",
		Deskripsi:     "Jalan rusak Desa B",
		Latitude:      -6.33,
		Longitude:     108.32,
		ImageURL:      "https://example.com/b.jpg",
		TipeKerusakan: "Lubang",
		Status:        "menunggu",
	}
	config.DB.Create(&laporanDesaB)

	t.Run("Admin Pemdes A dilarang mengakses riwayat laporan Desa B (IDOR Guard)", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/admin/laporan/"+strconv.Itoa(int(laporanDesaB.ID))+"/status-history", nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(laporanDesaB.ID))}}
		c.Set("user_id", adminPemdesA.ID)
		c.Set("role", string(models.RoleAdminPemdes))

		GetRiwayatStatusLaporan(c)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("Admin PU dapat mengakses riwayat laporan", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/admin/laporan/"+strconv.Itoa(int(laporanDesaB.ID))+"/status-history", nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(laporanDesaB.ID))}}
		c.Set("user_id", adminPU.ID)
		c.Set("role", string(models.RoleAdminPu))

		GetRiwayatStatusLaporan(c)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("Superadmin dapat mengakses riwayat laporan secara global", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/admin/laporan/"+strconv.Itoa(int(laporanDesaB.ID))+"/status-history", nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(laporanDesaB.ID))}}
		c.Set("user_id", superAdmin.ID)
		c.Set("role", string(models.RoleSuperAdmin))

		GetRiwayatStatusLaporan(c)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("Warga ditolak dari endpoint riwayat status admin", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/admin/laporan/"+strconv.Itoa(int(laporanDesaB.ID))+"/status-history", nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(laporanDesaB.ID))}}
		c.Set("user_id", wargaUser.ID)
		c.Set("role", string(models.RoleWarga))

		GetRiwayatStatusLaporan(c)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestRiwayatStatus_ChronologicalOrderAndEmptyState(t *testing.T) {
	if !ensureTestDB(t) {
		return
	}
	gin.SetMode(gin.TestMode)

	wilayah := models.Wilayah{Nama: fmt.Sprintf("Wilayah Kronologis %d", time.Now().UnixNano()), Tipe: "desa"}
	config.DB.Create(&wilayah)

	admin := models.User{
		Name:     "Super Admin Kronologis",
		Email:    fmt.Sprintf("super_kronologis_%d@example.com", time.Now().UnixNano()),
		Password: "password123",
		Role:     models.RoleSuperAdmin,
	}
	config.DB.Create(&admin)

	laporan := models.LaporanKerusakan{
		UserID:        admin.ID,
		WilayahID:     wilayah.ID,
		JenisJalan:    "kabupaten",
		Judul:         "Laporan Kronologis",
		Deskripsi:     "Pengujian urutan kronologis",
		Latitude:      -6.35,
		Longitude:     108.30,
		ImageURL:      "https://example.com/k.jpg",
		TipeKerusakan: "Retak",
		Status:        "menunggu",
	}
	config.DB.Create(&laporan)

	t.Run("Laporan lama tanpa riwayat mengembalikan array kosong", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/admin/laporan/"+strconv.Itoa(int(laporan.ID))+"/status-history", nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(laporan.ID))}}
		c.Set("user_id", admin.ID)
		c.Set("role", string(models.RoleSuperAdmin))

		GetRiwayatStatusLaporan(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["status"] != "success" {
			t.Errorf("expected status 'success', got '%v'", resp["status"])
		}
		dataList, ok := resp["data"].([]interface{})
		if !ok || len(dataList) != 0 {
			t.Errorf("expected empty data array, got: %v", resp["data"])
		}
	})

	t.Run("Multi-entri riwayat diurutkan ASC dari terlama ke terbaru", func(t *testing.T) {
		time1 := time.Now().Add(-2 * time.Hour)
		time2 := time.Now().Add(-1 * time.Hour)

		config.DB.Create(&models.RiwayatStatusLaporan{
			LaporanID:  laporan.ID,
			StatusLama: "menunggu",
			StatusBaru: "proses",
			UserID:     &admin.ID,
			Role:       "super_admin",
			Catatan:    "Catatan transisi 1",
			CreatedAt:  time1,
		})

		config.DB.Create(&models.RiwayatStatusLaporan{
			LaporanID:  laporan.ID,
			StatusLama: "proses",
			StatusBaru: "selesai",
			UserID:     &admin.ID,
			Role:       "super_admin",
			Catatan:    "Catatan transisi 2",
			CreatedAt:  time2,
		})

		req, _ := http.NewRequest(http.MethodGet, "/api/admin/laporan/"+strconv.Itoa(int(laporan.ID))+"/status-history", nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(laporan.ID))}}
		c.Set("user_id", admin.ID)
		c.Set("role", string(models.RoleSuperAdmin))

		GetRiwayatStatusLaporan(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp struct {
			Status string                        `json:"status"`
			Data   []models.RiwayatStatusLaporan `json:"data"`
			Total  int                           `json:"total"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.Total != 2 {
			t.Fatalf("expected total 2, got %d", resp.Total)
		}
		if resp.Data[0].StatusLama != "menunggu" || resp.Data[0].StatusBaru != "proses" {
			t.Errorf("unexpected data[0]: %+v", resp.Data[0])
		}
		if resp.Data[1].StatusLama != "proses" || resp.Data[1].StatusBaru != "selesai" {
			t.Errorf("unexpected data[1]: %+v", resp.Data[1])
		}
		if !resp.Data[0].CreatedAt.Before(resp.Data[1].CreatedAt) {
			t.Errorf("Riwayat harus diurutkan secara kronologis (ASC)")
		}
	})
}

func TestRiwayatStatus_TableNotAvailable_RejectsStatusUpdateWith503(t *testing.T) {
	if !ensureTestDB(t) {
		return
	}
	gin.SetMode(gin.TestMode)

	wilayah := models.Wilayah{Nama: fmt.Sprintf("Wilayah 503 Test %d", time.Now().UnixNano()), Tipe: "desa"}
	config.DB.Create(&wilayah)

	admin := models.User{
		Name:      "Admin Pemdes 503",
		Email:     fmt.Sprintf("admin503_%d@example.com", time.Now().UnixNano()),
		Password:  "password123",
		Role:      models.RoleAdminPemdes,
		WilayahID: &wilayah.ID,
	}
	config.DB.Create(&admin)

	laporan := models.LaporanKerusakan{
		UserID:        admin.ID,
		WilayahID:     wilayah.ID,
		JenisJalan:    "desa",
		Judul:         "Laporan 503 Test",
		Deskripsi:     "Pengujian jika tabel riwayat belum migrasi",
		Latitude:      -6.36,
		Longitude:     108.31,
		ImageURL:      "https://example.com/503.jpg",
		TipeKerusakan: "Lubang",
		Status:        "menunggu",
	}
	config.DB.Create(&laporan)

	// Simulasikan tabel riwayat tidak tersedia dengan rename sementara
	if config.DB.Migrator().HasTable(&models.RiwayatStatusLaporan{}) {
		config.DB.Migrator().RenameTable(&models.RiwayatStatusLaporan{}, "riwayat_status_laporan_temp_503")
		defer func() {
			config.DB.Migrator().RenameTable("riwayat_status_laporan_temp_503", &models.RiwayatStatusLaporan{})
		}()
	}

	body, _ := json.Marshal(map[string]string{
		"status":        "proses",
		"catatan_admin": "Mencoba ubah status saat tabel riwayat tidak ada",
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/admin/laporan/"+strconv.Itoa(int(laporan.ID))+"/status", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(laporan.ID))}}
	c.Set("user_id", admin.ID)
	c.Set("role", string(models.RoleAdminPemdes))

	UpdateStatusLaporan(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable saat tabel riwayat tidak ada, got %d: %s", w.Code, w.Body.String())
	}

	// Verifikasi status laporan di DB TIDAK BERUBAH (tetap 'menunggu')
	var freshLaporan models.LaporanKerusakan
	config.DB.First(&freshLaporan, laporan.ID)
	if freshLaporan.Status != "menunggu" {
		t.Fatalf("Status laporan di database harus tetap 'menunggu', didapat: '%s'", freshLaporan.Status)
	}
}

func TestRiwayatStatus_ForeignKeySetNullOnUserDeletion(t *testing.T) {
	if !ensureTestDB(t) {
		return
	}
	gin.SetMode(gin.TestMode)

	wilayah := models.Wilayah{Nama: fmt.Sprintf("Wilayah FK Test %d", time.Now().UnixNano()), Tipe: "desa"}
	config.DB.Create(&wilayah)

	pelapor := models.User{
		Name:     "Warga Pelapor FK",
		Email:    fmt.Sprintf("warga_fk_%d@example.com", time.Now().UnixNano()),
		Password: "password123",
		Role:     models.RoleWarga,
	}
	config.DB.Create(&pelapor)

	admin := models.User{
		Name:      "Admin Akan Dihapus",
		Email:     fmt.Sprintf("admin_del_%d@example.com", time.Now().UnixNano()),
		Password:  "password123",
		Role:      models.RoleAdminPemdes,
		WilayahID: &wilayah.ID,
	}
	config.DB.Create(&admin)

	laporan := models.LaporanKerusakan{
		UserID:        pelapor.ID,
		WilayahID:     wilayah.ID,
		JenisJalan:    "desa",
		Judul:         "Laporan FK User Test",
		Deskripsi:     "Pengujian ketahanan jejak audit jika user dihapus",
		Latitude:      -6.37,
		Longitude:     108.32,
		ImageURL:      "https://example.com/fk.jpg",
		TipeKerusakan: "Amblas",
		Status:        "menunggu",
	}
	config.DB.Create(&laporan)

	// Buat riwayat status yang dicatat oleh admin
	riwayat := models.RiwayatStatusLaporan{
		LaporanID:  laporan.ID,
		StatusLama: "menunggu",
		StatusBaru: "proses",
		UserID:     &admin.ID,
		Role:       string(models.RoleAdminPemdes),
		Catatan:    "Status diubah sebelum admin resign",
		CreatedAt:  time.Now(),
	}
	if err := config.DB.Create(&riwayat).Error; err != nil {
		t.Fatalf("Gagal membuat data riwayat test: %v", err)
	}

	// Lakukan hard delete pada admin
	if err := config.DB.Unscoped().Delete(&admin).Error; err != nil {
		t.Fatalf("Gagal menghapus admin test: %v", err)
	}

	// Verifikasi riwayat status masih ada, tidak terhapus, dan UserID menjadi NULL (ON DELETE SET NULL)
	var freshRiwayat models.RiwayatStatusLaporan
	if err := config.DB.First(&freshRiwayat, riwayat.ID).Error; err != nil {
		t.Fatalf("Riwayat status TIDAK boleh hilang saat user dihapus! Err: %v", err)
	}

	if freshRiwayat.UserID != nil {
		t.Errorf("UserID di riwayat status harus menjadi NULL setelah user dihapus, didapat: %v", *freshRiwayat.UserID)
	}
	if freshRiwayat.Role != string(models.RoleAdminPemdes) {
		t.Errorf("Role di riwayat harus tetap tersimpan, got '%s'", freshRiwayat.Role)
	}
	if freshRiwayat.Catatan != "Status diubah sebelum admin resign" {
		t.Errorf("Catatan di riwayat harus tetap tersimpan, got '%s'", freshRiwayat.Catatan)
	}
}

func TestRiwayatStatus_InsertFailure_RollsBackReportStatus(t *testing.T) {
	if !ensureTestDB(t) {
		return
	}
	gin.SetMode(gin.TestMode)

	wilayah := models.Wilayah{Nama: fmt.Sprintf("Wilayah Rollback Test %d", time.Now().UnixNano()), Tipe: "desa"}
	config.DB.Create(&wilayah)

	admin := models.User{
		Name:      "Admin Rollback",
		Email:     fmt.Sprintf("admin_rb_%d@example.com", time.Now().UnixNano()),
		Password:  "password123",
		Role:      models.RoleAdminPemdes,
		WilayahID: &wilayah.ID,
	}
	config.DB.Create(&admin)

	laporan := models.LaporanKerusakan{
		UserID:        admin.ID,
		WilayahID:     wilayah.ID,
		JenisJalan:    "desa",
		Judul:         "Laporan Rollback Test",
		Deskripsi:     "Pengujian rollback transaksi saat insert riwayat gagal",
		Latitude:      -6.38,
		Longitude:     108.33,
		ImageURL:      "https://example.com/rb.jpg",
		TipeKerusakan: "Lubang",
		Status:        "menunggu",
	}
	config.DB.Create(&laporan)

	// Uji perilaku atomik transaksi secara langsung pada database
	tx := config.DB.Begin()
	laporan.Status = "proses"
	if err := tx.Save(&laporan).Error; err != nil {
		tx.Rollback()
		t.Fatalf("Gagal menyimpan update laporan: %v", err)
	}

	// Simulasikan kegagalan saat insert riwayat status (misal: ID laporan tidak valid pada FK)
	invalidRiwayat := models.RiwayatStatusLaporan{
		LaporanID:  999999999, // ID laporan tidak ada di database, memicu FK violation
		StatusLama: "menunggu",
		StatusBaru: "proses",
		UserID:     &admin.ID,
		Role:       "admin_pemdes",
		CreatedAt:  time.Now(),
	}
	if err := tx.Create(&invalidRiwayat).Error; err != nil {
		// Rollback transaksi secara atomik
		tx.Rollback()
	} else {
		tx.Commit()
		t.Fatalf("Seharusnya insert dengan FK invalid gagal")
	}

	// Verifikasi laporan di database UTUH dan TIDAK berubah statusnya (tetap "menunggu")
	var freshLaporan models.LaporanKerusakan
	config.DB.First(&freshLaporan, laporan.ID)
	if freshLaporan.Status != "menunggu" {
		t.Fatalf("Transaksi gagal harus rollback status laporan ke 'menunggu', didapat: '%s'", freshLaporan.Status)
	}
}
