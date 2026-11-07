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
