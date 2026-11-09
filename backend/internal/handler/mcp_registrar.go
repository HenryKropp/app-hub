package handler

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/HenryKropp/app-hub/backend/internal/bundle"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

// mcpProtocolo is the registration protocol Claude follows (formato_do_app).
//
//go:embed mcp_protocolo.md
var mcpProtocolo string

const (
	msgKindWindow    = "Apps de janela e .exe ainda não sobem pelo Claude: eles não rodam no servidor. Registre pelo site, em Registrar apps."
	msgKindScheduled = "Registre como script e agende pelo Ziv, na ficha do app."
	msgKindSiteOnly  = "Esse tipo só é registrado pelo site."
	msgEnvioNotFound = "Não achei esse envio. Chame preparar_envio de novo."
	msgEnvioUsed     = "Esse envio já foi usado ou venceu. Chame preparar_envio de novo."
	msgEnvioWaiting  = "O Ziv ainda não recebeu o zip desse envio. Rode o comando curl que preparar_envio mostrou e confira se ele respondeu \"recebido\"."
	msgSiteOnly      = "Não mudei nome/área/responsável: isso só muda pelo site."
)

// kindRefusal is why an app of this kind cannot come through Claude, or "".
func kindRefusal(kind, entrypoint string) string {
	if strings.HasSuffix(strings.ToLower(entrypoint), ".exe") {
		return msgKindWindow
	}
	switch kind {
	case manifest.KindScript, manifest.KindWeb:
		return ""
	case manifest.KindDesktop, manifest.KindExecutable:
		return msgKindWindow
	case manifest.KindScheduled:
		return msgKindScheduled
	}
	return msgKindSiteOnly
}

// ---- formato_do_app ----

func (h *Handler) toolFormatoDoApp(ctx context.Context, _ *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, any, error) {
	var areas []areaJSON
	if _, _, msg := h.toolAPI(ctx, "GET", "/api/areas", nil, &areas); msg != "" {
		return toolFail(msg)
	}
	names := make([]string, 0, len(areas))
	for _, a := range areas {
		names = append(names, a.Name)
	}
	text := mcpProtocolo + "\n## Desta empresa\n\nÁreas do Ziv: " + strings.Join(names, ", ") + ".\n" +
		"Limites do pacote: até 100 arquivos e 10 MB por app (o que fica de fora não conta).\n"
	return toolOK(text, map[string]any{
		"protocolo": mcpProtocolo, "areas": names, "tipos": []string{manifest.KindScript, manifest.KindWeb},
		"limites": map[string]int{"arquivos": bundle.SiteLimits.MaxFiles, "mb": bundle.SiteLimits.MaxTotal >> 20},
	})
}

// ---- registrar_app ----

type registrarIn struct {
	Envio             int64          `json:"envio" jsonschema:"o número que preparar_envio devolveu, depois do curl"`
	AppID             int64          `json:"app_id,omitempty" jsonschema:"para mandar versão nova de um app que já existe"`
	Manifesto         map[string]any `json:"manifesto,omitempty" jsonschema:"o manifesto do app, no formato de formato_do_app; obrigatório para app novo; numa versão nova, só resumo, entradas, saídas e (scripts) arquivo de início e dependências mudam"`
	Nota              string         `json:"nota,omitempty" jsonschema:"o que mudou nesta versão"`
	ConfirmarSegredos bool           `json:"confirmar_segredos,omitempty" jsonschema:"true só depois que a pessoa confirmar"`
}

// sentAnswer is what /api/scripts/import and /api/apps/{id}/sources answer.
type sentAnswer struct {
	ID            int64  `json:"id"`
	Version       int32  `json:"version"`
	Changed       *bool  `json:"changed"`
	BlockedReason string `json:"blocked_reason"`
}

// envioFiles opens the person's upload; msg is the refusal for Claude.
func (h *Handler) envioFiles(ctx context.Context, envio int64) (sqlc.McpUpload, bundle.Result, string) {
	up, err := h.q.GetMCPUpload(ctx, sqlc.GetMCPUploadParams{ID: envio, PersonID: mcpPersonFrom(ctx)})
	if errors.Is(err, pgx.ErrNoRows) {
		return up, bundle.Result{}, msgEnvioNotFound
	}
	if err != nil {
		slog.Error("mcp get upload", "err", err)
		return up, bundle.Result{}, msgMCPAPIDown
	}
	if up.UsedAt.Valid || !up.ExpiresAt.Time.After(time.Now()) {
		return up, bundle.Result{}, msgEnvioUsed
	}
	if !up.ReceivedAt.Valid {
		return up, bundle.Result{}, msgEnvioWaiting
	}
	var st storedEnvio
	if err := json.Unmarshal(up.Content, &st); err != nil {
		return up, bundle.Result{}, "Não consegui ler este envio. Prepare um envio novo."
	}
	return up, bundle.Result{Files: st.Files, Dropped: st.Dropped}, ""
}

