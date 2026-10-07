// Package deploy runs shared apps: test, publish (side by side), roll back,
// start, sleep, stop, unshare, copies. Everything that changes a shared
// app's state goes through here.
package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/appdb"
	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
	"github.com/HenryKropp/app-hub/backend/internal/shared/health"
	"github.com/HenryKropp/app-hub/backend/internal/shared/recipes"
	"github.com/HenryKropp/app-hub/backend/internal/shared/stage"
	"github.com/HenryKropp/app-hub/backend/internal/vault"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

var (
	ErrBusy       = errors.New("Já existe uma publicação em andamento para este app.")
	ErrNoVault    = errors.New("Os apps compartilhados precisam da chave HUB_SECRET_KEY no servidor. Peça ao administrador para configurá-la.")
	ErrNoPrevious = errors.New("Não há versão anterior para voltar.")
	ErrNotShared  = errors.New("Este app ainda não é compartilhado.")
	ErrBadSlug    = errors.New("O nome curto do app só pode ter letras minúsculas, números e hífens (sem dois hífens seguidos) para virar endereço.")
)

type Options struct {
	DataDir       string        // SHARED_DATA_DIR
	Domain        string        // SHARED_APPS_DOMAIN, ex.: "localhost:8080"
	Scheme        string        // "http" ou "https" (do BASE_URL do Hub)
	HealthTimeout time.Duration // 90 s; testes usam 1 s
}

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

type Service struct {
	q     *sqlc.Queries
	eng   engine.Engine
	dbs   appdb.DBs
	vault *vault.Vault
	opt   Options
	spawn func(func())
	now   func() time.Time

	mu            sync.Mutex
	busy          map[int64]bool
	waking        map[int64]bool
	ports         map[int64]int     // app → porta da versão no ar
	testPorts     map[int64]int     // app → porta da caixa de teste
	testPasswords map[string]string // banco de teste → senha (só em memória)
}

func New(q *sqlc.Queries, eng engine.Engine, dbs appdb.DBs, v *vault.Vault, opt Options) *Service {
	if opt.HealthTimeout == 0 {
		opt.HealthTimeout = 90 * time.Second
	}
	if opt.Scheme == "" {
		opt.Scheme = "http"
	}
	return &Service{q: q, eng: eng, dbs: dbs, vault: v, opt: opt,
		spawn: func(f func()) { go f() }, now: time.Now,
		busy: map[int64]bool{}, waking: map[int64]bool{}, ports: map[int64]int{}, testPorts: map[int64]int{},
		testPasswords: map[string]string{}}
}

// SetSpawn controls when background work runs (tests). Call before use.
func (s *Service) SetSpawn(fn func(func())) { s.spawn = fn }

// SetNow replaces the clock (tests). Call before use.
func (s *Service) SetNow(fn func() time.Time) { s.now = fn }

// Slug validates the app's slug for use in an address.
func (s *Service) Slug(appSlug string) (string, error) {
	if len(appSlug) > 40 || !slugRe.MatchString(appSlug) {
		return "", ErrBadSlug
	}
	return appSlug, nil
}

func (s *Service) AppURL(slug string) string { return s.opt.Scheme + "://" + slug + "." + s.opt.Domain }
func (s *Service) TestURL(slug string) string {
	return s.opt.Scheme + "://" + slug + "--teste." + s.opt.Domain
}

// Target is the host port (on 127.0.0.1) of the app's live (or test) container.
func (s *Service) Target(appID int64, test bool) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.ports
	if test {
		m = s.testPorts
	}
	p, ok := m[appID]
	return p, ok
}

func (s *Service) setTarget(appID int64, test bool, port int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.ports
	if test {
		m = s.testPorts
	}
	if port == 0 {
		delete(m, appID)
	} else {
		m[appID] = port
	}
}

func (s *Service) setTestPassword(db, pw string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if pw == "" {
		delete(s.testPasswords, db)
	} else {
		s.testPasswords[db] = pw
	}
}

func (s *Service) testPassword(db string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.testPasswords[db]
}

// Busy reports whether a test, publish, rollback, restore or unshare is
// running for the app in this Hub.
func (s *Service) Busy(appID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.busy[appID]
}

// lock marks the app busy; the caller must call the returned func when done.
func (s *Service) lock(appID int64) (func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy[appID] {
		return nil, ErrBusy
	}
	s.busy[appID] = true
	return func() { s.mu.Lock(); delete(s.busy, appID); s.mu.Unlock() }, nil
}

