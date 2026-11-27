package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/judge/compiler"
	"github.com/tahsinarafat/aioj/internal/judge/executor"
)

// The admin panel used to blank the whole page when Detect found nothing: the
// handler built its lists by appending to nil slices, so an install with no
// detected tools marshalled `"compilers": null`, and the panel called .map() on
// that null. The contract is "always an array", which these tests pin.
//
// The second half is the sandbox seam. Detection ran os.Stat against the
// backend container, which (Dockerfile.api) ships no compilers at all — so a
// healthy install reported nothing found. Detection must go through the judge
// sandbox, which is the only place the toolchains exist.

func adminDetectReq() *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/languages/detect", nil)
	return req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey,
		&auth.Claims{UserID: "root", Username: "root", Role: "admin"}))
}

// decodeDetect runs Detect against a fake sandbox and returns the raw JSON, so a
// `null` list is visible rather than being silently normalised by decoding.
func decodeDetect(t *testing.T, srv *httptest.Server) map[string]json.RawMessage {
	t.Helper()
	var exec *executor.Client
	if srv != nil {
		exec = executor.NewClient(srv.URL)
	}
	h := NewAdminLanguageHandler(newMemLanguageStore(), compiler.NewLoader(newMemLanguageStore()), exec)

	rec := httptest.NewRecorder()
	h.Detect(rec, adminDetectReq())

	if rec.Code != http.StatusOK {
		t.Fatalf("Detect status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("Detect returned invalid JSON: %v (body: %s)", err, rec.Body.String())
	}
	return body
}

// TestDetectReturnsArraysWhenNothingFound is the regression test for the blank
// page. `null` here means the React panel throws and the admin page goes white.
func TestDetectReturnsArraysWhenNothingFound(t *testing.T) {
	// A sandbox that answers "no such tool" for every probe.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]executor.CmdResult{{Status: "Nonzero Exit Status", ExitStatus: 127}})
	}))
	defer srv.Close()

	body := decodeDetect(t, srv)

	for _, key := range []string{"compilers", "interpreters"} {
		raw, ok := body[key]
		if !ok {
			t.Fatalf("response has no %q key", key)
		}
		if string(raw) == "null" {
			t.Fatalf("%q was JSON null; the admin panel calls .map() on it and the "+
				"whole page blanks. It must be an empty array.", key)
		}
		if got := string(raw); got != "[]" {
			t.Fatalf("%q = %s, want []", key, got)
		}
	}
}

// An unreachable judge endpoint must still produce arrays, not a 500 and not
// null: the panel needs "none found", which is honest.
func TestDetectReturnsArraysWhenSandboxUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := srv.URL
	srv.Close() // nothing listening now

	st := newMemLanguageStore()
	h := NewAdminLanguageHandler(st, compiler.NewLoader(st), executor.NewClient(deadURL))

	rec := httptest.NewRecorder()
	h.Detect(rec, adminDetectReq())

	if rec.Code != http.StatusOK {
		t.Fatalf("Detect status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var body struct {
		Compilers    []DetectedTool `json:"compilers"`
		Interpreters []DetectedTool `json:"interpreters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Compilers == nil || body.Interpreters == nil {
		t.Fatalf("an unreachable sandbox produced a nil list: compilers=%v interpreters=%v",
			body.Compilers, body.Interpreters)
	}
}

// No executor client at all (server misconfigured) must not panic or emit null.
func TestDetectWithoutExecutorReturnsArrays(t *testing.T) {
	st := newMemLanguageStore()
	h := NewAdminLanguageHandler(st, compiler.NewLoader(st), nil)

	rec := httptest.NewRecorder()
	h.Detect(rec, adminDetectReq())

	if rec.Code != http.StatusOK {
		t.Fatalf("Detect status = %d, want 200", rec.Code)
	}
	var body struct {
		Compilers    []DetectedTool `json:"compilers"`
		Interpreters []DetectedTool `json:"interpreters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Compilers == nil || body.Interpreters == nil {
		t.Fatalf("nil list: compilers=%v interpreters=%v", body.Compilers, body.Interpreters)
	}
}

// Detection must ask the sandbox, not the local filesystem. This asserts the
// request actually reached the fake judge server.
func TestDetectProbesTheSandbox(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		json.NewEncoder(w).Encode([]executor.CmdResult{{Status: "Nonzero Exit Status", ExitStatus: 127}})
	}))
	defer srv.Close()

	decodeDetect(t, srv)

	if calls == 0 {
		t.Fatal("Detect never called the judge sandbox; it is probing the backend " +
			"filesystem, where no compilers are installed, so it always finds nothing")
	}
}

