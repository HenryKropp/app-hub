// Package workspace routes explicit workspace paths to independent database
// namespaces and independent handler state. The URL is a selector, not identity.
package workspace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/HenryKropp/app-hub/backend/internal/config"
	"github.com/HenryKropp/app-hub/backend/internal/database"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/handler"
	"github.com/HenryKropp/app-hub/backend/internal/shared/tenant"
	"github.com/HenryKropp/app-hub/backend/internal/vault"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type entry struct {
	name, slug, schema string
	key                string // tenant.KeyOf(schema): names the company's containers, databases and folders
	pool               *pgxpool.Pool
	http               http.Handler
}
type Portal struct {
	cfg            config.Config
	root           *pgxpool.Pool
	mu             sync.RWMutex
	entries        map[string]*entry
	createMu       sync.Mutex
	credentialKeys *vault.Keyring
	// shared runs on every company's handler (principal included) before
	// its routes are registered: the shared-app parts are plugged in here.
	shared func(Company, *handler.Handler)
	// clock stops every company's schedule clock on Close.
	clock     context.Context
	stopClock context.CancelFunc
}

// Option adjusts a Portal before its companies are attached.
type Option func(*Portal)

// Company is what the shared-app parts need to know about one company.
type Company struct {
	Tenant  tenant.Tenant
	Name    string
	Pool    *pgxpool.Pool
	BaseURL string // the company's own Hub address (BASE_URL plus /w/<slug>)
}

// WithShared runs fn on every company's handler, the principal included,
// before its routes are registered. Each company gets its own containers,
// databases, folders and app addresses, named by its key (ADR 025).
func WithShared(fn func(Company, *handler.Handler)) Option {
	return func(p *Portal) { p.shared = fn }
}

