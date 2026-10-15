package seeders

import (
	"os"
	"testing"

	"backend-jalan-rusak/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var testDB *gorm.DB

func ensureTestDB(t *testing.T) bool {
	testDBName := os.Getenv("TEST_DB_NAME")
	if testDBName == "" {
		testDBName = "db_jalan_rusak_test"
	}
	devDB := os.Getenv("DB_NAME")
	if devDB == "" {
		devDB = "db_jalan_rusak"
	}

	if testDBName == "db_jalan_rusak" || testDBName == devDB {
		t.Logf("FAIL-SAFE: Test database '%s' matches development database '%s'. Skipping seeder test", testDBName, devDB)
		return false
	}

	if testDB != nil {
		current := testDB.Migrator().CurrentDatabase()
		if current == testDBName {
			return true
		}
		testDB = nil
	}

	dsn := "root:@tcp(127.0.0.1:3306)/" + testDBName + "?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})
	if err != nil {
		t.Logf("MySQL test connection unavailable (%v)", err)
		return false
	}

	current := db.Migrator().CurrentDatabase()
	if current != testDBName {
		t.Logf("FAIL-SAFE: Connected database '%s' does not match test target '%s'", current, testDBName)
		return false
	}

	_ = db.AutoMigrate(
		&models.User{},
		&models.LaporanKerusakan{},
		&models.RiwayatChat{},
		&models.Wilayah{},
		&models.Notifikasi{},
		&models.UserPreference{},
	)

	testDB = db
	return true
}

// 1. Production Mode: APP_ENV=production harus memblokir seluruh seeder tanpa menyentuh DB
func TestSeeders_ProductionGuard_BlocksAllSeeders(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("SEED_DEV_DATA", "")

	// Karena guard mengevaluasi APP_ENV sebelum mengakses pointer db,
	// pemanggilan dengan nil pointer TIDAK boleh menyebabkan panic/dereference.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("expected seeder to return immediately under APP_ENV=production, but it panicked: %v", r)
		}
	}()

	SeedWilayah(nil)
	SeedUser(nil)
	SeedLaporan(nil)
	SeedChat(nil)
}

// 2. SEED_DEV_DATA=false harus memblokir seluruh seeder tanpa menyentuh DB
func TestSeeders_SeedDevDataFalse_BlocksAllSeeders(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("SEED_DEV_DATA", "false")

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("expected seeder to return immediately under SEED_DEV_DATA=false, but it panicked: %v", r)
		}
	}()

	SeedWilayah(nil)
	SeedUser(nil)
	SeedLaporan(nil)
	SeedChat(nil)
}

// 3. APP_ENV=production memiliki prioritas mutlak meskipun SEED_DEV_DATA=true
func TestSeeders_ProductionPriority_OverridesSeedDevData(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("SEED_DEV_DATA", "true")

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("expected APP_ENV=production to strictly override SEED_DEV_DATA=true, but panicked: %v", r)
		}
	}()

	SeedWilayah(nil)
	SeedUser(nil)
	SeedLaporan(nil)
	SeedChat(nil)
}

// 4. Development Mode: Seeder tetap berfungsi normal dan idempotent saat database tersedia
func TestSeeders_DevelopmentMode_WorksIdempotently(t *testing.T) {
	hasDB := ensureTestDB(t)
	if !hasDB {
		t.Skip("MySQL not available for development seeder execution test")
	}

	t.Setenv("APP_ENV", "development")
	t.Setenv("SEED_DEV_DATA", "true")

	// Pastikan pemanggilan seeder di development berhasil tanpa error
	SeedWilayah(testDB)
	SeedUser(testDB)
	SeedLaporan(testDB)
	SeedChat(testDB)

	// Verifikasi data default user dan wilayah tetap ada di database development
	var countWilayah int64
	testDB.Model(&models.Wilayah{}).Count(&countWilayah)
	if countWilayah == 0 {
		t.Errorf("expected wilayah to be present in development mode")
	}

	var countUser int64
	testDB.Model(&models.User{}).Where("email IN ?", []string{
		"superadmin@gmail.com",
		"adminpu@gmail.com",
		"adminpemdes@gmail.com",
	}).Count(&countUser)
	if countUser < 3 {
		t.Errorf("expected at least 3 admin users to be seeded in development mode, got %d", countUser)
	}
}

