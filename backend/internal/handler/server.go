package handler

import (
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/shared/costs"
	"github.com/HenryKropp/app-hub/backend/internal/shared/hostinfo"
)

const maxMonthlyCostCents = 100_000_000

// Whoever has manage_server acts on any shared app, even one the catalog does
// not show them: caring for the server is the role's job (see ADR 010).

func (h *Handler) getServer(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageServer); !ok {
		return
	}
	if !h.sharedReady(w, r) {
		return
	}
	ctx := r.Context()
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthLen := start.AddDate(0, 1, 0).Sub(start)
	info, err := hostinfo.Read(h.shared.DataDir)
	if err != nil {
		slog.Warn("hostinfo", "err", err)
	}
	settings, err := h.q.GetServerSettings(ctx)
	if err != nil {
		internalError(w, "server settings", err)
		return
	}
	apps, err := h.q.ListSharedApps(ctx)
	if err != nil {
		internalError(w, "list shared", err)
		return
	}
	totals, err := h.q.UsageTotalsSince(ctx, pgtype.Timestamptz{Time: start, Valid: true})
	if err != nil {
		slog.Warn("usage totals", "err", err)
	}
	byApp := map[int64]sqlc.UsageTotalsSinceRow{}
	for _, t := range totals {
		byApp[t.AppID] = t
	}
	backupBytes := map[int64]int64{}
	if sums, err := h.q.SumBackupBytes(ctx); err == nil {
		for _, s := range sums {
			backupBytes[s.AppID] = s.Total
		}
	}
	var uses []costs.AppUse
	for _, a := range apps {
		uses = append(uses, costs.AppUse{AppID: a.AppID, MemoryMB: int(a.MemoryMb), Minutes: byApp[a.AppID].Minutes})
	}
	per, hub := costs.Estimate(settings.MonthlyCostCents, int(info.MemTotal>>20), now.Sub(start), monthLen, uses)
	type row struct {
		AppID       int64   `json:"app_id"`
		Name        string  `json:"name"`
		Slug        string  `json:"slug"`
		Status      string  `json:"status"`
		AlwaysOn    bool    `json:"always_on"`
		MemoryMB    int32   `json:"memory_mb"`
		CPUs        float64 `json:"cpus"`
		IdleMinutes int32   `json:"idle_minutes"`
		HoursOn     float64 `json:"hours_on"`
		AvgMemMB    int64   `json:"avg_mem_mb"`
		MaxMemMB    int64   `json:"max_mem_mb"`
		AvgCPU      float32 `json:"avg_cpu"`
		DBBytes     int64   `json:"db_bytes"`
		BackupBytes int64   `json:"backup_bytes"`
		CostCents   int64   `json:"cost_cents"`
	}
	out := []row{}
	for _, a := range apps {
		u := byApp[a.AppID]
		var dbBytes int64
		if a.DbName != "" && h.shared.DBs != nil {
			dbBytes, _ = h.shared.DBs.Size(ctx, a.DbName)
		}
		out = append(out, row{AppID: a.AppID, Name: a.AppName, Slug: a.Slug, Status: a.Status, AlwaysOn: a.AlwaysOn,
			MemoryMB: a.MemoryMb, CPUs: numericFloat(a.Cpus), IdleMinutes: a.IdleMinutes,
			HoursOn: math.Round(float64(u.Minutes)/60*10) / 10, AvgMemMB: u.AvgMem >> 20, MaxMemMB: u.MaxMem >> 20, AvgCPU: u.AvgCpu,
			DBBytes: dbBytes, BackupBytes: backupBytes[a.AppID], CostCents: per[a.AppID]})
	}
	engineOK := h.shared.Eng != nil && h.shared.Eng.Available(ctx) == nil
	writeJSON(w, http.StatusOK, map[string]any{
		"memory_total": info.MemTotal, "memory_used": info.MemUsed, "disk_total": info.DiskTotal, "disk_used": info.DiskUsed,
		"disk_warning":       info.DiskTotal > 0 && float64(info.DiskUsed)/float64(info.DiskTotal) > 0.9,
		"engine_ok":          engineOK,
		"monthly_cost_cents": settings.MonthlyCostCents, "hub_cents": hub, "period_start": start, "apps": out,
	})
}

