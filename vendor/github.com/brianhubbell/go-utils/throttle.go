package goutils

import (
	"sync"
	"time"
)

// Throttle limits execution of a function to at most once per rate duration per key.
type Throttle struct {
	fn          func(key string)
	rate        time.Duration
	mu          sync.Mutex
	execHistory map[string]time.Time
}

// NewThrottle creates a Throttle that calls fn at most once per rate duration per key.
func NewThrottle(fn func(key string), rate time.Duration) *Throttle {
	return &Throttle{
		fn:          fn,
		rate:        rate,
		execHistory: make(map[string]time.Time),
	}
}

// Exec attempts to execute the throttled function for the given key.
// Returns true if the function was called, false if it was throttled.
func (t *Throttle) Exec(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if last, ok := t.execHistory[key]; ok {
		if time.Since(last) < t.rate {
			return false
		}
	}

	t.fn(key)
	t.execHistory[key] = time.Now()
	return true
}
