package handler

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// dueSchedule stores a daily schedule of Bia (person 3) on her computer,
// due one minute before now.
func dueSchedule(t *testing.T, h *Handler, now time.Time, connected bool) (scheduleID, appID, computerID int64) {
	t.Helper()
	appID = insertApp(t, scriptManifest())
	computerID = computerOf(t, 3, "PC da Bia")
	if !connected {
		exec(t, `UPDATE computers SET last_seen_at = now() - interval '1 hour' WHERE id = $1`, computerID)
	}
	if err := testPool.QueryRow(context.Background(), `INSERT INTO schedules (app_id, person_id, computer_id, frequency, inputs, files, output_folder, next_run_at)
		VALUES ($1, 3, $2, '{"kind":"minutes","every":5}', '{"mes":["outubro"]}', '{}', 'C:\saida', $3) RETURNING id`,
		appID, computerID, now.Add(-time.Minute)).Scan(&scheduleID); err != nil {
		t.Fatal(err)
	}
	return
}

type schedState struct {
	Active               bool
	PausedReason, Situation string
	NextRunAt            time.Time
	Runs, Missed, Jobs   int
}

func readSchedule(t *testing.T, id int64) schedState {
	t.Helper()
	var s schedState
	ctx := context.Background()
	if err := testPool.QueryRow(ctx, `SELECT active, paused_reason, situation, next_run_at,
		(SELECT count(*) FROM runs WHERE schedule_id = s.id),
		(SELECT count(*) FROM runs WHERE schedule_id = s.id AND status = 'missed'),
		(SELECT count(*) FROM agent_jobs j JOIN runs r ON r.id = j.run_id WHERE r.schedule_id = s.id)
		FROM schedules s WHERE id = $1`, id).Scan(&s.Active, &s.PausedReason, &s.Situation, &s.NextRunAt, &s.Runs, &s.Missed, &s.Jobs); err != nil {
		t.Fatal(err)
	}
	return s
}

func schedulerTeam(t *testing.T) *Handler {
	t.Helper()
	admin, h := newPoolClient(t)
	setupAdmin(t, admin)
	createPersonAs(t, admin, "Ana", "ana@alis.com", "criador")
	createPersonAs(t, admin, "Bia", "bia@alis.com", "usuario")
	return h
}

func TestSchedulerFiresDueSchedule(t *testing.T) {
	h := schedulerTeam(t)
	now := time.Now()
	sid, _, _ := dueSchedule(t, h, now, true)
	h.schedulerTick(context.Background(), now)
	s := readSchedule(t, sid)
	if s.Runs != 1 || s.Jobs != 1 || !s.Active || !s.NextRunAt.After(now) {
		t.Fatalf("%+v", s)
	}
	var origin string
	_ = testPool.QueryRow(context.Background(), `SELECT origin FROM runs WHERE schedule_id = $1`, sid).Scan(&origin)
	if origin != "schedule" {
		t.Fatalf("origin %q", origin)
	}
}

func TestSchedulerClaimsOnce(t *testing.T) {
	h := schedulerTeam(t)
	now := time.Now()
	sid, _, _ := dueSchedule(t, h, now, true)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); h.schedulerTick(context.Background(), now) }()
	}
	wg.Wait()
	if s := readSchedule(t, sid); s.Runs != 1 {
		t.Fatalf("%+v", s)
	}
}

func TestSchedulerOfflineComputerCreatesMissed(t *testing.T) {
	h := schedulerTeam(t)
	now := time.Now()
	sid, _, _ := dueSchedule(t, h, now, false)
	h.schedulerTick(context.Background(), now)
	s := readSchedule(t, sid)
	if s.Missed != 1 || s.Jobs != 0 || !s.Active {
		t.Fatalf("%+v", s)
	}
	var msg string
	_ = testPool.QueryRow(context.Background(), `SELECT error_message FROM runs WHERE schedule_id = $1`, sid).Scan(&msg)
	if msg != "O computador PC da Bia estava desligado" {
		t.Fatalf("%q", msg)
	}
}

// tickAgain makes the schedule due again and runs the clock.
func tickAgain(t *testing.T, h *Handler, sid int64, now time.Time) {
	t.Helper()
	exec(t, `UPDATE schedules SET next_run_at = $2 WHERE id = $1`, sid, now.Add(-time.Minute))
	h.schedulerTick(context.Background(), now)
}

