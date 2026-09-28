package seeders

import (
	"log"
	"os"
	"time"

	"backend-jalan-rusak/models"

	"gorm.io/gorm"
)

type ChatSeedData struct {
	ReportJudul string
	Pesan       string
	Balasan     *string
	WaktuKirim  string
	WaktuBalas  string
}

func SeedChat(db *gorm.DB) {
	// 1. Guard pemisahan development vs production
	if os.Getenv("APP_ENV") == "production" || os.Getenv("SEED_DEV_DATA") == "false" {
		log.Println("Seeder: Mode production / SEED_DEV_DATA=false, melewati seed chat development")
		return
	}

	// 2. Konten balasan admin dummy resmi
	balasanLobener := "Selamat pagi Bapak. Laporan sudah kami terima dan ditugaskan ke Kaur Pembangunan Desa Lobener Lor. Penambalan aspal cold-mix dan pemadatan telah dijadwalkan."
	balasanMargadana := "Betul Pak, tim pemeliharaan desa dijadwalkan mulai pengerjaan perataan jalan besok pagi."

	// 3. Dataset percakapan dummy terhubung dengan laporan desa resmi
	chatSeeds := []ChatSeedData{
		// Percakapan untuk Laporan Lobener Lor (Wilayah B - Pemdes Lobener Lor)
		// Message 1: Percakapan awal dengan balasan admin
		{
			ReportJudul: "[DEV] Jalan Desa Lobener Lor - Lubang Jalan",
			Pesan:       "Selamat pagi Bapak/Ibu Admin Desa, izin menanyakan perihal laporan lubang jalan di dekat balai desa Lobener Lor apakah sudah masuk jadwal penanganan?",
			Balasan:     &balasanLobener,
			WaktuKirim:  "2026-09-25 09:15:00",
			WaktuBalas:  "2026-09-25 09:30:00",
		},
		// Message 2: Follow-up dari warga yang belum dibalas admin (menunggu balasan)
		{
			ReportJudul: "[DEV] Jalan Desa Lobener Lor - Lubang Jalan",
			Pesan:       "Baik Pak Admin, terima kasih banyak atas responnya. Apakah saat pengerjaan nanti ada pengalihan arus sementara di sekitar balai desa?",
			Balasan:     nil,
			WaktuKirim:  "2026-09-25 10:05:00",
			WaktuBalas:  "",
		},
		// Percakapan untuk Laporan Sukaurip (Wilayah A - Pemdes Indramayu) - Belum dibalas
		{
			ReportJudul: "[DEV] Jalan Desa Sukaurip - Lubang Parah",
			Pesan:       "Selamat siang Admin, mohon info apakah lubang parah di Desa Sukaurip sudah dipasangi rambu peringatan sebelum pengerjaan?",
			Balasan:     nil,
			WaktuKirim:  "2026-09-25 11:20:00",
			WaktuBalas:  "",
		},
		// Percakapan untuk Laporan Margadana (Wilayah A - Pemdes Indramayu) - Sudah dibalas
		{
			ReportJudul: "[DEV] Jalan Desa Margadana - Retak Buaya",
			Pesan:       "Pak Admin Pemdes, material pengurukan sudah tampak tiba di Margadana. Apakah pengerjaan dimulai besok?",
			Balasan:     &balasanMargadana,
			WaktuKirim:  "2026-09-25 13:00:00",
			WaktuBalas:  "2026-09-25 13:45:00",
		},
	}

	seededCount := 0
	timeLayout := "2006-01-02 15:04:05"

	for _, item := range chatSeeds {
		// Cari laporan terkait berdasarkan judul resmi
		var laporan models.LaporanKerusakan
		if err := db.Where("judul = ? AND deleted_at IS NULL", item.ReportJudul).First(&laporan).Error; err != nil {
			log.Printf("Seeder Chat: Laporan '%s' belum tersedia, lewati seed chat\n", item.ReportJudul)
			continue
		}

		// Idempotency: cek apakah pesan ini sudah ada untuk laporan terkait
		var existingCount int64
		db.Model(&models.RiwayatChat{}).
			Where("laporan_kerusakan_id = ? AND pesan = ? AND deleted_at IS NULL", laporan.ID, item.Pesan).
			Count(&existingCount)
		if existingCount > 0 {
			continue
		}

		// Parse timestamp deterministik
		waktuKirim, err := time.ParseInLocation(timeLayout, item.WaktuKirim, time.Local)
		if err != nil {
			waktuKirim = time.Now()
		}

		var adminID *uint
		var waktuBalasPtr *time.Time

		if item.Balasan != nil && *item.Balasan != "" {
			// Cari admin pemdes yang sesuai dengan wilayah laporan
			var adminUser models.User
			if err := db.Where("role = ? AND wilayah_id = ?", models.RoleAdminPemdes, laporan.WilayahID).First(&adminUser).Error; err == nil {
				adminID = &adminUser.ID
			}

			if item.WaktuBalas != "" {
				wb, errWB := time.ParseInLocation(timeLayout, item.WaktuBalas, time.Local)
				if errWB == nil {
					waktuBalasPtr = &wb
				}
			}
		}

		newChat := models.RiwayatChat{
			LaporanKerusakanID: laporan.ID,
			UserID:             laporan.UserID,
			Pesan:              item.Pesan,
			AdminID:            adminID,
			Balasan:            item.Balasan,
			DibalasAt:          waktuBalasPtr,
		}
		newChat.CreatedAt = waktuKirim
		newChat.UpdatedAt = waktuKirim

		if err := db.Create(&newChat).Error; err == nil {
			seededCount++
		} else {
			log.Printf("Seeder Chat: Gagal membuat chat untuk laporan %s: %v\n", item.ReportJudul, err)
		}
	}

	if seededCount > 0 {
		log.Printf("Seeder: Berhasil memasukkan %d data chat development\n", seededCount)
	} else {
		log.Println("Seeder: Seluruh data chat development sudah ada (idempotent)")
	}
}
