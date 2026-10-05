package handler

import (
	"net/http"
	"testing"
)

type appAccessOut struct {
	Restricted bool      `json:"restricted"`
	Areas      []nameRef `json:"areas"`
	People     []nameRef `json:"people"`
	Grants     []struct {
		PersonID    int64    `json:"person_id"`
		Permissions []string `json:"permissions"`
	} `json:"grants"`
	AreaGrants []struct {
		PersonID    int64    `json:"person_id"`
		Permissions []string `json:"permissions"`
	} `json:"area_grants"`
}

func TestRestrictToAreaAndPeople(t *testing.T) {
	admin, _, bia, id := team(t)
	exec(t, "INSERT INTO area_members (area_id, person_id) VALUES (2, 3)")
	var out appAccessOut
	if code := admin.do("PUT", appPath(id, "/access"), map[string]any{"restricted": true, "area_ids": []int64{2}, "person_ids": []int64{}}, &out); code != http.StatusOK || !out.Restricted || len(out.Areas) != 1 {
		t.Fatalf("restrict %d %+v", code, out)
	}
	if code := bia.do("GET", appPath(id, ""), nil, nil); code != http.StatusOK {
		t.Fatalf("member of a listed area = %d", code)
	}
	admin.do("PUT", appPath(id, "/access"), map[string]any{"restricted": true, "area_ids": []int64{1}, "person_ids": []int64{}}, nil)
	if code := bia.do("GET", appPath(id, ""), nil, nil); code != http.StatusNotFound {
		t.Fatalf("after leaving the list = %d", code)
	}
}

func TestOwnerRestrictsButCannotGrant(t *testing.T) {
	_, ana, _, id := team(t)
	ownerOf(t, id, 2)
	if code := ana.do("PUT", appPath(id, "/access"), map[string]any{"restricted": true}, nil); code != http.StatusOK {
		t.Fatalf("owner restricts = %d", code)
	}
	if code := ana.do("PUT", appPath(id, "/grants"), map[string]any{"person_id": 3, "permissions": []string{"edit"}}, nil); code != http.StatusForbidden {
		t.Fatalf("owner grants = %d", code)
	}
}

func TestLeaderGrantsPerAppAndWholeArea(t *testing.T) {
	_, ana, bia, id := team(t)
	makeLeader(t, 1, 2)
	var out appAccessOut
	if code := ana.do("PUT", appPath(id, "/grants"), map[string]any{"person_id": 3, "permissions": []string{"edit", "voar"}}, nil); code != http.StatusBadRequest {
		t.Fatalf("unknown grant = %d", code)
	}
	if code := ana.do("PUT", appPath(id, "/grants"), map[string]any{"person_id": 3, "permissions": []string{"edit"}}, &out); code != http.StatusOK || len(out.Grants) != 1 {
		t.Fatalf("grant %d %+v", code, out)
	}
	var d appDetail
	bia.do("GET", appPath(id, ""), nil, &d)
	if !d.Can.Edit {
		t.Fatal("grant not applied")
	}
	ana.do("PUT", appPath(id, "/grants"), map[string]any{"person_id": 3, "permissions": []string{}}, nil)
	ana.do("PUT", appPath(id, "/grants"), map[string]any{"person_id": 3, "permissions": []string{"runs"}, "whole_area": true}, &out)
	if len(out.AreaGrants) != 1 || len(out.Grants) != 0 {
		t.Fatalf("area grant %+v", out)
	}
	later := insertApp(t, filesPrepManifestWithID("outro"))
	bia.do("GET", appPath(later, ""), nil, &d)
	if !d.Can.Runs || d.Can.Edit {
		t.Fatalf("area grant on a later app: %+v", d.Can)
	}
}

func TestAccessNeedsTech(t *testing.T) {
	_, _, bia, id := team(t)
	if code := bia.do("GET", appPath(id, "/access"), nil, nil); code != http.StatusForbidden {
		t.Fatalf("colleague reads access = %d", code)
	}
}

func TestAccessUnknownIDsChangeNothing(t *testing.T) {
	admin, _, _, id := team(t)
	if code := admin.do("PUT", appPath(id, "/access"), map[string]any{"restricted": true, "area_ids": []int64{1, 999}, "person_ids": []int64{}}, nil); code != http.StatusBadRequest {
		t.Fatalf("unknown area = %d", code)
	}
	if code := admin.do("PUT", appPath(id, "/access"), map[string]any{"restricted": true, "area_ids": []int64{}, "person_ids": []int64{999}}, nil); code != http.StatusBadRequest {
		t.Fatalf("unknown person = %d", code)
	}
	var out appAccessOut
	admin.do("GET", appPath(id, "/access"), nil, &out)
	if out.Restricted {
		t.Fatal("a rejected save changed the app")
	}
	if code := admin.do("PUT", appPath(id, "/grants"), map[string]any{"person_id": 999, "permissions": []string{"edit"}}, nil); code != http.StatusBadRequest {
		t.Fatalf("grant to unknown person = %d", code)
	}
}
