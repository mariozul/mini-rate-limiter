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

type RateLimiter struct {
	mu      sync.Mutex
	clients map[string]clientWindow
	now     func() time.Time
}

type clientWindow struct {
	started time.Time
	count   int
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{clients: make(map[string]clientWindow), now: time.Now}
}

func (l *RateLimiter) Allow(req Request) bool {
	if req.IsLoadtest {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

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
		return false
	}

	client.count++
	l.clients[req.IP] = client
	return true
}
