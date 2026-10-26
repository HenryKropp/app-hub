package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/hostpath"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/scanner"
)

type similarApp struct {
	Name       string `json:"name"`
	AppID      int64  `json:"app_id,omitempty"`
	FolderName string `json:"folder_name,omitempty"`
}

type scanItem struct {
	scanner.Diagnosis
	FolderDisplay   string       `json:"folder_display"`
	RegisteredAppID int64        `json:"registered_app_id"`
	Similar         []similarApp `json:"similar"`
}

func (h *Handler) scan(w http.ResponseWriter, r *http.Request) {
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
		writeError(w, http.StatusForbidden, "Você não tem permissão para registrar apps.")
		return
	}
	var body struct {
		Path string `json:"path"`
		Mode string `json:"mode"` // "batch" or "single"
	}
	if !readJSON(w, r, &body) {
		return
	}
	local := hostpath.ToLocal(body.Path)
	if local == "" {
		writeError(w, http.StatusBadRequest, "Informe o caminho da pasta.")
		return
	}
	if info, err := os.Stat(local); err != nil || !info.IsDir() {
		writeError(w, http.StatusBadRequest, "Não encontrei essa pasta. Confira o caminho (ex.: C:\\Users\\voce\\Documentos\\Apps).")
		return
	}

	var diags []scanner.Diagnosis
	if body.Mode == "single" {
		d, err := scanner.Scan(local)
		if err != nil {
			internalError(w, "scan", err)
			return
		}
		diags = []scanner.Diagnosis{d}
	} else {
		var err error
		if diags, err = scanner.ScanBatch(local); err != nil {
			internalError(w, "scan batch", err)
			return
		}
	}

	existing, err := h.q.ListAppIdentities(r.Context())
	if err != nil {
		internalError(w, "list apps", err)
		return
	}
	// A similar app is named only if the person can see it, so a scan never
	// reveals a restricted app.
	links, err := h.q.ListAppLinks(r.Context())
	if err != nil {
		internalError(w, "list app links", err)
		return
	}
	visible := make(map[int64]bool, len(links))
	for _, l := range links {
		ref := access.App{ID: l.ID, AreaID: l.AreaID, OwnerID: l.OwnerID, BackupID: l.BackupID.Int64, Restricted: l.Restricted}
		visible[l.ID] = acc.For(ref).View
	}
	items := make([]scanItem, 0, len(diags))
	for i, d := range diags {
		item := scanItem{Diagnosis: d, FolderDisplay: hostpath.ToDisplay(d.FolderPath), Similar: []similarApp{}}
		for _, a := range existing {
			if a.FolderPath == d.FolderPath {
				if visible[a.ID] {
					item.RegisteredAppID = a.ID
				}
				continue
			}
			if visible[a.ID] && scanner.LooksDuplicate(a.Name, d.Manifest.Name) {
				item.Similar = append(item.Similar, similarApp{Name: a.Name, AppID: a.ID})
			}
		}
		for j, other := range diags {
			if i != j && scanner.LooksDuplicate(other.Manifest.Name, d.Manifest.Name) {
				item.Similar = append(item.Similar, similarApp{Name: other.Manifest.Name, FolderName: other.FolderName})
			}
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"root_display": hostpath.ToDisplay(local), "items": items})
}

type appSummary struct {
	sqlc.ListAppsRow
	FolderDisplay    string       `json:"folder_display"`
	CanRun           bool         `json:"can_run"`
	RememberedFolder string       `json:"remembered_folder"`
	Can              access.Can   `json:"can"`
	Shared           *sharedBrief `json:"shared"`
}

