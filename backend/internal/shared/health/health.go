// Package health waits for a shared app to answer on its port.
package health

import (
	"context"
	"errors"
	"net/http"
	"time"
)

var ErrNoAnswer = errors.New("o app ligou, mas não respondeu")

var client = &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func Check(ctx context.Context, url string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	res, err := client.Do(req)
	if err != nil {
		return false
	}
	res.Body.Close()
	return res.StatusCode < 500
}

func Wait(ctx context.Context, url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if Check(ctx, url) {
			return nil
		}
		if time.Now().After(deadline) {
			return ErrNoAnswer
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}
