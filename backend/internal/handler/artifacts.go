package handler

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/scanner"
)

const (
	// maxArtifactBytes is the largest HTML file an artifact may carry.
	maxArtifactBytes = 16 << 20
	// maxArtifactBody leaves room for base64 (about a third more) and JSON.
	maxArtifactBody = 24 << 20

	// artifactCSP makes the uploaded page run in an opaque origin: its scripts
	// run, but it cannot read the Hub's cookies, storage or API as the viewer.
	// Never add allow-same-origin here.
	artifactCSP = "sandbox allow-scripts allow-popups allow-forms allow-downloads"

	msgArtifactNotFound = "Artifact não encontrado."
	msgArtifactTooBig   = "O arquivo passa de 16 MB."
	msgArtifactNotHTML  = "Esse arquivo não parece uma página HTML."
)

type artifactSummaryJSON struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Summary    string    `json:"summary"`
	Kind       string    `json:"kind"`
	Tags       []string  `json:"tags"`
	AreaID     int64     `json:"area_id"`
	Area       string    `json:"area"`
	OwnerName  string    `json:"owner_name"`
	Restricted bool      `json:"restricted"`
	Version    int32     `json:"version"`
	UpdatedAt  time.Time `json:"updated_at"`
	Favorite   bool      `json:"favorite"`
}

type artifactVersionJSON struct {
	Number       int32     `json:"number"`
	Kind         string    `json:"kind"`
	URL          string    `json:"url"`
	FileName     string    `json:"file_name"`
	Size         int64     `json:"size"`
	Note         string    `json:"note"`
	AuthorName   string    `json:"author_name"`
	RestoredFrom *int32    `json:"restored_from"`
	CreatedAt    time.Time `json:"created_at"`
	Embeddable   bool      `json:"embeddable"` // a link whose site lets the Ziv show it inside the page
}

type artifactDetailJSON struct {
	artifactSummaryJSON
	OwnerID    int64                 `json:"owner_id"`
	BackupID   *int64                `json:"backup_id"`
	BackupName string                `json:"backup_name"`
	Origin     string                `json:"origin"`
	Can        access.Can            `json:"can"`
	Versions   []artifactVersionJSON `json:"versions"`
	Current    int32                 `json:"current"`
}

// artifactInput is one version as sent by the browser (create or new version).
type artifactInput struct {
	Kind           string `json:"kind"`
	URL            string `json:"url"`
	FileName       string `json:"file_name"`
	ContentBase64  string `json:"content_base64"`
	Note           string `json:"note"`
	ConfirmSecrets bool   `json:"confirm_secrets"`
}

// readArtifactJSON reads a body that may carry a 16 MB file in base64;
// readJSON's limit is too small for it.
func readArtifactJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxArtifactBody)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusBadRequest, msgArtifactTooBig)
			return false
		}
		writeError(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return false
	}
	return true
}

