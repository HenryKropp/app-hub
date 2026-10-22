package manifest

import (
	"encoding/base64"
	"testing"
)

func TestScriptInputsValidation(t *testing.T) {
	m := Manifest{Kind: KindScript, Entrypoint: "main.py", Inputs: []Input{
		{ID: "file", Label: "Arquivo", Type: "file", Source: "argument", Required: true},
		{ID: "amount", Label: "Valor", Type: "number", Source: "argument", Required: true},
		{ID: "date", Label: "Data", Type: "date", Source: "argument", Required: true},
		{ID: "choice", Label: "Opção", Type: "choice", Source: "argument", Options: []string{"A", "B"}, Multiple: true},
	}}
	values := map[string][]string{"amount": {"-2.5"}, "date": {"2026-10-22"}, "choice": {"A", "B"}}
	files := map[string]InputFile{"file": {Name: "data.csv", ContentBase64: base64.StdEncoding.EncodeToString([]byte("a,b"))}}
	if err := m.ValidateRunInputs(values, files); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"NaN", "Infinity", "not-number"} {
		values["amount"] = []string{bad}
		if m.ValidateRunInputs(values, files) == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
	values["amount"] = []string{"1"}
	values["date"] = []string{"2026-02-30"}
	if m.ValidateRunInputs(values, files) == nil {
		t.Fatal("invalid date")
	}
	values["date"] = []string{"2026-10-22"}
	values["choice"] = []string{"unknown"}
	if m.ValidateRunInputs(values, files) == nil {
		t.Fatal("unknown choice")
	}
	delete(values, "choice")
	values["not_in_manifest"] = []string{"value"}
	if m.ValidateRunInputs(values, files) == nil {
		t.Fatal("unknown input")
	}
	delete(values, "not_in_manifest")
	files["file"] = InputFile{Name: "../main.py", ContentBase64: ""}
	if m.ValidateRunInputs(values, files) == nil {
		t.Fatal("unsafe file name")
	}
}

func TestScriptPathsAndCodeUploads(t *testing.T) {
	for _, p := range []string{"../x.py", `C:\main.py`, "/main.py", "foo/../main.py", "aux.txt", "main.py:stream", "a//b", "a/CON.txt"} {
		if SafeRelative(p) {
			t.Errorf("accepted %q", p)
		}
	}
	if ScriptRuntime("app/main.py") != "python" || ScriptRuntime("main.mjs") != "node" || ScriptRuntime("main.bat") != "" {
		t.Fatal("runtime allowlist")
	}
	m := Manifest{Kind: KindScript, Entrypoint: "main.py", Inputs: []Input{{ID: "file", Label: "Arquivo", Type: "file", Source: "fixed_file", Path: "module.py"}}}
	if len(m.ScriptProblems()) == 0 {
		t.Fatal("code overwrite allowed")
	}
	m.Inputs[0].Path = "input/data.csv"
	m.Inputs[0].Flag = "--file & powershell"
	if len(m.ScriptProblems()) == 0 {
		t.Fatal("unsafe flag")
	}
	m.Inputs[0].Flag = "--input-file"
	if len(m.ScriptProblems()) != 0 {
		t.Fatal(m.ScriptProblems())
	}
}
