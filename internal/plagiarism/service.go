package plagiarism

import (
	"context"
	"errors"
	"log/slog"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type Service struct {
	store    store.PlagiarismStore
	contestS store.ContestStore
	subS     store.SubmissionStore

	// engine performs the scan. fallback runs instead when engine fails at
	// runtime (a missing binary, a crashed process, a protocol mismatch), so a
	// broken deployment degrades to the slower engine instead of failing every
	// report. It is nil when engine is already the native one.
	engine   Engine
	fallback Engine
}

// NewService wires a service with the pure-Go engine.
func NewService(s store.PlagiarismStore, c store.ContestStore, sub store.SubmissionStore) *Service {
	return &Service{store: s, contestS: c, subS: sub, engine: NativeEngine{}}
}

// NewServiceWithEngine wires a service with a specific engine, keeping the
// pure-Go engine as a runtime fallback unless that is the engine requested.
func NewServiceWithEngine(s store.PlagiarismStore, c store.ContestStore, sub store.SubmissionStore, engine Engine) *Service {
	svc := NewService(s, c, sub)
	if engine == nil {
		return svc
	}
	svc.engine = engine
	if _, isNative := engine.(NativeEngine); !isNative {
		svc.fallback = NativeEngine{}
	}
	return svc
}

// EngineName reports which engine this service will use, for startup logs.
func (s *Service) EngineName() string {
	if s.engine == nil {
		return "none"
	}
	return s.engine.Name()
}

// RunCheck compares accepted submissions inside a contest and stores the pairs
// above threshold. It honours ctx so a caller-imposed deadline (or shutdown)
// cannot leave the report stuck in "running".
//
// The scan is one batch call to the engine. The previous implementation looped
// pair by pair and wrote each flagged pair as it went; that kept partial progress
// visible but cost a full re-tokenization and a 2D LCS matrix per comparison,
// which is why a mid-sized contest could not finish inside its own timeout.
func (s *Service) RunCheck(ctx context.Context, reportID string, contestID string, threshold float64) {
	if err := s.store.UpdateReportStatus(ctx, reportID, model.PlagiarismStatusRunning, ""); err != nil {
		slog.Error("plagiarism: mark running failed", "report", reportID, "error", err)
	}

	problems, err := s.contestS.GetProblems(ctx, contestID)
	if err != nil {
		s.fail(ctx, reportID, "failed to get problems: "+err.Error())
		return
	}

	sources, err := s.collectSources(ctx, contestID, problems)
	if err != nil {
		s.failCancelled(ctx, reportID, err)
		return
	}

	// The report's denominator is computed in Go so it stays correct regardless
	// of which engine ran.
	totalPairs := CountEligiblePairs(sources)

	result, err := s.scan(ctx, sources, threshold)
	if err != nil {
		if ctx.Err() != nil {
			s.failCancelled(ctx, reportID, ctx.Err())
			return
		}
		s.fail(ctx, reportID, "similarity scan failed: "+err.Error())
		return
	}
	if result.PairsTotal != totalPairs {
		// Engines must agree with CountEligiblePairs. A mismatch would change the
		// number an admin sees, so it is worth a warning even though the
		// authoritative value is the one stored.
		slog.Warn("plagiarism: engine pair count disagrees with the service",
			"report", reportID, "engine", s.engine.Name(),
			"engine_total", result.PairsTotal, "expected", totalPairs)
	}

	authorOf := make(map[string]string, len(sources))
	for _, src := range sources {
		authorOf[src.Key] = src.UserID
	}

	flagged := 0
	for i, p := range result.Pairs {
		// A long scan for a big contest can still take a moment; check between
		// rows so cancellation is respected without per-row overhead.
		if i%64 == 0 {
			if err := ctx.Err(); err != nil {
				s.failCancelled(ctx, reportID, err)
				return
			}
		}
		pair := &model.PlagiarismPair{
			ReportID:      reportID,
			ProblemID:     p.ProblemID,
			SubmissionAID: p.A,
			SubmissionBID: p.B,
			UserAID:       authorOf[p.A],
			UserBID:       authorOf[p.B],
			Similarity:    p.Similarity,
			MatchedLines:  p.MatchedTokens,
		}
		if err := s.store.CreatePair(ctx, pair); err != nil {
			slog.Error("plagiarism: store pair failed", "report", reportID, "error", err)
			continue
		}
		flagged++
	}

	if err := s.store.UpdateReportCounts(ctx, reportID, totalPairs, flagged); err != nil {
		slog.Error("plagiarism: update counts failed", "report", reportID, "error", err)
	}
	if err := s.store.UpdateReportStatus(ctx, reportID, model.PlagiarismStatusCompleted, ""); err != nil {
		slog.Error("plagiarism: mark completed failed", "report", reportID, "error", err)
	}
	slog.Info("plagiarism check completed",
		"report", reportID,
		"engine", s.engine.Name(),
		"submissions", len(sources),
		"total", totalPairs,
		"scored", result.PairsScored,
		"flagged", flagged)
}

// collectSources gathers the contest's accepted submissions per problem, capped
// at maxSubmissionsPerProblem. A problem whose submissions cannot be listed is
// skipped, as before: one unreadable problem must not void the whole report.
func (s *Service) collectSources(ctx context.Context, contestID string, problems []model.ContestProblem) ([]Source, error) {
	var sources []Source
	for _, p := range problems {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		subs, _, err := s.subS.ListByProblem(ctx, p.ProblemID, 0, maxSubmissionsPerProblem)
		if err != nil {
			slog.Warn("plagiarism: skipping problem", "problem", p.ProblemID, "error", err)
			continue
		}
		for _, sub := range subs {
			if sub.ContestID != contestID || sub.Status != model.StatusAC {
				continue
			}
			sources = append(sources, Source{
				Key:       sub.ID,
				ProblemID: p.ProblemID,
				UserID:    sub.UserID,
				Code:      sub.SourceCode,
			})
		}
	}
	return sources, nil
}

// scan runs the configured engine, falling back to the native one on failure.
func (s *Service) scan(ctx context.Context, sources []Source, threshold float64) (*ScanResult, error) {
	result, err := s.engine.FindSimilar(ctx, sources, threshold)
	if err == nil || s.fallback == nil || ctx.Err() != nil {
		return result, err
	}

	// A cancelled context is not an engine failure; do not spend more CPU on it.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil, err
	}

	slog.Error("plagiarism: engine failed; falling back to the pure-Go engine",
		"engine", s.engine.Name(), "error", err)
	return s.fallback.FindSimilar(ctx, sources, threshold)
}

func (s *Service) fail(ctx context.Context, reportID, message string) {
	_ = s.store.UpdateReportStatus(ctx, reportID, model.PlagiarismStatusFailed, message)
}

// failCancelled records a cancellation without letting the cancelled context
// suppress the write — a report stuck in "running" would block the next attempt.
func (s *Service) failCancelled(ctx context.Context, reportID string, cause error) {
	_ = s.store.UpdateReportStatus(context.WithoutCancel(ctx), reportID,
		model.PlagiarismStatusFailed, "cancelled: "+cause.Error())
}
