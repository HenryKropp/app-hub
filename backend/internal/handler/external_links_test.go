package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// fakeMail keeps the e-mails it was asked to send.
type fakeMail struct{ sent []fakeMessage }
type fakeMessage struct{ to, subject, text string }

func (f *fakeMail) Send(_ context.Context, to, subject, text string) error {
	f.sent = append(f.sent, fakeMessage{to, subject, text})
	return nil
}

// linksTeam: admin Henry, criador Ana (with share_external), usuário Bia; one
// HTML artifact made by Ana in Research.
func linksTeam(t *testing.T) (admin, ana, bia *client, h *Handler, artifactID int64) {
	t.Helper()
	admin, h = newPoolClient(t)
	h.cfg.SharedDomain = "ziv.test"
	setupAdmin(t, admin)
	createPersonAs(t, admin, "Ana", "ana@exemplo.com", "criador")
	createPersonAs(t, admin, "Bia", "bia@exemplo.com", "usuario")
	ana, bia = admin.fresh(), admin.fresh()
	loginAs(t, ana, "ana@exemplo.com")
	loginAs(t, bia, "bia@exemplo.com")
	setRole(t, "criador", "register_apps", "share_external")
	d := createArtifact(t, ana, fileBody("Relatório mensal", "rel.html", []byte(pageHTML)))
	return admin, ana, bia, h, d.ID
}

func linksPath(id int64) string { return fmt.Sprintf("/api/artifacts/%d/links", id) }

func openLinkBody() map[string]any {
	return map[string]any{"kind": "open", "content": "latest", "expires_days": 30}
}

func TestCreateLinkNeedsShareExternal(t *testing.T) {
	_, ana, _, _, id := linksTeam(t)
	setRole(t, "criador", "register_apps")
	var e struct{ Error string }
	if code := ana.do("POST", linksPath(id), openLinkBody(), &e); code != http.StatusForbidden || e.Error != "Você não tem permissão para compartilhar fora da empresa." {
		t.Fatalf("status %d %q", code, e.Error)
	}
}

func TestCreateLinkNeedsToSeeArtifact(t *testing.T) {
	_, _, bia, _, id := linksTeam(t)
	setRole(t, "usuario", "share_external")
	exec(t, "UPDATE artifacts SET restricted = true WHERE id = $1", id)
	if code := bia.do("POST", linksPath(id), openLinkBody(), nil); code != http.StatusNotFound {
		t.Fatalf("hidden artifact = %d", code)
	}
}

func TestCreateLinkRejectsLinkArtifact(t *testing.T) {
	_, ana, _, _, _ := linksTeam(t)
	d := createArtifact(t, ana, linkBody("Painel", "https://exemplo.com/painel"))
	if code := ana.do("POST", linksPath(d.ID), openLinkBody(), nil); code != http.StatusBadRequest {
		t.Fatalf("link artifact = %d", code)
	}
}

func TestCreateEmailLinkWithoutMailIs400(t *testing.T) {
	_, ana, _, _, id := linksTeam(t)
	var e struct{ Error string }
	body := map[string]any{"kind": "email", "email": "cliente@fora.com", "content": "latest", "expires_days": 30}
	if code := ana.do("POST", linksPath(id), body, &e); code != http.StatusBadRequest || e.Error != "O envio de e-mail ainda não está ligado neste Ziv." {
		t.Fatalf("status %d %q", code, e.Error)
	}
}

func TestCreateEmailLinkNeedsValidEmail(t *testing.T) {
	_, ana, _, h, id := linksTeam(t)
	h.mail = &fakeMail{}
	body := map[string]any{"kind": "email", "email": "nao-e-email", "content": "latest", "expires_days": 30}
	if code := ana.do("POST", linksPath(id), body, nil); code != http.StatusBadRequest {
		t.Fatalf("bad e-mail = %d", code)
	}
}

func TestCreateLinkBadExpiry(t *testing.T) {
	_, ana, _, _, id := linksTeam(t)
	for _, days := range []int{0, 366} {
		body := openLinkBody()
		body["expires_days"] = days
		if code := ana.do("POST", linksPath(id), body, nil); code != http.StatusBadRequest {
			t.Fatalf("expires_days %d = %d", days, code)
		}
	}
}

