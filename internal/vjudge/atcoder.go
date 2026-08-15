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
	config       BotConfig
	client       *http.Client
	submitClient *AtCoderSubmitClient
	mu           sync.RWMutex
	state        BotState
	loggedIn     bool
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

func NewAtCoderBotWithSubmit(cfg BotConfig, submitClient *AtCoderSubmitClient) *AtCoderBot {
	bot := NewAtCoderBot(cfg)
	bot.submitClient = submitClient
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
	b.mu.RLock()
	cookies := b.config.Cookies
	b.mu.RUnlock()

	if len(cookies) > 0 {
		b.mu.Lock()
		b.loggedIn = true
		b.mu.Unlock()
		slog.Info("atcoder logged in via stored cookies")
		return cookies, nil
	}

	if b.submitClient != nil && b.config.Username != "" && b.config.Password != "" {
		cookies, err := b.submitClient.Login(ctx, b.config.Username, b.config.Password)
		if err != nil {
			return nil, fmt.Errorf("atcoder: submit service login: %w", err)
		}
		b.SetCookies(cookies)
		slog.Info("atcoder: logged in via submit service", "cookies", len(cookies))
		return cookies, nil
	}

	return nil, fmt.Errorf("atcoder: no credentials — set username/password, or provide cookies")
}

