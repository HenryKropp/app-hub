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
	msgTelaAprovacao  = "Esperando a aprovação de um administrador; publico depois que aprovarem."
	msgTelaPublicando = "Ainda publicando; chame de novo com esperar 30."
	// msgTelaLerLog: after a failure, read the log before sending anything again.
	msgTelaLerLog       = "Antes de mandar outra versão, leia o log acima e corrija a causa no código. Não publique de novo sem mudar nada: o resultado seria o mesmo."
	msgTelaSemPermissao = "Você não tem permissão para publicar este app no Ziv. Quem cuida dele (ou um administrador) publica."
)

type publicarTelaIn struct {
	AppID   int64    `json:"app_id"`
	Ciente  []string `json:"ciente,omitempty" jsonschema:"os avisos que a pessoa disse estar ciente: file_data, windows_paths, secrets, container_limits"`
	Comando string   `json:"comando,omitempty" jsonschema:"o comando de início, só se a checagem pedir e a pessoa confirmar"`
	Esperar int      `json:"esperar,omitempty" jsonschema:"segundos para esperar o resultado, até 30"`
	// MCP F4: how to check a new version before it takes the place of the one in the air.
	ConferirCaminho *string `json:"conferir_caminho,omitempty" jsonschema:"a página que o Ziv abre para conferir a versão nova, começando com / (padrão /)"`
	ConferirTexto   *string `json:"conferir_texto,omitempty" jsonschema:"um texto que essa página sempre mostra quando está certa; vazio deixa de conferir. Não serve para Streamlit (a página só aparece com JavaScript)"`
}

type telaOut struct {
	AppID    int64    `json:"app_id"`
	Situacao string   `json:"situacao"` // no_ar, erro, publicando, faltam_chaves, pendencias, esperando_aprovacao
	Endereco string   `json:"endereco"`
	Faltam   []string `json:"faltam"`
	Avisos   []string `json:"avisos"`
	Erro     string   `json:"erro"`
	// MCP F1: whether it is really in the air, which version runs, the stage
	// while publishing and the end of the log when it fails.
	NoAr       bool   `json:"no_ar"`
	VersaoNoAr *int32 `json:"versao_no_ar,omitempty"`
	Etapa      string `json:"etapa,omitempty"`
	Segundos   int    `json:"segundos,omitempty"`
	Log        string `json:"log,omitempty"`
	// MCP D: a ready sentence for the person when the publish ends.
	MensagemParaPessoa string `json:"mensagem_para_pessoa,omitempty"`
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
	var cfg []configItem
	if status, raw, msg := h.toolAPI(ctx, "GET", base+"/shared/config", nil, &cfg); msg != "" {
		return sharedFailure(status, raw, msg), false
	}
	hasValue := map[string]bool{}
	for _, c := range cfg {
		hasValue[c.Name] = c.HasValue
	}
	env := mergeEnv(cur.Env, first.EnvReads, d.Manifest.Keys, hasValue)
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
	healthPath, checkText := cur.HealthPath, cur.CheckText
	if in.ConferirCaminho != nil {
		healthPath = strings.TrimSpace(*in.ConferirCaminho)
	}
	if in.ConferirTexto != nil {
		checkText = strings.TrimSpace(*in.ConferirTexto)
	}
	if st.Shared != nil && cur.Enabled && slices.Equal(env, cur.Env) && command == cur.Command && len(acks) == len(st.Shared.Acks) &&
		healthPath == cur.HealthPath && checkText == cur.CheckText {
		return "", true
	}
	if acks == nil {
		acks = []string{}
	}
	body := SharedSettings{Enabled: true, AlwaysOn: cur.AlwaysOn, IdleMin: cur.IdleMin, MemoryMB: cur.MemoryMB, CPUs: cur.CPUs,
		Port: cur.Port, HealthPath: healthPath, Database: cur.Database, Command: command, Env: env, CheckText: checkText, Acks: acks, UpdatedAt: d.UpdatedAt.Time}
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
	var st sharedGet
	stOK := false
	if dep.Status != "building" {
		_, _, msg := h.toolAPI(ctx, "GET", base+"/shared", nil, &st)
		stOK = msg == ""
		if stOK {
			out.NoAr, _ = telaNoAr(st)
			out.VersaoNoAr = st.LiveVersion
		}
	}
	switch dep.Status {
	case "live":
		if !stOK || st.Shared == nil {
			return toolFail(msgMCPAPIDown)
		}
		out.Situacao, out.Endereco = "no_ar", st.Shared.URL
		out.MensagemParaPessoa = msgPessoaTela(true, st.Shared.URL, nil)
		text := "No ar: " + st.Shared.URL
		if dep.VersionNumber != nil {
			text += fmt.Sprintf(" (versão %d)", *dep.VersionNumber)
		}
		return toolOK(text+".", out)
	case "failed":
		out.Situacao, out.Erro = "erro", dep.ErrorMessage
		what := "A publicação falhou"
		if dep.VersionNumber != nil {
			what = fmt.Sprintf("A publicação da versão %d falhou", *dep.VersionNumber)
		}
		parts := []string{what + ": " + strings.TrimSuffix(strings.TrimSpace(dep.ErrorMessage), ".") + "."}
		switch {
		case !stOK:
		case out.NoAr && st.LiveVersion != nil:
			parts = append(parts, fmt.Sprintf("Nada mudou para quem usa: a versão %d continua no ar.", *st.LiveVersion))
		default:
			_, frase := telaNoAr(st)
			parts = append(parts, "O app não está no ar. "+frase)
		}
		if dep.BuildLog != nil {
			if t := tailLines(*dep.BuildLog, logLinesMCP); t != "" {
				out.Log = t
				parts = append(parts, "Fim do log:\n"+fenced(t))
			}
		}
		if stOK && out.NoAr {
			out.MensagemParaPessoa = msgPessoaTela(false, "", st.LiveVersion)
		} else {
			out.MensagemParaPessoa = msgPessoaTela(false, "", nil)
		}
		parts = append(parts, msgTelaLerLog)
		return toolFailWith(strings.Join(parts, "\n\n"), out)
	case "building":
		out.Situacao, out.Etapa, out.Segundos = "publicando", dep.Stage, dep.Seconds
		text := "Ainda publicando"
		if dep.Stage != "" {
			text += ": " + dep.Stage
		}
		if dep.Seconds > 0 {
			text += fmt.Sprintf(" (%d s desde o início)", dep.Seconds)
		}
		return toolOK(text+". Chame de novo com esperar 30.", out)
	}
	return toolFail(fmt.Sprintf("A publicação terminou com a situação %q.", dep.Status))
}

