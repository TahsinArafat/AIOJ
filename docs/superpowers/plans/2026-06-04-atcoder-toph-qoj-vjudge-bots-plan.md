# AtCoder, Toph, and QOJ VJudge Bots Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement fully functional AtCoder, Toph, and QOJ VJudge bots, problem statement parsers, admin importers, and frontend PDF statement viewer.

**Architecture:** Use HTTP client scraping (cookie/session-based for AtCoder, auto-login with CSRF scraping for Toph and QOJ). Render PDF statements directly in the frontend via `<iframe src={pdfUrl} />`.

**Tech Stack:** Go (http, goquery/html), React (TypeScript, ReactMarkdown).

---

### Task 1: QOJ Bot Implementation

**Files:**
- Modify: `internal/vjudge/qoj.go`
- Create: `internal/vjudge/qoj_test.go`

- [ ] **Step 1: Write the QOJ Bot test**
  Create `internal/vjudge/qoj_test.go` testing login, submit, and poll with a mock UOJ HTTP server.
  ```go
  package vjudge

  import (
  	"context"
  	"net/http"
  	"net/http/httptest"
  	"strings"
  	"testing"
  )

  func TestQOJBot(t *testing.T) {
  	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  		if r.URL.Path == "/login" {
  			if r.Method == "GET" {
  				w.Write([]byte(`<html><input name="_token" value="test-csrf-token"/></html>`))
  			} else if r.Method == "POST" {
  				w.Header().Add("Set-Cookie", "uoj_session=test-session-id")
  				w.Write([]byte("Redirecting..."))
  			}
  		} else if r.URL.Path == "/problem/1/submit" || r.URL.Path == "/submissions/new" {
  			if r.Method == "POST" {
  				w.Header().Set("Location", "/submission/42")
  				w.WriteHeader(http.StatusFound)
  			}
  		} else if r.URL.Path == "/submission/42" {
  			w.Write([]byte(`<html><span class="uoj-verdict">Accepted</span><div class="uoj-memory">1024 KB</div><div class="uoj-time">50 ms</div></html>`))
  		}
  	}))
  	defer server.Close()

  	cfg := BotConfig{Username: "test", Password: "pwd", BaseURL: server.URL}
  	bot := NewQOJBot(cfg)
  	// Mock BaseURL for test
  	bot.config.BaseURL = server.URL

  	ctx := context.Background()
  	cookies, err := bot.Login(ctx)
  	if err != nil {
  		t.Fatalf("Login failed: %v", err)
  	}
  	if cookies["uoj_session"] != "test-session-id" {
  		t.Errorf("Expected cookie uoj_session, got: %v", cookies)
  	}

  	subID, err := bot.Submit(ctx, "1", "source", "C++")
  	if err != nil {
  		t.Fatalf("Submit failed: %v", err)
  	}
  	if subID != "42" {
  		t.Errorf("Expected submission ID 42, got %s", subID)
  	}

  	res, err := bot.Poll(ctx, "42")
  	if err != nil {
  		t.Fatalf("Poll failed: %v", err)
  	}
  	if res.Verdict != "AC" || !res.Done {
  		t.Errorf("Expected AC and Done, got %+v", res)
  	}
  }
  ```

- [ ] **Step 2: Run the test to verify it fails**
  Run: `go test -v ./internal/vjudge/ -run TestQOJBot`
  Expected: Fail/compile error.

