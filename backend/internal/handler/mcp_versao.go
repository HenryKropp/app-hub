package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/HenryKropp/app-hub/backend/internal/bundle"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/testplan"
)

// Etapa 2 (ADR 044), the Claude side: a script version goes to test on the
// PC and only goes live when the test passes. Confirming the test plan is
// never a tool: it is done on the site by the owner or an admin.

const (
	msgVersaoTela      = "Apps com tela ainda usam registrar_app e publicar_app_com_tela; enviar_versao chega para eles na próxima etapa."
	msgRegistrarScript = "Scripts agora vão por enviar_versao, que testa antes de pôr no ar. Escreva a seção teste da receita (veja receita_app_ziv) e chame enviar_versao com o mesmo envio."
	mcpTextFileLimit   = 200 << 10
	mcpLogShown        = 6000
)

type enviarVersaoIn struct {
	Envio             int64          `json:"envio,omitempty" jsonschema:"o número do envio, depois que o curl mostrou recebido; sem ele, a ferramenta prepara o envio e devolve os comandos"`
	AppID             int64          `json:"app_id,omitempty" jsonschema:"para mandar versão nova de um app que já existe"`
	Manifesto         map[string]any `json:"manifesto,omitempty" jsonschema:"o manifesto no formato de formato_do_app, com a seção teste; obrigatório para app novo; numa versão nova, só os campos que mudam (resumo, entradas, saídas, arquivo de início, dependências, teste)"`
	Nota              string         `json:"nota,omitempty" jsonschema:"o que mudou nesta versão"`
	ConfirmarSegredos bool           `json:"confirmar_segredos,omitempty" jsonschema:"true só depois que a pessoa confirmar"`
	Versao            int64          `json:"versao,omitempty" jsonschema:"só com app_id e sem envio: começa o teste de uma versão já mandada que não começou (por exemplo, depois de guardar as chaves)"`
}

// draftAnswer is GET /api/apps/{id}/drafts/{d}.
type draftAnswer struct {
	ID            int64            `json:"id"`
	Status        string           `json:"status"`
	StatusReason  string           `json:"status_reason"`
	VersionNumber *int32           `json:"version_number"`
	Stage         string           `json:"stage"`
	StageLabel    string           `json:"stage_label"`
	Checks        []testplan.Check `json:"checks"`
	ExitCode      *int32           `json:"exit_code"`
	Log           string           `json:"log"`
	Files         []testFile       `json:"files"`
	ConfirmURL    string           `json:"confirm_url"`
}

// draftSituacao says a draft's status in words.
func draftSituacao(d draftAnswer) string {
	switch d.Status {
	case "rascunho":
		return "rascunho, o teste não começou"
	case "testando":
		return "em teste"
	case "aprovada":
		return "aprovada, indo para o ar"
	case "reprovada":
		return "reprovada"
	case "no_ar":
		return "no ar"
	case "cancelada":
		return "cancelada"
	}
	return d.Status
}

func draftLive(status string) bool { return status == "testando" || status == "aprovada" }

