package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/judge/compiler"
	"github.com/tahsinarafat/aioj/internal/judge/executor"
	"gopkg.in/yaml.v3"
)

type AdminLanguageHandler struct {
	langDir string
	// exec probes for toolchains through the judge sandbox rather than the
	// backend's own filesystem. The backend image (Dockerfile.api) deliberately
	// ships no compilers, so a host-side os.Stat/exec found nothing and reported
	// an empty tool list on a healthy install.
	exec *executor.Client
}

func NewAdminLanguageHandler(langDir string, exec *executor.Client) *AdminLanguageHandler {
	return &AdminLanguageHandler{langDir: langDir, exec: exec}
}

type LanguageConfigResponse struct {
	Name                  string            `json:"name"`
	Key                   string            `json:"key"`
	CompileCmd            string            `json:"compile"`
	Runtime               string            `json:"runtime,omitempty"`
	CopyIn                map[string]string `json:"copy_in,omitempty"`
	TimeLimitMultiplier   float64           `json:"time_limit_multiplier"`
	MemoryLimitMultiplier float64           `json:"memory_limit_multiplier"`
	SeccompRule           string            `json:"seccomp_rule"`
	Extensions            []string          `json:"extensions"`
	Mono                  bool              `json:"mono,omitempty"`
	FilePath              string            `json:"file_path"`
}

type DetectedTool struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version"`
}

const (
	// A probe is a version print or a tiny compile. Both finish in well under a
	// second on a warm sandbox; the cap only stops a wedged interpreter from
	// holding the request open.
	detectionTimeout     = 10 * time.Second
	detectionCPULimit    = 5_000_000_000 // 5 s
	detectionMemoryLimit = 268_435_456   // 256 MB
)

// toolProbe describes one toolchain to look for. Probe is a shell command run
// inside the judge sandbox; Version is appended to it to capture the banner.
//
// Compiles marks the probes that are real compile tests. It matters: printing
// `gcc -m32 --version` succeeds on a toolchain with no 32-bit multilib, which is
// how the panel came to advertise "GCC 32-bit" while every 32-bit submission
// failed. A compile probe only reports the tool when the compile links, which is
// the same path a submission takes.
type toolProbe struct {
	Name     string
	Binary   string
	Probe    string
	Version  string
	Compiles bool
}

var knownCompilers = []toolProbe{
	{Name: "GCC (C)", Binary: "/usr/bin/gcc", Probe: "gcc --version", Version: "--version"},
	{Name: "G++ (C++)", Binary: "/usr/bin/g++", Probe: "g++ --version", Version: "--version"},
	{Name: "Clang", Binary: "/usr/bin/clang", Probe: "clang --version", Version: "--version"},
	{Name: "Clang++", Binary: "/usr/bin/clang++", Probe: "clang++ --version", Version: "--version"},
	{
		Name:     "GCC 32-bit",
		Binary:   "/usr/bin/gcc",
		Probe:    `printf 'int main(void){return 0;}' > t.c && gcc -m32 -o t32 t.c`,
		Version:  "--version",
		Compiles: true,
	},
	{
		Name:     "G++ 32-bit",
		Binary:   "/usr/bin/g++",
		Probe:    `printf 'int main(){return 0;}' > t.cpp && g++ -m32 -o t32cpp t.cpp`,
		Version:  "--version",
		Compiles: true,
	},
	{Name: "Rustc", Binary: "/usr/bin/rustc", Probe: "rustc --version", Version: "--version"},
	{Name: "Javac", Binary: "/usr/bin/javac", Probe: "javac -version", Version: "-version"},
	{Name: "Mono MCS", Binary: "/usr/bin/mcs", Probe: "mcs --version", Version: "--version"},
	{Name: "GDC (D)", Binary: "/usr/bin/gdc", Probe: "gdc --version", Version: "--version"},
	{Name: "Go", Binary: "/usr/local/go/bin/go", Probe: "go version", Version: "version"},
	{Name: "Kotlin", Binary: "/usr/bin/kotlinc", Probe: "kotlinc -version", Version: "-version"},
	{Name: "Scala", Binary: "/usr/bin/scalac", Probe: "scalac -version", Version: "-version"},
	{Name: "Swift", Binary: "/usr/bin/swift", Probe: "swift --version", Version: "--version"},
	{Name: "Zig", Binary: "/usr/bin/zig", Probe: "zig version", Version: "version"},
}

