package utils

import (
	"strings"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"
)

// CekAksesLaporan memvalidasi apakah role dan user tertentu berhak mengakses laporan kerusakan
func CekAksesLaporan(role string, userID uint, laporan models.LaporanKerusakan) bool {
	if laporan.ID == 0 || userID == 0 || laporan.DeletedAt.Valid {
		return false
	}

	jenisJalan := strings.ToLower(laporan.JenisJalan)

	switch role {
	case string(models.RoleWarga):
		return laporan.UserID == userID

	case string(models.RoleAdminPemdes):
		if jenisJalan != "desa" {
			return false
		}

		if config.DB == nil {
			return false
		}

		var admin models.User
		if err := config.DB.First(&admin, userID).Error; err != nil {
			return false
		}
		return admin.WilayahID != nil && *admin.WilayahID > 0 && laporan.WilayahID > 0 && *admin.WilayahID == laporan.WilayahID

	case string(models.RoleAdminPu):
		// Jalan kabupaten, provinsi, dan nasional (karena studi kasus terfokus di Kabupaten Indramayu
		// dan jalan provinsi/nasional di wilayah kab ditampung oleh Admin PU & Superadmin)
		if jenisJalan != "kabupaten" && jenisJalan != "provinsi" && jenisJalan != "nasional" {
			return false
		}
		return true

	case string(models.RoleSuperAdmin):
		return true

	default:
		return false
	}
}
