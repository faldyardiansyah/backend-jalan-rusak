package config

import (
	"fmt"
	"log"
	"os"
	"time"

	"backend-jalan-rusak/models"
	"backend-jalan-rusak/seeders"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	// buat hapus huruf s di database tablenya
	"gorm.io/gorm/schema" 
)

var DB *gorm.DB
var Cld *cloudinary.Cloudinary

func ConnectDatabase(){
	err := godotenv.Load()
	if err != nil {
		log.Println("Peringatan: file .env tidak di temukan, menggunakan env system yang default")
	}

	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbUser, dbPass, dbHost, dbPort, dbName,
	)

	// buat membuka koneksi database gunain grom
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		// buat hapus huruf s
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})
	if err != nil {
		log.Fatalf("Gagal terhhubung ke database MYSQL: %v", err)
	}

	if sqlDB, errPool := DB.DB(); errPool == nil {
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetConnMaxLifetime(time.Hour)
	}

	// ini buat ngelakuin migrasi otomatis setiap modelnya
	err = DB.AutoMigrate(
		&models.User{},
		&models.LaporanKerusakan{},
		&models.RiwayatChat{},
		&models.Wilayah{},
		&models.Notifikasi{},
		&models.UserPreference{},
	)

	if err != nil {
		log.Fatalf("Gagal melakukan migrasi ke database: %v", err)
	}

	if os.Getenv("APP_ENV") == "production" || os.Getenv("SEED_DEV_DATA") == "false" {
		log.Println("Database: Mode production / SEED_DEV_DATA=false, melewati eksekusi seluruh seeder otomatis")
	} else {
		seeders.SeedWilayah(DB)
		seeders.SeedUser(DB)
		seeders.SeedLaporan(DB)
		seeders.SeedChat(DB)
		log.Println("Database: Seluruh data seeder development berhasil dijalankan")
	}

	log.Println("Berhasil terhubung ke database MySQL & migrasinya sukses")
}

func InitCloudinary(){
	var err error
	cldURL := os.Getenv("CLOUDINARY_URL")
	if cldURL == "" {
		log.Fatal("CLOUDINARY_URL ga bisa konek")
	}

	Cld, err = cloudinary.NewFromURL(cldURL)
	if err != nil {
		log.Fatalf("Gagal terhubung ke cloudinary: %v", err)
	}

	log.Println("Berhasil terhubung ke cloudinary")
}

// GetTestDBName returns the configured test database name (default: "db_jalan_rusak_test").
func GetTestDBName() string {
	if name := os.Getenv("TEST_DB_NAME"); name != "" {
		return name
	}
	return "db_jalan_rusak_test"
}

// GetTestDSN returns the MySQL DSN for testing, strictly preventing connection to db_jalan_rusak.
func GetTestDSN() (string, error) {
	testDB := GetTestDBName()
	devDB := os.Getenv("DB_NAME")
	if devDB == "" {
		devDB = "db_jalan_rusak"
	}

	if testDB == "db_jalan_rusak" || testDB == devDB {
		return "", fmt.Errorf("FAIL-SAFE TRIGGERED: Test database target '%s' matches development database '%s'. Tests are forbidden from mutating persistent development data", testDB, devDB)
	}

	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "root"
	}
	dbPass := os.Getenv("DB_PASS")
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "127.0.0.1"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "3306"
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbUser, dbPass, dbHost, dbPort, testDB,
	)
	return dsn, nil
}

// EnsureTestDB connects to the isolated test database, preventing tests from mutating development data.
func EnsureTestDB() (*gorm.DB, error) {
	testDB := GetTestDBName()
	devDB := os.Getenv("DB_NAME")
	if devDB == "" {
		devDB = "db_jalan_rusak"
	}

	if testDB == "db_jalan_rusak" || testDB == devDB {
		return nil, fmt.Errorf("FAIL-SAFE TRIGGERED: Test database target '%s' matches development database '%s'. Tests are forbidden from mutating persistent development data", testDB, devDB)
	}

	if DB != nil {
		current := DB.Migrator().CurrentDatabase()
		if current == testDB {
			return DB, nil
		}
		// Refuse to reuse any DB connection pointing to dev database!
		DB = nil
	}

	dsn, err := GetTestDSN()
	if err != nil {
		return nil, err
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("isolated test database '%s' unavailable: %w", testDB, err)
	}

	current := db.Migrator().CurrentDatabase()
	if current != testDB {
		return nil, fmt.Errorf("FAIL-SAFE: Connected database '%s' does not match target test database '%s'", current, testDB)
	}

	err = db.AutoMigrate(
		&models.User{},
		&models.LaporanKerusakan{},
		&models.RiwayatChat{},
		&models.Wilayah{},
		&models.Notifikasi{},
		&models.UserPreference{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to auto-migrate test database '%s': %w", testDB, err)
	}

	DB = db
	return DB, nil
}
