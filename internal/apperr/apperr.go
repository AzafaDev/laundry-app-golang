package apperr

import "github.com/gin-gonic/gin"

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
