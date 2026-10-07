package gateway

import (
	_ "embed"
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
)

//go:embed page.html
var pageHTML string

var pageTmpl = template.Must(template.New("p").Parse(pageHTML))

type page struct {
	App, Title, Message, HubURL string
	Refresh                     bool
}

func wantsHTML(r *http.Request) bool {
	return r.Method == http.MethodGet && strings.Contains(r.Header.Get("Accept"), "text/html")
}

// show writes a gateway page, or a short JSON error for API calls.
func (g *Gateway) show(w http.ResponseWriter, r *http.Request, status int, p page) {
	if status == http.StatusServiceUnavailable {
		w.Header().Set("Retry-After", "5")
	}
	w.Header().Set("Cache-Control", "no-store")
	if !wantsHTML(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"error":` + jsonString(p.Title+". "+p.Message) + `}`))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = pageTmpl.Execute(w, p)
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func owner(t Target) string {
	if t.Owner == "" {
		return "quem cuida do app"
	}
	return t.Owner
}
