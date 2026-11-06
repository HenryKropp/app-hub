package handler

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// envioZip is a small zip with one main.py.
func envioZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("main.py")
	w.Write([]byte("print('oi')\n"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// securedServer serves the handler's routes behind Secure, as production does.
func securedServer(t *testing.T, h *Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h.Secure(h.api))
	t.Cleanup(srv.Close)
	return srv
}

// putEnvio PUTs body to the upload address (its path, on srv) as curl does.
func putEnvio(t *testing.T, srv *httptest.Server, endereco string, body []byte) (int, map[string]any) {
	t.Helper()
	path := strings.TrimPrefix(endereco, "http://hub.test")
	req, _ := http.NewRequest("PUT", srv.URL+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/zip")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return res.StatusCode, out
}

func prepararEnvio(t *testing.T, c *client) (int64, string, map[string]any) {
	t.Helper()
	s := toolSession(t, c)
	res, text, out := callTool(t, s, "preparar_envio", map[string]any{})
	if res.IsError {
		t.Fatalf("preparar_envio: %s", text)
	}
	return int64(out["envio"].(float64)), out["endereco"].(string), out
}

func TestEnvioRoundTrip(t *testing.T) {
	c, h := newPoolClient(t)
	p := setupAdmin(t, c)
	id, endereco, _ := prepararEnvio(t, c)
	if !strings.HasPrefix(endereco, "http://hub.test/api/mcp/envios/") {
		t.Fatalf("endereco %q", endereco)
	}
	srv := securedServer(t, h)
	if code, out := putEnvio(t, srv, endereco, envioZip(t)); code != http.StatusOK || out["recebido"] != true || out["arquivos"] != float64(1) {
		t.Fatalf("put = %d %+v", code, out)
	}
	if code, _ := putEnvio(t, srv, endereco, envioZip(t)); code != http.StatusNotFound {
		t.Fatalf("second put = %d", code)
	}
	var person int64
	var received bool
	if err := testPool.QueryRow(context.Background(), `SELECT person_id, received_at IS NOT NULL FROM mcp_uploads WHERE id = $1`, id).Scan(&person, &received); err != nil {
		t.Fatal(err)
	}
	if person != p.ID || !received {
		t.Fatalf("row person %d received %v", person, received)
	}
}

func TestEnvioExpired(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	id, endereco, _ := prepararEnvio(t, c)
	if _, err := testPool.Exec(context.Background(), `UPDATE mcp_uploads SET expires_at = now() - interval '1 second' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	code, out := putEnvio(t, securedServer(t, h), endereco, envioZip(t))
	if code != http.StatusNotFound || !strings.Contains(out["error"].(string), "preparar_envio") {
		t.Fatalf("put = %d %+v", code, out)
	}
}

func TestEnvioRejectsBadZip(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	id, endereco, _ := prepararEnvio(t, c)
	srv := securedServer(t, h)
	if code, out := putEnvio(t, srv, endereco, []byte("hello")); code != http.StatusBadRequest || out["error"] != "O envio não é um arquivo .zip válido." {
		t.Fatalf("put = %d %+v", code, out)
	}
	var received bool
	if err := testPool.QueryRow(context.Background(), `SELECT received_at IS NOT NULL FROM mcp_uploads WHERE id = $1`, id).Scan(&received); err != nil || received {
		t.Fatalf("received %v err %v", received, err)
	}
	if code, _ := putEnvio(t, srv, endereco, envioZip(t)); code != http.StatusOK {
		t.Fatalf("good zip after bad = %d", code)
	}
}

func TestEnvioTooBig(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	_, endereco, _ := prepararEnvio(t, c)
	code, out := putEnvio(t, securedServer(t, h), endereco, make([]byte, 12<<20+1))
	if code != http.StatusRequestEntityTooLarge || out["error"] != "O pacote passa de 12 MB. Deixe de fora dados e saídas acumuladas." {
		t.Fatalf("put = %d %+v", code, out)
	}
}

func TestEnvioOpenLimit(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	s := toolSession(t, c)
	for i := 0; i < 5; i++ {
		if res, text, _ := callTool(t, s, "preparar_envio", map[string]any{}); res.IsError {
			t.Fatalf("envio %d: %s", i+1, text)
		}
	}
	res, text, _ := callTool(t, s, "preparar_envio", map[string]any{})
	if !res.IsError || !strings.Contains(text, "5 envios abertos") {
		t.Fatalf("sixth = %v %q", res.IsError, text)
	}
}

func TestEnvioCommandsUseTemp(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	_, endereco, out := prepararEnvio(t, c)
	win := out["comando_windows"].(string)
	if !strings.Contains(win, `"%TEMP%\ziv-envio-`) || !strings.Contains(win, `-C "<PASTA>" .`) || !strings.Contains(win, endereco) {
		t.Fatalf("comando_windows %q", win)
	}
	if strings.Contains(win, `<PASTA>\`) || strings.Contains(win, `<PASTA>/`) {
		t.Fatalf("comando_windows writes into the app folder: %q", win)
	}
	for _, ex := range []string{"--exclude=.git", "--exclude=.env", `--exclude=".env.*"`, `--exclude="*.pem"`, "--exclude=node_modules"} {
		if !strings.Contains(win, ex) {
			t.Fatalf("comando_windows misses %s", ex)
		}
	}
	unix := out["comando_mac_linux"].(string)
	if !strings.Contains(unix, `cd "<PASTA>"`) || !strings.Contains(unix, "TMPDIR") || !strings.Contains(unix, endereco) || !strings.Contains(unix, "'.env'") {
		t.Fatalf("comando_mac_linux %q", unix)
	}
	if v, _ := out["vale_ate"].(string); func() error { _, err := time.Parse(time.RFC3339, v); return err }() != nil {
		t.Fatalf("vale_ate %v", out["vale_ate"])
	}
}

// A made-up address is refused before the body is read or unpacked.
func TestEnvioUnknownTokenNotRead(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	if code, out := putEnvio(t, securedServer(t, h), "http://hub.test/api/mcp/envios/inventado", []byte("isto não é um zip")); code != http.StatusNotFound || out["error"] != msgEnvioGone {
		t.Fatalf("%d %+v", code, out)
	}
}

// Only the kept files are stored: a .env inside the zip never reaches the database.
func TestEnvioStoresOnlyKept(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	id, endereco, _ := prepararEnvio(t, c)
	body := zipOf(t, map[string]string{"main.py": "print(1)\n", ".env": "SENHA=valor-zivteste-987\n"})
	if code, out := putEnvio(t, securedServer(t, h), endereco, body); code != http.StatusOK {
		t.Fatalf("%d %+v", code, out)
	}
	var content []byte
	testPool.QueryRow(context.Background(), `SELECT content FROM mcp_uploads WHERE id = $1`, id).Scan(&content)
	if len(content) == 0 || bytes.Contains(content, []byte("valor-zivteste-987")) {
		t.Fatalf("stored %d bytes, secret kept", len(content))
	}
}
