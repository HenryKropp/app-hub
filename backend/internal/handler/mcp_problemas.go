package handler

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

// MCP F3a: one list of what went wrong in the apps the person takes care of,
// so Claude looks before touching an app and after publishing or running.
// Everything goes through the API as the person: nothing she could not see.

const (
	problemasDias      = 7
	problemasMaxDias   = 30
	problemasMaxApps   = 30 // screen apps and settings looked at in one call
	problemasMaxRuns   = 10 // failed runs detailed in one call
	problemasLogLinhas = 15
)

type problemasIn struct {
	AppID int64 `json:"app_id,omitempty" jsonschema:"só os problemas deste app; sem ele, todos os apps que a pessoa cuida"`
	Dias  int   `json:"dias,omitempty" jsonschema:"quantos dias olhar para trás nas execuções, até 30 (padrão 7)"`
}

type problema struct {
	Tipo     string `json:"tipo"` // execucao_com_erro, app_fora_do_ar, publicacao_falhou, agenda_com_problema, chave_faltando
	AppID    int64  `json:"app_id"`
	App      string `json:"app"`
	Quando   string `json:"quando,omitempty"`
	Versao   *int32 `json:"versao,omitempty"`
	Resumo   string `json:"resumo"`
	Trecho   string `json:"trecho_do_log,omitempty"`
	Link     string `json:"link"`
	sortTime time.Time
}

type problemasOut struct {
	Problemas []problema `json:"problemas"`
	Olhados   int        `json:"apps_olhados"`
	Cortado   bool       `json:"lista_cortada"`
}

// problemApp is the part of GET /api/apps that problemas needs.
type problemApp struct {
	ID     int64        `json:"id"`
	Name   string       `json:"name"`
	Kind   string       `json:"kind"`
	Can    access.Can   `json:"can"`
	Shared *sharedBrief `json:"shared"`
}

func (h *Handler) toolProblemas(ctx context.Context, _ *mcp.CallToolRequest, in problemasIn) (*mcp.CallToolResult, any, error) {
	dias := problemasDias
	if in.Dias > 0 {
		dias = min(in.Dias, problemasMaxDias)
	}
	since := time.Now().AddDate(0, 0, -dias)

	var apps []problemApp
	if _, _, msg := h.toolAPI(ctx, "GET", "/api/apps", nil, &apps); msg != "" {
		return toolFail(msg)
	}
	names := map[int64]string{}
	tech := map[int64]bool{}
	var mine []problemApp // the apps whose technical part the person sees
	for _, a := range apps {
		names[a.ID], tech[a.ID] = a.Name, a.Can.Tech
		if (in.AppID == 0 || a.ID == in.AppID) && a.Can.Tech {
			mine = append(mine, a)
		}
	}
	if in.AppID != 0 {
		if _, seen := names[in.AppID]; !seen {
			return toolFail("Não encontrei isso no Ziv, ou você não tem acesso.")
		}
		if !tech[in.AppID] {
			return toolFail("Os problemas deste app só aparecem para quem cuida dele (ou um administrador).")
		}
	}
	out := problemasOut{Problemas: []problema{}}
	if len(mine) > problemasMaxApps {
		mine, out.Cortado = mine[:problemasMaxApps], true
	}
	out.Olhados = len(mine)

	out.Problemas = append(out.Problemas, h.runProblems(ctx, in.AppID, since, &out.Cortado)...)
	out.Problemas = append(out.Problemas, h.scheduleProblems(ctx, in.AppID)...)
	for _, a := range mine {
		if a.Kind == manifest.KindWeb && a.Shared != nil {
			out.Problemas = append(out.Problemas, h.telaProblems(ctx, a)...)
		}
		out.Problemas = append(out.Problemas, h.settingsProblems(ctx, a)...)
	}
	sort.SliceStable(out.Problemas, func(i, j int) bool { return out.Problemas[i].sortTime.After(out.Problemas[j].sortTime) })

	var b strings.Builder
	scope := "nos apps que você cuida"
	if in.AppID != 0 {
		scope = fmt.Sprintf("no app %s", names[in.AppID])
	}
	if len(out.Problemas) == 0 {
		fmt.Fprintf(&b, "Nenhum problema %s nos últimos %d dias.", scope, dias)
	} else {
		fmt.Fprintf(&b, "%d problema(s) %s (execuções dos últimos %d dias):\n", len(out.Problemas), scope, dias)
		for _, p := range out.Problemas {
			when := ""
			if p.Quando != "" {
				when = " · " + p.sortTime.Local().Format("02/01 15:04")
			}
			fmt.Fprintf(&b, "- %s%s: %s %s\n", p.App, when, p.Resumo, p.Link)
			if p.Trecho != "" {
				b.WriteString(fenced(p.Trecho) + "\n")
			}
		}
		b.WriteString("Conte à pessoa o que achou, em poucas palavras. Para corrigir, leia o log (ver_log_app ou ver_execucao) antes de mudar o código.")
	}
	if out.Cortado {
		b.WriteString("\nA lista foi cortada: há mais apps ou execuções. Peça os problemas de um app por vez com app_id.")
	}
	return toolOK(strings.TrimSpace(b.String()), out)
}