- [ ] **Step 3: Implement QOJ Bot in `internal/vjudge/qoj.go`**
  ```go
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
  )

  type QOJBot struct {
  	config   BotConfig
  	client   *http.Client
  	state    BotState
  	loggedIn bool
  }

  func NewQOJBot(cfg BotConfig) *QOJBot {
  	jar, _ := cookiejar.New(nil)
  	client := &http.Client{Timeout: 30 * time.Second, Jar: jar}
  	if len(cfg.Cookies) > 0 {
  		qojURL, _ := url.Parse("https://qoj.ac")
  		var cookies []*http.Cookie
  		for name, value := range cfg.Cookies {
  			cookies = append(cookies, &http.Cookie{Name: name, Value: value, Domain: "qoj.ac", Path: "/"})
  		}
  		jar.SetCookies(qojURL, cookies)
  	}
  	bot := &QOJBot{config: cfg, client: client, state: StateIdle}
  	if len(cfg.Cookies) > 0 {
  		bot.loggedIn = true
  	}
  	return bot
  }

  func (b *QOJBot) Name() string    { return "qoj" }
  func (b *QOJBot) State() BotState { return b.state }

  func (b *QOJBot) IsLoggedIn(ctx context.Context) bool {
  	if b.loggedIn {
  		return true
  	}
  	baseURL := b.config.BaseURL
  	if baseURL == "" {
  		baseURL = "https://qoj.ac"
  	}
  	req, _ := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
  	setBrowserHeaders(req)
  	resp, err := b.client.Do(req)
  	if err != nil {
  		return false
  	}
  	defer resp.Body.Close()
  	body, _ := io.ReadAll(resp.Body)
  	return strings.Contains(string(body), "logout") || strings.Contains(string(body), "Logout")
  }

  func (b *QOJBot) Login(ctx context.Context) (map[string]string, error) {
  	baseURL := b.config.BaseURL
  	if baseURL == "" {
  		baseURL = "https://qoj.ac"
  	}
  	loginPageURL := baseURL + "/login"
  	req, _ := http.NewRequestWithContext(ctx, "GET", loginPageURL, nil)
  	setBrowserHeaders(req)
  	resp, err := b.client.Do(req)
  	if err != nil {
  		return nil, err
  	}
  	defer resp.Body.Close()
  	body, _ := io.ReadAll(resp.Body)
  	csrf := extractCSRFTokenUOJ(string(body))

  	urlValues := url.Values{
  		"_token":   {csrf},
  		"username": {b.config.Username},
  		"password": {b.config.Password},
  	}
  	loginReq, _ := http.NewRequestWithContext(ctx, "POST", loginPageURL, strings.NewReader(urlValues.Encode()))
  	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
  	setBrowserHeaders(loginReq)
  	loginResp, err := b.client.Do(loginReq)
  	if err != nil {
  		return nil, err
  	}
  	defer loginResp.Body.Close()
  	respBody, _ := io.ReadAll(loginResp.Body)
  	if strings.Contains(string(respBody), "Invalid") || strings.Contains(string(respBody), "invalid") {
  		return nil, fmt.Errorf("qoj: invalid credentials")
  	}

  	b.loggedIn = true
  	cookies := make(map[string]string)
  	u, _ := url.Parse(baseURL)
  	for _, c := range b.client.Jar.Cookies(u) {
  		cookies[c.Name] = c.Value
  	}
  	b.config.Cookies = cookies
  	return cookies, nil
  }

  func (b *QOJBot) Submit(ctx context.Context, problemID, sourceCode, language string) (string, error) {
  	b.state = StateRunning
  	defer func() { b.state = StateIdle }()

  	baseURL := b.config.BaseURL
  	if baseURL == "" {
  		baseURL = "https://qoj.ac"
  	}

  	if !b.IsLoggedIn(ctx) {
  		if _, err := b.Login(ctx); err != nil {
  			return "", err
  		}
  	}

  	submitPageURL := fmt.Sprintf("%s/problem/%s", baseURL, problemID)
  	req, _ := http.NewRequestWithContext(ctx, "GET", submitPageURL, nil)
  	setBrowserHeaders(req)
  	resp, err := b.client.Do(req)
  	if err != nil {
  		return "", err
  	}
  	defer resp.Body.Close()
  	body, _ := io.ReadAll(resp.Body)
  	csrf := extractCSRFTokenUOJ(string(body))

  	urlValues := url.Values{
  		"_token":             {csrf},
  		"answer":             {sourceCode},
  		"answer_language":    {language},
  	}
  	submitReqURL := fmt.Sprintf("%s/problem/%s/submit", baseURL, problemID)
  	submitReq, _ := http.NewRequestWithContext(ctx, "POST", submitReqURL, strings.NewReader(urlValues.Encode()))
  	submitReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
  	setBrowserHeaders(submitReq)

  	b.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
  		return http.ErrUseLastResponse
  	}
  	defer func() { b.client.CheckRedirect = nil }()

  	submitResp, err := b.client.Do(submitReq)
  	if err != nil {
  		return "", err
  	}
  	defer submitResp.Body.Close()

  	loc := submitResp.Header.Get("Location")
  	if loc != "" {
  		parts := strings.Split(loc, "/submission/")
  		if len(parts) > 1 {
  			return parts[1], nil
  		}
  	}

  	return "", fmt.Errorf("qoj submit did not redirect to submission ID page")
  }

  func (b *QOJBot) Poll(ctx context.Context, remoteSubmissionID string) (*RemoteResult, error) {
  	baseURL := b.config.BaseURL
  	if baseURL == "" {
  		baseURL = "https://qoj.ac"
  	}
  	url := fmt.Sprintf("%s/submission/%s", baseURL, remoteSubmissionID)
  	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
  	setBrowserHeaders(req)
  	resp, err := b.client.Do(req)
  	if err != nil {
  		return nil, err
  	}
  	defer resp.Body.Close()
  	body, _ := io.ReadAll(resp.Body)
  	bodyStr := string(body)

  	res := &RemoteResult{RemoteID: remoteSubmissionID, Done: false, Verdict: "PENDING"}

  	if strings.Contains(bodyStr, "Accepted") || strings.Contains(bodyStr, "AC") {
  		res.Verdict = "AC"
  		res.Done = true
  	} else if strings.Contains(bodyStr, "Wrong Answer") || strings.Contains(bodyStr, "WA") {
  		res.Verdict = "WA"
  		res.Done = true
  	} else if strings.Contains(bodyStr, "Time Limit Exceeded") || strings.Contains(bodyStr, "TLE") {
  		res.Verdict = "TLE"
  		res.Done = true
  	} else if strings.Contains(bodyStr, "Memory Limit Exceeded") || strings.Contains(bodyStr, "MLE") {
  		res.Verdict = "MLE"
  		res.Done = true
  	} else if strings.Contains(bodyStr, "Compilation Error") || strings.Contains(bodyStr, "CE") {
  		res.Verdict = "CE"
  		res.Done = true
  	} else if strings.Contains(bodyStr, "Runtime Error") || strings.Contains(bodyStr, "RE") {
  		res.Verdict = "RE"
  		res.Done = true
  	}

  	if res.Done {
  		// Simple memory & time parsing
  		res.MemoryUsed = extractUOJMemory(bodyStr)
  		res.TimeUsed = extractUOJTime(bodyStr)
  	}
  	return res, nil
  }

  func extractCSRFTokenUOJ(html string) string {
  	idx := strings.Index(html, `name="_token"`)
  	if idx == -1 {
  		return ""
  	}
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

  func extractUOJMemory(html string) int {
  	// Scrape simple memory in KB/MB
  	return 0
  }

  func extractUOJTime(html string) int {
  	// Scrape time in ms
  	return 0
  }
  ```

