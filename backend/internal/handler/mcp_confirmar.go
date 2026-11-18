package handler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// planBoxTimeout is how long the box waits for the person.
const planBoxTimeout = 10 * time.Minute

const msgPlanConfirmedChat = "Plano de teste confirmado por você no Claude Code."

// askPlanInChat asks the person, in a box Claude Code shows (MCP
// elicitation), to confirm the test plan of app appID. The answer comes from
// the program, never from the model (ADR 047). Only the owner or an admin,
// with a PC of their own, is asked. confirmed is true when the plan was
// stored; note is a refusal to tell (plan changed), else "".
func (h *Handler) askPlanInChat(ctx context.Context, ss *mcp.ServerSession, appID int64) (confirmed bool, note string) {
	if ss == nil {
		return false, ""
	}
	if p := ss.InitializeParams(); p == nil || p.Capabilities == nil || p.Capabilities.Elicitation == nil {
		return false, ""
	}
	me, err := h.q.GetPerson(ctx, mcpPersonFrom(ctx))
	if err != nil {
		return false, ""
	}
	a, err := h.q.GetApp(ctx, appID)
	if err != nil {
		return false, ""
	}
	acc, err := h.loadAccess(ctx, me)
	if err != nil || !ownerOrAdmin(acc, me, a) {
		return false, ""
	}
	v, err := h.testPlanView(ctx, a, me, acc)
	if err != nil || v.Plan == nil || len(v.Computers) == 0 {
		return false, ""
	}
	// One box per plan and person: after Recusar, closing or the timeout,
	// following up with ver_versao gives the link instead of a new box.
	asked := fmt.Sprintf("%d:%d:%s", appID, me.ID, v.Fingerprint)
	if _, done := h.planBoxes.LoadOrStore(asked, true); done {
		return false, ""
	}

	var msg strings.Builder
	fmt.Fprintf(&msg, "Confirmar o plano de teste de %q?\nO teste roda no seu computador.", a.Name)
	if len(v.Plan.Keys) > 0 {
		fmt.Fprintf(&msg, "\nUsa as chaves: %s.", strings.Join(v.Plan.Keys, ", "))
	} else {
		msg.WriteString("\nNão usa chaves do Cofre.")
	}
	if len(v.Plan.Acessa) > 0 {
		fmt.Fprintf(&msg, "\nAcessa: %s.", strings.Join(v.Plan.Acessa, ", "))
	}
	if v.Plan.Pula != "" {
		fmt.Fprintf(&msg, "\nPula: %s.", v.Plan.Pula)
	}
	// One PC: no question. Several: the person picks; labels carry the
	// number, so two PCs with the same name stay apart.
	labels := map[string]int64{}
	props := map[string]any{}
	if len(v.Computers) == 1 {
		fmt.Fprintf(&msg, "\nComputador: %s.", v.Computers[0].Name)
	} else {
		var enum []string
		for _, c := range v.Computers {
			l := fmt.Sprintf("%s (nº %d)", c.Name, c.ID)
			labels[l] = c.ID
			enum = append(enum, l)
		}
		props["computador"] = map[string]any{"type": "string", "title": "Em qual computador", "enum": enum, "default": enum[0]}
	}
	box, cancel := context.WithTimeout(ctx, planBoxTimeout)
	defer cancel()
	res, err := ss.Elicit(box, &mcp.ElicitParams{Message: msg.String(),
		RequestedSchema: map[string]any{"type": "object", "properties": props}})
	if err != nil {
		slog.Info("plan box not answered", "app", appID, "err", err)
		return false, ""
	}
	if res.Action != "accept" {
		return false, ""
	}
	pc := v.Computers[0].ID
	if len(v.Computers) > 1 {
		chosen, _ := res.Content["computador"].(string)
		id, ok := labels[chosen]
		if !ok {
			return false, ""
		}
		pc = id
	}
	// The box may stay open for minutes: check the plan, the person and the
	// PC as they are now, not as they were when the box opened.
	if a, err = h.q.GetApp(ctx, appID); err != nil {
		return false, ""
	}
	if me, err = h.q.GetPerson(ctx, me.ID); err != nil {
		return false, ""
	}
	if acc, err = h.loadAccess(ctx, me); err != nil || !ownerOrAdmin(acc, me, a) {
		return false, ""
	}
	if c, err := h.q.GetComputer(ctx, pc); err != nil || c.PersonID != me.ID || c.RevokedAt.Valid {
		return false, ""
	}
	code, refusal, err := h.confirmTestPlanAs(ctx, a, me, pc, v.Fingerprint, planViaClaudeCode)
	if err != nil {
		slog.Error("confirm plan in chat", "app", appID, "err", err)
		return false, ""
	}
	if code != "" {
		return false, refusal
	}
	return true, ""
}
