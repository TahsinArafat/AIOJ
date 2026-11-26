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
	"github.com/tahsinarafat/aioj/lang"
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
	StackLimitKB          int               `json:"stack_limit_kb,omitempty"`
	SeccompRule           string            `json:"seccomp_rule"`
	Extensions            []string          `json:"extensions"`
	Mono                  bool              `json:"mono,omitempty"`
	FilePath              string            `json:"file_path"`
}

type DetectedTool struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version"`
	// Binary is the bare name as it appears on PATH, which is what a lang/*.yaml
	// compile or runtime command should reference. Distinct from Path when the
	// tool is a versioned sibling (`python3.12` resolving to /usr/bin/python3).
	Binary string `json:"binary,omitempty"`
	// Class is "compiler" or "interpreter"; the panel files the tool by it.
	Class string `json:"class,omitempty"`
}

const (
	// A probe is a version print or a tiny compile. Both finish in well under a
	// second on a warm sandbox; the cap only stops a wedged interpreter from
	// holding the request open.
	detectionTimeout     = 10 * time.Second
	detectionCPULimit    = 5_000_000_000 // 5 s
	detectionMemoryLimit = 268_435_456   // 256 MB
)

// toolHint names a toolchain the panel can label. It carries no probe: what is
// installed is discovered from the judge image (discoverTools), and a hint only
// decides what a discovered binary is called and which column it lands in.
type toolHint struct {
	Name   string
	Binary string
}

// toolClass tells the two halves of the panel apart.
type toolClass string

const (
	classCompiler    toolClass = "compiler"
	classInterpreter toolClass = "interpreter"
)

// knownCompilers and knownInterpreters are the *hint* list, not the source of
// truth: they name the tools AIOJ ships language configs for, so the panel can
// label a bare `python3.12` as "Python 3" and put the 32-bit variants in the
// right column. What is actually installed comes from the judge image at
// request time (discoverTools), because a list maintained here drifts from
// Dockerfile.judge the moment either one changes — and the drift is silent,
// showing "not detected" for a tool that is plainly there.
var knownCompilers = []toolHint{
	{Name: "GCC (C)", Binary: "gcc"},
	{Name: "G++ (C++)", Binary: "g++"},
	{Name: "GCC 32-bit", Binary: "gcc"},
	{Name: "G++ 32-bit", Binary: "g++"},
	{Name: "Clang", Binary: "clang"},
	{Name: "Clang++", Binary: "clang++"},
	{Name: "Rustc", Binary: "rustc"},
	{Name: "Javac", Binary: "javac"},
	{Name: "Mono MCS", Binary: "mcs"},
	{Name: "GDC (D)", Binary: "gdc"},
	{Name: "Go", Binary: "go"},
	{Name: "Kotlin", Binary: "kotlinc"},
	{Name: "Scala", Binary: "scalac"},
	{Name: "Swift", Binary: "swift"},
	{Name: "Zig", Binary: "zig"},
}

var knownInterpreters = []toolHint{
	{Name: "Python 3", Binary: "python3"},
	{Name: "Python 2", Binary: "python2"},
	{Name: "PyPy 3", Binary: "pypy3"},
	{Name: "Node.js", Binary: "node"},
	{Name: "Ruby", Binary: "ruby"},
	{Name: "Perl", Binary: "perl"},
	{Name: "PHP", Binary: "php"},
	{Name: "Lua", Binary: "lua"},
	{Name: "R (script)", Binary: "Rscript"},
	{Name: "Julia", Binary: "julia"},
	{Name: "Haskell (GHC)", Binary: "ghc"},
	{Name: "Erlang", Binary: "erl"},
	{Name: "Elixir", Binary: "elixir"},
	{Name: "Java (runtime)", Binary: "java"},
	{Name: "Mono (runtime)", Binary: "mono"},
	{Name: "Tclsh", Binary: "tclsh"},
}

