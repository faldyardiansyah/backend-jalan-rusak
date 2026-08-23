package models

import (
	"gorm.io/gorm"
)

type UserRole string

const (
	RoleWarga       UserRole = "warga"
	RoleAdminPemdes UserRole = "admin_pemdes"
	RoleAdminPu     UserRole = "admin_pu"
	RoleSuperAdmin  UserRole = "super_admin"
)

type User struct {
	gorm.Model
	Name          string             `json:"name" gorm:"type:varchar(150);not null"`
	Email         string             `json:"email" gorm:"type:varchar(150);unique;not null"`
	Password      string             `json:"-" gorm:"type:varchar(150);not null"`
	Role          UserRole           `json:"role" gorm:"type:varchar(50);default:'warga';not null"`
	WilayahID     *uint              `json:"wilayah_id" gorm:"default:null"`
	Wilayah       Wilayah            `json:"wilayah,omitempty" gorm:"foreignKey:WilayahID"`
	ProfilePhoto  string             `json:"profile_photo" gorm:"type:text"`
	Reports       []LaporanKerusakan `json:"reports,omitempty" gorm:"foreignKey:UserID;references:ID"`
	ChatHistories []RiwayatChat      `json:"riwayat_chat,omitempty" gorm:"foreignKey:UserID;references:ID"`
}
