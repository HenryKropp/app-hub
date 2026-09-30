package agent

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

func recordRun(t *testing.T, state, folder string, paths ...string) {
	t.Helper()
	files := []agentproto.FileEntry{}
	for _, p := range paths {
		files = append(files, agentproto.FileEntry{Path: p, Change: "new"})
	}
	if err := saveRecord(state, 5, folder, files); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareSendSingle(t *testing.T) {
	state, folder := t.TempDir(), t.TempDir()
	write(t, folder, "P/MD_Output/pack.md", "# pack")
	recordRun(t, state, folder, "P/MD_Output/pack.md")
	o, err := PrepareSend(state, agentproto.SendFilePayload{RunID: 5, Paths: []string{"P/MD_Output/pack.md"}})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := o.Stream(&buf); err != nil || buf.String() != "# pack" {
		t.Fatalf("%v %q", err, buf.String())
	}
}

func TestPrepareSendZipSkipsMissing(t *testing.T) {
	state, folder := t.TempDir(), t.TempDir()
	write(t, folder, "a.pdf", "A")
	recordRun(t, state, folder, "a.pdf", "b.pdf")
	o, err := PrepareSend(state, agentproto.SendFilePayload{RunID: 5, Paths: []string{"a.pdf", "b.pdf"}, Zip: true})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := o.Stream(&buf); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil || len(zr.File) != 1 || zr.File[0].Name != "a.pdf" {
		t.Fatalf("zip %v %+v", err, zr)
	}
}

func TestPrepareSendRefuses(t *testing.T) {
	state, folder := t.TempDir(), t.TempDir()
	write(t, folder, "a.pdf", "A")
	write(t, folder, "segredo.txt", "S")
	recordRun(t, state, folder, "a.pdf", "../fora.txt")
	for _, p := range [][]string{{"segredo.txt"}, {"../fora.txt"}} {
		if _, err := PrepareSend(state, agentproto.SendFilePayload{RunID: 5, Paths: p}); !errors.Is(err, ErrNotAllowed) {
			t.Fatalf("%v: %v", p, err)
		}
	}
	if _, err := PrepareSend(state, agentproto.SendFilePayload{RunID: 99, Paths: []string{"a.pdf"}}); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("unknown run: %v", err)
	}
	_ = os.Remove(filepath.Join(folder, "a.pdf"))
	if _, err := PrepareSend(state, agentproto.SendFilePayload{RunID: 5, Paths: []string{"a.pdf"}}); !errors.Is(err, ErrMissing) {
		t.Fatalf("missing: %v", err)
	}
}
