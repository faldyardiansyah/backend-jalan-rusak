package controllers

import (
	"net/http"
	"strings"
	"time"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type RegisterInput struct {
	Nama     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginInput struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func Register(c *gin.Context) {
	var input RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	email := strings.ToLower(strings.TrimSpace(input.Email))

	// Cek apakah email sudah terdaftar
	var count int64
	if err := config.DB.Model(&models.User{}).
		Where("email = ?", email).
		Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Terjadi kesalahan pada server",
		})
		return
	}
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Email sudah terdaftar",
		})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal mengenkripsi password",
		})
		return
	}

	now := time.Now()
	user := models.User{
		Name:              strings.TrimSpace(input.Nama),
		Email:             email,
		Password:          string(hashedPassword),
		Role:              models.RoleWarga,
		PasswordChangedAt: &now,
	}

	if err := config.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Email sudah terdaftar atau terjadi kesalahan",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Registrasi berhasil",
		"data": gin.H{
			"nama":  user.Name,
			"email": user.Email,
			"role":  user.Role,
		},
	})
}

func Login(c *gin.Context) {
	var input LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	email := strings.ToLower(strings.TrimSpace(input.Email))

	var user models.User
	if err := config.DB.Where("email = ?", email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Email atau password salah",
		})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Email atau password salah",
		})
		return
	}

	now := time.Now()
	config.DB.Model(&models.User{}).Where("id = ?", user.ID).Update("last_login_at", &now)
	user.LastLoginAt = &now

	token, err := utils.GenerateToken(user.ID, user.Email, user.Role, user.WilayahID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal membuat token",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Login berhasil",
		"token":   token,
		"user": gin.H{
			"id":           user.ID,
			"nama":         user.Name,
			"email":        user.Email,
			"role":         user.Role,
			"wilayah_id":   user.WilayahID,
			"profil_photo": user.ProfilePhoto,
		},
	})
}

// buat update foto tapi opsional
func UpdateProfilePhoto(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User tidak terautentikasi",
		})
		return
	}
	userID := userIDVal.(uint)

	fileHeader, err := c.FormFile("foto")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Foto wajib diunggah",
		})
		return
	}

	imageURL, err := utils.UploadCloudinary(fileHeader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal upload foto",
		})
		return
	}

	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User tidak ditemukan",
		})
		return
	}

	user.ProfilePhoto = imageURL
	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal menyimpan foto",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Foto profil berhasil diperbarui",
		"data": gin.H{
			"id":           user.ID,
			"nama":         user.Name,
			"email":        user.Email,
			"role":         user.Role,
			"wilayah_id":   user.WilayahID,
			"profil_photo": user.ProfilePhoto,
		},
	})
}