func (h *Handler) toolEnviarVersao(ctx context.Context, req *mcp.CallToolRequest, in enviarVersaoIn) (*mcp.CallToolResult, any, error) {
	if in.Versao != 0 {
		if in.AppID == 0 || in.Envio != 0 {
			return toolFail("Com versao, mande só app_id e versao (sem envio).")
		}
		var cur appDetail
		if _, _, msg := h.toolAPI(ctx, "GET", fmt.Sprintf("/api/apps/%d", in.AppID), nil, &cur); msg != "" {
			return toolFail(msg)
		}
		return h.startVersaoTest(ctx, in.AppID, in.Versao, cur.Manifest.Name, false, nil, nil, bundle.Result{})
	}
	if in.AppID == 0 && len(in.Manifesto) == 0 {
		return toolFail("Para um app novo, mande o manifesto em manifesto, no formato de formato_do_app, com a seção teste.")
	}
	var sent manifest.Manifest
	if len(in.Manifesto) > 0 {
		raw, _ := json.Marshal(in.Manifesto)
		if err := json.Unmarshal(raw, &sent); err != nil {
			return toolFail("Não consegui ler o manifesto. Siga o formato de formato_do_app.")
		}
	}
	// Refusals that need no files come before the envio is prepared or used.
	var cur appDetail
	kind, entry := sent.Kind, sent.Entrypoint
	if in.AppID != 0 {
		if _, _, msg := h.toolAPI(ctx, "GET", fmt.Sprintf("/api/apps/%d", in.AppID), nil, &cur); msg != "" {
			return toolFail(msg)
		}
		kind, entry = cur.Manifest.Kind, cur.Manifest.Entrypoint
	}
	if kind == "" {
		kind = manifest.KindScript
	}
	if kind == manifest.KindWeb {
		return toolFail(msgVersaoTela)
	}
	if msg := kindRefusal(kind, entry); msg != "" {
		return toolFail(msg)
	}
	if in.AppID == 0 && sent.Teste == nil {
		return toolFail(msgTestPlanMissing)
	}
	var ficha *fichaUpdate
	var changed, ignored []string
	if in.AppID != 0 && len(in.Manifesto) > 0 {
		ficha, changed, ignored = fichaChanges(cur.Manifest, in.Manifesto, sent)
	}
	if in.AppID != 0 && cur.Manifest.Teste == nil && (ficha == nil || ficha.Teste == nil) {
		return toolFail(msgTestPlanMissing)
	}

	if in.Envio == 0 {
		res, out, err := h.toolPrepararEnvio(ctx, req, emptyIn{})
		if err != nil || res.IsError {
			return res, out, err
		}
		id := out.(map[string]any)["envio"]
		text := res.Content[0].(*mcp.TextContent).Text
		cut := strings.Index(text, "Quando o curl mostrar")
		if cut < 0 {
			cut = len(text)
		}
		text = text[:cut] + fmt.Sprintf("Quando o curl mostrar \"recebido\": true, chame enviar_versao de novo com envio %d, com os mesmos app_id e manifesto, e apague o .zip temporário.", id)
		return toolOK(text, out)
	}

	up, res, msg := h.envioFiles(ctx, in.Envio)
	if msg != "" {
		return toolFail(msg)
	}
	if ficha != nil && ficha.Entrypoint != nil {
		entry = *ficha.Entrypoint
	}
	if in.AppID == 0 || cur.StoredCode {
		if msg := missingEntrypoint(entry, res); msg != "" {
			return toolFail(msg)
		}
	}
	files := inputFiles(res)
	appID, name := in.AppID, cur.Manifest.Name
	if in.AppID == 0 {
		var ans sentAnswer
		body := map[string]any{"manifest": sent, "files": files, "confirm_secrets": in.ConfirmarSegredos, "in_test": true}
		status, raw, msg := h.auditedAPI(ctx, "POST", "/api/scripts/import", "POST /api/scripts/import", body, &ans)
		if msg != "" {
			return toolFail(versaoFailure(status, raw, msg))
		}
		appID, name = ans.ID, strings.TrimSpace(sent.Name)
	}
	body := map[string]any{"files": files, "confirm_secrets": in.ConfirmarSegredos, "note": in.Nota}
	if ficha != nil {
		body["ficha"] = ficha
	}
	var created struct {
		DraftID int64 `json:"draft_id"`
	}
	status, raw, msg := h.auditedAPI(ctx, "POST", fmt.Sprintf("/api/apps/%d/drafts", appID), "POST /api/apps/{id}/drafts", body, &created)
	if msg != "" {
		return toolFail(versaoFailure(status, raw, msg))
	}
	if err := h.q.UseMCPUpload(ctx, up.ID); err != nil {
		slog.Error("mcp use upload", "err", err)
	}
	var b strings.Builder
	if in.AppID == 0 {
		fmt.Fprintf(&b, "Registrei o app %q em teste (versão %d). Ele só aparece para a equipe depois que o primeiro teste passar.", name, created.DraftID)
	} else {
		fmt.Fprintf(&b, "Mandei a versão %d do %q para teste. A versão no ar continua a mesma até o teste passar.", created.DraftID, name)
	}
	if len(changed) > 0 {
		b.WriteString("\nMuda na ficha: " + strings.Join(changed, ", ") + ".")
	}
	if len(ignored) > 0 {
		b.WriteString("\n" + msgSiteOnly)
	}
	return h.startVersaoTest(ctx, appID, created.DraftID, name, true, &b, nil, res)
}

