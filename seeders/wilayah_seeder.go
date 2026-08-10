package seeders

import (
	"log"

	"backend-jalan-rusak/models"

	"gorm.io/gorm"
)

func SeedWilayah(db *gorm.DB) {
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