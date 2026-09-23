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
	var remoteID sql.NullString
	var remoteURL sql.NullString
	var botID sql.NullString
	var botSlug sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id,problem_id,user_id,COALESCE(contest_id::text,''),language,source_code,source_code_gz,code_size,
		        status,score,time_used,memory_used,compile_output,judge_result,
		        judged_by,created_at,judged_at,submission_type,remote_id,remote_url,
		        COALESCE(bot_id,''),COALESCE(bot_slug,'') FROM submissions WHERE id=$1`, id).Scan(
		&sub.ID, &sub.ProblemID, &sub.UserID, &cid, &sub.Language, &sub.SourceCode, &compressed, &sub.CodeSize,
		&sub.Status, &sub.Score, &sub.TimeUsed, &sub.MemoryUsed, &co, &jr,
		&sub.JudgedBy, &sub.CreatedAt, &ja, &sub.SubmissionType, &remoteID, &remoteURL, &botID, &botSlug)
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
	sub.RemoteID = remoteID.String
	sub.RemoteURL = remoteURL.String
	sub.BotID = botID.String
	sub.BotSlug = botSlug.String
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

// ListPublicByUser returns submissions for any user without authentication.
func (s *SubmissionStore) ListPublicByUser(ctx context.Context, userID string, offset, limit int) ([]model.Submission, int, error) {
	var total int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM submissions WHERE user_id=$1", userID).Scan(&total)

	rows, err := s.db.QueryContext(ctx,
		`SELECT s.id, s.problem_id, s.language, s.status, s.score, s.time_used, s.memory_used, s.created_at, s.submission_type
		 FROM submissions s
		 WHERE s.user_id = $1
		 ORDER BY s.created_at DESC OFFSET $2 LIMIT $3`,
		userID, offset, limit)
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
	if status == model.StatusJudging {
		// Legacy/manual callers do not own a claim token. Keep the timestamp so
		// RequeueStale can eventually recover the row, but local judge workers
		// use ClaimPending below so their results can be fenced.
		s.db.Exec("UPDATE submissions SET status=$1, judging_started_at=NOW(), judging_claim_token=NULL WHERE id=$2", status, id)
		return
	}
	s.db.Exec("UPDATE submissions SET status=$1, judging_started_at=NULL, judging_claim_token=NULL WHERE id=$2", status, id)
}

// ClaimPending atomically moves one pending row to judging and records the
// unique owner token. Multiple workers may dequeue the same id after a retry;
// only the UPDATE from pending can win.
func (s *SubmissionStore) ClaimPending(ctx context.Context, id, claimToken string) (bool, error) {
	if claimToken == "" {
		return false, fmt.Errorf("claim pending: empty claim token")
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE submissions s
		    SET status='judging', judging_started_at=NOW(), judging_claim_token=$2
		  WHERE s.id=$1
		    AND s.status='pending'
		    AND COALESCE(s.remote_id,'') = ''
		    AND EXISTS (
		        SELECT 1 FROM problems p
		        WHERE p.id=s.problem_id
		          AND COALESCE(p.source,'local') IN ('','local')
		    )`,
		id, claimToken)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// RequeueStale reclaims submissions abandoned in the "judging" state.
//
// The single UPDATE ... WHERE status='judging' ... RETURNING id is the
// concurrency primitive: Postgres row locks serialize competing sweeps, so a
// row flipped back to 'pending' by one worker is no longer 'judging' and cannot
// be returned again — which is what keeps reclamation exactly-once and stops a
// later requeue from overwriting an already-written verdict.
func (s *SubmissionStore) RequeueStale(ctx context.Context, olderThan time.Duration) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`UPDATE submissions
		    SET status='pending', judging_started_at=NULL, judging_claim_token=NULL
		  WHERE status='judging'
		    AND judging_started_at IS NOT NULL
		    AND judging_started_at < NOW() - $1::interval
		  RETURNING id`,
		fmt.Sprintf("%d seconds", int(olderThan.Seconds())))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *SubmissionStore) UpdateResult(ctx context.Context, id string, status model.SubmissionStatus, score, timeUsed, memoryUsed int, compileOutput string, results []model.TestCaseResult) error {
	_, err := s.updateResult(ctx, id, "", status, score, timeUsed, memoryUsed, compileOutput, results)
	return err
}

