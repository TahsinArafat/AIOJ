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
	"time"

	"github.com/tahsinarafat/aioj/internal/store"
)

type CSESBot struct {
	config     BotConfig
	client     *http.Client
	state      BotState
	remoteLang store.RemoteLanguageStore
	loggedIn   bool
}

func NewCSESBotWithStore(cfg BotConfig, remoteLang store.RemoteLanguageStore) *CSESBot {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Timeout: 30 * time.Second, Jar: jar}
	if len(cfg.Cookies) > 0 {
		csesURL, _ := url.Parse("https://cses.fi")
		var cookies []*http.Cookie
		for name, value := range cfg.Cookies {
			cookies = append(cookies, &http.Cookie{Name: name, Value: value, Domain: "cses.fi", Path: "/"})
		}
		jar.SetCookies(csesURL, cookies)
	}
	bot := &CSESBot{config: cfg, client: client, state: StateIdle, remoteLang: remoteLang}
	if len(cfg.Cookies) > 0 {
		bot.loggedIn = true
	}
	return bot
}

func (b *CSESBot) Name() string    { return "cses" }
func (b *CSESBot) State() BotState { return b.state }

func (b *CSESBot) IsLoggedIn(ctx context.Context) bool {
	if b.loggedIn {
		return true
	}
	body, err := b.fetch(ctx, "https://cses.fi/problemset/")
	if err != nil {
		return false
	}
	return strings.Contains(body, "Logout")
}

func (b *CSESBot) Login(ctx context.Context) (map[string]string, error) {
	loginURL := "https://cses.fi/login"
	body, err := b.fetch(ctx, loginURL)
	if err != nil {
		return nil, err
	}
	csrf := extractCSRFCookie(body)
	if csrf == "" {
		return nil, fmt.Errorf("cses: no CSRF token")
	}

	urlValues := url.Values{
		"csrf_token": {csrf},
		"nick":       {b.config.Username},
		"pass":       {b.config.Password},
	}
	req, _ := http.NewRequestWithContext(ctx, "POST", loginURL, strings.NewReader(urlValues.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://cses.fi")
	req.Header.Set("Referer", loginURL)
	setBrowserHeaders(req)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if strings.Contains(string(respBody), "Invalid username") {
		return nil, fmt.Errorf("cses: invalid credentials")
	}

	b.loggedIn = true
	slog.Info("cses logged in", "user", b.config.Username)
	return nil, nil
}

func (b *CSESBot) Submit(ctx context.Context, problemID, sourceCode, language string) (string, error) {
	b.state = StateRunning
	defer func() { b.state = StateIdle }()

	if b.config.Username == "" {
		return fmt.Sprintf("cses-%d", time.Now().UnixNano()), nil
	}

	langID := b.resolveLangID(language)
	submitPageURL := fmt.Sprintf("https://cses.fi/problemset/submit/%s/", problemID)

	body, err := b.fetch(ctx, submitPageURL)
	if err != nil {
		return "", err
	}

	if !strings.Contains(body, "Logout") {
		if _, err := b.Login(ctx); err != nil {
			return "", fmt.Errorf("login: %w", err)
		}
		body, _ = b.fetch(ctx, submitPageURL)
	}

	csrf := extractCSRFCookie(body)
	if csrf == "" {
		return "", fmt.Errorf("cses: no CSRF token")
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.WriteField("csrf_token", csrf)
	w.WriteField("task", problemID)
	w.WriteField("lang", langID)
	w.WriteField("target", "problemset")
	w.WriteField("type", "course")
	p, _ := w.CreateFormFile("file", "solution.cpp")
	p.Write([]byte(sourceCode))
	w.Close()

	req, _ := http.NewRequestWithContext(ctx, "POST", "https://cses.fi/course/send.php", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Referer", submitPageURL)
	setBrowserHeaders(req)

	resp, err := b.client.Do(req)
	if err != nil {
		return "", err
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	resultID := extractResultID(string(respBody))
	if resultID != "" {
		slog.Info("cses submitted", "problem", problemID, "result_id", resultID)
		return resultID, nil
	}

	return fmt.Sprintf("cses-%d", time.Now().UnixNano()), nil
}

func (b *CSESBot) Poll(ctx context.Context, remoteID string) (*RemoteResult, error) {
	if strings.HasPrefix(remoteID, "cses-") {
		return &RemoteResult{RemoteID: remoteID, Verdict: "PENDING", Done: false}, nil
	}

	body, err := b.fetch(ctx, fmt.Sprintf("https://cses.fi/problemset/result/%s/", remoteID))
	if err != nil {
		return &RemoteResult{RemoteID: remoteID, Done: false, Verdict: "PENDING"}, nil
	}

	if strings.Contains(body, "READY") {
		r := &RemoteResult{RemoteID: remoteID, Done: true}
		if strings.Contains(body, "ACCEPTED") {
			r.Verdict = "AC"
		} else if strings.Contains(body, "WRONG ANSWER") {
			r.Verdict = "WA"
		} else if strings.Contains(body, "TIME LIMIT") {
			r.Verdict = "TLE"
		} else if strings.Contains(body, "COMPILATION ERROR") {
			r.Verdict = "CE"
		} else if strings.Contains(body, "RUNTIME ERROR") {
			r.Verdict = "RE"
		} else {
			r.Verdict = "WA"
		}
		r.TimeUsed = extractMaxTime(body)
		return r, nil
	}

	return &RemoteResult{RemoteID: remoteID, Done: false, Verdict: "PENDING"}, nil
}

func extractMaxTime(html string) int {
	var maxTime float64
	var walk func(string)
	walk = func(s string) {
		idx := strings.Index(s, "<td>")
		if idx == -1 {
			return
		}
		after := s[idx+4:]
		end := strings.Index(after, "</td>")
		if end == -1 {
			return
		}
		content := after[:end]
		if strings.HasSuffix(content, " s") {
			var secs float64
			fmt.Sscanf(strings.TrimSpace(content), "%f", &secs)
			if secs > maxTime {
				maxTime = secs
			}
		}
		walk(after[end+5:])
	}
	idx := strings.Index(html, "Test&nbsp;results")
	if idx == -1 {
		return 0
	}
	walk(html[idx:])
	return int(maxTime * 1000)
}

func (b *CSESBot) MarkBotSuccess(ctx context.Context, botID string) {}
func (b *CSESBot) MarkBotError(ctx context.Context, botID string)    {}

func (b *CSESBot) fetch(ctx context.Context, url string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	setBrowserHeaders(req)
	resp, err := b.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body), nil
}

func (b *CSESBot) resolveLangID(localLang string) string {
	if b.remoteLang != nil {
		langs, err := b.remoteLang.ListByPlatform(context.Background(), "cses")
		if err == nil {
			for _, l := range langs {
				if l.LocalID == localLang && l.Enabled {
					return l.RemoteID
				}
			}
		}
	}
	return "C++"
}

func extractCSRFCookie(html string) string {
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

func extractResultID(html string) string {
	idx := strings.Index(html, "result/")
	if idx == -1 {
		return ""
	}
	idx += 7
	end := strings.IndexAny(html[idx:], "/\"' \t\n><")
	if end == -1 {
		return ""
	}
	return html[idx : idx+end]
}

func setBrowserHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
}
