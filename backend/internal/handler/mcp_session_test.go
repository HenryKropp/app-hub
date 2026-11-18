package handler

import (
	"net/http"
	"strings"
	"testing"
)

// rawMCP posts one JSON-RPC message with a token and, if given, a session.
func rawMCP(t *testing.T, url, token, session, body string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("POST", url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+token)
	if session != "" {
		req.Header.Set("Mcp-Session-Id", session)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

const mcpInit = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`
const mcpList = `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`

// The MCP keeps a session (to ask the person in the chat, plano de teste),
// and a session belongs to the person whose token opened it.
func TestMCPSessionBoundToPerson(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	createPersonAs(t, c, "Ana", "ana@exemplo.com", "criador")
	ana := c.fresh()
	loginAs(t, ana, "ana@exemplo.com")
	url := c.srv.URL + "/mcp"
	henryCode, anaCode := newMCPCode(t, c).Code, newMCPCode(t, ana).Code

	resp := rawMCP(t, url, henryCode, "", mcpInit)
	session := resp.Header.Get("Mcp-Session-Id")
	if resp.StatusCode != http.StatusOK || session == "" {
		t.Fatalf("initialize %d session %q", resp.StatusCode, session)
	}
	rawMCP(t, url, henryCode, session, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if r := rawMCP(t, url, henryCode, session, mcpList); r.StatusCode != http.StatusOK {
		t.Fatalf("own session %d", r.StatusCode)
	}
	if r := rawMCP(t, url, anaCode, session, mcpList); r.StatusCode == http.StatusOK {
		t.Fatal("another person's token used Henry's session")
	}
}
