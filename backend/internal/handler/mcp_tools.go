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
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

//go:embed mcp_guia.md
var mcpGuia string

const (
	mcpSearchLimit = 20
	mcpRunsLimit   = 10
	mcpMaxWait     = 30
	mcpFilesLimit  = 100
	msgMCPAPIDown  = "Não consegui falar com o Ziv agora. Tente de novo."
)

// runStatusLabels are the run situations as the Ziv screens say them.
var runStatusLabels = map[string]string{
	"waiting":     "Aguardando o computador",
	"running":     "Rodando",
	"done":        "Concluída",
	"no_files":    "Sem arquivos novos",
	"error":       "Erro",
	"interrupted": "Interrompida",
	"stopped":     "Parada",
}

func runStatusLabel(s string) string {
	if l, ok := runStatusLabels[s]; ok {
		return l
	}
	return s
}

func runIsLive(s string) bool { return s == "waiting" || s == "running" }

// foldText lowers s and drops accents, so "Análise" matches "analise".
func foldText(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, strings.ToLower(s))
	if err != nil {
		return strings.ToLower(s)
	}
	return out
}

// mcpLink is an absolute link to a page of this company's Ziv.
func (h *Handler) mcpLink(format string, args ...any) string {
	return strings.TrimRight(h.cfg.BaseURL, "/") + fmt.Sprintf(format, args...)
}

func toolOK(text string, structured any) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, structured, nil
}

func toolFail(text string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
}

// apiFailure is the message a tool shows for a non-2xx answer: the
// handler's own text when it has one.
func apiFailure(status int, raw []byte) string {
	if msg := apiError(raw); msg != "" {
		return msg
	}
	switch status {
	case http.StatusNotFound:
		return "Não encontrei isso no Ziv, ou você não tem acesso."
	case http.StatusUnauthorized:
		return "Conecte o Claude ao Ziv de novo."
	case http.StatusForbidden:
		return "Você não tem permissão para isso no Ziv."
	}
	return fmt.Sprintf("O Ziv não conseguiu fazer isso agora (erro %d).", status)
}

// toolAPI calls one API route as the MCP person and decodes a 2xx answer
// into out. On failure it returns the status, the raw body and the text for
// the person.
func (h *Handler) toolAPI(ctx context.Context, method, route string, body, out any) (int, []byte, string) {
	status, raw, err := h.callAPI(ctx, method, route, body)
	if errors.Is(err, errMCPRoute) {
		return 0, nil, "Não encontrei isso no Ziv, ou você não tem acesso."
	}
	if err != nil {
		slog.Error("mcp tool api", "method", method, "err", err)
		return 0, nil, msgMCPAPIDown
	}
	if status < 200 || status > 299 {
		return status, raw, apiFailure(status, raw)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			slog.Error("mcp tool decode", "method", method, "err", err)
			return status, raw, msgMCPAPIDown
		}
	}
	return status, raw, ""
}

// auditedAPI is toolAPI for routes that change something. callAPI skips
// Secure, so the audit row Secure would write is written here, with the same
// shape (no body, only the route template and the status) and an action
// that says it came through Claude.
func (h *Handler) auditedAPI(ctx context.Context, method, route, pattern string, body, out any) (int, []byte, string) {
	actor := mcpPersonFrom(ctx)
	eventID, err := h.auditStart(ctx, &actor, "mcp.request.started")
	if err != nil {
		slog.Error("mcp audit start failed")
		return 0, nil, "Não foi possível registrar a operação com segurança."
	}
	status, raw, msg := h.toolAPI(ctx, method, route, body, out)
	h.auditFinish(context.WithoutCancel(ctx), eventID, "mcp.request.completed", pattern, status)
	return status, raw, msg
}

