// Package recipes builds a container recipe for the kinds of app the Hub
// knows, so the creator does not have to write a Dockerfile. Recipes live in
// the Hub; they are never written into the app folder.
package recipes

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

const DockerfileName = ".apphub.Dockerfile"

type Recipe struct {
	Kind         string            `json:"kind"`
	Dockerfile   string            `json:"dockerfile"`
	Port         int               `json:"port"`
	Command      string            `json:"command"`
	NeedsConfirm bool              `json:"needs_confirm"`
	ExtraFiles   map[string]string `json:"-"`
	Requirements []string          `json:"requirements"`
	// Browser: the recipe brings Playwright's Chromium, its system libraries
	// and Xvfb (a virtual screen), because the Python app uses Playwright.
	Browser bool `json:"browser"`
}

// BrowserMemoryMB is what a screen app with the browser gets by default, and
// may use even where the company's cap per app is lower: Chromium does not
// run in less.
const BrowserMemoryMB = 512

var (
	exposeRe   = regexp.MustCompile(`(?mi)^\s*EXPOSE\s+(\d+)`)
	pyFromRe   = regexp.MustCompile(`^\s*from\s+([A-Za-z_][A-Za-z0-9_]*)`)
	pyImportRe = regexp.MustCompile(`^\s*import\s+(.+)`)
	pyNameRe   = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)`)
	skip       = map[string]bool{".git": true, ".venv": true, "venv": true, "node_modules": true, "__pycache__": true}
	// import name → pip name, for the common ones whose names differ.
	pipName = map[string]string{"sklearn": "scikit-learn", "cv2": "opencv-python", "PIL": "pillow", "yaml": "pyyaml", "dotenv": "python-dotenv", "docx": "python-docx", "bs4": "beautifulsoup4"}
)

type files struct {
	src  versions.Folder
	list []string // top two levels, slash paths
}

func list(src versions.Folder) (files, error) {
	f := files{src: src}
	err := src.Walk(func(rel string, dir bool, _ int64, _ time.Time) error {
		if dir {
			if skip[path.Base(rel)] || strings.Count(rel, "/") >= 2 {
				return fsSkipDir
			}
			return nil
		}
		f.list = append(f.list, rel)
		return nil
	})
	return f, err
}

func (f files) has(rel string) bool {
	for _, p := range f.list {
		if p == rel {
			return true
		}
	}
	return false
}

func (f files) read(rel string) string {
	r, err := f.src.Open(rel)
	if err != nil {
		return ""
	}
	defer r.Close()
	b, _ := io.ReadAll(io.LimitReader(r, 1<<20))
	return string(b)
}

// ErrCommandLines: the start command must be a single line.
var ErrCommandLines = errors.New(manifest.MsgCommandLines)

func Detect(src versions.Folder, m manifest.Manifest) (Recipe, error) {
	s := manifest.Shared{Enabled: true}
	if m.Shared != nil {
		s = *m.Shared
	}
	s = s.WithDefaults()
	if strings.ContainsAny(s.Command, "\r\n") { // would add lines to the generated Dockerfile
		return Recipe{}, ErrCommandLines
	}
	f, err := list(src)
	if err != nil {
		return Recipe{}, err
	}
	if f.has("Dockerfile") {
		r := Recipe{Kind: "dockerfile", Port: s.Port}
		if r.Port == 0 {
			if m := exposeRe.FindStringSubmatch(f.read("Dockerfile")); m != nil {
				r.Port, _ = strconv.Atoi(m[1])
			}
		}
		return r, nil
	}
	if f.has("package.json") {
		var pkg struct{ Scripts map[string]string }
		if json.Unmarshal([]byte(f.read("package.json")), &pkg) == nil && pkg.Scripts["start"] != "" {
			return nodeRecipe(s, f.has("package-lock.json")), nil
		}
	}
	reqText := f.read("requirements.txt")
	imports := f.pythonImports()
	libs := strings.ToLower(reqText)
	uses := func(lib string) bool { return strings.Contains(libs, lib) || imports[lib] }
	entry := m.Entrypoint
	switch {
	case uses("streamlit"):
		if entry == "" || !strings.HasSuffix(entry, ".py") {
			entry = f.fileImporting("streamlit")
		}
		r := pyBase(s, f, reqText, imports)
		r.Kind, r.Port = "streamlit", 8501
		r.Command = "streamlit run " + entry + " --server.port $PORT --server.address 0.0.0.0 --server.headless true"
		r.Dockerfile += r.cmd()
		return r, nil
	case uses("fastapi"), uses("flask"), uses("django"):
		r := pyBase(s, f, reqText, imports)
		r.Port = 8000
		mod := strings.TrimSuffix(path.Base(entry), ".py")
		if mod == "" {
			mod = "app"
		}
		switch {
		case uses("fastapi"):
			r.Kind, r.Command = "fastapi", "uvicorn "+mod+":app --host 0.0.0.0 --port $PORT"
		case uses("flask"):
			r.Kind, r.Command = "flask", "gunicorn -b 0.0.0.0:$PORT "+mod+":app"
		default:
			r.Kind, r.Command = "django", "gunicorn -b 0.0.0.0:$PORT "+f.djangoProject()+".wsgi"
		}
		r.NeedsConfirm = true
		if s.Command != "" {
			r.Command, r.NeedsConfirm = s.Command, false
		}
		server := "gunicorn"
		if r.Kind == "fastapi" {
			server = "uvicorn"
		}
		r.Dockerfile += "RUN pip install --no-cache-dir " + server + "\n" + r.cmd()
		return r, nil
	}
	return Recipe{}, nil
}

var assignRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// cmdLine is the Dockerfile CMD for a start command. Shell form keeps $PORT
// expanding; "exec" makes the app replace the shell as PID 1, so it gets
// Podman's stop signal and shuts down cleanly instead of being killed after
// 10 s (a plain sh does not pass the signal on). A command that already
// starts with exec, chains several commands or starts with VAR=x is kept as
// written: exec in front would break it. The command is a single line
// (Detect refuses line breaks).
func cmdLine(command string) string {
	c := strings.TrimSpace(command)
	if !strings.HasPrefix(c, "exec ") && !strings.ContainsAny(c, ";&|") && !assignRe.MatchString(c) {
		c = "exec " + c
	}
	return "CMD " + c + "\n"
}

// xvfbStart turns on the virtual screen the app names in DISPLAY (":99"),
// unless one is already there. An app that sets DISPLAY expects a screen; one
// that leaves it empty starts its own, or runs the invisible browser.
const xvfbStart = `if [ -n "$DISPLAY" ] && [ ! -e "/tmp/.X11-unix/X${DISPLAY#:}" ]; then Xvfb "$DISPLAY" -screen 0 1280x800x24 -nolisten tcp >/dev/null 2>&1 & fi; `

// cmd is the recipe's CMD line; with the browser it first turns on the
// virtual screen.
func (r Recipe) cmd() string {
	c := cmdLine(r.Command)
	if r.Browser {
		c = "CMD " + xvfbStart + strings.TrimPrefix(c, "CMD ")
	}
	return c
}

func pyBase(s manifest.Shared, f files, reqText string, imports map[string]bool) Recipe {
	r := Recipe{ExtraFiles: map[string]string{}, Browser: usesPlaywright(reqText, imports)}
	if strings.TrimSpace(reqText) == "" {
		local := map[string]bool{}
		for _, p := range f.list {
			if strings.HasSuffix(p, ".py") {
				local[strings.TrimSuffix(path.Base(p), ".py")] = true
			}
			if i := strings.Index(p, "/"); i > 0 {
				local[p[:i]] = true // top-level folder, e.g. a local package
			}
		}
		for imp := range imports {
			if !local[imp] && !stdlib[imp] {
				name := imp
				if n, ok := pipName[imp]; ok {
					name = n
				}
				r.Requirements = append(r.Requirements, name)
			}
		}
		sort.Strings(r.Requirements)
		r.ExtraFiles["requirements.txt"] = strings.Join(r.Requirements, "\n") + "\n"
	}
	if r.Browser {
		r.Dockerfile = fmt.Sprintf(browserBase, s.Python, playwrightVersion(reqText))
		return r
	}
	r.Dockerfile = fmt.Sprintf(`FROM docker.io/library/python:%s-slim
