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

	var wilayahIndramayu models.Wilayah

	if err := db.Where("nama = ?", "Indramayu").
		First(&wilayahIndramayu).Error; err != nil {
		log.Fatal("Wilayah Indramayu belum tersedia: ", err)
	}

	var wilayahLobenerLor models.Wilayah

	if err := db.Where("nama = ?", "Lobener Lor").
		First(&wilayahLobenerLor).Error; err != nil {
		log.Fatal("Wilayah Lobener Lor belum tersedia: ", err)
	}

	// hash password
	hashedPassword, err := bcrypt.GenerateFromPassword(
		[]byte("12345678"),
		bcrypt.DefaultCost,
	)

	if err != nil {
		log.Fatal("Gagal mengenkripsi password seeder: ", err)
	}

	// superadmin
	superAdmin := models.User{Name: "Super Admin", Email: "superadmin@gmail.com", Password: string(hashedPassword), Role: models.RoleSuperAdmin}

	// Admin PU
	adminPU := models.User{
		Name:     "Admin Dinas PU",
		Email:    "adminpu@gmail.com",
		Password: string(hashedPassword),
		Role:     models.RoleAdminPu,
	}

	// Admin Pemdes
	adminPemdes := models.User{
		Name:      "Admin Dinas Pemdes",
		Email:     "adminpemdes@gmail.com",
		Password:  string(hashedPassword),
		Role:      models.RoleAdminPemdes,
		WilayahID: &wilayahLobenerLor.ID,
	}

	// Warga
	warga := models.User{
		Name:      "Warga",
		Email:     "faldy@gmail.com",
		Password:  string(hashedPassword),
		Role:      models.RoleWarga,
		WilayahID: &wilayahIndramayu.ID,
	}

	// simpan ke database
	if err := db.Create(&superAdmin).Error; err != nil {
		log.Fatal("Gagal membuat Super Admin: ", err)
	}
	if err := db.Create(&adminPU).Error; err != nil {
		log.Fatal("Gagal membuat Admin PU: ", err)
	}

	if err := db.Create(&adminPemdes).Error; err != nil {
		log.Fatal("Gagal membuat Admin Pemdes: ", err)
	}

	if err := db.Create(&warga).Error; err != nil {
		log.Fatal("Gagal membuat Warga: ", err)
	}

	log.Println("Seeder: Berhasil memasukkan data dummy user")
}
