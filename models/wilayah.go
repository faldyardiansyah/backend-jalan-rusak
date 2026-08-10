package models

import "gorm.io/gorm"

type Wilayah struct {
	gorm.Model
	Nama string `json:"nama" gorm:"type:varchar(250);not null"`
	Tipe string `json:"tipe wilayah" gorm:"type:varchar(50);not null"`
}

