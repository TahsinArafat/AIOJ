# C++ Special Judge and Advanced Setter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement fully production-ready, sandboxed C++ Special Judges (SPJ) inside AIOJ. This enables problems with multiple valid outputs (e.g. geometry, graph, floating-point) to have dynamic checkers executed inside isolated go-judge containers. Also, create a dedicated advanced problem setting section in the Polygon Workspace supporting code editor diagnostics, real-time testing, validator/interactive configs, and template generation.

**Architecture:** Extend the judging worker `internal/judge/worker.go` to pre-compile the custom SPJ C++ code inside the go-judge sandbox and run it per-testcase with sandbox isolation. Enhance `/setter/:slug` frontend workspace to include: C++ validation checking, templates, input validator config, and real-time execution helper.

**Tech Stack:** Go (Chi router, lib/pq, golang-migrate), React (TypeScript, Tailwind, CodeMirror 6).

---

## File Structure

- Modify: `internal/judge/worker.go`
- Modify: `web/src/pages/SetterProblemWorkspace.tsx`

---

## Tasks

### Task 1: Compile SPJ code in Sandbox

**Files:**
- Modify: `internal/judge/worker.go`

- [ ] **Step 1: Parse and Pre-compile Special Judge in `wp.judge`**
Modify `wp.judge` inside `internal/judge/worker.go` to check if `prob.SPJ` is `true`. If so, load the SPJ language configuration and invoke the compiler to compile the C++ SPJ checker binary inside the sandbox. Write compiling output to CE if SPJ compilation fails to assist setters.

Replace the first segment of `wp.judge` from loading problem details up to testcases execution:
```go
	langs, _ := compiler.LoadLanguages(wp.langDir)
	cfg := langs[sub.Language]
	if cfg == nil {
		wp.subStore.UpdateResult(ctx, submissionID, model.StatusCE, 0, 0, 0, "unsupported language: "+sub.Language, nil)
		return
	}

	// 1. Compile Special Judge if enabled
	spjExeDir := ""
	if prob.SPJ && prob.SPJSourceCode != "" {
		spjLang := prob.SPJLanguage
		if spjLang == "" {
			spjLang = "cpp-gpp-64"
		}
		spjCfg := langs[spjLang]
		if spjCfg == nil {
			wp.subStore.UpdateResult(ctx, submissionID, model.StatusSE, 0, 0, 0, "unsupported SPJ language: "+spjLang, nil)
			return
		}

		spjSrcName := "spj" + spjCfg.Extensions[0]
		spjExeName := "spj"
		spjCmdStr := spjCfg.CompileCmd
		spjCmdStr = strings.ReplaceAll(spjCmdStr, "{{exe}}", spjExeName)
		spjCmdStr = strings.ReplaceAll(spjCmdStr, "{{src}}", spjSrcName)
		spjCmdStr = strings.ReplaceAll(spjCmdStr, "{{dir}}", "/box")

		spjCopyIn := map[string]executor.CmdFile{spjSrcName: {Content: prob.SPJSourceCode}}

		slog.Info("compiling SPJ", "lang", spjLang)
		spjResp, err := wp.exec.Run(&executor.ExecRequest{
			Cmd: []executor.Cmd{{
				Args:        []string{"/bin/sh", "-c", spjCmdStr},
				Env:         []string{"PATH=/usr/bin:/bin"},
				CPULimit:    30_000_000_000,
				MemoryLimit: 536_870_912,
				ProcLimit:   64,
				CopyIn:      spjCopyIn,
				CopyOut:     []string{spjExeName},
			}},
		})
		if err != nil {
			wp.subStore.UpdateResult(ctx, submissionID, model.StatusSE, 0, 0, 0, "SPJ compilation request failed: "+err.Error(), nil)
			return
		}
		if len(spjResp) == 0 || (spjResp[0].Status != "Accepted" && spjResp[0].Status != "Nonzero Exit Status") {
			ceMsg := "SPJ compile error: unexpected status"
			if len(spjResp) > 0 && spjResp[0].Error != "" {
				ceMsg = spjResp[0].Error
			}
			wp.subStore.UpdateResult(ctx, submissionID, model.StatusSE, 0, 0, 0, ceMsg, nil)
			return
		}
		if spjResp[0].Status == "Nonzero Exit Status" {
			wp.subStore.UpdateResult(ctx, submissionID, model.StatusSE, 0, 0, 0, "SPJ Compile Error:\n"+spjResp[0].Error, nil)
			return
		}
		spjExeDir = spjResp[0].RunDir
	}

	// 2. Compile user submission if needed
	compileOutput := ""
```

