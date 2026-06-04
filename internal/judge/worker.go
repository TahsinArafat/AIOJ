package judge

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tahsinarafat/aioj/internal/judge/checker"
	"github.com/tahsinarafat/aioj/internal/judge/compiler"
	"github.com/tahsinarafat/aioj/internal/judge/executor"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/queue"
	"github.com/tahsinarafat/aioj/internal/store"
)

type WorkerPool struct {
	queue          queue.JudgeQueue
	exec           *executor.Client
	langDir        string
	sem            chan struct{}
	subStore       store.SubmissionStore
	probStore      store.ProblemStore
	langLimitStore store.LanguageLimitStore
	balloonStore   store.BalloonStore
}

func NewWorkerPool(q queue.JudgeQueue, exec *executor.Client, langDir string, concurrency int,
	subStore store.SubmissionStore, probStore store.ProblemStore, langLimitStore store.LanguageLimitStore, balloonStore store.BalloonStore) *WorkerPool {
	return &WorkerPool{
		queue:          q,
		exec:           exec,
		langDir:        langDir,
		sem:            make(chan struct{}, concurrency),
		subStore:       subStore,
		probStore:      probStore,
		langLimitStore: langLimitStore,
		balloonStore:   balloonStore,
	}
}

func (wp *WorkerPool) Start(ctx context.Context) {
	for {
		subID, err := wp.queue.Dequeue(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("dequeue failed", "error", err)
			time.Sleep(1 * time.Second)
			continue
		}
		if subID == "" {
			continue
		}
		select {
		case wp.sem <- struct{}{}:
			go func(id string) {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("judge panic", "id", id, "panic", r)
						wp.subStore.UpdateResult(context.Background(), id, model.StatusSE, 0, 0, 0, fmt.Sprintf("panic: %v", r), nil)
					}
					<-wp.sem
				}()
				wp.judge(ctx, id)
			}(subID)
		case <-ctx.Done():
			return
		}
	}
}

