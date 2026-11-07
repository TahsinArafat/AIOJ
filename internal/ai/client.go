// Package ai provides an [OI]-compatible chat-completions client.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultTimeout = 120 * time.Second

// ErrToolsUnsupported is returned when the endpoint rejects a tool request.
// The caller treats it as a capability signal, not a hard failure: a model
// without tool support still has to be usable through the single-shot path.
var ErrToolsUnsupported = errors.New("ai: endpoint does not support tool calling")

type Client struct {
	endpoint         string
	apiKey           string
	model            string
	http             *http.Client
	mode             string
	maxOutputTokens  int
	maxResponseBytes int64
}

type ClientOptions struct {
	Mode                string
	Timeout             time.Duration
	MaxOutputTokens     int
	MaxResponseBytes    int64
	AllowPrivateNetwork bool
}

type CompletionResult struct {
	Content          string
	PromptTokens     int
	CompletionTokens int
	Mode             string
	// ToolCalls is populated when the model asks for tools instead of (or in
	// addition to) answering.
	ToolCalls []ToolCall
	// FinishReason is the provider's stop reason, normalised to the Finish*
	// constants where possible.
	FinishReason string
}

func New(endpoint, apiKey, model string) *Client {
	return NewWithOptions(endpoint, apiKey, model, ClientOptions{AllowPrivateNetwork: true})
}

func NewWithOptions(endpoint, apiKey, model string, opts ClientOptions) *Client {
	if opts.Timeout <= 0 {
		opts.Timeout = defaultTimeout
	}
	if opts.MaxOutputTokens <= 0 {
		opts.MaxOutputTokens = 8192
	}
	if opts.MaxResponseBytes <= 0 {
		opts.MaxResponseBytes = 4 << 20
	}
	if opts.Mode == "" {
		opts.Mode = "auto"
	}
	return &Client{
		endpoint: strings.TrimRight(endpoint, "/"), apiKey: apiKey, model: model,
		http: (EndpointPolicy{AllowPrivate: opts.AllowPrivateNetwork}).HTTPClient(opts.Timeout),
		mode: opts.Mode, maxOutputTokens: opts.MaxOutputTokens, maxResponseBytes: opts.MaxResponseBytes,
	}
}

func (c *Client) Complete(ctx context.Context, system, user string) (string, error) {
	result, err := c.CompleteJSON(ctx, system, user, nil)
	if err != nil {
		return "", err
	}
	return result.Content, nil
}

// CompleteJSON negotiates structured output. auto tries json_schema,
// json_object, then plain text containing JSON.
func (c *Client) CompleteJSON(ctx context.Context, system, user string, schema map[string]any) (*CompletionResult, error) {
	modes := []string{c.mode}
	if c.mode == "auto" {
		modes = []string{"json_schema", "json_object", "text_json"}
	}
	var lastErr error
	for _, mode := range modes {
		result, err := c.completeMode(ctx, system, user, schema, mode)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// CompleteWithTools runs one turn of a tool conversation.
//
// No response_format is sent: forcing strict JSON and tool calls at the same
// time is rejected by most providers, and the tool protocol already gives the
// loop a structured contract. Structured output is negotiated only on the
// single-shot path (CompleteJSON).
func (c *Client) CompleteWithTools(ctx context.Context, messages []Message, tools []Tool) (*CompletionResult, error) {
	if len(tools) == 0 {
		return nil, errors.New("ai: CompleteWithTools requires at least one tool")
	}
	return c.chat(ctx, messages, tools)
}

func (c *Client) completeMode(ctx context.Context, system, user string, schema map[string]any, mode string) (*CompletionResult, error) {
	messages := []Message{
		{Role: RoleSystem, Content: system},
		{Role: RoleUser, Content: user},
	}
	extra := map[string]any{}
	if mode == "json_schema" && schema != nil {
		extra["response_format"] = map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "aioj_artifact", "strict": true, "schema": schema}}
	} else if mode == "json_schema" || mode == "json_object" {
		extra["response_format"] = map[string]string{"type": "json_object"}
	}
	result, err := c.send(ctx, messages, nil, extra)
	if err != nil {
		return nil, err
	}
	if mode == "text_json" {
		result.Content = extractJSONObject(result.Content)
	}
	if !json.Valid([]byte(result.Content)) {
		return nil, fmt.Errorf("ai: model did not return valid JSON")
	}
	result.Mode = mode
	return result, nil
}

func (c *Client) chat(ctx context.Context, messages []Message, tools []Tool) (*CompletionResult, error) {
	result, err := c.send(ctx, messages, tools, nil)
	if err != nil {
		return nil, err
	}
	result.Mode = "tools"
	if len(result.ToolCalls) == 0 && strings.TrimSpace(result.Content) == "" {
		return nil, fmt.Errorf("ai: empty response from model")
	}
	return result, nil
}

// send performs one chat-completions request and decodes the first choice.
func (c *Client) send(ctx context.Context, messages []Message, tools []Tool, extra map[string]any) (*CompletionResult, error) {
	payload := map[string]any{
		"model":      c.model,
		"messages":   messages,
		"max_tokens": c.maxOutputTokens,
	}
	if len(tools) > 0 {
		payload["tools"] = tools
		// Let the model decide whether to answer or call a tool; every provider
		// that supports tools accepts "auto".
		payload["tool_choice"] = "auto"
	}
	for k, v := range extra {
		payload[k] = v
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("ai: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ai: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ai: request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, c.maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("ai: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		// A 4xx on a tool request is almost always "tools are not supported by
		// this model/endpoint". Surface that distinctly so the caller can fall
		// back instead of treating it as a transient failure and retrying.
		if len(tools) > 0 && resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return nil, fmt.Errorf("%w: upstream returned %d", ErrToolsUnsupported, resp.StatusCode)
		}
		return nil, fmt.Errorf("ai: upstream returned %d", resp.StatusCode)
	}
	if int64(len(respBody)) > c.maxResponseBytes {
		return nil, fmt.Errorf("ai: response exceeds size limit")
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content   string     `json:"content"`
				ToolCalls []ToolCall `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("ai: decode response: %w", err)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("ai: model error")
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("ai: empty response from model")
	}

	choice := parsed.Choices[0]
	result := &CompletionResult{
		Content:          choice.Message.Content,
		PromptTokens:     parsed.Usage.PromptTokens,
		CompletionTokens: parsed.Usage.CompletionTokens,
		ToolCalls:        choice.Message.ToolCalls,
		FinishReason:     choice.FinishReason,
	}
	// Some providers omit finish_reason; infer it from the payload so the loop
	// never has to special-case "no reason".
	if result.FinishReason == "" {
		if len(result.ToolCalls) > 0 {
			result.FinishReason = FinishToolCalls
		} else {
			result.FinishReason = FinishStop
		}
	}
	return result, nil
}

func extractJSONObject(s string) string {
	start, end := strings.IndexByte(s, '{'), strings.LastIndexByte(s, '}')
	if start >= 0 && end >= start {
		return s[start : end+1]
	}
	return s
}
