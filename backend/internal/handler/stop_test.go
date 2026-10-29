package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

// openRun starts a run and has the agent open it; it returns the run, the
// run job and the agent token.
func openRun(t *testing.T, c *client, canStop bool) (runJSON, agentproto.Job, string) {
	t.Helper()
	app := insertApp(t, filesPrepManifest())
	token := pairComputer(t, c, "PC")
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version, CanUpdate: true, CanStop: canStop}, nil)
	var run runJSON
	c.do("POST", appPath(app, "/runs"), map[string]string{}, &run)
	var job agentproto.Job
	c.agent("GET", "/api/agent/next", token, nil, &job)
	c.agent("POST", "/api/agent/jobs/"+strconv.FormatInt(job.ID, 10)+"/result", token, agentproto.Result{Event: agentproto.EventStarted}, nil)
	c.do("GET", "/api/runs/"+strconv.FormatInt(run.ID, 10), nil, &run)
	return run, job, token
}

func TestStopOpenRun(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	run, runJob, token := openRun(t, c, true)
	if run.Status != "running" || !run.CanStop {
		t.Fatalf("open run %+v", run)
	}
	runPath := "/api/runs/" + strconv.FormatInt(run.ID, 10)

	code := make(chan int, 1)
	go func() { code <- c.do("POST", runPath+"/stop", nil, nil) }()

	// The agent gets the stop, closes the app and reports both jobs.
	var job agentproto.Job
	for i := 0; i < 50 && job.ID == 0; i++ {
		c.agent("GET", "/api/agent/next", token, nil, &job)
	}
	var p agentproto.StopPayload
	if job.Kind != agentproto.KindStop || json.Unmarshal(job.Payload, &p) != nil || p.RunID != run.ID {
		t.Fatalf("stop job %+v", job)
	}
	files := []agentproto.FileEntry{{Path: "parcial.pdf", Size: 3, Modified: time.Now(), Change: "new"}}
	c.agent("POST", "/api/agent/jobs/"+strconv.FormatInt(runJob.ID, 10)+"/result", token, agentproto.Result{Event: agentproto.EventFinished, ExitCode: 1, Files: files, Stopped: true}, nil)
	c.agent("POST", "/api/agent/jobs/"+strconv.FormatInt(job.ID, 10)+"/result", token, agentproto.Result{Event: agentproto.EventDone}, nil)
	if got := <-code; got != http.StatusNoContent {
		t.Fatalf("stop %d", got)
	}
	c.do("GET", runPath, nil, &run)
	if run.Status != "stopped" || run.ErrorMessage != stoppedMessage || run.CanStop {
		t.Fatalf("stopped run %+v", run)
	}
	n := 0
	for _, g := range run.Groups {
		n += len(g.Files)
	}
	if n != 1 {
		t.Fatalf("stopped run keeps its files: %d", n)
	}
	if got := c.do("POST", runPath+"/stop", nil, nil); got != http.StatusConflict {
		t.Fatalf("stop again %d", got)
	}
}

func TestStopAppAlreadyClosedOnComputer(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	run, _, token := openRun(t, c, true)
	code := make(chan int, 1)
	go func() { code <- c.do("POST", "/api/runs/"+strconv.FormatInt(run.ID, 10)+"/stop", nil, nil) }()
	var job agentproto.Job
	for i := 0; i < 50 && job.ID == 0; i++ {
		c.agent("GET", "/api/agent/next", token, nil, &job)
	}
	c.agent("POST", "/api/agent/jobs/"+strconv.FormatInt(job.ID, 10)+"/result", token, agentproto.Result{Event: agentproto.EventFailed, Error: "Este app já tinha fechado."}, nil)
	if got := <-code; got != http.StatusConflict {
		t.Fatalf("stop closed app %d", got)
	}
}

func TestStopWaitingRunInHubAlone(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	token := pairComputer(t, c, "PC")
	var run runJSON
	c.do("POST", appPath(app, "/runs"), map[string]string{}, &run)
	if run.Status != "waiting" {
		t.Fatalf("waiting run %+v", run)
	}
	runPath := "/api/runs/" + strconv.FormatInt(run.ID, 10)
	if got := c.do("POST", runPath+"/stop", nil, nil); got != http.StatusNoContent {
		t.Fatalf("stop waiting %d", got)
	}
	c.do("GET", runPath, nil, &run)
	if run.Status != "stopped" {
		t.Fatalf("after stop %+v", run)
	}
	// The agent no longer receives the canceled run.
	if got := c.agent("GET", "/api/agent/next", token, nil, nil); got != http.StatusNoContent {
		t.Fatalf("agent still got a job: %d", got)
	}
}

func TestStopRefusedForOldAgentAndOtherPerson(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	run, _, _ := openRun(t, c, false)
	if run.CanStop {
		t.Fatalf("old agent offers stop: %+v", run)
	}
	runPath := "/api/runs/" + strconv.FormatInt(run.ID, 10)
	if got := c.do("POST", runPath+"/stop", nil, nil); got != http.StatusConflict {
		t.Fatalf("old agent stop %d", got)
	}

	createPersonAs(t, c, "Ana", "ana@alis.com", "usuario")
	ana := c.fresh()
	loginAs(t, ana, "ana@alis.com")
	if got := ana.do("POST", runPath+"/stop", nil, nil); got != http.StatusNotFound {
		t.Fatalf("other person stop %d", got)
	}
	var jobs int
	if err := testPool.QueryRow(context.Background(), "SELECT count(*) FROM agent_jobs WHERE kind = 'stop'").Scan(&jobs); err != nil || jobs != 0 {
		t.Fatalf("stop jobs queued: %d %v", jobs, err)
	}
}

func TestActiveRunsOfferStop(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	run, _, _ := openRun(t, c, true)
	var active []activeRunJSON
	c.do("GET", "/api/runs/active", nil, &active)
	if len(active) != 1 || active[0].ID != run.ID || !active[0].CanStop {
		t.Fatalf("active %+v", active)
	}
}
