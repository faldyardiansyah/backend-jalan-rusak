package config_test

import (
	"os"
	"strings"
	"testing"

	"backend-jalan-rusak/config"
)

func TestStartupConfig_DefaultOptInBehavior(t *testing.T) {
	// 1. Verifikasi bahwa tanpa konfigurasi eksplisit, AUTO_MIGRATE dan SEED_DEV_DATA tidak aktif
	if os.Getenv("AUTO_MIGRATE") == "true" {
		t.Errorf("AUTO_MIGRATE tidak boleh aktif secara default")
	}
	if os.Getenv("SEED_DEV_DATA") == "true" {
		t.Errorf("SEED_DEV_DATA tidak boleh aktif secara default")
	}
}

func TestEnsureTestDB_ProtectsDevelopmentDatabase(t *testing.T) {
	// Pastikan fail-safe aktif jika TEST_DB_NAME diarahkan ke database development
	origTestDB := os.Getenv("TEST_DB_NAME")
	defer os.Setenv("TEST_DB_NAME", origTestDB)

	os.Setenv("TEST_DB_NAME", "db_jalan_rusak")
	_, err := config.EnsureTestDB()
	if err == nil {
		t.Fatalf("EnsureTestDB harus menolak koneksi jika target database adalah db_jalan_rusak")
	}
	if !strings.Contains(err.Error(), "FAIL-SAFE TRIGGERED") {
		t.Errorf("expected error containing 'FAIL-SAFE TRIGGERED', got: %v", err)
	}
}
