package deploy_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy/deploytest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
	"github.com/HenryKropp/app-hub/backend/internal/vault"
	"github.com/jackc/pgx/v5/pgtype"
)

var itoa = deploytest.Itoa

func setup(t *testing.T) *deploytest.Env { return deploytest.Setup(t, pool) }

func TestPublishFirstTime(t *testing.T) {
	e := setup(t)
	d := e.Publish(t)
	if d.Status != "live" {
		t.Fatalf("deployment %s: %s\n%s", d.Status, d.ErrorMessage, d.BuildLog)
	}
	sa := e.Shared(t)
	if sa.Status != "running" || !sa.CurrentDeploymentID.Valid || sa.CurrentDeploymentID.Int64 != d.ID || sa.DbName != "app_"+itoa(e.App.ID) {
		t.Fatalf("%+v", sa)
	}
	if got := e.Eng.Running(); len(got) != 1 || got[0] != "ziv-notas-"+itoa(d.ID) {
		t.Fatalf("running %v", got)
	}
	var env map[string]string
	for _, c := range e.Eng.Containers {
		env = c.Spec.Env
	}
	if env["PORT"] != "8501" || env["BASE_URL"] != "http://notas.localhost:8080" || env["APPHUB_DATA_DIR"] != "/data" ||
		!strings.HasPrefix(env["DATABASE_URL"], "postgres://app_") {
		t.Fatalf("env %v", env)
	}
	if _, ok := e.S.Target(e.App.ID, false); !ok {
		t.Fatal("no target")
	}
	if !strings.Contains(e.Eng.Images[d.ImageID].Tag, "localhost/ziv/notas:") {
		t.Fatalf("tag %+v", e.Eng.Images)
	}
}

func TestDeployOneAtATime(t *testing.T) {
	e := setup(t)
	release := make(chan struct{})
	e.S.SetSpawn(func(f func()) { go func() { <-release; f() }() })
	first, err := e.S.Publish(context.Background(), e.App, e.M, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.S.Publish(context.Background(), e.App, e.M, 1); !errors.Is(err, deploy.ErrBusy) {
		t.Fatalf("second publish err %v", err)
	}
	if st := e.Shared(t).Status; st != "starting" {
		t.Fatalf("first publish in progress: status %s", st)
	}
	close(release)
	e.WaitDeployment(t, first.ID)
	third, err := e.S.Publish(context.Background(), e.App, e.M, 1)
	if err != nil {
		t.Fatalf("after finishing, publish again: %v", err)
	}
	// The third publish runs in the background: let it end, then shut the
	// fake containers, before the test's folders and database go away.
	e.WaitDeployment(t, third.ID)
	e.Eng.Close()
}

func TestNewVersionUnhealthyKeepsOld(t *testing.T) {
	e := setup(t)
	old := e.Publish(t)
	oldPort, _ := e.S.Target(e.App.ID, false)
	e.Eng.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) })
	d := e.Publish(t)
	if d.Status != "failed" || !strings.Contains(d.ErrorMessage, "não respondeu na porta 8501") {
		t.Fatalf("%s %q", d.Status, d.ErrorMessage)
	}
	sa := e.Shared(t)
	if sa.CurrentDeploymentID.Int64 != old.ID || sa.Status != "running" {
		t.Fatalf("%+v", sa)
	}
	if port, _ := e.S.Target(e.App.ID, false); port != oldPort {
		t.Fatal("target moved to the broken version")
	}
	if got := e.Eng.Running(); len(got) != 1 || got[0] != "ziv-notas-"+itoa(old.ID) {
		t.Fatalf("running %v", got)
	}
}

func TestNewVersionSwapsAndBacksUp(t *testing.T) {
	e := setup(t)
	old := e.Publish(t)
	d := e.Publish(t)
	if d.Status != "live" {
		t.Fatalf("%s %s", d.Status, d.ErrorMessage)
	}
	sa := e.Shared(t)
	if sa.CurrentDeploymentID.Int64 != d.ID || sa.PreviousDeploymentID.Int64 != old.ID {
		t.Fatalf("%+v", sa)
	}
	if got := e.Eng.Running(); len(got) != 1 || got[0] != "ziv-notas-"+itoa(d.ID) {
		t.Fatalf("old still running: %v", got)
	}
	if _, err := e.Q.LatestPreDeployBackup(context.Background(), sqlc.LatestPreDeployBackupParams{AppID: e.App.ID, DeploymentID: pgtype.Int8{Int64: d.ID, Valid: true}}); err != nil {
		t.Fatalf("no pre_deploy backup: %v", err)
	}
	if _, ok := e.Eng.Images[old.ImageID]; !ok {
		t.Fatal("previous image removed; rollback needs it")
	}
}

func TestBuildFailureKeepsLogTail(t *testing.T) {
	e := setup(t)
	e.Eng.BuildLog = strings.Repeat("linha\n", 100)
	e.Eng.BuildErr = errors.New("ERROR: No matching distribution found for pandsa")
	d := e.Publish(t)
	if d.Status != "failed" || !strings.Contains(d.ErrorMessage, "montar") || strings.Count(d.BuildLog, "\n") > 41 {
		t.Fatalf("%q (%d lines)", d.ErrorMessage, strings.Count(d.BuildLog, "\n"))
	}
	if e.Shared(t).Status != "draft" {
		t.Fatal("first publish failed: app should stay draft")
	}
}

