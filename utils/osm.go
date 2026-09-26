package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"backend-jalan-rusak/models"
	"gorm.io/gorm"
)

type osmResponse struct {
	Address struct {
		Village string `json:"village"`
		Suburb  string `json:"suburb"`
		Town    string `json:"town"`
	} `json:"address"`
	Extratags struct {
		Highway string `json:"highway"`
	} `json:"extratags"`
}

var osmBaseURL = "https://nominatim.openstreetmap.org/reverse"
var osmHTTPClient = &http.Client{
	Timeout: 5 * time.Second,
}

func ReverseGeocodeOSM(lat, lng float64) (namaWilayah string, jenisJalan string, err error) {
	url := fmt.Sprintf("%s?lat=%f&lon=%f&format=json&extratags=1", osmBaseURL, lat, lng)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", "", fmt.Errorf("gagal membuat request geocoding: %w", err)
	}
	req.Header.Set("User-Agent", "Jalan-Rusak/1.0")

	resp, err := osmHTTPClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("koneksi geocoding gagal atau timeout: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("layanan geocoding mengembalikan status HTTP %d", resp.StatusCode)
	}

	var result osmResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("gagal membaca data geocoding: %w", err)
	}

	if result.Address.Village != "" {
		namaWilayah = result.Address.Village
	} else if result.Address.Suburb != "" {
		namaWilayah = result.Address.Suburb
	} else if result.Address.Town != "" {
		namaWilayah = result.Address.Town
	} else {
		return "", "", fmt.Errorf("wilayah tidak ditemukan pada koordinat tersebut")
	}

	hw := strings.ToLower(result.Extratags.Highway)
	switch hw {
	case "trunk":
		jenisJalan = "nasional"
	case "primary":
		jenisJalan = "provinsi"
	case "secondary", "tertiary":
		jenisJalan = "kabupaten"
	default:
		jenisJalan = "desa"
	}

	return namaWilayah, jenisJalan, nil
}

func FindWilayahByNama(db *gorm.DB, nama string) (models.Wilayah, error) {
	var w models.Wilayah
	namaBersih := strings.TrimSpace(nama)
	err := db.Where("nama LIKE ?", "%"+namaBersih+"%").First(&w).Error
	return w, err
}
