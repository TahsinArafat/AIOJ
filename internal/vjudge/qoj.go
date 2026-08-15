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

	"github.com/tahsinarafat/aioj/internal/store"
)

type QOJBot struct {
	config     BotConfig
	client     *http.Client
	mu         sync.RWMutex
	state      BotState
	remoteLang store.RemoteLanguageStore
	loggedIn   bool
}

func NewQOJBot(cfg BotConfig) *QOJBot {
	jar, err := cookiejar.New(nil)
	if err != nil {
		slog.Error("qoj: failed to create cookie jar", "err", err)
		jar, _ = cookiejar.New(nil)
	}
	client := &http.Client{
		Timeout: 30 * time.Second,
		Jar:     jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	if len(cfg.Cookies) > 0 && cfg.BaseURL != "" {
		baseURL, parseErr := url.Parse(cfg.BaseURL)
		if parseErr != nil {
			slog.Error("qoj: failed to parse base URL", "url", cfg.BaseURL, "err", parseErr)
		} else {
			var cookies []*http.Cookie
			for name, value := range cfg.Cookies {
				cookies = append(cookies, &http.Cookie{Name: name, Value: value, Domain: baseURL.Hostname(), Path: "/"})
			}
			jar.SetCookies(baseURL, cookies)
		}
	}
	bot := &QOJBot{config: cfg, client: client, state: StateIdle}
	if len(cfg.Cookies) > 0 {
		bot.loggedIn = true
	}
	return bot
}

func (b *QOJBot) Name() string { return "qoj" }

func (b *QOJBot) Configure(acc BotConfig) {
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
			baseURL, _ = url.Parse("https://qoj.ac")
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

func (b *QOJBot) SetCookies(cookies map[string]string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.config.Cookies = cookies
	var baseURL *url.URL
	if b.config.BaseURL != "" {
		baseURL, _ = url.Parse(b.config.BaseURL)
	} else {
		baseURL, _ = url.Parse("https://qoj.ac")
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

func (b *QOJBot) State() BotState {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

func (b *QOJBot) setState(s BotState) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = s
}

func (b *QOJBot) IsLoggedIn(ctx context.Context) bool {
	if b.loggedIn {
		return true
	}
	body, err := b.fetch(ctx, b.baseURL()+"/submissions")
	if err != nil {
		return false
	}
	return strings.Contains(body, "Logout") || strings.Contains(body, "logout")
}

func (b *QOJBot) Login(ctx context.Context) (map[string]string, error) {
	loginURL := b.baseURL() + "/login"
	body, err := b.fetch(ctx, loginURL)
	if err != nil {
		return nil, fmt.Errorf("qoj: fetch login page: %w", err)
	}

	csrf := extractQOJToken(body)
	if csrf == "" {
		return nil, fmt.Errorf("qoj: no CSRF token found")
	}

	form := url.Values{
		"_token":    {csrf},
		"user_name": {b.config.Username},
		"password":  {b.config.Password},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", loginURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("qoj: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", b.baseURL())
	req.Header.Set("Referer", loginURL)
	setBrowserHeaders(req)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qoj: login request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("qoj: read login response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("qoj: invalid credentials")
	}
	if strings.Contains(string(respBody), "Invalid username or password") {
		return nil, fmt.Errorf("qoj: invalid credentials")
	}

	b.mu.Lock()
	b.loggedIn = true
	b.mu.Unlock()

	slog.Info("qoj logged in", "user", b.config.Username)
	return nil, nil
}

func (b *QOJBot) Submit(ctx context.Context, problemID, sourceCode, language string) (string, error) {
	b.setState(StateRunning)
	defer b.setState(StateIdle)

	if b.config.Username == "" {
		return fmt.Sprintf("qoj-%d", time.Now().UnixNano()), nil
	}

	langID := b.resolveLangID(ctx, language)
	submitPageURL := fmt.Sprintf("%s/problem/%s", b.baseURL(), problemID)

	body, err := b.fetch(ctx, submitPageURL)
	if err != nil {
		return "", fmt.Errorf("qoj: fetch submit page: %w", err)
	}

	if !strings.Contains(body, "Logout") && !strings.Contains(body, "logout") {
		if _, err := b.Login(ctx); err != nil {
			return "", fmt.Errorf("qoj: login: %w", err)
		}
		body, err = b.fetch(ctx, submitPageURL)
		if err != nil {
			return "", fmt.Errorf("qoj: fetch submit page after login: %w", err)
		}
	}

	csrf := extractQOJToken(body)
	if csrf == "" {
		return "", fmt.Errorf("qoj: no CSRF token")
	}

	form := url.Values{
		"_token":      {csrf},
		"answer":      {sourceCode},
		"language":    {langID},
	}

	submitURL := fmt.Sprintf("%s/problem/%s/submit", b.baseURL(), problemID)
	req, err := http.NewRequestWithContext(ctx, "POST", submitURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("qoj: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", submitPageURL)
	setBrowserHeaders(req)

	resp, err := b.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("qoj: submit request: %w", err)
	}
	defer resp.Body.Close()

	location := resp.Header.Get("Location")
	if location != "" {
		if sid := extractQOJSid(location); sid != "" {
			slog.Info("qoj submitted", "problem", problemID, "remote_id", sid)
			return sid, nil
		}
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("qoj: read submit response: %w", err)
	}
	if sid := extractQOJSid(string(respBody)); sid != "" {
		slog.Info("qoj submitted", "problem", problemID, "remote_id", sid)
		return sid, nil
	}

	return fmt.Sprintf("qoj-%d", time.Now().UnixNano()), nil
}

func (b *QOJBot) Poll(ctx context.Context, remoteSubmissionID string) (*RemoteResult, error) {
	if strings.HasPrefix(remoteSubmissionID, "qoj-") {
		return &RemoteResult{RemoteID: remoteSubmissionID, Verdict: "PENDING", Done: false}, nil
	}

	statusURL := fmt.Sprintf("%s/submission/%s", b.baseURL(), remoteSubmissionID)
	body, err := b.fetch(ctx, statusURL)
	if err != nil {
		return nil, fmt.Errorf("qoj: poll status: %w", err)
	}

	return parseQOJStatus(body, remoteSubmissionID), nil
}

func (b *QOJBot) baseURL() string {
	if b.config.BaseURL != "" {
		return strings.TrimRight(b.config.BaseURL, "/")
	}
	return "http://localhost"
}

func (b *QOJBot) resolveLangID(ctx context.Context, localLang string) string {
	if b.remoteLang != nil {
		langs, err := b.remoteLang.ListByPlatform(ctx, "qoj")
		if err == nil {
			for _, l := range langs {
				if l.LocalID == localLang && l.Enabled {
					return l.RemoteID
				}
			}
		}
	}
	return "0"
}

func (b *QOJBot) fetch(ctx context.Context, url string) (string, error) {
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
		return "", fmt.Errorf("qoj: read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("qoj: unexpected status %d from %s", resp.StatusCode, url)
	}

	return string(body), nil
}

// extractQOJToken extracts CSRF token from name="_token" input field
func extractQOJToken(html string) string {
	idx := strings.Index(html, `name="_token"`)
	if idx == -1 {
		return ""
	}
	idx += len(`name="_token"`)
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

// extractQOJSid extracts submission ID from URL or HTML
func extractQOJSid(s string) string {
	if idx := strings.Index(s, "/submission/"); idx != -1 {
		start := idx + 12
		end := strings.IndexAny(s[start:], "/\"' &\t\n")
		if end == -1 {
			return s[start:]
		}
		return s[start : start+end]
	}
	if idx := strings.Index(s, "sid="); idx != -1 {
		start := idx + 4
		end := strings.IndexAny(s[start:], "/\"' &\t\n")
		if end == -1 {
			return s[start:]
		}
		return s[start : start+end]
	}
	if idx := strings.Index(s, "submission_id="); idx != -1 {
		start := idx + 14
		end := strings.IndexAny(s[start:], "/\"' &\t\n")
		if end == -1 {
			return s[start:]
		}
		return s[start : start+end]
	}
	return ""
}

// parseQOJStatus parses verdict, time, and memory from QOJ status page HTML
func parseQOJStatus(html string, remoteID string) *RemoteResult {
	result := &RemoteResult{RemoteID: remoteID}

	lines := strings.Split(html, "\n")
	for _, line := range lines {
		lower := strings.ToLower(line)

		if strings.Contains(lower, "<td>") {
			timeVal, memVal, verdict := parseQOJTableRow(line)
			if verdict != "" {
				result.Done = true
				result.Verdict = verdict
				result.TimeUsed = timeVal
				result.MemoryUsed = memVal
				return result
			}
		}
	}

	if strings.Contains(strings.ToLower(html), "running") ||
		strings.Contains(strings.ToLower(html), "pending") ||
		strings.Contains(strings.ToLower(html), "judging") {
		return &RemoteResult{RemoteID: remoteID, Done: false, Verdict: "PENDING"}
	}

	return &RemoteResult{RemoteID: remoteID, Done: false, Verdict: "PENDING"}
}

// parseQOJTableRow parses a table row and extracts verdict, time, memory
func parseQOJTableRow(row string) (timeMs int, memKB int, verdict string) {
	cells := extractTableCells(row)
	if len(cells) < 3 {
		return 0, 0, ""
	}

	verdictStr := strings.TrimSpace(cells[1])
	verdict = normalizeQOJVerdict(verdictStr)

	if verdict == "" {
		return 0, 0, ""
	}

	timeStr := strings.TrimSpace(cells[2])
	timeMs = parseQOJTime(timeStr)

	memStr := ""
	if len(cells) > 3 {
		memStr = strings.TrimSpace(cells[3])
		memKB = parseQOJMemory(memStr)
	}

	return timeMs, memKB, verdict
}

// extractTableCells extracts content between <td> tags
func extractTableCells(html string) []string {
	var cells []string
	remaining := html
	for {
		idx := strings.Index(remaining, "<td>")
		if idx == -1 {
			break
		}
		start := idx + 4
		endIdx := strings.Index(remaining[start:], "</td>")
		if endIdx == -1 {
			break
		}
		cell := remaining[start : start+endIdx]
		cell = stripHTMLTags(cell)
		cells = append(cells, cell)
		remaining = remaining[start+endIdx+5:]
	}
	return cells
}

// stripHTMLTags removes HTML tags from a string
func stripHTMLTags(s string) string {
	var sb strings.Builder
	inTag := false
	for _, ch := range s {
		if ch == '<' {
			inTag = true
			continue
		}
		if ch == '>' {
			inTag = false
			continue
		}
		if !inTag {
			sb.WriteRune(ch)
		}
	}
	return sb.String()
}

// normalizeQOJVerdict converts QOJ verdict text to standard verdict code
func normalizeQOJVerdict(verdict string) string {
	verdict = strings.TrimSpace(verdict)
	verdictLower := strings.ToLower(verdict)

	switch {
	case strings.Contains(verdictLower, "accepted") || verdictLower == "ac":
		return "AC"
	case strings.Contains(verdictLower, "wrong answer") || verdictLower == "wa":
		return "WA"
	case strings.Contains(verdictLower, "time limit") || verdictLower == "tle":
		return "TLE"
	case strings.Contains(verdictLower, "runtime error") || verdictLower == "re":
		return "RE"
	case strings.Contains(verdictLower, "compilation error") || verdictLower == "ce":
		return "CE"
	case strings.Contains(verdictLower, "memory limit") || verdictLower == "mle":
		return "MLE"
	case strings.Contains(verdictLower, "presentation error") || verdictLower == "pe":
		return "PE"
	case verdictLower == "running" || verdictLower == "pending" || verdictLower == "judging":
		return ""
	case verdictLower == "submitted":
		return ""
	}
	return ""
}

// parseQOJTime parses time string like "0.25s" or "250ms" to milliseconds
func parseQOJTime(s string) int {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0
	}
	s = strings.ToLower(s)

	if strings.HasSuffix(s, "ms") {
		var val float64
		fmt.Sscanf(strings.TrimSuffix(s, "ms"), "%f", &val)
		return int(val)
	}
	if strings.HasSuffix(s, "s") {
		var val float64
		fmt.Sscanf(strings.TrimSuffix(s, "s"), "%f", &val)
		return int(val * 1000)
	}

	var val float64
	fmt.Sscanf(s, "%f", &val)
	if val > 0 && val < 100 {
		return int(val * 1000)
	}
	return int(val)
}

// parseQOJMemory parses memory string like "32768KB" or "32MB" to kilobytes
func parseQOJMemory(s string) int {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0
	}
	s = strings.ToLower(s)

	if strings.HasSuffix(s, "kb") {
		var val float64
		fmt.Sscanf(strings.TrimSuffix(s, "kb"), "%f", &val)
		return int(val)
	}
	if strings.HasSuffix(s, "mb") {
		var val float64
		fmt.Sscanf(strings.TrimSuffix(s, "mb"), "%f", &val)
		return int(val * 1024)
	}
	if strings.HasSuffix(s, "gb") {
		var val float64
		fmt.Sscanf(strings.TrimSuffix(s, "gb"), "%f", &val)
		return int(val * 1024 * 1024)
	}

	var val float64
	fmt.Sscanf(s, "%f", &val)
	return int(val)
}

// FetchLanguages implements Bot.FetchLanguages.
// QOJBot does not support remote language enumeration.
func (b *QOJBot) FetchLanguages(ctx context.Context) ([]RemoteLanguageItem, error) {
	return nil, nil
}
