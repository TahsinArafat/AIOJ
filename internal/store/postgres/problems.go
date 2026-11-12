package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/lib/pq"
	"github.com/tahsinarafat/aioj/internal/model"
)

type ProblemStore struct {
	db *sql.DB
}

func NewProblemStore(db *sql.DB) *ProblemStore {
	return &ProblemStore{db: db}
}

func (s *ProblemStore) Create(ctx context.Context, p *model.Problem) error {
	if p.Difficulty == "" {
		p.Difficulty = "easy"
	}
	if p.ScoringMode == "" {
		p.ScoringMode = "complete"
	}
	if p.SubtaskAggregation == "" {
		p.SubtaskAggregation = "min"
	}
	// A nil slice would be rendered by pq.Array as the SQL literal NULL, which
	// violates the NOT NULL constraint on problems.tags (SQLSTATE 23502) and
	// surfaced only as a bare "create failed" 500. Normalise to an empty array.
	if p.Tags == nil {
		p.Tags = []string{}
	}

	samples, err := json.Marshal(p.SampleCases)
	if err != nil {
		return fmt.Errorf("marshal sample_cases: %w", err)
	}
	scores, err := json.Marshal(p.TestCaseScore)
	if err != nil {
		return fmt.Errorf("marshal testcase_score: %w", err)
	}

	// A caller that predates the review workflow (the AI apply path, the seeder)
	// leaves Status empty. Treat that as a draft rather than sending '' to a
	// CHECK constraint that would reject the insert outright — and keep visible
	// consistent with it, since the constraint ties the two together.
	if p.Status == "" {
		p.Status = model.ProblemStatusDraft
	}
	if p.ProblemType == "" {
		p.ProblemType = model.ProblemTypeStandard
	}
	p.Visible = p.Status == model.ProblemStatusPublished

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx, `INSERT INTO problems
		(id,slug,title,description,constraints,input_format,output_format,hint,sample_cases,
		 time_limit,memory_limit,difficulty,rating,tags,visible,testdata_path,testcase_score,
		 spj,spj_language,spj_source_code,spj_version,source,remote_id,created_by,
		 checker_type,float_epsilon,interactive,interactor_language,interactor_source_code,
		 scoring_mode,subtask_aggregation,ai_generated,status,review_note,reviewed_at,problem_type,
		 validator_language,validator_source)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33,$34,$35,$36,$37,$38)
		RETURNING created_at,updated_at`,
		p.ID, p.Slug, p.Title, p.Description, p.Constraints, p.InputFormat, p.OutputFormat, p.Hint, samples,
		p.TimeLimit, p.MemoryLimit, p.Difficulty, p.Rating, pq.Array(p.Tags), p.Visible, p.TestdataPath, scores,
		p.SPJ, p.SPJLanguage, p.SPJSourceCode, p.SPJVersion, p.Source, p.RemoteID, p.CreatedBy,
		p.CheckerType, p.FloatEpsilon, p.Interactive, p.InteractorLanguage, p.InteractorSourceCode,
		p.ScoringMode, p.SubtaskAggregation, p.AIGenerated, p.Status, p.ReviewNote, p.ReviewedAt, p.ProblemType,
		p.ValidatorLanguage, p.ValidatorSource,
	).Scan(&p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert problem: %w", err)
	}

	if p.CreatedBy != "" {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO problem_permissions(problem_id, user_id, access_level) VALUES($1,$2,$3)
			 ON CONFLICT(problem_id, user_id) DO UPDATE SET access_level=$3`,
			p.ID, p.CreatedBy, "owner")
		if err != nil {
			return fmt.Errorf("add owner permission: %w", err)
		}
	}

	return tx.Commit()
}

func (s *ProblemStore) GetByID(ctx context.Context, id string) (*model.Problem, error) {
	return s.getBy(ctx, "id", id)
}

func (s *ProblemStore) GetBySlug(ctx context.Context, slug string) (*model.Problem, error) {
	return s.getBy(ctx, "slug", slug)
}

// problemColumns is the one column list that produces a complete model.Problem,
// paired with scanProblem below. Both the practice read (getBy) and the contest
// read (ContestStore.GetContestProblemByIndex) select it so the two cannot drift
// again: contest statements silently lost their Constraints block because the
// contest query was a hand-maintained copy that never gained the column.
//
// Every nullable column is COALESCEd: scanning a NULL into a plain string or
// bool fails the whole read.
const problemColumns = `p.id, p.slug, p.title, p.description, COALESCE(p.constraints,''),
	p.input_format, p.output_format, p.hint, p.sample_cases,
	p.time_limit, p.memory_limit, p.difficulty, COALESCE(p.rating,0), p.tags, p.visible, p.testdata_path,
	p.testcase_score, p.spj, p.spj_language, p.spj_source_code, p.spj_version, p.checker_type,
	p.float_epsilon, p.submission_count, p.accepted_count, p.source, p.remote_id,
	COALESCE(p.created_by::text,''), p.created_at, p.updated_at,
	COALESCE(p.interactive,false), COALESCE(p.interactor_language,''), COALESCE(p.interactor_source_code,''),
	COALESCE(p.scoring_mode,'complete'), COALESCE(p.subtask_aggregation,'min'),
	COALESCE(p.ai_generated,false),
	COALESCE(p.status,'draft'), COALESCE(p.review_note,''), p.reviewed_at,
	COALESCE(p.problem_type,'standard'),
	COALESCE(p.validator_language,''), COALESCE(p.validator_source,'')`

// unreleasedContestSQL is true when the problem is being used by a contest that
// is open right now: published, started, and not finished yet. Such a problem is
// not part of the archive, the way Codeforces keeps a round's problems out of the
// problem set until the round is over. Every public archive read excludes it with
// `NOT unreleasedContestSQL`.
//
// All three conditions are load-bearing, and each one was learned the hard way:
//
//   - `c.start_time <= NOW()` bounds it below. Without it a round scheduled for
//     next week took its problems out of the archive today, days before anybody
//     could be spoilt — the problem simply vanished from the catalogue while its
//     setter was still writing it up.
//   - `c.end_time > NOW()` bounds it above, so a finished round's problems go
//     straight back into the catalogue.
//   - `c.visible` keeps a draft round from concealing anything. visible mirrors
//     status = 'published' (ContestStore.SetReviewState), and a round nobody can
//     enter spoils nothing — yet an abandoned half-configured draft pinned a
//     published problem out of the archive indefinitely, with nothing on screen
//     to explain why.
//
// So a problem is withheld exactly while some reader who is not supposed to see
// it could actually reach it.
const unreleasedContestSQL = `EXISTS (
	SELECT 1 FROM contest_problems cp
	JOIN contests c ON c.id = cp.contest_id
	WHERE cp.problem_id = p.id
	  AND c.visible
	  AND c.start_time <= NOW()
	  AND c.end_time > NOW()
)`

// archivedProblemSQL is the condition a problem must satisfy to appear in the
// public archive at all: approved by an admin, and not still in play in a
// contest.
//
// It reads status, not visible. The CHECK constraint only guarantees
// published => visible, so a row could in principle be visible while still a
// draft; status is the actual review decision, and filtering on it means the
// archive cannot leak an unapproved problem even if visible is set by hand in
// psql or by a future caller that forgets to derive it.
const archivedProblemSQL = "p.status = 'published' AND NOT " + unreleasedContestSQL

// selectableProblemSQL is what a contest's problem picker searches: the public
// archive, plus the problems the caller created or holds a permission row on.
//
// The second and third clauses are why a setter can finally attach their own
// limited (visible = false) draft to a round. They are deliberately scoped to
// the caller — p.created_by = $1 and perm.user_id = $1 — so opening the picker
// wider than the archive does not also expose every other setter's private
// drafts, which is the bug the archive filter was protecting against.
const selectableProblemSQL = `((` + archivedProblemSQL + `)
	OR p.created_by = $1
	OR EXISTS (SELECT 1 FROM problem_permissions perm WHERE perm.problem_id = p.id AND perm.user_id = $1))`

// InUnreleasedContest reports whether the problem is being used by a contest
// that is open right now. This is the predicate behind the archive exclusion and
// behind withholding a problem's difficulty, so an upcoming or unpublished round
// reports false here and leaves the problem alone. See unreleasedContestSQL.
func (s *ProblemStore) InUnreleasedContest(ctx context.Context, problemID string) (bool, error) {
	var locked bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS (
		    SELECT 1 FROM contest_problems cp
		    JOIN contests c ON c.id = cp.contest_id
		    WHERE cp.problem_id = $1
		      AND c.visible
		      AND c.start_time <= NOW()
		      AND c.end_time > NOW()
		 )`, problemID).Scan(&locked)
	return locked, err
}

