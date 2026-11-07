package model

import (
	"encoding/json"
	"time"
)

// AIGenerationJob is serialised straight to API clients, so its JSON tags are
// part of the frontend contract (web/src/types/api.ts, AIGenerationJob).
//
// Without tags Go emits the field names verbatim ("Status", "Proposal"), the
// browser read undefined for every property, and AI Studio broke in two ways:
// a finished job could not be resumed, and rendering one threw
// "Cannot read properties of undefined (reading 'replaceAll')" on job.stage,
// which blanked the whole route.
type AIGenerationJob struct {
	ID                   string          `json:"id"`
	ActorID              string          `json:"actor_id"`
	ProblemID            *string         `json:"problem_id,omitempty"`
	AIModelID            *string         `json:"ai_model_id,omitempty"`
	ArtifactType         string          `json:"artifact_type"`
	Status               string          `json:"status"`
	Stage                string          `json:"stage"`
	Progress             int             `json:"progress"`
	SubmittedBrief       json.RawMessage `json:"submitted_brief"`
	NormalizedBrief      json.RawMessage `json:"normalized_brief"`
	ContextSnapshot      json.RawMessage `json:"context_snapshot,omitempty"`
	BaseProblemUpdatedAt *time.Time      `json:"base_problem_updated_at,omitempty"`
	ConnectionSnapshot   json.RawMessage `json:"connection_snapshot,omitempty"`
	PromptVersion        string          `json:"prompt_version"`
	PromptHash           string          `json:"prompt_hash"`
	Proposal             json.RawMessage `json:"proposal,omitempty"`
	ValidationReport     json.RawMessage `json:"validation_report,omitempty"`
	ErrorCode            string          `json:"error_code"`
	ErrorMessage         string          `json:"error_message"`
	PromptTokens         int             `json:"prompt_tokens"`
	CompletionTokens     int             `json:"completion_tokens"`
	LatencyMS            int             `json:"latency_ms"`
	RetryOf              *string         `json:"retry_of,omitempty"`
	// ClaimToken is worker-internal and must not leak to clients.
	ClaimToken        *string    `json:"-"`
	HeartbeatAt       *time.Time `json:"heartbeat_at,omitempty"`
	CancelRequestedAt *time.Time `json:"cancel_requested_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	FinishedAt        *time.Time `json:"finished_at,omitempty"`
	AppliedAt         *time.Time `json:"applied_at,omitempty"`
}

type ProblemGenerationAsset struct {
	ID                        string
	ProblemID                 string
	JobID                     string
	GeneratorLanguage         string
	GeneratorSource           string
	ReferenceSolutionLanguage string
	ReferenceSolutionSource   string
	SeedManifest              json.RawMessage
	ValidationSummary         json.RawMessage
	TestdataPath              string
	CreatedAt                 time.Time
}
