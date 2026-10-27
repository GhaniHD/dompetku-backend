package utils

import (
	"testing"
	"time"
)

func TestCalendarDate(t *testing.T) {
	for in, want := range map[string]string{
		"2026-10-21T17:00:00Z":      "2026-10-22",
		"2026-11-21T16:59:59.999Z":  "2026-11-21",
		"2026-10-22T00:00:00+07:00": "2026-10-22",
		"2026-10-22T12:00:00+07:00": "2026-10-22",
	} {
		v, _ := time.Parse(time.RFC3339Nano, in)
		if got := DateOnly(*CalendarDate(&v)); got != want {
			t.Errorf("%s → %s, ingin %s", in, got, want)
		}
	}
	if CalendarDate(nil) != nil {
		t.Error("nil harus tetap nil")
	}
}