// Configure creates or updates the shared_apps row from the manifest
// (marking the app shared, status draft if new).
func (s *Service) Configure(ctx context.Context, app sqlc.App, m manifest.Manifest) (sqlc.SharedApp, error) {
	slug, err := s.Slug(app.Slug)
	if err != nil {
		return sqlc.SharedApp{}, err
	}
	sh := manifest.Shared{Enabled: true}
	if m.Shared != nil {
		sh = *m.Shared
	}
	sh = sh.WithDefaults()
	return s.q.UpsertSharedApp(ctx, sqlc.UpsertSharedAppParams{
		AppID: app.ID, Slug: slug, AlwaysOn: sh.AlwaysOn, IdleMinutes: int32(sh.IdleMin), MemoryMb: int32(sh.MemoryMB),
		Cpus: numeric(sh.CPUs), HealthPath: sh.HealthPath, Port: int32(sh.Port), WantsDb: sh.Database == "postgres",
	})
}

type job struct {
	app  sqlc.App
	sa   sqlc.SharedApp
	m    manifest.Manifest
	dep  sqlc.SharedDeployment
	rec  recipes.Recipe
	by   int64
	test bool
}

func (s *Service) Publish(ctx context.Context, app sqlc.App, m manifest.Manifest, by int64) (sqlc.SharedDeployment, error) {
	return s.begin(ctx, app, m, by, false)
}

func (s *Service) Test(ctx context.Context, app sqlc.App, m manifest.Manifest, by int64) (sqlc.SharedDeployment, error) {
	return s.begin(ctx, app, m, by, true)
}

// begin checks everything that can be checked right away (so the person sees
// the error on the button), records the deployment and runs the rest in the
// background.
func (s *Service) begin(ctx context.Context, app sqlc.App, m manifest.Manifest, by int64, test bool) (sqlc.SharedDeployment, error) {
	if s.vault == nil {
		return sqlc.SharedDeployment{}, ErrNoVault
	}
	if err := s.eng.Available(ctx); err != nil {
		return sqlc.SharedDeployment{}, err
	}
	unlock, err := s.lock(app.ID)
	if err != nil {
		return sqlc.SharedDeployment{}, err
	}
	dep, j, err := s.prepare(ctx, app, m, by, test)
	if err != nil {
		unlock()
		return sqlc.SharedDeployment{}, err
	}
	s.spawn(func() {
		defer unlock()
		s.runJob(context.Background(), j)
	})
	return dep, nil
}

func (s *Service) prepare(ctx context.Context, app sqlc.App, m manifest.Manifest, by int64, test bool) (sqlc.SharedDeployment, job, error) {
	sa, err := s.Configure(ctx, app, m)
	if err != nil {
		return sqlc.SharedDeployment{}, job{}, err
	}
	rec, err := recipes.Detect(versions.LocalFolder{Root: app.FolderPath}, m)
	if err != nil {
		return sqlc.SharedDeployment{}, job{}, err
	}
	if rec.Kind == "" {
		return sqlc.SharedDeployment{}, job{}, errors.New("O Hub não sabe ligar este app. Veja a checagem para o que falta.")
	}
	if rec.Port == 0 {
		rec.Port = int(sa.Port)
	}
	missing, err := s.missingSettings(ctx, app.ID, m)
	if err != nil {
		return sqlc.SharedDeployment{}, job{}, err
	}
	if len(missing) > 0 {
		return sqlc.SharedDeployment{}, job{}, MissingSettings{Names: missing}
	}
	kind := "live"
	var expires pgtype.Timestamptz
	if test {
		kind = "test"
		expires = pgtype.Timestamptz{Time: s.now().Add(30 * time.Minute), Valid: true}
	}
	var versionID pgtype.Int8
	if v, err := s.q.LatestAppVersion(ctx, app.ID); err == nil {
		versionID = pgtype.Int8{Int64: v.ID, Valid: true}
	}
	dep, err := s.q.InsertDeployment(ctx, sqlc.InsertDeploymentParams{AppID: app.ID, VersionID: versionID, Kind: kind,
		Recipe: rec.Dockerfile, CreatedBy: pgtype.Int8{Int64: by, Valid: true}, ExpiresAt: expires})
	if err != nil {
		if isUniqueViolation(err) {
			return sqlc.SharedDeployment{}, job{}, ErrBusy
		}
		return sqlc.SharedDeployment{}, job{}, err
	}
	if !test && !sa.CurrentDeploymentID.Valid {
		_ = s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: app.ID, Status: "starting"})
	}
	return dep, job{app: app, sa: sa, m: m, dep: dep, rec: rec, by: by, test: test}, nil
}