// A tool whose version command exits non-zero (javac, java, erl all do) must
// still be reported. Only "command not found" (exit 127) means absent.
func TestDetectTreatsNonZeroVersionExitAsPresent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req executor.ExecRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		script := req.Cmd[0].Args[len(req.Cmd[0].Args)-1]

		// The listing of PATH is what discovery enumerates.
		if strings.Contains(script, "ls -1") {
			json.NewEncoder(w).Encode([]executor.CmdResult{{
				Status: "Accepted",
				Files:  map[string]string{"stdout": "javac\n"},
			}})
			return
		}
		// `command -v` resolves it...
		if strings.Contains(script, "command -v") {
			json.NewEncoder(w).Encode([]executor.CmdResult{{
				Status:     "Nonzero Exit Status",
				ExitStatus: 1,
				Files:      map[string]string{"path.txt": "/usr/bin/javac", "version.txt": "javac 21.0.1"},
			}})
			return
		}
		// ...and the version banner prints but exits non-zero, as javac does.
		json.NewEncoder(w).Encode([]executor.CmdResult{{
			Status:     "Nonzero Exit Status",
			ExitStatus: 1,
			Files:      map[string]string{"version.txt": "javac 21.0.1"},
		}})
	}))
	defer srv.Close()

	body := decodeDetect(t, srv)

	var all []DetectedTool
	for _, key := range []string{"compilers", "interpreters"} {
		var list []DetectedTool
		if err := json.Unmarshal(body[key], &list); err != nil {
			t.Fatal(err)
		}
		all = append(all, list...)
	}

	if len(all) == 0 {
		t.Fatal("a tool that prints its version and exits 1 was reported absent; " +
			"javac/java/erl behave exactly this way, so they would all vanish")
	}
	if all[0].Version != "javac 21.0.1" {
		t.Errorf("version = %q, want the banner the tool printed", all[0].Version)
	}
	// javac compiles, so it belongs with the compilers regardless of which hint
	// table happens to name it.
	if all[0].Class != "compiler" {
		t.Errorf("class = %q, want compiler for javac", all[0].Class)
	}
}

// Discovery enumerates the judge image rather than walking a list maintained in
// this file. A tool the image has but the table never named must still appear —
// that drift is exactly how an installed toolchain read as "not detected".
func TestDetectFindsAToolNobodyListed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req executor.ExecRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		script := req.Cmd[0].Args[len(req.Cmd[0].Args)-1]
		out := func(files map[string]string, status ...int) {
			code := 0
			if len(status) > 0 {
				code = status[0]
			}
			json.NewEncoder(w).Encode([]executor.CmdResult{{
				Status:     "Nonzero Exit Status",
				ExitStatus: code,
				Files:      files,
			}})
		}

		switch {
		case strings.Contains(script, "ls -1"):
			// The unversioned name is absent on this image: only the versioned
			// binary exists, which is the case a hardcoded table misses.
			out(map[string]string{"stdout": "mytool\npython3.12\n"})
		case strings.Contains(script, "command -v"):
			name := strings.Trim(toolNameFromScript(script), "'")
			out(map[string]string{
				"path.txt":    "/usr/bin/" + name,
				"version.txt": name + " 1.0",
			}, 1)
		default:
			out(map[string]string{}, 1)
		}
	}))
	defer srv.Close()

	body := decodeDetect(t, srv)

	var all []DetectedTool
	for _, key := range []string{"compilers", "interpreters"} {
		var list []DetectedTool
		if err := json.Unmarshal(body[key], &list); err != nil {
			t.Fatal(err)
		}
		all = append(all, list...)
	}

	// The unlisted tool is reported under its own name, since no hint names it.
	var found bool
	for _, d := range all {
		if d.Name == "mytool" && d.Path == "/usr/bin/mytool" {
			found = true
		}
		// A versioned sibling still resolves to the shipped label, so the panel
		// reads "Python 3" rather than an unfamiliar name.
		if d.Binary == "python3.12" && d.Name != "Python 3" {
			t.Errorf("python3.12 was labelled %q, want the shipped name for the tool", d.Name)
		}
	}
	if !found {
		t.Fatalf("a tool present in the image but absent from the hint tables was not detected: %+v", all)
	}
}

// The 32-bit variants must stay distinguishable from their 64-bit siblings.
// Reporting "GCC 32-bit" from a bare `-m32 --version` used to be a false
// positive — that succeeds on a toolchain with no 32-bit multilib — so the
// label is only worth showing when the variant genuinely differs, which it does
// here by name.
func TestThirtyTwoBitHintsAreDistinctFrom64Bit(t *testing.T) {
	var saw32 bool
	for _, tool := range knownCompilers {
		if !isThirtyTwoBit(tool.Name) {
			continue
		}
		saw32 = true
		if tool.Binary == "" {
			t.Fatalf("%s has no binary name, so discovery cannot label a 32-bit find", tool.Name)
		}
	}
	if !saw32 {
		t.Fatal("no 32-bit hints remain; the panel would have no name for a multilib toolchain")
	}
}

