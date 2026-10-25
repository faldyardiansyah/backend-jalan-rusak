package utils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrOSMTidakTersedia menandakan layanan Overpass gagal (BUKAN "jalan tidak ada").
var ErrOSMTidakTersedia = errors.New("layanan data jalan OpenStreetMap tidak tersedia")

var (
	// Variabel (bukan const) supaya bisa diperkecil saat unit test.
	overpassAttemptTimeout = 8 * time.Second
	overpassBackoffBase    = 500 * time.Millisecond

	// Ganti dengan endpoint yang lolos tes ketersediaan Anda,
	// atau atur lewat environment variable OVERPASS_URLS (dipisah koma).
	defaultOverpassURLs = []string{
		"https://overpass-api.de/api/interpreter",
		"https://overpass.private.coffee/api/interpreter",
	}
)

const (
	// Ganti dengan email/kontak Anda. Operator Overpass/OSM meminta User-Agent yang bisa dihubungi.
	overpassUserAgent = "ROADIS/1.0 (proyek mahasiswa; kontak: GANTI-DENGAN-EMAIL-ANDA)"

	overpassBackoffMax = 4 * time.Second
	overpassCooldown   = 30 * time.Second
	overpassCacheTTL   = 24 * time.Hour
	overpassStaleMax   = 7 * 24 * time.Hour
	overpassMaxBody    = 8 << 20 // 8 MB

	overpassTileSize   = 0.001 // sekitar 110 m
	overpassTileRadius = 200   // meter, dari pusat tile
	overpassMaxRadius  = 100   // radius maksimum yang boleh diminta pemanggil

	overpassBudgetNormal   = 20 * time.Second
	overpassAttemptsNormal = 4
	overpassBudgetCepat    = 6 * time.Second
	overpassAttemptsCepat  = 2
)

// ---------- Tipe data respons Overpass ----------

type overpassElement struct {
	Tags map[string]string `json:"tags"`

	Geometry []struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	} `json:"geometry"`

	Center *struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	} `json:"center"`
}

type overpassResponse struct {
	Remark   string            `json:"remark"`
	Elements []overpassElement `json:"elements"`
}

// ---------- Daftar endpoint + cooldown ----------

func daftarEndpointOverpass() []string {
	raw := strings.TrimSpace(os.Getenv("OVERPASS_URLS"))
	if raw == "" {
		return defaultOverpassURLs
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return defaultOverpassURLs
	}
	return out
}

type overpassEndpointState struct {
	mu       sync.Mutex
	next     int
	cooldown map[string]time.Time
}

var overpassEP = &overpassEndpointState{cooldown: map[string]time.Time{}}

// pilih memilih endpoint yang tidak sedang cooldown (bergiliran).
// Jika semuanya cooldown, pilih yang cooldown-nya paling cepat selesai.
func (s *overpassEndpointState) pilih(eps []string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	n := len(eps)
	for i := 0; i < n; i++ {
		idx := (s.next + i) % n
		if !now.Before(s.cooldown[eps[idx]]) {
			s.next = (idx + 1) % n
			return eps[idx]
		}
	}
	best := eps[0]
	for _, ep := range eps[1:] {
		if s.cooldown[ep].Before(s.cooldown[best]) {
			best = ep
		}
	}
	return best
}

func (s *overpassEndpointState) gagal(ep string, retryAfter time.Duration) {
	d := overpassCooldown
	if retryAfter > d {
		d = retryAfter
	}
	if d > 60*time.Second {
		d = 60 * time.Second
	}
	s.mu.Lock()
	s.cooldown[ep] = time.Now().Add(d)
	s.mu.Unlock()
}

func (s *overpassEndpointState) sehat(ep string) {
	s.mu.Lock()
	delete(s.cooldown, ep)
	s.mu.Unlock()
}

// ---------- Satu percobaan HTTP ----------

