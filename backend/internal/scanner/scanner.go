// Package scanner diagnoses an app folder with plain rules (no AI): language,
// kind, entrypoint, dependencies, inputs, outputs and secrets. The result is a
// draft manifest plus the questions a person must confirm.
package scanner

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

const (
	maxFiles     = 4000
	maxDepth     = 4
	maxCodeBytes = 512 << 10
	maxConfBytes = 256 << 10
)

type Question struct {
	Field string `json:"field"`
	Text  string `json:"text"`
}

type Diagnosis struct {
	FolderPath        string            `json:"folder_path"`
	FolderName        string            `json:"folder_name"`
	Manifest          manifest.Manifest `json:"manifest"`
	Findings          []string          `json:"findings"`
	Questions         []Question        `json:"questions"`
	EntrypointOptions []string          `json:"entrypoint_options"`
	Secrets           []SecretFinding   `json:"secrets"`
	FileCount         int               `json:"file_count"`
	Truncated         bool              `json:"truncated"`
	HasManifest       bool              `json:"has_manifest"`
	SuggestSkip       bool              `json:"suggest_skip"`
}

var skipDirs = map[string]bool{
	".git": true, "node_modules": true, ".venv": true, "venv": true, "env": true, "__pycache__": true,
	".mypy_cache": true, ".pytest_cache": true, "site-packages": true, ".idea": true, ".vscode": true,
	".claude": true, "build": true, ".next": true, "target": true,
}

var (
	codeLang = map[string]string{
		".py": "python", ".pyw": "python", ".ipynb": "python", ".js": "javascript", ".mjs": "javascript",
		".ts": "typescript", ".tsx": "typescript", ".go": "go", ".ps1": "powershell", ".bat": "batch",
		".cmd": "batch", ".vbs": "vbscript", ".r": "r",
	}
	confExt = map[string]bool{".json": true, ".env": true, ".ini": true, ".cfg": true, ".toml": true, ".yaml": true, ".yml": true, ".txt": true}
	docExt  = map[string]bool{".docx": true, ".doc": true, ".pdf": true, ".xlsx": true, ".xlsm": true, ".xls": true, ".pptx": true, ".md": true, ".url": true, ".html": true, ".csv": true, ".txt": true}
)

type file struct {
	rel   string // slash-separated, relative to the app folder
	ext   string
	size  int64
	depth int
}

func listFiles(dir string) ([]file, bool) {
	var files []file
	truncated := false
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		depth := strings.Count(rel, "/")
		if d.IsDir() {
			if skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".") || depth >= maxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ":Zone.Identifier") || strings.HasPrefix(d.Name(), "~$") {
			return nil
		}
		if len(files) >= maxFiles {
			truncated = true
			return filepath.SkipAll
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		files = append(files, file{rel: rel, ext: strings.ToLower(path.Ext(rel)), size: info.Size(), depth: depth})
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })
	return files, truncated
}

