package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	

	"gorm.io/gorm"
	"backend-jalan-rusak/models"
)

type osmResponse struct {
	Address struct {
		Village string `json:"village"`
		Suburb string `json:"suburb"`
		Town string `json:"town"`
	} `json:"address"`
	Extratags struct {
		Highway string `json:"highway"`
	} `json:"extratags"`
}

func ReverseGeocodeOSM(lat, lng float64) (namaWilayah string, jenisJalan string, err error) {
	url := fmt.Sprintf("https://nominatim.openstreetmap.org/reverse?lat=%f&lon=%f&format=json&extratags=1", lat, lng)

	req, err  := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", "", err
	}

	req.Header.Set("User-Agent", "Jalan-Rusak/1.0")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var result osmResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}

	if result.Address.Village != "" {
		namaWilayah = result.Address.Village
	} else if result.Address.Suburb != "" {
		namaWilayah = result.Address.Suburb
	} else if result.Address.Town != "" {
		namaWilayah = result.Address.Town
	} else {
		return "", "", fmt.Errorf("wilayah tidak ditemukan")
	}

	// ini buat otomatis tentukan jenis jalan berdasarkan osm 
	jenisJalan = "desa" //ini buat defaultnya
	hw := result.Extratags.Highway
	if hw == "primary" || hw == "secondary" || hw == "tertiary" {
		jenisJalan = "jalan"
	}

	return namaWilayah, jenisJalan, nil
}

// buat mengirim ke database
func FindWilayahByNama(db *gorm.DB, nama string) (models.Wilayah, error) {
	var w models.Wilayah
	namaBersih := strings.TrimSpace(nama)
	err := db.Where("nama LIKE ?", "%"+namaBersih+"%").First(&w).Error
	return w, err
}