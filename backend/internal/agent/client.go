package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

var (
	ErrUnauthorized = errors.New("agent token rejected")
	ErrGone         = errors.New("pairing expired or used")
	ErrNotFound     = errors.New("hub does not know this job")
)

// Client talks to the Hub. Every call is started by the agent.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
	// RequestTimeout bounds every call except uploads (default 60 s, above
	// the Hub's 25 s long-poll), so a half-open connection can't freeze the agent.
	RequestTimeout time.Duration
}

func (c *Client) timeout() time.Duration {
	if c.RequestTimeout > 0 {
		return c.RequestTimeout
	}
	return 60 * time.Second
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader, contentType string, out any) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout())
	defer cancel()
	return c.send(ctx, method, path, body, contentType, out)
}

func (c *Client) send(ctx context.Context, method, path string, body io.Reader, contentType string, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return 0, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return resp.StatusCode, ErrUnauthorized
	case resp.StatusCode == http.StatusGone:
		return resp.StatusCode, ErrGone
	case resp.StatusCode == http.StatusNotFound:
		return resp.StatusCode, ErrNotFound
	case resp.StatusCode >= 300:
		return resp.StatusCode, fmt.Errorf("hub %s %s: %d", method, path, resp.StatusCode)
	}
	if out != nil && resp.StatusCode == http.StatusOK {
		return resp.StatusCode, json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode, nil
}

func (c *Client) postJSON(ctx context.Context, path string, in, out any) (int, error) {
	b, err := json.Marshal(in)
	if err != nil {
		return 0, err
	}
	return c.do(ctx, "POST", path, bytes.NewReader(b), "application/json", out)
}

func (c *Client) PairStart(ctx context.Context, name string) (agentproto.PairStartResponse, error) {
	var out agentproto.PairStartResponse
	_, err := c.postJSON(ctx, "/api/agent/pair/start", agentproto.PairStartRequest{ComputerName: name}, &out)
	return out, err
}

func (c *Client) PairPoll(ctx context.Context, secret string) (agentproto.PairPollResponse, error) {
	var out agentproto.PairPollResponse
	_, err := c.postJSON(ctx, "/api/agent/pair/poll", agentproto.PairPollRequest{Secret: secret}, &out)
	return out, err
}

func (c *Client) Hello(ctx context.Context, name string) error {
	_, err := c.postJSON(ctx, "/api/agent/hello", agentproto.Hello{Name: name, Version: agentproto.Version, CanUpdate: true, CanStop: true}, nil)
	return err
}

// Next waits for the next job; nil means none arrived in the Hub's wait.
func (c *Client) Next(ctx context.Context) (*agentproto.Job, error) {
	var j agentproto.Job
	code, err := c.do(ctx, "GET", "/api/agent/next", nil, "", &j)
	if err != nil || code == http.StatusNoContent {
		return nil, err
	}
	return &j, nil
}

func (c *Client) Result(ctx context.Context, jobID int64, r agentproto.Result) error {
	_, err := c.postJSON(ctx, "/api/agent/jobs/"+strconv.FormatInt(jobID, 10)+"/result", r, nil)
	return err
}

// DownloadBinary streams the Hub's agent binary into w. No timeout: it is a
// multi-MB file.
func (c *Client) DownloadBinary(ctx context.Context, w io.Writer) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.BaseURL+"/api/agent/binary", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return 0, ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("hub GET /api/agent/binary: %d", resp.StatusCode)
	}
	return io.Copy(w, resp.Body)
}

func (c *Client) Upload(ctx context.Context, jobID int64, body io.Reader) error {
	// No timeout: a big file or zip can take long.
	_, err := c.send(ctx, "POST", "/api/agent/jobs/"+strconv.FormatInt(jobID, 10)+"/upload", body, "application/octet-stream", nil)
	return err
}
