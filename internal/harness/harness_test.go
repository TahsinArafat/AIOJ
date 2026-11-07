package harness

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/ai"
)

// scriptedModel replays a fixed list of responses, recording the messages it
// was given so tests can assert on what the loop actually sent.
type scriptedModel struct {
	responses []*ai.CompletionResult
	calls     int
	seen      [][]ai.Message
}

func (m *scriptedModel) CompleteWithTools(_ context.Context, messages []ai.Message, _ []ai.Tool) (*ai.CompletionResult, error) {
	snapshot := make([]ai.Message, len(messages))
	copy(snapshot, messages)
	m.seen = append(m.seen, snapshot)
	if m.calls >= len(m.responses) {
		return nil, errors.New("scripted model exhausted")
	}
	res := m.responses[m.calls]
	m.calls++
	return res, nil
}

// scriptedTools answers calls from a table and records what it was asked.
type scriptedTools struct {
	handlers map[string]func(args string) (Result, error)
	calls    []ai.ToolCall
}

func (s *scriptedTools) Tools() []ai.Tool {
	return []ai.Tool{ai.NewTool("compile", "compile", map[string]any{"type": "object"})}
}

func (s *scriptedTools) Execute(_ context.Context, call ai.ToolCall) (Result, error) {
	s.calls = append(s.calls, call)
	if h, ok := s.handlers[call.Function.Name]; ok {
		return h(call.Function.Arguments)
	}
	return Result{}, errors.New("unknown tool " + call.Function.Name)
}

func toolCall(id, name, args string) ai.ToolCall {
	return ai.ToolCall{ID: id, Type: "function", Function: ai.ToolCallFunction{Name: name, Arguments: args}}
}