// startVersaoTest starts the test of draft d and says the next step. head is
// what was already said about the sending.
func (h *Handler) startVersaoTest(ctx context.Context, appID, draftID int64, name string, sentNow bool, head *strings.Builder, _ any, res bundle.Result) (*mcp.CallToolResult, any, error) {
	var b strings.Builder
	if head != nil {
		b.WriteString(head.String())
	}
	missing := h.envioNotes(ctx, &b, res, appID)
	var d draftAnswer
	status, raw, msg := h.auditedAPI(ctx, "POST", fmt.Sprintf("/api/apps/%d/drafts/%d/test", appID, draftID), "POST /api/apps/{id}/drafts/{id}/test", nil, &d)
	if msg != "" {
		var e struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(raw, &e)
		if e.Code == "keys_missing" {
			// Not an error: the version is kept and waits for the keys.
			fmt.Fprintf(&b, "\nO teste ainda não começou: falta guardar %s. Depois de guardar (enviar_chaves, ou o dono do app no site), chame enviar_versao com app_id %d e versao %d.",
				strings.Join(missing, ", "), appID, draftID)
			return toolOK(b.String(), map[string]any{"app_id": appID, "versao": draftID, "situacao": "rascunho, o teste não começou", "etapa": "", "confirmar": "", "chaves_faltando": missing})
		}
		if sentNow {
			return toolFail(b.String() + fmt.Sprintf("\nO teste não começou: %s Corrija e chame enviar_versao de novo (com um envio novo).", msg))
		}
		_ = status
		return toolFail("O teste não começou: " + msg)
	}

	confirm := ""
	if d.ConfirmURL != "" {
		confirm = h.mcpLink("%s", d.ConfirmURL)
	}
	if !sentNow {
		fmt.Fprintf(&b, "Comecei o teste da versão %d do %q.", d.ID, name)
	}
	if confirm != "" {
		fmt.Fprintf(&b, "\nMande este link ao responsável pelo app para confirmar o plano de teste: %s. Depois acompanhe com ver_versao.", confirm)
	} else {
		fmt.Fprintf(&b, "\nAcompanhe com ver_versao (app_id %d, versao %d, esperar 30).", appID, d.ID)
	}
	return toolOK(b.String(), map[string]any{
		"app_id": appID, "versao": d.ID, "situacao": draftSituacao(d), "etapa": d.StageLabel, "confirmar": confirm, "chaves_faltando": missing,
	})
}

// versaoFailure is sentFailure with this tool's name in the next step.
func versaoFailure(status int, raw []byte, msg string) string {
	return strings.ReplaceAll(sentFailure(status, raw, msg), "registrar_app", "enviar_versao")
}

// envioNotes tells what stayed out of the package and which keys the app
// reads; it returns the keys still empty.
func (h *Handler) envioNotes(ctx context.Context, b *strings.Builder, res bundle.Result, appID int64) []string {
	if len(res.Dropped) > 0 {
		b.WriteString("\nDeixei de fora: " + strings.Join(res.Dropped, ", ") + " (senhas e pastas que não sobem).")
		for _, d := range res.Dropped {
			if !strings.HasSuffix(d, "/") {
				fmt.Fprintf(b, " Se o app precisa dessas chaves, ele deve ler cada uma do ambiente (os.environ[\"NOME\"]) e a ficha pode listar os nomes em keys. Para guardar os valores do .env nas Configurações do app, chame enviar_chaves com app_id %d e rode o curl (sem abrir o arquivo); nunca mande os valores pela conversa.", appID)
				break
			}
		}
	}
	missing := []string{}
	a, err := h.q.GetApp(ctx, appID)
	if err != nil || len(manifestOf(a).Keys) == 0 {
		return missing
	}
	keys, err := h.appKeys(ctx, a)
	if err != nil {
		return missing
	}
	var names []string
	for _, k := range keys.Keys {
		names = append(names, k.Name)
	}
	fmt.Fprintf(b, "\nO app lê das Configurações do app: %s.", strings.Join(names, ", "))
	if len(keys.Missing) > 0 {
		fmt.Fprintf(b, " Falta preencher: %s. Se estão no .env da pasta, use enviar_chaves; senão, o dono do app ou um administrador preenche em %s, nunca pela conversa. O teste não roda sem elas.",
			strings.Join(keys.Missing, ", "), h.mcpLink("/apps/%d#chaves", appID))
		missing = keys.Missing
	}
	return missing
}

// ---- ver_versao ----

type verVersaoIn struct {
	AppID   int64 `json:"app_id" jsonschema:"o app"`
	Versao  int64 `json:"versao" jsonschema:"o número que enviar_versao devolveu em versao"`
	Esperar int   `json:"esperar,omitempty" jsonschema:"segundos para esperar uma mudança, até 30"`
}

