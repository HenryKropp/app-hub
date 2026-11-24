package workspace

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
)

// linkIn puts an open link to an HTML artifact into company slug and returns its token.
func linkIn(t *testing.T, p *Portal, slug string) string {
	t.Helper()
	ctx := context.Background()
	e := p.load(ctx, slug)
	if e == nil {
		t.Fatalf("company %s not loaded", slug)
	}
	steps := []string{
		`INSERT INTO areas (name) SELECT 'Geral' WHERE NOT EXISTS (SELECT 1 FROM areas)`,
		`UPDATE roles SET permissions = array_append(permissions, 'share_external') WHERE key = 'admin' AND NOT ('share_external' = ANY(permissions))`,
		`INSERT INTO code_blobs (sha256, content, size) VALUES ('abc', '<p>oi</p>', 9) ON CONFLICT DO NOTHING`,
		`INSERT INTO artifacts (name, kind, area_id, owner_id, created_by) SELECT 'Relatório', 'file', (SELECT min(id) FROM areas), min(id), min(id) FROM people`,
		`INSERT INTO artifact_versions (artifact_id, number, kind, blob_sha256, file_name) SELECT max(id), 1, 'file', 'abc', 'r.html' FROM artifacts`,
		`INSERT INTO external_links (token, artifact_id, kind, created_by, expires_at) SELECT 'tok-` + slug + `', max(a.id), 'open', (SELECT min(id) FROM people), now() + interval '1 day' FROM artifacts a`,
	}
	for _, s := range steps {
		if _, err := e.pool.Exec(ctx, s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	return "tok-" + slug
}

func getLink(t *testing.T, srv *httptest.Server, path string) (int, string) {
	t.Helper()
	resp, err := http.Get(srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestServeLinkStaysInItsCompany(t *testing.T) {
	p, _, _ := testPortal(t)
	ctx := context.Background()
	for _, c := range []struct{ name, slug, email string }{{"Horizonte", "horizonte", "ana@h.com"}, {"Aurora", "aurora", "bia@a.com"}} {
		if _, err := p.create(ctx, c.name, c.slug, sqlc.Person{Name: "X", Email: c.email, PasswordHash: "x"}, false); err != nil {
			t.Fatal(err)
		}
	}
	token := linkIn(t, p, "horizonte")
	srv := httptest.NewServer(http.HandlerFunc(p.ServeLink))
	defer srv.Close()
	if code, body := getLink(t, srv, "/horizonte/"+token+"/conteudo"); code != 200 || body != "<p>oi</p>" {
		t.Fatalf("own company %d %q", code, body)
	}
	if code, body := getLink(t, srv, "/horizonte/"+token); code != 200 || !strings.Contains(body, "Horizonte compartilhou com você") {
		t.Fatalf("page %d %s", code, body)
	}
	for _, path := range []string{"/aurora/" + token, "/aurora/" + token + "/conteudo", "/nao-existe/" + token, "/horizonte", "/"} {
		if code, body := getLink(t, srv, path); code != 404 || !strings.Contains(body, "Este link não está mais disponível") || !strings.Contains(body, `class="logo" href="http://localhost/"`) {
			t.Fatalf("%s = %d %s", path, code, body)
		}
	}
}
