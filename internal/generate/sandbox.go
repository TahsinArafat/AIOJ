package generate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tahsinarafat/aioj/internal/judge/compiler"
	"github.com/tahsinarafat/aioj/internal/judge/executor"
)

// SandboxValidator compiles AI-generated C++ and runs deterministic smoke
// checks exclusively in go-judge. It never executes generated code on the API host.
type SandboxValidator struct {
	Exec        *executor.Client
	LangDir     string
	StagingRoot string
}

func (v *SandboxValidator) Validate(ctx context.Context, p *Proposal, jobID string) ([]ValidationCheck, error) {
	if v == nil || v.Exec == nil {
		return nil, fmt.Errorf("sandbox executor is unavailable")
	}
	langs, err := compiler.LoadLanguages(v.LangDir)
	if err != nil {
		return nil, err
	}
	cfg := langs["cpp-gpp-64"]
	if cfg == nil {
		return nil, fmt.Errorf("C++17 language is not configured")
	}
	comp := compiler.New(v.Exec)
	gen, err := comp.CompileContestant(ctx, p.GeneratorCode, cfg)
	if err != nil || !gen.Success {
		return nil, fmt.Errorf("generator compile failed: %s", gen.Output)
	}
	sol, err := comp.CompileContestant(ctx, p.SolutionCode, cfg)
	if err != nil || !sol.Success {
		return nil, fmt.Errorf("reference solution compile failed: %s", sol.Output)
	}
	checks := []ValidationCheck{{Name: "generator_compile", Passed: true, Message: "generator compiled"}, {Name: "solution_compile", Passed: true, Message: "reference solution compiled"}}
	if v.StagingRoot == "" {
		return checks, fmt.Errorf("AI staging root is not configured")
	}
	stage := filepath.Join(v.StagingRoot, jobID)
	if err := os.MkdirAll(stage, 0o750); err != nil {
		return checks, err
	}
	inputs := []struct{ name, input string }{}
	for i, seed := range p.Seeds {
		a, err := v.runGenerator(ctx, gen, cfg, seed)
		if err != nil {
			return checks, err
		}
		b, err := v.runGenerator(ctx, gen, cfg, seed)
		if err != nil {
			return checks, err
		}
		if a != b {
			return checks, fmt.Errorf("generator is nondeterministic for seed %d", seed)
		}
		if strings.TrimSpace(a) == "" {
			return checks, fmt.Errorf("generator emitted empty testcase for seed %d", seed)
		}
		if len(a) > 8<<20 {
			return checks, fmt.Errorf("generator output exceeds 8 MiB for seed %d", seed)
		}
		if err := os.WriteFile(filepath.Join(stage, fmt.Sprintf("generated-%03d.in", i+1)), []byte(a), 0o640); err != nil {
			return checks, err
		}
		inputs = append(inputs, struct{ name, input string }{fmt.Sprintf("generated-%03d", i+1), a})
	}
	for i, c := range p.CuratedCases {
		if err := os.WriteFile(filepath.Join(stage, fmt.Sprintf("curated-%03d.in", i+1)), []byte(c.Input), 0o640); err != nil {
			return checks, err
		}
		inputs = append(inputs, struct{ name, input string }{fmt.Sprintf("curated-%03d", i+1), c.Input})
	}
	for _, tc := range inputs {
		out, err := v.runSolution(ctx, sol, cfg, tc.input)
		if err != nil {
			return checks, err
		}
		if len(out) > 8<<20 {
			return checks, fmt.Errorf("solution output exceeds 8 MiB for %s", tc.name)
		}
		if err := os.WriteFile(filepath.Join(stage, tc.name+".out"), []byte(out), 0o640); err != nil {
			return checks, err
		}
	}
	checks = append(checks, ValidationCheck{Name: "reference_solution_cases", Passed: true, Message: "reference solution produced every expected output"})
	checks = append(checks, ValidationCheck{Name: "generator_determinism", Passed: true, Message: "generator repeated identically for all seeds"})
	return checks, nil
}

func (v *SandboxValidator) runSolution(ctx context.Context, cr *compiler.CompileResult, cfg *compiler.LangConfig, input string) (string, error) {
	artifact := cr.Files["Main"]
	args := []string{"/bin/sh", "-c", "./Main < input.txt > output.txt"}
	res, err := v.Exec.Run(&executor.ExecRequest{Cmd: []executor.Cmd{{Args: args, Env: []string{"PATH=/usr/bin:/bin", "HOME=/tmp"}, CopyIn: map[string]executor.CmdFile{"Main": artifact, "input.txt": {Content: input}}, CopyOut: []string{"output.txt"}, CPULimit: 5e9, MemoryLimit: 512 * 1024 * 1024, ProcLimit: 32}}})
	if err != nil || len(res) == 0 {
		return "", fmt.Errorf("reference solution execution failed")
	}
	if res[0].Status != "Accepted" {
		return "", fmt.Errorf("reference solution failed: %s", res[0].Status)
	}
	return res[0].Files["output.txt"], nil
}

func (v *SandboxValidator) runGenerator(ctx context.Context, cr *compiler.CompileResult, cfg *compiler.LangConfig, seed int64) (string, error) {
	artifact := cr.Files["Main"]
	args := []string{"./Main", fmt.Sprint(seed)}
	if cfg.Runtime != "" {
		args = strings.Fields(strings.ReplaceAll(strings.ReplaceAll(cfg.Runtime, "{{dir}}", "."), "{{exe}}", "Main"))
		args = append(args, fmt.Sprint(seed))
	}
	res, err := v.Exec.Run(&executor.ExecRequest{Cmd: []executor.Cmd{{Args: args, Env: []string{"PATH=/usr/bin:/bin", "HOME=/tmp"}, Files: []executor.CmdFile{}, CopyIn: map[string]executor.CmdFile{"Main": artifact}, CopyOut: []string{"stdout"}, CPULimit: 2e9, MemoryLimit: 256 * 1024 * 1024, ProcLimit: 16}}})
	if err != nil || len(res) == 0 {
		return "", fmt.Errorf("generator execution failed")
	}
	if res[0].Status != "Accepted" {
		return "", fmt.Errorf("generator failed: %s", res[0].Status)
	}
	return res[0].Files["stdout"], nil
}
