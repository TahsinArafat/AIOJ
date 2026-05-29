package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type CmdFile struct {
	Content string `json:"content"`
	Src     string `json:"src,omitempty"`
}

type Cmd struct {
	Args        []string           `json:"args"`
	Env         []string           `json:"env,omitempty"`
	Files       []CmdFile          `json:"files,omitempty"`
	CPULimit    uint64             `json:"cpuLimit"`
	MemoryLimit uint64             `json:"memoryLimit"`
	ProcLimit   uint64             `json:"procLimit"`
	CopyIn      map[string]CmdFile `json:"copyIn,omitempty"`
	CopyOut     []string           `json:"copyOut,omitempty"`
	CopyOutDir  string             `json:"copyOutDir,omitempty"`
}

type ExecRequest struct {
	Cmd       []Cmd `json:"cmd"`
	PipeInput bool  `json:"pipeInput,omitempty"`
}

type CmdResult struct {
	Status     string             `json:"status"`
	ExitStatus int                `json:"exitStatus"`
	Error      string             `json:"error,omitempty"`
	Time       uint64             `json:"time"`
	Memory     uint64             `json:"memory"`
	RunDir     string             `json:"runDir"`
	Files      map[string]string  `json:"files,omitempty"`
}

type Client struct {
	endpoint string
	http     *http.Client
}

func NewClient(endpoint string) *Client {
	return &Client{endpoint: endpoint, http: &http.Client{Timeout: 30 * time.Second}}
}

func (c *Client) Run(req *ExecRequest) ([]CmdResult, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	slog.Info("executor request", "body_len", len(body), "cmds", len(req.Cmd))
	if len(req.Cmd) > 0 {
		slog.Info("executor cmd", "args", req.Cmd[0].Args, "copyIn_keys", len(req.Cmd[0].CopyIn), "files_len", len(req.Cmd[0].Files))
	}
	resp, err := c.http.Post(c.endpoint+"/run", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	slog.Info("executor response", "status", resp.StatusCode, "body_len", len(respBody))
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
