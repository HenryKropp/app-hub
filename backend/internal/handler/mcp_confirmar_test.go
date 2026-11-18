package handler

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// elicitSession is toolSession for a client that shows the Ziv's box: answer
// decides what the person "clicks"; asked counts the boxes shown.
func elicitSession(t *testing.T, c *client, answer func(*mcp.ElicitRequest) (*mcp.ElicitResult, error)) (*mcp.ClientSession, *atomic.Int32) {
	t.Helper()
	asked := &atomic.Int32{}
	client := mcp.NewClient(&mcp.Implementation{Name: "teste", Version: "1"}, &mcp.ClientOptions{
		ElicitationHandler: func(_ context.Context, r *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			asked.Add(1)
			return answer(r)
		},
	})
	s, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:   c.srv.URL + "/mcp",
		HTTPClient: &http.Client{Transport: bearerTransport{newMCPCode(t, c).Code}},
		MaxRetries: -1,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, asked
}

func accept(*mcp.ElicitRequest) (*mcp.ElicitResult, error) {
	return &mcp.ElicitResult{Action: "accept", Content: map[string]any{}}, nil
}

func decline(*mcp.ElicitRequest) (*mcp.ElicitResult, error) {
	return &mcp.ElicitResult{Action: "decline"}, nil
}

func planVia(t *testing.T, appID int64) string {
	t.Helper()
	var via string
	_ = testPool.QueryRow(context.Background(), "SELECT confirmed_via FROM app_test_plans WHERE app_id = $1", appID).Scan(&via)
	return via
}

const planLink = "Mande este link ao responsável pelo app para confirmar o plano de teste"

func TestChatConfirmStartsTest(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	pairComputer(t, c, "PC")
	var msg string
	s, asked := elicitSession(t, c, func(r *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
		msg = r.Params.Message
		return accept(r)
	})
	appID, draftID, text := enviarNovo(t, h, s)
	if asked.Load() != 1 || !strings.Contains(msg, "Conta CNPJs") || !strings.Contains(msg, "nada externo") {
		t.Fatalf("asked %d %q", asked.Load(), msg)
	}
	if planVia(t, appID) != planViaClaudeCode {
		t.Fatalf("via %q", planVia(t, appID))
	}
	if strings.Contains(text, planLink) || !strings.Contains(text, "Plano de teste confirmado por você no Claude Code.") {
		t.Fatalf("text %q", text)
	}
	if d := draftRow(t, h, draftID); d.Stage == "esperando_confirmacao" {
		t.Fatalf("draft still waits for the plan: %q", d.Stage)
	}
}

func TestChatConfirmDeclinedKeepsLink(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	pairComputer(t, c, "PC")
	s, asked := elicitSession(t, c, decline)
	appID, _, text := enviarNovo(t, h, s)
	if asked.Load() != 1 || planVia(t, appID) != "" || !strings.Contains(text, planLink) {
		t.Fatalf("asked %d via %q text %q", asked.Load(), planVia(t, appID), text)
	}
}

func TestChatConfirmNoCapabilityKeepsLink(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	pairComputer(t, c, "PC")
	s := toolSession(t, c)
	appID, _, text := enviarNovo(t, h, s)
	if planVia(t, appID) != "" || !strings.Contains(text, planLink) {
		t.Fatalf("via %q text %q", planVia(t, appID), text)
	}
}

func TestChatConfirmOnlyOwnerOrAdmin(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	pairComputer(t, c, "PC")
	appID, draftID, _ := enviarNovo(t, h, toolSession(t, c))
	createPersonAs(t, c, "Ana", "ana@exemplo.com", "criador")
	ana := c.fresh()
	loginAs(t, ana, "ana@exemplo.com")
	exec(t, "INSERT INTO area_members (area_id, person_id) SELECT id, 2 FROM areas WHERE name = 'Research' ON CONFLICT DO NOTHING")
	exec(t, "UPDATE apps SET restricted = FALSE WHERE id = $1", appID)
	pairComputer(t, ana, "PC da Ana")
	s, asked := elicitSession(t, ana, accept)
	callTool(t, s, "ver_versao", map[string]any{"app_id": appID, "versao": draftID})
	if asked.Load() != 0 || planVia(t, appID) != "" {
		t.Fatalf("asked %d via %q", asked.Load(), planVia(t, appID))
	}
}

