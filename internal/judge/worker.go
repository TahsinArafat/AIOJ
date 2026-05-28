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
	queue     queue.JudgeQueue
	exec      *executor.Client
	langDir   string
	sem       chan struct{}
	subStore  store.SubmissionStore
	probStore store.ProblemStore
}

func NewWorkerPool(q queue.JudgeQueue, exec *executor.Client, langDir string, concurrency int,
	subStore store.SubmissionStore, probStore store.ProblemStore) *WorkerPool {
	return &WorkerPool{
		queue:     q,
		exec:      exec,
		langDir:   langDir,
		sem:       make(chan struct{}, concurrency),
		subStore:  subStore,
		probStore: probStore,
	}
}

func (wp *WorkerPool) Start(ctx context.Context) {
	for {
		subID, err := wp.queue.Dequeue(ctx)
		if err != nil {
			return
		}
		select {
		case wp.sem <- struct{}{}:
			go func(id string) {
				defer func() { <-wp.sem }()
				wp.judge(ctx, id)
			}(subID)
		case <-ctx.Done():
			return
		}
	}
}

func (wp *WorkerPool) judge(ctx context.Context, submissionID string) {
	slog.Info("judging", "id", submissionID)
	sub, err := wp.subStore.GetByID(ctx, submissionID)
	if err != nil || sub == nil {
		slog.Error("load sub failed", "id", submissionID, "error", err)
		return
	}

	wp.subStore.UpdateStatus(ctx, submissionID, model.StatusJudging)

	prob, err := wp.probStore.GetByID(ctx, sub.ProblemID)
	if err != nil || prob == nil {
		wp.subStore.UpdateResult(ctx, submissionID, model.StatusSE, 0, 0, 0, "problem not found", nil)
		return
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

	compileOutput := ""
	compiledExeDir := ""
	if cfg.CompileCmd != "" {
		srcName := "Main" + cfg.Extensions[0]
		exeName := "Main"
		cmdStr := cfg.CompileCmd
		cmdStr = strings.ReplaceAll(cmdStr, "{{exe}}", exeName)
		cmdStr = strings.ReplaceAll(cmdStr, "{{src}}", srcName)
		cmdStr = strings.ReplaceAll(cmdStr, "{{dir}}", "/box")

		copyIn := map[string]executor.CmdFile{srcName: {Content: sub.SourceCode}}

		slog.Info("compiling", "lang", sub.Language)
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
			wp.subStore.UpdateResult(ctx, submissionID, model.StatusCE, 0, 0, 0, err.Error(), nil)
			return
		}
		compiledStatus := resp[0].Status
		if len(resp) == 0 || (compiledStatus != "Accepted" && compiledStatus != "Nonzero Exit Status") {
			ce := "compile error: unexpected status: " + compiledStatus
			if resp[0].Error != "" {
				ce = resp[0].Error
			}
			wp.subStore.UpdateResult(ctx, submissionID, model.StatusCE, 0, 0, 0, ce, nil)
			return
		}
		if compiledStatus == "Nonzero Exit Status" {
			ceMsg := "compile error: nonzero exit status"
			if resp[0].Error != "" {
				ceMsg = resp[0].Error
			}
			wp.subStore.UpdateResult(ctx, submissionID, model.StatusCE, 0, 0, 0, ceMsg, nil)
			return
		}
		compiledExeDir = resp[0].RunDir
	}

	srcName := "Main" + cfg.Extensions[0]
	exeFile := map[string]executor.CmdFile{}
	if cfg.Runtime != "" {
		exeFile[srcName] = executor.CmdFile{Content: sub.SourceCode}
	} else {
		exeFile["Main"] = executor.CmdFile{Src: filepath.Join(compiledExeDir, "Main")}
	}

	timeLimitMs, memoryLimitKB := wp.getEffectiveLimits(prob, sub.Language)
	cpuNs := uint64(float64(timeLimitMs)*cfg.TimeLimitMultiplier) * 1_000_000
	memBytes := uint64(float64(memoryLimitKB)*cfg.MemoryLimitMultiplier) * 1024

	if prob.HasSubtasks() && prob.ScoringMode == "partial" {
		wp.evaluateSubtasks(ctx, sub, prob, cfg, exeFile, spjExeDir)
		return
	}

	results := make([]model.TestCaseResult, 0)
	finalStatus := model.StatusAC
	maxTime, maxMem, totalScore := 0, 0, 0

	for _, tc := range prob.TestCaseScore {
		r := wp.runTestCase(ctx, prob, cfg, tc, exeFile, spjExeDir, cpuNs, memBytes)
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

func (wp *WorkerPool) runTestCase(
	ctx context.Context,
	prob *model.Problem,
	langCfg *compiler.LangConfig,
	tc model.TestCaseScore,
	exeFile map[string]executor.CmdFile,
	spjExeDir string,
	cpuLimitNs uint64,
	memLimitBytes uint64,
) model.TestCaseResult {
	srcName := "Main" + langCfg.Extensions[0]
	var args []string
	if langCfg.Runtime != "" {
		rtParts := strings.Fields(langCfg.Runtime)
		args = append(rtParts, "/box/"+srcName)
	} else {
		args = []string{"/box/Main"}
	}

	inputContent := loadFile(filepath.Join(prob.TestdataPath, tc.InputName))

	start := time.Now()
	resp, err := wp.exec.Run(&executor.ExecRequest{
		Cmd: []executor.Cmd{{
			Args:        args,
			Env:         []string{"PATH=/usr/bin:/bin"},
			CPULimit:    cpuLimitNs,
			MemoryLimit: memLimitBytes,
			ProcLimit:   16,
			CopyIn:      exeFile,
			Files: []executor.CmdFile{
				{Content: inputContent},
				{Name: "stdout", Max: 10 * 1024 * 1024},
				{Name: "stderr", Max: 10 * 1024 * 1024},
			},
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
		if f, ok := cr.Files["stdout"]; ok {
			output = f
		}
		expected := loadFile(filepath.Join(prob.TestdataPath, tc.OutputName))

		if prob.SPJ && spjExeDir != "" {
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
			chk := checker.GetChecker(prob.CheckerType, prob.FloatEpsilon)
			if ck := chk.Check(nil, []byte(expected), []byte(output)); ck.Passed {
				r.Status = model.StatusAC
				r.Score = tc.Score
			} else {
				r.Status = model.StatusWA
				r.Detail = ck.Message
			}
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
	exeFile map[string]executor.CmdFile,
	spjExeDir string,
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
			result := wp.runTestCase(ctx, prob, langCfg, tc, exeFile, spjExeDir, cpuLimitNs, memLimitBytes)
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
	compiledExeDir := ""
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
		compiledExeDir = resp[0].RunDir
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
	interExeDir := interResp[0].RunDir

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
			"interactor": {Src: filepath.Join(interExeDir, "interactor")},
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
			contestantRunCopyIn["Main"] = executor.CmdFile{Src: filepath.Join(compiledExeDir, "Main")}
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
	wp.probStore.UpdateCounts(ctx, sub.ProblemID, 1, boolToInt(finalStatus == model.StatusAC))
}
