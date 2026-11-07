package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type CmdFile struct {
	Content string `json:"content,omitempty"`
	FileID  string `json:"fileId,omitempty"`
	Src     string `json:"src,omitempty"`
}

// MarshalJSON emits exactly one go-judge file variant. In particular, an
// empty input file must still contain `"content": ""`; omitempty would turn it
// into `{}`, which go-judge rejects as an invalid file type.
func (f CmdFile) MarshalJSON() ([]byte, error) {
	type wire struct {
		Content *string `json:"content,omitempty"`
		FileID  string  `json:"fileId,omitempty"`
		Src     string  `json:"src,omitempty"`
	}
	if f.FileID != "" {
		return json.Marshal(wire{FileID: f.FileID})
	}
	if f.Src != "" {
		return json.Marshal(wire{Src: f.Src})
	}
	content := f.Content
	return json.Marshal(wire{Content: &content})
}

type Cmd struct {
	Args          []string           `json:"args"`
	Env           []string           `json:"env,omitempty"`
	Files         []CmdFile          `json:"files,omitempty"`
	CPULimit      uint64             `json:"cpuLimit"`
	MemoryLimit   uint64             `json:"memoryLimit"`
	ProcLimit     uint64             `json:"procLimit"`
	CopyIn        map[string]CmdFile `json:"copyIn,omitempty"`
	CopyOut       []string           `json:"copyOut,omitempty"`
	CopyOutCached []string           `json:"copyOutCached,omitempty"`
	CopyOutDir    string             `json:"copyOutDir,omitempty"`
}

type ExecRequest struct {
	Cmd       []Cmd `json:"cmd"`
	PipeInput bool  `json:"pipeInput,omitempty"`
}

type CmdResult struct {
	Status     string            `json:"status"`
	ExitStatus int               `json:"exitStatus"`
	Error      string            `json:"error,omitempty"`
	Time       uint64            `json:"time"`
	Memory     uint64            `json:"memory"`
	RunDir     string            `json:"runDir"`
	Files      map[string]string `json:"files,omitempty"`
	FileIDs    map[string]string `json:"fileIds,omitempty"`
}

type Client struct {
	endpoint string
	http     *http.Client
}

func NewClient(endpoint string) *Client {
	// DisableKeepAlives forces a fresh TCP connection for every request.
	// Sandbox runs are infrequent (one per test case) and go-judge may close
	// idle connections before we reuse them, causing a silent "connection reset
	// by peer" that the Go transport will not retry for non-idempotent POST
	// requests. A fresh connection per call avoids this at negligible cost.
	transport := &http.Transport{DisableKeepAlives: true}
	return &Client{endpoint: endpoint, http: &http.Client{Timeout: 60 * time.Second, Transport: transport}}
}

// Run submits an execution request to go-judge. The provided context controls
// cancellation: if it is cancelled (e.g. because the job timeout fired) the
// HTTP request is aborted and the error is returned immediately.
func (c *Client) Run(ctx context.Context, req *ExecRequest) ([]CmdResult, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	// Debug, not Info: a single judgment runs one command per test case, so
	// Info-level logging here emitted hundreds of lines per second during a
	// contest and drowned the lines that matter.
	slog.Debug("executor request", "body_len", len(body), "cmds", len(req.Cmd))
	if len(req.Cmd) > 0 {
		slog.Debug("executor cmd", "args", req.Cmd[0].Args, "copyIn_keys", len(req.Cmd[0].CopyIn), "files_len", len(req.Cmd[0].Files))
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/run", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	slog.Debug("executor response", "status", resp.StatusCode, "body_len", len(respBody))
	if resp.StatusCode != 200 {
		slog.Error("executor error response", "status", resp.StatusCode, "body", string(respBody[:min(len(respBody), 500)]))
	}
	var results []CmdResult
	if err := json.Unmarshal(respBody, &results); err != nil {
		var errMsg string
		if json.Unmarshal(respBody, &errMsg) == nil {
			return nil, fmt.Errorf("executor error: %s", errMsg)
		}
		return nil, fmt.Errorf("decode: %w (body: %s)", err, string(respBody[:min(len(respBody), 500)]))
	}
	return results, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
