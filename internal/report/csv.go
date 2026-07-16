package report

import (
	"encoding/csv"

	"github.com/gin-gonic/gin"
)

// writeCSV streams a CSV response. withBOM prepends a UTF-8 byte-order-mark
// so Excel opens the file with correct encoding — the TS source does this
// for sales and employee-performance exports, but deliberately not for
// attendance export (replicated as-is, not standardized across all three).
func writeCSV(c *gin.Context, filename string, header []string, rows [][]string, withBOM bool) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename="+filename)

	if withBOM {
		_, _ = c.Writer.Write([]byte("\xEF\xBB\xBF"))
	}

	w := csv.NewWriter(c.Writer)
	_ = w.Write(header)
	_ = w.WriteAll(rows)
	w.Flush()
}