// cleanFileName keeps only the last part of the name sent by the browser.
func cleanFileName(name string) string {
	return strings.TrimSpace(path.Base(strings.ReplaceAll(strings.TrimSpace(name), `\`, "/")))
}

// validateArtifactInput checks one version. It returns "" when it is fine.
func validateArtifactInput(kind, rawURL, fileName string, content []byte, dev bool) string {
	switch kind {
	case "link":
		u, err := url.Parse(rawURL)
		if err != nil || u.Host == "" || u.User != nil || strings.ContainsAny(rawURL, " \t\r\n") {
			return "Cole um link que comece com https://."
		}
		local := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"
		if !(u.Scheme == "https" || dev && u.Scheme == "http" && local) {
			return "Cole um link que comece com https://."
		}
		return ""
	case "file":
		ext := strings.ToLower(path.Ext(fileName))
		if ext != ".html" && ext != ".htm" {
			return "Envie um arquivo .html."
		}
		if len(content) > maxArtifactBytes {
			return msgArtifactTooBig
		}
		if len(content) == 0 || !utf8.Valid(content) || slices.Contains(content, 0) {
			return msgArtifactNotHTML
		}
		return ""
	default:
		return "Escolha link ou arquivo."
	}
}

// normalizeTags trims, lowercases and drops empty and repeated tags.
func normalizeTags(in []string) []string {
	out := []string{}
	for _, t := range in {
		t = strings.ToLower(strings.TrimSpace(t))
		if t != "" && !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out
}

// preparedVersion is a validated version ready to store.
type preparedVersion struct {
	kind       string
	url        string
	fileName   string
	content    []byte
	note       string
	embeddable bool
}

// prepareVersion validates the input and runs the secrets policy, writing the
// error itself. Nothing is stored here.
func (h *Handler) prepareVersion(w http.ResponseWriter, r *http.Request, in artifactInput) (preparedVersion, bool) {
	p := preparedVersion{kind: in.Kind, note: strings.TrimSpace(in.Note)}
	if in.Kind == "link" {
		p.url = strings.TrimSpace(in.URL)
	} else if in.Kind == "file" {
		p.fileName = cleanFileName(in.FileName)
		data, err := base64.StdEncoding.DecodeString(in.ContentBase64)
		if err != nil {
			writeError(w, http.StatusBadRequest, msgArtifactNotHTML)
			return p, false
		}
		p.content = data
	}
	if msg := validateArtifactInput(p.kind, p.url, p.fileName, p.content, h.cfg.DevMode); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return p, false
	}
	if p.kind == "link" {
		// Network call, before any transaction: up to 5 s.
		p.embeddable = !h.zivHost(p.url) && h.embedCheck(r.Context(), p.url, h.zivHost)
		return p, true
	}
	// Same policy as script import: only a count leaves the handler.
	secretCount := len(scanner.FindSecrets(p.fileName, string(p.content)))
	if secretCount > 0 && h.workspace != nil {
		settings, err := h.securityConfig(r.Context())
		if err != nil {
			internalError(w, "artifact security policy", err)
			return p, false
		}
		if settings.BlockSecrets {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"code": "artifact_secrets_blocked", "error": "Possíveis senhas ou chaves no arquivo. Retire as credenciais antes de enviar; a política da empresa não permite ignorar este bloqueio.", "secret_count": secretCount})
			return p, false
		}
	}
	if secretCount > 0 && !in.ConfirmSecrets {
		writeJSON(w, http.StatusConflict, map[string]any{
			"code":         "artifact_secrets_confirmation_required",
			"error":        "Possíveis senhas ou chaves no arquivo. Revise o arquivo ou confirme o envio sem alterações.",
			"secret_count": secretCount,
		})
		return p, false
	}
	return p, true
}

// addVersion stores p as the next version of the artifact and makes the
// artifact's kind follow it. Run inside a transaction. The artifact row is
// locked first so two uploads at once take numbers one after the other.
func addVersion(ctx context.Context, q *sqlc.Queries, artifactID int64, p preparedVersion, authorID int64, restoredFrom pgtype.Int4, blob pgtype.Text, size int64) error {
	if err := q.LockArtifact(ctx, artifactID); err != nil {
		return err
	}
	next := int32(1)
	if last, err := q.LatestArtifactVersion(ctx, artifactID); err == nil {
		next = last.Number + 1
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	params := sqlc.CreateArtifactVersionParams{ArtifactID: artifactID, Number: next, Kind: p.kind, FileName: p.fileName,
		Size: size, Note: p.note, RestoredFrom: restoredFrom, AuthorID: pgInt8(authorID), BlobSha256: blob}
	if p.kind == "link" {
		params.Url = pgtype.Text{String: p.url, Valid: true}
		params.Embeddable = p.embeddable
	}
	if p.kind == "file" && !blob.Valid {
		sum := sha256.Sum256(p.content)
		sha := hex.EncodeToString(sum[:])
		if err := q.PutCodeBlob(ctx, sqlc.PutCodeBlobParams{Sha256: sha, Content: p.content, Size: int64(len(p.content))}); err != nil {
			return err
		}
		params.BlobSha256 = pgtype.Text{String: sha, Valid: true}
		params.Size = int64(len(p.content))
	}
	if _, err := q.CreateArtifactVersion(ctx, params); err != nil {
		return err
	}
	return q.SetArtifactKind(ctx, sqlc.SetArtifactKindParams{ID: artifactID, Kind: p.kind})
}

// artifactFor loads the artifact in the URL and what me can do with it. An
// artifact me cannot see answers 404, the same as one that does not exist.
func (h *Handler) artifactFor(w http.ResponseWriter, r *http.Request) (sqlc.Artifact, sqlc.Person, *access.Person, access.Can, bool) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return sqlc.Artifact{}, me, nil, access.Can{}, false
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, msgArtifactNotFound)
		return sqlc.Artifact{}, me, nil, access.Can{}, false
	}
	a, err := h.q.GetArtifact(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, msgArtifactNotFound)
		return sqlc.Artifact{}, me, nil, access.Can{}, false
	}
	if err != nil {
		internalError(w, "get artifact", err)
		return sqlc.Artifact{}, me, nil, access.Can{}, false
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "load access", err)
		return sqlc.Artifact{}, me, nil, access.Can{}, false
	}
	can := acc.ForArtifact(artifactRef(a))
	if !can.View {
		// Zero values, so a careless caller cannot leak the hidden artifact.
		writeError(w, http.StatusNotFound, msgArtifactNotFound)
		return sqlc.Artifact{}, me, nil, access.Can{}, false
	}
	return a, me, acc, can, true
}

func (h *Handler) artifactDetail(ctx context.Context, a sqlc.Artifact, can access.Can) (artifactDetailJSON, error) {
	area, err := h.q.GetArea(ctx, a.AreaID)
	if err != nil {
		return artifactDetailJSON{}, err
	}
	owner, err := h.q.GetPerson(ctx, a.OwnerID)
	if err != nil {
		return artifactDetailJSON{}, err
	}
	d := artifactDetailJSON{
		artifactSummaryJSON: artifactSummaryJSON{ID: a.ID, Name: a.Name, Summary: a.Summary, Kind: a.Kind, Tags: a.Tags,
			AreaID: a.AreaID, Area: area.Name, OwnerName: owner.Name, Restricted: a.Restricted, UpdatedAt: a.UpdatedAt.Time},
		OwnerID: a.OwnerID, Origin: a.Origin, Can: can, Versions: []artifactVersionJSON{},
	}
	if d.Tags == nil {
		d.Tags = []string{}
	}
	if a.BackupID.Valid {
		backup, err := h.q.GetPerson(ctx, a.BackupID.Int64)
		if err != nil {
			return d, err
		}
		id := a.BackupID.Int64
		d.BackupID, d.BackupName = &id, backup.Name
	}
	versions, err := h.q.ListArtifactVersions(ctx, a.ID)
	if err != nil {
		return d, err
	}
	for _, v := range versions {
		out := artifactVersionJSON{Number: v.Number, Kind: v.Kind, URL: v.Url.String, FileName: v.FileName, Size: v.Size,
			Note: v.Note, AuthorName: v.AuthorName.String, CreatedAt: v.CreatedAt.Time, Embeddable: v.Embeddable}
		if v.RestoredFrom.Valid {
			n := v.RestoredFrom.Int32
			out.RestoredFrom = &n
		}
		d.Versions = append(d.Versions, out)
	}
	if len(d.Versions) > 0 {
		d.Current = d.Versions[0].Number
	}
	d.Version = d.Current
	return d, nil
}

// writeArtifactDetail reloads the artifact and answers with its detail.
func (h *Handler) writeArtifactDetail(w http.ResponseWriter, r *http.Request, me sqlc.Person, id int64, status int) {
	ctx := r.Context()
	a, err := h.q.GetArtifact(ctx, id)
	if err != nil {
		internalError(w, "get artifact", err)
		return
	}
	acc, err := h.loadAccess(ctx, me)
	if err != nil {
		internalError(w, "load access", err)
		return
	}
	d, err := h.artifactDetail(ctx, a, acc.ForArtifact(artifactRef(a)))
	if err != nil {
		internalError(w, "artifact detail", err)
		return
	}
	if d.Favorite, err = h.isFavorite(ctx, me.ID, "artifact", a.ID); err != nil {
		internalError(w, "favorite", err)
		return
	}
	writeJSON(w, status, d)
}

func (h *Handler) listArtifacts(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "load access", err)
		return
	}
	rows, err := h.q.ListArtifacts(r.Context())
	if err != nil {
		internalError(w, "list artifacts", err)
		return
	}
	stars, err := h.favoriteSet(r.Context(), me.ID)
	if err != nil {
		internalError(w, "list favorites", err)
		return
	}
	out := []artifactSummaryJSON{}
	for _, a := range rows {
		ref := access.App{ID: a.ID, AreaID: a.AreaID, OwnerID: a.OwnerID, BackupID: a.BackupID.Int64, Restricted: a.Restricted}
		if !acc.ForArtifact(ref).View {
			continue
		}
		tags := a.Tags
		if tags == nil {
			tags = []string{}
		}
		out = append(out, artifactSummaryJSON{ID: a.ID, Name: a.Name, Summary: a.Summary, Kind: a.Kind, Tags: tags,
			AreaID: a.AreaID, Area: a.AreaName, OwnerName: a.OwnerName, Restricted: a.Restricted,
			Version: a.CurrentVersion, UpdatedAt: a.UpdatedAt.Time, Favorite: stars[favoriteKey{"artifact", a.ID}]})
	}
	slices.SortStableFunc(out, func(x, y artifactSummaryJSON) int { return y.UpdatedAt.Compare(x.UpdatedAt) })
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) createArtifact(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "load access", err)
		return
	}
	if !acc.CanRegister() {
		writeError(w, http.StatusForbidden, "Você não tem permissão para registrar artifacts.")
		return
	}
	var b struct {
		artifactInput
		Name    string   `json:"name"`
		Summary string   `json:"summary"`
		Tags    []string `json:"tags"`
		AreaID  int64    `json:"area_id"`
	}
	if !readArtifactJSON(w, r, &b) {
		return
	}
	ctx := r.Context()
	name := strings.TrimSpace(b.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "Dê um nome ao artifact.")
		return
	}
	if _, err := h.q.GetArea(ctx, b.AreaID); err != nil {
		writeError(w, http.StatusBadRequest, "Escolha uma área existente.")
		return
	}
	p, ok := h.prepareVersion(w, r, b.artifactInput)
	if !ok {
		return
	}
	var id int64
	err = h.q.InTx(ctx, func(q *sqlc.Queries) error {
		a, err := q.CreateArtifact(ctx, sqlc.CreateArtifactParams{Name: name, Summary: strings.TrimSpace(b.Summary), Kind: p.kind,
			Tags: normalizeTags(b.Tags), AreaID: b.AreaID, OwnerID: me.ID, Origin: originOf(ctx), CreatedBy: me.ID})
		if err != nil {
			return err
		}
		id = a.ID
		return addVersion(ctx, q, a.ID, p, me.ID, pgtype.Int4{}, pgtype.Text{}, 0)
	})
	if err != nil {
		internalError(w, "create artifact", err)
		return
	}
	h.writeArtifactDetail(w, r, me, id, http.StatusCreated)
}

func (h *Handler) getArtifact(w http.ResponseWriter, r *http.Request) {
	a, me, _, can, ok := h.artifactFor(w, r)
	if !ok {
		return
	}
	d, err := h.artifactDetail(r.Context(), a, can)
	if err != nil {
		internalError(w, "artifact detail", err)
		return
	}
	if d.Favorite, err = h.isFavorite(r.Context(), me.ID, "artifact", a.ID); err != nil {
		internalError(w, "favorite", err)
		return
	}
	// Opening is recorded only here (not on save). A failure never blocks the page.
	if err := h.q.RecordArtifactOpen(r.Context(), sqlc.RecordArtifactOpenParams{ArtifactID: a.ID, PersonID: me.ID}); err != nil {
		slog.Error("record artifact open", "err", err)
	}
	writeJSON(w, http.StatusOK, d)
}

func (h *Handler) updateArtifact(w http.ResponseWriter, r *http.Request) {
	a, me, _, can, ok := h.artifactFor(w, r)
	if !ok {
		return
	}
	if !can.Edit {
		writeError(w, http.StatusForbidden, "Você não pode editar este artifact.")
		return
	}
	var b struct {
		Name     string   `json:"name"`
		Summary  string   `json:"summary"`
		Tags     []string `json:"tags"`
		OwnerID  *int64   `json:"owner_id"`
		BackupID *int64   `json:"backup_id"` // 0 = no backup
		AreaID   *int64   `json:"area_id"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	ctx := r.Context()
	name := strings.TrimSpace(b.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "Dê um nome ao artifact.")
		return
	}
	care := b.OwnerID != nil || b.BackupID != nil || b.AreaID != nil
	if care && !can.Access {
		writeError(w, http.StatusForbidden, "Só quem cuida do artifact muda o dono, o substituto ou a área.")
		return
	}
	careParams := sqlc.UpdateArtifactCareParams{ID: a.ID, OwnerID: a.OwnerID, BackupID: a.BackupID, AreaID: a.AreaID}
	if b.OwnerID != nil {
		if _, err := h.q.GetPerson(ctx, *b.OwnerID); err != nil {
			writeError(w, http.StatusBadRequest, "Escolha o dono na lista.")
			return
		}
		careParams.OwnerID = *b.OwnerID
	}
	if b.BackupID != nil {
		careParams.BackupID = pgtype.Int8{}
		if *b.BackupID != 0 {
			if _, err := h.q.GetPerson(ctx, *b.BackupID); err != nil {
				writeError(w, http.StatusBadRequest, "Escolha o substituto na lista.")
				return
			}
			careParams.BackupID = pgInt8(*b.BackupID)
		}
	}
	if b.AreaID != nil {
		if _, err := h.q.GetArea(ctx, *b.AreaID); err != nil {
			writeError(w, http.StatusBadRequest, "Escolha uma área existente.")
			return
		}
		careParams.AreaID = *b.AreaID
	}
	err := h.q.InTx(ctx, func(q *sqlc.Queries) error {
		if _, err := q.UpdateArtifactFicha(ctx, sqlc.UpdateArtifactFichaParams{ID: a.ID, Name: name,
			Summary: strings.TrimSpace(b.Summary), Tags: normalizeTags(b.Tags)}); err != nil {
			return err
		}
		if care {
			_, err := q.UpdateArtifactCare(ctx, careParams)
			return err
		}
		return nil
	})
	if err != nil {
		internalError(w, "update artifact", err)
		return
	}
	h.writeArtifactDetail(w, r, me, a.ID, http.StatusOK)
}

func (h *Handler) addArtifactVersion(w http.ResponseWriter, r *http.Request) {
	a, me, _, can, ok := h.artifactFor(w, r)
	if !ok {
		return
	}
	if !can.Edit {
		writeError(w, http.StatusForbidden, "Você não pode mandar uma nova versão deste artifact.")
		return
	}
	var in artifactInput
	if !readArtifactJSON(w, r, &in) {
		return
	}
	p, ok := h.prepareVersion(w, r, in)
	if !ok {
		return
	}
	ctx := r.Context()
	err := h.q.InTx(ctx, func(q *sqlc.Queries) error {
		return addVersion(ctx, q, a.ID, p, me.ID, pgtype.Int4{}, pgtype.Text{}, 0)
	})
	if err != nil {
		internalError(w, "add artifact version", err)
		return
	}
	h.writeArtifactDetail(w, r, me, a.ID, http.StatusOK)
}

// artifactVersionNumber loads version raw (the latest when empty), answering 404 itself.
func (h *Handler) artifactVersionNumber(w http.ResponseWriter, r *http.Request, raw string, artifactID int64) (sqlc.ArtifactVersion, bool) {
	var (
		v   sqlc.ArtifactVersion
		err error
	)
	if raw == "" {
		v, err = h.q.LatestArtifactVersion(r.Context(), artifactID)
	} else {
		n, perr := strconv.ParseInt(raw, 10, 32)
		if perr != nil {
			writeError(w, http.StatusNotFound, "Versão não encontrada.")
			return v, false
		}
		v, err = h.q.GetArtifactVersion(r.Context(), sqlc.GetArtifactVersionParams{ArtifactID: artifactID, Number: int32(n)})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Versão não encontrada.")
		return v, false
	}
	if err != nil {
		internalError(w, "get artifact version", err)
		return v, false
	}
	return v, true
}

func (h *Handler) restoreArtifactVersion(w http.ResponseWriter, r *http.Request) {
	a, me, _, can, ok := h.artifactFor(w, r)
	if !ok {
		return
	}
	if !can.Restore {
		writeError(w, http.StatusForbidden, "Você não pode voltar versões deste artifact.")
		return
	}
	old, ok := h.artifactVersionNumber(w, r, r.PathValue("n"), a.ID)
	if !ok {
		return
	}
	ctx := r.Context()
	p := preparedVersion{kind: old.Kind, url: old.Url.String, fileName: old.FileName, embeddable: old.Embeddable && !h.zivHost(old.Url.String)}
	err := h.q.InTx(ctx, func(q *sqlc.Queries) error {
		return addVersion(ctx, q, a.ID, p, me.ID, pgtype.Int4{Int32: old.Number, Valid: true}, old.BlobSha256, old.Size)
	})
	if err != nil {
		internalError(w, "restore artifact version", err)
		return
	}
	h.writeArtifactDetail(w, r, me, a.ID, http.StatusOK)
}

func (h *Handler) deleteArtifact(w http.ResponseWriter, r *http.Request) {
	a, _, acc, can, ok := h.artifactFor(w, r)
	if !ok {
		return
	}
	if !can.Access && !acc.Has(access.ManageRoles) {
		writeError(w, http.StatusForbidden, "Só quem cuida do artifact pode apagá-lo.")
		return
	}
	err := h.q.InTx(r.Context(), func(q *sqlc.Queries) error {
		if err := q.DeleteItemFavorites(r.Context(), sqlc.DeleteItemFavoritesParams{Kind: "artifact", ItemID: a.ID}); err != nil {
			return err
		}
		return q.DeleteArtifact(r.Context(), a.ID)
	})
	if err != nil {
		internalError(w, "delete artifact", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// artifactContent serves the uploaded HTML. It is untrusted third-party code:
// the CSP sandbox (without allow-same-origin) puts it in an opaque origin, so
// even opened directly it cannot act as the viewer on the Hub.
func (h *Handler) artifactContent(w http.ResponseWriter, r *http.Request) {
	a, _, _, _, ok := h.artifactFor(w, r)
	if !ok {
		return
	}
	v, ok := h.artifactVersionNumber(w, r, r.URL.Query().Get("v"), a.ID)
	if !ok {
		return
	}
	if v.Kind != "file" || !v.BlobSha256.Valid {
		writeError(w, http.StatusNotFound, "Essa versão é um link, não um arquivo.")
		return
	}
	content, err := h.q.GetCodeBlob(r.Context(), v.BlobSha256.String)
	if err != nil {
		internalError(w, "artifact content", err)
		return
	}
	hd := w.Header()
	hd.Set("Content-Type", "text/html; charset=utf-8")
	// Only the Ziv's own page may frame it (overrides NoFraming).
	hd.Set("Content-Security-Policy", artifactCSP+"; frame-ancestors 'self'")
	hd.Set("X-Frame-Options", "SAMEORIGIN")
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Referrer-Policy", "no-referrer")
	hd.Set("Cache-Control", "private, no-store")
	if r.URL.Query().Get("download") == "1" {
		name := v.FileName
		if name == "" {
			name = "artifact.html"
		}
		disposition := mime.FormatMediaType("attachment", map[string]string{"filename": name})
		if disposition == "" {
			disposition = `attachment; filename="artifact.html"`
		}
		hd.Set("Content-Disposition", disposition)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}
