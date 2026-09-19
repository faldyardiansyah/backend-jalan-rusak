package main

import (
	"backend-jalan-rusak/config"
	"backend-jalan-rusak/routes"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	config.ConnectDatabase()
	config.InitCloudinary()

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowAllOrigins: true, // khusus development
		AllowMethods:    []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:    []string{"Origin", "Content-Type", "Authorization"},
	}))

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	routes.SetupRoutes(r)

	r.Run()
}