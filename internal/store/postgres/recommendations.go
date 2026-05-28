package postgres

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/lib/pq"
	"github.com/tahsinarafat/aioj/internal/model"
)

// ratingToDifficulty maps a user's contest rating to a problem difficulty band.
func ratingToDifficulty(rating int) string {
	if rating < 1400 {
		return "easy"
	}
	if rating < 1900 {
		return "medium"
	}
	return "hard"
}

// applyWeakTagDefaults fills in default weak tags when the query returns fewer than 2.
func applyWeakTagDefaults(tags []string) []string {
	switch len(tags) {
	case 0:
		return []string{"dp", "graphs"}
	case 1:
		return append(tags, "greedy")
	default:
		return tags
	}
}

// buildHybridList combines up to 2 progression, up to 2 weak-tag, and 1 daily challenge
// problem into a single deduplicated list.
func buildHybridList(progression, weak []model.ProblemListItem, daily *model.ProblemListItem) []model.ProblemListItem {
	seen := make(map[string]struct{})
	var hybrid []model.ProblemListItem

	addUnique := func(item model.ProblemListItem) {
		if _, ok := seen[item.ID]; ok {
			return
		}
		seen[item.ID] = struct{}{}
		hybrid = append(hybrid, item)
	}

	for i := 0; i < len(progression) && i < 2; i++ {
		addUnique(progression[i])
	}
	for i := 0; i < len(weak) && i < 2; i++ {
		addUnique(weak[i])
	}
	if daily != nil {
		addUnique(*daily)
	}

	if hybrid == nil {
		return []model.ProblemListItem{}
	}
	return hybrid
}

// GetRecommendations returns personalised problem recommendations for a user
// based on their contest rating, submission history, and weak-topic analysis.
func (s *ProblemStore) GetRecommendations(ctx context.Context, userID string, currentRating int) (*model.RecommendationsResponse, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	difficulty := ratingToDifficulty(currentRating)

	// --- 1. Progression: visible unsolved problems in the user's difficulty band ---
	progQuery := `
		SELECT p.id, p.slug, p.title, p.difficulty, p.tags,
		       p.submission_count, p.accepted_count, p.source
		FROM problems p
		WHERE p.visible = true
		  AND p.difficulty = $1
		  AND p.id NOT IN (
		      SELECT DISTINCT problem_id FROM submissions
		      WHERE user_id = $2 AND status = 'ac'
		  )
		ORDER BY p.accepted_count DESC, p.created_at DESC
		LIMIT 5`

	progRows, err := s.db.QueryContext(ctx, progQuery, difficulty, userID)
	if err != nil {
		return nil, fmt.Errorf("progression query: %w", err)
	}
	defer progRows.Close()

	progression := make([]model.ProblemListItem, 0)
	for progRows.Next() {
		var item model.ProblemListItem
		var tags []string
		if err := progRows.Scan(
			&item.ID, &item.Slug, &item.Title, &item.Difficulty,
			pq.Array(&tags), &item.SubmissionCount, &item.AcceptedCount, &item.Source,
		); err != nil {
			return nil, fmt.Errorf("progression scan: %w", err)
		}
		item.Tags = tags
		progression = append(progression, item)
	}

	// --- 2. Weak-tag identification ---
	// Find top 2 tags where the user has the most incorrect submissions
	// on problems they have NEVER solved (no subsequent AC).
	weakTagsQuery := `
		SELECT tag, COUNT(*) AS fail_count
		FROM (
			SELECT DISTINCT s.problem_id, t.tag
			FROM submissions s
			JOIN problems p ON s.problem_id = p.id
			CROSS JOIN LATERAL unnest(p.tags) AS t(tag)
			WHERE s.user_id = $1
			  AND s.status != 'ac'
			  AND s.problem_id NOT IN (
			      SELECT DISTINCT problem_id FROM submissions
			      WHERE user_id = $1 AND status = 'ac'
			  )
		) sub
		GROUP BY tag
		ORDER BY fail_count DESC
		LIMIT 2`

	tagRows, err := s.db.QueryContext(ctx, weakTagsQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("weak tags query: %w", err)
	}
	defer tagRows.Close()

	var weakTags []string
	for tagRows.Next() {
		var tag string
		var count int
		if err := tagRows.Scan(&tag, &count); err != nil {
			return nil, fmt.Errorf("weak tags scan: %w", err)
		}
		weakTags = append(weakTags, tag)
	}
	weakTags = applyWeakTagDefaults(weakTags)

	// --- 3. Weak-tag problems: visible, unsolved problems matching weak tags ---
	weakProbQuery := `
		SELECT p.id, p.slug, p.title, p.difficulty, p.tags,
		       p.submission_count, p.accepted_count, p.source
		FROM problems p
		WHERE p.visible = true
		  AND p.tags && $1
		  AND p.id NOT IN (
		      SELECT DISTINCT problem_id FROM submissions
		      WHERE user_id = $2 AND status = 'ac'
		  )
		ORDER BY p.accepted_count DESC
		LIMIT 5`

	weakRows, err := s.db.QueryContext(ctx, weakProbQuery, pq.Array(weakTags), userID)
	if err != nil {
		return nil, fmt.Errorf("weak problems query: %w", err)
	}
	defer weakRows.Close()

	weakProblems := make([]model.ProblemListItem, 0)
	for weakRows.Next() {
		var item model.ProblemListItem
		var tags []string
		if err := weakRows.Scan(
			&item.ID, &item.Slug, &item.Title, &item.Difficulty,
			pq.Array(&tags), &item.SubmissionCount, &item.AcceptedCount, &item.Source,
		); err != nil {
			return nil, fmt.Errorf("weak problems scan: %w", err)
		}
		item.Tags = tags
		weakProblems = append(weakProblems, item)
	}

	// --- 4. Daily challenge: one random visible unsolved problem in difficulty band ---
	var daily *model.ProblemListItem
	dailyQuery := `
		SELECT p.id, p.slug, p.title, p.difficulty, p.tags,
		       p.submission_count, p.accepted_count, p.source
		FROM problems p
		WHERE p.visible = true
		  AND p.difficulty = $1
		  AND p.id NOT IN (
		      SELECT DISTINCT problem_id FROM submissions
		      WHERE user_id = $2 AND status = 'ac'
		  )
		LIMIT 20`

	dailyRows, err := s.db.QueryContext(ctx, dailyQuery, difficulty, userID)
	if err == nil {
		defer dailyRows.Close()
		var candidates []model.ProblemListItem
		for dailyRows.Next() {
			var item model.ProblemListItem
			var tags []string
			if err := dailyRows.Scan(
				&item.ID, &item.Slug, &item.Title, &item.Difficulty,
				pq.Array(&tags), &item.SubmissionCount, &item.AcceptedCount, &item.Source,
			); err == nil {
				item.Tags = tags
				candidates = append(candidates, item)
			}
		}
		if len(candidates) > 0 {
			selected := candidates[rand.Intn(len(candidates))]
			daily = &selected
		}
	}

	// --- 5. Build hybrid list (deduplicated) ---
	hybrid := buildHybridList(progression, weakProblems, daily)

	return &model.RecommendationsResponse{
		Progression: progression,
		WeakTags: model.WeakTagsRecommendations{
			Tags:     weakTags,
			Problems: weakProblems,
		},
		Hybrid: hybrid,
	}, nil
}