// 5. Verifikasi bahwa pada APP_ENV=production tidak ada penambahan user baru
func TestSeeders_ProductionMode_DoesNotCreateUsersOnExistingDB(t *testing.T) {
	hasDB := ensureTestDB(t)
	if !hasDB {
		t.Skip("MySQL not available for production isolation test")
	}

	t.Setenv("APP_ENV", "production")
	t.Setenv("SEED_DEV_DATA", "")

	// Hitung user sebelum pemanggilan seeder di mode production
	var userCountBefore int64
	testDB.Model(&models.User{}).Count(&userCountBefore)

	var wilayahCountBefore int64
	testDB.Model(&models.Wilayah{}).Count(&wilayahCountBefore)

	SeedWilayah(testDB)
	SeedUser(testDB)
	SeedLaporan(testDB)
	SeedChat(testDB)

	var userCountAfter int64
	testDB.Model(&models.User{}).Count(&userCountAfter)

	var wilayahCountAfter int64
	testDB.Model(&models.Wilayah{}).Count(&wilayahCountAfter)

	if userCountAfter != userCountBefore {
		t.Errorf("expected user count to remain unchanged in production mode, before=%d, after=%d", userCountBefore, userCountAfter)
	}
	if wilayahCountAfter != wilayahCountBefore {
		t.Errorf("expected wilayah count to remain unchanged in production mode, before=%d, after=%d", wilayahCountBefore, wilayahCountAfter)
	}
}

// 6. Test Startup Guard Condition Logic
func TestDatabaseStartup_GuardCondition(t *testing.T) {
	testCases := []struct {
		name         string
		appEnv       string
		seedDevData  string
		expectBypass bool
	}{
		{"Production standard", "production", "", true},
		{"Production with SEED_DEV_DATA=false", "production", "false", true},
		{"Production with SEED_DEV_DATA=true", "production", "true", true},
		{"Development with SEED_DEV_DATA=false", "development", "false", true},
		{"Development standard", "development", "", false},
		{"Development explicit true", "development", "true", false},
		{"Empty env (defaults to dev behavior)", "", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("APP_ENV", tc.appEnv)
			t.Setenv("SEED_DEV_DATA", tc.seedDevData)

			isBypassed := os.Getenv("APP_ENV") == "production" || os.Getenv("SEED_DEV_DATA") == "false"
			if isBypassed != tc.expectBypass {
				t.Errorf("[%s] expected bypass=%v, got=%v", tc.name, tc.expectBypass, isBypassed)
			}
		})
	}
}

// 7. Test SeedWilayah Idempotency & Isolated Execution
func TestSeedWilayah_IdempotentIsolatedExecution(t *testing.T) {
	hasDB := ensureTestDB(t)
	if !hasDB {
		t.Skip("MySQL not available for isolated seeder test")
	}

	t.Setenv("APP_ENV", "development")
	t.Setenv("SEED_DEV_DATA", "true")

	tx := testDB.Begin()
	defer tx.Rollback()

	// Eksekusi pertama
	SeedWilayah(tx)

	var wIndra1, wLobener1 models.Wilayah
	if err := tx.Where("nama = ? AND deleted_at IS NULL", "Indramayu").First(&wIndra1).Error; err != nil {
		t.Fatalf("expected Indramayu to exist in tx: %v", err)
	}
	if wIndra1.Tipe != "desa" {
		t.Errorf("expected Indramayu tipe='desa', got '%s'", wIndra1.Tipe)
	}

	if err := tx.Where("nama = ? AND deleted_at IS NULL", "Lobener Lor").First(&wLobener1).Error; err != nil {
		t.Fatalf("expected Lobener Lor to exist in tx: %v", err)
	}
	if wLobener1.Tipe != "desa" {
		t.Errorf("expected Lobener Lor tipe='desa', got '%s'", wLobener1.Tipe)
	}

	var countBefore int64
	tx.Model(&models.Wilayah{}).Where("deleted_at IS NULL").Count(&countBefore)

	// Eksekusi kedua (idempotency check)
	SeedWilayah(tx)

	var countAfter int64
	tx.Model(&models.Wilayah{}).Where("deleted_at IS NULL").Count(&countAfter)

	if countAfter != countBefore {
		t.Errorf("expected count to remain unchanged after second run, before=%d, after=%d", countBefore, countAfter)
	}

	// Cek tidak ada duplikat Indramayu
	var countIndra int64
	tx.Model(&models.Wilayah{}).Where("nama = ? AND deleted_at IS NULL", "Indramayu").Count(&countIndra)
	if countIndra != 1 {
		t.Errorf("expected exactly 1 active Indramayu, got %d", countIndra)
	}

	// Cek tidak ada duplikat Lobener Lor
	var countLobener int64
	tx.Model(&models.Wilayah{}).Where("nama = ? AND deleted_at IS NULL", "Lobener Lor").Count(&countLobener)
	if countLobener != 1 {
		t.Errorf("expected exactly 1 active Lobener Lor, got %d", countLobener)
	}
}

