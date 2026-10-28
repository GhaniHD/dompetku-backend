package service

import (
	"context"
	"net/mail"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestCheckEmailAddress(t *testing.T) {
	cases := map[string]string{
		"budi@gmail.com":      "",
		"budi@mail.com":       "",
		"budi@kantor.co.id":   "",
		"budi@gmial.com":      "budi@gmail.com",
		"budi@gmai.com":       "budi@gmail.com",
		"budi@yahooo.com":     "budi@yahoo.com",
		"budi@outlok.com":     "budi@outlook.com",
		"budi@mailinator.com": "email sementara",
		"budi":                "format email",
		"budi@localhost":      "format email",
		"Budi <budi@x.com>":   "format email",
	}
	for email, want := range cases {
		err := CheckEmailAddress(context.Background(), email, false)
		switch {
		case want == "" && err != nil:
			t.Errorf("%s ditolak: %v", email, err)
		case want != "" && (err == nil || !strings.Contains(err.Error(), want)):
			t.Errorf("%s: ingin pesan memuat %q, dapat %v", email, want, err)
		}
	}
}

func TestFormatRupiah(t *testing.T) {
	for v, want := range map[float64]string{0: "Rp 0", 999: "Rp 999", 1000: "Rp 1.000", 1250000.4: "Rp 1.250.000", -50000: "−Rp 50.000"} {
		if got := formatRupiah(v); got != want {
			t.Errorf("formatRupiah(%v) = %q, ingin %q", v, got, want)
		}
	}
}

func TestBuildMessageRejectsHeaderInjection(t *testing.T) {
	from := &mail.Address{Name: "MyFulus", Address: "noreply@myfulus.my.id"}
	if _, err := buildMessage(from, "", Mail{To: "a@b.com\r\nBcc: x@y.com", Subject: "hai"}); err == nil {
		t.Fatal("header injection di alamat tujuan harus ditolak")
	}
	raw, err := buildMessage(from, "halo@myfulus.my.id", Mail{To: "a@b.com", Subject: "Kode 123456", HTML: "<p>x</p>", Text: "x", Unsubscribe: "https://api/x"})
	if err != nil {
		t.Fatal(err)
	}
	msg := string(raw)
	for _, want := range []string{"Reply-To: halo@myfulus.my.id", "List-Unsubscribe: <https://api/x>", "List-Unsubscribe-Post: List-Unsubscribe=One-Click", "multipart/alternative"} {
		if !strings.Contains(msg, want) {
			t.Errorf("pesan tidak memuat %q", want)
		}
	}
}

func TestWeeklySlotSpreadsUsers(t *testing.T) {
	count := map[int]int{}
	for i := 0; i < 700; i++ {
		count[int(weeklySlot(uuid.New()))]++
	}
	for d := 0; d < 7; d++ {
		if count[d] < 60 || count[d] > 140 {
			t.Fatalf("pembagian hari tidak merata: %v", count)
		}
	}
}

func TestRenderEmailEscapesName(t *testing.T) {
	r := renderEmail("subjek", emailView{Title: "Halo", Paragraphs: []string{"Halo <script>alert(1)</script>"}})
	if strings.Contains(r.HTML, "<script>") {
		t.Fatal("nama pengguna harus di-escape di HTML")
	}
	if !strings.Contains(r.Text, "<script>") {
		t.Fatal("versi teks menyimpan teks asli")
	}
}
