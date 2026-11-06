package handler

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/readiness"
)

// MCP Etapa 2: Claude publishes a screen app the same way the site's
// Compartilhar › Publicar does: the same checklist, the same "estou ciente"
// marks, the same settings (filled only on the site) and the same approval.

const (
	msgTelaAprovacao    = "Esperando a aprovação de um administrador; publico depois que aprovarem."
	msgTelaPublicando   = "Ainda publicando; chame de novo com esperar 30."
	msgTelaSemPermissao = "Você não tem permissão para publicar este app no Ziv. Quem cuida dele (ou um administrador) publica."
)

type publicarTelaIn struct {
	AppID   int64    `json:"app_id"`
	Ciente  []string `json:"ciente,omitempty" jsonschema:"os avisos que a pessoa disse estar ciente: file_data, windows_paths, secrets, container_limits"`
	Comando string   `json:"comando,omitempty" jsonschema:"o comando de início, só se a checagem pedir e a pessoa confirmar"`
	Esperar int      `json:"esperar,omitempty" jsonschema:"segundos para esperar o resultado, até 30"`
}

type telaOut struct {
	AppID    int64    `json:"app_id"`
	Situacao string   `json:"situacao"` // no_ar, erro, publicando, faltam_chaves, pendencias, esperando_aprovacao
	Endereco string   `json:"endereco"`
	Faltam   []string `json:"faltam"`
	Avisos   []string `json:"avisos"`
	Erro     string   `json:"erro"`
}

type sharedGet struct {
	Shared      *sharedState    `json:"shared"`
	Deployments []deploymentOut `json:"deployments"`
	LiveVersion *int32          `json:"live_version"`
}

// toolFailWith is toolFail that still gives Claude the structured answer.
func toolFailWith(text string, structured any) (*mcp.CallToolResult, any, error) {
	res, _, err := toolFail(text)
	return res, structured, err
}

// sharedFailure is the text for a failed /shared call: the off sentence when
// the company's screen apps are off, else the handler's own text.
func sharedFailure(status int, raw []byte, msg string) string {
	if status == http.StatusServiceUnavailable && bytes.Contains(raw, []byte(`"shared_runtime_unavailable"`)) {
		return msgWebOff
	}
	return msg
}

// buildingPublish is the newest publish still building, if any.
func buildingPublish(deps []deploymentOut) *deploymentOut {
	for i := range deps {
		if deps[i].Kind == "live" {
			if deps[i].Status == "building" {
				return &deps[i]
			}
			return nil
		}
	}
	return nil
}

func (h *Handler) toolPublicarTela(ctx context.Context, _ *mcp.CallToolRequest, in publicarTelaIn) (*mcp.CallToolResult, any, error) {
	if in.AppID == 0 {
		return toolFail("Mande em app_id o número do app (registrar_app devolve).")
	}
	if !h.sharedOn(ctx) {
		return toolFail(msgWebOff)
	}
	base := fmt.Sprintf("/api/apps/%d", in.AppID)
	out := telaOut{AppID: in.AppID, Faltam: []string{}, Avisos: []string{}}

	var d appDetail
	if _, _, msg := h.toolAPI(ctx, "GET", base, nil, &d); msg != "" {
		return toolFail(msg)
	}
	if d.Manifest.Kind != manifest.KindWeb {
		return toolFail("Este app não é um app com tela: não fica no ar num endereço. Rode com rodar_app.")
	}
	if !d.Can.Restore {
		return toolFail(msgTelaSemPermissao)
	}
	var st sharedGet
	if status, raw, msg := h.toolAPI(ctx, "GET", base+"/shared", nil, &st); msg != "" {
		return toolFail(sharedFailure(status, raw, msg))
	}
	// A repeat call while the publish is still building waits on it.
	if dep := buildingPublish(st.Deployments); dep != nil {
		return h.waitPublish(ctx, base, *dep, in.Esperar, out)
	}

	var first readiness.Report
	if status, raw, msg := h.toolAPI(ctx, "GET", base+"/shared/check", nil, &first); msg != "" {
		return toolFail(sharedFailure(status, raw, msg))
	}
	if msg, ok := h.saveTelaSettings(ctx, base, d, st, first, in); !ok {
		return toolFail(msg)
	}

	var rep readiness.Report
	if status, raw, msg := h.toolAPI(ctx, "GET", base+"/shared/check", nil, &rep); msg != "" {
		return toolFail(sharedFailure(status, raw, msg))
	}
	var missing, warns []string
	for _, it := range rep.Items {
		switch {
		case it.Status == "missing":
			line := it.Title + ": " + it.Detail
			if it.ClaudeText != "" {
				line += " Para resolver no código: " + it.ClaudeText
			}
			missing = append(missing, "- "+line)
		case it.Status == "warn" && !it.Acked:
			out.Avisos = append(out.Avisos, it.Key)
			warns = append(warns, fmt.Sprintf("- %s (%s): %s", it.Title, it.Key, it.Detail))
		}
	}
	if len(missing) > 0 {
		out.Situacao = "pendencias"
		return toolFailWith("A checagem do Ziv tem pendências:\n"+strings.Join(missing, "\n")+"\nNada foi publicado.", out)
	}
	if len(warns) > 0 {
		out.Situacao = "pendencias"
		return toolFailWith("A checagem do Ziv tem avisos:\n"+strings.Join(warns, "\n")+
			"\nPergunte à pessoa se está ciente de cada um; só então chame de novo com ciente. Nada foi publicado.", out)
	}

	var cfg []configItem
	if status, raw, msg := h.toolAPI(ctx, "GET", base+"/shared/config", nil, &cfg); msg != "" {
		return toolFail(sharedFailure(status, raw, msg))
	}
	for _, c := range cfg {
		if c.Required && !c.HasValue && c.Default == "" {
			out.Faltam = append(out.Faltam, c.Name)
		}
	}
	if len(out.Faltam) > 0 {
		out.Situacao = "faltam_chaves"
		return toolOK(fmt.Sprintf("Registrei, mas falta preencher no Ziv: %s. A pessoa preenche em %s (parte Compartilhar › Configurações) — nunca pela conversa. "+
			"Quando ela avisar, chame publicar_app_com_tela de novo.", strings.Join(out.Faltam, ", "), h.mcpLink("/apps/%d", in.AppID)), out)
	}

	var dep deploymentOut
	status, raw, msg := h.auditedAPI(ctx, "POST", base+"/shared/publish", "POST /api/apps/{id}/shared/publish", nil, &dep)
	switch {
	case msg == "":
	case status == http.StatusConflict && msg == busyMsg:
		// someone else (or the site) started one in between: wait on it
		var again sharedGet
		if _, _, m := h.toolAPI(ctx, "GET", base+"/shared", nil, &again); m != "" {
			return toolFail(m)
		}
		b := buildingPublish(again.Deployments)
		if b == nil {
			return toolFail(busyMsg)
		}
		dep = *b
	case status == http.StatusForbidden && strings.Contains(strings.ToLower(msg), "aprova"):
		out.Situacao = "esperando_aprovacao"
		return toolFailWith(msgTelaAprovacao, out)
	default:
		return toolFail(sharedFailure(status, raw, msg))
	}
	return h.waitPublish(ctx, base, dep, in.Esperar, out)
}

