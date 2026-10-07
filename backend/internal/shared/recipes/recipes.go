// Package recipes builds a container recipe for the kinds of app the Hub
// knows, so the creator does not have to write a Dockerfile. Recipes live in
// the Hub; they are never written into the app folder.
package recipes

import (
	"bufio"
	"encoding/json"
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
}

var (
	exposeRe   = regexp.MustCompile(`(?mi)^\s*EXPOSE\s+(\d+)`)
	pyImportRe = regexp.MustCompile(`(?m)^\s*(?:from|import)\s+([A-Za-z_][A-Za-z0-9_]*)`)
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

func Detect(src versions.Folder, m manifest.Manifest) (Recipe, error) {
	s := manifest.Shared{Enabled: true}
	if m.Shared != nil {
		s = *m.Shared
	}
	s = s.WithDefaults()
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
		r.Dockerfile += "CMD " + r.Command + "\n"
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
		r.Dockerfile += "RUN pip install --no-cache-dir " + server + "\nCMD " + r.Command + "\n"
		return r, nil
	}
	return Recipe{}, nil
}

func pyBase(s manifest.Shared, f files, reqText string, imports map[string]bool) Recipe {
	r := Recipe{ExtraFiles: map[string]string{}}
	if strings.TrimSpace(reqText) == "" {
		local := map[string]bool{}
		for _, p := range f.list {
			if strings.HasSuffix(p, ".py") {
				local[strings.TrimSuffix(path.Base(p), ".py")] = true
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
	r.Dockerfile = fmt.Sprintf(`FROM docker.io/library/python:%s-slim
WORKDIR /app
ENV PYTHONUNBUFFERED=1
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
`, s.Python)
	return r
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
			if m := pyImportRe.FindStringSubmatch(sc.Text()); m != nil {
				out[m[1]] = true
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
	for _, n := range strings.Fields(`os sys re json csv datetime time math pathlib typing collections itertools functools subprocess logging sqlite3 io shutil glob random uuid hashlib base64 threading dataclasses enum urllib http email zipfile tempfile argparse decimal statistics string textwrap traceback copy pickle unittest asyncio socket ssl smtplib xml html calendar locale platform signal struct queue contextlib abc inspect warnings operator secrets getpass configparser pprint`) {
		m[n] = true
	}
	return m
}()
