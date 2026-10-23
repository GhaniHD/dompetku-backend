// Package budgetcycle menghitung periode budget yang dimulai pada tanggal
// gajian user, bukan selalu tanggal 1.
//
// Periode diberi nama sesuai bulan mulainya: dengan tanggal mulai 25, periode
// "Oktober 2026" adalah 25 Okt 2026 sampai 24 Nov 2026. Tanggal 29–31 pada bulan
// yang lebih pendek dipindah ke hari terakhir bulan itu.
package budgetcycle

import "time"

const (
	MinStartDay     = 1
	MaxStartDay     = 31
	DefaultStartDay = 1
)

func Normalize(startDay int) int {
	if startDay < MinStartDay || startDay > MaxStartDay {
		return DefaultStartDay
	}
	return startDay
}

func daysIn(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func startOf(startDay, year int, month time.Month) time.Time {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	day := Normalize(startDay)
	if last := daysIn(first.Year(), first.Month()); day > last {
		day = last
	}
	return time.Date(first.Year(), first.Month(), day, 0, 0, 0, 0, time.UTC)
}

// Period mengembalikan rentang [start, end) untuk periode bulan & tahun tersebut.
func Period(startDay, year, month int) (start, end time.Time) {
	start = startOf(startDay, year, time.Month(month))
	end = startOf(startDay, year, time.Month(month)+1)
	return start, end
}

// PeriodOf mengembalikan bulan & tahun periode yang memuat tanggal tersebut.
func PeriodOf(startDay int, date time.Time) (year, month int) {
	d := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	if d.Before(startOf(startDay, d.Year(), d.Month())) {
		prev := time.Date(d.Year(), d.Month()-1, 1, 0, 0, 0, 0, time.UTC)
		return prev.Year(), int(prev.Month())
	}
	return d.Year(), int(d.Month())
}
