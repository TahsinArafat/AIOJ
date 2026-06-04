package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

type ContestStore struct{ db *sql.DB }

func NewContestStore(db *sql.DB) *ContestStore { return &ContestStore{db: db} }

func (s *ContestStore) Create(ctx context.Context, c *model.Contest) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var configJSON []byte
	if c.EducationalConfig != nil {
		configJSON, _ = json.Marshal(c.EducationalConfig)
	}

	var formatConfigJSON []byte
	if len(c.FormatConfig) > 0 {
		formatConfigJSON = c.FormatConfig
	} else {
		formatConfigJSON = []byte("{}")
	}

	var slug interface{}
	if c.Slug != "" {
		slug = c.Slug
	}

	err = tx.QueryRowContext(ctx,
		`INSERT INTO contests(id,slug,title,type,format,format_config,start_time,end_time,freeze_time,password,visible,description,
		                    registration_required,registration_deadline,max_participants,division,educational_config,created_by)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18) RETURNING created_at`,
		c.ID, slug, c.Title, c.Type, c.Format, formatConfigJSON, c.StartTime, c.EndTime, c.FreezeTime, c.Password, c.Visible, c.Description,
		c.RegistrationRequired, c.RegistrationDeadline, c.MaxParticipants, c.Division, configJSON, c.CreatedBy,
	).Scan(&c.CreatedAt)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO contest_permissions(contest_id, user_id, access_level) VALUES($1,$2,'manager')`,
		c.ID, c.CreatedBy)
	if err != nil {
		return err
	}
	if c.GroupID != "" {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO group_contests (group_id, contest_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			c.GroupID, c.ID)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *ContestStore) GetByID(ctx context.Context, id string) (*model.Contest, error) {
	if _, err := uuid.Parse(id); err != nil {
		return s.GetBySlug(ctx, id)
	}
	return s.getByWhere(ctx, "id=$1", id)
}

func (s *ContestStore) GetBySlug(ctx context.Context, slug string) (*model.Contest, error) {
	// Try numeric display_id first
	if num, err := strconv.Atoi(slug); err == nil {
		if c, err := s.getByWhere(ctx, "display_id=$1", num); err == nil && c != nil {
			return c, nil
		}
	}
	return s.getByWhere(ctx, "slug=$1", slug)
}

func (s *ContestStore) getByWhere(ctx context.Context, where string, args ...interface{}) (*model.Contest, error) {
	var c model.Contest
	var configJSON []byte
	var formatConfigJSON []byte
	var slug sql.NullString
	var groupID sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id,display_id,slug,title,type,format,format_config,start_time,end_time,freeze_time,password,visible,description,
		        registration_required,registration_deadline,max_participants,division,educational_config,
		        upsolving_enabled,virtual_contest_enabled,pdf_enabled,statement_hidden,created_by,created_at,
		        (SELECT group_id::text FROM group_contests WHERE contest_id = id LIMIT 1) AS group_id
		 FROM contests WHERE `+where, args...).Scan(
		&c.ID, &c.DisplayID, &slug, &c.Title, &c.Type, &c.Format, &formatConfigJSON, &c.StartTime, &c.EndTime, &c.FreezeTime,
		&c.Password, &c.Visible, &c.Description,
		&c.RegistrationRequired, &c.RegistrationDeadline, &c.MaxParticipants,
		&c.Division, &configJSON,
		&c.UpsolvingEnabled, &c.VirtualContestEnabled, &c.PDFEnabled, &c.StatementHidden,
		&c.CreatedBy, &c.CreatedAt, &groupID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if groupID.Valid {
		c.GroupID = groupID.String
	}
	if slug.Valid {
		c.Slug = slug.String
	}
	if len(configJSON) > 0 {
		var config model.EducationalRoundConfig
		if err := json.Unmarshal(configJSON, &config); err == nil {
			c.EducationalConfig = &config
		}
	}
	if len(formatConfigJSON) > 0 {
		c.FormatConfig = formatConfigJSON
	}
	if c.Password != "" {
		c.HasPassword = true
	}
	return &c, nil
}

func (s *ContestStore) List(ctx context.Context, offset, limit int) ([]model.Contest, int, error) {
	return s.ListWithDivision(ctx, offset, limit, nil)
}

