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

	"github.com/tahsinarafat/aioj/internal/model"
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

func (b *CodeforcesBot) Name() string                        { return "codeforces" }
func (b *CodeforcesBot) State() BotState                     { return b.state }
func (b *CodeforcesBot) IsLoggedIn(ctx context.Context) bool { return len(b.config.Cookies) > 0 }
func (b *CodeforcesBot) Login(ctx context.Context) (map[string]string, error) {
	if b.cfSubmit != nil && b.config.Username != "" && b.config.Password != "" {
		proxy := ""
		if b.config.ProxyEnabled {
			proxy = b.config.ProxyURL
		}
		cookies, err := b.cfSubmit.Login(ctx, b.config.Username, b.config.Password, proxy)
		if err == nil && len(cookies) > 0 {
			b.SetCookies(cookies)
			return cookies, nil
		}
	}
	err := b.login(ctx)
	return b.config.Cookies, err
}

// Configure implements Bot.Configure using a BotConfig.
func (b *CodeforcesBot) Configure(cfg BotConfig) {
	b.config.Username = cfg.Username
	b.config.Password = cfg.Password
	b.config.APIKey = cfg.APIKey
	b.config.APISecret = cfg.APISecret
	b.config.ProxyURL = cfg.ProxyURL
	b.config.ProxyEnabled = cfg.ProxyEnabled
	b.SetCookies(cfg.Cookies)
}

// ConfigureFromAccount configures the bot from a BotAccount model (used internally).
func (b *CodeforcesBot) ConfigureFromAccount(acc model.BotAccount) {
	b.Configure(BotConfig{
		Username:     acc.PlatformUser,
		Password:     acc.PlatformPass,
		APIKey:       acc.APIKey,
		APISecret:    acc.APISecret,
		ProxyURL:     acc.ProxyURL,
		ProxyEnabled: acc.ProxyEnabled,
		Cookies:      acc.SessionData,
	})
}

