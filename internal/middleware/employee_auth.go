package middleware

import (
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
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		claims, err := auth.VerifyEmployeeAccessToken(token, secret)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		var employeeUUID pgtype.UUID
		if err := employeeUUID.Scan(claims.EmployeeID); err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
			return
		}

		employee, err := queries.GetEmployeeByID(ctx.Request.Context(), employeeUUID)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
			return
		}

		// Two independent reject conditions — do NOT combine with && or ||.
		if employee.TokenVersion != claims.TokenVersion {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token has been revoked"})
			return
		}

		if !employee.IsActive {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "account is inactive"})
			return
		}

		ctx.Set("employee_id", claims.EmployeeID)
		ctx.Set("role", employee.Role)

		ctx.Next()
	}
}
