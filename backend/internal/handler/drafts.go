package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Rascunho de versão (ADR 043). A draft holds the files and ficha changes of
// a version that is not live yet: it changes nothing that runs until it is
// published, and only then gets a number in app_versions.

const draftReplacedReason = "substituída por versão mais nova"

// createDraft keeps a new version as a draft (POST /api/apps/{id}/drafts).
// The body is the same as /sources, without updated_at: the ficha changes
// apply over the ficha as it is when the draft is published.
func (h *Handler) createDraft(w http.ResponseWriter, r *http.Request) {
	a, me, _, can, ok := h.appFor(w, r)
	if !ok {
		return
	}
	if !can.Edit {
		writeError(w, http.StatusForbidden, "Você não pode mudar este app.")
		return
	}
	var b struct {
		Files          map[string]manifest.InputFile `json:"files"`
		ConfirmSecrets bool                          `json:"confirm_secrets"`
		Note           string                        `json:"note"`
		Ficha          *fichaUpdate                  `json:"ficha"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 15<<20))
	dec.DisallowUnknownFields()
	if dec.Decode(&b) != nil || dec.Decode(&struct{}{}) != io.EOF {
		writeError(w, http.StatusBadRequest, "Não consegui ler os arquivos. Envie até 10 MB, só com files, confirm_secrets, note e ficha.")
		return
	}
	b.Note = strings.TrimSpace(b.Note)
	if utf8.RuneCountInString(b.Note) > 2000 {
		writeError(w, http.StatusBadRequest, "A nota da versão pode ter até 2.000 caracteres.")
		return
	}
	if !h.keepsCode(w, r, a) {
		return
	}
	var current manifest.Manifest
	if err := json.Unmarshal(a.Manifest, &current); err != nil {
		internalError(w, "decode manifest", err)
		return
	}
	m := current
	var ficha []byte
	if b.Ficha != nil {
		var ok bool
		if m, ok = validFicha(w, current, b.Ficha); !ok {
			return
		}
		ficha, _ = json.Marshal(b.Ficha)
	}
	if _, ok := h.checkSentFiles(w, r, m, b.Files, b.ConfirmSecrets); !ok {
		return
	}
	files, _ := json.Marshal(b.Files)
	ctx := r.Context()
	// One open draft per app: the newer one replaces the others.
	unlock := h.lockApp(a.ID)
	defer unlock()
	if err := h.q.CancelOpenDrafts(ctx, sqlc.CancelOpenDraftsParams{AppID: a.ID, Reason: draftReplacedReason}); err != nil {
		internalError(w, "cancel drafts", err)
		return
	}
	d, err := h.q.InsertDraft(ctx, sqlc.InsertDraftParams{
		AppID: a.ID, AuthorID: pgtype.Int8{Int64: me.ID, Valid: true}, Channel: originOf(ctx),
		Files: files, Ficha: ficha, Note: b.Note,
	})
	if err != nil {
		internalError(w, "insert draft", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"draft_id": d.ID, "status": d.Status})
}

// draftFromPath loads the app and draft in the URL for someone who can edit
// the app. ok is false when w was answered.
func (h *Handler) draftFromPath(w http.ResponseWriter, r *http.Request) (sqlc.App, sqlc.Person, sqlc.AppDraft, bool) {
	a, me, _, can, ok := h.appFor(w, r)
	if !ok {
		return a, me, sqlc.AppDraft{}, false
	}
	if !can.Edit {
		writeError(w, http.StatusForbidden, "Você não pode mudar este app.")
		return a, me, sqlc.AppDraft{}, false
	}
	id, err := strconv.ParseInt(r.PathValue("d"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "Rascunho não encontrado.")
		return a, me, sqlc.AppDraft{}, false
	}
	d, err := h.q.GetDraft(r.Context(), sqlc.GetDraftParams{AppID: a.ID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Rascunho não encontrado.")
		return a, me, d, false
	} else if err != nil {
		internalError(w, "get draft", err)
		return a, me, d, false
	}
	return a, me, d, true
}

// getDraft answers GET /api/apps/{id}/drafts/{d}.
func (h *Handler) getDraft(w http.ResponseWriter, r *http.Request) {
	_, _, d, ok := h.draftFromPath(w, r)
	if !ok {
		return
	}
	var number *int32
	if d.VersionNumber.Valid {
		number = &d.VersionNumber.Int32
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": d.ID, "status": d.Status, "status_reason": d.StatusReason,
		"version_number": number, "note": d.Note, "created_at": d.CreatedAt.Time,
	})
}

// publishDraftRoute is the manual door (POST /api/apps/{id}/drafts/{d}/publish):
// the draft goes live as "não testada".
func (h *Handler) publishDraftRoute(w http.ResponseWriter, r *http.Request) {
	a, me, d, ok := h.draftFromPath(w, r)
	if !ok {
		return
	}
	saved, v, changed, ok := h.publishDraft(w, r, a, me, d, false)
	if !ok {
		return
	}
	blocked, err := h.executionBlock(r.Context(), saved.ID)
	if err != nil {
		internalError(w, "approval", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": saved.ID, "version": v.Number, "changed": changed, "blocked_reason": blocked, "updated_at": saved.UpdatedAt.Time})
}

// publishDraft puts draft d live through applySources, with its test mark,
// and marks it no_ar. It refuses a draft that is no longer open. ok is false
// when w was answered.
func (h *Handler) publishDraft(w http.ResponseWriter, r *http.Request, a sqlc.App, me sqlc.Person, d sqlc.AppDraft, tested bool) (sqlc.App, sqlc.AppVersion, bool, bool) {
	if d.Status == "no_ar" || d.Status == "cancelada" {
		writeJSON(w, http.StatusConflict, map[string]string{"code": "draft_closed", "error": "Este rascunho não está mais aberto."})
		return a, sqlc.AppVersion{}, false, false
	}
	var files map[string]manifest.InputFile
	if err := json.Unmarshal(d.Files, &files); err != nil {
		internalError(w, "decode draft files", err)
		return a, sqlc.AppVersion{}, false, false
	}
	var ficha *fichaUpdate
	if len(d.Ficha) > 0 {
		ficha = &fichaUpdate{}
		if err := json.Unmarshal(d.Ficha, ficha); err != nil {
			internalError(w, "decode draft ficha", err)
			return a, sqlc.AppVersion{}, false, false
		}
	}
	saved, v, changed, ok := h.applySources(w, r, a, me, files, ficha, d.Note, pgtype.Bool{Bool: tested, Valid: true})
	if !ok {
		return saved, v, changed, false
	}
	if err := h.q.MarkDraftPublished(r.Context(), sqlc.MarkDraftPublishedParams{ID: d.ID, VersionNumber: pgtype.Int4{Int32: v.Number, Valid: true}}); err != nil {
		internalError(w, "mark draft", err)
		return saved, v, changed, false
	}
	return saved, v, changed, true
}
