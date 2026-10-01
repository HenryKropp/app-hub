package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

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
	if _, ok := h.requireRole(w, r, "admin", "criador"); !ok {
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
	items := make([]scanItem, 0, len(diags))
	for i, d := range diags {
		item := scanItem{Diagnosis: d, FolderDisplay: hostpath.ToDisplay(d.FolderPath), Similar: []similarApp{}}
		for _, a := range existing {
			if a.FolderPath == d.FolderPath {
				item.RegisteredAppID = a.ID
				continue
			}
			if scanner.LooksDuplicate(a.Name, d.Manifest.Name) {
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
	FolderDisplay string `json:"folder_display"`
}

func (h *Handler) listApps(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentPerson(w, r); !ok {
		return
	}
	apps, err := h.q.ListApps(r.Context())
	if err != nil {
		internalError(w, "list apps", err)
		return
	}
	out := make([]appSummary, 0, len(apps))
	for _, a := range apps {
		out = append(out, appSummary{ListAppsRow: a, FolderDisplay: hostpath.ToDisplay(a.FolderPath)})
	}
	writeJSON(w, http.StatusOK, out)
}

type appDetail struct {
	ID             int64              `json:"id"`
	FolderPath     string             `json:"folder_path"`
	FolderDisplay  string             `json:"folder_display"`
	OwnerID        int64              `json:"owner_id"`
	BackupID       *int64             `json:"backup_id"`
	Manifest       manifest.Manifest  `json:"manifest"`
	UpdatedAt      pgtype.Timestamptz `json:"updated_at"`
	CanRun         bool               `json:"can_run"`
	Version        *versionRef        `json:"version"`
	FolderReadable bool               `json:"folder_readable"`
}

func toAppDetail(a sqlc.App) (appDetail, error) {
	d := appDetail{ID: a.ID, FolderPath: a.FolderPath, FolderDisplay: hostpath.ToDisplay(a.FolderPath), OwnerID: a.OwnerID, UpdatedAt: a.UpdatedAt}
	if a.BackupID.Valid {
		d.BackupID = &a.BackupID.Int64
	}
	err := json.Unmarshal(a.Manifest, &d.Manifest)
	d.CanRun = canRun(d.Manifest)
	return d, err
}

func (h *Handler) getApp(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentPerson(w, r); !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "App não encontrado.")
		return
	}
	a, err := h.q.GetApp(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "App não encontrado.")
		return
	}
	if err != nil {
		internalError(w, "get app", err)
		return
	}
	d, err := toAppDetail(a)
	if err != nil {
		internalError(w, "decode manifest", err)
		return
	}
	v, readable, err := h.checkVersion(context.WithoutCancel(r.Context()), a)
	if err != nil {
		slog.Error("version check", "app", a.ID, "err", err)
	}
	d.FolderReadable = readable
	if v.ID != 0 {
		d.Version = &versionRef{Number: v.Number, CreatedAt: v.CreatedAt.Time}
	}
	writeJSON(w, http.StatusOK, d)
}

// saveApp publishes an app: it writes app-hub.yaml and FICHA.md into the app
// folder (never touching code) and records the app in the catalog.
func (h *Handler) saveApp(w http.ResponseWriter, r *http.Request) {
	me, ok := h.requireRole(w, r, "admin", "criador")
	if !ok {
		return
	}
	var body struct {
		FolderPath string            `json:"folder_path"`
		OwnerID    int64             `json:"owner_id"`
		BackupID   *int64            `json:"backup_id"`
		Manifest   manifest.Manifest `json:"manifest"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	folder := hostpath.ToLocal(body.FolderPath)
	if info, err := os.Stat(folder); err != nil || !info.IsDir() {
		writeError(w, http.StatusBadRequest, "A pasta do app não foi encontrada.")
		return
	}
	m := body.Manifest
	m.ManifestVersion = manifest.Version
	m.Name = strings.TrimSpace(m.Name)
	m.Summary = strings.TrimSpace(m.Summary)
	m.Area = strings.TrimSpace(m.Area)

	m.Owner, m.Backup = nil, nil
	if body.OwnerID != 0 {
		owner, err := h.q.GetPerson(r.Context(), body.OwnerID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			internalError(w, "get owner", err)
			return
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
			return
		}
		if err == nil {
			m.Backup = &manifest.PersonRef{Name: backup.Name, Email: backup.Email}
			backupID = pgtype.Int8{Int64: backup.ID, Valid: true}
		}
	}
	if errs := m.Validate(); len(errs) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": errs[0], "errors": errs})
		return
	}

	base := m.ID
	if base == "" {
		base = manifest.Slugify(m.Name)
	}
	m.ID = base
	for i := 2; ; i++ {
		taken, err := h.q.SlugExists(r.Context(), sqlc.SlugExistsParams{Slug: m.ID, FolderPath: folder})
		if err != nil {
			internalError(w, "slug", err)
			return
		}
		if !taken {
			break
		}
		m.ID = fmt.Sprintf("%s-%d", base, i)
	}

	if err := manifest.WriteToFolder(folder, m); err != nil {
		if errors.Is(err, manifest.ErrForeignFile) {
			writeError(w, http.StatusConflict, "Não salvei: "+err.Error()+". Renomeie esse arquivo na pasta do app e tente de novo.")
			return
		}
		internalError(w, "write manifest", err)
		return
	}
	manifestJSON, err := json.Marshal(m)
	if err != nil {
		internalError(w, "encode manifest", err)
		return
	}
	old, oldErr := h.q.GetAppByFolder(r.Context(), folder)
	if oldErr != nil && !errors.Is(oldErr, pgx.ErrNoRows) {
		internalError(w, "get app by folder", oldErr)
		return
	}
	a, err := h.q.UpsertApp(r.Context(), sqlc.UpsertAppParams{
		Slug: m.ID, Name: m.Name, Summary: m.Summary, Kind: m.Kind, Language: m.Language, Area: m.Area,
		FolderPath: folder, OwnerID: body.OwnerID, BackupID: backupID, Manifest: manifestJSON, CreatedBy: me.ID,
	})
	if err != nil {
		internalError(w, "upsert app", err)
		return
	}
	changes := manifest.Created()
	if oldErr == nil {
		var oldM manifest.Manifest
		if err := json.Unmarshal(old.Manifest, &oldM); err != nil {
			internalError(w, "decode old manifest", err)
			return
		}
		changes = manifest.Diff(oldM, m)
	}
	if len(changes) > 0 {
		raw, _ := json.Marshal(changes)
		if err := h.q.InsertAppChange(r.Context(), sqlc.InsertAppChangeParams{AppID: a.ID, PersonID: me.ID, Changes: raw}); err != nil {
			internalError(w, "insert app change", err)
			return
		}
	}
	// Ficha change is recorded before the check, so "Ficha criada" precedes versão 1.
	if _, _, err := h.checkVersion(context.WithoutCancel(r.Context()), a); err != nil {
		slog.Error("version check", "app", a.ID, "err", err)
	}
	d, err := toAppDetail(a)
	if err != nil {
		internalError(w, "decode manifest", err)
		return
	}
	writeJSON(w, http.StatusCreated, d)
}