func TestChatConfirmPlanChangedRefused(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	pairComputer(t, c, "PC")
	s, _ := elicitSession(t, c, func(r *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
		// The plan changes while the box is open.
		exec(t, `UPDATE app_drafts SET ficha = jsonb_set(COALESCE(ficha, '{}'::jsonb), '{teste,pula}', '"outra coisa"')`)
		exec(t, `UPDATE apps SET manifest = jsonb_set(manifest, '{teste,pula}', '"outra coisa"')`)
		return accept(r)
	})
	appID, _, text := enviarNovo(t, h, s)
	if planVia(t, appID) != "" || !strings.Contains(text, msgPlanChanged) {
		t.Fatalf("via %q text %q", planVia(t, appID), text)
	}
}

func TestChatConfirmNoPC(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	s, asked := elicitSession(t, c, accept)
	appID, _, _ := enviarNovo(t, h, s)
	if asked.Load() != 0 || planVia(t, appID) != "" {
		t.Fatalf("asked %d via %q", asked.Load(), planVia(t, appID))
	}
}

func TestChatConfirmChoosesPC(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	pairComputer(t, c, "Casa")
	pairComputer(t, c, "Escritório")
	var options []string
	s, _ := elicitSession(t, c, func(r *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
		props := r.Params.RequestedSchema.(map[string]any)["properties"].(map[string]any)
		for _, o := range props["computador"].(map[string]any)["enum"].([]any) {
			options = append(options, o.(string))
		}
		return &mcp.ElicitResult{Action: "accept", Content: map[string]any{"computador": options[len(options)-1]}}, nil
	})
	appID, _, _ := enviarNovo(t, h, s)
	var name string
	_ = testPool.QueryRow(context.Background(), "SELECT c.name FROM app_test_plans p JOIN computers c ON c.id = p.computer_id WHERE p.app_id = $1", appID).Scan(&name)
	if len(options) != 2 || !strings.HasPrefix(options[1], name) {
		t.Fatalf("options %v chosen %q", options, name)
	}
}

func TestChatConfirmAskedOnceAfterDecline(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	pairComputer(t, c, "PC")
	s, asked := elicitSession(t, c, decline)
	appID, draftID, _ := enviarNovo(t, h, s)
	// Following up with ver_versao must not bring the box back.
	_, text, _ := callTool(t, s, "ver_versao", map[string]any{"app_id": appID, "versao": draftID})
	if asked.Load() != 1 || planVia(t, appID) != "" || !strings.Contains(text, "/plano-de-teste") {
		t.Fatalf("asked %d via %q text %q", asked.Load(), planVia(t, appID), text)
	}
}

func TestChatConfirmPCRevokedWhileOpen(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	pairComputer(t, c, "PC")
	s, _ := elicitSession(t, c, func(r *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
		exec(t, "UPDATE computers SET revoked_at = now()")
		return accept(r)
	})
	appID, _, text := enviarNovo(t, h, s)
	if planVia(t, appID) != "" || !strings.Contains(text, planLink) {
		t.Fatalf("via %q text %q", planVia(t, appID), text)
	}
}

func TestChatConfirmNoLongerOwnerWhileOpen(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	pairComputer(t, c, "PC")
	createPersonAs(t, c, "Ana", "ana@exemplo.com", "criador")
	s, _ := elicitSession(t, c, func(r *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
		// The app goes to Ana and the person stops being admin.
		exec(t, "UPDATE apps SET owner_id = 2")
		exec(t, "UPDATE people SET role_id = (SELECT id FROM roles WHERE key = 'usuario') WHERE id = 1")
		return accept(r)
	})
	appID, _, text := enviarNovo(t, h, s)
	if planVia(t, appID) != "" || !strings.Contains(text, planLink) {
		t.Fatalf("via %q text %q", planVia(t, appID), text)
	}
}