var (
	reImport      = regexp.MustCompile(`(?m)^\s*(?:from\s+([A-Za-z_][\w\.]*)\s+import\b|import\s+([A-Za-z_][\w\.]*(?:\s*,\s*[A-Za-z_][\w\.]*)*))`)
	reMainGuard   = regexp.MustCompile(`if\s+__name__\s*==\s*["']__main__["']`)
	reDialogFile  = regexp.MustCompile(`askopenfilenames?\s*\(`)
	reDialogDir   = regexp.MustCompile(`askdirectory\s*\(`)
	reMainloop    = regexp.MustCompile(`\.mainloop\s*\(|\.exec_?\s*\(\s*\)|webview\.start\s*\(`)
	reWithdraw    = regexp.MustCompile(`\.withdraw\s*\(`)
	reInputCall   = regexp.MustCompile(`(?:^|[^\w\.])input\s*\(\s*(?:[fr]?["']([^"'\n]*)["'])?`)
	reArgument    = regexp.MustCompile(`add_argument\(\s*["'](-{0,2}[\w\-]+)["'](?:[^)]*help\s*=\s*["']([^"']+)["'])?`)
	reTitle       = regexp.MustCompile(`title\s*=\s*["']([^"']+)["']`)
	reFileLiteral = regexp.MustCompile(`["']([^"'\n]{1,200}?\.(?i:csv|xlsx|xlsm|xls|json|txt|pdf|docx|parquet|html))["']`)
	reWriteHint   = regexp.MustCompile(`to_excel|to_csv|to_parquet|ExcelWriter|\.save\s*\(|savefig|write_text|open\([^)]*["'](?:w|wb|a|w\+)["']|shutil\.copy`)
	reOutVar      = regexp.MustCompile(`(?i)^\s*\w*(out|saida|output|result|destino|dest|relatorio|report)\w*\s*=`)
	reServerPy    = regexp.MustCompile(`app\.run\s*\(|uvicorn\.run|HTTPServer\s*\(|serve_forever|\.launch\s*\(`)
	rePort        = regexp.MustCompile(`(?i)port\s*[=:]\s*(\d{4,5})`)
	reSchtasks    = regexp.MustCompile(`(?i)schtasks`)
	reSchedule    = regexp.MustCompile(`(?i)/SC\s+(\w+)`)
	reScheduleMo  = regexp.MustCompile(`(?i)/MO\s+(\d+)`)
	reScheduleSt  = regexp.MustCompile(`(?i)/ST\s+(\d{1,2}:\d{2})`)
	reURLFile     = regexp.MustCompile(`(?m)^URL=(.+)$`)
	reDocstring   = regexp.MustCompile(`^\s*(?:#[^\n]*\n\s*)*(?:"""|''')\s*([^\n]+?)\s*(?:"""|''')?\s*(?:\n|$)`)
	reArchive     = regexp.MustCompile(`(?i)(^|[^a-z])(old|antigo|antigos|arquivo|arquivado|backup|backups|lixo)([^a-z]|$)`)
	reDatePrefix  = regexp.MustCompile(`^\d{4}[\s._-]\d{2}[\s._-]\d{2}\s*[-–]?\s*`)
	reLangPrefix  = regexp.MustCompile(`(?i)^python\s+`)
	rePlaceholder = regexp.MustCompile(`\{[^}]*\}`)
)

var webModules = map[string]bool{"streamlit": true, "flask": true, "fastapi": true, "dash": true, "gradio": true, "django": true, "uvicorn": true, "aiohttp": true}
var guiModules = map[string]bool{"tkinter": true, "customtkinter": true, "PyQt5": true, "PyQt6": true, "PySide6": true, "PySide2": true, "wx": true, "webview": true, "kivy": true, "flet": true}

var pipNames = map[string]string{
	"cv2": "opencv-python", "PIL": "pillow", "sklearn": "scikit-learn", "fitz": "pymupdf", "docx": "python-docx",
	"yaml": "pyyaml", "bs4": "beautifulsoup4", "dotenv": "python-dotenv", "win32com": "pywin32", "pptx": "python-pptx",
	"dateutil": "python-dateutil", "webview": "pywebview",
}

type pySignals struct {
	rel         string
	imports     map[string]bool
	mainGuard   bool
	dialogFile  []string
	dialogDir   []string
	mainloop    bool
	withdraw    bool
	prompts     []string
	arguments   [][2]string
	server      bool
	port        string
	docstring   string
	fileInputs  []string
	fileOutputs []string
}

func analyzePython(rel, text string) pySignals {
	s := pySignals{rel: rel, imports: map[string]bool{}}
	for _, m := range reImport.FindAllStringSubmatch(text, -1) {
		mods := m[1]
		if mods == "" {
			mods = m[2]
		}
		for _, mod := range strings.Split(mods, ",") {
			mod = strings.TrimSpace(strings.Fields(mod + " ")[0])
			if mod != "" {
				s.imports[strings.Split(mod, ".")[0]] = true
			}
		}
	}
	s.mainGuard = reMainGuard.MatchString(text)
	s.mainloop = reMainloop.MatchString(text)
	s.withdraw = reWithdraw.MatchString(text)
	s.server = reServerPy.MatchString(text)
	if m := rePort.FindStringSubmatch(text); m != nil {
		s.port = m[1]
	}
	if m := reDocstring.FindStringSubmatch(text); m != nil {
		s.docstring = strings.TrimSpace(m[1])
	}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		title := ""
		if m := reTitle.FindStringSubmatch(line); m != nil {
			title = m[1]
		}
		if reDialogFile.MatchString(line) {
			s.dialogFile = append(s.dialogFile, title)
		}
		if reDialogDir.MatchString(line) {
			s.dialogDir = append(s.dialogDir, title)
		}
		if m := reInputCall.FindStringSubmatch(line); m != nil && !strings.Contains(line, "def input") {
			s.prompts = append(s.prompts, strings.TrimSpace(m[1]))
		}
		if m := reArgument.FindStringSubmatch(line); m != nil {
			s.arguments = append(s.arguments, [2]string{m[1], m[2]})
		}
		for _, lm := range reFileLiteral.FindAllStringSubmatch(stripFStringFields(line), -1) {
			lit := lm[1]
			if strings.Contains(lit, "://") {
				continue
			}
			if rePlaceholder.MatchString(lit) || reWriteHint.MatchString(line) || reOutVar.MatchString(line) {
				s.fileOutputs = append(s.fileOutputs, lit)
			} else {
				s.fileInputs = append(s.fileInputs, lit)
			}
		}
	}
	return s
}

