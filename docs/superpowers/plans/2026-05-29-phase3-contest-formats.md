# Phase 3: Contest Depth — Pluggable Contest Format Registry

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Decouple contest scoring logic from hardcoded structures in `internal/api/handler/contest.go` and implement a clean pluggable registry pattern supporting 5 formats: ACM/ICPC, OI, IOI, AtCoder, Codeforces.

**Architecture:** Create a dynamic contest format registry under `internal/contest/format/` using Go's `init()` self-registration pattern. Update SQL database, model definitions, store layer, and handler functions, then rewrite the React SPA creation forms and scoreboard layout.

**Tech Stack:** Go 1.21+, PostgreSQL 18, React 19, TypeScript, Tailwind CSS

---

## File Structure

```
internal/contest/format/
├── types.go              # Core types: ScoringContext, Score, Rank, ProblemResult
├── registry.go           # Register/Get/List functions (database/sql pattern)
├── formats.go            # Blank imports to trigger init() registration
├── acm/acm.go            # ACM/ICPC format implementation
├── oi/oi.go              # OI format implementation
├── ioi/ioi.go            # IOI format implementation
├── atcoder/atcoder.go    # AtCoder format implementation
├── codeforces/codeforces.go # Codeforces format implementation
└── format_test.go        # Comprehensive test suite
```

---

## Tasks

### Task 1: Database Migration

**Files:**
- Create: `internal/store/migrations/000026_contest_formats.up.sql`
- Create: `internal/store/migrations/000026_contest_formats.down.sql`

- [ ] **Step 1: Write UP migration**
Create `internal/store/migrations/000026_contest_formats.up.sql`:

```sql
-- Add format column to contests table
ALTER TABLE contests ADD COLUMN format VARCHAR(20) NOT NULL DEFAULT 'acm';

-- Add format_config JSONB column for format-specific configuration
ALTER TABLE contests ADD COLUMN format_config JSONB NOT NULL DEFAULT '{}';

-- Backfill existing contests based on type
UPDATE contests SET format = 'acm' WHERE type IN ('acm', 'practice');
UPDATE contests SET format = 'oi' WHERE type = 'oi';
UPDATE contests SET format = 'ioi' WHERE type = 'ioi';
UPDATE contests SET format = 'codeforces' WHERE type = 'educational';

-- Add check constraint for valid formats
ALTER TABLE contests ADD CONSTRAINT valid_format 
    CHECK (format IN ('acm', 'oi', 'ioi', 'atcoder', 'codeforces'));

-- Add index for format queries
CREATE INDEX idx_contests_format ON contests(format);

-- Add comment for documentation
COMMENT ON COLUMN contests.format IS 'Scoring algorithm: acm, oi, ioi, atcoder, codeforces';
COMMENT ON COLUMN contests.format_config IS 'JSON configuration for format-specific settings';
```

- [ ] **Step 2: Write DOWN migration**
Create `internal/store/migrations/000026_contest_formats.down.sql`:

```sql
-- Remove index
DROP INDEX IF EXISTS idx_contests_format;

-- Remove columns
ALTER TABLE contests DROP COLUMN IF EXISTS format_config;
ALTER TABLE contests DROP COLUMN IF EXISTS format;
```

- [ ] **Step 3: Run migration**
Run `make migrate-up` to apply the migrations cleanly.

- [ ] **Step 4: Commit**
```bash
git add internal/store/migrations/000026_*
git commit -m "feat(contest-formats): add migrations for pluggable contest formats"
```

---

### Task 2: Core Types, Registry & Wiring

**Files:**
- Create: `internal/contest/format/types.go`
- Create: `internal/contest/format/registry.go`
- Create: `internal/contest/format/formats.go`

- [ ] **Step 1: Create types.go**
Create `internal/contest/format/types.go` containing all required scoring structures:

```go
package format

import (
	"encoding/json"
	"time"
)

type Submission struct {
	ID        int64
	UserID    int64
	ProblemID int64
	Status    string
	Score     float64
	CreatedAt time.Time
}

type Problem struct {
	ID           int64
	Index        string
	Title        string
	InitialScore int
	MaxScore     int
	Config       json.RawMessage
}

type ScoringContext struct {
	ContestID           int64
	ContestDuration     time.Duration
	SubmissionStartTime time.Time
	Problem             Problem
	Submissions         []Submission
	FormatConfig        json.RawMessage
}

type ProblemResult struct {
	ProblemID     int64
	ProblemIndex  string
	Solved        bool
	Score         float64
	Attempts      int
	Penalty       int
	FirstACTime   *time.Time
	SubtaskScores map[string]float64
}

type ParticipantScore struct {
	UserID       int64
	Username     string
	Problems     []ProblemResult
	TotalSolved  int
	TotalScore   float64
	TotalPenalty int
}

type Rank struct {
	Position int
	Score    ParticipantScore
}

type ContestFormat interface {
	Name() string
	ScoreProblem(ctx ScoringContext) (ProblemResult, error)
	RankParticipants(participants []ParticipantScore) []Rank
	ValidateConfig(config json.RawMessage) error
	DefaultConfig() json.RawMessage
}
```

- [ ] **Step 2: Create registry.go**
Create `internal/contest/format/registry.go` for blank self-registration lookup:

