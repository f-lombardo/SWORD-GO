package web

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

type callbackReplayGuard struct {
	mu      sync.Mutex
	seen    map[string]time.Time
	ttl     time.Duration
	maxSkew time.Duration
}

func newCallbackReplayGuard(ttl time.Duration) *callbackReplayGuard {
	return &callbackReplayGuard{
		seen:    make(map[string]time.Time),
		ttl:     ttl,
		maxSkew: 2 * time.Minute,
	}
}

func (g *callbackReplayGuard) Validate(scope string, id int64, timestamp string, nonce string, now time.Time) error {
	trimmedNonce := strings.TrimSpace(nonce)
	if trimmedNonce == "" || len(trimmedNonce) > 256 || strings.ContainsAny(trimmedNonce, "\r\n\t ") {
		return fmt.Errorf("invalid nonce")
	}

	parsedTS, err := strconv.ParseInt(strings.TrimSpace(timestamp), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp")
	}
	issuedAt := time.Unix(parsedTS, 0).UTC()
	now = now.UTC()
	if issuedAt.After(now.Add(g.maxSkew)) {
		return fmt.Errorf("timestamp in the future")
	}
	if now.Sub(issuedAt) > g.ttl {
		return fmt.Errorf("stale callback")
	}

	key := fmt.Sprintf("%s:%d:%s", scope, id, trimmedNonce)
	g.mu.Lock()
	defer g.mu.Unlock()

	cutoff := now.Add(-g.ttl)
	for seenKey, seenTime := range g.seen {
		if seenTime.Before(cutoff) {
			delete(g.seen, seenKey)
		}
	}

	if _, exists := g.seen[key]; exists {
		return fmt.Errorf("replayed callback")
	}

	g.seen[key] = now
	return nil
}
