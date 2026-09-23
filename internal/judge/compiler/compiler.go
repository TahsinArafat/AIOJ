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
	Success bool
	// Output contains compiler stderr / error message on failure, or any
	// informational output on success.
	Output string
	// Files holds compiled artifacts keyed by filename. Native executables and
	// tar archives use go-judge file IDs because JSON strings cannot safely
	// carry arbitrary binary bytes.
	Files map[string]executor.CmdFile
	// TarMode is true for languages (e.g. Java) where the compiled output is a
	// tar archive rather than a single executable.
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

// CompileContestant compiles a contestant's source code. It handles both
// single-binary and tar-mode (Java) output.
func (c *Compiler) CompileContestant(ctx context.Context, sourceCode string, cfg *LangConfig) (*CompileResult, error) {
	if cfg.CompileCmd == "" {
		return &CompileResult{Success: true, Files: map[string]executor.CmdFile{}}, nil
	}

	srcName := "Main" + cfg.Extensions[0]
	tarMode := cfg.Key == "java"
	compileCmd := buildCompileCmd(cfg.CompileCmd, "Main", srcName)
	artifact := "Main"
	args := []string{"/bin/sh", "-c", compileCmd}
	if tarMode {
		args = []string{"/bin/sh", "-c", compileCmd + " && tar -cf compile.tar --exclude=compile.tar --exclude=" + srcName + " ."}
		artifact = "compile.tar"
	}

	resp, err := c.exec.Run(&executor.ExecRequest{Cmd: []executor.Cmd{{
		Args:          args,
		Env:           []string{"PATH=/usr/bin:/bin", "HOME=/tmp"},
		CPULimit:      compileCPULimit,
		MemoryLimit:   compileMemoryLimit,
		ProcLimit:     compileProcLimit,
		CopyIn:        map[string]executor.CmdFile{srcName: {Content: sourceCode}},
		CopyOutCached: []string{artifact},
	}}})
	if err != nil {
		return nil, fmt.Errorf("contestant compile request: %w", err)
	}
	return parseCompileResponse(resp, artifact, tarMode)
}

// CompileSPJ compiles a Special Judge binary and returns its file-ID-backed
// artifact. The caller must pass that CmdFile back to go-judge unchanged.
func (c *Compiler) CompileSPJ(ctx context.Context, sourceCode, lang string, langs map[string]*LangConfig) (executor.CmdFile, error) {
	return c.compileAuxiliary(ctx, sourceCode, lang, langs, "spj", "SPJ")
}

// CompileInteractor compiles an interactor binary and returns its file-ID-backed
// artifact. The caller must pass that CmdFile back to go-judge unchanged.
func (c *Compiler) CompileInteractor(ctx context.Context, sourceCode, lang string, langs map[string]*LangConfig) (executor.CmdFile, error) {
	return c.compileAuxiliary(ctx, sourceCode, lang, langs, "interactor", "interactor")
}

func (c *Compiler) compileAuxiliary(ctx context.Context, sourceCode, lang string, langs map[string]*LangConfig, stem, label string) (executor.CmdFile, error) {
	if lang == "" {
		lang = "cpp-gpp-64"
	}
	cfg, ok := langs[lang]
	if !ok {
		return executor.CmdFile{}, fmt.Errorf("unsupported %s language: %s", label, lang)
	}
	srcName := stem + cfg.Extensions[0]
	artifact := stem
	cmd := buildCompileCmd(cfg.CompileCmd, artifact, srcName)
	resp, err := c.exec.Run(&executor.ExecRequest{Cmd: []executor.Cmd{{
		Args:          []string{"/bin/sh", "-c", cmd},
		Env:           []string{"PATH=/usr/bin:/bin"},
		CPULimit:      compileCPULimit,
		MemoryLimit:   compileMemoryLimit,
		ProcLimit:     compileProcLimit,
		CopyIn:        map[string]executor.CmdFile{srcName: {Content: sourceCode}},
		CopyOutCached: []string{artifact},
	}}})
	if err != nil {
		return executor.CmdFile{}, fmt.Errorf("%s compile request: %w", label, err)
	}
	if len(resp) == 0 {
		return executor.CmdFile{}, fmt.Errorf("%s compile: no result from executor", label)
	}
	cr := resp[0]
	if cr.Status != "Accepted" {
		msg := cr.Error
		if msg == "" {
			msg = label + " compile error (status: " + cr.Status + ")"
		}
		return executor.CmdFile{}, fmt.Errorf("%s compile error: %s", label, msg)
	}
	return cachedArtifact(cr, artifact, label)
}

func cachedArtifact(cr executor.CmdResult, name, label string) (executor.CmdFile, error) {
	if id := cr.FileIDs[name]; id != "" {
		return executor.CmdFile{FileID: id}, nil
	}
	return executor.CmdFile{}, fmt.Errorf("%s compile succeeded but %s artifact was not returned as a file ID", label, name)
}

// buildCompileCmd substitutes {{exe}}, {{src}}, and {{dir}} in a compile
// command template.
func buildCompileCmd(template, exe, src string) string {
	s := strings.ReplaceAll(template, "{{exe}}", exe)
	s = strings.ReplaceAll(s, "{{src}}", src)
	s = strings.ReplaceAll(s, "{{dir}}", "/box")
	return s
}

func parseCompileResponse(resp []executor.CmdResult, artifact string, tarMode bool) (*CompileResult, error) {
	if len(resp) == 0 {
		return &CompileResult{Success: false, Output: "no result from executor"}, nil
	}
	cr := resp[0]
	if cr.Status != "Accepted" {
		output := cr.Error
		if v := cr.Files["error.txt"]; v != "" {
			output = v
		} else if v := cr.Files["output.txt"]; v != "" {
			output = v
		}
		if output == "" {
			output = "compile error: nonzero exit status"
		}
		return &CompileResult{Success: false, Output: output}, nil
	}
	file, err := cachedArtifact(cr, artifact, "contestant")
	if err != nil {
		return &CompileResult{Success: false, Output: err.Error()}, nil
	}
	return &CompileResult{
		Success: true,
		Files:   map[string]executor.CmdFile{artifact: file},
		TarMode: tarMode,
	}, nil
}
