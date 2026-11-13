package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A program that listens but answers too late is "slow"; nobody listening is not.
func TestProbeTellsSlowFromSilent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if ok, slow := Probe(ctx, srv.URL); ok || !slow {
		t.Fatalf("slow page: ok=%v slow=%v", ok, slow)
	}
	if ok, slow := Probe(context.Background(), "http://127.0.0.1:1/"); ok || slow {
		t.Fatalf("nobody listening: ok=%v slow=%v", ok, slow)
	}
}
