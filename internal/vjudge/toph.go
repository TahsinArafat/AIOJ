package vjudge

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tahsinarafat/aioj/internal/store"
)

type TophBot struct {
	config     BotConfig
	client     *http.Client
	mu         sync.RWMutex
	state      BotState
	remoteLang store.RemoteLanguageStore
	loggedIn   bool
}

func NewTophBot(cfg BotConfig) *TophBot {
	jar, err := cookiejar.New(nil)
	if err != nil {
		slog.Error("toph: failed to create cookie jar", "err", err)
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
			slog.Error("toph: failed to parse base URL", "url", cfg.BaseURL, "err", parseErr)
		} else {
			var cookies []*http.Cookie
			for name, value := range cfg.Cookies {
				cookies = append(cookies, &http.Cookie{Name: name, Value: value, Domain: baseURL.Hostname(), Path: "/"})
			}
			jar.SetCookies(baseURL, cookies)
		}
	}
	bot := &TophBot{config: cfg, client: client, state: StateIdle}
	if len(cfg.Cookies) > 0 {
		bot.loggedIn = true
	}
	return bot
}

func (b *TophBot) Name() string { return "toph" }

func (b *TophBot) Configure(acc BotConfig) {
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
			baseURL, _ = url.Parse("https://toph.co")
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

func (b *TophBot) SetCookies(cookies map[string]string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.config.Cookies = cookies
	var baseURL *url.URL
	if b.config.BaseURL != "" {
		baseURL, _ = url.Parse(b.config.BaseURL)
	} else {
		baseURL, _ = url.Parse("https://toph.co")
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

func (b *TophBot) State() BotState {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

func (b *TophBot) setState(s BotState) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = s
}

func (b *TophBot) IsLoggedIn(ctx context.Context) bool {
	b.mu.RLock()
	if b.loggedIn {
		b.mu.RUnlock()
		return true
	}
	b.mu.RUnlock()

	body, err := b.fetch(ctx, b.baseURL()+"/")
	if err != nil {
		return false
	}
	return strings.Contains(body, "Logout") || strings.Contains(body, "logout")
}

func (b *TophBot) Login(ctx context.Context) (map[string]string, error) {
	loginURL := b.baseURL() + "/login"
	_, err := b.fetch(ctx, loginURL)
	if err != nil {
		return nil, fmt.Errorf("toph: fetch login page: %w", err)
	}

	form := url.Values{
		"handle":   {b.config.Username},
		"password": {b.config.Password},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", loginURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("toph: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", b.baseURL())
	req.Header.Set("Referer", loginURL)
	setBrowserHeaders(req)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("toph: login request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("toph: read login response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("toph: invalid credentials")
	}
	if strings.Contains(string(respBody), "Invalid username or password") || strings.Contains(string(respBody), "invalid") {
		return nil, fmt.Errorf("toph: invalid credentials")
	}

	b.mu.Lock()
	b.loggedIn = true
	b.mu.Unlock()

	slog.Info("toph logged in", "user", b.config.Username)
	
	cookies := make(map[string]string)
	u, _ := url.Parse(b.baseURL())
	for _, c := range b.client.Jar.Cookies(u) {
		cookies[c.Name] = c.Value
	}
	b.config.Cookies = cookies
	return cookies, nil
}

func (b *TophBot) Submit(ctx context.Context, problemID, sourceCode, language string) (string, error) {
	b.setState(StateRunning)
	defer b.setState(StateIdle)

	if b.config.Username == "" {
		return fmt.Sprintf("toph-%d", time.Now().UnixNano()), nil
	}

	langID := b.resolveLangID(ctx, language)
	submitPageURL := fmt.Sprintf("%s/p/%s", b.baseURL(), problemID)

	body, err := b.fetch(ctx, submitPageURL)
	if err != nil {
		return "", fmt.Errorf("toph: fetch submit page: %w", err)
	}

	if !strings.Contains(body, "Logout") && !strings.Contains(body, "logout") {
		if _, err := b.Login(ctx); err != nil {
			return "", fmt.Errorf("toph: login: %w", err)
		}
		body, err = b.fetch(ctx, submitPageURL)
		if err != nil {
			return "", fmt.Errorf("toph: fetch submit page after login: %w", err)
		}
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("languageId", langID); err != nil {
		return "", fmt.Errorf("toph: write language field: %w", err)
	}
	p, err := w.CreateFormFile("source", "solution.cpp")
	if err != nil {
		return "", fmt.Errorf("toph: create form file: %w", err)
	}
	if _, err := p.Write([]byte(sourceCode)); err != nil {
		return "", fmt.Errorf("toph: write source: %w", err)
	}
	w.Close()

	submitURL := fmt.Sprintf("%s/p/%s/submit", b.baseURL(), problemID)
	req, err := http.NewRequestWithContext(ctx, "POST", submitURL, &buf)
	if err != nil {
		return "", fmt.Errorf("toph: create request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Referer", submitPageURL)
	setBrowserHeaders(req)

	resp, err := b.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("toph: submit request: %w", err)
	}
	defer resp.Body.Close()

	location := resp.Header.Get("Location")
	if location != "" {
		if sid := extractTophSID(location); sid != "" {
			slog.Info("toph submitted", "problem", problemID, "remote_id", sid)
			return sid, nil
		}
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("toph: read submit response: %w", err)
	}
	if sid := extractTophSID(string(respBody)); sid != "" {
		slog.Info("toph submitted", "problem", problemID, "remote_id", sid)
		return sid, nil
	}

	return fmt.Sprintf("toph-%d", time.Now().UnixNano()), nil
}

func (b *TophBot) Poll(ctx context.Context, remoteSubmissionID string) (*RemoteResult, error) {
	if strings.HasPrefix(remoteSubmissionID, "toph-") {
		return &RemoteResult{RemoteID: remoteSubmissionID, Verdict: "PENDING", Done: false}, nil
	}

	statusURL := fmt.Sprintf("%s/s/%s", b.baseURL(), remoteSubmissionID)
	body, err := b.fetch(ctx, statusURL)
	if err != nil {
		return nil, fmt.Errorf("toph: poll status: %w", err)
	}

	return parseTophStatus(body, remoteSubmissionID), nil
}

func (b *TophBot) baseURL() string {
	if b.config.BaseURL != "" {
		return strings.TrimRight(b.config.BaseURL, "/")
	}
	return "https://toph.co"
}

func (b *TophBot) resolveLangID(ctx context.Context, localLang string) string {
	if b.remoteLang != nil {
		langs, err := b.remoteLang.ListByPlatform(ctx, "toph")
		if err == nil {
			for _, l := range langs {
				if l.LocalID == localLang && l.Enabled {
					return l.RemoteID
				}
			}
		}
	}
	lower := strings.ToLower(localLang)
	if strings.Contains(lower, "c++") || strings.Contains(lower, "cpp") || lower == "g++" {
		return "5d828f1e9d55050001e97ee4"
	}
	if strings.Contains(lower, "python") || lower == "py" {
		return "65e95623bf7f4496a585714d"
	}
	return localLang
}

func (b *TophBot) fetch(ctx context.Context, url string) (string, error) {
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
		return "", fmt.Errorf("toph: read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("toph: unexpected status %d from %s", resp.StatusCode, url)
	}

	return string(body), nil
}

// extractTophCSRFToken extracts CSRF token from name="csrf" input field
func extractTophCSRFToken(html string) string {
	idx := strings.Index(html, `name="csrf"`)
	if idx == -1 {
		return ""
	}
	idx += len(`name="csrf"`)
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

// extractTophSID extracts submission ID from URL or HTML
func extractTophSID(s string) string {
	// Try to extract from /status/12345 format
	if idx := strings.Index(s, "/status/"); idx != -1 {
		start := idx + 8
		end := strings.IndexAny(s[start:], "/\"' &\t\n")
		if end == -1 {
			return s[start:]
		}
		return s[start : start+end]
	}
	// Try to extract from /s/12345 format
	if idx := strings.Index(s, "/s/"); idx != -1 {
		start := idx + 3
		end := strings.IndexAny(s[start:], "/\"' &\t\n")
		if end == -1 {
			return s[start:]
		}
		return s[start : start+end]
	}
	// Try to extract from submission_id=12345 format
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

// parseTophStatus parses verdict, time, and memory from Toph status page HTML
func parseTophStatus(html string, remoteID string) *RemoteResult {
	result := &RemoteResult{RemoteID: remoteID}

	// Look for verdict
	verdict := extractTophVerdict(html)
	if verdict == "" {
		// Check if still running/pending
		if strings.Contains(strings.ToLower(html), "running") ||
			strings.Contains(strings.ToLower(html), "pending") ||
			strings.Contains(strings.ToLower(html), "judging") {
			return &RemoteResult{RemoteID: remoteID, Done: false, Verdict: "PENDING"}
		}
		return &RemoteResult{RemoteID: remoteID, Done: false, Verdict: "PENDING"}
	}

	result.Done = true
	result.Verdict = verdict
	result.TimeUsed = extractTophTime(html)
	result.MemoryUsed = extractTophMemory(html)
	return result
}

// extractTophVerdict extracts verdict text from HTML
func extractTophVerdict(html string) string {
	if idx := strings.Index(html, `class="label -verdict-`); idx != -1 {
		start := idx + len(`class="label -verdict-`)
		end := strings.IndexAny(html[start:], `" `)
		if end != -1 {
			verCode := html[start : start+end]
			normalized := normalizeTophVerdict(verCode)
			if normalized != "" {
				return normalized
			}
		}
	}
	// Try to find verdict in various formats
	verdictPatterns := []string{
		`class="verdict"`, `class="verdict-text"`, `class="status"`,
		`class="result"`, `class="judgement"`,
	}

	for _, pattern := range verdictPatterns {
		idx := strings.Index(html, pattern)
		if idx != -1 {
			// Extract content after the pattern
			remaining := html[idx+len(pattern):]
			// Skip past closing >
			if closeIdx := strings.Index(remaining, ">"); closeIdx != -1 {
				remaining = remaining[closeIdx+1:]
				// Find closing tag
				if endIdx := strings.IndexAny(remaining, "<"); endIdx != -1 {
					verdict := strings.TrimSpace(remaining[:endIdx])
					if verdict != "" {
						return normalizeTophVerdict(verdict)
					}
				}
			}
		}
	}

	// Try to find verdict in table cells
	lines := strings.Split(html, "\n")
	for _, line := range lines {
		if strings.Contains(line, "<td>") {
			cells := extractTableCells(line)
			for _, cell := range cells {
				normalized := normalizeTophVerdict(cell)
				if normalized != "" {
					return normalized
				}
			}
		}
	}

	return ""
}

// extractTophTime extracts time from HTML
func extractTophTime(html string) int {
	// Try to find time in various formats
	timePatterns := []string{
		`class=submview__cpu`, `class="time"`, `class="time-used"`, `class="execution-time"`,
	}

	for _, pattern := range timePatterns {
		idx := strings.Index(html, pattern)
		if idx != -1 {
			remaining := html[idx+len(pattern):]
			if closeIdx := strings.Index(remaining, ">"); closeIdx != -1 {
				remaining = remaining[closeIdx+1:]
				if endIdx := strings.IndexAny(remaining, "<"); endIdx != -1 {
					timeStr := strings.TrimSpace(remaining[:endIdx])
					if ms := parseTophTime(timeStr); ms > 0 {
						return ms
					}
				}
			}
		}
	}

	// Try to find time in table cells
	lines := strings.Split(html, "\n")
	for _, line := range lines {
		if strings.Contains(line, "<td>") {
			cells := extractTableCells(line)
			for _, cell := range cells {
				if ms := parseTophTime(cell); ms > 0 {
					return ms
				}
			}
		}
	}

	return 0
}

// extractTophMemory extracts memory from HTML
func extractTophMemory(html string) int {
	// Try to find memory in various formats
	memPatterns := []string{
		`class=submview__memory`, `class="memory"`, `class="memory-used"`, `class="peak-memory"`,
	}

	for _, pattern := range memPatterns {
		idx := strings.Index(html, pattern)
		if idx != -1 {
			remaining := html[idx+len(pattern):]
			if closeIdx := strings.Index(remaining, ">"); closeIdx != -1 {
				remaining = remaining[closeIdx+1:]
				if endIdx := strings.IndexAny(remaining, "<"); endIdx != -1 {
					memStr := strings.TrimSpace(remaining[:endIdx])
					if kb := parseTophMemory(memStr); kb > 0 {
						return kb
					}
				}
			}
		}
	}

	// Try to find memory in table cells
	lines := strings.Split(html, "\n")
	for _, line := range lines {
		if strings.Contains(line, "<td>") {
			cells := extractTableCells(line)
			for _, cell := range cells {
				if kb := parseTophMemory(cell); kb > 0 {
					return kb
				}
			}
		}
	}

	return 0
}

// normalizeTophVerdict converts Toph verdict text to standard verdict code
func normalizeTophVerdict(verdict string) string {
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

// parseTophTime parses time string like "0.25s" or "250ms" to milliseconds
func parseTophTime(s string) int {
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

// parseTophMemory parses memory string like "32768KB" or "32MB" to kilobytes
func parseTophMemory(s string) int {
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
