package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
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

	// A caller that predates the review workflow leaves Status empty; treat that
	// as a draft rather than sending '' to a CHECK constraint, and keep visible
	// consistent with it — publishing is an admin decision.
	if c.Status == "" {
		c.Status = model.ContestStatusDraft
	}
	c.Visible = c.Status == model.ContestStatusPublished
	// The column is NOT NULL, so a caller that predates it must not write SQL NULL.
	// Normalising at the store rather than in each handler keeps the default in one
	// place. Only invalid entries are dropped; an empty map is a valid config and
	// means "every surface inherits the base gate".
	accessConfigJSON, err := json.Marshal(model.NormalizeAccessConfig(c.AccessConfig))
	if err != nil {
		return fmt.Errorf("marshal access_config: %w", err)
	}

	err = tx.QueryRowContext(ctx,
		`INSERT INTO contests(id,slug,title,type,format,format_config,start_time,end_time,freeze_time,password_hash,visible,description,
		                    registration_required,registration_deadline,max_participants,division,educational_config,created_by,
		                    status,is_rated,rated_for_min,rated_for_max,access_config,listed)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24) RETURNING created_at`,
		c.ID, slug, c.Title, c.Type, c.Format, formatConfigJSON, c.StartTime, c.EndTime, c.FreezeTime, c.PasswordHash, c.Visible, c.Description,
		c.RegistrationRequired, c.RegistrationDeadline, c.MaxParticipants, c.Division, configJSON, c.CreatedBy,
		c.Status, c.IsRated, c.RatedForMin, c.RatedForMax, accessConfigJSON, c.Listed,
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
	var accessConfigJSON []byte
	var slug sql.NullString
	var groupID sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id,display_id,slug,title,type,format,format_config,start_time,end_time,freeze_time,password_hash,visible,description,
		        registration_required,registration_deadline,max_participants,division,educational_config,
		        upsolving_enabled,virtual_contest_enabled,pdf_enabled,statement_hidden,created_by,created_at,
		        COALESCE(status,'draft'), COALESCE(review_note,''),
		        COALESCE(is_rated,true), COALESCE(rated_for_min,0), COALESCE(rated_for_max,9999),
		        access_config, listed,
		        (SELECT group_id::text FROM group_contests WHERE contest_id = id LIMIT 1) AS group_id
		 FROM contests WHERE `+where, args...).Scan(
		&c.ID, &c.DisplayID, &slug, &c.Title, &c.Type, &c.Format, &formatConfigJSON, &c.StartTime, &c.EndTime, &c.FreezeTime,
		&c.PasswordHash, &c.Visible, &c.Description,
		&c.RegistrationRequired, &c.RegistrationDeadline, &c.MaxParticipants,
		&c.Division, &configJSON,
		&c.UpsolvingEnabled, &c.VirtualContestEnabled, &c.PDFEnabled, &c.StatementHidden,
		&c.CreatedBy, &c.CreatedAt, &c.Status, &c.ReviewNote,
		&c.IsRated, &c.RatedForMin, &c.RatedForMax, &accessConfigJSON, &c.Listed, &groupID)
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
	c.AccessConfig = decodeAccessConfig(accessConfigJSON)
	if c.PasswordHash != "" {
		c.HasPassword = true
	}
	return &c, nil
}

// decodeAccessConfig turns the stored JSONB into a map, always non-nil.
//
// A malformed or dangling value cannot fail a read: this is a settings blob on
// the hot path of every contest page, and a 500 on the contest because somebody
// hand-edited a row would trade a cosmetic problem for an outage. Invalid
// entries are dropped and the result is a map, so every caller can index it
// without a nil check. The single failure mode is a surface falling back to its
// inherited audience, which is the conservative direction — a surface becoming
// MORE open is the one that would be dangerous.
func decodeAccessConfig(raw []byte) model.AccessConfig {
	out := model.AccessConfig{}
	if len(raw) == 0 {
		return out
	}
	var stored model.AccessConfig
	if err := json.Unmarshal(raw, &stored); err != nil {
		return out
	}
	return model.NormalizeAccessConfig(stored)
}

func (s *ContestStore) List(ctx context.Context, offset, limit int) ([]model.Contest, int, error) {
	return s.ListForViewer(ctx, offset, limit, nil, "")
}

// ListForViewer is the catalog listing as one caller sees it. viewerID is ""
// for an anonymous visitor.
//
// A password-protected or group-restricted round is omitted unless the viewer is
// entitled to it. Before this the listing filtered on visible alone, so every
// invite-only round was enumerable by anybody — which is how people found
// contests they had not been invited to.
func (s *ContestStore) ListForViewer(ctx context.Context, offset, limit int, division *int, viewerID string) ([]model.Contest, int, error) {
	visibility := fmt.Sprintf(contestVisibilityPredicate, 1, 1, 1, 1, 1, 1)

	var total int
	countQuery := "SELECT COUNT(*) FROM contests WHERE visible=true AND listed=true AND " + visibility
	args := []interface{}{viewerID}
	argIdx := 2

	if division != nil {
		countQuery += fmt.Sprintf(" AND division = $%d", argIdx)
		args = append(args, *division)
		argIdx++
	}
	s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)

	rowsQuery := `SELECT id,display_id,slug,title,type,format,format_config,start_time,end_time,freeze_time,password_hash,visible,description,
	              registration_required,registration_deadline,max_participants,division,educational_config,
	              created_at,access_config,listed
	              FROM contests WHERE visible=true AND listed=true AND ` + visibility
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
		var accessConfigJSON []byte
		var slug sql.NullString
		rows.Scan(&c.ID, &c.DisplayID, &slug, &c.Title, &c.Type, &c.Format, &formatConfigJSON, &c.StartTime, &c.EndTime, &c.FreezeTime, &c.PasswordHash, &c.Visible, &c.Description,
			&c.RegistrationRequired, &c.RegistrationDeadline, &c.MaxParticipants, &c.Division, &configJSON, &c.CreatedAt, &accessConfigJSON, &c.Listed)
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
		c.AccessConfig = decodeAccessConfig(accessConfigJSON)
		if c.PasswordHash != "" {
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

	// visible is derived from status, never taken from the caller: editing a
	// contest must not be a way to publish one. Same rule as problems.
	if c.Status == "" {
		c.Status = model.ContestStatusDraft
	}
	// Every placeholder must be both referenced and supplied. This statement
	// once carried visible=$10 while also deriving visible from status, which
	// left $10 dangling: Postgres cannot infer the type of a parameter that
	// appears nowhere, so every settings save failed with 500 "update failed".
	//
	// Registration controls and division are written here for the first time.
	// They were accepted on create and then never updated, so a round's entry
	// policy could not be changed after it existed.
	//
	// access_config is a whole-map replace, not a merge: the manage screen renders
	// every surface at once, so a merge would only give a stale tab a way to
	// resurrect an audience another tab had just cleared.
	accessConfigJSON, err := json.Marshal(model.NormalizeAccessConfig(c.AccessConfig))
	if err != nil {
		return fmt.Errorf("marshal access_config: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE contests SET title=$1, type=$2, format=$3, format_config=$4, start_time=$5, end_time=$6,
	 freeze_time=$7, password_hash=$8, description=$9, pdf_enabled=$10, statement_hidden=$11,
	 slug=$13, upsolving_enabled=$14, virtual_contest_enabled=$15,
	 visible=($16::varchar='published'), status=$16, is_rated=$17, rated_for_min=$18, rated_for_max=$19,
	 registration_required=$20, registration_deadline=$21, max_participants=$22, division=$23,
	 access_config=$24, listed=$25
	 WHERE id=$12`,
		c.Title, c.Type, c.Format, formatConfigJSON, c.StartTime, c.EndTime, c.FreezeTime, c.PasswordHash, c.Description,
		c.PDFEnabled, c.StatementHidden, c.ID, slug, c.UpsolvingEnabled, c.VirtualContestEnabled,
		c.Status, c.IsRated, c.RatedForMin, c.RatedForMax,
		c.RegistrationRequired, c.RegistrationDeadline, c.MaxParticipants, c.Division,
		accessConfigJSON, c.Listed)
	if err != nil {
		return err
	}

	_, _ = s.db.ExecContext(ctx, `DELETE FROM group_contests WHERE contest_id = $1`, c.ID)
	if c.GroupID != "" {
		_, _ = s.db.ExecContext(ctx, `INSERT INTO group_contests (group_id, contest_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, c.GroupID, c.ID)
	}

	return nil
}

// SetReviewState records an admin's verdict on a contest. Publishing is the only
// thing that makes a round visible, so a proposal cannot go live by itself.
func (s *ContestStore) SetReviewState(ctx context.Context, id, status, note, reviewerID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE contests
		    SET status=$2, review_note=$3, reviewed_by=NULLIF($4,'')::uuid, reviewed_at=NOW(),
		        visible=($2::varchar='published')
		  WHERE id=$1`,
		id, status, note, reviewerID)
	return err
}

