package apphelper

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

func newTestContext(rawQuery string) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/?"+rawQuery, nil)
	return c
}

func TestParsePagination(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		defaultLimit int32
		maxLimit     int32
		wantLimit    int32
		wantOffset   int32
	}{
		{
			name:         "no params falls back to default",
			query:        "",
			defaultLimit: 10,
			maxLimit:     50,
			wantLimit:    10,
			wantOffset:   0,
		},
		{
			name:         "explicit valid limit and offset",
			query:        "limit=20&offset=40",
			defaultLimit: 10,
			maxLimit:     50,
			wantLimit:    20,
			wantOffset:   40,
		},
		{
			name:         "limit above max is clamped",
			query:        "limit=1000",
			defaultLimit: 10,
			maxLimit:     50,
			wantLimit:    50,
			wantOffset:   0,
		},
		{
			name:         "invalid limit falls back to default",
			query:        "limit=notanumber",
			defaultLimit: 10,
			maxLimit:     50,
			wantLimit:    10,
			wantOffset:   0,
		},
		{
			name:         "zero or negative limit falls back to default",
			query:        "limit=-5",
			defaultLimit: 10,
			maxLimit:     50,
			wantLimit:    10,
			wantOffset:   0,
		},
		{
			name:         "negative offset is ignored, stays 0",
			query:        "offset=-5",
			defaultLimit: 10,
			maxLimit:     50,
			wantLimit:    10,
			wantOffset:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestContext(tt.query)
			limit, offset := ParsePagination(c, tt.defaultLimit, tt.maxLimit)
			if limit != tt.wantLimit {
				t.Errorf("limit = %d, want %d", limit, tt.wantLimit)
			}
			if offset != tt.wantOffset {
				t.Errorf("offset = %d, want %d", offset, tt.wantOffset)
			}
		})
	}
}

func TestNumericFloat64RoundTrip(t *testing.T) {
	tests := []float64{0, 1, -1, 123.45, 0.1, 999999.999}

	for _, v := range tests {
		n, err := Float64ToNumeric(v)
		if err != nil {
			t.Fatalf("Float64ToNumeric(%v) error: %v", v, err)
		}
		got := NumericToFloat64(n)
		if got != v {
			t.Errorf("round trip for %v = %v, want %v", v, got, v)
		}
	}
}

func TestNumericToFloat64ZeroValue(t *testing.T) {
	var n pgtype.Numeric
	got := NumericToFloat64(n)
	if got != 0 {
		t.Errorf("NumericToFloat64(zero value) = %v, want 0", got)
	}
}
