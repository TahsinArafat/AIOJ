package vjudge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type AtCoderSubmitClient struct {
	baseURL    string
	httpClient *http.Client
}

type atcoderSubmitLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type atcoderSubmitRequest struct {
	ContestID  string `json:"contest_id"`
	ProblemID  string `json:"problem_id"`
	SourceCode string `json:"source_code"`
	LangID     string `json:"lang_id"`
	Username   string `json:"username"`
	Password   string `json:"password"`
}

type atcoderSubmitResponse struct {
	Status       string            `json:"status"`
	Message      string            `json:"message"`
	Error        string            `json:"error"`
	SubmissionID string            `json:"submission_id"`
	Cookies      map[string]string `json:"cookies"`
}

func NewAtCoderSubmitClient(baseURL string) *AtCoderSubmitClient {
	return &AtCoderSubmitClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

func (c *AtCoderSubmitClient) doRequest(ctx context.Context, method, path string, body interface{}) (*atcoderSubmitResponse, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result atcoderSubmitResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("atcoder-submit error: %s", result.Error)
	}

	return &result, nil
}

func (c *AtCoderSubmitClient) Login(ctx context.Context, username, password string) (map[string]string, error) {
	result, err := c.doRequest(ctx, "POST", "/login", atcoderSubmitLoginRequest{
		Username: username,
		Password: password,
	})
	if err != nil {
		return nil, err
	}
	return result.Cookies, nil
}

type atcoderLanguagesResponse struct {
	Status    string               `json:"status"`
	Languages []RemoteLanguageItem `json:"languages"`
}

func (c *AtCoderSubmitClient) FetchLanguages(ctx context.Context) ([]RemoteLanguageItem, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/languages", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result atcoderLanguagesResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if result.Status != "ok" {
		return nil, fmt.Errorf("atcoder-submit: unexpected status %s", result.Status)
	}

	return result.Languages, nil
}

func (c *AtCoderSubmitClient) Submit(ctx context.Context, contestID, problemID, sourceCode, langID, username, password string) (string, error) {
	result, err := c.doRequest(ctx, "POST", "/submit", atcoderSubmitRequest{
		ContestID:  contestID,
		ProblemID:  problemID,
		SourceCode: sourceCode,
		LangID:     langID,
		Username:   username,
		Password:   password,
	})
	if err != nil {
		return "", err
	}
	return result.SubmissionID, nil
}