func New(ctx context.Context, root *pgxpool.Pool, cfg config.Config, opts ...Option) (*Portal, error) {
	p := &Portal{cfg: cfg, root: root, entries: map[string]*entry{}}
	p.clock, p.stopClock = context.WithCancel(context.Background())
	for _, o := range opts {
		o(p)
	}
	// No generated fallback, SESSION_SECRET reuse or local key file. An unset
	// keyring leaves the vault locked; malformed configuration stops startup.
	if raw := os.Getenv("CREDENTIAL_VAULT_KEYRING"); raw != "" {
		var err error
		p.credentialKeys, err = vault.ParseKeyring(raw)
		if err != nil {
			return nil, vault.ErrEnvelope
		}
	}
	// Registry is intentionally outside tenant migration histories.
	_, err := root.Exec(ctx, `CREATE TABLE IF NOT EXISTS public.workspace_registry (
		slug TEXT PRIMARY KEY, name TEXT NOT NULL, schema_name TEXT UNIQUE NOT NULL,
		ready BOOLEAN NOT NULL DEFAULT FALSE, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	INSERT INTO public.workspace_registry(slug,name,schema_name,ready)
	VALUES('principal','Minha empresa','public',TRUE) ON CONFLICT DO NOTHING;
	ALTER TABLE public.workspace_registry ADD COLUMN IF NOT EXISTS shared_apps BOOLEAN;
	UPDATE public.workspace_registry SET shared_apps = (slug = 'teste') WHERE shared_apps IS NULL;
	ALTER TABLE public.workspace_registry ALTER COLUMN shared_apps SET DEFAULT FALSE;`)
	if err != nil {
		return nil, err
	}
	rows, err := root.Query(ctx, `SELECT slug,name,schema_name FROM public.workspace_registry WHERE ready=TRUE ORDER BY slug`)
	if err != nil {
		return nil, err
	}
	var entries []*entry
	for rows.Next() {
		e := &entry{}
		if err := rows.Scan(&e.slug, &e.name, &e.schema); err != nil {
			rows.Close()
			return nil, err
		}
		entries = append(entries, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.slug == "principal" && e.schema == "public" {
			e.pool = root
		} else {
			if !regexp.MustCompile(`^tenant_[a-f0-9]{24}$`).MatchString(e.schema) {
				p.Close()
				return nil, fmt.Errorf("invalid workspace namespace")
			}
			e.pool, err = p.newPool(ctx, e.schema)
			if err != nil {
				p.Close()
				return nil, err
			}
			if err = database.Migrate(ctx, e.pool); err != nil {
				e.pool.Close()
				p.Close()
				return nil, err
			}
		}
		p.attach(e)
		p.entries[e.slug] = e
	}
	return p, nil
}

func (p *Portal) newPool(ctx context.Context, schema string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(p.cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 8
	// Do not append public: a missing tenant table must fail, not fall back
	// to the original company's table. Never take this identifier from HTTP.
	cfg.ConnConfig.RuntimeParams["search_path"] = pgx.Identifier{schema}.Sanitize()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func (p *Portal) attach(e *entry) {
	cfg := p.cfg
	base := ""
	if e.slug != "principal" {
		base = "/w/" + e.slug
		cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/") + base
	}
	h := handler.New(sqlc.New(e.pool), cfg)
	h.SetCredentialKeys(p.credentialKeys)
	opts := handler.WorkspaceOptions{Slug: e.slug, Name: e.name, BasePath: base, Create: p.create, FindLogins: p.findLogins}
	slug := e.slug
	opts.SharedEnabled = func(ctx context.Context) bool { return p.SharedEnabled(ctx, slug) }
	if e.slug == "principal" {
		opts.Companies, opts.SetCompanyShared = p.Companies, p.SetCompanyShared
	}
	h.SetWorkspace(opts)
	key, keyErr := tenant.KeyOf(e.schema)
	e.key = key
	if p.shared != nil {
		if keyErr != nil {
			slog.Error("shared apps off: company has no key", "slug", e.slug, "err", keyErr)
		} else if p.keyTaken(e) {
			slog.Error("shared apps off: company key collides", "slug", e.slug, "key", e.key)
		} else {
			p.shared(Company{Tenant: tenant.Tenant{Slug: e.slug, Key: e.key}, Name: e.name, Pool: e.pool, BaseURL: cfg.BaseURL}, h)
		}
	}
	mux := http.NewServeMux()
	h.Register(mux)
	e.http = h.Secure(mux)
	h.StartScheduler(p.clock)
}

// SharedEnabled says whether the company's screen apps are on. The principal
// company is always on; any error answers off (fails closed).
func (p *Portal) SharedEnabled(ctx context.Context, slug string) bool {
	if slug == "principal" {
		return true
	}
	var on bool
	if err := p.root.QueryRow(ctx, `SELECT COALESCE(shared_apps,FALSE) FROM public.workspace_registry WHERE slug=$1 AND ready=TRUE`, slug).Scan(&on); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Warn("workspace shared apps", "slug", slug, "err", err)
		}
		return false
	}
	return on
}

// Companies lists every ready company except the principal, by name.
func (p *Portal) Companies(ctx context.Context) ([]handler.CompanyShared, error) {
	rows, err := p.root.Query(ctx, `SELECT slug,name,COALESCE(shared_apps,FALSE) FROM public.workspace_registry
		WHERE ready=TRUE AND slug<>'principal' ORDER BY name, slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []handler.CompanyShared{}
	for rows.Next() {
		var c handler.CompanyShared
		if err := rows.Scan(&c.Slug, &c.Name, &c.SharedApps); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetCompanyShared turns a company's screen apps on or off. The principal
// company and unknown slugs answer handler.ErrWorkspaceNotFound.
func (p *Portal) SetCompanyShared(ctx context.Context, slug string, on bool) error {
	if slug == "principal" {
		return handler.ErrWorkspaceNotFound
	}
	tag, err := p.root.Exec(ctx, `UPDATE public.workspace_registry SET shared_apps=$2 WHERE slug=$1 AND ready=TRUE`, slug, on)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return handler.ErrWorkspaceNotFound
	}
	return nil
}

// keyTaken reports whether another attached company already has e's key
// (the key is the first 11 hex digits of the schema; a clash would mix two
// companies' containers, so the second one gets no shared apps).
func (p *Portal) keyTaken(e *entry) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for slug, other := range p.entries {
		if slug != e.slug && other.key == e.key {
			return true
		}
	}
	return false
}

// Load attaches a company created by another process (the app-address
// router calls it for a company it does not know yet). Unknown or invalid
// slugs answer false.
func (p *Portal) Load(ctx context.Context, slug string) bool { return p.load(ctx, slug) != nil }

func (p *Portal) create(ctx context.Context, name, slug string, me sqlc.Person) (result string, err error) {
	p.createMu.Lock()
	defer p.createMu.Unlock()
	// Serialize across portal processes as well as concurrent HTTP requests.
	conn, err := p.root.Acquire(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Release()
	_, err = conn.Exec(ctx, `SELECT pg_advisory_lock(731940216)`)
	if err != nil {
		return "", err
	}
	defer conn.Exec(context.Background(), `SELECT pg_advisory_unlock(731940216)`)
	var count int
	if err = conn.QueryRow(ctx, `SELECT count(*) FROM public.workspace_registry`).Scan(&count); err != nil {
		return "", err
	}
	limit := p.cfg.CompanyLimit
	if limit <= 0 {
		limit = 20
	}
	if count >= limit {
		return "", handler.ErrWorkspaceCapacity
	}
	// A daily budget keeps a burst of fake signups from using up the portal.
	perDay := p.cfg.CompanySignupsPerDay
	if perDay <= 0 {
		perDay = 10
	}
	var today int
	if err = conn.QueryRow(ctx, `SELECT count(*) FROM public.workspace_registry WHERE slug<>'principal' AND created_at > now() - interval '24 hours'`).Scan(&today); err != nil {
		return "", err
	}
	if today >= perDay {
		return "", handler.ErrWorkspaceCapacity
	}
	// The company key is the schema's first 11 hex digits: draw again (at
	// most 5 times) while another company already starts the same way.
	var schema string
	for try := 0; ; try++ {
		buf := make([]byte, 12)
		if _, err = rand.Read(buf); err != nil {
			return "", err
		}
		schema = "tenant_" + hex.EncodeToString(buf)
		var taken bool
		if err = conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.workspace_registry WHERE left(schema_name,18)=left($1,18))`, schema).Scan(&taken); err != nil {
			return "", err
		}
		if !taken {
			break
		}
		if try == 4 {
			return "", errors.New("could not draw a unique company key")
		}
	}
	e := &entry{slug: slug, name: name, schema: schema}
	_, err = conn.Exec(ctx, `INSERT INTO public.workspace_registry(slug,name,schema_name) VALUES($1,$2,$3)`, slug, name, e.schema)
	if err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" {
			return "", handler.ErrWorkspaceExists
		}
		return "", err
	}
	success := false
	defer func() {
		if !success {
			if e.pool != nil {
				e.pool.Close()
			}
			// Only this newly generated, unpublished namespace is rolled back.
			_, _ = conn.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+pgx.Identifier{e.schema}.Sanitize()+" CASCADE")
			_, _ = conn.Exec(context.Background(), `DELETE FROM public.workspace_registry WHERE slug=$1 AND ready=FALSE`, slug)
		}
	}()
	if _, err = conn.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{e.schema}.Sanitize()); err != nil {
		return "", err
	}
	if e.pool, err = p.newPool(ctx, e.schema); err != nil {
		return "", err
	}
	if err = database.Migrate(ctx, e.pool); err != nil {
		return "", err
	}
	q := sqlc.New(e.pool)
	// Migration 010 inserted the row with approval on; new companies start off.
	if err = q.DisableAppApprovalByDefault(ctx); err != nil {
		return "", err
	}
	role, err := q.GetRoleByKey(ctx, pgtype.Text{String: "admin", Valid: true})
	if err != nil {
		return "", err
	}
	// Accounts are workspace-scoped. Initial credentials match the creator;
	// future password lifecycle is independent, not platform-wide SSO.
	if _, err = q.CreatePerson(ctx, sqlc.CreatePersonParams{Name: me.Name, Email: me.Email, PasswordHash: me.PasswordHash, RoleID: role.ID}); err != nil {
		return "", err
	}
	if err = q.SetCompanyName(ctx, name); err != nil {
		return "", err
	}
	if _, err = conn.Exec(ctx, `UPDATE public.workspace_registry SET ready=TRUE WHERE slug=$1`, slug); err != nil {
		return "", err
	}
	p.attach(e)
	p.mu.Lock()
	p.entries[slug] = e
	p.mu.Unlock()
	success = true
	return "/w/" + slug + "/", nil
}

