package ai

// Tool-calling protocol types.
//
// These mirror the OpenAI-compatible chat-completions shape, which every
// provider AIOJ talks to speaks (the endpoint is configured as
// "[OI]-compatible" in Admin › AI Models). Keeping the wire types local rather
// than importing a vendor SDK is what lets one deployment point at OpenAI,
// vLLM, Ollama or a gateway without a code change.
//
// Three roles participate in a tool conversation:
//
//	assistant → content + tool_calls        (the model asks for tools)
//	tool      → content + tool_call_id      (we answer each call)
//	assistant → content                     (the model's final answer)
//
// A tool result is addressed by tool_call_id, so several calls can be issued in
// one turn and answered in any order — the loop must not assume one at a time.

// Message is one turn in a conversation.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content,omitempty"`
	// Name is the tool name on a `tool` message. Some providers require it;
	// others ignore it, so it is always populated for tool results.
	Name string `json:"name,omitempty"`
	// ToolCalls is set by the assistant when it wants tools run.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// ToolCallID addresses which call a `tool` message answers.
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// Message roles.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// ToolCall is one tool invocation requested by the model.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction carries the tool name and its JSON-encoded arguments.
//
// Arguments is a string, not an object: the protocol defines it as a JSON
// string, and providers differ in how they escape it. Decoding it ourselves
// (rather than letting encoding/json do it) keeps malformed arguments a
// recoverable error the loop can report back to the model instead of a
// transport-level failure.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Tool describes one callable function to the model.
type Tool struct {
	Type     string     `json:"type"`
	Function ToolSchema `json:"function"`
}

// ToolSchema is the JSON-schema description of a tool.
type ToolSchema struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// Parameters is a JSON Schema object. Strict providers require
	// additionalProperties:false and a complete "required" list.
	Parameters map[string]any `json:"parameters"`
}

// NewTool builds a function tool definition.
func NewTool(name, description string, parameters map[string]any) Tool {
	return Tool{
		Type: "function",
		Function: ToolSchema{
			Name:        name,
			Description: description,
			Parameters:  parameters,
		},
	}
}

// ToolMessage builds the message that answers a tool call.
func ToolMessage(callID, name, content string) Message {
	return Message{Role: RoleTool, ToolCallID: callID, Name: name, Content: content}
}

// Finish reasons that matter to a loop.
const (
	FinishStop      = "stop"
	FinishToolCalls = "tool_calls"
	FinishLength    = "length"
)
