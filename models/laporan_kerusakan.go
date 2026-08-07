package models

import (
	"gorm.io/gorm"
)

type LaporanKerusakan struct {
	gorm.Model
	UserID        uint    `json:"user_id" gorm:"not null"`
	User          User    `json:"user" gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Judul         string  `json:"judul" gorm:"type:varchar(150);not null"`
	Deskripsi     string  `json:"deskripsi" gorm:"type:text;not null"`
	Latitude      float64 `json:"latitude" gorm:"not null"`
	Longitude     float64 `json:"longitude" gorm:"not null"`
	ImageURL      string  `json:"image_url" gorm:"type:text;not null"`
	TipeKerusakan string  `json:"tipe_kerusakan" gorm:"type:varchar(150);not null"`
	Status        string  `json:"status" gorm:"type:varchar(50);default:'menunggu';not null"`
	DitugaskanKe  string  `json:"ditugaskan_ke" gorm:"type:varchar(150)"`
	FotoBukti     string  `json:"foto_bukti" gorm:"type:text"`
	CatatanAdmin  string  `json:"catatan_admin" gorm:"type:text"`
}