// registerMCPTools is the one place where tools are added to the server.
func (h *Handler) registerMCPTools(s *mcp.Server) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	// Run and publish create things and never delete: Claude still asks
	// "permitir?" because they are not read-only.
	notDestructive := false
	changes := &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &notDestructive}
	// enviar_chaves replaces a key already in the vault with the .env's value.
	replaces := true
	replacesKeys := &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &replaces}

	mcp.AddTool(s, &mcp.Tool{
		Name: "buscar",
		Description: "Procura apps e artifacts no Ziv por palavra, área ou etiqueta (sem diferença de acentos ou maiúsculas). " +
			"Sem texto, lista o que a pessoa vê. Devolve até 20 itens com nome, resumo, área e link.",
		Annotations: readOnly,
	}, h.toolBuscar)

	mcp.AddTool(s, &mcp.Tool{
		Name: "ver_app",
		Description: "Mostra a ficha de um app do Ziv: o que ele pede (entradas), o que entrega, quem cuida, a versão, " +
			"se fica aberto no servidor e se a pessoa pode rodar pelo Claude. Use antes de rodar_app.",
		Annotations: readOnly,
	}, h.toolVerApp)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ver_artifact",
		Description: "Mostra um artifact do Ziv: nome, resumo, etiquetas, área, quantas versões tem, a versão atual e o link.",
		Annotations: readOnly,
	}, h.toolVerArtifact)

	mcp.AddTool(s, &mcp.Tool{
		Name: "rodar_app",
		Description: "Roda um app do Ziv no computador da pessoa, pelo Agente do Ziv. Veja antes com ver_app o que ele pede. " +
			"Em entradas, mande as respostas pelo id de cada entrada (texto, número, data ou escolha). " +
			"Entradas de arquivo ainda não podem ir pelo Claude: nesse caso, peça para a pessoa rodar pelo Ziv. " +
			"Antes de rodar, PERGUNTE à pessoa em qual pasta do computador dela os arquivos devem ficar e espere a resposta, mesmo que ela não tenha falado de pasta; nunca escolha uma pasta sozinho. " +
			"Se ver_app mostrar pasta_de_sempre, ofereça essa pasta na pergunta. Mande a resposta em pasta_saida, com o caminho completo (ex.: C:\\Users\\ana\\Documentos). " +
			"Depois de rodar, chame ver_execucao com esperar 30, de novo e de novo na mesma resposta, até a execução terminar; " +
			"então diga o resultado e onde ficaram os arquivos. Se a conversa parar antes, confira de novo com ver_execucao quando a pessoa perguntar.",
		Annotations: changes,
	}, h.toolRodarApp)

	mcp.AddTool(s, &mcp.Tool{
		Name: "ver_execucao",
		Description: "Mostra como está uma execução: a situação (aguardando, rodando, concluída, erro...), a mensagem de erro, " +
			"a pasta e os arquivos gerados, e o link. Com esperar (até 30 segundos), espera uma mudança antes de responder. " +
			"Enquanto a execução não terminar, chame de novo com esperar 30.",
		Annotations: readOnly,
	}, h.toolVerExecucao)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "minhas_execucoes",
		Description: "Lista as últimas 10 execuções da pessoa no Ziv, com a situação e o link de cada uma.",
		Annotations: readOnly,
	}, h.toolMinhasExecucoes)

	mcp.AddTool(s, &mcp.Tool{
		Name: "publicar_artifact",
		Description: "Publica no Ziv uma página HTML feita pelo Claude, como artifact. Antes, leia guia_visual para a página ficar com a cara do Ziv. " +
			"Sem artifact_id, cria um artifact novo com nome, resumo, etiquetas e área (o nome da área; sem ela, a área da pessoa). " +
			"Com artifact_id, manda o HTML como versão nova desse artifact (o nome não muda) e nota diz o que mudou. " +
			"Se o Ziv avisar que o HTML parece ter senhas ou chaves, pergunte à pessoa antes de mandar de novo com confirmar_segredos true.",
		Annotations: changes,
	}, h.toolPublicarArtifact)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "guia_visual",
		Description: "Devolve o guia visual do Ziv (cores, fonte Geist, bordas) e as regras para o HTML de um artifact funcionar dentro do Ziv. Leia antes de fazer um artifact.",
		Annotations: readOnly,
	}, h.toolGuiaVisual)

	mcp.AddTool(s, &mcp.Tool{
		Name: "preparar_envio",
		Description: "Prepara o envio do código de um app para o Ziv. Devolve um endereço de uso único (vale 30 minutos) e os comandos para zipar a pasta " +
			"(sem .env, .git, venv, node_modules) numa pasta temporária e mandar com curl. Use só depois que a pessoa disse \"pode registrar\".",
		Annotations: changes,
	}, h.toolPrepararEnvio)

	mcp.AddTool(s, &mcp.Tool{
		Name: "enviar_chaves",
		Description: "Preenche o cofre de um app do Ziv com as senhas e chaves do arquivo .env da pasta do app, sem que os valores passem pela conversa. " +
			"Devolve um endereço de uso único (30 minutos) e o comando curl que manda o .env direto ao Ziv. Nunca abra, leia nem mostre o .env. " +
			"Use depois de registrar_app quando a pasta tiver um .env, ou quando a pessoa pedir para preencher o cofre. Só o dono do app ou um administrador pode.",
		Annotations: replacesKeys,
	}, h.toolEnviarChaves)

	mcp.AddTool(s, &mcp.Tool{
		Name: "formato_do_app",
		Description: "Devolve o protocolo para registrar ou atualizar um app no Ziv a partir de uma pasta do computador: como ler a pasta, " +
			"o formato do manifesto, as áreas da empresa e os limites. Leia antes de registrar_app.",
		Annotations: readOnly,
	}, h.toolFormatoDoApp)

	mcp.AddTool(s, &mcp.Tool{
		Name: "registrar_app",
		Description: "Registra no Ziv um app novo (script ou app com tela) com o código que chegou por preparar_envio, ou manda uma versão nova de um app " +
			"que já existe (app_id). Siga o protocolo de formato_do_app e use só depois que a pessoa disse \"pode registrar\" (ou \"pode mandar\"). " +
			"Se o Ziv avisar que o código parece ter senhas ou chaves, pergunte à pessoa antes de mandar de novo com confirmar_segredos true.",
		Annotations: changes,
	}, h.toolRegistrarApp)

	mcp.AddTool(s, &mcp.Tool{
		Name: "publicar_app_com_tela",
		Description: "Põe no ar (ou atualiza) um app com tela do Ziv, no endereço próprio dele, como o botão Publicar do site. Chame depois de registrar_app de um app com tela. " +
			"Se a checagem do Ziv trouxer avisos, mostre cada um à pessoa e só chame de novo com ciente depois que ela disser que está ciente. " +
			"Se faltar preencher chaves, a pessoa preenche no Ziv, nunca pela conversa. Enquanto estiver publicando, chame de novo com esperar 30.",
		Annotations: changes,
	}, h.toolPublicarTela)
}