func (s *ContestStore) ListWithDivision(ctx context.Context, offset, limit int, division *int) ([]model.Contest, int, error) {
	var total int
	countQuery := "SELECT COUNT(*) FROM contests WHERE visible=true"
	args := []interface{}{}
	argIdx := 1

	if division != nil {
		countQuery += fmt.Sprintf(" AND division = $%d", argIdx)
		args = append(args, *division)
		argIdx++
	}
	s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)

	rowsQuery := `SELECT id,display_id,slug,title,type,format,format_config,start_time,end_time,freeze_time,password,visible,description,
	              registration_required,registration_deadline,max_participants,division,educational_config,
	              created_at
	              FROM contests WHERE visible=true`
	if division != nil {
		rowsQuery += fmt.Sprintf(" AND division = $%d", argIdx)
		argIdx++
	}
	rowsQuery += fmt.Sprintf(" ORDER BY start_time DESC OFFSET $%d LIMIT $%d", argIdx, argIdx+1)
	args = append(args, offset, limit)

	rows, err := s.db.QueryContext(ctx, rowsQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []model.Contest
	for rows.Next() {
		var c model.Contest
		var configJSON []byte
		var formatConfigJSON []byte
		var slug sql.NullString
		rows.Scan(&c.ID, &c.DisplayID, &slug, &c.Title, &c.Type, &c.Format, &formatConfigJSON, &c.StartTime, &c.EndTime, &c.FreezeTime, &c.Password, &c.Visible, &c.Description,
			&c.RegistrationRequired, &c.RegistrationDeadline, &c.MaxParticipants, &c.Division, &configJSON, &c.CreatedAt)
		if slug.Valid {
			c.Slug = slug.String
		}
		if len(configJSON) > 0 {
			var config model.EducationalRoundConfig
			if err := json.Unmarshal(configJSON, &config); err == nil {
				c.EducationalConfig = &config
			}
		}
		if len(formatConfigJSON) > 0 {
			c.FormatConfig = formatConfigJSON
		}
		if c.Password != "" {
			c.HasPassword = true
		}
		items = append(items, c)
	}
	if items == nil {
		items = []model.Contest{}
	}
	return items, total, nil
}

func (s *ContestStore) Update(ctx context.Context, c *model.Contest) error {
	var formatConfigJSON []byte
	if len(c.FormatConfig) > 0 {
		formatConfigJSON = c.FormatConfig
	} else {
		formatConfigJSON = []byte("{}")
	}

	var slug interface{}
	if c.Slug != "" {
		slug = c.Slug
	}

	_, err := s.db.ExecContext(ctx,
		`UPDATE contests SET title=$1, type=$2, format=$3, format_config=$4, start_time=$5, end_time=$6,
	 freeze_time=$7, password=$8, description=$9, visible=$10, pdf_enabled=$11, statement_hidden=$12,
	 slug=$14, upsolving_enabled=$15, virtual_contest_enabled=$16
	 WHERE id=$13`,
		c.Title, c.Type, c.Format, formatConfigJSON, c.StartTime, c.EndTime, c.FreezeTime, c.Password, c.Description, c.Visible, c.PDFEnabled, c.StatementHidden, c.ID, slug, c.UpsolvingEnabled, c.VirtualContestEnabled)
	if err != nil {
		return err
	}

	_, _ = s.db.ExecContext(ctx, `DELETE FROM group_contests WHERE contest_id = $1`, c.ID)
	if c.GroupID != "" {
		_, _ = s.db.ExecContext(ctx, `INSERT INTO group_contests (group_id, contest_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, c.GroupID, c.ID)
	}

	return nil
}

func (s *ContestStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM contests WHERE id=$1`, id)
	return err
}

func (s *ContestStore) AddProblem(ctx context.Context, contestID, problemID, index string, score, sortOrder int) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO contest_problems(contest_id,problem_id,index,score,sort_order) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`,
		contestID, problemID, index, score, sortOrder)
	return err
}

func (s *ContestStore) UpdateProblem(ctx context.Context, contestID, problemID, index string, score, sortOrder int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE contest_problems SET index=$3, score=$4, sort_order=$5 WHERE contest_id=$1 AND problem_id=$2`,
		contestID, problemID, index, score, sortOrder)
	return err
}

func (s *ContestStore) RemoveProblem(ctx context.Context, contestID, problemID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM contest_problems WHERE contest_id=$1 AND problem_id=$2`,
		contestID, problemID)
	return err
}

func (s *ContestStore) GetProblems(ctx context.Context, contestID string) ([]model.ContestProblem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT cp.contest_id,cp.problem_id,cp.index,cp.score,cp.sort_order,COALESCE(p.title,''),COALESCE(p.slug,'')
		 FROM contest_problems cp LEFT JOIN problems p ON p.id=cp.problem_id WHERE cp.contest_id=$1 ORDER BY cp.sort_order`, contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []model.ContestProblem
	for rows.Next() {
		var cp model.ContestProblem
		rows.Scan(&cp.ContestID, &cp.ProblemID, &cp.Index, &cp.Score, &cp.SortOrder, &cp.Title, &cp.Slug)
		items = append(items, cp)
	}
	return items, nil
}

