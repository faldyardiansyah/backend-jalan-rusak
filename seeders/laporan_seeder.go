package seeders

import (
	"log"
	"os"

	"backend-jalan-rusak/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func SeedLaporan(db *gorm.DB) {
	// 1. Guard pemisahan development vs production
	if os.Getenv("APP_ENV") == "production" || os.Getenv("SEED_DEV_DATA") == "false" {
		log.Println("Seeder: Mode production / SEED_DEV_DATA=false, melewati seed laporan development")
		return
	}

	// 2. Pastikan Wilayah Indramayu (Wilayah A) & Lobener Lor (Wilayah B) tersedia
	var wilayahIndramayu models.Wilayah
	if err := db.Where("nama = ?", "Indramayu").First(&wilayahIndramayu).Error; err != nil {
		log.Println("Seeder Laporan: Wilayah Indramayu belum tersedia, lewati seed laporan")
		return
	}

	var wilayahLobenerLor models.Wilayah
	if err := db.Where("nama = ?", "Lobener Lor").First(&wilayahLobenerLor).Error; err != nil {
		log.Println("Seeder Laporan: Wilayah Lobener Lor belum tersedia, lewati seed laporan")
		return
	}

	// 3. Pastikan Akun Admin Pemdes Wilayah A (Indramayu) tersedia untuk pengujian isolasi wilayah
	var adminPemdesA models.User
	if err := db.Where("email = ?", "adminpemdes_indramayu@gmail.com").First(&adminPemdesA).Error; err != nil {
		hashedPassword, errPass := bcrypt.GenerateFromPassword([]byte("12345678"), bcrypt.DefaultCost)
		if errPass == nil {
			adminPemdesA = models.User{
				Name:      "Admin Dinas Pemdes Indramayu",
				Email:     "adminpemdes_indramayu@gmail.com",
				Password:  string(hashedPassword),
				Role:      models.RoleAdminPemdes,
				WilayahID: &wilayahIndramayu.ID,
			}
			db.Create(&adminPemdesA)
			log.Println("Seeder: Berhasil menambahkan Admin Pemdes Wilayah A (Indramayu)")
		}
	}

	// 4. Ambil 3 user Warga sebagai pelapor
	var wargaFaldy models.User
	if err := db.Where("email = ?", "faldy@gmail.com").First(&wargaFaldy).Error; err != nil {
		log.Println("Seeder Laporan: User Faldy Ardiansyah belum tersedia, lewati seed laporan")
		return
	}

	var wargaSiti models.User
	if err := db.Where("email = ?", "siti.aminah@roadis.local").First(&wargaSiti).Error; err != nil {
		wargaSiti = wargaFaldy
	}

	var wargaBambang models.User
	if err := db.Where("email = ?", "bambang.prasetyo@roadis.local").First(&wargaBambang).Error; err != nil {
		wargaBambang = wargaFaldy
	}

	// 5. Data development reports (Idempotent: cek judul sebelum create)
	placeholderImg := "https://images.unsplash.com/photo-1515162816999-a0c47dc192f7?w=800"
	evidenceImg := "https://images.unsplash.com/photo-1544620347-c4fd4a3d5957?w=800"

	devReports := []models.LaporanKerusakan{
		// REPORT A: Wilayah A (Indramayu) - Desa - Menunggu
		{
			UserID:        wargaFaldy.ID,
			WilayahID:     wilayahIndramayu.ID,
			JenisJalan:    "desa",
			Judul:         "[DEV] Jalan Desa Sukaurip - Lubang Parah",
			Deskripsi:     "Terdapat beberapa lubang berdiameter 40-60 cm dengan kedalaman sekitar 10 cm yang membahayakan pengendara motor di Desa Sukaurip.",
			Latitude:      -6.3400,
			Longitude:     108.3300,
			ImageURL:      placeholderImg,
			TipeKerusakan: "Lubang",
			Status:        "menunggu",
		},
		// REPORT B: Wilayah A (Indramayu) - Desa - Proses
		{
			UserID:        wargaFaldy.ID,
			WilayahID:     wilayahIndramayu.ID,
			JenisJalan:    "desa",
			Judul:         "[DEV] Jalan Desa Margadana - Retak Buaya",
			Deskripsi:     "Struktur aspal retak buaya cukup luas di sepanjang 50 meter jalan penghubung antar dusun Margadana.",
			Latitude:      -6.3550,
			Longitude:     108.3150,
			ImageURL:      placeholderImg,
			TipeKerusakan: "Retak Buaya",
			Status:        "proses",
			DitugaskanKe:  "Tim Pemeliharaan Desa Margadana",
			CatatanAdmin:  "Material pengurukan dan perataan telah tiba di lokasi desa.",
		},
		// REPORT C: Wilayah B (Lobener Lor) - Desa - Selesai (Warga: Faldy Ardiansyah)
		{
			UserID:        wargaFaldy.ID,
			WilayahID:     wilayahLobenerLor.ID,
			JenisJalan:    "desa",
			Judul:         "[DEV] Jalan Desa Lobener Lor - Lubang Jalan",
			Deskripsi:     "Lubang jalan di dekat balai desa Lobener Lor yang sebelumnya tergenang saat hujan.",
			Latitude:      -6.4150,
			Longitude:     108.2830,
			ImageURL:      placeholderImg,
			TipeKerusakan: "Lubang",
			Status:        "selesai",
			DitugaskanKe:  "Kaur Pembangunan Desa Lobener Lor",
			FotoBukti:     evidenceImg,
			CatatanAdmin:  "Penambalan aspal cold-mix dan pemadatan telah selesai 100%.",
		},
		// REPORT C2 (TAMBAHAN 1): Wilayah B (Lobener Lor) - Desa - Proses (Warga: Siti Aminah)
		{
			UserID:        wargaSiti.ID,
			WilayahID:     wilayahLobenerLor.ID,
			JenisJalan:    "desa",
			Judul:         "[DEV] Jalan Desa Lobener Lor - Bahu Jalan Amblas",
			Deskripsi:     "Bahu jalan desa di dekat saluran irigasi Lobener Lor amblas sedalam 25 cm, memerlukan pengurukan batu dan pemadatan ulang.",
			Latitude:      -6.4180,
			Longitude:     108.2850,
			ImageURL:      placeholderImg,
			TipeKerusakan: "Jalan Amblas",
			Status:        "proses",
			DitugaskanKe:  "Tim Pemeliharaan Desa Lobener Lor",
			CatatanAdmin:  "Material pengurukan dan alat pemadat telah dijadwalkan ke lokasi dusun timur.",
		},
		// REPORT C3 (TAMBAHAN 2): Wilayah B (Lobener Lor) - Desa - Menunggu (Warga: Bambang Prasetyo)
		{
			UserID:        wargaBambang.ID,
			WilayahID:     wilayahLobenerLor.ID,
			JenisJalan:    "desa",
			Judul:         "[DEV] Jalan Desa Lobener Lor - Genangan dan Lubang",
			Deskripsi:     "Permukaan jalan desa berlubang dengan diameter 40 cm dan kerap tergenang air saat hujan, membahayakan pengguna sepeda motor.",
			Latitude:      -6.4120,
			Longitude:     108.2810,
			ImageURL:      placeholderImg,
			TipeKerusakan: "Lubang",
			Status:        "menunggu",
		},
		// REPORT D: Wilayah A (Indramayu) - Kabupaten - Menunggu
		{
			UserID:        wargaFaldy.ID,
			WilayahID:     wilayahIndramayu.ID,
			JenisJalan:    "kabupaten",
			Judul:         "[DEV] Jalan Kabupaten Terisi-Cikedung - Amblas",
			Deskripsi:     "Bahu jalan kabupaten amblas sedalam 30 cm akibat gerusan air irigasi, separuh jalan dipasangi rambu darurat.",
			Latitude:      -6.3700,
			Longitude:     108.2600,
			ImageURL:      placeholderImg,
			TipeKerusakan: "Jalan Amblas",
			Status:        "menunggu",
		},
		// REPORT E: Wilayah B (Lobener Lor) - Kabupaten - Proses
		{
			UserID:        wargaFaldy.ID,
			WilayahID:     wilayahLobenerLor.ID,
			JenisJalan:    "kabupaten",
			Judul:         "[DEV] Jalan Kabupaten Jatibarang-Sleman - Retak Melintang",
			Deskripsi:     "Retak melintang jalan kabupaten yang menyebabkan guncangan keras bagi kendaraan angkutan barang.",
			Latitude:      -6.4700,
			Longitude:     108.3050,
			ImageURL:      placeholderImg,
			TipeKerusakan: "Retak Melintang",
			Status:        "proses",
			DitugaskanKe:  "UPTD Pengawasan Jalan Dinas PU Indramayu",
			CatatanAdmin:  "Alat berat perata aspal dan tim pelaksana sudah mulai pengerjaan perbaikan.",
		},
		// REPORT F: Wilayah B (Lobener Lor) - Kabupaten - Selesai
		{
			UserID:        wargaFaldy.ID,
			WilayahID:     wilayahLobenerLor.ID,
			JenisJalan:    "kabupaten",
			Judul:         "[DEV] Jalan Kabupaten Lobener-Jatibarang - Bergelombang",
			Deskripsi:     "Permukaan aspal jalan kabupaten bergelombang parah akibat muatan tonase berlebih.",
			Latitude:      -6.4350,
			Longitude:     108.2950,
			ImageURL:      placeholderImg,
			TipeKerusakan: "Permukaan Bergelombang",
			Status:        "selesai",
			DitugaskanKe:  "Dinas PUPR Kabupaten Indramayu",
			FotoBukti:     evidenceImg,
			CatatanAdmin:  "Overlay aspal hotmix sepanjang 200 meter telah tuntas dikerjakan.",
		},
		// REPORT G: Wilayah A (Indramayu) - Provinsi - Menunggu
		{
			UserID:        wargaFaldy.ID,
			WilayahID:     wilayahIndramayu.ID,
			JenisJalan:    "provinsi",
			Judul:         "[DEV] Jalan Provinsi Indramayu-Karangampel - Lubang Besar",
			Deskripsi:     "Ruas jalan provinsi mengalami kerusakan lubang berurutan dengan diameter lebih dari 50 cm.",
			Latitude:      -6.3600,
			Longitude:     108.3800,
			ImageURL:      placeholderImg,
			TipeKerusakan: "Lubang",
			Status:        "menunggu",
		},
		// REPORT H: Wilayah B (Lobener Lor) - Nasional - Proses
		{
			UserID:        wargaFaldy.ID,
			WilayahID:     wilayahLobenerLor.ID,
			JenisJalan:    "nasional",
			Judul:         "[DEV] Jalan Nasional Pantura Lobener - Retak Memanjang",
			Deskripsi:     "Jalur utama Pantura segmen Lobener mengalami retak memanjang pada lajur lambat.",
			Latitude:      -6.4200,
			Longitude:     108.2800,
			ImageURL:      placeholderImg,
			TipeKerusakan: "Retak Memanjang",
			Status:        "proses",
			DitugaskanKe:  "Balai Besar Pelaksanaan Jalan Nasional (BBPJN)",
			CatatanAdmin:  "Koordinasi penanganan dan penjadwalan patching lajur pantura bersama tim BBPJN.",
		},
	}

	seededCount := 0
	for _, lap := range devReports {
		var existing models.LaporanKerusakan
		err := db.Where("judul = ?", lap.Judul).First(&existing).Error
		if err != nil {
			if errCreate := db.Create(&lap).Error; errCreate == nil {
				seededCount++
			}
		} else {
			// Update UserID jika belum selaras
			if existing.UserID != lap.UserID {
				db.Model(&existing).Update("user_id", lap.UserID)
			}
		}
	}

	if seededCount > 0 {
		log.Printf("Seeder: Berhasil memasukkan %d data laporan development baru\n", seededCount)
	} else {
		log.Println("Seeder: Seluruh data laporan development sudah ada dan tersinkronisasi (idempotent)")
	}
}
