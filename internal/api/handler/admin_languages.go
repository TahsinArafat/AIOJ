package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/judge/compiler"
	"gopkg.in/yaml.v3"
)

type AdminLanguageHandler struct {
	langDir string
}

func NewAdminLanguageHandler(langDir string) *AdminLanguageHandler {
	return &AdminLanguageHandler{langDir: langDir}
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

var knownCompilers = []struct {
	Name    string
	Binary  string
	VersionArgs []string
}{
	{"GCC (C)", "/usr/bin/gcc", []string{"--version"}},
	{"G++ (C++)", "/usr/bin/g++", []string{"--version"}},
	{"Clang", "/usr/bin/clang", []string{"--version"}},
	{"Clang++", "/usr/bin/clang++", []string{"--version"}},
	{"GCC 32-bit", "/usr/bin/gcc", []string{"-m32", "--version"}},
	{"G++ 32-bit", "/usr/bin/g++", []string{"-m32", "--version"}},
	{"Rustc", "/usr/bin/rustc", []string{"--version"}},
	{"Javac", "/usr/bin/javac", []string{"-version"}},
	{"Mono MCS", "/usr/bin/mcs", []string{"--version"}},
	{"GDC (D)", "/usr/bin/gdc", []string{"--version"}},
	{"Go", "/usr/local/go/bin/go", []string{"version"}},
	{"Kotlin", "/usr/bin/kotlinc", []string{"-version"}},
	{"Scala", "/usr/bin/scalac", []string{"-version"}},
	{"Swift", "/usr/bin/swift", []string{"--version"}},
	{"Zig", "/usr/bin/zig", []string{"version"}},
}

var knownInterpreters = []struct {
	Name    string
	Binary  string
	VersionArgs []string
}{
	{"Python 3", "/usr/bin/python3", []string{"--version"}},
	{"Python 2", "/usr/bin/python2", []string{"--version"}},
	{"PyPy 3", "/usr/bin/pypy3", []string{"--version"}},
	{"Node.js", "/usr/bin/node", []string{"--version"}},
	{"Ruby", "/usr/bin/ruby", []string{"--version"}},
	{"Perl", "/usr/bin/perl", []string{"--version"}},
	{"PHP", "/usr/bin/php", []string{"--version"}},
	{"Lua", "/usr/bin/lua", []string{"-v"}},
	{"R (script)", "/usr/bin/Rscript", []string{"--version"}},
	{"Julia", "/usr/bin/julia", []string{"--version"}},
	{"Haskell (GHC)", "/usr/bin/ghc", []string{"--version"}},
	{"Erlang", "/usr/bin/erl", []string{"-version"}},
	{"Elixir", "/usr/bin/elixir", []string{"--version"}},
	{"Java (runtime)", "/usr/bin/java", []string{"-version"}},
	{"Mono (runtime)", "/usr/bin/mono", []string{"--version"}},
	{"Tclsh", "/usr/bin/tclsh", []string{""}},
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

	filePath := filepath.Join(h.langDir, key+".yaml")
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
		"key":                   lang.Key,
		"name":                  lang.Name,
		"compile_command":       lang.CompileCmd,
		"runtime":               lang.Runtime,
		"time_limit_multiplier": lang.TimeLimitMultiplier,
		"memory_limit_multiplier": lang.MemoryLimitMultiplier,
		"seccomp_rule":          lang.SeccompRule,
		"extensions":            lang.Extensions,
		"status":                "config_valid",
		"message":               fmt.Sprintf("Language '%s' (%s) configuration is valid", lang.Name, lang.Key),
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

	filePath := filepath.Join(h.langDir, key+".yaml")
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

	type detected struct {
		Compilers    []DetectedTool `json:"compilers"`
		Interpreters []DetectedTool `json:"interpreters"`
	}

	result := detected{}

	for _, c := range knownCompilers {
		tool := detectTool(c.Name, c.Binary, c.VersionArgs)
		if tool != nil {
			result.Compilers = append(result.Compilers, *tool)
		}
	}

	for _, i := range knownInterpreters {
		tool := detectTool(i.Name, i.Binary, i.VersionArgs)
		if tool != nil {
			result.Interpreters = append(result.Interpreters, *tool)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func detectTool(name, binary string, versionArgs []string) *DetectedTool {
	info, err := os.Stat(binary)
	if err != nil || info.IsDir() {
		return nil
	}

	version := ""
	if len(versionArgs) > 0 && versionArgs[0] != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, versionArgs...)
		out, err := cmd.CombinedOutput()
		if err == nil {
			version = strings.TrimSpace(string(out))
			lines := strings.Split(version, "\n")
			if len(lines) > 0 {
				version = lines[0]
			}
			if len(version) > 100 {
				version = version[:100] + "..."
			}
		}
	}

	return &DetectedTool{
		Name:    name,
		Path:    binary,
		Version: version,
	}
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
