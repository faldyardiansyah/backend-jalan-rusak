package admin

import (
	"net/http"
	"strconv"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/models"
	
	"github.com/gin-gonic/gin"
)

func GetAllLaporan(c *gin.Context) {
    roleVal, _ := c.Get("role")
    userIDVal, _ := c.Get("user_id")
    role := roleVal.(string)
    userID := userIDVal.(uint)

    statusFilter := c.Query("status")
    searchKeyword := c.Query("search")
    pageStr := c.DefaultQuery("page", "1")
    limitStr := c.DefaultQuery("limit", "10")

    page, _ := strconv.Atoi(pageStr)
    limit, _ := strconv.Atoi(limitStr)
    offset := (page - 1) * limit

    var listLaporan []models.LaporanKerusakan
    var totalData int64

    query := config.DB.Model(&models.LaporanKerusakan{}).
        Preload("User").
        Preload("Wilayah").
        Joins("JOIN user ON user.id = laporan_kerusakan.user_id"). 
        Where("laporan_kerusakan.deleted_at IS NULL")

    if role == "admin_pemdes" {
        var adminUser models.User
        config.DB.First(&adminUser, userID)
        
        query = query.Where("laporan_kerusakan.wilayah_id = ? AND laporan_kerusakan.jenis_jalan = ?", adminUser.WilayahID, "desa")
    } else if role == "admin_pu" {
        query = query.Where("laporan_kerusakan.jenis_jalan = ?", "kabupaten")
    }
    // Jika superadmin, tidak ada filter wilayah

    if statusFilter != "" {
        query = query.Where("laporan_kerusakan.status = ?", statusFilter)
    }

    if searchKeyword != "" {
        likePattern := "%" + searchKeyword + "%"
        query = query.Where("laporan_kerusakan.judul LIKE ? OR laporan_kerusakan.deskripsi LIKE ? OR user.name LIKE ?", likePattern, likePattern, likePattern)
    }

    query.Count(&totalData)

    err := query.Order("laporan_kerusakan.created_at DESC").
        Limit(limit).
        Offset(offset).
        Find(&listLaporan).Error

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Gagal mengambil laporan kerusakan",
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "status":  "success",
        "message": "Laporan kerusakan berhasil diambil",
        "data":    listLaporan,
        "total":   totalData,
    })
}