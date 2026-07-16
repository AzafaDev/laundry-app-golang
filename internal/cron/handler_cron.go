package cron

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// TriggerAutoCompleteOrders is a super_admin-only manual trigger for
// RunAutoCompleteOrders, so it can be exercised now without waiting for the
// hourly ticker — same rationale as ticket #5's TriggerSweep.
func (h *Handler) TriggerAutoCompleteOrders(c *gin.Context) {
	completed, err := RunAutoCompleteOrders(c.Request.Context(), h.Pool, h.Queries)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"completed": completed})
}

// TriggerCleanupTokens is a super_admin-only manual trigger for
// RunCleanupTokens.
func (h *Handler) TriggerCleanupTokens(c *gin.Context) {
	if err := RunCleanupTokens(c.Request.Context(), h.Queries); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "cleanup completed"})
}
