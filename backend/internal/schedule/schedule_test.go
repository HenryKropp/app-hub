package schedule

import (
	"testing"
	"time"
)

func sp(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, Location)
	if err != nil {
		panic(err)
	}
	return t
}

func TestNext(t *testing.T) {
	cases := []struct {
		name  string
		f     Frequency
		after string
		want  string
	}{
		{"minutes 5", Frequency{Kind: "minutes", Every: 5}, "2026-11-06 10:02", "2026-11-06 10:05"},
		{"minutes on the mark", Frequency{Kind: "minutes", Every: 5}, "2026-11-06 10:05", "2026-11-06 10:10"},
		{"minutes 60 across midnight", Frequency{Kind: "minutes", Every: 60}, "2026-11-06 23:30", "2026-11-07 00:00"},
		{"minutes 7 restarts at midnight", Frequency{Kind: "minutes", Every: 7}, "2026-11-06 23:58", "2026-11-07 00:00"},
		{"hours 24", Frequency{Kind: "hours", Every: 24}, "2026-11-06 10:00", "2026-11-07 00:00"},
		{"hours 3", Frequency{Kind: "hours", Every: 3}, "2026-11-06 10:00", "2026-11-06 12:00"},
		{"daily strictly after", Frequency{Kind: "daily", At: "07:00"}, "2026-11-06 07:00", "2026-11-07 07:00"},
		{"daily later today", Frequency{Kind: "daily", At: "07:00"}, "2026-11-06 06:59", "2026-11-06 07:00"},
		{"weekdays friday to monday", Frequency{Kind: "weekdays", At: "07:30", Days: []int{1, 3, 5}}, "2026-11-06 08:00", "2026-11-09 07:30"},
		{"weekdays saturday 23:59", Frequency{Kind: "weekdays", At: "23:59", Days: []int{6}}, "2026-11-07 23:59", "2026-11-14 23:59"},
		{"monthly 28 into february", Frequency{Kind: "monthly", At: "09:00", Day: 28}, "2027-01-28 10:00", "2027-02-28 09:00"},
		{"monthly new year", Frequency{Kind: "monthly", At: "00:00", Day: 1}, "2026-12-31 23:00", "2027-01-01 00:00"},
	}
	for _, c := range cases {
		got := c.f.Next(sp(c.after))
		if !got.Equal(sp(c.want)) {
			t.Errorf("%s: got %s want %s", c.name, got.In(Location).Format("2006-01-02 15:04"), c.want)
		}
	}
}

func TestValidate(t *testing.T) {
	bad := []Frequency{
		{Kind: "minutes", Every: 4},
		{Kind: "minutes", Every: 1441},
		{Kind: "hours", Every: 25},
		{Kind: "hours", Every: 0},
		{Kind: "daily", At: "7h"},
		{Kind: "daily", At: "24:00"},
		{Kind: "weekdays", At: "07:00"},
		{Kind: "weekdays", At: "07:00", Days: []int{7}},
		{Kind: "monthly", At: "07:00", Day: 29},
		{Kind: "yearly"},
	}
	for _, f := range bad {
		if f.Validate() == nil {
			t.Errorf("%+v should be refused", f)
		}
	}
	good := []Frequency{
		{Kind: "minutes", Every: 5},
		{Kind: "hours", Every: 24},
		{Kind: "daily", At: "07:00"},
		{Kind: "weekdays", At: "07:30", Days: []int{1, 3, 5}},
		{Kind: "monthly", At: "09:00", Day: 28},
	}
	for _, f := range good {
		if err := f.Validate(); err != nil {
			t.Errorf("%+v: %v", f, err)
		}
	}
	if err := (Frequency{Kind: "minutes", Every: 4}).Validate(); err == nil || err.Error() != "No mínimo a cada 5 minutos." {
		t.Errorf("floor message: %v", err)
	}
}

func TestDescribe(t *testing.T) {
	cases := map[string]Frequency{
		"A cada 5 minutos":                {Kind: "minutes", Every: 5},
		"A cada hora":                     {Kind: "hours", Every: 1},
		"A cada 3 horas":                  {Kind: "hours", Every: 3},
		"Todo dia às 7h":                  {Kind: "daily", At: "07:00"},
		"Seg, qua e sex às 7h30":          {Kind: "weekdays", At: "07:30", Days: []int{5, 1, 3}},
		"Todo mês no dia 5 às 9h":         {Kind: "monthly", At: "09:00", Day: 5},
		"Dom às 23h59":                    {Kind: "weekdays", At: "23:59", Days: []int{0}},
	}
	for want, f := range cases {
		if got := f.Describe(); got != want {
			t.Errorf("got %q want %q", got, want)
		}
	}
}

func TestUpcoming(t *testing.T) {
	got := Frequency{Kind: "daily", At: "07:00"}.Upcoming(sp("2026-11-06 08:00"), 3)
	if len(got) != 3 || !got[0].Equal(sp("2026-11-07 07:00")) || !got[2].Equal(sp("2026-11-09 07:00")) {
		t.Fatalf("%v", got)
	}
}
