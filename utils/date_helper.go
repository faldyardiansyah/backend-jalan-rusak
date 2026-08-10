package utils

import (
	"fmt"
	"time"
)

func getZoneName(t time.Time) string {
	_, offset := t.Zone()
	hours := offset / 3600

	switch hours {
	case 7:
		return "WIB"
	case 8:
		return "WITA"
	case 9:
		return "WIT"
	default:
		return "WIB"
	}
}

func FormatTanggalIndo(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "-"
	}

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return "-"
	}

	tLokal := t.In(loc)

	namaHari := []string{
		"Minggu",
		"Senin",
		"Selasa",
		"Rabu",
		"Kamis",
		"Jumat",
		"Sabtu",
	}

	namaBulan := []string{
		"Januari",
		"Februari",
		"Maret",
		"April",
		"Mei",
		"Juni",
		"Juli",
		"Agustus",
		"September",
		"Oktober",
		"November",
		"Desember",
	}

	hari := namaHari[tLokal.Weekday()]
	tgl := tLokal.Day()
	bulan := namaBulan[tLokal.Month()-1]
	thn := tLokal.Year()
	jam := tLokal.Format("15:04")
	zona := getZoneName(tLokal)

	return fmt.Sprintf(
		"%s, %d %s %d jam %s %s",
		hari,
		tgl,
		bulan,
		thn,
		jam,
		zona,
	)
}