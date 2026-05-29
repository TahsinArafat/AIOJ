package plagiarism

import (
	"context"
	"log/slog"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type Service struct {
	store    store.PlagiarismStore
	contestS store.ContestStore
	subS     store.SubmissionStore
}

func NewService(s store.PlagiarismStore, c store.ContestStore, sub store.SubmissionStore) *Service {
	return &Service{store: s, contestS: c, subS: sub}
}

func (s *Service) RunCheck(ctx context.Context, reportID string, contestID string, threshold float64) {
	_ = s.store.UpdateReportStatus(ctx, reportID, model.PlagiarismStatusRunning, "")

	problems, err := s.contestS.GetProblems(ctx, contestID)
	if err != nil {
		_ = s.store.UpdateReportStatus(ctx, reportID, model.PlagiarismStatusFailed, "failed to get problems: "+err.Error())
		return
	}

	totalPairs := 0
	flaggedPairs := 0

	for _, p := range problems {
		subs, _, err := s.subS.ListByProblem(ctx, p.ProblemID, 0, 1000)
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
					_ = s.store.CreatePair(ctx, pair)
				}
			}
		}
	}

	_ = s.store.UpdateReportCounts(ctx, reportID, totalPairs, flaggedPairs)
	_ = s.store.UpdateReportStatus(ctx, reportID, model.PlagiarismStatusCompleted, "")
	slog.Info("plagiarism check completed", "report", reportID, "total", totalPairs, "flagged", flaggedPairs)
}
