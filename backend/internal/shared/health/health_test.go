package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestWaitUntilHealthy(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) < 3 {
			w.WriteHeader(502)
			return
		}
		w.WriteHeader(404) // < 500 conta como "respondeu"
	}))
	defer srv.Close()
	if err := Wait(context.Background(), srv.URL, 5*time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestWaitGivesUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }))
	defer srv.Close()
	err := Wait(context.Background(), srv.URL, 1200*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "não respondeu") {
		t.Fatalf("err %v", err)
	}
}
