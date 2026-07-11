package middleware

import (
	"laundry-app-with-golang/internal/auth"
	db "laundry-app-with-golang/internal/db/generated"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

func AuthMiddleware(secret string, queries *db.Queries) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token, err := ctx.Cookie("access_token")
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		claims, err := auth.VerifyAccessToken(token, secret)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		var customerUUID pgtype.UUID
		if err := customerUUID.Scan(claims.CustomerID); err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
			return
		}

		customer, err := queries.GetCustomerByID(ctx.Request.Context(), customerUUID)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
			return
		}

		if customer.TokenVersion != claims.TokenVersion {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token has been revoked"})
			return
		}

		ctx.Set("customer_id", claims.CustomerID)

		ctx.Next()
	}
}