// Discovery must confirm a toolchain can do its job, not merely that a file with
// its name exists. A binary that is present but cannot produce a working
// executable is not a compiler, and offering it produces a language whose every
// submission fails.
func TestDetectChecksAnUnknownToolActuallyCompiles(t *testing.T) {
	var compileProbed bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req executor.ExecRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		script := req.Cmd[0].Args[len(req.Cmd[0].Args)-1]

		switch {
		case strings.Contains(script, "ls -1"):
			json.NewEncoder(w).Encode([]executor.CmdResult{{
				Status: "Accepted",
				Files:  map[string]string{"stdout": "mysterycc\n"},
			}})
		case strings.Contains(script, "command -v"):
			json.NewEncoder(w).Encode([]executor.CmdResult{{
				Status:     "Nonzero Exit Status",
				ExitStatus: 1,
				Files:      map[string]string{"path.txt": "/usr/bin/mysterycc", "version.txt": "mysterycc 9"},
			}})
		case strings.Contains(script, "compile.txt"):
			compileProbed = true
			// Present, but it cannot link anything.
			json.NewEncoder(w).Encode([]executor.CmdResult{{
				Status:     "Nonzero Exit Status",
				ExitStatus: 1,
			}})
		default:
			json.NewEncoder(w).Encode([]executor.CmdResult{{Status: "Accepted"}})
		}
	}))
	defer srv.Close()

	body := decodeDetect(t, srv)

	if !compileProbed {
		t.Fatal("an unrecognised binary was never asked to compile anything; " +
			"classifying by name alone would file any executable as a compiler")
	}
	var compilers []DetectedTool
	if err := json.Unmarshal(body["compilers"], &compilers); err != nil {
		t.Fatal(err)
	}
	for _, c := range compilers {
		if c.Binary == "mysterycc" {
			t.Fatalf("%s was filed as a compiler although it cannot produce an executable", c.Binary)
		}
	}
}

func isThirtyTwoBit(name string) bool {
	return strings.Contains(name, "32-bit")
}

// Discovery must search the same PATH a submission gets. A tool found anywhere
// else could not be exec'd by a compile or a run, so reporting it would hand the
// operator a language that fails the moment anyone submits to it.
func TestDetectSearchesTheSandboxRunPath(t *testing.T) {
	var seenPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req executor.ExecRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		for _, e := range req.Cmd[0].Env {
			if strings.HasPrefix(e, "PATH=") {
				seenPath = e
			}
		}
		json.NewEncoder(w).Encode([]executor.CmdResult{{Status: "Accepted"}})
	}))
	defer srv.Close()

	decodeDetect(t, srv)

	if !strings.Contains(seenPath, "PATH="+detectionSearchPath) {
		t.Fatalf("probe env = %q, want PATH=%s — the same path internal/judge sets for a real run",
			seenPath, detectionSearchPath)
	}
}

// A name filter is not a verdict, but a version-suffixed sibling must not be
// dropped either: on an image where only `python3.12` exists, it is the name a
// lang/*.yaml has to reference. Unversioned names are probed first, since they
// are the ones a config should prefer.
func TestCandidateNamesKeepsVersionedBinariesAndDropsShellNoise(t *testing.T) {
	got := candidateNames("sh\npython3.12\npython3\nmytool\n.bashrc\n")

	want := []string{"python3", "mytool", "python3.12"}
	if len(got) != len(want) {
		t.Fatalf("candidates = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("candidates = %v, want %v", got, want)
		}
	}
}

// A tool name reaches a shell command line, so it has to be quoted. The names
// come from the image's own bin directories, which is exactly the kind of input
// that should not be trusted with a bare interpolation.
func TestShellQuoteNeutralisesAWordSplit(t *testing.T) {
	got := shellQuote("my tool; rm -rf /")

	// The whole string is one single-quoted word, so the shell passes it to the
	// command as a single argument instead of running the second command.
	if got != `'my tool; rm -rf /'` {
		t.Fatalf("quoted = %q, want the name as one single-quoted argument", got)
	}
	// An embedded quote must not end the quoting early.
	if got := shellQuote("a'b"); got != `'a'\''b'` {
		t.Fatalf("quoted = %q, want the embedded quote escaped", got)
	}
}

// One Detect click must not become an unbounded fan-out of sandbox processes on
// a judge container shared with live submissions.
func TestDetectionCapsHowManyBinariesItProbes(t *testing.T) {
	var probes int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req executor.ExecRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		script := req.Cmd[0].Args[len(req.Cmd[0].Args)-1]
		if strings.Contains(script, "ls -1") {
			// A pathological image: far more executables than any toolchain.
			var b strings.Builder
			for i := 0; i < maxProbesPerDetection*3; i++ {
				fmt.Fprintf(&b, "tool%d\n", i)
			}
			json.NewEncoder(w).Encode([]executor.CmdResult{{
				Status: "Accepted",
				Files:  map[string]string{"stdout": b.String()},
			}})
			return
		}
		if strings.Contains(script, "command -v") {
			probes++
		}
		json.NewEncoder(w).Encode([]executor.CmdResult{{Status: "Accepted"}})
	}))
	defer srv.Close()

	decodeDetect(t, srv)

	if probes > maxProbesPerDetection {
		t.Fatalf("%d version probes for one Detect, cap is %d", probes, maxProbesPerDetection)
	}
}