func TestRollbackWithData(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	old := e.Publish(t)
	db := e.Shared(t).DbName
	e.DBs.Data[db] = "v1"
	d := e.Publish(t)
	e.DBs.Data[db] = "v2"
	if err := e.S.Rollback(ctx, e.App.ID, true, 1); err != nil {
		t.Fatal(err)
	}
	sa := e.Shared(t)
	if sa.CurrentDeploymentID.Int64 != old.ID || sa.PreviousDeploymentID.Int64 != d.ID || e.DBs.Data[db] != "v1" {
		t.Fatalf("%+v data=%q", sa, e.DBs.Data[db])
	}
	if err := e.S.Rollback(ctx, e.App.ID, false, 1); err != nil { // volta de novo para a nova, sem mexer nos dados
		t.Fatal(err)
	}
	if e.DBs.Data[db] != "v1" {
		t.Fatal("rollback without data changed the data")
	}
}

func TestRollbackWithoutPrevious(t *testing.T) {
	e := setup(t)
	e.Publish(t)
	if err := e.S.Rollback(context.Background(), e.App.ID, false, 1); !errors.Is(err, deploy.ErrNoPrevious) {
		t.Fatalf("err %v", err)
	}
}

func TestMissingRequiredSetting(t *testing.T) {
	e := setup(t)
	e.M.Shared.Env = []manifest.EnvVar{{Name: "ADMIN_EMAILS", Label: "E-mails dos administradores", Required: true}}
	_, err := e.S.Publish(context.Background(), e.App, e.M, 1)
	var miss deploy.MissingSettings
	if !errors.As(err, &miss) || miss.Error() != "Falta preencher: E-mails dos administradores." {
		t.Fatalf("err %v", err)
	}
}

func TestNoVault(t *testing.T) {
	e := setup(t)
	if _, err := e.NewServiceWithoutVault().Publish(context.Background(), e.App, e.M, 1); !errors.Is(err, deploy.ErrNoVault) {
		t.Fatalf("err %v", err)
	}
}

