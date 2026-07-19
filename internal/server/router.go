package server

import (
	"laundry-app-with-golang/internal/attendance"
	"laundry-app-with-golang/internal/clothingtype"
	"laundry-app-with-golang/internal/config"
	"laundry-app-with-golang/internal/cron"
	"laundry-app-with-golang/internal/csrf"
	"laundry-app-with-golang/internal/customer"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/employee"
	"laundry-app-with-golang/internal/laundryitem"
	"laundry-app-with-golang/internal/middleware"
	"laundry-app-with-golang/internal/notification"
	"laundry-app-with-golang/internal/order"
	"laundry-app-with-golang/internal/outlet"
	"laundry-app-with-golang/internal/payment"
	"laundry-app-with-golang/internal/ratelimit"
	"laundry-app-with-golang/internal/report"
	"laundry-app-with-golang/internal/shift"
	"laundry-app-with-golang/internal/sse"
	"laundry-app-with-golang/internal/wilayah"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func NewRouter(customerHandler *customer.Handler, employeeHandler *employee.Handler, wilayahHandler *wilayah.Handler, outletHandler *outlet.Handler, orderHandler *order.Handler, laundryItemHandler *laundryitem.Handler, clothingTypeHandler *clothingtype.Handler, shiftHandler *shift.Handler, attendanceHandler *attendance.Handler, paymentHandler *payment.Handler, notificationHandler *notification.Handler, cronHandler *cron.Handler, reportHandler *report.Handler, sseHandler *sse.Handler, cfg config.Config, queries *db.Queries) *gin.Engine {
	router := gin.Default()

	isProd := cfg.GoEnv == "production"

	globalMax := 1000
	loginMax := 100
	authMax := 200
	if isProd {
		globalMax = 500
		loginMax = 10
		authMax = 20
	}
	globalLimiter := ratelimit.NewLimiter(rate.Every(15*time.Minute/time.Duration(globalMax)), globalMax)
	loginLimiter := ratelimit.NewLimiter(rate.Every(15*time.Minute/time.Duration(loginMax)), loginMax)
	authLimiter := ratelimit.NewLimiter(rate.Every(15*time.Minute/time.Duration(authMax)), authMax)

	router.Use(ratelimit.Middleware(globalLimiter, false))

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.FrontendURL},
		AllowMethods:     []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	authMiddleware := middleware.AuthMiddleware(cfg.JWTAccessSecret, queries)
	employeeAuthMiddleware := middleware.EmployeeAuthMiddleware(cfg.JWTEmployeeAccessSecret, queries)
	csrfMiddleware := csrf.Middleware()
	loginRateLimit := ratelimit.Middleware(loginLimiter, true)
	authRateLimit := ratelimit.Middleware(authLimiter, false)

	router.GET("/health", healthCheck)

	router.GET("/api/v1/events", sseHandler.Stream)

	router.GET("/api/v1/customer/profile", authMiddleware, customerHandler.Profile)

	router.POST("/api/v1/customer/auth/register", authRateLimit, customerHandler.Register)
	router.POST("/api/v1/customer/auth/login", loginRateLimit, customerHandler.Login)
	router.POST("/api/v1/customer/auth/refresh", customerHandler.Refresh)
	router.POST("/api/v1/customer/auth/logout", customerHandler.Logout)
	router.POST("/api/v1/customer/auth/verify", authRateLimit, customerHandler.Verify)
	router.POST("/api/v1/customer/auth/resend-verification", authRateLimit, customerHandler.ResendVerification)
	router.POST("/api/v1/customer/auth/forgot-password", authRateLimit, customerHandler.ForgotPassword)
	router.POST("/api/v1/customer/auth/reset-password", authRateLimit, customerHandler.ResetPassword)
	router.GET("/api/v1/customer/auth/google", customerHandler.GoogleLogin)
	router.GET("/api/v1/customer/auth/google/callback", customerHandler.GoogleCallback)

	router.PATCH("/api/v1/customer/profile", authMiddleware, csrfMiddleware, customerHandler.UpdateProfile)
	router.PATCH("/api/v1/customer/profile/password", authMiddleware, csrfMiddleware, customerHandler.ChangePassword)

	router.POST("/api/v1/customer/profile/email", authMiddleware, csrfMiddleware, customerHandler.RequestEmailChange)
	router.POST("/api/v1/customer/profile/email/verify", authMiddleware, csrfMiddleware, authRateLimit, customerHandler.VerifyEmailChange)
	router.POST("/api/v1/customer/profile/avatar", authMiddleware, csrfMiddleware, customerHandler.UploadAvatar)

	router.GET("/api/v1/customer/addresses", authMiddleware, customerHandler.ListAddresses)
	router.POST("/api/v1/customer/addresses", authMiddleware, csrfMiddleware, customerHandler.CreateAddress)
	router.GET("/api/v1/customer/addresses/:id", authMiddleware, customerHandler.GetAddressByID)
	router.PATCH("/api/v1/customer/addresses/:id", authMiddleware, csrfMiddleware, customerHandler.UpdateAddress)
	router.PATCH("/api/v1/customer/addresses/:id/primary", authMiddleware, csrfMiddleware, customerHandler.SetPrimaryAddress)
	router.DELETE("/api/v1/customer/addresses/:id", authMiddleware, csrfMiddleware, customerHandler.DeleteAddress)

	router.GET("/api/v1/customer/geocode", authMiddleware, customerHandler.Geocode)
	router.GET("/api/v1/customer/geocode/search", authMiddleware, customerHandler.SearchGeocode)

	router.POST("/api/v1/customer/orders", authMiddleware, csrfMiddleware, orderHandler.CreateOrder)
	router.GET("/api/v1/customer/orders", authMiddleware, orderHandler.ListOrders)
	router.GET("/api/v1/customer/orders/:id", authMiddleware, orderHandler.GetOrderDetail)
	router.POST("/api/v1/customer/orders/:id/complaint", authMiddleware, csrfMiddleware, orderHandler.CreateComplaint)
	router.PATCH("/api/v1/customer/orders/:id/complete", authMiddleware, csrfMiddleware, orderHandler.CompleteOrder)

	router.POST("/api/v1/customer/orders/:id/payment/create-transaction", authMiddleware, csrfMiddleware, paymentHandler.CreateTransaction)
	router.GET("/api/v1/customer/orders/:id/payment/status", authMiddleware, paymentHandler.GetPaymentStatus)
	router.POST("/api/v1/customer/orders/:id/payment/sync", authMiddleware, csrfMiddleware, paymentHandler.SyncPaymentStatus)

	router.GET("/api/v1/customer/notifications", authMiddleware, notificationHandler.ListCustomerNotifications)
	router.GET("/api/v1/customer/notifications/unread-count", authMiddleware, notificationHandler.GetCustomerUnreadCount)
	router.PATCH("/api/v1/customer/notifications/:id/read", authMiddleware, csrfMiddleware, notificationHandler.MarkCustomerNotificationRead)
	router.PATCH("/api/v1/customer/notifications/read-all", authMiddleware, csrfMiddleware, notificationHandler.MarkAllCustomerNotificationsRead)

	// Deliberately public — no authMiddleware. Midtrans calls this directly,
	// no user session.
	router.POST("/api/v1/payment/notification", paymentHandler.HandleWebhook)

	// Deliberately public — no authMiddleware. Used by unauthenticated
	// visitors to see laundry item pricing before signing up.
	router.GET("/api/v1/customer/laundry-items", laundryItemHandler.ListPublicLaundryItems)

	router.POST("/api/v1/employee/auth/login", loginRateLimit, employeeHandler.Login)
	router.POST("/api/v1/employee/auth/refresh", employeeHandler.Refresh)
	router.POST("/api/v1/employee/auth/logout", employeeHandler.Logout)
	router.POST("/api/v1/employee/auth/forgot-password", authRateLimit, employeeHandler.ForgotPassword)
	router.POST("/api/v1/employee/auth/reset-password", employeeHandler.ResetPassword)

	router.GET("/api/v1/employee/profile", employeeAuthMiddleware, employeeHandler.Profile)
	router.PATCH("/api/v1/employee/profile/password", employeeAuthMiddleware, csrfMiddleware, employeeHandler.ChangePassword)

	staffRoles := middleware.RequireRole("washing_worker", "ironing_worker", "packing_worker", "driver")
	router.POST("/api/v1/employee/attendance/check-in", employeeAuthMiddleware, csrfMiddleware, staffRoles, attendanceHandler.CheckIn)
	router.POST("/api/v1/employee/attendance/check-out", employeeAuthMiddleware, csrfMiddleware, staffRoles, attendanceHandler.CheckOut)
	router.GET("/api/v1/employee/attendance/my-logs", employeeAuthMiddleware, staffRoles, attendanceHandler.MyAttendanceLogs)
	router.GET("/api/v1/employee/attendance/today", employeeAuthMiddleware, staffRoles, attendanceHandler.TodayAttendance)
	router.GET("/api/v1/employee/attendance/current-shift", employeeAuthMiddleware, staffRoles, attendanceHandler.CurrentShift)

	router.GET("/api/v1/employee/notifications", employeeAuthMiddleware, notificationHandler.ListEmployeeNotifications)
	router.GET("/api/v1/employee/notifications/unread-count", employeeAuthMiddleware, notificationHandler.GetEmployeeUnreadCount)
	router.PATCH("/api/v1/employee/notifications/:id/read", employeeAuthMiddleware, csrfMiddleware, notificationHandler.MarkEmployeeNotificationRead)
	router.PATCH("/api/v1/employee/notifications/read-all", employeeAuthMiddleware, csrfMiddleware, notificationHandler.MarkAllEmployeeNotificationsRead)

	router.GET("/api/v1/employee/admin/employees", employeeAuthMiddleware, middleware.RequireRole("super_admin", "outlet_admin"), employeeHandler.ListEmployees)
	router.GET("/api/v1/employee/admin/employees/:id", employeeAuthMiddleware, middleware.RequireRole("super_admin"), employeeHandler.GetEmployeeByIDAdmin)
	router.POST("/api/v1/employee/admin/employees", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), employeeHandler.CreateEmployee)
	router.PATCH("/api/v1/employee/admin/employees/:id", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), employeeHandler.UpdateEmployee)
	router.PATCH("/api/v1/employee/admin/employees/:id/outlet", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), employeeHandler.AssignEmployeeOutlet)
	router.POST("/api/v1/employee/admin/employees/:id/resend-invite", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), employeeHandler.ResendInvite)
	router.DELETE("/api/v1/employee/admin/employees/:id", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), employeeHandler.SoftDeleteEmployee)
	router.DELETE("/api/v1/employee/admin/employees/:id/permanent", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), employeeHandler.HardDeleteEmployee)

	router.GET("/api/v1/employee/admin/outlets", employeeAuthMiddleware, middleware.RequireRole("super_admin", "outlet_admin"), outletHandler.ListOutlets)
	router.GET("/api/v1/employee/admin/outlets/:id", employeeAuthMiddleware, middleware.RequireRole("super_admin"), outletHandler.GetOutletByID)
	router.POST("/api/v1/employee/admin/outlets", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), outletHandler.CreateOutlet)
	router.PATCH("/api/v1/employee/admin/outlets/:id", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), outletHandler.UpdateOutlet)
	router.DELETE("/api/v1/employee/admin/outlets/:id", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), outletHandler.SoftDeleteOutlet)

	router.GET("/api/v1/employee/admin/geocode/search", employeeAuthMiddleware, middleware.RequireRole("super_admin"), employeeHandler.SearchGeocode)

	router.GET("/api/v1/employee/admin/laundry-items", employeeAuthMiddleware, middleware.RequireRole("super_admin", "outlet_admin"), laundryItemHandler.ListLaundryItems)
	router.GET("/api/v1/employee/admin/laundry-items/:id", employeeAuthMiddleware, middleware.RequireRole("super_admin"), laundryItemHandler.GetLaundryItemByID)
	router.POST("/api/v1/employee/admin/laundry-items", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), laundryItemHandler.CreateLaundryItem)
	router.PATCH("/api/v1/employee/admin/laundry-items/:id", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), laundryItemHandler.UpdateLaundryItem)
	router.DELETE("/api/v1/employee/admin/laundry-items/:id", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), laundryItemHandler.SoftDeleteLaundryItem)
	router.DELETE("/api/v1/employee/admin/laundry-items/:id/permanent", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), laundryItemHandler.HardDeleteLaundryItem)

	router.GET("/api/v1/employee/admin/clothing-types", employeeAuthMiddleware, middleware.RequireRole("super_admin", "outlet_admin"), clothingTypeHandler.ListClothingTypes)
	router.GET("/api/v1/employee/admin/clothing-types/:id", employeeAuthMiddleware, middleware.RequireRole("super_admin"), clothingTypeHandler.GetClothingTypeByID)
	router.POST("/api/v1/employee/admin/clothing-types", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), clothingTypeHandler.CreateClothingType)
	router.PATCH("/api/v1/employee/admin/clothing-types/:id", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), clothingTypeHandler.UpdateClothingType)
	router.DELETE("/api/v1/employee/admin/clothing-types/:id", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), clothingTypeHandler.SoftDeleteClothingType)

	router.GET("/api/v1/employee/admin/shifts", employeeAuthMiddleware, middleware.RequireRole("super_admin"), shiftHandler.ListWorkShifts)
	router.GET("/api/v1/employee/admin/shifts/deleted", employeeAuthMiddleware, middleware.RequireRole("super_admin"), shiftHandler.ListWorkShiftsDeleted)
	router.GET("/api/v1/employee/admin/shifts/:id", employeeAuthMiddleware, middleware.RequireRole("super_admin"), shiftHandler.GetWorkShiftByID)
	router.POST("/api/v1/employee/admin/shifts", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), shiftHandler.CreateWorkShift)
	router.PATCH("/api/v1/employee/admin/shifts/:id", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), shiftHandler.UpdateWorkShift)
	router.DELETE("/api/v1/employee/admin/shifts/:id", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), shiftHandler.SoftDeleteWorkShift)
	router.DELETE("/api/v1/employee/admin/shifts/:id/permanent", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), shiftHandler.HardDeleteWorkShift)

	router.GET("/api/v1/employee/admin/employees/:id/shifts", employeeAuthMiddleware, middleware.RequireRole("super_admin"), shiftHandler.ListEmployeeShifts)
	router.POST("/api/v1/employee/admin/employees/:id/shifts", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), shiftHandler.CreateEmployeeShift)
	router.DELETE("/api/v1/employee/admin/employees/:id/shifts/:shiftRecordId", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), shiftHandler.DeleteEmployeeShift)

	router.GET("/api/v1/employee/admin/attendance/report", employeeAuthMiddleware, middleware.RequireRole("super_admin", "outlet_admin"), attendanceHandler.ListAttendanceReport)
	router.GET("/api/v1/employee/admin/attendance/report/export", employeeAuthMiddleware, middleware.RequireRole("super_admin", "outlet_admin"), attendanceHandler.ExportAttendanceReport)
	router.POST("/api/v1/employee/admin/attendance/sweep", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), attendanceHandler.TriggerSweep)

	reportRoles := middleware.RequireRole("super_admin", "outlet_admin")
	router.GET("/api/v1/employee/admin/reports/sales", employeeAuthMiddleware, reportRoles, reportHandler.GetSalesReport)
	router.GET("/api/v1/employee/admin/reports/sales/export", employeeAuthMiddleware, reportRoles, reportHandler.ExportSalesReport)
	router.GET("/api/v1/employee/admin/reports/employee-performance", employeeAuthMiddleware, reportRoles, reportHandler.GetEmployeePerformanceReport)
	router.GET("/api/v1/employee/admin/reports/employee-performance/export", employeeAuthMiddleware, reportRoles, reportHandler.ExportEmployeePerformanceReport)

	router.POST("/api/v1/employee/admin/cron/auto-complete-orders", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), cronHandler.TriggerAutoCompleteOrders)
	router.POST("/api/v1/employee/admin/cron/cleanup-tokens", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("super_admin"), cronHandler.TriggerCleanupTokens)

	router.GET("/api/v1/employee/admin/orders", employeeAuthMiddleware, middleware.RequireRole("outlet_admin"), orderHandler.ListOutletOrders)
	router.GET("/api/v1/employee/admin/orders/pending-process", employeeAuthMiddleware, middleware.RequireRole("outlet_admin"), orderHandler.GetPendingProcessOrders)
	router.GET("/api/v1/employee/admin/orders/:id", employeeAuthMiddleware, middleware.RequireRole("outlet_admin"), orderHandler.GetOutletOrderDetail)
	router.POST("/api/v1/employee/admin/orders/:id/process", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("outlet_admin"), orderHandler.ProcessOrder)

	router.GET("/api/v1/employee/admin/bypass-requests", employeeAuthMiddleware, middleware.RequireRole("super_admin", "outlet_admin"), orderHandler.ListBypassRequests)
	router.GET("/api/v1/employee/admin/bypass-requests/:id", employeeAuthMiddleware, middleware.RequireRole("super_admin", "outlet_admin"), orderHandler.GetBypassRequest)
	router.PATCH("/api/v1/employee/admin/bypass-requests/:id/review", employeeAuthMiddleware, csrfMiddleware, middleware.RequireRole("outlet_admin"), orderHandler.ReviewBypassRequest)

	adminComplaintRoles := middleware.RequireRole("super_admin", "outlet_admin")
	router.GET("/api/v1/employee/admin/complaints", employeeAuthMiddleware, adminComplaintRoles, orderHandler.ListComplaints)
	router.GET("/api/v1/employee/admin/complaints/stats", employeeAuthMiddleware, adminComplaintRoles, orderHandler.GetComplaintStats)
	router.GET("/api/v1/employee/admin/complaints/:id", employeeAuthMiddleware, adminComplaintRoles, orderHandler.GetComplaintByID)
	router.PATCH("/api/v1/employee/admin/complaints/:id/status", employeeAuthMiddleware, csrfMiddleware, adminComplaintRoles, orderHandler.UpdateComplaintStatus)

	router.GET("/api/v1/employee/admin/dashboard/stats", employeeAuthMiddleware, adminComplaintRoles, orderHandler.GetDashboardStats)

	workerRoles := middleware.RequireRole("washing_worker", "ironing_worker", "packing_worker")
	router.GET("/api/v1/employee/worker/station/:station/orders", employeeAuthMiddleware, workerRoles, orderHandler.GetStationOrders)
	router.GET("/api/v1/employee/worker/station/:station/orders/:orderId/items", employeeAuthMiddleware, workerRoles, orderHandler.GetStationOrderItems)
	router.GET("/api/v1/employee/worker/station/:station/history", employeeAuthMiddleware, workerRoles, orderHandler.GetStationHistory)
	router.POST("/api/v1/employee/worker/station/:station/orders/:orderId/submit-items", employeeAuthMiddleware, csrfMiddleware, workerRoles, orderHandler.SubmitItems)
	router.PATCH("/api/v1/employee/worker/station/:station/orders/:orderId/complete", employeeAuthMiddleware, csrfMiddleware, workerRoles, orderHandler.CompleteStation)
	router.POST("/api/v1/employee/worker/bypass", employeeAuthMiddleware, csrfMiddleware, workerRoles, orderHandler.CreateBypassRequest)
	router.GET("/api/v1/employee/worker/orders/:orderId/bypass", employeeAuthMiddleware, workerRoles, orderHandler.GetBypassByOrder)

	driverRoles := middleware.RequireRole("driver")
	router.GET("/api/v1/employee/driver/pickups/available", employeeAuthMiddleware, driverRoles, orderHandler.GetAvailablePickups)
	router.GET("/api/v1/employee/driver/deliveries/available", employeeAuthMiddleware, driverRoles, orderHandler.GetAvailableDeliveries)
	router.GET("/api/v1/employee/driver/tasks/active", employeeAuthMiddleware, driverRoles, orderHandler.GetActiveTask)
	router.POST("/api/v1/employee/driver/tasks/:taskId/claim", employeeAuthMiddleware, csrfMiddleware, driverRoles, orderHandler.ClaimTask)
	router.PATCH("/api/v1/employee/driver/tasks/:taskId/complete", employeeAuthMiddleware, csrfMiddleware, driverRoles, orderHandler.CompleteTask)
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