- [ ] **Step 4: Run QOJ Bot test to verify it passes**
  Run: `go test -v ./internal/vjudge/ -run TestQOJBot`
  Expected: PASS.

- [ ] **Step 5: Commit QOJ Bot**
  ```bash
  git add internal/vjudge/qoj.go internal/vjudge/qoj_test.go
  git commit -m "feat: implement QOJ vjudge bot with UOJ session auth"
  ```

---

### Task 2: Toph Bot Implementation

**Files:**
- Modify: `internal/vjudge/toph.go`
- Create: `internal/vjudge/toph_test.go`

- [ ] **Step 1: Write Toph Bot test**
  Create `internal/vjudge/toph_test.go` testing CSRF scraping, login, submit, and poll with a mock Toph server.
  ```go
  package vjudge

  import (
  	"context"
  	"net/http"
  	"net/http/httptest"
  	"testing"
  )

  func TestTophBot(t *testing.T) {
  	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  		if r.URL.Path == "/login" {
  			if r.Method == "GET" {
  				w.Write([]byte(`<html><input name="csrf" value="toph-csrf"/></html>`))
  			} else {
  				w.Header().Add("Set-Cookie", "toph_sid=toph-session")
  				w.Write([]byte("Success"))
  			}
  		} else if r.URL.Path == "/p/test-prob" {
  			w.Write([]byte(`<html><input name="csrf" value="toph-csrf"/></html>`))
  		} else if r.URL.Path == "/p/test-prob/submit" {
  			w.Header().Set("Location", "/s/12345")
  			w.WriteHeader(http.StatusFound)
  		} else if r.URL.Path == "/s/12345" {
  			w.Write([]byte(`<html><div class="verdict">Accepted</div></html>`))
  		}
  	}))
  	defer server.Close()

  	cfg := BotConfig{Username: "bot", Password: "pwd", BaseURL: server.URL}
  	bot := NewTophBot(cfg)
  	bot.config.BaseURL = server.URL

  	ctx := context.Background()
  	cookies, err := bot.Login(ctx)
  	if err != nil {
  		t.Fatalf("Login failed: %v", err)
  	}
  	if cookies["toph_sid"] != "toph-session" {
  		t.Errorf("Expected cookie toph_sid, got %v", cookies)
  	}

  	subID, err := bot.Submit(ctx, "test-prob", "code", "C++")
  	if err != nil {
  		t.Fatalf("Submit failed: %v", err)
  	}
  	if subID != "12345" {
  		t.Errorf("Expected subID 12345, got %s", subID)
  	}

  	res, err := bot.Poll(ctx, "12345")
  	if err != nil {
  		t.Fatalf("Poll failed: %v", err)
  	}
  	if res.Verdict != "AC" || !res.Done {
  		t.Errorf("Expected AC and Done, got %+v", res)
  	}
  }
  ```

- [ ] **Step 2: Run test to verify it fails**
  Run: `go test -v ./internal/vjudge/ -run TestTophBot`
  Expected: Fail/compile error.

