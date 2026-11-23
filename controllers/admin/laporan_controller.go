package admin

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"

	"github.com/gin-gonic/gin"
	"github.com/go-pdf/fpdf"
	"gorm.io/gorm"
)

type adminLaporanQueryConfig struct {
	Query       *gorm.DB
	OrderClause string
	StatusCode  int
	Message     string
}

// buildAdminLaporanQuery membangun query laporan terproteksi dengan validasi filter dan hak akses RBAC.
func buildAdminLaporanQuery(c *gin.Context) adminLaporanQueryConfig {
	roleVal, existsRole := c.Get("role")
	userIDVal, existsUserID := c.Get("user_id")
	if !existsRole || !existsUserID {
		return adminLaporanQueryConfig{
			StatusCode: http.StatusUnauthorized,
			Message:    "User tidak terautentikasi",
		}
	}

	role, okRole := roleVal.(string)
	if !okRole || role == "" {
		return adminLaporanQueryConfig{
			StatusCode: http.StatusBadRequest,
			Message:    "Role tidak valid",
		}
	}

	if role != "admin_pemdes" && role != "admin_pu" && role != "super_admin" {
		return adminLaporanQueryConfig{
			StatusCode: http.StatusForbidden,
			Message:    "Akses tidak diizinkan",
		}
	}

	var userID uint
	switch v := userIDVal.(type) {
	case uint:
		userID = v
	case float64:
		userID = uint(v)
	case int:
		userID = uint(v)
	default:
		return adminLaporanQueryConfig{
			StatusCode: http.StatusBadRequest,
			Message:    "User ID tidak valid",
		}
	}

	statusFilter := strings.ToLower(strings.TrimSpace(c.Query("status")))
	severityFilter := strings.ToLower(strings.TrimSpace(c.Query("severity")))
	searchKeyword := strings.TrimSpace(c.Query("search"))
	jenisJalanFilter := strings.ToLower(strings.TrimSpace(c.Query("jenis_jalan")))
	wilayahIDParam := strings.TrimSpace(c.Query("wilayah_id"))
	startDateStr := strings.TrimSpace(c.Query("start_date"))
	endDateStr := strings.TrimSpace(c.Query("end_date"))
	sortByParam := strings.ToLower(strings.TrimSpace(c.Query("sort_by")))
	if sortByParam == "" {
		sortByParam = strings.ToLower(strings.TrimSpace(c.Query("sort")))
	}

	// Validasi filter status
	if statusFilter != "" && statusFilter != "all" {
		switch statusFilter {
		case "menunggu", "proses", "selesai", "ditolak":
			// valid
		default:
			return adminLaporanQueryConfig{
				StatusCode: http.StatusBadRequest,
				Message:    "Parameter status tidak valid. Gunakan 'menunggu', 'proses', 'selesai', atau 'ditolak'",
			}
		}
	}

	// Validasi filter jenis_jalan
	if jenisJalanFilter != "" && jenisJalanFilter != "all" {
		switch jenisJalanFilter {
		case "desa", "kabupaten", "provinsi", "nasional":
			// valid
		default:
			return adminLaporanQueryConfig{
				StatusCode: http.StatusBadRequest,
				Message:    "Parameter jenis_jalan tidak valid. Gunakan 'desa', 'kabupaten', 'provinsi', atau 'nasional'",
			}
		}
	}

	// Validasi filter wilayah_id jika ada
	var hasWilayahFilter bool
	var filterWilayahID uint
	if wilayahIDParam != "" && strings.ToLower(wilayahIDParam) != "all" {
		parsedWID, errWID := strconv.ParseUint(wilayahIDParam, 10, 32)
		if errWID != nil || parsedWID == 0 {
			return adminLaporanQueryConfig{
				StatusCode: http.StatusBadRequest,
				Message:    "Parameter wilayah_id tidak valid",
			}
		}
		filterWilayahID = uint(parsedWID)
		hasWilayahFilter = true
	}

	// Validasi filter tanggal jika diberikan
	const dateLayout = "2006-01-02"
	var startDate, endDate time.Time
	var hasStartDate, hasEndDate bool

	if startDateStr != "" {
		parsedStart, err := time.ParseInLocation(dateLayout, startDateStr, time.Local)
		if err != nil {
			return adminLaporanQueryConfig{
				StatusCode: http.StatusBadRequest,
				Message:    "Format start_date tidak valid. Gunakan format YYYY-MM-DD",
			}
		}
		startDate = parsedStart
		hasStartDate = true
	}

	if endDateStr != "" {
		parsedEnd, err := time.ParseInLocation(dateLayout, endDateStr, time.Local)
		if err != nil {
			return adminLaporanQueryConfig{
				StatusCode: http.StatusBadRequest,
				Message:    "Format end_date tidak valid. Gunakan format YYYY-MM-DD",
			}
		}
		endDate = parsedEnd
		hasEndDate = true
	}

	if hasStartDate && hasEndDate && startDate.After(endDate) {
		return adminLaporanQueryConfig{
			StatusCode: http.StatusBadRequest,
			Message:    "start_date tidak boleh melebihi end_date",
		}
	}

	// Validasi dan penentuan klausul sorting deterministik
	var orderClause string
	switch sortByParam {
	case "", "newest", "terbaru":
		orderClause = "laporan_kerusakan.created_at DESC, laporan_kerusakan.id DESC"
	case "oldest", "terlama":
		orderClause = "laporan_kerusakan.created_at ASC, laporan_kerusakan.id ASC"
	default:
		return adminLaporanQueryConfig{
			StatusCode: http.StatusBadRequest,
			Message:    "Parameter sort_by tidak valid. Gunakan 'newest'/'terbaru' atau 'oldest'/'terlama'",
		}
	}

	if severityFilter != "" && severityFilter != "all" {
		switch severityFilter {
		case "ringan", "sedang", "berat", "belum_dinilai":
			// valid
		default:
			return adminLaporanQueryConfig{
				StatusCode: http.StatusBadRequest,
				Message:    "Parameter severity tidak valid. Gunakan 'ringan', 'sedang', 'berat', atau 'belum_dinilai'",
			}
		}
	}

	if config.DB == nil {
		return adminLaporanQueryConfig{
			StatusCode: http.StatusInternalServerError,
			Message:    "Koneksi database tidak tersedia",
		}
	}

	query := config.DB.Model(&models.LaporanKerusakan{}).
		Preload("User").
		Preload("Wilayah").
		Joins("LEFT JOIN user ON user.id = laporan_kerusakan.user_id AND user.deleted_at IS NULL").
		Where("laporan_kerusakan.deleted_at IS NULL")

	switch role {
	case "admin_pemdes":
		var adminUser models.User
		if err := config.DB.First(&adminUser, userID).Error; err != nil {
			return adminLaporanQueryConfig{
				StatusCode: http.StatusNotFound,
				Message:    "Data admin tidak ditemukan",
			}
		}

		if adminUser.WilayahID == nil || *adminUser.WilayahID == 0 {
			return adminLaporanQueryConfig{
				StatusCode: http.StatusForbidden,
				Message:    "Admin Pemdes belum memiliki wilayah",
			}
		}

		if hasWilayahFilter && filterWilayahID != *adminUser.WilayahID {
			return adminLaporanQueryConfig{
				StatusCode: http.StatusForbidden,
				Message:    "Admin Pemdes tidak dapat mengakses wilayah di luar kewenangannya",
			}
		}

		if jenisJalanFilter != "" && jenisJalanFilter != "all" && jenisJalanFilter != "desa" {
			return adminLaporanQueryConfig{
				StatusCode: http.StatusForbidden,
				Message:    "Admin Pemdes hanya memiliki kewenangan pada jalan desa",
			}
		}

		query = query.Where("laporan_kerusakan.wilayah_id = ? AND laporan_kerusakan.jenis_jalan = ?", *adminUser.WilayahID, "desa")
	case "admin_pu":
		// PU-5.1: Admin PU dapat melihat laporan dari SEMUA kewenangan jalan
		if jenisJalanFilter != "" && jenisJalanFilter != "all" {
			query = query.Where("laporan_kerusakan.jenis_jalan = ?", jenisJalanFilter)
		}
		if hasWilayahFilter {
			query = query.Where("laporan_kerusakan.wilayah_id = ?", filterWilayahID)
		}
	case "super_admin":
		// Superadmin melihat seluruh laporan aktif dengan filter opsional jenis_jalan dan wilayah_id
		if jenisJalanFilter != "" && jenisJalanFilter != "all" {
			query = query.Where("laporan_kerusakan.jenis_jalan = ?", jenisJalanFilter)
		}
		if hasWilayahFilter {
			query = query.Where("laporan_kerusakan.wilayah_id = ?", filterWilayahID)
		}
	default:
		return adminLaporanQueryConfig{
			StatusCode: http.StatusForbidden,
			Message:    "Akses tidak diizinkan",
		}
	}

	if statusFilter != "" && statusFilter != "all" {
		query = query.Where("laporan_kerusakan.status = ?", statusFilter)
	}

	if severityFilter != "" && severityFilter != "all" {
		switch severityFilter {
		case "ringan", "sedang", "berat":
			query = query.Where("laporan_kerusakan.severity = ?", severityFilter)
		case "belum_dinilai":
			query = query.Where("laporan_kerusakan.severity = ? OR laporan_kerusakan.severity IS NULL OR laporan_kerusakan.severity = ''", "belum_dinilai")
		}
	}

	if searchKeyword != "" {
		likePattern := "%" + searchKeyword + "%"
		cleanIDStr := strings.TrimPrefix(searchKeyword, "#")
		if numID, errID := strconv.ParseUint(cleanIDStr, 10, 32); errID == nil && numID > 0 {
			query = query.Where(
				"laporan_kerusakan.judul LIKE ? OR laporan_kerusakan.deskripsi LIKE ? OR laporan_kerusakan.tipe_kerusakan LIKE ? OR user.name LIKE ? OR laporan_kerusakan.id = ?",
				likePattern, likePattern, likePattern, likePattern, numID,
			)
		} else {
			query = query.Where(
				"laporan_kerusakan.judul LIKE ? OR laporan_kerusakan.deskripsi LIKE ? OR laporan_kerusakan.tipe_kerusakan LIKE ? OR user.name LIKE ?",
				likePattern, likePattern, likePattern, likePattern,
			)
		}
	}

	if hasStartDate {
		query = query.Where("laporan_kerusakan.created_at >= ?", startDate)
	}

	if hasEndDate {
		endOfDayExclusive := endDate.AddDate(0, 0, 1)
		query = query.Where("laporan_kerusakan.created_at < ?", endOfDayExclusive)
	}

	return adminLaporanQueryConfig{
		Query:       query,
		OrderClause: orderClause,
	}
}

