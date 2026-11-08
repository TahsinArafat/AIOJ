// Package harness runs a bounded, resumable multi-turn tool loop.
//
// It is deliberately independent of what the tools do. The model chooses which
// tool to call; it never chooses what gets executed — that is the ToolExecutor's
// job, behind its own validation and sandbox. This split is what lets the same
// loop run in-process (tools backed by go-judge directly) or in a separate
// container (tools behind an HTTP gateway) without a second implementation.
//
// Three invariants the loop guarantees:
//
//  1. It terminates. Every dimension is budgeted (turns, tool calls, tokens,
//     wall clock, sandbox CPU, materialized bytes) and exhausting any of them
//     stops the run with a reason instead of spinning.
//  2. It is resumable. The transcript is the source of truth, so a crashed
//     worker continues at the next turn instead of paying for the whole run
//     again — the specific thing a fixed retry loop gets wrong.
//  3. It is replayable. Every turn is stored with its tool calls and results,
//     so a run can be reconstructed offline.
package harness

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/tahsinarafat/aioj/internal/ai"
)

// Model is the chat client the loop drives. *ai.Client satisfies it; the
// interface exists so tests can script a conversation.
type Model interface {
	CompleteWithTools(ctx context.Context, messages []ai.Message, tools []ai.Tool) (*ai.CompletionResult, error)
}

// ToolExecutor owns the catalog and runs tool calls.
//
// Implementations MUST validate their own arguments: the model is untrusted
// input. They must also report what they consumed (bytes, CPU) so the loop can
// enforce budgets the model cannot see.
type ToolExecutor interface {
	// Tools returns the catalog offered to the model.
	Tools() []ai.Tool
	// Execute runs one call. A returned error is reported to the model as a
	// failed tool result rather than aborting the run: a malformed argument is
	// something the model can usually correct on the next turn.
	Execute(ctx context.Context, call ai.ToolCall) (Result, error)
}

// Result is what a tool reports back.
type Result struct {
	// Content is the JSON payload the model sees. Implementations must keep it
	// small: bulk data belongs on disk, described by hash and size, never in
	// the context window.
	Content string
	// BytesMaterialized is data written to disk by this call, for budgeting.
	BytesMaterialized int64
	// SandboxCPU is compute consumed in the sandbox, for budgeting.
	SandboxCPU time.Duration
}

// Stop reasons.
const (
	StopCompleted     = "completed"
	StopMaxTurns      = "max_turns"
	StopMaxToolCalls  = "max_tool_calls"
	StopMaxTokens     = "max_total_tokens"
	StopMaxWallClock  = "max_wall_clock"
	StopMaxSandboxCPU = "max_sandbox_cpu"
	StopMaxBytes      = "max_bytes"
	StopCancelled     = "cancelled"
)

// Budget bounds one run. A zero field means "no limit for this dimension",
// except MaxTurns which defaults to a sane value so an unbounded loop cannot
// be configured by accident.
type Budget struct {
	MaxTurns       int
	MaxToolCalls   int
	MaxTotalTokens int
	MaxWallClock   time.Duration
	MaxSandboxCPU  time.Duration
	MaxBytes       int64
}

// DefaultBudget is the generic loop floor, not the shipped generation policy.
//
// MaxTurns here is deliberately small because this package knows nothing about
// the tool catalog. The real generator spine (constraints, limits, shapes,
// generator, solution, materialize, run_reference, check_samples,
// check_determinism, finalize) needs more than ten calls, so
// problemgen.DefaultBudget raises MaxTurns to cover the spine plus repair
// headroom. Do not treat this value as sufficient for a generation run.
func DefaultBudget() Budget {
	return Budget{
		MaxTurns:       10,
		MaxToolCalls:   60,
		MaxTotalTokens: 200_000,
		MaxWallClock:   5 * time.Minute,
		MaxSandboxCPU:  120 * time.Second,
		MaxBytes:       64 << 20,
	}
}