// runProblems: runs that ended in error or were interrupted, newest first.
func (h *Handler) runProblems(ctx context.Context, appID int64, since time.Time, cut *bool) []problema {
	var list runListJSON
	if _, _, msg := h.toolAPI(ctx, "GET", "/api/runs?scope=all", nil, &list); msg != "" {
		return nil
	}
	var out []problema
	for _, x := range list.Items {
		if x.CreatedAt.Before(since) || (appID != 0 && x.AppID != appID) {
			continue
		}
		if x.Status != "error" && x.Status != "interrupted" {
			continue
		}
		if len(out) == problemasMaxRuns {
			*cut = true
			break
		}
		what := "A execução terminou com erro"
		if x.Status == "interrupted" {
			what = "A execução foi interrompida"
		}
		p := problema{Tipo: "execucao_com_erro", AppID: x.AppID, App: x.AppName, Quando: x.CreatedAt.Format(time.RFC3339),
			Resumo: what + ".", Link: h.mcpLink("/execucoes/%d", x.ID), sortTime: x.CreatedAt}
		var run runJSON
		if _, _, msg := h.toolAPI(ctx, "GET", fmt.Sprintf("/api/runs/%d", x.ID), nil, &run); msg == "" {
			p.Versao = run.VersionNumber
			if m := strings.TrimSpace(run.ErrorMessage); m != "" {
				p.Resumo = what + ": " + m
			}
		}
		out = append(out, p)
	}
	return out
}

// scheduleProblems: the person's schedules that paused or skipped a time.
func (h *Handler) scheduleProblems(ctx context.Context, appID int64) []problema {
	var list []scheduleJSON
	if _, _, msg := h.toolAPI(ctx, "GET", "/api/me/schedules", nil, &list); msg != "" {
		return nil
	}
	var out []problema
	for _, s := range list {
		if appID != 0 && s.AppID != appID {
			continue
		}
		agenda := "A agenda"
		if n := strings.TrimSpace(s.Name); n != "" {
			agenda = "A agenda " + n
		}
		var why string
		switch {
		case !s.Active && s.PausedReason != "":
			why = agenda + " está pausada: " + s.PausedReason
		case s.Situation != "":
			why = agenda + ": " + s.Situation
		default:
			continue
		}
		p := problema{Tipo: "agenda_com_problema", AppID: s.AppID, App: s.AppName, Resumo: why, Link: h.mcpLink("/apps/%d", s.AppID)}
		if s.LastRunAt != nil {
			p.Quando, p.sortTime = s.LastRunAt.Format(time.RFC3339), *s.LastRunAt
		}
		out = append(out, p)
	}
	return out
}

// telaProblems: a screen app that should be in the air and is not, and a
// publish that failed after the version in the air.
func (h *Handler) telaProblems(ctx context.Context, a problemApp) []problema {
	var st sharedGet
	if _, _, msg := h.toolAPI(ctx, "GET", fmt.Sprintf("/api/apps/%d/shared", a.ID), nil, &st); msg != "" || st.Shared == nil {
		return nil
	}
	link := h.mcpLink("/apps/%d", a.ID)
	var out []problema
	if noAr, frase := telaNoAr(st); !noAr && st.Shared.Status != "stopped" && st.LiveVersion != nil {
		p := problema{Tipo: "app_fora_do_ar", AppID: a.ID, App: a.Name, Versao: st.LiveVersion, Resumo: frase, Link: link, sortTime: time.Now()}
		var logs struct {
			Text string `json:"text"`
		}
		if _, _, msg := h.toolAPI(ctx, "GET", fmt.Sprintf("/api/apps/%d/shared/logs", a.ID), nil, &logs); msg == "" {
			p.Trecho = tailLines(logs.Text, problemasLogLinhas)
		}
		out = append(out, p)
	}
	if dep := lastPublish(st.Deployments); dep != nil && dep.Status == "failed" {
		p := problema{Tipo: "publicacao_falhou", AppID: a.ID, App: a.Name, Versao: dep.VersionNumber, Link: link,
			Resumo: "A última publicação falhou: " + strings.TrimSpace(dep.ErrorMessage)}
		if dep.CreatedAt.Valid {
			p.Quando, p.sortTime = dep.CreatedAt.Time.Format(time.RFC3339), dep.CreatedAt.Time
		}
		if dep.BuildLog != nil {
			p.Trecho = tailLines(*dep.BuildLog, problemasLogLinhas)
		}
		out = append(out, p)
	}
	return out
}

// settingsProblems: required keys or settings still empty.
func (h *Handler) settingsProblems(ctx context.Context, a problemApp) []problema {
	var d appDetail
	if _, _, msg := h.toolAPI(ctx, "GET", fmt.Sprintf("/api/apps/%d", a.ID), nil, &d); msg != "" || len(d.MissingSettings) == 0 {
		return nil
	}
	return []problema{{Tipo: "chave_faltando", AppID: a.ID, App: a.Name,
		Resumo: "Falta preencher em Configurações: " + strings.Join(d.MissingSettings, ", ") + ". Quem cuida preenche no site (ou com enviar_chaves).",
		Link:   h.mcpLink("/apps/%d#chaves", a.ID)}}
}
