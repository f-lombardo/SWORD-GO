package web

import (
	"strconv"
	"testing"
	"time"
)

func TestCallbackReplayGuardRejectsReplay(t *testing.T) {
	guard := newCallbackReplayGuard(10 * time.Minute)
	now := time.Now().UTC()
	ts := strconv.FormatInt(now.Unix(), 10)

	if err := guard.Validate("server_provision", 10, ts, "nonce-1", now); err != nil {
		t.Fatalf("expected first callback to be accepted, got %v", err)
	}
	if err := guard.Validate("server_provision", 10, ts, "nonce-1", now.Add(5*time.Second)); err == nil {
		t.Fatalf("expected replay callback to be rejected")
	}
}

func TestCallbackReplayGuardRejectsStaleTimestamp(t *testing.T) {
	guard := newCallbackReplayGuard(10 * time.Minute)
	now := time.Now().UTC()
	ts := strconv.FormatInt(now.Add(-11*time.Minute).Unix(), 10)
	if err := guard.Validate("site_install", 5, ts, "nonce-2", now); err == nil {
		t.Fatalf("expected stale callback to be rejected")
	}
}
