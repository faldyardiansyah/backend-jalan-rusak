package utils

import (
	"os"
	"testing"
	"time"

	"backend-jalan-rusak/models"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateAndValidateToken_Success(t *testing.T) {
	t.Setenv("JWT_SECRET", "test_secret_key_1234567890_abcdefghij")

	var wilayahID uint = 42
	userID := uint(10)
	email := "testuser@roadis.id"
	role := models.RoleAdminPemdes

	token, err := GenerateToken(userID, email, role, &wilayahID)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	if token == "" {
		t.Fatal("GenerateToken returned empty token")
	}

	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("expected UserID %d, got %d", userID, claims.UserID)
	}
	if claims.Email != email {
		t.Errorf("expected Email %s, got %s", email, claims.Email)
	}
	if claims.Role != role {
		t.Errorf("expected Role %s, got %s", role, claims.Role)
	}
	if claims.WilayahID == nil || *claims.WilayahID != wilayahID {
		t.Errorf("expected WilayahID %d, got %v", wilayahID, claims.WilayahID)
	}
}

func TestValidateToken_WrongSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "secret_one_1234567890_abcdefghij")

	userID := uint(1)
	email := "user@roadis.id"
	role := models.RoleWarga

	token, err := GenerateToken(userID, email, role, nil)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	// Change secret key
	t.Setenv("JWT_SECRET", "secret_two_different_1234567890_xyz")

	_, err = ValidateToken(token)
	if err == nil {
		t.Fatal("expected ValidateToken to fail with wrong secret, but succeeded")
	}
}

func TestValidateToken_Expired(t *testing.T) {
	secret := "test_secret_for_expired_test_12345"
	t.Setenv("JWT_SECRET", secret)

	claims := JWTClaim{
		UserID: 1,
		Email:  "expired@roadis.id",
		Role:   models.RoleWarga,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)), // Expired 1 hour ago
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}

	rawToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := rawToken.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign expired token: %v", err)
	}

	_, err = ValidateToken(tokenString)
	if err == nil {
		t.Fatal("expected ValidateToken to fail for expired token, but succeeded")
	}
}

func TestValidateToken_Empty(t *testing.T) {
	t.Setenv("JWT_SECRET", "some_secret_1234567890")

	_, err := ValidateToken("")
	if err == nil {
		t.Fatal("expected ValidateToken to fail for empty string, but succeeded")
	}
}

func TestGenerateToken_MissingSecret(t *testing.T) {
	// Temporarily clear environment
	origSecret := os.Getenv("JWT_SECRET")
	origSecretKey := os.Getenv("JWT_SECRET_KEY")
	defer func() {
		os.Setenv("JWT_SECRET", origSecret)
		os.Setenv("JWT_SECRET_KEY", origSecretKey)
	}()

	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("JWT_SECRET_KEY")

	// We test getJWTSecret directly when env is empty
	sec, err := getJWTSecret()
	if err == nil && len(sec) == 0 {
		t.Fatal("expected getJWTSecret to return error when no env is set, but succeeded with empty secret")
	}
}
