package models

import (
	"time"

	"gorm.io/gorm"
)

type RiwayatChat struct {
	gorm.Model
	LaporanKerusakanID uint             `json:"laporan_kerusakan_id" gorm:"not null`
	LaporanKerusakan   LaporanKerusakan `json:"laporan_kerusakan" gorm:"foreignKey:LaporanKerusakanID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	UserID uint   `json:"user_id gorm:"not null"`
	User   User   `json:"user" gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Pesan  string `json:"pesan" gorm:"type:text;not null"`

	AdminID   *uint      `json:"admin_id"`
	Admin     *User      `gorm:"foreignKey:AdminID" json:"admin,omitempty"`
	Balasan   *string    `json:"balasan" gorm:"type:text"`
	DibalasAt *time.Time `json:"dibalas_at"` 
}

func (*RiwayatChat) TableName() string {
	return "riwayat_chat"
}