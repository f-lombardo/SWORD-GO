package web

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type loginRateLimiter struct {
	mu        sync.Mutex
	window    time.Duration
	limit     int
	entries   map[string][]time.Time
	blockedTo map[string]time.Time
}

func newLoginRateLimiter(limit int, window time.Duration) *loginRateLimiter {
	return &loginRateLimiter{
		window:    window,
		limit:     limit,
		entries:   make(map[string][]time.Time),
		blockedTo: make(map[string]time.Time),
	}
}

func (l *loginRateLimiter) Allow(clientID string, now time.Time) bool {
	now = now.UTC()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cleanup(clientID, now)

	blockedUntil, blocked := l.blockedTo[clientID]
	if blocked && blockedUntil.After(now) {
		return false
	}
	return true
}

func (l *loginRateLimiter) RegisterFailure(clientID string, now time.Time) {
	now = now.UTC()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cleanup(clientID, now)

	l.entries[clientID] = append(l.entries[clientID], now)
	if len(l.entries[clientID]) >= l.limit {
		l.blockedTo[clientID] = now.Add(l.window)
		l.entries[clientID] = nil
	}
}

func (l *loginRateLimiter) Reset(clientID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, clientID)
	delete(l.blockedTo, clientID)
}

func (l *loginRateLimiter) cleanup(clientID string, now time.Time) {
	cutoff := now.Add(-l.window)
	if attempts, ok := l.entries[clientID]; ok {
		filtered := attempts[:0]
		for _, attemptAt := range attempts {
			if attemptAt.After(cutoff) {
				filtered = append(filtered, attemptAt)
			}
		}
		if len(filtered) == 0 {
			delete(l.entries, clientID)
		} else {
			l.entries[clientID] = filtered
		}
	}
	if blockedUntil, ok := l.blockedTo[clientID]; ok && !blockedUntil.After(now) {
		delete(l.blockedTo, clientID)
	}
}

func clientIdentifier(r *http.Request) string {
	if forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwardedFor != "" {
		client := strings.TrimSpace(strings.Split(forwardedFor, ",")[0])
		if client != "" {
			return client
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	if trimmed := strings.TrimSpace(r.RemoteAddr); trimmed != "" {
		return trimmed
	}
	return "unknown"
}
