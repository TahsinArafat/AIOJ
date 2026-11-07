package plagiarism

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Environment keys controlling engine selection.
const (
	// EngineEnv picks an engine: "auto" (default), "native", or "rust".
	EngineEnv = "PLAGIARISM_ENGINE"
	// BinaryEnv points at the aioj-plagiarism binary. Setting it implies the
	// Rust engine is available; pointing it at a missing file is a startup error
	// rather than a silent downgrade.
	BinaryEnv = "PLAGIARISM_RUST_BIN"
)

// DefaultBinaryName is the executable the Dockerfile installs and the name
// looked up on PATH and in the usual build directories.
const DefaultBinaryName = "aioj-plagiarism"

// RustEngine runs the similarity scan in the `aioj-plagiarism` process.
//
// It is a subprocess rather than cgo or a resident service on purpose:
//
//   - no cgo means the backend keeps building with CGO_ENABLED=0, the alpine
//     image stays small, and the Go build is not coupled to a Rust toolchain;
//   - no resident service means no new port, no new auth surface, and nothing to
//     keep alive — a scan is a short-lived, bounded, CPU-bound job;
//   - a crash or a protocol change surfaces as an error the service can fall
//     back from, instead of a corrupted report.
//
// One process handles one whole report, so the fixed cost is a single fork/exec
// and one JSON request.
type RustEngine struct {
	bin string
}

// NewRustEngine returns an engine backed by the binary at bin.
func NewRustEngine(bin string) *RustEngine { return &RustEngine{bin: bin} }

// Name implements Engine.
func (e *RustEngine) Name() string { return "rust" }

// Binary reports the executable path, for logs and diagnostics.
func (e *RustEngine) Binary() string { return e.bin }

// ---- wire format (mirrors rust/plagiarism/src/main.rs) ---------------------

type rustRequest struct {
	Threshold float64     `json:"threshold"`
	Inputs    []rustInput `json:"inputs"`
}

type rustInput struct {
	Key       string `json:"key"`
	ProblemID string `json:"problem_id"`
	UserID    string `json:"user_id"`
	Code      string `json:"code"`
}

type rustPairPayload struct {
	ProblemID     string  `json:"problem_id"`
	A             string  `json:"a"`
	B             string  `json:"b"`
	Similarity    float64 `json:"similarity"`
	MatchedTokens int     `json:"matched_tokens"`
}

type rustSummary struct {
	Inputs      int    `json:"inputs"`
	PairsTotal  int    `json:"pairs_total"`
	PairsScored int    `json:"pairs_scored"`
	Flagged     int    `json:"flagged"`
	Engine      string `json:"engine"`
	ElapsedMS   int64  `json:"elapsed_ms"`
}

// rustLine is one newline-delimited response object. Exactly one field is set
// per line, which is why all three are pointers.
type rustLine struct {
	Pair    *rustPairPayload `json:"pair"`
	Summary *rustSummary     `json:"summary"`
	Error   string           `json:"error"`
}