func inputFiles(res bundle.Result) map[string]manifest.InputFile {
	files := make(map[string]manifest.InputFile, len(res.Files))
	for name, data := range res.Files {
		files[name] = manifest.InputFile{Name: path.Base(name), ContentBase64: base64.StdEncoding.EncodeToString(data)}
	}
	return files
}

func missingEntrypoint(entry string, res bundle.Result) string {
	if entry == "" {
		return ""
	}
	if _, ok := res.Files[entry]; ok {
		return ""
	}
	return fmt.Sprintf("O arquivo de início %s não está no pacote. Confira se o zip foi feito de dentro da pasta do app (tar com -C \"<pasta>\" .).", entry)
}

// sentFailure turns an import/sources refusal into the text for Claude.
func sentFailure(status int, raw []byte, msg string) string {
	var e struct {
		Code        string       `json:"code"`
		Error       string       `json:"error"`
		SecretCount int          `json:"secret_count"`
		Findings    []secretSpot `json:"findings"`
	}
	_ = json.Unmarshal(raw, &e)
	switch {
	case status == http.StatusConflict && e.Code == "script_secrets_confirmation_required":
		return fmt.Sprintf("O Ziv achou %d trecho(s) no código que parecem senhas ou chaves:", e.SecretCount) + spotLines(e.Findings) +
			"\nNada foi mandado. Veja cada linha: se for uma senha ou chave de verdade, troque o valor pela leitura do ambiente (os.environ[\"NOME\"]), liste o NOME em keys na ficha (a pessoa preenche o valor nas Configurações do app, nunca pela conversa) e prepare um envio novo; " +
			"se não for (por exemplo, o nome de um campo), pergunte à pessoa se quer mandar mesmo assim; só se ela confirmar, chame registrar_app de novo com o mesmo envio e confirmar_segredos: true. Nunca mostre os valores na conversa."
	case status == http.StatusUnprocessableEntity && e.Code == "script_secrets_blocked":
		return e.Error + spotLines(e.Findings) +
			"\nVeja cada linha: troque cada senha ou chave pela leitura do ambiente (os.environ[\"NOME\"]) e liste o NOME em keys na ficha (a pessoa preenche o valor nas Configurações do app, nunca pela conversa); se a linha não tem senha, reescreva para não parecer uma (por exemplo, sem atribuir a uma variável chamada token ou senha). Depois prepare um envio novo. Nunca mostre os valores na conversa."
	case status == http.StatusConflict && e.Code == "no_stored_code":
		return e.Error
	case status == http.StatusBadRequest:
		return msg + " Nada foi mandado."
	}
	return msg
}