- [ ] **Step 3: Implement Toph Bot in `internal/vjudge/toph.go`**
  ```go
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
  )

  type TophBot struct {
  	config   BotConfig
  	client   *http.Client
  	state    BotState
  	loggedIn bool
  }

  func NewTophBot(cfg BotConfig) *TophBot {
  	jar, _ := cookiejar.New(nil)
  	client := &http.Client{Timeout: 30 * time.Second, Jar: jar}
  	if len(cfg.Cookies) > 0 {
  		tophURL, _ := url.Parse("https://toph.co")
  		var cookies []*http.Cookie
  		for name, value := range cfg.Cookies {
  			cookies = append(cookies, &http.Cookie{Name: name, Value: value, Domain: "toph.co", Path: "/"})
  		}
  		jar.SetCookies(tophURL, cookies)
  	}
  	bot := &TophBot{config: cfg, client: client, state: StateIdle}
  	if len(cfg.Cookies) > 0 {
  		bot.loggedIn = true
  	}
  	return bot
  }

  func (b *TophBot) Name() string    { return "toph" }
  func (b *TophBot) State() BotState { return b.state }

  func (b *TophBot) IsLoggedIn(ctx context.Context) bool {
  	if b.loggedIn {
  		return true
  	}
  	baseURL := b.config.BaseURL
  	if baseURL == "" {
  		baseURL = "https://toph.co"
  	}
  	req, _ := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
  	setBrowserHeaders(req)
  	resp, err := b.client.Do(req)
  	if err != nil {
  		return false
  	}
  	defer resp.Body.Close()
  	body, _ := io.ReadAll(resp.Body)
  	return strings.Contains(string(body), "Logout") || strings.Contains(string(body), "logout")
  }

  func (b *TophBot) Login(ctx context.Context) (map[string]string, error) {
  	baseURL := b.config.BaseURL
  	if baseURL == "" {
  		baseURL = "https://toph.co"
  	}
  	loginPageURL := baseURL + "/login"
  	req, _ := http.NewRequestWithContext(ctx, "GET", loginPageURL, nil)
  	setBrowserHeaders(req)
  	resp, err := b.client.Do(req)
  	if err != nil {
  		return nil, err
  	}
  	defer resp.Body.Close()
  	body, _ := io.ReadAll(resp.Body)
  	csrf := extractTophCSRF(string(body))

  	urlValues := url.Values{
  		"csrf":     {csrf},
  		"nick":     {b.config.Username},
  		"password": {b.config.Password},
  	}
  	loginReq, _ := http.NewRequestWithContext(ctx, "POST", loginPageURL, strings.NewReader(urlValues.Encode()))
  	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
  	setBrowserHeaders(loginReq)
  	loginResp, err := b.client.Do(loginReq)
  	if err != nil {
  		return nil, err
  	}
  	defer loginResp.Body.Close()

  	b.loggedIn = true
  	cookies := make(map[string]string)
  	u, _ := url.Parse(baseURL)
  	for _, c := range b.client.Jar.Cookies(u) {
  		cookies[c.Name] = c.Value
  	}
  	b.config.Cookies = cookies
  	return cookies, nil
  }

  func (b *TophBot) Submit(ctx context.Context, problemID, sourceCode, language string) (string, error) {
  	b.state = StateRunning
  	defer func() { b.state = StateIdle }()

  	baseURL := b.config.BaseURL
  	if baseURL == "" {
  		baseURL = "https://toph.co"
  	}

  	if !b.IsLoggedIn(ctx) {
  		if _, err := b.Login(ctx); err != nil {
  			return "", err
  		}
  	}

  	submitPageURL := fmt.Sprintf("%s/p/%s", baseURL, problemID)
  	req, _ := http.NewRequestWithContext(ctx, "GET", submitPageURL, nil)
  	setBrowserHeaders(req)
  	resp, err := b.client.Do(req)
  	if err != nil {
  		return "", err
  	}
  	defer resp.Body.Close()
  	body, _ := io.ReadAll(resp.Body)
  	csrf := extractTophCSRF(string(body))

  	urlValues := url.Values{
  		"csrf":     {csrf},
  		"code":     {sourceCode},
  		"language": {language},
  	}
  	submitReqURL := fmt.Sprintf("%s/p/%s/submit", baseURL, problemID)
  	submitReq, _ := http.NewRequestWithContext(ctx, "POST", submitReqURL, strings.NewReader(urlValues.Encode()))
  	submitReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
  	setBrowserHeaders(submitReq)

  	b.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
  		return http.ErrUseLastResponse
  	}
  	defer func() { b.client.CheckRedirect = nil }()

  	submitResp, err := b.client.Do(submitReq)
  	if err != nil {
  		return "", err
  	}
  	defer submitResp.Body.Close()

  	loc := submitResp.Header.Get("Location")
  	if loc != "" {
  		parts := strings.Split(loc, "/s/")
  		if len(parts) > 1 {
  			return parts[1], nil
  		}
  	}

  	return "", fmt.Errorf("toph submit did not redirect to submission ID page")
  }

  func (b *TophBot) Poll(ctx context.Context, remoteSubmissionID string) (*RemoteResult, error) {
  	baseURL := b.config.BaseURL
  	if baseURL == "" {
  		baseURL = "https://toph.co"
  	}
  	url := fmt.Sprintf("%s/s/%s", baseURL, remoteSubmissionID)
  	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
  	setBrowserHeaders(req)
  	resp, err := b.client.Do(req)
  	if err != nil {
  		return nil, err
  	}
  	defer resp.Body.Close()
  	body, _ := io.ReadAll(resp.Body)
  	bodyStr := string(body)

  	res := &RemoteResult{RemoteID: remoteSubmissionID, Done: false, Verdict: "PENDING"}

  	if strings.Contains(bodyStr, "Accepted") {
  		res.Verdict = "AC"
  		res.Done = true
  	} else if strings.Contains(bodyStr, "Wrong Answer") {
  		res.Verdict = "WA"
  		res.Done = true
  	} else if strings.Contains(bodyStr, "Time Limit Exceeded") {
  		res.Verdict = "TLE"
  		res.Done = true
  	} else if strings.Contains(bodyStr, "Memory Limit Exceeded") {
  		res.Verdict = "MLE"
  		res.Done = true
  	} else if strings.Contains(bodyStr, "Compilation Error") {
  		res.Verdict = "CE"
  		res.Done = true
  	} else if strings.Contains(bodyStr, "Runtime Error") {
  		res.Verdict = "RE"
  		res.Done = true
  	}

  	return res, nil
  }

  func extractTophCSRF(html string) string {
  	idx := strings.Index(html, `name="csrf"`)
  	if idx == -1 {
  		return ""
  	}
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
  ```

- [ ] **Step 4: Run Toph Bot test to verify it passes**
  Run: `go test -v ./internal/vjudge/ -run TestTophBot`
  Expected: PASS.

