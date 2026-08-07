package main

import (
	"backend-jalan-rusak/config"
	
	"github.com/gin-gonic/gin"
)

func main() {

	// buat ngehubungin databasenya
	config.ConnectDatabase()
	config.InitCloudinary()

	r := gin.Default()

	r.GET("/ping", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{
			"status":  "success",
			"message": "Backend Go Gin untuk Laporan Jalan Rusak siap digunakan!",
		})
	})

	r.Run()
}