var knownInterpreters = []toolProbe{
	{Name: "Python 3", Binary: "/usr/bin/python3", Probe: "python3 --version", Version: "--version"},
	{Name: "Python 2", Binary: "/usr/bin/python2", Probe: "python2 --version", Version: "--version"},
	{Name: "PyPy 3", Binary: "/usr/bin/pypy3", Probe: "pypy3 --version", Version: "--version"},
	{Name: "Node.js", Binary: "/usr/bin/node", Probe: "node --version", Version: "--version"},
	{Name: "Ruby", Binary: "/usr/bin/ruby", Probe: "ruby --version", Version: "--version"},
	{Name: "Perl", Binary: "/usr/bin/perl", Probe: "perl --version", Version: "--version"},
	{Name: "PHP", Binary: "/usr/bin/php", Probe: "php --version", Version: "--version"},
	{Name: "Lua", Binary: "/usr/bin/lua", Probe: "lua -v", Version: "-v"},
	{Name: "R (script)", Binary: "/usr/bin/Rscript", Probe: "Rscript --version", Version: "--version"},
	{Name: "Julia", Binary: "/usr/bin/julia", Probe: "julia --version", Version: "--version"},
	{Name: "Haskell (GHC)", Binary: "/usr/bin/ghc", Probe: "ghc --version", Version: "--version"},
	{Name: "Erlang", Binary: "/usr/bin/erl", Probe: "erl -version", Version: "-version"},
	{Name: "Elixir", Binary: "/usr/bin/elixir", Probe: "elixir --version", Version: "--version"},
	{Name: "Java (runtime)", Binary: "/usr/bin/java", Probe: "java -version", Version: "-version"},
	{Name: "Mono (runtime)", Binary: "/usr/bin/mono", Probe: "mono --version", Version: "--version"},
	{Name: "Tclsh", Binary: "/usr/bin/tclsh", Probe: "echo 'exit 0' | tclsh", Version: ""},
}

