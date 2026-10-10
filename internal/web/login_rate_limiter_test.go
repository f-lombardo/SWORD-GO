package web

import (
	"testing"
	"time"
)

func TestLoginRateLimiterBlocksAfterLimit(t *testing.T) {
	limiter := newLoginRateLimiter(3, 10*time.Minute)
	now := time.Now().UTC()
	client := "203.0.113.10"

	if !limiter.Allow(client, now) {
		t.Fatalf("expected initial attempt to be allowed")
	}
	limiter.RegisterFailure(client, now)
	limiter.RegisterFailure(client, now.Add(time.Second))
	limiter.RegisterFailure(client, now.Add(2*time.Second))

	if limiter.Allow(client, now.Add(3*time.Second)) {
		t.Fatalf("expected client to be blocked after limit")
	}
	if !limiter.Allow(client, now.Add(11*time.Minute)) {
		t.Fatalf("expected block to expire after window")
	}
}

func TestLoginRateLimiterResetClearsBlock(t *testing.T) {
	limiter := newLoginRateLimiter(2, 5*time.Minute)
	now := time.Now().UTC()
	client := "198.51.100.20"

	limiter.RegisterFailure(client, now)
	limiter.RegisterFailure(client, now.Add(time.Second))
	if limiter.Allow(client, now.Add(2*time.Second)) {
		t.Fatalf("expected blocked client before reset")
	}
	limiter.Reset(client)
	if !limiter.Allow(client, now.Add(2*time.Second)) {
		t.Fatalf("expected reset to clear block")
	}
}
