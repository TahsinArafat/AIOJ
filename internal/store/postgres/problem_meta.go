package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"
)

// ProblemMetaStore reads the few columns needed to render a link preview.
//
// Kept apart from ProblemStore on purpose: those queries load the statement,
// sample cases and testcase scores, which a share card never shows. The crawler
// path is hit by every social-network unfurl, so it should stay a single
// narrow lookup by the indexed slug.
type ProblemMetaStore struct {
	db *sql.DB
}

func NewProblemMetaStore(db *sql.DB) *ProblemMetaStore {
	return &ProblemMetaStore{db: db}
}

// ProblemMeta is the share-card subset of one problem row.
type ProblemMeta struct {
	Slug        string
	Title       string
	Description string
	Difficulty  string
	Tags        []string
	TimeLimit   int // milliseconds, as stored
	MemoryLimit int // megabytes, as stored
	Visible     bool
}

// GetBySlug returns the problem, or nil when no row has that slug.
//
// Visibility is returned rather than filtered so the caller decides: a private
// problem must answer "not found" to a crawler without becoming one.
func (s *ProblemMetaStore) GetBySlug(ctx context.Context, slug string) (*ProblemMeta, error) {
	var m ProblemMeta
	var tags []string
	err := s.db.QueryRowContext(ctx, `
		SELECT slug, title, COALESCE(description, ''), COALESCE(difficulty, ''),
		       COALESCE(tags, '{}'), COALESCE(time_limit, 0), COALESCE(memory_limit, 0),
		       COALESCE(visible, false)
		  FROM problems
		 WHERE slug = $1`, slug).Scan(
		&m.Slug, &m.Title, &m.Description, &m.Difficulty,
		pq.Array(&tags), &m.TimeLimit, &m.MemoryLimit, &m.Visible,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.Tags = tags
	return &m, nil
}
