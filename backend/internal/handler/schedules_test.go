package handler

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

// The schedules table exists, and runs accept the "schedule" origin and the
// "missed" status.
func TestSchedulesTableAndMissedRun(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	pairComputer(t, c, "PC")
	ctx := context.Background()
	var sid int64
	if err := testPool.QueryRow(ctx, `INSERT INTO schedules (app_id, person_id, computer_id, frequency, output_folder, next_run_at)
		VALUES ($1, 1, 1, '{"kind":"daily","at":"07:00"}', 'C:\out', now()) RETURNING id`, app).Scan(&sid); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool.Exec(ctx, `INSERT INTO runs (app_id, person_id, computer_id, output_folder, status, origin, schedule_id)
		VALUES ($1, 1, 1, 'C:\out', 'missed', 'schedule', $2)`, app, sid); err != nil {
		t.Fatal(err)
	}
}

func scriptManifest() manifest.Manifest {
	return manifest.Manifest{ManifestVersion: 1, ID: "relatorio", Name: "Relatório", Kind: manifest.KindScript, Language: "python", Entrypoint: "main.py",
		Inputs: []manifest.Input{
			{ID: "mes", Label: "Mês", Type: "text", Source: "argument", Required: true},
			{ID: "base", Label: "Base", Type: "file", Source: "fixed_file", Path: "base.csv"},
		}}
}

