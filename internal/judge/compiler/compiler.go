package compiler

import (
	"context"
	"fmt"
	"strings"

	"github.com/tahsinarafat/aioj/internal/judge/executor"
)

const (
	compileCPULimit    = 30_000_000_000 // 30 s
	compileMemoryLimit = 536_870_912    // 512 MB
	compileProcLimit   = 64
)

// CompileResult is returned by all Compile* methods.
type CompileResult struct {
	// Success is true when compilation succeeded.
	Success bool
	// Output contains compiler stderr / error message on failure, or
	// any informational output on success.
	Output string
	// Files holds the compiled artefacts keyed by filename (e.g. "Main",
	// "compile.tar").  Only populated on success.
	Files map[string]executor.CmdFile
	// TarMode is true for languages (e.g. Java) where the compiled output
	// is a tar archive rather than a single executable.
	TarMode bool
}

// Compiler compiles source code via the sandbox executor.
type Compiler struct {
	exec *executor.Client
}

// New returns a Compiler backed by the given executor client.
func New(exec *executor.Client) *Compiler {
	return &Compiler{exec: exec}
}

// CompileContestant compiles a contestant's source code.
// It handles both single-binary and tar-mode (Java) output.
func (c *Compiler) CompileContestant(
	ctx context.Context,
	sourceCode string,
	cfg *LangConfig,
) (*CompileResult, error) {
	if cfg.CompileCmd == "" {
		// Interpreted language — nothing to compile.
		return &CompileResult{Success: true, Files: map[string]executor.CmdFile{}}, nil
	}

	srcName := "Main" + cfg.Extensions[0]
	tarMode := cfg.Key == "java"

	compileCmdStr := buildCompileCmd(cfg.CompileCmd, "Main", srcName)

	var cmdArgs []string
	var copyOut []string
	if tarMode {
		cmdArgs = []string{
			"/bin/sh", "-c",
			compileCmdStr + " && tar -cf compile.tar --exclude=compile.tar --exclude=" + srcName + " .",
		}
		copyOut = []string{"compile.tar"}
	} else {
		cmdArgs = []string{"/bin/sh", "-c", compileCmdStr}
		copyOut = []string{"Main"}
	}

	resp, err := c.exec.Run(&executor.ExecRequest{
		Cmd: []executor.Cmd{{
			Args:        cmdArgs,
			Env:         []string{"PATH=/usr/bin:/bin", "HOME=/tmp"},
			CPULimit:    compileCPULimit,
			MemoryLimit: compileMemoryLimit,
			ProcLimit:   compileProcLimit,
			CopyIn:      map[string]executor.CmdFile{srcName: {Content: sourceCode}},
			CopyOut:     copyOut,
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("contestant compile request: %w", err)
	}
	return parseCompileResponse(resp, srcName, tarMode)
}

// CompileSPJ compiles a Special Judge binary.
// Returns the compiled binary content and the executor run-dir, plus any error.
func (c *Compiler) CompileSPJ(
	ctx context.Context,
	sourceCode string,
	lang string,
	langs map[string]*LangConfig,
) (binContent string, err error) {
	if lang == "" {
		lang = "cpp-gpp-64"
	}
	cfg, ok := langs[lang]
	if !ok {
		return "", fmt.Errorf("unsupported SPJ language: %s", lang)
	}

	srcName := "spj" + cfg.Extensions[0]
	exeName := "spj"
	cmdStr := buildCompileCmd(cfg.CompileCmd, exeName, srcName)

	resp, err := c.exec.Run(&executor.ExecRequest{
		Cmd: []executor.Cmd{{
			Args:        []string{"/bin/sh", "-c", cmdStr},
			Env:         []string{"PATH=/usr/bin:/bin"},
			CPULimit:    compileCPULimit,
			MemoryLimit: compileMemoryLimit,
			ProcLimit:   compileProcLimit,
			CopyIn:      map[string]executor.CmdFile{srcName: {Content: sourceCode}},
			CopyOut:     []string{exeName},
		}},
	})
	if err != nil {
		return "", fmt.Errorf("SPJ compile request: %w", err)
	}
	if len(resp) == 0 {
		return "", fmt.Errorf("SPJ compile: no result from executor")
	}
	cr := resp[0]
	if cr.Status != "Accepted" {
		msg := cr.Error
		if msg == "" {
			msg = "SPJ compile error (status: " + cr.Status + ")"
		}
		return "", fmt.Errorf("SPJ compile error: %s", msg)
	}
	return cr.Files[exeName], nil
}

// CompileInteractor compiles an interactor binary.
func (c *Compiler) CompileInteractor(
	ctx context.Context,
	sourceCode string,
	lang string,
	langs map[string]*LangConfig,
) (binContent string, err error) {
	if lang == "" {
		lang = "cpp-gpp-64"
	}
	cfg, ok := langs[lang]
	if !ok {
		return "", fmt.Errorf("unsupported interactor language: %s", lang)
	}

	srcName := "interactor" + cfg.Extensions[0]
	exeName := "interactor"
	cmdStr := buildCompileCmd(cfg.CompileCmd, exeName, srcName)

	resp, err := c.exec.Run(&executor.ExecRequest{
		Cmd: []executor.Cmd{{
			Args:        []string{"/bin/sh", "-c", cmdStr},
			Env:         []string{"PATH=/usr/bin:/bin"},
			CPULimit:    compileCPULimit,
			MemoryLimit: compileMemoryLimit,
			ProcLimit:   compileProcLimit,
			CopyIn:      map[string]executor.CmdFile{srcName: {Content: sourceCode}},
			CopyOut:     []string{exeName},
		}},
	})
	if err != nil {
		return "", fmt.Errorf("interactor compile request: %w", err)
	}
	if len(resp) == 0 {
		return "", fmt.Errorf("interactor compile: no result from executor")
	}
	cr := resp[0]
	if cr.Status != "Accepted" {
		msg := cr.Error
		if msg == "" {
			msg = "interactor compile error (status: " + cr.Status + ")"
		}
		return "", fmt.Errorf("interactor compile error: %s", msg)
	}
	return cr.Files[exeName], nil
}

// ---------- helpers ----------

// buildCompileCmd substitutes {{exe}}, {{src}}, and {{dir}} in a compile
// command template.
func buildCompileCmd(template, exe, src string) string {
	s := strings.ReplaceAll(template, "{{exe}}", exe)
	s = strings.ReplaceAll(s, "{{src}}", src)
	s = strings.ReplaceAll(s, "{{dir}}", "/box")
	return s
}

// parseCompileResponse converts a raw executor response into a CompileResult.
func parseCompileResponse(resp []executor.CmdResult, srcName string, tarMode bool) (*CompileResult, error) {
	if len(resp) == 0 {
		return &CompileResult{Success: false, Output: "no result from executor"}, nil
	}
	cr := resp[0]
	if cr.Status != "Accepted" {
		output := cr.Error
		if v, ok := cr.Files["error.txt"]; ok && v != "" {
			output = v
		} else if v, ok := cr.Files["output.txt"]; ok && v != "" {
			output = v
		}
		if output == "" {
			output = "compile error: nonzero exit status"
		}
		return &CompileResult{Success: false, Output: output}, nil
	}

	files := make(map[string]executor.CmdFile)
	for fname, content := range cr.Files {
		if fname != srcName {
			files[fname] = executor.CmdFile{Content: content}
		}
	}
	return &CompileResult{Success: true, Files: files, TarMode: tarMode}, nil
}