// withDefaults fills unset limits from DefaultBudget so a partially-specified
// budget still terminates.
func (b Budget) withDefaults() Budget {
	d := DefaultBudget()
	if b.MaxTurns <= 0 {
		b.MaxTurns = d.MaxTurns
	}
	if b.MaxToolCalls <= 0 {
		b.MaxToolCalls = d.MaxToolCalls
	}
	if b.MaxTotalTokens <= 0 {
		b.MaxTotalTokens = d.MaxTotalTokens
	}
	if b.MaxWallClock <= 0 {
		b.MaxWallClock = d.MaxWallClock
	}
	if b.MaxSandboxCPU <= 0 {
		b.MaxSandboxCPU = d.MaxSandboxCPU
	}
	if b.MaxBytes <= 0 {
		b.MaxBytes = d.MaxBytes
	}
	return b
}

// Usage is what a run consumed.
type Usage struct {
	Turns        int           `json:"turns"`
	ToolCalls    int           `json:"tool_calls"`
	TotalTokens  int           `json:"total_tokens"`
	SandboxCPU   time.Duration `json:"sandbox_cpu"`
	BytesWritten int64         `json:"bytes_written"`
	Elapsed      time.Duration `json:"elapsed"`
}

// Outcome is the result of a run.
type Outcome struct {
	// FinalContent is the model's last assistant message. Empty when the run
	// stopped on a budget before the model produced one.
	FinalContent string
	StopReason   string
	Usage        Usage
	// Messages is the full conversation, for storage or debugging.
	Messages []ai.Message
}

// Completed reports whether the model finished on its own.
func (o *Outcome) Completed() bool { return o.StopReason == StopCompleted }

// Turn is one recorded step of a conversation, for the transcript store.
type Turn struct {
	Index            int
	Message          ai.Message
	PromptTokens     int
	CompletionTokens int
	LatencyMS        int
}

// Transcript persists turns so a run can be resumed and audited.
type Transcript interface {
	// Append records one turn.
	Append(ctx context.Context, jobID string, turn Turn) error
	// Load returns the turns recorded so far, oldest first.
	Load(ctx context.Context, jobID string) ([]Turn, error)
}

// Loop runs a conversation to completion or to a budget limit.
type Loop struct {
	Model      Model
	Tools      ToolExecutor
	Transcript Transcript
	Budget     Budget
	System     string
	// Now is injectable so tests can drive the wall clock deterministically.
	Now func() time.Time
}

// ErrNoTools is returned when the executor offers no catalog.
var ErrNoTools = errors.New("harness: tool executor offers no tools")