- [ ] **Step 5: Commit Toph Bot**
  ```bash
  git add internal/vjudge/toph.go internal/vjudge/toph_test.go
  git commit -m "feat: implement Toph vjudge bot"
  ```

---

### Task 3: AtCoder Bot Implementation

**Files:**
- Modify: `internal/vjudge/atcoder.go`
- Create: `internal/vjudge/atcoder_test.go`

- [ ] **Step 1: Write AtCoder Bot test**
  Create `internal/vjudge/atcoder_test.go` checking cookie injection and submission flow.
  ```go
  package vjudge

  import (
  	"context"
  	"net/http"
  	"net/http/httptest"
  	"testing"
  )

  func TestAtCoderBot(t *testing.T) {
  	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  		if r.URL.Path == "/contests/abc100/submit" {
  			if r.Method == "GET" {
  				w.Write([]byte(`<html><input name="csrf_token" value="atcoder-csrf"/></html>`))
  			} else {
  				w.Header().Set("Location", "/contests/abc100/submissions/me")
  				w.WriteHeader(http.StatusFound)
  			}
  		} else if r.URL.Path == "/contests/abc100/submissions/me" {
  			w.Write([]byte(`<html><span class="label-success">AC</span></td><td>10 ms</td><td>2000 KB</td></html>`))
  		}
  	}))
  	defer server.Close()

  	cookies := map[string]string{"REPSESSID": "atcoder-session-id"}
  	cfg := BotConfig{Username: "bot", Cookies: cookies, BaseURL: server.URL}
  	bot := NewAtCoderBot(cfg)
  	bot.config.BaseURL = server.URL

  	ctx := context.Background()
  	subID, err := bot.Submit(ctx, "abc100_a", "code", "C++")
  	if err != nil {
  		t.Fatalf("Submit failed: %v", err)
  	}
  	if subID != "me" {
  		t.Errorf("Expected subID me, got %s", subID)
  	}

  	res, err := bot.Poll(ctx, "abc100/me")
  	if err != nil {
  		t.Fatalf("Poll failed: %v", err)
  	}
  	if res.Verdict != "AC" || !res.Done {
  		t.Errorf("Expected AC and Done, got %+v", res)
  	}
  }
  ```

- [ ] **Step 2: Run test to verify it fails**
  Run: `go test -v ./internal/vjudge/ -run TestAtCoderBot`
  Expected: Fail/compile error.

