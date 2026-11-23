package models

import (
	"time"
)

// RiwayatStatusLaporan merekam jejak audit resmi perubahan status penanganan laporan.
type RiwayatStatusLaporan struct {
	ID         uint              `json:"id" gorm:"primaryKey;autoIncrement"`
	LaporanID  uint              `json:"laporan_id" gorm:"not null;index"`
	Laporan    *LaporanKerusakan `json:"laporan,omitempty" gorm:"foreignKey:LaporanID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	StatusLama string            `json:"status_lama" gorm:"type:varchar(50);not null"`
	StatusBaru string            `json:"status_baru" gorm:"type:varchar(50);not null"`
	UserID     *uint             `json:"user_id" gorm:"index"`
	User       *User             `json:"user,omitempty" gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	Role       string            `json:"role" gorm:"type:varchar(50);not null"`
	Catatan    string            `json:"catatan" gorm:"type:text"`
	CreatedAt  time.Time         `json:"created_at" gorm:"not null;index"`
}

// TableName menentukan nama tabel fisik di database MySQL.
func (RiwayatStatusLaporan) TableName() string {
	return "riwayat_status_laporan"
}
