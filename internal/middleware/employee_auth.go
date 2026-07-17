package middleware

import (
	"laundry-app-with-golang/internal/apperr"
	"laundry-app-with-golang/internal/auth"
	db "laundry-app-with-golang/internal/db/generated"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

func EmployeeAuthMiddleware(secret string, queries *db.Queries) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token, err := ctx.Cookie("staff_access_token")
		if err != nil {
			apperr.AbortWithError(ctx, http.StatusUnauthorized, "invalid_session")
			return
		}

		claims, err := auth.VerifyEmployeeAccessToken(token, secret)
		if err != nil {
			apperr.AbortWithError(ctx, http.StatusUnauthorized, "invalid_session")
			return
		}

		var employeeUUID pgtype.UUID
		if err := employeeUUID.Scan(claims.EmployeeID); err != nil {
			apperr.AbortWithError(ctx, http.StatusUnauthorized, "invalid_session")
			return
		}

		employee, err := queries.GetEmployeeByID(ctx.Request.Context(), employeeUUID)
		if err != nil {
			apperr.AbortWithError(ctx, http.StatusUnauthorized, "invalid_session")
			return
		}

		// Two independent reject conditions — do NOT combine with && or ||.
		if employee.TokenVersion != claims.TokenVersion {
			apperr.AbortWithError(ctx, http.StatusUnauthorized, "token_revoked")
			return
		}

		if !employee.IsActive {
			apperr.AbortWithError(ctx, http.StatusUnauthorized, "account_inactive")
			return
		}

		ctx.Set("employee_id", claims.EmployeeID)
		ctx.Set("role", employee.Role)
		ctx.Set("outlet_id", employee.OutletID)

		ctx.Next()
	}
}
