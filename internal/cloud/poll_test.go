package cloud

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWaitForSuccess(t *testing.T) {
	attempts := 0
	val, err := WaitFor(context.Background(), 5, 10*time.Millisecond, func() (*string, error) {
		attempts++
		if attempts < 3 {
			return nil, nil
		}
		res := "ready"
		return &res, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val == nil || *val != "ready" {
		t.Fatalf("expected 'ready', got %#v", val)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestWaitForContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	val, err := WaitFor(ctx, 5, 10*time.Millisecond, func() (*string, error) {
		res := "never"
		return &res, nil
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled error, got %v", err)
	}
	if val != nil {
		t.Fatalf("expected nil value, got %#v", val)
	}
}

func TestWaitForExhaustedAttempts(t *testing.T) {
	val, err := WaitFor(context.Background(), 2, 5*time.Millisecond, func() (*string, error) {
		return nil, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != nil {
		t.Fatalf("expected nil value when exhausted, got %#v", val)
	}
}

func TestWaitForReturnsLastError(t *testing.T) {
	expectedErr := errors.New("network failure")
	val, err := WaitFor(context.Background(), 2, 5*time.Millisecond, func() (*string, error) {
		return nil, expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
	if val != nil {
		t.Fatalf("expected nil value, got %#v", val)
	}
}
