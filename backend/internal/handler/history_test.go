package handler

import (
	"net/http"
	"testing"
)

func TestHistoryMixesAndPages(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c) // versão 1 + "Ficha criada"
	editFile(t, dir, "src/main.py", "print('v2')\n")
	c.do("POST", appPath(id, "/check"), nil, nil) // versão 2

	var hist historyJSON
	if code := c.do("GET", appPath(id, "/history"), nil, &hist); code != http.StatusOK {
		t.Fatalf("history %d", code)
	}
	if len(hist.Items) != 3 || hist.Items[0].Type != "version" || hist.Items[0].Version.Number != 2 || !hist.Items[0].Version.Current ||
		hist.Items[0].Version.AuthorName != "Henry" || !hist.Items[0].Version.AuthorProbable || hist.More {
		t.Fatalf("%+v", hist.Items)
	}
	var code_ historyJSON
	c.do("GET", appPath(id, "/history?type=code"), nil, &code_)
	if len(code_.Items) != 2 {
		t.Fatalf("code filter %+v", code_.Items)
	}
	var ficha historyJSON
	c.do("GET", appPath(id, "/history?type=ficha"), nil, &ficha)
	if len(ficha.Items) != 1 || ficha.Items[0].Ficha.Changes[0].Label != "Ficha criada" {
		t.Fatalf("ficha filter %+v", ficha.Items)
	}

	h.historyPage = 2
	var p1, p2 historyJSON
	c.do("GET", appPath(id, "/history"), nil, &p1)
	if len(p1.Items) != 2 || !p1.More || p1.NextBefore == "" {
		t.Fatalf("page1 %+v", p1)
	}
	c.do("GET", appPath(id, "/history?before="+p1.NextBefore), nil, &p2)
	if len(p2.Items) != 1 || p2.More || p2.Items[0].Type != "ficha" {
		t.Fatalf("page2 %+v", p2)
	}
}

func TestHistoryRunVisibility(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	pairComputer(t, c, "PC")
	c.do("POST", appPath(app, "/runs"), map[string]string{}, nil)
	createPersonAs(t, c, "Bia", "bia@alis.com", "usuario")
	bia := c.fresh()
	loginAs(t, bia, "bia@alis.com")
	var hist historyJSON
	bia.do("GET", appPath(app, "/history?type=runs"), nil, &hist)
	if len(hist.Items) != 0 {
		t.Fatalf("usuario sees others' runs: %+v", hist.Items)
	}
	var mine historyJSON
	c.do("GET", appPath(app, "/history?type=runs"), nil, &mine)
	if len(mine.Items) != 1 || mine.Items[0].Run.PersonName != "Henry" {
		t.Fatalf("%+v", mine.Items)
	}
}

func TestHistoryBadParams(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	if code := c.do("GET", appPath(app, "/history?before=ontem"), nil, nil); code != http.StatusBadRequest {
		t.Fatalf("bad before %d", code)
	}
	if code := c.do("GET", appPath(app, "/history?type=x"), nil, nil); code != http.StatusBadRequest {
		t.Fatalf("bad type %d", code)
	}
	if code := c.do("GET", "/api/apps/999/history", nil, nil); code != http.StatusNotFound {
		t.Fatalf("missing app %d", code)
	}
}
