// Package bundle opens the zip a person's Claude sends to the Ziv (MCP Etapa
// 2) without trusting anything in it: names, sizes and counts are checked
// against the real bytes, and secrets and junk folders are left out.
package bundle

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

// Limits bound one upload.
type Limits struct{ MaxZipBytes, MaxEntries, MaxFiles, MaxTotal int }

// SiteLimits are the same limits the site's script import uses.
var SiteLimits = Limits{MaxZipBytes: 12 << 20, MaxEntries: 5000, MaxFiles: 100, MaxTotal: manifest.MaxInputBytes}

// Result is what is kept from the zip.
type Result struct {
	Files   map[string][]byte // slash paths, relative, kept
	Dropped []string          // left out (secret files; junk folders once, as "node_modules/"), sorted, at most 20
}

const maxDropped = 20

var (
	errNotZip   = errors.New("O envio não é um arquivo .zip válido.")
	errCount    = errors.New("Envie de 1 a 100 arquivos do app. Deixe de fora dados e saídas acumuladas.")
	errTooLarge = errors.New("Os arquivos podem somar no máximo 10 MB.")
)

var junkDirs = map[string]bool{".git": true, ".venv": true, "venv": true, "node_modules": true, "__pycache__": true, ".mypy_cache": true, ".pytest_cache": true}

// IsSecretFile reports a credentials file by its base name: .env, .env.*,
// *.pem, *.key, id_rsa (any case).
func IsSecretFile(base string) bool {
	b := strings.ToLower(base)
	return b == ".env" || strings.HasPrefix(b, ".env.") || strings.HasSuffix(b, ".pem") || strings.HasSuffix(b, ".key") || b == "id_rsa"
}

// Extract opens data as a zip and returns the files to keep. Every error is
// a Portuguese sentence for the person.
func Extract(data []byte, lim Limits) (Result, error) {
	if len(data) > lim.MaxZipBytes {
		return Result{}, errTooLarge
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil && !(errors.Is(err, zip.ErrInsecurePath) && zr != nil) { // insecure names are refused below, by name
		return Result{}, errNotZip
	}
	if len(zr.File) > lim.MaxEntries {
		return Result{}, errCount
	}
	res := Result{Files: map[string][]byte{}}
	dropped := map[string]bool{}
	seen := map[string]bool{}
	total := 0
	for _, f := range zr.File {
		name := strings.ReplaceAll(f.Name, `\`, "/")
		for strings.HasPrefix(name, "./") {
			name = name[2:]
		}
		if strings.HasSuffix(name, "/") || f.FileInfo().IsDir() {
			continue
		}
		if f.Mode()&fs.ModeSymlink != 0 {
			return Result{}, errors.New("O pacote tem um atalho: " + name + ".")
		}
		if f.Flags&1 != 0 {
			return Result{}, errors.New("O pacote tem um arquivo com senha: " + name + ".")
		}
		if !manifest.SafeRelative(name) {
			return Result{}, errors.New("O pacote tem um caminho inseguro: " + name + ".")
		}
		parts := strings.Split(name, "/")
		if junk := junkPrefix(parts); junk != "" {
			dropped[junk] = true
			continue
		}
		base := parts[len(parts)-1]
		if IsSecretFile(base) {
			dropped[name] = true
			continue
		}
		if manifest.IsManifestName(base) || strings.EqualFold(base, manifest.FichaName) || strings.HasPrefix(strings.ToLower(base), ".apphub-") {
			continue
		}
		key := strings.ToLower(name)
		if seen[key] {
			return Result{}, errors.New("O pacote tem dois arquivos com o mesmo nome: " + name + ".")
		}
		seen[key] = true
		if len(seen) > lim.MaxFiles {
			return Result{}, errCount
		}
		rc, err := f.Open()
		if err != nil {
			return Result{}, errNotZip
		}
		body, err := io.ReadAll(io.LimitReader(rc, int64(lim.MaxTotal-total)+1))
		rc.Close()
		if err != nil {
			return Result{}, errNotZip
		}
		total += len(body)
		if total > lim.MaxTotal {
			return Result{}, errTooLarge
		}
		res.Files[name] = body
	}
	if len(res.Files) == 0 {
		return Result{}, errCount
	}
	for d := range dropped {
		res.Dropped = append(res.Dropped, d)
	}
	sort.Strings(res.Dropped)
	if len(res.Dropped) > maxDropped {
		res.Dropped = res.Dropped[:maxDropped]
	}
	return res, nil
}

// junkPrefix is "a/node_modules/" when a folder in the path is a junk folder.
func junkPrefix(parts []string) string {
	for i, p := range parts[:len(parts)-1] {
		if junkDirs[strings.ToLower(p)] {
			return path.Join(parts[:i+1]...) + "/"
		}
	}
	return ""
}
