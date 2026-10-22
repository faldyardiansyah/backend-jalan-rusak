
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

type overpassResponse struct {
	Elements []struct {
		Tags map[string]string `json:"tags"`

		Geometry []struct {
			Lat float64 `json:"lat"`
			Lon float64 `json:"lon"`
		} `json:"geometry"`

		Center *struct {
			Lat float64 `json:"lat"`
			Lon float64 `json:"lon"`
		} `json:"center"`
	} `json:"elements"`
}

type InfoJalanOSM struct {
	NamaJalan  string  `json:"nama_jalan"`
	JenisJalan string  `json:"jenis_jalan"`
	JarakMeter float64 `json:"jarak_meter"`
	Terdeteksi bool    `json:"terdeteksi"`
	Keterangan string  `json:"keterangan"`
}

var osmBaseURL = "https://nominatim.openstreetmap.org/reverse"

var overpassURLs = []string{
	"https://overpass-api.de/api/interpreter",
	"https://overpass.kumi.systems/api/interpreter",
}

var osmHTTPClient = &http.Client{
	Timeout: 25 * time.Second,
}

// ReverseGeocodeOSM tetap menggunakan signature lama
// agar bisa dipanggil oleh CreateLaporan.
func ReverseGeocodeOSM(
	lat, lng float64,
) (namaWilayah string, jenisJalan string, err error) {
	if !IsValidCoordinate(lat, lng) {
		return "", "belum_pasti", fmt.Errorf("koordinat tidak valid")
	}

	jenisJalan = "belum_pasti"

	params := url.Values{}
	params.Set("lat", fmt.Sprintf("%.7f", lat))
	params.Set("lon", fmt.Sprintf("%.7f", lng))
	params.Set("format", "jsonv2")
	params.Set("addressdetails", "1")
	params.Set("accept-language", "id")

	req, err := http.NewRequest(
		http.MethodGet,
		osmBaseURL+"?"+params.Encode(),
		nil,
	)

	if err == nil {
		req.Header.Set("User-Agent", "ROADIS/1.0")

		resp, requestErr := osmHTTPClient.Do(req)
		if requestErr == nil {
			if resp.StatusCode == http.StatusOK {
				var data osmResponse
				decodeErr := json.NewDecoder(resp.Body).Decode(&data)
				resp.Body.Close()

				if decodeErr == nil {
					switch {
					case data.Address.Village != "":
						namaWilayah = data.Address.Village
					case data.Address.Suburb != "":
						namaWilayah = data.Address.Suburb
					case data.Address.Town != "":
						namaWilayah = data.Address.Town
					}
				}
			} else {
				resp.Body.Close()
			}
		}
	}

	// Fungsi lama tetap dapat digunakan untuk CreateLaporan.
	hasil, errJalan := cariJalanOSM(lat, lng, 100)
	if errJalan == nil && hasil.Terdeteksi {
		jenisJalan = hasil.JenisJalan
	}

	return namaWilayah, jenisJalan, nil
}

// ValidasiJarakJalanOSM mencari jalan terdekat dalam radius 20 meter.
func ValidasiJarakJalanOSM(
	lat, lng float64,
) (InfoJalanOSM, error) {
	if !IsValidCoordinate(lat, lng) {
		return InfoJalanOSM{}, fmt.Errorf("koordinat tidak valid")
	}

	return cariJalanOSM(lat, lng, 20)
}

// PerkiraanKewenanganJalan memperkirakan kewenangan berdasarkan
// kelas highway OSM. Hasilnya bukan penetapan hukum resmi.
func PerkiraanKewenanganJalan(highway string) string {
	switch strings.ToLower(strings.TrimSpace(highway)) {
	case "motorway", "motorway_link",
		"trunk", "trunk_link":
		return "nasional"

	case "primary", "primary_link",
		"secondary", "secondary_link":
		return "belum_pasti"

	case "tertiary", "tertiary_link":
		return "kabupaten"

	case "residential", "living_street",
		"unclassified", "service", "road":
		return "belum_pasti"

	default:
		return "belum_pasti"
	}
}

