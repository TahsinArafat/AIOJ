package oi

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/tahsinarafat/aioj/internal/contest/format"
)

func init() {
	format.Register("oi", func(config json.RawMessage) (format.ContestFormat, error) {
		cfg := DefaultConfig()
		if len(config) > 0 {
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("invalid oi config: %w", err)
			}
		}
		if cfg.MaxScorePerProblem <= 0 {
			return nil, fmt.Errorf("max_score_per_problem must be positive, got %d", cfg.MaxScorePerProblem)
		}
		return &OIFormat{config: cfg}, nil
	})
}

type Config struct {
	MaxScorePerProblem int `json:"max_score_per_problem"`
}

func DefaultConfig() Config {
	return Config{
		MaxScorePerProblem: 100,
	}
}

type OIFormat struct {
	config Config
}

func (f *OIFormat) Name() string { return "oi" }

func (f *OIFormat) ScoreProblem(ctx format.ScoringContext) (format.ProblemResult, error) {
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

	maxScore := 0.0
	for _, sub := range ctx.Submissions {
		if sub.Score > maxScore {
			maxScore = sub.Score
		}
	}

	normalizedScore := maxScore * float64(f.config.MaxScorePerProblem) / 100.0
	result.Score = normalizedScore
	result.Solved = normalizedScore >= float64(f.config.MaxScorePerProblem)

	return result, nil
}

func (f *OIFormat) RankParticipants(participants []format.ParticipantScore) []format.Rank {
	sorted := make([]format.ParticipantScore, len(participants))
	copy(sorted, participants)

	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].TotalScore != sorted[j].TotalScore {
			return sorted[i].TotalScore > sorted[j].TotalScore
		}
		return sorted[i].TotalPenalty < sorted[j].TotalPenalty
	})

	ranks := make([]format.Rank, len(sorted))
	for i, p := range sorted {
		position := i + 1
		if i > 0 && p.TotalScore == sorted[i-1].TotalScore {
			position = ranks[i-1].Position
		}
		ranks[i] = format.Rank{Position: position, Score: p}
	}

	return ranks
}

func (f *OIFormat) ValidateConfig(config json.RawMessage) error {
	var cfg Config
	if err := json.Unmarshal(config, &cfg); err != nil {
		return fmt.Errorf("invalid oi config: %w", err)
	}
	if cfg.MaxScorePerProblem <= 0 {
		return fmt.Errorf("max_score_per_problem must be positive")
	}
	return nil
}

func (f *OIFormat) DefaultConfig() json.RawMessage {
	cfg := DefaultConfig()
	data, _ := json.Marshal(cfg)
	return data
}