// ---- buscar ----

type buscarIn struct {
	Texto string `json:"texto,omitempty" jsonschema:"palavras para procurar no nome, resumo, área ou etiquetas"`
	Tipo  string `json:"tipo,omitempty" jsonschema:"app ou artifact; vazio procura nos dois"`
}

type buscarItem struct {
	Tipo   string `json:"tipo"`
	ID     int64  `json:"id"`
	Nome   string `json:"nome"`
	Resumo string `json:"resumo"`
	Area   string `json:"area"`
	Link   string `json:"link"`
}

func (h *Handler) toolBuscar(ctx context.Context, _ *mcp.CallToolRequest, in buscarIn) (*mcp.CallToolResult, any, error) {
	tipo := strings.ToLower(strings.TrimSpace(in.Tipo))
	if tipo != "" && tipo != "app" && tipo != "artifact" {
		return toolFail(`O tipo deve ser "app" ou "artifact" (ou vazio, para os dois).`)
	}
	words := strings.Fields(foldText(in.Texto))
	matches := func(fields ...string) bool {
		hay := foldText(strings.Join(fields, " "))
		for _, w := range words {
			if !strings.Contains(hay, w) {
				return false
			}
		}
		return true
	}
	found := []buscarItem{}
	if tipo != "artifact" {
		var apps []struct {
			ID      int64  `json:"id"`
			Name    string `json:"name"`
			Summary string `json:"summary"`
			Area    string `json:"area"`
		}
		if _, _, msg := h.toolAPI(ctx, "GET", "/api/apps", nil, &apps); msg != "" {
			return toolFail(msg)
		}
		for _, a := range apps {
			if matches(a.Name, a.Summary, a.Area) {
				found = append(found, buscarItem{Tipo: "app", ID: a.ID, Nome: a.Name, Resumo: a.Summary, Area: a.Area, Link: h.mcpLink("/apps/%d", a.ID)})
			}
		}
	}
	if tipo != "app" {
		var arts []struct {
			ID      int64    `json:"id"`
			Name    string   `json:"name"`
			Summary string   `json:"summary"`
			Area    string   `json:"area"`
			Tags    []string `json:"tags"`
		}
		if _, _, msg := h.toolAPI(ctx, "GET", "/api/artifacts", nil, &arts); msg != "" {
			return toolFail(msg)
		}
		for _, a := range arts {
			if matches(a.Name, a.Summary, a.Area, strings.Join(a.Tags, " ")) {
				found = append(found, buscarItem{Tipo: "artifact", ID: a.ID, Nome: a.Name, Resumo: a.Summary, Area: a.Area, Link: h.mcpLink("/artifacts/%d", a.ID)})
			}
		}
	}
	total := len(found)
	if total > mcpSearchLimit {
		found = found[:mcpSearchLimit]
	}
	var b strings.Builder
	switch {
	case total == 0:
		b.WriteString("Não achei nada no Ziv com essa busca.")
	case total > len(found):
		fmt.Fprintf(&b, "Achei %d itens; mostrando os primeiros %d. Refine a busca para ver os outros.\n", total, len(found))
	default:
		fmt.Fprintf(&b, "Achei %d item(ns):\n", total)
	}
	for _, it := range found {
		fmt.Fprintf(&b, "- [%s %d] %s", it.Tipo, it.ID, it.Nome)
		if it.Resumo != "" {
			b.WriteString(" — " + it.Resumo)
		}
		if it.Area != "" {
			b.WriteString(" (área: " + it.Area + ")")
		}
		b.WriteString(" " + it.Link + "\n")
	}
	return toolOK(strings.TrimSpace(b.String()), map[string]any{"itens": found, "total": total})
}

