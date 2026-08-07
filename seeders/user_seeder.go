package seeders

import (
	"log"

	"backend-jalan-rusak/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)


func SeedUser(db *gorm.DB) {
	var count int64
	db.Model(&models.User{}).Count(&count)

	if count > 0 {
		log.Println("Seeder: Data user sudah ada")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("12345678"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal("Gagal mengenkripsi password seeder: ", err)
	}

	adminPU := models.User{
		Name: "Admin Dinas PU",
		Email: "adminpu@gmail.com",
		Password: string(hashedPassword),
		Role: "admin_pu",
		Domisili: "Indramayu",	
	}

	adminPemdes := models.User{
		Name: "Admin Dinas Pemdes",
		Email: "adminpemdes@gmail.com",
		Password: string(hashedPassword),
		Role: "admin_pemdes",
		Domisili: "Lobener Lor",	
	}

	warga := models.User{
		Name: "Warga",
		Email: "faldy@gmail.com",
		Password: string(hashedPassword),
		Role: "warga",
		Domisili: "Indramayu",	
	}

	// buat nyimpan ke db
	db.Create(&adminPU)
	db.Create(&adminPemdes)
	db.Create(&warga)

	log.Println("Seeder: Berhasil memasukan data dummy user")
}