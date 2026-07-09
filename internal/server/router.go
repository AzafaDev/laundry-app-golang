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

	router.POST("/api/v1/customer/register", customerHandler.Register)
	router.POST("/api/v1/customer/login", customerHandler.Login)

	router.GET("/api/v1/customer/me", middleware.AuthMiddleware(cfg.JWTAccessSecret), customerHandler.Me)

	return router
}

func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
