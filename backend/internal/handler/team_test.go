package handler

import (
	"net/http"
	"strconv"
	"testing"
)

type roleOut struct {
	ID          int64    `json:"id"`
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
	PeopleCount int64    `json:"people_count"`
}

type areaOut struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	AppCount int64  `json:"app_count"`
	Leaders  []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"leaders"`
	Members []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"members"`
}

func TestRolesCRUD(t *testing.T) {
	admin, ana, _, _ := team(t)
	var r roleOut
	if code := admin.do("POST", "/api/roles", map[string]any{"name": "Sub-admin", "permissions": []string{"see_all_runs", "edit_any_app"}}, &r); code != http.StatusCreated {
		t.Fatalf("create %d", code)
	}
	if code := admin.do("POST", "/api/roles", map[string]any{"name": "X", "permissions": []string{"voar"}}, nil); code != http.StatusBadRequest {
		t.Fatalf("unknown permission = %d", code)
	}
	if code := ana.do("POST", "/api/roles", map[string]any{"name": "Y"}, nil); code != http.StatusForbidden {
		t.Fatalf("criador creating role = %d", code)
	}
	if code := admin.do("PUT", "/api/people/2", map[string]any{"role_id": r.ID}, nil); code != http.StatusOK {
		t.Fatalf("give role %d", code)
	}
	if code := admin.do("DELETE", "/api/roles/"+itoa(r.ID), nil, nil); code != http.StatusConflict {
		t.Fatalf("delete role in use = %d", code)
	}
	if code := admin.do("DELETE", "/api/roles/2", nil, nil); code != http.StatusConflict {
		t.Fatalf("delete built-in = %d", code)
	}
	var st struct {
		Person personJSON `json:"person"`
	}
	ana.do("GET", "/api/auth/status", nil, &st)
	if st.Person.RoleName != "Sub-admin" || !st.Person.SeesOthersRuns {
		t.Fatalf("ana after new role: %+v", st.Person)
	}
}

func TestLastManagerProtected(t *testing.T) {
	admin, _, _, _ := team(t)
	const msg = "A empresa precisa de ao menos uma pessoa que cuide das pessoas e dos cargos."
	var out map[string]any
	if code := admin.do("PUT", "/api/people/1", map[string]any{"role_id": 3}, &out); code != http.StatusConflict || out["error"] != msg {
		t.Fatalf("demote last admin %d %v", code, out)
	}
	if code := admin.do("PUT", "/api/roles/1", map[string]any{"name": "Administrador", "permissions": []string{"register_apps"}}, &out); code != http.StatusConflict || out["error"] != msg {
		t.Fatalf("strip admin role %d %v", code, out)
	}
	createPersonAs(t, admin, "Caio", "caio@alis.com", "admin")
	if code := admin.do("PUT", "/api/people/1", map[string]any{"role_id": 3}, nil); code != http.StatusOK {
		t.Fatalf("demote with another admin = %d", code)
	}
}