func (h *Handler) listApps(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "load access", err)
		return
	}
	apps, err := h.q.ListApps(r.Context())
	if err != nil {
		internalError(w, "list apps", err)
		return
	}
	folders, err := h.q.ListRunFolders(r.Context(), me.ID)
	if err != nil {
		internalError(w, "list run folders", err)
		return
	}
	remembered := make(map[int64]string, len(folders))
	for _, f := range folders {
		remembered[f.AppID] = f.Folder
	}
	shared, err := h.sharedBriefs(r)
	if err != nil {
		internalError(w, "list shared apps", err)
		return
	}
	out := make([]appSummary, 0, len(apps))
	for _, a := range apps {
		can := acc.For(access.App{ID: a.ID, AreaID: a.AreaID, OwnerID: a.OwnerID, BackupID: a.BackupID.Int64, Restricted: a.Restricted})
		if !can.View {
			continue
		}
		out = append(out, appSummary{ListAppsRow: a, FolderDisplay: hostpath.ToDisplay(a.FolderPath),
			CanRun: canRunKind(a.Kind, a.Entrypoint), RememberedFolder: remembered[a.ID], Can: can, Shared: shared[a.ID]})
	}
	writeJSON(w, http.StatusOK, out)
}

type appDetail struct {
	ID               int64              `json:"id"`
	FolderPath       string             `json:"folder_path"`
	FolderDisplay    string             `json:"folder_display"`
	OwnerID          int64              `json:"owner_id"`
	BackupID         *int64             `json:"backup_id"`
	Manifest         manifest.Manifest  `json:"manifest"`
	UpdatedAt        pgtype.Timestamptz `json:"updated_at"`
	CanRun           bool               `json:"can_run"`
	Version          *versionRef        `json:"version"`
	FolderReadable   bool               `json:"folder_readable"`
	RememberedFolder string             `json:"remembered_folder"`
	AreaID           int64              `json:"area_id"`
	Restricted       bool               `json:"restricted"`
	Can              access.Can         `json:"can"`
	DependsOn        []appLink          `json:"depends_on_apps,omitempty"`
	UsedBy           []appLink          `json:"used_by,omitempty"`
	Shared           *sharedBrief       `json:"shared"`
	NeedsRestart     bool               `json:"needs_restart,omitempty"` // PUT /shared: the running app must be turned on again
	NeedsPublish     bool               `json:"needs_publish,omitempty"` // PUT /shared: a new port only counts after a new publish
}

type appLink struct {
	ID      int64  `json:"id"`
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	Visible bool   `json:"visible"`
}

func toAppDetail(a sqlc.App) (appDetail, error) {
	d := appDetail{ID: a.ID, FolderPath: a.FolderPath, FolderDisplay: hostpath.ToDisplay(a.FolderPath), OwnerID: a.OwnerID, UpdatedAt: a.UpdatedAt,
		AreaID: a.AreaID, Restricted: a.Restricted}
	if a.BackupID.Valid {
		d.BackupID = &a.BackupID.Int64
	}
	err := json.Unmarshal(a.Manifest, &d.Manifest)
	d.CanRun = canRun(d.Manifest)
	return d, err
}

func (h *Handler) getApp(w http.ResponseWriter, r *http.Request) {
	a, me, _, _, ok := h.appFor(w, r)
	if !ok {
		return
	}
	var m manifest.Manifest
	if err := json.Unmarshal(a.Manifest, &m); err != nil {
		internalError(w, "decode manifest", err)
		return
	}
	if m.Kind == manifest.KindScript {
		if err := h.hydrateImportedScript(r.Context(), a, m); err != nil {
			internalError(w, "restore imported working copy", err)
			return
		}
	}
	h.writeDetail(w, r, me, a, http.StatusOK)
}

// writeDetail answers with the full ficha of a: version, remembered folder,
// what me can do and, for whoever sees the technical part, the links to other apps.
func (h *Handler) writeDetail(w http.ResponseWriter, r *http.Request, me sqlc.Person, a sqlc.App, status int) {
	h.writeDetailWith(w, r, me, a, status, nil)
}

