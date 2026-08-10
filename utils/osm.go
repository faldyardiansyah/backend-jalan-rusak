package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"backend-jalan-rusak/models"

	"gorm.io/gorm"
)

type osmAddress struct {
	Village string `json:"village"`
	Suburb  string `json:"suburb"`
	Town    string `json:"town"`
	Conty   string `json:"county"`
	Road    string `json:"road"`
}

type osmResponse struct {
	Address osmAddress `json:"address"`
}

func ReverseGeocodeOSM(lat, lng float64) (namaWilayah string, err error) {
	url := fmt.Sprintf("https://nominatim.openstreetmap.org/reverse?lat=%f&lon=%f&format=json", lat, lng)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Jalan-Rusak/1.0")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result osmResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", err
	}

	if result.Address.Village != "" {
		return result.Address.Village, nil
	}

	if result.Address.Suburb != "" {
		return result.Address.Suburb, nil
	}

	if result.Address.Town != "" {
		return result.Address.Town, nil
	}
	return "", fmt.Errorf("tidak dapat menemukan wilayah untuk koordinat (%f, %f)", lat, lng)
}

// find wilayah yang cocok
func FindWilayahByNama(db *gorm.DB, nama string) (models.Wilayah, error) {
	var w models.Wilayah
	namaBersih := strings.TrimSpace(nama)
	err := db.Where("nama LIKE ?", "%"+namaBersih+"%").First(&w).Error
	return w, err
}