```go
package format

import (
	"encoding/json"
	"fmt"
	"sync"
)

type FormatFactory func(config json.RawMessage) (ContestFormat, error)

var (
	mu        sync.RWMutex
	factories = map[string]FormatFactory{}
)

func Register(name string, factory FormatFactory) {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := factories[name]; ok {
		panic("contest format already registered: " + name)
	}
	factories[name] = factory
}

func Get(name string) (FormatFactory, bool) {
	mu.RLock()
	defer mu.RUnlock()
	f, ok := factories[name]
	return f, ok
}

func MustGet(name string) FormatFactory {
	f, ok := Get(name)
	if !ok {
		panic("contest format not found: " + name)
	}
	return f
}

func List() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(factories))
	for n := range factories {
		names = append(names, n)
	}
	return names
}

func Create(name string, config json.RawMessage) (ContestFormat, error) {
	factory, ok := Get(name)
	if !ok {
		return nil, fmt.Errorf("unknown contest format: %s", name)
	}
	return factory(config)
}

func MustCreate(name string, config json.RawMessage) ContestFormat {
	f, err := Create(name, config)
	if err != nil {
		panic(err)
	}
	return f
}
```

- [ ] **Step 3: Create formats.go**
Create `internal/contest/format/formats.go` to import blank implementations:

```go
package format

import (
	_ "github.com/tahsinarafat/aioj/internal/contest/format/acm"
	_ "github.com/tahsinarafat/aioj/internal/contest/format/atcoder"
	_ "github.com/tahsinarafat/aioj/internal/contest/format/codeforces"
	_ "github.com/tahsinarafat/aioj/internal/contest/format/ioi"
	_ "github.com/tahsinarafat/aioj/internal/contest/format/oi"
)
```

- [ ] **Step 4: Commit**
```bash
git add internal/contest/format/types.go internal/contest/format/registry.go internal/contest/format/formats.go
git commit -m "feat(contest-formats): implement core format interface and factory registry"
```

---

### Task 3: ACM/ICPC Format Implementation

**Files:**
- Create: `internal/contest/format/acm/acm.go`
- Create: `internal/contest/format/acm/acm_test.go`

- [ ] **Step 1: Write acm.go**
Create `internal/contest/format/acm/acm.go`:

```go
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
```

- [ ] **Step 2: Write acm_test.go**
Write full tests to verify scoring and ranking with zero global state.

- [ ] **Step 3: Commit**
```bash
git add internal/contest/format/acm/
git commit -m "feat(contest-formats): implement ACM/ICPC scoring format"
```

---

### Task 4: OI Format Implementation

**Files:**
- Create: `internal/contest/format/oi/oi.go`

- [ ] **Step 1: Write oi.go**
Create `internal/contest/format/oi/oi.go`:

```go
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
	return Config{MaxScorePerProblem: 100}
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
```

- [ ] **Step 2: Commit**
```bash
git add internal/contest/format/oi/
git commit -m "feat(contest-formats): implement OI scoring format"
```

---

### Task 5: IOI Format Implementation

**Files:**
- Create: `internal/contest/format/ioi/ioi.go`

- [ ] **Step 1: Write ioi.go**
Create `internal/contest/format/ioi/ioi.go`:

```go
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
	if ctx.Problem.Config != nil {
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
```

- [ ] **5.2** Commit
```bash
git add internal/contest/format/ioi/
git commit -m "feat(contest-formats): implement IOI subtask scoring format"
```

---

### Task 6: AtCoder Format Implementation

**Files:**
- Create: `internal/contest/format/atcoder/atcoder.go`

- [ ] **6.1** Create atcoder.go

```go
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
```

- [ ] **6.2** Commit
```bash
git add internal/contest/format/atcoder/
git commit -m "feat(contest-formats): implement AtCoder scoring format"
```

---

### Task 7: Codeforces Format Implementation

**Files:**
- Create: `internal/contest/format/codeforces/codeforces.go`

- [ ] **7.1** Create codeforces.go

```go
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
```

- [ ] **7.2** Commit
```bash
git add internal/contest/format/codeforces/
git commit -m "feat(contest-formats): implement Codeforces scoring format"
```

---

### Task 8: Store and Model Updates

**Files:**
- Modify: `internal/model/contest.go`
- Modify: `internal/store/postgres/contests.go`

- [ ] **8.1** Apply SQL migration for columns
`make migrate-up` runs `000026_contest_formats.up.sql`.

- [ ] **8.2** Modify model `Contest` and `CreateContestRequest` as shown in Design Section 4.

- [ ] **8.3** Modify `internal/store/postgres/contests.go`
Update `Create`, `GetByID`, `List`, `UpdateContestFormat`, `GetContestFormat` to read and write the new `format` and `format_config` fields properly.

---

### Task 9: Handler Refactoring

**Files:**
- Modify: `internal/api/handler/contest.go`

- [ ] **9.1** Refactor `Scoreboard` method to delegate to `format.Create()` registry and score using `ScoreProblem` and `RankParticipants`.

---

### Task 10: Frontend Updates

**Files:**
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/pages/ContestCreate.tsx`
- Modify: `web/src/pages/ContestDetail.tsx`
- Modify: `web/src/pages/ContestScoreboard.tsx`

---

**Plan Status**: ✅ Ready for Execution  
**Next Step**: Deactivate Plan Mode → Execute commits 1-15 in ultrawork session

---

Would you like me to start the implementation of these tasks? I will wait for you to deactivate Plan Mode or give the signal to begin writing.
