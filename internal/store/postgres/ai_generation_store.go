package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tahsinarafat/aioj/internal/model"
	"log/slog"
	"time"
)

type AIGenerationStore struct{ db *sql.DB }

func NewAIGenerationStore(db *sql.DB) *AIGenerationStore { return &AIGenerationStore{db: db} }

func (s *AIGenerationStore) Create(ctx context.Context, j *model.AIGenerationJob) error {
	return s.db.QueryRowContext(ctx, `INSERT INTO ai_generation_jobs
      (actor_id,problem_id,ai_model_id,artifact_type,submitted_brief,normalized_brief,context_snapshot,base_problem_updated_at,connection_snapshot,prompt_version,prompt_hash,retry_of)
      VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id,status,stage,progress,created_at`,
		j.ActorID, j.ProblemID, j.AIModelID, j.ArtifactType, j.SubmittedBrief, j.NormalizedBrief, j.ContextSnapshot, j.BaseProblemUpdatedAt, j.ConnectionSnapshot, j.PromptVersion, j.PromptHash, j.RetryOf).Scan(&j.ID, &j.Status, &j.Stage, &j.Progress, &j.CreatedAt)
}

const aiJobColumns = `id,actor_id,problem_id,ai_model_id,artifact_type,status,stage,progress,submitted_brief,normalized_brief,context_snapshot,base_problem_updated_at,connection_snapshot,prompt_version,prompt_hash,proposal,validation_report,error_code,error_message,prompt_tokens,completion_tokens,latency_ms,retry_of,claim_token,heartbeat_at,cancel_requested_at,created_at,started_at,finished_at,applied_at`

func scanAIJob(row interface{ Scan(...any) error }) (*model.AIGenerationJob, error) {
	var j model.AIGenerationJob
	// Nullable jsonb columns must be scanned into *[]byte so SQL NULL is
	// handled correctly; json.RawMessage ([]byte) cannot receive NULL.
	var proposal, validationReport []byte
	err := row.Scan(
		&j.ID, &j.ActorID, &j.ProblemID, &j.AIModelID, &j.ArtifactType,
		&j.Status, &j.Stage, &j.Progress,
		&j.SubmittedBrief, &j.NormalizedBrief, &j.ContextSnapshot,
		&j.BaseProblemUpdatedAt, &j.ConnectionSnapshot,
		&j.PromptVersion, &j.PromptHash,
		&proposal, &validationReport,
		&j.ErrorCode, &j.ErrorMessage,
		&j.PromptTokens, &j.CompletionTokens, &j.LatencyMS,
		&j.RetryOf, &j.ClaimToken, &j.HeartbeatAt, &j.CancelRequestedAt,
		&j.CreatedAt, &j.StartedAt, &j.FinishedAt, &j.AppliedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	j.Proposal = proposal
	j.ValidationReport = validationReport
	return &j, nil
}
func (s *AIGenerationStore) GetByID(ctx context.Context, id string) (*model.AIGenerationJob, error) {
	return scanAIJob(s.db.QueryRowContext(ctx, `SELECT `+aiJobColumns+` FROM ai_generation_jobs WHERE id=$1`, id))
}
func (s *AIGenerationStore) ListByProblem(ctx context.Context, problemID string, offset, limit int) ([]model.AIGenerationJob, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+aiJobColumns+` FROM ai_generation_jobs WHERE problem_id=$1 ORDER BY created_at DESC OFFSET $2 LIMIT $3`, problemID, offset, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.AIGenerationJob{}
	for rows.Next() {
		j, err := scanAIJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *j)
	}
	return out, rows.Err()
}
func (s *AIGenerationStore) CountActiveByActor(ctx context.Context, actorID string) (running, queued int, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FILTER (WHERE status='running'),COUNT(*) FILTER (WHERE status='queued') FROM ai_generation_jobs WHERE actor_id=$1 AND status IN ('running','queued')`, actorID).Scan(&running, &queued)
	return
}
func (s *AIGenerationStore) ClaimNext(ctx context.Context, token string) (*model.AIGenerationJob, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var id string
	err = tx.QueryRowContext(ctx, `SELECT id FROM ai_generation_jobs WHERE status='queued' ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE ai_generation_jobs SET status='running',stage='drafting',progress=5,claim_token=$2,started_at=NOW(),heartbeat_at=NOW() WHERE id=$1`, id, token)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}
func (s *AIGenerationStore) Heartbeat(ctx context.Context, id, token, stage string, progress int) error {
	_, err := s.db.ExecContext(ctx, `UPDATE ai_generation_jobs SET heartbeat_at=NOW(),stage=$3,progress=$4 WHERE id=$1 AND claim_token=$2 AND status='running'`, id, token, stage, progress)
	return err
}
func (s *AIGenerationStore) Complete(ctx context.Context, id, token string, proposal, validation json.RawMessage, promptTokens, completionTokens, latencyMS int) error {
	res, err := s.db.ExecContext(ctx, `UPDATE ai_generation_jobs SET status='ready',stage='ready',progress=100,proposal=$3,validation_report=$4,prompt_tokens=$5,completion_tokens=$6,latency_ms=$7,finished_at=NOW(),heartbeat_at=NOW() WHERE id=$1 AND claim_token=$2 AND status='running'`, id, token, proposal, validation, promptTokens, completionTokens, latencyMS)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		slog.Error("ai jobs: Complete affected 0 rows — stale claim token or already finished", "job_id", id)
	}
	return nil
}

// SetManifest stores the case manifest a harness run produced and marks the job
// ready for review. A run that stopped on a budget or failed a gate stores the
// manifest too, so the admin can see what was built and why it was rejected.
func (s *AIGenerationStore) SetManifest(ctx context.Context, id, token string, manifest, report json.RawMessage, ready bool, turns int) error {
	status := "validation_failed"
	if ready {
		status = "ready"
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE ai_generation_jobs
		SET status = $3, stage = $3, progress = 100,
		    case_manifest = $4, validation_report = $5, turn_count = $6,
		    harness_version = 'problemgen-v1',
		    finished_at = NOW(), heartbeat_at = NOW()
		WHERE id = $1 AND claim_token = $2 AND status = 'running'`,
		id, token, status, nullableJSON(manifest), nullableJSON(report), turns)
	if err != nil {
		return fmt.Errorf("ai jobs: set manifest: %w", err)
	}
	return nil
}

