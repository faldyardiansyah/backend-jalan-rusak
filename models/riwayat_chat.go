package models

import (
	"time"

	"gorm.io/gorm"
)

type RiwayatChat struct {
	gorm.Model
	LaporanKerusakanID uint             `json:"laporan_kerusakan_id" gorm:"not null"`
	LaporanKerusakan   LaporanKerusakan `json:"laporan_kerusakan" gorm:"foreignKey:LaporanKerusakanID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	UserID uint   `json:"user_id" gorm:"not null"`
	User   User   `json:"user" gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Pesan  string `json:"pesan" gorm:"type:text;not null"`

	AdminID   *uint      `json:"admin_id"`
	Admin     *User      `gorm:"foreignKey:AdminID" json:"admin,omitempty"`
	Balasan   *string    `json:"balasan" gorm:"type:text"`
	DibalasAt *time.Time `json:"dibalas_at"`

	LampiranBalasanURL      *string `json:"lampiran_balasan_url" gorm:"type:varchar(255)"`
	LampiranBalasanNama     *string `json:"lampiran_balasan_nama" gorm:"type:varchar(255)"`
	LampiranBalasanMimeType *string `json:"lampiran_balasan_mime_type" gorm:"type:varchar(100)"`
}

func (*RiwayatChat) TableName() string {
	return "riwayat_chat"
}
