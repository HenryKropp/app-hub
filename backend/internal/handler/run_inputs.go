package handler

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

// Execuções visíveis (ADR 033): each run keeps what was filled in its form,
// so the run page can say which values produced the files. Only typed values
// and file names are kept, never file contents; an input that looks like a
// password keeps no value at all.

// storedInput is one input of runs.inputs.
type storedInput struct {
	Values []string `json:"values,omitempty"`
	File   string   `json:"file,omitempty"`
	Masked bool     `json:"masked,omitempty"`
}

// runInputJSON is one filled input as the run page shows it.
type runInputJSON struct {
	ID     string   `json:"id"`
	Label  string   `json:"label"`
	Type   string   `json:"type"`
	Values []string `json:"values"`
	File   string   `json:"file,omitempty"`
	Masked bool     `json:"masked"`
}


// secretInput says an input should never have its value shown or kept: a
// password or secret type (no such type exists yet, but a ficha may say so)
// or a name that reads like one.
func secretInput(in manifest.Input) bool {
	t := strings.ToLower(in.Type)
	return t == "password" || t == "secret" || secretLike.MatchString(in.ID) || secretLike.MatchString(in.Label)
}

// recordInputs turns a run's form into what runs.inputs keeps. It always
// returns a JSON object, so a run without a form reads "nothing filled".
func recordInputs(m manifest.Manifest, values map[string][]string, files map[string]manifest.InputFile) []byte {
	byID := map[string]manifest.Input{}
	for _, in := range m.Inputs {
		byID[in.ID] = in
	}
	out := map[string]storedInput{}
	for id, v := range values {
		if len(v) == 0 || (len(v) == 1 && strings.TrimSpace(v[0]) == "") {
			continue
		}
		if in, ok := byID[id]; ok && secretInput(in) {
			out[id] = storedInput{Masked: true}
			continue
		}
		out[id] = storedInput{Values: append([]string(nil), v...)}
	}
	for id, f := range files {
		if f.Name == "" {
			continue
		}
		out[id] = storedInput{File: f.Name}
	}
	b, _ := json.Marshal(out)
	return b
}

// runInputs reads what a run had filled, labeled by the app's current ficha
// and in its order. ok is false when the run is older than the record and no
// schedule says what it used.
func (h *Handler) runInputs(ctx context.Context, row sqlc.GetRunRow, m manifest.Manifest) ([]runInputJSON, bool, error) {
	raw := row.Inputs
	if raw == nil && row.ScheduleID.Valid {
		// A schedule's runs (and the ones it missed) used the schedule's form.
		s, err := h.q.GetSchedule(ctx, row.ScheduleID.Int64)
		if err != nil {
			return nil, false, nil // the schedule was deleted: nothing to say
		}
		var values map[string][]string
		var files map[string]manifest.InputFile
		_ = json.Unmarshal(s.Inputs, &values)
		_ = json.Unmarshal(s.Files, &files)
		raw = recordInputs(m, values, files)
	}
	if raw == nil {
		return []runInputJSON{}, false, nil
	}
	var stored map[string]storedInput
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, false, err
	}
	out := []runInputJSON{}
	add := func(id string, in manifest.Input, known bool) {
		s, ok := stored[id]
		if !ok {
			return
		}
		delete(stored, id)
		j := runInputJSON{ID: id, Label: id, Type: "text", Values: []string{}, File: s.File, Masked: s.Masked}
		if known {
			if in.Label != "" {
				j.Label = in.Label
			}
			j.Type = in.Type
			// A ficha that now calls the input a password hides old values too.
			j.Masked = j.Masked || (secretInput(in) && s.File == "")
		} else {
			j.Masked = j.Masked || (secretLike.MatchString(id) && s.File == "")
		}
		if s.File != "" {
			j.Type = "file"
		}
		if !j.Masked && s.Values != nil {
			j.Values = s.Values
		}
		out = append(out, j)
	}
	for _, in := range m.Inputs {
		add(in.ID, in, true)
	}
	// Inputs the ficha no longer has keep their id as the name.
	rest := make([]string, 0, len(stored))
	for id := range stored {
		rest = append(rest, id)
	}
	sort.Strings(rest)
	for _, id := range rest {
		add(id, manifest.Input{}, false)
	}
	return out, true, nil
}