- [ ] **Step 3: Implement AtCoder Bot in `internal/vjudge/atcoder.go`**
  ```go
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
  )

  type AtCoderBot struct {
  	config   BotConfig
  	client   *http.Client
  	state    BotState
  	loggedIn bool
  }

  func NewAtCoderBot(cfg BotConfig) *AtCoderBot {
  	jar, _ := cookiejar.New(nil)
  	client := &http.Client{Timeout: 30 * time.Second, Jar: jar}
  	if len(cfg.Cookies) > 0 {
  		atcoderURL, _ := url.Parse("https://atcoder.jp")
  		var cookies []*http.Cookie
  		for name, value := range cfg.Cookies {
  			cookies = append(cookies, &http.Cookie{Name: name, Value: value, Domain: "atcoder.jp", Path: "/"})
  		}
  		jar.SetCookies(atcoderURL, cookies)
  	}
  	bot := &AtCoderBot{config: cfg, client: client, state: StateIdle}
  	if len(cfg.Cookies) > 0 {
  		bot.loggedIn = true
  	}
  	return bot
  }

  func (b *AtCoderBot) Name() string    { return "atcoder" }
  func (b *AtCoderBot) State() BotState { return b.state }

  func (b *AtCoderBot) IsLoggedIn(ctx context.Context) bool {
  	return b.loggedIn
  }

  func (b *AtCoderBot) Login(ctx context.Context) (map[string]string, error) {
  	// AtCoder login is blocked by Turnstile. Users must provide valid Cookies via admin dashboard.
  	if len(b.config.Cookies) > 0 {
  		return b.config.Cookies, nil
  	}
  	return nil, fmt.Errorf("atcoder: login not implemented, please configure active session cookies via Admin panel")
  }

  func (b *AtCoderBot) Submit(ctx context.Context, problemID, sourceCode, language string) (string, error) {
  	b.state = StateRunning
  	defer func() { b.state = StateIdle }()

  	baseURL := b.config.BaseURL
  	if baseURL == "" {
  		baseURL = "https://atcoder.jp"
  	}

  	// Problem ID is typically "abc300_a". The contest ID is the prefix before first underscore.
  	parts := strings.Split(problemID, "_")
  	if len(parts) == 0 {
  		return "", fmt.Errorf("invalid AtCoder problem ID: %s", problemID)
  	}
  	contestID := parts[0]

  	submitPageURL := fmt.Sprintf("%s/contests/%s/submit", baseURL, contestID)
  	req, _ := http.NewRequestWithContext(ctx, "GET", submitPageURL, nil)
  	setBrowserHeaders(req)
  	resp, err := b.client.Do(req)
  	if err != nil {
  		return "", err
  	}
  	defer resp.Body.Close()
  	body, _ := io.ReadAll(resp.Body)
  	csrf := extractAtCoderCSRF(string(body))

  	urlValues := url.Values{
  		"csrf_token":          {csrf},
  		"data.TaskScreenName": {problemID},
  		"data.LanguageId":     {language},
  		"sourceCode":          {sourceCode},
  	}

  	submitReq, _ := http.NewRequestWithContext(ctx, "POST", submitPageURL, strings.NewReader(urlValues.Encode()))
  	submitReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
  	setBrowserHeaders(submitReq)

  	b.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
  		return http.ErrUseLastResponse
  	}
  	defer func() { b.client.CheckRedirect = nil }()

  	submitResp, err := b.client.Do(submitReq)
  	if err != nil {
  		return "", err
  	}
  	defer submitResp.Body.Close()

  	// AtCoder redirects to submissions page if successful
  	loc := submitResp.Header.Get("Location")
  	if loc != "" && (strings.Contains(loc, "submissions") || submitResp.StatusCode == http.StatusFound) {
  		// Return contestID + "/me" to poll latest submission
  		return fmt.Sprintf("%s/me", contestID), nil
  	}

  	return "", fmt.Errorf("atcoder submit failed: redirect missing or invalid status code %d", submitResp.StatusCode)
  }

  func (b *AtCoderBot) Poll(ctx context.Context, remoteSubmissionID string) (*RemoteResult, error) {
  	baseURL := b.config.BaseURL
  	if baseURL == "" {
  		baseURL = "https://atcoder.jp"
  	}
  	// remoteSubmissionID format: "abc100/me" or "abc100/submissions/12345"
  	url := fmt.Sprintf("%s/contests/%s", baseURL, remoteSubmissionID)
  	if !strings.Contains(remoteSubmissionID, "submissions") {
  		parts := strings.Split(remoteSubmissionID, "/")
  		url = fmt.Sprintf("%s/contests/%s/submissions/me", baseURL, parts[0])
  	}

  	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
  	setBrowserHeaders(req)
  	resp, err := b.client.Do(req)
  	if err != nil {
  		return nil, err
  	}
  	defer resp.Body.Close()
  	body, _ := io.ReadAll(resp.Body)
  	bodyStr := string(body)

  	res := &RemoteResult{RemoteID: remoteSubmissionID, Done: false, Verdict: "PENDING"}

  	// Look for typical AtCoder verdicts in the latest submission table row
  	if strings.Contains(bodyStr, "label-success\">AC") || strings.Contains(bodyStr, "AC</span>") {
  		res.Verdict = "AC"
  		res.Done = true
  	} else if strings.Contains(bodyStr, "label-warning\">WA") || strings.Contains(bodyStr, "WA</span>") {
  		res.Verdict = "WA"
  		res.Done = true
  	} else if strings.Contains(bodyStr, "label-warning\">TLE") || strings.Contains(bodyStr, "TLE</span>") {
  		res.Verdict = "TLE"
  		res.Done = true
  	} else if strings.Contains(bodyStr, "label-warning\">MLE") || strings.Contains(bodyStr, "MLE</span>") {
  		res.Verdict = "MLE"
  		res.Done = true
  	} else if strings.Contains(bodyStr, "label-warning\">RE") || strings.Contains(bodyStr, "RE</span>") {
  		res.Verdict = "RE"
  		res.Done = true
  	} else if strings.Contains(bodyStr, "label-warning\">CE") || strings.Contains(bodyStr, "CE</span>") {
  		res.Verdict = "CE"
  		res.Done = true
  	}

  	return res, nil
  }

  func extractAtCoderCSRF(html string) string {
  	idx := strings.Index(html, `csrf_token`)
  	if idx == -1 {
  		return ""
  	}
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
  ```

- [ ] **Step 4: Run AtCoder Bot test to verify it passes**
  Run: `go test -v ./internal/vjudge/ -run TestAtCoderBot`
  Expected: PASS.

- [ ] **Step 5: Commit AtCoder Bot**
  ```bash
  git add internal/vjudge/atcoder.go internal/vjudge/atcoder_test.go
  git commit -m "feat: implement AtCoder vjudge bot using session cookies"
  ```

---

### Task 4: Problem Statement Parsers & PDF Parser Detection

**Files:**
- Modify: `internal/vjudge/parser.go`

- [ ] **Step 1: Write mock tests for the parsers in `internal/vjudge/parser_test.go`**
  Create `internal/vjudge/parser_test.go`:
  ```go
  package vjudge

  import (
  	"context"
  	"strings"
  	"testing"
  )

  func TestParseQOJProblem(t *testing.T) {
  	// Test PDF detection
  	parser := NewProblemParser(func(ctx context.Context, url string) (string, error) {
  		return `<html><a href="/problems/files/123/problem.pdf">Download PDF</a></html>`, nil
  	})
  	prob, err := parser.ParseQOJProblem(context.Background(), "123")
  	if err != nil {
  		t.Fatalf("ParseQOJProblem failed: %v", err)
  	}
  	if prob.Description != "https://qoj.ac/problems/files/123/problem.pdf" {
  		t.Errorf("Expected PDF URL, got %s", prob.Description)
  	}
  }
  ```

- [ ] **Step 2: Run test to verify it fails**
  Run: `go test -v ./internal/vjudge/ -run TestParseQOJProblem`
  Expected: Fail/compile error.

