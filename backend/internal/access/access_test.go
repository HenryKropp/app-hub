package access

import "testing"

func person(id int64) *Person {
	return &Person{ID: id, Perms: map[string]bool{}, Leads: map[int64]bool{}, Member: map[int64]bool{},
		ViewerApps: map[int64]bool{}, AppGrants: map[int64]map[string]bool{}, AreaGrants: map[int64]map[string]bool{}}
}

var app = App{ID: 10, AreaID: 1, OwnerID: 1, BackupID: 2}

func TestColleagueOnOpenApp(t *testing.T) {
	c := person(9).For(app)
	if !c.View || c.Edit || c.Restore || c.Tech || c.Runs || c.Access || c.Grants {
		t.Fatalf("%+v", c)
	}
}

func TestOwnerAndBackupCareButDoNotSeeOthersRuns(t *testing.T) {
	for _, id := range []int64{1, 2} {
		c := person(id).For(app)
		if !c.View || !c.Edit || !c.Restore || !c.Tech || !c.Access || c.Runs || c.Grants {
			t.Fatalf("person %d: %+v", id, c)
		}
	}
}

func TestLeaderDoesEverythingInArea(t *testing.T) {
	p := person(5)
	p.Leads[1] = true
	c := p.For(App{ID: 10, AreaID: 1, OwnerID: 1, Restricted: true})
	if !(c.View && c.Edit && c.Restore && c.Tech && c.Runs && c.Access && c.Grants) {
		t.Fatalf("%+v", c)
	}
	if other := p.For(App{ID: 11, AreaID: 2, OwnerID: 1}); other.Edit || other.Grants {
		t.Fatalf("leader of area 1 in area 2: %+v", other)
	}
}

func TestRolePermissions(t *testing.T) {
	cases := []struct {
		perm string
		want func(Can) bool
	}{
		{EditAnyApp, func(c Can) bool { return c.Edit && c.Tech && !c.Restore && !c.Runs }},
		{RestoreAnyApp, func(c Can) bool { return c.Restore && c.Tech && !c.Edit }},
		{SeeTechAnyApp, func(c Can) bool { return c.Tech && !c.Edit }},
		{SeeAllRuns, func(c Can) bool { return c.Runs && !c.Tech }},
		{ManageAreas, func(c Can) bool { return c.Grants && c.Access && !c.Edit }},
	}
	for _, tc := range cases {
		p := person(9)
		p.Perms[tc.perm] = true
		if c := p.For(app); !tc.want(c) {
			t.Fatalf("%s: %+v", tc.perm, c)
		}
	}
}

func TestGrantsPerAppAndPerArea(t *testing.T) {
	p := person(9)
	p.AppGrants[10] = map[string]bool{GrantEdit: true}
	if c := p.For(app); !c.Edit || !c.Tech || c.Restore {
		t.Fatalf("app grant: %+v", c)
	}
	if c := p.For(App{ID: 11, AreaID: 1, OwnerID: 1}); c.Edit {
		t.Fatal("app grant leaked to another app")
	}
	q := person(9)
	q.AreaGrants[1] = map[string]bool{GrantRuns: true}
	if c := q.For(App{ID: 99, AreaID: 1, OwnerID: 1}); !c.Runs || c.Edit {
		t.Fatalf("area grant on a later app: %+v", c)
	}
}

func TestRestrictedVisibility(t *testing.T) {
	r := App{ID: 10, AreaID: 1, OwnerID: 1, Restricted: true}
	if person(9).For(r).View {
		t.Fatal("stranger sees restricted app")
	}
	v := person(9)
	v.ViewerApps[10] = true
	if c := v.For(r); !c.View || c.Edit {
		t.Fatalf("listed viewer: %+v", c)
	}
	all := person(9)
	all.Perms[SeeAllApps] = true
	if !all.For(r).View {
		t.Fatal("see_all_apps")
	}
	g := person(9)
	g.AppGrants[10] = map[string]bool{GrantRuns: true}
	if !g.For(r).View {
		t.Fatal("someone with an extra access must see the app")
	}
}

func TestRegisterAndTeam(t *testing.T) {
	if person(9).CanRegister() || person(9).SeesTeam() {
		t.Fatal("colleague")
	}
	l := person(9)
	l.Leads[1] = true
	if !l.CanRegister() || !l.SeesTeam() {
		t.Fatal("leader registers and sees Equipe")
	}
	c := person(9)
	c.Perms[RegisterApps] = true
	if !c.CanRegister() || c.SeesTeam() {
		t.Fatal("creator")
	}
}

// Company-wide role boxes reach only apps the person can see: on a restricted
// app they are not listed for, each one alone gives nothing at all.
func TestCompanyWidePermsDoNotRevealRestricted(t *testing.T) {
	r := App{ID: 10, AreaID: 1, OwnerID: 1, Restricted: true}
	for _, perm := range []string{RegisterApps, EditAnyApp, RestoreAnyApp, SeeTechAnyApp, SeeAllRuns, ManagePeople, ManageRoles} {
		p := person(9)
		p.Perms[perm] = true
		if c := p.For(r); c != (Can{}) {
			t.Fatalf("%s on a restricted app: %+v", perm, c)
		}
	}
	// Once the app is visible (see_all_apps or listed), the boxes apply.
	for _, perm := range []string{EditAnyApp, RestoreAnyApp, SeeTechAnyApp, SeeAllRuns} {
		p := person(9)
		p.Perms[perm] = true
		p.Perms[SeeAllApps] = true
		if c := p.For(r); !c.View || !c.Tech && !c.Runs {
			t.Fatalf("%s + see_all_apps: %+v", perm, c)
		}
		q := person(9)
		q.Perms[perm] = true
		q.ViewerApps[10] = true
		if c := q.For(r); !c.View || !c.Tech && !c.Runs {
			t.Fatalf("%s + viewer: %+v", perm, c)
		}
	}
}

func TestRestrictedVisibleToCarersAndGrants(t *testing.T) {
	r := App{ID: 10, AreaID: 1, OwnerID: 1, BackupID: 2, Restricted: true}
	for _, id := range []int64{1, 2} {
		if c := person(id).For(r); !c.View || !c.Access {
			t.Fatalf("carer %d: %+v", id, c)
		}
	}
	m := person(9)
	m.Perms[ManageAreas] = true
	if c := m.For(r); !c.View || !c.Access || !c.Grants {
		t.Fatalf("manage_areas: %+v", c)
	}
	for _, g := range Grants {
		a := person(9)
		a.AreaGrants[1] = map[string]bool{g: true}
		if !a.For(r).View {
			t.Fatalf("area grant %s must see the app", g)
		}
		p := person(9)
		p.AppGrants[10] = map[string]bool{g: true}
		if !p.For(r).View {
			t.Fatalf("app grant %s must see the app", g)
		}
	}
	other := person(9)
	other.AppGrants[11] = map[string]bool{GrantEdit: true}
	other.AreaGrants[2] = map[string]bool{GrantRuns: true}
	if c := other.For(r); c != (Can{}) {
		t.Fatalf("grants on other apps/areas: %+v", c)
	}
}
