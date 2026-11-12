package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
	"github.com/tahsinarafat/aioj/internal/model"
)

type SearchStore struct {
	db *sql.DB
}

func NewSearchStore(db *sql.DB) *SearchStore {
	return &SearchStore{db: db}
}

func (s *SearchStore) SearchProblems(ctx context.Context, query string, limit int) ([]model.ProblemListItem, error) {
	// Search is a public archive read, so it uses the same archive condition as the
	// problem list: a problem still in play in a contest is not findable here.
	searchQuery := fmt.Sprintf(`SELECT p.id, p.slug, p.title, p.difficulty, p.tags,
		p.submission_count, p.accepted_count, p.source
		FROM problems p
		WHERE %s AND (p.title ILIKE '%%' || $1 || '%%' OR p.slug ILIKE '%%' || $1 || '%%')
		ORDER BY p.title
		LIMIT $2`, archivedProblemSQL)
	rows, err := s.db.QueryContext(ctx, searchQuery, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.ProblemListItem
	for rows.Next() {
		var p model.ProblemListItem
		if err := rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Difficulty, pq.Array(&p.Tags),
			&p.SubmissionCount, &p.AcceptedCount, &p.Source); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	if items == nil {
		items = []model.ProblemListItem{}
	}
	return items, nil
}

func (s *SearchStore) SearchUsers(ctx context.Context, query string, limit int) ([]model.User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, username, email, role, is_bot, created_at
		 FROM users
		 WHERE username ILIKE '%' || $1 || '%'
		 ORDER BY username
		 LIMIT $2`,
		query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.IsBot, &u.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, u)
	}
	if items == nil {
		items = []model.User{}
	}
	return items, nil
}

// SearchContests is the contest half of global search, as one caller sees it.
//
// It used to list every visible round by title, so searching any word surfaced the
// title of invite-only rounds — enumerable by anybody, without a grant.
func (s *SearchStore) SearchContests(ctx context.Context, query string, limit int, viewerID string) ([]model.Contest, error) {
	visibility := fmt.Sprintf(contestVisibilityPredicate, 3, 3, 3, 3, 3, 3)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, display_id, slug, title, type, start_time, end_time, visible, description,
		        registration_required, registration_deadline, max_participants, division,
		        created_by, created_at
		 FROM contests
		 WHERE visible = true AND title ILIKE '%' || $1 || '%' AND `+visibility+`
		 ORDER BY title
		 LIMIT $2`,
		query, limit, viewerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.Contest
	for rows.Next() {
		var c model.Contest
		var slug sql.NullString
		if err := rows.Scan(&c.ID, &c.DisplayID, &slug, &c.Title, &c.Type, &c.StartTime, &c.EndTime,
			&c.Visible, &c.Description, &c.RegistrationRequired, &c.RegistrationDeadline,
			&c.MaxParticipants, &c.Division, &c.CreatedBy, &c.CreatedAt); err != nil {
			return nil, err
		}
		if slug.Valid {
			c.Slug = slug.String
		}
		items = append(items, c)
	}
	if items == nil {
		items = []model.Contest{}
	}
	return items, nil
}

type SearchResult struct {
	Problems []model.ProblemListItem `json:"problems"`
	Users    []model.User            `json:"users"`
	Contests []model.Contest         `json:"contests"`
}

// SearchAll runs every search kind for one caller. viewerID is "" for anonymous,
// and it is threaded through to the contest half because an invite-only round's
// title must not be discoverable by someone who has not unlocked it.
func (s *SearchStore) SearchAll(ctx context.Context, query string, limit int, viewerID string) (*SearchResult, error) {
	problems, err := s.SearchProblems(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	users, err := s.SearchUsers(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	contests, err := s.SearchContests(ctx, query, limit, viewerID)
	if err != nil {
		return nil, err
	}

	return &SearchResult{
		Problems: problems,
		Users:    users,
		Contests: contests,
	}, nil
}
