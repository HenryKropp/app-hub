package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/HenryKropp/app-hub/backend/internal/bundle"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
)

// MCP Etapa 2: Claude sends an app's code as a zip to a one-time address.
// The address is the only credential (Claude's shell has no session), so it
// is a 256-bit token stored only as its hash, valid 30 minutes and once.

const (
	envioTTL      = 30 * time.Minute
	envioMaxOpen  = 5
	envioRoute    = "/api/mcp/envios/"
	msgEnvioBig   = "O pacote passa de 12 MB. Deixe de fora dados e saídas acumuladas."
	msgEnvioGone  = "Este endereço de envio não vale mais. Peça um novo com preparar_envio."
	msgEnvioLimit = "Você já tem 5 envios abertos. Termine ou espere 30 minutos."
)

// receiveUpload stores the zip sent to a preparar_envio address. Nothing is
// stored when the zip is refused, so the same address can be used again.
func (h *Handler) receiveUpload(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, int64(bundle.SiteLimits.MaxZipBytes)))
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		writeError(w, http.StatusRequestEntityTooLarge, msgEnvioBig)
		return
	}
	if err != nil || len(body) == 0 {
		writeError(w, http.StatusBadRequest, "O envio chegou vazio. Mande o .zip do app.")
		return
	}
	res, err := bundle.Extract(body, bundle.SiteLimits)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_, err = h.q.ReceiveMCPUpload(r.Context(), sqlc.ReceiveMCPUploadParams{Content: body, Size: int32(len(body)), TokenHash: hashToken(token)})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, msgEnvioGone)
		return
	}
	if err != nil {
		internalError(w, "receive mcp upload", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"recebido": true, "arquivos": len(res.Files)})
}

// envioExcludes are left out on the person's computer, before the zip leaves it.
var envioExcludes = []string{".git", ".venv", "venv", "node_modules", "__pycache__", ".env", ".env.*", "*.pem", "*.key"}

func envioWindows(id int64, endereco string) string {
	var ex strings.Builder
	for _, e := range envioExcludes {
		if strings.ContainsAny(e, "*") {
			fmt.Fprintf(&ex, ` --exclude="%s"`, e)
		} else {
			fmt.Fprintf(&ex, " --exclude=%s", e)
		}
	}
	zip := fmt.Sprintf(`%%TEMP%%\ziv-envio-%d.zip`, id)
	return fmt.Sprintf(`tar.exe -a -c -f "%s"%s -C "<PASTA>" .`+"\n"+
		`curl.exe -sS --fail-with-body -X PUT -H "Content-Type: application/zip" --data-binary "@%s" "%s"`, zip, ex.String(), zip, endereco)
}

func envioGitBash(id int64, endereco string) string {
	var ex strings.Builder
	for _, e := range envioExcludes {
		fmt.Fprintf(&ex, " --exclude='%s'", e)
	}
	zip := fmt.Sprintf(`$TEMP/ziv-envio-%d.zip`, id)
	return fmt.Sprintf(`"$SYSTEMROOT/System32/tar.exe" -a -c -f "%s"%s -C "<PASTA>" .`+"\n"+
		`curl -sS --fail-with-body -T "%s" -H "Content-Type: application/zip" "%s"`, zip, ex.String(), zip, endereco)
}

func envioMacLinux(id int64, endereco string) string {
	var ex strings.Builder
	for _, e := range envioExcludes {
		if strings.HasPrefix(e, ".env") || strings.HasPrefix(e, "*") {
			fmt.Fprintf(&ex, " '%s' '*/%s'", e, e)
		} else {
			fmt.Fprintf(&ex, " '%s/*' '*/%s/*'", e, e)
		}
	}
	zip := fmt.Sprintf(`${TMPDIR:-/tmp}/ziv-envio-%d.zip`, id)
	return fmt.Sprintf(`(cd "<PASTA>" && zip -q -r -X "%s" . -x%s)`+"\n"+
		`curl -sS --fail-with-body -T "%s" -H "Content-Type: application/zip" "%s"`, zip, ex.String(), zip, endereco)
}

func (h *Handler) toolPrepararEnvio(ctx context.Context, _ *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, any, error) {
	person := mcpPersonFrom(ctx)
	if err := h.q.DeleteExpiredMCPUploads(ctx); err != nil {
		slog.Error("mcp delete expired uploads", "err", err)
	}
	open, err := h.q.CountOpenMCPUploads(ctx, person)
	if err != nil {
		slog.Error("mcp count uploads", "err", err)
		return toolFail(msgMCPAPIDown)
	}
	if open >= envioMaxOpen {
		return toolFail(msgEnvioLimit)
	}
	token, hash, err := newToken()
	if err != nil {
		return toolFail(msgMCPAPIDown)
	}
	until := time.Now().Add(envioTTL)
	id, err := h.q.CreateMCPUpload(ctx, sqlc.CreateMCPUploadParams{TokenHash: hash, PersonID: person, ExpiresAt: pgtype.Timestamptz{Time: until, Valid: true}})
	if err != nil {
		slog.Error("mcp create upload", "err", err)
		return toolFail(msgMCPAPIDown)
	}
	endereco := h.mcpLink("%s%s", envioRoute, token)
	out := map[string]any{
		"envio":             id,
		"endereco":          endereco,
		"vale_ate":          until.UTC().Format(time.RFC3339),
		"comando_windows":   envioWindows(id, endereco),
		"comando_git_bash":  envioGitBash(id, endereco),
		"comando_mac_linux": envioMacLinux(id, endereco),
	}
	text := fmt.Sprintf("Envio %d pronto. O endereço vale uma vez, até %s (30 minutos).\n"+
		"Troque <PASTA> pela pasta do app no computador da pessoa e rode os comandos do sistema dela: "+
		"comando_windows no Prompt de Comando (cmd), comando_git_bash no Git Bash do Windows, comando_mac_linux no Mac ou Linux. "+
		"O .zip vai para a pasta temporária, nunca para a pasta do app, e fica sem .env, chaves, .git, venv e node_modules.\n"+
		"Quando o curl mostrar \"recebido\": true, chame registrar_app com envio %d e apague o .zip temporário.",
		id, until.Local().Format("15:04"), id)
	return toolOK(text, out)
}
