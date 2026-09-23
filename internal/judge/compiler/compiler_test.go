package compiler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tahsinarafat/aioj/internal/judge/executor"
)

// Compiled binaries are not UTF-8. go-judge's raw JSON `files` field replaces
// invalid bytes, which corrupted ELF headers and produced Exec format errors.
// The cache/fileId path is byte-safe and is therefore part of the compiler's
// public HTTP contract.
func TestCompileContestantPreservesBinaryWithFileID(t *testing.T) {
	var got executor.ExecRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode([]executor.CmdResult{{
			Status:  "Accepted",
			FileIDs: map[string]string{"Main": "file-main-1"},
		}})
	}))
	defer srv.Close()

	client := executor.NewClient(srv.URL)
	gotResult, err := New(client).CompileContestant(t.Context(), "int main() {}", &LangConfig{
		Key: "cpp-gpp-64", CompileCmd: "g++ {{src}} -o {{exe}}", Extensions: []string{".cpp"},
	})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if !gotResult.Success {
		t.Fatalf("compile failed: %s", gotResult.Output)
	}
	if len(got.Cmd) != 1 || len(got.Cmd[0].CopyOutCached) != 1 || got.Cmd[0].CopyOutCached[0] != "Main" {
		t.Fatalf("compile request must cache Main, got %#v", got.Cmd)
	}
	if len(got.Cmd[0].CopyOut) != 0 {
		t.Fatalf("binary must not use raw copyOut, got %#v", got.Cmd[0].CopyOut)
	}
	bin, ok := gotResult.Files["Main"]
	if !ok || bin.FileID != "file-main-1" || bin.Content != "" {
		t.Fatalf("compiled Main = %#v, want fileId only", bin)
	}
}
