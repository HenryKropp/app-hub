package governance

import (
	"reflect"
	"testing"
)

func TestDecide(t *testing.T) {
	risky := []string{"site: xp.com"}
	cases := []struct {
		name   string
		rule   Rule
		ever   bool
		added  []string
		it     bool
		reason string
		want   Need
	}{
		{"new always", First, false, nil, false, ReasonNew, Need{true, []string{StepLeader}}},
		{"new with TI", Risk, false, nil, true, ReasonNew, Need{true, []string{StepLeader, StepIT}}},
		{"every plain version", Every, true, nil, false, ReasonVersion, Need{true, []string{StepLeader}}},
		{"risk plain version", Risk, true, nil, true, ReasonVersion, Need{false, nil}},
		{"risk risky version", Risk, true, risky, true, ReasonVersion, Need{true, []string{StepLeader, StepIT}}},
		{"risk risky ficha", Risk, true, risky, false, ReasonFicha, Need{true, []string{StepLeader}}},
		{"first after approval", First, true, risky, false, ReasonVersion, Need{false, nil}},
		{"first before approval", First, false, nil, false, ReasonVersion, Need{true, []string{StepLeader}}},
		{"area only leader", Every, true, nil, true, ReasonArea, Need{true, []string{StepLeader}}},
	}
	for _, c := range cases {
		if got := Decide(c.rule, c.ever, c.added, c.it, c.reason); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %+v want %+v", c.name, got, c.want)
		}
	}
}

func TestEffectiveItemRuleWins(t *testing.T) {
	first, every := First, Every
	// Only the admin can save a looser item rule; once saved, it counts.
	if got := Effective(Every, &first); got != First {
		t.Fatalf("company every + item first = %s", got)
	}
	if got := Effective(First, &every); got != Every {
		t.Fatalf("company first + item every = %s", got)
	}
	if got := Effective(Risk, nil); got != Risk {
		t.Fatalf("no item rule = %s", got)
	}
}

func TestStricter(t *testing.T) {
	if Stricter(Risk, First) != Risk || Stricter(First, Every) != Every || Stricter(Risk, Risk) != Risk {
		t.Fatal("order every > risk > first")
	}
	if !Valid("every") || Valid("never") {
		t.Fatal("Valid")
	}
}

func TestMayDecideAdminOnlyWithoutLeader(t *testing.T) {
	admin := Approver{PersonID: 1, Perms: map[string]bool{"manage_roles": true}, Leads: map[int64]bool{}}
	if ok, _ := MayDecide(StepLeader, 3, true, admin); ok {
		t.Fatal("admin passed over an existing leader")
	}
	if ok, why := MayDecide(StepLeader, 3, false, admin); !ok || why != OnBehalfAdmin {
		t.Fatalf("admin without leader: %v %q", ok, why)
	}
	leader := Approver{PersonID: 2, Perms: map[string]bool{}, Leads: map[int64]bool{3: true}}
	if ok, why := MayDecide(StepLeader, 3, true, leader); !ok || why != "" {
		t.Fatalf("leader: %v %q", ok, why)
	}
	if ok, _ := MayDecide(StepLeader, 4, true, leader); ok {
		t.Fatal("leader of another area")
	}
	it := Approver{PersonID: 3, Perms: map[string]bool{"approve_it": true}, Leads: map[int64]bool{}}
	if ok, _ := MayDecide(StepIT, 3, true, it); !ok {
		t.Fatal("approve_it")
	}
	if ok, _ := MayDecide(StepIT, 3, true, leader); ok {
		t.Fatal("leader is not TI")
	}
}

func TestAutoStepsSelf(t *testing.T) {
	both := Approver{PersonID: 1, Perms: map[string]bool{"approve_it": true, "manage_roles": true}, Leads: map[int64]bool{3: true}}
	got := AutoSteps([]string{StepLeader, StepIT}, 3, true, both)
	if !reflect.DeepEqual(got, map[string]string{StepLeader: OnBehalfSelf, StepIT: OnBehalfSelf}) {
		t.Fatalf("both: %v", got)
	}
	plain := Approver{PersonID: 2, Perms: map[string]bool{}, Leads: map[int64]bool{}}
	if got := AutoSteps([]string{StepLeader}, 3, true, plain); len(got) != 0 {
		t.Fatalf("plain: %v", got)
	}
	// One-person company: the admin is alone and the area has no leader.
	alone := Approver{PersonID: 1, Perms: map[string]bool{"manage_roles": true, "approve_it": true}, Leads: map[int64]bool{}}
	if got := AutoSteps([]string{StepLeader, StepIT}, 3, false, alone); len(got) != 2 {
		t.Fatalf("alone: %v", got)
	}
}
