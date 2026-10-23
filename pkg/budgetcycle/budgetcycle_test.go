package budgetcycle

import (
	"testing"
	"time"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestPeriod(t *testing.T) {
	cases := []struct {
		name               string
		startDay, y, m     int
		wantStart, wantEnd time.Time
	}{
		{"tanggal 1 sama dengan bulan kalender", 1, 2026, 10, date(2026, 10, 1), date(2026, 11, 1)},
		{"gajian tanggal 25", 25, 2026, 10, date(2026, 10, 25), date(2026, 11, 25)},
		{"melewati tahun baru", 25, 2026, 12, date(2026, 12, 25), date(2027, 1, 25)},
		{"tanggal 31 di bulan 30 hari", 31, 2026, 4, date(2026, 4, 30), date(2026, 5, 31)},
		{"tanggal 31 menjelang Februari", 31, 2026, 1, date(2026, 1, 31), date(2026, 2, 28)},
		{"tanggal 30 di Februari kabisat", 30, 2028, 2, date(2028, 2, 29), date(2028, 3, 30)},
		{"nilai tidak valid jadi tanggal 1", 0, 2026, 10, date(2026, 10, 1), date(2026, 11, 1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end := Period(tc.startDay, tc.y, tc.m)
			if !start.Equal(tc.wantStart) || !end.Equal(tc.wantEnd) {
				t.Fatalf("periode %s – %s, ingin %s – %s", start.Format("2006-01-02"), end.Format("2006-01-02"),
					tc.wantStart.Format("2006-01-02"), tc.wantEnd.Format("2006-01-02"))
			}
		})
	}
}

func TestPeriodOf(t *testing.T) {
	cases := []struct {
		name     string
		startDay int
		date     time.Time
		wantY    int
		wantM    int
	}{
		{"tepat tanggal gajian", 25, date(2026, 10, 25), 2026, 10},
		{"sehari sebelum gajian masuk periode lalu", 25, date(2026, 10, 24), 2026, 9},
		{"awal bulan berikutnya masih periode lalu", 25, date(2026, 11, 3), 2026, 10},
		{"awal Januari masih periode Desember", 25, date(2027, 1, 10), 2026, 12},
		{"tanggal 31 di Februari", 31, date(2026, 2, 28), 2026, 2},
		{"27 Februari sebelum gajian akhir bulan", 31, date(2026, 2, 27), 2026, 1},
		{"tanggal 1 = bulan kalender", 1, date(2026, 10, 1), 2026, 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			y, m := PeriodOf(tc.startDay, tc.date)
			if y != tc.wantY || m != tc.wantM {
				t.Fatalf("periode %d/%d, ingin %d/%d", m, y, tc.wantM, tc.wantY)
			}
			start, end := Period(tc.startDay, y, m)
			if tc.date.Before(start) || !tc.date.Before(end) {
				t.Fatalf("tanggal %s di luar periode %s – %s", tc.date.Format("2006-01-02"),
					start.Format("2006-01-02"), end.Format("2006-01-02"))
			}
		})
	}
}

func TestEveryDayBelongsToExactlyOnePeriod(t *testing.T) {
	for _, startDay := range []int{1, 15, 25, 28, 29, 30, 31} {
		for d := date(2026, 1, 1); d.Before(date(2029, 1, 1)); d = d.AddDate(0, 0, 1) {
			y, m := PeriodOf(startDay, d)
			start, end := Period(startDay, y, m)
			if d.Before(start) || !d.Before(end) {
				t.Fatalf("startDay %d: %s tidak berada di periode %d/%d (%s – %s)", startDay,
					d.Format("2006-01-02"), m, y, start.Format("2006-01-02"), end.Format("2006-01-02"))
			}
		}
	}
}
