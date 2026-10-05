// Package access decides what a person may do in the Hub. It is pure (no
// database): the handler loads the facts once per request and asks here.
package access

// Role permissions: company-wide, ticked per role in Equipe → Cargos.
const (
	RegisterApps  = "register_apps"
	EditAnyApp    = "edit_any_app"
	RestoreAnyApp = "restore_any_app"
	SeeTechAnyApp = "see_tech_any_app"
	SeeAllRuns    = "see_all_runs"
	SeeAllApps    = "see_all_apps"
	ManagePeople  = "manage_people"
	ManageAreas   = "manage_areas"
	ManageRoles   = "manage_roles"
)

var RolePerms = []string{RegisterApps, EditAnyApp, RestoreAnyApp, SeeTechAnyApp, SeeAllRuns, SeeAllApps, ManagePeople, ManageAreas, ManageRoles}

// Extra accesses an area leader gives to one person, per app or for the whole area.
const (
	GrantEdit    = "edit"
	GrantRestore = "restore"
	GrantTech    = "tech"
	GrantRuns    = "runs"
)

var Grants = []string{GrantEdit, GrantRestore, GrantTech, GrantRuns}

type Person struct {
	ID         int64
	Perms      map[string]bool
	Leads      map[int64]bool // area ids
	Member     map[int64]bool // area ids
	ViewerApps map[int64]bool // restricted apps that list the person or one of their areas
	AppGrants  map[int64]map[string]bool
	AreaGrants map[int64]map[string]bool
}

func (p *Person) Has(perm string) bool { return p.Perms[perm] }

// CanRegister: the role allows it, or the person leads an area.
func (p *Person) CanRegister() bool { return p.Has(RegisterApps) || len(p.Leads) > 0 }

// SeesTeam: Equipe shows for whoever manages people, areas or roles, or leads an area.
func (p *Person) SeesTeam() bool {
	return p.Has(ManagePeople) || p.Has(ManageAreas) || p.Has(ManageRoles) || len(p.Leads) > 0
}

type App struct {
	ID, AreaID, OwnerID, BackupID int64 // BackupID 0 = no backup
	Restricted                    bool
}

type Can struct {
	View    bool `json:"view"`    // see it in the catalog and run it
	Edit    bool `json:"edit"`    // change the ficha
	Restore bool `json:"restore"` // go back to an older version
	Tech    bool `json:"tech"`    // see the technical part of the ficha
	Runs    bool `json:"runs"`    // see everyone's runs of this app
	Access  bool `json:"access"`  // open/restrict the app and choose who sees it
	Grants  bool `json:"grants"`  // give and take extra accesses
}

func (p *Person) For(a App) Can {
	leader := p.Leads[a.AreaID]
	cares := leader || a.OwnerID == p.ID || (a.BackupID != 0 && a.BackupID == p.ID)
	granted := func(g string) bool { return p.AppGrants[a.ID][g] || p.AreaGrants[a.AreaID][g] }

	var c Can
	c.Edit = cares || p.Has(EditAnyApp) || granted(GrantEdit)
	c.Restore = cares || p.Has(RestoreAnyApp) || granted(GrantRestore)
	c.Tech = c.Edit || c.Restore || p.Has(SeeTechAnyApp) || granted(GrantTech)
	c.Runs = leader || p.Has(SeeAllRuns) || granted(GrantRuns)
	c.Grants = leader || p.Has(ManageAreas)
	c.Access = cares || c.Grants
	c.View = !a.Restricted || p.Has(SeeAllApps) || p.ViewerApps[a.ID] || c.Access || c.Tech || c.Runs
	return c
}