func (h *AdminLanguageHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil || claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	langs, err := compiler.LoadLanguages(h.langDir)
	if err != nil {
		http.Error(w, "failed to load languages: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var result []LanguageConfigResponse
	for _, lang := range langs {
		result = append(result, LanguageConfigResponse{
			Name:                  lang.Name,
			Key:                   lang.Key,
			CompileCmd:            lang.CompileCmd,
			Runtime:               lang.Runtime,
			CopyIn:                lang.CopyIn,
			TimeLimitMultiplier:   lang.TimeLimitMultiplier,
			MemoryLimitMultiplier: lang.MemoryLimitMultiplier,
			SeccompRule:           lang.SeccompRule,
			Extensions:            lang.Extensions,
			Mono:                  lang.Mono,
			FilePath:              lang.Key + ".yaml",
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Key < result[j].Key
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"data": result})
}

func (h *AdminLanguageHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil || claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	key := chi.URLParam(r, "key")
	lang, err := h.loadLang(key)
	if err != nil {
		http.Error(w, "language not found", http.StatusNotFound)
		return
	}

	resp := LanguageConfigResponse{
		Name:                  lang.Name,
		Key:                   lang.Key,
		CompileCmd:            lang.CompileCmd,
		Runtime:               lang.Runtime,
		CopyIn:                lang.CopyIn,
		TimeLimitMultiplier:   lang.TimeLimitMultiplier,
		MemoryLimitMultiplier: lang.MemoryLimitMultiplier,
		SeccompRule:           lang.SeccompRule,
		Extensions:            lang.Extensions,
		Mono:                  lang.Mono,
		FilePath:              key + ".yaml",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *AdminLanguageHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil || claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	key := chi.URLParam(r, "key")
	filePath := filepath.Join(h.langDir, key+".yaml")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "language not found; use POST /api/admin/languages to create new languages", http.StatusNotFound)
		return
	}

	var cfg compiler.LangConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if cfg.Key == "" {
		cfg.Key = key
	}
	if cfg.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if cfg.TimeLimitMultiplier <= 0 {
		cfg.TimeLimitMultiplier = 1.0
	}
	if cfg.MemoryLimitMultiplier <= 0 {
		cfg.MemoryLimitMultiplier = 1.0
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		http.Error(w, "failed to marshal yaml", http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		http.Error(w, "failed to write file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "key": key})
}

func (h *AdminLanguageHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil || claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	key := chi.URLParam(r, "key")
	filePath := filepath.Join(h.langDir, key+".yaml")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "language not found", http.StatusNotFound)
		return
	}

	if err := os.Remove(filePath); err != nil {
		http.Error(w, "failed to delete: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AdminLanguageHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil || claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var cfg compiler.LangConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if cfg.Key == "" || cfg.Name == "" {
		http.Error(w, "key and name are required", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(h.langDir, cfg.Key+".yaml")
	if _, err := os.Stat(filePath); err == nil {
		http.Error(w, "language already exists", http.StatusConflict)
		return
	}

	if cfg.TimeLimitMultiplier <= 0 {
		cfg.TimeLimitMultiplier = 1.0
	}
	if cfg.MemoryLimitMultiplier <= 0 {
		cfg.MemoryLimitMultiplier = 1.0
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		http.Error(w, "failed to marshal yaml", http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		http.Error(w, "failed to write file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "key": cfg.Key})
}

func (h *AdminLanguageHandler) Test(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil || claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	key := chi.URLParam(r, "key")
	lang, err := h.loadLang(key)
	if err != nil {
		http.Error(w, "language not found", http.StatusNotFound)
		return
	}

	result := map[string]interface{}{
		"key":                     lang.Key,
		"name":                    lang.Name,
		"compile_command":         lang.CompileCmd,
		"runtime":                 lang.Runtime,
		"time_limit_multiplier":   lang.TimeLimitMultiplier,
		"memory_limit_multiplier": lang.MemoryLimitMultiplier,
		"seccomp_rule":            lang.SeccompRule,
		"extensions":              lang.Extensions,
		"status":                  "config_valid",
		"message":                 fmt.Sprintf("Language '%s' (%s) configuration is valid", lang.Name, lang.Key),
	}

	if lang.CompileCmd == "" && lang.Runtime == "" {
		result["status"] = "warning"
		result["message"] = "No compile command or runtime specified"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *AdminLanguageHandler) GetRaw(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil || claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	key := chi.URLParam(r, "key")
	filePath := filepath.Join(h.langDir, key+".yaml")

	data, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/yaml")
	w.Write(data)
}

func (h *AdminLanguageHandler) UpdateRaw(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil || claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	key := chi.URLParam(r, "key")
	filePath := filepath.Join(h.langDir, key+".yaml")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "language not found; use POST /api/admin/languages to create new languages", http.StatusNotFound)
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var cfg compiler.LangConfig
	if err := yaml.Unmarshal([]byte(body.Content), &cfg); err != nil {
		http.Error(w, "invalid YAML: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := os.WriteFile(filePath, []byte(body.Content), 0644); err != nil {
		http.Error(w, "failed to write file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func (h *AdminLanguageHandler) loadLang(key string) (*compiler.LangConfig, error) {
	filePath := filepath.Join(h.langDir, strings.ToLower(key)+".yaml")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var cfg compiler.LangConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (h *AdminLanguageHandler) Detect(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil || claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Non-nil slices on purpose. A nil slice marshals to `null`, and the admin
	// panel called `.map()` on it, so the whole page blanked whenever nothing
	// was found — which is exactly what happened on the lean backend image that
	// ships no compilers. An empty array renders "No compilers detected".
	result := struct {
		Compilers    []DetectedTool `json:"compilers"`
		Interpreters []DetectedTool `json:"interpreters"`
	}{
		Compilers:    []DetectedTool{},
		Interpreters: []DetectedTool{},
	}

	// Concurrency is bounded well below the tool count. Every probe is a sandbox
	// process, and the judge container has a fixed memory budget shared with live
	// submissions — firing ~30 at once to answer one admin click could pressure
	// the host into holding real judgments.
	const maxConcurrentProbes = 4
	sem := make(chan struct{}, maxConcurrentProbes)

	var wg sync.WaitGroup
	var mu sync.Mutex

	probe := func(tool toolProbe, into *[]DetectedTool) {
		defer wg.Done()
		sem <- struct{}{}
		defer func() { <-sem }()

		found := h.detectTool(r.Context(), tool)
		if found == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		*into = append(*into, *found)
	}

	wg.Add(len(knownCompilers) + len(knownInterpreters))
	for _, c := range knownCompilers {
		go probe(c, &result.Compilers)
	}
	for _, i := range knownInterpreters {
		go probe(i, &result.Interpreters)
	}
	wg.Wait()

	respondJSON(w, http.StatusOK, result)
}

// detectTool runs one tool's check inside the judge sandbox. The compilers live
// in the judge image, not the backend image, so probing locally only ever
// reported "nothing found".
//
// Availability and version come from a single sandbox call: the probe decides
// whether the tool is usable, and the same run captures the version banner.
func (h *AdminLanguageHandler) detectTool(ctx context.Context, tool toolProbe) *DetectedTool {
	if h.exec == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, detectionTimeout)
	defer cancel()

	// Each branch keeps a meaningful exit status, so one run both decides
	// availability and captures the banner.
	//
	//  - compile probes: the version command runs only if the compile linked,
	//    so a failure to link leaves the probe failing.
	//  - version probes: the command's own output is the banner, so it is
	//    redirected; the exit status is what separates a missing binary from a
	//    tool that prints then exits non-zero (javac, java, erl).
	var script string
	if tool.Compiles {
		script = tool.Probe
		if tool.Version != "" {
			script += " && " + tool.Binary + " " + tool.Version + " > version.txt 2>&1"
		}
	} else {
		script = tool.Probe + " > version.txt 2>&1"
	}

	resp, err := h.exec.Run(ctx, &executor.ExecRequest{Cmd: []executor.Cmd{{
		Args:        []string{"/bin/sh", "-c", script},
		Env:         []string{"PATH=/usr/bin:/bin:/usr/local/go/bin", "HOME=/tmp"},
		CPULimit:    detectionCPULimit,
		MemoryLimit: detectionMemoryLimit,
		ProcLimit:   64,
		CopyOut:     []string{"version.txt"},
	}}})
	if err != nil || len(resp) == 0 {
		return nil
	}

	cr := resp[0]
	if tool.Compiles {
		// The compile only counts when it actually linked; a missing 32-bit
		// libgcc surfaces here as a rejected or non-zero run.
		if cr.Status != "Accepted" || cr.ExitStatus != 0 {
			return nil
		}
	} else if cr.ExitStatus == 127 {
		// 127 is the shell's "command not found", the one status that means the
		// tool is genuinely absent.
		return nil
	}

	return &DetectedTool{Name: tool.Name, Path: tool.Binary, Version: firstLine(cr.Files["version.txt"])}
}

// firstLine trims a captured version banner to the one line worth showing.
func firstLine(out string) string {
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return ""
	}
	line := strings.Split(trimmed, "\n")[0]
	if len(line) > 100 {
		line = line[:100] + "..."
	}
	return line
}

var languageTemplates = map[string]LanguageConfigResponse{
	"cpp-gcc": {
		Name:                  "C++ (GCC)",
		Key:                   "cpp-gcc",
		CompileCmd:            "/usr/bin/g++ -O2 -std=c++17 -o {{exe}} {{src}}",
		TimeLimitMultiplier:   1.0,
		MemoryLimitMultiplier: 1.0,
		SeccompRule:           "c_cpp",
		Extensions:            []string{".cpp", ".cxx", ".cc"},
	},
	"c-gcc": {
		Name:                  "C (GCC)",
		Key:                   "c-gcc",
		CompileCmd:            "/usr/bin/gcc -O2 -std=c11 -o {{exe}} {{src}}",
		TimeLimitMultiplier:   1.0,
		MemoryLimitMultiplier: 1.0,
		SeccompRule:           "c_cpp",
		Extensions:            []string{".c"},
	},
	"python3": {
		Name:                  "Python 3",
		Key:                   "python3",
		Runtime:               "/usr/bin/python3",
		TimeLimitMultiplier:   3.0,
		MemoryLimitMultiplier: 2.0,
		SeccompRule:           "general",
		Extensions:            []string{".py"},
	},
	"java": {
		Name:                  "Java",
		Key:                   "java",
		CompileCmd:            "/usr/bin/javac -d {{dir}} {{src}}",
		Runtime:               "/usr/bin/java -cp {{dir}} Main",
		TimeLimitMultiplier:   2.0,
		MemoryLimitMultiplier: 2.0,
		SeccompRule:           "general",
		Extensions:            []string{".java"},
	},
	"rust": {
		Name:                  "Rust",
		Key:                   "rust",
		CompileCmd:            "/usr/bin/rustc -O -o {{exe}} {{src}}",
		TimeLimitMultiplier:   1.0,
		MemoryLimitMultiplier: 1.0,
		SeccompRule:           "general",
		Extensions:            []string{".rs"},
	},
	"go": {
		Name:                  "Go",
		Key:                   "go",
		CompileCmd:            "/usr/local/go/bin/go build -o {{exe}} {{src}}",
		TimeLimitMultiplier:   1.0,
		MemoryLimitMultiplier: 1.0,
		SeccompRule:           "general",
		Extensions:            []string{".go"},
	},
	"nodejs": {
		Name:                  "Node.js",
		Key:                   "nodejs",
		Runtime:               "/usr/bin/node",
		TimeLimitMultiplier:   2.0,
		MemoryLimitMultiplier: 1.5,
		SeccompRule:           "node",
		Extensions:            []string{".js"},
	},
	"csharp": {
		Name:                  "C# (Mono)",
		Key:                   "csharp",
		CompileCmd:            "/usr/bin/mcs -out:{{exe}} {{src}}",
		Runtime:               "/usr/bin/mono {{exe}}",
		TimeLimitMultiplier:   1.5,
		MemoryLimitMultiplier: 1.5,
		SeccompRule:           "general",
		Extensions:            []string{".cs"},
		Mono:                  true,
	},
	"pypy": {
		Name:                  "PyPy 3",
		Key:                   "pypy",
		Runtime:               "/usr/bin/pypy3",
		TimeLimitMultiplier:   2.5,
		MemoryLimitMultiplier: 2.0,
		SeccompRule:           "general",
		Extensions:            []string{".py"},
	},
	"kotlin": {
		Name:                  "Kotlin",
		Key:                   "kotlin",
		CompileCmd:            "/usr/bin/kotlinc -include-runtime -jar {{exe}} {{src}}",
		Runtime:               "/usr/bin/java -jar {{exe}}",
		TimeLimitMultiplier:   2.0,
		MemoryLimitMultiplier: 2.0,
		SeccompRule:           "general",
		Extensions:            []string{".kt"},
	},
	"ruby": {
		Name:                  "Ruby",
		Key:                   "ruby",
		Runtime:               "/usr/bin/ruby",
		TimeLimitMultiplier:   3.0,
		MemoryLimitMultiplier: 2.0,
		SeccompRule:           "general",
		Extensions:            []string{".rb"},
	},
	"perl": {
		Name:                  "Perl",
		Key:                   "perl",
		Runtime:               "/usr/bin/perl",
		TimeLimitMultiplier:   3.0,
		MemoryLimitMultiplier: 2.0,
		SeccompRule:           "general",
		Extensions:            []string{".pl"},
	},
}

func (h *AdminLanguageHandler) Templates(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil || claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var templates []LanguageConfigResponse
	for _, t := range languageTemplates {
		templates = append(templates, t)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"data": templates})
}
