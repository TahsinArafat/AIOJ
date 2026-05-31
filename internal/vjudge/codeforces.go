package vjudge

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	cloudflare "github.com/sriharsha-y/go-cfscraper/lib"
)

type CodeforcesBot struct {
	config   BotConfig
	client   *http.Client
	scraper  *cloudflare.Scraper
	state    BotState
	cfSubmit *CFSubmitClient
}

func NewCodeforcesBot(cfg BotConfig) *CodeforcesBot {
	jar, _ := cookiejar.New(nil)
	scraper, _ := cloudflare.New()
	client := &http.Client{Timeout: 60 * time.Second, Jar: jar}

	if len(cfg.Cookies) > 0 {
		cfURL, _ := url.Parse("https://codeforces.com")
		var cookies []*http.Cookie
		for name, value := range cfg.Cookies {
			cookies = append(cookies, &http.Cookie{Name: name, Value: value, Domain: "codeforces.com", Path: "/"})
		}
		jar.SetCookies(cfURL, cookies)
	}

	return &CodeforcesBot{
		config:  cfg,
		client:  client,
		scraper: scraper,
		state:   StateIdle,
	}
}

func NewCodeforcesBotWithSubmit(cfg BotConfig, cfSubmit *CFSubmitClient) *CodeforcesBot {
	bot := NewCodeforcesBot(cfg)
	bot.cfSubmit = cfSubmit
	return bot
}

func (b *CodeforcesBot) Name() string          { return "codeforces" }
func (b *CodeforcesBot) State() BotState       { return b.state }
func (b *CodeforcesBot) IsLoggedIn(ctx context.Context) bool { return len(b.config.Cookies) > 0 }
func (b *CodeforcesBot) Login(ctx context.Context) (map[string]string, error) {
	b.login(ctx)
	return b.config.Cookies, nil
}
func (b *CodeforcesBot) SetCookies(cookies map[string]string) {
	cfURL, _ := url.Parse("https://codeforces.com")
	var js []*http.Cookie
	for name, value := range cookies {
		js = append(js, &http.Cookie{Name: name, Value: value, Domain: "codeforces.com", Path: "/"})
	}
	b.client.Jar.SetCookies(cfURL, js)
	b.config.Cookies = cookies
}

func (b *CodeforcesBot) cfAPIRequest(ctx context.Context, methodName string, params map[string]string) (map[string]interface{}, error) {
	params["apiKey"] = b.config.APIKey
	params["time"] = strconv.FormatInt(time.Now().Unix(), 10)

	randHex := fmt.Sprintf("%06x", rand.Intn(0xFFFFFF))
	var parts []string
	for k, v := range params {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	queryStr := strings.Join(parts, "&")
	hashInput := randHex + "/" + methodName + "?" + queryStr + "#" + b.config.APISecret
	hash := sha512.Sum512([]byte(hashInput))
	apiSig := randHex + hex.EncodeToString(hash[:])

	u := fmt.Sprintf("https://codeforces.com/api/%s?%s&apiSig=%s", methodName, queryStr, apiSig)
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse CF API response: %w", err)
	}
	if result["status"] == "FAILED" {
		return nil, fmt.Errorf("CF API error: %v", result["comment"])
	}
	return result, nil
}

func (b *CodeforcesBot) fetchWithScraper(ctx context.Context, url string) (string, error) {
	if b.scraper != nil {
		resp, err := b.scraper.Get(ctx, url)
		if err == nil && resp.StatusCode == 200 {
			return string(resp.Body), nil
		}
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	resp, err := b.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body), nil
}

func (b *CodeforcesBot) csrf(ctx context.Context, pageURL string) (string, error) {
	body, err := b.fetchWithScraper(ctx, pageURL)
	if err != nil {
		return "", err
	}
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
				if string(k) == "name" && string(v) == "X-Csrf-Token" {
					isCSRF = true
				}
				if string(k) == "content" {
					content = string(v)
				}
				hasAttr = more
			}
			if isCSRF && content != "" {
				return content, nil
			}
		}
	}
	return "", fmt.Errorf("CSRF token not found")
}

