package mail

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResendSendsJSON(t *testing.T) {
	var got map[string]any
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	s := &resend{apiKey: "k", from: "Ziv <codigo@ziv.app>", endpoint: srv.URL, client: srv.Client()}
	if err := s.Send(context.Background(), "ana@cliente.com", "Assunto", "Texto"); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer k" || got["from"] != "Ziv <codigo@ziv.app>" || got["subject"] != "Assunto" || got["text"] != "Texto" {
		t.Fatalf("auth %q body %v", auth, got)
	}
	to, _ := got["to"].([]any)
	if len(to) != 1 || to[0] != "ana@cliente.com" {
		t.Fatalf("to %v", got["to"])
	}
}

func TestResendErrorStatusFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	defer srv.Close()
	s := &resend{apiKey: "k", from: "f", endpoint: srv.URL, client: srv.Client()}
	if err := s.Send(context.Background(), "a@b.com", "s", "t"); err == nil {
		t.Fatal("403 should fail")
	}
}

func TestFromConfig(t *testing.T) {
	if FromConfig("", "", false) != nil {
		t.Fatal("no key and no dev mode: no sender")
	}
	if _, ok := FromConfig("", "", true).(Log); !ok {
		t.Fatal("dev mode without key: Log")
	}
	if _, ok := FromConfig("k", "f", true).(*resend); !ok {
		t.Fatal("key and sender: Resend")
	}
	if FromConfig("k", "", false) != nil {
		t.Fatal("key without sender: no sender")
	}
}
