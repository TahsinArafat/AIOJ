package postgres

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type SubmissionStore struct{ db *sql.DB }

func NewSubmissionStore(db *sql.DB) *SubmissionStore { return &SubmissionStore{db: db} }

// remoteSourceList renders model.RemotePlatforms as a SQL list. The values are
// compiled-in constants, so inlining them is safe; building it from the model
// keeps the judge worker and the vjudge submit worker from disagreeing about
// what counts as remote.
var remoteSourceList = func() string {
	quoted := make([]string, len(model.RemotePlatforms))
	for i, p := range model.RemotePlatforms {
		quoted[i] = "'" + p + "'"
	}
	return "(" + strings.Join(quoted, ",") + ")"
}()

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
		`INSERT INTO submissions(id,problem_id,user_id,contest_id,language,source_code,source_code_gz,code_size,status,submission_type,is_upsolving)
		 VALUES($1,$2,$3,$4,$5,'',$6,$7,$8,$9,$10) RETURNING created_at`,
		sub.ID, sub.ProblemID, sub.UserID, cid, sub.Language, compressed, sub.CodeSize, sub.Status, sub.SubmissionType, sub.IsUpsolving,
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
		`SELECT s.id,s.problem_id,s.user_id,COALESCE(s.contest_id::text,''),s.language,s.source_code,
		        s.source_code_gz,s.code_size,s.status,s.score,s.time_used,s.memory_used,
		        s.compile_output,s.judge_result,s.judged_by,s.created_at,s.judged_at,s.submission_type,
		        s.remote_id,s.remote_url,COALESCE(s.bot_id,''),COALESCE(s.bot_slug,''),
		        COALESCE(s.is_upsolving,false),COALESCE(s.judge_error,''),
		        COALESCE(u.username,'')
		   FROM submissions s
		   LEFT JOIN users u ON u.id = s.user_id
		  WHERE s.id=$1`, id).Scan(
		&sub.ID, &sub.ProblemID, &sub.UserID, &cid, &sub.Language, &sub.SourceCode, &compressed, &sub.CodeSize,
		&sub.Status, &sub.Score, &sub.TimeUsed, &sub.MemoryUsed, &co, &jr,
		&sub.JudgedBy, &sub.CreatedAt, &ja, &sub.SubmissionType, &remoteID, &remoteURL, &botID, &botSlug, &sub.IsUpsolving,
		&sub.JudgeError, &sub.Username)
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
	// Legacy rows (judged before judge_error existed) kept the message in
	// compile_output; surface it as the judge error so the UI explains itself.
	if sub.Status == model.StatusSE && sub.JudgeError == "" {
		sub.JudgeError = sub.CompileOutput
	}
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
		          AND COALESCE(p.source,'') NOT IN `+remoteSourceList+`
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
// be returned again — which makes each stale claim eligible for reclamation once
// per judging attempt and stops a later requeue from overwriting an already-written verdict.
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

	// A System Error's message is the judge's own failure text, not compiler
	// output: keep it in judge_error so the UI never shows it as if the
	// contestant's code failed to compile, and so setters can read it.
	judgeError := ""
	if status == model.StatusSE {
		judgeError = compileOutput
		if judgeError == "" {
			// Some system errors never reach the worker's error path: a missing
			// testdata file, for instance, is reported per case. Falling back to
			// the case details keeps judge_error explanatory instead of empty —
			// an empty one is indistinguishable from "nothing was recorded".
			judgeError = summarizeCaseFailures(results)
		}
	}
	query := `UPDATE submissions
	             SET status=$1,score=$2,time_used=$3,memory_used=$4,compile_output=$5,judge_result=$6,judged_at=$7,
	                 judge_error=$9, judging_started_at=NULL,judging_claim_token=NULL
	           WHERE id=$8`
	args := []interface{}{status, score, timeUsed, memoryUsed, compileOutput, jr, time.Now(), id, judgeError}
	if claimToken != "" {
		// $9 is judge_error; the claim token takes the next slot.
		query += ` AND status='judging' AND judging_claim_token=$10`
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
		 WHERE s.status = 'pending' AND s.remote_id = '' AND p.source IN `+remoteSourceList+`
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
	// The LEFT JOIN to contest_problems resolves the round's letter for each
	// row. It is LEFT, not INNER, so a row whose problem has since been removed
	// from the round (or a submission written with a problem_id the contest
	// never listed) still appears instead of silently vanishing; the letter
	// comes back empty and the caller falls back to the problem's identity.
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT s.id,s.user_id,COALESCE(u.username,''),s.problem_id,COALESCE(cp.index,''),s.language,s.status,s.score,s.time_used,s.memory_used,s.created_at,s.submission_type
		 FROM submissions s
		 LEFT JOIN users u ON s.user_id = u.id
		 LEFT JOIN contest_problems cp ON cp.contest_id = s.contest_id AND cp.problem_id = s.problem_id
		 WHERE %s ORDER BY s.created_at DESC OFFSET $%d LIMIT $%d`, where, argIdx, argIdx+1), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []model.Submission
	for rows.Next() {
		var sub model.Submission
		rows.Scan(&sub.ID, &sub.UserID, &sub.Username, &sub.ProblemID, &sub.ProblemIndex, &sub.Language, &sub.Status, &sub.Score, &sub.TimeUsed, &sub.MemoryUsed, &sub.CreatedAt, &sub.SubmissionType)
		items = append(items, sub)
	}
	if items == nil {
		items = []model.Submission{}
	}
	return items, total, nil
}

// ListPending returns IDs of local pending submissions, ordered oldest first.
func (s *SubmissionStore) ListPending(ctx context.Context, limit int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT s.id
		   FROM submissions s
		   JOIN problems p ON p.id=s.problem_id
		  WHERE s.status='pending'
		    AND COALESCE(s.remote_id,'') = ''
		    AND COALESCE(p.source,'') NOT IN `+remoteSourceList+`
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

// ListPendingWithContext returns pending local submissions enriched with their
// contest window so the reclaimer can assign the correct queue priority.
func (s *SubmissionStore) ListPendingWithContext(ctx context.Context, limit int) ([]store.PendingSubmissionInfo, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id,
		       c.start_time,
		       c.end_time,
		       COALESCE(s.is_upsolving, false)
		  FROM submissions s
		  JOIN problems p ON p.id = s.problem_id
		  LEFT JOIN contests c ON c.id = s.contest_id
		 WHERE s.status = 'pending'
		   AND COALESCE(s.remote_id, '') = ''
		   AND COALESCE(p.source,'') NOT IN `+remoteSourceList+`
		 ORDER BY s.created_at ASC
		 LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.PendingSubmissionInfo
	for rows.Next() {
		var info store.PendingSubmissionInfo
		if err := rows.Scan(&info.ID, &info.ContestStart, &info.ContestEnd, &info.IsUpsolving); err != nil {
			return nil, err
		}
		out = append(out, info)
	}
	return out, rows.Err()
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

// ListForBulkRejudge returns IDs of judged local submissions in a contest,
// optionally filtered to one problem.  Pending/judging rows are excluded so
// they don't get double-enqueued by a concurrent reclaimer sweep.
func (s *SubmissionStore) ListForBulkRejudge(ctx context.Context, contestID, problemID string) ([]string, error) {
	query := `
		SELECT s.id
		  FROM submissions s
		  JOIN problems p ON p.id = s.problem_id
		 WHERE s.contest_id = $1
		   AND s.status NOT IN ('pending', 'judging')
		   AND COALESCE(s.remote_id, '') = ''
		   AND COALESCE(p.source,'') NOT IN ` + remoteSourceList + ``
	args := []interface{}{contestID}
	if problemID != "" {
		args = append(args, problemID)
		query += ` AND s.problem_id = $2`
	}
	query += ` ORDER BY s.created_at ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
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

// ResetForRejudge atomically resets a batch of submission IDs to 'pending',
// clears prior verdicts, and clears any stale claim token.
func (s *SubmissionStore) ResetForRejudge(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	// Build $1,$2,... placeholder list.
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE submissions
		   SET status = 'pending',
		       score = 0,
		       time_used = 0,
		       memory_used = 0,
		       compile_output = '',
		       judge_error = '',
		       -- judge_result is JSONB NOT NULL DEFAULT '[]': writing NULL here
		       -- made every rejudge fail with a not-null violation.
		       judge_result = '[]'::jsonb,
		       judging_started_at = NULL,
		       judging_claim_token = NULL,
		       judged_at = NULL
		 WHERE id IN (`+strings.Join(placeholders, ",")+`)`, args...)
	return err
}

// submissionListColumns is the projection shared by the listing queries that
// have to name the submitter and the problem. Kept in one place so the scan
// order below cannot drift from the SELECT.
// judgeErrorExpr is the failure text of a System Error.
//
// Rows judged before judge_error existed carry the message in compile_output
// instead, and the UI hides compile_output unless the verdict is a compile
// error — so those historical failures looked like they had no explanation.
// NULLIF keeps a genuinely empty judge_error distinguishable from a missing
// one.
const judgeErrorExpr = `COALESCE(NULLIF(s.judge_error,''), CASE WHEN s.status='se' THEN s.compile_output ELSE '' END)`

const submissionListColumns = `s.id,s.user_id,s.problem_id,COALESCE(u.username,''),
	COALESCE(p.slug,''),COALESCE(p.title,''),
	s.language,s.status,s.score,s.time_used,s.memory_used,s.created_at,s.submission_type,
	COALESCE(s.contest_id::text,''),COALESCE(s.is_upsolving,false),` + judgeErrorExpr

func scanSubmissionList(rows *sql.Rows) ([]model.Submission, error) {
	var items []model.Submission
	for rows.Next() {
		var sub model.Submission
		if err := rows.Scan(&sub.ID, &sub.UserID, &sub.ProblemID, &sub.Username,
			&sub.ProblemSlug, &sub.ProblemTitle,
			&sub.Language, &sub.Status, &sub.Score, &sub.TimeUsed, &sub.MemoryUsed, &sub.CreatedAt, &sub.SubmissionType,
			&sub.ContestID, &sub.IsUpsolving, &sub.JudgeError); err != nil {
			return nil, err
		}
		items = append(items, sub)
	}
	if items == nil {
		items = []model.Submission{}
	}
	return items, rows.Err()
}

// ListByProblemManage lists one problem's submissions for its setters.
func (s *SubmissionStore) ListByProblemManage(ctx context.Context, problemID string, offset, limit int, filter model.SubmissionFilter) ([]model.Submission, int, error) {
	where := "s.problem_id=$1"
	args := []interface{}{problemID}
	argIdx := 2
	add := func(clause string, v interface{}) {
		where += fmt.Sprintf(" AND "+clause+"=$%d", argIdx)
		args = append(args, v)
		argIdx++
	}
	if filter.UserID != "" {
		add("s.user_id", filter.UserID)
	}
	if filter.Username != "" {
		where += fmt.Sprintf(" AND LOWER(u.username)=LOWER($%d)", argIdx)
		args = append(args, filter.Username)
		argIdx++
	}
	if filter.ContestID != "" {
		add("s.contest_id", filter.ContestID)
	}
	if filter.Language != "" {
		add("s.language", filter.Language)
	}
	if filter.Status != "" {
		add("s.status", filter.Status)
	}

	const joins = ` FROM submissions s
		LEFT JOIN users u ON s.user_id = u.id
		LEFT JOIN problems p ON s.problem_id = p.id`
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*)"+joins+" WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, offset, limit)
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT %s%s WHERE %s ORDER BY s.created_at DESC, s.id OFFSET $%d LIMIT $%d`,
			submissionListColumns, joins, where, argIdx, argIdx+1), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items, err := scanSubmissionList(rows)
	return items, total, err
}

