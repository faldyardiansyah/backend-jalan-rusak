package models

import "gorm.io/gorm"

type Notifikasi struct {
	gorm.Model
	UserID    uint   `json:"user_id" gorm:"not null"`
	LaporanID uint   `json:"laporan_id"`
	Judul     string `json:"judul" gorm:"type:varchar(200);not null"`
	Pesan     string `json:"pesan" gorm:"type:text;not null"`
	IsRead    bool   `json:"is_read" gorm:"default:false"`
}