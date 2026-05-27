package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
)

type SubmissionStore struct{ db *sql.DB }

func NewSubmissionStore(db *sql.DB) *SubmissionStore { return &SubmissionStore{db: db} }

func (s *SubmissionStore) Create(ctx context.Context, sub *model.Submission) error {
	cid := sql.NullString{String: sub.ContestID, Valid: sub.ContestID != ""}
	return s.db.QueryRowContext(ctx,
		`INSERT INTO submissions(id,problem_id,user_id,contest_id,language,source_code,code_size,status)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING created_at`,
		sub.ID, sub.ProblemID, sub.UserID, cid, sub.Language, sub.SourceCode, sub.CodeSize, sub.Status,
	).Scan(&sub.CreatedAt)
}

func (s *SubmissionStore) GetByID(ctx context.Context, id string) (*model.Submission, error) {
	var sub model.Submission
	var cid sql.NullString
	var co sql.NullString
	var jr []byte
	var ja sql.NullTime
	err := s.db.QueryRowContext(ctx,
		`SELECT id,problem_id,user_id,COALESCE(contest_id::text,''),language,source_code,code_size,
		        status,score,time_used,memory_used,compile_output,judge_result,
		        judged_by,created_at,judged_at FROM submissions WHERE id=$1`, id).Scan(
		&sub.ID, &sub.ProblemID, &sub.UserID, &cid, &sub.Language, &sub.SourceCode, &sub.CodeSize,
		&sub.Status, &sub.Score, &sub.TimeUsed, &sub.MemoryUsed, &co, &jr,
		&sub.JudgedBy, &sub.CreatedAt, &ja)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if cid.Valid {
		sub.ContestID = cid.String
	}
	sub.CompileOutput = co.String
	if ja.Valid {
		sub.JudgedAt = &ja.Time
	}
	if jr != nil {
		json.Unmarshal(jr, &sub.JudgeResult)
	}
	return &sub, nil
}

func (s *SubmissionStore) ListByProblem(ctx context.Context, pid string, offset, limit int) ([]model.Submission, int, error) {
	var total int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM submissions WHERE problem_id=$1", pid).Scan(&total)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,user_id,language,status,score,time_used,memory_used,created_at
		 FROM submissions WHERE problem_id=$1 ORDER BY created_at DESC OFFSET $2 LIMIT $3`, pid, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []model.Submission
	for rows.Next() {
		var sub model.Submission
		rows.Scan(&sub.ID, &sub.UserID, &sub.Language, &sub.Status, &sub.Score, &sub.TimeUsed, &sub.MemoryUsed, &sub.CreatedAt)
		items = append(items, sub)
	}
	if items == nil {
		items = []model.Submission{}
	}
	return items, total, nil
}

func (s *SubmissionStore) ListByUser(ctx context.Context, uid string, offset, limit int) ([]model.Submission, int, error) {
	var total int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM submissions WHERE user_id=$1", uid).Scan(&total)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,problem_id,language,status,score,time_used,memory_used,created_at
		 FROM submissions WHERE user_id=$1 ORDER BY created_at DESC OFFSET $2 LIMIT $3`, uid, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []model.Submission
	for rows.Next() {
		var sub model.Submission
		rows.Scan(&sub.ID, &sub.ProblemID, &sub.Language, &sub.Status, &sub.Score, &sub.TimeUsed, &sub.MemoryUsed, &sub.CreatedAt)
		items = append(items, sub)
	}
	if items == nil {
		items = []model.Submission{}
	}
	return items, total, nil
}

func (s *SubmissionStore) UpdateStatus(_ context.Context, id string, status model.SubmissionStatus) {
	s.db.Exec("UPDATE submissions SET status=$1 WHERE id=$2", status, id)
}

func (s *SubmissionStore) UpdateResult(_ context.Context, id string, status model.SubmissionStatus, score, timeUsed, memoryUsed int, compileOutput string, results []model.TestCaseResult) error {
	jr, _ := json.Marshal(results)
	_, err := s.db.Exec(
		`UPDATE submissions SET status=$1,score=$2,time_used=$3,memory_used=$4,compile_output=$5,judge_result=$6,judged_at=$7 WHERE id=$8`,
		status, score, timeUsed, memoryUsed, compileOutput, jr, time.Now(), id)
	return err
}

func (s *SubmissionStore) ListPending(_ context.Context, limit int) ([]string, error) {
	rows, err := s.db.Query(`SELECT id FROM submissions WHERE status='pending' ORDER BY created_at ASC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}
	return ids, nil
}
