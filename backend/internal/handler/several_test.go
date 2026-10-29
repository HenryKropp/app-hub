package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

func TestSeveralAppsAtOnceNeedSeparateFolders(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	cvm := insertApp(t, filesPrepManifest())
	other := insertApp(t, filesPrepManifestWithID("outro-app"))
	token := pairComputer(t, c, "PC")

	start := func(app int64, folder string) (int, string) {
		var body struct {
			Error string `json:"error"`
		}
		code := c.do("POST", appPath(app, "/runs"), map[string]string{"output_folder": folder}, &body)
		return code, body.Error
	}
	if code, msg := start(cvm, `C:\Dados\CVM`); code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("first %d %s", code, msg)
	}
	if code, msg := start(other, `C:\Dados\Outro`); code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("second app in another folder %d %s", code, msg)
	}
	for _, folder := range []string{`c:\dados\cvm\`, `C:\Dados\CVM\Março`, `C:\Dados`} {
		code, msg := start(other, folder)
		if code != http.StatusConflict || msg != "Essa pasta já está sendo usada pelo Eq. Research Files Prep, que está aberto agora. Escolha outra pasta ou espere ele fechar." {
			t.Fatalf("%s: %d %q", folder, code, msg)
		}
	}

	// The agent learns the app's name, to explain its own refusal.
	var job agentproto.Job
	c.agent("GET", "/api/agent/next", token, nil, &job)
	var p agentproto.RunPayload
	if json.Unmarshal(job.Payload, &p) != nil || p.AppName != "Eq. Research Files Prep" {
		t.Fatalf("payload %s", job.Payload)
	}
	// Once the first app closes, its folder is free again.
	c.agent("POST", "/api/agent/jobs/"+strconv.FormatInt(job.ID, 10)+"/result", token, agentproto.Result{Event: agentproto.EventStarted}, nil)
	c.agent("POST", "/api/agent/jobs/"+strconv.FormatInt(job.ID, 10)+"/result", token, agentproto.Result{Event: agentproto.EventFinished}, nil)
	if code, msg := start(other, `C:\Dados\CVM`); code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("folder after the app closed %d %s", code, msg)
	}
}

func TestLiveRunsKnowScripts(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	script := insertApp(t, manifest.Manifest{ID: "script-x", Name: "Script X", Kind: manifest.KindScript, Language: "python", Entrypoint: "main.py"})
	pairComputer(t, c, "PC")
	ctx := context.Background()
	var computer int64
	if err := testPool.QueryRow(ctx, "SELECT id FROM computers ORDER BY id DESC LIMIT 1").Scan(&computer); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool.Exec(ctx, `INSERT INTO runs (app_id, person_id, computer_id, output_folder) VALUES ($1, 1, $2, 'C:\Dados')`, script, computer); err != nil {
		t.Fatal(err)
	}
	live, err := sqlc.New(testPool).ListLiveRunsOnComputer(ctx, computer)
	if err != nil || len(live) != 1 || !live[0].IsScript || live[0].AppName != "Script X" {
		t.Fatalf("%+v %v", live, err)
	}
}