func (s *ContestStore) GetScoreboardRows(ctx context.Context, contestID string, beforeTime *time.Time) ([]ScoreboardRow, error) {
	query := `SELECT s.user_id, s.problem_id, s.status, s.score, s.created_at
			  FROM submissions s WHERE s.contest_id=$1`
	args := []interface{}{contestID}
	if beforeTime != nil {
		query += " AND s.created_at < $2"
		args = append(args, *beforeTime)
	}
	query += " ORDER BY s.created_at ASC"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ScoreboardRow
	for rows.Next() {
		var r ScoreboardRow
		rows.Scan(&r.UserID, &r.ProblemID, &r.Status, &r.Score, &r.CreatedAt)
		result = append(result, r)
	}
	return result, nil
}

func (s *ContestStore) GetParticipants(ctx context.Context, contestID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT user_id FROM submissions WHERE contest_id=$1`, contestID)
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

func (s *ContestStore) GetUsername(ctx context.Context, userID string) string {
	var username string
	s.db.QueryRowContext(ctx, "SELECT username FROM users WHERE id=$1", userID).Scan(&username)
	return username
}

type ScoreboardRow struct {
	UserID    string
	ProblemID string
	Status    string
	Score     int
	CreatedAt time.Time
}

func (s *ContestStore) AddPermission(ctx context.Context, contestID, userID, accessLevel string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO contest_permissions(contest_id, user_id, access_level) VALUES($1,$2,$3)
		 ON CONFLICT(contest_id, user_id) DO UPDATE SET access_level=$3`, contestID, userID, accessLevel)
	return err
}

func (s *ContestStore) RemovePermission(ctx context.Context, contestID, userID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM contest_permissions WHERE contest_id=$1 AND user_id=$2", contestID, userID)
	return err
}

func (s *ContestStore) GetPermissions(ctx context.Context, contestID string) ([]model.ContestPermission, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT p.contest_id, p.user_id, p.access_level, u.username 
		 FROM contest_permissions p JOIN users u ON p.user_id = u.id WHERE p.contest_id=$1`, contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []model.ContestPermission
	for rows.Next() {
		var cp model.ContestPermission
		rows.Scan(&cp.ContestID, &cp.UserID, &cp.AccessLevel, &cp.Username)
		items = append(items, cp)
	}
	if items == nil {
		items = []model.ContestPermission{}
	}
	return items, nil
}

func (s *ContestStore) HasAccess(ctx context.Context, contestID, userID string, requiredLevels ...string) bool {
	var level string
	err := s.db.QueryRowContext(ctx, "SELECT access_level FROM contest_permissions WHERE contest_id=$1 AND user_id=$2", contestID, userID).Scan(&level)
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

func (s *ContestStore) RegisterTeam(ctx context.Context, contestID, teamID string) (*model.TeamRegistration, error) {
	var reg model.TeamRegistration
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO team_registrations(contest_id, team_id)
		 VALUES($1, $2)
		 ON CONFLICT(contest_id, team_id) DO NOTHING
		 RETURNING id, contest_id, team_id, registered_at`,
		contestID, teamID).Scan(&reg.ID, &reg.ContestID, &reg.TeamID, &reg.RegisteredAt)
	if err != nil {
		return nil, err
	}
	return &reg, nil
}

