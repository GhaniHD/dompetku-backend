// Package ratelimit menyediakan pembatas request fixed-window di memori.
// Cukup untuk satu instance server; data hilang saat restart.
package ratelimit

import (
	"sync"
	"sync/atomic"
	"time"
)

type window struct {
	start time.Time
	per   time.Duration
	count int
}

type Limiter struct {
	mu      sync.Mutex
	hits    map[string]*window
	calls   int
	enabled atomic.Bool
	now     func() time.Time
}

func New() *Limiter {
	l := &Limiter{hits: map[string]*window{}, now: time.Now}
	l.enabled.Store(true)
	return l
}

// SetEnabled dipakai integration test agar ratusan request dari 127.0.0.1 tidak tertahan.
func (l *Limiter) SetEnabled(on bool) { l.enabled.Store(on) }

func (l *Limiter) current(key string, per time.Duration) *window {
	now := l.now()
	w, ok := l.hits[key]
	if !ok || now.Sub(w.start) >= w.per {
		w = &window{start: now, per: per}
		l.hits[key] = w
	}
	return w
}

func (l *Limiter) sweep() {
	l.calls++
	if l.calls%1000 != 0 {
		return
	}
	now := l.now()
	for k, w := range l.hits {
		if now.Sub(w.start) >= w.per {
			delete(l.hits, k)
		}
	}
}

// Allow mencatat satu hit dan melaporkan apakah key masih di bawah limit.
// retry adalah sisa waktu sampai jendela direset.
func (l *Limiter) Allow(key string, limit int, per time.Duration) (bool, time.Duration) {
	if !l.enabled.Load() {
		return true, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep()
	w := l.current(key, per)
	w.count++
	return w.count <= limit, w.per - l.now().Sub(w.start)
}

// Blocked memeriksa tanpa menambah hit.
func (l *Limiter) Blocked(key string, limit int, per time.Duration) (bool, time.Duration) {
	if !l.enabled.Load() {
		return false, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	w := l.current(key, per)
	return w.count >= limit, w.per - l.now().Sub(w.start)
}

func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	delete(l.hits, key)
	l.mu.Unlock()
}
