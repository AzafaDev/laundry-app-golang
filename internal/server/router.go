package server

import (
	"laundry-app-with-golang/internal/config"
	"laundry-app-with-golang/internal/customer"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/middleware"
	"net/http"

	"github.com/gin-gonic/gin"
)

func NewRouter(customerHandler *customer.Handler, cfg config.Config, queries *db.Queries) *gin.Engine {
	router := gin.Default()
	authMiddleware := middleware.AuthMiddleware(cfg.JWTAccessSecret, queries)

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

	return router
}

func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
