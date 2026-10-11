// Package ai provides an OpenAI-compatible chat-completions client.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultTimeout = 120 * time.Second

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

func (c *Client) completeMode(ctx context.Context, system, user string, schema map[string]any, mode string) (*CompletionResult, error) {
	payload := map[string]any{
		"model":      c.model,
		"messages":   []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": user}},
		"max_tokens": c.maxOutputTokens,
	}
	if mode == "json_schema" && schema != nil {
		payload["response_format"] = map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "aioj_artifact", "strict": true, "schema": schema}}
	} else if mode == "json_schema" || mode == "json_object" {
		payload["response_format"] = map[string]string{"type": "json_object"}
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
		return nil, fmt.Errorf("ai: upstream returned %d", resp.StatusCode)
	}
	if int64(len(respBody)) > c.maxResponseBytes {
		return nil, fmt.Errorf("ai: response exceeds size limit")
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
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
	if len(parsed.Choices) == 0 || parsed.Choices[0].Message.Content == "" {
		return nil, fmt.Errorf("ai: empty response from model")
	}
	content := parsed.Choices[0].Message.Content
	if mode == "text_json" {
		content = extractJSONObject(content)
	}
	if !json.Valid([]byte(content)) {
		return nil, fmt.Errorf("ai: model did not return valid JSON")
	}
	return &CompletionResult{Content: content, PromptTokens: parsed.Usage.PromptTokens, CompletionTokens: parsed.Usage.CompletionTokens, Mode: mode}, nil
}

func extractJSONObject(s string) string {
	start, end := strings.IndexByte(s, '{'), strings.LastIndexByte(s, '}')
	if start >= 0 && end >= start {
		return s[start : end+1]
	}
	return s
}