func TestSchedulerPausesAfterFiveFailures(t *testing.T) {
	h := schedulerTeam(t)
	now := time.Now()
	sid, _, _ := dueSchedule(t, h, now, false)
	var auditFrom int64
	_ = testPool.QueryRow(context.Background(), `SELECT COALESCE(max(id), 0) FROM security_audit`).Scan(&auditFrom)
	for i := 0; i < 4; i++ {
		tickAgain(t, h, sid, now)
	}
	if s := readSchedule(t, sid); !s.Active || s.Missed != 4 {
		t.Fatalf("after 4: %+v", s)
	}
	tickAgain(t, h, sid, now)
	s := readSchedule(t, sid)
	if s.Active || s.Missed != 5 || s.PausedReason != "Falhou 5 vezes seguidas" {
		t.Fatalf("after 5: %+v", s)
	}
	tickAgain(t, h, sid, now)
	if s := readSchedule(t, sid); s.Missed != 5 {
		t.Fatalf("paused schedule fired: %+v", s)
	}
	var audits int
	_ = testPool.QueryRow(context.Background(), `SELECT count(*) FROM security_audit WHERE id > $1 AND action = 'schedule.paused' AND actor_id IS NULL AND route = 'schedule'`, auditFrom).Scan(&audits)
	if audits != 1 {
		t.Fatalf("audits %d", audits)
	}
}

func TestSchedulerSkipsWhilePreviousRunning(t *testing.T) {
	h := schedulerTeam(t)
	now := time.Now()
	sid, _, _ := dueSchedule(t, h, now, true)
	h.schedulerTick(context.Background(), now)
	tickAgain(t, h, sid, now)
	s := readSchedule(t, sid)
	if s.Runs != 1 || s.Situation != "Pulada: a anterior ainda estava rodando" || !s.Active {
		t.Fatalf("%+v", s)
	}
}

func TestSchedulerWaitsOnApproval(t *testing.T) {
	h := schedulerTeam(t)
	h.SetWorkspace(WorkspaceOptions{Slug: "teste", Name: "Teste"})
	now := time.Now()
	sid, app, _ := dueSchedule(t, h, now, true)
	var before bool
	_ = testPool.QueryRow(context.Background(), `SELECT require_app_approval FROM security_settings WHERE id = TRUE`).Scan(&before)
	exec(t, `UPDATE security_settings SET require_app_approval = true WHERE id = TRUE`)
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `UPDATE security_settings SET require_app_approval = $1 WHERE id = TRUE`, before)
	})
	exec(t, `INSERT INTO app_security (app_id, approved) VALUES ($1, false) ON CONFLICT (app_id) DO UPDATE SET approved = false`, app)
	h.schedulerTick(context.Background(), now)
	s := readSchedule(t, sid)
	if s.Runs != 0 || !s.Active || s.Situation != "Esperando aprovação do app" || !s.NextRunAt.After(now) {
		t.Fatalf("%+v", s)
	}
}

func TestSchedulerPausesWhenAccessLost(t *testing.T) {
	h := schedulerTeam(t)
	now := time.Now()
	sid, app, _ := dueSchedule(t, h, now, true)
	restrict(t, app)
	h.schedulerTick(context.Background(), now)
	s := readSchedule(t, sid)
	if s.Runs != 0 || s.Active || s.PausedReason != "Você não tem mais acesso a este app" {
		t.Fatalf("%+v", s)
	}
}

func TestSchedulerPausesWhenInputsInvalid(t *testing.T) {
	h := schedulerTeam(t)
	now := time.Now()
	sid, _, _ := dueSchedule(t, h, now, true)
	exec(t, `UPDATE schedules SET inputs = '{}' WHERE id = $1`, sid) // "mes" is required
	h.schedulerTick(context.Background(), now)
	s := readSchedule(t, sid)
	if s.Runs != 0 || s.Active || s.PausedReason != "O app mudou e os campos guardados não servem mais. Edite o agendamento." {
		t.Fatalf("%+v", s)
	}
}