func GetAllLaporan(c *gin.Context) {
	cfg := buildAdminLaporanQuery(c)
	if cfg.StatusCode != 0 {
		c.JSON(cfg.StatusCode, gin.H{
			"status":  "error",
			"message": cfg.Message,
		})
		return
	}

	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")

	page, errPage := strconv.Atoi(pageStr)
	if errPage != nil || page < 1 {
		page = 1
	}
	limit, errLimit := strconv.Atoi(limitStr)
	if errLimit != nil || limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	var totalData int64
	cfg.Query.Count(&totalData)

	listLaporan := make([]models.LaporanKerusakan, 0)
	err := cfg.Query.Order(cfg.OrderClause).
		Limit(limit).
		Offset(offset).
		Find(&listLaporan).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil laporan kerusakan",
			"error":   "Gagal mengambil laporan kerusakan",
		})
		return
	}

	for i := range listLaporan {
		if listLaporan[i].Severity == "" {
			listLaporan[i].Severity = "belum_dinilai"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Laporan kerusakan berhasil diambil",
		"data":    listLaporan,
		"total":   totalData,
	})
}

const maxExportRows = 10000

// getLaporanExportHeaders mengembalikan 20 nama kolom standar untuk ekspor laporan (CSV & XLSX).
func getLaporanExportHeaders() []string {
	return []string{
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
}

// formatLaporanExportRow memetakan data entitas laporan ke 20 kolom terstandarisasi
// dengan sanitasi formula injection dan formatting yang aman.
func formatLaporanExportRow(lap *models.LaporanKerusakan, index int) []string {
	sev := lap.Severity
	if sev == "" {
		sev = "belum_dinilai"
	}
	var sevLabel string
	switch strings.ToLower(sev) {
	case "ringan":
		sevLabel = "Ringan"
	case "sedang":
		sevLabel = "Sedang"
	case "berat":
		sevLabel = "Berat"
	default:
		sevLabel = "Belum Dinilai"
	}

	var statusLabel string
	switch strings.ToLower(lap.Status) {
	case "menunggu":
		statusLabel = "Menunggu"
	case "proses":
		statusLabel = "Diproses"
	case "selesai":
		statusLabel = "Selesai"
	case "ditolak":
		statusLabel = "Ditolak"
	default:
		statusLabel = lap.Status
	}

	var kewenanganLabel string
	switch strings.ToLower(lap.JenisJalan) {
	case "desa":
		kewenanganLabel = "Jalan Desa"
	case "kabupaten":
		kewenanganLabel = "Jalan Kabupaten"
	case "provinsi":
		kewenanganLabel = "Jalan Provinsi"
	case "nasional":
		kewenanganLabel = "Jalan Nasional"
	default:
		kewenanganLabel = "Belum Teridentifikasi"
	}

	wilayahNama := lap.Wilayah.Nama
	if wilayahNama == "" {
		wilayahNama = "-"
	}

	pelaporNama := lap.User.Name
	if pelaporNama == "" {
		pelaporNama = "Warga"
	}

	aiConfStr := "-"
	if lap.AIConfidence != nil {
		aiConfStr = fmt.Sprintf("%.2f%%", *lap.AIConfidence*100)
	}

	aiModelStr := lap.AIModel
	if aiModelStr == "" {
		aiModelStr = "-"
	}

	petugasStr := lap.DitugaskanKe
	if petugasStr == "" {
		petugasStr = "-"
	}

	catatanStr := lap.CatatanAdmin
	if catatanStr == "" {
		catatanStr = "-"
	}

	buktiStr := lap.FotoBukti
	if buktiStr == "" {
		buktiStr = "-"
	}

	fotoLaporan := lap.ImageURL
	if fotoLaporan == "" {
		fotoLaporan = "-"
	}

	return []string{
		strconv.Itoa(index + 1),
		fmt.Sprintf("#JK-%d", lap.ID),
		lap.CreatedAt.Format("2006-01-02 15:04:05"),
		sanitizeCSVField(lap.Judul),
		sanitizeCSVField(lap.Deskripsi),
		sanitizeCSVField(lap.TipeKerusakan),
		sevLabel,
		statusLabel,
		kewenanganLabel,
		sanitizeCSVField(wilayahNama),
		fmt.Sprintf("%.6f", lap.Latitude),
		fmt.Sprintf("%.6f", lap.Longitude),
		sanitizeCSVField(pelaporNama),
		sanitizeCSVField(lap.User.Email),
		sanitizeCSVField(petugasStr),
		sanitizeCSVField(catatanStr),
		sanitizeCSVField(aiModelStr),
		aiConfStr,
		fotoLaporan,
		buktiStr,
	}
}

// getExcelColumnName mengonversi indeks kolom 1-based ke huruf alfabet kolom Excel (1 -> A, 20 -> T).
func getExcelColumnName(col int) string {
	name := ""
	for col > 0 {
		col--
		name = string(rune('A'+(col%26))) + name
		col /= 26
	}
	return name
}

// escapeXML mengamankan karakter khusus XML.
func escapeXML(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

// generateLaporanXLSX menghasilkan berkas OpenXML SpreadsheetML (.xlsx) yang valid
// menggunakan standard library archive/zip dan encoding/xml tanpa dependensi eksternal.
func generateLaporanXLSX(headers []string, rows [][]string) ([]byte, error) {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	// 1. [Content_Types].xml
	contentTypes := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
</Types>`
	if f, err := zw.Create("[Content_Types].xml"); err != nil {
		return nil, err
	} else if _, err := f.Write([]byte(contentTypes)); err != nil {
		return nil, err
	}

	// 2. _rels/.rels
	rootRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`
	if f, err := zw.Create("_rels/.rels"); err != nil {
		return nil, err
	} else if _, err := f.Write([]byte(rootRels)); err != nil {
		return nil, err
	}

	// 3. xl/_rels/workbook.xml.rels
	wbRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`
	if f, err := zw.Create("xl/_rels/workbook.xml.rels"); err != nil {
		return nil, err
	} else if _, err := f.Write([]byte(wbRels)); err != nil {
		return nil, err
	}

	// 4. xl/workbook.xml
	wb := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>
    <sheet name="Laporan Jalan Rusak" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`
	if f, err := zw.Create("xl/workbook.xml"); err != nil {
		return nil, err
	} else if _, err := f.Write([]byte(wb)); err != nil {
		return nil, err
	}

	// 5. xl/styles.xml
	styles := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <fonts count="2">
    <font><sz val="11"/><color rgb="FF1F2937"/><name val="Segoe UI"/><family val="2"/></font>
    <font><b/><sz val="11"/><color rgb="FF0F172A"/><name val="Segoe UI"/><family val="2"/></font>
  </fonts>
  <fills count="3">
    <fill><patternFill patternType="none"/></fill>
    <fill><patternFill patternType="gray125"/></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFEBF2FA"/></patternFill></fill>
  </fills>
  <borders count="2">
    <border><left/><right/><top/><bottom/><diagonal/></border>
    <border>
      <left style="thin"><color rgb="FFE2E8F0"/></left>
      <right style="thin"><color rgb="FFE2E8F0"/></right>
      <top style="thin"><color rgb="FFE2E8F0"/></top>
      <bottom style="thin"><color rgb="FFE2E8F0"/></bottom>
    </border>
  </borders>
  <cellStyleXfs count="1">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0"/>
  </cellStyleXfs>
  <cellXfs count="3">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>
    <xf numFmtId="0" fontId="1" fillId="2" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1"/>
    <xf numFmtId="0" fontId="0" fillId="0" borderId="1" xfId="0" applyBorder="1"/>
  </cellXfs>
</styleSheet>`
	if f, err := zw.Create("xl/styles.xml"); err != nil {
		return nil, err
	} else if _, err := f.Write([]byte(styles)); err != nil {
		return nil, err
	}

	// 6. xl/worksheets/sheet1.xml
	var sheetBuf strings.Builder
	sheetBuf.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <cols>
    <col min="1" max="1" width="6" customWidth="1"/>
    <col min="2" max="2" width="12" customWidth="1"/>
    <col min="3" max="3" width="20" customWidth="1"/>
    <col min="4" max="4" width="28" customWidth="1"/>
    <col min="5" max="5" width="35" customWidth="1"/>
    <col min="6" max="6" width="18" customWidth="1"/>
    <col min="7" max="7" width="18" customWidth="1"/>
    <col min="8" max="8" width="16" customWidth="1"/>
    <col min="9" max="9" width="18" customWidth="1"/>
    <col min="10" max="10" width="20" customWidth="1"/>
    <col min="11" max="11" width="14" customWidth="1"/>
    <col min="12" max="12" width="14" customWidth="1"/>
    <col min="13" max="13" width="22" customWidth="1"/>
    <col min="14" max="14" width="25" customWidth="1"/>
    <col min="15" max="15" width="22" customWidth="1"/>
    <col min="16" max="16" width="28" customWidth="1"/>
    <col min="17" max="17" width="18" customWidth="1"/>
    <col min="18" max="18" width="16" customWidth="1"/>
    <col min="19" max="19" width="30" customWidth="1"/>
    <col min="20" max="20" width="30" customWidth="1"/>
  </cols>
  <sheetData>`)

	// Header row (r=1, style=1)
	sheetBuf.WriteString("\n    <row r=\"1\">")
	for cIdx, h := range headers {
		colLetter := getExcelColumnName(cIdx + 1)
		cellRef := fmt.Sprintf("%s1", colLetter)
		sheetBuf.WriteString(fmt.Sprintf(`<c r="%s" t="inlineStr" s="1"><is><t>%s</t></is></c>`, cellRef, escapeXML(h)))
	}
	sheetBuf.WriteString("</row>")

	// Data rows (style=2)
	for rIdx, row := range rows {
		rowNum := rIdx + 2
		sheetBuf.WriteString(fmt.Sprintf("\n    <row r=\"%d\">", rowNum))
		for cIdx, val := range row {
			colLetter := getExcelColumnName(cIdx + 1)
			cellRef := fmt.Sprintf("%s%d", colLetter, rowNum)
			sheetBuf.WriteString(fmt.Sprintf(`<c r="%s" t="inlineStr" s="2"><is><t>%s</t></is></c>`, cellRef, escapeXML(val)))
		}
		sheetBuf.WriteString("</row>")
	}

	sheetBuf.WriteString("\n  </sheetData>\n</worksheet>")

	if f, err := zw.Create("xl/worksheets/sheet1.xml"); err != nil {
		return nil, err
	} else if _, err := f.Write([]byte(sheetBuf.String())); err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// ExportLaporanCSV mengekspor data laporan kerusakan terfilter ke format CSV
// dengan sanitasi formula injection, UTF-8 BOM, dan pembatasan baris yang aman.
func ExportLaporanCSV(c *gin.Context) {
	cfg := buildAdminLaporanQuery(c)
	if cfg.StatusCode != 0 {
		c.JSON(cfg.StatusCode, gin.H{
			"status":  "error",
			"message": cfg.Message,
		})
		return
	}

	var totalData int64
	if err := cfg.Query.Count(&totalData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menghitung jumlah data laporan: " + err.Error(),
		})
		return
	}

	if totalData > maxExportRows {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Jumlah data yang akan diekspor (%d baris) melebihi batas maksimal %d baris. Silakan persempit filter tanggal atau wilayah Anda.", totalData, maxExportRows),
		})
		return
	}

	listLaporan := make([]models.LaporanKerusakan, 0)
	err := cfg.Query.Order(cfg.OrderClause).
		Limit(maxExportRows).
		Find(&listLaporan).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal memproses ekspor data laporan: " + err.Error(),
		})
		return
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("laporan-jalan-rusak-%s.csv", timestamp)

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	// UTF-8 BOM untuk kompatibilitas Microsoft Excel
	_, _ = c.Writer.Write([]byte("\xef\xbb\xbf"))

	writer := csv.NewWriter(c.Writer)
	header := getLaporanExportHeaders()
	if err := writer.Write(header); err != nil {
		return
	}

	for i, lap := range listLaporan {
		row := formatLaporanExportRow(&lap, i)
		_ = writer.Write(row)
	}

	writer.Flush()
}

