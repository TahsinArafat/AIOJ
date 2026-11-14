package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/lib/pq"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/recommend"
)

// GetRecommendations returns personalised problem recommendations for a user.
//
// This method only runs the four queries. Deciding what the rows mean — the
// difficulty band, the weak-tag defaults, the daily pick, the hybrid
// composition — lives in internal/recommend, which is a pure function of its
// inputs and is tested as one. The policy was previously inline here, reachable
// only through a database query, so no test could pin it.
func (s *ProblemStore) GetRecommendations(ctx context.Context, userID string, currentRating int) (*model.RecommendationsResponse, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	difficulty := recommend.DifficultyFor(currentRating)

	// 1. Progression: unsolved problems in the user's difficulty band.
	progression, err := s.listUnsolvedByDifficulty(ctx, difficulty, userID, 5)
	if err != nil {
		return nil, fmt.Errorf("progression query: %w", err)
	}

	// 2. Weak tags: the topics the user has failed on without ever solving.
	//
	// The defaults are applied here, before the next query, because that query
	// searches for problems carrying these tags. A user with no failed
	// submissions gets no weak tags from the database, and passing that empty set
	// on would return no weak-tag problems at all — which is what the integration
	// test caught. recommend.DefaultWeakTags is the same function Build calls, so
	// the tags reported and the tags searched for cannot diverge.
	weakTags, err := s.weakTags(ctx, userID, 2)
	if err != nil {
		return nil, fmt.Errorf("weak tags query: %w", err)
	}
	weakTags = recommend.DefaultWeakTags(weakTags)

	// 3. Problems matching those topics, still unsolved.
	weakProblems, err := s.listUnsolvedByTags(ctx, weakTags, userID, 5)
	if err != nil {
		return nil, fmt.Errorf("weak problems query: %w", err)
	}

	// 4. Daily-challenge candidates. A query failure is not fatal here: the
	// daily pick is one section of the response, and the other three still stand.
	daily, err := s.listUnsolvedByDifficulty(ctx, difficulty, userID, 20)
	if err != nil {
		slog.Warn("recommendations: daily candidates", "user_id", userID, "error", err)
		daily = nil
	}

	return recommend.Build(currentRating, progression, weakProblems, weakTags, daily, nil), nil
}

// unsolvedProblemColumns is the projection every recommendation query returns.
// The scan helper below is the only thing that reads it, so the column list and
// the Scan call cannot drift apart — which is what they did before, in four
// separate copies.
const unsolvedProblemColumns = `p.id, p.slug, p.title, p.difficulty, p.tags,
	       p.submission_count, p.accepted_count, p.source`

// scanUnsolved reads one row of unsolvedProblemColumns.
func scanUnsolved(rows *sql.Rows) (model.ProblemListItem, error) {
	var item model.ProblemListItem
	var tags []string
	if err := rows.Scan(
		&item.ID, &item.Slug, &item.Title, &item.Difficulty,
		pq.Array(&tags), &item.SubmissionCount, &item.AcceptedCount, &item.Source,
	); err != nil {
		return item, err
	}
	item.Tags = tags
	return item, nil
}

// listUnsolvedByDifficulty returns visible problems in one difficulty band that
// the user has not solved.
func (s *ProblemStore) listUnsolvedByDifficulty(ctx context.Context, difficulty, userID string, limit int) ([]model.ProblemListItem, error) {
	q := `
		SELECT ` + unsolvedProblemColumns + `
		FROM problems p
		WHERE p.visible = true
		  AND p.difficulty = $1
		  AND p.id NOT IN (
		      SELECT DISTINCT problem_id FROM submissions
		      WHERE user_id = $2 AND status = 'ac'
		  )
		ORDER BY p.accepted_count DESC, p.created_at DESC
		LIMIT $3`

	rows, err := s.db.QueryContext(ctx, q, difficulty, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]model.ProblemListItem, 0)
	for rows.Next() {
		item, err := scanUnsolved(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// listUnsolvedByTags returns visible unsolved problems carrying any of the tags.
func (s *ProblemStore) listUnsolvedByTags(ctx context.Context, tags []string, userID string, limit int) ([]model.ProblemListItem, error) {
	if len(tags) == 0 {
		return []model.ProblemListItem{}, nil
	}
	q := `
		SELECT ` + unsolvedProblemColumns + `
		FROM problems p
		WHERE p.visible = true
		  AND p.tags && $1
		  AND p.id NOT IN (
		      SELECT DISTINCT problem_id FROM submissions
		      WHERE user_id = $2 AND status = 'ac'
		  )
		ORDER BY p.accepted_count DESC
		LIMIT $3`

	rows, err := s.db.QueryContext(ctx, q, pq.Array(tags), userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]model.ProblemListItem, 0)
	for rows.Next() {
		item, err := scanUnsolved(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// weakTags returns the topics the user has most often failed on, among problems
// they have never solved.
//
// "Never solved" is the load-bearing condition: a problem failed on and then
// solved is not a weakness, and counting it would recommend practice in a topic
// the user has already demonstrated.
func (s *ProblemStore) weakTags(ctx context.Context, userID string, limit int) ([]string, error) {
	q := `
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
		LIMIT $2`

	rows, err := s.db.QueryContext(ctx, q, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make([]string, 0, limit)
	for rows.Next() {
		var tag string
		var count int
		if err := rows.Scan(&tag, &count); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}