func TestCreateAndListLink(t *testing.T) {
	admin, ana, _, h, id := linksTeam(t)
	h.mail = &fakeMail{}
	var l linkJSON
	if code := ana.do("POST", linksPath(id), openLinkBody(), &l); code != http.StatusCreated {
		t.Fatalf("create = %d", code)
	}
	var token string
	if err := testPool.QueryRow(context.Background(), "SELECT token FROM external_links WHERE id=$1", l.ID).Scan(&token); err != nil {
		t.Fatal(err)
	}
	if l.URL != "http://link.ziv.test/principal/"+token || l.Status != "ativo" || l.Kind != "open" || l.Content != "latest" || l.CreatedByName != "Ana" {
		t.Fatalf("link %+v", l)
	}
	pin := map[string]any{"kind": "email", "email": "Cliente@Fora.com", "content": "version", "version": 1, "expires_days": 7}
	var p linkJSON
	if code := ana.do("POST", linksPath(id), pin, &p); code != http.StatusCreated || p.Recipient != "cliente@fora.com" || p.VersionNumber != 1 {
		t.Fatalf("pinned = %d %+v", code, p)
	}
	var list artifactLinksJSON
	ana.do("GET", linksPath(id), nil, &list)
	if !list.CanShare || !list.EmailAvailable || len(list.Links) != 2 {
		t.Fatalf("artifact list %+v", list)
	}
	var all companyLinksJSON
	if code := admin.do("GET", "/api/links", nil, &all); code != http.StatusOK || len(all.Links) != 2 || all.Links[0].ArtifactName != "Relatório mensal" {
		t.Fatalf("company list %d %+v", code, all)
	}
}

func TestCreateLinkPinsOnlyOwnVersion(t *testing.T) {
	_, ana, _, _, id := linksTeam(t)
	body := openLinkBody()
	body["content"], body["version"] = "version", 9
	if code := ana.do("POST", linksPath(id), body, nil); code != http.StatusBadRequest {
		t.Fatalf("missing version = %d", code)
	}
}

func TestCancelLink(t *testing.T) {
	_, ana, _, _, id := linksTeam(t)
	var l linkJSON
	ana.do("POST", linksPath(id), openLinkBody(), &l)
	if code := ana.do("POST", fmt.Sprintf("/api/links/%d/cancel", l.ID), nil, nil); code != http.StatusNoContent {
		t.Fatalf("cancel = %d", code)
	}
	var list artifactLinksJSON
	ana.do("GET", linksPath(id), nil, &list)
	if len(list.Links) != 1 || list.Links[0].Status != "cancelado" {
		t.Fatalf("after cancel %+v", list.Links)
	}
}

func TestCancelLinkNeedsShareExternal(t *testing.T) {
	_, ana, bia, _, id := linksTeam(t)
	var l linkJSON
	ana.do("POST", linksPath(id), openLinkBody(), &l)
	if code := bia.do("POST", fmt.Sprintf("/api/links/%d/cancel", l.ID), nil, nil); code != http.StatusForbidden {
		t.Fatalf("bia cancel = %d", code)
	}
}

func TestLinkClosedWhenCreatorLosesShare(t *testing.T) {
	admin, ana, _, _, id := linksTeam(t)
	ana.do("POST", linksPath(id), openLinkBody(), nil)
	setRole(t, "criador", "register_apps")
	var all companyLinksJSON
	admin.do("GET", "/api/links", nil, &all)
	if len(all.Links) != 1 || all.Links[0].Status != "fechado" {
		t.Fatalf("status %+v", all.Links)
	}
}

func TestCompanyLinksOnlyForAdmin(t *testing.T) {
	_, ana, _, _, _ := linksTeam(t)
	if code := ana.do("GET", "/api/links", nil, nil); code != http.StatusForbidden {
		t.Fatalf("ana company links = %d", code)
	}
}

func TestArtifactLinksListHiddenWithoutShare(t *testing.T) {
	_, _, bia, _, id := linksTeam(t)
	var list artifactLinksJSON
	if code := bia.do("GET", linksPath(id), nil, &list); code != http.StatusOK || list.CanShare || len(list.Links) != 0 {
		t.Fatalf("bia list %d %+v", code, list)
	}
	_ = strings.TrimSpace
}