func TestTestDeploymentIsSeparate(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	d, err := e.S.Test(ctx, e.App, e.M, 1)
	if err != nil {
		t.Fatal(err)
	}
	d = e.WaitDeployment(t, d.ID)
	if d.Status != "ready" || d.TestDbName == "" || e.DBs.DBs[d.TestDbName] == "" {
		t.Fatalf("%+v", d)
	}
	if _, ok := e.S.Target(e.App.ID, true); !ok {
		t.Fatal("no test target")
	}
	if e.Shared(t).Status != "draft" {
		t.Fatal("test changed the app status")
	}
	if err := e.S.EndTest(ctx, d.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := e.DBs.DBs[d.TestDbName]; ok || len(e.Eng.Running()) != 0 {
		t.Fatal("test leftovers")
	}
}

func TestStopSleepStartUnshare(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	e.Publish(t)
	if err := e.S.Sleep(ctx, e.App.ID); err != nil || e.Shared(t).Status != "sleeping" || len(e.Eng.Running()) != 0 {
		t.Fatalf("sleep %v %s", err, e.Shared(t).Status)
	}
	if err := e.S.Run(ctx, e.App.ID); err != nil || e.Shared(t).Status != "running" {
		t.Fatalf("run %v", err)
	}
	if err := e.S.Stop(ctx, e.App.ID, 1); err != nil || e.Shared(t).Status != "stopped" || !e.Shared(t).StoppedBy.Valid {
		t.Fatalf("stop %v", err)
	}
	if err := e.S.Unshare(ctx, e.App.ID, 1); err != nil {
		t.Fatal(err)
	}
	sa := e.Shared(t)
	if !sa.UnsharedAt.Valid || len(e.Eng.Running()) != 0 || len(e.Eng.Images) != 0 {
		t.Fatalf("%+v images=%d", sa, len(e.Eng.Images))
	}
	if _, ok := e.DBs.DBs[sa.DbName]; !ok {
		t.Fatal("the database must stay for 30 days")
	}
}

func TestBackupAndRestore(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	e.Publish(t)
	db := e.Shared(t).DbName
	e.DBs.Data[db] = "antes"
	b, err := e.S.Backup(ctx, e.App.ID, "manual", nil, ptr(int64(1)))
	if err != nil || b.SizeBytes == 0 {
		t.Fatalf("%+v %v", b, err)
	}
	e.DBs.Data[db] = "depois"
	if err := e.S.Restore(ctx, e.App.ID, b.ID, 1); err != nil {
		t.Fatal(err)
	}
	if e.DBs.Data[db] != "antes" || e.Shared(t).Status != "running" {
		t.Fatalf("data %q status %s", e.DBs.Data[db], e.Shared(t).Status)
	}
	list, _ := e.Q.ListBackups(ctx, e.App.ID)
	if len(list) != 2 || list[0].Kind != "pre_restore" {
		t.Fatalf("%+v", list)
	}
}

// Copies on disk are encrypted: reading the server's disk does not show the data.
func TestBackupIsEncrypted(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	e.Publish(t)
	e.DBs.Data[e.Shared(t).DbName] = "cpf-12345678900"
	b, err := e.S.Backup(ctx, e.App.ID, "manual", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(b.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !vault.Sealed(raw) || bytes.Contains(raw, []byte("cpf-12345678900")) {
		t.Fatalf("copy is not encrypted: %q", raw)
	}
}

// Copies made before encryption still restore, and SealOldBackups encrypts them.
func TestOldPlainBackups(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	e.Publish(t)
	db := e.Shared(t).DbName
	path := filepath.Join(t.TempDir(), "old.dump")
	if err := os.WriteFile(path, []byte("dump:antigo"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	old, err := e.Q.InsertBackup(ctx, sqlc.InsertBackupParams{AppID: e.App.ID, Kind: "manual", Path: path, SizeBytes: 11, ExpiresAt: now, CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.S.Restore(ctx, e.App.ID, old.ID, 1); err != nil || e.DBs.Data[db] != "antigo" {
		t.Fatalf("plain restore: %v, data %q", err, e.DBs.Data[db])
	}
	e.S.SealOldBackups(ctx)
	raw, _ := os.ReadFile(path)
	if !vault.Sealed(raw) || bytes.Contains(raw, []byte("antigo")) {
		t.Fatalf("old copy not sealed: %q", raw)
	}
	if b, _ := e.Q.GetBackup(ctx, sqlc.GetBackupParams{ID: old.ID, AppID: e.App.ID}); b.SizeBytes != int64(len(raw)) {
		t.Fatalf("size %d, file %d", b.SizeBytes, len(raw))
	}
	// A second start leaves the sealed copy as it is.
	e.S.SealOldBackups(ctx)
	if again, _ := os.ReadFile(path); !bytes.Equal(again, raw) {
		t.Fatal("sealed copy sealed again")
	}
	e.DBs.Data[db] = "outro"
	if err := e.S.Restore(ctx, e.App.ID, old.ID, 1); err != nil || e.DBs.Data[db] != "antigo" {
		t.Fatalf("sealed restore: %v, data %q", err, e.DBs.Data[db])
	}
}

// The unique index catches a second publish even when the in-memory lock
// cannot (another Service: the Hub restarted with a deployment building).
func TestDeployOneAtATimeAcrossServices(t *testing.T) {
	e := setup(t)
	release := make(chan struct{})
	e.S.SetSpawn(func(f func()) { go func() { <-release; f() }() })
	first, err := e.S.Publish(context.Background(), e.App, e.M, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.NewService().Publish(context.Background(), e.App, e.M, 1); !errors.Is(err, deploy.ErrBusy) {
		t.Fatalf("publish from another service err %v", err)
	}
	close(release)
	e.WaitDeployment(t, first.ID)
}

func TestOtherOperationsBusy(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	e.Publish(t)
	e.Publish(t)
	release := make(chan struct{})
	e.S.SetSpawn(func(f func()) { go func() { <-release; f() }() })
	d, err := e.S.Publish(ctx, e.App, e.M, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.S.Test(ctx, e.App, e.M, 1); !errors.Is(err, deploy.ErrBusy) {
		t.Errorf("test: %v", err)
	}
	if err := e.S.Rollback(ctx, e.App.ID, false, 1); !errors.Is(err, deploy.ErrBusy) {
		t.Errorf("rollback: %v", err)
	}
	if err := e.S.Restore(ctx, e.App.ID, 1, 1); !errors.Is(err, deploy.ErrBusy) {
		t.Errorf("restore: %v", err)
	}
	if err := e.S.Unshare(ctx, e.App.ID, 1); !errors.Is(err, deploy.ErrBusy) {
		t.Errorf("unshare: %v", err)
	}
	close(release)
	e.WaitDeployment(t, d.ID)
}

func TestPublishRecordsEventAndKeepsOnlyPreviousImage(t *testing.T) {
	e := setup(t)
	first := e.Publish(t)
	second := e.Publish(t)
	third := e.Publish(t)
	if third.Status != "live" {
		t.Fatalf("%s %s", third.Status, third.ErrorMessage)
	}
	if _, ok := e.Eng.Images[first.ImageID]; ok {
		t.Error("image of the version before the previous one was kept")
	}
	if _, ok := e.Eng.Images[second.ImageID]; !ok {
		t.Error("previous image removed")
	}
	if _, ok := e.Eng.Images[third.ImageID]; !ok {
		t.Error("live image removed")
	}
	old, _ := e.Q.GetDeployment(context.Background(), second.ID)
	if old.Status != "retired" {
		t.Errorf("previous deployment %s", old.Status)
	}
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM app_changes
		WHERE app_id = $1 AND person_id = 1 AND changes::text LIKE '%Publicou a versão atual%'`, e.App.ID).Scan(&n); err != nil || n != 3 {
		t.Fatalf("publish events %d %v", n, err)
	}
	if entries, _ := os.ReadDir(filepath.Join(e.DataDir(), "stage")); len(entries) != 0 {
		t.Fatalf("build area left behind: %v", entries)
	}
}

func TestEnvFromSettings(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	sealed, err := e.Vault().Seal("s3nha")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []sqlc.UpsertAppConfigParams{
		{AppID: e.App.ID, Name: "ADMIN_EMAILS", ValuePlain: "a@alis.com"},
		{AppID: e.App.ID, Name: "SMTP_PASSWORD", IsSecret: true, ValueCipher: sealed},
		{AppID: e.App.ID, Name: "_internal", ValuePlain: "nao vai"},
	} {
		if err := e.Q.UpsertAppConfig(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	e.M.Shared.Env = []manifest.EnvVar{
		{Name: "ADMIN_EMAILS", Label: "E-mails", Required: true},
		{Name: "SMTP_PASSWORD", Label: "Senha do e-mail", Secret: true, Required: true},
		{Name: "TZ", Label: "Fuso", Default: "America/Sao_Paulo"},
	}
	d := e.Publish(t)
	if d.Status != "live" {
		t.Fatalf("%s %s", d.Status, d.ErrorMessage)
	}
	var env map[string]string
	for _, c := range e.Eng.Containers {
		env = c.Spec.Env
	}
	if env["ADMIN_EMAILS"] != "a@alis.com" || env["SMTP_PASSWORD"] != "s3nha" || env["TZ"] != "America/Sao_Paulo" {
		t.Fatalf("env %v", env)
	}
	if _, ok := env["_internal"]; ok {
		t.Fatal("internal setting leaked into the container")
	}
	if _, ok := env["_db_password"]; ok {
		t.Fatal("db password leaked as a setting")
	}
	db := e.Shared(t).DbName
	if want := e.DBs.URL(db, e.DBs.DBs[db]); env["DATABASE_URL"] != want {
		t.Fatalf("DATABASE_URL %q want %q", env["DATABASE_URL"], want)
	}
	rows, _ := e.Q.ListAppConfig(ctx, e.App.ID)
	found := false
	for _, r := range rows {
		if r.Name == "_db_password" {
			found = r.IsSecret && r.ValuePlain == "" && len(r.ValueCipher) > 0
		}
	}
	if !found {
		t.Fatalf("db password not kept sealed: %+v", rows)
	}
	// The second publish reuses the same password.
	pw := e.DBs.DBs[db]
	e.Publish(t)
	if e.DBs.DBs[db] != pw {
		t.Fatal("db password changed between publishes")
	}
}

func TestMissingSettingsListsAllAndCreatesNothing(t *testing.T) {
	e := setup(t)
	e.M.Shared.Env = []manifest.EnvVar{
		{Name: "ADMIN_EMAILS", Label: "E-mails dos administradores", Required: true},
		{Name: "SMTP_PASSWORD", Label: "Senha do e-mail", Secret: true, Required: true},
		{Name: "TZ", Label: "Fuso", Required: true, Default: "UTC"},
	}
	_, err := e.S.Test(context.Background(), e.App, e.M, 1)
	if err == nil || err.Error() != "Falta preencher: E-mails dos administradores, Senha do e-mail." {
		t.Fatalf("err %v", err)
	}
	if list, _ := e.Q.ListDeployments(context.Background(), sqlc.ListDeploymentsParams{AppID: e.App.ID, Limit: 10}); len(list) != 0 {
		t.Fatalf("deployment created: %+v", list)
	}
}

func TestNoVaultRunAndTest(t *testing.T) {
	e := setup(t)
	s := e.NewServiceWithoutVault()
	if _, err := s.Test(context.Background(), e.App, e.M, 1); !errors.Is(err, deploy.ErrNoVault) {
		t.Fatalf("test err %v", err)
	}
	if err := s.Run(context.Background(), e.App.ID); !errors.Is(err, deploy.ErrNoVault) {
		t.Fatalf("run err %v", err)
	}
}

func TestTestEnvAndNewTestEndsOld(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	first := e.StartTest(t)
	if first.Status != "ready" {
		t.Fatalf("%s %s", first.Status, first.ErrorMessage)
	}
	var env map[string]string
	for _, c := range e.Eng.Containers {
		env = c.Spec.Env
	}
	if env["BASE_URL"] != "http://notas--teste.localhost:8080" ||
		env["DATABASE_URL"] != e.DBs.URL(first.TestDbName, e.DBs.DBs[first.TestDbName]) ||
		first.TestDbName != "app_"+itoa(e.App.ID)+"_test_"+itoa(first.ID) {
		t.Fatalf("env %v db %s", env, first.TestDbName)
	}
	if got := e.Eng.Running(); len(got) != 1 || got[0] != "ziv-notas-teste-"+itoa(first.ID) {
		t.Fatalf("running %v", got)
	}
	if !first.ExpiresAt.Valid || first.ExpiresAt.Time.Sub(first.CreatedAt.Time) < 29*time.Minute {
		t.Fatalf("expires %v", first.ExpiresAt)
	}
	second := e.StartTest(t)
	if old, _ := e.Q.GetDeployment(ctx, first.ID); old.Status != "retired" {
		t.Fatalf("old test %s", old.Status)
	}
	if _, ok := e.DBs.DBs[first.TestDbName]; ok {
		t.Fatal("old test database kept")
	}
	// Publishing ends the running test.
	e.Publish(t)
	if d, _ := e.Q.GetDeployment(ctx, second.ID); d.Status != "retired" {
		t.Fatalf("test after publish %s", d.Status)
	}
	if _, ok := e.S.Target(e.App.ID, true); ok {
		t.Fatal("test target kept after publish")
	}
	if got := e.Eng.Running(); len(got) != 1 || got[0] != "ziv-notas-"+itoa(e.Shared(t).CurrentDeploymentID.Int64) {
		t.Fatalf("running %v", got)
	}
}

func TestEndTestRemovesTestData(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	d := e.StartTest(t)
	dir := filepath.Join(e.DataDir(), "apps", itoa(e.App.ID), "test-data")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := e.S.EndTest(ctx, d.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("test-data kept: %v", err)
	}
	if _, ok := e.S.Target(e.App.ID, true); ok {
		t.Fatal("test target kept")
	}
}

func TestDataDirMounted(t *testing.T) {
	e := setup(t)
	e.Publish(t)
	for _, c := range e.Eng.Containers {
		if want := filepath.Join(e.DataDir(), "apps", itoa(e.App.ID), "data"); c.Spec.DataDir != want {
			t.Fatalf("data dir %q want %q", c.Spec.DataDir, want)
		}
		if c.Spec.Labels["ziv.app"] != itoa(e.App.ID) || c.Spec.Labels["ziv.deployment"] == "" || c.Spec.MemoryMB != 512 || c.Spec.CPUs != 1 {
			t.Fatalf("spec %+v", c.Spec)
		}
	}
}

func TestRollbackUsesPreviousPort(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	old := e.Publish(t)
	if old.Port != 8501 {
		t.Fatalf("port not recorded: %d", old.Port)
	}
	// The old version listened elsewhere (the recipe today says 8501).
	if _, err := pool.Exec(ctx, `UPDATE shared_deployments SET port = 8600 WHERE id = $1`, old.ID); err != nil {
		t.Fatal(err)
	}
	e.Publish(t)
	if err := e.S.Rollback(ctx, e.App.ID, false, 1); err != nil {
		t.Fatal(err)
	}
	for _, c := range e.Eng.Containers {
		if c.Running && (c.Spec.Port != 8600 || c.Spec.Env["PORT"] != "8600") {
			t.Fatalf("rolled back on port %d (PORT=%s), the old version listens on 8600", c.Spec.Port, c.Spec.Env["PORT"])
		}
	}
}

func TestBackupFailureLeavesNothing(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	e.Publish(t)
	e.DBs.FailDump = errors.New("pg_dump: boom")
	if _, err := e.S.Backup(ctx, e.App.ID, "manual", nil, nil); err == nil {
		t.Fatal("expected error")
	}
	if list, _ := e.Q.ListBackups(ctx, e.App.ID); len(list) != 0 {
		t.Fatalf("%+v", list)
	}
	if entries, _ := os.ReadDir(filepath.Join(e.DataDir(), "backups", itoa(e.App.ID))); len(entries) != 0 {
		t.Fatalf("files left: %v", entries)
	}
}

func TestTwoBackupsSameSecond(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	e.Publish(t)
	fixed := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	e.S.SetNow(func() time.Time { return fixed })
	a, err := e.S.Backup(ctx, e.App.ID, "manual", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := e.S.Backup(ctx, e.App.ID, "manual", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if a.Path == b.Path || e.S.BackupPath(a) != a.Path {
		t.Fatalf("%s %s", a.Path, b.Path)
	}
}

func TestStartResetsRestarts(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	e.Publish(t)
	if err := e.S.Sleep(ctx, e.App.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.Q.SetSharedRestarts(ctx, sqlc.SetSharedRestartsParams{AppID: e.App.ID, RestartCount: 3,
		RestartWindowStart: pgtype.Timestamptz{Time: time.Now(), Valid: true}}); err != nil {
		t.Fatal(err)
	}
	if err := e.S.Start(ctx, e.App.ID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100 && e.Shared(t).Status != "running"; i++ {
		time.Sleep(50 * time.Millisecond)
	}
	sa := e.Shared(t)
	if sa.Status != "running" || sa.RestartCount != 0 || sa.RestartWindowStart.Valid || len(e.Eng.Running()) != 1 {
		t.Fatalf("%+v running=%v", sa, e.Eng.Running())
	}
}

func TestPurge(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	e.Publish(t)
	db := e.Shared(t).DbName
	if _, err := e.S.Backup(ctx, e.App.ID, "manual", nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := e.S.Unshare(ctx, e.App.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := e.S.Purge(ctx, e.App.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := e.DBs.DBs[db]; ok {
		t.Fatal("database kept")
	}
	if _, err := e.Q.GetSharedApp(ctx, e.App.ID); err == nil {
		t.Fatal("shared row kept")
	}
	if _, err := os.Stat(filepath.Join(e.DataDir(), "backups", itoa(e.App.ID))); !os.IsNotExist(err) {
		t.Fatal("backups kept")
	}
}

func TestSlugAndURLs(t *testing.T) {
	e := setup(t)
	for _, bad := range []string{"Notas", "a--b", "-a", "a_b", strings.Repeat("a", 41)} {
		if _, err := e.S.Slug(bad); !errors.Is(err, deploy.ErrBadSlug) {
			t.Errorf("%q accepted", bad)
		}
	}
	if s, err := e.S.Slug("notas-2"); err != nil || s != "notas-2" {
		t.Fatal(err)
	}
	if e.S.AppURL("notas") != "http://notas.localhost:8080" || e.S.TestURL("notas") != "http://notas--teste.localhost:8080" {
		t.Fatal(e.S.AppURL("notas"))
	}
}

// Data restore fails during a rollback with data: the current version must
// come back up (restoreFile had stopped it).
func TestRollbackWithDataRestoreFailsKeepsAppUp(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	e.Publish(t)
	db := e.Shared(t).DbName
	e.DBs.Data[db] = "v1"
	d := e.Publish(t)
	e.DBs.Data[db] = "v2"
	e.DBs.FailRestore = errors.New("pg_restore: boom")
	err := e.S.Rollback(ctx, e.App.ID, true, 1)
	if err == nil || !strings.Contains(err.Error(), "antes de restaurar") {
		t.Fatalf("err %v", err)
	}
	sa := e.Shared(t)
	if sa.Status != "running" || sa.CurrentDeploymentID.Int64 != d.ID {
		t.Fatalf("%+v", sa)
	}
	if got := e.Eng.Running(); len(got) != 1 || got[0] != "ziv-notas-"+itoa(d.ID) {
		t.Fatalf("running %v", got)
	}
	if _, ok := e.S.Target(e.App.ID, false); !ok {
		t.Fatal("no target: the app was left down")
	}
}

// The previous version does not answer after its data was restored: the
// current data comes back and the current version is turned on again.
func TestRollbackWithDataPreviousUnhealthyPutsDataBack(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	old := e.Publish(t)
	db := e.Shared(t).DbName
	e.DBs.Data[db] = "v1"
	d := e.Publish(t)
	e.DBs.Data[db] = "v2"
	broken := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) })
	e.Eng.HandlerFor = func(name string) http.Handler {
		if name == "ziv-notas-"+itoa(old.ID) {
			return broken
		}
		return nil
	}
	err := e.S.Rollback(ctx, e.App.ID, true, 1)
	if err == nil || !strings.Contains(err.Error(), "Nada mudou") || !strings.Contains(err.Error(), "não respondeu") {
		t.Fatalf("err %v", err)
	}
	if e.DBs.Data[db] != "v2" {
		t.Fatalf("data %q: the current version runs on the old data", e.DBs.Data[db])
	}
	sa := e.Shared(t)
	if sa.Status != "running" || sa.CurrentDeploymentID.Int64 != d.ID || sa.PreviousDeploymentID.Int64 != old.ID {
		t.Fatalf("%+v", sa)
	}
	if got := e.Eng.Running(); len(got) != 1 || got[0] != "ziv-notas-"+itoa(d.ID) {
		t.Fatalf("running %v", got)
	}
}

// failWrite makes Postgres reject one bookkeeping write until the test ends.
func failWrite(t *testing.T, table, when string) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `CREATE OR REPLACE FUNCTION test_fail_write() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN IF `+when+` THEN RAISE EXCEPTION 'test: write refused'; END IF; RETURN NEW; END $$;
		CREATE TRIGGER test_fail_write BEFORE UPDATE ON `+table+` FOR EACH ROW EXECUTE FUNCTION test_fail_write();`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DROP TRIGGER IF EXISTS test_fail_write ON `+table+`; DROP FUNCTION IF EXISTS test_fail_write();`)
	})
}

func TestSwapNotRecordedKeepsOld(t *testing.T) {
	e := setup(t)
	old := e.Publish(t)
	oldPort, _ := e.S.Target(e.App.ID, false)
	failWrite(t, "shared_apps", "OLD.current_deployment_id IS NOT NULL AND NEW.current_deployment_id IS DISTINCT FROM OLD.current_deployment_id")
	d := e.Publish(t)
	if d.Status != "failed" || !strings.Contains(d.ErrorMessage, "registrar") {
		t.Fatalf("%s %q", d.Status, d.ErrorMessage)
	}
	sa := e.Shared(t)
	if sa.CurrentDeploymentID.Int64 != old.ID || sa.Status != "running" {
		t.Fatalf("%+v", sa)
	}
	if port, _ := e.S.Target(e.App.ID, false); port != oldPort {
		t.Fatal("target moved")
	}
	if got := e.Eng.Running(); len(got) != 1 || got[0] != "ziv-notas-"+itoa(old.ID) {
		t.Fatalf("running %v", got)
	}
	if len(e.Eng.Images) != 1 {
		t.Fatalf("new image kept: %v", e.Eng.Images)
	}
	// Nothing is left "building": the next publish goes through.
	e.Publish(t)
}

func TestFinishNotRecordedLeavesNoBuilding(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	failWrite(t, "shared_deployments", "OLD.status = 'building' AND NEW.status = 'live'")
	d := e.Publish(t)
	if d.Status != "failed" || !strings.Contains(d.ErrorMessage, "registrar") {
		t.Fatalf("%s %q", d.Status, d.ErrorMessage)
	}
	if e.Shared(t).Status != "draft" || len(e.Eng.Running()) != 0 || len(e.Eng.Images) != 0 {
		t.Fatalf("%s running=%v images=%v", e.Shared(t).Status, e.Eng.Running(), e.Eng.Images)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM shared_deployments WHERE status = 'building'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("building rows %d %v", n, err)
	}
}

func TestLifecycleWhilePublishing(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	e.Publish(t)
	release := make(chan struct{})
	e.S.SetSpawn(func(f func()) { go func() { <-release; f() }() })
	d, err := e.S.Publish(ctx, e.App, e.M, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.S.Stop(ctx, e.App.ID, 1); !errors.Is(err, deploy.ErrBusy) {
		t.Errorf("stop: %v", err)
	}
	if err := e.S.Start(ctx, e.App.ID); !errors.Is(err, deploy.ErrBusy) {
		t.Errorf("start: %v", err)
	}
	if err := e.S.Run(ctx, e.App.ID); !errors.Is(err, deploy.ErrBusy) {
		t.Errorf("run: %v", err)
	}
	if err := e.S.Sleep(ctx, e.App.ID); err != nil {
		t.Errorf("sleep: %v", err)
	}
	e.S.Wake(e.App.ID) // no-op: would leave a container the publish does not know
	if st := e.Shared(t).Status; st != "running" || len(e.Eng.Running()) != 1 {
		t.Fatalf("status %s running %v", st, e.Eng.Running())
	}
	close(release)
	d = e.WaitDeployment(t, d.ID)
	time.Sleep(100 * time.Millisecond) // a stray Wake would have run by now
	if got := e.Eng.Running(); d.Status != "live" || len(got) != 1 || got[0] != "ziv-notas-"+itoa(d.ID) {
		t.Fatalf("%s running %v", d.Status, got)
	}
	if e.Shared(t).Status != "running" {
		t.Fatal(e.Shared(t).Status)
	}
	starts := 0
	for _, c := range e.Eng.CallsCopy() {
		if c == "start ziv-notas-"+itoa(d.ID) {
			starts++
		}
	}
	if starts != 1 {
		t.Fatalf("new version started %d times", starts)
	}
}

func TestEndTestWhileBuilding(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	release := make(chan struct{})
	e.S.SetSpawn(func(f func()) { go func() { <-release; f() }() })
	d, err := e.S.Test(ctx, e.App, e.M, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.S.EndTest(ctx, d.ID); !errors.Is(err, deploy.ErrBusy) {
		t.Fatalf("end test while building: %v", err)
	}
	close(release)
	d = e.WaitDeployment(t, d.ID)
	if d.Status != "ready" {
		t.Fatal(d.Status)
	}
	if err := e.S.EndTest(ctx, d.ID); err != nil || len(e.Eng.Running()) != 0 {
		t.Fatalf("%v %v", err, e.Eng.Running())
	}
}

type panicBuild struct{ *engine.Fake }

func (panicBuild) Build(context.Context, engine.BuildSpec) (string, string, error) { panic("boom") }

func TestPanicInJobFailsDeployment(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	s := deploy.New(e.Q, panicBuild{e.Eng}, e.DBs, e.Vault(), deploy.Options{DataDir: e.DataDir(), Domain: "localhost:8080", HealthTimeout: time.Second})
	d, err := s.Publish(ctx, e.App, e.M, 1)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		if d, _ = e.Q.GetDeployment(ctx, d.ID); d.Status != "building" && !s.Busy(e.App.ID) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if d.Status != "failed" || s.Busy(e.App.ID) || e.Shared(t).Status != "draft" {
		t.Fatalf("%s busy=%v %s", d.Status, s.Busy(e.App.ID), e.Shared(t).Status)
	}
}

// Each app runs on its own network (live and test boxes), removed on unshare.
func TestEachAppHasItsOwnNetwork(t *testing.T) {
	e := setup(t)
	d := e.Publish(t)
	tst := e.StartTest(t)
	want := "ziv-apps-" + itoa(e.App.ID)
	for _, c := range e.Eng.Containers {
		if c.Spec.Network != want {
			t.Fatalf("%s on network %q, want %q", c.Spec.Name, c.Spec.Network, want)
		}
	}
	if d.Status != "live" || tst.Status != "ready" || !e.Eng.HasNetwork(want) {
		t.Fatalf("%s %s %v", d.Status, tst.Status, e.Eng.Networks)
	}
	if err := e.S.Unshare(context.Background(), e.App.ID, 1); err != nil {
		t.Fatal(err)
	}
	if e.Eng.HasNetwork(want) {
		t.Fatal("network left behind after unshare")
	}
}

// "Parar" wins: a restart (supervisor) or a wake (gateway) that runs after it
// does not turn the app back on. Only Ligar does.
func TestStopNotUndoneByRunOrWake(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	e.Publish(t)
	if err := e.S.Stop(ctx, e.App.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := e.S.Run(ctx, e.App.ID); !errors.Is(err, deploy.ErrStopped) {
		t.Fatalf("run after stop: %v", err)
	}
	done := make(chan struct{})
	e.S.SetSpawn(func(f func()) { go func() { defer close(done); f() }() })
	e.S.Wake(e.App.ID)
	<-done
	if st := e.Shared(t).Status; st != "stopped" || len(e.Eng.Running()) != 0 {
		t.Fatalf("stopped app came back: %s %v", st, e.Eng.Running())
	}
	e.S.SetSpawn(func(f func()) { go f() })
	if err := e.S.Start(ctx, e.App.ID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100 && e.Shared(t).Status != "running"; i++ {
		time.Sleep(50 * time.Millisecond)
	}
	if st := e.Shared(t).Status; st != "running" || len(e.Eng.Running()) != 1 {
		t.Fatalf("Ligar after Parar: %s %v", st, e.Eng.Running())
	}
}

// Restore stops the app first, so the "antes de restaurar" copy has
// everything written until then; if that copy fails the app comes back.
func TestRestoreStopsBeforeCopying(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	e.Publish(t)
	b, err := e.S.Backup(ctx, e.App.ID, "manual", nil, ptr(int64(1)))
	if err != nil {
		t.Fatal(err)
	}
	runningAtCopy := -1
	e.DBs.OnDump = func(string) { runningAtCopy = len(e.Eng.Running()) }
	if err := e.S.Restore(ctx, e.App.ID, b.ID, 1); err != nil {
		t.Fatal(err)
	}
	if runningAtCopy != 0 {
		t.Fatalf("the app was still running (%d) when its data was copied", runningAtCopy)
	}
	e.DBs.OnDump = nil
	e.DBs.FailDump = errors.New("pg_dump: boom")
	err = e.S.Restore(ctx, e.App.ID, b.ID, 1)
	if err == nil || !strings.Contains(err.Error(), "Não foi possível copiar os dados atuais") {
		t.Fatalf("err %v", err)
	}
	if st := e.Shared(t).Status; st != "running" || len(e.Eng.Running()) != 1 {
		t.Fatalf("app not brought back: %s %v", st, e.Eng.Running())
	}
	if _, ok := e.S.Target(e.App.ID, false); !ok {
		t.Fatal("no target: the app was left down")
	}
}

// A failed copy says so plainly; the technical reason stays in the log.
func TestBackupErrorIsPlain(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	e.Publish(t)
	raw := errors.New(`pg_dump: error: connection to server on socket "/var/run/postgresql/.s.PGSQL.5432" failed`)
	e.DBs.FailDump = raw
	_, err := e.S.Backup(ctx, e.App.ID, "manual", nil, ptr(int64(1)))
	if err == nil || err.Error() != "Não foi possível fazer a cópia agora. Tente de novo em alguns minutos." || !errors.Is(err, raw) {
		t.Fatalf("%v", err)
	}
}

// A test box whose database cannot be dropped still loses its address and
// container; the drop is tried again later.
func TestEndTestClearsTargetWhenDropFails(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	e.Publish(t)
	d := e.StartTest(t)
	e.DBs.FailDrop = errors.New("drop: database is being accessed by other users")
	if err := e.S.EndTest(ctx, d.ID); err == nil {
		t.Fatal("drop error not reported")
	}
	if _, open := e.S.Target(e.App.ID, true); open {
		t.Fatal("test address still open")
	}
	for _, n := range e.Eng.Running() {
		if strings.Contains(n, "teste") {
			t.Fatalf("test container still running: %v", e.Eng.Running())
		}
	}
	e.DBs.FailDrop = nil
	if err := e.S.EndTest(ctx, d.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.Q.GetDeployment(ctx, d.ID); got.Status != "retired" {
		t.Fatalf("status %s", got.Status)
	}
}

func TestMoveBackupPathsAfterAdoptingDataDir(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	old, nw := "/home/x/.local/share/app-hub/shared", "/home/x/.local/share/ziv/shared"
	b, err := e.Q.InsertBackup(ctx, sqlc.InsertBackupParams{AppID: e.App.ID, Kind: "daily", Path: old + "/backups/1/a.dump", SizeBytes: 1, CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}, ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	other, _ := e.Q.InsertBackup(ctx, sqlc.InsertBackupParams{AppID: e.App.ID, Kind: "daily", Path: "/elsewhere/app-hub/shared/b.dump", SizeBytes: 1, CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}, ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}})
	n, err := deploy.MoveBackupPaths(ctx, pool, old, nw)
	if err != nil || n != 1 {
		t.Fatalf("moved %d %v", n, err)
	}
	var got, untouched string
	pool.QueryRow(ctx, `SELECT path FROM shared_backups WHERE id = $1`, b.ID).Scan(&got)
	pool.QueryRow(ctx, `SELECT path FROM shared_backups WHERE id = $1`, other.ID).Scan(&untouched)
	if got != nw+"/backups/1/a.dump" || untouched != "/elsewhere/app-hub/shared/b.dump" {
		t.Fatalf("got %q, untouched %q", got, untouched)
	}
	if n, _ := deploy.MoveBackupPaths(ctx, pool, old, nw); n != 0 {
		t.Fatalf("second run moved %d", n)
	}
}

func TestPublishRemovesLegacyNetwork(t *testing.T) {
	e := setup(t)
	e.Publish(t)
	e.Publish(t)
	want := "remove-network app-hub-apps-" + itoa(e.App.ID)
	for _, c := range e.Eng.CallsCopy() {
		if c == want {
			return
		}
	}
	t.Fatalf("no %q in %v", want, e.Eng.CallsCopy())
}
