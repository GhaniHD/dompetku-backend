package integration_test

import (
	"net/http"
	"testing"
)

type incomeProfile struct {
	StartDay  int    `json:"budget_start_day"`
	Type      string `json:"income_type"`
	SetupDone bool   `json:"income_setup_done"`
}

func profileOf(t *testing.T, u testUser) incomeProfile {
	t.Helper()
	status, r := call(t, http.MethodGet, "/api/profile", u.Token, nil)
	expectStatus(t, status, http.StatusOK, r)
	return decode[incomeProfile](t, r)
}

func setIncome(t *testing.T, u testUser, body map[string]any) incomeProfile {
	t.Helper()
	status, r := call(t, http.MethodPut, "/api/profile/income", u.Token, body)
	expectStatus(t, status, http.StatusOK, r)
	return decode[incomeProfile](t, r)
}

func TestIncomeType(t *testing.T) {
	u := newUser(t)
	if p := profileOf(t, u); p.Type != "salary" || p.SetupDone {
		t.Fatalf("user baru %+v, ingin salary dan belum diatur", p)
	}

	if p := setIncome(t, u, map[string]any{"income_type": "salary", "start_day": 25}); p.Type != "salary" || p.StartDay != 25 || !p.SetupDone {
		t.Fatalf("setelah pilih gaji tgl 25: %+v", p)
	}

	// Tanpa gaji tetap: periode dipaksa bulan kalender walau start_day dikirim.
	if p := setIncome(t, u, map[string]any{"income_type": "irregular", "start_day": 25}); p.Type != "irregular" || p.StartDay != 1 {
		t.Fatalf("setelah pilih tidak tetap: %+v", p)
	}

	// Kembali ke gaji tanpa tanggal: tetap memakai tanggal yang tersimpan.
	if p := setIncome(t, u, map[string]any{"income_type": "salary"}); p.Type != "salary" || p.StartDay != 1 {
		t.Fatalf("kembali ke gaji: %+v", p)
	}

	for _, body := range []map[string]any{
		{"income_type": "freelance"},
		{"income_type": "salary", "start_day": 32},
		{},
	} {
		status, r := call(t, http.MethodPut, "/api/profile/income", u.Token, body)
		expectStatus(t, status, http.StatusBadRequest, r)
	}
}
