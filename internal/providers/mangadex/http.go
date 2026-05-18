package mangadex

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

const maxRateLimitRetries = 3

func (pr *Provider) doWithRateLimitRetry(req *http.Request) (*http.Response, error) {
	currentReq := req

	for attempt := 0; ; attempt++ {
		resp, err := pr.client.Do(currentReq)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusTooManyRequests || attempt >= maxRateLimitRetries {
			return resp, nil
		}

		wait := retryAfterDelay(resp.Header.Get("Retry-After"), attempt)
		_ = resp.Body.Close()

		if err := sleepWithContext(currentReq.Context(), wait); err != nil {
			return nil, fmt.Errorf("wait for rate limit reset: %w", err)
		}

		currentReq = currentReq.Clone(currentReq.Context())
	}
}

func retryAfterDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}

	if retryAt, err := http.ParseTime(value); err == nil {
		delay := time.Until(retryAt)
		if delay > 0 {
			return delay
		}
	}

	delay := time.Second << attempt
	if delay > 8*time.Second {
		return 8 * time.Second
	}
	return delay
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