// cariJalanOSM mengambil ruas jalan dan memilih yang paling dekat.
func cariJalanOSM(
	lat, lng, radius float64,
) (InfoJalanOSM, error) {
	info := InfoJalanOSM{
		NamaJalan:  "",
		JenisJalan: "belum_pasti",
		JarakMeter: -1,
		Terdeteksi: false,
		Keterangan: "Jalan belum teridentifikasi.",
	}

	query := fmt.Sprintf(
		`[out:json][timeout:20];
		way(around:%.0f,%.7f,%.7f)["highway"];
		out tags center geom;`,
		radius, lat, lng,
	)

	var data overpassResponse
	var lastErr error
	berhasil := false

	for _, endpoint := range overpassURLs {
		form := url.Values{}
		form.Set("data", query)

		req, err := http.NewRequest(
			http.MethodPost,
			endpoint,
			strings.NewReader(form.Encode()),
		)
		if err != nil {
			lastErr = err
			continue
		}

		req.Header.Set(
			"Content-Type",
			"application/x-www-form-urlencoded",
		)
		req.Header.Set("User-Agent", "ROADIS/1.0")

		resp, err := osmHTTPClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf(
				"Overpass HTTP %d",
				resp.StatusCode,
			)
			resp.Body.Close()
			continue
		}

		decodeErr := json.NewDecoder(resp.Body).Decode(&data)
		resp.Body.Close()

		if decodeErr != nil {
			lastErr = decodeErr
			continue
		}

		berhasil = true
		break
	}

	if !berhasil {
		if lastErr == nil {
			lastErr = fmt.Errorf("layanan OSM tidak tersedia")
		}
		return info, lastErr
	}

	jarakTerbaik := math.MaxFloat64
	var tagsTerbaik map[string]string

	for _, element := range data.Elements {
		if strings.TrimSpace(element.Tags["highway"]) == "" {
			continue
		}

		jarak := math.MaxFloat64

		for i := 0; i+1 < len(element.Geometry); i++ {
			a := element.Geometry[i]
			b := element.Geometry[i+1]

			d := jarakKeSegmen(
				lat, lng,
				a.Lat, a.Lon,
				b.Lat, b.Lon,
			)

			if d < jarak {
				jarak = d
			}
		}

		if len(element.Geometry) < 2 && element.Center != nil {
			jarak = jarakMeter(
				lat, lng,
				element.Center.Lat,
				element.Center.Lon,
			)
		}

		if jarak < jarakTerbaik {
			jarakTerbaik = jarak
			tagsTerbaik = element.Tags
		}
	}

	if tagsTerbaik == nil || jarakTerbaik > radius {
		info.Keterangan = fmt.Sprintf(
			"Tidak ditemukan jalan dalam radius %.0f meter.",
			radius,
		)
		return info, nil
	}

	info.NamaJalan = strings.TrimSpace(tagsTerbaik["name"])
	if info.NamaJalan == "" {
		info.NamaJalan = "Nama jalan belum tersedia di OSM"
	}

	info.JarakMeter = math.Round(jarakTerbaik*100) / 100
	info.Terdeteksi = true
	info.JenisJalan = PerkiraanKewenanganJalan(
		tagsTerbaik["highway"],
	)
	info.Keterangan = "Jalan ditemukan melalui OSM. Kewenangan merupakan perkiraan berdasarkan kelas jalan."

	return info, nil
}

// Menghitung jarak dua koordinat dalam meter.
func jarakMeter(
	lat1, lon1, lat2, lon2 float64,
) float64 {
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

	return math.Hypot(
		ax+t*dx,
		ay+t*dy,
	)
}

// Mencari wilayah berdasarkan nama OSM di database wilayah yang sudah ada.
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