// panggilOverpass mengembalikan: data, bolehDiulang, retryAfter, error.
func panggilOverpass(
	ctx context.Context, endpoint, query string,
) (*overpassResponse, bool, time.Duration, error) {
	ctxAttempt, cancel := context.WithTimeout(ctx, overpassAttemptTimeout)
	defer cancel()

	form := url.Values{}
	form.Set("data", query)

	req, err := http.NewRequestWithContext(
		ctxAttempt, http.MethodPost, endpoint, strings.NewReader(form.Encode()),
	)
	if err != nil {
		return nil, false, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", overpassUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, true, 0, err // timeout / koneksi gagal -> boleh diulang
	}
	defer resp.Body.Close()

	var retryAfter time.Duration
	if v := strings.TrimSpace(resp.Header.Get("Retry-After")); v != "" {
		if sec, convErr := strconv.Atoi(v); convErr == nil && sec > 0 {
			retryAfter = time.Duration(sec) * time.Second
		}
	}

	if resp.StatusCode != http.StatusOK {
		// 400 = query salah (kesalahan kita), tidak ada gunanya diulang.
		boleh := resp.StatusCode != http.StatusBadRequest
		return nil, boleh, retryAfter, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var data overpassResponse
	dec := json.NewDecoder(io.LimitReader(resp.Body, overpassMaxBody))
	if err := dec.Decode(&data); err != nil {
		return nil, true, 0, fmt.Errorf("respons bukan JSON valid: %w", err)
	}

	// HTTP 200 tetapi server gagal memproses query: JANGAN dianggap "tidak ada jalan".
	if strings.Contains(strings.ToLower(data.Remark), "runtime error") {
		return nil, true, 0, fmt.Errorf("Overpass melaporkan error: %s", data.Remark)
	}
	if data.Elements == nil {
		return nil, true, 0, errors.New("respons tidak memuat field elements")
	}

	return &data, false, 0, nil
}

func backoffOverpass(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		if retryAfter > 5*time.Second {
			return 5 * time.Second
		}
		return retryAfter
	}
	d := overpassBackoffBase << uint(attempt)
	if d > overpassBackoffMax {
		d = overpassBackoffMax
	}
	// jitter: 50%..100% dari d
	return d/2 + time.Duration(rand.Int63n(int64(d/2)+1))
}

// ambilOverpass menjalankan query dengan retry + backoff + rotasi endpoint.
func ambilOverpass(
	ctx context.Context, query string, maxAttempts int,
) (*overpassResponse, error) {
	eps := daftarEndpointOverpass()
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if ctx.Err() != nil {
			break
		}

		ep := overpassEP.pilih(eps)
		data, boleh, retryAfter, err := panggilOverpass(ctx, ep, query)
		if err == nil {
			overpassEP.sehat(ep)
			return data, nil
		}

		lastErr = fmt.Errorf("%s: %w", ep, err)
		log.Printf("[ROADIS OVERPASS] percobaan %d/%d gagal: %v",
			attempt+1, maxAttempts, lastErr)

		if !boleh {
			return nil, lastErr // bukan ErrOSMTidakTersedia: kesalahan query
		}

		overpassEP.gagal(ep, retryAfter)

		if attempt == maxAttempts-1 {
			break
		}
		select {
		case <-ctx.Done():
		case <-time.After(backoffOverpass(attempt, retryAfter)):
		}
	}

	if lastErr == nil {
		lastErr = errors.New("batas waktu habis")
	}
	return nil, fmt.Errorf("%w: %v", ErrOSMTidakTersedia, lastErr)
}

// ---------- Cache per tile + penggabungan request identik ----------

type tileEntry struct {
	elements []overpassElement
	fetched  time.Time
}

var tileCache = struct {
	sync.Mutex
	m map[string]tileEntry
}{m: map[string]tileEntry{}}

type overpassCall struct {
	done     chan struct{}
	elements []overpassElement
	err      error
}

var inflightOverpass = struct {
	sync.Mutex
	m map[string]*overpassCall
}{m: map[string]*overpassCall{}}

func cacheTileGet(key string) (tileEntry, bool) {
	tileCache.Lock()
	defer tileCache.Unlock()
	e, ok := tileCache.m[key]
	return e, ok
}

func cacheTileSet(key string, el []overpassElement) {
	tileCache.Lock()
	defer tileCache.Unlock()
	if len(tileCache.m) >= 1000 { // batas sederhana agar memori tidak membengkak
		tileCache.m = map[string]tileEntry{}
	}
	tileCache.m[key] = tileEntry{elements: el, fetched: time.Now()}
}

func tileOverpass(lat, lng float64) (key string, cLat, cLng float64) {
	iy := math.Floor(lat / overpassTileSize)
	ix := math.Floor(lng / overpassTileSize)
	cLat = (iy + 0.5) * overpassTileSize
	cLng = (ix + 0.5) * overpassTileSize
	key = fmt.Sprintf("%d:%d", int64(iy), int64(ix))
	return
}

// staleAtauError: pakai cache lama (maks 7 hari) bila ada, kalau tidak teruskan error.
func staleAtauError(key string, err error) ([]overpassElement, bool, error) {
	if e, ok := cacheTileGet(key); ok && time.Since(e.fetched) < overpassStaleMax {
		log.Printf("[ROADIS OVERPASS] memakai cache lama (umur %s) karena: %v",
			time.Since(e.fetched).Round(time.Minute), err)
		return e.elements, true, nil
	}
	return nil, false, err
}

