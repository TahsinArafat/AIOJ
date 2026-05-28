package codeforces

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/tahsinarafat/aioj/internal/contest/format"
)

func init() {
	format.Register("codeforces", func(config json.RawMessage) (format.ContestFormat, error) {
		cfg := DefaultConfig()
		if len(config) > 0 {
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("invalid codeforces config: %w", err)
			}
		}
		if cfg.DecayFactor <= 0 {
			return nil, fmt.Errorf("decay_factor must be positive, got %d", cfg.DecayFactor)
		}
		if cfg.MinScoreRatio < 0 || cfg.MinScoreRatio > 1 {
			return nil, fmt.Errorf("min_score_ratio must be between 0 and 1, got %f", cfg.MinScoreRatio)
		}
		return &CodeforcesFormat{config: cfg}, nil
	})
}

type Config struct {
	InitialScores          []int   `json:"initial_scores"`
	DecayFactor            int     `json:"decay_factor"`
	MinScoreRatio          float64 `json:"min_score_ratio"`
	WrongSubmissionPenalty int     `json:"wrong_submission_penalty"`
}

func DefaultConfig() Config {
	return Config{
		InitialScores:          []int{500, 1000, 1500, 2000, 2500},
		DecayFactor:            250,
		MinScoreRatio:          0.3,
		WrongSubmissionPenalty: 50,
	}
}

type CodeforcesFormat struct {
	config Config
}

func (f *CodeforcesFormat) Name() string { return "codeforces" }

func (f *CodeforcesFormat) ScoreProblem(ctx format.ScoringContext) (format.ProblemResult, error) {
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

	initialScore := f.getInitialScore(ctx.Problem)

	var wrongBeforeAC int
	for _, sub := range ctx.Submissions {
		if sub.Status == "AC" {
			result.Solved = true
			firstACTime := sub.CreatedAt
			result.FirstACTime = &firstACTime

			minutes := sub.CreatedAt.Sub(ctx.SubmissionStartTime).Minutes()
			duration := ctx.ContestDuration.Minutes()

			decay := (120.0 * float64(initialScore) * minutes) / (float64(f.config.DecayFactor) * duration)
			penalty := float64(f.config.WrongSubmissionPenalty * wrongBeforeAC)
			rawScore := float64(initialScore) - decay - penalty

			minScore := float64(initialScore) * f.config.MinScoreRatio
			finalScore := math.Max(rawScore, minScore)

			result.Score = finalScore
			result.Penalty = int(minutes) + (wrongBeforeAC * 20)
			break
		}
		wrongBeforeAC++
	}

	return result, nil
}

func (f *CodeforcesFormat) getInitialScore(problem format.Problem) int {
	if len(f.config.InitialScores) == 0 {
		return 500
	}
	index := int(problem.Index[0] - 'A')
	if index < len(f.config.InitialScores) {
		return f.config.InitialScores[index]
	}
	return f.config.InitialScores[len(f.config.InitialScores)-1]
}

func (f *CodeforcesFormat) RankParticipants(participants []format.ParticipantScore) []format.Rank {
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
		if i > 0 && p.TotalScore == sorted[i-1].TotalScore && p.TotalPenalty == sorted[i-1].TotalPenalty {
			position = ranks[i-1].Position
		}
		ranks[i] = format.Rank{Position: position, Score: p}
	}

	return ranks
}

func (f *CodeforcesFormat) ValidateConfig(config json.RawMessage) error {
	var cfg Config
	if err := json.Unmarshal(config, &cfg); err != nil {
		return fmt.Errorf("invalid codeforces config: %w", err)
	}
	if cfg.DecayFactor <= 0 {
		return fmt.Errorf("decay_factor must be positive")
	}
	if cfg.MinScoreRatio < 0 || cfg.MinScoreRatio > 1 {
		return fmt.Errorf("min_score_ratio must be between 0 and 1")
	}
	if len(cfg.InitialScores) == 0 {
		return fmt.Errorf("initial_scores must not be empty")
	}
	for _, s := range cfg.InitialScores {
		if s <= 0 {
			return fmt.Errorf("initial_scores must be positive")
		}
	}
	return nil
}

func (f *CodeforcesFormat) DefaultConfig() json.RawMessage {
	cfg := DefaultConfig()
	data, _ := json.Marshal(cfg)
	return data
}