// writeDetailWith is writeDetail with a last touch on the answer.
func (h *Handler) writeDetailWith(w http.ResponseWriter, r *http.Request, me sqlc.Person, a sqlc.App, status int, edit func(*appDetail)) {
	d, err := toAppDetail(a)
	if err != nil {
		internalError(w, "decode manifest", err)
		return
	}
	d.FolderReadable = true // known only after POST /check
	if v, err := h.q.LatestAppVersion(r.Context(), a.ID); err == nil {
		d.Version = &versionRef{Number: v.Number, CreatedAt: v.CreatedAt.Time}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		internalError(w, "latest version", err)
		return
	}
	if f, err := h.q.GetRunFolder(r.Context(), sqlc.GetRunFolderParams{PersonID: me.ID, AppID: a.ID}); err == nil {
		d.RememberedFolder = f
	} else if !errors.Is(err, pgx.ErrNoRows) {
		internalError(w, "run folder", err)
		return
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "load access", err)
		return
	}
	d.Can = acc.For(appRef(a))
	if d.Shared, err = h.sharedBriefOf(r, a.ID); err != nil {
		internalError(w, "shared brief", err)
		return
	}
	if d.Can.Tech {
		if d.DependsOn, d.UsedBy, err = h.appLinks(r.Context(), acc, a, d.Manifest); err != nil {
			internalError(w, "app links", err)
			return
		}
	}
	if edit != nil {
		edit(&d)
	}
	writeJSON(w, status, d)
}

// appLinks resolves depends_on ids to catalog apps and finds who depends on a.
// Apps the person cannot see keep their name but lose the link (Visible false).
func (h *Handler) appLinks(ctx context.Context, acc *access.Person, a sqlc.App, m manifest.Manifest) ([]appLink, []appLink, error) {
	rows, err := h.q.ListAppLinks(ctx)
	if err != nil {
		return nil, nil, err
	}
	bySlug := map[string]sqlc.ListAppLinksRow{}
	for _, x := range rows {
		bySlug[x.Slug] = x
	}
	visible := func(x sqlc.ListAppLinksRow) bool {
		return acc.For(access.App{ID: x.ID, AreaID: x.AreaID, OwnerID: x.OwnerID, BackupID: x.BackupID.Int64, Restricted: x.Restricted}).View
	}
	deps := []appLink{}
	for _, slug := range m.DependsOn {
		if x, ok := bySlug[slug]; ok {
			deps = append(deps, appLink{ID: x.ID, Slug: x.Slug, Name: x.Name, Visible: visible(x)})
		} else {
			deps = append(deps, appLink{Slug: slug, Name: slug})
		}
	}
	usedBy := []appLink{}
	for _, x := range rows {
		var ids []string
		if raw, err := json.Marshal(x.DependsOn); err == nil {
			_ = json.Unmarshal(raw, &ids)
		}
		if x.ID != a.ID && slices.Contains(ids, a.Slug) {
			usedBy = append(usedBy, appLink{ID: x.ID, Slug: x.Slug, Name: x.Name, Visible: visible(x)})
		}
	}
	return deps, usedBy, nil
}

// checkApp compares the app folder with its latest version (ADR 006). The ficha
// calls it right after opening, so the page never waits for OneDrive.
func (h *Handler) checkApp(w http.ResponseWriter, r *http.Request) {
	a, _, _, _, ok := h.appFor(w, r)
	if !ok {
		return
	}
	v, readable, err := h.checkVersion(context.WithoutCancel(r.Context()), a)
	if err != nil {
		slog.Error("version check", "app", a.ID, "err", err)
	}
	out := struct {
		FolderReadable bool        `json:"folder_readable"`
		Version        *versionRef `json:"version"`
	}{FolderReadable: readable}
	if v.ID != 0 {
		out.Version = &versionRef{Number: v.Number, CreatedAt: v.CreatedAt.Time}
	}
	writeJSON(w, http.StatusOK, out)
}