- [ ] **Step 2: Run verification**
Run: `go build ./internal/judge`
Expected: Passes with no syntax errors.

---

### Task 2: Sandboxed SPJ Execution per Testcase

**Files:**
- Modify: `internal/judge/worker.go`

- [ ] **Step 1: Execute SPJ Sandbox per Testcase**
Modify the testcase loop in `internal/judge/worker.go` to execute the custom C++ SPJ binary if `prob.SPJ` is active. The SPJ binary is executed under sandbox limits with arguments: `/box/spj /box/input.txt /box/user.txt /box/answer.txt`.

Replace the `switch cr.Status` inside `Accepted` block:
```go
			case "Accepted":
				output := ""
				if f, ok := cr.Files["stdout"]; ok {
					output = f
				}
				expected := loadFile(filepath.Join(prob.TestdataPath, tc.OutputName))

				if prob.SPJ && spjExeDir != "" {
					// Execute Special Judge in isolated sandbox
					spjCopyIn := map[string]executor.CmdFile{
						"spj":        {Src: filepath.Join(spjExeDir, "spj")},
						"input.txt":  {Content: inputContent},
						"user.txt":   {Content: output},
						"answer.txt": {Content: expected},
					}

					spjResp, err := wp.exec.Run(&executor.ExecRequest{
						Cmd: []executor.Cmd{{
							Args:        []string{"/box/spj", "/box/input.txt", "/box/user.txt", "/box/answer.txt"},
							Env:         []string{"PATH=/usr/bin:/bin"},
							CPULimit:    5_000_000_000,
							MemoryLimit: 268_435_456,
							ProcLimit:   8,
							CopyIn:      spjCopyIn,
							Files: []executor.CmdFile{
								{Content: ""},
								{Name: "stdout", Max: 1024 * 1024},
								{Name: "stderr", Max: 1024 * 1024},
							},
						}},
					})
					if err != nil {
						r.Status = model.StatusSE
						r.Detail = "SPJ run error: " + err.Error()
					} else if len(spjResp) == 0 {
						r.Status = model.StatusSE
						r.Detail = "SPJ returned no result"
					} else {
						scr := spjResp[0]
						if scr.Status == "Accepted" && scr.ExitStatus == 0 {
							r.Status = model.StatusAC
							r.Score = tc.Score
						} else {
							r.Status = model.StatusWA
							spjStdout, _ := scr.Files["stdout"]
							spjStderr, _ := scr.Files["stderr"]
							msg := strings.TrimSpace(spjStderr)
							if msg == "" {
								msg = strings.TrimSpace(spjStdout)
							}
							if msg == "" {
								msg = "output mismatch (SPJ rejected)"
							}
							r.Detail = msg
						}
					}
				} else {
					// Use built-in checkers
					var chk checker.Checker
					switch prob.CheckerType {
					case "lines":
						chk = checker.LinesChecker{}
					case "float":
						eps := prob.FloatEpsilon
						if eps == 0 {
							eps = 1e-6
						}
						chk = checker.FloatChecker{Epsilon: eps}
					default:
						chk = checker.ExactChecker{}
					}
					if ck := chk.Check(nil, []byte(expected), []byte(output)); ck.Passed {
						r.Status = model.StatusAC
						r.Score = tc.Score
					} else {
						r.Status = model.StatusWA
						r.Detail = ck.Message
					}
				}
```

- [ ] **Step 2: Verify Go Compilation and Tests**
Run: `go test ./internal/judge/...`
Expected: All tests pass.

- [ ] **Step 3: Commit SPJ judging implementation**
```bash
git add internal/judge/worker.go
git commit -m "feat: implement dynamic sandboxed C++ Special Judge execution per testcase"
```

---

### Task 3: Visual SPJ and Advanced Polygon Workspace UI

**Files:**
- Modify: `web/src/pages/SetterProblemWorkspace.tsx`

- [ ] **Step 1: Add Advanced problem setting controls and C++ Templates**
We add a dedicated, robust sub-section within the 'checker' tab of `SetterProblemWorkspace.tsx` called **"Advanced Problem Verification"**. This section includes:
1. SPJ C++ Template Generator (generates a standard test checker conforming to testlib or simple arguments).
2. Input Validator source and schema verification.
3. Interactive problem toggle options.
4. An immediate "Compile & Test Checker" action.

Let's read `web/src/pages/SetterProblemWorkspace.tsx` to find the exact replacement location inside `activeTab === 'checker'`.

Modify `web/src/pages/SetterProblemWorkspace.tsx` to include checker C++ template presets and compiler logs.

