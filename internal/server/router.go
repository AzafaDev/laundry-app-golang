package server

import (
	"laundry-app-with-golang/internal/config"
	"laundry-app-with-golang/internal/customer"
	"laundry-app-with-golang/internal/middleware"
	"net/http"

	"github.com/gin-gonic/gin"
)

func NewRouter(customerHandler *customer.Handler, cfg config.Config) *gin.Engine {
	router := gin.Default()

	router.GET("/health", healthCheck)

	router.GET("/api/v1/customer/me", middleware.AuthMiddleware(cfg.JWTAccessSecret), customerHandler.Me)
	router.GET("/api/v1/customer/profile", middleware.AuthMiddleware(cfg.JWTAccessSecret), customerHandler.Profile)

	router.POST("/api/v1/customer/auth/register", customerHandler.Register)
	router.POST("/api/v1/customer/auth/login", customerHandler.Login)
	router.POST("/api/v1/customer/auth/refresh", customerHandler.Refresh)
	router.POST("/api/v1/customer/auth/logout", customerHandler.Logout)
	router.POST("/api/v1/customer/auth/verify", customerHandler.Verify)
	router.POST("/api/v1/customer/auth/resend-verification", customerHandler.ResendVerification)
	router.POST("/api/v1/customer/auth/forgot-password", customerHandler.ForgotPassword)
	router.POST("/api/v1/customer/auth/reset-password", customerHandler.ResetPassword)
	router.PATCH("/api/v1/customer/profile/password", middleware.AuthMiddleware(cfg.JWTAccessSecret), customerHandler.ChangePassword)

	return router
}

func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
