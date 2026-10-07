package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/scanner"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy"
	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
	"github.com/HenryKropp/app-hub/backend/internal/shared/readiness"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

const (
	busyMsg          = "Já tem uma mudança em andamento neste app. Espere terminar e tente de novo."
	engineDownMsg    = "O servidor de apps não está disponível. Confira se o Podman está ligado."
	folderUnreadable = "Não consegui ler a pasta do app. Confira se ela existe e está sincronizada."
)

// knownAcks are the checklist warnings a person may mark "estou ciente".
var knownAcks = []string{"file_data", "windows_paths", "secrets", "container_limits"}

// SharedSettings is the body of PUT /shared: the manifest's shared block plus
// the "estou ciente" marks.
type SharedSettings struct {
	Enabled    bool              `json:"enabled"`
	AlwaysOn   bool              `json:"always_on"`
	IdleMin    int               `json:"idle_minutes"`
	MemoryMB   int               `json:"memory_mb"`
	CPUs       float64           `json:"cpus"`
	Port       int               `json:"port"`
	HealthPath string            `json:"health_path"`
	Database   string            `json:"database"`
	Command    string            `json:"command"`
	Env        []manifest.EnvVar `json:"env"`
	Acks       []string          `json:"acks"`
	UpdatedAt  time.Time         `json:"updated_at"` // the same guard against a stale save as the ficha
}

type sharedState struct {
	Status        string   `json:"status"`
	StatusMessage string   `json:"status_message"`
	StoppedBy     string   `json:"stopped_by"`
	AlwaysOn      bool     `json:"always_on"`
	IdleMinutes   int32    `json:"idle_minutes"`
	MemoryMB      int32    `json:"memory_mb"`
	CPUs          float64  `json:"cpus"`
	WantsDB       bool     `json:"wants_db"`
	URL           string   `json:"url"`
	Acks          []string `json:"acks"`
	HasPrevious   bool     `json:"has_previous"`
}

// sharedBrief is what the catalog and the ficha show about a shared app.
type sharedBrief struct {
	Status string `json:"status"`
	URL    string `json:"url"`
	Slug   string `json:"slug"`
}

// deploymentOut is one deployment as the API shows it. The build log and the
// technical ids only go to whoever sees the technical part.
type deploymentOut struct {
	ID            int64              `json:"id"`
	Kind          string             `json:"kind"`
	Status        string             `json:"status"`
	ErrorMessage  string             `json:"error_message"`
	VersionNumber *int32             `json:"version_number"`
	CreatedByName string             `json:"created_by_name"`
	CreatedAt     pgtype.Timestamptz `json:"created_at"`
	FinishedAt    pgtype.Timestamptz `json:"finished_at"`
	ExpiresAt     pgtype.Timestamptz `json:"expires_at"`
	BuildLog      *string            `json:"build_log,omitempty"`
	Port          *int32             `json:"port,omitempty"`
}

func toDeploymentOut(d sqlc.SharedDeployment, tech bool) deploymentOut {
	out := deploymentOut{ID: d.ID, Kind: d.Kind, Status: d.Status, ErrorMessage: d.ErrorMessage,
		CreatedAt: d.CreatedAt, FinishedAt: d.FinishedAt, ExpiresAt: d.ExpiresAt}
	if tech {
		out.BuildLog, out.Port = &d.BuildLog, &d.Port
	}
	return out
}

// deploymentOutFull is toDeploymentOut plus the version number and the name of
// who started it, looked up (the list endpoint gets them from its join).
func (h *Handler) deploymentOutFull(r *http.Request, d sqlc.SharedDeployment, tech bool) deploymentOut {
	out := toDeploymentOut(d, tech)
	if d.VersionID.Valid {
		if v, err := h.q.GetAppVersionByID(r.Context(), d.VersionID.Int64); err == nil {
			n := v.Number
			out.VersionNumber = &n
		}
	}
	if d.CreatedBy.Valid {
		if p, err := h.q.GetPerson(r.Context(), d.CreatedBy.Int64); err == nil {
			out.CreatedByName = p.Name
		}
	}
	return out
}

