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

type CFSubmitClient struct {
	baseURL    string
	httpClient *http.Client
}

type cfSubmitLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Proxy    string `json:"proxy,omitempty"`
}

type cfSubmitRequest struct {
	ProblemCode  string            `json:"problem_code"`
	SourceCode   string            `json:"source_code"`
	LangID       string            `json:"lang_id"`
	Handle       string            `json:"handle"`
	SubmissionID string            `json:"submission_id"`
	Username     string            `json:"username"`
	Password     string            `json:"password"`
	Cookies      map[string]string `json:"cookies,omitempty"`
	Proxy        string            `json:"proxy,omitempty"`
}

type cfSubmitResponse struct {
	Status       string `json:"status"`
	Message      string `json:"message"`
	Error        string `json:"error"`
	SubmissionID string `json:"submission_id"`
	Cookies      map[string]string `json:"cookies"`
}

func NewCFSubmitClient(baseURL string) *CFSubmitClient {
	return &CFSubmitClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

func (c *CFSubmitClient) doRequest(ctx context.Context, method, path string, body interface{}) (*cfSubmitResponse, error) {
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

	var result cfSubmitResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("cf-submit error: %s", result.Error)
	}

	return &result, nil
}

func (c *CFSubmitClient) Login(ctx context.Context, username, password, proxy string) (map[string]string, error) {
	result, err := c.doRequest(ctx, "POST", "/login", cfSubmitLoginRequest{
		Username: username,
		Password: password,
		Proxy:    proxy,
	})
	if err != nil {
		return nil, err
	}

	return result.Cookies, nil
}

func (c *CFSubmitClient) SeedCookies(ctx context.Context, cookies map[string]string) error {
	_, err := c.doRequest(ctx, "POST", "/seed-cookies", map[string]interface{}{
		"cookies": cookies,
	})
	return err
}

type cfLanguagesResponse struct {
	Status    string               `json:"status"`
	Languages []RemoteLanguageItem `json:"languages"`
}

func (c *CFSubmitClient) FetchLanguages(ctx context.Context) ([]RemoteLanguageItem, error) {
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

	var result cfLanguagesResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if result.Status != "ok" {
		return nil, fmt.Errorf("cf-submit: unexpected status %s", result.Status)
	}

	return result.Languages, nil
}

func (c *CFSubmitClient) Submit(ctx context.Context, problemCode, sourceCode, langID, handle, submissionID, username, password string, cookies map[string]string, proxy string) (string, error) {
	result, err := c.doRequest(ctx, "POST", "/submit", cfSubmitRequest{
		ProblemCode:  problemCode,
		SourceCode:   sourceCode,
		LangID:       langID,
		Handle:       handle,
		SubmissionID: submissionID,
		Username:     username,
		Password:     password,
		Cookies:      cookies,
		Proxy:        proxy,
	})
	if err != nil {
		return "", err
	}

	return result.SubmissionID, nil
}

func (c *CFSubmitClient) GetStatus(ctx context.Context, handle string) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/status/"+handle, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return result, nil
}