func TestAreasAndLeaders(t *testing.T) {
	admin, ana, bia, _ := team(t)
	var a areaOut
	if code := admin.do("POST", "/api/areas", map[string]string{"name": "Operações"}, &a); code != http.StatusCreated {
		t.Fatalf("create area %d", code)
	}
	if code := admin.do("POST", "/api/areas", map[string]string{"name": "Operações"}, nil); code != http.StatusConflict {
		t.Fatalf("duplicate = %d", code)
	}
	if code := admin.do("PUT", "/api/areas/"+itoa(a.ID)+"/leaders", map[string]any{"person_ids": []int64{2}}, nil); code != http.StatusOK {
		t.Fatalf("leaders %d", code)
	}
	if code := ana.do("PUT", "/api/areas/"+itoa(a.ID)+"/members", map[string]any{"person_ids": []int64{3}}, nil); code != http.StatusOK {
		t.Fatalf("leader sets members %d", code)
	}
	if code := ana.do("PUT", "/api/areas/1/members", map[string]any{"person_ids": []int64{3}}, nil); code != http.StatusForbidden {
		t.Fatalf("leader on another area = %d", code)
	}
	if code := ana.do("PUT", "/api/areas/"+itoa(a.ID)+"/leaders", map[string]any{"person_ids": []int64{3}}, nil); code != http.StatusForbidden {
		t.Fatalf("leader choosing leaders = %d", code)
	}
	var areas []areaOut
	bia.do("GET", "/api/areas", nil, &areas)
	var ops areaOut
	for _, x := range areas {
		if x.ID == a.ID {
			ops = x
		}
	}
	if len(ops.Leaders) != 1 || len(ops.Members) != 1 {
		t.Fatalf("area = %+v", ops)
	}
	if code := admin.do("DELETE", "/api/areas/1", nil, nil); code != http.StatusConflict {
		t.Fatalf("delete area with apps = %d", code)
	}
	if code := admin.do("PUT", "/api/areas/1", map[string]string{"name": "Pesquisa"}, nil); code != http.StatusOK {
		t.Fatalf("rename %d", code)
	}
	var list []appSummary
	admin.do("GET", "/api/apps", nil, &list)
	if list[0].Area != "Pesquisa" {
		t.Fatalf("rename must reach the apps: %+v", list[0])
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func TestAreaPeopleAtomic(t *testing.T) {
	admin, _, _, _ := team(t)
	var a areaOut
	admin.do("POST", "/api/areas", map[string]string{"name": "Atômica"}, &a)
	p := "/api/areas/" + itoa(a.ID)
	for _, k := range []string{"/leaders", "/members"} {
		if code := admin.do("PUT", p+k, map[string]any{"person_ids": []int64{2}}, nil); code != http.StatusOK {
			t.Fatalf("set %s %d", k, code)
		}
		var out map[string]any
		if code := admin.do("PUT", p+k, map[string]any{"person_ids": []int64{3, 9999}}, &out); code != http.StatusBadRequest || out["error"] != "Pessoa não encontrada." {
			t.Fatalf("unknown id %s = %d %v", k, code, out)
		}
	}
	var areas []areaOut
	admin.do("GET", "/api/areas", nil, &areas)
	for _, x := range areas {
		if x.ID == a.ID && (len(x.Leaders) != 1 || x.Leaders[0].ID != 2 || len(x.Members) != 1 || x.Members[0].ID != 2) {
			t.Fatalf("area changed: %+v", x)
		}
	}
}

func TestCannotGrantMoreThanOwn(t *testing.T) {
	admin, ana, _, _ := team(t)
	var r roleOut
	admin.do("POST", "/api/roles", map[string]any{"name": "RH", "permissions": []string{"manage_people"}}, &r)
	if code := admin.do("PUT", "/api/people/2", map[string]any{"role_id": r.ID}, nil); code != http.StatusOK {
		t.Fatalf("give RH %d", code)
	}
	const msg = "Você não pode dar um cargo com mais permissões que o seu."
	for _, id := range []string{"2", "3"} {
		var out map[string]any
		if code := ana.do("PUT", "/api/people/"+id, map[string]any{"role_id": 1}, &out); code != http.StatusForbidden || out["error"] != msg {
			t.Fatalf("ana gives admin to %s = %d %v", id, code, out)
		}
	}
	if code := ana.do("PUT", "/api/people/3", map[string]any{"role_id": 3}, nil); code != http.StatusOK {
		t.Fatalf("ana gives usuario = %d", code)
	}
}

func TestCannotCreatePersonWithBiggerRole(t *testing.T) {
	admin, ana, _, _ := team(t)
	var r roleOut
	admin.do("POST", "/api/roles", map[string]any{"name": "RH", "permissions": []string{"manage_people"}}, &r)
	if code := admin.do("PUT", "/api/people/2", map[string]any{"role_id": r.ID}, nil); code != http.StatusOK {
		t.Fatalf("give RH %d", code)
	}
	const msg = "Você não pode dar um cargo com mais permissões que o seu."
	body := map[string]any{"name": "Caio", "email": "caio@alis.com", "password": "senha-forte-1", "role_id": 1}
	var out map[string]any
	if code := ana.do("POST", "/api/people", body, &out); code != http.StatusForbidden || out["error"] != msg {
		t.Fatalf("RH creates an admin = %d %v", code, out)
	}
	body["role"], body["role_id"] = "admin", 0
	if code := ana.do("POST", "/api/people", body, &out); code != http.StatusForbidden {
		t.Fatalf("RH creates an admin by key = %d %v", code, out)
	}
	body["role_id"] = r.ID
	if code := ana.do("POST", "/api/people", body, nil); code != http.StatusCreated {
		t.Fatalf("RH creates another RH = %d", code)
	}
	body["email"], body["role_id"] = "dani@alis.com", 9999
	if code := ana.do("POST", "/api/people", body, &out); code != http.StatusBadRequest || out["error"] != "Escolha o cargo da pessoa." {
		t.Fatalf("unknown role = %d %v", code, out)
	}
}

func TestCannotChangeRoleOfBiggerPerson(t *testing.T) {
	admin, ana, _, _ := team(t)
	var r roleOut
	admin.do("POST", "/api/roles", map[string]any{"name": "RH", "permissions": []string{"manage_people"}}, &r)
	admin.do("PUT", "/api/people/2", map[string]any{"role_id": r.ID}, nil)
	createPersonAs(t, admin, "Caio", "caio@alis.com", "admin") // id 4, another admin
	const msg = "Você não pode mudar o cargo de quem tem mais permissões que você."
	var out map[string]any
	if code := ana.do("PUT", "/api/people/4", map[string]any{"role_id": 3}, &out); code != http.StatusForbidden || out["error"] != msg {
		t.Fatalf("RH demotes an admin = %d %v", code, out)
	}
	// Someone whose role is within hers: allowed, both directions.
	if code := ana.do("PUT", "/api/people/3", map[string]any{"role_id": r.ID}, nil); code != http.StatusOK {
		t.Fatalf("RH promotes usuario to RH = %d", code)
	}
	if code := ana.do("PUT", "/api/people/3", map[string]any{"role_id": 3}, nil); code != http.StatusOK {
		t.Fatalf("RH moves RH back to usuario = %d", code)
	}
	// Bia as criador (register_apps, which Ana lacks): Ana cannot touch her.
	admin.do("PUT", "/api/people/3", map[string]any{"role_id": 2}, nil)
	if code := ana.do("PUT", "/api/people/3", map[string]any{"role_id": 3}, &out); code != http.StatusForbidden || out["error"] != msg {
		t.Fatalf("RH demotes criador = %d %v", code, out)
	}
}