var (
	dummyHashOnce sync.Once
	dummyHash     []byte
)

// equalizeTiming spends one cost-12 bcrypt comparison so an e-mail that exists
// nowhere answers in about the same time as one that does.
func equalizeTiming(password string) {
	dummyHashOnce.Do(func() {
		dummyHash, _ = bcrypt.GenerateFromPassword([]byte("ziv-no-such-account"), 12)
	})
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
}

// findLogins lists the companies, other than the main one, where email and
// password sign in, sorted by name. A company that fails to answer is logged
// and skipped; if every company with this e-mail failed, it returns the error
// rather than a misleading "no match".
func (p *Portal) findLogins(ctx context.Context, email, password string) ([]handler.LoginMatch, error) {
	// Companies created by another server process are attached first.
	rows, err := p.root.Query(ctx, `SELECT slug FROM public.workspace_registry WHERE ready=TRUE AND slug<>'principal' ORDER BY slug`)
	if err != nil {
		return nil, err
	}
	slugs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	var out []handler.LoginMatch
	var failed error
	compared := false
	for _, slug := range slugs {
		p.mu.RLock()
		e := p.entries[slug]
		p.mu.RUnlock()
		if e == nil {
			if e = p.load(ctx, slug); e == nil {
				slog.Warn("find logins: company not loaded", "slug", slug)
				failed = errors.New("company not loaded: " + slug)
				continue
			}
		}
		person, err := sqlc.New(e.pool).GetPersonByEmail(ctx, email)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			slog.Warn("find logins: lookup failed", "slug", slug, "err", err)
			failed = err
			continue
		}
		if person.PasswordHash == "" {
			continue
		}
		compared = true
		if bcrypt.CompareHashAndPassword([]byte(person.PasswordHash), []byte(password)) == nil {
			out = append(out, handler.LoginMatch{Slug: e.slug, Name: e.name, URL: "/w/" + e.slug + "/"})
		}
	}
	if !compared {
		equalizeTiming(password)
	}
	if len(out) == 0 && failed != nil {
		return nil, failed
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Slug < out[j].Slug
	})
	return out, nil
}