func (h *Handler) putServerCost(w http.ResponseWriter, r *http.Request) {
	me, _, ok := h.requirePerm(w, r, access.ManageServer)
	if !ok {
		return
	}
	var body struct {
		MonthlyCostCents int64 `json:"monthly_cost_cents"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.MonthlyCostCents < 0 || body.MonthlyCostCents > maxMonthlyCostCents {
		writeError(w, http.StatusBadRequest, "O custo do servidor precisa estar entre R$ 0 e R$ 1.000.000 por mês. Confira o valor e tente de novo.")
		return
	}
	if err := h.q.SetServerCost(r.Context(), sqlc.SetServerCostParams{MonthlyCostCents: body.MonthlyCostCents, UpdatedBy: pgtype.Int8{Int64: me.ID, Valid: true}}); err != nil {
		internalError(w, "set server cost", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"monthly_cost_cents": body.MonthlyCostCents})
}

// serverApp loads the shared app in the URL for someone with manage_server.
func (h *Handler) serverApp(w http.ResponseWriter, r *http.Request) (sqlc.Person, sqlc.App, sqlc.SharedApp, bool) {
	me, _, ok := h.requirePerm(w, r, access.ManageServer)
	if !ok || !h.sharedReady(w, r) {
		return me, sqlc.App{}, sqlc.SharedApp{}, false
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "App não encontrado. Atualize a página e tente de novo.")
		return me, sqlc.App{}, sqlc.SharedApp{}, false
	}
	sa, err := h.q.GetSharedApp(r.Context(), id)
	if err != nil || sa.UnsharedAt.Valid {
		writeError(w, http.StatusNotFound, "Esse app não está compartilhado. Atualize a página e tente de novo.")
		return me, sqlc.App{}, sqlc.SharedApp{}, false
	}
	a, err := h.q.GetApp(r.Context(), id)
	if err != nil {
		internalError(w, "get app", err)
		return me, sqlc.App{}, sqlc.SharedApp{}, false
	}
	return me, a, sa, true
}

func (h *Handler) putServerApp(w http.ResponseWriter, r *http.Request) {
	me, a, sa, ok := h.serverApp(w, r)
	if !ok {
		return
	}
	var body struct {
		MemoryMB    int     `json:"memory_mb"`
		CPUs        float64 `json:"cpus"`
		AlwaysOn    bool    `json:"always_on"`
		IdleMinutes int     `json:"idle_minutes"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	switch {
	case body.MemoryMB < 128 || body.MemoryMB > 16384:
		writeError(w, http.StatusBadRequest, "A memória do app precisa estar entre 128 e 16384 MB. Ajuste o valor e tente de novo.")
		return
	case body.CPUs < 0.1 || body.CPUs > 16:
		writeError(w, http.StatusBadRequest, "O processamento do app precisa estar entre 0,1 e 16. Ajuste o valor e tente de novo.")
		return
	case body.IdleMinutes < 5 || body.IdleMinutes > 1440:
		writeError(w, http.StatusBadRequest, "O tempo para dormir precisa estar entre 5 e 1440 minutos. Ajuste o valor e tente de novo.")
		return
	}
	m := manifestOf(a)
	if m.Shared == nil {
		writeError(w, http.StatusConflict, "O app perdeu a configuração de compartilhamento. Abra a ficha do app e compartilhe de novo.")
		return
	}
	sh := *m.Shared
	sh.MemoryMB, sh.CPUs, sh.AlwaysOn, sh.IdleMin = body.MemoryMB, body.CPUs, body.AlwaysOn, body.IdleMinutes
	m.Shared = &sh
	var backup *int64
	if a.BackupID.Valid {
		backup = &a.BackupID.Int64
	}
	saved, ok := h.writeApp(w, r, me, a.FolderPath, appBody{FolderPath: a.FolderPath, OwnerID: a.OwnerID, BackupID: backup, Manifest: m}, &a)
	if !ok {
		return
	}
	if _, err := h.shared.Svc.Configure(r.Context(), saved, manifestOf(saved)); err != nil {
		sharedError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"needs_restart": sa.Status == "running"})
}

func (h *Handler) stopServerApp(w http.ResponseWriter, r *http.Request) {
	me, a, _, ok := h.serverApp(w, r)
	if !ok {
		return
	}
	if err := h.shared.Svc.Stop(r.Context(), a.ID, me.ID); err != nil {
		sharedError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

func (h *Handler) startServerApp(w http.ResponseWriter, r *http.Request) {
	_, a, sa, ok := h.serverApp(w, r)
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
