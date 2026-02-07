package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	rate     int
	burst    int
	stopChan chan struct{}
	wg       sync.WaitGroup
}

type visitor struct {
	tokens     int
	lastRefill time.Time
}

func NewRateLimiter(rate, burst int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		burst:    burst,
		stopChan: make(chan struct{}),
	}

	rl.wg.Add(1)
	go rl.cleanup()

	return rl
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getIP(r)

		if !rl.allow(ip) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[key]
	if !exists {
		if rl.burst <= 0 {
			rl.visitors[key] = &visitor{
				tokens:     0,
				lastRefill: time.Now(),
			}
			return false
		}
		rl.visitors[key] = &visitor{
			tokens:     rl.burst - 1,
			lastRefill: time.Now(),
		}
		return true
	}

	now := time.Now()
	elapsed := now.Sub(v.lastRefill)
	refill := int(elapsed.Seconds()) * rl.rate

	v.tokens += refill
	maxTokens := rl.burst
	if rl.rate > rl.burst {
		maxTokens = rl.rate
	}
	if v.tokens > maxTokens {
		v.tokens = maxTokens
	}
	v.lastRefill = now

	if v.tokens > 0 {
		v.tokens--
		return true
	}

	return false
}

func (rl *RateLimiter) cleanup() {
	defer rl.wg.Done()
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			for key, v := range rl.visitors {
				if time.Since(v.lastRefill) > 10*time.Minute {
					delete(rl.visitors, key)
				}
			}
			rl.mu.Unlock()
		case <-rl.stopChan:
			return
		}
	}
}

func (rl *RateLimiter) Stop() {
	close(rl.stopChan)
	rl.wg.Wait()
}

func getIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		return forwarded
	}

	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	return r.RemoteAddr
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func WriteJSONError(w http.ResponseWriter, code int, errCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	resp := ErrorResponse{
		Error:   http.StatusText(code),
		Code:    errCode,
		Message: message,
	}

	json.NewEncoder(w).Encode(resp)
}

func PaginationParams(r *http.Request) (limit, offset int) {
	limit = 20
	offset = 0

	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	if raw := r.URL.Query().Get("offset"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = v
		}
	}

	return limit, offset
}
