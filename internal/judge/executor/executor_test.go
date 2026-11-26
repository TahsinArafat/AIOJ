package executor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCmdFileMarshalPreservesEmptyContent(t *testing.T) {
	data, err := json.Marshal(CmdFile{Content: ""})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), `{"content":""}`; got != want {
		t.Fatalf("empty content JSON = %s, want %s", got, want)
	}

	data, err = json.Marshal(CmdFile{FileID: "file-1"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), `{"fileId":"file-1"}`; got != want {
		t.Fatalf("file ID JSON = %s, want %s", got, want)
	}

	// A stream entry must serialise as {"name","max"} and nothing else. go-judge
	// picks the file variant from whichever key is present; emitting content
	// alongside name would make it an invalid file type.
	data, err = json.Marshal(CmdFile{Name: "stdout", Max: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), `{"name":"stdout","max":4096}`; got != want {
		t.Fatalf("stream JSON = %s, want %s", got, want)
	}
}

func TestRun_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]CmdResult{{Status: "Accepted", ExitStatus: 0, Time: 1000000, Memory: 4096}})
	}))
	defer srv.Close()
	client := NewClient(srv.URL)
	resp, err := client.Run(context.Background(), &ExecRequest{Cmd: []Cmd{{Args: []string{"/bin/echo", "hi"}, CPULimit: 1e9, MemoryLimit: 1 << 26}}})
	if err != nil {
		t.Fatal(err)
	}
	if resp[0].Status != "Accepted" {
		t.Fatalf("got %s", resp[0].Status)
	}
}

func TestRun_Error(t *testing.T) {
	_, err := NewClient("http://localhost:19999").Run(context.Background(), &ExecRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
}

// TestRun_ContextCancelled verifies that a cancelled context aborts the
// request. Sandbox validation relies on this so a job timeout cannot leave a
// worker goroutine blocked on go-judge indefinitely.
func TestRun_ContextCancelled(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	defer func() { close(block); srv.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewClient(srv.URL).Run(ctx, &ExecRequest{Cmd: []Cmd{{Args: []string{"/bin/echo"}}}})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

// go-judge applies rlimits itself before exec, so `stackLimit` on the request is
// the only thing that can raise RLIMIT_STACK. The wire name is pinned here
// because a typo would be silently ignored: go-judge decodes unknown JSON fields
// without complaint and the program would just run under the sandbox default.
func TestCmdMarshalsStackLimit(t *testing.T) {
	raw, err := json.Marshal(Cmd{Args: []string{"./Main"}, StackLimit: 268435456})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got, ok := wire["stackLimit"]
	if !ok {
		t.Fatalf("no stackLimit on the wire: %s", raw)
	}
	// go-judge takes bytes as a plain integer, not a string or a duration.
	if got != float64(268435456) {
		t.Errorf("stackLimit = %v, want 268435456 bytes", got)
	}
}

// An unset limit must be omitted rather than sent as 0, so the distinction
// between "no limit configured" and "a zero limit" survives the wire.
func TestCmdOmitsAnUnsetStackLimit(t *testing.T) {
	raw, err := json.Marshal(Cmd{Args: []string{"./Main"}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := wire["stackLimit"]; ok {
		t.Errorf("stackLimit present for an unset limit: %s", raw)
	}
}
