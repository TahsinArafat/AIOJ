package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type CmdFile struct {
	Content string `json:"content,omitempty"`
	Src     string `json:"src,omitempty"`
}

type Cmd struct {
	Args        []string           `json:"args"`
	Env         []string           `json:"env,omitempty"`
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
	resp, err := c.http.Post(c.endpoint+"/run", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	var results []CmdResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return results, nil
}
