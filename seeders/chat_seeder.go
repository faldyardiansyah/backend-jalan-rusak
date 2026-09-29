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

	// 2. Pembersihan aman (Soft-delete) data chat hasil testing HTTP/integrasi sebelumnya (Phase I)
	testPesanPatterns := []string{
		"%Dokumentasi webp%",
		"%Tes upload pdf%",
		"%Tes oversize%",
		"%Tes spoof%",
		"%Foto kondisi jalan setelah penanganan%",
	}
	for _, pattern := range testPesanPatterns {
		var testChats []models.RiwayatChat
		if err := db.Where("pesan LIKE ? AND deleted_at IS NULL", pattern).Find(&testChats).Error; err == nil && len(testChats) > 0 {
			for _, tc := range testChats {
				db.Delete(&tc)
				log.Printf("Seeder Chat: Membersihkan record chat test ID %d: %s\n", tc.ID, tc.Pesan)
			}
		}
	}

	// Reset balasan test pada record chat dummy jika sempat tertimpa saat testing
	var modifiedChats []models.RiwayatChat
	if err := db.Where("(balasan LIKE ? OR balasan LIKE ? OR balasan LIKE ?) AND deleted_at IS NULL",
		"%Dokumentasi webp%", "%Tes %", "%Foto kondisi jalan setelah penanganan%").Find(&modifiedChats).Error; err == nil {
		for _, mc := range modifiedChats {
			db.Model(&mc).Updates(map[string]interface{}{
				"lampiran_balasan_url":       nil,
				"lampiran_balasan_nama":      nil,
				"lampiran_balasan_mime_type": nil,
			})
			log.Printf("Seeder Chat: Mereset attachment balasan test pada chat ID %d\n", mc.ID)
		}
	}

	// 3. Konten balasan admin dummy resmi
	balasanLobener1 := "Selamat pagi Pak Faldy. Laporan sudah kami terima dan ditugaskan ke Kaur Pembangunan Desa Lobener Lor. Penambalan aspal cold-mix dan pemadatan telah dijadwalkan."
	balasanLobener2 := "Pengalihan arus kendaraan roda empat dikoordinasikan dengan Linmas desa setempat selama proses pengerjaan penambalan berlangsung."
	balasanLobenerSiti := "Selamat siang Ibu Siti. Tim pemeliharaan desa sudah mulai melakukan pengurukan batu di lokasi amblas tersebut. Terima kasih atas laporannya."
	balasanMargadana := "Betul Pak, tim pemeliharaan desa dijadwalkan mulai pengerjaan perataan jalan besok pagi."

	// 4. Dataset percakapan dummy terhubung dengan laporan desa resmi
	chatSeeds := []ChatSeedData{
		// Percakapan 1 (Warga: Faldy Ardiansyah) - Laporan Desa Lobener Lor (Lubang Jalan) - SUDAH DIBALAS
		{
			ReportJudul: "[DEV] Jalan Desa Lobener Lor - Lubang Jalan",
			Pesan:       "Selamat pagi Bapak/Ibu Admin Desa, izin menanyakan perihal laporan lubang jalan di dekat balai desa Lobener Lor apakah sudah masuk jadwal penanganan?",
			Balasan:     &balasanLobener1,
			WaktuKirim:  "2026-09-25 09:15:00",
			WaktuBalas:  "2026-09-25 09:30:00",
		},
		{
			ReportJudul: "[DEV] Jalan Desa Lobener Lor - Lubang Jalan",
			Pesan:       "Baik Pak Admin, terima kasih banyak atas responnya. Apakah saat pengerjaan nanti ada pengalihan arus sementara di sekitar balai desa?",
			Balasan:     &balasanLobener2,
			WaktuKirim:  "2026-09-25 10:05:00",
			WaktuBalas:  "2026-09-25 10:20:00",
		},
		// Percakapan 2 (Warga: Siti Aminah) - Laporan Desa Lobener Lor (Bahu Jalan Amblas) - SUDAH DIBALAS
		{
			ReportJudul: "[DEV] Jalan Desa Lobener Lor - Bahu Jalan Amblas",
			Pesan:       "Pak Admin, apakah laporan kerusakan bahu jalan amblas di dekat irigasi desa Lobener Lor sudah mulai diperbaiki?",
			Balasan:     &balasanLobenerSiti,
			WaktuKirim:  "2026-09-26 10:15:00",
			WaktuBalas:  "2026-09-26 10:45:00",
		},
		// Percakapan 3 (Warga: Bambang Prasetyo) - Laporan Desa Lobener Lor (Genangan dan Lubang) - BELUM DIBALAS
		{
			ReportJudul: "[DEV] Jalan Desa Lobener Lor - Genangan dan Lubang",
			Pesan:       "Pak Admin, kondisi jalan berlubang dan tergenang air di dusun barat semakin parah saat hujan. Apakah sudah ada jadwal pengecekan ke lokasi?",
			Balasan:     nil,
			WaktuKirim:  "2026-09-26 14:00:00",
			WaktuBalas:  "",
		},
		// Percakapan Wilayah A (Indramayu) - Laporan Sukaurip (Belum dibalas)
		{
			ReportJudul: "[DEV] Jalan Desa Sukaurip - Lubang Parah",
			Pesan:       "Selamat siang Admin, mohon info apakah lubang parah di Desa Sukaurip sudah dipasangi rambu peringatan sebelum pengerjaan?",
			Balasan:     nil,
			WaktuKirim:  "2026-09-25 11:20:00",
			WaktuBalas:  "",
		},
		// Percakapan Wilayah A (Indramayu) - Laporan Margadana (Sudah dibalas)
		{
			ReportJudul: "[DEV] Jalan Desa Margadana - Retak Buaya",
			Pesan:       "Pak Admin Pemdes, material pengurukan sudah tampak tiba di Margadana. Apakah pengerjaan dimulai besok?",
			Balasan:     &balasanMargadana,
			WaktuKirim:  "2026-09-25 13:00:00",
			WaktuBalas:  "2026-09-25 13:45:00",
		},
	}

	seededCount := 0
	updatedCount := 0
	timeLayout := "2006-01-02 15:04:05"

	for _, item := range chatSeeds {
		// Cari laporan terkait berdasarkan judul resmi
		var laporan models.LaporanKerusakan
		if err := db.Where("judul = ? AND deleted_at IS NULL", item.ReportJudul).First(&laporan).Error; err != nil {
			log.Printf("Seeder Chat: Laporan '%s' belum tersedia, lewati seed chat\n", item.ReportJudul)
			continue
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

		// Idempotency: cek apakah pesan ini sudah ada untuk laporan terkait
		var existingChat models.RiwayatChat
		errFind := db.Where("laporan_kerusakan_id = ? AND pesan = ? AND deleted_at IS NULL", laporan.ID, item.Pesan).First(&existingChat).Error
		if errFind == nil {
			// Update UserID jika belum sinkron dengan laporan.UserID
			updates := map[string]interface{}{}
			if existingChat.UserID != laporan.UserID {
				updates["user_id"] = laporan.UserID
			}
			// Jika balasan berbeda (misal tertimpa saat testing seperti Chat 9), kembalikan ke balasan resmi seeder
			if item.Balasan != nil && *item.Balasan != "" {
				if existingChat.Balasan == nil || *existingChat.Balasan != *item.Balasan {
					updates["balasan"] = *item.Balasan
					if waktuBalasPtr != nil {
						updates["dibalas_at"] = waktuBalasPtr
					}
					if adminID != nil {
						updates["admin_id"] = adminID
					}
				}
			}
			if len(updates) > 0 {
				if errUpd := db.Model(&existingChat).Updates(updates).Error; errUpd == nil {
					updatedCount++
				}
			}
			continue
		}

		// Parse timestamp deterministik
		waktuKirim, err := time.ParseInLocation(timeLayout, item.WaktuKirim, time.Local)
		if err != nil {
			waktuKirim = time.Now()
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

	if seededCount > 0 || updatedCount > 0 {
		log.Printf("Seeder: Berhasil menyinkronkan data chat (dibuat: %d, diperbarui: %d)\n", seededCount, updatedCount)
	} else {
		log.Println("Seeder: Seluruh data chat development sudah sinkron dan sesuai (idempotent)")
	}
}

