package config

import (
	"os"
	"strings"
	"testing"
)

func TestGetTestDBName_Default(t *testing.T) {
	orig := os.Getenv("TEST_DB_NAME")
	defer func() { _ = os.Setenv("TEST_DB_NAME", orig) }()

	_ = os.Unsetenv("TEST_DB_NAME")
	if name := GetTestDBName(); name != "db_jalan_rusak_test" {
		t.Errorf("expected default 'db_jalan_rusak_test', got '%s'", name)
	}
}

func TestGetTestDBName_CustomEnv(t *testing.T) {
	orig := os.Getenv("TEST_DB_NAME")
	defer func() { _ = os.Setenv("TEST_DB_NAME", orig) }()

	_ = os.Setenv("TEST_DB_NAME", "my_custom_isolated_test_db")
	if name := GetTestDBName(); name != "my_custom_isolated_test_db" {
		t.Errorf("expected 'my_custom_isolated_test_db', got '%s'", name)
	}
}

func TestGetTestDSN_FailSafe_BlocksDevelopmentDB(t *testing.T) {
	origTest := os.Getenv("TEST_DB_NAME")
	origDev := os.Getenv("DB_NAME")
	defer func() {
		_ = os.Setenv("TEST_DB_NAME", origTest)
		_ = os.Setenv("DB_NAME", origDev)
	}()

	// 1. Explicitly setting TEST_DB_NAME to db_jalan_rusak must be blocked
	_ = os.Setenv("TEST_DB_NAME", "db_jalan_rusak")
	_, err := GetTestDSN()
	if err == nil {
		t.Fatal("expected fail-safe error when TEST_DB_NAME='db_jalan_rusak', got nil")
	}
	if !strings.Contains(err.Error(), "FAIL-SAFE TRIGGERED") {
		t.Errorf("expected error message to contain 'FAIL-SAFE TRIGGERED', got: %v", err)
	}

	// 2. Setting TEST_DB_NAME to match custom DB_NAME must also be blocked
	_ = os.Setenv("DB_NAME", "custom_prod_db")
	_ = os.Setenv("TEST_DB_NAME", "custom_prod_db")
	_, err = GetTestDSN()
	if err == nil {
		t.Fatal("expected fail-safe error when TEST_DB_NAME matches DB_NAME, got nil")
	}
	if !strings.Contains(err.Error(), "FAIL-SAFE TRIGGERED") {
		t.Errorf("expected error message to contain 'FAIL-SAFE TRIGGERED', got: %v", err)
	}
}

func TestEnsureTestDB_FailSafe(t *testing.T) {
	origTest := os.Getenv("TEST_DB_NAME")
	defer func() { _ = os.Setenv("TEST_DB_NAME", origTest) }()

	_ = os.Setenv("TEST_DB_NAME", "db_jalan_rusak")
	_, err := EnsureTestDB()
	if err == nil {
		t.Fatal("expected EnsureTestDB to reject 'db_jalan_rusak', got nil error")
	}
	if !strings.Contains(err.Error(), "FAIL-SAFE TRIGGERED") {
		t.Errorf("expected error message to contain 'FAIL-SAFE TRIGGERED', got: %v", err)
	}
}

func TestEnsureTestDB_NonExistentTestDB_FailsGracefullyWithoutCreation(t *testing.T) {
	origTest := os.Getenv("TEST_DB_NAME")
	defer func() { _ = os.Setenv("TEST_DB_NAME", origTest) }()

	_ = os.Setenv("TEST_DB_NAME", "db_jalan_rusak_test")
	_, err := EnsureTestDB()
	if err == nil {
		t.Log("Note: db_jalan_rusak_test connected successfully")
	} else {
		// When test database does not exist, it must return an error and NOT create or fallback to dev
		if !strings.Contains(err.Error(), "unavailable") {
			t.Errorf("expected error to indicate test database unavailable, got: %v", err)
		}
	}
}
