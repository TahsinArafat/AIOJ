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

	samples, err := json.Marshal(p.SampleCases)
	if err != nil {
		return fmt.Errorf("marshal sample_cases: %w", err)
	}
	scores, err := json.Marshal(p.TestCaseScore)
	if err != nil {
		return fmt.Errorf("marshal testcase_score: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx, `INSERT INTO problems
		(id,slug,title,description,input_format,output_format,hint,sample_cases,
		 time_limit,memory_limit,difficulty,tags,visible,testdata_path,testcase_score,
		 spj,spj_language,spj_source_code,spj_version,source,remote_id,created_by,
		 checker_type,float_epsilon,interactive,interactor_language,interactor_source_code,
		 scoring_mode,subtask_aggregation,ai_generated)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30)
		RETURNING created_at,updated_at`,
		p.ID, p.Slug, p.Title, p.Description, p.InputFormat, p.OutputFormat, p.Hint, samples,
		p.TimeLimit, p.MemoryLimit, p.Difficulty, pq.Array(p.Tags), p.Visible, p.TestdataPath, scores,
		p.SPJ, p.SPJLanguage, p.SPJSourceCode, p.SPJVersion, p.Source, p.RemoteID, p.CreatedBy,
		p.CheckerType, p.FloatEpsilon, p.Interactive, p.InteractorLanguage, p.InteractorSourceCode,
		p.ScoringMode, p.SubtaskAggregation, p.AIGenerated,
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

func (s *ProblemStore) getBy(ctx context.Context, field, value string) (*model.Problem, error) {
	var p model.Problem
	var samples, scores []byte
	var tags []string
	err := s.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT
		id,slug,title,description,input_format,output_format,hint,sample_cases,
		time_limit,memory_limit,difficulty,tags,visible,testdata_path,testcase_score,
		spj,spj_language,spj_source_code,spj_version,checker_type,float_epsilon,submission_count,accepted_count,
		source,remote_id,created_by,created_at,updated_at,
		interactive,COALESCE(interactor_language,''),COALESCE(interactor_source_code,''),
		COALESCE(scoring_mode,'complete'),COALESCE(subtask_aggregation,'min'),
		COALESCE(ai_generated,false) FROM problems WHERE %s=$1`, field), value).Scan(
		&p.ID, &p.Slug, &p.Title, &p.Description, &p.InputFormat, &p.OutputFormat, &p.Hint, &samples,
		&p.TimeLimit, &p.MemoryLimit, &p.Difficulty, pq.Array(&tags), &p.Visible, &p.TestdataPath, &scores,
		&p.SPJ, &p.SPJLanguage, &p.SPJSourceCode, &p.SPJVersion, &p.CheckerType, &p.FloatEpsilon, &p.SubmissionCount, &p.AcceptedCount,
		&p.Source, &p.RemoteID, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
		&p.Interactive, &p.InteractorLanguage, &p.InteractorSourceCode,
		&p.ScoringMode, &p.SubtaskAggregation, &p.AIGenerated)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	json.Unmarshal(samples, &p.SampleCases)
	json.Unmarshal(scores, &p.TestCaseScore)
	p.Tags = tags
	return &p, nil
}

func (s *ProblemStore) List(ctx context.Context, offset, limit int) ([]model.ProblemListItem, int, error) {
	var total int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM problems WHERE visible=true`).Scan(&total)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,slug,title,difficulty,tags,submission_count,accepted_count,source
		 FROM problems WHERE visible=true ORDER BY created_at DESC OFFSET $1 LIMIT $2`, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []model.ProblemListItem
	for rows.Next() {
		var item model.ProblemListItem
		var tags []string
		rows.Scan(&item.ID, &item.Slug, &item.Title, &item.Difficulty, pq.Array(&tags),
			&item.SubmissionCount, &item.AcceptedCount, &item.Source)
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
	_, err = s.db.ExecContext(ctx, `UPDATE problems SET
		title=$2, description=$3, input_format=$4, output_format=$5, hint=$6, sample_cases=$7,
		time_limit=$8, memory_limit=$9, difficulty=$10, tags=$11, visible=$12,
		testcase_score=$13, spj=$14, spj_language=$15, spj_source_code=$16, checker_type=$17, float_epsilon=$18,
		interactive=$19, interactor_language=$20, interactor_source_code=$21,
		scoring_mode=$22, subtask_aggregation=$23, updated_at=NOW() WHERE id=$1`,
		id, p.Title, p.Description, p.InputFormat, p.OutputFormat, p.Hint, samples,
		p.TimeLimit, p.MemoryLimit, p.Difficulty, pq.Array(p.Tags), p.Visible,
		scores, p.SPJ, p.SPJLanguage, p.SPJSourceCode, p.CheckerType, p.FloatEpsilon,
		p.Interactive, p.InteractorLanguage, p.InteractorSourceCode,
		p.ScoringMode, p.SubtaskAggregation)
	return err
}

func (s *ProblemStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM problems WHERE id=$1", id)
	return err
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

func (s *ProblemStore) ListWithFilter(ctx context.Context, offset, limit int, difficulty string, tags []string, search string, source string, rating string, sortBy string) ([]model.ProblemListItem, int, error) {
	where := []string{"p.visible = true"}
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
		where = append(where, fmt.Sprintf("(p.title ILIKE $%d OR p.slug ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+search+"%")
		argIdx++
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

	selectQuery := fmt.Sprintf(`SELECT p.id, p.slug, p.title, p.difficulty, p.tags, p.submission_count, p.accepted_count, p.source
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
		rows.Scan(&item.ID, &item.Slug, &item.Title, &item.Difficulty, pq.Array(&tagArr),
			&item.SubmissionCount, &item.AcceptedCount, &item.Source)
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
		`SELECT DISTINCT p.id, p.slug, p.title, p.difficulty, p.tags, p.submission_count, p.accepted_count, p.source, p.created_at
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
		rows.Scan(&item.ID, &item.Slug, &item.Title, &item.Difficulty, pq.Array(&tags),
			&item.SubmissionCount, &item.AcceptedCount, &item.Source, &createdAt)
		item.Tags = tags
		items = append(items, item)
	}
	if items == nil {
		items = []model.ProblemListItem{}
	}
	return items, total, nil
}
