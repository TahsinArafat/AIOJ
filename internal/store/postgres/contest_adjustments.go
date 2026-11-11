package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

type ContestAdjustmentStoreImpl struct {
	db *sql.DB
}

func NewContestAdjustmentStore(db *sql.DB) *ContestAdjustmentStoreImpl {
	return &ContestAdjustmentStoreImpl{db: db}
}

func (s *ContestAdjustmentStoreImpl) Create(ctx context.Context, a *model.ContestAdjustment) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO contest_adjustments
		   (id, contest_id, user_id, scope, problem_id, submission_id,
		    point_delta, solved_delta, penalty_delta, reason, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		 RETURNING created_at`,
		a.ID, a.ContestID, a.UserID, a.Scope, a.ProblemID, a.SubmissionID,
		a.PointDelta, a.SolvedDelta, a.PenaltyDelta, a.Reason, a.CreatedBy,
	).Scan(&a.CreatedAt)
	if err != nil {
		return fmt.Errorf("create contest adjustment: %w", err)
	}
	return nil
}

// ListByContest returns the correction history for a round, newest first.
//
// The joins name the person and, where there is one, the onsite team they
// compete as — a judge reading this months later is looking for "why did Team
// Rocket lose 100 points", not for a UUID. The problem letter comes from
// contest_problems because that is the only place a round's index exists.
func (s *ContestAdjustmentStoreImpl) ListByContest(ctx context.Context, contestID string) ([]model.ContestAdjustment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT a.id, a.contest_id, a.user_id, COALESCE(u.username,''),
		        COALESCE(ob.team_name, ''),
		        a.scope, a.problem_id, cp.index, a.submission_id,
		        a.point_delta, a.solved_delta, a.penalty_delta, a.reason,
		        a.created_by, a.created_at
		   FROM contest_adjustments a
		   JOIN users u ON u.id = a.user_id
		   LEFT JOIN onsite_batch_users ob
		          ON ob.used_by = a.user_id AND ob.contest_id = a.contest_id
		   LEFT JOIN contest_problems cp
		          ON cp.contest_id = a.contest_id AND cp.problem_id = a.problem_id
		  WHERE a.contest_id = $1
		  ORDER BY a.created_at DESC`, contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.ContestAdjustment
	for rows.Next() {
		var a model.ContestAdjustment
		if err := rows.Scan(&a.ID, &a.ContestID, &a.UserID, &a.Username, &a.TeamName,
			&a.Scope, &a.ProblemID, &a.ProblemIndex, &a.SubmissionID,
			&a.PointDelta, &a.SolvedDelta, &a.PenaltyDelta, &a.Reason,
			&a.CreatedBy, &a.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	if items == nil {
		items = []model.ContestAdjustment{}
	}
	return items, rows.Err()
}

// Delete is scoped to the contest the handler authorized against, so a manager
// of round A cannot remove a correction from round B by guessing its id.
func (s *ContestAdjustmentStoreImpl) Delete(ctx context.Context, contestID, id string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM contest_adjustments WHERE id = $1 AND contest_id = $2`, id, contestID)
	return err
}

// TotalsByUser sums each participant's corrections in one pass.
//
// The scoreboard applies these after the format has scored the submissions, so
// the computer's answer stays reproducible and the human correction is a
// separate, visible layer on top of it.
func (s *ContestAdjustmentStoreImpl) TotalsByUser(ctx context.Context, contestID string) (map[string]model.AdjustmentTotals, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT user_id, SUM(point_delta), SUM(solved_delta), SUM(penalty_delta)
		   FROM contest_adjustments
		  WHERE contest_id = $1
		  GROUP BY user_id`, contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]model.AdjustmentTotals)
	for rows.Next() {
		var userID string
		var t model.AdjustmentTotals
		if err := rows.Scan(&userID, &t.PointDelta, &t.SolvedDelta, &t.PenaltyDelta); err != nil {
			return nil, err
		}
		out[userID] = t
	}
	return out, rows.Err()
}