// fichaChanges builds the ficha update from the fields Claude sent that
// differ from the current ficha, and lists the site-only fields it ignored.
func fichaChanges(cur manifest.Manifest, sent map[string]any, next manifest.Manifest) (*fichaUpdate, []string, []string) {
	var f fichaUpdate
	var changed, ignored []string
	has := func(k string) bool { _, ok := sent[k]; return ok }
	if has("summary") && strings.TrimSpace(next.Summary) != cur.Summary {
		f.Summary = &next.Summary
		changed = append(changed, "resumo")
	}
	if has("inputs") && !reflect.DeepEqual(normInputs(next.Inputs), normInputs(cur.Inputs)) {
		f.Inputs = &next.Inputs
		changed = append(changed, "entradas")
	}
	if has("outputs") && !reflect.DeepEqual(normJSON(next.Outputs), normJSON(cur.Outputs)) {
		f.Outputs = &next.Outputs
		changed = append(changed, "saídas")
	}
	if cur.Kind != manifest.KindWeb {
		if has("entrypoint") && next.Entrypoint != cur.Entrypoint {
			f.Entrypoint = &next.Entrypoint
			changed = append(changed, "arquivo de início")
		}
		if has("runtime") && !reflect.DeepEqual(normJSON(next.Runtime), normJSON(cur.Runtime)) {
			f.Runtime = &next.Runtime
			changed = append(changed, "dependências")
		}
		if has("keys") && !reflect.DeepEqual(normJSON(next.Keys), normJSON(cur.Keys)) {
			f.Keys = &next.Keys
			changed = append(changed, "chaves e senhas")
		}
	}
	if has("name") && strings.TrimSpace(next.Name) != cur.Name {
		ignored = append(ignored, "nome")
	}
	if has("area") && foldText(strings.TrimSpace(next.Area)) != foldText(cur.Area) {
		ignored = append(ignored, "área")
	}
	if has("owner") && next.Owner != nil && (cur.Owner == nil || !strings.EqualFold(next.Owner.Email, cur.Owner.Email)) {
		ignored = append(ignored, "responsável")
	}
	if has("backup") && ((next.Backup == nil) != (cur.Backup == nil) || next.Backup != nil && cur.Backup != nil && !strings.EqualFold(next.Backup.Email, cur.Backup.Email)) {
		ignored = append(ignored, "reserva")
	}
	if len(changed) == 0 {
		return nil, nil, ignored
	}
	return &f, changed, ignored
}

func normInputs(in []manifest.Input) any {
	if len(in) == 0 {
		return nil
	}
	return normJSON(in)
}

// normJSON compares values as their JSON (nil and empty lists alike).
func normJSON(v any) any {
	raw, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(raw, &out)
	return dropEmpty(out)
}

func dropEmpty(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			if e = dropEmpty(e); e == nil {
				delete(x, k)
			} else {
				x[k] = e
			}
		}
		if len(x) == 0 {
			return nil
		}
	case []any:
		if len(x) == 0 {
			return nil
		}
		for i := range x {
			x[i] = dropEmpty(x[i])
		}
	case string:
		if x == "" {
			return nil
		}
	case bool:
		if !x {
			return nil
		}
	}
	return v
}

