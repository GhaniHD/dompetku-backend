package utils

import "time"

var WIB = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Jakarta"); err == nil {
		return loc
	}
	return time.FixedZone("WIB", 7*60*60)
}()

// CalendarDate mengubah waktu dari aplikasi (mis. 2026-10-21T17:00:00Z, yaitu
// 22 Okt 00:00 WIB) menjadi tanggal kalender WIB pada tengah malam UTC.
func CalendarDate(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	w := t.In(WIB)
	d := time.Date(w.Year(), w.Month(), w.Day(), 0, 0, 0, 0, time.UTC)
	return &d
}

// DateOnly memformat tanggal untuk dibandingkan dengan kolom bertipe date.
func DateOnly(t time.Time) string {
	return t.Format("2006-01-02")
}