// ExportLaporanXLSX mengekspor data laporan kerusakan terfilter ke format Microsoft Excel (.xlsx)
// menggunakan struktur OpenXML SpreadsheetML native, sanitasi formula injection, dan penegakan RBAC.
func ExportLaporanXLSX(c *gin.Context) {
	cfg := buildAdminLaporanQuery(c)
	if cfg.StatusCode != 0 {
		c.JSON(cfg.StatusCode, gin.H{
			"status":  "error",
			"message": cfg.Message,
		})
		return
	}

	var totalData int64
	if err := cfg.Query.Count(&totalData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menghitung jumlah data laporan: " + err.Error(),
		})
		return
	}

	if totalData > maxExportRows {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Jumlah data yang akan diekspor (%d baris) melebihi batas maksimal %d baris. Silakan persempit filter tanggal atau wilayah Anda.", totalData, maxExportRows),
		})
		return
	}

	listLaporan := make([]models.LaporanKerusakan, 0)
	err := cfg.Query.Order(cfg.OrderClause).
		Limit(maxExportRows).
		Find(&listLaporan).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal memproses ekspor data laporan: " + err.Error(),
		})
		return
	}

	headers := getLaporanExportHeaders()
	rows := make([][]string, len(listLaporan))
	for i, lap := range listLaporan {
		rows[i] = formatLaporanExportRow(&lap, i)
	}

	xlsxBytes, err := generateLaporanXLSX(headers, rows)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menghasilkan berkas Excel (.xlsx): " + err.Error(),
		})
		return
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("laporan-jalan-rusak-%s.xlsx", timestamp)

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("Content-Length", strconv.Itoa(len(xlsxBytes)))
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	_, _ = c.Writer.Write(xlsxBytes)
}