// ListByStatus is the admin contest review queue.
func (s *ContestStore) ListByStatus(ctx context.Context, status string, offset, limit int) ([]model.Contest, int, error) {
	var total int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM contests WHERE status=$1`, status).Scan(&total)

	rows, err := s.db.QueryContext(ctx,
		`SELECT id,display_id,COALESCE(slug,''),title,type,format,format_config,start_time,end_time,freeze_time,password_hash,visible,description,
		        registration_required,registration_deadline,max_participants,division,educational_config,
		        upsolving_enabled,virtual_contest_enabled,pdf_enabled,statement_hidden,created_by,created_at,
		        COALESCE(status,'draft'), COALESCE(review_note,''),
		        COALESCE(is_rated,true), COALESCE(rated_for_min,0), COALESCE(rated_for_max,9999),
		        access_config, listed
		   FROM contests WHERE status=$1 ORDER BY created_at ASC OFFSET $2 LIMIT $3`, status, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.Contest
	for rows.Next() {
		var c model.Contest
		var configJSON, formatConfigJSON []byte
		var accessConfigJSON []byte
		rows.Scan(&c.ID, &c.DisplayID, &c.Slug, &c.Title, &c.Type, &c.Format, &formatConfigJSON,
			&c.StartTime, &c.EndTime, &c.FreezeTime, &c.PasswordHash, &c.Visible, &c.Description,
			&c.RegistrationRequired, &c.RegistrationDeadline, &c.MaxParticipants, &c.Division, &configJSON,
			&c.UpsolvingEnabled, &c.VirtualContestEnabled, &c.PDFEnabled, &c.StatementHidden,
			&c.CreatedBy, &c.CreatedAt, &c.Status, &c.ReviewNote,
			&c.IsRated, &c.RatedForMin, &c.RatedForMax, &accessConfigJSON, &c.Listed)
		c.AccessConfig = decodeAccessConfig(accessConfigJSON)
		items = append(items, c)
	}
	if items == nil {
		items = []model.Contest{}
	}
	return items, total, nil
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

// RenumberProblems rewrites every problem's index and sort_order from its
// position, closing any gap left by a removal.
//
// The order is the existing sort_order with index as the tie-break, so a round
// whose sort_order was never maintained still comes out in the order a reader
// expects. One transaction: a half-applied renumber would leave duplicate or
// missing letters, which is exactly the state this exists to remove.
//
// Returns the new ordering so the caller can answer with what it wrote.
func (s *ContestStore) RenumberProblems(ctx context.Context, contestID string) ([]model.ContestProblem, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx,
		`SELECT problem_id FROM contest_problems
		  WHERE contest_id = $1
		  ORDER BY sort_order, index`, contestID)
	if err != nil {
		return nil, fmt.Errorf("list contest problems: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]model.ContestProblem, 0, len(ids))
	for i, pid := range ids {
		idx := model.ContestProblemIndex(i)
		if _, err := tx.ExecContext(ctx,
			`UPDATE contest_problems SET index = $3, sort_order = $4
			  WHERE contest_id = $1 AND problem_id = $2`,
			contestID, pid, idx, i); err != nil {
			return nil, fmt.Errorf("renumber %s: %w", pid, err)
		}
		out = append(out, model.ContestProblem{ContestID: contestID, ProblemID: pid, Index: idx, SortOrder: i})
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return out, nil
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

// GetProblemProgress returns, for every problem in the contest, how many
// participants have solved it and what this viewer has done with it.
//
// Both halves come from one scan of the round's submissions rather than a query
// per problem: a contest problem list is a single page load and was otherwise
// N+1 twice over.
//
// viewerID may be empty (anonymous), in which case only the round-wide counts
// are populated and My* stay false.
//
// beforeTime applies to the round-wide count only. During a standings freeze the
// public scoreboard hides post-freeze solves, and a live "solved by N" next to it
// would give that away by arithmetic — so the count is cut at the same instant.
// The viewer's own state ignores it on purpose: their own verdicts were never
// secret from them, and hiding a solve they just made would read as a bug.
func (s *ContestStore) GetProblemProgress(ctx context.Context, contestID, viewerID string, beforeTime *time.Time) (map[string]model.ContestProblemProgress, error) {
	var cutoff interface{}
	if beforeTime != nil {
		cutoff = *beforeTime
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT problem_id,
		        COUNT(DISTINCT user_id) FILTER (WHERE status = 'ac' AND ($3::timestamptz IS NULL OR created_at < $3)) AS solved_count,
		        COALESCE(BOOL_OR(status = 'ac') FILTER (WHERE user_id = NULLIF($2,'')::uuid), false) AS my_solved,
		        COALESCE(BOOL_OR(user_id = NULLIF($2,'')::uuid), false) AS my_attempted,
		        COUNT(*) FILTER (WHERE user_id = NULLIF($2,'')::uuid) AS my_submissions
		   FROM submissions
		  WHERE contest_id = $1
		  GROUP BY problem_id`, contestID, viewerID, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]model.ContestProblemProgress)
	for rows.Next() {
		var problemID string
		var p model.ContestProblemProgress
		if err := rows.Scan(&problemID, &p.SolvedCount, &p.MySolved, &p.MyAttempted, &p.MySubmissions); err != nil {
			return nil, err
		}
		out[problemID] = p
	}
	return out, rows.Err()
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