func (s *AIGenerationStore) Fail(ctx context.Context, id, token, code, message string, validation json.RawMessage) error {
	res, err := s.db.ExecContext(ctx, `UPDATE ai_generation_jobs SET status=CASE WHEN $5 IS NULL THEN 'failed' ELSE 'validation_failed' END,stage='failed',error_code=$3,error_message=$4,validation_report=$5,finished_at=NOW() WHERE id=$1 AND claim_token=$2 AND status='running'`, id, token, code, message, nullableJSON(validation))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		slog.Error("ai jobs: Fail affected 0 rows — stale claim token or already finished", "job_id", id, "code", code)
	}
	return nil
}
func nullableJSON(v json.RawMessage) any {
	if len(v) == 0 {
		return nil
	}
	return v
}
func (s *AIGenerationStore) RequestCancel(ctx context.Context, id, actorID string) (bool, error) {
	r, err := s.db.ExecContext(ctx, `UPDATE ai_generation_jobs SET cancel_requested_at=NOW(),status=CASE WHEN status='queued' THEN 'cancelled' ELSE status END,finished_at=CASE WHEN status='queued' THEN NOW() ELSE finished_at END WHERE id=$1 AND actor_id=$2 AND status IN ('queued','running')`, id, actorID)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n == 1, nil
}
func (s *AIGenerationStore) MarkCancelled(ctx context.Context, id, token string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE ai_generation_jobs SET status='cancelled',stage='cancelled',finished_at=NOW() WHERE id=$1 AND claim_token=$2`, id, token)
	return err
}
func (s *AIGenerationStore) IsCancellationRequested(ctx context.Context, id string) (bool, error) {
	var v bool
	err := s.db.QueryRowContext(ctx, `SELECT cancel_requested_at IS NOT NULL FROM ai_generation_jobs WHERE id=$1`, id).Scan(&v)
	return v, err
}
func (s *AIGenerationStore) MarkDiscarded(ctx context.Context, id, actorID string) (bool, error) {
	r, err := s.db.ExecContext(ctx, `UPDATE ai_generation_jobs SET status='discarded',finished_at=COALESCE(finished_at,NOW()) WHERE id=$1 AND actor_id=$2 AND status IN ('ready','validation_failed','failed')`, id, actorID)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n == 1, nil
}
func (s *AIGenerationStore) MarkApplied(ctx context.Context, id string) error {
	r, err := s.db.ExecContext(ctx, `UPDATE ai_generation_jobs SET status='applied',applied_at=NOW() WHERE id=$1 AND status='ready'`, id)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return errors.New("job was already applied or is not ready")
	}
	return nil
}
func (s *AIGenerationStore) RequeueStale(ctx context.Context, older time.Duration) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `UPDATE ai_generation_jobs SET status='queued',stage='queued',progress=0,claim_token=NULL,heartbeat_at=NULL,started_at=NULL WHERE status='running' AND heartbeat_at < NOW() - ($1 * INTERVAL '1 second') RETURNING id`, int(older.Seconds()))
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

// PurgeCompletedBefore deletes terminal jobs older than cutoff.
//
// Only terminal states are eligible: 'queued' and 'running' rows are skipped
// regardless of age, so retention can never delete a job out from under the
// worker that holds its claim. Returns the number of rows removed.
func (s *AIGenerationStore) PurgeCompletedBefore(ctx context.Context, cutoff time.Time) (int, error) {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM ai_generation_jobs
		 WHERE created_at < $1
		   AND status NOT IN ('queued', 'running')`, cutoff)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
