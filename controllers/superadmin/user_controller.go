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
	roleFilter  := c.Query("role")
	searchKeyword := c.Query("search")
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)
	offset := (page - 1) * limit

	var users []models.User
	var totalData int64

	query := config.DB.Model(&models.User{}).Where("delete_at IS NULL")

	if roleFilter != "" {
		query = query.Where("role = ?", roleFilter)
	}

	if searchKeyword != "" {
		pattern := "%" + searchKeyword + "%"
		query = query.Where("name LIKE ? OR email LIKE ? OR domisili LIKE ?", pattern, pattern, pattern)
	}

	query.Count(&totalData)

	query.Select("id, created_at, name, email, role, domisili, profile_photo").Offset(offset).Order("id desc").Limit(limit).Offset(offset).Find(&users)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"users": users,
		"page": page,
		"limit": limit,
		"total_data": totalData,
	})
}

func CreateUser(c *gin.Context) {
	var input struct {
		Name  string `json:"name" binding:"required"`
		Email string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
		Role string `json:"role" binding:"required"`
		Domisili string `json:"domisili" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": inputErrorMsg(err) ,
		})
		return
	}

	var existingUser models.User
	if err := config.DB.Where("email = ?" , input.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Email sudah terdaftar",
		})
		return
	}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	newUser := models.User{
		Name:  input.Name,
		Email: input.Email,
		Password: string(hashedPassword),
		Role: input.Role,
		Domisili: input.Domisili,
	}

	if err := config.DB.Create(&newUser).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal membuat pengguna baru",
		})
		return
	}

	newUser.Password = ""
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"message": "Pengguna berhasil dibuat" + input.Role,
		"user": newUser,
	})
}

func ShowUser(c *gin.Context) {
	id := c.Param("id")

	var user models.User
	if err := config.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Pengguna tidak ditemukan",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"user": user,
	})
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

	var input struct {
		Name  string `json:"name" binding:"required"`
		Email string `json:"email" binding:"required"`
		Role string `json:"role" binding:"required"`
		Domisili string `json:"domisili" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": inputErrorMsg(err) ,
		})
		return
	}

	user.Name = input.Name
	user.Email = input.Email
	user.Role = input.Role
	user.Domisili = input.Domisili

	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal memperbarui pengguna",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"message": "Pengguna berhasil diperbarui",
		"user": user,
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

	config.DB.Delete(&user)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"message": "Pengguna berhasil dihapus",
	})
}

func inputErrorMsg(err error) string {
	return "Input tidak valid: " + err.Error()
}