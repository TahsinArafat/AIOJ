package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

type GymStore struct {
	db *sql.DB
}

func NewGymStore(db *sql.DB) *GymStore {
	return &GymStore{db: db}
}

func (s *GymStore) Create(ctx context.Context, g *model.GymContest) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO gym_contests (contest_id, difficulty_rating, category, country, season, description, is_public, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id, created_at`,
		g.ContestID, g.DifficultyRating, g.Category, g.Country, g.Season, g.Description, g.IsPublic, g.CreatedBy,
	).Scan(&g.ID, &g.CreatedAt)
}

func (s *GymStore) GetByID(ctx context.Context, id string) (*model.GymContest, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, nil
	}
	var g model.GymContest
	var slug sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT g.id, g.contest_id, c.title, c.slug, c.display_id, g.difficulty_rating, g.category, g.country, g.season,
		        g.description, g.is_public, g.solve_count, g.created_by, u.username, g.created_at
		 FROM gym_contests g
		 JOIN contests c ON g.contest_id = c.id
		 JOIN users u ON g.created_by = u.id
		 WHERE g.id = $1`,
		id).Scan(&g.ID, &g.ContestID, &g.ContestTitle, &slug, &g.ContestDisplayID, &g.DifficultyRating, &g.Category,
		&g.Country, &g.Season, &g.Description, &g.IsPublic, &g.SolveCount,
		&g.CreatedBy, &g.CreatorName, &g.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if slug.Valid {
		g.ContestSlug = slug.String
	}
	return &g, nil
}

func (s *GymStore) List(ctx context.Context, offset, limit int, filter model.GymFilter) ([]model.GymContest, int, error) {
	where := []string{"g.is_public = true"}
	args := []interface{}{}
	argIdx := 1

	if filter.Category != "" {
		where = append(where, fmt.Sprintf("g.category = $%d", argIdx))
		args = append(args, filter.Category)
		argIdx++
	}

	if filter.MinRating != nil {
		where = append(where, fmt.Sprintf("g.difficulty_rating >= $%d", argIdx))
		args = append(args, *filter.MinRating)
		argIdx++
	}

	if filter.MaxRating != nil {
		where = append(where, fmt.Sprintf("g.difficulty_rating <= $%d", argIdx))
		args = append(args, *filter.MaxRating)
		argIdx++
	}

	if filter.Country != "" {
		where = append(where, fmt.Sprintf("g.country ILIKE $%d", argIdx))
		args = append(args, "%"+filter.Country+"%")
		argIdx++
	}

	if filter.Search != "" {
		where = append(where, fmt.Sprintf("(c.title ILIKE $%d OR g.description ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+filter.Search+"%")
		argIdx++
	}

	whereClause := strings.Join(where, " AND ")

	var total int
	s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM gym_contests g JOIN contests c ON g.contest_id = c.id WHERE "+whereClause,
		args...).Scan(&total)

	query := fmt.Sprintf(`SELECT g.id, g.contest_id, c.title, c.slug, c.display_id, g.difficulty_rating, g.category, g.country,
	                              g.season, g.description, g.is_public, g.solve_count, g.created_by, u.username, g.created_at
	                      FROM gym_contests g
	                      JOIN contests c ON g.contest_id = c.id
	                      JOIN users u ON g.created_by = u.id
	                      WHERE %s
	                      ORDER BY g.created_at DESC OFFSET $%d LIMIT $%d`,
		whereClause, argIdx, argIdx+1)
	args = append(args, offset, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.GymContest
	for rows.Next() {
		var g model.GymContest
		var slug sql.NullString
		if err := rows.Scan(&g.ID, &g.ContestID, &g.ContestTitle, &slug, &g.ContestDisplayID, &g.DifficultyRating, &g.Category,
			&g.Country, &g.Season, &g.Description, &g.IsPublic, &g.SolveCount,
			&g.CreatedBy, &g.CreatorName, &g.CreatedAt); err != nil {
			return nil, 0, err
		}
		if slug.Valid {
			g.ContestSlug = slug.String
		}
		items = append(items, g)
	}
	if items == nil {
		items = []model.GymContest{}
	}
	return items, total, nil
}

func (s *GymStore) MarkSolved(ctx context.Context, gymID, userID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO gym_solves (gym_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		gymID, userID)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "UPDATE gym_contests SET solve_count = solve_count + 1 WHERE id = $1", gymID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *GymStore) IsSolved(ctx context.Context, gymID, userID string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM gym_solves WHERE gym_id = $1 AND user_id = $2)",
		gymID, userID).Scan(&exists)
	return exists, err
}
