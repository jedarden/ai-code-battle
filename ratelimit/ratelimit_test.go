package ratelimit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBucketAllowsUpToMax(t *testing.T) {
	b := NewBucket(3, 1.0) // 3 tokens, 1/sec refill

	for i := 0; i < 3; i++ {
		if !b.Allow() {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if b.Allow() {
		t.Fatal("4th request should be denied")
	}
}

func TestBucketRefills(t *testing.T) {
	b := NewBucket(1, 100.0) // 1 token, 100/sec refill (fast refill)

	if !b.Allow() {
		t.Fatal("first request should be allowed")
	}
	if b.Allow() {
		t.Fatal("second request should be denied (bucket empty)")
	}

	// Wait for refill
	time.Sleep(20 * time.Millisecond)

	if !b.Allow() {
		t.Fatal("request after refill should be allowed")
	}
}

func TestBucketRetryAfter(t *testing.T) {
	b := NewBucket(1, 1.0) // 1 token, 1/sec refill
	b.Allow()              // drain

	retry := b.RetryAfter()
	if retry < 1 {
		t.Fatalf("RetryAfter = %d, want >= 1", retry)
	}
}

func TestLimiterCreatesBucketsOnDemand(t *testing.T) {
	l := NewLimiter(2, 1.0)

	_, ok1 := l.Allow("key-a")
	if !ok1 {
		t.Fatal("first request for key-a should be allowed")
	}
	_, ok2 := l.Allow("key-b")
	if !ok2 {
		t.Fatal("first request for key-b should be allowed")
	}

	// key-a has 1 token left
	_, ok3 := l.Allow("key-a")
	if !ok3 {
		t.Fatal("second request for key-a should be allowed")
	}
	_, ok4 := l.Allow("key-a")
	if ok4 {
		t.Fatal("third request for key-a should be denied")
	}

	// key-b still has 1 token
	_, ok5 := l.Allow("key-b")
	if !ok5 {
		t.Fatal("second request for key-b should be allowed (independent bucket)")
	}
}

func TestMiddlewareAllowsWhenUnderLimit(t *testing.T) {
	l := NewLimiter(2, 1.0)
	called := false
	handler := l.Middleware(func(r *http.Request) string { return "ip1" }, nil)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Fatal("handler should have been called")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestMiddlewareRejectsOnLimit(t *testing.T) {
	l := NewLimiter(1, 0.0001) // 1 token, extremely slow refill
	onLimitCalled := false
	mw := l.Middleware(func(r *http.Request) string { return "ip1" }, func() {
		onLimitCalled = true
	})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request passes
	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("first request: status = %d, want 200", w.Code)
	}

	// Second request is rate limited
	req2 := httptest.NewRequest("POST", "/test", nil)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: status = %d, want 429", w2.Code)
	}
	if !onLimitCalled {
		t.Fatal("onLimit callback should have been called")
	}
	if h := w2.Header().Get("Retry-After"); h == "" {
		t.Fatal("Retry-After header should be set")
	}

	var body map[string]string
	json.NewDecoder(w2.Body).Decode(&body)
	if body["error"] != "rate limit exceeded" {
		t.Fatalf("error body = %q, want 'rate limit exceeded'", body["error"])
	}
}

func TestMiddlewareKeysByIP(t *testing.T) {
	l := NewLimiter(1, 0.0001) // 1 token per key
	keyCount := 0
	mw := l.Middleware(func(r *http.Request) string {
		keyCount++
		return r.RemoteAddr
	}, nil)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First IP gets one request
	req1 := httptest.NewRequest("POST", "/test", nil)
	req1.RemoteAddr = "1.2.3.4:1234"
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	// Second IP also gets one request (different bucket)
	req2 := httptest.NewRequest("POST", "/test", nil)
	req2.RemoteAddr = "5.6.7.8:5678"
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w1.Code != http.StatusOK {
		t.Fatalf("first IP: status = %d, want 200", w1.Code)
	}
	if w2.Code != http.StatusOK {
		t.Fatalf("second IP: status = %d, want 200", w2.Code)
	}
}

func TestCleanupRemovesStaleBuckets(t *testing.T) {
	l := NewLimiter(1, 1.0)

	l.Allow("stale-key")
	l.Allow("fresh-key")

	if len(l.buckets) != 2 {
		t.Fatalf("expected 2 buckets, got %d", len(l.buckets))
	}

	// Manually age the stale bucket
	l.buckets["stale-key"].mu.Lock()
	l.buckets["stale-key"].lastTime = time.Now().Add(-2 * time.Hour)
	l.buckets["stale-key"].mu.Unlock()

	l.Cleanup(time.Hour)

	if len(l.buckets) != 1 {
		t.Fatalf("expected 1 bucket after cleanup, got %d", len(l.buckets))
	}
	if _, ok := l.buckets["stale-key"]; ok {
		t.Fatal("stale-key should have been cleaned up")
	}
	if _, ok := l.buckets["fresh-key"]; !ok {
		t.Fatal("fresh-key should still be present")
	}
}

func TestFloodRegisterEndpoint(t *testing.T) {
	// Simulates the verification requirement: flood test against /register
	// returns 429 after threshold.
	l := NewLimiter(5, 5.0/3600) // 5/hour per IP
	mw := l.Middleware(func(r *http.Request) string { return "1.2.3.4" }, nil)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	okCount := 0
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("POST", "/api/register", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code == http.StatusCreated {
			okCount++
		}
	}

	if okCount != 5 {
		t.Fatalf("expected 5 successful requests, got %d", okCount)
	}

	// 6th request should be 429
	req := httptest.NewRequest("POST", "/api/register", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("6th request: status = %d, want 429", w.Code)
	}
}

func TestLegitimateTrafficPasses(t *testing.T) {
	// Verifies that normal traffic patterns don't get rate limited.
	// 3 registrations from different IPs should all succeed.
	l := NewLimiter(5, 5.0/3600) // 5/hour per IP
	mw := l.Middleware(func(r *http.Request) string { return r.RemoteAddr }, nil)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	for i, ip := range []string{"1.1.1.1:1111", "2.2.2.2:2222", "3.3.3.3:3333"} {
		req := httptest.NewRequest("POST", "/api/register", nil)
		req.RemoteAddr = ip
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("request %d from %s: status = %d, want 201", i+1, ip, w.Code)
		}
	}
}