// FetchLanguages implements Bot.FetchLanguages.
func (b *CodeforcesBot) FetchLanguages(ctx context.Context) ([]RemoteLanguageItem, error) {
	if b.cfSubmit != nil {
		return b.cfSubmit.FetchLanguages(ctx)
	}
	return nil, nil
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
		proxy := ""
		if b.config.ProxyEnabled {
			proxy = b.config.ProxyURL
		}
		subID, err := b.cfSubmit.Submit(ctx, problemID, sourceCode, langID, b.config.Username, "", b.config.Username, b.config.Password, nil, proxy)
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

// SubmissionURL is the browser-facing page for a Codeforces submission.
//
// problemRemoteID carries "contestId/problemIndex" for a CF problem (see
// parser.go), and the contest id is the first half — a submission belongs to
// the contest it was made in, so the URL needs that half, not the problem index.
func (b *CodeforcesBot) SubmissionURL(problemRemoteID, remoteID string) string {
	if remoteID == "" {
		return ""
	}
	contestID, _, ok := strings.Cut(problemRemoteID, "/")
	if !ok || contestID == "" {
		// A gym or mashup problem may be imported without the contest prefix.
		// Better no link than one that 404s.
		return ""
	}
	return fmt.Sprintf("https://codeforces.com/contest/%s/submission/%s", contestID, remoteID)
}

// PollBatch answers for many submissions in one request.
//
// Codeforces exposes /api/user.status, which returns a handle's recent
// submissions with verdicts. The service used to reach for it by testing
// `platform == "codeforces"`; implementing BatchPoller is what lets it ask
// instead. The advantage is real — one request per account instead of one per
// submission — which is why this is a capability rather than something the
// generic loop could do anyway.
//
// Only the handle already configured on this bot is queried, and each account's
// own submissions are fetched by whoever calls this with that account's bot.
func (b *CodeforcesBot) PollBatch(ctx context.Context, handle string, want []PendingResult) (map[string]*RemoteResult, error) {
	out := make(map[string]*RemoteResult, len(want))
	if len(want) == 0 {
		return out, nil
	}
	if handle == "" {
		// No handle means user.status cannot be queried. An error rather than an
		// empty map, so the caller falls back to per-submission polling instead
		// of concluding that nothing has been judged.
		return nil, fmt.Errorf("codeforces: no handle given for batch poll")
	}

	var (
		status string
		result []map[string]interface{}
	)
	if b.config.APIKey != "" && b.config.APISecret != "" {
		raw, err := b.cfAPIRequest(ctx, "user.status", map[string]string{
			"handle": handle,
			"count":  "50",
		})
		if err != nil {
			return nil, err
		}
		status, _ = raw["status"].(string)
		if res, ok := raw["result"].([]interface{}); ok {
			for _, item := range res {
				if sub, ok := item.(map[string]interface{}); ok {
					result = append(result, sub)
				}
			}
		}
	} else {
		u := fmt.Sprintf("https://codeforces.com/api/user.status?handle=%s&count=50", handle)
		body, err := b.fetchWithScraper(ctx, u)
		if err != nil {
			return nil, err
		}
		var parsed struct {
			Status string                   `json:"status"`
			Result []map[string]interface{} `json:"result"`
		}
		if err := json.Unmarshal([]byte(body), &parsed); err != nil {
			return nil, fmt.Errorf("parse CF user.status: %w", err)
		}
		status = parsed.Status
		result = parsed.Result
	}
	if status != "OK" {
		return nil, fmt.Errorf("codeforces user.status reported %q", status)
	}

	return mapCFSubmissionBatch(result, want), nil
}

// mapCFSubmissionBatch turns a user.status result set into RemoteResults for the
// submissions the caller asked about.
//
// A free function on purpose: this is where the judgement actually happens —
// which ids match, which are still judging, and how the platform's field names
// and units translate — and it is the part worth testing. Keeping it out of
// PollBatch means the test does not need a Codeforces account, a live handle or
// an HTTP server, none of which the logic depends on.
//
// Only ids in want are returned. user.status reports a handle's whole recent
// history, so passing an unrelated submission through would write a verdict
// against a row that never asked for one.
func mapCFSubmissionBatch(result []map[string]interface{}, want []PendingResult) map[string]*RemoteResult {
	out := make(map[string]*RemoteResult, len(want))
	wanted := make(map[string]struct{}, len(want))
	for _, w := range want {
		wanted[w.RemoteID] = struct{}{}
	}

	for _, sub := range result {
		if sub == nil {
			continue
		}
		id := fmt.Sprintf("%.0f", sub["id"])
		if _, ok := wanted[id]; !ok {
			continue
		}
		// fmt.Sprintf("%v", nil) is "<nil>", which normalizeVerdict has no case
		// for and would answer Wrong Answer to. A submission with no verdict
		// yet is still being judged, so it is skipped rather than guessed at.
		raw, ok := sub["verdict"].(string)
		if !ok {
			continue
		}
		v := normalizeVerdict(raw)
		if !v.IsDone() {
			continue
		}
		r := &RemoteResult{RemoteID: id, Verdict: v, Done: true}
		// The API reports milliseconds and bytes; the model stores ms and KB.
		if t, ok := sub["timeConsumedMillis"].(float64); ok {
			r.TimeUsed = int(t)
		} else if t, ok := sub["time"].(float64); ok {
			// The scraper-based path returns the raw user.status JSON, whose
			// field names and units differ from the API's.
			r.TimeUsed = int(t * 1000)
		}
		if m, ok := sub["memoryConsumedBytes"].(float64); ok {
			r.MemoryUsed = int(m / 1024)
		} else if m, ok := sub["memory"].(float64); ok {
			r.MemoryUsed = int(m / 1024)
		}
		out[id] = r
	}
	return out
}

// init registers this platform's factory (see registry.go).
//
// The bot prefers the out-of-process submit client when one is configured: that
// client holds a real browser session, which is what gets past Cloudflare where
// an in-process scrape does not.
func init() {
	RegisterBotFactory("codeforces", func(cfg BotConfig, deps BotDeps) Bot {
		if deps.CFSubmit != nil {
			return NewCodeforcesBotWithSubmit(cfg, deps.CFSubmit)
		}
		return NewCodeforcesBot(cfg)
	})
}

// ValidateCookies loads a page that requires a session and reports whether the
// cookies still identify a logged-in user.
//
// Implements CookieValidator (see bot.go), which is what lets the admin console
// ask "can this platform be validated?" without naming Codeforces.
func (b *CodeforcesBot) ValidateCookies(ctx context.Context, cookies map[string]string) error {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Timeout: 15 * time.Second, Jar: jar}
	cfURL, _ := url.Parse("https://codeforces.com")
	jarCookies := make([]*http.Cookie, 0, len(cookies))
	for name, value := range cookies {
		jarCookies = append(jarCookies, &http.Cookie{Name: name, Value: value, Domain: "codeforces.com", Path: "/"})
	}
	client.Jar.SetCookies(cfURL, jarCookies)
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://codeforces.com/problemset/status?my=on", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "Codeforces: Enter") || strings.Contains(string(body), "handleOrEmail") {
		return fmt.Errorf("cookies expired or invalid - not logged in")
	}
	return nil
}