// FindSimilar implements Engine by streaming the batch to the subprocess.
func (e *RustEngine) FindSimilar(ctx context.Context, sources []Source, threshold float64) (*ScanResult, error) {
	req := rustRequest{
		Threshold: threshold,
		Inputs:    make([]rustInput, len(sources)),
	}
	for i, s := range sources {
		req.Inputs[i] = rustInput{
			Key:       s.Key,
			ProblemID: s.ProblemID,
			UserID:    s.UserID,
			Code:      s.Code,
		}
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	cmd := exec.CommandContext(ctx, e.bin)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	result, parseErr := parseRustOutput(&stdout)
	if parseErr != nil {
		// The engine always writes a JSON error line before exiting non-zero, so
		// a parse failure means the binary is not what we think it is.
		msg := strings.TrimSpace(stderr.String())
		if runErr != nil {
			return nil, fmt.Errorf("run %s: %w%s", e.bin, runErr, indent(msg))
		}
		return nil, fmt.Errorf("decode response from %s: %w%s", e.bin, parseErr, indent(msg))
	}
	if result.Err != "" {
		return nil, fmt.Errorf("%s: %s", e.bin, result.Err)
	}
	if runErr != nil {
		return nil, fmt.Errorf("run %s: %w", e.bin, runErr)
	}

	out := &ScanResult{
		Pairs:       result.Pairs,
		PairsTotal:  result.PairsTotal,
		PairsScored: result.PairsScored,
	}
	if result.Summary != nil {
		slog.Debug("plagiarism: rust scan finished",
			"inputs", result.Summary.Inputs,
			"pairs_total", result.Summary.PairsTotal,
			"pairs_scored", result.Summary.PairsScored,
			"flagged", result.Summary.Flagged,
			"elapsed_ms", result.Summary.ElapsedMS,
		)
	}
	if len(out.Pairs) > 1 {
		// The engine sorts already; sorting again keeps the Go-side guarantee
		// independent of the wire order.
		sortPairs(out.Pairs)
	}
	return out, nil
}

type rustDecoded struct {
	Pairs       []Pair
	PairsTotal  int
	PairsScored int
	Summary     *rustSummary
	Err         string
}

func parseRustOutput(stdout *bytes.Buffer) (*rustDecoded, error) {
	out := &rustDecoded{Pairs: []Pair{}}
	dec := json.NewDecoder(stdout)
	for {
		var line rustLine
		err := dec.Decode(&line)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch {
		case line.Error != "":
			out.Err = line.Error
		case line.Pair != nil:
			out.Pairs = append(out.Pairs, Pair{
				ProblemID:     line.Pair.ProblemID,
				A:             line.Pair.A,
				B:             line.Pair.B,
				Similarity:    line.Pair.Similarity,
				MatchedTokens: line.Pair.MatchedTokens,
			})
		case line.Summary != nil:
			out.Summary = line.Summary
			out.PairsTotal = line.Summary.PairsTotal
			out.PairsScored = line.Summary.PairsScored
		}
	}
	return out, nil
}

func indent(s string) string {
	if s == "" {
		return ""
	}
	return ": " + s
}

// SelectEngine builds the engine described by the environment.
//
// The default is "auto": use the Rust binary when it is present and the pure-Go
// engine otherwise. That makes a checkout without a Rust toolchain work with no
// configuration, while a deployment that ships the binary gets it automatically.
// Explicit requests fail loudly instead of downgrading, because an operator who
// wrote PLAGIARISM_ENGINE=rust wants to know it is not in effect.
func SelectEngine() (Engine, error) {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv(EngineEnv)))

	switch mode {
	case "native":
		return NativeEngine{}, nil

	case "rust":
		bin, err := locateRustBinary()
		if err != nil {
			return nil, fmt.Errorf("%s=rust but the engine is unavailable: %w", EngineEnv, err)
		}
		slog.Info("plagiarism: using the Rust similarity engine", "binary", bin)
		return NewRustEngine(bin), nil

	case "", "auto":
		bin, err := locateRustBinary()
		if err != nil {
			slog.Info("plagiarism: using the pure-Go similarity engine",
				"reason", err.Error(),
				"hint", "build it with `make rust-build`, or set "+BinaryEnv)
			return NativeEngine{}, nil
		}
		slog.Info("plagiarism: using the Rust similarity engine", "binary", bin)
		return NewRustEngine(bin), nil

	default:
		return nil, fmt.Errorf("unknown %s value %q (want auto, native or rust)", EngineEnv, mode)
	}
}

// locateRustBinary finds the engine executable. An explicit BinaryEnv that does
// not exist is an error, not a fallback: the operator meant it.
func locateRustBinary() (string, error) {
	if p := strings.TrimSpace(os.Getenv(BinaryEnv)); p != "" {
		if isExecutable(p) {
			return p, nil
		}
		return "", fmt.Errorf("%s points at %q, which is not an executable file", BinaryEnv, p)
	}

	relative := [][]string{
		{"bin", DefaultBinaryName},
		{"rust", "plagiarism", "target", "release", DefaultBinaryName},
		{"rust", "plagiarism", "target", "debug", DefaultBinaryName},
	}

	// Look both beside the process and at the module root. Tests run with the
	// package directory as their working directory, so without the second root a
	// `go test ./internal/plagiarism` would never find a freshly built binary.
	roots := []string{"."}
	if root, err := moduleRoot(); err == nil && root != "" {
		roots = append(roots, root)
	}

	for _, root := range roots {
		for _, parts := range relative {
			candidate := filepath.Join(append([]string{root}, parts...)...)
			if isExecutable(candidate) {
				return candidate, nil
			}
		}
	}

	if p, err := exec.LookPath(DefaultBinaryName); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("%s was not found (looked in ./bin, <module>/bin, rust/plagiarism/target/{release,debug} and PATH)", DefaultBinaryName)
}

// moduleRoot walks up from the working directory to the directory holding go.mod.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0o111 != 0
}
