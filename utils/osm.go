
package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"backend-jalan-rusak/models"
	"gorm.io/gorm"
)

type osmResponse struct {
	Category string `json:"category"`
	Type     string `json:"type"`
	Name     string `json:"name"`

	Address struct {
		Road    string `json:"road"`
		Village string `json:"village"`
		Suburb  string `json:"suburb"`
		Town    string `json:"town"`
	} `json:"address"`
}

var osmBaseURL = "https://nominatim.openstreetmap.org/reverse"

var osmHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}

// ReverseGeocodeOSM mengambil wilayah dan perkiraan kewenangan jalan
// berdasarkan objek OSM yang ditemukan di sekitar koordinat.
func ReverseGeocodeOSM(
	lat, lng float64,
) (namaWilayah string, jenisJalan string, err error) {

	if !IsValidCoordinate(lat, lng) {
		return "", "", fmt.Errorf(
			"koordinat tidak valid: lat=%f, lng=%f",
			lat, lng,
		)
	}

	params := url.Values{}
	params.Set("lat", fmt.Sprintf("%f", lat))
	params.Set("lon", fmt.Sprintf("%f", lng))
	params.Set("format", "jsonv2")
	params.Set("addressdetails", "1")
	params.Set("accept-language", "id")

	requestURL := osmBaseURL + "?" + params.Encode()

	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return "", "", fmt.Errorf(
			"gagal membuat request OSM: %w", err,
		)
	}

	req.Header.Set("User-Agent", "ROADIS/1.0 (road damage reporting app)")

	resp, err := osmHTTPClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf(
			"koneksi OSM gagal atau timeout: %w", err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf(
			"layanan OSM mengembalikan HTTP %d",
			resp.StatusCode,
		)
	}

	var result osmResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf(
			"gagal membaca respons OSM: %w", err,
		)
	}

	// Nama wilayah untuk membantu pencocokan desa.
	switch {
	case result.Address.Village != "":
		namaWilayah = result.Address.Village
	case result.Address.Suburb != "":
		namaWilayah = result.Address.Suburb
	case result.Address.Town != "":
		namaWilayah = result.Address.Town
	default:
		namaWilayah = ""
	}

	// Jangan menebak kewenangan jika objek yang ditemukan
	category := strings.ToLower(strings.TrimSpace(result.Category))
	hw := strings.ToLower(strings.TrimSpace(result.Type))

	if category != "highway" {
		return namaWilayah, "belum_pasti", nil
	}

	switch hw {
	case "motorway", "motorway_link",
		"trunk", "trunk_link":
		jenisJalan = "nasional"

	case "primary", "primary_link":
		jenisJalan = "belum_pasti"

	case "secondary", "secondary_link",
		"tertiary", "tertiary_link",
		"unclassified", "residential",
		"living_street", "service",
		"road":
		// Belum bisa memastikan apakah jalan kabupaten
		jenisJalan = "belum_pasti"

	default:
		jenisJalan = "belum_pasti"
	}

	return namaWilayah, jenisJalan, nil
}

func FindWilayahByNama(
	db *gorm.DB,
	nama string,
) (models.Wilayah, error) {

	if db == nil {
		return models.Wilayah{}, fmt.Errorf(
			"database tidak tersedia",
		)
	}

	namaBersih := strings.TrimSpace(nama)
	if namaBersih == "" {
		return models.Wilayah{}, fmt.Errorf(
			"nama wilayah kosong",
		)
	}

	var w models.Wilayah
	err := db.Where(
		"nama LIKE ? AND deleted_at IS NULL",
		"%"+namaBersih+"%",
	).First(&w).Error

	return w, err
}