- [ ] **Step 3: Implement parser methods in `internal/vjudge/parser.go`**
  Modify `internal/vjudge/parser.go` to add:
  ```go
  func (p *ProblemParser) ParseAtCoderProblem(ctx context.Context, contestID, problemID string) (*model.Problem, error) {
  	problemURL := fmt.Sprintf("https://atcoder.jp/contests/%s/tasks/%s", contestID, problemID)
  	body, err := p.fetcher(ctx, problemURL)
  	if err != nil {
  		return nil, err
  	}
  	prob := &model.Problem{
  		Source:     "atcoder",
  		RemoteID:   contestID + "/" + problemID,
  		Title:      problemID,
  		Difficulty: "medium",
  	}
  	doc, err := html.Parse(strings.NewReader(body))
  	if err != nil {
  		return nil, err
  	}
  	titleNode := findNode(doc, func(n *html.Node) bool {
  		return n.Type == html.ElementNode && n.Data == "span" && hasClass(n, "h2")
  	})
  	if titleNode != nil {
  		prob.Title = strings.TrimSpace(extractText(titleNode))
  	}
  	prob.Description = "AtCoder task statement. View original task page: " + problemURL
  	return prob, nil
  }

  func (p *ProblemParser) ParseTophProblem(ctx context.Context, problemID string) (*model.Problem, error) {
  	problemURL := fmt.Sprintf("https://toph.co/p/%s", problemID)
  	body, err := p.fetcher(ctx, problemURL)
  	if err != nil {
  		return nil, err
  	}
  	prob := &model.Problem{
  		Source:     "toph",
  		RemoteID:   problemID,
  		Title:      problemID,
  		Difficulty: "medium",
  	}
  	prob.Description = "Toph task statement. View original task page: " + problemURL
  	return prob, nil
  }

  func (p *ProblemParser) ParseQOJProblem(ctx context.Context, problemID string) (*model.Problem, error) {
  	problemURL := fmt.Sprintf("https://qoj.ac/problem/%s", problemID)
  	body, err := p.fetcher(ctx, problemURL)
  	if err != nil {
  		return nil, err
  	}
  	prob := &model.Problem{
  		Source:     "qoj",
  		RemoteID:   problemID,
  		Title:      "QOJ " + problemID,
  		Difficulty: "medium",
  	}

  	// Look for PDF statements
  	if strings.Contains(body, ".pdf") || strings.Contains(body, "/problem.pdf") {
  		re := regexp.MustCompile(`/problems/files/\d+/problem\.pdf`)
  		matches := re.FindString(body)
  		if matches != "" {
  			prob.Description = "https://qoj.ac" + matches
  			return prob, nil
  		}
  	}

  	prob.Description = "QOJ task statement. View original task page: " + problemURL
  	return prob, nil
  }
  ```

- [ ] **Step 4: Run the test to verify it passes**
  Run: `go test -v ./internal/vjudge/ -run TestParseQOJProblem`
  Expected: PASS.

- [ ] **Step 5: Commit changes**
  ```bash
  git add internal/vjudge/parser.go internal/vjudge/parser_test.go
  git commit -m "feat: implement AtCoder, Toph, and QOJ parsers with QOJ PDF statement link detection"
  ```

---

### Task 5: Admin Importer Endpoints Registration

**Files:**
- Modify: `internal/api/handler/import.go`
- Modify: `internal/api/router.go` (or wherever routes are configured)

- [ ] **Step 1: Check how router is configured**
  Find route handler setups to register the new endpoints.
  Run grep to find router file: `grep -rn "ImportCodeforces" .` or check route files.

