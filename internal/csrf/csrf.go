package csrf

import (
	"crypto/subtle"
	"net/http"

	"laundry-app-with-golang/internal/apperr"
	"laundry-app-with-golang/internal/auth"

	"github.com/gin-gonic/gin"
)

const CookieName = "csrf_token"
const HeaderName = "X-CSRF-Token"

// GenerateToken returns a random token suitable for the csrf_token cookie.
func GenerateToken() (string, error) {
	return auth.GenerateRandomToken()
}

// Middleware implements the double-submit cookie check: the csrf_token
// cookie and the X-CSRF-Token header must both be present and equal.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		cookieToken, err := c.Cookie(CookieName)
		if err != nil || cookieToken == "" {
			apperr.AbortWithError(c, http.StatusForbidden, "csrf_token_mismatch")
			return
		}

		headerToken := c.GetHeader(HeaderName)
		if headerToken == "" || subtle.ConstantTimeCompare([]byte(cookieToken), []byte(headerToken)) != 1 {
			apperr.AbortWithError(c, http.StatusForbidden, "csrf_token_mismatch")
			return
		}

		c.Next()
	}
}
