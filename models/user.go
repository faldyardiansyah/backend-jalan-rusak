package models

import (
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Name          string             `json:"name" gorm:"type:varchar(150);not null"`
	Email         string             `json:"email" gorm:"type:varchar(100);unique;not null"`
	Password      string             `json:"password" gorm:"type:varchar(255);not null"`
	Role          string             `json:"role" gorm:"type:varchar(50);default:'warga';not null"` 
	Domisili      string             `json:"domisili" gorm:"type:text"`                            
	ProfilePhoto  string             `json:"profile_photo" gorm:"type:text"`                       
	Reports       []LaporanKerusakan `json:"reports,omitempty" gorm:"foreignKey:UserID;references:ID"`
	ChatHistories []RiwayatChat      `json:"riwayat_chat,omitempty" gorm:"foreignKey:UserID;references:ID"`
}