func (b *AtCoderBot) Submit(ctx context.Context, problemID, sourceCode, language string) (string, error) {
	b.setState(StateRunning)
	defer b.setState(StateIdle)

	contestID, err := parseAtCoderContestID(problemID)
	if err != nil {
		return "", fmt.Errorf("atcoder: %w", err)
	}

	if !b.IsLoggedIn(ctx) {
		if _, err := b.Login(ctx); err != nil {
			return "", fmt.Errorf("atcoder: not logged in: %w", err)
		}
	}

	langID := resolveAtCoderLangID(language)

	if b.submitClient != nil && b.config.Username != "" && b.config.Password != "" {
		remoteID, err := b.submitClient.Submit(ctx, contestID, problemID, sourceCode, langID, b.config.Username, b.config.Password)
		if err != nil {
			slog.Error("atcoder: submit service failed", "err", err)
		} else if remoteID != "" {
			formattedID := contestID + "/" + remoteID
			slog.Info("atcoder submitted via submit service", "problem", problemID, "remote_id", formattedID)
			return formattedID, nil
		}
	}

	submitPageURL := fmt.Sprintf("%s/contests/%s/submit", b.baseURL(), contestID)

	body, err := b.fetch(ctx, submitPageURL)
	if err == nil {
		csrf := extractAtCoderCSRFToken(body)
		if csrf != "" {
			form := url.Values{
				"csrf_token":          {csrf},
				"data.TaskScreenName": {problemID},
				"data.LanguageId":     {langID},
				"sourceCode":          {sourceCode},
			}

			req, err := http.NewRequestWithContext(ctx, "POST", submitPageURL, strings.NewReader(form.Encode()))
			if err == nil {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Set("Referer", submitPageURL)
				setBrowserHeaders(req)

				resp, err := b.client.Do(req)
				if err == nil {
					defer resp.Body.Close()
					location := resp.Header.Get("Location")
					respBody, _ := io.ReadAll(resp.Body)
					combined := string(respBody) + location

					slog.Info("atcoder: direct submit result",
						"status", resp.StatusCode,
						"location", location,
						"respLen", len(respBody))

					if subID := extractAtCoderSubmissionID(location); subID != "" {
						remoteID := contestID + "/" + subID
						slog.Info("atcoder submitted", "problem", problemID, "remote_id", remoteID)
						return remoteID, nil
					}

					if subID := extractAtCoderLatestSubmissionID(combined); subID != "" {
						remoteID := contestID + "/" + subID
						slog.Info("atcoder submitted", "problem", problemID, "remote_id", remoteID)
						return remoteID, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("atcoder: submit failed via all methods")
}

func resolveAtCoderLangID(language string) string {
	switch language {
	case "go", "go1", "Go", "Go1":
		return "5001"
	case "python", "python3", "Python", "Python3", "py":
		return "5028"
	case "cpp", "c++", "C++":
		return "5001"
	case "java", "Java":
		return "5002"
	case "c", "C":
		return "5003"
	case "rust", "Rust":
		return "5014"
	case "javascript", "js", "JavaScript", "Node.js":
		return "5013"
	default:
		return "5001"
	}
}

func (b *AtCoderBot) Poll(ctx context.Context, remoteSubmissionID string) (*RemoteResult, error) {
	parts := strings.SplitN(remoteSubmissionID, "/", 2)
	if len(parts) != 2 {
		return &RemoteResult{RemoteID: remoteSubmissionID, Done: false, Verdict: "PENDING"}, nil
	}
	contestID := parts[0]
	submissionID := parts[1]

	detailURL := fmt.Sprintf("%s/contests/%s/submissions/%s", b.baseURL(), contestID, submissionID)

	body, err := b.fetch(ctx, detailURL)
	if err != nil {
		return &RemoteResult{RemoteID: remoteSubmissionID, Done: false, Verdict: "PENDING"}, nil
	}

	return parseAtCoderSubmissionDetail(body, remoteSubmissionID), nil
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

func parseAtCoderContestID(problemID string) (string, error) {
	parts := strings.SplitN(problemID, "_", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid problem ID format: %s (expected contest_problem)", problemID)
	}
	return parts[0], nil
}

func extractAtCoderCSRFToken(html string) string {
	return extractHiddenField(html, "csrf_token")
}

func extractHiddenField(html, name string) string {
	pattern := `name="` + name + `"`
	idx := strings.Index(html, pattern)
	if idx == -1 {
		return ""
	}
	idx += len(pattern)
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

func extractAtCoderSubmissionID(s string) string {
	if s == "" {
		return ""
	}
	idx := strings.LastIndex(s, "/submissions/")
	if idx != -1 {
		id := s[idx+len("/submissions/"):]
		id = strings.TrimRight(id, "/ \n\r\t")
		for _, c := range id {
			if c < '0' || c > '9' {
				return ""
			}
		}
		if len(id) > 0 {
			return id
		}
	}
	return ""
}

func extractAtCoderLatestSubmissionID(html string) string {
	idx := strings.LastIndex(html, "/submissions/")
	if idx == -1 {
		return ""
	}
	id := html[idx+len("/submissions/"):]
	end := strings.IndexAny(id, "\"' <")
	if end != -1 {
		id = id[:end]
	}
	for _, c := range id {
		if c < '0' || c > '9' {
			return ""
		}
	}
	return id
}

func parseAtCoderSubmissionDetail(html string, remoteID string) *RemoteResult {
	result := &RemoteResult{RemoteID: remoteID}

	if strings.Contains(html, "No such submission") || strings.Contains(html, "404 Not Found") {
		result.Done = true
		result.Verdict = "CE"
		return result
	}

	if strings.Contains(html, "judge-result-success") || strings.Contains(html, ">AC<") || strings.Contains(html, ">Accepted<") {
		result.Done = true
		result.Verdict = "AC"
	} else if strings.Contains(html, "judge-result-danger") || strings.Contains(html, ">WA<") || strings.Contains(html, ">Wrong Answer<") {
		result.Done = true
		result.Verdict = "WA"
	} else if strings.Contains(html, "judge-result-warning") {
		if strings.Contains(html, ">TLE<") || strings.Contains(html, ">Time Limit Exceeded<") {
			result.Verdict = "TLE"
		} else if strings.Contains(html, ">MLE<") || strings.Contains(html, ">Memory Limit Exceeded<") {
			result.Verdict = "MLE"
		} else {
			result.Verdict = "TLE"
		}
		result.Done = true
	} else if strings.Contains(html, "judge-result-info") || strings.Contains(html, ">RE<") || strings.Contains(html, ">Runtime Error<") {
		result.Done = true
		result.Verdict = "RE"
	} else if strings.Contains(html, ">CE<") || strings.Contains(html, ">Compilation Error<") {
		result.Done = true
		result.Verdict = "CE"
	} else if strings.Contains(html, ">OLE<") || strings.Contains(html, ">Output Limit Exceeded<") {
		result.Done = true
		result.Verdict = "OLE"
	} else if strings.Contains(html, ">WJ<") || strings.Contains(html, ">Waiting for Judging<") || strings.Contains(html, ">judging<") {
		return &RemoteResult{RemoteID: remoteID, Done: false, Verdict: "PENDING"}
	} else {
		return &RemoteResult{RemoteID: remoteID, Done: false, Verdict: "PENDING"}
	}

	lines := strings.Split(html, "\n")
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "ms") {
			timeStr := extractTimeValue(line)
			if timeStr > 0 {
				result.TimeUsed = timeStr
			}
		}
		if strings.Contains(lower, "kb") {
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

// FetchLanguages implements Bot.FetchLanguages.
func (b *AtCoderBot) FetchLanguages(ctx context.Context) ([]RemoteLanguageItem, error) {
	if b.submitClient != nil {
		return b.submitClient.FetchLanguages(ctx)
	}
	return nil, nil
}
