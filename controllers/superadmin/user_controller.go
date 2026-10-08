package superadmin

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func GetAllUsers(c *gin.Context) {
	if config.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Koneksi database tidak tersedia",
		})
		return
	}

	roleFilter := strings.TrimSpace(c.Query("role"))
	searchKeyword := strings.TrimSpace(c.Query("search"))
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")

	page, errPage := strconv.Atoi(pageStr)
	if errPage != nil || page < 1 {
		page = 1
	}
	if page > 100000 {
		page = 100000
	}
	limit, errLimit := strconv.Atoi(limitStr)
	if errLimit != nil || limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	offset := (page - 1) * limit

	users := make([]models.User, 0)
	var totalData int64

	query := config.DB.
		Model(&models.User{}).
		Where("deleted_at IS NULL")

	if roleFilter != "" {
		roleLower := strings.ToLower(roleFilter)
		if roleLower == string(models.RoleWarga) ||
			roleLower == string(models.RoleAdminPemdes) ||
			roleLower == string(models.RoleAdminPu) ||
			roleLower == string(models.RoleSuperAdmin) {
			query = query.Where("role = ?", roleLower)
		} else {
			query = query.Where("1 = 0")
		}
	}

	if searchKeyword != "" {
		pattern := "%" + searchKeyword + "%"
		query = query.Where(
			"name LIKE ? OR email LIKE ?",
			pattern,
			pattern,
		)
	}

	if err := query.Count(&totalData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menghitung total pengguna",
		})
		return
	}

	if err := query.
		Preload("Wilayah").
		Select("id, created_at, updated_at, name, email, role, wilayah_id, profile_photo, phone").
		Order("id DESC").
		Limit(limit).
		Offset(offset).
		Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil daftar pengguna",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"users":      users,
			"page":       page,
			"limit":      limit,
			"total":      totalData,
			"total_data": totalData,
		},
	})
}

type CreateUserInput struct {
	Name      string `json:"name" binding:"required"`
	Email     string `json:"email" binding:"required"`
	Password  string `json:"password" binding:"required"`
	Role      string `json:"role" binding:"required"`
	WilayahID *uint  `json:"wilayah_id"`
}

func CreateUser(c *gin.Context) {
	var input CreateUserInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": InputErrorMsg(err),
			"error":   InputErrorMsg(err),
		})
		return
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Nama tidak boleh kosong",
			"error":   "Nama tidak boleh kosong",
		})
		return
	}

	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" || !emailRegex.MatchString(email) {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Format email tidak valid",
			"error":   "Format email tidak valid",
		})
		return
	}

	password := strings.TrimSpace(input.Password)
	if password == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Password tidak boleh kosong atau hanya berisi spasi",
			"error":   "Password tidak boleh kosong atau hanya berisi spasi",
		})
		return
	}
	if len(password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Password minimal 6 karakter",
			"error":   "Password minimal 6 karakter",
		})
		return
	}

	role := models.UserRole(strings.ToLower(strings.TrimSpace(input.Role)))
	if role != models.RoleAdminPemdes &&
		role != models.RoleAdminPu &&
		role != models.RoleSuperAdmin &&
		role != models.RoleWarga {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Role harus salah satu dari: warga, admin_pemdes, admin_pu, super_admin",
			"error":   "Role harus salah satu dari: warga, admin_pemdes, admin_pu, super_admin",
		})
		return
	}

	if role == models.RoleAdminPemdes {
		if input.WilayahID == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Admin pemdes wajib memiliki wilayah",
				"error":   "Admin pemdes wajib memiliki wilayah",
			})
			return
		}
	} else {
		if input.WilayahID != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Hanya Admin Pemdes yang boleh memiliki wilayah",
				"error":   "Hanya Admin Pemdes yang boleh memiliki wilayah",
			})
			return
		}
	}

	if config.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"error":   "Koneksi database tidak tersedia",
			"message": "Koneksi database tidak tersedia",
		})
		return
	}

	if role == models.RoleAdminPemdes {
		var w models.Wilayah
		if err := config.DB.Where("deleted_at IS NULL").First(&w, *input.WilayahID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Wilayah yang dipilih tidak valid atau tidak ditemukan",
				"error":   "Wilayah yang dipilih tidak valid atau tidak ditemukan",
			})
			return
		}
	}

	var existingUser models.User
	if err := config.DB.
		Where("LOWER(email) = ? AND deleted_at IS NULL", email).
		First(&existingUser).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Email sudah terdaftar",
			"error":   "Email sudah terdaftar",
		})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengenkripsi password",
			"error":   "Gagal mengenkripsi password",
		})
		return
	}

	now := time.Now()
	newUser := models.User{
		Name:              name,
		Email:             email,
		Password:          string(hashedPassword),
		Role:              role,
		WilayahID:         input.WilayahID,
		TokenVersion:      1,
		PasswordChangedAt: &now,
	}

	if err := config.DB.Create(&newUser).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal membuat pengguna",
			"error":   "Gagal membuat pengguna",
		})
		return
	}

	config.DB.Preload("Wilayah").First(&newUser, newUser.ID)
	newUser.Password = ""

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Pengguna berhasil dibuat",
		"data":    newUser,
	})
}

