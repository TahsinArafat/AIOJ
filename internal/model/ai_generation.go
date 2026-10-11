package model

import (
	"encoding/json"
	"time"
)

type AIGenerationJob struct {
	ID                   string
	ActorID              string
	ProblemID            *string
	AIModelID            *string
	ArtifactType         string
	Status               string
	Stage                string
	Progress             int
	SubmittedBrief       json.RawMessage
	NormalizedBrief      json.RawMessage
	ContextSnapshot      json.RawMessage
	BaseProblemUpdatedAt *time.Time
	ConnectionSnapshot   json.RawMessage
	PromptVersion        string
	PromptHash           string
	Proposal             json.RawMessage
	ValidationReport     json.RawMessage
	ErrorCode            string
	ErrorMessage         string
	PromptTokens         int
	CompletionTokens     int
	LatencyMS            int
	RetryOf              *string
	ClaimToken           *string
	HeartbeatAt          *time.Time
	CancelRequestedAt    *time.Time
	CreatedAt            time.Time
	StartedAt            *time.Time
	FinishedAt           *time.Time
	AppliedAt            *time.Time
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