// detectionSearchPath is the PATH a compile or a run gets inside the sandbox
// (internal/judge/compiler and internal/judge/runner both set exactly this).
// Discovery searches the same path, so a tool it reports is a tool a submission
// can actually exec — a binary found somewhere else would be a lie.
const detectionSearchPath = "/usr/bin:/bin:/usr/local/go/bin"

// hintLabel resolves a discovered binary to the name the panel should show, so
// `python3.12` reads "Python 3".
//
// Both tables are searched because a name can appear in either — `java` is a
// runtime and also what javac lives beside — and the label is the same string
// either way. Only the *class* needs the tie broken, and that is classifyTool's
// job.
func hintLabel(name string) (string, bool) {
	base := baseToolName(name)
	for _, table := range [][]toolHint{knownCompilers, knownInterpreters} {
		for _, t := range table {
			if t.Binary == name || baseToolName(t.Binary) == base {
				return t.Name, true
			}
		}
	}
	return "", false
}

// classifyTool decides which column a discovered binary belongs in, and how to
// label it.
//
// The rule is the toolchain's own behaviour rather than its name: a binary that
// can turn source into an executable is a compiler, everything else is a runtime
// the panel lists among the interpreters.
func (h *AdminLanguageHandler) classifyTool(ctx context.Context, bin string) (toolClass, string) {
	label, known := hintLabel(bin)
	if !known {
		// Unknown binary: ask it. The probe compiles a one-line program and runs
		// the result, which is the same path a submission takes — a `--help` exit
		// code would misfile `clang` and `g++` alike.
		if h.toolCompiles(ctx, bin) {
			return classCompiler, bin
		}
		return classInterpreter, bin
	}
	// A known name is classified by which table claims it, preferring the
	// compiler reading: `javac` must not be filed under "Java (runtime)" just
	// because that hint shares its directory.
	if _, ok := matchHint(bin, classCompiler); ok {
		return classCompiler, label
	}
	return classInterpreter, label
}

func matchHint(name string, class toolClass) (toolHint, bool) {
	table := knownInterpreters
	if class == classCompiler {
		table = knownCompilers
	}
	base := baseToolName(name)
	for _, t := range table {
		if t.Binary == name || baseToolName(t.Binary) == base {
			return t, true
		}
	}
	return toolHint{}, false
}

