package judge

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
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

	langs, _ := compiler.LoadLanguages(wp.langDir)
	cfg := langs[sub.Language]
	if cfg == nil {
		wp.subStore.UpdateResult(ctx, submissionID, model.StatusCE, 0, 0, 0, "unsupported language: "+sub.Language, nil)
		return
	}

	// Compile if needed
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
		if len(resp.Results) == 0 || resp.Results[0].Status != "Accepted" {
			ce := "compile error"
			if len(resp.Results) > 0 {
				ce = resp.Results[0].Error
			}
			wp.subStore.UpdateResult(ctx, submissionID, model.StatusCE, 0, 0, 0, ce, nil)
			return
		}
		compiledExeDir = resp.Results[0].RunDir
	}

	// Run test cases
	var results []model.TestCaseResult
	finalStatus := model.StatusAC
	maxTime, maxMem, totalScore := 0, 0, 0

	srcName := "Main" + cfg.Extensions[0]

	for _, tc := range prob.TestCaseScore {
		cpuNs := uint64(float64(prob.TimeLimit) * 1e6 * cfg.TimeLimitMultiplier)
		memBytes := uint64(float64(prob.MemoryLimit) * 1024 * cfg.MemoryLimitMultiplier)

		copyIn := map[string]executor.CmdFile{}
		var args []string

		if cfg.Runtime != "" {
			// Interpreted
			rtParts := strings.Fields(cfg.Runtime)
			args = append(rtParts, "/box/"+srcName)
			copyIn[srcName] = executor.CmdFile{Content: sub.SourceCode}
		} else {
			// Compiled
			args = []string{"/box/Main"}
			copyIn["Main"] = executor.CmdFile{Src: filepath.Join(compiledExeDir, "Main")}
		}

		start := time.Now()
		resp, err := wp.exec.Run(&executor.ExecRequest{
			Cmd: []executor.Cmd{{
				Args:        args,
				Env:         []string{"PATH=/usr/bin:/bin"},
				CPULimit:    cpuNs,
				MemoryLimit: memBytes,
				ProcLimit:   16,
				CopyIn:      copyIn,
				CopyOut:     []string{"stdout"},
			}},
			PipeInput: true,
		})
		elapsed := int(time.Since(start).Milliseconds())

		r := model.TestCaseResult{CaseName: tc.InputName, Time: elapsed}

		if err != nil {
			r.Status = model.StatusSE
			r.Detail = err.Error()
		} else if len(resp.Results) == 0 {
			r.Status = model.StatusSE
			r.Detail = "no result"
		} else {
			cr := resp.Results[0]
			r.Memory = int(cr.Memory / 1024)
			switch cr.Status {
			case "Accepted":
				output := ""
				if f, ok := cr.Files["stdout"]; ok {
					output = f.Content
				}
				expected := loadFile(filepath.Join(prob.TestdataPath, tc.OutputName))
				chk := checker.GetChecker("exact")
				if ck := chk.Check(nil, []byte(expected), []byte(output)); ck.Passed {
					r.Status = model.StatusAC
					r.Score = tc.Score
				} else {
					r.Status = model.StatusWA
					r.Detail = ck.Message
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