// mergeEnv rebuilds the settings list from what the code reads now (MCP F2):
//   - a setting the code no longer reads, without a value, a default or a
//     place in the ficha's keys, leaves the list;
//   - one the code reads is required only if a read has no default;
//   - the ficha's keys (sent by Claude with the version) have the last word
//     on label and required;
//   - a new name the code reads joins the list.
func mergeEnv(cur []manifest.EnvVar, reads map[string]bool, keys []manifest.Key, hasValue map[string]bool) []manifest.EnvVar {
	key := map[string]manifest.Key{}
	for _, k := range keys {
		key[k.Name] = k
	}
	out := []manifest.EnvVar{}
	seen := map[string]bool{}
	for _, e := range cur {
		required, read := reads[e.Name]
		k, inKeys := key[e.Name]
		if !read && !inKeys && !hasValue[e.Name] && e.Default == "" {
			continue
		}
		if read {
			e.Required = required
		}
		if inKeys {
			e.Required = k.Required
			if k.Label != "" {
				e.Label = k.Label
			}
		}
		seen[e.Name] = true
		out = append(out, e)
	}
	var added []string
	for n := range reads {
		if !seen[n] && !slices.Contains(manifest.ReservedEnv, n) {
			added = append(added, n)
		}
	}
	slices.Sort(added)
	for _, n := range added {
		e := manifest.EnvVar{Name: n, Label: n, Secret: looksSecret(n, ""), Required: reads[n]}
		if k, ok := key[n]; ok {
			e.Required = k.Required
			if k.Label != "" {
				e.Label = k.Label
			}
		}
		out = append(out, e)
	}
	return out
}
