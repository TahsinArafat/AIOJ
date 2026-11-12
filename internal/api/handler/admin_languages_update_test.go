package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/judge/compiler"
	"github.com/tahsinarafat/aioj/lang"
	"gopkg.in/yaml.v3"
)

// Saving a language from the admin panel used to erase the fields the form had
// just sent. The handler decodes the request body into compiler.LangConfig,
// which carried only `yaml` tags — so encoding/json matched keys against Go
// field names instead ("compile" vs CompileCmd) and left every mismatched field
// at its zero value. The handler then marshalled that empty struct back over
// the file: the compile command, copy_in, seccomp rule and both multipliers
// were all silently destroyed, which the panel showed as "the command field
// vanished after saving".
//
// These tests drive the real handler over real YAML files, because the bug was
// invisible at the handler boundary — the response was a cheerful 200.

const sampleLang = `name: "C++ (G++ 64-bit)"
key: cpp-gpp-64
compile: /usr/bin/g++ -O2 -std=c++17 -o {{exe}} {{src}}
copy_in:
  extra.h: content
time_limit_multiplier: 1
memory_limit_multiplier: 2
seccomp_rule: c_cpp
extensions:
  - .cpp
`

func adminLangReq(method, url, body string) *http.Request {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, url, nil)
	} else {
		r = httptest.NewRequest(method, url, strings.NewReader(body))
	}
	r = r.WithContext(context.WithValue(r.Context(), middleware.UserContextKey,
		&auth.Claims{UserID: "root", Username: "root", Role: "admin"}))

	// Update/Get read the key from the chi route params, which a bare
	// httptest request does not populate.
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("key", "cpp-gpp-64")
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// writeSampleLang places the canonical config in a temp dir and returns it.
func writeSampleLang(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cpp-gpp-64.yaml"), []byte(sampleLang), 0644); err != nil {
		t.Fatalf("seed lang file: %v", err)
	}
	return dir
}

func readLangFile(t *testing.T, dir string) compiler.LangConfig {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "cpp-gpp-64.yaml"))
	if err != nil {
		t.Fatalf("read saved lang file: %v", err)
	}
	var cfg compiler.LangConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("saved lang file is not valid YAML: %v\n%s", err, data)
	}
	return cfg
}

