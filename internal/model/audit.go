package model

import (
	"encoding/json"
	"time"
)

// AuditEntry is one admin action recorded for compliance / forensics.
type AuditEntry struct {
	ID         string          `json:"id"`
	ActorID    string          `json:"actor_id,omitempty"`
	ActorName  string          `json:"actor_name"`
	Action     string          `json:"action"`
	TargetType string          `json:"target_type,omitempty"`
	TargetID   string          `json:"target_id,omitempty"`
	Detail     json.RawMessage `json:"detail"`
	IP         string          `json:"ip,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}