// sharedError maps the deploy service's errors to answers in Portuguese.
func sharedError(w http.ResponseWriter, err error) {
	var miss deploy.MissingSettings
	switch {
	case errors.Is(err, deploy.ErrBusy):
		writeError(w, http.StatusConflict, busyMsg)
	case errors.As(err, &miss):
		writeError(w, http.StatusBadRequest, miss.Error())
	case errors.Is(err, engine.ErrUnavailable):
		slog.Warn("shared: engine unavailable", "err", err)
		writeError(w, http.StatusServiceUnavailable, engineDownMsg)
	case errors.Is(err, deploy.ErrNoVault):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, deploy.ErrNoPrevious), errors.Is(err, deploy.ErrNotShared), errors.Is(err, deploy.ErrBadSlug):
		writeError(w, http.StatusBadRequest, err.Error())
	case isPortuguese(err):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		internalError(w, "shared", err)
	}
}

// isPortuguese: the service's own messages start with a capital letter and end
// with a period; technical errors (pgx, exec, wrapped) do not.
func isPortuguese(err error) bool {
	m := err.Error()
	return m != "" && m[0] >= 'A' && m[0] <= 'Z' && strings.HasSuffix(m, ".") && !strings.Contains(m, ": ")
}

func canView(c access.Can) bool    { return c.View }
func canRestore(c access.Can) bool { return c.Restore }
func canTech(c access.Can) bool    { return c.Tech }

// sharedFor loads the app, checks a permission, and the shared row (nil if the
// app was never shared). On failure it has already answered.
func (h *Handler) sharedFor(w http.ResponseWriter, r *http.Request, need func(access.Can) bool) (sqlc.App, sqlc.Person, access.Can, *sqlc.SharedApp, bool) {
	if !h.sharedReady(w, r) {
		return sqlc.App{}, sqlc.Person{}, access.Can{}, nil, false
	}
	a, me, _, can, ok := h.appFor(w, r)
	if !ok {
		return a, me, can, nil, false
	}
	if !need(can) {
		writeError(w, http.StatusForbidden, "Você não tem permissão para isso.")
		return a, me, can, nil, false
	}
	sa, err := h.q.GetSharedApp(r.Context(), a.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, me, can, nil, true
	}
	if err != nil {
		internalError(w, "get shared app", err)
		return a, me, can, nil, false
	}
	return a, me, can, &sa, true
}

// sharedRow is like sharedFor but answers ErrNotShared when the app is not
// shared (never was, or was unshared).
func (h *Handler) sharedRow(w http.ResponseWriter, r *http.Request, need func(access.Can) bool) (sqlc.App, sqlc.Person, access.Can, sqlc.SharedApp, bool) {
	a, me, can, sa, ok := h.sharedFor(w, r, need)
	if !ok {
		return a, me, can, sqlc.SharedApp{}, false
	}
	if sa == nil || sa.UnsharedAt.Valid {
		sharedError(w, deploy.ErrNotShared)
		return a, me, can, sqlc.SharedApp{}, false
	}
	return a, me, can, *sa, true
}

