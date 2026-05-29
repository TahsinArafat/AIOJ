package postgres

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
)

type SubmissionStore struct{ db *sql.DB }

func NewSubmissionStore(db *sql.DB) *SubmissionStore { return &SubmissionStore{db: db} }

func compressCode(src string) []byte {
	if src == "" {
		return nil
	}
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	w.Write([]byte(src))
	w.Close()
	return buf.Bytes()
}

func decompressCode(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return ""
	}
	defer r.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		return ""
	}
	return string(out)
}

func (s *SubmissionStore) Create(ctx context.Context, sub *model.Submission) error {
	if sub.SubmissionType == "" {
		sub.SubmissionType = "code"
	}
	cid := sql.NullString{String: sub.ContestID, Valid: sub.ContestID != ""}
	compressed := compressCode(sub.SourceCode)
	return s.db.QueryRowContext(ctx,
		`INSERT INTO submissions(id,problem_id,user_id,contest_id,language,source_code,source_code_gz,code_size,status,submission_type)
		 VALUES($1,$2,$3,$4,$5,'',$6,$7,$8,$9) RETURNING created_at`,
		sub.ID, sub.ProblemID, sub.UserID, cid, sub.Language, compressed, sub.CodeSize, sub.Status, sub.SubmissionType,
	).Scan(&sub.CreatedAt)
}

