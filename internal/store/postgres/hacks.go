package postgres

import (
	"context"
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/model"
)

type HackStore struct {
	db *sql.DB
}

func NewHackStore(db *sql.DB) *HackStore {
	return &HackStore{db: db}
}

func (s *HackStore) Create(ctx context.Context, h *model.Hack) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO hacks (contest_id, problem_id, hacker_id, defender_id, submission_id, test_input)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`,
		h.ContestID, h.ProblemID, h.HackerID, h.DefenderID, h.SubmissionID, h.TestInput,
	).Scan(&h.ID, &h.CreatedAt)
}

func (s *HackStore) GetByID(ctx context.Context, id string) (*model.Hack, error) {
	var h model.Hack
	err := s.db.QueryRowContext(ctx,
		`SELECT id, contest_id, problem_id, hacker_id, defender_id, submission_id,
		        test_input, expected_output, actual_output, status, success,
		        hacker_rating_change, defender_rating_change, created_at, judged_at
		 FROM hacks WHERE id = $1`,
		id).Scan(&h.ID, &h.ContestID, &h.ProblemID, &h.HackerID, &h.DefenderID,
		&h.SubmissionID, &h.TestInput, &h.ExpectedOutput, &h.ActualOutput,
		&h.Status, &h.Success, &h.HackerRatingChange, &h.DefenderRatingChange,
		&h.CreatedAt, &h.JudgedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func (s *HackStore) UpdateStatus(ctx context.Context, id, status string, success bool) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE hacks SET status = $1, success = $2, judged_at = NOW() WHERE id = $3`,
		status, success, id)
	return err
}

func (s *HackStore) GetByContest(ctx context.Context, contestID string) ([]model.Hack, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT h.id, h.contest_id, h.problem_id, h.hacker_id, hu.username, h.defender_id, du.username,
		        h.submission_id, h.test_input, h.status, h.success, h.created_at
		 FROM hacks h
		 JOIN users hu ON h.hacker_id = hu.id
		 JOIN users du ON h.defender_id = du.id
		 WHERE h.contest_id = $1
		 ORDER BY h.created_at DESC`,
		contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hacks []model.Hack
	for rows.Next() {
		var h model.Hack
		if err := rows.Scan(&h.ID, &h.ContestID, &h.ProblemID, &h.HackerID, &h.HackerUsername,
			&h.DefenderID, &h.DefenderUsername, &h.SubmissionID, &h.TestInput,
			&h.Status, &h.Success, &h.CreatedAt); err != nil {
			return nil, err
		}
		hacks = append(hacks, h)
	}
	if hacks == nil {
		hacks = []model.Hack{}
	}
	return hacks, nil
}

func (s *HackStore) GetHackableSubmissions(ctx context.Context, contestID, problemID string) ([]model.Submission, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT s.id, s.problem_id, s.user_id, s.language, s.status, s.created_at
		 FROM submissions s
		 WHERE s.contest_id = $1 AND s.problem_id = $2 AND s.status = 'ac'
		 ORDER BY s.created_at DESC`,
		contestID, problemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var submissions []model.Submission
	for rows.Next() {
		var s model.Submission
		if err := rows.Scan(&s.ID, &s.ProblemID, &s.UserID, &s.Language, &s.Status, &s.CreatedAt); err != nil {
			return nil, err
		}
		submissions = append(submissions, s)
	}
	if submissions == nil {
		submissions = []model.Submission{}
	}
	return submissions, nil
}
