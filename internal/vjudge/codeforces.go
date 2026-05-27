package vjudge

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type CodeforcesBot struct {
	config BotConfig
	client *http.Client
	state  BotState
}

func NewCodeforcesBot(cfg BotConfig) *CodeforcesBot {
	jar, _ := cookiejar.New(nil)
	return &CodeforcesBot{
		config: cfg,
		client: &http.Client{Timeout: 30 * time.Second, Jar: jar},
		state:  StateIdle,
	}
}

func (b *CodeforcesBot) Name() string    { return "codeforces" }
func (b *CodeforcesBot) State() BotState { return b.state }

func (b *CodeforcesBot) csrf(pageURL string) (string, error) {
	req, _ := http.NewRequest("GET", pageURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := b.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return extractCSRFToken(resp.Body), nil
}

func extractCSRFToken(body io.Reader) string {
	z := html.NewTokenizer(body)
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
				if string(k) == "name" && string(v) == "X-Csrf-Token" {
					isCSRF = true
				}
				if string(k) == "content" {
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

func (b *CodeforcesBot) login(ctx context.Context) error {
	loginURL := "https://codeforces.com/enter"
	csrf, err := b.csrf(loginURL)
	if err != nil {
		return fmt.Errorf("get login page: %w", err)
	}
	data := url.Values{
		"csrf_token":     {csrf},
		"action":         {"enter"},
		"handleOrEmail":  {b.config.Username},
		"password":       {b.config.Password},
		"remember":       {"on"},
		"_tta":           {"1"},
	}
	req, _ := http.NewRequestWithContext(ctx, "POST", loginURL, strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", loginURL)
	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "Invalid handle or password") {
		return fmt.Errorf("codeforces: invalid credentials")
	}
	return nil
}

func (b *CodeforcesBot) Submit(ctx context.Context, problemID, sourceCode, language string) (string, error) {
	b.state = StateRunning
	if err := b.login(ctx); err != nil {
		b.state = StateError
		return "", err
	}
	// Map language key to Codeforces programTypeId
	langMap := map[string]string{
		"cpp-gpp-64": "54", "cpp-gpp-32": "53",
		"c-gcc-64": "43", "c-gcc-32": "43",
		"cpp-clang": "52", "python": "70", "pypy": "41",
		"java": "60", "rust": "75", "nodejs": "55", "csharp": "65",
	}
	langID, ok := langMap[language]
	if !ok {
		langID = "54" // default to G++17
	}

	submitURL := "https://codeforces.com/problemset/submit"
	csrf, err := b.csrf(submitURL)
	if err != nil {
		b.state = StateError
		return "", err
	}

	data := url.Values{
		"csrf_token":           {csrf},
		"action":               {"submitSolutionFormSubmitted"},
		"submittedProblemCode": {problemID},
		"programTypeId":        {langID},
		"source":               {sourceCode},
		"sourceCodeConfirmed":  {"true"},
		"tabSize":              {"4"},
	}
	req, _ := http.NewRequestWithContext(ctx, "POST", submitURL, strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", submitURL)

	client := &http.Client{
		Timeout:       30 * time.Second,
		Jar:           b.client.Jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		b.state = StateError
		return "", err
	}
	defer resp.Body.Close()

	// Extract submission ID from redirect: /problemset/submission/1234
	loc := resp.Header.Get("Location")
	parts := strings.Split(loc, "/")
	if len(parts) == 0 {
		b.state = StateError
		return "", fmt.Errorf("no redirect from CF submit")
	}
	subID := parts[len(parts)-1]
	b.state = StateIdle
	return subID, nil
}

func (b *CodeforcesBot) Poll(ctx context.Context, remoteID string) (*RemoteResult, error) {
	u := fmt.Sprintf("https://codeforces.com/api/user.status?handle=%s&count=5", b.config.Username)
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	content := string(body)

	r := &RemoteResult{RemoteID: remoteID, Done: false}
	// Simple verdict extraction from JSON response
	if strings.Contains(content, `"id":`+remoteID) {
		if strings.Contains(content, `"verdict":"OK"`) {
			r.Verdict = "AC"
			r.Done = true
		}
		if strings.Contains(content, `"verdict":"WRONG_ANSWER"`) {
			r.Verdict = "WA"
			r.Done = true
		}
		if strings.Contains(content, `"verdict":"TIME_LIMIT_EXCEEDED"`) {
			r.Verdict = "TLE"
			r.Done = true
		}
		if strings.Contains(content, `"verdict":"COMPILATION_ERROR"`) {
			r.Verdict = "CE"
			r.Done = true
		}
		if strings.Contains(content, `"verdict":"RUNTIME_ERROR"`) {
			r.Verdict = "RE"
			r.Done = true
		}
	}
	return r, nil
}
