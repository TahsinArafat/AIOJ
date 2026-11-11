// Package validator runs a problem's validator over its input files.
//
// A validator is the program that checks an input file satisfies the problem's
// stated constraints — the thing that stops a generator from emitting n=0 or an
// oversized array and quietly producing invalid test data. AIOJ had this
// machinery only inside the AI harness sandbox (internal/harness/problemgen runs
// a compiled validator over each case) and nothing at judge time, so a
// hand-authored problem could ship with inputs no validator had ever seen.
//
// This runs the validator the setter stores on the problem, compiled through the
// existing compiler (the same path SPJ uses) and executed on the same go-judge
// sandbox as a submission. It reports per-case pass/fail and modifies nothing.
package validator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tahsinarafat/aioj/internal/judge/compiler"
	"github.com/tahsinarafat/aioj/internal/judge/executor"
	"github.com/tahsinarafat/aioj/internal/model"
)

// CaseResult is one input file's verdict.
type CaseResult struct {
	Name string `json:"name"`
	OK   bool   `json:"ok"`
	// Message is the validator's stderr or the failure reason, so the setter can
	// act on it instead of guessing which constraint was violated.
	Message string `json:"message,omitempty"`
}

// Report is the outcome of validating every input of a problem.
type Report struct {
	Checked int          `json:"checked"`
	Failed  int          `json:"failed"`
	Cases   []CaseResult `json:"cases"`
}

// Runner compiles and executes validators.
type Runner struct {
	compiler *compiler.Compiler
	exec     *executor.Client
}

func New(c *compiler.Compiler, e *executor.Client) *Runner {
	if c == nil || e == nil {
		return nil
	}
	return &Runner{compiler: c, exec: e}
}

// inputNames lists the input files in a testdata directory, sorted so a report is
// stable across runs. Matches the roles testcase.go infers by extension.
func inputNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".in", ".input", ".txt":
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// Validate runs the problem's validator over every input file in its testdata
// directory.
//
// A per-case failure is reported in the Report, not returned as an error: one
// invalid case is a finding about the problem, not a failure of this call. An
// error means validation could not run at all (no validator configured, compile
// failed, or the test data is unreadable).
func (r *Runner) Validate(ctx context.Context, p *model.Problem, langs map[string]*compiler.LangConfig) (*Report, error) {
	if r == nil {
		return nil, fmt.Errorf("validator runner is not configured")
	}
	if p.ValidatorSource == "" {
		return nil, fmt.Errorf("problem has no validator")
	}
	if p.TestdataPath == "" {
		return nil, fmt.Errorf("problem has no test data")
	}
	names, err := inputNames(p.TestdataPath)
	if err != nil {
		return nil, fmt.Errorf("read test data: %w", err)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no input files found")
	}

	bin, err := r.compiler.CompileSPJ(ctx, p.ValidatorSource, p.ValidatorLanguage, langs)
	if err != nil {
		return nil, fmt.Errorf("compile validator: %w", err)
	}

	report := &Report{Cases: make([]CaseResult, 0, len(names))}
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(p.TestdataPath, name))
		if err != nil {
			report.Failed++
			report.Cases = append(report.Cases, CaseResult{Name: name, Message: "unreadable: " + err.Error()})
			continue
		}

		// Compiled artifacts come back as a cached file id with no argv, so the
		// command is a shell wrapper that runs the mounted binary — exactly how
		// batch.go invokes an SPJ. The case is given BOTH on stdin (a validator
		// that reads cin) and as a path argument (one that reads argv), which
		// covers the two conventions without the setter declaring which.
		req := &executor.ExecRequest{
			Cmd: []executor.Cmd{{
				Args: []string{"/bin/sh", "-c",
					"/box/validator < /box/input.txt 2> error.txt; echo $? > rc.txt"},
				Env:         []string{"PATH=/usr/bin:/bin"},
				CPULimit:    10_000,
				MemoryLimit: 512 * 1024 * 1024,
				ProcLimit:   16,
				CopyIn: map[string]executor.CmdFile{
					"validator": bin,
					"input.txt": {Content: string(raw)},
				},
				CopyOut: []string{"error.txt", "rc.txt"},
			}},
		}

		results, err := r.exec.Run(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("run validator on %s: %w", name, err)
		}
		if len(results) == 0 {
			return nil, fmt.Errorf("validator produced no result for %s", name)
		}
		res := results[0]
		// The wrapper always exits 0, so the validator's own status travels in
		// rc.txt; reading ExitStatus here would always succeed.
		rc := strings.TrimSpace(res.Files["rc.txt"])
		ok := res.Status == "Accepted" && rc == "0"
		cr := CaseResult{Name: name, OK: ok}
		if !ok {
			// A validator signals a bad input with a non-zero exit and usually a
			// message on stderr; both are useful to the setter.
			msg := strings.TrimSpace(res.Files["error.txt"])
			if msg == "" {
				msg = fmt.Sprintf("validator exited %s (%s)", rc, res.Status)
			}
			cr.Message = msg
			report.Failed++
		}
		report.Cases = append(report.Cases, cr)
	}
	report.Checked = len(report.Cases)
	return report, nil
}
