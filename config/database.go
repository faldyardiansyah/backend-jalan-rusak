package config

import (
	"fmt"
	"log"
	"os"

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

	// ini buat ngelakuin migrasi otomatis setiap modelnya
	err = DB.AutoMigrate(
		&models.User{},
		&models.LaporanKerusakan{},
		&models.RiwayatChat{},
		&models.Wilayah{},
		&models.Notifikasi{},
	)

	if err != nil {
		log.Fatalf("Gagal melakukan migrasi ke database: %v", err)
	}

	seeders.SeedWilayah(DB)
	seeders.SeedUser(DB)

	log.Println("Berhasil terhubung ke datasebase MySQL & migrasinya sukses serta data seeder berhasil dijalankan")
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