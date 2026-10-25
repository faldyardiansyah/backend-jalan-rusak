package utils

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
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
}

type InfoJalanOSM struct {
	NamaJalan  string  `json:"nama_jalan"`
	JenisJalan string  `json:"jenis_jalan"`
	JarakMeter float64 `json:"jarak_meter"`
	Terdeteksi bool    `json:"terdeteksi"`
	Keterangan string  `json:"keterangan"`
}

var osmBaseURL = "https://nominatim.openstreetmap.org/reverse"

var osmHTTPClient = &http.Client{
	Timeout: 8 * time.Second,
}

// ReverseGeocodeOSM tetap mempertahankan signature lama.
// Nama wilayah dicari melalui Nominatim, sedangkan jenis jalan
// diambil dari GeoJSON lokal.
func ReverseGeocodeOSM(
	lat, lng float64,
) (namaWilayah string, jenisJalan string, err error) {
	if !IsValidCoordinate(lat, lng) {
		return "", "belum_pasti", fmt.Errorf("koordinat tidak valid")
	}

	jenisJalan = "belum_pasti"

	// Ambil jenis jalan dari data lokal terlebih dahulu.
	hasil, errJalan := cariJalanLokal(lat, lng, 100)
	if errJalan == nil && hasil.Terdeteksi {
		jenisJalan = hasil.JenisJalan
	}

	// Reverse geocoding tetap menggunakan Nominatim
	// hanya untuk mendapatkan nama wilayah.
	params := url.Values{}
	params.Set("lat", fmt.Sprintf("%.7f", lat))
	params.Set("lon", fmt.Sprintf("%.7f", lng))
	params.Set("format", "jsonv2")
	params.Set("addressdetails", "1")
	params.Set("accept-language", "id")

	req, reqErr := http.NewRequest(
		http.MethodGet,
		osmBaseURL+"?"+params.Encode(),
		nil,
	)
	if reqErr != nil {
		return namaWilayah, jenisJalan, nil
	}

	req.Header.Set("User-Agent", "ROADIS/1.0 (road reporting application)")

	resp, requestErr := osmHTTPClient.Do(req)
	if requestErr != nil {
		return namaWilayah, jenisJalan, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return namaWilayah, jenisJalan, nil
	}

	var data osmResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&data); decodeErr != nil {
		return namaWilayah, jenisJalan, nil
	}

	switch {
	case data.Address.Village != "":
		namaWilayah = data.Address.Village
	case data.Address.Suburb != "":
		namaWilayah = data.Address.Suburb
	case data.Address.Town != "":
		namaWilayah = data.Address.Town
	}

	return namaWilayah, jenisJalan, nil
}

// ValidasiJarakJalanOSM mencari ruas jalan lokal dalam radius 20 meter.
func ValidasiJarakJalanOSM(
	lat, lng float64,
) (InfoJalanOSM, error) {
	if !IsValidCoordinate(lat, lng) {
		return InfoJalanOSM{}, fmt.Errorf("koordinat tidak valid")
	}

	return cariJalanLokal(lat, lng, 20)
}

// PerkiraanKewenanganJalan hanya perkiraan berdasarkan kelas highway OSM.
// Kelas OSM tidak membuktikan kewenangan administratif resmi.
func PerkiraanKewenanganJalan(highway string) string {
	switch strings.ToLower(strings.TrimSpace(highway)) {
	case "motorway", "motorway_link", "trunk", "trunk_link":
		return "nasional"
	case "primary", "primary_link", "secondary", "secondary_link":
		return "belum_pasti"
	case "tertiary", "tertiary_link":
		return "kabupaten"
	case "residential", "living_street", "unclassified", "service", "road":
		return "belum_pasti"
	default:
		return "belum_pasti"
	}
}

// Menghitung jarak dua koordinat dalam meter.
func jarakMeter(lat1, lon1, lat2, lon2 float64) float64 {
	const radiusBumi = 6371000.0

	toRad := func(v float64) float64 {
		return v * math.Pi / 180
	}

	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*
			math.Sin(dLon/2)*math.Sin(dLon/2)

	a = math.Max(0, math.Min(1, a))

	return 2 * radiusBumi * math.Asin(math.Sqrt(a))
}

// Menghitung jarak titik GPS ke segmen jalan.
func jarakKeSegmen(
	lat, lon, lat1, lon1, lat2, lon2 float64,
) float64 {
	const meterPerDerajatLat = 111320.0

	meterPerDerajatLon := meterPerDerajatLat *
		math.Cos(lat*math.Pi/180)

	x := func(v float64) float64 {
		return (v - lon) * meterPerDerajatLon
	}
	y := func(v float64) float64 {
		return (v - lat) * meterPerDerajatLat
	}

	ax, ay := x(lon1), y(lat1)
	bx, by := x(lon2), y(lat2)

	dx, dy := bx-ax, by-ay
	panjangKuadrat := dx*dx + dy*dy

	if panjangKuadrat == 0 {
		return math.Hypot(ax, ay)
	}

	t := -(ax*dx + ay*dy) / panjangKuadrat
	t = math.Max(0, math.Min(1, t))

	return math.Hypot(ax+t*dx, ay+t*dy)
}

// Mencari wilayah berdasarkan nama OSM di database wilayah.
func FindWilayahByNama(
	db *gorm.DB,
	nama string,
) (models.Wilayah, error) {
	if db == nil {
		return models.Wilayah{}, fmt.Errorf("database tidak tersedia")
	}

	nama = strings.TrimSpace(nama)
	if nama == "" {
		return models.Wilayah{}, fmt.Errorf("nama wilayah kosong")
	}

	var wilayah models.Wilayah
	err := db.Where(
		"nama LIKE ? AND deleted_at IS NULL",
		"%"+nama+"%",
	).First(&wilayah).Error

	return wilayah, err
}