func TestLoopRunsUntilTheModelAnswers(t *testing.T) {
	model := &scriptedModel{responses: []*ai.CompletionResult{
		{ToolCalls: []ai.ToolCall{toolCall("c1", "compile", `{"artifact":"generator"}`)}, PromptTokens: 10, CompletionTokens: 5},
		{Content: "done", PromptTokens: 20, CompletionTokens: 5},
	}}
	tools := &scriptedTools{handlers: map[string]func(string) (Result, error){
		"compile": func(string) (Result, error) { return Result{Content: `{"ok":true}`}, nil },
	}}
	tr := NewMemoryTranscript()

	out, err := (&Loop{Model: model, Tools: tools, Transcript: tr, System: "sys"}).
		Run(context.Background(), "job-1", []ai.Message{{Role: ai.RoleUser, Content: "go"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Completed() {
		t.Fatalf("stop reason = %q, want completed", out.StopReason)
	}
	if out.FinalContent != "done" {
		t.Fatalf("final content = %q", out.FinalContent)
	}
	if out.Usage.Turns != 2 || out.Usage.ToolCalls != 1 {
		t.Fatalf("usage = %+v", out.Usage)
	}
	if out.Usage.TotalTokens != 40 {
		t.Fatalf("total tokens = %d, want 40", out.Usage.TotalTokens)
	}
	// system + user + assistant(tool_calls) + tool result + assistant(final)
	if got := tr.Len("job-1"); got != 3 {
		t.Fatalf("recorded turns = %d, want 3 (2 assistant + 1 tool)", got)
	}

	// The tool result must be addressed back to the call that asked for it.
	last := model.seen[len(model.seen)-1]
	var found bool
	for _, m := range last {
		if m.Role == ai.RoleTool && m.ToolCallID == "c1" && m.Name == "compile" {
			found = true
		}
	}
	if !found {
		t.Fatalf("tool result not fed back correctly: %+v", last)
	}
}

func TestLoopHandlesSeveralToolCallsInOneTurn(t *testing.T) {
	model := &scriptedModel{responses: []*ai.CompletionResult{
		{ToolCalls: []ai.ToolCall{
			toolCall("a", "compile", `{}`),
			toolCall("b", "compile", `{}`),
			toolCall("c", "compile", `{}`),
		}},
		{Content: "ok"},
	}}
	tools := &scriptedTools{handlers: map[string]func(string) (Result, error){
		"compile": func(string) (Result, error) { return Result{Content: `{"ok":true}`}, nil },
	}}

	out, err := (&Loop{Model: model, Tools: tools}).Run(context.Background(), "j", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Usage.ToolCalls != 3 {
		t.Fatalf("tool calls = %d, want 3", out.Usage.ToolCalls)
	}
	if !out.Completed() {
		t.Fatalf("stop reason = %q", out.StopReason)
	}
}

func TestLoopStopsAtMaxTurns(t *testing.T) {
	// A model that always asks for another tool would otherwise never return.
	model := &scriptedModel{}
	for i := 0; i < 20; i++ {
		model.responses = append(model.responses, &ai.CompletionResult{
			ToolCalls: []ai.ToolCall{toolCall("c", "compile", `{}`)},
		})
	}
	tools := &scriptedTools{handlers: map[string]func(string) (Result, error){
		"compile": func(string) (Result, error) { return Result{Content: `{}`}, nil },
	}}

	out, err := (&Loop{Model: model, Tools: tools, Budget: Budget{MaxTurns: 3}}).Run(context.Background(), "j", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.StopReason != StopMaxTurns {
		t.Fatalf("stop reason = %q, want %q", out.StopReason, StopMaxTurns)
	}
	if out.Usage.Turns != 3 {
		t.Fatalf("turns = %d, want 3", out.Usage.Turns)
	}
}

func TestLoopStopsAtMaxToolCalls(t *testing.T) {
	model := &scriptedModel{}
	for i := 0; i < 10; i++ {
		model.responses = append(model.responses, &ai.CompletionResult{
			ToolCalls: []ai.ToolCall{
				toolCall("a", "compile", `{}`),
				toolCall("b", "compile", `{}`),
			},
		})
	}
	tools := &scriptedTools{handlers: map[string]func(string) (Result, error){
		"compile": func(string) (Result, error) { return Result{Content: `{}`}, nil },
	}}

	out, err := (&Loop{Model: model, Tools: tools, Budget: Budget{MaxToolCalls: 5, MaxTurns: 10}}).Run(context.Background(), "j", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.StopReason != StopMaxToolCalls {
		t.Fatalf("stop reason = %q, want %q", out.StopReason, StopMaxToolCalls)
	}
	if out.Usage.ToolCalls != 5 {
		t.Fatalf("tool calls = %d, want exactly the budget", out.Usage.ToolCalls)
	}
}

func TestLoopStopsAtTokenBudget(t *testing.T) {
	model := &scriptedModel{responses: []*ai.CompletionResult{
		{ToolCalls: []ai.ToolCall{toolCall("c", "compile", `{}`)}, PromptTokens: 600, CompletionTokens: 600},
		{Content: "never reached"},
	}}
	tools := &scriptedTools{handlers: map[string]func(string) (Result, error){
		"compile": func(string) (Result, error) { return Result{Content: `{}`}, nil },
	}}

	out, err := (&Loop{Model: model, Tools: tools, Budget: Budget{MaxTotalTokens: 1000, MaxTurns: 5}}).Run(context.Background(), "j", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.StopReason != StopMaxTokens {
		t.Fatalf("stop reason = %q, want %q", out.StopReason, StopMaxTokens)
	}
}

func TestLoopStopsAtSandboxCPUBudget(t *testing.T) {
	model := &scriptedModel{}
	for i := 0; i < 5; i++ {
		model.responses = append(model.responses, &ai.CompletionResult{
			ToolCalls: []ai.ToolCall{toolCall("c", "compile", `{}`)},
		})
	}
	tools := &scriptedTools{handlers: map[string]func(string) (Result, error){
		"compile": func(string) (Result, error) {
			return Result{Content: `{}`, SandboxCPU: 4 * time.Second}, nil
		},
	}}

	out, err := (&Loop{Model: model, Tools: tools, Budget: Budget{MaxSandboxCPU: 10 * time.Second, MaxTurns: 5}}).Run(context.Background(), "j", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.StopReason != StopMaxSandboxCPU {
		t.Fatalf("stop reason = %q, want %q", out.StopReason, StopMaxSandboxCPU)
	}
}

func TestLoopStopsAtByteBudget(t *testing.T) {
	model := &scriptedModel{}
	for i := 0; i < 5; i++ {
		model.responses = append(model.responses, &ai.CompletionResult{
			ToolCalls: []ai.ToolCall{toolCall("c", "compile", `{}`)},
		})
	}
	tools := &scriptedTools{handlers: map[string]func(string) (Result, error){
		"compile": func(string) (Result, error) {
			return Result{Content: `{}`, BytesMaterialized: 400}, nil
		},
	}}

	out, err := (&Loop{Model: model, Tools: tools, Budget: Budget{MaxBytes: 1000, MaxTurns: 5}}).Run(context.Background(), "j", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.StopReason != StopMaxBytes {
		t.Fatalf("stop reason = %q, want %q", out.StopReason, StopMaxBytes)
	}
}

func TestLoopReportsToolFailureToTheModelInsteadOfAborting(t *testing.T) {
	// A bad argument is recoverable: the model gets the error and tries again.
	model := &scriptedModel{responses: []*ai.CompletionResult{
		{ToolCalls: []ai.ToolCall{toolCall("c1", "compile", `not json`)}},
		{Content: "recovered"},
	}}
	tools := &scriptedTools{handlers: map[string]func(string) (Result, error){
		"compile": func(args string) (Result, error) {
			if !json.Valid([]byte(args)) {
				return Result{}, errors.New("arguments must be JSON")
			}
			return Result{Content: `{"ok":true}`}, nil
		},
	}}

	out, err := (&Loop{Model: model, Tools: tools}).Run(context.Background(), "j", nil)
	if err != nil {
		t.Fatalf("a tool error must not abort the run: %v", err)
	}
	if !out.Completed() {
		t.Fatalf("stop reason = %q", out.StopReason)
	}
	last := model.seen[len(model.seen)-1]
	var sawError bool
	for _, m := range last {
		if m.Role == ai.RoleTool && strings.Contains(m.Content, `"ok":false`) {
			sawError = true
		}
	}
	if !sawError {
		t.Fatalf("the model was not told about the failure: %+v", last)
	}
}

func TestLoopResumesFromTheTranscript(t *testing.T) {
	// The point of the transcript: a crashed worker continues instead of
	// paying for the whole conversation again.
	tr := NewMemoryTranscript()
	_ = tr.Append(context.Background(), "job-9", Turn{Index: 0, Message: ai.Message{Role: ai.RoleUser, Content: "first"}})
	_ = tr.Append(context.Background(), "job-9", Turn{Index: 0, Message: ai.Message{Role: ai.RoleAssistant, Content: "partial"}})

	model := &scriptedModel{responses: []*ai.CompletionResult{{Content: "finished"}}}
	tools := &scriptedTools{handlers: map[string]func(string) (Result, error){}} // no tools needed

	out, err := (&Loop{Model: model, Tools: tools, Transcript: tr, System: "sys"}).
		Run(context.Background(), "job-9", []ai.Message{{Role: ai.RoleUser, Content: "SHOULD BE IGNORED"}})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Completed() {
		t.Fatalf("stop reason = %q", out.StopReason)
	}

	// The stored conversation is authoritative, so the system prompt must not
	// be re-injected and the initial message must not be duplicated.
	sent := model.seen[0]
	for _, m := range sent {
		if m.Role == ai.RoleSystem {
			t.Fatal("system prompt re-injected on resume")
		}
		if m.Content == "SHOULD BE IGNORED" {
			t.Fatal("initial message used despite a recorded transcript")
		}
	}
	if len(sent) != 2 {
		t.Fatalf("resumed conversation has %d messages, want the 2 recorded ones", len(sent))
	}
}

func TestLoopStopsWhenTheContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	model := &scriptedModel{responses: []*ai.CompletionResult{{Content: "unused"}}}
	out, err := (&Loop{Model: model, Tools: &scriptedTools{}}).Run(ctx, "j", nil)
	if err != nil {
		t.Fatalf("cancellation must not be an error: %v", err)
	}
	if out.StopReason != StopCancelled {
		t.Fatalf("stop reason = %q, want %q", out.StopReason, StopCancelled)
	}
	if model.calls != 0 {
		t.Fatal("the model was called after cancellation")
	}
}

// emptyTools offers no catalog at all.
type emptyTools struct{}

func (emptyTools) Tools() []ai.Tool { return nil }
func (emptyTools) Execute(context.Context, ai.ToolCall) (Result, error) {
	return Result{}, errors.New("no tools")
}

func TestLoopRequiresAToolCatalog(t *testing.T) {
	// An empty catalog would let the model "answer" without ever touching the
	// sandbox, which silently produces unvalidated problems.
	_, err := (&Loop{Model: &scriptedModel{}, Tools: emptyTools{}}).Run(context.Background(), "j", nil)
	if !errors.Is(err, ErrNoTools) {
		t.Fatalf("err = %v, want ErrNoTools", err)
	}
}

func TestBudgetDefaultsAreApplied(t *testing.T) {
	b := Budget{}.withDefaults()
	d := DefaultBudget()
	if b != d {
		t.Fatalf("empty budget = %+v, want the defaults %+v", b, d)
	}
	// A partially specified budget keeps the explicit value.
	b = Budget{MaxTurns: 2}.withDefaults()
	if b.MaxTurns != 2 {
		t.Fatalf("MaxTurns = %d, want 2", b.MaxTurns)
	}
	if b.MaxToolCalls != d.MaxToolCalls {
		t.Fatal("unspecified limits must fall back to the defaults")
	}
}