const maxPDFRows = 1000

func sanitizePDFText(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 32 && r <= 126 {
			b.WriteRune(r)
		} else if r == 0x0A || r == 0x0D {
			b.WriteByte(' ')
		} else {
			b.WriteByte(' ')
		}
	}
	return strings.TrimSpace(b.String())
}

func truncatePDFText(s string, maxLen int) string {
	clean := sanitizePDFText(s)
	if len(clean) > maxLen {
		if maxLen > 3 {
			return clean[:maxLen-3] + "..."
		}
		return clean[:maxLen]
	}
	return clean
}

func generateLaporanPDF(scopeStr, filterSummaryStr string, listLaporan []models.LaporanKerusakan) ([]byte, error) {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(15, 12, 15)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AliasNbPages("{nb}")

	exportTimeStr := time.Now().Format("02 Jan 2006, 15:04 WIB")

	tableHeaders := []struct {
		title string
		width float64
		align string
	}{
		{"No", 10, "C"},
		{"ID", 20, "C"},
		{"Tanggal", 32, "C"},
		{"Judul Kerusakan", 56, "L"},
		{"Wilayah / Desa", 36, "L"},
		{"Kewenangan", 28, "C"},
		{"Tipe Kerusakan", 31, "L"},
		{"Severity", 27, "C"},
		{"Status", 27, "C"},
	}

	drawTableHeader := func() {
		pdf.SetFont("Arial", "B", 8)
		pdf.SetFillColor(235, 242, 250) // Blue pale #EBF2FA
		pdf.SetTextColor(15, 23, 42)
		pdf.SetDrawColor(200, 215, 230)
		for _, h := range tableHeaders {
			pdf.CellFormat(h.width, 7.5, h.title, "1", 0, h.align, true, 0, "")
		}
		pdf.Ln(-1)
	}

	pdf.SetHeaderFunc(func() {
		if pdf.PageNo() == 1 {
			// Title Banner Box
			pdf.SetFillColor(15, 23, 42) // Navy deepest #0F172A
			pdf.Rect(15, 12, 267, 16, "F")

			pdf.SetFont("Arial", "B", 13)
			pdf.SetTextColor(255, 255, 255)
			pdf.SetXY(18, 15)
			pdf.CellFormat(185, 10, "LAPORAN KERUSAKAN JALAN - ROADIS", "", 0, "L", false, 0, "")

			pdf.SetFont("Arial", "", 8)
			pdf.SetTextColor(200, 220, 240)
			pdf.SetXY(205, 16)
			pdf.CellFormat(74, 8, "Waktu Ekspor: "+exportTimeStr, "", 0, "R", false, 0, "")

			// Metadata Box
			pdf.SetY(31)
			pdf.SetFont("Arial", "", 8)
			pdf.SetTextColor(50, 50, 50)
			pdf.CellFormat(0, 4.5, "Cakupan Otoritas: "+scopeStr, "", 1, "L", false, 0, "")
			pdf.CellFormat(0, 4.5, "Filter Diterapkan: "+filterSummaryStr, "", 1, "L", false, 0, "")
			pdf.CellFormat(0, 4.5, fmt.Sprintf("Total Data: %d Laporan Terverifikasi", len(listLaporan)), "", 1, "L", false, 0, "")
			pdf.Ln(2)

			drawTableHeader()
		} else {
			pdf.SetY(12)
			pdf.SetFont("Arial", "I", 8)
			pdf.SetTextColor(100, 116, 139)
			pdf.CellFormat(0, 5, fmt.Sprintf("Laporan Kerusakan Jalan - ROADIS (Lanjutan Halaman %d)", pdf.PageNo()), "", 1, "L", false, 0, "")
			pdf.Ln(1)

			drawTableHeader()
		}
	})

	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Arial", "I", 8)
		pdf.SetTextColor(120, 140, 160)
		pdf.CellFormat(0, 8, fmt.Sprintf("ROADIS - Sistem Informasi Kerusakan Jalan | Halaman %d/{nb}", pdf.PageNo()), "", 0, "C", false, 0, "")
	})

	pdf.AddPage()

	if len(listLaporan) == 0 {
		pdf.SetFont("Arial", "I", 9)
		pdf.SetTextColor(100, 116, 139)
		pdf.SetFillColor(255, 255, 255)
		pdf.CellFormat(267, 12, "Tidak ada data laporan yang sesuai dengan filter yang dipilih.", "1", 1, "C", true, 0, "")
	} else {
		pdf.SetFont("Arial", "", 8)
		pdf.SetTextColor(30, 41, 59)
		pdf.SetDrawColor(226, 232, 240)

		for i, lap := range listLaporan {
			if i%2 == 1 {
				pdf.SetFillColor(248, 250, 252)
			} else {
				pdf.SetFillColor(255, 255, 255)
			}

			// Format values konsisten dengan CSV/XLSX
			sev := lap.Severity
			if sev == "" {
				sev = "belum_dinilai"
			}
			var sevLabel string
			switch strings.ToLower(sev) {
			case "ringan":
				sevLabel = "Ringan"
			case "sedang":
				sevLabel = "Sedang"
			case "berat":
				sevLabel = "Berat"
			default:
				sevLabel = "Belum Dinilai"
			}

			var statusLabel string
			switch strings.ToLower(lap.Status) {
			case "menunggu":
				statusLabel = "Menunggu"
			case "proses":
				statusLabel = "Diproses"
			case "selesai":
				statusLabel = "Selesai"
			case "ditolak":
				statusLabel = "Ditolak"
			default:
				statusLabel = lap.Status
			}

			var kewenanganLabel string
			switch strings.ToLower(lap.JenisJalan) {
			case "desa":
				kewenanganLabel = "Jalan Desa"
			case "kabupaten":
				kewenanganLabel = "Jalan Kab."
			case "provinsi":
				kewenanganLabel = "Jalan Prov."
			case "nasional":
				kewenanganLabel = "Jalan Nas."
			default:
				kewenanganLabel = "-"
			}

			wilayahNama := lap.Wilayah.Nama
			if wilayahNama == "" {
				wilayahNama = "-"
			}

			pdf.CellFormat(10, 6.5, strconv.Itoa(i+1), "1", 0, "C", true, 0, "")
			pdf.CellFormat(20, 6.5, fmt.Sprintf("#JK-%d", lap.ID), "1", 0, "C", true, 0, "")
			pdf.CellFormat(32, 6.5, lap.CreatedAt.Format("2006-01-02 15:04"), "1", 0, "C", true, 0, "")
			pdf.CellFormat(56, 6.5, truncatePDFText(lap.Judul, 36), "1", 0, "L", true, 0, "")
			pdf.CellFormat(36, 6.5, truncatePDFText(wilayahNama, 24), "1", 0, "L", true, 0, "")
			pdf.CellFormat(28, 6.5, kewenanganLabel, "1", 0, "C", true, 0, "")
			pdf.CellFormat(31, 6.5, truncatePDFText(lap.TipeKerusakan, 20), "1", 0, "L", true, 0, "")
			pdf.CellFormat(27, 6.5, sevLabel, "1", 0, "C", true, 0, "")
			pdf.CellFormat(27, 6.5, statusLabel, "1", 1, "C", true, 0, "")
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ExportLaporanPDF mengekspor rekapitulasi data laporan terfilter ke format PDF landscape
// dengan tata letak administratif resmi, pagination multi-halaman otomatis, dan penegakan RBAC.
func ExportLaporanPDF(c *gin.Context) {
	cfg := buildAdminLaporanQuery(c)
	if cfg.StatusCode != 0 {
		c.JSON(cfg.StatusCode, gin.H{
			"status":  "error",
			"message": cfg.Message,
		})
		return
	}

	var totalData int64
	if err := cfg.Query.Count(&totalData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menghitung jumlah data laporan: " + err.Error(),
		})
		return
	}

	if totalData > maxPDFRows {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Jumlah data untuk ekspor PDF (%d baris) melebihi batas maksimal %d baris. Silakan persempit filter tanggal atau wilayah Anda, atau gunakan ekspor Excel/CSV untuk dataset besar.", totalData, maxPDFRows),
		})
		return
	}

	listLaporan := make([]models.LaporanKerusakan, 0)
	err := cfg.Query.Order(cfg.OrderClause).
		Limit(maxPDFRows).
		Find(&listLaporan).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal memproses data laporan untuk PDF: " + err.Error(),
		})
		return
	}

	// Bentuk teks cakupan wewenang
	roleVal, _ := c.Get("role")
	role, _ := roleVal.(string)
	var scopeStr string
	switch role {
	case "admin_pemdes":
		scopeStr = "Admin Pemdes (Kewenangan: Jalan Desa Wilayah Sendiri)"
	case "admin_pu":
		scopeStr = "Admin Dinas PU (Cakupan Monitoring PU-5.1: Seluruh Kewenangan Jalan)"
	case "super_admin":
		scopeStr = "Superadmin (Akses Global Seluruh Wilayah & Kewenangan)"
	default:
		scopeStr = "Pengguna Terautentikasi"
	}

	// Bentuk ringkasan filter aktif
	var filterParts []string
	if st := strings.TrimSpace(c.Query("status")); st != "" && st != "all" {
		filterParts = append(filterParts, "Status: "+st)
	}
	if sv := strings.TrimSpace(c.Query("severity")); sv != "" && sv != "all" {
		filterParts = append(filterParts, "Severity: "+sv)
	}
	if jj := strings.TrimSpace(c.Query("jenis_jalan")); jj != "" && jj != "all" {
		filterParts = append(filterParts, "Jenis Jalan: "+jj)
	}
	if wid := strings.TrimSpace(c.Query("wilayah_id")); wid != "" && wid != "all" {
		filterParts = append(filterParts, "Wilayah ID: "+wid)
	}
	startD := strings.TrimSpace(c.Query("start_date"))
	endD := strings.TrimSpace(c.Query("end_date"))
	if startD != "" || endD != "" {
		s := startD
		if s == "" {
			s = "Awal"
		}
		e := endD
		if e == "" {
			e = "Sekarang"
		}
		filterParts = append(filterParts, fmt.Sprintf("Periode: %s s/d %s", s, e))
	}
	if q := strings.TrimSpace(c.Query("search")); q != "" {
		filterParts = append(filterParts, fmt.Sprintf("Pencarian: \"%s\"", q))
	}

	filterSummaryStr := "Semua Data (Tanpa Filter Tambahan)"
	if len(filterParts) > 0 {
		filterSummaryStr = strings.Join(filterParts, " | ")
	}

	pdfBytes, err := generateLaporanPDF(scopeStr, filterSummaryStr, listLaporan)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal membuat dokumen PDF: " + err.Error(),
		})
		return
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("laporan-jalan-rusak-%s.pdf", timestamp)

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("Content-Length", strconv.Itoa(len(pdfBytes)))
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	_, _ = c.Writer.Write(pdfBytes)
}