// ---- ver_app ----

type idIn struct {
	ID int64 `json:"id" jsonschema:"o número do item no Ziv"`
}

type entradaOut struct {
	ID          string   `json:"id"`
	Rotulo      string   `json:"rotulo"`
	Tipo        string   `json:"tipo"`
	Obrigatoria bool     `json:"obrigatoria"`
	Opcoes      []string `json:"opcoes,omitempty"`
	Multipla    bool     `json:"multipla,omitempty"`
}

type saidaOut struct {
	Rotulo  string `json:"rotulo"`
	Arquivo string `json:"arquivo"`
}

func (h *Handler) toolVerApp(ctx context.Context, _ *mcp.CallToolRequest, in idIn) (*mcp.CallToolResult, any, error) {
	var d appDetail
	if _, _, msg := h.toolAPI(ctx, "GET", fmt.Sprintf("/api/apps/%d", in.ID), nil, &d); msg != "" {
		return toolFail(msg)
	}
	// The catalog row has the owner and area names the ficha lacks.
	m := d.Manifest
	var list []struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		Summary   string `json:"summary"`
		Area      string `json:"area"`
		OwnerName string `json:"owner_name"`
	}
	if _, _, msg := h.toolAPI(ctx, "GET", "/api/apps", nil, &list); msg != "" {
		return toolFail(msg)
	}
	// Name and summary as the catalog shows them (and buscar returns them).
	nome, resumo, area, owner := m.Name, m.Summary, "", ""
	for _, a := range list {
		if a.ID == d.ID {
			nome, resumo, area, owner = a.Name, a.Summary, a.Area, a.OwnerName
		}
	}
	entradas := []entradaOut{}
	for _, x := range m.Inputs {
		if x.Source == "window" { // asked by the app's own window, never before running
			continue
		}
		entradas = append(entradas, entradaOut{ID: x.ID, Rotulo: x.Label, Tipo: x.Type, Obrigatoria: x.Required, Opcoes: x.Options, Multipla: x.Multiple})
	}
	saidas := []saidaOut{}
	for _, o := range m.Outputs {
		saidas = append(saidas, saidaOut{Rotulo: o.Label, Arquivo: o.Pattern})
	}
	substituto := ""
	if m.Backup != nil {
		substituto = m.Backup.Name
	}
	out := map[string]any{
		"id": d.ID, "nome": nome, "resumo": resumo, "tipo": m.Kind, "area": area,
		"dono": owner, "substituto": substituto, "entradas": entradas, "saidas": saidas,
		"compartilhado": d.Shared != nil, "pode_rodar": d.CanRun && d.Shared == nil,
		"link": h.mcpLink("/apps/%d", d.ID),
	}
	if d.Version != nil {
		out["versao"] = d.Version.Number
	}
	if d.Shared != nil {
		out["endereco"] = d.Shared.URL
	}
	if d.RememberedFolder != "" {
		out["pasta_de_sempre"] = d.RememberedFolder
	}
	out["codigo_guardado"] = d.StoredCode
	if d.StoredCode && d.Can.Edit {
		inputs := m.Inputs
		if inputs == nil {
			inputs = []manifest.Input{}
		}
		outputs := m.Outputs
		if outputs == nil {
			outputs = []manifest.Output{}
		}
		out["ficha_editavel"] = map[string]any{"summary": m.Summary, "inputs": inputs, "outputs": outputs,
			"entrypoint": m.Entrypoint, "runtime": m.Runtime, "updated_at": d.UpdatedAt.Time}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s (app %d)\n", nome, d.ID)
	if resumo != "" {
		b.WriteString(resumo + "\n")
	}
	if label, ok := manifest.KindLabels[m.Kind]; ok {
		b.WriteString("Tipo: " + label + "\n")
	}
	if area != "" {
		b.WriteString("Área: " + area + "\n")
	}
	if owner != "" {
		b.WriteString("Dono: " + owner)
		if substituto != "" {
			b.WriteString("; substituto: " + substituto)
		}
		b.WriteString("\n")
	}
	if d.Version != nil {
		fmt.Fprintf(&b, "Versão: %d\n", d.Version.Number)
	}
	switch {
	case d.Shared != nil:
		b.WriteString("Este app fica aberto no servidor; abra pelo link: " + d.Shared.URL + "\n")
	case d.CanRun:
		b.WriteString("Pode rodar pelo Claude, no computador da pessoa (rodar_app). Antes, pergunte em qual pasta os arquivos devem ficar.\n")
		if d.RememberedFolder != "" {
			b.WriteString("Pasta de sempre: " + d.RememberedFolder + "\n")
		}
	default:
		b.WriteString("Este app não roda pelo Ziv.\n")
	}
	switch {
	case d.StoredCode && d.Can.Edit:
		b.WriteString("Pode receber versão nova pelo Claude (registrar_app com app_id).\n")
	case d.StoredCode:
		b.WriteString("Só quem pode mudar este app manda versões novas.\n")
	default:
		b.WriteString(noStoredCodeMsg + "\n")
	}
	if len(entradas) > 0 {
		b.WriteString("Entradas:\n")
		for _, e := range entradas {
			fmt.Fprintf(&b, "- %s (id %q, %s", e.Rotulo, e.ID, e.Tipo)
			if e.Obrigatoria {
				b.WriteString(", obrigatória")
			}
			if len(e.Opcoes) > 0 {
				b.WriteString(", opções: " + strings.Join(e.Opcoes, " | "))
			}
			if e.Tipo == "file" {
				b.WriteString(", arquivo: só pelo Ziv")
			}
			b.WriteString(")\n")
		}
	}
	if len(saidas) > 0 {
		b.WriteString("Entrega:\n")
		for _, s := range saidas {
			fmt.Fprintf(&b, "- %s (%s)\n", s.Rotulo, s.Arquivo)
		}
	}
	b.WriteString("Link: " + h.mcpLink("/apps/%d", d.ID))
	return toolOK(b.String(), out)
}