// saveTelaSettings saves the shared settings when they change: the settings
// the code reads, the confirmed start command and the "estou ciente" marks.
// Saving rewrites the ficha, so nothing is saved when nothing changes.
func (h *Handler) saveTelaSettings(ctx context.Context, base string, d appDetail, st sharedGet, first readiness.Report, in publicarTelaIn) (string, bool) {
	var cur manifest.Shared
	if d.Manifest.Shared != nil {
		cur = *d.Manifest.Shared
	}
	env := slices.Clone(cur.Env)
	for _, e := range first.SuggestEnv {
		if !slices.ContainsFunc(env, func(x manifest.EnvVar) bool { return x.Name == e.Name }) {
			env = append(env, e)
		}
	}
	command := cur.Command
	if c := strings.TrimSpace(in.Comando); c != "" {
		command = c
	}
	var acks []string
	if st.Shared != nil {
		acks = slices.Clone(st.Shared.Acks)
	}
	for _, k := range in.Ciente {
		k = strings.TrimSpace(k)
		if slices.Contains(knownAcks, k) && !slices.Contains(acks, k) {
			acks = append(acks, k)
		}
	}
	if st.Shared != nil && cur.Enabled && len(env) == len(cur.Env) && command == cur.Command && len(acks) == len(st.Shared.Acks) {
		return "", true
	}
	if acks == nil {
		acks = []string{}
	}
	body := SharedSettings{Enabled: true, AlwaysOn: cur.AlwaysOn, IdleMin: cur.IdleMin, MemoryMB: cur.MemoryMB, CPUs: cur.CPUs,
		Port: cur.Port, HealthPath: cur.HealthPath, Database: cur.Database, Command: command, Env: env, Acks: acks, UpdatedAt: d.UpdatedAt.Time}
	if status, raw, msg := h.auditedAPI(ctx, "PUT", base+"/shared", "PUT /api/apps/{id}/shared", body, nil); msg != "" {
		return sharedFailure(status, raw, msg), false
	}
	return "", true
}

// waitPublish follows one publish for up to esperar seconds (30 when not given).
func (h *Handler) waitPublish(ctx context.Context, base string, dep deploymentOut, esperar int, out telaOut) (*mcp.CallToolResult, any, error) {
	wait := mcpMaxWait
	if esperar != 0 {
		wait = min(max(esperar, 0), mcpMaxWait)
	}
	route := fmt.Sprintf("%s/shared/deployments/%d", base, dep.ID)
	deadline := time.Now().Add(time.Duration(wait) * time.Second)
	for dep.Status == "building" && time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return toolFail(msgTelaPublicando)
		case <-time.After(time.Second):
		}
		var next deploymentOut
		if _, _, msg := h.toolAPI(ctx, "GET", route, nil, &next); msg != "" {
			return toolFail(msg)
		}
		dep = next
	}
	switch dep.Status {
	case "live":
		var st sharedGet
		if _, _, msg := h.toolAPI(ctx, "GET", base+"/shared", nil, &st); msg != "" || st.Shared == nil {
			return toolFail(msgMCPAPIDown)
		}
		out.Situacao, out.Endereco = "no_ar", st.Shared.URL
		text := "No ar: " + st.Shared.URL
		if dep.VersionNumber != nil {
			text += fmt.Sprintf(" (versão %d)", *dep.VersionNumber)
		}
		return toolOK(text+".", out)
	case "failed":
		out.Situacao, out.Erro = "erro", dep.ErrorMessage
		erro := " Erro: " + strings.TrimSuffix(dep.ErrorMessage, ".") + "."
		var st sharedGet
		if _, _, msg := h.toolAPI(ctx, "GET", base+"/shared", nil, &st); msg == "" && st.LiveVersion != nil {
			return toolFailWith("A versão nova não abriu; a anterior continua no ar."+erro, out)
		}
		return toolFailWith("O app não abriu, então não está no ar."+erro, out)
	case "building":
		out.Situacao = "publicando"
		return toolOK(msgTelaPublicando, out)
	}
	return toolFail(fmt.Sprintf("A publicação terminou com a situação %q.", dep.Status))
}
