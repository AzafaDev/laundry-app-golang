package middleware

import (
	"laundry-app-with-golang/internal/apperr"
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
			apperr.AbortWithError(ctx, http.StatusUnauthorized, "invalid_session")
			return
		}

		claims, err := auth.VerifyAccessToken(token, secret)
		if err != nil {
			apperr.AbortWithError(ctx, http.StatusUnauthorized, "invalid_session")
			return
		}

		var customerUUID pgtype.UUID
		if err := customerUUID.Scan(claims.CustomerID); err != nil {
			apperr.AbortWithError(ctx, http.StatusUnauthorized, "invalid_session")
			return
		}

		customer, err := queries.GetCustomerByID(ctx.Request.Context(), customerUUID)
		if err != nil {
			apperr.AbortWithError(ctx, http.StatusUnauthorized, "invalid_session")
			return
		}

		if customer.TokenVersion != claims.TokenVersion {
			apperr.AbortWithError(ctx, http.StatusUnauthorized, "token_revoked")
			return
		}

		ctx.Set("customer_id", claims.CustomerID)

		ctx.Next()
	}
}
