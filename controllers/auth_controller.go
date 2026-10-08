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
			"status":  "error",
			"message": err.Error(),
			"error":   err.Error(),
		})
		return
	}

	if config.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"error":   "Koneksi database tidak tersedia",
			"message": "Koneksi database tidak tersedia",
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
			"status":  "error",
			"message": "Terjadi kesalahan pada server",
			"error":   "Terjadi kesalahan pada server",
		})
		return
	}
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"status":  "error",
			"message": "Email sudah terdaftar",
			"error":   "Email sudah terdaftar",
		})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengenkripsi password",
			"error":   "Gagal mengenkripsi password",
		})
		return
	}

	now := time.Now()
	user := models.User{
		Name:              strings.TrimSpace(input.Nama),
		Email:             email,
		Password:          string(hashedPassword),
		Role:              models.RoleWarga,
		TokenVersion:      1,
		PasswordChangedAt: &now,
	}

	if err := config.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Email sudah terdaftar atau terjadi kesalahan",
			"error":   "Email sudah terdaftar atau terjadi kesalahan",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":  "success",
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
			"status":  "error",
			"message": err.Error(),
			"error":   err.Error(),
		})
		return
	}

	if config.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"error":   "Koneksi database tidak tersedia",
			"message": "Koneksi database tidak tersedia",
		})
		return
	}

	email := strings.ToLower(strings.TrimSpace(input.Email))

	var user models.User
	if err := config.DB.Where("email = ?", email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "Email atau password salah",
			"error":   "Email atau password salah",
		})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "Email atau password salah",
			"error":   "Email atau password salah",
		})
		return
	}

	now := time.Now()
	config.DB.Model(&models.User{}).Where("id = ?", user.ID).Update("last_login_at", &now)
	user.LastLoginAt = &now

	if user.TokenVersion == 0 {
		user.TokenVersion = 1
		config.DB.Model(&models.User{}).Where("id = ?", user.ID).Update("token_version", 1)
	}

	token, err := utils.GenerateToken(user.ID, user.Email, user.Role, user.WilayahID, user.TokenVersion)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal membuat token",
			"error":   "Gagal membuat token",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
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

// buat update foto tapi opsional (endpoint legacy /api/profile/photo)
func UpdateProfilePhoto(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	if config.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"error":   "Koneksi database tidak tersedia",
			"message": "Koneksi database tidak tersedia",
		})
		return
	}

	fileHeader, err := c.FormFile("foto")
	if err != nil {
		fileHeader, err = c.FormFile("avatar")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"error":   "Foto wajib diunggah (field: foto)",
				"message": "Foto wajib diunggah (field: foto)",
			})
			return
		}
	}

	// Validasi file terpusat via utils.ValidateImageFile (maksimal 2 MB, JPG/PNG/WEBP, byte sniffing)
	if errVal := utils.ValidateImageFile(fileHeader, 2*1024*1024); errVal != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"error":   errVal.Error(),
			"message": errVal.Error(),
		})
		return
	}

	var user models.User
	if err := config.DB.Where("deleted_at IS NULL").First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"error":   "User tidak ditemukan",
			"message": "User tidak ditemukan",
		})
		return
	}

	oldPhoto := ""
	if user.AvatarURL != nil && *user.AvatarURL != "" {
		oldPhoto = *user.AvatarURL
	} else if user.ProfilePhoto != "" {
		oldPhoto = user.ProfilePhoto
	}

	imageURL, err := AvatarUploader(fileHeader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"error":   "Gagal upload foto",
			"message": "Gagal upload foto",
		})
		return
	}

	user.ProfilePhoto = imageURL
	user.AvatarURL = &imageURL
	if err := config.DB.Save(&user).Error; err != nil {
		_ = AvatarDeleter(imageURL)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"error":   "Gagal menyimpan foto",
			"message": "Gagal menyimpan foto",
		})
		return
	}

	if oldPhoto != "" && oldPhoto != imageURL {
		go func(urlToDelete string) {
			_ = AvatarDeleter(urlToDelete)
		}(oldPhoto)
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
			"avatar_url":   user.AvatarURL,
		},
	})
}