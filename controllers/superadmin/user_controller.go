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
	roleFilter := strings.TrimSpace(c.Query("role"))
	searchKeyword := strings.TrimSpace(c.Query("search"))
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")

	page, errPage := strconv.Atoi(pageStr)
	if errPage != nil || page < 1 {
		page = 1
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
		query = query.Where("role = ?", roleFilter)
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
		Select("id, created_at, updated_at, name, email, role, wilayah_id, profile_photo").
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
			"error": InputErrorMsg(err),
		})
		return
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Nama tidak boleh kosong",
		})
		return
	}

	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" || !emailRegex.MatchString(email) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Format email tidak valid",
		})
		return
	}

	password := strings.TrimSpace(input.Password)
	if password == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Password tidak boleh kosong atau hanya berisi spasi",
		})
		return
	}
	if len(password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Password minimal 6 karakter",
		})
		return
	}

	role := models.UserRole(strings.ToLower(strings.TrimSpace(input.Role)))
	if role != models.RoleAdminPemdes &&
		role != models.RoleAdminPu &&
		role != models.RoleSuperAdmin &&
		role != models.RoleWarga {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Role harus salah satu dari: warga, admin_pemdes, admin_pu, super_admin",
		})
		return
	}

	if role == models.RoleAdminPemdes {
		if input.WilayahID == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Admin pemdes wajib memiliki wilayah",
			})
			return
		}
		var w models.Wilayah
		if err := config.DB.Where("deleted_at IS NULL").First(&w, *input.WilayahID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Wilayah yang dipilih tidak valid atau tidak ditemukan",
			})
			return
		}
	} else {
		if input.WilayahID != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Hanya Admin Pemdes yang boleh memiliki wilayah",
			})
			return
		}
	}

	var existingUser models.User
	if err := config.DB.
		Where("LOWER(email) = ? AND deleted_at IS NULL", email).
		First(&existingUser).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Email sudah terdaftar",
		})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal mengenkripsi password",
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
		PasswordChangedAt: &now,
	}

	if err := config.DB.Create(&newUser).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal membuat pengguna",
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
	id := c.Param("id")

	var user models.User
	if err := config.DB.
		Preload("Wilayah").
		Where("deleted_at IS NULL").
		First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Pengguna tidak ditemukan",
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
	id := c.Param("id")

	var user models.User
	if err := config.DB.Where("deleted_at IS NULL").First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Pengguna tidak ditemukan",
		})
		return
	}

	var input UpdateUserInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
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
				"error": "Format email tidak valid",
			})
			return
		}
		var existingUser models.User
		if err := config.DB.
			Where("LOWER(email) = ? AND id != ? AND deleted_at IS NULL", email, user.ID).
			First(&existingUser).Error; err == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Email sudah terdaftar",
			})
			return
		}
		user.Email = email
	}

	password := strings.TrimSpace(input.Password)
	if password != "" {
		if len(password) < 6 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Password minimal 6 karakter",
			})
			return
		}
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Gagal mengenkripsi password",
			})
			return
		}
		user.Password = string(hashedPassword)
		now := time.Now()
		user.PasswordChangedAt = &now
	}

	roleStr := strings.ToLower(strings.TrimSpace(input.Role))
	if roleStr != "" {
		newRole := models.UserRole(roleStr)
		if newRole != models.RoleAdminPemdes &&
			newRole != models.RoleAdminPu &&
			newRole != models.RoleSuperAdmin &&
			newRole != models.RoleWarga {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Role tidak valid",
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
					"error": "Tidak dapat mengubah role Superadmin terakhir",
				})
				return
			}
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
				"error": "Admin pemdes wajib memiliki wilayah",
			})
			return
		}
		var w models.Wilayah
		if err := config.DB.Where("deleted_at IS NULL").First(&w, *targetWilayahID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Wilayah yang dipilih tidak valid atau tidak ditemukan",
			})
			return
		}
		user.WilayahID = targetWilayahID
	} else {
		user.WilayahID = nil
	}

	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal memperbarui pengguna",
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
	id := c.Param("id")

	var user models.User
	if err := config.DB.Where("deleted_at IS NULL").First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Pengguna tidak ditemukan",
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
		if callerID == user.ID {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Tidak dapat menghapus akun sendiri",
			})
			return
		}
	}

	if user.Role == models.RoleSuperAdmin {
		var countSuperadmin int64
		config.DB.Model(&models.User{}).
			Where("role = ? AND id != ? AND deleted_at IS NULL", models.RoleSuperAdmin, user.ID).
			Count(&countSuperadmin)
		if countSuperadmin == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Tidak dapat menghapus Superadmin terakhir",
			})
			return
		}
	}

	if user.Role == models.RoleWarga {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Tidak dapat menghapus warga",
		})
		return
	}

	if err := config.DB.Delete(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal menghapus pengguna",
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