// toolCompiles reports whether bin can produce a working executable.
//
// Source and output are chosen per binary because the input language is the
// compiler's, not ours: handing `hello.cpp` to gcc fails, and that failure would
// read as "not a compiler".
func (h *AdminLanguageHandler) toolCompiles(ctx context.Context, bin string) bool {
	src, out := "t.c", "t"
	switch {
	case strings.Contains(bin, "++") || strings.Contains(bin, "clang++"):
		src = "t.cpp"
	case strings.Contains(bin, "rustc"):
		src = "t.rs"
	case strings.Contains(bin, "go"):
		src = "t.go"
	}

	script := "printf 'int main(void){return 0;}' > " + src +
		" 2>/dev/null || printf 'fn main(){}' > " + src
	script += "; " + bin + " -o " + out + " " + src + " > compile.txt 2>&1 && ./" + out

	resp, err := h.exec.Run(ctx, &executor.ExecRequest{Cmd: []executor.Cmd{{
		Args:        []string{"/bin/sh", "-c", script},
		Env:         []string{"PATH=" + detectionSearchPath, "HOME=/tmp"},
		CPULimit:    detectionCPULimit,
		MemoryLimit: detectionMemoryLimit,
		ProcLimit:   64,
	}}})
	if err != nil || len(resp) == 0 {
		return false
	}
	cr := resp[0]
	return cr.Status == "Accepted" && cr.ExitStatus == 0
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
			StackLimitKB:          lang.StackLimitKB,
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
		StackLimitKB:          lang.StackLimitKB,
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
		"stack_limit_kb":          lang.StackLimitKB,
		"seccomp_rule":            lang.SeccompRule,
		"extensions":              lang.Extensions,
		"status":                  "config_valid",
		"message":                 fmt.Sprintf("Language '%s' (%s) configuration is valid", lang.Name, lang.Key),
	}

	switch {
	case lang.CompileCmd == "" && lang.Runtime == "":
		result["status"] = "warning"
		result["message"] = "No compile command or runtime specified"
	case hasPELinkerStackFlag(lang.CompileCmd):
		// The judge image is Linux/ELF. `-Wl,--stack=` is a PE/COFF option:
		// GNU ld rejects it, so the compile fails and every submission in this
		// language is a compile error. It is worth catching here because the
		// flag looks plausible and the failure only shows up per submission.
		result["status"] = "error"
		result["message"] = "-Wl,--stack= is a Windows (PE/COFF) linker option and GNU ld rejects it on Linux, " +
			"so every submission in this language fails to compile. Remove it and set stack_limit_kb instead, " +
			"which raises RLIMIT_STACK for the run"
	case lang.StackLimitKB <= 0:
		// Not an error — the sandbox default is a real choice — but worth
		// saying out loud, because a contestant who writes a deep recursion
		// gets a segfault that looks like a broken judge.
		result["status"] = "warning"
		result["message"] = "No stack_limit_kb set: the sandbox default applies, so deep recursion will crash with a runtime error"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// hasPELinkerStackFlag reports whether a compile command passes a linker stack
// size the way MinGW does.
//
// The judge toolchain is Linux/ELF, where GNU ld has no --stack option at all
// and exits non-zero on it, so the language is unusable rather than merely
// misconfigured. The knob that does work is stack_limit_kb, which sets
// RLIMIT_STACK for the run.
func hasPELinkerStackFlag(compileCmd string) bool {
	return strings.Contains(compileCmd, "-Wl,--stack") ||
		strings.Contains(compileCmd, "-Wl,-stack_size")
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

// Reset restores a language to the config that ships in lang/*.yaml.
//
// The panel edits those files in place, so a save that drops a field — or an
// operator clearing a command by hand — leaves the language broken with no way
// back short of a redeploy. Reset is that way back, and it restores the real
// shipped bytes rather than a second hardcoded copy, so the two cannot drift.
//
// A language an operator created has no shipped default; that is a 404 with a
// message saying so, not a silent no-op.
func (h *AdminLanguageHandler) Reset(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil || claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	key := chi.URLParam(r, "key")
	defaults, ok := lang.Default(key)
	if !ok {
		http.Error(w, fmt.Sprintf("language %q has no built-in default to restore "+
			"(only keys shipped in lang/*.yaml can be reset)", key), http.StatusNotFound)
		return
	}

	// Validate before writing. A malformed shipped file would otherwise be
	// written to disk and then ignored by the judge's loader, which skips
	// unparseable configs — the language would disappear from every submission
	// dropdown with no error anywhere.
	var cfg compiler.LangConfig
	if err := yaml.Unmarshal(defaults, &cfg); err != nil {
		http.Error(w, "embedded default is invalid: "+err.Error(), http.StatusInternalServerError)
		return
	}

	filePath := filepath.Join(h.langDir, key+".yaml")
	if err := os.WriteFile(filePath, defaults, 0644); err != nil {
		http.Error(w, "failed to write file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LanguageConfigResponse{
		Name:                  cfg.Name,
		Key:                   cfg.Key,
		CompileCmd:            cfg.CompileCmd,
		Runtime:               cfg.Runtime,
		CopyIn:                cfg.CopyIn,
		TimeLimitMultiplier:   cfg.TimeLimitMultiplier,
		MemoryLimitMultiplier: cfg.MemoryLimitMultiplier,
		StackLimitKB:          cfg.StackLimitKB,
		SeccompRule:           cfg.SeccompRule,
		Extensions:            cfg.Extensions,
		Mono:                  cfg.Mono,
		FilePath:              key + ".yaml",
	})
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

	for _, t := range h.discoverTools(r.Context()) {
		if t.Class == string(classCompiler) {
			result.Compilers = append(result.Compilers, t)
		} else {
			result.Interpreters = append(result.Interpreters, t)
		}
	}
	// Stable order: discovery finishes in whatever order the version probes
	// return, which would shuffle the panel on every click.
	sort.Slice(result.Compilers, func(i, j int) bool { return result.Compilers[i].Name < result.Compilers[j].Name })
	sort.Slice(result.Interpreters, func(i, j int) bool { return result.Interpreters[i].Name < result.Interpreters[j].Name })

	respondJSON(w, http.StatusOK, result)
}

// discoverTools asks the judge image what it can actually run.
//
// The list used to be the hardcoded knownCompilers/knownInterpreters tables, and
// every entry had to be added to Dockerfile.judge, to this file, and to
// lang/*.yaml by hand. Miss one and the panel reports a tool that is installed
// as "not detected", which is indistinguishable from it being absent — the
// 32-bit entries were only correct because someone remembered to make them
// compile probes.
//
// So the image is enumerated instead: list the executable files on the same PATH
// a submission gets, then ask each one for its version. The tables survive only
// as labels for the names the panel shows.
func (h *AdminLanguageHandler) discoverTools(ctx context.Context) []DetectedTool {
	if h.exec == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, detectionTimeout)
	defer cancel()

	// A discovery run must not be able to write outside the sandbox's own /w and
	// /tmp, and it needs no compiler: it only lists and runs.
	listed, err := h.exec.Run(ctx, &executor.ExecRequest{Cmd: []executor.Cmd{{
		Args: []string{"/bin/sh", "-c",
			"for d in $(echo \"$PATH\" | tr ':' ' '); do " +
				"[ -d \"$d\" ] && ls -1 \"$d\" 2>/dev/null; done | sort -u"},
		Env:         []string{"PATH=" + detectionSearchPath, "HOME=/tmp"},
		CPULimit:    detectionCPULimit,
		MemoryLimit: detectionMemoryLimit,
		ProcLimit:   64,
		CopyOut:     []string{"stdout"},
	}}})
	if err != nil || len(listed) == 0 {
		return nil
	}

	names := candidateNames(listed[0].Files["stdout"])
	if len(names) > maxProbesPerDetection {
		names = names[:maxProbesPerDetection]
	}
	if len(names) == 0 {
		return nil
	}

	// Concurrency is bounded well below the tool count. Every probe is a sandbox
	// process, and the judge container has a fixed memory budget shared with live
	// submissions — firing the whole image at once to answer one admin click
	// could pressure the host into holding real judgments.
	const maxConcurrentProbes = 4
	sem := make(chan struct{}, maxConcurrentProbes)

	var wg sync.WaitGroup
	var mu sync.Mutex
	out := make([]DetectedTool, 0, len(names))

	for _, name := range names {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			bin, version, ok := h.probeVersion(ctx, name)
			if !ok {
				return
			}
			class, label := h.classifyTool(ctx, name)
			mu.Lock()
			defer mu.Unlock()
			// Path is the resolved binary when the sandbox answered `command -v`,
			// otherwise the bare name — which is what a lang/*.yaml should use
			// anyway, since it is resolved through PATH at run time.
			path := bin
			if path == "" {
				path = name
			}
			out = append(out, DetectedTool{
				Name:    label,
				Path:    path,
				Version: version,
				Binary:  name,
				Class:   string(class),
			})
		}(name)
	}
	wg.Wait()

	return out
}

