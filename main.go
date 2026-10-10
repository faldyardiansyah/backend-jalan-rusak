package main

import (
	"log"

	"backend-jalan-rusak/config"
	"backend-jalan-rusak/middlewares"
	"backend-jalan-rusak/routes"

	"github.com/gin-gonic/gin"
)

func main() {
	config.ConnectDatabase()
	config.InitCloudinary()

	// Guard konfigurasi CORS mode production: fail-closed saat startup jika allowlist kosong/invalid
	if err := middlewares.ValidateCORSConfig(); err != nil {
		log.Fatalf("Fatal: %v", err)
	}

	r := gin.Default()

	r.Use(middlewares.CORSMiddleware())

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	routes.SetupRoutes(r)

	r.Run()
}