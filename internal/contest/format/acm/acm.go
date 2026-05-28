package acm

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/tahsinarafat/aioj/internal/contest/format"
)

func init() {
	format.Register("acm", func(config json.RawMessage) (format.ContestFormat, error) {
		cfg := DefaultConfig()
		if len(config) > 0 {
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("invalid acm config: %w", err)
			}
		}
		if cfg.PenaltyPerWrong < 0 {
			return nil, fmt.Errorf("penalty_per_wrong must be non-negative, got %d", cfg.PenaltyPerWrong)
		}
		return &ACMFormat{config: cfg}, nil
	})
}

type Config struct {
	PenaltyPerWrong int  `json:"penalty_per_wrong"`
	TimePenalty     bool `json:"time_penalty"`
}

func DefaultConfig() Config {
	return Config{
		PenaltyPerWrong: 20,
		TimePenalty:     true,
	}
}

type ACMFormat struct {
	config Config
}

func (f *ACMFormat) Name() string { return "acm" }

func (f *ACMFormat) ScoreProblem(ctx format.ScoringContext) (format.ProblemResult, error) {
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

	wrongAttempts := 0
	for _, sub := range ctx.Submissions {
		result.Attempts++
		if sub.Status == "AC" {
			result.Solved = true
			result.Score = 1
			firstACTime := sub.CreatedAt
			result.FirstACTime = &firstACTime

			if f.config.TimePenalty {
				minutes := int(sub.CreatedAt.Sub(ctx.SubmissionStartTime).Minutes())
				result.Penalty = minutes + (wrongAttempts * f.config.PenaltyPerWrong)
			}
			break
		}
		wrongAttempts++
	}

	return result, nil
}

func (f *ACMFormat) RankParticipants(participants []format.ParticipantScore) []format.Rank {
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

func (f *ACMFormat) ValidateConfig(config json.RawMessage) error {
	var cfg Config
	if err := json.Unmarshal(config, &cfg); err != nil {
		return fmt.Errorf("invalid acm config: %w", err)
	}
	if cfg.PenaltyPerWrong < 0 {
		return fmt.Errorf("penalty_per_wrong must be non-negative")
	}
	return nil
}

func (f *ACMFormat) DefaultConfig() json.RawMessage {
	cfg := DefaultConfig()
	data, _ := json.Marshal(cfg)
	return data
}
