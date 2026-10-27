package integration_test

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestResponsesAreGzipped(t *testing.T) {
	u := newUser(t)
	createWallet(t, u, "Tunai", 50_000)

	get := func(path string) *http.Response {
		req, _ := http.NewRequest(http.MethodGet, server.URL+path, nil)
		req.Header.Set("Authorization", "Bearer "+u.Token)
		req.Header.Set("Accept-Encoding", "gzip")
		res, err := http.DefaultTransport.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	res := get("/api/wallets")
	defer res.Body.Close()
	if res.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("respons JSON tidak dikompres: %v", res.Header)
	}
	zr, err := gzip.NewReader(res.Body)
	if err != nil {
		t.Fatalf("body bukan gzip valid: %v", err)
	}
	raw, _ := io.ReadAll(zr)
	var out struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Data) != 1 || out.Data[0].Name != "Tunai" {
		t.Fatalf("isi setelah dekompres %s (%v)", raw, err)
	}

	xlsx := get("/api/export/transactions.xlsx")
	defer xlsx.Body.Close()
	if xlsx.StatusCode != http.StatusOK || xlsx.Header.Get("Content-Encoding") != "" {
		t.Fatalf("file Excel tidak boleh dikompres ulang: %d %v", xlsx.StatusCode, xlsx.Header)
	}

	unauth, _ := http.NewRequest(http.MethodGet, server.URL+"/api/wallets", nil)
	unauth.Header.Set("Accept-Encoding", "gzip")
	r2, err := http.DefaultClient.Do(unauth)
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Body.Close()
	body, _ := io.ReadAll(r2.Body)
	if r2.StatusCode != http.StatusUnauthorized || len(body) == 0 {
		t.Fatalf("respons error tanpa token %d %q", r2.StatusCode, body)
	}
}