func (s *ContestStore) ListTeamRegistrations(ctx context.Context, contestID string) ([]model.TeamRegistration, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT tr.id, tr.contest_id, tr.team_id, t.name, tr.registered_at
		 FROM team_registrations tr
		 JOIN teams t ON tr.team_id = t.id
		 WHERE tr.contest_id=$1
		 ORDER BY tr.registered_at`,
		contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []model.TeamRegistration
	for rows.Next() {
		var tr model.TeamRegistration
		rows.Scan(&tr.ID, &tr.ContestID, &tr.TeamID, &tr.TeamName, &tr.RegisteredAt)
		items = append(items, tr)
	}
	if items == nil {
		items = []model.TeamRegistration{}
	}
	return items, nil
}

// GetContestProblemByIndex returns a problem from a contest by its index (A, B, C...).
// Unlike GetBySlug/GetByID, this does NOT check problem.visible - the problem is
// accessible if it's in the contest, regardless of its visibility setting.
func (s *ContestStore) GetContestProblemByIndex(ctx context.Context, contestID, index string) (*model.Problem, error) {
	query := `
		SELECT p.id, p.slug, p.title, p.description, p.input_format, p.output_format,
		       p.hint, p.time_limit, p.memory_limit, p.difficulty, p.tags,
		       p.sample_cases,
		       p.visible, p.spj, p.spj_language, p.spj_source_code, p.checker_type,
		       p.float_epsilon, p.interactive, p.interactor_language, p.interactor_source_code,
		       p.scoring_mode, p.subtask_aggregation, p.submission_count, p.accepted_count,
		       p.source, p.remote_id, p.created_by, p.created_at, p.updated_at
		FROM problems p
		JOIN contest_problems cp ON p.id = cp.problem_id
		WHERE cp.contest_id = $1 AND cp.index = $2`

	var p model.Problem
	var tags []byte
	var samples []byte
	err := s.db.QueryRowContext(ctx, query, contestID, index).Scan(
		&p.ID, &p.Slug, &p.Title, &p.Description, &p.InputFormat, &p.OutputFormat,
		&p.Hint, &p.TimeLimit, &p.MemoryLimit, &p.Difficulty, &tags,
		&samples,
		&p.Visible, &p.SPJ, &p.SPJLanguage, &p.SPJSourceCode, &p.CheckerType,
		&p.FloatEpsilon, &p.Interactive, &p.InteractorLanguage, &p.InteractorSourceCode,
		&p.ScoringMode, &p.SubtaskAggregation, &p.SubmissionCount, &p.AcceptedCount,
		&p.Source, &p.RemoteID, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get contest problem by index: %w", err)
	}

	if len(tags) > 0 {
		if err := json.Unmarshal(tags, &p.Tags); err != nil {
			p.Tags = []string{}
		}
	}

	if len(samples) > 0 {
		if err := json.Unmarshal(samples, &p.SampleCases); err != nil {
			p.SampleCases = []model.SampleCase{}
		}
	}

	return &p, nil
}

// IsParticipant checks if a user has participated in a contest (has submissions or registrations).
func (s *ContestStore) IsParticipant(ctx context.Context, contestID, userID string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM submissions WHERE contest_id = $1 AND user_id = $2
			UNION
			SELECT 1 FROM contest_registrations WHERE contest_id = $1 AND user_id = $2
		)`,
		contestID, userID,
	).Scan(&exists)
	return exists, err
}

// ContestProblemStats holds per-problem statistics for a contest.
type ContestProblemStats struct {
	ProblemID      string  `json:"problem_id"`
	Index          string  `json:"index"`
	Title          string  `json:"title"`
	TotalSubs      int     `json:"total_submissions"`
	Accepted       int     `json:"accepted"`
	SolveRate      float64 `json:"solve_rate"`
}

// ContestStats holds aggregate statistics for a contest.
type ContestStats struct {
	TotalParticipants    int                          `json:"total_participants"`
	TotalSubmissions     int                          `json:"total_submissions"`
	AcceptedSubmissions  int                          `json:"accepted_submissions"`
	Problems             []ContestProblemStats        `json:"problems"`
	Languages            map[string]int               `json:"languages"`
	Verdicts             map[string]int               `json:"verdicts"`
}

