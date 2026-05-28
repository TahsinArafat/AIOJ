package ioi

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/tahsinarafat/aioj/internal/contest/format"
)

func init() {
	format.Register("ioi", func(config json.RawMessage) (format.ContestFormat, error) {
		cfg := DefaultConfig()
		if len(config) > 0 {
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("invalid ioi config: %w", err)
			}
		}
		return &IOIFormat{config: cfg}, nil
	})
}

type SubtaskConfig struct {
	ID         string  `json:"id"`
	Points     float64 `json:"points"`
	TestCases  int     `json:"test_cases"`
	Dependency string  `json:"dependency,omitempty"`
}

type ProblemConfig struct {
	Subtasks []SubtaskConfig `json:"subtasks"`
}

type Config struct {
	PartialCredit  bool `json:"partial_credit"`
	SubtaskScoring bool `json:"subtask_scoring"`
}

func DefaultConfig() Config {
	return Config{
		PartialCredit:  true,
		SubtaskScoring: true,
	}
}

type IOIFormat struct {
	config Config
}

func (f *IOIFormat) Name() string { return "ioi" }

func (f *IOIFormat) ScoreProblem(ctx format.ScoringContext) (format.ProblemResult, error) {
	result := format.ProblemResult{
		ProblemID:     ctx.Problem.ID,
		ProblemIndex:  ctx.Problem.Index,
		Solved:        false,
		Score:         0,
		Attempts:      0,
		Penalty:       0,
		SubtaskScores: make(map[string]float64),
	}

	if len(ctx.Submissions) == 0 {
		return result, nil
	}

	result.Attempts = len(ctx.Submissions)

	var problemCfg ProblemConfig
	if len(ctx.Problem.Config) > 0 {
		if err := json.Unmarshal(ctx.Problem.Config, &problemCfg); err != nil {
			return result, fmt.Errorf("invalid problem config: %w", err)
		}
	}

	if len(problemCfg.Subtasks) == 0 {
		maxScore := 0.0
		for _, sub := range ctx.Submissions {
			if sub.Score > maxScore {
				maxScore = sub.Score
			}
		}
		result.Score = maxScore
		result.Solved = maxScore >= 100
		return result, nil
	}

	subtaskBest := make(map[string]float64)
	for _, st := range problemCfg.Subtasks {
		subtaskBest[st.ID] = 0
	}

	for _, sub := range ctx.Submissions {
		subtaskScores := f.parseSubtaskScores(sub, problemCfg)
		for stID, score := range subtaskScores {
			if score > subtaskBest[stID] {
				subtaskBest[stID] = score
			}
		}
	}

	totalScore := 0.0
	for _, st := range problemCfg.Subtasks {
		score := subtaskBest[st.ID]

		if st.Dependency != "" {
			depScore := subtaskBest[st.Dependency]
			depMax := f.getSubtaskMaxPoints(st.Dependency, problemCfg)
			if depScore < depMax {
				score = 0
			}
		}

		result.SubtaskScores[st.ID] = score
		totalScore += score
	}

	result.Score = totalScore
	result.Solved = totalScore >= f.getMaxPoints(problemCfg)

	return result, nil
}

func (f *IOIFormat) parseSubtaskScores(sub format.Submission, cfg ProblemConfig) map[string]float64 {
	scores := make(map[string]float64)
	totalPoints := 0.0
	for _, st := range cfg.Subtasks {
		totalPoints += st.Points
	}

	for _, st := range cfg.Subtasks {
		scores[st.ID] = (sub.Score / 100.0) * st.Points
	}

	return scores
}

func (f *IOIFormat) getSubtaskMaxPoints(subtaskID string, cfg ProblemConfig) float64 {
	for _, st := range cfg.Subtasks {
		if st.ID == subtaskID {
			return st.Points
		}
	}
	return 0
}

func (f *IOIFormat) getMaxPoints(cfg ProblemConfig) float64 {
	total := 0.0
	for _, st := range cfg.Subtasks {
		total += st.Points
	}
	return total
}

func (f *IOIFormat) RankParticipants(participants []format.ParticipantScore) []format.Rank {
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

func (f *IOIFormat) ValidateConfig(config json.RawMessage) error {
	var cfg Config
	if err := json.Unmarshal(config, &cfg); err != nil {
		return fmt.Errorf("invalid ioi config: %w", err)
	}
	return nil
}

func (f *IOIFormat) DefaultConfig() json.RawMessage {
	cfg := DefaultConfig()
	data, _ := json.Marshal(cfg)
	return data
}