// problemScanner is the *sql.Row / *sql.Rows shape scanProblem needs.
type problemScanner interface {
	Scan(dest ...any) error
}

// scanProblem reads one row selected with problemColumns into p.
func scanProblem(row problemScanner, p *model.Problem) error {
	var samples, scores []byte
	if err := row.Scan(
		&p.ID, &p.Slug, &p.Title, &p.Description, &p.Constraints, &p.InputFormat, &p.OutputFormat, &p.Hint, &samples,
		&p.TimeLimit, &p.MemoryLimit, &p.Difficulty, &p.Rating, pq.Array(&p.Tags), &p.Visible, &p.TestdataPath,
		&scores, &p.SPJ, &p.SPJLanguage, &p.SPJSourceCode, &p.SPJVersion, &p.CheckerType, &p.FloatEpsilon,
		&p.SubmissionCount, &p.AcceptedCount, &p.Source, &p.RemoteID, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
		&p.Interactive, &p.InteractorLanguage, &p.InteractorSourceCode, &p.ScoringMode, &p.SubtaskAggregation,
		&p.AIGenerated,
		&p.Status, &p.ReviewNote, &p.ReviewedAt, &p.ProblemType,
		&p.ValidatorLanguage, &p.ValidatorSource,
	); err != nil {
		return err
	}
	if len(samples) > 0 {
		if err := json.Unmarshal(samples, &p.SampleCases); err != nil {
			p.SampleCases = []model.SampleCase{}
		}
	}
	if len(scores) > 0 {
		if err := json.Unmarshal(scores, &p.TestCaseScore); err != nil {
			p.TestCaseScore = []model.TestCaseScore{}
		}
	}
	if p.Tags == nil {
		p.Tags = []string{}
	}
	return nil
}

