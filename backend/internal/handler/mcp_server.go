package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const mcpInstructions = "O Ziv é o lugar da empresa onde ficam os apps, os scripts e os artifacts do time. " +
	"Estas ferramentas agem como a pessoa que conectou o Claude: veem e fazem só o que ela pode ver e fazer no Ziv."

// mcpPersonFrom is the person a tool call acts for (0 outside MCP).
func mcpPersonFrom(ctx context.Context) int64 {
	id, _ := mcpPersonID(ctx)
	return id
}

// newMCPServer builds the MCP server for this company with all its tools.
func (h *Handler) newMCPServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "ziv", Version: "1"}, &mcp.ServerOptions{Instructions: mcpInstructions})
	h.registerMCPTools(s)
	return s
}

// mcpHTTP is the /mcp endpoint: a stateless MCP server behind a bearer check.
func (h *Handler) mcpHTTP() http.Handler {
	return h.mcpHTTPFor(h.newMCPServer())
}

func (h *Handler) mcpHTTPFor(s *mcp.Server) http.Handler {
	sdk := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s }, &mcp.StreamableHTTPOptions{
		Stateless: true,
		// The endpoint needs a bearer token, and behind a proxy on the same
		// machine the public Host is never localhost.
		DisableLocalhostProtection: true,
		// The SDK default (4 MiB) is below what the screen allows: a 16 MiB
		// HTML plus JSON escaping and the tool call envelope.
		MaxRequestBodyBytes:          maxArtifactBody + 1<<20,
		PropagateRequestCancellation: true,
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme, token, _ := strings.Cut(r.Header.Get("Authorization"), " ")
		if !strings.EqualFold(scheme, "Bearer") || !strings.HasPrefix(token, "ziv_") {
			token = ""
		}
		_, p, err := h.mcpConnection(r.Context(), token)
		if err != nil {
			if !errors.Is(err, errMCPUnauthorized) {
				internalError(w, "mcp connection", err)
				return
			}
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+h.mcpResourceMetadataURL()+`"`)
			writeError(w, http.StatusUnauthorized, "Conecte o Claude ao Ziv de novo.")
			return
		}
		sdk.ServeHTTP(w, r.WithContext(withMCPPerson(r.Context(), p.ID)))
	})
}

// mcpResourceMetadataURL is <origin>/.well-known/oauth-protected-resource<basePath>/mcp.
func (h *Handler) mcpResourceMetadataURL() string {
	origin := strings.TrimRight(h.cfg.BaseURL, "/")
	if u, err := url.Parse(h.cfg.BaseURL); err == nil && u.Scheme != "" && u.Host != "" {
		origin = u.Scheme + "://" + u.Host
	}
	base := ""
	if h.workspace != nil {
		base = h.workspace.BasePath
	}
	return origin + "/.well-known/oauth-protected-resource" + base + "/mcp"
}

// mcpRoutes are the only API routes MCP tools may call: each one was
// reviewed for use by Claude. {id} matches digits only.
var mcpRoutes = []string{
	"GET /api/apps",
	"GET /api/apps/{id}",
	"POST /api/apps/{id}/runs",
	"GET /api/runs",
	"GET /api/runs/{id}",
	"GET /api/artifacts",
	"GET /api/artifacts/{id}",
	"POST /api/artifacts",
	"POST /api/artifacts/{id}/versions",
	"GET /api/areas",
	"POST /api/scripts/import",
}

var errMCPRoute = errors.New("mcp: rota não liberada para o Claude")

func mcpRouteAllowed(method, path string) bool {
	got := strings.Split(path, "/")
	for _, route := range mcpRoutes {
		m, pattern, _ := strings.Cut(route, " ")
		want := strings.Split(pattern, "/")
		if m != method || len(want) != len(got) {
			continue
		}
		ok := true
		for i, seg := range want {
			if seg == "{id}" {
				ok = got[i] != "" && strings.Trim(got[i], "0123456789") == ""
			} else {
				ok = seg == got[i]
			}
			if !ok {
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// callAPI runs one API route internally as the MCP person in ctx (set by
// mcpHTTP, see withMCPPerson); without one the route answers 401. It goes
// straight to the route table, so everything Secure adds is bypassed: the
// browser checks, the rate limits, the audit and the portal guard against
// reading server folders. That is why only the routes in mcpRoutes may be
// called; any other route is refused with errMCPRoute.
func (h *Handler) callAPI(ctx context.Context, method, path string, body any) (int, []byte, error) {
	route, _, _ := strings.Cut(path, "?")
	if !mcpRouteAllowed(method, route) {
		return 0, nil, fmt.Errorf("%w: %s %s", errMCPRoute, method, route)
	}
	var rd io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		rd = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, path, rd)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.api.ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes(), nil
}

// apiError is the "error" text of an API JSON reply, or "".
func apiError(raw []byte) string {
	var e struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(raw, &e) != nil {
		return ""
	}
	return e.Error
}