// ListPublic lists submissions site-wide for the status page.
//
// Visibility is decided in SQL: an anonymous reader only sees submissions of
// visible problems that are not inside a contest that is still running (a
// live contest's verdicts would leak the problem set). A signed-in reader also
// sees their own rows — including hidden problems and live contests — so their
// status page never hides work they just submitted. Admins (ShowAll) skip
// both restrictions.
func (s *SubmissionStore) ListPublic(ctx context.Context, offset, limit int, filter model.PublicSubmissionFilter) ([]model.Submission, int, error) {
	const joins = ` FROM submissions s
		LEFT JOIN users u ON s.user_id = u.id
		LEFT JOIN problems p ON s.problem_id = p.id
		LEFT JOIN contests c ON s.contest_id = c.id`

	args := []interface{}{}
	// Seeded with TRUE: an admin (ShowAll) with no filters contributes no
	// clause of its own, and an empty WHERE is a syntax error — every admin
	// load of the status page 500'd on it.
	conds := []string{"TRUE"}
	next := func(v interface{}) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	const openToPublic = "COALESCE(p.visible,false) = true AND (c.id IS NULL OR NOW() >= c.end_time)"
	switch {
	case filter.ShowAll:
	case filter.ViewerID != "":
		conds = append(conds, fmt.Sprintf("(s.user_id = %s OR (%s))", next(filter.ViewerID), openToPublic))
	default:
		conds = append(conds, "("+openToPublic+")")
	}

	if filter.ProblemSlug != "" {
		conds = append(conds, "p.slug = "+next(filter.ProblemSlug))
	}
	if filter.ProblemID != "" {
		conds = append(conds, "s.problem_id = "+next(filter.ProblemID))
	}
	if filter.UserID != "" {
		conds = append(conds, "s.user_id = "+next(filter.UserID))
	}
	if filter.Username != "" {
		conds = append(conds, "LOWER(u.username) = LOWER("+next(filter.Username)+")")
	}
	if filter.ContestID != "" {
		conds = append(conds, "s.contest_id = "+next(filter.ContestID))
	}
	if filter.Language != "" {
		conds = append(conds, "s.language = "+next(filter.Language))
	}
	if filter.Status != "" {
		conds = append(conds, "s.status = "+next(filter.Status))
	}

	where := strings.Join(conds, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*)"+joins+" WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// ORDER BY id as a tiebreaker: created_at is not unique, and without it two
	// rows sharing a timestamp could swap places between pages.
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT %s%s WHERE %s ORDER BY s.created_at DESC, s.id LIMIT $%d OFFSET $%d`,
			submissionListColumns, joins, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items, err := scanSubmissionList(rows)
	// The status page is public: judge failure text stays with admins and the
	// problem's setters, who read it from the setter listing instead.
	for i := range items {
		items[i].JudgeError = ""
	}
	return items, total, err
}

// ListIDsForProblemRejudge returns the judged local submissions of a problem.
func (s *SubmissionStore) ListIDsForProblemRejudge(ctx context.Context, problemID string, filter model.SubmissionFilter) ([]string, error) {
	query := `
		SELECT s.id
		  FROM submissions s
		  JOIN problems p ON p.id = s.problem_id
		 WHERE s.problem_id = $1
		   AND s.status NOT IN ('pending', 'judging')
		   AND COALESCE(s.remote_id, '') = ''
		   AND COALESCE(p.source,'') NOT IN ` + remoteSourceList + ``
	args := []interface{}{problemID}
	if filter.ContestID != "" {
		args = append(args, filter.ContestID)
		query += ` AND s.contest_id = $2`
	}
	query += ` ORDER BY s.created_at ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
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

// RecomputeProblemCounts rebuilds the denormalised counters from the table.
func (s *SubmissionStore) RecomputeProblemCounts(ctx context.Context, problemID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE problems p
		   SET submission_count = (SELECT COUNT(*) FROM submissions s WHERE s.problem_id = p.id),
		       accepted_count   = (SELECT COUNT(*) FROM submissions s WHERE s.problem_id = p.id AND s.status = 'ac')
		 WHERE p.id = $1`, problemID)
	return err
}

// ListRecentFailures returns the newest System Error submissions with the
// judge's failure text, for the admin troubleshooting panel.
func (s *SubmissionStore) ListRecentFailures(ctx context.Context, limit int) ([]model.Submission, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT %s
		  FROM submissions s
		  LEFT JOIN users u ON s.user_id = u.id
		  LEFT JOIN problems p ON s.problem_id = p.id
		 WHERE s.status = 'se'
		 ORDER BY s.created_at DESC
		 LIMIT $1`, submissionListColumns), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSubmissionList(rows)
}

