// Package health waits for a shared app to answer on its port.
package health

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"time"
)

var ErrNoAnswer = errors.New("o app ligou, mas não respondeu")

// AnswerTimeout is how long one try waits for the page. A Flask or Streamlit
// page on the free 1 GB server can take several seconds, more while the app
// is still starting (ADR 041). A var only so tests can shorten it.
var AnswerTimeout = 15 * time.Second

var client = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

// do sends req with one try's time limit.
func do(req *http.Request) (*http.Response, error) {
	c := *client
	c.Timeout = AnswerTimeout
	return c.Do(req)
}

func Check(ctx context.Context, url string) bool {
	ok, _ := Probe(ctx, url)
	return ok
}

// Probe is Check that also says whether the try ran out of time (the program
// listens but answers too slowly), as opposed to nobody listening.
func Probe(ctx context.Context, url string) (ok, slow bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, false
	}
	res, err := do(req)
	if err != nil {
		var ne net.Error
		return false, errors.As(err, &ne) && ne.Timeout()
	}
	res.Body.Close()
	return res.StatusCode < 500, false
}

// Page reads the page at url (up to 1 MB) when it answers below 500.
func Page(ctx context.Context, url string) (string, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", false
	}
	res, err := do(req)
	if err != nil {
		return "", false
	}
	defer res.Body.Close()
	if res.StatusCode >= 500 {
		return "", false
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", false
	}
	return string(b), true
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
