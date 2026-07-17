package shift

import (
	db "laundry-app-with-golang/internal/db/generated"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestCivilDateStart(t *testing.T) {
	tests := []struct {
		name string
		in   time.Time
		want time.Time
	}{
		{
			name: "already midnight WIB",
			in:   time.Date(2026, 3, 5, 0, 0, 0, 0, JakartaLocation),
			want: time.Date(2026, 3, 5, 0, 0, 0, 0, JakartaLocation),
		},
		{
			name: "early morning WIB stays on the same civil date",
			in:   time.Date(2026, 3, 5, 2, 30, 0, 0, JakartaLocation),
			want: time.Date(2026, 3, 5, 0, 0, 0, 0, JakartaLocation),
		},
		{
			name: "late evening WIB stays on the same civil date",
			in:   time.Date(2026, 3, 5, 23, 59, 59, 0, JakartaLocation),
			want: time.Date(2026, 3, 5, 0, 0, 0, 0, JakartaLocation),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CivilDateStart(tt.in)
			if !got.Equal(tt.want) || got.Location().String() != tt.want.Location().String() {
				t.Errorf("CivilDateStart(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func microsFromHHMM(h, m int) int64 {
	return int64((h*3600 + m*60) * 1_000_000)
}

func TestResolveShiftWindow(t *testing.T) {
	targetDate := time.Date(2026, 3, 5, 12, 0, 0, 0, JakartaLocation)

	t.Run("normal same-day shift", func(t *testing.T) {
		ws := db.WorkShift{
			StartTime: pgtype.Time{Microseconds: microsFromHHMM(8, 0), Valid: true},
			EndTime:   pgtype.Time{Microseconds: microsFromHHMM(16, 0), Valid: true},
		}

		start, end := ResolveShiftWindow(ws, targetDate)

		wantStart := time.Date(2026, 3, 5, 8, 0, 0, 0, JakartaLocation)
		wantEnd := time.Date(2026, 3, 5, 16, 0, 0, 0, JakartaLocation)
		if !start.Equal(wantStart) {
			t.Errorf("start = %v, want %v", start, wantStart)
		}
		if !end.Equal(wantEnd) {
			t.Errorf("end = %v, want %v", end, wantEnd)
		}
	})

	t.Run("overnight shift crossing midnight", func(t *testing.T) {
		ws := db.WorkShift{
			StartTime: pgtype.Time{Microseconds: microsFromHHMM(22, 0), Valid: true},
			EndTime:   pgtype.Time{Microseconds: microsFromHHMM(6, 0), Valid: true},
		}

		start, end := ResolveShiftWindow(ws, targetDate)

		wantStart := time.Date(2026, 3, 5, 22, 0, 0, 0, JakartaLocation)
		wantEnd := time.Date(2026, 3, 6, 6, 0, 0, 0, JakartaLocation)
		if !start.Equal(wantStart) {
			t.Errorf("start = %v, want %v", start, wantStart)
		}
		if !end.Equal(wantEnd) {
			t.Errorf("end = %v, want %v (should be pushed 24h forward)", end, wantEnd)
		}
		if !end.After(start) {
			t.Errorf("end (%v) should be after start (%v) for an overnight shift", end, start)
		}
	})
}

func TestJakartaLocationLoaded(t *testing.T) {
	if JakartaLocation == nil {
		t.Fatal("JakartaLocation should never be nil")
	}
	// Happy-path only: forcing time.LoadLocation to fail in-process to
	// exercise the UTC fallback isn't practical without refactoring init()
	// to accept an injectable loader.
	if JakartaLocation.String() != "Asia/Jakarta" && JakartaLocation.String() != "UTC" {
		t.Errorf("unexpected JakartaLocation: %v", JakartaLocation)
	}
}