// UpdateResultClaimed is the local judge-worker write path. The claim token
// prevents a worker that was presumed dead (but later resumed) from
// overwriting the verdict produced by the replacement worker.
func (s *SubmissionStore) UpdateResultClaimed(ctx context.Context, id, claimToken string, status model.SubmissionStatus, score, timeUsed, memoryUsed int, compileOutput string, results []model.TestCaseResult) (bool, error) {
	if claimToken == "" {
		return false, fmt.Errorf("update claimed result: empty claim token")
	}
	return s.updateResult(ctx, id, claimToken, status, score, timeUsed, memoryUsed, compileOutput, results)
}

func (s *SubmissionStore) updateResult(ctx context.Context, id, claimToken string, status model.SubmissionStatus, score, timeUsed, memoryUsed int, compileOutput string, results []model.TestCaseResult) (bool, error) {
	jr, _ := json.Marshal(results)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	query := `UPDATE submissions
	             SET status=$1,score=$2,time_used=$3,memory_used=$4,compile_output=$5,judge_result=$6,judged_at=$7,
	                 judging_started_at=NULL,judging_claim_token=NULL
	           WHERE id=$8`
	args := []interface{}{status, score, timeUsed, memoryUsed, compileOutput, jr, time.Now(), id}
	if claimToken != "" {
		query += ` AND status='judging' AND judging_claim_token=$9`
		args = append(args, claimToken)
	}
	res, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return false, err
	}
	updated, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if updated != 1 {
		return false, nil
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
		// Achievement milestones (first_ac, ten_ac, …) from distinct AC problems.
		var userID string
		var solved int
		_ = tx.QueryRowContext(ctx,
			`SELECT s.user_id,
				COUNT(DISTINCT s.problem_id) FILTER (WHERE s.status = 'ac')
			 FROM submissions s WHERE s.user_id = (
			   SELECT user_id FROM submissions WHERE id = $1
			 ) GROUP BY s.user_id`, id).Scan(&userID, &solved)
		if userID != "" && solved > 0 {
			awardAchievementMilestones(ctx, tx, userID, solved)
		}
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// awardAchievementMilestones is called inside UpdateResult's transaction.
// Kept package-level so the store does not need a second DB handle.
func awardAchievementMilestones(ctx context.Context, tx *sql.Tx, userID string, solved int) {
	type ms struct {
		n    int
		code string
	}
	for _, m := range []ms{{1, "first_ac"}, {10, "ten_ac"}, {55, "fifty_ac"}, {100, "hundred_ac"}} {
		if solved < m.n {
			continue
		}
		_, _ = tx.ExecContext(ctx, `
			INSERT INTO user_achievements (user_id, achievement_id)
			SELECT u.id, a.id FROM achievements a
			CROSS JOIN (SELECT $1::uuid AS id) u
			WHERE a.code = $2
			ON CONFLICT DO NOTHING`, userID, m.code)
	}
}

func (s *SubmissionStore) UpdateRemoteID(ctx context.Context, id string, remoteID string, remoteURL string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE submissions SET remote_id=$1, remote_url=$2 WHERE id=$3", remoteID, remoteURL, id)
	return err
}

func (s *SubmissionStore) UpdateBotID(ctx context.Context, id string, botID string, botSlug string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE submissions SET bot_id=$1, bot_slug=$2 WHERE id=$3", botID, botSlug, id)
	return err
}

func (s *SubmissionStore) GetUnsubmittedRemoteSubmissions(ctx context.Context) ([]model.Submission, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT s.id, s.problem_id, s.language, s.source_code_gz, s.status, s.created_at
		 FROM submissions s
		 JOIN problems p ON s.problem_id = p.id
		 WHERE s.status = 'pending' AND s.remote_id = '' AND p.source != '' AND p.source != 'local'
		 ORDER BY s.created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []model.Submission
	for rows.Next() {
		var sub model.Submission
		var compressed []byte
		if err := rows.Scan(&sub.ID, &sub.ProblemID, &sub.Language, &compressed, &sub.Status, &sub.CreatedAt); err != nil {
			return nil, err
		}
		if len(compressed) > 0 {
			sub.SourceCode = decompressCode(compressed)
		}
		items = append(items, sub)
	}
	return items, nil
}

func (s *SubmissionStore) GetPendingRemoteSubmissions(ctx context.Context) ([]model.PendingRemoteSubmission, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT s.id, s.remote_id, COALESCE(s.bot_id,''), COALESCE(s.bot_slug,''), s.status, COALESCE(p.source,'')
		 FROM submissions s
		 LEFT JOIN problems p ON s.problem_id = p.id
		 WHERE s.remote_id != '' AND s.status IN ('pending')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []model.PendingRemoteSubmission
	for rows.Next() {
		var ps model.PendingRemoteSubmission
		if err := rows.Scan(&ps.ID, &ps.RemoteID, &ps.BotID, &ps.BotSlug, &ps.Status, &ps.Platform); err != nil {
			return nil, err
		}
		items = append(items, ps)
	}
	if items == nil {
		items = []model.PendingRemoteSubmission{}
	}
	return items, nil
}

func (s *SubmissionStore) ListByContest(ctx context.Context, contestID string, offset, limit int, filter model.SubmissionFilter) ([]model.Submission, int, error) {
	where := "s.contest_id=$1"
	args := []interface{}{contestID}
	argIdx := 2
	if filter.UserID != "" {
		where += fmt.Sprintf(" AND s.user_id=$%d", argIdx)
		args = append(args, filter.UserID)
		argIdx++
	}
	if filter.ProblemID != "" {
		where += fmt.Sprintf(" AND s.problem_id=$%d", argIdx)
		args = append(args, filter.ProblemID)
		argIdx++
	}
	if filter.Language != "" {
		where += fmt.Sprintf(" AND s.language=$%d", argIdx)
		args = append(args, filter.Language)
		argIdx++
	}
	if filter.Status != "" {
		where += fmt.Sprintf(" AND s.status=$%d", argIdx)
		args = append(args, filter.Status)
		argIdx++
	}
	var total int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM submissions s WHERE "+where, args...).Scan(&total)
	args = append(args, offset, limit)
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT s.id,s.user_id,COALESCE(u.username,''),s.problem_id,s.language,s.status,s.score,s.time_used,s.memory_used,s.created_at,s.submission_type
		 FROM submissions s LEFT JOIN users u ON s.user_id = u.id WHERE %s ORDER BY s.created_at DESC OFFSET $%d LIMIT $%d`, where, argIdx, argIdx+1), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []model.Submission
	for rows.Next() {
		var sub model.Submission
		rows.Scan(&sub.ID, &sub.UserID, &sub.Username, &sub.ProblemID, &sub.Language, &sub.Status, &sub.Score, &sub.TimeUsed, &sub.MemoryUsed, &sub.CreatedAt, &sub.SubmissionType)
		items = append(items, sub)
	}
	if items == nil {
		items = []model.Submission{}
	}
	return items, total, nil
}

func (s *SubmissionStore) ListPending(ctx context.Context, limit int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT s.id
		   FROM submissions s
		   JOIN problems p ON p.id=s.problem_id
		  WHERE s.status='pending'
		    AND COALESCE(s.remote_id,'') = ''
		    AND COALESCE(p.source,'local') IN ('','local')
		  ORDER BY s.created_at ASC
		  LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
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
