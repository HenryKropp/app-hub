package bundle

import (
	"archive/zip"
	"bytes"
	"fmt"
	"hash/crc32"
	"io/fs"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type entry struct {
	name string
	body string
	mode fs.FileMode
}

func makeZip(t *testing.T, entries ...entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.mode != 0 {
			h.SetMode(e.mode)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func keys(m map[string][]byte) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func wantErr(t *testing.T, err error, part string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), part) {
		t.Fatalf("err = %v, want %q", err, part)
	}
}

func TestExtractKeepsCodeDropsJunk(t *testing.T) {
	z := makeZip(t, entry{name: "main.py", body: "print(1)"}, entry{name: "lib/util.py", body: "x=1"}, entry{name: ".env", body: "SENHA=1"},
		entry{name: ".git/config", body: "x"}, entry{name: "node_modules/x/i.js", body: "x"}, entry{name: "node_modules/y/j.js", body: "y"},
		entry{name: "ziv.yaml", body: "id: x"}, entry{name: "FICHA.md", body: "x"}, entry{name: ".apphub-state", body: "x"}, entry{name: "lib/", body: ""})
	r, err := Extract(z, SiteLimits)
	if err != nil {
		t.Fatal(err)
	}
	if got := keys(r.Files); !reflect.DeepEqual(got, []string{"lib/util.py", "main.py"}) {
		t.Fatal(got)
	}
	if string(r.Files["main.py"]) != "print(1)" {
		t.Fatal(string(r.Files["main.py"]))
	}
	if !reflect.DeepEqual(r.Dropped, []string{".env", ".git/", "node_modules/"}) {
		t.Fatal(r.Dropped)
	}
}

func TestExtractRejectsZipSlip(t *testing.T) {
	_, err := Extract(makeZip(t, entry{name: "../evil.py", body: "x"}), SiteLimits)
	wantErr(t, err, "caminho")
	_, err = Extract(makeZip(t, entry{name: "/etc/passwd", body: "x"}), SiteLimits)
	wantErr(t, err, "caminho")
	_, err = Extract(makeZip(t, entry{name: `C:\Windows\x.py`, body: "x"}), SiteLimits)
	wantErr(t, err, "caminho")
}

func TestExtractRejectsSymlink(t *testing.T) {
	_, err := Extract(makeZip(t, entry{name: "link", body: "/etc/passwd", mode: fs.ModeSymlink | 0o777}), SiteLimits)
	wantErr(t, err, "atalho")
}

func TestExtractRejectsEncrypted(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.CreateRaw(&zip.FileHeader{Name: "a.py", Method: zip.Store, Flags: 1, CRC32: crc32.ChecksumIEEE([]byte("x")), CompressedSize64: 1, UncompressedSize64: 1})
	w.Write([]byte("x"))
	zw.Close()
	_, err := Extract(buf.Bytes(), SiteLimits)
	wantErr(t, err, "senha")
}

func TestExtractZipBombCountsRealBytes(t *testing.T) {
	big := strings.Repeat("\x00", 11<<20)
	_, err := Extract(makeZip(t, entry{name: "big.bin", body: big}), SiteLimits)
	wantErr(t, err, "Os arquivos podem somar no máximo 10 MB.")

	// A header that lies about the size is still caught by counting real bytes.
	var deflated bytes.Buffer
	{
		var tmp bytes.Buffer
		zw := zip.NewWriter(&tmp)
		w, _ := zw.CreateHeader(&zip.FileHeader{Name: "x", Method: zip.Deflate})
		w.Write([]byte(big))
		zw.Close()
		zr, _ := zip.NewReader(bytes.NewReader(tmp.Bytes()), int64(tmp.Len()))
		raw, _ := zr.File[0].OpenRaw()
		b := new(bytes.Buffer)
		b.ReadFrom(raw)
		deflated = *b
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateRaw(&zip.FileHeader{Name: "liar.bin", Method: zip.Deflate, CRC32: crc32.ChecksumIEEE([]byte(big)),
		CompressedSize64: uint64(deflated.Len()), UncompressedSize64: 1})
	if err != nil {
		t.Fatal(err)
	}
	w.Write(deflated.Bytes())
	zw.Close()
	_, err = Extract(buf.Bytes(), SiteLimits)
	if err == nil {
		t.Fatal("lying header accepted")
	}
}

func TestExtractTooManyFiles(t *testing.T) {
	var es []entry
	for i := 0; i < 101; i++ {
		es = append(es, entry{name: fmt.Sprintf("f%d.py", i), body: "x"})
	}
	_, err := Extract(makeZip(t, es...), SiteLimits)
	wantErr(t, err, "100 arquivos")
	es = nil
	for i := 0; i < 5001; i++ {
		es = append(es, entry{name: fmt.Sprintf(".git/o%d", i), body: ""})
	}
	_, err = Extract(makeZip(t, append(es, entry{name: "main.py", body: "x"})...), SiteLimits)
	wantErr(t, err, "100 arquivos")
	_, err = Extract(makeZip(t, entry{name: ".env", body: "x"}), SiteLimits)
	wantErr(t, err, "100 arquivos")
}

func TestExtractBackslashNames(t *testing.T) {
	r, err := Extract(makeZip(t, entry{name: `src\app.py`, body: "x"}, entry{name: "./main.py", body: "y"}), SiteLimits)
	if err != nil {
		t.Fatal(err)
	}
	if got := keys(r.Files); !reflect.DeepEqual(got, []string{"main.py", "src/app.py"}) {
		t.Fatal(got)
	}
}

func TestExtractDuplicateCase(t *testing.T) {
	_, err := Extract(makeZip(t, entry{name: "Main.py", body: "x"}, entry{name: "main.py", body: "y"}), SiteLimits)
	wantErr(t, err, "dois arquivos com o mesmo nome")
}

func TestExtractNotAZip(t *testing.T) {
	_, err := Extract([]byte("hello"), SiteLimits)
	wantErr(t, err, "O envio não é um arquivo .zip válido.")
}

func TestIsSecretFile(t *testing.T) {
	for _, n := range []string{".env", ".ENV", ".env.local", "chave.pem", "a.KEY", "id_rsa"} {
		if !IsSecretFile(n) {
			t.Error(n)
		}
	}
	for _, n := range []string{"env.py", "main.py", "keys.py", "pem.txt"} {
		if IsSecretFile(n) {
			t.Error(n)
		}
	}
}