func ShowUser(c *gin.Context) {
	idStr := c.Param("id")
	id, errID := strconv.ParseUint(idStr, 10, 32)
	if errID != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"error":   "ID pengguna tidak valid",
			"message": "ID pengguna tidak valid",
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

	var user models.User
	if err := config.DB.
		Preload("Wilayah").
		Where("deleted_at IS NULL").
		First(&user, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Pengguna tidak ditemukan",
			"error":   "Pengguna tidak ditemukan",
		})
		return
	}

	user.Password = ""

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   user,
	})
}

type UpdateUserInput struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	Role      string `json:"role"`
	WilayahID *uint  `json:"wilayah_id"`
}

func UpdateUser(c *gin.Context) {
	idStr := c.Param("id")
	id, errID := strconv.ParseUint(idStr, 10, 32)
	if errID != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"error":   "ID pengguna tidak valid",
			"message": "ID pengguna tidak valid",
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

	var user models.User
	if err := config.DB.Where("deleted_at IS NULL").First(&user, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Pengguna tidak ditemukan",
			"error":   "Pengguna tidak ditemukan",
		})
		return
	}

	var input UpdateUserInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": err.Error(),
			"error":   err.Error(),
		})
		return
	}

	name := strings.TrimSpace(input.Name)
	if name != "" {
		user.Name = name
	}

	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email != "" && email != strings.ToLower(user.Email) {
		if !emailRegex.MatchString(email) {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Format email tidak valid",
				"error":   "Format email tidak valid",
			})
			return
		}
		var existingUser models.User
		if err := config.DB.
			Where("LOWER(email) = ? AND id != ? AND deleted_at IS NULL", email, user.ID).
			First(&existingUser).Error; err == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Email sudah terdaftar",
				"error":   "Email sudah terdaftar",
			})
			return
		}
		user.Email = email
	}

	if strings.TrimSpace(input.Password) != "" {
		password := strings.TrimSpace(input.Password)
		if len(password) < 6 {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Password minimal 6 karakter",
				"error":   "Password minimal 6 karakter",
			})
			return
		}
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Gagal mengenkripsi password",
				"error":   "Gagal mengenkripsi password",
			})
			return
		}
		user.Password = string(hashedPassword)
		now := time.Now()
		user.PasswordChangedAt = &now
		user.TokenVersion = user.TokenVersion + 1
	}

	roleStr := strings.ToLower(strings.TrimSpace(input.Role))
	if roleStr != "" {
		newRole := models.UserRole(roleStr)
		if newRole != models.RoleAdminPemdes &&
			newRole != models.RoleAdminPu &&
			newRole != models.RoleSuperAdmin &&
			newRole != models.RoleWarga {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Role tidak valid",
				"error":   "Role tidak valid",
			})
			return
		}

		// Proteksi Superadmin terakhir
		if user.Role == models.RoleSuperAdmin && newRole != models.RoleSuperAdmin {
			var countSuperadmin int64
			config.DB.Model(&models.User{}).
				Where("role = ? AND id != ? AND deleted_at IS NULL", models.RoleSuperAdmin, user.ID).
				Count(&countSuperadmin)
			if countSuperadmin == 0 {
				c.JSON(http.StatusBadRequest, gin.H{
					"status":  "error",
					"message": "Tidak dapat mengubah role Superadmin terakhir",
					"error":   "Tidak dapat mengubah role Superadmin terakhir",
				})
				return
			}
		}

		if newRole != user.Role {
			user.TokenVersion = user.TokenVersion + 1
		}
		user.Role = newRole
	}

	if user.Role == models.RoleAdminPemdes {
		targetWilayahID := input.WilayahID
		if targetWilayahID == nil {
			targetWilayahID = user.WilayahID
		}
		if targetWilayahID == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Admin pemdes wajib memiliki wilayah",
				"error":   "Admin pemdes wajib memiliki wilayah",
			})
			return
		}
		var w models.Wilayah
		if err := config.DB.Where("deleted_at IS NULL").First(&w, *targetWilayahID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Wilayah yang dipilih tidak valid atau tidak ditemukan",
				"error":   "Wilayah yang dipilih tidak valid atau tidak ditemukan",
			})
			return
		}
		if user.WilayahID == nil || *user.WilayahID != *targetWilayahID {
			user.TokenVersion = user.TokenVersion + 1
		}
		user.WilayahID = targetWilayahID
	} else {
		if input.WilayahID != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Hanya Admin Pemdes yang boleh memiliki wilayah",
				"error":   "Hanya Admin Pemdes yang boleh memiliki wilayah",
			})
			return
		}
		if user.WilayahID != nil {
			user.TokenVersion = user.TokenVersion + 1
		}
		user.WilayahID = nil
	}

	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal memperbarui pengguna",
			"error":   "Gagal memperbarui pengguna",
		})
		return
	}

	config.DB.Preload("Wilayah").First(&user, user.ID)
	user.Password = ""

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Pengguna berhasil diperbarui",
		"data":    user,
	})
}

