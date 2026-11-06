// Package schedule turns a schedule's frequency (the ready-made options the
// person picks: every N minutes or hours, daily, some weekdays, monthly) into
// run times and into a plain-Portuguese description. Everything happens in
// Brasília time.
package schedule

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	_ "time/tzdata" // the server may not ship zoneinfo
)

// Location is the only time zone schedules use.
var Location = mustLoad("America/Sao_Paulo")

func mustLoad(name string) *time.Location {
	l, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return l
}

// Grace is how late a run may still start after the Ziv was down.
const Grace = time.Hour

// MinMinutes is the shortest interval a schedule may have.
const MinMinutes = 5

type Frequency struct {
	Kind  string `json:"kind"`            // minutes | hours | daily | weekdays | monthly
	Every int    `json:"every,omitempty"` // minutes, hours
	At    string `json:"at,omitempty"`    // "HH:MM" for daily, weekdays, monthly
	Days  []int  `json:"days,omitempty"`  // weekdays: 0 = domingo .. 6 = sábado
	Day   int    `json:"day,omitempty"`   // monthly: 1..28
}

func parseAt(at string) (h, m int, ok bool) {
	if len(at) != 5 || at[2] != ':' {
		return 0, 0, false
	}
	if _, err := fmt.Sscanf(at, "%02d:%02d", &h, &m); err != nil {
		return 0, 0, false
	}
	return h, m, h >= 0 && h < 24 && m >= 0 && m < 60
}

// Validate explains, in Portuguese, why a frequency cannot be used.
func (f Frequency) Validate() error {
	switch f.Kind {
	case "minutes":
		if f.Every < MinMinutes {
			return errors.New("No mínimo a cada 5 minutos.")
		}
		if f.Every > 1440 {
			return errors.New("No máximo a cada 1440 minutos (um dia).")
		}
		return nil
	case "hours":
		if f.Every < 1 || f.Every > 24 {
			return errors.New("Escolha de 1 a 24 horas.")
		}
		return nil
	case "daily", "weekdays", "monthly":
		if _, _, ok := parseAt(f.At); !ok {
			return errors.New("Informe o horário no formato HH:MM, por exemplo 07:00.")
		}
		if f.Kind == "weekdays" {
			if len(f.Days) == 0 {
				return errors.New("Escolha pelo menos um dia da semana.")
			}
			for _, d := range f.Days {
				if d < 0 || d > 6 {
					return errors.New("Dia da semana inválido.")
				}
			}
		}
		if f.Kind == "monthly" && (f.Day < 1 || f.Day > 28) {
			return errors.New("Escolha um dia do mês de 1 a 28.")
		}
		return nil
	}
	return errors.New("Escolha com que frequência o app roda.")
}

// Next is the first run time strictly after the given moment. Intervals in
// minutes or hours count from local midnight, so "a cada 15 minutos" runs at
// :00, :15, :30, :45 whatever the time the schedule was made.
func (f Frequency) Next(after time.Time) time.Time {
	t := after.In(Location)
	midnight := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, Location)
	switch f.Kind {
	case "minutes", "hours":
		step := time.Duration(f.Every) * time.Minute
		if f.Kind == "hours" {
			step = time.Duration(f.Every) * time.Hour
		}
		n := t.Sub(midnight)/step + 1
		next := midnight.Add(n * step)
		tomorrow := midnight.AddDate(0, 0, 1)
		if next.After(tomorrow) {
			next = tomorrow
		}
		return next
	}
	h, m, _ := parseAt(f.At)
	for d := 0; d < 400; d++ {
		day := midnight.AddDate(0, 0, d)
		at := time.Date(day.Year(), day.Month(), day.Day(), h, m, 0, 0, Location)
		if !at.After(t) {
			continue
		}
		switch f.Kind {
		case "daily":
			return at
		case "weekdays":
			if slices.Contains(f.Days, int(at.Weekday())) {
				return at
			}
		case "monthly":
			if at.Day() == f.Day {
				return at
			}
		}
	}
	return time.Time{}
}

// Upcoming lists the next n run times after the given moment.
func (f Frequency) Upcoming(after time.Time, n int) []time.Time {
	out := make([]time.Time, 0, n)
	for i := 0; i < n; i++ {
		after = f.Next(after)
		if after.IsZero() {
			break
		}
		out = append(out, after)
	}
	return out
}

var dayNames = []string{"Dom", "Seg", "Ter", "Qua", "Qui", "Sex", "Sáb"}

func hour(at string) string {
	h, m, _ := parseAt(at)
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh%02d", h, m)
}

func joinPT(items []string) string {
	if len(items) == 1 {
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " e " + items[len(items)-1]
}

// Describe says the frequency in plain Portuguese ("Todo dia às 7h").
func (f Frequency) Describe() string {
	switch f.Kind {
	case "minutes":
		return fmt.Sprintf("A cada %d minutos", f.Every)
	case "hours":
		if f.Every == 1 {
			return "A cada hora"
		}
		return fmt.Sprintf("A cada %d horas", f.Every)
	case "daily":
		return "Todo dia às " + hour(f.At)
	case "weekdays":
		days := slices.Clone(f.Days)
		slices.Sort(days)
		days = slices.Compact(days)
		names := make([]string, 0, len(days))
		for i, d := range days {
			n := dayNames[d]
			if i > 0 {
				n = strings.ToLower(n)
			}
			names = append(names, n)
		}
		return joinPT(names) + " às " + hour(f.At)
	case "monthly":
		return fmt.Sprintf("Todo mês no dia %d às %s", f.Day, hour(f.At))
	}
	return ""
}