// Run drives the conversation for jobID, resuming from any recorded turns.
//
// initial is used only when there is no recorded transcript; on resume the
// stored conversation wins, because it is the authoritative record of what the
// model has already seen.
func (l *Loop) Run(ctx context.Context, jobID string, initial []ai.Message) (*Outcome, error) {
	if l.Model == nil {
		return nil, errors.New("harness: model is required")
	}
	if l.Tools == nil {
		return nil, ErrNoTools
	}
	catalog := l.Tools.Tools()
	if len(catalog) == 0 {
		return nil, ErrNoTools
	}

	budget := l.Budget.withDefaults()
	now := l.Now
	if now == nil {
		now = time.Now
	}
	started := now()

	var messages []ai.Message
	var usage Usage
	if l.Transcript != nil {
		recorded, err := l.Transcript.Load(ctx, jobID)
		if err != nil {
			return nil, fmt.Errorf("harness: load transcript: %w", err)
		}
		for _, t := range recorded {
			messages = append(messages, t.Message)
			usage.TotalTokens += t.PromptTokens + t.CompletionTokens
		}
		if len(recorded) > 0 {
			usage.Turns = recorded[len(recorded)-1].Index + 1
			slog.Info("harness: resuming", "job", jobID, "turns", usage.Turns)
		}
	}
	if len(messages) == 0 {
		if l.System != "" {
			messages = append(messages, ai.Message{Role: ai.RoleSystem, Content: l.System})
		}
		messages = append(messages, initial...)
	}

	outcome := &Outcome{StopReason: StopCompleted}
	stop := func(reason string) *Outcome {
		outcome.StopReason = reason
		outcome.Usage = usage
		outcome.Usage.Elapsed = now().Sub(started)
		outcome.Messages = messages
		if len(messages) > 0 {
			for i := len(messages) - 1; i >= 0; i-- {
				if messages[i].Role == ai.RoleAssistant && messages[i].Content != "" {
					outcome.FinalContent = messages[i].Content
					break
				}
			}
		}
		return outcome
	}

	for {
		if err := ctx.Err(); err != nil {
			return stop(StopCancelled), nil
		}
		if usage.Turns >= budget.MaxTurns {
			return stop(StopMaxTurns), nil
		}
		if usage.TotalTokens >= budget.MaxTotalTokens {
			return stop(StopMaxTokens), nil
		}
		if now().Sub(started) >= budget.MaxWallClock {
			return stop(StopMaxWallClock), nil
		}

		turnStart := now()
		res, err := l.Model.CompleteWithTools(ctx, messages, catalog)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return stop(StopCancelled), nil
			}
			return nil, fmt.Errorf("harness: model turn %d: %w", usage.Turns, err)
		}
		latency := int(now().Sub(turnStart).Milliseconds())
		usage.Turns++
		usage.TotalTokens += res.PromptTokens + res.CompletionTokens

		assistant := ai.Message{
			Role:      ai.RoleAssistant,
			Content:   res.Content,
			ToolCalls: res.ToolCalls,
		}
		messages = append(messages, assistant)
		if err := l.record(ctx, jobID, Turn{
			Index:            usage.Turns - 1,
			Message:          assistant,
			PromptTokens:     res.PromptTokens,
			CompletionTokens: res.CompletionTokens,
			LatencyMS:        latency,
		}); err != nil {
			return nil, err
		}

		// No tool calls: the model answered. That is the only way a run ends
		// successfully.
		if len(res.ToolCalls) == 0 {
			return stop(StopCompleted), nil
		}

		for _, call := range res.ToolCalls {
			// A turn can carry several calls. Checking the context only between
			// turns would let a cancel wait for the whole batch — including
			// sandbox runs — before it is noticed.
			if ctx.Err() != nil {
				return stop(StopCancelled), nil
			}
			if usage.ToolCalls >= budget.MaxToolCalls {
				return stop(StopMaxToolCalls), nil
			}
			if usage.SandboxCPU >= budget.MaxSandboxCPU {
				return stop(StopMaxSandboxCPU), nil
			}
			if usage.BytesWritten >= budget.MaxBytes {
				return stop(StopMaxBytes), nil
			}

			usage.ToolCalls++
			content := l.execute(ctx, call, &usage)

			toolMsg := ai.ToolMessage(call.ID, call.Function.Name, content)
			messages = append(messages, toolMsg)
			if err := l.record(ctx, jobID, Turn{
				Index:   usage.Turns - 1,
				Message: toolMsg,
			}); err != nil {
				return nil, err
			}
		}
	}
}

// execute runs one tool call and renders its result for the model.
//
// A tool failure is reported as content, not as an aborted run: the model can
// usually fix a bad argument, and a crashed loop would waste the turns already
// spent. The error text is bounded so a stack trace cannot flood the context.
func (l *Loop) execute(ctx context.Context, call ai.ToolCall, usage *Usage) string {
	res, err := l.Tools.Execute(ctx, call)
	if err != nil {
		return toolErrorContent(err)
	}
	usage.SandboxCPU += res.SandboxCPU
	usage.BytesWritten += res.BytesMaterialized
	if res.Content == "" {
		return `{"ok":true}`
	}
	return res.Content
}

func (l *Loop) record(ctx context.Context, jobID string, turn Turn) error {
	if l.Transcript == nil {
		return nil
	}
	if err := l.Transcript.Append(ctx, jobID, turn); err != nil {
		return fmt.Errorf("harness: record turn %d: %w", turn.Index, err)
	}
	return nil
}

const maxToolErrorLen = 2000

func toolErrorContent(err error) string {
	msg := err.Error()
	if len(msg) > maxToolErrorLen {
		msg = msg[:maxToolErrorLen] + "…"
	}
	return fmt.Sprintf(`{"ok":false,"error":%q}`, msg)
}
