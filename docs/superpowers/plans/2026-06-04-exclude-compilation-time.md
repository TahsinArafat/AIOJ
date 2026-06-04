# Exclude Compilation Time from Standard Judging Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Exclude compilation time from standard judging and evaluateSubtasks execution limits and reported execution time.

**Architecture:** Separate compilation and execution phases for standard submissions:
1. Compile contestant code once at the beginning of the `judge` function (with a separate Go-Judge client run call).
2. If compilation fails, mark submission as CE and exit immediately.
3. If compilation succeeds, capture the compiled files (either `Main` executable for C/C++/Rust/C#, or a packaged `compile.tar` for Java) and pass them as `CopyIn` files to each testcase execution.
4. Update `runTestCase` to only execute the compiled binary, completely omitting compile commands from runtime limits and calculations.

**Tech Stack:** Go (Backend/Judge Worker), Go-Judge (Sandbox REST API)

---

### Task 1: Compile Contestant Code Separately

**Files:**
- Modify: `internal/judge/worker.go`

- [ ] **Step 1: Check compile time separation logic**

Read `internal/judge/worker.go` around line 160-230 to locate standard compilation setup.

- [ ] **Step 2: Add compilation helper and perform compilation in `judge` and `evaluateSubtasks`**

We will define a compilation phase inside `judge()` and pass the compiled files to `evaluateSubtasks()` and `runTestCase()`.

Let's modify `internal/judge/worker.go` to do compilation before running test cases. We will create a helper structure:

```go
type CompileResult struct {
	Success bool
	Output  string
	Files   map[string]executor.CmdFile
	TarMode bool
}
```

Implement the compile logic:

```go
func (wp *WorkerPool) compileContestantCode(
	ctx context.Context,
	sub *model.Submission,
	cfg *compiler.LangConfig,
	compileCmdStr string,
) (*CompileResult, error) {
	srcName := "Main" + cfg.Extensions[0]
	copyIn := map[string]executor.CmdFile{srcName: {Content: sub.SourceCode}}

	tarMode := cfg.Key == "java"
	var cmdArgs []string
	var copyOut []string

	if tarMode {
		cmdArgs = []string{"/bin/sh", "-c", compileCmdStr + " && tar -cf compile.tar --exclude=compile.tar --exclude=" + srcName + " ."}
		copyOut = []string{"compile.tar"}
	} else {
		cmdArgs = []string{"/bin/sh", "-c", compileCmdStr}
		copyOut = []string{"Main"}
	}

	resp, err := wp.exec.Run(&executor.ExecRequest{
		Cmd: []executor.Cmd{{
			Args:        cmdArgs,
			Env:         []string{"PATH=/usr/bin:/bin", "HOME=/tmp"},
			CPULimit:    30_000_000_000, // 30s CPU limit
			MemoryLimit: 536_870_912,    // 512MB RAM limit
			ProcLimit:   64,
			CopyIn:      copyIn,
			CopyOut:     copyOut,
		}},
	})
	if err != nil {
		return nil, err
	}
	if len(resp) == 0 {
		return &CompileResult{Success: false, Output: "no result"}, nil
	}

	cr := resp[0]
	if cr.Status != "Accepted" {
		compileOutput := cr.Error
		if fileErr, ok := cr.Files["error.txt"]; ok && fileErr != "" {
			compileOutput = fileErr
		} else if fileOut, ok := cr.Files["output.txt"]; ok && fileOut != "" {
			compileOutput = fileOut
		}
		if compileOutput == "" {
			compileOutput = "compile error: nonzero exit status"
		}
		return &CompileResult{Success: false, Output: compileOutput}, nil
	}

	compiledFiles := make(map[string]executor.CmdFile)
	for fname, content := range cr.Files {
		if fname != srcName {
			compiledFiles[fname] = executor.CmdFile{Content: content}
		}
	}

	return &CompileResult{
		Success: true,
		Files:   compiledFiles,
		TarMode: tarMode,
	}, nil
}
```

- [ ] **Step 3: Modify `runTestCase` signature and logic**

Change `runTestCase` to accept `compiledResult *CompileResult` instead of `compileCmdStr`:

```go
func (wp *WorkerPool) runTestCase(
	ctx context.Context,
	prob *model.Problem,
	langCfg *compiler.LangConfig,
	tc model.TestCaseScore,
	copyIn map[string]executor.CmdFile,
	compiledResult *CompileResult,
	spjExeDir string,
	spjBinContent string,
	cpuLimitNs uint64,
	memLimitBytes uint64,
) model.TestCaseResult {
```

Update `args` generation inside `runTestCase`:

```go
	srcName := "Main" + langCfg.Extensions[0]
	var args []string

	if compiledResult != nil && compiledResult.Success {
		// Put compiled files in copyIn
		for fname, file := range compiledResult.Files {
			copyIn[fname] = file
		}

		if langCfg.Runtime != "" {
			rtParts := strings.Fields(langCfg.Runtime)
			for i, p := range rtParts {
				rtParts[i] = strings.ReplaceAll(p, "{{dir}}", ".")
				rtParts[i] = strings.ReplaceAll(rtParts[i], "{{exe}}", "Main")
			}
			runCmd := strings.Join(rtParts, " ") + " < input.txt > output.txt 2> error.txt"
			if compiledResult.TarMode {
				runCmd = "tar -xf compile.tar && " + runCmd
			}
			args = []string{"/bin/sh", "-c", runCmd}
		} else {
			runCmd := "./Main < input.txt > output.txt 2> error.txt"
			if compiledResult.TarMode {
				runCmd = "tar -xf compile.tar && " + runCmd
			}
			args = []string{"/bin/sh", "-c", runCmd}
		}
	} else if langCfg.Runtime != "" {
		// Interpreted language without compilation
		rtParts := strings.Fields(langCfg.Runtime)
		runCmd := strings.Join(rtParts, " ") + " " + srcName + " < input.txt > output.txt 2> error.txt"
		args = []string{"/bin/sh", "-c", runCmd}
	} else {
		// Fallback for compiled languages without compile output / command config
		args = []string{"/bin/sh", "-c", "./Main < input.txt > output.txt 2> error.txt"}
	}
```

- [ ] **Step 4: Update `judge()` and `evaluateSubtasks()` callers in `internal/judge/worker.go`**

In `judge()` (lines 179-200), run compilation first:

```go
	var compiledResult *CompileResult
	if compileCmdStr != "" {
		var err error
		compiledResult, err = wp.compileContestantCode(ctx, sub, cfg, compileCmdStr)
		if err != nil {
			wp.subStore.UpdateResult(ctx, submissionID, model.StatusSE, 0, 0, 0, "compilation failed: "+err.Error(), nil)
			return
		}
		if !compiledResult.Success {
			wp.subStore.UpdateResult(ctx, submissionID, model.StatusCE, 0, 0, 0, compiledResult.Output, nil)
			wp.probStore.UpdateCounts(ctx, prob.ID, 1, 0)
			return
		}
	}

	if prob.HasSubtasks() && prob.ScoringMode == "partial" {
		wp.evaluateSubtasks(ctx, sub, prob, cfg, copyIn, compiledResult, spjExeDir, spjBinContent)
		return
	}
```

And update the loop in `judge()` and `evaluateSubtasks()` to pass `compiledResult` instead of `compileCmdStr`.

- [ ] **Step 5: Run Go tests to verify build succeeds**

Run: `go test ./internal/judge/...`
Expected: PASS
