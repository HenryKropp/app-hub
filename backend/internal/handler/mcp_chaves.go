package handler

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

// Chaves pelo Claude: Claude sends an app's .env from the person's computer
// straight to a one-time address, so the values fill the app's vault without
// ever entering the conversation. The address is the only credential (like
// preparar_envio): a 256-bit token stored as its hash, valid 30 minutes and
// once. The answer names what was saved, never a value.

const (
	chavesRoute    = "/api/mcp/chaves/"
	chavesMaxBytes = 64 << 10
	msgChavesGone  = "Este endereço de envio das chaves não vale mais. Peça um novo com enviar_chaves."
)

type ignoredKey struct {
	Nome   string `json:"nome"`
	Motivo string `json:"motivo"`
}

type chavesResult struct {
	Guardadas     []string     `json:"guardadas"`
	Configuracoes []string     `json:"configuracoes"`
	Ignoradas     []ignoredKey `json:"ignoradas"`
}

// envLine is one NAME=value line of a .env file.
type envLine struct{ name, value string }

// parseDotEnv reads a .env the way python-dotenv and most tools do: comments,
// "export ", quotes and inline comments after an unquoted value. A line it
// cannot read is reported by its line number, never by its content.
func parseDotEnv(body []byte) ([]envLine, []ignoredKey) {
	var out []envLine
	var bad []ignoredKey
	sc := bufio.NewScanner(bytes.NewReader(bytes.TrimPrefix(body, []byte("\xef\xbb\xbf"))))
	sc.Buffer(make([]byte, 0, 8192), chavesMaxBytes)
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(strings.TrimSuffix(sc.Text(), "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		name, value, ok := strings.Cut(line, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			bad = append(bad, ignoredKey{Nome: "linha " + strconv.Itoa(n), Motivo: "não está no formato NOME=valor"})
			continue
		}
		value = strings.TrimSpace(value)
		switch {
		case len(value) >= 2 && value[0] == '"' && strings.LastIndexByte(value, '"') > 0:
			value = value[1:strings.LastIndexByte(value, '"')]
			value = strings.NewReplacer(`\"`, `"`, `\\`, `\`).Replace(value)
		case len(value) >= 2 && value[0] == '\'' && strings.LastIndexByte(value, '\'') > 0:
			value = value[1:strings.LastIndexByte(value, '\'')]
		default:
			if i := strings.Index(value, " #"); i >= 0 {
				value = strings.TrimSpace(value[:i])
			}
		}
		// A repeated name keeps its last value, as python-dotenv does.
		if i := slices.IndexFunc(out, func(l envLine) bool { return l.name == name }); i >= 0 {
			out[i].value = value
			continue
		}
		out = append(out, envLine{name, value})
	}
	return out, bad
}

// receiveKeys fills the vault from a .env sent to an enviar_chaves address.
func (h *Handler) receiveKeys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	hash := hashToken(r.PathValue("token"))
	up, err := h.q.OpenMCPKeyUpload(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, msgChavesGone)
		return
	} else if err != nil {
		internalError(w, "mcp key upload open", err)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, chavesMaxBytes))
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		writeError(w, http.StatusRequestEntityTooLarge, "O arquivo passa de 64 KB. Mande só o .env do app.")
		return
	}
	if err != nil || len(bytes.TrimSpace(body)) == 0 {
		writeError(w, http.StatusBadRequest, "O envio chegou vazio. Mande o arquivo .env do app.")
		return
	}
	// The person may have lost the right to the vault since the address was made.
	a, err := h.q.GetApp(ctx, up.AppID)
	if err != nil {
		writeError(w, http.StatusNotFound, msgChavesGone)
		return
	}
	me, err := h.q.GetPerson(ctx, up.PersonID)
	if err != nil {
		writeError(w, http.StatusNotFound, msgChavesGone)
		return
	}
	acc, err := h.loadAccess(ctx, me)
	if err != nil {
		internalError(w, "mcp keys access", err)
		return
	}
	if !acc.For(appRef(a)).View || !canManageKeys(a, me, acc) {
		writeError(w, http.StatusForbidden, msgKeysManage)
		return
	}
	if h.shared.Vault == nil {
		writeError(w, http.StatusServiceUnavailable, msgKeysLocked)
		return
	}
	lines, ignored := parseDotEnv(body)
	if len(lines) == 0 && len(ignored) == 0 {
		writeError(w, http.StatusBadRequest, "O arquivo não tem nenhuma linha NOME=valor.")
		return
	}
	// Consume the address only now: a refused file can be sent again.
	if _, err := h.q.UseMCPKeyUpload(ctx, hash); errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, msgChavesGone)
		return
	} else if err != nil {
		internalError(w, "mcp key upload use", err)
		return
	}
	res, err := h.fillVault(ctx, a, me, lines)
	if err != nil {
		internalError(w, "mcp fill vault", err)
		return
	}
	res.Ignoradas = append(ignored, res.Ignoradas...)
	if len(res.Guardadas)+len(res.Configuracoes) > 0 {
		h.keyAudit(ctx, me.ID, "app_key.imported", "PUT /api/mcp/chaves/{token}")
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, res)
}

// fillVault saves each line: a secret into the vault, a plain setting of
// Configurações as it is. It keeps the vault's name rules and size cap.
func (h *Handler) fillVault(ctx context.Context, a sqlc.App, me sqlc.Person, lines []envLine) (chavesResult, error) {
	res := chavesResult{Guardadas: []string{}, Configuracoes: []string{}, Ignoradas: []ignoredKey{}}
	m := manifestOf(a)
	keys, _, err := h.vaultKeys(ctx, a)
	if err != nil {
		return res, err
	}
	inVault := map[string]bool{}
	for _, k := range keys {
		inVault[k.Name] = true
	}
	// A name already kept as a plain value (Outros ajustes) stays plain.
	rows, err := h.q.ListAppConfig(ctx, a.ID)
	if err != nil {
		return res, err
	}
	plain := map[string]bool{}
	for _, d := range plainDefs(m, rows) {
		plain[d.Name] = true
	}
	who := pgtype.Int8{Int64: me.ID, Valid: true}
	for _, l := range lines {
		switch {
		case plain[l.name]:
			if l.value == "" || len(l.value) > maxKeyValue || strings.ContainsAny(l.value, "\r\n\x00") {
				res.Ignoradas = append(res.Ignoradas, ignoredKey{l.name, "valor vazio ou inválido"})
				continue
			}
			if err := h.q.UpsertAppConfig(ctx, sqlc.UpsertAppConfigParams{AppID: a.ID, Name: l.name, ValuePlain: l.value, UpdatedBy: who}); err != nil {
				return res, err
			}
			res.Configuracoes = append(res.Configuracoes, l.name)
			continue
		case manifest.KeyNameProblem(l.name) != "":
			res.Ignoradas = append(res.Ignoradas, ignoredKey{l.name, manifest.KeyNameProblem(l.name)})
			continue
		case l.value == "":
			res.Ignoradas = append(res.Ignoradas, ignoredKey{l.name, "está vazia no .env"})
			continue
		case len(l.value) > maxKeyValue || strings.ContainsAny(l.value, "\r\n\x00"):
			res.Ignoradas = append(res.Ignoradas, ignoredKey{l.name, msgKeyBadValue})
			continue
		case !inVault[l.name] && len(inVault) >= manifest.MaxKeys:
			res.Ignoradas = append(res.Ignoradas, ignoredKey{l.name, fmt.Sprintf("o app já tem %d chaves e senhas, o máximo", manifest.MaxKeys)})
			continue
		}
		sealed, err := h.shared.Vault.Seal(l.value)
		if err != nil {
			return res, err
		}
		if err := h.q.UpsertAppConfig(ctx, sqlc.UpsertAppConfigParams{AppID: a.ID, Name: l.name, IsSecret: true, ValueCipher: sealed, UpdatedBy: who}); err != nil {
			return res, err
		}
		inVault[l.name] = true
		res.Guardadas = append(res.Guardadas, l.name)
	}
	sort.Strings(res.Guardadas)
	sort.Strings(res.Configuracoes)
	return res, nil
}

type enviarChavesIn struct {
	AppID int64 `json:"app_id" jsonschema:"o número do app no Ziv"`
}

func (h *Handler) toolEnviarChaves(ctx context.Context, _ *mcp.CallToolRequest, in enviarChavesIn) (*mcp.CallToolResult, any, error) {
	person := mcpPersonFrom(ctx)
	var keys keysList
	if _, _, msg := h.toolAPI(ctx, "GET", fmt.Sprintf("/api/apps/%d/keys", in.AppID), nil, &keys); msg != "" {
		return toolFail(msg)
	}
	a, err := h.q.GetApp(ctx, in.AppID)
	if err != nil {
		return toolFail("Não encontrei isso no Ziv, ou você não tem acesso.")
	}
	switch {
	case !manifest.HasVault(manifestOf(a).Kind):
		return toolFail(msgKeysNoVault)
	case !keys.CanManage:
		return toolFail(msgKeysManage)
	case keys.Locked:
		return toolFail(msgKeysLocked)
	}
	if err := h.q.DeleteExpiredMCPKeyUploads(ctx); err != nil {
		slog.Error("mcp delete expired key uploads", "err", err)
	}
	open, err := h.q.CountOpenMCPKeyUploads(ctx, person)
	if err != nil {
		slog.Error("mcp count key uploads", "err", err)
		return toolFail(msgMCPAPIDown)
	}
	if open >= envioMaxOpen {
		return toolFail("Você já tem 5 envios de chaves abertos. Use um deles ou espere 30 minutos.")
	}
	token, hash, err := newToken()
	if err != nil {
		return toolFail(msgMCPAPIDown)
	}
	until := time.Now().Add(envioTTL)
	if _, err := h.q.CreateMCPKeyUpload(ctx, sqlc.CreateMCPKeyUploadParams{TokenHash: hash, PersonID: person, AppID: a.ID,
		ExpiresAt: pgtype.Timestamptz{Time: until, Valid: true}}); err != nil {
		slog.Error("mcp create key upload", "err", err)
		return toolFail(msgMCPAPIDown)
	}
	endereco := h.mcpLink("%s%s", chavesRoute, token)
	out := map[string]any{
		"app_id":            a.ID,
		"endereco":          endereco,
		"vale_ate":          until.UTC().Format(time.RFC3339),
		"comando_windows":   fmt.Sprintf(`curl.exe -sS --fail-with-body -X PUT --data-binary "@<ARQUIVO_ENV>" "%s"`, endereco),
		"comando_git_bash":  fmt.Sprintf(`curl -sS --fail-with-body -T "<ARQUIVO_ENV>" "%s"`, endereco),
		"comando_mac_linux": fmt.Sprintf(`curl -sS --fail-with-body -T "<ARQUIVO_ENV>" "%s"`, endereco),
	}
	text := fmt.Sprintf("Endereço pronto para guardar as chaves e senhas do app %s. Vale uma vez, até %s (30 minutos).\n"+
		"Troque <ARQUIVO_ENV> pelo caminho completo do arquivo .env na pasta do app e rode o comando do sistema da pessoa. "+
		"NÃO abra, leia, mostre nem copie o .env: o curl manda o arquivo direto ao Ziv, e os valores nunca entram na conversa.\n"+
		"A resposta lista só os nomes: guardadas (chaves e senhas, escondidas), configuracoes (outros ajustes do app) e ignoradas (com o motivo). "+
		"Diga à pessoa quais chaves foram guardadas e mande o link %s para conferir na parte Configurações do app. O .env pode continuar na pasta: quando o app abre pelo Ziv, o valor guardado já está no ambiente, e o load_dotenv (Python) ou o dotenv (Node) não o troca.",
		a.Name, until.Local().Format("15:04"), h.mcpLink("/apps/%d#chaves", a.ID))
	return toolOK(text, out)
}