// summarizeCaseFailures turns per-case system errors into one readable line.
// Capped so a problem with hundreds of broken cases does not write a wall of
// text into the row.
func summarizeCaseFailures(results []model.TestCaseResult) string {
	const maxCases = 5
	var lines []string
	for _, r := range results {
		if r.Status != model.StatusSE || r.Detail == "" {
			continue
		}
		lines = append(lines, r.CaseName+": "+r.Detail)
		if len(lines) == maxCases {
			lines = append(lines, "…")
			break
		}
	}
	return strings.Join(lines, "\n")
}

// FeedRun is one submission as an external scorer needs to see it.
type FeedRun struct {
	SubmissionID string
	UserID       string
	Username     string
	TeamName     string
	ProblemIndex string
	Status       string
	Score        int
	TimeUsed     int
	MemoryUsed   int
	CreatedAt    time.Time
}

// ListFeed returns a contest's submissions in creation order for the CCS-style
// event feed.
//
// Ordered by (created_at, id) and cursored on the pair rather than an offset: a
// scorer polls this repeatedly, and an offset would re-send or skip rows as new
// submissions arrive. `since` is the opaque cursor returned by the previous call
// (format "RFC3339Nano|uuid"); an empty cursor starts from the beginning.
func (s *SubmissionStore) ListFeed(ctx context.Context, contestID, since string, limit int) ([]FeedRun, string, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}

	var rows *sql.Rows
	var err error
	if since == "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT s.id, s.user_id, COALESCE(u.username,''), s.status, s.score,
			        s.time_used, s.memory_used, s.created_at,
			        COALESCE(cp.index,''), COALESCE(ob.team_name,'')
			   FROM submissions s
			   LEFT JOIN users u ON u.id = s.user_id
			   LEFT JOIN contest_problems cp ON cp.contest_id = s.contest_id AND cp.problem_id = s.problem_id
			   LEFT JOIN onsite_batch_users ob ON ob.used_by = s.user_id AND ob.contest_id = s.contest_id
			  WHERE s.contest_id = $1
			  ORDER BY s.created_at ASC, s.id ASC
			  LIMIT $2`, contestID, limit)
	} else {
		at, id, ok := splitFeedCursor(since)
		if !ok {
			return nil, "", fmt.Errorf("invalid cursor")
		}
		rows, err = s.db.QueryContext(ctx,
			`SELECT s.id, s.user_id, COALESCE(u.username,''), s.status, s.score,
			        s.time_used, s.memory_used, s.created_at,
			        COALESCE(cp.index,''), COALESCE(ob.team_name,'')
			   FROM submissions s
			   LEFT JOIN users u ON u.id = s.user_id
			   LEFT JOIN contest_problems cp ON cp.contest_id = s.contest_id AND cp.problem_id = s.problem_id
			   LEFT JOIN onsite_batch_users ob ON ob.used_by = s.user_id AND ob.contest_id = s.contest_id
			  WHERE s.contest_id = $1
			    AND (s.created_at, s.id) > ($2, $3)
			  ORDER BY s.created_at ASC, s.id ASC
			  LIMIT $4`, contestID, at, id, limit)
	}
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var out []FeedRun
	for rows.Next() {
		var r FeedRun
		if err := rows.Scan(&r.SubmissionID, &r.UserID, &r.Username, &r.Status, &r.Score,
			&r.TimeUsed, &r.MemoryUsed, &r.CreatedAt, &r.ProblemIndex, &r.TeamName); err != nil {
			return nil, "", err
		}
		out = append(out, r)
	}
	if out == nil {
		out = []FeedRun{}
	}

	next := since
	if len(out) > 0 {
		last := out[len(out)-1]
		next = last.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + last.SubmissionID
	}
	return out, next, rows.Err()
}

// splitFeedCursor parses "RFC3339Nano|uuid".
func splitFeedCursor(c string) (time.Time, string, bool) {
	i := strings.LastIndex(c, "|")
	if i <= 0 || i == len(c)-1 {
		return time.Time{}, "", false
	}
	at, err := time.Parse(time.RFC3339Nano, c[:i])
	if err != nil {
		return time.Time{}, "", false
	}
	return at, c[i+1:], true
}
