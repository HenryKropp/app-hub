package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mcpCodeOut struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Code    string `json:"code"`
	Command string `json:"command"`
}

type mcpConnOut struct {
	ID         int64   `json:"id"`
	Kind       string  `json:"kind"`
	Name       string  `json:"name"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at"`
	Code       string  `json:"code"`
}

func TestMCPCodeCreateListRevoke(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	var out mcpCodeOut
	if code := c.do("POST", "/api/mcp/codes", map[string]string{"name": "Meu notebook"}, &out); code != http.StatusCreated {
		t.Fatalf("create = %d", code)
	}
	if !strings.HasPrefix(out.Code, "ziv_") || !strings.Contains(out.Command, "/mcp") || !strings.Contains(out.Command, out.Code) {
		t.Fatalf("bad create output: %+v", out)
	}
	if !strings.Contains(out.Command, `claude mcp add --transport http ziv http://hub.test/mcp --header "Authorization: Bearer `) {
		t.Fatalf("command = %s", out.Command)
	}
	var list []mcpConnOut
	c.do("GET", "/api/mcp/connections", nil, &list)
	if len(list) != 1 || list[0].Name != "Meu notebook" || list[0].Kind != "code" || list[0].Code != "" {
		t.Fatalf("list = %+v", list)
	}
	ctx := context.Background()
	if _, _, err := h.mcpConnection(ctx, out.Code); err != nil {
		t.Fatalf("token should work: %v", err)
	}
	if code := c.do("DELETE", fmt.Sprintf("/api/mcp/connections/%d", out.ID), nil, nil); code != http.StatusNoContent && code != http.StatusOK {
		t.Fatalf("delete = %d", code)
	}
	list = nil
	c.do("GET", "/api/mcp/connections", nil, &list)
	if len(list) != 0 {
		t.Fatalf("list after delete = %+v", list)
	}
	if _, _, err := h.mcpConnection(ctx, out.Code); !errors.Is(err, errMCPUnauthorized) {
		t.Fatalf("revoked token err = %v", err)
	}
	if _, _, err := h.mcpConnection(ctx, "ziv_desconhecido"); !errors.Is(err, errMCPUnauthorized) {
		t.Fatalf("unknown token err = %v", err)
	}
}

func TestMCPCodeOtherPersonCannotRevoke(t *testing.T) {
	admin, ana, _, _ := team(t)
	var out mcpCodeOut
	if code := admin.do("POST", "/api/mcp/codes", map[string]string{"name": "Do admin"}, &out); code != http.StatusCreated {
		t.Fatalf("create = %d", code)
	}
	if code := ana.do("DELETE", fmt.Sprintf("/api/mcp/connections/%d", out.ID), nil, nil); code != http.StatusNotFound {
		t.Fatalf("other person delete = %d, want 404", code)
	}
	var list []mcpConnOut
	admin.do("GET", "/api/mcp/connections", nil, &list)
	if len(list) != 1 {
		t.Fatalf("admin's row should remain: %+v", list)
	}
}

func TestCurrentPersonFromMCPContext(t *testing.T) {
	c, h := newPoolClient(t)
	p := setupAdmin(t, c)
	req := httptest.NewRequest("GET", "/api/auth/status", nil).WithContext(withMCPPerson(context.Background(), p.ID))
	rec := httptest.NewRecorder()
	h.api.ServeHTTP(rec, req)
	var st struct {
		Person *struct {
			ID int64 `json:"id"`
		} `json:"person"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil || st.Person == nil || st.Person.ID != p.ID {
		t.Fatalf("status person = %s", rec.Body.String())
	}
	if !viaMCP(withMCPPerson(context.Background(), p.ID)) || viaMCP(context.Background()) {
		t.Fatal("viaMCP wrong")
	}
}

func TestRunAndArtifactOriginMCP(t *testing.T) {
	c, h := newPoolClient(t)
	p := setupAdmin(t, c)
	body, _ := json.Marshal(linkBody("Pelo Claude", "https://exemplo.com/a"))
	req := httptest.NewRequest("POST", "/api/artifacts", bytes.NewReader(body)).WithContext(withMCPPerson(context.Background(), p.ID))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.api.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("mcp create = %d %s", rec.Code, rec.Body.String())
	}
	manual := createArtifact(t, c, linkBody("Manual", "https://exemplo.com/b"))
	if manual.Origin != "manual" {
		t.Fatalf("cookie origin = %q", manual.Origin)
	}
	var origins []string
	rows, err := testPool.Query(context.Background(), "SELECT origin FROM artifacts ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var o string
		_ = rows.Scan(&o)
		origins = append(origins, o)
	}
	if len(origins) != 2 || origins[0] != "mcp" || origins[1] != "manual" {
		t.Fatalf("origins = %v", origins)
	}
}
