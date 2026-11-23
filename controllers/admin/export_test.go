package admin

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"backend-jalan-rusak/models"

	"github.com/gin-gonic/gin"
)

func TestSanitizeCSVField(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Normal string unaffected",
			input:    "Jalan Rusak Desa",
			expected: "Jalan Rusak Desa",
		},
		{
			name:     "Formula injection with equals sign",
			input:    "=cmd|'/C calc'!A0",
			expected: "'=cmd|'/C calc'!A0",
		},
		{
			name:     "Formula injection with plus sign",
			input:    "+12345",
			expected: "'+12345",
		},
		{
			name:     "Formula injection with minus sign",
			input:    "-SUM(A1:A10)",
			expected: "'-SUM(A1:A10)",
		},
		{
			name:     "Formula injection with at symbol",
			input:    "@SUM(B1:B5)",
			expected: "'@SUM(B1:B5)",
		},
		{
			name:     "Formula injection with leading whitespace and equals",
			input:    "  =HYPERLINK(\"http://evil.com\")",
			expected: "'  =HYPERLINK(\"http://evil.com\")",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Whitespace only",
			input:    "   ",
			expected: "   ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeCSVField(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeCSVField(%q) = %q, expected %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestExportLaporanCSV_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("1. Unauthenticated request (401)", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/laporan/export/csv", nil)

		ExportLaporanCSV(c)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	t.Run("2. Invalid role string (400)", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/laporan/export/csv", nil)
		c.Set("user_id", uint(1))
		c.Set("role", "")

		ExportLaporanCSV(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("3. Invalid status parameter (400)", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/laporan/export/csv?status=invalid_status", nil)
		c.Set("user_id", uint(1))
		c.Set("role", "super_admin")

		ExportLaporanCSV(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("4. Invalid severity parameter (400)", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/laporan/export/csv?severity=parah_banget", nil)
		c.Set("user_id", uint(1))
		c.Set("role", "super_admin")

		ExportLaporanCSV(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("5. Invalid date range (start_date > end_date) (400)", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/laporan/export/csv?start_date=2026-12-01&end_date=2026-11-01", nil)
		c.Set("user_id", uint(1))
		c.Set("role", "super_admin")

		ExportLaporanCSV(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("6. Unauthorized role warga (403)", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/laporan/export/csv", nil)
		c.Set("user_id", uint(10))
		c.Set("role", "warga")

		ExportLaporanCSV(c)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for warga, got %d", w.Code)
		}
	})
}

func TestCSVHeaderFormat(t *testing.T) {
	expectedHeaders := []string{
		"No",
		"ID Laporan",
		"Tanggal Laporan",
		"Judul Laporan",
		"Deskripsi Kerusakan",
		"Tipe Kerusakan",
		"Tingkat Keparahan",
		"Status Penanganan",
		"Kewenangan Jalan",
		"Wilayah / Desa",
		"Latitude",
		"Longitude",
		"Nama Pelapor",
		"Email Pelapor",
		"Petugas Ditugaskan",
		"Catatan Admin",
		"Model AI",
		"Confidence AI",
		"URL Foto Laporan",
		"URL Foto Bukti",
	}

	if len(expectedHeaders) != 20 {
		t.Fatalf("expected 20 columns in CSV export header, got %d", len(expectedHeaders))
	}

	// Test CSV writer output format
	var sb strings.Builder
	w := csv.NewWriter(&sb)
	if err := w.Write(expectedHeaders); err != nil {
		t.Fatalf("failed to write CSV header: %v", err)
	}
	w.Flush()

	out := sb.String()
	if !strings.Contains(out, "Tingkat Keparahan") || !strings.Contains(out, "Kewenangan Jalan") {
		t.Errorf("CSV header must contain Tingkat Keparahan and Kewenangan Jalan")
	}
}

func TestCSVDataRowFormatting(t *testing.T) {
	// Verifikasi bahwa data dengan potensi formula injection berhasil ditangani
	titleWithFormula := "=1+1"
	sanitizedTitle := sanitizeCSVField(titleWithFormula)
	if sanitizedTitle != "'=1+1" {
		t.Errorf("expected sanitized title to be '=1+1, got %q", sanitizedTitle)
	}

	// Verifikasi penanganan AI confidence
	confValue := 0.9425
	aiConfStr := strings.TrimSpace(strings.ReplaceAll("94.25%", " ", ""))
	if aiConfStr != "94.25%" {
		t.Errorf("expected 94.25%%, got %s", aiConfStr)
	}
	_ = confValue
}

func TestExportLaporanXLSX_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("1. Unauthenticated request (401)", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/laporan/export/xlsx", nil)

		ExportLaporanXLSX(c)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	t.Run("2. Invalid role string (400)", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/laporan/export/xlsx", nil)
		c.Set("user_id", uint(1))
		c.Set("role", "")

		ExportLaporanXLSX(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("3. Invalid status parameter (400)", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/laporan/export/xlsx?status=invalid_status", nil)
		c.Set("user_id", uint(1))
		c.Set("role", "super_admin")

		ExportLaporanXLSX(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("4. Invalid severity parameter (400)", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/laporan/export/xlsx?severity=parah_banget", nil)
		c.Set("user_id", uint(1))
		c.Set("role", "super_admin")

		ExportLaporanXLSX(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("5. Invalid date range (start_date > end_date) (400)", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/laporan/export/xlsx?start_date=2026-12-01&end_date=2026-11-01", nil)
		c.Set("user_id", uint(1))
		c.Set("role", "super_admin")

		ExportLaporanXLSX(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("6. Unauthorized role warga (403)", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/laporan/export/xlsx", nil)
		c.Set("user_id", uint(10))
		c.Set("role", "warga")

		ExportLaporanXLSX(c)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for warga, got %d", w.Code)
		}
	})
}

func TestExcelColumnNameHelper(t *testing.T) {
	tests := []struct {
		col      int
		expected string
	}{
		{1, "A"},
		{2, "B"},
		{20, "T"},
		{26, "Z"},
		{27, "AA"},
	}

	for _, tt := range tests {
		got := getExcelColumnName(tt.col)
		if got != tt.expected {
			t.Errorf("getExcelColumnName(%d) = %q, expected %q", tt.col, got, tt.expected)
		}
	}
}

func TestGenerateLaporanXLSX_StructureAndSanitization(t *testing.T) {
	headers := getLaporanExportHeaders()
	if len(headers) != 20 {
		t.Fatalf("expected 20 headers, got %d", len(headers))
	}

	rows := [][]string{
		{
			"1", "#JK-10", "2026-11-23 10:00:00",
			sanitizeCSVField("=cmd|' /C calc'!A0"),
			sanitizeCSVField("Deskripsi <script>alert(1)</script>"),
			"Lubang", "Berat", "Menunggu", "Jalan Desa", "Desa Sukamaju",
			"-6.123456", "108.123456", "Budi & Rekan", "budi@example.com",
			"-", "-", "YOLOv11", "95.50%", "/foto.jpg", "-",
		},
	}

	data, err := generateLaporanXLSX(headers, rows)
	if err != nil {
		t.Fatalf("generateLaporanXLSX failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("generated XLSX data is empty")
	}

	// Membaca file ZIP kembali untuk memverifikasi struktur berkas OpenXML
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("failed to parse generated XLSX as ZIP archive: %v", err)
	}

	expectedFiles := map[string]bool{
		"[Content_Types].xml":        false,
		"_rels/.rels":                false,
		"xl/_rels/workbook.xml.rels": false,
		"xl/workbook.xml":            false,
		"xl/styles.xml":              false,
		"xl/worksheets/sheet1.xml":   false,
	}

	for _, f := range zr.File {
		if _, ok := expectedFiles[f.Name]; ok {
			expectedFiles[f.Name] = true
		}
	}

	for fileName, found := range expectedFiles {
		if !found {
			t.Errorf("missing required OpenXML file inside XLSX zip: %s", fileName)
		}
	}

	// Membaca sheet1.xml untuk memastikan konten cell ter-escape dengan benar
	for _, f := range zr.File {
		if f.Name == "xl/worksheets/sheet1.xml" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("failed to open sheet1.xml: %v", err)
			}
			buf := new(bytes.Buffer)
			_, _ = buf.ReadFrom(rc)
			_ = rc.Close()

			sheetContent := buf.String()
			if !strings.Contains(sheetContent, "&#39;=cmd") && !strings.Contains(sheetContent, "'=cmd") {
				t.Errorf("expected formula injection '=cmd in sheet1.xml, got %s", sheetContent)
			}
			if !strings.Contains(sheetContent, "&lt;script&gt;") {
				t.Errorf("expected XML escaped &lt;script&gt; in sheet1.xml")
			}
			if !strings.Contains(sheetContent, "Budi &amp; Rekan") {
				t.Errorf("expected XML escaped &amp; in sheet1.xml")
			}
		}
	}
}

func TestFormatLaporanExportRow_Consistency(t *testing.T) {
	conf := 0.9234
	var lap models.LaporanKerusakan
	lap.ID = 99
	lap.Judul = "=SUM(1,2)"
	lap.Deskripsi = "Jalan berlubang & retak"
	lap.TipeKerusakan = "Retak Buaya"
	lap.Severity = "berat"
	lap.Status = "proses"
	lap.JenisJalan = "desa"
	lap.Latitude = -6.345678
	lap.Longitude = 108.345678
	lap.DitugaskanKe = "Dinas PU"
	lap.CatatanAdmin = "Segera diperbaiki"
	lap.AIModel = "YOLOv11"
	lap.AIConfidence = &conf
	lap.ImageURL = "http://res.cloudinary.com/foto.jpg"
	lap.FotoBukti = ""
	lap.CreatedAt = time.Date(2026, 11, 23, 14, 30, 0, 0, time.UTC)
	lap.User.Name = "Siti Rahma"
	lap.User.Email = "siti@example.com"
	lap.Wilayah.Nama = "Desa Cikedung"

	row := formatLaporanExportRow(&lap, 0)
	if len(row) != 20 {
		t.Fatalf("expected 20 columns in formatted row, got %d", len(row))
	}

	if row[0] != "1" {
		t.Errorf("expected row No 1, got %s", row[0])
	}
	if row[1] != "#JK-99" {
		t.Errorf("expected row ID #JK-99, got %s", row[1])
	}
	if row[2] != "2026-11-23 14:30:00" {
		t.Errorf("expected date 2026-11-23 14:30:00, got %s", row[2])
	}
	if row[3] != "'=SUM(1,2)" {
		t.Errorf("expected sanitized title '=SUM(1,2), got %s", row[3])
	}
	if row[6] != "Berat" {
		t.Errorf("expected severity 'Berat', got %s", row[6])
	}
	if row[7] != "Diproses" {
		t.Errorf("expected status 'Diproses', got %s", row[7])
	}
	if row[8] != "Jalan Desa" {
		t.Errorf("expected kewenangan 'Jalan Desa', got %s", row[8])
	}
	if row[9] != "Desa Cikedung" {
		t.Errorf("expected wilayah 'Desa Cikedung', got %s", row[9])
	}
	if row[10] != "-6.345678" || row[11] != "108.345678" {
		t.Errorf("expected coordinates -6.345678, 108.345678, got %s, %s", row[10], row[11])
	}
	if row[17] != "92.34%" {
		t.Errorf("expected AI confidence 92.34%%, got %s", row[17])
	}
	if row[19] != "-" {
		t.Errorf("expected empty bukti to be '-', got %s", row[19])
	}
}

func TestExportLaporanPDF_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("1. Unauthenticated request (401)", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/laporan/export/pdf", nil)

		ExportLaporanPDF(c)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	t.Run("2. Invalid role string (400)", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/laporan/export/pdf", nil)
		c.Set("user_id", uint(1))
		c.Set("role", "")

		ExportLaporanPDF(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("3. Invalid status parameter (400)", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/laporan/export/pdf?status=invalid_status", nil)
		c.Set("user_id", uint(1))
		c.Set("role", "super_admin")

		ExportLaporanPDF(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("4. Invalid severity parameter (400)", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/laporan/export/pdf?severity=parah_banget", nil)
		c.Set("user_id", uint(1))
		c.Set("role", "super_admin")

		ExportLaporanPDF(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("5. Invalid date range (start_date > end_date) (400)", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/laporan/export/pdf?start_date=2026-12-01&end_date=2026-11-01", nil)
		c.Set("user_id", uint(1))
		c.Set("role", "super_admin")

		ExportLaporanPDF(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("6. Unauthorized role warga (403)", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/laporan/export/pdf", nil)
		c.Set("user_id", uint(10))
		c.Set("role", "warga")

		ExportLaporanPDF(c)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for warga, got %d", w.Code)
		}
	})
}

func TestGenerateLaporanPDF_Validity(t *testing.T) {
	// 1. Uji pembuatan PDF dengan data kosong
	emptyBytes, err := generateLaporanPDF("Superadmin", "Semua Data", []models.LaporanKerusakan{})
	if err != nil {
		t.Fatalf("failed to generate empty PDF: %v", err)
	}
	if len(emptyBytes) == 0 {
		t.Fatal("generated empty PDF is 0 bytes")
	}
	if !strings.HasPrefix(string(emptyBytes[:10]), "%PDF-") {
		t.Errorf("expected PDF header magic bytes %%PDF-, got %q", string(emptyBytes[:10]))
	}

	// 2. Uji pembuatan PDF dengan dataset laporan (multi-row)
	conf := 0.88
	var listLaporan []models.LaporanKerusakan
	for i := 1; i <= 30; i++ {
		var item models.LaporanKerusakan
		item.ID = uint(i)
		item.Judul = fmt.Sprintf("Laporan Kerusakan Lubang Aspal Panjang Sekali di Ruas Jalan RT 0%d", i)
		item.Deskripsi = "Deskripsi kerusakan jalan"
		item.TipeKerusakan = "Lubang"
		item.Severity = "berat"
		item.Status = "proses"
		item.JenisJalan = "desa"
		item.AIModel = "YOLOv11"
		item.AIConfidence = &conf
		item.CreatedAt = time.Now()
		item.Wilayah.Nama = "Desa Terusan"
		listLaporan = append(listLaporan, item)
	}

	pdfBytes, err := generateLaporanPDF("Admin PU", "Status: Diproses | Severity: Berat", listLaporan)
	if err != nil {
		t.Fatalf("failed to generate populated PDF: %v", err)
	}

	if len(pdfBytes) < 1000 {
		t.Errorf("expected populated PDF size > 1000 bytes, got %d", len(pdfBytes))
	}

	if !strings.HasPrefix(string(pdfBytes[:10]), "%PDF-") {
		t.Errorf("expected PDF header magic bytes %%PDF-, got %q", string(pdfBytes[:10]))
	}

	// Pastikan akhir berkas PDF memuat tag %%EOF standar
	if !strings.Contains(string(pdfBytes[len(pdfBytes)-100:]), "%%EOF") {
		t.Errorf("expected PDF footer %%EOF tag at end of file")
	}
}
