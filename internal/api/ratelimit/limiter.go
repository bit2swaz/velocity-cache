package ratelimit

import (
	"sync"
	"time"
)

// Limiter enforces a fixed-window request limit per identifier (e.g., IP address).
type Limiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	store  map[string]*entry
}

type entry struct {
	count int
	reset time.Time
}

// New creates a new Limiter.
func New(limit int, window time.Duration) *Limiter {
	if limit <= 0 {
		limit = 1
	}
	if window <= 0 {
		window = time.Hour
	}

	return &Limiter{
		limit:  limit,
		window: window,
		store:  make(map[string]*entry),
	}
}

// Allow reports whether the identifier is permitted to proceed. It returns the
// remaining duration until the counter resets.
func (l *Limiter) Allow(id string) (bool, time.Duration) {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.store[id]
	if !ok || now.After(e.reset) {
		l.store[id] = &entry{count: 1, reset: now.Add(l.window)}
		return true, l.window
	}

	if e.count >= l.limit {
		return false, time.Until(e.reset)
	}

	e.count++
	return true, time.Until(e.reset)
}

// Cleanup removes expired entries from the limiter to avoid unbounded growth.
func (l *Limiter) Cleanup() {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	for id, e := range l.store {
		if now.After(e.reset) {
			delete(l.store, id)
		}
	}
}
