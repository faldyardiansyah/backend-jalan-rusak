package routes

import (
	"backend-jalan-rusak/controllers"
	adminController "backend-jalan-rusak/controllers/admin"
	superAdminController "backend-jalan-rusak/controllers/superadmin"
	wargaController "backend-jalan-rusak/controllers/warga"
	"backend-jalan-rusak/middlewares"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {

	public := r.Group("/api")
	{
		public.POST("/register", controllers.Register)
		public.POST("/login", controllers.Login)
	}

	api := r.Group("/api")
	api.Use(middlewares.AuthMiddleware())
	{
		warga := api.Group("/warga")
		warga.Use(middlewares.RequireRole("warga"))
		{
			warga.POST("/laporan", wargaController.CreateLaporan)
			warga.GET("/laporan/riwayat", wargaController.GetRiwayatLaporan)
			warga.GET("/laporan/peta", wargaController.GetAllLaporanPeta)
			warga.GET("/laporan/:id/chat", controllers.GetChatByLaporanID)
			warga.POST("/laporan/:id/chat", controllers.SendPesanWarga)
		}

		admin := api.Group("/admin")
		admin.Use(middlewares.RequireRole("admin_pemdes", "admin_pu", "super_admin"))
		{
			admin.GET("/dashboard", adminController.GetDashboardStats)
			admin.GET("/laporan", adminController.GetAllLaporan)
			admin.PUT("/laporan/:id/status", adminController.UpdateStatusLaporan)
			admin.GET("/map/laporan", adminController.GetMapLaporan)
			admin.GET("/laporan/:id/chat", controllers.GetChatByLaporanID)
			admin.PUT("/chat/:chat_id", controllers.ReplyPesanAdmin)
		}

		superadmin := api.Group("/superadmin")
		superadmin.Use(middlewares.RequireRole("super_admin"))
		{
			superadmin.GET("/users", superAdminController.GetAllUsers)
			superadmin.POST("/users", superAdminController.CreateUser)
			superadmin.GET("/users/:id", superAdminController.ShowUser)
			superadmin.PUT("/users/:id", superAdminController.UpdateUser)
			superadmin.DELETE("/users/:id", superAdminController.DeleteUser)
			// superadmin.GET("/wilayah", superAdminController.GetAllWilayah)
			superadmin.DELETE("/laporan/:id", superAdminController.DeleteLaporanSpam)
		}
	}
}