// saveApp publishes an app: it writes app-hub.yaml and FICHA.md into the app
// folder (never touching code) and records the app in the catalog.
func (h *Handler) saveApp(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "load access", err)
		return
	}
	var body appBody
	if !readJSON(w, r, &body) {
		return
	}
	folder := hostpath.ToLocal(body.FolderPath)
	// Who may publish is decided before anything touches the folder: a new app
	// needs the right to register, an existing one the right to change it.
	old, oldErr := h.q.GetAppByFolder(r.Context(), folder)
	if oldErr != nil && !errors.Is(oldErr, pgx.ErrNoRows) {
		internalError(w, "get app by folder", oldErr)
		return
	}
	if oldErr == nil {
		can := acc.For(appRef(old))
		if !can.Edit {
			writeError(w, http.StatusForbidden, "Você não pode mudar este app.")
			return
		}
		if !can.Access && handsOver(old, body) {
			writeError(w, http.StatusForbidden, handOverMsg)
			return
		}
	}
	if oldErr != nil && !acc.CanRegister() {
		writeError(w, http.StatusForbidden, "Você não tem permissão para registrar apps.")
		return
	}
	if info, err := os.Stat(folder); err != nil || !info.IsDir() {
		writeError(w, http.StatusBadRequest, "A pasta do app não foi encontrada.")
		return
	}
	var oldPtr *sqlc.App
	if oldErr == nil {
		oldPtr = &old
	}
	a, ok := h.writeApp(w, r, me, folder, body, oldPtr)
	if !ok {
		return
	}
	h.writeDetail(w, r, me, a, http.StatusCreated) // the same answer a GET gives
}