func (b *CodeforcesBot) login(ctx context.Context) error {
	if len(b.config.Cookies) > 0 {
		return nil
	}
	loginURL := "https://codeforces.com/enter"
	csrfToken, err := b.csrf(ctx, loginURL)
	if err != nil {
		return fmt.Errorf("get login page: %w", err)
	}
	data := url.Values{
		"csrf_token":    {csrfToken},
		"action":        {"enter"},
		"handleOrEmail": {b.config.Username},
		"password":      {b.config.Password},
		"remember":      {"on"},
		"_tta":          {"1"},
	}
	req, _ := http.NewRequestWithContext(ctx, "POST", loginURL, strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
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
	if b.config.Username == "" && len(b.config.Cookies) == 0 {
		b.state = StateIdle
		return fmt.Sprintf("cf-%d", time.Now().UnixNano()), nil
	}

	langID := "54"
	if langMap, ok := map[string]string{
		"cpp-gpp-64": "54", "cpp-gpp-32": "53",
		"c-gcc-64": "43", "c-gcc-32": "43",
		"cpp-clang": "52", "python": "70", "pypy": "41",
		"java": "60", "rust": "75", "nodejs": "55", "csharp": "65",
	}[language]; ok {
		langID = langMap
	}

	if b.cfSubmit != nil && b.config.Username != "" && b.config.Password != "" {
		subID, err := b.cfSubmit.Submit(ctx, problemID, sourceCode, langID, b.config.Username, "", b.config.Username, b.config.Password)
		if err == nil && subID != "" {
			b.state = StateIdle
			return subID, nil
		}
		slog.Error("cf-submit failed, falling back to direct POST", "err", err)
	}

	if err := b.login(ctx); err != nil {
		b.state = StateError
		return "", err
	}
	submitURL := "https://codeforces.com/problemset/submit"
	csrfToken := ""
	if len(b.config.Cookies) == 0 {
		var err error
		csrfToken, err = b.csrf(ctx, submitURL)
		if err != nil {
			b.state = StateError
			return "", err
		}
	}
	data := url.Values{
		"csrf_token":           {csrfToken},
		"action":               {"submitSolutionFormSubmitted"},
		"submittedProblemCode": {problemID},
		"programTypeId":        {langID},
		"source":               {sourceCode},
		"sourceCodeConfirmed":  {"true"},
		"tabSize":              {"4"},
	}
	req, _ := http.NewRequestWithContext(ctx, "POST", submitURL, strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Referer", submitURL)
	client := &http.Client{
		Timeout:       60 * time.Second,
		Jar:           b.client.Jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		b.state = StateError
		return "", err
	}
	defer resp.Body.Close()
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
	if b.config.Username == "" || strings.HasPrefix(remoteID, "cf-") {
		return &RemoteResult{RemoteID: remoteID, Done: false, Verdict: "PENDING"}, nil
	}

	if b.config.APIKey != "" && b.config.APISecret != "" {
		result, err := b.cfAPIRequest(ctx, "user.status", map[string]string{
			"handle": b.config.Username,
			"count":  "5",
		})
		if err == nil {
			if res, ok := result["result"].([]interface{}); ok {
				for _, item := range res {
					if sub, ok := item.(map[string]interface{}); ok {
						if fmt.Sprintf("%.0f", sub["id"]) == remoteID {
							r := &RemoteResult{RemoteID: remoteID, Done: true}
							switch sub["verdict"] {
							case "OK":
								r.Verdict = "AC"
							case "WRONG_ANSWER":
								r.Verdict = "WA"
							case "TIME_LIMIT_EXCEEDED":
								r.Verdict = "TLE"
							case "COMPILATION_ERROR":
								r.Verdict = "CE"
							case "RUNTIME_ERROR":
								r.Verdict = "RE"
							case "IDLENESS_LIMIT_EXCEEDED":
								r.Verdict = "TLE"
							default:
								r.Verdict = "WA"
							}
							if t, ok := sub["time"].(float64); ok {
								r.TimeUsed = int(t * 1000)
							}
							if m, ok := sub["memory"].(float64); ok {
								r.MemoryUsed = int(m / 1024)
							}
							return r, nil
						}
					}
				}
			}
			return &RemoteResult{RemoteID: remoteID, Done: false, Verdict: "PENDING"}, nil
		}
	}

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