// ---- ver_artifact ----

func (h *Handler) toolVerArtifact(ctx context.Context, _ *mcp.CallToolRequest, in idIn) (*mcp.CallToolResult, any, error) {
	var d artifactDetailJSON
	if _, _, msg := h.toolAPI(ctx, "GET", fmt.Sprintf("/api/artifacts/%d", in.ID), nil, &d); msg != "" {
		return toolFail(msg)
	}
	tags := d.Tags
	if tags == nil {
		tags = []string{}
	}
	link := h.mcpLink("/artifacts/%d", d.ID)
	out := map[string]any{
		"id": d.ID, "nome": d.Name, "resumo": d.Summary, "etiquetas": tags, "area": d.Area,
		"dono": d.OwnerName, "versoes": len(d.Versions), "versao_atual": d.Current,
		"pelo_claude": d.Origin == "mcp", "pode_mandar_versao": d.Can.Edit, "link": link,
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s (artifact %d)\n", d.Name, d.ID)
	if d.Summary != "" {
		b.WriteString(d.Summary + "\n")
	}
	if len(tags) > 0 {
		b.WriteString("Etiquetas: " + strings.Join(tags, ", ") + "\n")
	}
	if d.Area != "" {
		b.WriteString("Área: " + d.Area + "\n")
	}
	if d.OwnerName != "" {
		b.WriteString("Dono: " + d.OwnerName + "\n")
	}
	fmt.Fprintf(&b, "Versão atual: %d de %d\n", d.Current, len(d.Versions))
	b.WriteString("Link: " + link)
	return toolOK(b.String(), out)
}

// ---- rodar_app ----

type rodarIn struct {
	ID         int64          `json:"id" jsonschema:"o número do app no Ziv"`
	Entradas   map[string]any `json:"entradas,omitempty" jsonschema:"as respostas do que o app pede, pelo id de cada entrada"`
	PastaSaida string         `json:"pasta_saida,omitempty" jsonschema:"a pasta que a pessoa escolheu, com o caminho completo no computador dela; pergunte antes de rodar"`
}

// runInputs turns the answers Claude sent into what the run form takes:
// text, or a list of texts for multiple choices.
func runInputs(in map[string]any) (map[string]any, string) {
	out := make(map[string]any, len(in))
	for id, v := range in {
		switch x := v.(type) {
		case nil:
		case string:
			out[id] = x
		case float64:
			out[id] = strconv.FormatFloat(x, 'f', -1, 64)
		case bool:
			out[id] = strconv.FormatBool(x)
		case []any:
			list := make([]string, 0, len(x))
			for _, item := range x {
				s, ok := item.(string)
				if !ok {
					return nil, fmt.Sprintf("A entrada %q deve ser um texto ou uma lista de textos.", id)
				}
				list = append(list, s)
			}
			out[id] = list
		default:
			return nil, fmt.Sprintf("A entrada %q deve ser um texto, um número ou uma lista de textos.", id)
		}
	}
	return out, ""
}

func (h *Handler) toolRodarApp(ctx context.Context, _ *mcp.CallToolRequest, in rodarIn) (*mcp.CallToolResult, any, error) {
	var d appDetail
	if _, _, msg := h.toolAPI(ctx, "GET", fmt.Sprintf("/api/apps/%d", in.ID), nil, &d); msg != "" {
		return toolFail(msg)
	}
	if d.Shared != nil {
		link := d.Shared.URL
		if link == "" {
			link = h.mcpLink("/apps/%d", d.ID)
		}
		return toolOK("Este app fica aberto no servidor; abra pelo link: "+link, map[string]any{"compartilhado": true, "link": link})
	}
	if strings.TrimSpace(in.PastaSaida) == "" {
		msg := "Falta a pasta. Não escolha uma pasta sozinho: pergunte à pessoa em qual pasta do computador dela os arquivos devem ficar, espere a resposta e então chame rodar_app de novo com pasta_saida."
		if d.RememberedFolder != "" {
			msg += " Sugira a pasta de sempre dela para este app (" + d.RememberedFolder + ") e pergunte se pode usar."
		}
		return toolFail(msg)
	}
	inputs, msg := runInputs(in.Entradas)
	if msg != "" {
		return toolFail(msg)
	}
	body := map[string]any{"output_folder": in.PastaSaida}
	if len(inputs) > 0 {
		body["inputs"] = inputs
	}
	var run runJSON
	if _, _, msg := h.auditedAPI(ctx, "POST", fmt.Sprintf("/api/apps/%d/runs", d.ID), "POST /api/apps/{id}/runs", body, &run); msg != "" {
		return toolFail(msg)
	}
	link := h.mcpLink("/execucoes/%d", run.ID)
	situacao := runStatusLabel(run.Status)
	text := fmt.Sprintf("Comecei a execução %d do %s (%s). Pasta dos arquivos: %s\n"+
		"Agora chame ver_execucao com id %d e esperar 30, de novo até terminar.\nLink: %s",
		run.ID, run.AppName, situacao, run.OutputFolder, run.ID, link)
	return toolOK(text, map[string]any{"execucao_id": run.ID, "situacao": situacao, "link": link})
}

// ---- ver_execucao ----

type verExecucaoIn struct {
	ID      int64 `json:"id" jsonschema:"o número da execução"`
	Esperar int   `json:"esperar,omitempty" jsonschema:"quantos segundos esperar por uma mudança, de 0 a 30"`
}

type arquivoOut struct {
	Nome    string `json:"nome"`
	Caminho string `json:"caminho"`
}

// fileOnComputer is where a run file sits on the person's (Windows) computer.
func fileOnComputer(folder, rel string) string {
	return strings.TrimRight(folder, `\/`) + `\` + strings.ReplaceAll(rel, "/", `\`)
}

func (h *Handler) toolVerExecucao(ctx context.Context, _ *mcp.CallToolRequest, in verExecucaoIn) (*mcp.CallToolResult, any, error) {
	route := fmt.Sprintf("/api/runs/%d", in.ID)
	var run runJSON
	if _, _, msg := h.toolAPI(ctx, "GET", route, nil, &run); msg != "" {
		return toolFail(msg)
	}
	wait := min(max(in.Esperar, 0), mcpMaxWait)
	if wait > 0 && runIsLive(run.Status) {
		first := run.Status
		ticker := time.NewTicker(time.Second)
		deadline := time.NewTimer(time.Duration(wait) * time.Second)
	poll:
		for {
			select {
			case <-ctx.Done():
				break poll
			case <-deadline.C:
				break poll
			case <-ticker.C:
				var next runJSON
				if _, _, msg := h.toolAPI(ctx, "GET", route, nil, &next); msg != "" {
					if ctx.Err() != nil {
						break poll
					}
					ticker.Stop()
					deadline.Stop()
					return toolFail(msg)
				}
				run = next
				if run.Status != first || !runIsLive(run.Status) {
					break poll
				}
			}
		}
		ticker.Stop()
		deadline.Stop()
	}

	link := h.mcpLink("/execucoes/%d", run.ID)
	situacao := runStatusLabel(run.Status)
	arquivos := []arquivoOut{}
	total := 0
	for _, g := range run.Groups {
		for _, f := range g.Files {
			total++
			if len(arquivos) < mcpFilesLimit {
				arquivos = append(arquivos, arquivoOut{Nome: path.Base(f.Path), Caminho: fileOnComputer(run.OutputFolder, f.Path)})
			}
		}
	}
	out := map[string]any{
		"execucao_id": run.ID, "app": run.AppName, "situacao": situacao, "terminou": !runIsLive(run.Status),
		"erro": run.ErrorMessage, "pasta": run.OutputFolder, "arquivos": arquivos, "total_arquivos": total, "link": link,
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Execução %d do %s: %s.\n", run.ID, run.AppName, situacao)
	if run.ErrorMessage != "" {
		b.WriteString("Mensagem: " + run.ErrorMessage + "\n")
	}
	if runIsLive(run.Status) {
		b.WriteString("Ainda não terminou: chame ver_execucao de novo com esperar 30.\n")
	}
	if run.OutputFolder != "" {
		b.WriteString("Pasta: " + run.OutputFolder + "\n")
	}
	if total > 0 {
		fmt.Fprintf(&b, "Arquivos (%d):\n", total)
		for _, a := range arquivos {
			b.WriteString("- " + a.Caminho + "\n")
		}
		if total > len(arquivos) {
			fmt.Fprintf(&b, "... e mais %d no link.\n", total-len(arquivos))
		}
	}
	b.WriteString("Link: " + link)
	return toolOK(b.String(), out)
}

// ---- minhas_execucoes ----

type emptyIn struct{}

func (h *Handler) toolMinhasExecucoes(ctx context.Context, _ *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, any, error) {
	var list runListJSON
	if _, _, msg := h.toolAPI(ctx, "GET", "/api/runs", nil, &list); msg != "" {
		return toolFail(msg)
	}
	items := list.Items
	if len(items) > mcpRunsLimit {
		items = items[:mcpRunsLimit]
	}
	execucoes := make([]map[string]any, 0, len(items))
	var b strings.Builder
	if len(items) == 0 {
		b.WriteString("Você ainda não tem execuções no Ziv.")
	} else {
		b.WriteString("Suas últimas execuções:\n")
	}
	for _, x := range items {
		link := h.mcpLink("/execucoes/%d", x.ID)
		situacao := runStatusLabel(x.Status)
		execucoes = append(execucoes, map[string]any{
			"execucao_id": x.ID, "app": x.AppName, "situacao": situacao, "pelo_claude": x.Origin == "mcp",
			"quando": x.CreatedAt.Format(time.RFC3339), "arquivos": x.FileCount, "pasta": x.OutputFolder, "link": link,
		})
		fmt.Fprintf(&b, "- %d · %s · %s · %s · %d arquivo(s) %s\n", x.ID, x.AppName, situacao, x.CreatedAt.Local().Format("02/01 15:04"), x.FileCount, link)
	}
	return toolOK(strings.TrimSpace(b.String()), map[string]any{"execucoes": execucoes})
}

// ---- publicar_artifact ----

type publicarIn struct {
	Nome              string   `json:"nome,omitempty" jsonschema:"o nome do artifact; obrigatório para criar, ignorado com artifact_id"`
	HTML              string   `json:"html" jsonschema:"a página inteira, em um arquivo HTML só"`
	Resumo            string   `json:"resumo,omitempty" jsonschema:"uma frase dizendo o que é"`
	Etiquetas         []string `json:"etiquetas,omitempty" jsonschema:"palavras para achar depois"`
	Area              string   `json:"area,omitempty" jsonschema:"o nome da área; sem ela, a área da pessoa"`
	ArtifactID        int64    `json:"artifact_id,omitempty" jsonschema:"para mandar uma versão nova de um artifact que já existe"`
	Nota              string   `json:"nota,omitempty" jsonschema:"o que mudou nesta versão"`
	ConfirmarSegredos bool     `json:"confirmar_segredos,omitempty" jsonschema:"true só depois que a pessoa confirmar que pode publicar mesmo com possíveis senhas ou chaves"`
}

// artifactArea finds the area by name, or the person's first area, or the first area.
func (h *Handler) artifactArea(ctx context.Context, name string) (int64, string) {
	var areas []areaJSON
	if _, _, msg := h.toolAPI(ctx, "GET", "/api/areas", nil, &areas); msg != "" {
		return 0, msg
	}
	if len(areas) == 0 {
		return 0, "O Ziv ainda não tem áreas. Peça para quem administra criar uma."
	}
	if want := foldText(strings.TrimSpace(name)); want != "" {
		names := make([]string, 0, len(areas))
		for _, a := range areas {
			if foldText(a.Name) == want {
				return a.ID, ""
			}
			names = append(names, a.Name)
		}
		return 0, "Não achei a área " + strconv.Quote(name) + ". As áreas do Ziv são: " + strings.Join(names, ", ") + "."
	}
	me := mcpPersonFrom(ctx)
	isMe := func(r nameRef) bool { return r.ID == me }
	for _, a := range areas {
		if slices.ContainsFunc(a.Members, isMe) || slices.ContainsFunc(a.Leaders, isMe) {
			return a.ID, ""
		}
	}
	return areas[0].ID, ""
}

func (h *Handler) toolPublicarArtifact(ctx context.Context, _ *mcp.CallToolRequest, in publicarIn) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.HTML) == "" {
		return toolFail("Mande o HTML da página em html.")
	}
	if len(in.HTML) > maxArtifactBytes {
		return toolFail(msgArtifactTooBig)
	}
	version := map[string]any{
		"kind": "file", "file_name": "artifact.html", "content_base64": base64.StdEncoding.EncodeToString([]byte(in.HTML)),
		"note": in.Nota, "confirm_secrets": in.ConfirmarSegredos,
	}
	var (
		d      artifactDetailJSON
		status int
		raw    []byte
		msg    string
	)
	if in.ArtifactID != 0 {
		status, raw, msg = h.auditedAPI(ctx, "POST", fmt.Sprintf("/api/artifacts/%d/versions", in.ArtifactID), "POST /api/artifacts/{id}/versions", version, &d)
	} else {
		if strings.TrimSpace(in.Nome) == "" {
			return toolFail("Dê um nome ao artifact em nome para criar um artifact novo.")
		}
		areaID, areaMsg := h.artifactArea(ctx, in.Area)
		if areaMsg != "" {
			return toolFail(areaMsg)
		}
		tags := in.Etiquetas
		if tags == nil {
			tags = []string{}
		}
		version["name"], version["summary"], version["tags"], version["area_id"] = in.Nome, in.Resumo, tags, areaID
		status, raw, msg = h.auditedAPI(ctx, "POST", "/api/artifacts", "POST /api/artifacts", version, &d)
	}
	if msg != "" {
		var e struct {
			Code        string `json:"code"`
			SecretCount int    `json:"secret_count"`
		}
		_ = json.Unmarshal(raw, &e)
		if status == http.StatusConflict && e.Code == "artifact_secrets_confirmation_required" {
			return toolFail(fmt.Sprintf("O Ziv achou %d trecho(s) no HTML que parecem senhas ou chaves. Nada foi publicado. "+
				"Tire esses trechos do HTML, ou pergunte à pessoa se quer publicar mesmo assim; "+
				"só se ela confirmar, chame publicar_artifact de novo com confirmar_segredos: true.", e.SecretCount))
		}
		return toolFail(msg)
	}
	link := h.mcpLink("/artifacts/%d", d.ID)
	text := fmt.Sprintf("Publiquei o artifact %q, versão %d.\nLink: %s", d.Name, d.Current, link)
	return toolOK(text, map[string]any{"artifact_id": d.ID, "versao": d.Current, "link": link})
}

// ---- guia_visual ----

func (h *Handler) toolGuiaVisual(context.Context, *mcp.CallToolRequest, emptyIn) (*mcp.CallToolResult, any, error) {
	return toolOK(mcpGuia, map[string]any{"guia": mcpGuia})
}
