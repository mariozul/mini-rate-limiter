package limiter

import (
	"sync"
	"time"
)

const (
	maxRequests = 5
	window      = time.Second
)

type Request struct {
	IP         string
	IsLoadtest bool
}

type RateLimiterStats struct {
	TotalRequests int64
	TotalBlocked  int64
	TotalBypassed int64
}

type RateLimiter struct {
	mu      sync.Mutex
	clients map[string]clientWindow
	now     func() time.Time
	stats   RateLimiterStats
}

type clientWindow struct {
	started time.Time
	count   int
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{clients: make(map[string]clientWindow), now: time.Now}
}

func (l *RateLimiter) Allow(req Request) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.stats.TotalRequests++
	if req.IsLoadtest {
		l.stats.TotalBypassed++
		return true
	}

	if l.clients == nil {
		l.clients = make(map[string]clientWindow)
	}
	if l.now == nil {
		l.now = time.Now
	}

	now := l.now()
	client, ok := l.clients[req.IP]
	if !ok || now.Sub(client.started) >= window {
		l.clients[req.IP] = clientWindow{started: now, count: 1}
		return true
	}
	if client.count >= maxRequests {
		l.stats.TotalBlocked++
		return false
	}

	client.count++
	l.clients[req.IP] = client
	return true
}

func (l *RateLimiter) Stats() RateLimiterStats {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.stats
}