func (s *ProblemStore) getBy(ctx context.Context, field, value string) (*model.Problem, error) {
	var p model.Problem
	query := fmt.Sprintf("SELECT %s FROM problems p WHERE p.%s=$1", problemColumns, field)
	err := scanProblem(s.db.QueryRowContext(ctx, query, value), &p)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// List returns the public archive (published, not currently in play in a live
// contest) for ordinary callers. An admin passes admin=true and gets every
// problem regardless of review state — including drafts — so the setter
// workspace's "All Problems" tab shows the person who approves problems the
// ones still waiting. The admin flag is a handler decision, never read from the
// problem row.
func (s *ProblemStore) List(ctx context.Context, offset, limit int, admin bool) ([]model.ProblemListItem, int, error) {
	where := archivedProblemSQL
	if admin {
		// An admin sees every problem regardless of review state, so an
		// unpublished draft is not invisible to the person who approves it.
		where = "TRUE"
	}
	var total int
	s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM problems p WHERE "+where).Scan(&total)
	query := fmt.Sprintf(`SELECT p.id, p.slug, p.title, p.difficulty, COALESCE(p.rating,0), p.tags,
		 p.submission_count, p.accepted_count, p.source, COALESCE(p.status,'draft')
		 FROM problems p WHERE %s ORDER BY p.created_at DESC OFFSET $1 LIMIT $2`, where)
	rows, err := s.db.QueryContext(ctx, query, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []model.ProblemListItem
	for rows.Next() {
		var item model.ProblemListItem
		var tags []string
		rows.Scan(&item.ID, &item.Slug, &item.Title, &item.Difficulty, &item.Rating, pq.Array(&tags),
			&item.SubmissionCount, &item.AcceptedCount, &item.Source, &item.Status)
		item.Tags = tags
		items = append(items, item)
	}
	if items == nil {
		items = []model.ProblemListItem{}
	}
	return items, total, nil
}

func (s *ProblemStore) UpdateCounts(ctx context.Context, id string, addSubmission, addAccepted int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE problems SET submission_count=submission_count+$2, accepted_count=accepted_count+$3 WHERE id=$1`,
		id, addSubmission, addAccepted)
	return err
}

func (s *ProblemStore) Update(ctx context.Context, id string, p *model.Problem) error {
	if p.Difficulty == "" {
		p.Difficulty = "easy"
	}
	samples, err := json.Marshal(p.SampleCases)
	if err != nil {
		return fmt.Errorf("marshal sample_cases: %w", err)
	}
	scores, err := json.Marshal(p.TestCaseScore)
	if err != nil {
		return fmt.Errorf("marshal testcase_score: %w", err)
	}
	// visible is derived from status, never taken from the caller. Publishing is
	// an admin decision enforced in the handler, so a client that sends
	// visible:true on a draft must not be able to publish it by editing. The
	// CHECK constraint from migration 000081 enforces the same invariant in the
	// database, so this cannot drift even if a future caller forgets.
	// Every placeholder must be both referenced and supplied. This statement
	// once carried visible=$14 while also deriving visible from status, which
	// left $14 dangling: Postgres cannot infer the type of a parameter that
	// appears nowhere, so every problem save failed with a 500.
	_, err = s.db.ExecContext(ctx, `UPDATE problems SET
		title=$2, description=$3, constraints=$4, input_format=$5, output_format=$6, hint=$7, sample_cases=$8,
		time_limit=$9, memory_limit=$10, difficulty=$11, rating=$12, tags=$13,
		testdata_path=$14, testcase_score=$15, spj=$16, spj_language=$17, spj_source_code=$18, checker_type=$19, float_epsilon=$20,
		interactive=$21, interactor_language=$22, interactor_source_code=$23,
		scoring_mode=$24, subtask_aggregation=$25,
		visible=($26::varchar='published'), status=$26, review_note=$27, reviewed_at=$28, problem_type=$29,
		validator_language=$30, validator_source=$31, updated_at=NOW() WHERE id=$1`,
		id, p.Title, p.Description, p.Constraints, p.InputFormat, p.OutputFormat, p.Hint, samples,
		p.TimeLimit, p.MemoryLimit, p.Difficulty, p.Rating, pq.Array(p.Tags), p.TestdataPath,
		scores, p.SPJ, p.SPJLanguage, p.SPJSourceCode, p.CheckerType, p.FloatEpsilon,
		p.Interactive, p.InteractorLanguage, p.InteractorSourceCode,
		p.ScoringMode, p.SubtaskAggregation, p.Status, p.ReviewNote, p.ReviewedAt, p.ProblemType,
		p.ValidatorLanguage, p.ValidatorSource)
	return err
}

func (s *ProblemStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM problems WHERE id=$1", id)
	return err
}

// SetReviewState records an admin's verdict on a problem and keeps visible in
// step with it, so approval is the only thing that makes a problem public and a
// rejection or a send-back makes it private again.
func (s *ProblemStore) SetReviewState(ctx context.Context, id, status, note, reviewerID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE problems
		    SET status=$2, review_note=$3, reviewed_by=NULLIF($4,'')::uuid, reviewed_at=NOW(),
		        visible=($2::varchar='published'), updated_at=NOW()
		  WHERE id=$1`,
		id, status, note, reviewerID)
	return err
}

// ListByStatus is the review queue. Pending is what the admin panel shows; the
// other states back the setter's own filtered view.
func (s *ProblemStore) ListByStatus(ctx context.Context, status string, offset, limit int) ([]model.ProblemListItem, int, error) {
	var total int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM problems WHERE status=$1`, status).Scan(&total)

	rows, err := s.db.QueryContext(ctx,
		`SELECT p.id, p.slug, p.title, p.difficulty, COALESCE(p.rating,0), p.tags,
		        p.submission_count, p.accepted_count, p.source, p.status
		   FROM problems p WHERE p.status=$1
		  ORDER BY p.created_at ASC OFFSET $2 LIMIT $3`, status, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.ProblemListItem
	for rows.Next() {
		var item model.ProblemListItem
		var tags []string
		rows.Scan(&item.ID, &item.Slug, &item.Title, &item.Difficulty, &item.Rating, pq.Array(&tags),
			&item.SubmissionCount, &item.AcceptedCount, &item.Source, &item.Status)
		item.Tags = tags
		items = append(items, item)
	}
	if items == nil {
		items = []model.ProblemListItem{}
	}
	return items, total, nil
}

func (s *ProblemStore) AddPermission(ctx context.Context, problemID, userID, accessLevel string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO problem_permissions(problem_id, user_id, access_level) VALUES($1,$2,$3)
		 ON CONFLICT(problem_id, user_id) DO UPDATE SET access_level=$3`, problemID, userID, accessLevel)
	return err
}

