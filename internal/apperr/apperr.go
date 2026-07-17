package apperr

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RespondError sends a structured error code to the client instead of a raw
// English sentence, so the frontend can map it to localized text.
func RespondError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"error": code})
}

// AbortWithError is RespondError for middleware, which must stop the
// handler chain via Abort instead of just writing a response.
func AbortWithError(c *gin.Context, status int, code string) {
	c.AbortWithStatusJSON(status, gin.H{"error": code})
}

// RespondInternalError logs the real error server-side and responds with a
// generic body — callers must never forward err.Error() to the client.
func RespondInternalError(c *gin.Context, err error) {
	log.Printf("internal error: %v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
}