// failure is what went wrong, in Portuguese, plus the log for "Ver detalhes".
type failure struct {
	msg string
	log string
}

func (s *Service) runJob(ctx context.Context, j job) {
	var f *failure
	if j.test {
		f = s.runTest(ctx, j)
	} else {
		f = s.runPublish(ctx, j)
	}
	if f == nil {
		return
	}
	slog.Warn("shared deploy failed", "app", j.app.ID, "deployment", j.dep.ID, "msg", f.msg, "log", lastLines(f.log, 10))
	if !j.test && !j.sa.CurrentDeploymentID.Valid {
		_ = s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: j.app.ID, Status: "draft", StatusMessage: f.msg})
	}
	if err := s.q.FinishDeployment(ctx, sqlc.FinishDeploymentParams{ID: j.dep.ID, Status: "failed",
		Port: int32(j.rec.Port), BuildLog: lastLines(f.log, 40), ErrorMessage: f.msg}); err != nil {
		slog.Error("shared deploy: record failure", "deployment", j.dep.ID, "err", err)
	}
}

// build copies the folder and builds the image. It returns the image ID and the log.
func (s *Service) build(ctx context.Context, j job) (string, string, *failure) {
	dir := filepath.Join(s.opt.DataDir, "stage", strconv.FormatInt(j.dep.ID, 10))
	_ = os.RemoveAll(dir) // stage.Copy wants a fresh folder
	defer os.RemoveAll(dir)
	df, err := stage.Copy(versions.LocalFolder{Root: j.app.FolderPath}, dir, j.m.Outputs, j.rec)
	if errors.Is(err, stage.ErrTooBig) {
		return "", "", &failure{msg: "A pasta do app passa de 500 MB. Tire arquivos grandes que não são do app (ou liste-os no .dockerignore)."}
	}
	if err != nil {
		return "", "", &failure{msg: "Não foi possível ler a pasta do app. Confira se ela existe e está sincronizada.", log: err.Error()}
	}
	labels := map[string]string{engine.Label: strconv.FormatInt(j.app.ID, 10)}
	img, log, err := s.eng.Build(ctx, engine.BuildSpec{ContextDir: dir, DockerfilePath: df,
		Tag: fmt.Sprintf("localhost/apphub/%s:%d", j.sa.Slug, j.dep.ID), Labels: labels})
	if err != nil {
		full := log + "\n" + err.Error()
		return "", log, &failure{msg: "Não foi possível montar o app. " + explainBuild(full), log: full}
	}
	return img, log, nil
}

// start starts a container for the deployment and waits for health. On
// failure the container is removed.
func (s *Service) start(ctx context.Context, j job, img, dbName string) (string, int, *failure) {
	env, err := s.env(ctx, j.app.ID, j.m, j.sa, dbName, j.rec.Port, j.test)
	if err != nil {
		return "", 0, &failure{msg: "Não foi possível preparar as configurações do app.", log: err.Error()}
	}
	appDir := filepath.Join(s.opt.DataDir, "apps", strconv.FormatInt(j.app.ID, 10))
	name := fmt.Sprintf("apphub-%s-%d", j.sa.Slug, j.dep.ID)
	data := filepath.Join(appDir, "data")
	if j.test {
		name = fmt.Sprintf("apphub-%s-teste-%d", j.sa.Slug, j.dep.ID)
		data = filepath.Join(appDir, "test-data")
	}
	if err := os.MkdirAll(data, 0o755); err != nil {
		return "", 0, &failure{msg: "Não foi possível criar a pasta de dados do app no servidor.", log: err.Error()}
	}
	cid, err := s.eng.Start(ctx, engine.StartSpec{Name: name, Image: img, Env: env, Port: j.rec.Port,
		MemoryMB: int(j.sa.MemoryMb), CPUs: toFloat(j.sa.Cpus), DataDir: data,
		Labels: map[string]string{engine.Label: strconv.FormatInt(j.app.ID, 10), "app-hub.deployment": strconv.FormatInt(j.dep.ID, 10)}})
	if err != nil {
		return "", 0, &failure{msg: "Não foi possível ligar o app no servidor.", log: err.Error()}
	}
	st, err := s.eng.Inspect(ctx, cid)
	if err == nil && st.HostPort != 0 {
		err = health.Wait(ctx, fmt.Sprintf("http://127.0.0.1:%d%s", st.HostPort, j.sa.HealthPath), s.opt.HealthTimeout)
	} else if err == nil {
		err = health.ErrNoAnswer
	}
	if err != nil {
		logs, _ := s.eng.Logs(ctx, cid, 40)
		_ = s.eng.Remove(ctx, cid)
		return "", 0, &failure{msg: fmt.Sprintf("O app ligou, mas não respondeu na porta %d. Confira se ele escuta em 0.0.0.0 na porta da variável PORT.", j.rec.Port), log: logs}
	}
	return cid, st.HostPort, nil
}

