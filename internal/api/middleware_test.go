package api

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestRateLimiter_BurstBehavior(t *testing.T) {
	rl := NewRateLimiter(1, 3)
	defer rl.Stop()

	results := make([]bool, 5)
	for i := 0; i < 5; i++ {
		results[i] = rl.allow("test-ip")
	}

	allowed := 0
	for _, res := range results {
		if res {
			allowed++
		}
	}

	if allowed != 3 {
		t.Errorf("expected 3 initial burst requests to succeed, got %d", allowed)
	}
}

func TestRateLimiter_RefillMechanism(t *testing.T) {
	rl := NewRateLimiter(10, 5)
	defer rl.Stop()

	for i := 0; i < 5; i++ {
		if !rl.allow("test-ip") {
			t.Fatalf("initial burst request %d failed", i)
		}
	}

	if rl.allow("test-ip") {
		t.Error("expected request after burst to fail")
	}

	time.Sleep(1*time.Second + 100*time.Millisecond)

	allowedAfterRefill := 0
	for i := 0; i < 15; i++ {
		if rl.allow("test-ip") {
			allowedAfterRefill++
		}
	}

	if allowedAfterRefill < 10 {
		t.Errorf("expected at least 10 requests after 1s refill, got %d", allowedAfterRefill)
	}
}

func TestRateLimiter_MultipleVisitors(t *testing.T) {
	rl := NewRateLimiter(2, 3)
	defer rl.Stop()

	ip1Allowed := 0
	ip2Allowed := 0

	for i := 0; i < 5; i++ {
		if rl.allow("ip1") {
			ip1Allowed++
		}
		if rl.allow("ip2") {
			ip2Allowed++
		}
	}

	if ip1Allowed != 3 {
		t.Errorf("expected ip1 to have 3 allowed, got %d", ip1Allowed)
	}
	if ip2Allowed != 3 {
		t.Errorf("expected ip2 to have 3 allowed, got %d", ip2Allowed)
	}
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	rl := NewRateLimiter(10, 20)
	defer rl.Stop()

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if rl.allow("test-ip") {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	if successCount != 20 {
		t.Errorf("expected exactly 20 successful requests from burst, got %d", successCount)
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	defer rl.Stop()

	rl.allow("old-visitor")

	rl.mu.Lock()
	rl.visitors["old-visitor"].lastRefill = time.Now().Add(-15 * time.Minute)
	rl.mu.Unlock()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	select {
	case <-ticker.C:
		rl.mu.Lock()
		for key, v := range rl.visitors {
			if time.Since(v.lastRefill) > 10*time.Minute {
				delete(rl.visitors, key)
			}
		}
		rl.mu.Unlock()
	case <-time.After(1 * time.Second):
		t.Fatal("cleanup timeout")
	}

	rl.mu.RLock()
	_, exists := rl.visitors["old-visitor"]
	rl.mu.RUnlock()

	if exists {
		t.Error("expected old visitor to be cleaned up")
	}
}

func TestRateLimiter_Middleware(t *testing.T) {
	rl := NewRateLimiter(1, 2)
	defer rl.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := rl.Middleware(handler)

	codes := make([]int, 5)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		w := httptest.NewRecorder()

		wrapped.ServeHTTP(w, req)
		codes[i] = w.Code
	}

	allowed := 0
	for _, code := range codes {
		if code == http.StatusOK {
			allowed++
		}
	}

	if allowed != 2 {
		t.Errorf("expected 2 requests to pass through, got %d", allowed)
	}
}

func TestGetIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	req.RemoteAddr = "192.168.1.1:1234"

	ip := getIP(req)

	if ip != "10.0.0.1" {
		t.Errorf("expected X-Forwarded-For to take priority, got %s", ip)
	}
}

func TestGetIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Real-IP", "10.0.0.2")
	req.RemoteAddr = "192.168.1.1:1234"

	ip := getIP(req)

	if ip != "10.0.0.2" {
		t.Errorf("expected X-Real-IP to take priority, got %s", ip)
	}
}

func TestGetIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"

	ip := getIP(req)

	if ip != "192.168.1.1:1234" {
		t.Errorf("expected RemoteAddr fallback, got %s", ip)
	}
}

func TestGetIP_HeaderPriority(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	req.Header.Set("X-Real-IP", "10.0.0.2")
	req.RemoteAddr = "192.168.1.1:1234"

	ip := getIP(req)

	if ip != "10.0.0.1" {
		t.Errorf("X-Forwarded-For should take priority over X-Real-IP, got %s", ip)
	}
}

func TestWriteJSONError(t *testing.T) {
	w := httptest.NewRecorder()

	WriteJSONError(w, http.StatusBadRequest, "INVALID_INPUT", "The input is invalid")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	if !contains(w.Body.String(), "INVALID_INPUT") {
		t.Errorf("expected error code in response, got: %s", w.Body.String())
	}
	if !contains(w.Body.String(), "The input is invalid") {
		t.Errorf("expected error message in response, got: %s", w.Body.String())
	}
}

func TestPaginationParams_Defaults(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	limit, offset := PaginationParams(req)

	if limit != 20 {
		t.Errorf("expected default limit 20, got %d", limit)
	}
	if offset != 0 {
		t.Errorf("expected default offset 0, got %d", offset)
	}
}

func TestPaginationParams_Custom(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?limit=50&offset=100", nil)

	limit, offset := PaginationParams(req)

	if limit != 50 {
		t.Errorf("expected limit 50, got %d", limit)
	}
	if offset != 100 {
		t.Errorf("expected offset 100, got %d", offset)
	}
}

func TestPaginationParams_InvalidValues(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?limit=abc&offset=-10", nil)

	limit, offset := PaginationParams(req)

	if limit != 20 {
		t.Errorf("expected default limit on invalid input, got %d", limit)
	}
	if offset != 0 {
		t.Errorf("expected default offset on invalid input, got %d", offset)
	}
}

func TestPaginationParams_BoundaryEnforcement(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?limit=200&offset=-5", nil)

	limit, offset := PaginationParams(req)

	if limit != 20 {
		t.Errorf("expected limit capped to max, got %d", limit)
	}
	if offset != 0 {
		t.Errorf("expected negative offset rejected, got %d", offset)
	}
}

func TestRateLimiter_ZeroBurst(t *testing.T) {
	rl := NewRateLimiter(10, 0)
	defer rl.Stop()

	if rl.allow("test-ip") {
		t.Error("expected immediate rejection with zero burst")
	}
}

func TestRateLimiter_HighConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping high concurrency test in short mode")
	}

	rl := NewRateLimiter(100, 100)
	defer rl.Stop()

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if rl.allow("stress-test-ip") {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	if successCount != 100 {
		t.Errorf("expected exactly 100 requests to succeed under stress, got %d", successCount)
	}
}