func (wp *WorkerPool) judge(ctx context.Context, submissionID string) {
	slog.Info("judging", "id", submissionID)
	sub, err := wp.subStore.GetByID(context.Background(), submissionID)
	if err != nil || sub == nil {
		slog.Error("load sub failed", "id", submissionID, "error", err)
		wp.subStore.UpdateResult(context.Background(), submissionID, model.StatusSE, 0, 0, 0, fmt.Sprintf("failed to load submission: %v", err), nil)
		return
	}

	wp.subStore.UpdateStatus(context.Background(), submissionID, model.StatusJudging)

	prob, err := wp.probStore.GetByID(ctx, sub.ProblemID)
	if err != nil || prob == nil {
		wp.subStore.UpdateResult(ctx, submissionID, model.StatusSE, 0, 0, 0, "problem not found", nil)
		return
	}

	// Load per-language limits for this problem
	if wp.langLimitStore != nil {
		langLimits, err := wp.langLimitStore.GetByProblem(ctx, prob.ID)
		if err == nil {
			for _, ll := range langLimits {
				prob.LanguageLimits = append(prob.LanguageLimits, *ll)
			}
		}
	}

	if sub.SubmissionType == model.SubmissionTypeOutput {
		wp.judgeOutputOnly(ctx, sub, prob)
		return
	}

	if prob.Interactive {
		wp.judgeInteractive(ctx, sub, prob)
		return
	}

	langs, _ := compiler.LoadLanguages(wp.langDir)
	cfg := langs[sub.Language]
	if cfg == nil {
		wp.subStore.UpdateResult(ctx, submissionID, model.StatusCE, 0, 0, 0, "unsupported language: "+sub.Language, nil)
		return
	}

	spjExeDir := ""
	spjBinContent := ""
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
		if binData, ok := spjResp[0].Files[spjExeName]; ok {
			spjBinContent = binData
		}
	}

	compileOutput := ""
	compileCmdStr := ""
	srcName := "Main" + cfg.Extensions[0]
	if cfg.CompileCmd != "" {
		compileCmdStr = cfg.CompileCmd
		compileCmdStr = strings.ReplaceAll(compileCmdStr, "{{exe}}", "Main")
		compileCmdStr = strings.ReplaceAll(compileCmdStr, "{{src}}", srcName)
		compileCmdStr = strings.ReplaceAll(compileCmdStr, "{{dir}}", "/box")
	}

	copyIn := map[string]executor.CmdFile{srcName: {Content: sub.SourceCode}}

	timeLimitMs, memoryLimitKB := wp.getEffectiveLimits(prob, sub.Language)
	cpuNs := uint64(float64(timeLimitMs)*cfg.TimeLimitMultiplier) * 1_000_000
	memBytes := uint64(float64(memoryLimitKB)*cfg.MemoryLimitMultiplier) * 1024

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
	if compiledResult != nil {
		compileOutput = compiledResult.Output
	}

	if prob.HasSubtasks() && prob.ScoringMode == "partial" {
		wp.evaluateSubtasks(ctx, sub, prob, cfg, copyIn, compiledResult, spjExeDir, spjBinContent)
		return
	}

	results := make([]model.TestCaseResult, 0)
	finalStatus := model.StatusAC
	maxTime, maxMem, totalScore := 0, 0, 0

	for _, tc := range prob.TestCaseScore {
		r := wp.runTestCase(ctx, prob, cfg, tc, copyIn, compiledResult, spjExeDir, spjBinContent, cpuNs, memBytes)
		results = append(results, r)
		if r.Time > maxTime {
			maxTime = r.Time
		}
		if r.Memory > maxMem {
			maxMem = r.Memory
		}
		if r.Status != model.StatusAC && finalStatus == model.StatusAC {
			finalStatus = r.Status
		}
		totalScore += r.Score
	}

	avgScore := 0
	if len(prob.TestCaseScore) > 0 {
		avgScore = totalScore / len(prob.TestCaseScore)
	}

	wp.subStore.UpdateResult(ctx, submissionID, finalStatus, avgScore, maxTime, maxMem, compileOutput, results)
	if finalStatus == model.StatusAC && sub.ContestID != "" {
		_ = wp.balloonStore.CreateRequest(ctx, sub.ContestID, sub.ID, sub.UserID, sub.ProblemID)
	}
	wp.probStore.UpdateCounts(ctx, prob.ID, 1, boolToInt(finalStatus == model.StatusAC))
	slog.Info("judged", "id", submissionID, "verdict", finalStatus)
}

func loadFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (wp *WorkerPool) getEffectiveLimits(prob *model.Problem, language string) (timeLimitMs int, memoryLimitKB int) {
	timeLimitMs = prob.TimeLimit
	memoryLimitKB = prob.MemoryLimit

	for _, ll := range prob.LanguageLimits {
		if ll.LanguageID == language {
			if ll.TimeLimitMs != nil {
				timeLimitMs = *ll.TimeLimitMs
			}
			if ll.MemoryLimitKB != nil {
				memoryLimitKB = *ll.MemoryLimitKB
			}
			break
		}
	}

	return
}