// candidateNames reduces a raw directory listing to names worth probing.
//
// The image contains thousands of files and probing each would be thousands of
// sandbox processes to answer one click, so the listing is cut down by what a
// toolchain can plausibly be called before anything is executed.
func candidateNames(listing string) []string {
	var bins, libs []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(listing, "\n") {
		name := strings.TrimSpace(line)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		if !plausiblyATool(name) {
			continue
		}
		if strings.Contains(name, "-") || strings.Contains(name, ".") {
			// `python3.12`, `gcc-12`, `java-21-openjdk`: versioned or package
			// qualified, so a real candidate but not the name to prefer.
			libs = append(libs, name)
			continue
		}
		bins = append(bins, name)
	}
	// Unversioned names first: they are what lang/*.yaml should reference, and
	// they bound the work when a distro ships both.
	return append(bins, libs...)
}

// maxProbesPerDetection caps how many binaries one Detect call will run.
//
// Discovery is one admin click, and every probe is a sandbox process on a judge
// container shared with live submissions. The cap is generous relative to a real
// toolchain count (the judge image has ~20) but stops a pathological image from
// turning the panel into a load generator.
const maxProbesPerDetection = 200

// plausiblyATool is a name filter, not a verdict: a false positive costs one
// cheap probe, a false negative hides a real toolchain.
func plausiblyATool(name string) bool {
	if strings.HasPrefix(name, ".") || strings.Contains(name, " ") {
		return false
	}
	switch name {
	case "sh", "bash", "dash", "ls", "cat", "cp", "mv", "rm", "sed", "awk", "grep",
		"tar", "gzip", "env", "test", "true", "false", "expr", "sort", "uniq", "tr":
		return false
	}
	return true
}

