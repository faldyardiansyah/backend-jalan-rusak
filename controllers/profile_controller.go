package controllers

import (
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"
	"backend-jalan-rusak/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

var (
	// AvatarUploader dan AvatarDeleter menggunakan fungsi Cloudinary existing
	// Bisa di-override pada unit test
	AvatarUploader = utils.UploadAvatarCloudinary
	AvatarDeleter  = utils.DeleteCloudinary
)

// Regex validasi nomor telepon Indonesia:
// Menerima awalan 0, +62, atau 62 diikuti 8-14 digit, dapat mengandung spasi atau tanda hubung (-)
var indonesianPhoneRegex = regexp.MustCompile(`^(\+62|62|0)[0-9\- ]{8,20}$`)

func IsValidIndonesianPhone(phone string) bool {
	trimmed := strings.TrimSpace(phone)
	if len(trimmed) > 25 {
		return false
	}
	if !indonesianPhoneRegex.MatchString(trimmed) {
		return false
	}
	// Hitung hanya digit numerik
	digitsOnly := regexp.MustCompile(`[0-9]`).FindAllString(trimmed, -1)
	totalDigits := len(digitsOnly)
	return totalDigits >= 9 && totalDigits <= 16
}

// GET /api/profile
func GetProfile(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	var user models.User
	if err := config.DB.Preload("Wilayah").Where("deleted_at IS NULL").First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Pengguna tidak ditemukan",
		})
		return
	}

	avatarURL := user.AvatarURL
	if avatarURL == nil && user.ProfilePhoto != "" {
		avatarURL = &user.ProfilePhoto
	}

	var wilayahData *models.Wilayah
	if user.WilayahID != nil && user.Wilayah.ID != 0 {
		wilayahData = &user.Wilayah
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Profil berhasil diambil",
		"data": gin.H{
			"id":            user.ID,
			"name":          user.Name,
			"email":         user.Email,
			"phone":         user.Phone,
			"role":          user.Role,
			"wilayah_id":    user.WilayahID,
			"wilayah":       wilayahData,
			"avatar_url":    avatarURL,
			"last_login_at": user.LastLoginAt,
			"password_changed_at": user.PasswordChangedAt,
		},
	})
}

type UpdateProfileInput struct {
	Name  *string `json:"name"`
	Phone *string `json:"phone"`
}

// PUT /api/profile
func UpdateProfile(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	var input UpdateProfileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Format request tidak valid: " + err.Error(),
		})
		return
	}

	var user models.User
	if err := config.DB.Preload("Wilayah").Where("deleted_at IS NULL").First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Pengguna tidak ditemukan",
		})
		return
	}

	// Validasi Name
	if input.Name == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Nama wajib diisi",
		})
		return
	}

	trimmedName := strings.TrimSpace(*input.Name)
	if trimmedName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Nama tidak boleh kosong atau hanya berisi spasi",
		})
		return
	}
	if len([]rune(trimmedName)) > 150 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Nama maksimal 150 karakter",
		})
		return
	}
	user.Name = trimmedName

	// Validasi Phone (opsional)
	if input.Phone != nil {
		trimmedPhone := strings.TrimSpace(*input.Phone)
		if trimmedPhone == "" {
			user.Phone = nil
		} else {
			if !IsValidIndonesianPhone(trimmedPhone) {
				c.JSON(http.StatusBadRequest, gin.H{
					"status":  "error",
					"message": "Format nomor telepon tidak valid. Gunakan format nomor telepon Indonesia yang umum (contoh: 08123456789 atau +628123456789)",
				})
				return
			}
			user.Phone = &trimmedPhone
		}
	}

	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal memperbarui profil",
		})
		return
	}

	avatarURL := user.AvatarURL
	if avatarURL == nil && user.ProfilePhoto != "" {
		avatarURL = &user.ProfilePhoto
	}

	var wilayahData *models.Wilayah
	if user.WilayahID != nil && user.Wilayah.ID != 0 {
		wilayahData = &user.Wilayah
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Profil berhasil diperbarui",
		"data": gin.H{
			"id":            user.ID,
			"name":          user.Name,
			"email":         user.Email,
			"phone":         user.Phone,
			"role":          user.Role,
			"wilayah_id":    user.WilayahID,
			"wilayah":       wilayahData,
			"avatar_url":    avatarURL,
			"last_login_at": user.LastLoginAt,
			"password_changed_at": user.PasswordChangedAt,
		},
	})
}