func (h *Handler) toolRegistrarApp(ctx context.Context, _ *mcp.CallToolRequest, in registrarIn) (*mcp.CallToolResult, any, error) {
	if in.Envio == 0 {
		return toolFail("Mande em envio o número que preparar_envio devolveu.")
	}
	var sent manifest.Manifest
	if len(in.Manifesto) > 0 {
		raw, _ := json.Marshal(in.Manifesto)
		if err := json.Unmarshal(raw, &sent); err != nil {
			return toolFail("Não consegui ler o manifesto. Siga o formato de formato_do_app.")
		}
	}
	if in.AppID == 0 && len(in.Manifesto) == 0 {
		return toolFail("Para um app novo, mande o manifesto em manifesto, no formato de formato_do_app.")
	}
	if in.AppID == 0 {
		if msg := kindRefusal(sent.Kind, sent.Entrypoint); msg != "" {
			return toolFail(msg)
		}
	}
	up, res, msg := h.envioFiles(ctx, in.Envio)
	if msg != "" {
		return toolFail(msg)
	}

	var (
		ans     sentAnswer
		name    string
		changed []string
		ignored []string
		web     bool
	)
	if in.AppID == 0 {
		if msg := missingEntrypoint(sent.Entrypoint, res); msg != "" {
			return toolFail(msg)
		}
		body := map[string]any{"manifest": sent, "files": inputFiles(res), "confirm_secrets": in.ConfirmarSegredos}
		status, raw, msg := h.auditedAPI(ctx, "POST", "/api/scripts/import", "POST /api/scripts/import", body, &ans)
		if msg != "" {
			return toolFail(sentFailure(status, raw, msg))
		}
		name = strings.TrimSpace(sent.Name)
		web = sent.Kind == manifest.KindWeb
	} else {
		var d appDetail
		if _, _, msg := h.toolAPI(ctx, "GET", fmt.Sprintf("/api/apps/%d", in.AppID), nil, &d); msg != "" {
			return toolFail(msg)
		}
		cur := d.Manifest
		body := map[string]any{"files": inputFiles(res), "confirm_secrets": in.ConfirmarSegredos, "note": in.Nota}
		entry := cur.Entrypoint
		if len(in.Manifesto) > 0 {
			var f *fichaUpdate
			f, changed, ignored = fichaChanges(cur, in.Manifesto, sent)
			if f != nil {
				body["ficha"], body["updated_at"] = f, d.UpdatedAt.Time
				if f.Entrypoint != nil {
					entry = *f.Entrypoint
				}
			}
		}
		if d.StoredCode {
			if msg := missingEntrypoint(entry, res); msg != "" {
				return toolFail(msg)
			}
		}
		status, raw, msg := h.auditedAPI(ctx, "POST", fmt.Sprintf("/api/apps/%d/sources", in.AppID), "POST /api/apps/{id}/sources", body, &ans)
		if msg != "" {
			return toolFail(sentFailure(status, raw, msg))
		}
		name = cur.Name
		web = cur.Kind == manifest.KindWeb
	}
	if err := h.q.UseMCPUpload(ctx, up.ID); err != nil {
		slog.Error("mcp use upload", "err", err)
	}

	mudou := ans.Changed == nil || *ans.Changed
	link := h.mcpLink("/apps/%d", ans.ID)
	var b strings.Builder
	switch {
	case in.AppID == 0:
		fmt.Fprintf(&b, "Registrei o app %q (versão %d). Link: %s", name, ans.Version, link)
	case mudou:
		fmt.Fprintf(&b, "Mandei a versão %d do %q. Link: %s", ans.Version, name, link)
	default:
		fmt.Fprintf(&b, "O código é igual ao da versão %d; nada mudou. Link: %s", ans.Version, link)
	}
	if len(changed) > 0 {
		b.WriteString("\nMudei na ficha: " + strings.Join(changed, ", ") + ".")
	}
	if len(ignored) > 0 {
		b.WriteString("\n" + msgSiteOnly)
	}
	dropped := res.Dropped
	if dropped == nil {
		dropped = []string{}
	}
	if len(dropped) > 0 {
		b.WriteString("\nDeixei de fora: " + strings.Join(dropped, ", ") + " (senhas e pastas que não sobem).")
		for _, d := range dropped {
			if !strings.HasSuffix(d, "/") {
				fmt.Fprintf(&b, " Se o app precisa dessas chaves, ele deve ler cada uma do ambiente (os.environ[\"NOME\"]) e a ficha pode listar os nomes em keys. Para guardar os valores do .env nas Configurações do app, chame enviar_chaves com app_id %d e rode o curl (sem abrir o arquivo); nunca mande os valores pela conversa.", ans.ID)
				break
			}
		}
	}
	waiting := ans.BlockedReason != ""
	if waiting {
		if strings.Contains(strings.ToLower(ans.BlockedReason), "aprova") {
			b.WriteString("\nEstá esperando a aprovação de um administrador; só depois dá para rodar.")
		} else {
			b.WriteString("\n" + ans.BlockedReason)
		}
	}
	if web && (in.AppID == 0 || mudou) {
		fmt.Fprintf(&b, "\nAgora chame publicar_app_com_tela com app_id %d.", ans.ID)
	}
	missing := []string{}
	if a, err := h.q.GetApp(ctx, ans.ID); err == nil && len(manifestOf(a).Keys) > 0 {
		keys, err := h.appKeys(ctx, a)
		if err == nil {
			missing = keys.Missing
			var names []string
			for _, k := range keys.Keys {
				names = append(names, k.Name)
			}
			fmt.Fprintf(&b, "\nO app lê das Configurações do app: %s.", strings.Join(names, ", "))
			if len(missing) > 0 {
				fmt.Fprintf(&b, " Falta preencher: %s. Se estão no .env da pasta, use enviar_chaves; senão, o dono do app ou um administrador preenche em %s, nunca pela conversa. Até lá o app não abre.",
					strings.Join(missing, ", "), h.mcpLink("/apps/%d#chaves", ans.ID))
			}
		}
	}
	return toolOK(b.String(), map[string]any{
		"app_id": ans.ID, "versao": ans.Version, "mudou": mudou, "link": link,
		"esperando_aprovacao": waiting, "deixados_de_fora": dropped, "chaves_faltando": missing,
	})
}

// spotLines lists where the probable secrets are, one per line, without values.
func spotLines(spots []secretSpot) string {
	var b strings.Builder
	for _, p := range spots {
		fmt.Fprintf(&b, "\n- %s, linha %d (%s)", p.File, p.Line, p.Name)
	}
	return b.String()
}
