package seeders

import (
	"log"
	"os"

	"backend-jalan-rusak/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func SeedUser(db *gorm.DB) {
	// 1. Guard pemisahan development vs production
	if os.Getenv("APP_ENV") == "production" || os.Getenv("SEED_DEV_DATA") == "false" {
		log.Println("Seeder: Mode production / SEED_DEV_DATA=false, melewati seed user default")
		return
	}

	var wilayahIndramayu models.Wilayah
	if err := db.Where("nama = ?", "Indramayu").First(&wilayahIndramayu).Error; err != nil {
		log.Println("Seeder User: Wilayah Indramayu belum tersedia, lewati seed user")
		return
	}

	var wilayahLobenerLor models.Wilayah
	if err := db.Where("nama = ?", "Lobener Lor").First(&wilayahLobenerLor).Error; err != nil {
		log.Println("Seeder User: Wilayah Lobener Lor belum tersedia, lewati seed user")
		return
	}

	// hash password
	hashedPassword, err := bcrypt.GenerateFromPassword(
		[]byte("12345678"),
		bcrypt.DefaultCost,
	)
	if err != nil {
		log.Fatal("Gagal mengenkripsi password seeder: ", err)
	}

	// 1. Super Admin
	seedUserItem(db, models.User{
		Name:     "Super Admin",
		Email:    "superadmin@gmail.com",
		Password: string(hashedPassword),
		Role:     models.RoleSuperAdmin,
	})

	// 2. Admin PU
	seedUserItem(db, models.User{
		Name:     "Admin Dinas PU",
		Email:    "adminpu@gmail.com",
		Password: string(hashedPassword),
		Role:     models.RoleAdminPu,
	})

	// 3. Admin Pemdes Lobener Lor
	seedUserItem(db, models.User{
		Name:      "Admin Dinas Pemdes",
		Email:     "adminpemdes@gmail.com",
		Password:  string(hashedPassword),
		Role:      models.RoleAdminPemdes,
		WilayahID: &wilayahLobenerLor.ID,
	})

	// 4. Warga 1: Faldy Ardiansyah (Existing User "Warga" dengan email faldy@gmail.com)
	var existingWarga models.User
	if err := db.Where("email = ?", "faldy@gmail.com").First(&existingWarga).Error; err == nil {
		existingWarga.Name = "Faldy Ardiansyah"
		existingWarga.WilayahID = &wilayahLobenerLor.ID
		db.Save(&existingWarga)
	} else {
		seedUserItem(db, models.User{
			Name:      "Faldy Ardiansyah",
			Email:     "faldy@gmail.com",
			Password:  string(hashedPassword),
			Role:      models.RoleWarga,
			WilayahID: &wilayahLobenerLor.ID,
		})
	}

	// 5. Warga 2: Siti Aminah
	seedUserItem(db, models.User{
		Name:      "Siti Aminah",
		Email:     "siti.aminah@roadis.local",
		Password:  string(hashedPassword),
		Role:      models.RoleWarga,
		WilayahID: &wilayahLobenerLor.ID,
	})

	// 6. Warga 3: Bambang Prasetyo
	seedUserItem(db, models.User{
		Name:      "Bambang Prasetyo",
		Email:     "bambang.prasetyo@roadis.local",
		Password:  string(hashedPassword),
		Role:      models.RoleWarga,
		WilayahID: &wilayahLobenerLor.ID,
	})

	log.Println("Seeder: Berhasil memastikan data dummy user (idempotent)")
}

func seedUserItem(db *gorm.DB, u models.User) {
	var count int64
	db.Model(&models.User{}).Where("email = ?", u.Email).Count(&count)
	if count == 0 {
		if err := db.Create(&u).Error; err != nil {
			log.Printf("Seeder: Gagal membuat user %s: %v\n", u.Email, err)
		}
	} else {
		db.Model(&models.User{}).Where("email = ?", u.Email).Updates(map[string]interface{}{
			"name":       u.Name,
			"wilayah_id": u.WilayahID,
		})
	}
}
