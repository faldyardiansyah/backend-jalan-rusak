package utils

import (
	"errors"
	"os"
	"time"

	"backend-jalan-rusak/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
)

type JWTClaim struct {
	UserID    uint            `json:"user_id"`
	Email     string          `json:"email"`
	Role      models.UserRole `json:"role"`
	WilayahID *uint           `json:"wilayah_id"`
	jwt.RegisteredClaims
}

func getJWTSecret() ([]byte, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = os.Getenv("JWT_SECRET_KEY")
	}
	if secret == "" {
		_ = godotenv.Load()
		secret = os.Getenv("JWT_SECRET")
		if secret == "" {
			secret = os.Getenv("JWT_SECRET_KEY")
		}
	}

	if len(secret) == 0 {
		return nil, errors.New("JWT_SECRET environment variable is not configured")
	}

	return []byte(secret), nil
}

func GenerateToken(
	userID uint,
	email string,
	role models.UserRole,
	wilayahID *uint,
) (string, error) {

	secretKey, err := getJWTSecret()
	if err != nil {
		return "", err
	}

	var expTime time.Duration

	if role == models.RoleWarga {
		expTime = 30 * 24 * time.Hour
	} else {
		expTime = 24 * time.Hour
	}

	claims := JWTClaim{
		UserID:    userID,
		Email:     email,
		Role:      role,
		WilayahID: wilayahID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expTime)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	)

	return token.SignedString(secretKey)
}

func ValidateToken(tokenString string) (*JWTClaim, error) {
	secretKey, err := getJWTSecret()
	if err != nil {
		return nil, err
	}

	if tokenString == "" {
		return nil, errors.New("token cannot be empty")
	}

	token, err := jwt.ParseWithClaims(
		tokenString,
		&JWTClaim{},
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return secretKey, nil
		},
	)

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*JWTClaim)

	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}
