// Package ratelimit provides token-bucket rate limiting for HTTP handlers.
package ratelimit

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Bucket is a token-bucket rate limiter for a single key.
type Bucket struct {
	mu       sync.Mutex
	tokens   float64
	max      float64
	refill   float64 // tokens added per second
	lastTime time.Time
}

// NewBucket creates a bucket that holds max tokens and refills at the given
// rate (tokens per second). The bucket starts full.
func NewBucket(max, refillPerSec float64) *Bucket {
	return &Bucket{
		tokens:   max,
		max:      max,
		refill:   refillPerSec,
		lastTime: time.Now(),
	}
}

// Allow consumes one token. Returns true if a token was available.
func (b *Bucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastTime).Seconds()
	b.lastTime = now
	b.tokens += elapsed * b.refill
	if b.tokens > b.max {
		b.tokens = b.max
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// RetryAfter returns the number of seconds until the next token is available.
// Call after Allow() returns false.
func (b *Bucket) RetryAfter() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	deficit := 1.0 - b.tokens
	if deficit <= 0 {
		return 1
	}
	secs := deficit / b.refill
	if secs < 1 {
		return 1
	}
	return int(secs)
}

// Limiter holds a collection of buckets keyed by string (e.g. "ip:endpoint").
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*Bucket
	max     float64
	refill  float64
}

// NewLimiter creates a Limiter where each key gets max tokens, refilling at
// refillPerSec tokens per second.
func NewLimiter(max, refillPerSec float64) *Limiter {
	return &Limiter{
		buckets: make(map[string]*Bucket),
		max:     max,
		refill:  refillPerSec,
	}
}

// Allow checks the bucket for the given key. Creates one if needed.
func (l *Limiter) Allow(key string) (*Bucket, bool) {
	l.mu.Lock()
	b, ok := l.buckets[key]
	if !ok {
		b = NewBucket(l.max, l.refill)
		l.buckets[key] = b
	}
	l.mu.Unlock()

	return b, b.Allow()
}

// Cleanup removes buckets that haven't been used in the given duration.
// Call periodically to prevent unbounded memory growth.
func (l *Limiter) Cleanup(maxAge time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for k, b := range l.buckets {
		b.mu.Lock()
		if b.lastTime.Before(cutoff) {
			delete(l.buckets, k)
		}
		b.mu.Unlock()
	}
}

// Middleware returns an http.Handler that applies per-key rate limiting.
// On limit breach it responds with HTTP 429 and a Retry-After header.
// onLimit is called (if non-nil) when a request is rate-limited, for metrics.
func (l *Limiter) Middleware(keyFunc func(*http.Request) string, onLimit func()) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)
			bucket, ok := l.Allow(key)
			if !ok {
				if onLimit != nil {
					onLimit()
				}
				retry := bucket.RetryAfter()
				w.Header().Set("Retry-After", fmt.Sprintf("%.0f", time.Duration(retry).Seconds()))
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
