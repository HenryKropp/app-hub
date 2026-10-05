package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

type versionItem struct {
	Number         int32            `json:"number"`
	Origin         string           `json:"origin"`
	RestoredFrom   *int32           `json:"restored_from"`
	AuthorName     string           `json:"author_name"`
	AuthorProbable bool             `json:"author_probable"`
	Description    string           `json:"description"`
	Summary        versions.Summary `json:"summary"`
	FilesChangedAt *time.Time       `json:"files_changed_at"`
	CreatedAt      time.Time        `json:"created_at"`
	Current        bool             `json:"current"`
}

type runItem struct {
	ID            int64  `json:"id"`
	Status        string `json:"status"`
	PersonName    string `json:"person_name"`
	VersionNumber *int32 `json:"version_number"`
}

type fichaItem struct {
	PersonName string            `json:"person_name"`
	Changes    []manifest.Change `json:"changes"`
}

type historyItem struct {
	Type    string       `json:"type"`
	At      time.Time    `json:"at"`
	Version *versionItem `json:"version,omitempty"`
	Run     *runItem     `json:"run,omitempty"`
	Ficha   *fichaItem   `json:"ficha,omitempty"`
}

type historyJSON struct {
	Items      []historyItem `json:"items"`
	More       bool          `json:"more"`
	NextBefore string        `json:"next_before"`
}

func int4Ptr(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	return &v.Int32
}

func newVersionItem(number int32, origin string, restoredFrom pgtype.Int4, author pgtype.Text, probable bool,
	description string, summary []byte, changedAt, createdAt pgtype.Timestamptz, current int32) versionItem {
	it := versionItem{Number: number, Origin: origin, RestoredFrom: int4Ptr(restoredFrom), AuthorName: author.String,
		AuthorProbable: probable, Description: description, FilesChangedAt: tsPtr(changedAt), CreatedAt: createdAt.Time,
		Current: number == current, Summary: versions.Summary{Binaries: []string{}}}
	_ = json.Unmarshal(summary, &it.Summary)
	return it
}

// appFromPath loads the app named in the URL, answering 404 itself.
func (h *Handler) appFromPath(w http.ResponseWriter, r *http.Request) (sqlc.App, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "App não encontrado.")
		return sqlc.App{}, false
	}
	a, err := h.q.GetApp(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "App não encontrado.")
		return sqlc.App{}, false
	}
	if err != nil {
		internalError(w, "get app", err)
		return sqlc.App{}, false
	}
	return a, true
}

func (h *Handler) appHistory(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	typ := r.URL.Query().Get("type")
	if typ == "" {
		typ = "all"
	}
	if typ != "all" && typ != "code" && typ != "runs" && typ != "ficha" {
		writeError(w, http.StatusBadRequest, "Filtro desconhecido.")
		return
	}
	before := time.Now().Add(time.Hour)
	if s := r.URL.Query().Get("before"); s != "" {
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Não entendi a data para carregar mais.")
			return
		}
		before = t
	}
	a, ok := h.appFromPath(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	var current int32
	if v, err := h.q.LatestAppVersion(ctx, a.ID); err == nil {
		current = v.Number
	}
	page := h.historyPage
	ts := pgtype.Timestamptz{Time: before, Valid: true}
	lim := int32(page + 1)
	var items []historyItem

	if typ == "all" || typ == "code" {
		rows, err := h.q.ListVersionsBefore(ctx, sqlc.ListVersionsBeforeParams{AppID: a.ID, Before: ts, Lim: lim})
		if err != nil {
			internalError(w, "list versions", err)
			return
		}
		for _, v := range rows {
			it := newVersionItem(v.Number, v.Origin, v.RestoredFrom, v.AuthorName, v.AuthorProbable, v.Description, v.Summary, v.FilesChangedAt, v.CreatedAt, current)
			items = append(items, historyItem{Type: "version", At: v.CreatedAt.Time, Version: &it})
		}
	}
	if typ == "all" || typ == "runs" {
		if err := h.q.ExpireStaleRuns(ctx); err != nil {
			internalError(w, "expire runs", err)
			return
		}
		rows, err := h.q.ListRunsBefore(ctx, sqlc.ListRunsBeforeParams{AppID: a.ID, Before: ts, SeeAll: h.roleKey(ctx, me) == "admin", PersonID: me.ID, Lim: lim})
		if err != nil {
			internalError(w, "list runs", err)
			return
		}
		for _, x := range rows {
			items = append(items, historyItem{Type: "run", At: x.CreatedAt.Time,
				Run: &runItem{ID: x.ID, Status: x.Status, PersonName: x.PersonName, VersionNumber: int4Ptr(x.VersionNumber)}})
		}
	}
	if typ == "all" || typ == "ficha" {
		rows, err := h.q.ListChangesBefore(ctx, sqlc.ListChangesBeforeParams{AppID: a.ID, Before: ts, Lim: lim})
		if err != nil {
			internalError(w, "list changes", err)
			return
		}
		for _, x := range rows {
			f := fichaItem{PersonName: x.PersonName, Changes: []manifest.Change{}}
			_ = json.Unmarshal(x.Changes, &f.Changes)
			items = append(items, historyItem{Type: "ficha", At: x.CreatedAt.Time, Ficha: &f})
		}
	}

	sort.SliceStable(items, func(i, j int) bool { return items[i].At.After(items[j].At) })
	out := historyJSON{Items: items}
	if len(items) > page {
		out.Items, out.More = items[:page], true
	}
	if out.Items == nil {
		out.Items = []historyItem{}
	}
	if out.More {
		out.NextBefore = out.Items[len(out.Items)-1].At.Format(time.RFC3339Nano)
	}
	writeJSON(w, http.StatusOK, out)
}
