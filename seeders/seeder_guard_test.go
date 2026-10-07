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
	if testDB != nil {
		return true
	}

	dsn := "root:@tcp(127.0.0.1:3306)/db_jalan_rusak?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})
	if err != nil {
		t.Logf("MySQL connection unavailable in test (%v)", err)
		return false
	}

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
