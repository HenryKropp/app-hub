package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

// MCP F1: Claude sees whether a screen app is really in the air, which
// version runs, and the app's log, instead of guessing.

// logLinesMCP is how much of a log goes back to Claude by default.
const logLinesMCP = 50

// telaNoAr says whether the screen app answers at its address now (or wakes
// up when opened), and a short sentence for Claude.
func telaNoAr(st sharedGet) (bool, string) {
	if st.Shared == nil {
		return false, "O app com tela ainda não foi publicado."
	}
	ver := ""
	if st.LiveVersion != nil {
		ver = fmt.Sprintf(" (versão %d)", *st.LiveVersion)
	}
	switch st.Shared.Status {
	case "running":
		if st.LiveVersion != nil {
			return true, "No ar" + ver + "."
		}
	case "sleeping":
		if st.LiveVersion != nil {
			return true, "No ar" + ver + ", dormindo: acorda sozinho quando alguém abre o endereço."
		}
	case "starting":
		return false, "Ligando" + ver + "; ainda não está no ar."
	case "crashed":
		msg := "Caiu: o programa parou sozinho" + ver + "."
		if m := strings.TrimSpace(st.Shared.StatusMessage); m != "" {
			msg += " " + m
		}
		return false, msg + " Veja o motivo com ver_log_app."
	case "stopped":
		by := ""
		if st.Shared.StoppedBy != "" {
			by = " por " + st.Shared.StoppedBy
		}
		return false, "Parado" + by + ": não está no ar. Quem cuida liga de novo pela ficha do app."
	}
	return false, "Não está no ar: nenhuma versão chegou a ligar."
}

// lastPublish is the newest publish (not a test), if any.
func lastPublish(deps []deploymentOut) *deploymentOut {
	for i := range deps {
		if deps[i].Kind == "live" {
			return &deps[i]
		}
	}
	return nil
}

// tailLines keeps the last n lines of a log.
func tailLines(text string, n int) string {
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// fenced puts a log between ``` so Claude reads it as text from the app.
func fenced(log string) string {
	return "```\n" + strings.ReplaceAll(log, "```", "'''") + "\n```"
}

// ---- ver_log_app ----

type verLogIn struct {
	AppID  int64 `json:"app_id"`
	Linhas int   `json:"linhas,omitempty" jsonschema:"quantas linhas do fim do log, até 200 (padrão 50)"`
}

type verLogOut struct {
	AppID    int64  `json:"app_id"`
	NoAr     bool   `json:"no_ar"`
	Situacao string `json:"situacao"`
	Origem   string `json:"origem"` // app_no_ar, ultima_publicacao, nenhum
	Log      string `json:"log"`
}

func (h *Handler) toolVerLogApp(ctx context.Context, _ *mcp.CallToolRequest, in verLogIn) (*mcp.CallToolResult, any, error) {
	if in.AppID == 0 {
		return toolFail("Mande em app_id o número do app.")
	}
	n := logLinesMCP
	if in.Linhas > 0 {
		n = min(in.Linhas, 200)
	}
	base := fmt.Sprintf("/api/apps/%d", in.AppID)
	var d appDetail
	if _, _, msg := h.toolAPI(ctx, "GET", base, nil, &d); msg != "" {
		return toolFail(msg)
	}
	if d.Manifest.Kind != manifest.KindWeb {
		return toolFail("Este app não é um app com tela. O resultado de cada vez que ele rodou está em ver_execucao.")
	}
	if !d.Can.Tech {
		return toolFail("O log deste app só aparece para quem cuida dele (ou um administrador).")
	}
	var st sharedGet
	if status, raw, msg := h.toolAPI(ctx, "GET", base+"/shared", nil, &st); msg != "" {
		return toolFail(sharedFailure(status, raw, msg))
	}
	out := verLogOut{AppID: in.AppID}
	out.NoAr, out.Situacao = telaNoAr(st)

	var logs struct {
		Text string `json:"text"`
	}
	if status, raw, msg := h.toolAPI(ctx, "GET", base+"/shared/logs", nil, &logs); msg != "" {
		return toolFail(sharedFailure(status, raw, msg))
	}
	parts := []string{out.Situacao}
	if t := tailLines(logs.Text, n); t != "" {
		out.Origem, out.Log = "app_no_ar", t
		parts = append(parts, "Fim do log da versão que está ligada:\n"+fenced(t))
	}
	// A failed publish newer than what runs: its log explains why it never went up.
	if dep := lastPublish(st.Deployments); dep != nil && dep.Status == "failed" {
		erro := strings.TrimSpace(dep.ErrorMessage)
		line := "A última publicação falhou"
		if dep.VersionNumber != nil {
			line = fmt.Sprintf("A última publicação (versão %d) falhou", *dep.VersionNumber)
		}
		if erro != "" {
			line += ": " + erro
		}
		parts = append(parts, line)
		if dep.BuildLog != nil {
			if t := tailLines(*dep.BuildLog, n); t != "" {
				if out.Log == "" {
					out.Origem, out.Log = "ultima_publicacao", t
				}
				parts = append(parts, "Fim do log dessa publicação:\n"+fenced(t))
			}
		}
	}
	if out.Log == "" {
		out.Origem = "nenhum"
		parts = append(parts, "Não há log para mostrar: nenhuma versão está ligada e nenhuma publicação falhou.")
	}
	return toolOK(strings.Join(parts, "\n\n"), out)
}

// verAppTela adds to ver_app whether the screen app is in the air, the
// version that runs and how the last publish went.
func (h *Handler) verAppTela(ctx context.Context, appID int64, out map[string]any) {
	var st sharedGet
	if _, _, msg := h.toolAPI(ctx, "GET", fmt.Sprintf("/api/apps/%d/shared", appID), nil, &st); msg != "" {
		return
	}
	noAr, frase := telaNoAr(st)
	out["no_ar"], out["situacao_no_ar"] = noAr, frase
	if st.LiveVersion != nil {
		out["versao_no_ar"] = *st.LiveVersion
	}
	if dep := lastPublish(st.Deployments); dep != nil {
		last := map[string]any{"situacao": dep.Status}
		if dep.VersionNumber != nil {
			last["versao"] = *dep.VersionNumber
		}
		if dep.ErrorMessage != "" {
			last["erro"] = dep.ErrorMessage
		}
		if dep.Status == "building" && dep.Stage != "" {
			last["etapa"] = dep.Stage
		}
		out["ultima_publicacao"] = last
	}
}
