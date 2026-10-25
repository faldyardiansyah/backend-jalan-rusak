package utils

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	tLat = -6.386742
	tLng = 108.295131

	jsonAdaJalan = `{"elements":[{"type":"way","tags":{"highway":"tertiary","name":"Jl. Uji"},"geometry":[{"lat":-6.386742,"lon":108.295100},{"lat":-6.386742,"lon":108.295200}]}]}`
	jsonKosong   = `{"elements":[]}`
	jsonRemark   = `{"remark":"runtime error: Query timed out in \"query\" at line 2 after 8 seconds.","elements":[]}`
)

func resetOverpassState() {
	tileCache.Lock()
	tileCache.m = map[string]tileEntry{}
	tileCache.Unlock()
	overpassEP.mu.Lock()
	overpassEP.next = 0
	overpassEP.cooldown = map[string]time.Time{}
	overpassEP.mu.Unlock()
}

func siapkanOverpassTest(t *testing.T, urls ...string) {
	t.Helper()
	t.Setenv("OVERPASS_URLS", strings.Join(urls, ","))
	oldT, oldB := overpassAttemptTimeout, overpassBackoffBase
	overpassAttemptTimeout = 300 * time.Millisecond
	overpassBackoffBase = time.Millisecond
	resetOverpassState()
	t.Cleanup(func() {
		overpassAttemptTimeout, overpassBackoffBase = oldT, oldB
		resetOverpassState()
	})
}

func serverJSON(t *testing.T, hits *int32, status int, body string) string {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(hits, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(s.Close)
	return s.URL
}

func TestOverpass_Normal(t *testing.T) {
	var h int32
	siapkanOverpassTest(t, serverJSON(t, &h, 200, jsonAdaJalan))

	info, err := cariJalanOSM(tLat, tLng, 20)
	if err != nil || !info.Terdeteksi || info.NamaJalan != "Jl. Uji" {
		t.Fatalf("harusnya ditemukan, dapat info=%+v err=%v", info, err)
	}
	if info.JenisJalan != "kabupaten" {
		t.Fatalf("tertiary harusnya kabupaten, dapat %q", info.JenisJalan)
	}
}

func TestOverpass_TidakAdaJalan(t *testing.T) {
	var h int32
	siapkanOverpassTest(t, serverJSON(t, &h, 200, jsonKosong))

	info, err := cariJalanOSM(tLat, tLng, 20)
	if err != nil || info.Terdeteksi {
		t.Fatalf("harusnya tidak ada jalan TANPA error, dapat info=%+v err=%v", info, err)
	}
}

func TestOverpass_RemarkBukanTidakAdaJalan(t *testing.T) {
	var h int32
	siapkanOverpassTest(t, serverJSON(t, &h, 200, jsonRemark))

	_, err := cariJalanOSM(tLat, tLng, 20)
	if !errors.Is(err, ErrOSMTidakTersedia) {
		t.Fatalf("HTTP 200 + runtime error harusnya ErrOSMTidakTersedia, dapat %v", err)
	}
}

func TestOverpass_FailoverDari502(t *testing.T) {
	var a, b int32
	siapkanOverpassTest(t,
		serverJSON(t, &a, 502, "Gateway Timeout"),
		serverJSON(t, &b, 200, jsonAdaJalan),
	)

	info, err := cariJalanOSM(tLat, tLng, 20)
	if err != nil || !info.Terdeteksi {
		t.Fatalf("harusnya berhasil lewat endpoint kedua: info=%+v err=%v", info, err)
	}
	if atomic.LoadInt32(&a) != 1 || atomic.LoadInt32(&b) != 1 {
		t.Fatalf("hit A=%d B=%d, harapan 1 dan 1", a, b)
	}
}

func TestOverpass_SemuaGagal503(t *testing.T) {
	var h int32
	siapkanOverpassTest(t, serverJSON(t, &h, 503, "busy"))

	_, err := cariJalanOSM(tLat, tLng, 20)
	if !errors.Is(err, ErrOSMTidakTersedia) {
		t.Fatalf("harusnya ErrOSMTidakTersedia, dapat %v", err)
	}
	if atomic.LoadInt32(&h) != int32(overpassAttemptsNormal) {
		t.Fatalf("harusnya %d percobaan, dapat %d", overpassAttemptsNormal, h)
	}
}

func TestOverpass_Timeout(t *testing.T) {
	var h int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&h, 1)
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
	}))
	t.Cleanup(s.Close)
	siapkanOverpassTest(t, s.URL)

	_, err := cariJalanOSM(tLat, tLng, 20)
	if !errors.Is(err, ErrOSMTidakTersedia) {
		t.Fatalf("timeout harusnya ErrOSMTidakTersedia, dapat %v", err)
	}
}

func TestOverpass_CacheMencegahPanggilanUlang(t *testing.T) {
	var h int32
	siapkanOverpassTest(t, serverJSON(t, &h, 200, jsonAdaJalan))

	for i := 0; i < 3; i++ {
		if _, err := cariJalanOSM(tLat, tLng, 20); err != nil {
			t.Fatal(err)
		}
	}
	if atomic.LoadInt32(&h) != 1 {
		t.Fatalf("server harusnya hanya dipanggil 1 kali, dapat %d", h)
	}
}

func TestOverpass_StaleSaatLayananMati(t *testing.T) {
	var h int32
	siapkanOverpassTest(t, serverJSON(t, &h, 503, "busy"))

	var seed overpassResponse
	if err := json.Unmarshal([]byte(jsonAdaJalan), &seed); err != nil {
		t.Fatal(err)
	}
	key, _, _ := tileOverpass(tLat, tLng)
	tileCache.Lock()
	tileCache.m[key] = tileEntry{elements: seed.Elements, fetched: time.Now().Add(-48 * time.Hour)}
	tileCache.Unlock()

	info, err := cariJalanOSM(tLat, tLng, 20)
	if err != nil || !info.Terdeteksi || !strings.Contains(info.Keterangan, "cache") {
		t.Fatalf("harusnya memakai cache lama: info=%+v err=%v", info, err)
	}
}
