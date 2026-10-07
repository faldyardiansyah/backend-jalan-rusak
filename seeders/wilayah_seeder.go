package seeders

import (
	"log"
	"os"

	"backend-jalan-rusak/models"

	"gorm.io/gorm"
)

func SeedWilayah(db *gorm.DB) {
	// 1. Guard pemisahan development vs production
	if os.Getenv("APP_ENV") == "production" || os.Getenv("SEED_DEV_DATA") == "false" {
		log.Println("Seeder: Mode production / SEED_DEV_DATA=false, melewati seed wilayah default")
		return
	}

	var count int64

	db.Model(&models.Wilayah{}).Count(&count)

	if count > 0 {
		log.Println("Seeder: Data wilayah sudah ada")
		return
	}

	wilayah := []models.Wilayah{
		{
			Nama: "Indramayu",
		},
		{
			Nama: "Lobener Lor",
		},
	}

	if err := db.Create(&wilayah).Error; err != nil {
		log.Fatal("Seeder: Gagal memasukkan data wilayah: ", err)
	}

	log.Println("Seeder: Berhasil memasukkan data wilayah")
}