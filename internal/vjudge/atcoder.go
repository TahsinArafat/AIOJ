package vjudge

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"
)

type AtCoderBot struct {
	config   BotConfig
	client   *http.Client
	mu       sync.RWMutex
	state    BotState
	loggedIn bool
}

func NewAtCoderBot(cfg BotConfig) *AtCoderBot {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Timeout: 30 * time.Second,
		Jar:     jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	if len(cfg.Cookies) > 0 && cfg.BaseURL != "" {
		baseURL, _ := url.Parse(cfg.BaseURL)
		var cookies []*http.Cookie
		for name, value := range cfg.Cookies {
			cookies = append(cookies, &http.Cookie{Name: name, Value: value, Domain: baseURL.Hostname(), Path: "/"})
		}
		jar.SetCookies(baseURL, cookies)
	}
	bot := &AtCoderBot{config: cfg, client: client, state: StateIdle}
	if len(cfg.Cookies) > 0 {
		bot.loggedIn = true
	}
	return bot
}

func (b *AtCoderBot) Name() string { return "atcoder" }

func (b *AtCoderBot) Configure(acc BotConfig) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.config.Username = acc.Username
	b.config.Password = acc.Password
	b.config.ProxyURL = acc.ProxyURL
	b.config.ProxyEnabled = acc.ProxyEnabled
	b.config.Cookies = acc.Cookies
	
	if len(acc.Cookies) > 0 {
		var baseURL *url.URL
		if b.config.BaseURL != "" {
			baseURL, _ = url.Parse(b.config.BaseURL)
		} else {
			baseURL, _ = url.Parse("https://atcoder.jp")
		}
		if baseURL != nil {
			var cookies []*http.Cookie
			for name, value := range acc.Cookies {
				cookies = append(cookies, &http.Cookie{Name: name, Value: value, Domain: baseURL.Hostname(), Path: "/"})
			}
			b.client.Jar.SetCookies(baseURL, cookies)
		}
		b.loggedIn = true
	}
}

func (b *AtCoderBot) SetCookies(cookies map[string]string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.config.Cookies = cookies
	var baseURL *url.URL
	if b.config.BaseURL != "" {
		baseURL, _ = url.Parse(b.config.BaseURL)
	} else {
		baseURL, _ = url.Parse("https://atcoder.jp")
	}
	if baseURL != nil {
		var cookiesList []*http.Cookie
		for name, value := range cookies {
			cookiesList = append(cookiesList, &http.Cookie{Name: name, Value: value, Domain: baseURL.Hostname(), Path: "/"})
		}
		b.client.Jar.SetCookies(baseURL, cookiesList)
	}
	b.loggedIn = true
}

func (b *AtCoderBot) State() BotState {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

func (b *AtCoderBot) setState(s BotState) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = s
}

func (b *AtCoderBot) IsLoggedIn(ctx context.Context) bool {
	b.mu.RLock()
	if b.loggedIn {
		b.mu.RUnlock()
		return true
	}
	b.mu.RUnlock()
	return false
}

func (b *AtCoderBot) Login(ctx context.Context) (map[string]string, error) {
	// AtCoder uses Cloudflare Turnstile which blocks automated login
	// Cookies must be provided manually by the user
	if len(b.config.Cookies) == 0 {
		return nil, fmt.Errorf("atcoder: cookies required (Turnstile blocks automated login)")
	}
	b.mu.Lock()
	b.loggedIn = true
	b.mu.Unlock()
	slog.Info("atcoder logged in via cookies")
	return nil, nil
}

func (b *AtCoderBot) Submit(ctx context.Context, problemID, sourceCode, language string) (string, error) {
	b.setState(StateRunning)
	defer b.setState(StateIdle)

	// Parse contest ID from problem ID (e.g., abc300_a -> abc300)
	contestID, err := parseAtCoderContestID(problemID)
	if err != nil {
		return "", fmt.Errorf("atcoder: %w", err)
	}

	// Check if logged in
	if !b.IsLoggedIn(ctx) {
		return "", fmt.Errorf("atcoder: not logged in (cookies required)")
	}

	// Fetch submit page to get CSRF token
	submitPageURL := fmt.Sprintf("%s/contests/%s/submit", b.baseURL(), contestID)
	body, err := b.fetch(ctx, submitPageURL)
	if err != nil {
		return "", fmt.Errorf("atcoder: fetch submit page: %w", err)
	}

	csrf := extractAtCoderCSRFToken(body)
	if csrf == "" {
		return "", fmt.Errorf("atcoder: no CSRF token found")
	}

	// Prepare form data
	form := url.Values{
		"csrf_token":          {csrf},
		"data.TaskScreenName": {problemID},
		"data.LanguageId":     {language},
		"sourceCode":          {sourceCode},
	}

	// Submit (prevent redirects to inspect Location header)
	req, err := http.NewRequestWithContext(ctx, "POST", submitPageURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("atcoder: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", submitPageURL)
	setBrowserHeaders(req)

	resp, err := b.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("atcoder: submit request: %w", err)
	}
	defer resp.Body.Close()

	// Check for redirect to submissions page
	location := resp.Header.Get("Location")
	if location != "" && strings.Contains(location, "/submissions/me") {
		// Extract submission ID from redirect URL
		// Format: /contests/{contest_id}/submissions/me
		remoteID := contestID + "/me"
		slog.Info("atcoder submitted", "problem", problemID, "remote_id", remoteID)
		return remoteID, nil
	}

	return "", fmt.Errorf("atcoder: unexpected response from submit endpoint")
}

