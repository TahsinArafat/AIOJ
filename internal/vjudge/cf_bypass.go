package vjudge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type CFBypassClient struct {
	baseURL    string
	httpClient *http.Client
}

type flaresolverrRequest struct {
	CMD        string                 `json:"cmd"`
	URL        string                 `json:"url,omitempty"`
	Session    string                 `json:"session,omitempty"`
	PostData   string                 `json:"postData,omitempty"`
	MaxTimeout int                    `json:"maxTimeout,omitempty"`
	Cookies    []flaresolverrCookie   `json:"cookies,omitempty"`
}

type flaresolverrResponse struct {
	Status  string                `json:"status"`
	Message string                `json:"message"`
	Session string                `json:"session,omitempty"`
	Solution *flaresolverrSolution `json:"solution,omitempty"`
}

type flaresolverrSolution struct {
	URL      string               `json:"url"`
	Status   int                  `json:"status"`
	Response string               `json:"response"`
	Cookies  []flaresolverrCookie `json:"cookies"`
}

type flaresolverrCookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

func NewCFBypassClient(baseURL string) *CFBypassClient {
	return &CFBypassClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (c *CFBypassClient) doRequest(ctx context.Context, req flaresolverrRequest) (*flaresolverrResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result flaresolverrResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if result.Status != "ok" {
		return nil, fmt.Errorf("flaresolverr error: %s", result.Message)
	}

	return &result, nil
}

func (c *CFBypassClient) GetCookies(ctx context.Context, targetURL string) (map[string]string, string, error) {
	result, err := c.doRequest(ctx, flaresolverrRequest{
		CMD:        "request.get",
		URL:        targetURL,
		MaxTimeout: 60000,
	})
	if err != nil {
		return nil, "", err
	}

	if result.Solution == nil {
		return nil, "", fmt.Errorf("no solution in response")
	}

	cookies := make(map[string]string)
	for _, ck := range result.Solution.Cookies {
		cookies[ck.Name] = ck.Value
	}

	return cookies, result.Solution.Response, nil
}

func (c *CFBypassClient) Get(ctx context.Context, targetURL string) (*flaresolverrSolution, error) {
	result, err := c.doRequest(ctx, flaresolverrRequest{
		CMD:        "request.get",
		URL:        targetURL,
		MaxTimeout: 60000,
	})
	if err != nil {
		return nil, err
	}
	return result.Solution, nil
}

func (c *CFBypassClient) Post(ctx context.Context, targetURL string, postData string) (*flaresolverrSolution, error) {
	result, err := c.doRequest(ctx, flaresolverrRequest{
		CMD:        "request.post",
		URL:        targetURL,
		PostData:   postData,
		MaxTimeout: 60000,
	})
	if err != nil {
		return nil, err
	}
	return result.Solution, nil
}



func (c *CFBypassClient) PostWithSession(ctx context.Context, targetURL string, postData string, sessionID string) (*flaresolverrSolution, error) {
	result, err := c.doRequest(ctx, flaresolverrRequest{
		CMD:        "request.post",
		URL:        targetURL,
		PostData:   postData,
		Session:    sessionID,
		MaxTimeout: 60000,
	})
	if err != nil {
		return nil, err
	}
	return result.Solution, nil
}

func (c *CFBypassClient) GetWithSession(ctx context.Context, targetURL string, sessionID string) (*flaresolverrSolution, error) {
	result, err := c.doRequest(ctx, flaresolverrRequest{
		CMD:        "request.get",
		URL:        targetURL,
		Session:    sessionID,
		MaxTimeout: 60000,
	})
	if err != nil {
		return nil, err
	}
	return result.Solution, nil
}

func (c *CFBypassClient) CreateSession(ctx context.Context) (string, error) {
	result, err := c.doRequest(ctx, flaresolverrRequest{
		CMD: "sessions.create",
	})
	if err != nil {
		return "", err
	}
	return result.Message, nil
}

func (c *CFBypassClient) DestroySession(ctx context.Context, sessionID string) error {
	_, err := c.doRequest(ctx, flaresolverrRequest{
		CMD:     "sessions.destroy",
		Session: sessionID,
	})
	return err
}

func (c *CFBypassClient) PostThroughBypass(ctx context.Context, targetURL string, formData url.Values, extraHeaders map[string]string) ([]byte, http.Header, error) {
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, nil, fmt.Errorf("parse URL: %w", err)
	}

	bypassURL := fmt.Sprintf("%s%s", c.baseURL, parsedURL.Path)
	if parsedURL.RawQuery != "" {
		bypassURL += "?" + parsedURL.RawQuery
	}

	req, err := http.NewRequestWithContext(ctx, "POST", bypassURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("x-hostname", parsedURL.Host)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for key, value := range extraHeaders {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read response: %w", err)
	}

	return body, resp.Header, nil
}

func (c *CFBypassClient) IsAvailable(ctx context.Context) bool {
	_, _, err := c.GetCookies(ctx, "https://codeforces.com")
	return err == nil
}
