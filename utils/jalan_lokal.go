package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type geoJSONJalan struct {
	Type     string              `json:"type"`
	Features []geoJSONFiturJalan `json:"features"`
}

type geoJSONFiturJalan struct {
	Properties map[string]interface{} `json:"properties"`
	Geometry   struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	} `json:"geometry"`
}

type titikJalan struct {
	Lon float64
	Lat float64
}

type dataJalanLokal struct {
	Nama  string
	Jenis string
	Titik []titikJalan
}

var (
	jalanLokalOnce sync.Once
	jalanLokalData []dataJalanLokal
	jalanLokalErr  error
)

func muatJalanLokal() ([]dataJalanLokal, error) {
	jalanLokalOnce.Do(func() {
		// Jalankan server dari root proyek backend.
		path := filepath.Join("data", "ROADIS_Jalan_Indramayu.geojson")

		file, err := os.Open(path)
		if err != nil {
			jalanLokalErr = fmt.Errorf(
				"gagal membuka GeoJSON %q: %w", path, err,
			)
			return
		}
		defer file.Close()

		var koleksi geoJSONJalan
		if err := json.NewDecoder(file).Decode(&koleksi); err != nil {
			jalanLokalErr = fmt.Errorf("GeoJSON tidak valid: %w", err)
			return
		}

		for _, fitur := range koleksi.Features {
			jenis := nilaiProperti(fitur.Properties, "highway")
			if jenis == "" {
				continue
			}

			nama := nilaiProperti(fitur.Properties, "name")
			titik, err := bacaKoordinatJalan(
				fitur.Geometry.Type,
				fitur.Geometry.Coordinates,
			)
			if err != nil || len(titik) < 2 {
				continue
			}

			jalanLokalData = append(jalanLokalData, dataJalanLokal{
				Nama:  nama,
				Jenis: strings.ToLower(strings.TrimSpace(jenis)),
				Titik: titik,
			})
		}

		if len(jalanLokalData) == 0 {
			jalanLokalErr = fmt.Errorf(
				"tidak ditemukan fitur garis jalan dengan properti highway",
			)
		}
	})

	return jalanLokalData, jalanLokalErr
}

func nilaiProperti(properties map[string]interface{}, key string) string {
	if properties == nil {
		return ""
	}

	value, ok := properties[key]
	if !ok || value == nil {
		return ""
	}

	return strings.TrimSpace(fmt.Sprint(value))
}

func bacaKoordinatJalan(
	geometryType string,
	raw json.RawMessage,
) ([]titikJalan, error) {
	var hasil []titikJalan

	switch geometryType {
	case "LineString":
		var coords [][]float64
		if err := json.Unmarshal(raw, &coords); err != nil {
			return nil, err
		}

		for _, c := range coords {
			if len(c) >= 2 {
				hasil = append(hasil, titikJalan{Lon: c[0], Lat: c[1]})
			}
		}

	case "MultiLineString":
		var lines [][][]float64
		if err := json.Unmarshal(raw, &lines); err != nil {
			return nil, err
		}

		for _, line := range lines {
			for _, c := range line {
				if len(c) >= 2 {
					hasil = append(hasil, titikJalan{Lon: c[0], Lat: c[1]})
				}
			}
		}
	}

	return hasil, nil
}

func cariJalanLokal(
	lat, lng, radiusMeter float64,
) (InfoJalanOSM, error) {
	if !IsValidCoordinate(lat, lng) {
		return InfoJalanOSM{}, fmt.Errorf("koordinat tidak valid")
	}

	jalan, err := muatJalanLokal()
	if err != nil {
		return InfoJalanOSM{}, err
	}

	hasil := InfoJalanOSM{
		JenisJalan: "belum_pasti",
		JarakMeter: -1,
		Terdeteksi: false,
		Keterangan: "Tidak ditemukan jalan dalam radius pencarian",
	}

	for _, ruas := range jalan {
		for i := 0; i < len(ruas.Titik)-1; i++ {
			a := ruas.Titik[i]
			b := ruas.Titik[i+1]

			jarak := jarakKeSegmen(
				lat, lng,
				a.Lat, a.Lon,
				b.Lat, b.Lon,
			)

			if jarak > radiusMeter {
				continue
			}

			if hasil.Terdeteksi && jarak >= hasil.JarakMeter {
				continue
			}

			hasil.NamaJalan = ruas.Nama
			if hasil.NamaJalan == "" {
				hasil.NamaJalan = "Nama jalan tidak tersedia di OSM"
			}

			hasil.JenisJalan = ruas.Jenis
			hasil.JarakMeter = jarak
			hasil.Terdeteksi = true
			hasil.Keterangan = "Jalan ditemukan dari data OSM lokal"
		}
	}

	return hasil, nil
}
