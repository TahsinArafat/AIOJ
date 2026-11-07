package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/tahsinarafat/aioj/internal/ai"
	"github.com/tahsinarafat/aioj/internal/harness"
)

// HarnessTranscript persists tool-conversation turns so a run can resume after
// a crash and be replayed for audit.
//
// It satisfies harness.Transcript. Messages are ordered by
// (turn_index, message_index), where message_index is assigned by the database
// as "next free slot in this turn": an assistant message and each of its tool
// results share a turn index, and the tool_call_id carries the association.
type HarnessTranscript struct{ db *sql.DB }

// NewHarnessTranscript builds the store.
func NewHarnessTranscript(db *sql.DB) *HarnessTranscript {
	return &HarnessTranscript{db: db}
}

// Append records one message of a turn.
func (t *HarnessTranscript) Append(ctx context.Context, jobID string, turn harness.Turn) error {
	var toolCalls json.RawMessage
	if len(turn.Message.ToolCalls) > 0 {
		raw, err := json.Marshal(turn.Message.ToolCalls)
		if err != nil {
			return fmt.Errorf("harness transcript: encode tool calls: %w", err)
		}
		toolCalls = raw
	}

	// message_index is computed in SQL so concurrent appends within a turn
	// cannot collide, and so a resumed worker continues the numbering instead
	// of restarting it.
	_, err := t.db.ExecContext(ctx, `
		INSERT INTO ai_harness_turns
			(job_id, turn_index, message_index, role, content, name, tool_call_id, tool_calls,
			 prompt_tokens, completion_tokens, latency_ms)
		SELECT $1, $2, COALESCE(MAX(message_index), -1) + 1, $3, $4, $5, $6, $7, $8, $9, $10
		FROM ai_harness_turns
		WHERE job_id = $1 AND turn_index = $2`,
		jobID, turn.Index, turn.Message.Role, turn.Message.Content, turn.Message.Name,
		turn.Message.ToolCallID, toolCalls, turn.PromptTokens, turn.CompletionTokens, turn.LatencyMS)
	if err != nil {
		return fmt.Errorf("harness transcript: append: %w", err)
	}
	return nil
}

// Load returns the recorded messages oldest first.
func (t *HarnessTranscript) Load(ctx context.Context, jobID string) ([]harness.Turn, error) {
	rows, err := t.db.QueryContext(ctx, `
		SELECT turn_index, role, content, name, tool_call_id, tool_calls,
		       prompt_tokens, completion_tokens, latency_ms
		FROM ai_harness_turns
		WHERE job_id = $1
		ORDER BY turn_index, message_index`, jobID)
	if err != nil {
		return nil, fmt.Errorf("harness transcript: load: %w", err)
	}
	defer rows.Close()

	var out []harness.Turn
	for rows.Next() {
		var (
			turn      harness.Turn
			role      string
			content   string
			name      string
			callID    string
			toolCalls []byte
		)
		if err := rows.Scan(&turn.Index, &role, &content, &name, &callID, &toolCalls,
			&turn.PromptTokens, &turn.CompletionTokens, &turn.LatencyMS); err != nil {
			return nil, fmt.Errorf("harness transcript: scan: %w", err)
		}
		turn.Message = ai.Message{Role: role, Content: content, Name: name, ToolCallID: callID}
		if len(toolCalls) > 0 {
			if err := json.Unmarshal(toolCalls, &turn.Message.ToolCalls); err != nil {
				return nil, fmt.Errorf("harness transcript: decode tool calls: %w", err)
			}
		}
		out = append(out, turn)
	}
	return out, rows.Err()
}
