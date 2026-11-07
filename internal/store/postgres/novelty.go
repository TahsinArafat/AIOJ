package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/tahsinarafat/aioj/internal/harness/problemgen"
)

// NoveltyCorpus answers "has this problem been set before?" against the local
// problem archive, so the generator can refuse to publish a near-duplicate.
//
// It uses the indexes migration 57 already built: the tsvector for topical
// overlap and pg_trgm for title similarity. Both are cheap and need no new
// infrastructure — in particular no embeddings, which would mean a vector
// extension, a backfill and a second model to keep in sync.
type NoveltyCorpus struct{ db *sql.DB }

// NewNoveltyCorpus builds the corpus.
func NewNoveltyCorpus(db *sql.DB) *NoveltyCorpus { return &NoveltyCorpus{db: db} }

// Similar returns the closest existing problems.
//
// Similarity is the greater of title trigram similarity and full-text rank
// normalised to 0..1. Including the statement catches a reworded duplicate with
// a new title, which a title-only check would miss.
func (c *NoveltyCorpus) Similar(ctx context.Context, title, description string, limit int) ([]problemgen.SimilarProblem, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	// A blank description is normal mid-generation; fall back to the title so
	// the full-text arm still contributes something.
	if description == "" {
		description = title
	}

	const q = `
		WITH scored AS (
			SELECT p.slug,
			       p.title,
			       GREATEST(
			           similarity(p.title, $1),
			           LEAST(1.0, ts_rank(p.search_vector, plainto_tsquery('simple', $2)))
			       ) AS sim
			FROM problems p
			WHERE p.visible = true
			  AND (p.search_vector @@ plainto_tsquery('simple', $2) OR p.title % $1)
		)
		SELECT slug, title, sim
		FROM scored
		WHERE sim > 0
		ORDER BY sim DESC, slug
		LIMIT $3`

	rows, err := c.db.QueryContext(ctx, q, title, description, limit)
	if err != nil {
		return nil, fmt.Errorf("novelty: query: %w", err)
	}
	defer rows.Close()

	out := make([]problemgen.SimilarProblem, 0, limit)
	for rows.Next() {
		var s problemgen.SimilarProblem
		if err := rows.Scan(&s.Slug, &s.Title, &s.Similarity); err != nil {
			return nil, fmt.Errorf("novelty: scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