// TestUpdateKeepsCompileCommand is the direct regression test: submit the exact
// JSON body the panel produces and require the compile command to survive.
func TestUpdateKeepsCompileCommand(t *testing.T) {
	dir := writeSampleLang(t)
	h := NewAdminLanguageHandler(dir, nil)

	// Shape mirrors LanguageConfigResponse, which is what the panel PUTs back.
	body := `{
	  "name": "C++ (G++ 64-bit)",
	  "key": "cpp-gpp-64",
	  "compile": "/usr/bin/g++ -O2 -std=c++17 -o {{exe}} {{src}}",
	  "runtime": "",
	  "copy_in": {"extra.h": "content"},
	  "time_limit_multiplier": 1,
	  "memory_limit_multiplier": 2,
	  "seccomp_rule": "c_cpp",
	  "extensions": [".cpp"],
	  "mono": false
	}`

	rec := httptest.NewRecorder()
	h.Update(rec, adminLangReq(http.MethodPut, "/api/admin/languages/cpp-gpp-64", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("Update status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	got := readLangFile(t, dir)
	if got.CompileCmd == "" {
		t.Fatal("compile command was erased by save — the admin form's compile field vanishes after saving")
	}
	if got.CompileCmd != "/usr/bin/g++ -O2 -std=c++17 -o {{exe}} {{src}}" {
		t.Errorf("compile = %q, want the saved command preserved", got.CompileCmd)
	}
	if got.SeccompRule != "c_cpp" {
		t.Errorf("seccomp_rule = %q, want c_cpp", got.SeccompRule)
	}
	if got.MemoryLimitMultiplier != 2 {
		t.Errorf("memory_limit_multiplier = %v, want 2", got.MemoryLimitMultiplier)
	}
	if got.TimeLimitMultiplier != 1 {
		t.Errorf("time_limit_multiplier = %v, want 1", got.TimeLimitMultiplier)
	}
	if got.CopyIn["extra.h"] != "content" {
		t.Errorf("copy_in = %v, want the extra.h entry preserved", got.CopyIn)
	}
	if len(got.Extensions) != 1 || got.Extensions[0] != ".cpp" {
		t.Errorf("extensions = %v, want [.cpp]", got.Extensions)
	}
}

// TestUpdateRoundTripsRuntimeForInterpretedLanguages covers the other half of
// the same tag bug: interpreted languages carry no compile command at all, so
// an erased runtime leaves them completely unrunnable.
func TestUpdateRoundTripsRuntimeForInterpretedLanguages(t *testing.T) {
	dir := t.TempDir()
	seed := "name: Python 3\nkey: python\ncompile: \"\"\nruntime: /usr/bin/python3\nseccomp_rule: general\nextensions: [.py]\n"
	if err := os.WriteFile(filepath.Join(dir, "python.yaml"), []byte(seed), 0644); err != nil {
		t.Fatalf("seed lang file: %v", err)
	}

	h := NewAdminLanguageHandler(dir, nil)
	body := `{"name":"Python 3","key":"python","compile":"","runtime":"/usr/bin/python3",
	          "time_limit_multiplier":3,"memory_limit_multiplier":2,
	          "seccomp_rule":"general","extensions":[".py"]}`

	req := adminLangReq(http.MethodPut, "/api/admin/languages/python", body)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("key", "python")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Update status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	data, err := os.ReadFile(filepath.Join(dir, "python.yaml"))
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	var got compiler.LangConfig
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}
	if got.Runtime != "/usr/bin/python3" {
		t.Fatalf("runtime = %q, want /usr/bin/python3 — an erased runtime makes every Python submission unrunnable", got.Runtime)
	}
	if got.TimeLimitMultiplier != 3 {
		t.Errorf("time_limit_multiplier = %v, want 3", got.TimeLimitMultiplier)
	}
}

// TestLangConfigJSONFieldNames pins the wire names independently of the handler,
// so a future edit to the struct cannot quietly reintroduce the mismatch.
func TestLangConfigJSONFieldNames(t *testing.T) {
	raw, err := json.Marshal(compiler.LangConfig{CompileCmd: "x", CopyIn: map[string]string{"a": "b"}, Runtime: "y"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"compile", "copy_in", "runtime"} {
		if _, ok := wire[key]; !ok {
			t.Errorf("LangConfig marshals without %q; the admin form sends that exact key and it would be dropped: %s", key, raw)
		}
	}
}

// Reset restores the config that ships in lang/*.yaml. The panel edits those
// files in place, so without this a save that drops a field (or a hand edit)
// leaves a language broken until the next deploy.
func TestResetRestoresTheShippedConfig(t *testing.T) {
	dir := t.TempDir()
	// A config an operator has mangled: no compile command, a bogus multiplier.
	broken := "name: mangled\nkey: cpp-gpp-64\ncompile: \"\"\ntime_limit_multiplier: 9\nseccomp_rule: nope\nextensions: []\n"
	if err := os.WriteFile(filepath.Join(dir, "cpp-gpp-64.yaml"), []byte(broken), 0644); err != nil {
		t.Fatalf("seed broken config: %v", err)
	}

	h := NewAdminLanguageHandler(dir, nil)
	rec := httptest.NewRecorder()
	h.Reset(rec, adminLangReq(http.MethodPost, "/api/admin/languages/cpp-gpp-64/reset", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("Reset status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	got := readLangFile(t, dir)
	if got.CompileCmd == "" {
		t.Fatal("reset left the compile command empty; it must restore the shipped one")
	}
	if got.Name == "mangled" {
		t.Error("reset did not replace the mangled name")
	}
	if got.SeccompRule == "nope" {
		t.Error("reset did not restore the shipped seccomp rule")
	}
	// The restored config must be the shipped file, byte for byte.
	shipped, ok := lang.Default("cpp-gpp-64")
	if !ok {
		t.Fatal("no embeddable default for cpp-gpp-64")
	}
	onDisk, err := os.ReadFile(filepath.Join(dir, "cpp-gpp-64.yaml"))
	if err != nil {
		t.Fatalf("read reset file: %v", err)
	}
	if string(onDisk) != string(shipped) {
		t.Errorf("reset wrote %q, want the shipped lang/cpp-gpp-64.yaml", string(onDisk))
	}
}

// A language an operator created has no shipped default, so Reset must say so
// rather than writing nothing and reporting success.
func TestResetRefusesLanguageWithoutADefault(t *testing.T) {
	dir := t.TempDir()
	h := NewAdminLanguageHandler(dir, nil)

	req := adminLangReq(http.MethodPost, "/api/admin/languages/operator-made/reset", "")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("key", "operator-made")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Reset(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("Reset status = %d, want 404 for a language with no default", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "no built-in default") {
		t.Errorf("body = %q, want an explanation of why nothing was restored", rec.Body.String())
	}
}