var registrySlug = regexp.MustCompile(`^[a-z][a-z0-9-]{2,39}$`)

// load attaches a company another server process created after this one
// started (Replit may run several copies). Unknown slugs stay a 404.
func (p *Portal) load(ctx context.Context, slug string) *entry {
	if !registrySlug.MatchString(slug) {
		return nil
	}
	p.createMu.Lock()
	defer p.createMu.Unlock()
	p.mu.RLock()
	e := p.entries[slug]
	p.mu.RUnlock()
	if e != nil {
		return e
	}
	e = &entry{slug: slug}
	err := p.root.QueryRow(ctx, `SELECT name,schema_name FROM public.workspace_registry WHERE slug=$1 AND ready=TRUE`, slug).Scan(&e.name, &e.schema)
	if err != nil || !regexp.MustCompile(`^tenant_[a-f0-9]{24}$`).MatchString(e.schema) {
		return nil
	}
	if e.pool, err = p.newPool(ctx, e.schema); err != nil {
		return nil
	}
	if err = database.Migrate(ctx, e.pool); err != nil {
		e.pool.Close()
		return nil
	}
	p.attach(e)
	p.mu.Lock()
	p.entries[slug] = e
	p.mu.Unlock()
	return e
}

func (p *Portal) Close() {
	p.stopClock()
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, e := range p.entries {
		if e.pool != p.root {
			e.pool.Close()
		}
	}
}

// splitWorkspacePath picks the company and the path inside it. Besides
// /w/<slug>/..., OAuth clients look up metadata at the root with the company
// path appended (RFC 9728/8414): /.well-known/<name>/w/<slug>/<tail>.
func splitWorkspacePath(p string) (slug, rest string) {
	for _, name := range []string{"oauth-protected-resource", "oauth-authorization-server"} {
		prefix := "/.well-known/" + name + "/w/"
		if after, ok := strings.CutPrefix(p, prefix); ok {
			slug, tail, found := strings.Cut(after, "/")
			if found {
				tail = "/" + tail
			}
			return slug, "/.well-known/" + name + tail
		}
	}
	if after, ok := strings.CutPrefix(p, "/w/"); ok {
		slug, tail, _ := strings.Cut(after, "/")
		return slug, "/" + tail
	}
	return "principal", p
}

func (p *Portal) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler.NoFraming(w.Header())
	slug, rest := splitWorkspacePath(r.URL.Path)
	p.mu.RLock()
	e := p.entries[slug]
	p.mu.RUnlock()
	if e == nil {
		e = p.load(r.Context(), slug)
	}
	if e == nil {
		http.NotFound(w, r)
		return
	}
	if strings.HasPrefix(rest, "/api/") || rest == "/up" || rest == "/mcp" ||
		strings.HasPrefix(rest, "/oauth/") || strings.HasPrefix(rest, "/.well-known/") {
		clone := r.Clone(r.Context())
		u := *r.URL
		u.Path = rest
		u.RawPath = ""
		clone.URL = &u
		// Route patterns for audit are written onto clone by ServeMux.
		e.http.ServeHTTP(w, clone)
		return
	}
	if strings.HasPrefix(rest, "/auth/") {
		http.NotFound(w, r)
		return
	}
	// SPA assets stay root-based. Never serve a filesystem path supplied
	// outside frontend/dist, including ../ or absolute path aliases.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	clean := path.Clean("/" + r.URL.Path)
	if strings.HasPrefix(clean, "/assets/") {
		http.ServeFile(w, r, filepath.Join("frontend/dist", filepath.FromSlash(strings.TrimPrefix(clean, "/"))))
		return
	}
	if clean == "/favicon.ico" || clean == "/vite.svg" {
		if _, err := os.Stat(filepath.Join("frontend/dist", strings.TrimPrefix(clean, "/"))); err == nil {
			http.ServeFile(w, r, filepath.Join("frontend/dist", strings.TrimPrefix(clean, "/")))
			return
		}
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, "frontend/dist/index.html")
}