// GetContestStats returns aggregate statistics for a contest.
func (s *ContestStore) GetContestStats(ctx context.Context, contestID string) (*ContestStats, error) {
	stats := &ContestStats{
		Languages: make(map[string]int),
		Verdicts:  make(map[string]int),
	}

	// Total participants: count distinct users who submitted in this contest.
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT user_id) FROM submissions WHERE contest_id=$1`, contestID,
	).Scan(&stats.TotalParticipants)
	if err != nil {
		return nil, fmt.Errorf("count participants: %w", err)
	}

	// Total submissions and accepted submissions.
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FILTER (WHERE status='ac'), COUNT(*)
		 FROM submissions WHERE contest_id=$1`, contestID,
	).Scan(&stats.AcceptedSubmissions, &stats.TotalSubmissions)
	if err != nil {
		return nil, fmt.Errorf("count submissions: %w", err)
	}

	// Per-problem stats.
	probRows, err := s.db.QueryContext(ctx,
		`SELECT cp.problem_id, cp.index, p.title,
		        COUNT(s.id) AS total_subs,
		        COUNT(s.id) FILTER (WHERE s.status='ac') AS accepted
		 FROM contest_problems cp
		 JOIN problems p ON cp.problem_id = p.id
		 LEFT JOIN submissions s ON s.contest_id = cp.contest_id AND s.problem_id = cp.problem_id
		 WHERE cp.contest_id = $1
		 GROUP BY cp.problem_id, cp.index, p.title, cp.sort_order
		 ORDER BY cp.sort_order`, contestID,
	)
	if err != nil {
		return nil, fmt.Errorf("query problem stats: %w", err)
	}
	defer probRows.Close()
	for probRows.Next() {
		var ps ContestProblemStats
		if err := probRows.Scan(&ps.ProblemID, &ps.Index, &ps.Title, &ps.TotalSubs, &ps.Accepted); err != nil {
			return nil, fmt.Errorf("scan problem stats: %w", err)
		}
		if ps.TotalSubs > 0 {
			ps.SolveRate = float64(ps.Accepted) / float64(ps.TotalSubs)
		}
		stats.Problems = append(stats.Problems, ps)
	}
	if stats.Problems == nil {
		stats.Problems = []ContestProblemStats{}
	}

	// Submissions grouped by language.
	langRows, err := s.db.QueryContext(ctx,
		`SELECT language, COUNT(*) FROM submissions WHERE contest_id=$1 GROUP BY language`, contestID,
	)
	if err != nil {
		return nil, fmt.Errorf("query language stats: %w", err)
	}
	defer langRows.Close()
	for langRows.Next() {
		var lang string
		var count int
		if err := langRows.Scan(&lang, &count); err != nil {
			return nil, fmt.Errorf("scan language stats: %w", err)
		}
		stats.Languages[lang] = count
	}

	// Submissions grouped by status (verdict).
	verdictRows, err := s.db.QueryContext(ctx,
		`SELECT status, COUNT(*) FROM submissions WHERE contest_id=$1 GROUP BY status`, contestID,
	)
	if err != nil {
		return nil, fmt.Errorf("query verdict stats: %w", err)
	}
	defer verdictRows.Close()
	for verdictRows.Next() {
		var status string
		var count int
		if err := verdictRows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan verdict stats: %w", err)
		}
		stats.Verdicts[status] = count
	}

	return stats, nil
}

func (s *ContestStore) CheckGroupRestriction(ctx context.Context, contestID, userID string) (bool, error) {
	var restricted bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM group_contests WHERE contest_id = $1)`, contestID).Scan(&restricted)
	if err != nil || !restricted {
		return true, err
	}
	var allowed bool
	err = s.db.QueryRowContext(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM group_members gm
			JOIN group_contests gc ON gm.group_id = gc.group_id
			WHERE gc.contest_id = $1 AND gm.user_id = $2
		)`, contestID, userID).Scan(&allowed)
	return allowed, err
}

func (s *ContestStore) ListByCreatedBy(ctx context.Context, userID string, offset, limit int) ([]model.Contest, int, error) {
	var total int
	s.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT c.id) FROM contests c
		 LEFT JOIN contest_permissions perm ON c.id = perm.contest_id
		 WHERE c.created_by = $1 OR perm.user_id = $1`, userID).Scan(&total)

	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT c.id, c.display_id, c.slug, c.title, c.type, c.format, c.format_config, c.start_time, c.end_time, c.freeze_time, c.password, c.visible, c.description,
		                 c.registration_required, c.registration_deadline, c.max_participants, c.division, c.educational_config,
		                 c.created_at
		 FROM contests c
		 LEFT JOIN contest_permissions perm ON c.id = perm.contest_id
		 WHERE c.created_by = $1 OR perm.user_id = $1
		 ORDER BY c.start_time DESC OFFSET $2 LIMIT $3`, userID, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []model.Contest
	for rows.Next() {
		var c model.Contest
		var configJSON []byte
		var formatConfigJSON []byte
		var slug sql.NullString
		rows.Scan(&c.ID, &c.DisplayID, &slug, &c.Title, &c.Type, &c.Format, &formatConfigJSON, &c.StartTime, &c.EndTime, &c.FreezeTime, &c.Password, &c.Visible, &c.Description,
			&c.RegistrationRequired, &c.RegistrationDeadline, &c.MaxParticipants, &c.Division, &configJSON, &c.CreatedAt)
		if slug.Valid {
			c.Slug = slug.String
		}
		if len(configJSON) > 0 {
			var config model.EducationalRoundConfig
			if err := json.Unmarshal(configJSON, &config); err == nil {
				c.EducationalConfig = &config
			}
		}
		if len(formatConfigJSON) > 0 {
			c.FormatConfig = formatConfigJSON
		}
		if c.Password != "" {
			c.HasPassword = true
		}
		items = append(items, c)
	}
	if items == nil {
		items = []model.Contest{}
	}
	return items, total, nil
}
