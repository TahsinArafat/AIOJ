package postgres

import (
	"context"
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/model"
)

type BalloonStore struct {
	db *sql.DB
}

func NewBalloonStore(db *sql.DB) *BalloonStore {
	return &BalloonStore{db: db}
}

func (s *BalloonStore) CreateRequest(ctx context.Context, contestID, submissionID, userID, problemID string) error {
	// First check if this user has already solved this problem in this contest to avoid duplicate balloons
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM balloon_requests 
			WHERE contest_id = $1 AND user_id = $2 AND problem_id = $3
		)`, contestID, userID, problemID).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	// Fetch a problem color based on the problem index or ID (e.g. cycle through standard balloon colors)
	var color string
	var problemIdx string
	err = s.db.QueryRowContext(ctx,
		`SELECT cp.index FROM contest_problems cp WHERE cp.contest_id = $1 AND cp.problem_id = $2`,
		contestID, problemID).Scan(&problemIdx)
	if err != nil {
		problemIdx = "A"
	}

	// Standard balloon color mapping by problem index
	colors := []string{"Red", "Blue", "Green", "Yellow", "Purple", "Orange", "Pink", "Gold", "Silver", "White"}
	charIdx := 0
	if len(problemIdx) > 0 {
		charIdx = int(problemIdx[0]-'A') % len(colors)
		if charIdx < 0 {
			charIdx = 0
		}
	}
	color = colors[charIdx]

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO balloon_requests(contest_id, submission_id, user_id, problem_id, color)
		 VALUES($1, $2, $3, $4, $5)`,
		contestID, submissionID, userID, problemID, color,
	)
	return err
}

func (s *BalloonStore) ListByContest(ctx context.Context, contestID string) ([]model.BalloonRequest, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT br.id, br.contest_id, br.submission_id, br.user_id, u.username, 
		        br.problem_id, cp.index, br.color, br.dispatched, br.created_at
		 FROM balloon_requests br
		 JOIN users u ON br.user_id = u.id
		 JOIN contest_problems cp ON br.contest_id = cp.contest_id AND br.problem_id = cp.problem_id
		 WHERE br.contest_id = $1
		 ORDER BY br.created_at DESC`, contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.BalloonRequest
	for rows.Next() {
		var r model.BalloonRequest
		err := rows.Scan(&r.ID, &r.ContestID, &r.SubmissionID, &r.UserID, &r.Username,
			&r.ProblemID, &r.ProblemIndex, &r.Color, &r.Dispatched, &r.CreatedAt)
		if err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	if list == nil {
		list = []model.BalloonRequest{}
	}
	return list, nil
}

func (s *BalloonStore) Dispatch(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE balloon_requests SET dispatched = TRUE WHERE id = $1`, id)
	return err
}