func DeleteUser(c *gin.Context) {
	idStr := c.Param("id")
	id, errID := strconv.ParseUint(idStr, 10, 32)
	if errID != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"error":   "ID pengguna tidak valid",
			"message": "ID pengguna tidak valid",
		})
		return
	}

	callerIDVal, existsCaller := c.Get("user_id")
	if existsCaller {
		var callerID uint
		switch v := callerIDVal.(type) {
		case uint:
			callerID = v
		case float64:
			callerID = uint(v)
		case int:
			callerID = uint(v)
		}
		if callerID == uint(id) {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Tidak dapat menghapus akun sendiri",
				"error":   "Tidak dapat menghapus akun sendiri",
			})
			return
		}
	}

	if config.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"error":   "Koneksi database tidak tersedia",
			"message": "Koneksi database tidak tersedia",
		})
		return
	}

	var user models.User
	if err := config.DB.Where("deleted_at IS NULL").First(&user, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Pengguna tidak ditemukan",
			"error":   "Pengguna tidak ditemukan",
		})
		return
	}

	if user.Role == models.RoleSuperAdmin {
		var countSuperadmin int64
		config.DB.Model(&models.User{}).
			Where("role = ? AND id != ? AND deleted_at IS NULL", models.RoleSuperAdmin, user.ID).
			Count(&countSuperadmin)
		if countSuperadmin == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Tidak dapat menghapus Superadmin terakhir",
				"error":   "Tidak dapat menghapus Superadmin terakhir",
			})
			return
		}
	}

	if user.Role == models.RoleWarga {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Tidak dapat menghapus warga",
			"error":   "Tidak dapat menghapus warga",
		})
		return
	}

	if err := config.DB.Delete(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menghapus pengguna",
			"error":   "Gagal menghapus pengguna",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Pengguna berhasil dihapus",
	})
}

func InputErrorMsg(err error) string {
	return "Input tidak valid: " + err.Error()
}
