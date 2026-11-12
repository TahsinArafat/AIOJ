package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
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
	h := NewAdminLanguageHandler(t.TempDir(), exec)

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

	h := NewAdminLanguageHandler(t.TempDir(), executor.NewClient(deadURL))

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
	h := &AdminLanguageHandler{langDir: t.TempDir()}

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
		json.NewEncoder(w).Encode([]executor.CmdResult{{Status: "Nonzero Exit Status", ExitStatus: 1}})
	}))
	defer srv.Close()

	body := decodeDetect(t, srv)

	var compilers []DetectedTool
	if err := json.Unmarshal(body["compilers"], &compilers); err != nil {
		t.Fatal(err)
	}
	if len(compilers) == 0 {
		t.Fatal("a tool that prints its version and exits 1 was reported absent; " +
			"javac/java/erl behave exactly this way, so they would all vanish")
	}
}

// The 32-bit entries must be real compile probes. Reporting "GCC 32-bit" from a
// bare `-m32 --version` is a false positive: that succeeds on a toolchain with
// no 32-bit multilib, which is why the panel showed 32-bit as available while
// every 32-bit submission failed to compile.
func TestThirtyTwoBitProbesCompileInsteadOfPrintingVersion(t *testing.T) {
	for _, tool := range knownCompilers {
		if !isThirtyTwoBit(tool.Name) {
			continue
		}
		if !tool.Compiles {
			t.Fatalf("%s is not marked as a compile probe; a version print cannot "+
				"prove 32-bit linking works", tool.Name)
		}
		if !strings.Contains(tool.Probe, "-m32") {
			t.Fatalf("%s probe does not pass -m32: %q", tool.Name, tool.Probe)
		}
	}
}

// A compile probe that the sandbox rejects must be reported absent even though
// the version command would have succeeded.
func TestDetectHidesThirtyTwoBitWhenCompileFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req executor.ExecRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		body := req.Cmd[0].Args[len(req.Cmd[0].Args)-1]
		if strings.Contains(body, "-m32") {
			// Simulate a toolchain without multilib.
			json.NewEncoder(w).Encode([]executor.CmdResult{{
				Status:     "Nonzero Exit Status",
				ExitStatus: 1,
				Files:      map[string]string{"error.txt": "cannot find -lgcc"},
			}})
			return
		}
		json.NewEncoder(w).Encode([]executor.CmdResult{{Status: "Accepted"}})
	}))
	defer srv.Close()

	body := decodeDetect(t, srv)

	var compilers []DetectedTool
	if err := json.Unmarshal(body["compilers"], &compilers); err != nil {
		t.Fatal(err)
	}
	for _, c := range compilers {
		if isThirtyTwoBit(c.Name) {
			t.Fatalf("%s was reported available although its -m32 compile failed", c.Name)
		}
	}
}

func isThirtyTwoBit(name string) bool {
	return strings.Contains(name, "32-bit")
}