type CompileResult struct {
	Success bool
	Output  string
	Files   map[string]executor.CmdFile
	TarMode bool
}

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
	srcName := "Main" + langCfg.Extensions[0]
	var args []string

	if compiledResult != nil && compiledResult.Success {
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
		rtParts := strings.Fields(langCfg.Runtime)
		runCmd := strings.Join(rtParts, " ") + " " + srcName + " < input.txt > output.txt 2> error.txt"
		args = []string{"/bin/sh", "-c", runCmd}
	} else {
		args = []string{"/bin/sh", "-c", "./Main < input.txt > output.txt 2> error.txt"}
	}

	inputContent := loadFile(filepath.Join(prob.TestdataPath, tc.InputName))
	copyIn["input.txt"] = executor.CmdFile{Content: inputContent}

	start := time.Now()
	resp, err := wp.exec.Run(&executor.ExecRequest{
		Cmd: []executor.Cmd{{
			Args:        args,
			Env:         []string{"PATH=/usr/bin:/bin", "HOME=/tmp"},
			CPULimit:    cpuLimitNs,
			MemoryLimit: memLimitBytes,
			ProcLimit:   64,
			CopyIn:      copyIn,
			CopyOut:     []string{"output.txt", "error.txt"},
		}},
	})
	elapsed := int(time.Since(start).Milliseconds())

	r := model.TestCaseResult{CaseName: tc.InputName, Time: elapsed}

	if err != nil {
		r.Status = model.StatusSE
		r.Detail = err.Error()
		return r
	}
	if len(resp) == 0 {
		r.Status = model.StatusSE
		r.Detail = "no result"
		return r
	}

	cr := resp[0]
	r.Memory = int(cr.Memory / 1024)

	switch cr.Status {
	case "Accepted":
		output := ""
		if f, ok := cr.Files["output.txt"]; ok {
			output = f
		}
		expected := loadFile(filepath.Join(prob.TestdataPath, tc.OutputName))

		if prob.SPJ && spjExeDir != "" {
			spjCopyIn := map[string]executor.CmdFile{
				"spj":        {Content: spjBinContent},
				"input.txt":  {Content: inputContent},
				"user.txt":   {Content: output},
				"answer.txt": {Content: expected},
			}

			spjResp, err := wp.exec.Run(&executor.ExecRequest{
				Cmd: []executor.Cmd{{
					Args:        []string{"/bin/sh", "-c", "/box/spj /box/input.txt /box/user.txt /box/answer.txt > output.txt 2> error.txt"},
					Env:         []string{"PATH=/usr/bin:/bin"},
					CPULimit:    5_000_000_000,
					MemoryLimit: 268_435_456,
					ProcLimit:   8,
					CopyIn:      spjCopyIn,
					CopyOut:     []string{"output.txt", "error.txt"},
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
					spjStdout, _ := scr.Files["output.txt"]
					spjStderr, _ := scr.Files["error.txt"]
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
			chk := checker.GetChecker(prob.CheckerType, prob.FloatEpsilon)
			if ck := chk.Check(nil, []byte(expected), []byte(output)); ck.Passed {
				r.Status = model.StatusAC
				r.Score = tc.Score
			} else {
				r.Status = model.StatusWA
				r.Detail = ck.Message
			}
		}
	case "Nonzero Exit Status":
		r.Status = model.StatusRE
		if errOut, ok := cr.Files["error.txt"]; ok && errOut != "" {
			r.Detail = errOut
		} else {
			r.Detail = cr.Error
		}
	case "TimeLimitExceeded":
		r.Status = model.StatusTLE
	case "MemoryLimitExceeded":
		r.Status = model.StatusMLE
	case "RuntimeError":
		r.Status = model.StatusRE
		r.Detail = cr.Error
	default:
		r.Status = model.StatusWA
		r.Detail = cr.Status
	}

	return r
}

func (wp *WorkerPool) evaluateSubtasks(
	ctx context.Context,
	sub *model.Submission,
	prob *model.Problem,
	langCfg *compiler.LangConfig,
	copyIn map[string]executor.CmdFile,
	compiledResult *CompileResult,
	spjExeDir string,
	spjBinContent string,
) {
	subtasks := prob.GetSubtasks()

	var subtaskIDs []int
	for id := range subtasks {
		subtaskIDs = append(subtaskIDs, id)
	}
	sort.Ints(subtaskIDs)

	timeLimitMs, memoryLimitKB := wp.getEffectiveLimits(prob, sub.Language)
	cpuLimitNs := uint64(float64(timeLimitMs)*langCfg.TimeLimitMultiplier) * 1_000_000
	memLimitBytes := uint64(float64(memoryLimitKB)*langCfg.MemoryLimitMultiplier) * 1024

	var allResults []model.TestCaseResult
	totalScore, maxScore := 0, 0
	maxTime, maxMemory := 0, 0
	finalStatus := model.StatusAC

	for _, stID := range subtaskIDs {
		cases := subtasks[stID]
		subtaskScore := 0
		subtaskFailed := false

		for _, tc := range cases {
			maxScore += tc.Score
			result := wp.runTestCase(ctx, prob, langCfg, tc, copyIn, compiledResult, spjExeDir, spjBinContent, cpuLimitNs, memLimitBytes)
			allResults = append(allResults, result)

			if result.Time > maxTime {
				maxTime = result.Time
			}
			if result.Memory > maxMemory {
				maxMemory = result.Memory
			}

			if result.Status == model.StatusAC {
				if !subtaskFailed {
					subtaskScore += tc.Score
				}
			} else {
				if prob.SubtaskAggregation == "min" {
					subtaskFailed = true
					subtaskScore = 0
				}
				if finalStatus == model.StatusAC {
					finalStatus = result.Status
				}
			}
		}
		totalScore += subtaskScore
	}

	percentageScore := 0
	if maxScore > 0 {
		percentageScore = (totalScore * 100) / maxScore
	}

	wp.subStore.UpdateResult(ctx, sub.ID, finalStatus, percentageScore, maxTime, maxMemory, "", allResults)
	if finalStatus == model.StatusAC && sub.ContestID != "" {
		_ = wp.balloonStore.CreateRequest(ctx, sub.ContestID, sub.ID, sub.UserID, sub.ProblemID)
	}
	wp.probStore.UpdateCounts(ctx, sub.ProblemID, 1, boolToInt(finalStatus == model.StatusAC))
}

func (wp *WorkerPool) judgeInteractive(ctx context.Context, sub *model.Submission, prob *model.Problem) {
	langs, _ := compiler.LoadLanguages(wp.langDir)
	cfg := langs[sub.Language]
	if cfg == nil {
		wp.subStore.UpdateResult(ctx, sub.ID, model.StatusCE, 0, 0, 0, "unsupported language: "+sub.Language, nil)
		return
	}

	// Compile contestant code
	compiledBinContent := ""
	if cfg.CompileCmd != "" {
		srcName := "Main" + cfg.Extensions[0]
		exeName := "Main"
		cmdStr := cfg.CompileCmd
		cmdStr = strings.ReplaceAll(cmdStr, "{{exe}}", exeName)
		cmdStr = strings.ReplaceAll(cmdStr, "{{src}}", srcName)
		cmdStr = strings.ReplaceAll(cmdStr, "{{dir}}", "/box")

		copyIn := map[string]executor.CmdFile{srcName: {Content: sub.SourceCode}}

		slog.Info("compiling contestant", "lang", sub.Language)
		resp, err := wp.exec.Run(&executor.ExecRequest{
			Cmd: []executor.Cmd{{
				Args:        []string{"/bin/sh", "-c", cmdStr},
				Env:         []string{"PATH=/usr/bin:/bin"},
				CPULimit:    30_000_000_000,
				MemoryLimit: 536_870_912,
				ProcLimit:   64,
				CopyIn:      copyIn,
				CopyOut:     []string{exeName},
			}},
		})
		if err != nil {
			wp.subStore.UpdateResult(ctx, sub.ID, model.StatusCE, 0, 0, 0, err.Error(), nil)
			return
		}
		if len(resp) == 0 || (resp[0].Status != "Accepted" && resp[0].Status != "Nonzero Exit Status") {
			ce := "compile error: unexpected status"
			if len(resp) > 0 && resp[0].Error != "" {
				ce = resp[0].Error
			}
			wp.subStore.UpdateResult(ctx, sub.ID, model.StatusCE, 0, 0, 0, ce, nil)
			return
		}
		if resp[0].Status == "Nonzero Exit Status" {
			ceMsg := "compile error: nonzero exit status"
			if resp[0].Error != "" {
				ceMsg = resp[0].Error
			}
			wp.subStore.UpdateResult(ctx, sub.ID, model.StatusCE, 0, 0, 0, ceMsg, nil)
			return
		}
		if binData, ok := resp[0].Files[exeName]; ok {
			compiledBinContent = binData
		}
	}

	// Compile interactor
	interLang := prob.InteractorLanguage
	if interLang == "" {
		interLang = "cpp-gpp-64"
	}
	interCfg := langs[interLang]
	if interCfg == nil {
		wp.subStore.UpdateResult(ctx, sub.ID, model.StatusSE, 0, 0, 0, "unsupported interactor language: "+interLang, nil)
		return
	}

	interSrcName := "interactor" + interCfg.Extensions[0]
	interExeName := "interactor"
	interCmdStr := interCfg.CompileCmd
	interCmdStr = strings.ReplaceAll(interCmdStr, "{{exe}}", interExeName)
	interCmdStr = strings.ReplaceAll(interCmdStr, "{{src}}", interSrcName)
	interCmdStr = strings.ReplaceAll(interCmdStr, "{{dir}}", "/box")

	interCopyIn := map[string]executor.CmdFile{interSrcName: {Content: prob.InteractorSourceCode}}

	slog.Info("compiling interactor", "lang", interLang)
	interResp, err := wp.exec.Run(&executor.ExecRequest{
		Cmd: []executor.Cmd{{
			Args:        []string{"/bin/sh", "-c", interCmdStr},
			Env:         []string{"PATH=/usr/bin:/bin"},
			CPULimit:    30_000_000_000,
			MemoryLimit: 536_870_912,
			ProcLimit:   64,
			CopyIn:      interCopyIn,
			CopyOut:     []string{interExeName},
		}},
	})
	if err != nil {
		wp.subStore.UpdateResult(ctx, sub.ID, model.StatusSE, 0, 0, 0, "interactor compilation request failed: "+err.Error(), nil)
		return
	}
	if len(interResp) == 0 || (interResp[0].Status != "Accepted" && interResp[0].Status != "Nonzero Exit Status") {
		ceMsg := "interactor compile error: unexpected status"
		if len(interResp) > 0 && interResp[0].Error != "" {
			ceMsg = interResp[0].Error
		}
		wp.subStore.UpdateResult(ctx, sub.ID, model.StatusSE, 0, 0, 0, ceMsg, nil)
		return
	}
	if interResp[0].Status == "Nonzero Exit Status" {
		wp.subStore.UpdateResult(ctx, sub.ID, model.StatusSE, 0, 0, 0, "interactor compile error:\n"+interResp[0].Error, nil)
		return
	}
	interBinContent := ""
	if binData, ok := interResp[0].Files["interactor"]; ok {
		interBinContent = binData
	}

	// Run interactive judging for each test case
	results := make([]model.TestCaseResult, 0)
	finalStatus := model.StatusAC
	maxTime, maxMem, totalScore := 0, 0, 0

	timeLimitMs, memoryLimitKB := wp.getEffectiveLimits(prob, sub.Language)
	cpuLimitNs := uint64(float64(timeLimitMs) * float64(cfg.TimeLimitMultiplier)) * 1_000_000
	memLimitBytes := uint64(float64(memoryLimitKB) * float64(cfg.MemoryLimitMultiplier)) * 1024

	srcName := "Main" + cfg.Extensions[0]

	for _, tc := range prob.TestCaseScore {
		inputContent := loadFile(filepath.Join(prob.TestdataPath, tc.InputName))

		interRunCopyIn := map[string]executor.CmdFile{
			"interactor": {Content: interBinContent},
		}
		// Some interactors read the input file
		if inputContent != "" {
			interRunCopyIn["input.txt"] = executor.CmdFile{Content: inputContent}
		}

		contestantRunCopyIn := map[string]executor.CmdFile{}
		var contestantArgs []string
		if cfg.Runtime != "" {
			rtParts := strings.Fields(cfg.Runtime)
			contestantArgs = append(rtParts, "/box/"+srcName)
			contestantRunCopyIn[srcName] = executor.CmdFile{Content: sub.SourceCode}
		} else {
			contestantArgs = []string{"/box/Main"}
			contestantRunCopyIn["Main"] = executor.CmdFile{Content: compiledBinContent}
		}

		interArgs := []string{"/box/interactor"}
		if inputContent != "" {
			interArgs = append(interArgs, "/box/input.txt")
		}

		result, err := wp.runInteractive(ctx, interArgs, interRunCopyIn, contestantArgs, contestantRunCopyIn, cpuLimitNs, memLimitBytes)
		if err != nil {
			results = append(results, model.TestCaseResult{
				CaseName: tc.InputName,
				Status:   model.StatusSE,
				Detail:   err.Error(),
			})
			if finalStatus == model.StatusAC {
				finalStatus = model.StatusSE
			}
			continue
		}

		r := model.TestCaseResult{
			CaseName: tc.InputName,
			Time:     int(result.Time / 1_000_000), // ns to ms
			Memory:   int(result.Memory / 1024),     // bytes to KB
		}

		switch result.Status {
		case "ac":
			r.Status = model.StatusAC
			r.Score = tc.Score
			totalScore += tc.Score
		case "wa":
			r.Status = model.StatusWA
			r.Detail = result.Message
		case "tle":
			r.Status = model.StatusTLE
		case "mle":
			r.Status = model.StatusMLE
		case "re":
			r.Status = model.StatusRE
			r.Detail = result.Message
		default:
			r.Status = model.StatusSE
			r.Detail = result.Message
		}

		results = append(results, r)
		if r.Time > maxTime {
			maxTime = r.Time
		}
		if r.Memory > maxMem {
			maxMem = r.Memory
		}
		if r.Status != model.StatusAC && finalStatus == model.StatusAC {
			finalStatus = r.Status
		}
	}

	avgScore := 0
	if len(prob.TestCaseScore) > 0 {
		avgScore = totalScore / len(prob.TestCaseScore)
	}

	wp.subStore.UpdateResult(ctx, sub.ID, finalStatus, avgScore, maxTime, maxMem, "", results)
	if finalStatus == model.StatusAC && sub.ContestID != "" {
		_ = wp.balloonStore.CreateRequest(ctx, sub.ContestID, sub.ID, sub.UserID, sub.ProblemID)
	}
	wp.probStore.UpdateCounts(ctx, prob.ID, 1, boolToInt(finalStatus == model.StatusAC))
	slog.Info("judged interactive", "id", sub.ID, "verdict", finalStatus)
}

func (wp *WorkerPool) judgeOutputOnly(ctx context.Context, sub *model.Submission, prob *model.Problem) {
	contestantOutput := []byte(sub.SourceCode)

	var results []model.TestCaseResult
	totalScore := 0
	maxScore := 0
	allPassed := true

	chk := checker.GetChecker(prob.CheckerType, prob.FloatEpsilon)

	for _, tc := range prob.TestCaseScore {
		maxScore += tc.Score

		expectedOutput, err := os.ReadFile(filepath.Join(prob.TestdataPath, tc.OutputName))
		if err != nil {
			results = append(results, model.TestCaseResult{
				CaseName: tc.InputName,
				Status:   model.StatusSE,
				Detail:   fmt.Sprintf("failed to load expected output: %v", err),
			})
			allPassed = false
			continue
		}

		result := chk.Check(nil, expectedOutput, contestantOutput)
		score := 0
		status := model.StatusWA
		if result.Passed {
			score = tc.Score
			status = model.StatusAC
		} else {
			allPassed = false
		}
		totalScore += score

		results = append(results, model.TestCaseResult{
			CaseName: tc.InputName,
			Status:   status,
			Score:    score,
			Detail:   result.Message,
		})
	}

	finalStatus := model.StatusWA
	if allPassed {
		finalStatus = model.StatusAC
	}

	percentageScore := 0
	if maxScore > 0 {
		percentageScore = (totalScore * 100) / maxScore
	}

	wp.subStore.UpdateResult(ctx, sub.ID, finalStatus, percentageScore, 0, 0, "", results)
	if finalStatus == model.StatusAC && sub.ContestID != "" {
		_ = wp.balloonStore.CreateRequest(ctx, sub.ContestID, sub.ID, sub.UserID, sub.ProblemID)
	}
	wp.probStore.UpdateCounts(ctx, sub.ProblemID, 1, boolToInt(finalStatus == model.StatusAC))
}
