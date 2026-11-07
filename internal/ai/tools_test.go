package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newFakeEndpoint serves one canned chat-completions response and records the
// request body so the test can assert on the wire shape.
func newFakeEndpoint(t *testing.T, status int, body string) (*Client, *[]byte) {
	t.Helper()
	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		captured = buf
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL+"/v1", "test-key", "test-model"), &captured
}

func TestCompleteWithToolsSendsToolsAndParsesCalls(t *testing.T) {
	client, captured := newFakeEndpoint(t, http.StatusOK, `{
		"choices":[{"message":{"content":"","tool_calls":[
			{"id":"call_1","type":"function","function":{"name":"compile","arguments":"{\"artifact\":\"generator\"}"}},
			{"id":"call_2","type":"function","function":{"name":"generate_case","arguments":"{\"seed\":7,\"n\":100000}"}}
		]},"finish_reason":"tool_calls"}],
		"usage":{"prompt_tokens":120,"completion_tokens":30}
	}`)

	tools := []Tool{NewTool("compile", "Compile an artifact", map[string]any{"type": "object"})}
	res, err := client.CompleteWithTools(context.Background(),
		[]Message{{Role: RoleUser, Content: "make a problem"}}, tools)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Several calls can arrive in one turn; the loop must handle all of them.
	if len(res.ToolCalls) != 2 {
		t.Fatalf("tool calls = %d, want 2", len(res.ToolCalls))
	}
	if res.ToolCalls[0].Function.Name != "compile" || res.ToolCalls[1].Function.Name != "generate_case" {
		t.Fatalf("unexpected tool names: %+v", res.ToolCalls)
	}
	if res.FinishReason != FinishToolCalls {
		t.Fatalf("finish reason = %q, want %q", res.FinishReason, FinishToolCalls)
	}
	if res.PromptTokens != 120 || res.CompletionTokens != 30 {
		t.Fatalf("usage not propagated: %+v", res)
	}

	var sent map[string]any
	if err := json.Unmarshal(*captured, &sent); err != nil {
		t.Fatalf("request body is not JSON: %v", err)
	}
	if _, ok := sent["tools"]; !ok {
		t.Fatal("request did not include tools")
	}
	if sent["tool_choice"] != "auto" {
		t.Fatalf("tool_choice = %v, want auto", sent["tool_choice"])
	}
	// Forcing a response_format alongside tools is rejected by most providers.
	if _, ok := sent["response_format"]; ok {
		t.Fatal("tool requests must not send response_format")
	}
}

func TestCompleteWithToolsReportsUnsupportedEndpoint(t *testing.T) {
	// A 4xx on a tool request means "this model cannot call tools", which the
	// caller must be able to distinguish from a transient failure.
	client, _ := newFakeEndpoint(t, http.StatusBadRequest, `{"error":{"message":"tools not supported"}}`)

	_, err := client.CompleteWithTools(context.Background(),
		[]Message{{Role: RoleUser, Content: "hi"}},
		[]Tool{NewTool("compile", "x", map[string]any{})})
	if !errors.Is(err, ErrToolsUnsupported) {
		t.Fatalf("err = %v, want ErrToolsUnsupported", err)
	}
}

func TestCompleteWithToolsRequiresATool(t *testing.T) {
	client, _ := newFakeEndpoint(t, http.StatusOK, `{}`)
	if _, err := client.CompleteWithTools(context.Background(), nil, nil); err == nil {
		t.Fatal("expected an error when no tools are supplied")
	}
}

func TestCompleteWithToolsRejectsAnEmptyAnswer(t *testing.T) {
	// Neither content nor a tool call is a dead end: the loop would spin.
	client, _ := newFakeEndpoint(t, http.StatusOK, `{"choices":[{"message":{"content":""},"finish_reason":"stop"}]}`)

	_, err := client.CompleteWithTools(context.Background(),
		[]Message{{Role: RoleUser, Content: "hi"}},
		[]Tool{NewTool("compile", "x", map[string]any{})})
	if err == nil || !strings.Contains(err.Error(), "empty response") {
		t.Fatalf("err = %v, want an empty-response error", err)
	}
}

func TestToolMessagesMarshalCorrectly(t *testing.T) {
	msgs := []Message{
		{Role: RoleUser, Content: "go"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "c1", Type: "function", Function: ToolCallFunction{Name: "compile", Arguments: "{}"}}}},
		ToolMessage("c1", "compile", `{"ok":true}`),
	}
	raw, err := json.Marshal(msgs)
	if err != nil {
		t.Fatal(err)
	}
	var back []map[string]any
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back[1]["role"] != RoleAssistant {
		t.Fatalf("assistant role = %v", back[1]["role"])
	}
	// An assistant turn that only calls tools must not carry an empty content
	// field; some providers reject it.
	if _, ok := back[1]["content"]; ok {
		t.Fatal("assistant tool-call message must omit empty content")
	}
	if back[2]["tool_call_id"] != "c1" {
		t.Fatalf("tool_call_id = %v, want c1", back[2]["tool_call_id"])
	}
	if back[2]["name"] != "compile" {
		t.Fatalf("name = %v, want compile", back[2]["name"])
	}
}

func TestCompleteJSONStillWorks(t *testing.T) {
	// The single-shot path is used by the legacy generator and the admin
	// "test connection" button; the tool work must not disturb it.
	client, captured := newFakeEndpoint(t, http.StatusOK, `{
		"choices":[{"message":{"content":"{\"title\":\"Two Sum\"}"},"finish_reason":"stop"}],
		"usage":{"prompt_tokens":10,"completion_tokens":5}
	}`)

	res, err := client.CompleteJSON(context.Background(), "sys", "usr", map[string]any{"type": "object"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Content, "Two Sum") {
		t.Fatalf("content = %q", res.Content)
	}

	var sent map[string]any
	if err := json.Unmarshal(*captured, &sent); err != nil {
		t.Fatal(err)
	}
	if _, ok := sent["tools"]; ok {
		t.Fatal("the single-shot path must not send tools")
	}
	if _, ok := sent["response_format"]; !ok {
		t.Fatal("the single-shot path must still negotiate response_format")
	}
}
