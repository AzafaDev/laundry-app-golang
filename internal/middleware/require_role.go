package middleware

import (
	"laundry-app-with-golang/internal/apperr"
	"net/http"

	"github.com/gin-gonic/gin"
)

func RequireRole(roles ...string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		role, exists := ctx.Get("role")
		if !exists {
			apperr.AbortWithError(ctx, http.StatusUnauthorized, "invalid_session")
			return
		}

		roleStr, _ := role.(string)
		for _, allowed := range roles {
			if roleStr == allowed {
				ctx.Next()
				return
			}
		}

		apperr.AbortWithError(ctx, http.StatusForbidden, "insufficient_permissions")
	}
}