func (s *Service) runPublish(ctx context.Context, j job) *failure {
	if t, err := s.q.ListOpenTests(ctx, j.app.ID); err == nil { // encerra o teste em andamento
		for _, d := range t {
			_ = s.EndTest(ctx, d.ID)
		}
	}
	live := j.sa.CurrentDeploymentID.Valid
	if live && j.sa.WantsDb && j.sa.DbName != "" {
		depID := j.dep.ID
		if _, err := s.Backup(ctx, j.app.ID, "pre_deploy", &depID, &j.by); err != nil {
			return &failure{msg: "Não foi possível fazer a cópia de segurança antes de publicar. Nada mudou no app.", log: err.Error()}
		}
	}
	img, log, f := s.build(ctx, j)
	if f != nil {
		return f
	}
	dbName := ""
	if j.sa.WantsDb {
		dbName = s.dbs.Name(j.app.ID)
		pw, err := s.dbPassword(ctx, j.app.ID)
		if err == nil {
			err = s.dbs.Ensure(ctx, dbName, pw)
		}
		if err == nil {
			err = s.q.SetSharedDBName(ctx, sqlc.SetSharedDBNameParams{AppID: j.app.ID, DbName: dbName})
		}
		if err != nil {
			_ = s.eng.RemoveImage(ctx, img)
			return &failure{msg: "Não foi possível preparar o banco do app.", log: log + "\n" + err.Error()}
		}
	}
	cid, port, f := s.start(ctx, j, img, dbName)
	if f != nil {
		_ = s.eng.RemoveImage(ctx, img)
		f.log = log + "\n" + f.log
		return f
	}
	// Troca: o porteiro passa a mandar para a nova, depois a antiga para.
	s.setTarget(j.app.ID, false, port)
	var prev pgtype.Int8
	if live {
		if old, err := s.q.GetDeployment(ctx, j.sa.CurrentDeploymentID.Int64); err == nil {
			if old.ContainerID != "" {
				_ = s.eng.Remove(ctx, old.ContainerID)
			}
			_ = s.q.SetDeploymentContainer(ctx, sqlc.SetDeploymentContainerParams{ID: old.ID})
			_ = s.q.SetDeploymentStatus(ctx, sqlc.SetDeploymentStatusParams{ID: old.ID, Status: "retired"})
			prev = pgtype.Int8{Int64: old.ID, Valid: true}
		}
		if j.sa.PreviousDeploymentID.Valid { // só a anterior guarda imagem
			if older, err := s.q.GetDeployment(ctx, j.sa.PreviousDeploymentID.Int64); err == nil && older.ImageID != "" {
				_ = s.eng.RemoveImage(ctx, older.ImageID)
			}
		}
	}
	_ = s.q.SetSharedDeployments(ctx, sqlc.SetSharedDeploymentsParams{AppID: j.app.ID,
		CurrentID: pgtype.Int8{Int64: j.dep.ID, Valid: true}, PreviousID: prev})
	_ = s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: j.app.ID, Status: "running"})
	_ = s.q.SetSharedAccessNow(ctx, j.app.ID)
	s.event(ctx, j.app.ID, &j.by, "Publicou "+s.versionLabel(ctx, j.dep))
	// Last: whoever waits for the deployment to leave "building" sees everything done.
	_ = s.q.FinishDeployment(ctx, sqlc.FinishDeploymentParams{ID: j.dep.ID, Status: "live", ImageID: img, ContainerID: cid,
		Port: int32(j.rec.Port), BuildLog: lastLines(log, 40)})
	return nil
}

