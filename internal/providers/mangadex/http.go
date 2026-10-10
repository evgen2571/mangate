package mangadex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const maxRateLimitRetries = 3

// getJSON owns the response body and validates the common MangaDex envelope.
// Each caller validates its endpoint-specific fields after decoding.
func (pr *Provider) getJSON(ctx context.Context, operation, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pr.api(path), nil)
	if err != nil {
		return fmt.Errorf("create %s request: %w", operation, err)
	}

	resp, err := pr.doWithRateLimitRetry(req, strings.HasPrefix(path, "at-home/server/"))
	if err != nil {
		return fmt.Errorf("%s request in %q: %w", operation, pr.Name(), err)
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s request in %q returned HTTP %d (%s)", operation, pr.Name(), resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	const maxResponseBytes = 16 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read %s response: %w", operation, err)
	}

	if len(body) > maxResponseBytes {
		return fmt.Errorf("%s response exceeds %d bytes", operation, maxResponseBytes)
	}

	var envelope struct {
		Result  string          `json:"result"`
		Data    json.RawMessage `json:"data"`
		Chapter json.RawMessage `json:"chapter"`
		Errors  []struct {
			Title  string `json:"title"`
			Detail string `json:"detail"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("decode %s response: %w", operation, err)
	}

	if (envelope.Result != "" && envelope.Result != "ok") || len(envelope.Errors) > 0 {
		details := make([]string, 0, len(envelope.Errors))
		for _, apiError := range envelope.Errors {
			details = append(details, strings.TrimSpace(apiError.Title+" "+apiError.Detail))
		}

		return fmt.Errorf("%s API error in %q: %s", operation, pr.Name(), strings.Join(details, "; "))
	}

	if (len(envelope.Data) == 0 || string(envelope.Data) == "null") && (len(envelope.Chapter) == 0 || string(envelope.Chapter) == "null") {
		return fmt.Errorf("%s response has no data", operation)
	}

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode %s response: %w", operation, err)
	}

	return nil
}

func (pr *Provider) doWithRateLimitRetry(req *http.Request, atHome bool) (*http.Response, error) {
	currentReq := req

	for attempt := 0; ; attempt++ {
		if atHome {
			if err := pr.paceAtHomeRequest(currentReq.Context()); err != nil {
				return nil, fmt.Errorf("wait for at-home request slot: %w", err)
			}
		}

		resp, err := pr.client.Do(currentReq)
		if err != nil {
			return nil, err
		}

		if !retryableProviderStatus(resp.StatusCode) || attempt >= maxRateLimitRetries {
			return resp, nil
		}

		wait := retryAfterDelay(resp.Header.Get("Retry-After"), attempt)
		if timestamp, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Retry-After"), 10, 64); err == nil {
			if delay := time.Until(time.Unix(timestamp, 0)); delay > wait {
				wait = delay
			}
		}

		resp.Body.Close()

		if err := sleepWithContext(currentReq.Context(), wait); err != nil {
			return nil, fmt.Errorf("wait for provider retry: %w", err)
		}

		currentReq = currentReq.Clone(currentReq.Context())
	}
}

func retryableProviderStatus(status int) bool {
	return status == http.StatusTooManyRequests ||
		status == http.StatusInternalServerError ||
		status == http.StatusBadGateway ||
		status == http.StatusServiceUnavailable ||
		status == http.StatusGatewayTimeout
}

func retryAfterDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 && seconds <= int64((1<<63-1)/time.Second) {
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