// computerOf stores a connected, up-to-date computer of personID.
func computerOf(t *testing.T, personID int64, name string) int64 {
	t.Helper()
	var id int64
	if err := testPool.QueryRow(context.Background(),
		`INSERT INTO computers (person_id, name, token_hash, agent_version, last_seen_at) VALUES ($1, $2, md5(random()::text), $3, now()) RETURNING id`,
		personID, name, agentproto.Version).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func scheduleForm(computer int64) map[string]any {
	return map[string]any{
		"name":          "Fechamento",
		"frequency":     map[string]any{"kind": "daily", "at": "07:00"},
		"computer_id":   computer,
		"output_folder": `C:\saida`,
		"inputs":        map[string]any{"mes": "outubro"},
		"files":         map[string]any{"base": map[string]string{"name": "base.csv", "content_base64": "YSxi"}},
	}
}

func TestCreateScheduleScriptOnly(t *testing.T) {
	admin, _, _, desktop := team(t)
	comp := computerOf(t, 1, "PC")
	body := scheduleForm(comp)
	body["inputs"], body["files"] = map[string]any{}, map[string]any{}
	var e map[string]string
	if code := admin.do("POST", appPath(desktop, "/schedules"), body, &e); code != 400 || e["error"] != "Só scripts podem ser agendados por enquanto." {
		t.Fatalf("%d %v", code, e)
	}
}

func TestCreateScheduleRejectsWindowInput(t *testing.T) {
	admin, _, _, _ := team(t)
	m := scriptManifest()
	m.Inputs = append(m.Inputs, manifest.Input{ID: "senha", Label: "Senha", Type: "text", Source: "window"})
	app := insertApp(t, m)
	var e map[string]string
	if code := admin.do("POST", appPath(app, "/schedules"), scheduleForm(computerOf(t, 1, "PC")), &e); code != 400 || e["error"] != "Este app pergunta coisas depois de abrir, então não dá para agendar." {
		t.Fatalf("%d %v", code, e)
	}
}

func TestCreateScheduleRejectsFiveMinuteFloor(t *testing.T) {
	admin, _, _, _ := team(t)
	app := insertApp(t, scriptManifest())
	body := scheduleForm(computerOf(t, 1, "PC"))
	body["frequency"] = map[string]any{"kind": "minutes", "every": 4}
	var e map[string]string
	if code := admin.do("POST", appPath(app, "/schedules"), body, &e); code != 400 || e["error"] != "No mínimo a cada 5 minutos." {
		t.Fatalf("%d %v", code, e)
	}
}

func TestCreateScheduleOtherPersonsComputer(t *testing.T) {
	_, _, bia, _ := team(t)
	app := insertApp(t, scriptManifest())
	var e map[string]string
	if code := bia.do("POST", appPath(app, "/schedules"), scheduleForm(computerOf(t, 1, "PC do Henry")), &e); code != 400 || e["error"] != "Escolha um computador seu." {
		t.Fatalf("%d %v", code, e)
	}
}

func TestCreateScheduleAndListOwn(t *testing.T) {
	_, _, bia, _ := team(t)
	app := insertApp(t, scriptManifest())
	var s scheduleJSON
	if code := bia.do("POST", appPath(app, "/schedules"), scheduleForm(computerOf(t, 3, "PC da Bia")), &s); code != 201 {
		t.Fatalf("%d", code)
	}
	if s.Describe != "Todo dia às 7h" || !s.Mine || len(s.Upcoming) != 3 || s.NextRunAt == nil || !s.Active || s.ComputerName != "PC da Bia" {
		t.Fatalf("%+v", s)
	}
	if s.Files["base"].Name != "base.csv" || s.Files["base"].ContentBase64 != "" {
		t.Fatalf("file content must not come back: %+v", s.Files)
	}
	var list []scheduleJSON
	bia.do("GET", appPath(app, "/schedules"), nil, &list)
	if len(list) != 1 || list[0].ID != s.ID {
		t.Fatalf("%+v", list)
	}
}

func TestListSchedulesHidesOthersValues(t *testing.T) {
	admin, ana, bia, _ := team(t)
	app := insertApp(t, scriptManifest()) // owned by Henry (admin)
	var s scheduleJSON
	if code := bia.do("POST", appPath(app, "/schedules"), scheduleForm(computerOf(t, 3, "PC da Bia")), &s); code != 201 {
		t.Fatalf("%d", code)
	}
	var list []scheduleJSON
	admin.do("GET", appPath(app, "/schedules"), nil, &list)
	if len(list) != 1 || list[0].Mine || list[0].OutputFolder != "" || list[0].Inputs != nil || list[0].Files != nil || list[0].PersonName != "Bia" {
		t.Fatalf("carer view: %+v", list)
	}
	var raw []map[string]any
	admin.do("GET", appPath(app, "/schedules"), nil, &raw)
	for _, k := range []string{"inputs", "files", "output_folder"} {
		if _, ok := raw[0][k]; ok {
			t.Fatalf("%s leaked: %v", k, raw[0])
		}
	}
	list = nil
	ana.do("GET", appPath(app, "/schedules"), nil, &list)
	if len(list) != 0 {
		t.Fatalf("ana sees others: %+v", list)
	}
	if code := ana.do("GET", "/api/schedules/"+strconv.FormatInt(s.ID, 10), nil, nil); code != 404 {
		t.Fatalf("ana get %d", code)
	}
	if code := admin.do("GET", "/api/schedules/"+strconv.FormatInt(s.ID, 10), nil, nil); code != 404 {
		t.Fatalf("carer must not open values %d", code)
	}
}

func TestPauseByAppCarer(t *testing.T) {
	admin, ana, bia, _ := team(t)
	app := insertApp(t, scriptManifest())
	var s scheduleJSON
	bia.do("POST", appPath(app, "/schedules"), scheduleForm(computerOf(t, 3, "PC da Bia")), &s)
	path := "/api/schedules/" + strconv.FormatInt(s.ID, 10)
	if code := ana.do("POST", path+"/pause", nil, nil); code != 404 {
		t.Fatalf("ana pause %d", code)
	}
	var paused scheduleJSON
	if code := admin.do("POST", path+"/pause", nil, &paused); code != 200 || paused.Active {
		t.Fatalf("carer pause %d %+v", code, paused)
	}
	if code := admin.do("POST", path+"/resume", nil, nil); code != 404 {
		t.Fatalf("only the owner resumes, got %d", code)
	}
	if code := admin.do("DELETE", path, nil, nil); code != 204 {
		t.Fatalf("carer delete %d", code)
	}
}

func TestResumeResetsFailures(t *testing.T) {
	_, _, bia, _ := team(t)
	app := insertApp(t, scriptManifest())
	var s scheduleJSON
	bia.do("POST", appPath(app, "/schedules"), scheduleForm(computerOf(t, 3, "PC da Bia")), &s)
	exec(t, `UPDATE schedules SET active=false, failures=5, paused_reason='Falhou 5 vezes seguidas', situation='x', next_run_at=now() - interval '3 days' WHERE id=$1`, s.ID)
	var r scheduleJSON
	if code := bia.do("POST", "/api/schedules/"+strconv.FormatInt(s.ID, 10)+"/resume", nil, &r); code != 200 {
		t.Fatalf("%d", code)
	}
	var failures int
	_ = testPool.QueryRow(context.Background(), `SELECT failures FROM schedules WHERE id=$1`, s.ID).Scan(&failures)
	if !r.Active || r.PausedReason != "" || r.Situation != "" || failures != 0 || !r.NextRunAt.After(time.Now()) {
		t.Fatalf("%+v failures=%d", r, failures)
	}
}

func TestEditKeepsStoredFileWhenOnlyNameSent(t *testing.T) {
	_, _, bia, _ := team(t)
	app := insertApp(t, scriptManifest())
	comp := computerOf(t, 3, "PC da Bia")
	var s scheduleJSON
	bia.do("POST", appPath(app, "/schedules"), scheduleForm(comp), &s)
	body := scheduleForm(comp)
	body["files"] = map[string]any{"base": map[string]string{"name": "base.csv"}}
	body["frequency"] = map[string]any{"kind": "minutes", "every": 15}
	var e scheduleJSON
	if code := bia.do("PUT", "/api/schedules/"+strconv.FormatInt(s.ID, 10), body, &e); code != 200 || e.Describe != "A cada 15 minutos" {
		t.Fatalf("%d %+v", code, e)
	}
	var raw string
	_ = testPool.QueryRow(context.Background(), `SELECT files->'base'->>'content_base64' FROM schedules WHERE id=$1`, s.ID).Scan(&raw)
	if raw != "YSxi" {
		t.Fatalf("stored file lost: %q", raw)
	}
}