// sanitizeCSVField mencegah serangan CSV Formula Injection (DDE)
// dengan menambahkan tanda kutip tunggal jika karakter awal adalah operator formula.
func sanitizeCSVField(val string) string {
	trimmed := strings.TrimSpace(val)
	if len(trimmed) > 0 {
		first := trimmed[0]
		if first == '=' || first == '+' || first == '-' || first == '@' || first == '\t' || first == '\r' {
			return "'" + val
		}
	}
	return val
}

func GetLaporanByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "ID laporan tidak valid",
		})
		return
	}

	roleVal, existsRole := c.Get("role")
	userIDVal, existsUserID := c.Get("user_id")
	if !existsRole || !existsUserID {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "User tidak terautentikasi",
		})
		return
	}

	role, okRole := roleVal.(string)
	if !okRole || role == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Role tidak valid",
		})
		return
	}

	var userID uint
	switch v := userIDVal.(type) {
	case uint:
		userID = v
	case float64:
		userID = uint(v)
	case int:
		userID = uint(v)
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "User ID tidak valid",
		})
		return
	}

	if config.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Koneksi database tidak tersedia",
		})
		return
	}

	var laporan models.LaporanKerusakan
	query := config.DB.Model(&models.LaporanKerusakan{}).
		Preload("User").
		Preload("Wilayah").
		Where("laporan_kerusakan.id = ? AND laporan_kerusakan.deleted_at IS NULL", id)

	switch role {
	case "admin_pemdes":
		var adminUser models.User
		if err := config.DB.First(&adminUser, userID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"status":  "error",
				"message": "Data admin tidak ditemukan",
			})
			return
		}

		if adminUser.WilayahID == nil || *adminUser.WilayahID == 0 {
			c.JSON(http.StatusForbidden, gin.H{
				"status":  "error",
				"message": "Admin Pemdes belum memiliki wilayah",
			})
			return
		}

		query = query.Where("laporan_kerusakan.wilayah_id = ? AND laporan_kerusakan.jenis_jalan = ?", *adminUser.WilayahID, "desa")
	case "admin_pu", "super_admin":
		// admin_pu dan super_admin: dapat melihat semua jenis jalan pada view detail laporan
	default:
		c.JSON(http.StatusForbidden, gin.H{
			"status":  "error",
			"message": "Akses tidak diizinkan",
		})
		return
	}

	if err := query.First(&laporan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Laporan tidak ditemukan",
		})
		return
	}

	if laporan.Severity == "" {
		laporan.Severity = "belum_dinilai"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Detail laporan berhasil diambil",
		"data":    laporan,
	})
}
