package server

import (
	"laundry-app-with-golang/internal/attendance"
	"laundry-app-with-golang/internal/clothingtype"
	"laundry-app-with-golang/internal/config"
	"laundry-app-with-golang/internal/customer"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/employee"
	"laundry-app-with-golang/internal/laundryitem"
	"laundry-app-with-golang/internal/middleware"
	"laundry-app-with-golang/internal/order"
	"laundry-app-with-golang/internal/outlet"
	"laundry-app-with-golang/internal/shift"
	"laundry-app-with-golang/internal/wilayah"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func NewRouter(customerHandler *customer.Handler, employeeHandler *employee.Handler, wilayahHandler *wilayah.Handler, outletHandler *outlet.Handler, orderHandler *order.Handler, laundryItemHandler *laundryitem.Handler, clothingTypeHandler *clothingtype.Handler, shiftHandler *shift.Handler, attendanceHandler *attendance.Handler, cfg config.Config, queries *db.Queries) *gin.Engine {
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

	router.POST("/api/v1/customer/orders", authMiddleware, orderHandler.CreateOrder)
	router.GET("/api/v1/customer/orders", authMiddleware, orderHandler.ListOrders)
	router.POST("/api/v1/customer/orders/:id/complaint", authMiddleware, orderHandler.CreateComplaint)

	// Deliberately public — no authMiddleware. Used by unauthenticated
	// visitors to see laundry item pricing before signing up.
	router.GET("/api/v1/customer/laundry-items", laundryItemHandler.ListPublicLaundryItems)

	router.POST("/api/v1/employee/auth/login", employeeHandler.Login)
	router.POST("/api/v1/employee/auth/refresh", employeeHandler.Refresh)
	router.POST("/api/v1/employee/auth/logout", employeeHandler.Logout)
	router.POST("/api/v1/employee/auth/forgot-password", employeeHandler.ForgotPassword)
	router.POST("/api/v1/employee/auth/reset-password", employeeHandler.ResetPassword)

	router.GET("/api/v1/employee/profile", employeeAuthMiddleware, employeeHandler.Profile)
	router.PATCH("/api/v1/employee/profile/password", employeeAuthMiddleware, employeeHandler.ChangePassword)

	router.POST("/api/v1/employee/attendance/check-in", employeeAuthMiddleware, attendanceHandler.CheckIn)
	router.POST("/api/v1/employee/attendance/check-out", employeeAuthMiddleware, attendanceHandler.CheckOut)
	router.GET("/api/v1/employee/attendance/my-logs", employeeAuthMiddleware, attendanceHandler.MyAttendanceLogs)
	router.GET("/api/v1/employee/attendance/today", employeeAuthMiddleware, attendanceHandler.TodayAttendance)
	router.GET("/api/v1/employee/attendance/current-shift", employeeAuthMiddleware, attendanceHandler.CurrentShift)

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

	router.GET("/api/v1/employee/admin/laundry-items", employeeAuthMiddleware, middleware.RequireRole("super_admin"), laundryItemHandler.ListLaundryItems)
	router.GET("/api/v1/employee/admin/laundry-items/:id", employeeAuthMiddleware, middleware.RequireRole("super_admin"), laundryItemHandler.GetLaundryItemByID)
	router.POST("/api/v1/employee/admin/laundry-items", employeeAuthMiddleware, middleware.RequireRole("super_admin"), laundryItemHandler.CreateLaundryItem)
	router.PATCH("/api/v1/employee/admin/laundry-items/:id", employeeAuthMiddleware, middleware.RequireRole("super_admin"), laundryItemHandler.UpdateLaundryItem)
	router.DELETE("/api/v1/employee/admin/laundry-items/:id", employeeAuthMiddleware, middleware.RequireRole("super_admin"), laundryItemHandler.SoftDeleteLaundryItem)
	router.DELETE("/api/v1/employee/admin/laundry-items/:id/permanent", employeeAuthMiddleware, middleware.RequireRole("super_admin"), laundryItemHandler.HardDeleteLaundryItem)

	router.GET("/api/v1/employee/admin/clothing-types", employeeAuthMiddleware, middleware.RequireRole("super_admin"), clothingTypeHandler.ListClothingTypes)
	router.GET("/api/v1/employee/admin/clothing-types/:id", employeeAuthMiddleware, middleware.RequireRole("super_admin"), clothingTypeHandler.GetClothingTypeByID)
	router.POST("/api/v1/employee/admin/clothing-types", employeeAuthMiddleware, middleware.RequireRole("super_admin"), clothingTypeHandler.CreateClothingType)
	router.PATCH("/api/v1/employee/admin/clothing-types/:id", employeeAuthMiddleware, middleware.RequireRole("super_admin"), clothingTypeHandler.UpdateClothingType)
	router.DELETE("/api/v1/employee/admin/clothing-types/:id", employeeAuthMiddleware, middleware.RequireRole("super_admin"), clothingTypeHandler.SoftDeleteClothingType)

	router.GET("/api/v1/employee/admin/shifts", employeeAuthMiddleware, middleware.RequireRole("super_admin"), shiftHandler.ListWorkShifts)
	router.GET("/api/v1/employee/admin/shifts/:id", employeeAuthMiddleware, middleware.RequireRole("super_admin"), shiftHandler.GetWorkShiftByID)
	router.POST("/api/v1/employee/admin/shifts", employeeAuthMiddleware, middleware.RequireRole("super_admin"), shiftHandler.CreateWorkShift)
	router.PATCH("/api/v1/employee/admin/shifts/:id", employeeAuthMiddleware, middleware.RequireRole("super_admin"), shiftHandler.UpdateWorkShift)
	router.DELETE("/api/v1/employee/admin/shifts/:id", employeeAuthMiddleware, middleware.RequireRole("super_admin"), shiftHandler.SoftDeleteWorkShift)
	router.DELETE("/api/v1/employee/admin/shifts/:id/permanent", employeeAuthMiddleware, middleware.RequireRole("super_admin"), shiftHandler.HardDeleteWorkShift)

	router.GET("/api/v1/employee/admin/employees/:id/shifts", employeeAuthMiddleware, middleware.RequireRole("super_admin"), shiftHandler.ListEmployeeShifts)
	router.POST("/api/v1/employee/admin/employees/:id/shifts", employeeAuthMiddleware, middleware.RequireRole("super_admin"), shiftHandler.CreateEmployeeShift)
	router.DELETE("/api/v1/employee/admin/employees/:id/shifts/:shiftRecordId", employeeAuthMiddleware, middleware.RequireRole("super_admin"), shiftHandler.DeleteEmployeeShift)

	router.GET("/api/v1/employee/admin/attendance/report", employeeAuthMiddleware, middleware.RequireRole("super_admin"), attendanceHandler.ListAttendanceReport)
	router.POST("/api/v1/employee/admin/attendance/sweep", employeeAuthMiddleware, middleware.RequireRole("super_admin"), attendanceHandler.TriggerSweep)

	router.POST("/api/v1/employee/admin/orders/:id/process", employeeAuthMiddleware, middleware.RequireRole("outlet_admin"), orderHandler.ProcessOrder)

	router.GET("/api/v1/employee/admin/bypass-requests", employeeAuthMiddleware, middleware.RequireRole("super_admin", "outlet_admin"), orderHandler.ListBypassRequests)
	router.GET("/api/v1/employee/admin/bypass-requests/:id", employeeAuthMiddleware, middleware.RequireRole("super_admin", "outlet_admin"), orderHandler.GetBypassRequest)
	router.PATCH("/api/v1/employee/admin/bypass-requests/:id/review", employeeAuthMiddleware, middleware.RequireRole("outlet_admin"), orderHandler.ReviewBypassRequest)

	workerRoles := middleware.RequireRole("washing_worker", "ironing_worker", "packing_worker")
	router.GET("/api/v1/employee/worker/station/:station/orders", employeeAuthMiddleware, workerRoles, orderHandler.GetStationOrders)
	router.POST("/api/v1/employee/worker/station/:station/orders/:orderId/submit-items", employeeAuthMiddleware, workerRoles, orderHandler.SubmitItems)
	router.PATCH("/api/v1/employee/worker/station/:station/orders/:orderId/complete", employeeAuthMiddleware, workerRoles, orderHandler.CompleteStation)
	router.POST("/api/v1/employee/worker/bypass", employeeAuthMiddleware, workerRoles, orderHandler.CreateBypassRequest)
	router.GET("/api/v1/employee/worker/orders/:orderId/bypass", employeeAuthMiddleware, workerRoles, orderHandler.GetBypassByOrder)

	driverRoles := middleware.RequireRole("driver")
	router.GET("/api/v1/employee/driver/pickups/available", employeeAuthMiddleware, driverRoles, orderHandler.GetAvailablePickups)
	router.GET("/api/v1/employee/driver/deliveries/available", employeeAuthMiddleware, driverRoles, orderHandler.GetAvailableDeliveries)
	router.GET("/api/v1/employee/driver/tasks/active", employeeAuthMiddleware, driverRoles, orderHandler.GetActiveTask)
	router.POST("/api/v1/employee/driver/tasks/:taskId/claim", employeeAuthMiddleware, driverRoles, orderHandler.ClaimTask)
	router.PATCH("/api/v1/employee/driver/tasks/:taskId/complete", employeeAuthMiddleware, driverRoles, orderHandler.CompleteTask)
	router.GET("/api/v1/employee/driver/tasks/history", employeeAuthMiddleware, driverRoles, orderHandler.GetTaskHistory)

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
