package attendance

import (
	"encoding/csv"

	"github.com/gin-gonic/gin"
)

// writeCSV streams a CSV response. Small helper duplicated from
// internal/report (ticket #8) rather than shared via a one-function
// internal/csv package — consistent with the "small per-package
// duplication is fine" precedent set since ticket #6/#7.
func writeCSV(c *gin.Context, filename string, header []string, rows [][]string, withBOM bool) {
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename="+filename)

	if withBOM {
		_, _ = c.Writer.Write([]byte("\xEF\xBB\xBF"))
	}

	w := csv.NewWriter(c.Writer)
	_ = w.Write(header)
	_ = w.WriteAll(rows)
	w.Flush()
}