func (s *Service) runTest(ctx context.Context, j job) *failure {
	if t, err := s.q.ListOpenTests(ctx, j.app.ID); err == nil { // um teste novo encerra o anterior
		for _, d := range t {
			if d.ID != j.dep.ID {
				_ = s.EndTest(ctx, d.ID)
			}
		}
	}
	img, log, f := s.build(ctx, j)
	if f != nil {
		return f
	}
	dbName := ""
	if j.sa.WantsDb {
		dbName = s.dbs.TestName(j.app.ID, j.dep.ID)
		pw := appdb.NewPassword()
		err := s.dbs.Ensure(ctx, dbName, pw)
		if err == nil {
			err = s.q.SetDeploymentTestDB(ctx, sqlc.SetDeploymentTestDBParams{ID: j.dep.ID, TestDbName: dbName})
		}
		if err != nil {
			_ = s.eng.RemoveImage(ctx, img)
			_ = s.dbs.Drop(ctx, dbName)
			return &failure{msg: "Não foi possível preparar o banco de teste.", log: log + "\n" + err.Error()}
		}
		s.setTestPassword(dbName, pw)
	}
	cid, port, f := s.start(ctx, j, img, dbName)
	if f != nil {
		_ = s.eng.RemoveImage(ctx, img)
		if dbName != "" {
			_ = s.dbs.Drop(ctx, dbName)
			s.setTestPassword(dbName, "")
		}
		f.log = log + "\n" + f.log
		return f
	}
	s.setTarget(j.app.ID, true, port)
	_ = s.q.FinishDeployment(ctx, sqlc.FinishDeploymentParams{ID: j.dep.ID, Status: "ready", ImageID: img, ContainerID: cid,
		Port: int32(j.rec.Port), BuildLog: lastLines(log, 40)})
	return nil
}

// EndTest removes a test deployment: container, image, test database and test data.
func (s *Service) EndTest(ctx context.Context, depID int64) error {
	d, err := s.q.GetDeployment(ctx, depID)
	if err != nil {
		return err
	}
	if d.ContainerID != "" {
		_ = s.eng.Remove(ctx, d.ContainerID)
	}
	if d.ImageID != "" {
		_ = s.eng.RemoveImage(ctx, d.ImageID)
	}
	if d.TestDbName != "" {
		if err := s.dbs.Drop(ctx, d.TestDbName); err != nil {
			return err
		}
		s.setTestPassword(d.TestDbName, "")
	}
	_ = os.RemoveAll(filepath.Join(s.opt.DataDir, "apps", strconv.FormatInt(d.AppID, 10), "test-data"))
	s.setTarget(d.AppID, true, 0)
	return s.q.SetDeploymentStatus(ctx, sqlc.SetDeploymentStatusParams{ID: d.ID, Status: "retired"})
}

