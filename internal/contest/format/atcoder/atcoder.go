package atcoder

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/tahsinarafat/aioj/internal/contest/format"
)

func init() {
	format.Register("atcoder", func(config json.RawMessage) (format.ContestFormat, error) {
		cfg := DefaultConfig()
		if len(config) > 0 {
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("invalid atcoder config: %w", err)
			}
		}
		return &AtCoderFormat{config: cfg}, nil
	})
}

type Config struct {
	PenaltyIsTimeOfAC     bool `json:"penalty_is_time_of_ac"`
	NoWrongAttemptPenalty bool `json:"no_wrong_attempt_penalty"`
}

func DefaultConfig() Config {
	return Config{
		PenaltyIsTimeOfAC:     true,
		NoWrongAttemptPenalty: true,
	}
}

type AtCoderFormat struct {
	config Config
}

func (f *AtCoderFormat) Name() string { return "atcoder" }

func (f *AtCoderFormat) ScoreProblem(ctx format.ScoringContext) (format.ProblemResult, error) {
	result := format.ProblemResult{
		ProblemID:    ctx.Problem.ID,
		ProblemIndex: ctx.Problem.Index,
		Solved:       false,
		Score:        0,
		Attempts:     0,
		Penalty:      0,
	}

	if len(ctx.Submissions) == 0 {
		return result, nil
	}

	result.Attempts = len(ctx.Submissions)

	for _, sub := range ctx.Submissions {
		result.Attempts = len(ctx.Submissions)
		if sub.Status == "AC" {
			result.Solved = true
			result.Score = 1
			firstACTime := sub.CreatedAt
			result.FirstACTime = &firstACTime

			if f.config.PenaltyIsTimeOfAC {
				minutes := int(sub.CreatedAt.Sub(ctx.SubmissionStartTime).Minutes())
				result.Penalty = minutes
			}
			break
		}
	}

	return result, nil
}

func (f *AtCoderFormat) RankParticipants(participants []format.ParticipantScore) []format.Rank {
	sorted := make([]format.ParticipantScore, len(participants))
	copy(sorted, participants)

	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].TotalSolved != sorted[j].TotalSolved {
			return sorted[i].TotalSolved > sorted[j].TotalSolved
		}
		return sorted[i].TotalPenalty < sorted[j].TotalPenalty
	})

	ranks := make([]format.Rank, len(sorted))
	for i, p := range sorted {
		position := i + 1
		if i > 0 && p.TotalSolved == sorted[i-1].TotalSolved && p.TotalPenalty == sorted[i-1].TotalPenalty {
			position = ranks[i-1].Position
		}
		ranks[i] = format.Rank{Position: position, Score: p}
	}

	return ranks
}

func (f *AtCoderFormat) ValidateConfig(config json.RawMessage) error {
	var cfg Config
	if err := json.Unmarshal(config, &cfg); err != nil {
		return fmt.Errorf("invalid atcoder config: %w", err)
	}
	return nil
}

func (f *AtCoderFormat) DefaultConfig() json.RawMessage {
	cfg := DefaultConfig()
	data, _ := json.Marshal(cfg)
	return data
}
