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

	if db == nil {
		return
	}

	defaultWilayah := []models.Wilayah{
		{
			Nama: "Indramayu",
			Tipe: "desa",
		},
		{
			Nama: "Lobener Lor",
			Tipe: "desa",
		},
	}

	for _, w := range defaultWilayah {
		var existing models.Wilayah
		err := db.Where("nama = ? AND deleted_at IS NULL", w.Nama).First(&existing).Error
		if err != nil {
			// Record belum ada -> Buat baru secara mandiri
			if errCreate := db.Create(&models.Wilayah{Nama: w.Nama, Tipe: w.Tipe}).Error; errCreate != nil {
				log.Printf("Seeder Wilayah: Gagal membuat wilayah %s: %v\n", w.Nama, errCreate)
			} else {
				log.Printf("Seeder Wilayah: Berhasil menambahkan wilayah %s (Tipe: %s)\n", w.Nama, w.Tipe)
			}
		} else {
			// Record sudah ada -> Sinkronkan tipe jika masih kosong atau tidak teridentifikasi
			if existing.Tipe == "" || existing.Tipe == "tidak_teridentifikasi" {
				if errUpdate := db.Model(&existing).Update("tipe", w.Tipe).Error; errUpdate != nil {
					log.Printf("Seeder Wilayah: Gagal memperbarui tipe wilayah %s: %v\n", existing.Nama, errUpdate)
				} else {
					log.Printf("Seeder Wilayah: Berhasil menyinkronkan tipe wilayah %s menjadi %s\n", existing.Nama, w.Tipe)
				}
			}
		}
	}

	log.Println("Seeder: Berhasil memastikan data master wilayah (idempotent)")
}
