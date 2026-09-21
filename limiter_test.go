package limiter

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRateLimiterAllowsFiveRequestsAndRejectsTheSixth(t *testing.T) {
	limiter := NewRateLimiter()
	req := Request{IP: "192.0.2.1"}
	for i := 0; i < maxRequests; i++ {
		if !limiter.Allow(req) { t.Fatalf("request %d was rejected", i+1) }
	}
	if limiter.Allow(req) { t.Fatal("sixth request was allowed") }
}

func TestRateLimiterTracksIPsIndependently(t *testing.T) {
	limiter := NewRateLimiter()
	for i := 0; i < maxRequests; i++ {
		if !limiter.Allow(Request{IP: "192.0.2.1"}) || !limiter.Allow(Request{IP: "192.0.2.2"}) { t.Fatalf("request %d was rejected", i+1) }
	}
	if limiter.Allow(Request{IP: "192.0.2.1"}) || limiter.Allow(Request{IP: "192.0.2.2"}) { t.Fatal("request beyond either IP's limit was allowed") }
}

func TestRateLimiterResetsAfterOneSecond(t *testing.T) {
	limiter := NewRateLimiter()
	current := time.Unix(0, 0)
	limiter.now = func() time.Time { return current }
	req := Request{IP: "192.0.2.1"}
	for i := 0; i < maxRequests; i++ { if !limiter.Allow(req) { t.Fatalf("request %d was rejected", i+1) } }
	current = current.Add(window)
	if !limiter.Allow(req) { t.Fatal("request was rejected after the window reset") }
}

func TestRateLimiterLoadtestBypassesAndDoesNotConsumeQuota(t *testing.T) {
	limiter := NewRateLimiter()
	req := Request{IP: "192.0.2.1"}
	loadtestReq := Request{IP: req.IP, IsLoadtest: true}
	for i := 0; i < 100; i++ { if !limiter.Allow(loadtestReq) { t.Fatalf("load-test request %d was rejected", i+1) } }
	for i := 0; i < maxRequests; i++ { if !limiter.Allow(req) { t.Fatalf("normal request %d was rejected after load-test bypasses", i+1) } }
	if limiter.Allow(req) { t.Fatal("normal request beyond the limit was allowed") }
}

func TestRateLimiterLoadtestBypassesRegardlessOfIP(t *testing.T) {
	limiter := NewRateLimiter()
	for i := 0; i < maxRequests; i++ { if !limiter.Allow(Request{IP: "192.0.2.1"}) { t.Fatalf("setup request %d was rejected", i+1) } }
	if !limiter.Allow(Request{IP: "192.0.2.1", IsLoadtest: true}) { t.Fatal("load-test request was rejected for a limited IP") }
	if !limiter.Allow(Request{IsLoadtest: true}) { t.Fatal("load-test request with an empty IP was rejected") }
}

func TestRateLimiterIsSafeForConcurrentCalls(t *testing.T) {
	limiter := NewRateLimiter()
	var allowed atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); if limiter.Allow(Request{IP: "192.0.2.1"}) { allowed.Add(1) } }()
	}
	wg.Wait()
	if got := allowed.Load(); got != maxRequests { t.Fatalf("allowed %d concurrent normal requests, want %d", got, maxRequests) }
}