// Rollback puts the previous version back in the air (optionally with the
// data from before the current version was published).
func (s *Service) Rollback(ctx context.Context, appID int64, withData bool, by int64) error {
	unlock, err := s.lock(appID)
	if err != nil {
		return err
	}
	defer unlock()
	sa, err := s.q.GetSharedApp(ctx, appID)
	if err != nil {
		return ErrNotShared
	}
	if !sa.PreviousDeploymentID.Valid || !sa.CurrentDeploymentID.Valid {
		return ErrNoPrevious
	}
	cur, err := s.q.GetDeployment(ctx, sa.CurrentDeploymentID.Int64)
	if err != nil {
		return err
	}
	prev, err := s.q.GetDeployment(ctx, sa.PreviousDeploymentID.Int64)
	if err != nil {
		return err
	}
	if prev.ImageID == "" {
		return ErrNoPrevious
	}
	if withData {
		b, err := s.q.LatestPreDeployBackup(ctx, sqlc.LatestPreDeployBackupParams{AppID: appID, DeploymentID: pgtype.Int8{Int64: cur.ID, Valid: true}})
		if err != nil {
			return errors.New("Não há cópia de segurança de antes desta versão para voltar os dados.")
		}
		if err := s.restoreFile(ctx, sa, b, by); err != nil {
			return err
		}
	}
	app, err := s.q.GetApp(ctx, appID)
	if err != nil {
		return err
	}
	m := manifestOf(app)
	port := int(prev.Port)
	if port == 0 {
		rec, _ := recipes.Detect(versions.LocalFolder{Root: app.FolderPath}, m)
		port = rec.Port
		if port == 0 {
			port = int(sa.Port)
		}
	}
	j := job{app: app, sa: sa, m: m, dep: prev, rec: recipes.Recipe{Port: port}, by: by}
	cid, hostPort, f := s.start(ctx, j, prev.ImageID, sa.DbName)
	if f != nil {
		slog.Warn("shared rollback failed", "app", appID, "deployment", prev.ID, "msg", f.msg)
		if withData { // restoreFile already stopped the current version: bring it back
			if err := s.Run(ctx, appID); err != nil {
				slog.Warn("shared rollback: restart current", "app", appID, "err", err)
			}
		}
		return errors.New(f.msg)
	}
	s.setTarget(appID, false, hostPort)
	if cur.ContainerID != "" {
		_ = s.eng.Remove(ctx, cur.ContainerID)
	}
	_ = s.q.SetDeploymentContainer(ctx, sqlc.SetDeploymentContainerParams{ID: cur.ID})
	_ = s.q.SetDeploymentContainer(ctx, sqlc.SetDeploymentContainerParams{ID: prev.ID, ContainerID: cid})
	_ = s.q.SetDeploymentStatus(ctx, sqlc.SetDeploymentStatusParams{ID: prev.ID, Status: "live"})
	_ = s.q.SetDeploymentStatus(ctx, sqlc.SetDeploymentStatusParams{ID: cur.ID, Status: "retired"})
	_ = s.q.SetSharedDeployments(ctx, sqlc.SetSharedDeploymentsParams{AppID: appID,
		CurrentID: pgtype.Int8{Int64: prev.ID, Valid: true}, PreviousID: pgtype.Int8{Int64: cur.ID, Valid: true}})
	_ = s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: appID, Status: "running"})
	label := "Voltou para " + s.versionLabel(ctx, prev)
	if withData {
		label += " (com os dados de antes)"
	}
	s.event(ctx, appID, &by, label)
	return nil
}

func numeric(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(strconv.FormatFloat(f, 'f', 2, 64))
	return n
}

func toFloat(n pgtype.Numeric) float64 {
	f, _ := n.Float64Value()
	return f.Float64
}

func isUniqueViolation(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && pe.Code == "23505"
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// explainBuild recognizes common build errors and says what to do.
func explainBuild(log string) string {
	switch {
	case strings.Contains(log, "No matching distribution found"), strings.Contains(log, "Could not find a version"):
		return "Uma biblioteca da lista não foi encontrada. Confira os nomes no requirements.txt."
	case strings.Contains(log, "npm ERR!") && strings.Contains(log, "ci"):
		return "As bibliotecas do Node não instalaram. Rode npm install no app e salve o package-lock.json."
	case strings.Contains(log, "no such file or directory"), strings.Contains(log, "COPY failed"):
		return "Falta um arquivo que a receita usa. Veja os detalhes."
	case strings.Contains(log, "toomanyrequests"), strings.Contains(log, "dial tcp"):
		return "O servidor não conseguiu baixar a base da receita. Tente de novo em alguns minutos."
	}
	return "Veja os detalhes para o motivo."
}

func manifestOf(app sqlc.App) manifest.Manifest {
	var m manifest.Manifest
	_ = json.Unmarshal(app.Manifest, &m)
	return m
}

func (s *Service) versionLabel(ctx context.Context, d sqlc.SharedDeployment) string {
	if d.VersionID.Valid {
		if v, err := s.q.GetAppVersionByID(ctx, d.VersionID.Int64); err == nil {
			return fmt.Sprintf("a versão %d", v.Number)
		}
	}
	return "a versão atual"
}

func (s *Service) event(ctx context.Context, appID int64, by *int64, label string) {
	raw, _ := json.Marshal([]manifest.Change{{Field: "shared", Label: label}})
	var err error
	if by == nil {
		err = s.q.InsertHubChange(ctx, sqlc.InsertHubChangeParams{AppID: appID, Changes: raw})
	} else {
		err = s.q.InsertAppChange(ctx, sqlc.InsertAppChangeParams{AppID: appID, PersonID: pgtype.Int8{Int64: *by, Valid: true}, Changes: raw})
	}
	if err != nil {
		slog.Warn("shared event", "app", appID, "err", err)
	}
}