type ChangePasswordInput struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// PUT /api/profile/password
func ChangePassword(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	var input ChangePasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Format request tidak valid: " + err.Error(),
		})
		return
	}

	if strings.TrimSpace(input.CurrentPassword) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Password saat ini wajib diisi",
		})
		return
	}

	trimmedNewPassword := strings.TrimSpace(input.NewPassword)
	if trimmedNewPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Password baru tidak boleh kosong atau hanya berisi spasi",
		})
		return
	}

	if len(input.NewPassword) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Password baru minimal 6 karakter",
		})
		return
	}

	var user models.User
	if err := config.DB.Where("deleted_at IS NULL").First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Pengguna tidak ditemukan",
		})
		return
	}

	// Verifikasi current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.CurrentPassword)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Password saat ini salah",
		})
		return
	}

	// Enkripsi new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengenkripsi password",
		})
		return
	}

	now := time.Now()
	user.Password = string(hashedPassword)
	user.PasswordChangedAt = &now

	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal memperbarui password",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Password berhasil diperbarui",
	})
}

// PUT /api/profile/avatar
func UploadAvatar(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	fileHeader, err := c.FormFile("avatar")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "File avatar wajib diunggah (field: avatar)",
		})
		return
	}

	// Validasi ukuran file (maksimal 2 MB)
	const maxFileSize = 2 * 1024 * 1024
	if fileHeader.Size > maxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ukuran file avatar melebihi batas maksimal 2 MB",
		})
		return
	}

	// Validasi ekstensi file
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	allowedExtensions := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".webp": true,
	}
	if !allowedExtensions[ext] {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ekstensi file tidak didukung. Format yang diizinkan: JPG, PNG, WEBP",
		})
		return
	}

	// Validasi tipe konten / signature byte
	src, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Gagal membaca file avatar",
		})
		return
	}
	defer src.Close()

	headerBuffer := make([]byte, 512)
	n, err := src.Read(headerBuffer)
	if err != nil && err != io.EOF {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Gagal memeriksa header file avatar",
		})
		return
	}

	detectedMIME := http.DetectContentType(headerBuffer[:n])
	allowedMIMEs := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
	}
	if !allowedMIMEs[detectedMIME] {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Tipe file tidak valid. Format yang didukung: image/jpeg, image/png, image/webp",
		})
		return
	}

	var user models.User
	if err := config.DB.Where("deleted_at IS NULL").First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Pengguna tidak ditemukan",
		})
		return
	}

	oldAvatar := ""
	if user.AvatarURL != nil && *user.AvatarURL != "" {
		oldAvatar = *user.AvatarURL
	} else if user.ProfilePhoto != "" {
		oldAvatar = user.ProfilePhoto
	}

	// Upload avatar baru ke Cloudinary
	newAvatarURL, err := AvatarUploader(fileHeader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengunggah avatar ke storage",
		})
		return
	}

	// Simpan URL baru ke DB
	user.AvatarURL = &newAvatarURL
	user.ProfilePhoto = newAvatarURL
	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal memperbarui avatar di database",
		})
		return
	}

	// Hapus avatar lama di Cloudinary setelah DB update berhasil (jika ada dan berbeda)
	if oldAvatar != "" && oldAvatar != newAvatarURL {
		go func(urlToDelete string) {
			_ = AvatarDeleter(urlToDelete)
		}(oldAvatar)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Avatar berhasil diperbarui",
		"data": gin.H{
			"avatar_url": user.AvatarURL,
		},
	})
}

// DELETE /api/profile/avatar
func DeleteAvatar(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	var user models.User
	if err := config.DB.Where("deleted_at IS NULL").First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Pengguna tidak ditemukan",
		})
		return
	}

	oldAvatar := ""
	if user.AvatarURL != nil && *user.AvatarURL != "" {
		oldAvatar = *user.AvatarURL
	} else if user.ProfilePhoto != "" {
		oldAvatar = user.ProfilePhoto
	}

	// Idempotent: jika tidak ada avatar, langsung return success
	if oldAvatar != "" {
		_ = AvatarDeleter(oldAvatar)
		user.AvatarURL = nil
		user.ProfilePhoto = ""
		if err := config.DB.Save(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Gagal menghapus avatar dari database",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Avatar berhasil dihapus",
	})
}
