package routes

import (
	"backend-jalan-rusak/controllers"
	adminController "backend-jalan-rusak/controllers/admin"
	superadminController "backend-jalan-rusak/controllers/superadmin"
	wargaController "backend-jalan-rusak/controllers/warga"
	"backend-jalan-rusak/middlewares"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {
	public := r.Group("/api"){
		public.POST("/auth/login", controllers.Login)
		public.POST("/auth/register", controllers.Register)
	}


	api := r.Group("/api")
	api.Use(middlewares.JWTAuth()){

		// buat warga
		warga := api.Group("/warga")
		warga.Use(middlewares.RequireRole("warga", "admin", "superadmin"))
		{
			warga.POST("/laporan", wargaController.CreateLaporan)
			warga.GET("laporan/riwayat", wargaController.GetRiwayatLaporan)
			warga.GET("laporan/:id/chat", controllers.GetChatByLaporanID)
			warga.POST("laporan/:id/chat", controllers.SendPesanWarga)
		}

		admin := api.Group("/admin")
		admin.Use(middlewares.RequireRole("admin", "superadmin")){
			admin.GET("/dashboard", adminController.GetDashboard)
			admin.GET("/laporan", adminController.GetAllLaporan)
			admin.PUT("/laporan/:id/status", adminController.UpdateStatusLaporan)
			admin.GET("map/laporan", adminController.GetMapLaporan)
			admin.GET("/laporan/:id/chat", controllers.GetChatByLaporanID)
			admin.PUT("/chat/:chat_id", controllers.ReplyPesanAdmin)
		}
	}
}