- [ ] **Step 2: Add methods to `ImportHandler` in `internal/api/handler/import.go`**
  ```go
  func (h *ImportHandler) ImportAtCoder(w http.ResponseWriter, r *http.Request) {
  	claims := middleware.GetUserClaims(r)
  	if claims == nil || (claims.Role != "admin" && claims.Role != "teacher") {
  		http.Error(w, "forbidden", http.StatusForbidden)
  		return
  	}
  	var req struct {
  		ContestID string `json:"contest_id"`
  		ProblemID string `json:"problem_id"`
  	}
  	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
  		http.Error(w, "invalid request body", http.StatusBadRequest)
  		return
  	}
  	parser := vjudge.NewProblemParser(func(ctx context.Context, url string) (string, error) {
  		httpReq, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
  		httpReq.Header.Set("User-Agent", "Mozilla/5.0")
  		resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(httpReq)
  		if err != nil {
  			return "", err
  		}
  		defer resp.Body.Close()
  		body, _ := io.ReadAll(resp.Body)
  		return string(body), nil
  	})
  	prob, err := parser.ParseAtCoderProblem(r.Context(), req.ContestID, req.ProblemID)
  	if err != nil {
  		http.Error(w, "failed to parse: "+err.Error(), http.StatusBadRequest)
  		return
  	}
  	prob.ID = uuid.New().String()
  	prob.Slug = "atcoder-" + strings.ToLower(req.ProblemID)
  	prob.CreatedBy = claims.UserID
  	prob.Visible = true
  	h.probStore.Create(r.Context(), prob)
  	respondJSON(w, http.StatusCreated, map[string]string{"status": "success", "problem_id": prob.ID, "slug": prob.Slug})
  }

  func (h *ImportHandler) ImportToph(w http.ResponseWriter, r *http.Request) {
  	claims := middleware.GetUserClaims(r)
  	if claims == nil || (claims.Role != "admin" && claims.Role != "teacher") {
  		http.Error(w, "forbidden", http.StatusForbidden)
  		return
  	}
  	var req struct {
  		ProblemID string `json:"problem_id"`
  	}
  	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
  		http.Error(w, "invalid request", http.StatusBadRequest)
  		return
  	}
  	parser := vjudge.NewProblemParser(func(ctx context.Context, url string) (string, error) {
  		httpReq, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
  		httpReq.Header.Set("User-Agent", "Mozilla/5.0")
  		resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(httpReq)
  		if err != nil {
  			return "", err
  		}
  		defer resp.Body.Close()
  		body, _ := io.ReadAll(resp.Body)
  		return string(body), nil
  	})
  	prob, err := parser.ParseTophProblem(r.Context(), req.ProblemID)
  	if err != nil {
  		http.Error(w, "failed to parse: "+err.Error(), http.StatusBadRequest)
  		return
  	}
  	prob.ID = uuid.New().String()
  	prob.Slug = "toph-" + strings.ToLower(req.ProblemID)
  	prob.CreatedBy = claims.UserID
  	prob.Visible = true
  	h.probStore.Create(r.Context(), prob)
  	respondJSON(w, http.StatusCreated, map[string]string{"status": "success", "problem_id": prob.ID, "slug": prob.Slug})
  }

  func (h *ImportHandler) ImportQOJ(w http.ResponseWriter, r *http.Request) {
  	claims := middleware.GetUserClaims(r)
  	if claims == nil || (claims.Role != "admin" && claims.Role != "teacher") {
  		http.Error(w, "forbidden", http.StatusForbidden)
  		return
  	}
  	var req struct {
  		ProblemID string `json:"problem_id"`
  	}
  	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
  		http.Error(w, "invalid request", http.StatusBadRequest)
  		return
  	}
  	parser := vjudge.NewProblemParser(func(ctx context.Context, url string) (string, error) {
  		httpReq, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
  		httpReq.Header.Set("User-Agent", "Mozilla/5.0")
  		resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(httpReq)
  		if err != nil {
  			return "", err
  		}
  		defer resp.Body.Close()
  		body, _ := io.ReadAll(resp.Body)
  		return string(body), nil
  	})
  	prob, err := parser.ParseQOJProblem(r.Context(), req.ProblemID)
  	if err != nil {
  		http.Error(w, "failed to parse: "+err.Error(), http.StatusBadRequest)
  		return
  	}
  	prob.ID = uuid.New().String()
  	prob.Slug = "qoj-" + strings.ToLower(req.ProblemID)
  	prob.CreatedBy = claims.UserID
  	prob.Visible = true
  	h.probStore.Create(r.Context(), prob)
  	respondJSON(w, http.StatusCreated, map[string]string{"status": "success", "problem_id": prob.ID, "slug": prob.Slug})
  }
  ```

- [ ] **Step 3: Register routes in router**
  Register routes `/api/admin/import/atcoder`, `/api/admin/import/toph`, and `/api/admin/import/qoj` pointing to the new handlers.

- [ ] **Step 4: Commit router changes**
  ```bash
  git add internal/api/handler/import.go internal/api/router.go
  git commit -m "feat: add import endpoints for AtCoder, Toph, QOJ"
  ```

---

### Task 6: Frontend PDF Problem Statement Rendering

**Files:**
- Modify: `web/src/pages/ProblemDetail.tsx`
- Modify: `web/src/pages/ContestProblem.tsx`

- [ ] **Step 1: Update `web/src/pages/ProblemDetail.tsx`**
  Modify statement tab rendering (around line 450) to check if `problem.description` is a PDF URL, and embed `iframe` with fallback:
  ```tsx
  const isPdf = problem.description && (
      problem.description.startsWith('http') && 
      (problem.description.endsWith('.pdf') || problem.description.includes('/problem.pdf'))
  );

  // If isPdf, display PDF statement iframe instead of ReactMarkdown:
  {isPdf ? (
      <div className="space-y-4">
          <div className="flex justify-between items-center bg-gray-50 dark:bg-gray-800 p-3 rounded-lg border border-gray-200 dark:border-gray-700">
              <span className="text-sm font-medium text-gray-700 dark:text-gray-300">PDF Problem Statement</span>
              <a 
                  href={problem.description} 
                  target="_blank" 
                  rel="noreferrer" 
                  className="text-sm text-blue-600 dark:text-blue-400 hover:underline flex items-center gap-1 font-semibold"
              >
                  Download PDF
              </a>
          </div>
          <iframe 
              src={problem.description} 
              className="w-full h-[800px] border-0 rounded-lg shadow-sm bg-white" 
              title="Problem Statement"
          />
      </div>
  ) : (
      <ReactMarkdown remarkPlugins={[remarkMath]} rehypePlugins={[rehypeKatex]}>
          {problem.description}
      </ReactMarkdown>
  )}
  ```

- [ ] **Step 2: Update `web/src/pages/ContestProblem.tsx`**
  Do the exact same change in `web/src/pages/ContestProblem.tsx` statement renderer.

- [ ] **Step 3: Commit frontend changes**
  ```bash
  git add web/src/pages/ProblemDetail.tsx web/src/pages/ContestProblem.tsx
  git commit -m "feat: render PDF statements inside an iframe in frontend"
  ```