// GetDisplayNames resolves the team label for generated onsite accounts in this
// contest. Accounts minted by the Onsite Teams panel never had their team name
// copied onto users, so a scoreboard that read only users.username showed a
// throwaway handle where the competitor expects their team. The join is the one
// the ICPC feed already uses (submissions.go ListFeed).
//
// One query for the whole page rather than per row: a standings page carries up
// to 500 entries and this used to be N+1 for the username alone.
func (s *ContestStore) GetDisplayNames(ctx context.Context, contestID string, userIDs []string) (map[string]model.ContestDisplayName, error) {
	out := make(map[string]model.ContestDisplayName, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT used_by, team_name, COALESCE(institution,'')
		   FROM onsite_batch_users
		  WHERE contest_id = $1 AND used_by = ANY($2)`,
		contestID, pq.Array(userIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var userID string
		var name model.ContestDisplayName
		if err := rows.Scan(&userID, &name.TeamName, &name.Institution); err != nil {
			return nil, err
		}
		out[userID] = name
	}
	return out, rows.Err()
}

// ScoreboardRow is the model row the standings computation consumes. Aliased
// so the interface can name it without postgres owning a type that crosses the
// store boundary.
type ScoreboardRow = model.ScoreboardRow

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
//
// It selects problemColumns and scans with scanProblem, exactly like the practice
// page, so a problem cannot render differently depending on where it is opened.
// The hand-written copy this replaced had lost `constraints` (so the Constraints
// block never appeared in a contest), read `tags` as raw bytes and JSON-decoded a
// Postgres array literal (so every problem came back untagged), and scanned
// nullable columns into plain strings (so a NULL interactor_language failed the
// whole read).
func (s *ContestStore) GetContestProblemByIndex(ctx context.Context, contestID, index string) (*model.Problem, error) {
	query := fmt.Sprintf(`SELECT %s
		FROM problems p
		JOIN contest_problems cp ON p.id = cp.problem_id
		WHERE cp.contest_id = $1 AND cp.index = $2`, problemColumns)

	var p model.Problem
	if err := scanProblem(s.db.QueryRowContext(ctx, query, contestID, index), &p); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get contest problem by index: %w", err)
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
type ContestProblemStats = model.ContestProblemStats

// ContestStats holds aggregate statistics for a contest.
type ContestStats = model.ContestStats

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
		`SELECT DISTINCT c.id, c.display_id, c.slug, c.title, c.type, c.format, c.format_config, c.start_time, c.end_time, c.freeze_time, c.password_hash, c.visible, c.description,
		                 c.registration_required, c.registration_deadline, c.max_participants, c.division, c.educational_config,
		                 c.created_at, c.access_config, c.listed
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
		var accessConfigJSON []byte
		var slug sql.NullString
		rows.Scan(&c.ID, &c.DisplayID, &slug, &c.Title, &c.Type, &c.Format, &formatConfigJSON, &c.StartTime, &c.EndTime, &c.FreezeTime, &c.PasswordHash, &c.Visible, &c.Description,
			&c.RegistrationRequired, &c.RegistrationDeadline, &c.MaxParticipants, &c.Division, &configJSON, &c.CreatedAt, &accessConfigJSON, &c.Listed)
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
		c.AccessConfig = decodeAccessConfig(accessConfigJSON)
		if c.PasswordHash != "" {
			c.HasPassword = true
		}
		items = append(items, c)
	}
	if items == nil {
		items = []model.Contest{}
	}
	return items, total, nil
}
