package ratelimit

import (
	"testing"
	"time"
)

func TestLimiterWindow(t *testing.T) {
	now := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	l := New()
	l.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		if ok, _ := l.Allow("k", 3, time.Minute); !ok {
			t.Fatalf("hit %d ditolak, padahal limit 3", i+1)
		}
	}
	if ok, retry := l.Allow("k", 3, time.Minute); ok || retry != time.Minute {
		t.Fatalf("hit ke-4 harus ditolak dengan retry 1 menit, dapat ok=%v retry=%v", ok, retry)
	}
	if ok, _ := l.Allow("lain", 3, time.Minute); !ok {
		t.Fatal("key lain ikut tertahan")
	}

	now = now.Add(time.Minute)
	if ok, _ := l.Allow("k", 3, time.Minute); !ok {
		t.Fatal("jendela baru masih tertahan")
	}

	l.Reset("k")
	if blocked, _ := l.Blocked("k", 1, time.Minute); blocked {
		t.Fatal("key yang direset masih tertahan")
	}

	l.SetEnabled(false)
	for i := 0; i < 10; i++ {
		if ok, _ := l.Allow("x", 1, time.Minute); !ok {
			t.Fatal("limiter nonaktif tetap menahan")
		}
	}
}
