// Package governance decides, without a database, whether something needs
// approval, which steps, and who may decide each step (ADR 046).
package governance

// Rule says when a new version needs approval again.
type Rule string

const (
	Every Rule = "every" // every version
	Risk  Rule = "risk"  // only when the version adds risk
	First Rule = "first" // only before the first approval
)

// Request reasons.
const (
	ReasonNew     = "new"
	ReasonVersion = "version"
	ReasonFicha   = "ficha"
	ReasonArea    = "area"
)

// Steps.
const (
	StepLeader = "leader"
	StepIT     = "it"
)

// Why a step was decided by someone other than a plain approver.
const (
	OnBehalfAdmin = "admin_no_leader"
	OnBehalfSelf  = "self"
)

func rank(r Rule) int {
	switch r {
	case Every:
		return 3
	case Risk:
		return 2
	case First:
		return 1
	}
	return 0
}

func Valid(s string) bool { return rank(Rule(s)) > 0 }

// Stricter returns the stricter of two rules (every > risk > first).
func Stricter(a, b Rule) Rule {
	if rank(b) > rank(a) {
		return b
	}
	return a
}

// Effective is the rule an item follows: its own when it has one, else the
// company's. Who may set an item looser than the company is checked when the
// rule is saved (CanLoosen): only the admin.
func Effective(company Rule, item *Rule) Rule {
	if item == nil || !Valid(string(*item)) {
		return company
	}
	return *item
}

// CanLoosen: only the admin sets an item looser than the company.
func CanLoosen(perms map[string]bool) bool { return perms["manage_roles"] }

type Need struct {
	Required bool
	Steps    []string
}

// Decide says whether a request needs approval and its steps. A new item and
// an area move always do; a version or a ficha change follows the rule.
func Decide(rule Rule, everApproved bool, added []string, itOn bool, reason string) Need {
	if reason == ReasonArea {
		return Need{true, []string{StepLeader}}
	}
	required := true
	if reason != ReasonNew {
		switch rule {
		case Risk:
			required = len(added) > 0
		case First:
			required = !everApproved
		}
	}
	if !required {
		return Need{}
	}
	steps := []string{StepLeader}
	if itOn {
		steps = append(steps, StepIT)
	}
	return Need{true, steps}
}

type Approver struct {
	PersonID int64
	Perms    map[string]bool
	Leads    map[int64]bool
}

// MayDecide says whether a may decide step for an item in areaID. The leader
// step is the area leader's; the admin (manage_roles) decides it only when the
// area has no leader. The TI step is approve_it's.
func MayDecide(step string, areaID int64, areaHasLeader bool, a Approver) (bool, string) {
	switch step {
	case StepLeader:
		if a.Leads[areaID] {
			return true, ""
		}
		if !areaHasLeader && a.Perms["manage_roles"] {
			return true, OnBehalfAdmin
		}
	case StepIT:
		return a.Perms["approve_it"], ""
	}
	return false, ""
}

// AutoSteps: the steps the requester already satisfies by asking (a leader's
// own app, TI's own app, the admin alone in an area without leader).
func AutoSteps(steps []string, areaID int64, areaHasLeader bool, requester Approver) map[string]string {
	out := map[string]string{}
	for _, s := range steps {
		if ok, _ := MayDecide(s, areaID, areaHasLeader, requester); ok {
			out[s] = OnBehalfSelf
		}
	}
	return out
}
