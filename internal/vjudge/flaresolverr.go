package vjudge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type FlareSolverrClient struct {
	baseURL    string
	httpClient *http.Client
	sessionID  string
	handle     string
}

func NewFlareSolverrClient(baseURL string) *FlareSolverrClient {
	return &FlareSolverrClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (c *FlareSolverrClient) request(ctx context.Context, req flaresolverrRequest) (*flaresolverrResponse, error) {
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

func (c *FlareSolverrClient) CreateSession(ctx context.Context) (string, error) {
	resp, err := c.request(ctx, flaresolverrRequest{
		CMD: "sessions.create",
	})
	if err != nil {
		return "", err
	}
	c.sessionID = resp.Session
	return resp.Session, nil
}

func (c *FlareSolverrClient) DestroySession(ctx context.Context) error {
	if c.sessionID == "" {
		return nil
	}
	_, err := c.request(ctx, flaresolverrRequest{
		CMD:     "sessions.destroy",
		Session: c.sessionID,
	})
	c.sessionID = ""
	return err
}

func (c *FlareSolverrClient) GetSessionID() string {
	return c.sessionID
}

func (c *FlareSolverrClient) SetSessionID(id string) {
	c.sessionID = id
}

func (c *FlareSolverrClient) Get(ctx context.Context, url string, cookies map[string]string) (*flaresolverrSolution, error) {
	var fsCookies []flaresolverrCookie
	for name, value := range cookies {
		fsCookies = append(fsCookies, flaresolverrCookie{
			Name:  name,
			Value: value,
		})
	}

	resp, err := c.request(ctx, flaresolverrRequest{
		CMD:       "request.get",
		URL:       url,
		Session:   c.sessionID,
		MaxTimeout: 60000,
		Cookies:   fsCookies,
	})
	if err != nil {
		return nil, err
	}
	return resp.Solution, nil
}

func (c *FlareSolverrClient) Post(ctx context.Context, url string, postData string, cookies map[string]string) (*flaresolverrSolution, error) {
	var fsCookies []flaresolverrCookie
	for name, value := range cookies {
		fsCookies = append(fsCookies, flaresolverrCookie{
			Name:  name,
			Value: value,
		})
	}

	resp, err := c.request(ctx, flaresolverrRequest{
		CMD:       "request.post",
		URL:       url,
		Session:   c.sessionID,
		PostData:  postData,
		MaxTimeout: 60000,
		Cookies:   fsCookies,
	})
	if err != nil {
		return nil, err
	}
	return resp.Solution, nil
}

func extractCSRF(body string) string {
	z := html.NewTokenizer(strings.NewReader(body))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		if tt == html.SelfClosingTagToken || tt == html.StartTagToken {
			name, hasAttr := z.TagName()
			if string(name) != "meta" || !hasAttr {
				continue
			}
			var isCSRF bool
			var content string
			for hasAttr {
				k, v, more := z.TagAttr()
				if strings.ToLower(string(k)) == "name" && strings.ToLower(string(v)) == "x-csrf-token" {
					isCSRF = true
				}
				if strings.ToLower(string(k)) == "content" {
					content = string(v)
				}
				hasAttr = more
			}
			if isCSRF && content != "" {
				return content
			}
		}
	}
	return ""
}

func (c *FlareSolverrClient) Login(ctx context.Context, username, password string) (map[string]string, error) {
	if c.sessionID == "" {
		_, err := c.CreateSession(ctx)
		if err != nil {
			return nil, fmt.Errorf("create session: %w", err)
		}
	}

	getSolution, err := c.Get(ctx, "https://codeforces.com/enter", nil)
	if err != nil {
		return nil, fmt.Errorf("get login page: %w", err)
	}

	csrfToken := ""
	if getSolution != nil && getSolution.Response != "" {
		csrfToken = extractCSRF(getSolution.Response)
	}

	postData := fmt.Sprintf("csrf_token=%s&action=enter&handleOrEmail=%s&password=%s&remember=on&_tta=1",
		url.QueryEscape(csrfToken),
		url.QueryEscape(username),
		url.QueryEscape(password))

	postSolution, err := c.Post(ctx, "https://codeforces.com/enter", postData, nil)
	if err != nil {
		return nil, fmt.Errorf("login request: %w", err)
	}

	if postSolution != nil && strings.Contains(postSolution.Response, "Invalid handle or password") {
		return nil, fmt.Errorf("codeforces: invalid credentials")
	}

	cookies := make(map[string]string)
	if postSolution != nil {
		for _, ck := range postSolution.Cookies {
			cookies[ck.Name] = ck.Value
		}
	}

	c.handle = username
	return cookies, nil
}

func (c *FlareSolverrClient) Submit(ctx context.Context, problemCode, sourceCode, langID string) (string, error) {
	if c.sessionID == "" {
		return "", fmt.Errorf("no session - call Login first")
	}

	subsBefore, _ := c.GetLatestSubmissionID(ctx)
	slog.Info("flaresolverr submit", "handle", c.handle, "problemCode", problemCode, "langID", langID, "subsBefore", subsBefore)

	getSolution, err := c.Get(ctx, "https://codeforces.com/problemset/submit", nil)
	if err != nil {
		return "", fmt.Errorf("get submit page: %w", err)
	}

	csrfToken := ""
	if getSolution != nil && getSolution.Response != "" {
		csrfToken = extractCSRF(getSolution.Response)
	}

	if csrfToken == "" {
		slog.Warn("flaresolverr submit: no CSRF token found, submitting anyway")
	}

	slog.Info("flaresolverr submit", "csrfToken", csrfToken)

	postData := fmt.Sprintf("csrf_token=%s&action=submitSolutionFormSubmitted&submittedProblemCode=%s&programTypeId=%s&source=%s&sourceCodeConfirmed=true&tabSize=4",
		url.QueryEscape(csrfToken),
		url.QueryEscape(problemCode),
		url.QueryEscape(langID),
		url.QueryEscape(sourceCode))

	postSolution, err := c.Post(ctx, "https://codeforces.com/problemset/submit", postData, nil)
	if err != nil {
		return "", fmt.Errorf("submit request: %w", err)
	}

	if postSolution != nil {
		slog.Info("flaresolverr submit POST", "url", postSolution.URL, "status", postSolution.Status)
	}

	for i := 0; i < 5; i++ {
		time.Sleep(2 * time.Second)
		subsAfter, err := c.GetLatestSubmissionID(ctx)
		slog.Info("flaresolverr submit check", "attempt", i+1, "subsAfter", subsAfter, "err", err)
		if err != nil {
			continue
		}
		if subsAfter != "" && subsAfter != subsBefore {
			return subsAfter, nil
		}
	}

	return "", fmt.Errorf("could not determine submission ID after submit")
}

func (c *FlareSolverrClient) GetLatestSubmissionID(ctx context.Context) (string, error) {
	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Get("https://codeforces.com/api/user.status?handle=" + url.QueryEscape(c.handle) + "&count=1")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var parsed struct {
		Status string                   `json:"status"`
		Result []map[string]interface{} `json:"result"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parse API response: %w", err)
	}
	if parsed.Status != "OK" || len(parsed.Result) == 0 {
		return "", nil
	}

	if id, ok := parsed.Result[0]["id"]; ok {
		return fmt.Sprintf("%.0f", id), nil
	}
	return "", nil
}

func (c *FlareSolverrClient) GetSubmissions(ctx context.Context, handle string) (string, error) {
	solution, err := c.Get(ctx, fmt.Sprintf("https://codeforces.com/submissions/%s", handle), nil)
	if err != nil {
		return "", err
	}
	return solution.Response, nil
}
