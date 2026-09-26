package utils

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReverseGeocodeOSM_Success(t *testing.T) {
	mockResponse := `{
		"address": {
			"village": "Desa Sukaurip",
			"suburb": "",
			"town": ""
		},
		"extratags": {
			"highway": "secondary"
		}
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	// Simpan konfigurasi asli dan kembalikan setelah test selesai
	origURL := osmBaseURL
	origClient := osmHTTPClient
	defer func() {
		osmBaseURL = origURL
		osmHTTPClient = origClient
	}()

	osmBaseURL = server.URL
	osmHTTPClient = server.Client()

	nama, jenisJalan, err := ReverseGeocodeOSM(-6.34, 108.33)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if nama != "Desa Sukaurip" {
		t.Errorf("expected nama 'Desa Sukaurip', got %q", nama)
	}

	if jenisJalan != "kabupaten" { // secondary -> kabupaten
		t.Errorf("expected jenisJalan 'kabupaten', got %q", jenisJalan)
	}
}

func TestReverseGeocodeOSM_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulasi server lambat / menggantung
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	origURL := osmBaseURL
	origClient := osmHTTPClient
	defer func() {
		osmBaseURL = origURL
		osmHTTPClient = origClient
	}()

	osmBaseURL = server.URL
	// Set timeout sangat singkat untuk menguji bahwa client tidak hang
	osmHTTPClient = &http.Client{
		Timeout: 50 * time.Millisecond,
	}

	start := time.Now()
	_, _, err := ReverseGeocodeOSM(-6.34, 108.33)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	if duration > 200*time.Millisecond {
		t.Errorf("request took too long (%v), timeout did not trigger properly", duration)
	}
}

func TestReverseGeocodeOSM_Non200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("503 Service Unavailable"))
	}))
	defer server.Close()

	origURL := osmBaseURL
	origClient := osmHTTPClient
	defer func() {
		osmBaseURL = origURL
		osmHTTPClient = origClient
	}()

	osmBaseURL = server.URL
	osmHTTPClient = server.Client()

	_, _, err := ReverseGeocodeOSM(-6.34, 108.33)
	if err == nil {
		t.Fatal("expected error on HTTP 503, got nil")
	}
}

func TestReverseGeocodeOSM_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{ invalid json ]"))
	}))
	defer server.Close()

	origURL := osmBaseURL
	origClient := osmHTTPClient
	defer func() {
		osmBaseURL = origURL
		osmHTTPClient = origClient
	}()

	osmBaseURL = server.URL
	osmHTTPClient = server.Client()

	_, _, err := ReverseGeocodeOSM(-6.34, 108.33)
	if err == nil {
		t.Fatal("expected decode error on malformed JSON, got nil")
	}
}
