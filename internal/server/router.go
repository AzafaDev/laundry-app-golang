package server

import (
	"laundry-app-with-golang/internal/config"
	"laundry-app-with-golang/internal/customer"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/employee"
	"laundry-app-with-golang/internal/middleware"
	"laundry-app-with-golang/internal/outlet"
	"laundry-app-with-golang/internal/wilayah"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func NewRouter(customerHandler *customer.Handler, employeeHandler *employee.Handler, wilayahHandler *wilayah.Handler, outletHandler *outlet.Handler, cfg config.Config, queries *db.Queries) *gin.Engine {
	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.FrontendURL},
		AllowMethods:     []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	authMiddleware := middleware.AuthMiddleware(cfg.JWTAccessSecret, queries)
	employeeAuthMiddleware := middleware.EmployeeAuthMiddleware(cfg.JWTEmployeeAccessSecret, queries)

	router.GET("/health", healthCheck)

	router.GET("/api/v1/customer/me", authMiddleware, customerHandler.Me)
	router.GET("/api/v1/customer/profile", authMiddleware, customerHandler.Profile)

	router.POST("/api/v1/customer/auth/register", customerHandler.Register)
	router.POST("/api/v1/customer/auth/login", customerHandler.Login)
	router.POST("/api/v1/customer/auth/refresh", customerHandler.Refresh)
	router.POST("/api/v1/customer/auth/logout", customerHandler.Logout)
	router.POST("/api/v1/customer/auth/verify", customerHandler.Verify)
	router.POST("/api/v1/customer/auth/resend-verification", customerHandler.ResendVerification)
	router.POST("/api/v1/customer/auth/forgot-password", customerHandler.ForgotPassword)
	router.POST("/api/v1/customer/auth/reset-password", customerHandler.ResetPassword)
	router.GET("/api/v1/customer/auth/google", customerHandler.GoogleLogin)
	router.GET("/api/v1/customer/auth/google/callback", customerHandler.GoogleCallback)

	router.PATCH("/api/v1/customer/profile", authMiddleware, customerHandler.UpdateProfile)
	router.PATCH("/api/v1/customer/profile/password", authMiddleware, customerHandler.ChangePassword)

	router.POST("/api/v1/customer/profile/email", authMiddleware, customerHandler.RequestEmailChange)
	router.POST("/api/v1/customer/profile/email/verify", authMiddleware, customerHandler.VerifyEmailChange)
	router.POST("/api/v1/customer/profile/avatar", authMiddleware, customerHandler.UploadAvatar)

	router.GET("/api/v1/customer/addresses", authMiddleware, customerHandler.ListAddresses)
	router.POST("/api/v1/customer/addresses", authMiddleware, customerHandler.CreateAddress)
	router.GET("/api/v1/customer/addresses/:id", authMiddleware, customerHandler.GetAddressByID)
	router.PATCH("/api/v1/customer/addresses/:id", authMiddleware, customerHandler.UpdateAddress)
	router.PATCH("/api/v1/customer/addresses/:id/primary", authMiddleware, customerHandler.SetPrimaryAddress)
	router.DELETE("/api/v1/customer/addresses/:id", authMiddleware, customerHandler.DeleteAddress)

	router.GET("/api/v1/customer/geocode", authMiddleware, customerHandler.Geocode)
	router.GET("/api/v1/customer/geocode/search", authMiddleware, customerHandler.SearchGeocode)

	router.POST("/api/v1/employee/auth/login", employeeHandler.Login)
	router.POST("/api/v1/employee/auth/refresh", employeeHandler.Refresh)
	router.POST("/api/v1/employee/auth/logout", employeeHandler.Logout)
	router.POST("/api/v1/employee/auth/forgot-password", employeeHandler.ForgotPassword)
	router.POST("/api/v1/employee/auth/reset-password", employeeHandler.ResetPassword)

	router.GET("/api/v1/employee/profile", employeeAuthMiddleware, employeeHandler.Profile)
	router.PATCH("/api/v1/employee/profile/password", employeeAuthMiddleware, employeeHandler.ChangePassword)

	router.GET("/api/v1/employee/admin/employees", employeeAuthMiddleware, middleware.RequireRole("super_admin"), employeeHandler.ListEmployees)
	router.GET("/api/v1/employee/admin/employees/:id", employeeAuthMiddleware, middleware.RequireRole("super_admin"), employeeHandler.GetEmployeeByIDAdmin)
	router.POST("/api/v1/employee/admin/employees", employeeAuthMiddleware, middleware.RequireRole("super_admin"), employeeHandler.CreateEmployee)
	router.PATCH("/api/v1/employee/admin/employees/:id", employeeAuthMiddleware, middleware.RequireRole("super_admin"), employeeHandler.UpdateEmployee)
	router.PATCH("/api/v1/employee/admin/employees/:id/outlet", employeeAuthMiddleware, middleware.RequireRole("super_admin"), employeeHandler.AssignEmployeeOutlet)
	router.POST("/api/v1/employee/admin/employees/:id/resend-invite", employeeAuthMiddleware, middleware.RequireRole("super_admin"), employeeHandler.ResendInvite)
	router.DELETE("/api/v1/employee/admin/employees/:id", employeeAuthMiddleware, middleware.RequireRole("super_admin"), employeeHandler.SoftDeleteEmployee)
	router.DELETE("/api/v1/employee/admin/employees/:id/permanent", employeeAuthMiddleware, middleware.RequireRole("super_admin"), employeeHandler.HardDeleteEmployee)

	router.GET("/api/v1/employee/admin/outlets", employeeAuthMiddleware, middleware.RequireRole("super_admin"), outletHandler.ListOutlets)
	router.GET("/api/v1/employee/admin/outlets/:id", employeeAuthMiddleware, middleware.RequireRole("super_admin"), outletHandler.GetOutletByID)
	router.POST("/api/v1/employee/admin/outlets", employeeAuthMiddleware, middleware.RequireRole("super_admin"), outletHandler.CreateOutlet)
	router.PATCH("/api/v1/employee/admin/outlets/:id", employeeAuthMiddleware, middleware.RequireRole("super_admin"), outletHandler.UpdateOutlet)
	router.DELETE("/api/v1/employee/admin/outlets/:id", employeeAuthMiddleware, middleware.RequireRole("super_admin"), outletHandler.SoftDeleteOutlet)

	router.GET("/api/v1/employee/admin/geocode/search", employeeAuthMiddleware, middleware.RequireRole("super_admin"), employeeHandler.SearchGeocode)

	router.GET("/api/v1/wilayah/provinces", wilayahHandler.ListProvinces)
	router.GET("/api/v1/wilayah/provinces/:id/cities", wilayahHandler.ListCitiesByProvince)
	router.GET("/api/v1/wilayah/cities/:id/districts", wilayahHandler.ListDistrictsByCity)

	return router
}

func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