// ambilElemenSekitar mengembalikan elemen jalan di sekitar titik,
// dan penanda apakah datanya berasal dari cache yang sudah kedaluwarsa.
func ambilElemenSekitar(
	lat, lng float64, budget time.Duration, maxAttempts int,
) ([]overpassElement, bool, error) {
	key, cLat, cLng := tileOverpass(lat, lng)

	if e, ok := cacheTileGet(key); ok && time.Since(e.fetched) < overpassCacheTTL {
		return e.elements, false, nil
	}

	inflightOverpass.Lock()
	if call, ok := inflightOverpass.m[key]; ok {
		inflightOverpass.Unlock()
		select {
		case <-call.done:
			if call.err == nil {
				return call.elements, false, nil
			}
			return staleAtauError(key, call.err)
		case <-time.After(budget):
			return staleAtauError(key, fmt.Errorf(
				"%w: menunggu permintaan lain melebihi batas waktu", ErrOSMTidakTersedia))
		}
	}
	call := &overpassCall{done: make(chan struct{})}
	inflightOverpass.m[key] = call
	inflightOverpass.Unlock()

	defer func() {
		inflightOverpass.Lock()
		delete(inflightOverpass.m, key)
		inflightOverpass.Unlock()
		close(call.done)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()

	query := fmt.Sprintf(
		"[out:json][timeout:8];\nway(around:%d,%.7f,%.7f)[\"highway\"];\nout tags center geom;",
		overpassTileRadius, cLat, cLng,
	)

	data, err := ambilOverpass(ctx, query, maxAttempts)
	if err != nil {
		call.err = err
		return staleAtauError(key, err)
	}

	call.elements = data.Elements
	cacheTileSet(key, data.Elements)
	return data.Elements, false, nil
}

// ---------- Pencarian jalan terdekat ----------

// cariJalanOSM: anggaran waktu normal (dipakai CekJalan).
func cariJalanOSM(lat, lng, radius float64) (InfoJalanOSM, error) {
	return cariJalanOSMDenganBudget(
		lat, lng, radius, overpassBudgetNormal, overpassAttemptsNormal,
	)
}

// cariJalanOSMDenganBudget mengembalikan:
//   - (info.Terdeteksi=true,  nil)                  jalan ditemukan
//   - (info.Terdeteksi=false, nil)                  Overpass menjawab benar, memang tidak ada jalan
//   - (_, error yang membungkus ErrOSMTidakTersedia) layanan gagal
func cariJalanOSMDenganBudget(
	lat, lng, radius float64, budget time.Duration, maxAttempts int,
) (InfoJalanOSM, error) {
	info := InfoJalanOSM{
		NamaJalan:  "",
		JenisJalan: "belum_pasti",
		JarakMeter: -1,
		Terdeteksi: false,
		Keterangan: "Jalan belum teridentifikasi.",
	}

	if radius <= 0 || radius > overpassMaxRadius {
		return info, fmt.Errorf("radius harus antara 1 dan %d meter", overpassMaxRadius)
	}

	elements, stale, err := ambilElemenSekitar(lat, lng, budget, maxAttempts)
	if err != nil {
		return info, err
	}

	jarakTerbaik := math.MaxFloat64
	var tagsTerbaik map[string]string

	for _, element := range elements {
		if strings.TrimSpace(element.Tags["highway"]) == "" {
			continue
		}

		jarak := math.MaxFloat64

		for i := 0; i+1 < len(element.Geometry); i++ {
			a := element.Geometry[i]
			b := element.Geometry[i+1]
			d := jarakKeSegmen(lat, lng, a.Lat, a.Lon, b.Lat, b.Lon)
			if d < jarak {
				jarak = d
			}
		}

		if len(element.Geometry) < 2 && element.Center != nil {
			jarak = jarakMeter(lat, lng, element.Center.Lat, element.Center.Lon)
		}

		if jarak < jarakTerbaik {
			jarakTerbaik = jarak
			tagsTerbaik = element.Tags
		}
	}

	catatanCache := ""
	if stale {
		catatanCache = " (data dari cache lama karena layanan OSM sedang tidak tersedia)"
	}

	if tagsTerbaik == nil || jarakTerbaik > radius {
		info.Keterangan = fmt.Sprintf(
			"Tidak ditemukan jalan dalam radius %.0f meter.%s", radius, catatanCache)
		return info, nil
	}

	info.NamaJalan = strings.TrimSpace(tagsTerbaik["name"])
	if info.NamaJalan == "" {
		info.NamaJalan = "Nama jalan belum tersedia di OSM"
	}
	info.JarakMeter = math.Round(jarakTerbaik*100) / 100
	info.Terdeteksi = true
	info.JenisJalan = PerkiraanKewenanganJalan(tagsTerbaik["highway"])
	info.Keterangan = "Jalan ditemukan melalui OSM. Kewenangan merupakan perkiraan berdasarkan kelas jalan." + catatanCache

	return info, nil
}