func TestSchedulerLateBeyondGraceDoesNotFire(t *testing.T) {
	h := schedulerTeam(t)
	now := time.Now()
	sid, _, _ := dueSchedule(t, h, now, true)
	exec(t, `UPDATE schedules SET next_run_at = $2 WHERE id = $1`, sid, now.Add(-2*time.Hour))
	h.schedulerTick(context.Background(), now)
	s := readSchedule(t, sid)
	if s.Runs != 0 || !s.Active || !s.NextRunAt.After(now) || s.Situation == "" {
		t.Fatalf("%+v", s)
	}
}

func TestScheduledRunDoneResetsFailures(t *testing.T) {
	h := schedulerTeam(t)
	now := time.Now()
	sid, _, comp := dueSchedule(t, h, now, false)
	for i := 0; i < 4; i++ {
		tickAgain(t, h, sid, now)
	}
	exec(t, `UPDATE computers SET last_seen_at = now() WHERE id = $1`, comp)
	tickAgain(t, h, sid, now)
	exec(t, `UPDATE runs SET status = 'done', finished_at = now() WHERE schedule_id = $1 AND status = 'waiting'`, sid)
	exec(t, `UPDATE computers SET last_seen_at = now() - interval '1 hour' WHERE id = $1`, comp)
	tickAgain(t, h, sid, now)
	if s := readSchedule(t, sid); !s.Active || s.Missed != 5 {
		t.Fatalf("a success must break the streak: %+v", s)
	}
}

func TestStoppedRunDoesNotCountAsFailure(t *testing.T) {
	h := schedulerTeam(t)
	now := time.Now()
	sid, _, comp := dueSchedule(t, h, now, false)
	for i := 0; i < 4; i++ {
		tickAgain(t, h, sid, now)
	}
	exec(t, `UPDATE computers SET last_seen_at = now() WHERE id = $1`, comp)
	tickAgain(t, h, sid, now)
	exec(t, `UPDATE runs SET status = 'stopped', finished_at = now() WHERE schedule_id = $1 AND status = 'waiting'`, sid)
	exec(t, `UPDATE computers SET last_seen_at = now() - interval '1 hour' WHERE id = $1`, comp)
	tickAgain(t, h, sid, now)
	if s := readSchedule(t, sid); s.Active || s.Missed != 5 {
		t.Fatalf("a stopped run is skipped, so 5 missed around it still pause: %+v", s)
	}
}

func TestHomeSkipsScheduledMissed(t *testing.T) {
	_, _, bia, _ := team(t)
	now := time.Now()
	sid, app, comp := func() (int64, int64, int64) {
		app := insertApp(t, scriptManifest())
		comp := computerOf(t, 3, "PC da Bia")
		var id int64
		if err := testPool.QueryRow(context.Background(), `INSERT INTO schedules (app_id, person_id, computer_id, frequency, output_folder, next_run_at, active, paused_reason)
			VALUES ($1, 3, $2, '{"kind":"daily","at":"07:00"}', 'C:\saida', $3, false, 'Falhou 5 vezes seguidas') RETURNING id`, app, comp, now).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id, app, comp
	}()
	exec(t, `INSERT INTO runs (app_id, person_id, computer_id, output_folder, status, origin, schedule_id, error_message)
		VALUES ($1, 3, $2, 'C:\saida', 'error', 'schedule', $3, 'x'), ($1, 3, $2, 'C:\saida', 'missed', 'schedule', $3, 'y')`, app, comp, sid)
	var out struct {
		Attention []struct {
			Kind    string `json:"kind"`
			AppID   int64  `json:"app_id"`
			Reason  string `json:"reason"`
		} `json:"attention"`
	}
	if code := bia.do("GET", "/api/home", nil, &out); code != 200 {
		t.Fatalf("%d", code)
	}
	paused := 0
	for _, w := range out.Attention {
		if strings.HasPrefix(w.Kind, "run_") {
			t.Fatalf("scheduled runs must not warn one by one: %+v", out.Attention)
		}
		if w.Kind == "schedule_paused" && w.AppID == app && w.Reason == "Falhou 5 vezes seguidas" {
			paused++
		}
	}
	if paused != 1 {
		t.Fatalf("%+v", out.Attention)
	}
}
