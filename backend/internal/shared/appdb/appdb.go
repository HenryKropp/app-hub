// Package appdb gives each shared app its own database and user on the
// Hub's Postgres server, and copies and restores them.
package appdb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
)

var safeName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,40}$`)
var safePassword = regexp.MustCompile(`^[A-Za-z0-9]{1,128}$`)
var errBadName = errors.New("nome de banco inválido")
var errBadPassword = errors.New("senha de banco inválida")

// DBs is what deploy and the supervisor need; Admin is the real one, Fake the test one.
type DBs interface {
	Ensure(ctx context.Context, name, password string) error  // cria (ou acerta a senha de) usuário e banco
	Drop(ctx context.Context, name string) error              // apaga banco e usuário; sem erro se não existem
	URL(name, password string) string                         // DATABASE_URL para a caixa
	Dump(ctx context.Context, name string, w io.Writer) error // formato custom do pg_dump
	Restore(ctx context.Context, name string, r io.Reader) error
	Size(ctx context.Context, name string) (int64, error)
	Name(appID int64) string                   // banco do app no ar
	TestName(appID, deploymentID int64) string // banco vazio da caixa de teste
}

type Admin struct {
	pool      *pgxpool.Pool
	host      string
	eng       engine.Engine
	container string
	adminUser string
	Prefix    string
}

func New(pool *pgxpool.Pool, hostForContainers string, eng engine.Engine, dbContainer, adminUser string) *Admin {
	return &Admin{pool: pool, host: hostForContainers, eng: eng, container: dbContainer, adminUser: adminUser, Prefix: "app_"}
}

func (a *Admin) Name(appID int64) string { return fmt.Sprintf("%s%d", a.Prefix, appID) }

func (a *Admin) TestName(appID, deploymentID int64) string {
	return fmt.Sprintf("%s%d_test_%d", a.Prefix, appID, deploymentID)
}

// NewPassword returns 24 random bytes in hex.
func NewPassword() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func ident(s string) string { return pgx.Identifier{s}.Sanitize() }

func (a *Admin) Ensure(ctx context.Context, name, password string) error {
	if !safeName.MatchString(name) {
		return errBadName
	}
	if !safePassword.MatchString(password) {
		return errBadPassword
	}
	var roleExists, dbExists bool
	if err := a.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1)", name).Scan(&roleExists); err != nil {
		return err
	}
	pw := "'" + password + "'" // alphanumeric only (checked above), so safe to inline
	verb := "CREATE"
	if roleExists {
		verb = "ALTER"
	}
	if _, err := a.pool.Exec(ctx, verb+" ROLE "+ident(name)+" WITH LOGIN PASSWORD "+pw+" CONNECTION LIMIT 20"); err != nil {
		return err
	}
	// Postgres 16+: CREATE DATABASE ... OWNER needs the creator to be able to SET ROLE to it.
	if _, err := a.pool.Exec(ctx, "GRANT "+ident(name)+" TO CURRENT_USER"); err != nil {
		return err
	}
	if err := a.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)", name).Scan(&dbExists); err != nil {
		return err
	}
	if !dbExists {
		if _, err := a.pool.Exec(ctx, "CREATE DATABASE "+ident(name)+" OWNER "+ident(name)); err != nil {
			return err
		}
	}
	_, err := a.pool.Exec(ctx, "REVOKE CONNECT ON DATABASE "+ident(name)+" FROM PUBLIC")
	return err
}

func (a *Admin) Drop(ctx context.Context, name string) error {
	if !safeName.MatchString(name) {
		return errBadName
	}
	if _, err := a.pool.Exec(ctx, "DROP DATABASE IF EXISTS "+ident(name)+" WITH (FORCE)"); err != nil {
		return err
	}
	_, err := a.pool.Exec(ctx, "DROP ROLE IF EXISTS "+ident(name))
	return err
}

// LockDown stops app users from opening the Hub's own database.
func (a *Admin) LockDown(ctx context.Context, hubDB string) error {
	if !safeName.MatchString(hubDB) {
		return errBadName
	}
	_, err := a.pool.Exec(ctx, "REVOKE CONNECT ON DATABASE "+ident(hubDB)+" FROM PUBLIC")
	return err
}

func (a *Admin) URL(name, password string) string {
	u := url.URL{Scheme: "postgres", User: url.UserPassword(name, password), Host: a.host, Path: "/" + name, RawQuery: "sslmode=disable"}
	return u.String()
}

func (a *Admin) Dump(ctx context.Context, name string, w io.Writer) error {
	if !safeName.MatchString(name) {
		return errBadName
	}
	return a.eng.Exec(ctx, a.container, nil, w, "pg_dump", "-U", a.adminUser, "-Fc", "--no-owner", name)
}

func (a *Admin) Restore(ctx context.Context, name string, r io.Reader) error {
	if !safeName.MatchString(name) {
		return errBadName
	}
	// Custom-format copies (pg_dump -Fc) load in one transaction: a failed
	// restore leaves the database as it was.
	return a.eng.Exec(ctx, a.container, r, io.Discard, "pg_restore", "-U", a.adminUser, "--clean", "--if-exists",
		"--single-transaction", "--exit-on-error", "--no-owner", "--role="+name, "-d", name)
}

func (a *Admin) Size(ctx context.Context, name string) (int64, error) {
	var n int64
	err := a.pool.QueryRow(ctx, "SELECT pg_database_size($1)", name).Scan(&n)
	return n, err
}

// Fake keeps databases and copies in memory.
type Fake struct {
	mu          sync.Mutex
	DBs         map[string]string // nome → senha
	Data        map[string]string // conteúdo "do banco", para provar restaurações
	FailDump    error
	FailRestore error             // Restore fails with this (and changes nothing)
	OnDump      func(name string) // called at the start of every Dump (tests)
	FailDrop    error             // Drop fails with this (and changes nothing)
	FailSize    error             // Size fails with this
}

func NewFake() *Fake { return &Fake{DBs: map[string]string{}, Data: map[string]string{}} }

func (f *Fake) Ensure(_ context.Context, n, pw string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.DBs[n] = pw
	return nil
}

func (f *Fake) Drop(_ context.Context, n string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.FailDrop != nil {
		return f.FailDrop
	}
	delete(f.DBs, n)
	delete(f.Data, n)
	return nil
}

func (f *Fake) URL(n, pw string) string { return "postgres://" + n + ":" + pw + "@db/" + n }

func (f *Fake) Dump(_ context.Context, n string, w io.Writer) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.OnDump != nil {
		f.OnDump(n)
	}
	if f.FailDump != nil {
		return f.FailDump
	}
	_, err := io.WriteString(w, "dump:"+f.Data[n])
	return err
}

func (f *Fake) Restore(_ context.Context, n string, r io.Reader) error {
	b, err := io.ReadAll(r)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.FailRestore != nil {
		return f.FailRestore
	}
	f.Data[n] = strings.TrimPrefix(string(b), "dump:")
	return err
}

func (f *Fake) Name(appID int64) string { return fmt.Sprintf("app_%d", appID) }

func (f *Fake) TestName(appID, dep int64) string { return fmt.Sprintf("app_%d_test_%d", appID, dep) }

func (f *Fake) Size(_ context.Context, n string) (int64, error) {
	if f.FailSize != nil {
		return 0, f.FailSize
	}
	return 8 << 20, nil
}

var _ DBs = (*Admin)(nil)
var _ DBs = (*Fake)(nil)