WORKDIR /app
ENV PYTHONUNBUFFERED=1
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
`, s.Python)
	return r
}

// PlaywrightVersion is the Playwright the browser layer installs when the app
// does not pin one. Pinned so the shared layer only changes when this does.
const PlaywrightVersion = "1.64.1"

// browserBase is the Python recipe with Playwright's Chromium. Bookworm,
// because Playwright installs the system libraries only on the Debian
// versions it supports. The browser layer comes before the app's own
// libraries and does not depend on them, so every app with the same
// Playwright shares it on disk (the build reuses an identical step). It
// installs the version the app pins (playwright==X), so a pinned app does not
// carry a second Chromium. After the app's libraries, "playwright install
// --with-deps chromium" does nothing new when the version is the same, and
// fetches the matching browser and its system libraries when a looser pin
// (>=, ~=) moved it. Xvfb is the virtual screen for sites that block the
// invisible browser; the app starts it itself when it wants it. libatomic1
// comes first: Playwright's own driver does not start without it.
const browserBase = `FROM docker.io/library/python:%s-slim-bookworm
WORKDIR /app
ENV PYTHONUNBUFFERED=1 PLAYWRIGHT_BROWSERS_PATH=/ms-playwright
RUN apt-get update \
 && apt-get install -y --no-install-recommends libatomic1 xvfb xauth \
 && pip install --no-cache-dir playwright==%s \
 && python -m playwright install --with-deps chromium \
 && rm -rf /var/lib/apt/lists/*
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt
RUN python -m playwright install --with-deps chromium \
 && rm -rf /var/lib/apt/lists/*
COPY . .
`

// reqPlaywrightRe: a requirements line for the playwright package itself,
// not for packages whose name only starts with it (playwright-stealth).
var (
	reqPlaywrightRe  = regexp.MustCompile(`(?mi)^[ \t]*playwright[ \t]*(?:[<>=!~;@\[#\r]|$)`)
	reqPlaywrightPin = regexp.MustCompile(`(?mi)^[ \t]*playwright[ \t]*==[ \t]*([0-9][0-9A-Za-z.]*)[ \t]*(?:[;#\r]|$)`)
)

// usesPlaywright: the Python app lists Playwright in requirements.txt or
// imports it.
func usesPlaywright(reqText string, imports map[string]bool) bool {
	return imports["playwright"] || reqPlaywrightRe.MatchString(reqText)
}

// playwrightVersion: the version the app pins, or PlaywrightVersion.
func playwrightVersion(reqText string) string {
	if m := reqPlaywrightPin.FindStringSubmatch(reqText); m != nil {
		return m[1]
	}
	return PlaywrightVersion
}

func nodeRecipe(s manifest.Shared, lock bool) Recipe {
	install := "npm ci"
	if !lock {
		install = "npm install"
	}
	df := fmt.Sprintf(`FROM docker.io/library/node:%s-slim
WORKDIR /app
COPY package*.json ./
RUN %s
COPY . .
RUN npm run build --if-present
CMD ["npm", "start"]
`, s.Node, install)
	return Recipe{Kind: "node", Port: 3000, Command: "npm start", Dockerfile: df}
}

// pythonImports lists top-level module names imported by .py files in the
// first two folder levels.
func (f files) pythonImports() map[string]bool {
	out := map[string]bool{}
	for _, p := range f.list {
		if !strings.HasSuffix(p, ".py") {
			continue
		}
		sc := bufio.NewScanner(strings.NewReader(f.read(p)))
		for sc.Scan() {
			line := sc.Text()
			if m := pyFromRe.FindStringSubmatch(line); m != nil {
				out[m[1]] = true
			} else if m := pyImportRe.FindStringSubmatch(line); m != nil {
				// "import os, numpy.linalg as la": first name of each item
				for _, item := range strings.Split(m[1], ",") {
					if n := pyNameRe.FindStringSubmatch(item); n != nil {
						out[n[1]] = true
					}
				}
			}
		}
	}
	return out
}

func (f files) fileImporting(mod string) string {
	for _, p := range f.list {
		if strings.HasSuffix(p, ".py") && regexp.MustCompile(`(?m)^\s*(import|from)\s+`+mod+`\b`).MatchString(f.read(p)) {
			return p
		}
	}
	return "app.py"
}

func (f files) djangoProject() string {
	for _, p := range f.list {
		if path.Base(p) == "wsgi.py" {
			return path.Dir(p)
		}
	}
	return "config"
}

var fsSkipDir = fs.SkipDir

// stdlib lists common Python standard-library modules, so they are not
// written into a generated requirements list.
var stdlib = func() map[string]bool {
	m := map[string]bool{}
	for _, n := range strings.Fields(`abc aifc antigravity argparse array ast asyncio atexit audioop base64 bdb binascii bisect builtins bz2 cProfile calendar cgi cgitb chunk cmath cmd code codecs codeop collections colorsys compileall concurrent configparser contextlib contextvars copy copyreg crypt csv ctypes curses dataclasses datetime dbm decimal difflib dis doctest email encodings ensurepip enum errno faulthandler fcntl filecmp fileinput fnmatch fractions ftplib functools gc genericpath getopt getpass gettext glob graphlib grp gzip hashlib heapq hmac html http idlelib imaplib imghdr importlib inspect io ipaddress itertools json keyword lib2to3 linecache locale logging lzma mailbox mailcap marshal math mimetypes mmap modulefinder msilib msvcrt multiprocessing netrc nis nntplib nt ntpath nturl2path numbers opcode operator optparse os ossaudiodev pathlib pdb pickle pickletools pipes pkgutil platform plistlib poplib posix posixpath pprint profile pstats pty pwd py_compile pyclbr pydoc pydoc_data pyexpat queue quopri random re readline reprlib resource rlcompleter runpy sched secrets select selectors shelve shlex shutil signal site smtplib sndhdr socket socketserver spwd sqlite3 sre_compile sre_constants sre_parse ssl stat statistics string stringprep struct subprocess sunau symtable sys sysconfig syslog tabnanny tarfile telnetlib tempfile termios textwrap this threading time timeit tkinter token tokenize tomllib trace traceback tracemalloc tty turtle turtledemo types typing unicodedata unittest urllib uu uuid venv warnings wave weakref webbrowser winreg winsound wsgiref xdrlib xml xmlrpc zipapp zipfile zipimport zlib zoneinfo __future__`) {
		m[n] = true
	}
	return m
}()

// Local is how the Agente starts a screen app on a person's computer: the
// same recipe as the 24h version, listening only on the computer itself.
type Local struct {
	Runtime      string   // "python" or "node"
	Args         []string // after the Python interpreter, or npm's arguments; "{PORT}" is the port
	DepsFile     string   // requirements.txt, when the app has one
	Requirements []string // libraries to install besides it
}

// ErrNoLocal: the app has no recipe the Agente can start on a computer.
var ErrNoLocal = errors.New("O Ziv ainda não sabe ligar este app no computador: ele precisa ser Streamlit, FastAPI, Flask, Django ou Node com \"npm start\".")

// LocalStart turns a recipe into a local start. gunicorn does not run on
// Windows, so a gunicorn command runs under waitress, which takes the same
// "module:app" target.
func LocalStart(r Recipe) (Local, error) {
	if r.Kind == "node" {
		return Local{Runtime: "node", Args: []string{"start"}}, nil
	}
	fields := strings.Fields(r.Command)
	if r.Kind == "" || r.Kind == "dockerfile" || len(fields) == 0 {
		return Local{}, ErrNoLocal
	}
	l := Local{Runtime: "python", Requirements: append([]string{}, r.Requirements...)}
	if len(r.Requirements) == 0 {
		l.DepsFile = "requirements.txt"
	}
	for i, f := range fields {
		f = strings.ReplaceAll(f, "0.0.0.0", "127.0.0.1")
		fields[i] = strings.ReplaceAll(f, "$PORT", "{PORT}")
	}
	switch tool := fields[0]; tool {
	case "python", "python3":
		l.Args = fields[1:]
	case "streamlit", "uvicorn", "flask":
		l.Args = append([]string{"-m"}, fields...)
		if tool == "uvicorn" {
			l.Requirements = append(l.Requirements, "uvicorn")
		}
	case "gunicorn":
		target := fields[len(fields)-1]
		if strings.HasPrefix(target, "-") {
			return Local{}, ErrNoLocal
		}
		if !strings.Contains(target, ":") {
			target += ":application" // Django's wsgi module
		}
		l.Args = []string{"-m", "waitress", "--listen=127.0.0.1:{PORT}", target}
		l.Requirements = append(l.Requirements, "waitress")
	default:
		return Local{}, ErrNoLocal
	}
	if len(l.Args) == 0 {
		return Local{}, ErrNoLocal
	}
	return l, nil
}
