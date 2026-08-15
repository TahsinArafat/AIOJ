package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/ai"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

// Service orchestrates problem generation: resolves the active AI model,
// calls the endpoint, and persists the resulting problem + editorial as drafts.
type Service struct {
	aiStore   store.AIModelStore
	probStore store.ProblemStore
	editStore store.EditorialStore
}

// NewService creates a Service.
func NewService(aiStore store.AIModelStore, probStore store.ProblemStore, editStore store.EditorialStore) *Service {
	return &Service{aiStore: aiStore, probStore: probStore, editStore: editStore}
}

// generatedContent mirrors the JSON schema the model is instructed to return.
type generatedContent struct {
	Title             string             `json:"title"`
	Description       string             `json:"description"`
	InputFormat       string             `json:"input_format"`
	OutputFormat      string             `json:"output_format"`
	Hint              string             `json:"hint"`
	SampleCases       []model.SampleCase `json:"sample_cases"`
	TimeLimitMs       int                `json:"time_limit_ms"`
	MemoryLimitKB     int                `json:"memory_limit_kb"`
	Tags              []string           `json:"tags"`
	Difficulty        string             `json:"difficulty"`
	SolutionCode      string             `json:"solution_code"`
	SolutionLanguage  string             `json:"solution_language"`
	EditorialTitle    string             `json:"editorial_title"`
	EditorialContent  string             `json:"editorial_content"`
	EditorialApproach string             `json:"editorial_approach"`
	TimeComplexity    string             `json:"time_complexity"`
	SpaceComplexity   string             `json:"space_complexity"`
}

// Generate calls the active AI model, saves the problem and editorial as drafts,
// and returns their IDs.
func (s *Service) Generate(ctx context.Context, userID string, req Request) (*GenerateResult, error) {
	// 1. Resolve the active AI model from the database.
	aiModel, err := s.aiStore.GetDefault(ctx)
	if err != nil {
		return nil, fmt.Errorf("generate: query AI model: %w", err)
	}
	if aiModel == nil {
		return nil, fmt.Errorf("no AI model is enabled — add and enable one in Admin › AI Models")
	}

	// 2. Build a one-shot client for this model.
	client := ai.New(aiModel.Endpoint, aiModel.APIKey, aiModel.ModelName)

	// 3. Build prompts.
	userMsg := buildUserPrompt(req)

	// 4. Call AI.
	raw, err := client.Complete(ctx, systemPrompt, userMsg)
	if err != nil {
		return nil, fmt.Errorf("generate: AI call failed: %w", err)
	}

	// 5. Parse the model's JSON response.
	var content generatedContent
	if err := json.Unmarshal([]byte(raw), &content); err != nil {
		return nil, fmt.Errorf("generate: failed to parse model output as JSON: %w (raw: %s)", err, truncate(raw, 300))
	}

	// 6. Apply sensible defaults for fields the model may omit.
	if content.TimeLimitMs <= 0 {
		content.TimeLimitMs = 2000
	}
	if content.MemoryLimitKB <= 0 {
		content.MemoryLimitKB = 262144
	}
	if content.SolutionLanguage == "" {
		content.SolutionLanguage = "cpp-gpp-64"
	}
	if content.Difficulty == "" {
		content.Difficulty = difficultyFromRating(req.Rating)
	}
	if len(content.Tags) == 0 {
		content.Tags = req.Tags
	}

	// 7. Build the Problem model.
	prob := &model.Problem{
		ID:                 uuid.New().String(),
		Slug:               slugify(content.Title),
		Title:              content.Title,
		Description:        content.Description,
		InputFormat:        content.InputFormat,
		OutputFormat:       content.OutputFormat,
		Hint:               content.Hint,
		SampleCases:        content.SampleCases,
		TimeLimit:          content.TimeLimitMs,
		MemoryLimit:        content.MemoryLimitKB * 1024, // store expects bytes
		Difficulty:         content.Difficulty,
		Tags:               content.Tags,
		Visible:            false, // draft — admin must review and publish
		Source:             "ai",
		AIGenerated:        true,
		CreatedBy:          userID,
		CheckerType:        "lines",
		ScoringMode:        "complete",
		SubtaskAggregation: "min",
	}

	// 8. Save the problem.
	if err := s.probStore.Create(ctx, prob); err != nil {
		return nil, fmt.Errorf("generate: save problem: %w", err)
	}

	// 9. Build the Editorial model.
	editorial := &model.Editorial{
		ProblemID:        prob.ID,
		UserID:           userID,
		Title:            content.EditorialTitle,
		Content:          content.EditorialContent,
		SolutionCode:     content.SolutionCode,
		SolutionLanguage: content.SolutionLanguage,
		Approach:         content.EditorialApproach,
		TimeComplexity:   content.TimeComplexity,
		SpaceComplexity:  content.SpaceComplexity,
		IsOfficial:       true,
	}

	// 10. Save the editorial.
	if err := s.editStore.Create(ctx, editorial); err != nil {
		// Problem is already saved; still return its info and note the editorial failure.
		return &GenerateResult{
			ProblemID:   prob.ID,
			ProblemSlug: prob.Slug,
		}, fmt.Errorf("generate: save editorial: %w", err)
	}

	return &GenerateResult{
		ProblemID:   prob.ID,
		ProblemSlug: prob.Slug,
		EditorialID: editorial.ID,
	}, nil
}

// difficultyFromRating maps a Codeforces rating to a difficulty label.
func difficultyFromRating(rating int) string {
	if rating <= 1400 {
		return "easy"
	}
	if rating <= 1900 {
		return "medium"
	}
	return "hard"
}

// nonAlphanumRe matches any character that is not a letter, digit, or hyphen.
var nonAlphanumRe = regexp.MustCompile(`[^a-z0-9]+`)

// slugify converts a title to a URL-safe slug (e.g. "Two Sum!" → "two-sum").
func slugify(title string) string {
	s := strings.ToLower(title)
	s = nonAlphanumRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 100 {
		s = s[:100]
	}
	if s == "" {
		s = uuid.New().String()[:8]
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
