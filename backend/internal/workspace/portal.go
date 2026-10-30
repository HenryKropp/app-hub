// Package workspace routes explicit workspace paths to independent database
// namespaces and independent handler state. The URL is a selector, not identity.
package workspace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/HenryKropp/app-hub/backend/internal/config"
	"github.com/HenryKropp/app-hub/backend/internal/database"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/handler"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type entry struct {
	name, slug, schema string
	pool               *pgxpool.Pool
	http               http.Handler
}
type Portal struct {
	cfg      config.Config
	root     *pgxpool.Pool
	mu       sync.RWMutex
	entries  map[string]*entry
	createMu sync.Mutex
}

func New(ctx context.Context, root *pgxpool.Pool, cfg config.Config) (*Portal, error) {
	p := &Portal{cfg: cfg, root: root, entries: map[string]*entry{}}
	// Registry is intentionally outside tenant migration histories.
	_, err := root.Exec(ctx, `CREATE TABLE IF NOT EXISTS public.workspace_registry (
		slug TEXT PRIMARY KEY, name TEXT NOT NULL, schema_name TEXT UNIQUE NOT NULL,
		ready BOOLEAN NOT NULL DEFAULT FALSE, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	INSERT INTO public.workspace_registry(slug,name,schema_name,ready)
	VALUES('principal','Minha empresa','public',TRUE) ON CONFLICT DO NOTHING;`)
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
	cfg.MaxConns = 4
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
	h.SetWorkspace(handler.WorkspaceOptions{Slug: e.slug, Name: e.name, BasePath: base, Create: p.create})
	mux := http.NewServeMux()
	h.Register(mux)
	e.http = h.Secure(mux)
}

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
	if count >= 20 {
return "", handler.ErrWorkspaceCapacity
	}
	buf := make([]byte, 12)
	if _, err = rand.Read(buf); err != nil {
		return "", err
	}
	e := &entry{slug: slug, name: name, schema: "tenant_" + hex.EncodeToString(buf)}
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

func (p *Portal) Close() {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, e := range p.entries {
		if e.pool != p.root {
			e.pool.Close()
		}
	}
}

func (p *Portal) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug := "principal"
	rest := r.URL.Path
	if strings.HasPrefix(rest, "/w/") {
		parts := strings.SplitN(strings.TrimPrefix(rest, "/w/"), "/", 2)
		slug = parts[0]
		rest = "/"
		if len(parts) == 2 {
			rest += parts[1]
		}
	}
	p.mu.RLock()
	e := p.entries[slug]
	p.mu.RUnlock()
	if e == nil {
		http.NotFound(w, r)
		return
	}
	if strings.HasPrefix(rest, "/api/") || rest == "/up" {
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