// baseToolName strips the version a distro appends to a tool name, so
// `python3.12` and `gcc-12` resolve to the hint that labels them.
//
// Only a numeric-looking tail is stripped: splitting at the first dot would turn
// `clang++` into `clang` and hand C++ a C label.
func baseToolName(name string) string {
	for i := len(name) - 1; i > 0; i-- {
		if name[i] != '-' && name[i] != '.' {
			continue
		}
		tail := name[i+1:]
		if tail == "" {
			break
		}
		if c := tail[0]; c < '0' || c > '9' {
			continue
		}
		return name[:i]
	}
	return name
}

// probeVersion runs `name --version` (and the two variants that use a single
// dash) inside the sandbox, returning the resolved path and first banner line.
//
// A non-zero exit is fine: javac, java and erl all print their banner and exit
// non-zero. Only 127 — the shell's "command not found" — means absent.
func (h *AdminLanguageHandler) probeVersion(ctx context.Context, name string) (bin, version string, ok bool) {
	// `command -v` resolves through the same PATH a submission uses, so the path
	// reported is the one that will actually be exec'd.
	script := "command -v " + shellQuote(name) + " > path.txt 2>&1 || exit 127\n" +
		"{ " + shellQuote(name) + " --version || " + shellQuote(name) + " -version || " +
		shellQuote(name) + " version; } > version.txt 2>&1 || :\n" +
		"exit 0"

	resp, err := h.exec.Run(ctx, &executor.ExecRequest{Cmd: []executor.Cmd{{
		Args:        []string{"/bin/sh", "-c", script},
		Env:         []string{"PATH=" + detectionSearchPath, "HOME=/tmp"},
		CPULimit:    detectionCPULimit,
		MemoryLimit: detectionMemoryLimit,
		ProcLimit:   64,
		CopyOut:     []string{"stdout", "path.txt", "version.txt"},
	}}})
	if err != nil || len(resp) == 0 {
		return "", "", false
	}

	cr := resp[0]
	// The resolved path is authoritative; the bare name is the fallback for a
	// sandbox that reports the command but not `command -v`'s answer.
	bin = firstLine(cr.Files["path.txt"])
	if bin == "" {
		bin = name
	}
	return bin, firstLine(cr.Files["version.txt"]), true
}

// toolNameFromScript recovers the tool name from a probeVersion script.
//
// The script is built by quoting the name, so this is the inverse of that
// quoting rather than a second copy of it — the two cannot drift apart.
func toolNameFromScript(script string) string {
	rest := script[strings.Index(script, "{"):]
	rest = strings.TrimSpace(strings.TrimPrefix(rest, "{"))
	name, _, _ := strings.Cut(rest, " ")
	return name
}

// shellQuote renders one argument safely for /bin/sh. Tool names come from a
// directory listing, so this is the boundary between the image's contents and a
// command line.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
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