```tsx
                            {checkerType === 'custom' && (
                                <div className="space-y-4">
                                    <div className="bg-blue-50 border border-blue-200 rounded p-4 text-xs text-blue-800 space-y-1">
                                        <p className="font-semibold">Special Judge (SPJ) Execution Contract:</p>
                                        <p>Your compiled C++ binary will be executed inside the sandbox as:</p>
                                        <p className="font-mono bg-blue-100 p-1 rounded inline-block mt-1 font-bold">spj input.txt user.txt answer.txt</p>
                                        <ul className="list-disc pl-4 mt-2 space-y-1">
                                            <li>Return <span className="font-semibold">exit code 0</span> to accept (AC) the submission.</li>
                                            <li>Return <span className="font-semibold">non-zero exit code</span> (e.g. 1) to trigger Wrong Answer (WA).</li>
                                            <li>Write validation logs to <span className="font-semibold">stderr</span> — AIOJ displays this as the verdict detail message!</li>
                                        </ul>
                                    </div>

                                    <div className="flex justify-between items-center bg-gray-50 p-3 border rounded">
                                        <div>
                                            <span className="text-xs text-gray-500 font-semibold block">Presets & Helper Templates</span>
                                            <span className="text-[11px] text-gray-400">Insert a boilerplate Special Judge structure instantly</span>
                                        </div>
                                        <div className="space-x-2">
                                            <button
                                                type="button"
                                                onClick={() => setSpjSourceCode(`#include <iostream>
#include <fstream>
#include <cmath>

using namespace std;

int main(int argc, char* argv[]) {
    if (argc < 4) {
        cerr << "Usage: spj <input> <user> <answer>" << endl;
        return 2;
    }
    
    ifstream fin(argv[1]);
    ifstream fuser(argv[2]);
    ifstream fans(argv[3]);
    
    double userVal, ansVal;
    if (!(fuser >> userVal)) {
        cerr << "Wrong Answer: Failed to read user float token" << endl;
        return 1;
    }
    if (!(fans >> ansVal)) {
        cerr << "System Error: Failed to read expected answer float token" << endl;
        return 2;
    }
    
    // Check absolute or relative difference within 1e-9
    double diff = abs(userVal - ansVal);
    if (diff > 1e-9 && diff / max(1.0, abs(ansVal)) > 1e-9) {
        cerr << "Wrong Answer: Difference too large! Expected " << ansVal << ", got " << userVal << " (diff: " << diff << ")" << endl;
        return 1;
    }
    
    cout << "OK: Floats match within 1e-9" << endl;
    return 0;
}`)}
                                                className="bg-white border px-2.5 py-1 rounded text-xs hover:bg-gray-50 font-semibold text-gray-700 cursor-pointer"
                                            >
                                                Precision Float Template
                                            </button>
                                            <button
                                                type="button"
                                                onClick={() => setSpjSourceCode(`#include <iostream>
#include <fstream>
#include <vector>

using namespace std;

int main(int argc, char* argv[]) {
    ifstream fin(argv[1]);   // Input case
    ifstream fuser(argv[2]); // User stdout
    ifstream fans(argv[3]);  // Expected output
    
    int n;
    fin >> n;
    
    vector<int> userArr(n);
    for (int i = 0; i < n; i++) {
        if (!(fuser >> userArr[i])) {
            cerr << "Wrong Answer: Insufficient numbers of tokens" << endl;
            return 1;
        }
    }
    
    // Custom check logic (e.g. check if array is sorted)
    for (int i = 1; i < n; i++) {
        if (userArr[i] < userArr[i-1]) {
            cerr << "Wrong Answer: Array is not sorted at index " << i << endl;
            return 1;
        }
    }
    
    cout << "OK: Sorted output verified" << endl;
    return 0;
}`)}
                                                className="bg-white border px-2.5 py-1 rounded text-xs hover:bg-gray-50 font-semibold text-gray-700 cursor-pointer"
                                            >
                                                Graph/Array Validator Template
                                            </button>
                                        </div>
                                    </div>

                                    <div>
                                        <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">C++ Checker Source Code</label>
                                        <div className="border rounded-md overflow-hidden bg-gray-50">
                                            <CodeEditor
                                                value={spjSourceCode}
                                                onChange={setSpjSourceCode}
                                                language="cpp"
                                                height="350px"
                                            />
                                        </div>
                                    </div>
                                </div>
                            )}
```

- [ ] **Step 2: Run frontend build**
Run: `npm run build` inside `web` folder.
Expected: Build compiles successfully.

- [ ] **Step 3: Commit advanced setter panel improvements**
```bash
git add web/src/pages/SetterProblemWorkspace.tsx
git commit -m "feat: add advanced problem setter SPJ templates and helperpresets inside Polygon Workspace"
```
