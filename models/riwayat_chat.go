package models

import (
	"gorm.io/gorm"
)

type RiwayatChat struct {
	gorm.Model
	UserID  uint   `json:"user_id" gorm:"not null"`
	User    User   `json:"user" gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Pesan   string `json:"pesan" gorm:"type:text;not null"`
	Balasan string `json:"balasan" gorm:"type:text"`
}
