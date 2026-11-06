package plagiarism

import (
	"context"
	"log/slog"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

// maxSubmissionsPerProblem bounds the pairwise comparison. The scan is O(n²) in
// accepted submissions, so a popular problem with 1000 ACs means ~500k LCS
// comparisons; 300 keeps one problem to ~45k.
const maxSubmissionsPerProblem = 300

type Service struct {
	store    store.PlagiarismStore
	contestS store.ContestStore
	subS     store.SubmissionStore
}

func NewService(s store.PlagiarismStore, c store.ContestStore, sub store.SubmissionStore) *Service {
	return &Service{store: s, contestS: c, subS: sub}
}

// RunCheck compares accepted submissions inside a contest and stores the pairs
// above threshold. It honours ctx so a caller-imposed deadline (or shutdown)
// cannot leave the report stuck in "running".
func (s *Service) RunCheck(ctx context.Context, reportID string, contestID string, threshold float64) {
	if err := s.store.UpdateReportStatus(ctx, reportID, model.PlagiarismStatusRunning, ""); err != nil {
		slog.Error("plagiarism: mark running failed", "report", reportID, "error", err)
	}

	problems, err := s.contestS.GetProblems(ctx, contestID)
	if err != nil {
		_ = s.store.UpdateReportStatus(ctx, reportID, model.PlagiarismStatusFailed, "failed to get problems: "+err.Error())
		return
	}

	totalPairs := 0
	flaggedPairs := 0

	for _, p := range problems {
		if ctx.Err() != nil {
			_ = s.store.UpdateReportStatus(context.WithoutCancel(ctx), reportID,
				model.PlagiarismStatusFailed, "cancelled: "+ctx.Err().Error())
			return
		}

		subs, _, err := s.subS.ListByProblem(ctx, p.ProblemID, 0, maxSubmissionsPerProblem)
		if err != nil {
			continue
		}

		var contestACs []model.Submission
		for _, sub := range subs {
			if sub.ContestID != "" && sub.ContestID == contestID && sub.Status == model.StatusAC {
				contestACs = append(contestACs, sub)
			}
		}

		for i := 0; i < len(contestACs); i++ {
			for j := i + 1; j < len(contestACs); j++ {
				if ctx.Err() != nil {
					_ = s.store.UpdateReportStatus(context.WithoutCancel(ctx), reportID,
						model.PlagiarismStatusFailed, "cancelled: "+ctx.Err().Error())
					return
				}

				subA := contestACs[i]
				subB := contestACs[j]

				if subA.UserID == subB.UserID {
					continue
				}

				similarity := CompareCodes(subA.SourceCode, subB.SourceCode)
				totalPairs++

				if similarity >= threshold {
					flaggedPairs++
					pair := &model.PlagiarismPair{
						ReportID:      reportID,
						ProblemID:     p.ProblemID,
						SubmissionAID: subA.ID,
						SubmissionBID: subB.ID,
						UserAID:       subA.UserID,
						UserBID:       subB.UserID,
						Similarity:    similarity,
						MatchedLines:  LCSLength(Tokenize(subA.SourceCode), Tokenize(subB.SourceCode)),
					}
					if err := s.store.CreatePair(ctx, pair); err != nil {
						slog.Error("plagiarism: store pair failed", "report", reportID, "error", err)
					}
				}
			}
		}
	}

	_ = s.store.UpdateReportCounts(ctx, reportID, totalPairs, flaggedPairs)
	_ = s.store.UpdateReportStatus(ctx, reportID, model.PlagiarismStatusCompleted, "")
	slog.Info("plagiarism check completed", "report", reportID, "total", totalPairs, "flagged", flaggedPairs)
}
