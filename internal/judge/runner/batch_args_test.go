package runner

import (
	"strings"
	"testing"

	"github.com/tahsinarafat/aioj/internal/judge/compiler"
	"github.com/tahsinarafat/aioj/internal/judge/executor"
)

// An interpreted language must be handed its source file on the command line.
//
// CompileContestant returns Success with an empty artefact map when there is no
// compile step, so branching on cr.Success built `pypy3 < input.txt > output.txt`
// — no script. The interpreter read the test input as its program, produced
// nothing for an empty input, and every Python/PyPy/Node submission was judged
// Wrong Answer despite correct code.
func TestBuildRunArgsInterpretedPassesSourceFile(t *testing.T) {
	for _, key := range []string{"pypy", "python", "nodejs"} {
		cfg := &compiler.LangConfig{
			Key:        key,
			CompileCmd: "",
			Runtime:    "/usr/bin/" + key,
			Extensions: []string{".py"},
		}
		// Exactly what CompileContestant returns for a compile-less language.
		cr := &compiler.CompileResult{Success: true, Files: map[string]executor.CmdFile{}}

		got := strings.Join(buildRunArgs(cfg, cr), " ")
		if !strings.Contains(got, "Main.py") {
			t.Errorf("%s: run command omits the source file: %q", key, got)
		}
		if !strings.Contains(got, "/usr/bin/"+key) {
			t.Errorf("%s: run command omits the interpreter: %q", key, got)
		}
	}
}

// A compiled language must run the artefact and never the source file.
func TestBuildRunArgsCompiledRunsArtefact(t *testing.T) {
	cfg := &compiler.LangConfig{
		Key:        "cpp-gpp-64",
		CompileCmd: "g++ -O2 -o {{exe}} {{src}}",
		Extensions: []string{".cpp"},
	}
	cr := &compiler.CompileResult{Success: true, Files: map[string]executor.CmdFile{"Main": {FileID: "abc"}}}

	got := strings.Join(buildRunArgs(cfg, cr), " ")
	if got != "/bin/sh -c ./Main < input.txt > output.txt 2> error.txt" {
		t.Errorf("compiled run command = %q", got)
	}
	if strings.Contains(got, "Main.cpp") {
		t.Errorf("a compiled language must not run its source file: %q", got)
	}
}

// Java ships its artefacts as a tarball that has to be unpacked first.
func TestBuildRunArgsUnpacksTarballMode(t *testing.T) {
	cfg := &compiler.LangConfig{
		Key:        "java",
		CompileCmd: "javac {{src}}",
		Runtime:    "java {{exe}}",
		Extensions: []string{".java"},
	}
	cr := &compiler.CompileResult{Success: true, TarMode: true, Files: map[string]executor.CmdFile{"compile.tar": {FileID: "abc"}}}

	got := strings.Join(buildRunArgs(cfg, cr), " ")
	if !strings.HasPrefix(got, "/bin/sh -c tar -xf compile.tar && ") {
		t.Errorf("tarball mode must unpack first, got %q", got)
	}
	if strings.Contains(got, "Main.java") {
		t.Errorf("java must run the compiled class, not the source: %q", got)
	}
}
