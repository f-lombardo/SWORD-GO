package cloud

import (
	"context"
	"time"
)

// WaitFor repeatedly invokes pollFunc until it returns a non-nil value, the context is cancelled, or attempts run out.
func WaitFor[T any](ctx context.Context, attempts int, interval time.Duration, pollFunc func() (*T, error)) (*T, error) {
	var lastErr error

	for range attempts {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}

		result, err := pollFunc()
		if err != nil {
			lastErr = err
			continue
		}

		if result != nil {
			return result, nil
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}

	return nil, nil
}