func (h *Handler) toolVerVersao(ctx context.Context, _ *mcp.CallToolRequest, in verVersaoIn) (*mcp.CallToolResult, any, error) {
	route := fmt.Sprintf("/api/apps/%d/drafts/%d", in.AppID, in.Versao)
	var d draftAnswer
	if _, _, msg := h.toolAPI(ctx, "GET", route, nil, &d); msg != "" {
		return toolFail(msg)
	}
	if wait := min(max(in.Esperar, 0), mcpMaxWait); wait > 0 && draftLive(d.Status) {
		first, firstStage := d.Status, d.Stage
		deadline := time.Now().Add(time.Duration(wait) * time.Second)
		for time.Now().Before(deadline) && ctx.Err() == nil {
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
			}
			var next draftAnswer
			if _, _, msg := h.toolAPI(ctx, "GET", route, nil, &next); msg != "" {
				if ctx.Err() != nil {
					break
				}
				return toolFail(msg)
			}
			d = next
			if d.Status != first || d.Stage != firstStage {
				break
			}
		}
	}
	var app appDetail
	h.toolAPI(ctx, "GET", fmt.Sprintf("/api/apps/%d", in.AppID), nil, &app)

	situacao := draftSituacao(d)
	var b strings.Builder
	fmt.Fprintf(&b, "Versão %d do %s: %s", d.ID, app.Manifest.Name, situacao)
	if d.StageLabel != "" {
		b.WriteString(" — " + d.StageLabel)
	}
	if d.Status == "no_ar" && d.VersionNumber != nil {
		fmt.Fprintf(&b, " (versão %d do app)", *d.VersionNumber)
	}
	b.WriteString(".")
	if d.StatusReason != "" {
		b.WriteString("\nMotivo: " + d.StatusReason)
	}
	if len(d.Checks) > 0 {
		b.WriteString("\nConferências:")
		for _, c := range d.Checks {
			mark := "✗"
			if c.OK {
				mark = "✓"
			}
			fmt.Fprintf(&b, "\n%s %s", mark, c.Detail)
		}
	}
	files := []map[string]any{}
	if len(d.Files) > 0 {
		b.WriteString("\nArquivos do teste:")
		for _, f := range d.Files {
			fmt.Fprintf(&b, "\n- %s (%d bytes)", f.Path, f.Size)
			if !f.Saved {
				b.WriteString(" — não guardado")
			}
			files = append(files, map[string]any{"caminho": f.Path, "tamanho": f.Size, "guardado": f.Saved})
		}
		b.WriteString("\nPara ler um arquivo guardado, use baixar_arquivo_teste.")
	}
	log := d.Log
	if len(log) > mcpLogShown {
		log = "…" + log[len(log)-mcpLogShown:]
	}
	if log != "" {
		b.WriteString("\nLog (senhas trocadas por ••••):\n" + log)
	}
	confirm := ""
	if d.ConfirmURL != "" {
		confirm = h.mcpLink("%s", d.ConfirmURL)
		fmt.Fprintf(&b, "\nO plano de teste espera confirmação: mande este link ao responsável pelo app: %s", confirm)
	}
	if draftLive(d.Status) && confirm == "" {
		fmt.Fprintf(&b, "\nAinda não terminou: chame ver_versao de novo com esperar 30.")
	}
	return toolOK(b.String(), map[string]any{
		"app_id": in.AppID, "versao": d.ID, "situacao": situacao, "etapa": d.StageLabel, "motivo": d.StatusReason,
		"terminou": !draftLive(d.Status), "conferencias": d.Checks, "arquivos": files, "log": log, "confirmar": confirm,
		"versao_no_ar": d.VersionNumber,
	})
}

// ---- baixar_arquivo_teste ----

type baixarArquivoTesteIn struct {
	AppID   int64  `json:"app_id" jsonschema:"o app"`
	Versao  int64  `json:"versao" jsonschema:"a versão que enviar_versao devolveu"`
	Caminho string `json:"caminho" jsonschema:"o caminho do arquivo, como ver_versao mostrou"`
}

func (h *Handler) toolBaixarArquivoTeste(ctx context.Context, _ *mcp.CallToolRequest, in baixarArquivoTesteIn) (*mcp.CallToolResult, any, error) {
	route := fmt.Sprintf("/api/apps/%d/drafts/%d/files?path=%s", in.AppID, in.Versao, url.QueryEscape(in.Caminho))
	_, raw, msg := h.toolAPI(ctx, "GET", route, nil, nil)
	if msg != "" {
		return toolFail(msg)
	}
	page := h.mcpLink("/apps/%d", in.AppID)
	if len(raw) > mcpTextFileLimit || !utf8.Valid(raw) || bytes.IndexByte(raw, 0) >= 0 {
		download := h.mcpLink("%s", route)
		text := fmt.Sprintf("O arquivo %s é binário ou passa de 200 KB, então não vem pela conversa. O dono do app ou um administrador baixa por este link, entrando no Ziv: %s (página do app: %s).", in.Caminho, download, page)
		return toolOK(text, map[string]any{"caminho": in.Caminho, "tamanho": len(raw), "link": download, "pagina": page})
	}
	// The file is the app's real work and may show a vault value (a page
	// saying "Olá, <usuário>"): masked before it reaches the conversation.
	content := string(raw)
	if a, err := h.q.GetApp(ctx, in.AppID); err == nil {
		content = h.maskFor(ctx, a, content)
	} else {
		return toolFail("Não consegui abrir este app agora.")
	}
	return toolOK(fmt.Sprintf("Arquivo %s do teste:\n%s", in.Caminho, content), map[string]any{"caminho": in.Caminho, "tamanho": len(raw), "conteudo": content})
}
