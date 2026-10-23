package integration_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
)

// fakeClaudeServer meniru endpoint Messages API dan sengaja mengembalikan nilai
// yang tidak valid (bulan 13, risk_level asing, angka minus) untuk menguji
// validasi di sisi server.
type fakeClaudeServer struct {
	*httptest.Server
	mu         sync.Mutex
	calls      int
	lastPrompt string
}

const fakeClaudeAnswer = "```json\n" + `{
  "summary": "Keuangan cukup stabil.",
  "highlights": ["Pengeluaran makan dominan"],
  "recommendations": ["Kurangi jajan"],
  "risk_level": "sangat-bahaya",
  "spending_patterns": [{"category": "Makan", "avg_monthly": 100000, "trend": "meroket", "trend_pct": 10}],
  "prediction": [
    {"month": 13, "label": "Salah", "predicted_income": 5000000, "predicted_expense": -100, "predicted_net": 0, "confidence": "pasti"},
    {"month": 14, "label": "Salah", "predicted_income": 5000000, "predicted_expense": 3000000, "predicted_net": 0, "confidence": "high"}
  ]
}` + "\n```"

func newFakeClaudeServer() *fakeClaudeServer {
	f := &fakeClaudeServer{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)

		f.mu.Lock()
		f.calls++
		if len(body.Messages) > 0 {
			f.lastPrompt = body.Messages[0].Content
		}
		f.mu.Unlock()

		if r.Header.Get("x-api-key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": "invalid x-api-key"}})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]string{{"type": "text", "text": fakeClaudeAnswer}},
		})
	}))
	return f
}

func (f *fakeClaudeServer) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeClaudeServer) LastPrompt() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastPrompt
}
