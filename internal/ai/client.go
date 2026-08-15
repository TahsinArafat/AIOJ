// Package ai provides a thin client for OpenAI-compatible chat completion APIs.
// It is designed to work with any self-hosted model that exposes the
// POST /chat/completions endpoint (vLLM, llama.cpp, Ollama, etc.).
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultTimeout = 120 * time.Second

// Client calls an OpenAI-compatible completions endpoint.
type Client struct {
	endpoint string
	apiKey   string
	model    string
	http     *http.Client
}

// New creates a Client.  endpoint is the base URL (e.g. "http://localhost:8080/v1").
func New(endpoint, apiKey, model string) *Client {
	return &Client{
		endpoint: endpoint,
		apiKey:   apiKey,
		model:    model,
		http:     &http.Client{Timeout: defaultTimeout},
	}
}

// Complete sends a chat completion request with a system and a user message.
// It instructs the model to respond in JSON (response_format: json_object) and
// returns the raw JSON string from the first choice.
func (c *Client) Complete(ctx context.Context, system, user string) (string, error) {
	payload := map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"response_format": map[string]string{"type": "json_object"},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("ai: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("ai: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("ai: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ai: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ai: upstream returned %d: %s", resp.StatusCode, truncate(string(respBody), 400))
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
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("ai: decode response: %w", err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("ai: model error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 || parsed.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("ai: empty response from model")
	}
	return parsed.Choices[0].Message.Content, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
