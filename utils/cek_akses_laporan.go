package utils

import (
	"strings"

	"backend-jalan-rusak/models"
	"backend-jalan-rusak/config"
)

// ini itu buat cek akses laporan per id user
func CekAksesLaporan(role string, userID uint, laporan models.LaporanKerusakan) bool {
	jenisJalan := strings.ToLower(laporan.JenisJalan)

	switch role {
	case string(models.RoleWarga):
		// ini buat warga ngecek laporan milik sendiri 
		return laporan.UserID == userID

	case string(models.RoleAdminPemdes):
		if jenisJalan != "desa" {
			return false
		}

		var admin models.User
		config.DB.First(&admin, userID)
		return admin.WilayahID != nil && laporan.WilayahID > 0 && *admin.WilayahID == laporan.WilayahID

	case string(models.RoleAdminPu):
		if jenisJalan != "kabupaten" {
			return false
		}
		return true

	case string(models.RoleSuperAdmin):
		return true

	default:
		return false
	}
}