// Scan diagnoses one app folder.
func Scan(dir string) (Diagnosis, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return Diagnosis{}, err
	}
	if !info.IsDir() {
		return Diagnosis{}, fs.ErrInvalid
	}
	d := Diagnosis{FolderPath: dir, FolderName: filepath.Base(dir)}
	if reArchive.MatchString(d.FolderName) {
		d.SuggestSkip = true
		d.Findings = append(d.Findings, "O nome da pasta sugere material antigo ou de arquivo. Sugiro não registrar.")
	}
	files, truncated := listFiles(dir)
	d.FileCount, d.Truncated = len(files), truncated
	if truncated {
		d.Findings = append(d.Findings, "A pasta é muito grande; analisei só os primeiros arquivos.")
	}

	if data, err := os.ReadFile(manifest.Locate(dir)); err == nil {
		if m, err := manifest.Parse(data); err == nil && m.Name != "" {
			d.Manifest = m
			d.HasManifest = true
			d.Findings = append(d.Findings, "Esta pasta já tem um manifesto do Ziv. Usei o que está nele.")
			return d, nil
		}
	}

	byRel := map[string]file{}
	baseNames := map[string]bool{}
	langCount := map[string]int{}
	var pyFiles []pySignals
	var launchers []file
	var exes []file
	var docs []file
	localModules := map[string]bool{}
	var readme, requirements, packageJSON, goMod string
	hasDocker, hasFrontendDir, hasBackendDir := false, false, false
	schedule := ""
	var urlLink string

	for _, f := range files {
		byRel[f.rel] = f
		base := path.Base(f.rel)
		baseNames[strings.ToLower(base)] = true
		if f.depth == 0 && (strings.HasPrefix(f.rel, "frontend/") || f.rel == "frontend") {
			hasFrontendDir = true
		}
		top := strings.Split(f.rel, "/")[0]
		if top == "frontend" {
			hasFrontendDir = true
		}
		if top == "backend" {
			hasBackendDir = true
		}
		if lang, ok := codeLang[f.ext]; ok {
			langCount[lang]++
		}
		if f.ext == ".py" || f.ext == ".pyw" {
			localModules[strings.TrimSuffix(base, f.ext)] = true
			if dirPart := path.Dir(f.rel); dirPart != "." {
				localModules[strings.Split(dirPart, "/")[0]] = true
			}
		}
		switch {
		case f.ext == ".exe" && f.depth <= 2:
			exes = append(exes, f)
		case f.ext == ".bat" || f.ext == ".cmd" || f.ext == ".vbs" || f.ext == ".ps1":
			if f.depth == 0 {
				launchers = append(launchers, f)
			}
		case docExt[f.ext] && f.depth == 0:
			docs = append(docs, f)
		}
		if f.depth == 0 {
			switch strings.ToLower(base) {
			case "readme.md", "leia-me.md", "leiame.md":
				readme = readFile(dir, f, maxConfBytes)
			case "requirements.txt":
				requirements = readFile(dir, f, maxConfBytes)
			case "package.json":
				packageJSON = readFile(dir, f, maxConfBytes)
			case "go.mod":
				goMod = "yes"
			case "dockerfile", "docker-compose.yml", "compose.yaml":
				hasDocker = true
			}
		}
		if f.rel == "backend/go.mod" {
			goMod = "yes"
		}
	}
	if requirements == "" {
		for _, f := range files {
			if path.Base(f.rel) == "requirements.txt" && f.depth <= 1 {
				requirements = readFile(dir, f, maxConfBytes)
				break
			}
		}
	}

	// Content analysis: code (for signals and secrets) and small config files (secrets only).
	for _, f := range files {
		_, isCode := codeLang[f.ext]
		switch {
		case isCode && f.size <= maxCodeBytes && f.depth <= 3:
			text := readFile(dir, f, maxCodeBytes)
			d.Secrets = append(d.Secrets, findSecrets(f.rel, text)...)
			if f.ext == ".py" || f.ext == ".pyw" {
				pyFiles = append(pyFiles, analyzePython(f.rel, text))
			}
		case confExt[f.ext] && f.size <= maxConfBytes && f.depth <= 2 && path.Base(f.rel) != "requirements.txt":
			d.Secrets = append(d.Secrets, findSecrets(f.rel, readFile(dir, f, maxConfBytes))...)
		}
		if f.ext == ".url" && f.depth == 0 && urlLink == "" {
			if m := reURLFile.FindStringSubmatch(readFile(dir, f, 4096)); m != nil {
				urlLink = strings.TrimSpace(m[1])
			}
		}
	}

	// Launchers (.bat/.vbs/.ps1): which .py they start, pythonw, schtasks, streamlit.
	launcherTargets := map[string]string{} // py rel -> launcher rel
	usesPythonw, usesStreamlitRun := false, false
	for _, l := range launchers {
		text := readFile(dir, l, maxCodeBytes)
		lower := strings.ToLower(text)
		if strings.Contains(lower, "pythonw") {
			usesPythonw = true
		}
		if strings.Contains(lower, "streamlit run") {
			usesStreamlitRun = true
		}
		if reSchtasks.MatchString(text) {
			schedule = describeSchedule(text)
		}
		for _, py := range pyFiles {
			if _, ok := launcherTargets[py.rel]; !ok && mentionsFile(lower, py.rel) {
				launcherTargets[py.rel] = l.rel
			}
		}
	}

	m := manifest.Manifest{ManifestVersion: manifest.Version, Name: suggestName(d.FolderName)}
	m.ID = manifest.Slugify(m.Name)
	m.Language = dominantLanguage(langCount)
	m.Runtime.Language = m.Language
	m.Schedule = schedule
	m.Summary = summaryFrom(readme)

	allImports := map[string]bool{}
	for _, py := range pyFiles {
		for k := range py.imports {
			allImports[k] = true
		}
	}
	codeFiles := 0
	for lang, n := range langCount {
		if lang != "batch" && lang != "vbscript" && lang != "powershell" {
			codeFiles += n
		}
	}

	var questions []Question
	var notes []string
	var findings []string

	if m.Language != "" {
		findings = append(findings, "Linguagem principal: "+langLabel(m.Language)+".")
	}

	switch {
	case goMod != "" && (hasFrontendDir || hasBackendDir || hasDocker):
		m.Kind = manifest.KindSystem
		findings = append(findings, "Parece um sistema com várias partes (servidor, tela, banco de dados). Por enquanto o Ziv só registra e documenta.")
	case codeFiles == 0 && len(exes) > 0:
		m.Kind = manifest.KindExecutable
		m.Entrypoint = pickExe(exes)
		m.Run.Command = `"` + m.Entrypoint + `"`
		findings = append(findings, "Encontrei um executável pronto: "+m.Entrypoint+".")
	case codeFiles == 0 && len(launchers) == 0:
		m.Kind = manifest.KindReference
		if urlLink != "" {
			m.Run.URL = urlLink
			findings = append(findings, "É um atalho para um endereço: "+urlLink+".")
		} else {
			findings = append(findings, "Não encontrei código, só documentos. Ele fica catalogado, sem botão Rodar.")
		}
		if m.Summary == "" && len(docs) > 0 {
			names := []string{}
			for _, doc := range docs {
				names = append(names, path.Base(doc.rel))
			}
			m.Summary = "Documentos: " + strings.Join(names, ", ") + "."
		}
	case len(pyFiles) > 0:
		diagnosePython(&m, pyFiles, launcherTargets, allImports, usesPythonw, usesStreamlitRun, baseNames, &d, &questions, &notes, &findings)
		if len(exes) > 0 {
			exe := pickExe(exes)
			findings = append(findings, "Também existe um executável pronto: "+exe+".")
			questions = append(questions, Question{Field: "kind", Text: "O app tem o código e também um executável pronto (" + exe + "). Se o Ziv deve abrir o executável, mude o tipo para \"Executável pronto\" e o arquivo de início para " + exe + "."})
		}
	case packageJSON != "":
		m.Kind = manifest.KindWeb
		m.Entrypoint = "package.json"
		m.Run.Command = "npm start"
		m.Runtime.DependenciesFile = "package.json"
		findings = append(findings, "É um projeto Node.js (package.json).")
		questions = append(questions, Question{Field: "kind", Text: "Confirme o tipo: projetos Node.js costumam abrir no navegador, mas podem ser só scripts."})
	default:
		m.Kind = manifest.KindScript
		if len(launchers) > 0 {
			m.Entrypoint = launchers[0].rel
			m.Run.Command = `"` + launchers[0].rel + `"`
		}
		questions = append(questions, Question{Field: "kind", Text: "Não reconheci bem este app. Confirme o tipo e o arquivo que inicia."})
	}

	if schedule != "" {
		findings = append(findings, "Existe um agendamento do Windows: "+schedule+".")
		if m.Kind != manifest.KindSystem && m.Kind != manifest.KindReference {
			m.Kind = manifest.KindScheduled
		}
	}

	if m.Language == "python" {
		if requirements != "" {
			m.Runtime.DependenciesFile = "requirements.txt"
			m.Runtime.Dependencies = parseRequirements(requirements)
		} else {
			m.Runtime.Dependencies = thirdParty(allImports, localModules)
			if len(m.Runtime.Dependencies) > 0 {
				notes = append(notes, "Não há lista de bibliotecas (requirements.txt). O Ziv deduziu pelas importações do código.")
			}
		}
		if len(m.Runtime.Dependencies) > 0 {
			findings = append(findings, "Bibliotecas usadas: "+strings.Join(m.Runtime.Dependencies, ", ")+".")
		}
	}

	if len(d.Secrets) > 0 {
		seen := map[string]bool{}
		for _, s := range d.Secrets {
			m.Secrets = append(m.Secrets, manifest.SecretRef{Name: s.Name, File: s.File, Line: s.Line, Hint: s.Hint})
			seen[s.File] = true
		}
		files := make([]string, 0, len(seen))
		for f := range seen {
			files = append(files, f)
		}
		sort.Strings(files)
		notes = append(notes, "Há chave ou senha escrita em: "+strings.Join(files, ", ")+". O ideal é guardar no cofre do Ziv.")
	}

	if m.Summary == "" {
		questions = append(questions, Question{Field: "summary", Text: "Escreva em uma frase o que o app faz, para quem nunca usou."})
	}
	questions = append(questions, Question{Field: "area", Text: "Em qual área da empresa este app é usado?"})

	d.Manifest = m
	d.Manifest.Notes = notes
	d.Findings = append(d.Findings, findings...)
	d.Questions = questions
	return d, nil
}