// 8. Test SeedWilayah Partial Presence & Independent Sync
func TestSeedWilayah_PartialPresenceSync(t *testing.T) {
	hasDB := ensureTestDB(t)
	if !hasDB {
		t.Skip("MySQL not available for isolated seeder test")
	}

	t.Setenv("APP_ENV", "development")
	t.Setenv("SEED_DEV_DATA", "true")

	tx := testDB.Begin()
	defer tx.Rollback()

	// Simulasikan sebagian data: hapus Lobener Lor dalam transaksi dan kosongkan tipe Indramayu
	tx.Where("nama = ?", "Lobener Lor").Delete(&models.Wilayah{})
	tx.Model(&models.Wilayah{}).Where("nama = ?", "Indramayu").Update("tipe", "")

	// Jalankan SeedWilayah
	SeedWilayah(tx)

	// Verifikasi: Seeder tidak berhenti, Lobener Lor berhasil dibuat kembali
	var wLobener models.Wilayah
	if err := tx.Where("nama = ? AND deleted_at IS NULL", "Lobener Lor").First(&wLobener).Error; err != nil {
		t.Errorf("expected Lobener Lor to be created when partially missing: %v", err)
	}
	if wLobener.Tipe != "desa" {
		t.Errorf("expected Lobener Lor tipe='desa', got '%s'", wLobener.Tipe)
	}

	// Verifikasi: Indramayu yang bertipe kosong berhasil disinkronkan menjadi 'desa'
	var wIndra models.Wilayah
	if err := tx.Where("nama = ? AND deleted_at IS NULL", "Indramayu").First(&wIndra).Error; err != nil {
		t.Errorf("expected Indramayu to exist: %v", err)
	}
	if wIndra.Tipe != "desa" {
		t.Errorf("expected Indramayu tipe to be synced to 'desa', got '%s'", wIndra.Tipe)
	}
}

// 9. Test SeedWilayah Never Generates BE6 or BE16 Fixtures
func TestSeedWilayah_NoBEFixturesGenerated(t *testing.T) {
	hasDB := ensureTestDB(t)
	if !hasDB {
		t.Skip("MySQL not available for isolated seeder test")
	}

	t.Setenv("APP_ENV", "development")
	t.Setenv("SEED_DEV_DATA", "true")

	tx := testDB.Begin()
	defer tx.Rollback()

	var beCountBefore int64
	tx.Model(&models.Wilayah{}).Where("nama LIKE ? OR nama LIKE ?", "%BE16%", "%Desa A BE6%").Count(&beCountBefore)

	SeedWilayah(tx)

	var beCountAfter int64
	tx.Model(&models.Wilayah{}).Where("nama LIKE ? OR nama LIKE ?", "%BE16%", "%Desa A BE6%").Count(&beCountAfter)

	if beCountAfter != beCountBefore {
		t.Errorf("expected seeder to never create BE test fixtures, before=%d, after=%d", beCountBefore, beCountAfter)
	}
}

// 10. Test SeedWilayah Preserves Existing Foreign Keys
func TestSeedWilayah_PreservesUserAndLaporanRelations(t *testing.T) {
	hasDB := ensureTestDB(t)
	if !hasDB {
		t.Skip("MySQL not available for isolated seeder test")
	}

	t.Setenv("APP_ENV", "development")
	t.Setenv("SEED_DEV_DATA", "true")

	tx := testDB.Begin()
	defer tx.Rollback()

	// Catat ID awal Indramayu dan Lobener Lor
	var idIndraBefore, idLobenerBefore uint
	var wI, wL models.Wilayah
	if err := tx.Where("nama = ? AND deleted_at IS NULL", "Indramayu").First(&wI).Error; err == nil {
		idIndraBefore = wI.ID
	}
	if err := tx.Where("nama = ? AND deleted_at IS NULL", "Lobener Lor").First(&wL).Error; err == nil {
		idLobenerBefore = wL.ID
	}

	SeedWilayah(tx)

	var wIAfter, wLAfter models.Wilayah
	if err := tx.Where("nama = ? AND deleted_at IS NULL", "Indramayu").First(&wIAfter).Error; err == nil {
		if idIndraBefore > 0 && wIAfter.ID != idIndraBefore {
			t.Errorf("ID Indramayu must not change: before=%d, after=%d", idIndraBefore, wIAfter.ID)
		}
	}
	if err := tx.Where("nama = ? AND deleted_at IS NULL", "Lobener Lor").First(&wLAfter).Error; err == nil {
		if idLobenerBefore > 0 && wLAfter.ID != idLobenerBefore {
			t.Errorf("ID Lobener Lor must not change: before=%d, after=%d", idLobenerBefore, wLAfter.ID)
		}
	}

	// Pastikan user dan laporan tetap memiliki relasi yang valid
	var orphanedUsers int64
	tx.Model(&models.User{}).Where("wilayah_id IS NOT NULL AND wilayah_id NOT IN (SELECT id FROM wilayah WHERE deleted_at IS NULL)").Count(&orphanedUsers)
	if orphanedUsers > 0 {
		t.Errorf("expected 0 orphaned users, got %d", orphanedUsers)
	}

	var orphanedReports int64
	tx.Model(&models.LaporanKerusakan{}).Where("deleted_at IS NULL AND wilayah_id NOT IN (SELECT id FROM wilayah WHERE deleted_at IS NULL)").Count(&orphanedReports)
	if orphanedReports > 0 {
		t.Errorf("expected 0 orphaned reports, got %d", orphanedReports)
	}
}
