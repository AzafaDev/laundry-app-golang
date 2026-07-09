package server

import (
	"laundry-app-with-golang/internal/customer"
	"net/http"

	"github.com/gin-gonic/gin"
)

func NewRouter(customerHandler *customer.Handler) *gin.Engine {
	router := gin.Default()

	router.GET("/health", healthCheck)

	router.POST("/api/v1/customer/register", customerHandler.Register)

	return router
}

func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