func diagnosePython(m *manifest.Manifest, pyFiles []pySignals, launcherTargets map[string]string, allImports map[string]bool,
	usesPythonw, usesStreamlitRun bool, baseNames map[string]bool, d *Diagnosis, questions *[]Question, notes, findings *[]string) {

	isWeb := usesStreamlitRun
	for k := range allImports {
		if webModules[k] {
			isWeb = true
		}
	}
	for _, py := range pyFiles {
		if py.server {
			isWeb = true
		}
	}
	hasGUI := false
	for k := range allImports {
		if guiModules[k] {
			hasGUI = true
		}
	}

	// Pick the entrypoint by score.
	type cand struct {
		rel   string
		score int
	}
	var cands []cand
	for _, py := range pyFiles {
		base := strings.ToLower(strings.TrimSuffix(path.Base(py.rel), path.Ext(py.rel)))
		score := 0
		if !strings.Contains(py.rel, "/") {
			score += 3
		}
		if py.mainGuard {
			score += 3
		}
		switch base {
		case "main", "app", "server", "run", "start", "iniciar", "home":
			score += 2
		}
		if _, ok := launcherTargets[py.rel]; ok {
			score += 10
		}
		if isWeb && (py.imports["streamlit"] || py.server) {
			score += 2
		}
		if strings.HasPrefix(base, "_") || strings.HasPrefix(base, "test") || base == "setup" || base == "__init__" {
			score -= 5
		}
		cands = append(cands, cand{py.rel, score})
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].score > cands[j].score })
	entry := cands[0].rel
	for _, c := range cands {
		if c.score >= cands[0].score-1 && c.score > 0 {
			d.EntrypointOptions = append(d.EntrypointOptions, c.rel)
		}
	}
	if len(d.EntrypointOptions) > 1 {
		*questions = append(*questions, Question{Field: "entrypoint", Text: "Encontrei mais de um arquivo que pode iniciar o app. Confirme qual é o certo."})
	} else {
		d.EntrypointOptions = []string{entry}
	}
	m.Entrypoint = entry
	var sig pySignals
	for _, py := range pyFiles {
		if py.rel == entry {
			sig = py
		}
	}
	if l, ok := launcherTargets[entry]; ok {
		*findings = append(*findings, "O app é iniciado pelo atalho "+l+".")
	}
	if m.Summary == "" && sig.docstring != "" && len(sig.docstring) > 12 {
		m.Summary = sig.docstring
	}

	// Collect all dialogs and inputs across files (small apps split code in modules).
	var dialogFiles, dialogDirs, prompts []string
	var args [][2]string
	var fixedInputs, outputs []string
	anyMainloop, anyWithdraw := false, false
	for _, py := range pyFiles {
		dialogFiles = append(dialogFiles, py.dialogFile...)
		dialogDirs = append(dialogDirs, py.dialogDir...)
		prompts = append(prompts, py.prompts...)
		args = append(args, py.arguments...)
		fixedInputs = append(fixedInputs, py.fileInputs...)
		outputs = append(outputs, py.fileOutputs...)
		anyMainloop = anyMainloop || py.mainloop
		anyWithdraw = anyWithdraw || py.withdraw
	}

	switch {
	case isWeb:
		m.Kind = manifest.KindWeb
		port := sig.port
		if allImports["streamlit"] {
			m.Run.Command = `streamlit run "` + entry + `"`
			if port == "" {
				port = "8501"
			}
		} else {
			m.Run.Command = `python "` + entry + `"`
		}
		if port != "" {
			m.Run.URL = "http://localhost:" + port
		}
		*findings = append(*findings, "Abre uma tela no navegador.")
	case hasGUI && (anyMainloop || usesPythonw) && !(anyWithdraw && len(dialogFiles)+len(dialogDirs) > 0 && !anyMainloop):
		m.Kind = manifest.KindDesktop
		m.Run.Command = `pythonw "` + entry + `"`
		*findings = append(*findings, "Abre uma janela própria no Windows.")
	default:
		m.Kind = manifest.KindScript
		m.Run.Command = `python "` + entry + `"`
		*findings = append(*findings, "Roda do começo ao fim e entrega um resultado.")
	}

	if m.Kind == manifest.KindScript {
		used := map[string]bool{}
		addInput := func(in manifest.Input) {
			base := in.ID
			for i := 2; used[in.ID]; i++ {
				in.ID = base + "_" + string(rune('0'+i))
			}
			used[in.ID] = true
			m.Inputs = append(m.Inputs, in)
		}
		for _, t := range dialogFiles {
			label := t
			if label == "" {
				label = "Arquivo de entrada"
			}
			addInput(manifest.Input{ID: "arquivo", Label: label, Type: "file", Source: "dialog", Required: true})
		}
		for _, t := range dialogDirs {
			label := t
			if label == "" {
				label = "Pasta de entrada"
			}
			addInput(manifest.Input{ID: "pasta", Label: label, Type: "folder", Source: "dialog", Required: true})
		}
		if len(dialogFiles)+len(dialogDirs) > 0 {
			*notes = append(*notes, "A entrada depende de uma janela de seleção do Windows. Para rodar pelo Ziv, o Ziv vai entregar o arquivo no lugar da janela.")
		}
		for _, p := range prompts {
			label := p
			if label == "" {
				label = "Resposta digitada"
			}
			addInput(manifest.Input{ID: "resposta", Label: strings.TrimRight(label, ": "), Type: "text", Source: "prompt", Required: true})
		}
		for _, a := range args {
			label := a[1]
			if label == "" {
				label = strings.TrimLeft(a[0], "-")
			}
			addInput(manifest.Input{ID: strings.ReplaceAll(strings.TrimLeft(a[0], "-"), "-", "_"), Label: label, Type: "text", Source: "argument", Required: !strings.HasPrefix(a[0], "-")})
		}
		seenFixed := map[string]bool{}
		for _, lit := range outputs {
			// A file the app writes is not an input, even if a copy already exists.
			seenFixed[strings.ToLower(path.Base(strings.ReplaceAll(lit, `\`, "/")))] = true
		}
		written := len(seenFixed)
		for _, lit := range fixedInputs {
			base := path.Base(strings.ReplaceAll(lit, `\`, "/"))
			key := strings.ToLower(base)
			if seenFixed[key] || !baseNames[key] {
				continue
			}
			seenFixed[key] = true
			addInput(manifest.Input{ID: inputID(base), Label: "Arquivo " + base, Type: "file", Source: "fixed_file", Path: base, Required: true})
			*questions = append(*questions, Question{Field: "inputs", Text: "O app lê o arquivo " + base + " da própria pasta. Ele muda a cada uso (é uma entrada)? Se for fixo do app, remova da lista de entradas."})
		}
		if len(seenFixed) > written {
			*notes = append(*notes, "O app lê um arquivo com nome fixo na própria pasta.")
		}
	}

	seenOut := map[string]bool{}
	for _, lit := range outputs {
		pattern := strings.ReplaceAll(rePlaceholder.ReplaceAllString(path.Base(strings.ReplaceAll(lit, `\`, "/")), "*"), "**", "*")
		if pattern == "" || seenOut[pattern] {
			continue
		}
		seenOut[pattern] = true
		m.Outputs = append(m.Outputs, manifest.Output{Label: outputLabel(pattern), Pattern: pattern})
	}
	if len(m.Outputs) > 0 && m.Kind == manifest.KindScript {
		*notes = append(*notes, "O app grava os resultados na própria pasta. No Ziv, cada execução guarda seus resultados separadamente.")
	}
	if m.Kind == manifest.KindScript && len(m.Outputs) == 0 {
		*questions = append(*questions, Question{Field: "outputs", Text: "Não identifiquei os arquivos que o app gera. Se ele gera algum, adicione em \"O que ele devolve\"."})
	}
}

// mentionsFile reports whether a launcher script (lowercased) refers to rel,
// either by its relative path (with / or \\) or by its bare file name.
func mentionsFile(lower, rel string) bool {
	rel = strings.ToLower(rel)
	for _, cand := range []string{rel, strings.ReplaceAll(rel, "/", `\`), path.Base(rel)} {
		for i := strings.Index(lower, cand); i >= 0; {
			before := byte(' ')
			if i > 0 {
				before = lower[i-1]
			}
			end := i + len(cand)
			after := byte(' ')
			if end < len(lower) {
				after = lower[end]
			}
			if strings.IndexByte(" \t\"'/\\=", before) >= 0 && strings.IndexByte(" \t\r\n\"'", after) >= 0 {
				return true
			}
			next := strings.Index(lower[i+1:], cand)
			if next < 0 {
				break
			}
			i += next + 1
		}
	}
	return false
}

// stripFStringFields replaces {…} fields inside f-strings with "*" so that
// f"relatorio_{datetime.now().strftime('%Y')}.xlsx" becomes "relatorio_*.xlsx".
func stripFStringFields(line string) string {
	if !strings.Contains(line, `f"`) && !strings.Contains(line, `f'`) {
		return line
	}
	var b strings.Builder
	depth := 0
	for _, r := range line {
		switch {
		case r == '{':
			if depth == 0 {
				b.WriteRune('*')
			}
			depth++
		case r == '}' && depth > 0:
			depth--
		case depth == 0:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func readFile(dir string, f file, limit int) string {
	fh, err := os.Open(filepath.Join(dir, filepath.FromSlash(f.rel)))
	if err != nil {
		return ""
	}
	defer fh.Close()
	buf := make([]byte, limit)
	n, _ := fh.Read(buf)
	return strings.ToValidUTF8(string(buf[:n]), "")
}

func describeSchedule(text string) string {
	sc := ""
	if m := reSchedule.FindStringSubmatch(text); m != nil {
		sc = strings.ToUpper(m[1])
	}
	mo := ""
	if m := reScheduleMo.FindStringSubmatch(text); m != nil {
		mo = m[1]
	}
	at := ""
	if m := reScheduleSt.FindStringSubmatch(text); m != nil {
		at = " às " + m[1]
	}
	every := func(unit string) string {
		if mo == "" || mo == "1" {
			return "A cada " + strings.TrimSuffix(unit, "s")
		}
		return "A cada " + mo + " " + unit
	}
	switch sc {
	case "MINUTE":
		return every("minutos")
	case "HOURLY":
		return every("horas")
	case "DAILY":
		return "Todo dia" + at
	case "WEEKLY":
		return "Toda semana" + at
	case "MONTHLY":
		return "Todo mês" + at
	case "ONLOGON":
		return "Ao entrar no Windows"
	case "ONSTART":
		return "Ao ligar o computador"
	}
	return "Agendado no Windows"
}

func suggestName(folder string) string {
	name := reDatePrefix.ReplaceAllString(folder, "")
	name = reLangPrefix.ReplaceAllString(name, "")
	name = strings.TrimSpace(name)
	if name == "" {
		return folder
	}
	r := []rune(name)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

var reMarkdownEmphasis = regexp.MustCompile("[*_`]{1,3}")

func summaryFrom(readme string) string {
	for _, para := range strings.Split(readme, "\n\n") {
		var lines []string
		for _, line := range strings.Split(strings.TrimSpace(para), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
				break
			}
			lines = append(lines, line)
		}
		p := strings.Join(lines, " ")
		if p == "" || strings.HasPrefix(p, "#") || strings.HasPrefix(p, "```") || strings.HasPrefix(p, "!") || strings.HasPrefix(p, "|") || strings.HasPrefix(p, ">") {
			continue
		}
		p = strings.Join(strings.Fields(reMarkdownEmphasis.ReplaceAllString(p, "")), " ")
		if len([]rune(p)) > 280 {
			p = string([]rune(p)[:277]) + "..."
		}
		return p
	}
	return ""
}

func dominantLanguage(count map[string]int) string {
	for _, lang := range []string{"python", "typescript", "javascript", "go", "r", "powershell", "batch", "vbscript"} {
		if count[lang] > 0 {
			if lang == "typescript" || lang == "javascript" {
				if count["go"] > 0 {
					return "go"
				}
			}
			return lang
		}
	}
	return ""
}

func langLabel(l string) string {
	switch l {
	case "python":
		return "Python"
	case "javascript":
		return "JavaScript"
	case "typescript":
		return "TypeScript"
	case "go":
		return "Go"
	case "powershell":
		return "PowerShell"
	case "batch":
		return "arquivos .bat do Windows"
	case "vbscript":
		return "VBScript"
	case "r":
		return "R"
	}
	return l
}

func pickExe(exes []file) string {
	sort.SliceStable(exes, func(i, j int) bool {
		if exes[i].depth != exes[j].depth {
			return exes[i].depth < exes[j].depth
		}
		return exes[i].rel < exes[j].rel
	})
	return exes[0].rel
}

func parseRequirements(text string) []string {
	var deps []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		if line == "" || strings.HasPrefix(line, "-") {
			continue
		}
		deps = append(deps, line)
	}
	return deps
}

func thirdParty(imports, local map[string]bool) []string {
	var deps []string
	for mod := range imports {
		if stdlib[mod] || local[mod] || strings.HasPrefix(mod, "_") {
			continue
		}
		if pip, ok := pipNames[mod]; ok {
			mod = pip
		}
		deps = append(deps, mod)
	}
	sort.Strings(deps)
	return deps
}

func inputID(base string) string {
	id := strings.ReplaceAll(manifest.Slugify(strings.TrimSuffix(base, path.Ext(base))), "-", "_")
	return id
}

func outputLabel(pattern string) string {
	switch strings.ToLower(path.Ext(pattern)) {
	case ".xlsx", ".xls", ".xlsm":
		return "Planilha gerada"
	case ".csv":
		return "Tabela CSV gerada"
	case ".pdf":
		return "PDF gerado"
	case ".html":
		return "Página gerada"
	case ".json":
		return "Dados gerados"
	case ".docx":
		return "Documento gerado"
	case ".md", ".txt":
		return "Texto gerado"
	}
	return "Arquivo gerado"
}

func foldAccents(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, _ := transform.String(t, s)
	return out
}

// ScanBatch treats every visible subfolder of root as one app and diagnoses them in parallel.
func ScanBatch(root string) ([]Diagnosis, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && !skipDirs[e.Name()] {
			dirs = append(dirs, filepath.Join(root, e.Name()))
		}
	}
	results := make([]Diagnosis, len(dirs))
	errs := make([]error, len(dirs))
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i, dir := range dirs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i], errs[i] = Scan(dir)
		}()
	}
	wg.Wait()
	out := results[:0]
	for i, r := range results {
		if errs[i] == nil {
			out = append(out, r)
		}
	}
	return out, nil
}
