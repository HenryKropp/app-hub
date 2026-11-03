package handler

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// A link artifact is shown inside the Ziv page only when its site allows
// being framed by others. The check runs once, when the link is saved.

const embedCheckTimeout = 5 * time.Second

// NoFraming tells browsers no other site may show the Ziv in a frame. Besides
// clickjacking, this keeps a framed outside link (which runs with
// allow-same-origin) from ever loading a Ziv page by redirecting to it.
func NoFraming(h http.Header) {
	h.Set("Content-Security-Policy", "frame-ancestors 'none'")
	h.Set("X-Frame-Options", "DENY")
}

// Ranges a link check must never reach, beyond loopback/private/link-local:
// carrier NAT, "this network", IETF and benchmark blocks, reserved space, and
// IPv6 forms that embed an IPv4 address (NAT64, IPv4-compatible, 6to4, Teredo).
var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("::/96"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("2001::/32"),
}

// publicAddr refuses addresses inside the server's own network, so a pasted
// link cannot make the Ziv probe internal services.
func publicAddr(a netip.Addr) bool {
	a = a.Unmap()
	if !a.IsValid() || !a.IsGlobalUnicast() || a.IsPrivate() {
		return false
	}
	for _, p := range blockedPrefixes {
		if p.Contains(a) {
			return false
		}
	}
	return true
}

var errPrivateAddr = errors.New("address not public")

var embedTransport = &http.Transport{
	Proxy: nil,
	DialContext: (&net.Dialer{
		Timeout: embedCheckTimeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			ap, err := netip.ParseAddrPort(address)
			if err != nil || !publicAddr(ap.Addr()) {
				return errPrivateAddr
			}
			return nil
		},
	}).DialContext,
	TLSHandshakeTimeout:   embedCheckTimeout,
	ResponseHeaderTimeout: embedCheckTimeout,
}

// embedSlots caps how many link checks run at once across the server.
var embedSlots = make(chan struct{}, 4)

// checkLinkEmbeddable fetches the link's headers and says whether a browser
// would let another site show it in a frame. Any failure answers false: the
// artifact then keeps the "Abrir" card. A redirect to a host for which own
// answers true (the Ziv itself) also answers false.
func checkLinkEmbeddable(ctx context.Context, rawURL string, own func(string) bool) bool {
	ctx, cancel := context.WithTimeout(ctx, embedCheckTimeout)
	defer cancel()
	select {
	case embedSlots <- struct{}{}:
		defer func() { <-embedSlots }()
	case <-ctx.Done():
		return false
	}
	reachedOwn := false
	client := &http.Client{
		Transport: embedTransport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if own != nil && own(req.URL.String()) {
				reachedOwn = true
				return http.ErrUseLastResponse
			}
			if len(via) > 5 || req.URL.Scheme != "https" {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	// HEAD first, so links with side effects on GET are not triggered;
	// GET only for servers that refuse HEAD.
	resp, err := embedRequest(ctx, client, http.MethodHead, rawURL)
	if err == nil && (resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotImplemented) {
		resp, err = embedRequest(ctx, client, http.MethodGet, rawURL)
	}
	if err != nil || reachedOwn {
		return false
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return false
	}
	return framingAllowed(resp.Header)
}

func embedRequest(ctx context.Context, client *http.Client, method, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if req.URL.Scheme != "https" {
		return nil, errors.New("only https")
	}
	req.Header.Set("User-Agent", "Ziv-link-check/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	return resp, nil
}

// framingAllowed reads X-Frame-Options and CSP frame-ancestors the way
// browsers do: frame-ancestors, when present, wins over X-Frame-Options. Only
// sources that allow any site count ("*", "https:"); a list of named sites is
// taken as "not us".
func framingAllowed(h http.Header) bool {
	found := false
	for _, policy := range h.Values("Content-Security-Policy") {
		for _, directive := range strings.Split(policy, ";") {
			fields := strings.Fields(strings.ToLower(directive))
			if len(fields) == 0 || fields[0] != "frame-ancestors" {
				continue
			}
			found = true
			open := false
			for _, src := range fields[1:] {
				if src == "*" || src == "https:" {
					open = true
				}
			}
			if !open {
				return false
			}
		}
	}
	if found {
		return true
	}
	return strings.TrimSpace(h.Get("X-Frame-Options")) == ""
}

// zivHost says whether a link points at the Ziv itself or its shared apps
// (same host or a subdomain). Those are never shown in a frame: the frame
// allows same-origin for outside sites, which must not mean the Ziv's origin.
func (h *Handler) zivHost(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return true
	}
	host := strings.ToLower(u.Hostname())
	for _, own := range []string{h.cfg.BaseURL, "https://" + h.cfg.SharedDomain} {
		o, err := url.Parse(own)
		if err != nil || o.Hostname() == "" {
			continue
		}
		oh := strings.ToLower(o.Hostname())
		if host == oh || strings.HasSuffix(host, "."+oh) {
			return true
		}
	}
	return false
}
