package vault

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"testing"
)

type brokerTransport func(*http.Request) (*http.Response, error)

func (f brokerTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestBrokerDoesNotForwardProviderContentOrErrors(t *testing.T) {
	const value = "synthetic-provider-echo-canary"
	var captured *http.Request
	client := &http.Client{
		Transport: brokerTransport(func(r *http.Request) (*http.Response, error) {
			captured = r
			if r.Method != "HEAD" || r.Body != nil || r.Header.Get("Authorization") != "Bearer "+value {
				t.Fatal("not a fixed authenticated HEAD")
			}
			return &http.Response{
				StatusCode: 302, Header: http.Header{"Location": {"https://other.example.test/"}, "X-Echo": {value}},
				Body: io.NopCloser(strings.NewReader(value)), Request: r,
			}, nil
		}),
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	code, err := doBearer(context.Background(), "https://api.example.test/check", []byte(value), client)
	if err != nil || code != 302 {
		t.Fatal("broker did not return status only")
	}
	if captured.Header.Get("Authorization") != "" {
		t.Fatal("authorization header retained")
	}
	client.Transport = brokerTransport(func(*http.Request) (*http.Response, error) { return nil, errors.New(value) })
	if _, err := doBearer(context.Background(), "https://api.example.test/check", []byte(value), client); err != ErrBroker || strings.Contains(err.Error(), value) {
		t.Fatal("provider error exposed")
	}
}

func TestBrokerDestinations(t *testing.T) {
	for _, endpoint := range []string{
		"http://api.example.test/check", "https://user:pass@example.test/", "https://api.example.test/?token=value",
		"https://api.example.test/#token", "https://127.0.0.1/", "https://[::1]/", "https://api.example.test:444/",
		"https://localhost/", "https://api.example.test/\\evil", "https://api.example.test/\r\n",
	} {
		if ValidateEndpoint(endpoint) {
			t.Fatal("unsafe destination accepted")
		}
	}
	if !ValidateEndpoint("https://api.example.test/check") {
		t.Fatal("HTTPS fixed path rejected")
	}
	for _, addr := range []string{"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.1.1", "169.254.169.254", "100.100.100.200", "::1", "fc00::1", "fe80::1", "::ffff:127.0.0.1", "2002:7f00:1::", "64:ff9b::7f00:1", "192.0.2.1"} {
		if publicAddress(netip.MustParseAddr(addr)) {
			t.Fatal("non-public address accepted")
		}
	}
	if !publicAddress(netip.MustParseAddr("8.8.8.8")) {
		t.Fatal("public address rejected")
	}
	if _, err := CheckBearer(context.Background(), "https://127.0.0.1/", []byte("test")); err != ErrBroker {
		t.Fatal("IP destination not rejected")
	}
	if _, err := CheckBearer(context.Background(), "https://api.example.test/", []byte("test\r\nInjected: value")); err != ErrBroker {
		t.Fatal("header injection not rejected")
	}
	// A syntactically valid hostname resolving to a private address must also
	// fail at dial time. localhost.localdomain is not assumed in this test.
	if _, err := CheckBearer(context.Background(), "https://unresolvable.invalid/", []byte("test")); err != ErrBroker {
		t.Fatal("DNS failure leaked raw errors")
	}
}