// updateApp saves one part of the ficha (the frontend sends the whole manifest
// with only that part changed). updated_at guards against a second, stale save.
func (h *Handler) updateApp(w http.ResponseWriter, r *http.Request) {
	a, me, _, can, ok := h.appFor(w, r)
	if !ok {
		return
	}
	if !can.Edit {
		writeError(w, http.StatusForbidden, "Você não pode mudar este app.")
		return
	}
	var body struct {
		appBody
		UpdatedAt time.Time `json:"updated_at"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if !can.Access && handsOver(a, body.appBody) {
		writeError(w, http.StatusForbidden, handOverMsg)
		return
	}
	if !body.UpdatedAt.Equal(a.UpdatedAt.Time) {
		writeError(w, http.StatusConflict, "Alguém mudou esta ficha agora há pouco. Recarregue para ver a versão nova.")
		return
	}
	if info, err := os.Stat(a.FolderPath); err != nil || !info.IsDir() {
		writeError(w, http.StatusConflict, "Não consegui ler a pasta do app agora. Nada foi salvo. Confira se o OneDrive está sincronizado e tente de novo.")
		return
	}
	saved, ok := h.writeApp(w, r, me, a.FolderPath, body.appBody, &a)
	if !ok {
		return
	}
	h.writeDetail(w, r, me, saved, http.StatusOK)
}

const handOverMsg = "Só quem cuida do acesso do app pode trocar o responsável, a reserva ou a área."

// handsOver: the body changes who cares for the app (owner, backup) or its
// area. Only someone with access control on the app may do that; an editor
// who did it could give themselves restore and access rights.
func handsOver(old sqlc.App, body appBody) bool {
	var backup int64
	if body.BackupID != nil {
		backup = *body.BackupID
	}
	return body.OwnerID != old.OwnerID || backup != old.BackupID.Int64 ||
		strings.TrimSpace(body.Manifest.Area) != old.Area
}

type appBody struct {
	FolderPath string            `json:"folder_path"`
	OwnerID    int64             `json:"owner_id"`
	BackupID   *int64            `json:"backup_id"`
	Manifest   manifest.Manifest `json:"manifest"`
}

// writeApp is the one way an app reaches the catalog: it validates, writes
// app-hub.yaml and FICHA.md into the folder (never touching code), records the
// app and what changed in the ficha. old is the app being updated, or nil.
// On failure it has already answered and returns false.
func (h *Handler) writeApp(w http.ResponseWriter, r *http.Request, me sqlc.Person, folder string, body appBody, old *sqlc.App) (sqlc.App, bool) {
	m := body.Manifest
	m.ManifestVersion = manifest.Version
	m.Name = strings.TrimSpace(m.Name)
	m.Summary = strings.TrimSpace(m.Summary)
	m.Area = strings.TrimSpace(m.Area)

	if old != nil {
		m.ID = old.Slug // an edit never renames the slug; validate against the real one
	}
	m.Owner, m.Backup = nil, nil
	if body.OwnerID != 0 {
		owner, err := h.q.GetPerson(r.Context(), body.OwnerID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			internalError(w, "get owner", err)
			return sqlc.App{}, false
		}
		if err == nil {
			m.Owner = &manifest.PersonRef{Name: owner.Name, Email: owner.Email}
		}
	}
	backupID := pgtype.Int8{}
	if body.BackupID != nil && *body.BackupID != 0 {
		backup, err := h.q.GetPerson(r.Context(), *body.BackupID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			internalError(w, "get backup", err)
			return sqlc.App{}, false
		}
		if err == nil {
			m.Backup = &manifest.PersonRef{Name: backup.Name, Email: backup.Email}
			backupID = pgtype.Int8{Int64: backup.ID, Valid: true}
		}
	}
	if errs := m.Validate(); len(errs) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": errs[0], "errors": errs})
		return sqlc.App{}, false
	}

	area, err := h.q.GetAreaByName(r.Context(), m.Area)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusBadRequest, "Escolha a área do app na lista.")
		return sqlc.App{}, false
	}
	if err != nil {
		internalError(w, "get area", err)
		return sqlc.App{}, false
	}

	base := m.ID
	if base == "" {
		base = manifest.Slugify(m.Name)
	}
	m.ID = base
	if old != nil {
		m.ID = old.Slug // an edit never renames the slug
	}
	for i := 2; old == nil; i++ {
		taken, err := h.q.SlugExists(r.Context(), sqlc.SlugExistsParams{Slug: m.ID, FolderPath: folder})
		if err != nil {
			internalError(w, "slug", err)
			return sqlc.App{}, false
		}
		if !taken {
			break
		}
		m.ID = fmt.Sprintf("%s-%d", base, i)
	}

	if err := manifest.WriteToFolder(folder, m); err != nil {
		if errors.Is(err, manifest.ErrForeignFile) {
			writeError(w, http.StatusConflict, "Não salvei: "+err.Error()+". Renomeie esse arquivo na pasta do app e tente de novo.")
			return sqlc.App{}, false
		}
		internalError(w, "write manifest", err)
		return sqlc.App{}, false
	}
	manifestJSON, err := json.Marshal(m)
	if err != nil {
		internalError(w, "encode manifest", err)
		return sqlc.App{}, false
	}
	a, err := h.q.UpsertApp(r.Context(), sqlc.UpsertAppParams{
		Slug: m.ID, Name: m.Name, Summary: m.Summary, Kind: m.Kind, Language: m.Language, Area: m.Area, AreaID: area.ID,
		FolderPath: folder, OwnerID: body.OwnerID, BackupID: backupID, Manifest: manifestJSON, CreatedBy: me.ID,
	})
	if err != nil {
		internalError(w, "upsert app", err)
		return sqlc.App{}, false
	}
	changes := manifest.Created()
	if old != nil {
		var oldM manifest.Manifest
		if err := json.Unmarshal(old.Manifest, &oldM); err != nil {
			internalError(w, "decode old manifest", err)
			return sqlc.App{}, false
		}
		changes = manifest.Diff(oldM, m)
	}
	if len(changes) > 0 {
		raw, _ := json.Marshal(changes)
		if err := h.q.InsertAppChange(r.Context(), sqlc.InsertAppChangeParams{AppID: a.ID, PersonID: pgtype.Int8{Int64: me.ID, Valid: true}, Changes: raw}); err != nil {
			internalError(w, "insert app change", err)
			return sqlc.App{}, false
		}
	}
	// Ficha change is recorded before the check, so "Ficha criada" precedes versão 1.
	if _, _, err := h.checkVersion(context.WithoutCancel(r.Context()), a); err != nil {
		slog.Error("version check", "app", a.ID, "err", err)
	}
	return a, true
}
