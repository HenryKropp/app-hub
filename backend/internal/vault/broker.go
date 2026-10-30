package vault

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var ErrBroker = errors.New("credential destination unavailable")

// ValidateEndpoint admits only fixed HTTPS paths, never userinfo, query tokens,
// fragments, custom ports or IP literals. DNS is checked again at connection.
func ValidateEndpoint(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && len(raw) <= 2048 && u.Scheme == "https" &&
		u.Hostname() != "" && u.User == nil && u.RawQuery == "" && !u.ForceQuery &&
		u.Fragment == "" && u.Opaque == "" && (u.Port() == "" || u.Port() == "443") &&
		net.ParseIP(u.Hostname()) == nil && strings.Contains(u.Hostname(), ".") &&
		!strings.ContainsAny(raw, "\r\n\\")
}

func publicAddress(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	// Non-public special-purpose ranges (including cloud metadata and CGNAT).
	for _, cidr := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001::/23", "2001:db8::/32", "2002::/16", "64:ff9b::/96"} {
		if netip.MustParsePrefix(cidr).Contains(ip) {
			return false
		}
	}
	return true
}

// CheckBearer is a deliberately narrow intermediary: fixed HEAD request,
// bearer header only, no redirects/proxy/body/response-header forwarding.
// It returns only a status code, not upstream data that could echo the secret.
func CheckBearer(ctx context.Context, endpoint string, value []byte) (int, error) {
	if !ValidateEndpoint(endpoint) || len(value) == 0 || strings.ContainsAny(string(value), "\r\n") {
		return 0, ErrBroker
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tr := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil || port != "443" {
				return nil, ErrBroker
			}
			ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
			if err != nil || len(ips) == 0 {
				return nil, ErrBroker
			}
			for _, ip := range ips {
				if !publicAddress(ip) {
					return nil, ErrBroker
				}
			}
			// Pin the checked IP. TLS still verifies the original hostname.
			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(ips[0].String(), port))
		},
		TLSHandshakeTimeout:    5 * time.Second,
		ResponseHeaderTimeout:  5 * time.Second,
		MaxResponseHeaderBytes: 16384,
	}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return doBearer(ctx, endpoint, value, client)
}

func doBearer(ctx context.Context, endpoint string, value []byte, client *http.Client) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, endpoint, nil)
	if err != nil {
		return 0, ErrBroker
	}
	req.Header.Set("Authorization", "Bearer "+string(value))
	resp, err := client.Do(req)
	req.Header.Del("Authorization")
	if err != nil {
		return 0, ErrBroker // URL errors may contain credentials: never return them.
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}