func (s *SubmissionStore) GetByID(ctx context.Context, id string) (*model.Submission, error) {
	var sub model.Submission
	var cid sql.NullString
	var co sql.NullString
	var jr []byte
	var ja sql.NullTime
	var compressed []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT id,problem_id,user_id,COALESCE(contest_id::text,''),language,source_code,source_code_gz,code_size,
		        status,score,time_used,memory_used,compile_output,judge_result,
		        judged_by,created_at,judged_at,submission_type FROM submissions WHERE id=$1`, id).Scan(
		&sub.ID, &sub.ProblemID, &sub.UserID, &cid, &sub.Language, &sub.SourceCode, &compressed, &sub.CodeSize,
		&sub.Status, &sub.Score, &sub.TimeUsed, &sub.MemoryUsed, &co, &jr,
		&sub.JudgedBy, &sub.CreatedAt, &ja, &sub.SubmissionType)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(compressed) > 0 {
		sub.SourceCode = decompressCode(compressed)
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
		`SELECT id,user_id,language,status,score,time_used,memory_used,created_at,submission_type
		 FROM submissions WHERE problem_id=$1 ORDER BY created_at DESC OFFSET $2 LIMIT $3`, pid, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []model.Submission
	for rows.Next() {
		var sub model.Submission
		rows.Scan(&sub.ID, &sub.UserID, &sub.Language, &sub.Status, &sub.Score, &sub.TimeUsed, &sub.MemoryUsed, &sub.CreatedAt, &sub.SubmissionType)
		items = append(items, sub)
	}
	if items == nil {
		items = []model.Submission{}
	}
	return items, total, nil
}

func (s *SubmissionStore) ListByUser(ctx context.Context, uid string, offset, limit int, problemID, contestID string) ([]model.Submission, int, error) {
	where := "user_id=$1"
	args := []interface{}{uid}
	argIdx := 2

	if problemID != "" {
		where += fmt.Sprintf(" AND problem_id=$%d", argIdx)
		args = append(args, problemID)
		argIdx++
	}
	if contestID != "" {
		where += fmt.Sprintf(" AND contest_id=$%d", argIdx)
		args = append(args, contestID)
		argIdx++
	}

	var total int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM submissions WHERE "+where, args...).Scan(&total)

	args = append(args, offset, limit)
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT id,problem_id,language,status,score,time_used,memory_used,created_at,submission_type
		 FROM submissions WHERE %s ORDER BY created_at DESC OFFSET $%d LIMIT $%d`, where, argIdx, argIdx+1), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []model.Submission
	for rows.Next() {
		var sub model.Submission
		rows.Scan(&sub.ID, &sub.ProblemID, &sub.Language, &sub.Status, &sub.Score, &sub.TimeUsed, &sub.MemoryUsed, &sub.CreatedAt, &sub.SubmissionType)
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

func (s *SubmissionStore) UpdateResult(ctx context.Context, id string, status model.SubmissionStatus, score, timeUsed, memoryUsed int, compileOutput string, results []model.TestCaseResult) error {
	jr, _ := json.Marshal(results)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`UPDATE submissions SET status=$1,score=$2,time_used=$3,memory_used=$4,compile_output=$5,judge_result=$6,judged_at=$7 WHERE id=$8`,
		status, score, timeUsed, memoryUsed, compileOutput, jr, time.Now(), id)
	if err != nil {
		return err
	}

	if status == model.StatusAC {
		_, _ = tx.ExecContext(ctx,
			`INSERT INTO training_plan_progress (plan_id, user_id, problem_id, completed, completed_at)
			 SELECT DISTINCT tps.plan_id, sub.user_id, sub.problem_id, true, NOW()
			 FROM submissions sub
			 JOIN training_plan_problems tpp ON sub.problem_id = tpp.problem_id
			 JOIN training_plan_sections tps ON tpp.section_id = tps.id
			 JOIN training_plan_enrollments tpe ON tpe.plan_id = tps.plan_id AND tpe.user_id = sub.user_id
			 WHERE sub.id = $1
			 ON CONFLICT (plan_id, user_id, problem_id)
			 DO UPDATE SET completed = true, completed_at = NOW()
			 WHERE NOT training_plan_progress.completed`,
			id)
	}

	return tx.Commit()
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

func (s *SubmissionStore) GetProblemStats(ctx context.Context, problemID string) (*model.ProblemStats, error) {
	stats := &model.ProblemStats{
		LanguageDistribution: make(map[string]int),
	}

	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM submissions WHERE problem_id = $1", problemID).Scan(&stats.TotalSubmissions)
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM submissions WHERE problem_id = $1 AND status = 'ac'", problemID).Scan(&stats.AcceptedSubmissions)

	if stats.TotalSubmissions > 0 {
		stats.AcceptanceRate = float64(stats.AcceptedSubmissions) / float64(stats.TotalSubmissions) * 100
	}

	s.db.QueryRowContext(ctx, "SELECT COUNT(DISTINCT user_id) FROM submissions WHERE problem_id = $1 AND status = 'ac'", problemID).Scan(&stats.UniqueSolvers)

	rows, err := s.db.QueryContext(ctx, `SELECT language, COUNT(*) FROM submissions WHERE problem_id = $1 GROUP BY language ORDER BY COUNT(*) DESC`, problemID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var lang string
			var count int
			rows.Scan(&lang, &count)
			stats.LanguageDistribution[lang] = count
		}
	}

	s.db.QueryRowContext(ctx, `SELECT COALESCE(AVG(attempts), 0) FROM (
		SELECT user_id, COUNT(*) as attempts FROM submissions WHERE problem_id = $1 GROUP BY user_id
		HAVING COUNT(CASE WHEN status = 'ac' THEN 1 END) > 0
	) t`, problemID).Scan(&stats.AverageAttempts)

	return stats, nil
}

func (s *SubmissionStore) GetUserStats(ctx context.Context, userID string) (*model.UserProblemStats, error) {
	stats := &model.UserProblemStats{}

	s.db.QueryRowContext(ctx, "SELECT problems_solved FROM user_profiles WHERE user_id = $1", userID).Scan(&stats.ProblemsSolved)
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM submissions WHERE user_id = $1", userID).Scan(&stats.TotalSubmissions)

	var accepted int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM submissions WHERE user_id = $1 AND status = 'ac'", userID).Scan(&accepted)
	if stats.TotalSubmissions > 0 {
		stats.AcceptanceRate = float64(accepted) / float64(stats.TotalSubmissions) * 100
	}

	s.db.QueryRowContext(ctx, `SELECT language FROM submissions WHERE user_id = $1 GROUP BY language ORDER BY COUNT(*) DESC LIMIT 1`, userID).Scan(&stats.FavoriteLanguage)

	return stats, nil
}

func (s *SubmissionStore) GetPlatformStats(ctx context.Context) (*model.PlatformStats, error) {
	var stats model.PlatformStats
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM problems WHERE visible=true").Scan(&stats.Problems)
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&stats.Users)
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM submissions").Scan(&stats.Submissions)
	return &stats, nil
}
