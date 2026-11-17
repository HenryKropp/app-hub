// Package mail sends the Ziv's e-mails (today only the external link code,
// ADR 045). Without RESEND_API_KEY and MAIL_FROM there is no sender, except
// in DEV_MODE, where the e-mail goes to the log.
package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type Sender interface {
	Send(ctx context.Context, to, subject, text string) error
}

type resend struct {
	apiKey, from, endpoint string
	client                 *http.Client
}

func NewResend(apiKey, from string) Sender {
	return &resend{apiKey: apiKey, from: from, endpoint: "https://api.resend.com/emails", client: &http.Client{Timeout: 10 * time.Second}}
}

func (s *resend) Send(ctx context.Context, to, subject, text string) error {
	body, err := json.Marshal(map[string]any{"from": s.from, "to": []string{to}, "subject": subject, "text": text})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("resend: status %d", resp.StatusCode)
	}
	return nil
}

// Log writes the e-mail to the server log (DEV_MODE only).
type Log struct{}

func (Log) Send(_ context.Context, to, subject, text string) error {
	slog.Info("e-mail (DEV_MODE, não enviado)", "to", to, "subject", subject, "text", text)
	return nil
}

// FromConfig picks Resend when it has a key and a sender, the log in dev
// mode, and nothing otherwise.
func FromConfig(apiKey, from string, devMode bool) Sender {
	switch {
	case apiKey != "" && from != "":
		return NewResend(apiKey, from)
	case devMode:
		return Log{}
	}
	return nil
}
