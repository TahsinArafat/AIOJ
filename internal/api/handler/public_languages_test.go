package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// The IDE and the submit forms used to carry their own hardcoded language
// list, so an admin edit in admin#languages never reached them. This endpoint
// is the seam that makes lang/*.yaml the single source of truth: whatever the
// files say is what the dropdowns offer.

func seedLangs(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"cpp-gpp-64.yaml": "name: C++ (G++ 64-bit)\nkey: cpp-gpp-64\ncompile: /usr/bin/g++ -o {{exe}} {{src}}\nextensions: [.cpp, .cxx]\n",
		"python.yaml":     "name: Python 3\nkey: python\nruntime: /usr/bin/python3\nextensions: [.py]\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}
	return dir
}

func listPublicLanguages(t *testing.T, dir string) []PublicLanguage {
	t.Helper()
	h := NewPublicLanguageHandler(dir)
	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/api/languages", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("List status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var body struct {
		Data []PublicLanguage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v (body: %s)", err, rec.Body.String())
	}
	return body.Data
}

// TestPublicLanguagesReflectTheFiles is the regression test for the original
// complaint: the IDE and the problem pages must read the judge's config, not a
// hardcoded list baked into the bundle.
func TestPublicLanguagesReflectTheFiles(t *testing.T) {
	got := listPublicLanguages(t, seedLangs(t))

	if len(got) != 2 {
		t.Fatalf("got %d languages, want the 2 that exist in lang/: %+v", len(got), got)
	}
	// Sorted by key, so the dropdown order is stable across reloads.
	if got[0].Key != "cpp-gpp-64" || got[1].Key != "python" {
		t.Errorf("order = [%s, %s], want [cpp-gpp-64, python]", got[0].Key, got[1].Key)
	}
	if got[0].Name != "C++ (G++ 64-bit)" {
		t.Errorf("name = %q, want the name from the YAML file", got[0].Name)
	}
	if len(got[0].Extensions) != 2 {
		t.Errorf("extensions = %v, want the two from the YAML file", got[0].Extensions)
	}
}

// TestPublicLanguagesFollowAnAdminEdit pins the live-read behaviour: the panel
// writes lang/<key>.yaml, and the very next request must show the new name.
// A cached or hardcoded list would keep serving the old one.
func TestPublicLanguagesFollowAnAdminEdit(t *testing.T) {
	dir := seedLangs(t)

	h := NewPublicLanguageHandler(dir)
	before := httptest.NewRecorder()
	h.List(before, httptest.NewRequest(http.MethodGet, "/api/languages", nil))

	renamed := "name: C++ 20\nkey: cpp-gpp-64\ncompile: /usr/bin/g++ -std=c++20 -o {{exe}} {{src}}\nextensions: [.cpp]\n"
	if err := os.WriteFile(filepath.Join(dir, "cpp-gpp-64.yaml"), []byte(renamed), 0644); err != nil {
		t.Fatalf("rewrite lang file: %v", err)
	}

	after := listPublicLanguages(t, dir)
	if after[0].Name != "C++ 20" {
		t.Fatalf("name after admin edit = %q, want \"C++ 20\" — the endpoint is not reading the live config", after[0].Name)
	}
}

// TestPublicLanguagesOmitSandboxPaths checks the response shape stays
// display-only. Compile/runtime strings name absolute paths inside the judge
// image; publishing them maps the sandbox for no UI benefit.
func TestPublicLanguagesOmitSandboxPaths(t *testing.T) {
	h := NewPublicLanguageHandler(seedLangs(t))
	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/api/languages", nil))

	var raw struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, lang := range raw.Data {
		for _, leaked := range []string{"compile", "runtime", "copy_in", "seccomp_rule"} {
			if _, ok := lang[leaked]; ok {
				t.Errorf("response exposes %q; only key/name/extensions belong on the public list", leaked)
			}
		}
	}
}

// A language with no extensions must marshal an empty array, not null: the SPA
// maps over this field.
func TestPublicLanguagesNeverMarshalNullExtensions(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "weird.yaml"),
		[]byte("name: Weird\nkey: weird\n"), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	h := NewPublicLanguageHandler(dir)
	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/api/languages", nil))

	var raw struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(raw.Data) != 1 {
		t.Fatalf("got %d entries, want 1", len(raw.Data))
	}
	if got := string(raw.Data[0]["extensions"]); got != "[]" {
		t.Errorf("extensions = %s, want [] (a null would break the SPA's .map)", got)
	}
}

// A missing lang directory must be a loud error. An empty list would render an
// empty dropdown and read as "this judge accepts no languages".
func TestPublicLanguagesFailLoudlyOnMissingDirectory(t *testing.T) {
	h := NewPublicLanguageHandler(filepath.Join(t.TempDir(), "does-not-exist"))
	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/api/languages", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 for a missing lang directory", rec.Code)
	}
}
