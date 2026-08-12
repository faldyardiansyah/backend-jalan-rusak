package superadmin

import (
	"net/http"
	"strconv"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func GetAllUsers(c *gin.Context) {
	roleFilter := c.Query("role")
	searchKeyword := c.Query("search")
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	offset := (page - 1) * limit

	var users []models.User
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

	query.Count(&totalData)

	query.
		Preload("Wilayah").
		Select("id, created_at, name, email, role, wilayah_id, profile_photo").
		Order("id DESC").
		Limit(limit).
		Offset(offset).
		Find(&users)

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

	role := models.UserRole(input.Role)

	if role != models.RoleAdminPemdes &&
		role != models.RoleAdminPu &&
		role != models.RoleSuperAdmin {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Role harus admin pemdes, admin pu, atau super admin",
		})
		return
	}

	if role == models.RoleAdminPemdes && input.WilayahID == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Admin pemdes wajib memiliki wilayah",
		})
		return
	}

	if role != models.RoleAdminPemdes && input.WilayahID != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Admin PU dan Super Admin tidak boleh memiliki wilayah",
		})
		return
	}

	var existingUser models.User

	if err := config.DB.
		Where("email = ?", input.Email).
		First(&existingUser).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Email sudah terdaftar",
		})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword(
		[]byte(input.Password),
		bcrypt.DefaultCost,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal membuat password",
		})
		return
	}

	newUser := models.User{
		Name:      input.Name,
		Email:     input.Email,
		Password:  string(hashedPassword),
		Role:      role,
		WilayahID: input.WilayahID,
	}

	if err := config.DB.Create(&newUser).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal membuat user",
		})
		return
	}

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
	Role      string `json:"role"`
	WilayahID *uint  `json:"wilayah_id"`
}

func UpdateUser(c *gin.Context) {
	id := c.Param("id")

	var user models.User

	if err := config.DB.First(&user, id).Error; err != nil {
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

	role := models.UserRole(input.Role)

	if role != models.RoleAdminPemdes &&
		role != models.RoleAdminPu &&
		role != models.RoleSuperAdmin {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Role tidak valid",
		})
		return
	}

	if role == models.RoleAdminPemdes && input.WilayahID == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Admin pemdes wajib memiliki wilayah",
		})
		return
	}

	if role != models.RoleAdminPemdes && input.WilayahID != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Admin PU dan Super Admin tidak boleh memiliki wilayah",
		})
		return
	}

	user.Name = input.Name
	user.Email = input.Email
	user.Role = role
	user.WilayahID = input.WilayahID

	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal memperbarui pengguna",
		})
		return
	}

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

	if err := config.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Pengguna tidak ditemukan",
		})
		return
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