func numericFloat(n pgtype.Numeric) float64 {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

func manifestOf(a sqlc.App) manifest.Manifest {
	var m manifest.Manifest
	_ = json.Unmarshal(a.Manifest, &m)
	return m
}

func (h *Handler) getShared(w http.ResponseWriter, r *http.Request) {
	a, _, can, sa, ok := h.sharedFor(w, r, canView)
	if !ok {
		return
	}
	out := map[string]any{"shared": nil}
	if sa != nil && !sa.UnsharedAt.Valid {
		st := sharedState{Status: sa.Status, StatusMessage: sa.StatusMessage, AlwaysOn: sa.AlwaysOn, IdleMinutes: sa.IdleMinutes,
			MemoryMB: sa.MemoryMb, CPUs: numericFloat(sa.Cpus), WantsDB: sa.WantsDb, URL: h.shared.Svc.AppURL(sa.Slug),
			Acks: sa.Acks, HasPrevious: sa.PreviousDeploymentID.Valid}
		if st.Acks == nil {
			st.Acks = []string{}
		}
		if sa.StoppedBy.Valid {
			if p, err := h.q.GetPerson(r.Context(), sa.StoppedBy.Int64); err == nil {
				st.StoppedBy = p.Name
			}
		}
		out["shared"] = st
		if sa.CurrentDeploymentID.Valid {
			if d, err := h.q.GetDeployment(r.Context(), sa.CurrentDeploymentID.Int64); err == nil && d.VersionID.Valid {
				if v, err := h.q.GetAppVersionByID(r.Context(), d.VersionID.Int64); err == nil {
					out["live_version"] = v.Number
				}
			}
		}
		if v, err := h.q.LatestAppVersion(r.Context(), a.ID); err == nil {
			out["latest_version"] = v.Number
		}
		if can.Restore {
			out["test_url"] = h.shared.Svc.TestURL(sa.Slug)
			_, out["test_open"] = h.shared.Svc.Target(a.ID, true)
			deps, err := h.q.ListDeployments(r.Context(), sqlc.ListDeploymentsParams{AppID: a.ID, Limit: 10})
			if err != nil {
				internalError(w, "list deployments", err)
				return
			}
			list := make([]deploymentOut, 0, len(deps))
			for _, d := range deps {
				o := toDeploymentOut(sqlc.SharedDeployment{ID: d.ID, Kind: d.Kind, Status: d.Status, ErrorMessage: d.ErrorMessage,
					CreatedAt: d.CreatedAt, FinishedAt: d.FinishedAt, ExpiresAt: d.ExpiresAt, BuildLog: d.BuildLog, Port: d.Port}, can.Tech)
				if d.VersionNumber.Valid {
					n := d.VersionNumber.Int32
					o.VersionNumber = &n
				}
				o.CreatedByName = d.CreatedByName.String
				list = append(list, o)
			}
			out["deployments"] = list
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) checkShared(w http.ResponseWriter, r *http.Request) {
	a, _, _, sa, ok := h.sharedFor(w, r, canRestore)
	if !ok {
		return
	}
	rep, err := h.readiness(a, sa)
	if err != nil {
		slog.Warn("shared check", "app", a.ID, "err", err)
		writeError(w, http.StatusBadRequest, folderUnreadable)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

// readiness reads the app folder and runs the checklist.
func (h *Handler) readiness(a sqlc.App, sa *sqlc.SharedApp) (readiness.Report, error) {
	diag, err := scanner.Scan(a.FolderPath)
	if err != nil {
		return readiness.Report{}, err
	}
	var acks []string
	if sa != nil {
		acks = sa.Acks
	}
	return readiness.Check(versions.LocalFolder{Root: a.FolderPath}, manifestOf(a), diag.Secrets, acks)
}

// startShared is "Testar" (test) or "Publicar".
func (h *Handler) startShared(w http.ResponseWriter, r *http.Request, test bool) {
	a, me, can, sa, ok := h.sharedFor(w, r, canRestore)
	if !ok {
		return
	}
	// The engine first, so "Podman desligado" shows as such.
	if err := h.shared.Eng.Available(r.Context()); err != nil {
		sharedError(w, err)
		return
	}
	rep, err := h.readiness(a, sa)
	if err != nil {
		slog.Warn("shared check", "app", a.ID, "err", err)
		writeError(w, http.StatusBadRequest, folderUnreadable)
		return
	}
	if !rep.Ready {
		writeError(w, http.StatusBadRequest, "A checagem ainda tem pendências. Resolva ou marque \"estou ciente\" antes de continuar.")
		return
	}
	m := withDatabase(manifestOf(a), rep)
	var d sqlc.SharedDeployment
	if test {
		d, err = h.shared.Svc.Test(r.Context(), a, m, me.ID)
	} else {
		d, err = h.shared.Svc.Publish(r.Context(), a, m, me.ID)
	}
	if err != nil {
		sharedError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, h.deploymentOutFull(r, d, can.Tech))
}

// withDatabase turns the app's database on when the check says the code
// reads DATABASE_URL (the checklist promises "O Hub cria um banco só deste app").
func withDatabase(m manifest.Manifest, rep readiness.Report) manifest.Manifest {
	if !rep.WantsDB {
		return m
	}
	sh := manifest.Shared{Enabled: true}
	if m.Shared != nil {
		sh = *m.Shared
	}
	sh.Database = "postgres"
	m.Shared = &sh
	return m
}

func (h *Handler) putShared(w http.ResponseWriter, r *http.Request) {
	a, me, _, sa, ok := h.sharedFor(w, r, canRestore)
	if !ok {
		return
	}
	var body SharedSettings
	if !readJSON(w, r, &body) {
		return
	}
	if !body.UpdatedAt.Equal(a.UpdatedAt.Time) {
		writeError(w, http.StatusConflict, "Alguém mudou esta ficha agora há pouco. Recarregue para ver a versão nova.")
		return
	}
	live := sa != nil && !sa.UnsharedAt.Valid && sa.CurrentDeploymentID.Valid
	if !body.Enabled && live {
		writeError(w, http.StatusConflict, "Para deixar de compartilhar, use o botão Deixar de compartilhar.")
		return
	}
	m := manifestOf(a)
	if body.Enabled {
		sh := manifest.Shared{Enabled: true, AlwaysOn: body.AlwaysOn, IdleMin: body.IdleMin, MemoryMB: body.MemoryMB,
			CPUs: body.CPUs, Port: body.Port, HealthPath: strings.TrimSpace(body.HealthPath), Database: body.Database,
			Command: strings.TrimSpace(body.Command), Env: body.Env}
		if m.Shared != nil { // fields this screen does not edit
			sh.Python, sh.Node = m.Shared.Python, m.Shared.Node
			if sh.Database == "" { // a save never turns the database off
				sh.Database = m.Shared.Database
			}
		}
		if sh.Database == "" {
			if rep, err := h.readiness(a, sa); err == nil && rep.WantsDB {
				sh.Database = "postgres"
			}
		}
		m.Shared = &sh
	} else {
		m.Shared = nil
	}
	if info, err := os.Stat(a.FolderPath); err != nil || !info.IsDir() {
		writeError(w, http.StatusConflict, "Não consegui ler a pasta do app agora. Nada foi salvo. Confira se o OneDrive está sincronizado e tente de novo.")
		return
	}
	var backup *int64
	if a.BackupID.Valid {
		backup = &a.BackupID.Int64
	}
	saved, ok := h.writeApp(w, r, me, a.FolderPath, appBody{FolderPath: a.FolderPath, OwnerID: a.OwnerID, BackupID: backup, Manifest: m}, &a)
	if !ok {
		return
	}
	if body.Enabled {
		if _, err := h.shared.Svc.Configure(r.Context(), saved, manifestOf(saved)); err != nil {
			sharedError(w, err)
			return
		}
		acks := []string{}
		for _, k := range body.Acks {
			if slices.Contains(knownAcks, k) && !slices.Contains(acks, k) {
				acks = append(acks, k)
			}
		}
		if err := h.q.SetSharedAcks(r.Context(), sqlc.SetSharedAcksParams{AppID: a.ID, Acks: acks}); err != nil {
			internalError(w, "set acks", err)
			return
		}
	}
	h.writeDetail(w, r, me, saved, http.StatusOK)
}

func (h *Handler) getSharedDeployment(w http.ResponseWriter, r *http.Request) {
	a, _, can, _, ok := h.sharedFor(w, r, canRestore)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("dep"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "Publicação não encontrada.")
		return
	}
	d, err := h.q.GetDeployment(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && d.AppID != a.ID {
		writeError(w, http.StatusNotFound, "Publicação não encontrada.")
		return
	}
	if err != nil {
		internalError(w, "get deployment", err)
		return
	}
	writeJSON(w, http.StatusOK, h.deploymentOutFull(r, d, can.Tech))
}

func (h *Handler) rollbackShared(w http.ResponseWriter, r *http.Request) {
	a, me, _, _, ok := h.sharedRow(w, r, canRestore)
	if !ok {
		return
	}
	var body struct {
		WithData bool `json:"with_data"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if err := h.shared.Svc.Rollback(r.Context(), a.ID, body.WithData, me.ID); err != nil {
		sharedError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

func (h *Handler) stopShared(w http.ResponseWriter, r *http.Request) {
	a, me, _, _, ok := h.sharedRow(w, r, canRestore)
	if !ok {
		return
	}
	if err := h.shared.Svc.Stop(r.Context(), a.ID, me.ID); err != nil {
		sharedError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

// startSharedApp is "Ligar"/"Religar".
func (h *Handler) startSharedApp(w http.ResponseWriter, r *http.Request) {
	a, _, _, sa, ok := h.sharedRow(w, r, canRestore)
	if !ok {
		return
	}
	if !sa.CurrentDeploymentID.Valid {
		writeError(w, http.StatusBadRequest, "Este app ainda não tem versão publicada. Publique antes de ligar.")
		return
	}
	if err := h.shared.Eng.Available(r.Context()); err != nil {
		sharedError(w, err)
		return
	}
	if err := h.shared.Svc.Start(r.Context(), a.ID); err != nil {
		sharedError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

func (h *Handler) unshareShared(w http.ResponseWriter, r *http.Request) {
	a, me, _, _, ok := h.sharedRow(w, r, canRestore)
	if !ok {
		return
	}
	if err := h.shared.Svc.Unshare(r.Context(), a.ID, me.ID); err != nil {
		sharedError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

// sharedLogs shows the live container's last 200 lines, passwords masked.
func (h *Handler) sharedLogs(w http.ResponseWriter, r *http.Request) {
	a, _, _, sa, ok := h.sharedFor(w, r, canTech)
	if !ok {
		return
	}
	text := ""
	if sa != nil && !sa.UnsharedAt.Valid && sa.CurrentDeploymentID.Valid {
		d, err := h.q.GetDeployment(r.Context(), sa.CurrentDeploymentID.Int64)
		if err != nil {
			internalError(w, "get deployment", err)
			return
		}
		if d.ContainerID != "" {
			logs, err := h.shared.Eng.Logs(r.Context(), d.ContainerID, 200)
			if err != nil {
				if errors.Is(err, engine.ErrUnavailable) {
					sharedError(w, err)
					return
				}
				slog.Warn("shared logs", "app", a.ID, "err", err)
			}
			text = scanner.MaskSecrets(logs)
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"text": text})
}

type usagePoint struct {
	At    time.Time `json:"at"`
	MemMB float64   `json:"mem_mb"`
	CPU   float32   `json:"cpu"`
}

// sharedUsage gives the last 24 hours of memory and processor use.
func (h *Handler) sharedUsage(w http.ResponseWriter, r *http.Request) {
	a, _, _, _, ok := h.sharedFor(w, r, canTech)
	if !ok {
		return
	}
	rows, err := h.q.UsageSince(r.Context(), sqlc.UsageSinceParams{AppID: a.ID,
		At: pgtype.Timestamptz{Time: time.Now().Add(-24 * time.Hour), Valid: true}})
	if err != nil {
		internalError(w, "usage", err)
		return
	}
	out := make([]usagePoint, 0, len(rows))
	for _, u := range rows {
		out = append(out, usagePoint{At: u.At.Time, MemMB: float64(u.MemBytes) / (1 << 20), CPU: u.CpuPct})
	}
	writeJSON(w, http.StatusOK, out)
}

// sharedBriefs maps app id → brief for every shared app (nil without the shared parts).
func (h *Handler) sharedBriefs(r *http.Request) (map[int64]*sharedBrief, error) {
	if h.shared.Svc == nil {
		return nil, nil
	}
	rows, err := h.q.ListSharedApps(r.Context())
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*sharedBrief, len(rows))
	for _, s := range rows {
		out[s.AppID] = &sharedBrief{Status: s.Status, URL: h.shared.Svc.AppURL(s.Slug), Slug: s.Slug}
	}
	return out, nil
}

// sharedBriefOf is the brief of one app, nil when it is not shared.
func (h *Handler) sharedBriefOf(r *http.Request, appID int64) (*sharedBrief, error) {
	if h.shared.Svc == nil {
		return nil, nil
	}
	sa, err := h.q.GetSharedApp(r.Context(), appID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if sa.UnsharedAt.Valid {
		return nil, nil
	}
	return &sharedBrief{Status: sa.Status, URL: h.shared.Svc.AppURL(sa.Slug), Slug: sa.Slug}, nil
}
