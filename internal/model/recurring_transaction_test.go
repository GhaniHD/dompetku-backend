package model

import (
	"testing"
	"time"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func TestRecurringNextAfter(t *testing.T) {
	monthly31 := &RecurringTransaction{Frequency: FrequencyMonthly, AnchorDay: 31}
	want := []time.Time{d(2026, 2, 28), d(2026, 3, 31), d(2026, 4, 30), d(2026, 5, 31)}
	current := d(2026, 1, 31)
	for _, w := range want {
		current = monthly31.NextAfter(current)
		if !current.Equal(w) {
			t.Fatalf("bulanan tgl 31: dapat %s, ingin %s", current.Format("2006-01-02"), w.Format("2006-01-02"))
		}
	}

	monthly25 := &RecurringTransaction{Frequency: FrequencyMonthly, AnchorDay: 25}
	if got := monthly25.NextAfter(d(2026, 12, 25)); !got.Equal(d(2027, 1, 25)) {
		t.Fatalf("bulanan melewati tahun: %s", got.Format("2006-01-02"))
	}

	weekly := &RecurringTransaction{Frequency: FrequencyWeekly, AnchorDay: 3}
	if got := weekly.NextAfter(d(2026, 12, 29)); !got.Equal(d(2027, 1, 5)) {
		t.Fatalf("mingguan: %s", got.Format("2006-01-02"))
	}
}

func TestRecurringFinished(t *testing.T) {
	end := d(2026, 12, 25)
	r := &RecurringTransaction{EndDate: &end}
	if r.Finished(d(2026, 12, 25)) {
		t.Fatal("tanggal akhir masih termasuk jadwal")
	}
	if !r.Finished(d(2026, 12, 26)) {
		t.Fatal("setelah tanggal akhir harus selesai")
	}
	if (&RecurringTransaction{}).Finished(d(2099, 1, 1)) {
		t.Fatal("tanpa tanggal akhir tidak pernah selesai")
	}
}
