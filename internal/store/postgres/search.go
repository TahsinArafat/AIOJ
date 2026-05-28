package postgres

import (
	"context"
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/model"
)

type SearchStore struct {
	db *sql.DB
}

func NewSearchStore(db *sql.DB) *SearchStore {
	return &SearchStore{db: db}
}

func (s *SearchStore) SearchProblems(ctx context.Context, query string, limit int) ([]model.ProblemListItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, slug, title, difficulty, tags, submission_count, accepted_count, source
		 FROM problems
		 WHERE visible = true AND (title ILIKE '%' || $1 || '%' OR slug ILIKE '%' || $1 || '%')
		 ORDER BY title
		 LIMIT $2`,
		query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.ProblemListItem
	for rows.Next() {
		var p model.ProblemListItem
		var tags []string
		if err := rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Difficulty, &tags, &p.SubmissionCount, &p.AcceptedCount, &p.Source); err != nil {
			return nil, err
		}
		p.Tags = tags
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

func (s *SearchStore) SearchContests(ctx context.Context, query string, limit int) ([]model.Contest, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title, type, start_time, end_time, visible, description,
		        registration_required, registration_deadline, max_participants, division,
		        created_by, created_at
		 FROM contests
		 WHERE visible = true AND title ILIKE '%' || $1 || '%'
		 ORDER BY title
		 LIMIT $2`,
		query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.Contest
	for rows.Next() {
		var c model.Contest
		if err := rows.Scan(&c.ID, &c.Title, &c.Type, &c.StartTime, &c.EndTime,
			&c.Visible, &c.Description, &c.RegistrationRequired, &c.RegistrationDeadline,
			&c.MaxParticipants, &c.Division, &c.CreatedBy, &c.CreatedAt); err != nil {
			return nil, err
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

func (s *SearchStore) SearchAll(ctx context.Context, query string, limit int) (*SearchResult, error) {
	problems, err := s.SearchProblems(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	users, err := s.SearchUsers(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	contests, err := s.SearchContests(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	return &SearchResult{
		Problems: problems,
		Users:    users,
		Contests: contests,
	}, nil
}