func (b *AtCoderBot) Poll(ctx context.Context, remoteSubmissionID string) (*RemoteResult, error) {
	// Parse remote submission ID to get contest ID
	// Format: {contest_id}/me or {contest_id}/me/{submission_number}
	parts := strings.SplitN(remoteSubmissionID, "/me", 2)
	if len(parts) == 0 {
		return &RemoteResult{RemoteID: remoteSubmissionID, Done: false, Verdict: "PENDING"}, nil
	}
	contestID := parts[0]

	// Fetch submissions page for the contest
	submissionsURL := fmt.Sprintf("%s/contests/%s/submissions/me", b.baseURL(), contestID)
	body, err := b.fetch(ctx, submissionsURL)
	if err != nil {
		return &RemoteResult{RemoteID: remoteSubmissionID, Done: false, Verdict: "PENDING"}, nil
	}

	// Parse the submissions page to find the latest submission
	return parseAtCoderSubmissions(body, remoteSubmissionID), nil
}

func (b *AtCoderBot) baseURL() string {
	if b.config.BaseURL != "" {
		return strings.TrimRight(b.config.BaseURL, "/")
	}
	return "https://atcoder.jp"
}

func (b *AtCoderBot) fetch(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	setBrowserHeaders(req)
	resp, err := b.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("atcoder: read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("atcoder: unexpected status %d from %s", resp.StatusCode, url)
	}

	return string(body), nil
}

// parseAtCoderContestID extracts contest ID from problem ID (e.g., abc300_a -> abc300)
func parseAtCoderContestID(problemID string) (string, error) {
	parts := strings.SplitN(problemID, "_", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid problem ID format: %s (expected contest_problem)", problemID)
	}
	return parts[0], nil
}

// extractAtCoderCSRFToken extracts CSRF token from input name="csrf_token"
func extractAtCoderCSRFToken(html string) string {
	idx := strings.Index(html, `name="csrf_token"`)
	if idx == -1 {
		return ""
	}
	idx += len(`name="csrf_token"`)
	valIdx := strings.Index(html[idx:], `value="`)
	if valIdx == -1 {
		return ""
	}
	start := idx + valIdx + 7
	end := strings.Index(html[start:], `"`)
	if end == -1 {
		return ""
	}
	return html[start : start+end]
}

// parseAtCoderSubmissions parses the submissions page HTML to extract the latest submission status
func parseAtCoderSubmissions(html string, remoteID string) *RemoteResult {
	// Look for verdict spans
	result := &RemoteResult{RemoteID: remoteID}

	// Parse verdict from span classes
	if strings.Contains(html, "judge-result-success") {
		result.Done = true
		result.Verdict = "AC"
	} else if strings.Contains(html, "judge-result-danger") {
		result.Done = true
		result.Verdict = "WA"
	} else if strings.Contains(html, "judge-result-warning") {
		// Could be TLE or other time/memory limit
		if strings.Contains(html, "TLE") {
			result.Verdict = "TLE"
		} else {
			result.Verdict = "TLE"
		}
		result.Done = true
	} else if strings.Contains(html, "judge-result-info") {
		result.Done = true
		result.Verdict = "RE"
	} else {
		// Still pending/judging
		return &RemoteResult{RemoteID: remoteID, Done: false, Verdict: "PENDING"}
	}

	// Parse time and memory from table cells
	// Look for patterns like "200 ms" and "8192 KB"
	lines := strings.Split(html, "\n")
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "ms") {
			// Extract time
			timeStr := extractTimeValue(line)
			if timeStr > 0 {
				result.TimeUsed = timeStr
			}
		}
		if strings.Contains(lower, "kb") {
			// Extract memory
			memStr := extractMemoryValue(line)
			if memStr > 0 {
				result.MemoryUsed = memStr
			}
		}
	}

	return result
}

// extractTimeValue extracts time value from HTML (e.g., "200 ms" -> 200)
func extractTimeValue(html string) int {
	idx := strings.Index(html, " ms")
	if idx == -1 {
		return 0
	}
	// Find the start of the number
	start := idx - 1
	for start >= 0 && html[start] >= '0' && html[start] <= '9' {
		start--
	}
	start++

	var val int
	fmt.Sscanf(html[start:idx], "%d", &val)
	return val
}

// extractMemoryValue extracts memory value from HTML (e.g., "8192 KB" -> 8192)
func extractMemoryValue(html string) int {
	idx := strings.Index(html, " KB")
	if idx == -1 {
		return 0
	}
	// Find the start of the number
	start := idx - 1
	for start >= 0 && html[start] >= '0' && html[start] <= '9' {
		start--
	}
	start++

	var val int
	fmt.Sscanf(html[start:idx], "%d", &val)
	return val
}