func (s *ProblemStore) RemovePermission(ctx context.Context, problemID, userID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM problem_permissions WHERE problem_id=$1 AND user_id=$2", problemID, userID)
	return err
}

func (s *ProblemStore) GetPermissions(ctx context.Context, problemID string) ([]model.ProblemPermission, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT p.problem_id, p.user_id, p.access_level, u.username 
		 FROM problem_permissions p JOIN users u ON p.user_id = u.id WHERE p.problem_id=$1`, problemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []model.ProblemPermission
	for rows.Next() {
		var pp model.ProblemPermission
		rows.Scan(&pp.ProblemID, &pp.UserID, &pp.AccessLevel, &pp.Username)
		items = append(items, pp)
	}
	if items == nil {
		items = []model.ProblemPermission{}
	}
	return items, nil
}

func (s *ProblemStore) HasAccess(ctx context.Context, problemID, userID string, requiredLevels ...string) bool {
	var level string
	err := s.db.QueryRowContext(ctx, "SELECT access_level FROM problem_permissions WHERE problem_id=$1 AND user_id=$2", problemID, userID).Scan(&level)
	if err != nil {
		return false
	}
	for _, req := range requiredLevels {
		if level == req {
			return true
		}
	}
	return false
}

func (s *ProblemStore) ListWithFilter(ctx context.Context, offset, limit int, admin bool, difficulty string, tags []string, search string, source string, rating string, sortBy string) ([]model.ProblemListItem, int, error) {
	// The archive restricts to published, non-live problems. An admin sees every
	// problem regardless of review state (drafts included), so the restriction
	// is lifted only for them and the admin flag stays a handler decision.
	where := []string{archivedProblemSQL}
	if admin {
		where = []string{"TRUE"}
	}
	args := []interface{}{}
	argIdx := 1

	if difficulty != "" {
		where = append(where, fmt.Sprintf("p.difficulty = $%d", argIdx))
		args = append(args, difficulty)
		argIdx++
	}

	if len(tags) > 0 {
		where = append(where, fmt.Sprintf("p.tags && $%d", argIdx))
		args = append(args, pq.Array(tags))
		argIdx++
	}

	if search != "" {
		// Hybrid search, ported from dmoj's judge/fulltext.py, which keeps a real
		// full-text index rather than relying on a LIKE scan.
		//
		// Full-text alone is not enough: a tsquery matches whole lexemes, so a
		// user typing "hel" would not find "hello". Substring matching alone is
		// not enough either: it ignores word boundaries and can't rank. So both
		// run, OR'd, and the trigram/tsvector indexes from migration 000057
		// serve each side.
		//
		// plainto_tsquery is used (not to_tsquery) because it treats the input as
		// plain text and never raises a syntax error on punctuation a user typed.
		where = append(where, fmt.Sprintf(
			"(p.search_vector @@ plainto_tsquery('simple', $%d) OR p.title ILIKE $%d OR p.slug ILIKE $%d)",
			argIdx, argIdx+1, argIdx+2))
		args = append(args, search, "%"+search+"%", "%"+search+"%")
		argIdx += 3
	}

	if source != "" {
		where = append(where, fmt.Sprintf("p.source = $%d", argIdx))
		args = append(args, source)
		argIdx++
	}

	if rating != "" {
		parts := strings.Split(rating, "-")
		if len(parts) == 2 {
			minR, _ := strconv.Atoi(parts[0])
			maxR, _ := strconv.Atoi(parts[1])
			where = append(where, fmt.Sprintf("p.rating >= $%d AND p.rating <= $%d", argIdx, argIdx+1))
			args = append(args, minR, maxR)
			argIdx += 2
		} else if strings.HasSuffix(rating, "+") {
			minR, _ := strconv.Atoi(strings.TrimSuffix(rating, "+"))
			where = append(where, fmt.Sprintf("p.rating >= $%d", argIdx))
			args = append(args, minR)
			argIdx++
		}
	}

	whereClause := strings.Join(where, " AND ")

	var total int
	countQuery := "SELECT COUNT(*) FROM problems p WHERE " + whereClause
	s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)

	orderClause := "p.created_at DESC"
	switch sortBy {
	case "oldest":
		orderClause = "p.created_at ASC"
	case "most_solved":
		orderClause = "p.accepted_count DESC"
	case "least_solved":
		orderClause = "p.accepted_count ASC"
	case "title_asc":
		orderClause = "p.title ASC"
	}

	selectQuery := fmt.Sprintf(`SELECT p.id, p.slug, p.title, p.difficulty, COALESCE(p.rating,0), p.tags,
		p.submission_count, p.accepted_count, p.source, COALESCE(p.status,'draft')
		FROM problems p WHERE %s ORDER BY %s OFFSET $%d LIMIT $%d`, whereClause, orderClause, argIdx, argIdx+1)
	args = append(args, offset, limit)

	rows, err := s.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.ProblemListItem
	for rows.Next() {
		var item model.ProblemListItem
		var tagArr []string
		rows.Scan(&item.ID, &item.Slug, &item.Title, &item.Difficulty, &item.Rating, pq.Array(&tagArr),
			&item.SubmissionCount, &item.AcceptedCount, &item.Source, &item.Status)
		item.Tags = tagArr
		items = append(items, item)
	}
	if items == nil {
		items = []model.ProblemListItem{}
	}
	return items, total, nil
}

func (s *ProblemStore) GetAllTags(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT DISTINCT unnest(tags) FROM problems ORDER BY 1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		rows.Scan(&tag)
		tags = append(tags, tag)
	}
	if tags == nil {
		tags = []string{}
	}
	return tags, nil
}

func (s *ProblemStore) ListByCreatedBy(ctx context.Context, userID string, offset, limit int) ([]model.ProblemListItem, int, error) {
	var total int
	s.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT p.id) FROM problems p
		 LEFT JOIN problem_permissions perm ON p.id = perm.problem_id
		 WHERE p.created_by = $1 OR perm.user_id = $1`, userID).Scan(&total)

	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT p.id, p.slug, p.title, p.difficulty, COALESCE(p.rating,0), p.tags, p.submission_count, p.accepted_count, p.source, p.created_at, COALESCE(p.status,'draft')
		 FROM problems p
		 LEFT JOIN problem_permissions perm ON p.id = perm.problem_id
		 WHERE p.created_by = $1 OR perm.user_id = $1
		 ORDER BY p.created_at DESC OFFSET $2 LIMIT $3`, userID, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []model.ProblemListItem
	for rows.Next() {
		var item model.ProblemListItem
		var tags []string
		var createdAt interface{}
		rows.Scan(&item.ID, &item.Slug, &item.Title, &item.Difficulty, &item.Rating, pq.Array(&tags),
			&item.SubmissionCount, &item.AcceptedCount, &item.Source, &createdAt, &item.Status)
		item.Tags = tags
		items = append(items, item)
	}
	if items == nil {
		items = []model.ProblemListItem{}
	}
	return items, total, nil
}

// ListSelectable is the contest problem picker's search: the public archive plus
// everything the caller may attach — their own problems, and any problem they
// hold a permission row on. See selectableProblemSQL.
func (s *ProblemStore) ListSelectable(ctx context.Context, userID string, offset, limit int, search string) ([]model.ProblemListItem, int, error) {
	where := []string{selectableProblemSQL}
	args := []interface{}{userID}
	argIdx := 2

	if search != "" {
		where = append(where, fmt.Sprintf(
			"(p.search_vector @@ plainto_tsquery('simple', $%d) OR p.title ILIKE $%d OR p.slug ILIKE $%d)",
			argIdx, argIdx+1, argIdx+2))
		args = append(args, search, "%"+search+"%", "%"+search+"%")
		argIdx += 3
	}

	whereClause := strings.Join(where, " AND ")

	var total int
	s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM problems p WHERE "+whereClause, args...).Scan(&total)

	selectQuery := fmt.Sprintf(`SELECT p.id, p.slug, p.title, p.difficulty, COALESCE(p.rating,0), p.tags,
		p.submission_count, p.accepted_count, p.source
		FROM problems p WHERE %s ORDER BY p.visible ASC, p.created_at DESC OFFSET $%d LIMIT $%d`,
		whereClause, argIdx, argIdx+1)
	args = append(args, offset, limit)

	rows, err := s.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.ProblemListItem
	for rows.Next() {
		var item model.ProblemListItem
		var tags []string
		rows.Scan(&item.ID, &item.Slug, &item.Title, &item.Difficulty, &item.Rating, pq.Array(&tags),
			&item.SubmissionCount, &item.AcceptedCount, &item.Source)
		item.Tags = tags
		items = append(items, item)
	}
	if items == nil {
		items = []model.ProblemListItem{}
	}
	return items, total, nil
}

// ListAccessLevels returns the user's access level for each requested problem.
// Problems with no permission row are absent from the map.
func (s *ProblemStore) ListAccessLevels(ctx context.Context, userID string, problemIDs []string) (map[string]string, error) {
	levels := make(map[string]string, len(problemIDs))
	if userID == "" || len(problemIDs) == 0 {
		return levels, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT problem_id::text, access_level FROM problem_permissions
		  WHERE user_id = $1 AND problem_id = ANY($2)`,
		userID, pq.Array(problemIDs))
	if err != nil {
		return levels, err
	}
	defer rows.Close()
	for rows.Next() {
		var problemID, level string
		if err := rows.Scan(&problemID, &level); err != nil {
			return levels, err
		}
		levels[problemID] = level
	}
	return levels, rows.Err()
}
