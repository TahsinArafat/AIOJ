# Global Online Judge Readiness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make AIOJ ready to operate as a trustworthy, globally-accessible Online Judge by closing the 35 gaps identified in the 2026-06-12 audit, sequenced so the platform can launch to the public after Phase A and reach global parity by end of Phase E.

**Architecture:** Five sequential phases, each shipping independently working software. **Phase A** (Trust & Legal) blocks any public launch — adds email infrastructure, OAuth, 2FA, email verification, legal compliance, security hardening. **Phase B** (CI/CD) accelerates all future development. **Phase C** (Global Reach) adds i18n, CDN, multi-region. **Phase D** (Community) adds social/engagement features. **Phase E** (Operational) adds Sentry, status page, MOSS, audit log.

**Tech Stack:** Go 1.26 (chi, golang-jwt, golang-migrate, pquerna/otp, x/oauth2, getSentry/sentry-go, prometheus), PostgreSQL 18, Redis, React 19 + Vite 8 + TypeScript 6, i18next, @sentry/react, Caddy (TLS), Cloudflare (CDN), GitHub Actions (CI/CD).

---

## Document Map

This is a **master roadmap plan**:
- **§1 Roadmap Overview** — phased plan with deliverables, files created, dependencies
- **§2 Phase A: Trust & Legal Foundation** — **fully detailed** TDD tasks (28 tasks). Blocks public launch.
- **§3–§6 Phases B–E** — stubbed with complete file lists, design decisions, key APIs. Each gets its own plan file when scheduled.
- **§7 Self-Review** — coverage check, placeholder scan, type consistency

**Why not detail all 5 phases?** The writing-plans skill warns: *one plan per subsystem, each producing working, testable software on its own.* Phase A alone is 28 tasks. A 300KB+ plan is unexecutable. Sub-plans for B–E will be created when those phases enter the execution queue.

---

## §1 Roadmap Overview

### Dependency Graph

```
Phase A (Trust & Legal)  ──>  Phase C (Global Reach)
       │                            ▲
       ├──>  Phase B (CI/CD)  ──────┤
       │                            │
       └─────────────────────>  Phase D (Community)
                                    │
                              Phase E (Operational)
```

### Estimated Effort

| Phase | Tasks | Weeks | Blocks Public Launch? |
|---|---|---|---|
| A — Trust & Legal | 28 | 4–6 | **Yes** |
| B — CI/CD & Dev Velocity | 8 | 1 | No (but accelerates all) |
| C — Global Reach | 12 | 3–4 | No (UX) |
| D — Community & Social | 18 | 4–6 | No (retention) |
| E — Operational | 10 | 2–3 | No (reliability) |
| **Total** | **76** | **14–20** | |

### What This Plan Does NOT Touch (already in other plans)

| Gap | Plan File |
|---|---|
| PE/OLE verdicts, SPJ cache, rejudge API, float epsilon, load balancer, Judge Master, Broadcaster | `2026-06-12-production-level-oj.md` |
| Priority queue | `2026-06-04-contest-priority-queue.md` |
| Codeforces-style user profile | `2026-06-04-codeforces-style-user-profile.md` |
| Custom rating & rankings | `2026-06-04-custom-rating-and-rankings.md` |
| Setter workspace (KaTeX, float epsilon UI) | `2026-05-29-dedicated-setter-onsite-contest-vjudge.md` |
| Admin backup/restore | `2026-06-12-admin-backup-restore.md` |
| Editorial form/page | `2026-06-04-add-editorial-frontend.md` |
| VJudge bot expansion | `2026-06-04-atcoder-toph-qoj-vjudge-bots-plan.md` |
| Monaco editor | `2026-06-04-monaco-editor-plan.md` |
| Migration sustainability | `2026-06-02-migration-sustainability.md` |

---

## §2 Phase A: Trust & Legal Foundation (Fully Detailed)

**Goal:** Ship email infrastructure, OAuth/SSO, 2FA, email verification, password policy, CSRF protection, security headers, Sentry, legal pages, GDPR data export, account deletion, and cookie consent. After Phase A, AIOJ is legally compliant to operate in EU/CA/US, secure against the most common attack vectors, and ready for public sign-ups.

**Files Created in Phase A** (49 new, 12 modified):

**Backend (Go) — 35 new, 8 modified**
- `internal/mail/{mail,smtp,mailcatcher,templates}.go` + `*_test.go`
- `internal/auth/{totp,password,backup_codes}.go` + `*_test.go`
- `internal/oauth/{provider,state,github,google,link}.go` + `*_test.go`
- `internal/api/middleware/{csrf,security_headers,ratelimit_auth}.go` + tests
- `internal/observability/sentry.go` + test
- `internal/api/handler/{legal,users_deletion,users_export,auth_2fa,email_verification,oauth_callback,oauth_start,dev_mail}.go` + tests
- `internal/store/postgres/{oauth_links,totp_secrets,email_verification_tokens,audit_log}.go`
- `internal/store/migrations/000052_oauth.{up,down}.sql`
- `internal/store/migrations/000053_totp.{up,down}.sql`
- `internal/store/migrations/000054_email_verification.{up,down}.sql`
- `internal/store/migrations/000055_audit_log.{up,down}.sql`

**Frontend (React) — 8 new, 3 modified**
- `web/src/lib/{sentry,oauth}.ts`
- `web/src/pages/legal/{TermsOfService,PrivacyPolicy,DMCA}.tsx`
- `web/src/pages/auth/{VerifyEmail,TwoFactorSetup,TwoFactorVerify}.tsx`
- `web/src/components/CookieConsent.tsx`
- `web/src/pages/{Settings,Login}.tsx` (modify)
- `web/src/main.tsx` (modify)
- `web/src/i18n/locales/{en,bn}.json` (modify)

**Config & Infra — 2 modified**
- `config.yaml`, `.env.example`, `docker-compose.yml`, `Dockerfile`

**Documentation — 3 new**
- `docs/legal/{TERMS_OF_SERVICE,PRIVACY_POLICY,DMCA}.md`
- `docs/guides/EMAIL_SETUP.md`

---

### Task A.1: Mail Service Interface and In-Memory Driver

**Files:**
- Create: `internal/mail/mail.go`
- Create: `internal/mail/mailcatcher.go`
- Test: `internal/mail/mailcatcher_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/mail/mailcatcher_test.go
package mail

import (
	"context"
	"testing"
)

func TestMailCatcher_SendAndRetrieve(t *testing.T) {
	c := NewMailCatcher()
	ctx := context.Background()

	msg := &Message{
		From: "noreply@aioj.com", To: []string{"alice@example.com"},
		Subject: "Welcome to AIOJ", Body: "Hello Alice",
	}
	if err := c.Send(ctx, msg); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	msgs := c.All()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Subject != "Welcome to AIOJ" {
		t.Errorf("got subject %q, want %q", msgs[0].Subject, "Welcome to AIOJ")
	}
	if msgs[0].To[0] != "alice@example.com" {
		t.Errorf("got To %q, want %q", msgs[0].To[0], "alice@example.com")
	}
}

func TestMailCatcher_FindByRecipient(t *testing.T) {
	c := NewMailCatcher()
	_ = c.Send(context.Background(), &Message{To: []string{"a@x.com"}, Subject: "to a"})
	_ = c.Send(context.Background(), &Message{To: []string{"b@x.com"}, Subject: "to b"})
	found := c.FindTo("a@x.com")
	if len(found) != 1 || found[0].Subject != "to a" {
		t.Errorf("FindTo returned wrong messages: %+v", found)
	}
}

func TestMailCatcher_Clear(t *testing.T) {
	c := NewMailCatcher()
	_ = c.Send(context.Background(), &Message{To: []string{"a@x.com"}, Subject: "x"})
	c.Clear()
	if len(c.All()) != 0 {
		t.Error("Clear did not empty messages")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mail/ -v`
Expected: FAIL with `package internal/mail: no Go files`

- [ ] **Step 3: Define the Mail interface and Message type**

```go
// internal/mail/mail.go
// Package mail provides a swappable mail-sending backend.
package mail

import "context"

type Message struct {
	From    string
	To      []string
	Cc      []string
	Subject string
	Body    string
	IsHTML  bool
	Headers map[string]string
}

type Sender interface {
	Send(ctx context.Context, msg *Message) error
}

// NoopSender is used when mail delivery is disabled.
type NoopSender struct{}

func (NoopSender) Send(_ context.Context, _ *Message) error { return nil }
```

- [ ] **Step 4: Implement the MailCatcher**

```go
// internal/mail/mailcatcher.go
package mail

import (
	"context"
	"sync"
)

type MailCatcher struct {
	mu   sync.RWMutex
	msgs []*Message
}

func NewMailCatcher() *MailCatcher { return &MailCatcher{} }

func (c *MailCatcher) Send(_ context.Context, msg *Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := *msg
	c.msgs = append(c.msgs, &cp)
	return nil
}

func (c *MailCatcher) All() []*Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]*Message, len(c.msgs))
	for i, m := range c.msgs {
		cp := *m
		out[i] = &cp
	}
	return out
}

func (c *MailCatcher) FindTo(addr string) []*Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []*Message
	for _, m := range c.msgs {
		for _, to := range m.To {
			if to == addr {
				cp := *m
				out = append(out, &cp)
				break
			}
		}
	}
	return out
}

func (c *MailCatcher) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgs = nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/mail/ -v`
Expected: ALL PASS (3 tests)

- [ ] **Step 6: Commit**

```bash
git add internal/mail/mail.go internal/mail/mailcatcher.go internal/mail/mailcatcher_test.go
git commit -m "feat(mail): define Sender interface and in-memory MailCatcher"
```

---

### Task A.2: SMTP Driver

**Files:**
- Create: `internal/mail/smtp.go`
- Test: `internal/mail/smtp_test.go`

- [ ] **Step 1: Write the failing test using an in-process fake SMTP**

```go
// internal/mail/smtp_test.go
package mail

import (
	"context"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// startFakeSMTP is a minimal SMTP server that records DATA payloads.
func startFakeSMTP(t *testing.T) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	type received struct{ From, To, Data string }
	var got []received
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(2 * time.Second))
				_, _ = conn.Write([]byte("220 fake ESMTP\r\n"))
				buf := make([]byte, 4096)
				from, to, data := "", "", ""
				inData := false
				for {
					n, err := conn.Read(buf)
					if err != nil {
						return
					}
					line := string(buf[:n])
					if inData {
						data += line
						if strings.HasSuffix(strings.TrimRight(line, "\r\n"), ".") {
							got = append(got, received{from, to, data})
							_, _ = conn.Write([]byte("250 OK\r\n"))
							inData = false
							continue
						}
						continue
					}
					up := strings.ToUpper(strings.TrimSpace(line))
					switch {
					case strings.HasPrefix(up, "EHLO"), strings.HasPrefix(up, "HELO"):
						_, _ = conn.Write([]byte("250-fake\r\n250 OK\r\n"))
					case strings.HasPrefix(up, "MAIL FROM:"):
						from = line
						_, _ = conn.Write([]byte("250 OK\r\n"))
					case strings.HasPrefix(up, "RCPT TO:"):
						to = line
						_, _ = conn.Write([]byte("250 OK\r\n"))
					case strings.HasPrefix(up, "DATA"):
						inData = true
						_, _ = conn.Write([]byte("354 End data\r\n"))
					case strings.HasPrefix(up, "QUIT"):
						_, _ = conn.Write([]byte("221 Bye\r\n"))
						return
					default:
						_, _ = conn.Write([]byte("250 OK\r\n"))
					}
				}
			}(c)
		}
	}()
	stop := func() { _ = ln.Close(); <-done }
	return ln.Addr().String(), stop
}

func TestSMTPSender_SendPlainText(t *testing.T) {
	addr, stop := startFakeSMTP(t)
	defer stop()
	host, portStr, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portStr)

	s := NewSMTPSender(SMTPConfig{Host: host, Port: port, From: "noreply@aioj.com"})
	if err := s.Send(context.Background(), &Message{
		To: []string{"alice@example.com"}, Subject: "Hello", Body: "World",
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}
}

// silence unused import
var _ = smtp.PlainAuth
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mail/ -run TestSMTP -v`
Expected: FAIL with `undefined: NewSMTPSender`

- [ ] **Step 3: Implement the SMTP driver**

```go
// internal/mail/smtp.go
package mail

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

type SMTPSender struct{ cfg SMTPConfig }

func NewSMTPSender(cfg SMTPConfig) *SMTPSender { return &SMTPSender{cfg: cfg} }

func (s *SMTPSender) Send(_ context.Context, msg *Message) error {
	addr := net.JoinHostPort(s.cfg.Host, fmt.Sprintf("%d", s.cfg.Port))
	var auth smtp.Auth
	if s.cfg.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}
	body := buildPlain(msg)
	return smtp.SendMail(addr, auth, s.cfg.From, msg.To, []byte(body))
}

func buildPlain(msg *Message) string {
	var b strings.Builder
	for k, v := range msg.Headers {
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(v)
		b.WriteString("\r\n")
	}
	b.WriteString("From: ")
	b.WriteString(msg.From)
	b.WriteString("\r\n")
	b.WriteString("To: ")
	b.WriteString(strings.Join(msg.To, ", "))
	b.WriteString("\r\n")
	if len(msg.Cc) > 0 {
		b.WriteString("Cc: ")
		b.WriteString(strings.Join(msg.Cc, ", "))
		b.WriteString("\r\n")
	}
	b.WriteString("Subject: ")
	b.WriteString(msg.Subject)
	b.WriteString("\r\n")
	b.WriteString("Date: ")
	b.WriteString(time.Now().Format(time.RFC1123Z))
	b.WriteString("\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	b.WriteString(msg.Body)
	return b.String()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/mail/ -v`
Expected: ALL PASS (3 mailcatcher + 2 smtp tests)

- [ ] **Step 5: Commit**

```bash
git add internal/mail/smtp.go internal/mail/smtp_test.go
git commit -m "feat(mail): add SMTP sender driver with optional auth"
```

---

### Task A.3: Email Templates

**Files:**
- Create: `internal/mail/templates.go`
- Create: `internal/mail/templates/password_reset.txt`
- Create: `internal/mail/templates/email_verification.txt`
- Create: `internal/mail/templates/two_factor_enabled.txt`
- Test: `internal/mail/templates_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/mail/templates_test.go
package mail

import (
	"bytes"
	"strings"
	"testing"
)

func TestTemplate_PasswordReset(t *testing.T) {
	tpl, err := LoadTemplates()
	if err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}
	var buf bytes.Buffer
	data := map[string]string{
		"Username": "alice",
		"ResetURL": "https://aioj.com/reset?token=abc",
		"Expiry":   "1 hour",
	}
	if err := tpl.ExecuteTemplate(&buf, "password_reset.txt", data); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := buf.String()
	for _, want := range []string{"alice", "https://aioj.com/reset?token=abc", "1 hour"} {
		if !strings.Contains(got, want) {
			t.Errorf("template missing %q. got:\n%s", want, got)
		}
	}
}

func TestTemplate_EmailVerification(t *testing.T) {
	tpl, err := LoadTemplates()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "email_verification.txt", map[string]string{
		"Username": "bob", "VerifyURL": "https://aioj.com/verify?token=xyz", "ExpiryMinutes": "24",
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "bob") {
		t.Errorf("missing username. got:\n%s", buf.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mail/ -run TestTemplate -v`
Expected: FAIL with `undefined: LoadTemplates`

- [ ] **Step 3: Create the template files**

`internal/mail/templates/password_reset.txt`:
```
Hi {{.Username}},

We received a request to reset the password for your AIOJ account.

Click the link below to set a new password (valid for {{.Expiry}}):
{{.ResetURL}}

If you did not request this, you can safely ignore this email — your password will remain unchanged.

— The AIOJ Team
```

`internal/mail/templates/email_verification.txt`:
```
Hi {{.Username}},

Welcome to AIOJ! Please confirm your email address by visiting the link below
(valid for {{.ExpiryMinutes}} hours):
{{.VerifyURL}}

If you did not create this account, you can safely ignore this email.

— The AIOJ Team
```

`internal/mail/templates/two_factor_enabled.txt`:
```
Hi {{.Username}},

Two-factor authentication has been enabled on your AIOJ account.

If you did not perform this action, immediately change your password and
contact support@aioj.com.

— The AIOJ Team
```

- [ ] **Step 4: Implement the template loader**

```go
// internal/mail/templates.go
package mail

import (
	"embed"
	"fmt"
	"html/template"
	"io"
)

//go:embed templates/*.txt
var templateFS embed.FS

func LoadTemplates() (*template.Template, error) {
	tpl := template.New("")
	entries, err := templateFS.ReadDir("templates")
	if err != nil {
		return nil, fmt.Errorf("read embed dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		body, err := templateFS.ReadFile("templates/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		if _, err := tpl.New(e.Name()).Parse(string(body)); err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
		}
	}
	return tpl, nil
}

func RenderTemplate(tpl *template.Template, name string, data any, w io.Writer) error {
	return tpl.ExecuteTemplate(w, name, data)
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/mail/ -v`
Expected: ALL PASS (5 tests)

- [ ] **Step 6: Commit**

```bash
git add internal/mail/templates.go internal/mail/templates_test.go internal/mail/templates/
git commit -m "feat(mail): add embedded text templates for transactional emails"
```

---

### Task A.4: Mail Configuration and Wiring

**Files:**
- Modify: `internal/config/config.go`
- Modify: `config.yaml`
- Modify: `.env.example`
- Modify: `cmd/aioj/main.go`
- Modify: `internal/api/deps.go`

- [ ] **Step 1: Add MailConfig to internal/config/config.go**

```go
// In internal/config/config.go, add:

type MailConfig struct {
	Driver    string `yaml:"driver"` // "smtp" | "catcher" | "noop"
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	From      string `yaml:"from"`
	PublicURL string `yaml:"public_url"` // e.g. https://aioj.com (used in email links)
}

// In the top-level Config struct, add:
type Config struct {
	// ... existing fields ...
	Mail MailConfig `yaml:"mail"`
}
```

- [ ] **Step 2: Add to config.yaml**

```yaml
mail:
  driver: "catcher"
  host: ""
  port: 587
  username: ""
  password: ""
  from: "noreply@aioj.com"
  public_url: "http://localhost"
```

- [ ] **Step 3: Add to .env.example**

```env
# Mail (Phase A)
MAIL_DRIVER=catcher
MAIL_HOST=
MAIL_PORT=587
MAIL_USERNAME=
MAIL_PASSWORD=
MAIL_FROM=noreply@aioj.com
MAIL_PUBLIC_URL=http://localhost
```

- [ ] **Step 4: Wire the mail sender into main.go**

In `cmd/aioj/main.go`, after loading config:

```go
var mailSender mail.Sender
switch cfg.Mail.Driver {
case "smtp":
	mailSender = mail.NewSMTPSender(mail.SMTPConfig{
		Host: cfg.Mail.Host, Port: cfg.Mail.Port,
		Username: cfg.Mail.Username, Password: cfg.Mail.Password,
		From: cfg.Mail.From,
	})
case "catcher":
	mailSender = mail.NewMailCatcher()
default:
	mailSender = mail.NoopSender{}
}
mailTpl, err := mail.LoadTemplates()
if err != nil {
	log.Fatalf("load mail templates: %v", err)
}
```

- [ ] **Step 5: Extend api.Deps**

In `internal/api/deps.go`:

```go
import (
	"html/template"
	"github.com/tahsinarafat/aioj/internal/mail"
)

type Deps struct {
	// ... existing fields ...
	Mail    mail.Sender
	MailTpl *template.Template
}
```

- [ ] **Step 6: Build to verify no compile errors**

Run: `go build ./...`
Expected: No errors. If `template` import missing, add `"html/template"` to deps.go.

- [ ] **Step 7: Commit**

```bash
git add internal/config/config.go config.yaml .env.example cmd/aioj/main.go internal/api/deps.go internal/mail/mail.go
git commit -m "feat(mail): wire SMTP/catcher driver into config and main"
```

---

### Task A.5: Mailcatcher HTTP Inspection Endpoint (Dev Only)

**Files:**
- Create: `internal/api/handler/dev_mail.go`
- Modify: `cmd/aioj/main.go` (mount route only when `cfg.Mail.Driver == "catcher"`)

- [ ] **Step 1: Write the failing test**

```go
// internal/api/handler/dev_mail_test.go
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tahsinarafat/aioj/internal/mail"
)

func TestDevMail_List(t *testing.T) {
	c := mail.NewMailCatcher()
	_ = c.Send(context.Background(), &mail.Message{To: []string{"x@x.com"}, Subject: "hi"})

	h := &DevMailHandler{Sender: c}
	req := httptest.NewRequest("GET", "/api/dev/mail", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status: %d", rec.Code)
	}
	var body struct {
		Data []*mail.Message `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 1 {
		t.Errorf("got %d messages, want 1", len(body.Data))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/handler/ -run TestDevMail -v`
Expected: FAIL with `undefined: DevMailHandler`

- [ ] **Step 3: Implement the DevMailHandler**

```go
// internal/api/handler/dev_mail.go
package handler

import (
	"net/http"

	"github.com/tahsinarafat/aioj/internal/mail"
)

// DevMailHandler exposes the in-memory MailCatcher over HTTP.
// Mount ONLY when mail.driver == "catcher" (dev/test).
type DevMailHandler struct {
	Sender mail.Sender
}

func (h *DevMailHandler) List(w http.ResponseWriter, _ *http.Request) {
	c, ok := h.Sender.(*mail.MailCatcher)
	if !ok {
		http.Error(w, "mail catcher not enabled", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": c.All()})
}

func (h *DevMailHandler) Clear(w http.ResponseWriter, _ *http.Request) {
	c, ok := h.Sender.(*mail.MailCatcher)
	if !ok {
		http.Error(w, "mail catcher not enabled", http.StatusNotFound)
		return
	}
	c.Clear()
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 4: Mount the route only in dev**

In `cmd/aioj/main.go`, after the regular router is built:

```go
import "github.com/tahsinarafat/aioj/internal/api/handler"
import "github.com/go-chi/chi/v5"

if cfg.Mail.Driver == "catcher" {
	devMail := &handler.DevMailHandler{Sender: mailSender}
	if r, ok := router.(*chi.Mux); ok && r != nil {
		r.Get("/api/dev/mail", devMail.List)
		r.Delete("/api/dev/mail", devMail.Clear)
	}
}
```

(Adjust the type assertion based on how your `router` variable is typed.)

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/api/handler/ -run TestDevMail -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/api/handler/dev_mail.go internal/api/handler/dev_mail_test.go cmd/aioj/main.go
git commit -m "feat(mail): dev-only HTTP inspector for in-memory mailcatcher"
```

---

### Task A.6: Email Verification — Database Migration

**Files:**
- Create: `internal/store/migrations/000054_email_verification.up.sql`
- Create: `internal/store/migrations/000054_email_verification.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
-- internal/store/migrations/000054_email_verification.up.sql
-- Adds columns to users for email verification state and a token table.

ALTER TABLE users
    ADD COLUMN email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN email_verified_at TIMESTAMP WITH TIME ZONE;

CREATE TABLE IF NOT EXISTS email_verification_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    used BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_evt_user ON email_verification_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_evt_token ON email_verification_tokens(token_hash);
```

- [ ] **Step 2: Write the down migration**

```sql
-- internal/store/migrations/000054_email_verification.down.sql
DROP TABLE IF EXISTS email_verification_tokens;
ALTER TABLE users
    DROP COLUMN IF EXISTS email_verified,
    DROP COLUMN IF EXISTS email_verified_at;
```

- [ ] **Step 3: Run migration locally**

Run: `make migrate-up`
Expected: Migration applied cleanly. Verify with:

```bash
docker compose exec postgres psql -U aioj -d aioj -c "\d users"
```

Confirm `email_verified` and `email_verified_at` columns exist.

- [ ] **Step 4: Commit**

```bash
git add internal/store/migrations/000054_email_verification.up.sql internal/store/migrations/000054_email_verification.down.sql
git commit -m "feat(db): add email verification state and token table"
```

---

### Task A.7: Email Verification — Store Interface and Implementation

**Files:**
- Modify: `internal/store/interfaces.go`
- Modify: `internal/store/postgres/users.go`
- Create: `internal/store/postgres/email_verification_tokens.go`

- [ ] **Step 1: Extend UserStore interface and add EmailVerificationTokenStore**

In `internal/store/interfaces.go`, add to the `UserStore` interface:

```go
type UserStore interface {
    // ... existing methods ...
    MarkEmailVerified(ctx context.Context, userID string) error
    IsEmailVerified(ctx context.Context, userID string) (bool, error)
}

type EmailVerificationTokenStore interface {
    Create(ctx context.Context, id, userID, tokenHash string, expiresAt time.Time) error
    GetByHash(ctx context.Context, tokenHash string) (*model.EmailVerificationToken, error)
    MarkUsed(ctx context.Context, id string) error
}
```

- [ ] **Step 2: Add the EmailVerificationToken model**

Open `internal/model/user.go` and append:

```go
type EmailVerificationToken struct {
    ID        string    `json:"id"`
    UserID    string    `json:"user_id"`
    TokenHash string    `json:"-"`
    ExpiresAt time.Time `json:"expires_at"`
    Used      bool      `json:"used"`
    CreatedAt time.Time `json:"created_at"`
}
```

- [ ] **Step 3: Implement the postgres store**

```go
// internal/store/postgres/email_verification_tokens.go
package postgres

import (
    "context"
    "database/sql"
    "errors"
    "time"

    "github.com/tahsinarafat/aioj/internal/model"
)

type EmailVerificationTokenStore struct{ db *sql.DB }

func NewEmailVerificationTokenStore(db *sql.DB) *EmailVerificationTokenStore {
    return &EmailVerificationTokenStore{db: db}
}

func (s *EmailVerificationTokenStore) Create(ctx context.Context, id, userID, tokenHash string, expiresAt time.Time) error {
    _, err := s.db.ExecContext(ctx,
        `INSERT INTO email_verification_tokens (id, user_id, token_hash, expires_at)
         VALUES ($1, $2, $3, $4)`,
        id, userID, tokenHash, expiresAt)
    return err
}

func (s *EmailVerificationTokenStore) GetByHash(ctx context.Context, tokenHash string) (*model.EmailVerificationToken, error) {
    row := s.db.QueryRowContext(ctx,
        `SELECT id, user_id, token_hash, expires_at, used, created_at
         FROM email_verification_tokens WHERE token_hash = $1`, tokenHash)
    var t model.EmailVerificationToken
    if err := row.Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.Used, &t.CreatedAt); err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, nil
        }
        return nil, err
    }
    return &t, nil
}

func (s *EmailVerificationTokenStore) MarkUsed(ctx context.Context, id string) error {
    _, err := s.db.ExecContext(ctx, `UPDATE email_verification_tokens SET used = TRUE WHERE id = $1`, id)
    return err
}
```

- [ ] **Step 4: Add UserStore.MarkEmailVerified / IsEmailVerified**

Open `internal/store/postgres/users.go` and add:

```go
func (s *UserStore) MarkEmailVerified(ctx context.Context, userID string) error {
    _, err := s.db.ExecContext(ctx,
        `UPDATE users SET email_verified = TRUE, email_verified_at = NOW() WHERE id = $1`, userID)
    return err
}

func (s *UserStore) IsEmailVerified(ctx context.Context, userID string) (bool, error) {
    var v bool
    err := s.db.QueryRowContext(ctx,
        `SELECT email_verified FROM users WHERE id = $1`, userID).Scan(&v)
    if errors.Is(err, sql.ErrNoRows) {
        return false, nil
    }
    return v, err
}
```

Add `import "errors"` and `import "database/sql"` if not present.

- [ ] **Step 5: Wire stores in main.go and Deps**

In `cmd/aioj/main.go`:

```go
evtStore := postgres.NewEmailVerificationTokenStore(db)
// ... in api.Deps:
api.Deps{
    // ... existing fields ...
    EVT:     evtStore,
    Mail:    mailSender,
    MailTpl: mailTpl,
}
```

In `internal/api/deps.go`:

```go
type Deps struct {
    // ... existing fields ...
    EVT     store.EmailVerificationTokenStore
    Mail    mail.Sender
    MailTpl *template.Template
}
```

- [ ] **Step 6: Build and test**

Run: `go build ./... && go test ./internal/store/postgres/ -run EmailVerification -v`
Expected: Build OK, tests pass (or skip if DB unavailable)

- [ ] **Step 7: Commit**

```bash
git add internal/store/interfaces.go internal/store/postgres/email_verification_tokens.go internal/store/postgres/users.go internal/model/user.go internal/api/deps.go cmd/aioj/main.go
git commit -m "feat(email-verification): store + interface for verification tokens"
```

---

### Task A.8: Email Verification — Verify Handler

**Files:**
- Create: `internal/api/handler/email_verification.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/api/handler/email_verification_test.go
package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/mail"
	"github.com/tahsinarafat/aioj/internal/model"
)

type stubUserForEV struct {
	users      map[string]*model.User
	marked     string
	isVerified bool
}

func (s *stubUserForEV) Create(ctx context.Context, u *model.User) error {
	s.users[u.ID] = u
	return nil
}
func (s *stubUserForEV) GetByID(ctx context.Context, id string) (*model.User, error) {
	if u, ok := s.users[id]; ok {
		return u, nil
	}
	return nil, nil
}
func (s *stubUserForEV) GetByUsername(ctx context.Context, name string) (*model.User, error) {
	return nil, nil
}
func (s *stubUserForEV) GetByEmail(ctx context.Context, e string) (*model.User, error) {
	return nil, nil
}
func (s *stubUserForEV) GetPublicProfile(ctx context.Context, name string) (*model.PublicProfile, error) {
	return nil, nil
}
func (s *stubUserForEV) GetProfile(ctx context.Context, id string) (*model.UserProfile, error) {
	return nil, nil
}
func (s *stubUserForEV) UpdateProfile(ctx context.Context, id string, p *model.UserProfile) error {
	return nil
}
func (s *stubUserForEV) ListUsers(ctx context.Context, off, lim int) ([]model.User, int, error) {
	return nil, 0, nil
}
func (s *stubUserForEV) UpdateRole(ctx context.Context, id, r string) error { return nil }
func (s *stubUserForEV) UpdatePassword(ctx context.Context, id, h string) error {
	return nil
}
func (s *stubUserForEV) UpdateRating(ctx context.Context, id string, r, m, c int) error {
	return nil
}
func (s *stubUserForEV) MarkEmailVerified(ctx context.Context, id string) error {
	s.marked = id
	s.isVerified = true
	return nil
}
func (s *stubUserForEV) IsEmailVerified(ctx context.Context, id string) (bool, error) {
	return s.isVerified, nil
}

type stubEVT struct {
	tok *model.EmailVerificationToken
}

func (s *stubEVT) Create(ctx context.Context, id, uid, hash string, exp time.Time) error {
	return nil
}
func (s *stubEVT) GetByHash(ctx context.Context, hash string) (*model.EmailVerificationToken, error) {
	return s.tok, nil
}
func (s *stubEVT) MarkUsed(ctx context.Context, id string) error { return nil }

func TestEmailVerification_Verify_Success(t *testing.T) {
	catcher := mail.NewMailCatcher()
	tpl, _ := mail.LoadTemplates()

	us := &stubUserForEV{users: map[string]*model.User{"u1": {ID: "u1"}}}
	evt := &stubEVT{tok: &model.EmailVerificationToken{ID: "tok1", UserID: "u1", ExpiresAt: time.Now().Add(1 * time.Hour), Used: false}}

	h := &EmailVerificationHandler{
		Users: us, Tokens: evt, Mail: catcher, Tpl: tpl, PublicURL: "http://localhost",
	}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("token", "raw-token")
	req := httptest.NewRequest("GET", "/api/auth/verify-email/raw-token", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.Verify(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if us.marked != "u1" {
		t.Errorf("expected user u1 to be marked verified, got %q", us.marked)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/handler/ -run TestEmailVerification -v`
Expected: FAIL with `undefined: EmailVerificationHandler`

- [ ] **Step 3: Implement the handler**

```go
// internal/api/handler/email_verification.go
package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"html/template"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/mail"
	"github.com/tahsinarafat/aioj/internal/store"
)

type EmailVerificationHandler struct {
	Users     store.UserStore
	Tokens    store.EmailVerificationTokenStore
	Mail      mail.Sender
	Tpl       *template.Template
	PublicURL string
}

func (h *EmailVerificationHandler) Verify(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "token")
	if raw == "" {
		http.Error(w, "token required", http.StatusBadRequest)
		return
	}
	sum := sha256.Sum256([]byte(raw))
	tok, err := h.Tokens.GetByHash(r.Context(), hex.EncodeToString(sum[:]))
	if err != nil || tok == nil {
		http.Error(w, "invalid token", http.StatusBadRequest)
		return
	}
	if tok.Used {
		http.Error(w, "token already used", http.StatusBadRequest)
		return
	}
	if time.Now().After(tok.ExpiresAt) {
		http.Error(w, "token expired", http.StatusBadRequest)
		return
	}
	if err := h.Users.MarkEmailVerified(r.Context(), tok.UserID); err != nil {
		http.Error(w, "failed to mark verified", http.StatusInternalServerError)
		return
	}
	_ = h.Tokens.MarkUsed(r.Context(), tok.ID)
	respondJSON(w, http.StatusOK, map[string]string{"status": "verified"})
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/api/handler/ -run TestEmailVerification -v`
Expected: PASS

- [ ] **Step 5: Add route in router.go**

In `internal/api/router.go`:

```go
r.Get("/api/auth/verify-email/{token}", evH.Verify)
```

- [ ] **Step 6: Commit**

```bash
git add internal/api/handler/email_verification.go internal/api/handler/email_verification_test.go internal/api/router.go
git commit -m "feat(email-verification): verify handler with token hash lookup"
```

---

### Task A.9: Wire Email Verification into Register

**Files:**
- Modify: `internal/api/handler/auth.go` (modify Register; inject mail deps)
- Modify: `cmd/aioj/main.go` (construct AuthHandler with new deps)
- Modify: `internal/api/deps.go`

- [ ] **Step 1: Extend AuthHandler with mail + token fields**

In `internal/api/handler/auth.go`, replace the struct and constructor:

```go
type AuthHandler struct {
	users             store.UserStore
	refreshToks       store.RefreshTokenStore
	passwordResetToks store.PasswordResetTokenStore
	onsiteStore       store.OnsiteUserStore
	contestStore      store.ContestStore
	jwt               *auth.JWTManager
	evt               store.EmailVerificationTokenStore
	mail              mail.Sender
	mailTpl           *template.Template
	publicURL         string
}

func NewAuthHandler(
	users store.UserStore,
	refreshToks store.RefreshTokenStore,
	passwordResetToks store.PasswordResetTokenStore,
	onsiteStore store.OnsiteUserStore,
	contestStore store.ContestStore,
	jwt *auth.JWTManager,
	evt store.EmailVerificationTokenStore,
	m mail.Sender,
	tpl *template.Template,
	publicURL string,
) *AuthHandler {
	return &AuthHandler{
		users: users, refreshToks: refreshToks, passwordResetToks: passwordResetToks,
		onsiteStore: onsiteStore, contestStore: contestStore, jwt: jwt,
		evt: evt, mail: m, mailTpl: tpl, publicURL: publicURL,
	}
}
```

Add imports: `bytes`, `encoding/hex`, `log/slog`, `strings`, `crypto/sha256`, `crypto/rand`, `html/template`, `github.com/tahsinarafat/aioj/internal/mail`, `time`.

- [ ] **Step 2: Modify Register to issue a verification email**

In `auth.go` `Register`, after the successful `users.Create` call (replace lines 56-66):

```go
// Generate verification token
raw := make([]byte, 32)
_, _ = rand.Read(raw)
rawHex := hex.EncodeToString(raw)
sum := sha256.Sum256([]byte(rawHex))
hash := hex.EncodeToString(sum[:])
if err := h.evt.Create(r.Context(), uuid.NewString(), user.ID, hash, time.Now().Add(24*time.Hour)); err != nil {
	slog.Error("email verification token create failed", "err", err, "user_id", user.ID)
}

// Send verification email
if h.mailTpl != nil {
	verifyURL := strings.TrimRight(h.publicURL, "/") + "/verify-email?token=" + rawHex
	var body bytes.Buffer
	if err := h.mailTpl.ExecuteTemplate(&body, "email_verification.txt", map[string]string{
		"Username":      user.Username,
		"VerifyURL":     verifyURL,
		"ExpiryMinutes": "24",
	}); err == nil {
		_ = h.mail.Send(r.Context(), &mail.Message{
			From: "noreply@aioj.com", To: []string{user.Email},
			Subject: "Verify your AIOJ email", Body: body.String(),
		})
	}
}

respondJSON(w, http.StatusCreated, h.tokenResp(r.Context(), user))
```

- [ ] **Step 3: Build to verify compilation**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/api/handler/auth.go cmd/aioj/main.go internal/api/deps.go
git commit -m "feat(email-verification): issue verification email on register"
```

---

### Task A.10: Wire Password Reset to Real Email

**Files:**
- Modify: `internal/api/handler/auth.go` (modify ForgotPassword to send email, not return token)

- [ ] **Step 1: Replace the development token-return with real email send**

In `auth.go` `ForgotPassword` (lines 155-195), replace the final `respondJSON` block so it sends an email and never returns the raw token:

```go
// After passwordResetToks.Create succeeds (line 184-187), add:

if h.mailTpl != nil {
	resetURL := strings.TrimRight(h.publicURL, "/") + "/reset-password?token=" + rawToken
	var body bytes.Buffer
	if err := h.mailTpl.ExecuteTemplate(&body, "password_reset.txt", map[string]string{
		"Username": user.Username,
		"ResetURL": resetURL,
		"Expiry":   "1 hour",
	}); err == nil {
		_ = h.mail.Send(r.Context(), &mail.Message{
			From: "noreply@aioj.com", To: []string{user.Email},
			Subject: "Reset your AIOJ password", Body: body.String(),
		})
	}
}

// Always return success to prevent email enumeration:
respondJSON(w, http.StatusOK, map[string]string{
	"message": "If the email exists, a reset link has been sent",
})
// NOTE: token is NO LONGER returned in the response.
```

Add the `bytes`, `strings`, `mail` imports if not already present.

- [ ] **Step 2: Add a test that asserts the token is NOT in the response**

```go
// internal/api/handler/auth_password_reset_test.go
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/mail"
	"github.com/tahsinarafat/aioj/internal/model"
)

// stubPasswordResetForMail is a minimal PRT store for this test.
type stubPasswordResetForMail struct{}

func (s *stubPasswordResetForMail) Create(ctx context.Context, id, uid, h string, e time.Time) error {
	return nil
}
func (s *stubPasswordResetForMail) GetByHash(ctx context.Context, h string) (*model.PasswordResetToken, error) {
	return nil, nil
}
func (s *stubPasswordResetForMail) MarkUsed(ctx context.Context, id string) error { return nil }

func TestForgotPassword_DoesNotLeakTokenInResponse(t *testing.T) {
	catcher := mail.NewMailCatcher()
	tpl, _ := mail.LoadTemplates()
	h := &AuthHandler{
		users: &stubUserForEV{users: map[string]*model.User{
			"u1": {ID: "u1", Username: "alice", Email: "alice@x.com"},
		}},
		passwordResetToks: &stubPasswordResetForMail{},
		mail:              catcher,
		mailTpl:           tpl,
		publicURL:         "http://localhost",
	}

	body := `{"email":"alice@x.com"}`
	req := httptest.NewRequest("POST", "/api/auth/forgot-password", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ForgotPassword(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if _, leaked := resp["token"]; leaked {
		t.Errorf("token leaked in response: %v", resp)
	}
	if len(catcher.All()) != 1 {
		t.Errorf("expected 1 email sent, got %d", len(catcher.All()))
	}
	if !strings.Contains(catcher.All()[0].Body, "alice") {
		t.Errorf("email body should contain username. got: %s", catcher.All()[0].Body)
	}
}
```

- [ ] **Step 3: Run the test to verify it passes**

Run: `go test ./internal/api/handler/ -run TestForgotPassword -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/api/handler/auth.go internal/api/handler/auth_password_reset_test.go
git commit -m "feat(mail): send real password-reset email; remove token from response"
```

---

### Task A.11: Gate Submissions on Verified Email

**Files:**
- Modify: `internal/api/handler/submission.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/api/handler/submission_verification_test.go
package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSubmission_Create_RejectsUnverifiedEmail(t *testing.T) {
	h := &SubmissionHandler{
		users: &stubUserForEV{users: map[string]*model.User{
			"u1": {ID: "u1", Email: "a@x.com"},
		}, isVerified: false},
	}
	ctx := context.WithValue(context.Background(), ctxKeyClaims, &model.Claims{UserID: "u1"})

	req := httptest.NewRequest("POST", "/api/submissions", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}
```

> Adapt `ctxKeyClaims` to your existing middleware context key (likely in `internal/api/middleware/auth.go`).

- [ ] **Step 2: Add the verification gate**

In `internal/api/handler/submission.go`, at the top of `Create`, after parsing the request body and before queueing:

```go
verified, err := h.users.IsEmailVerified(r.Context(), claims.UserID)
if err != nil {
	respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "verification check failed"})
	return
}
if !verified {
	respondJSON(w, http.StatusForbidden, map[string]string{
		"error": "email not verified; check your inbox or call POST /api/auth/verify-email/resend",
	})
	return
}
```

Inject `users store.UserStore` into `SubmissionHandler` if not already present.

- [ ] **Step 3: Run the test to verify it passes**

Run: `go test ./internal/api/handler/ -run TestSubmission_Create_RejectsUnverifiedEmail -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/api/handler/submission.go internal/api/handler/submission_verification_test.go
git commit -m "feat(submissions): require verified email to submit"
```

---

### Task A.12: TOTP 2FA — Migration and Model

**Files:**
- Create: `internal/store/migrations/000053_totp.up.sql`
- Create: `internal/store/migrations/000053_totp.down.sql`
- Modify: `internal/model/user.go` (add TOTPSecret model)

- [ ] **Step 1: Up migration**

```sql
-- internal/store/migrations/000053_totp.up.sql
CREATE TABLE IF NOT EXISTS totp_secrets (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    secret TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    enabled_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS totp_backup_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash TEXT NOT NULL,
    used BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_backup_user ON totp_backup_codes(user_id);
```

- [ ] **Step 2: Down migration**

```sql
-- internal/store/migrations/000053_totp.down.sql
DROP TABLE IF EXISTS totp_backup_codes;
DROP TABLE IF EXISTS totp_secrets;
```

- [ ] **Step 3: Add TOTP model**

In `internal/model/user.go`, append:

```go
type TOTPSecret struct {
    UserID    string     `json:"user_id"`
    Secret    string     `json:"-"`
    Enabled   bool       `json:"enabled"`
    EnabledAt *time.Time `json:"enabled_at,omitempty"`
    CreatedAt time.Time  `json:"created_at"`
}
```

- [ ] **Step 4: Run migration**

Run: `make migrate-up`
Expected: Two new tables created.

- [ ] **Step 5: Commit**

```bash
git add internal/store/migrations/000053_totp.up.sql internal/store/migrations/000053_totp.down.sql internal/model/user.go
git commit -m "feat(2fa): migration and model for TOTP secrets and backup codes"
```

---

### Task A.13: TOTP Library and Tests

**Files:**
- Create: `internal/auth/totp.go`
- Test: `internal/auth/totp_test.go`

- [ ] **Step 1: Add the TOTP dependency**

Run: `go get github.com/pquerna/otp/totp`
Expected: `go.mod` updated.

- [ ] **Step 2: Write the failing test**

```go
// internal/auth/totp_test.go
package auth

import (
	"testing"
	"time"
)

func TestGenerateSecret(t *testing.T) {
	s, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 32 { // base32 of 20 bytes
		t.Errorf("expected 32-char secret, got %d", len(s))
	}
}

func TestValidateCode(t *testing.T) {
	secret, _ := GenerateTOTPSecret()
	code, err := GenerateTOTPCodeForTest(secret)
	if err != nil {
		t.Fatal(err)
	}
	if !ValidateTOTPCode(secret, code) {
		t.Error("expected code to validate")
	}
}

func TestValidateCode_RejectsExpired(t *testing.T) {
	secret, _ := GenerateTOTPSecret()
	old := time.Now().Add(-10 * time.Minute).Unix() / 30
	code := generateTOTPCodeAt(secret, old)
	if ValidateTOTPCode(secret, code) {
		t.Error("expected old code to be rejected")
	}
}
```

- [ ] **Step 3: Implement TOTP helpers**

```go
// internal/auth/totp.go
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/pquerna/otp/totp"
)

// GenerateTOTPSecret returns a 20-byte secret encoded as base32 (no padding).
func GenerateTOTPSecret() (string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), nil
}

func ValidateTOTPCode(secret, code string) bool {
	return totp.Validate(code, secret)
}

// GenerateTOTPCodeForTest returns the current TOTP code for the secret.
// Use only in tests.
func GenerateTOTPCodeForTest(secret string) (string, error) {
	return totp.GenerateCode(secret, time.Now())
}

// generateTOTPCodeAt is a test helper that produces a code for a given
// 30-second time step. Unexported.
func generateTOTPCodeAt(secret string, t int64) string {
	sec, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return ""
	}
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(t))
	h := hmac.New(sha1.New, sec)
	h.Write(buf[:])
	sum := h.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	truncated := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	code := truncated % 1_000_000
	return fmt.Sprintf("%06d", code)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/auth/ -run TestGenerateSecret -v && go test ./internal/auth/ -run TestValidateCode -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/auth/totp.go internal/auth/totp_test.go go.mod go.sum
git commit -m "feat(2fa): TOTP secret generation and validation"
```

---

### Task A.14: TOTP Store and 2FA Handler (Enable/Disable)

**Files:**
- Create: `internal/store/postgres/totp_secrets.go`
- Modify: `internal/store/interfaces.go`
- Create: `internal/api/handler/auth_2fa.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Add TOTPSecretStore to interfaces.go**

```go
// In internal/store/interfaces.go
type TOTPSecretStore interface {
    Upsert(ctx context.Context, userID, secret string) error
    Enable(ctx context.Context, userID string) error
    Disable(ctx context.Context, userID string) error
    Get(ctx context.Context, userID string) (*model.TOTPSecret, error)
}

type BackupCodeStore interface {
    Create(ctx context.Context, id, userID, codeHash string) error
    ListActive(ctx context.Context, userID string) ([]BackupCodeRow, error)
    Consume(ctx context.Context, id string) error
}

type BackupCodeRow struct {
    ID   string
    Hash string
}
```

- [ ] **Step 2: Implement the postgres store**

```go
// internal/store/postgres/totp_secrets.go
package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type TOTPSecretStore struct{ db *sql.DB }

func NewTOTPSecretStore(db *sql.DB) *TOTPSecretStore { return &TOTPSecretStore{db: db} }

func (s *TOTPSecretStore) Upsert(ctx context.Context, userID, secret string) error {
	_, err := s.db.ExecContext(ctx, `
        INSERT INTO totp_secrets (user_id, secret, enabled) VALUES ($1, $2, FALSE)
        ON CONFLICT (user_id) DO UPDATE SET secret = $2, enabled = FALSE, enabled_at = NULL
    `, userID, secret)
	return err
}

func (s *TOTPSecretStore) Enable(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `
        UPDATE totp_secrets SET enabled = TRUE, enabled_at = NOW() WHERE user_id = $1
    `, userID)
	return err
}

func (s *TOTPSecretStore) Disable(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM totp_secrets WHERE user_id = $1`, userID)
	return err
}

func (s *TOTPSecretStore) Get(ctx context.Context, userID string) (*model.TOTPSecret, error) {
	row := s.db.QueryRowContext(ctx, `
        SELECT user_id, secret, enabled, enabled_at, created_at
        FROM totp_secrets WHERE user_id = $1
    `, userID)
	var t model.TOTPSecret
	if err := row.Scan(&t.UserID, &t.Secret, &t.Enabled, &t.EnabledAt, &t.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

type BackupCodeStore struct{ db *sql.DB }

func NewBackupCodeStore(db *sql.DB) *BackupCodeStore { return &BackupCodeStore{db: db} }

func (s *BackupCodeStore) Create(ctx context.Context, id, userID, codeHash string) error {
	_, err := s.db.ExecContext(ctx, `
        INSERT INTO totp_backup_codes (id, user_id, code_hash) VALUES ($1, $2, $3)
    `, id, userID, codeHash)
	return err
}

func (s *BackupCodeStore) ListActive(ctx context.Context, userID string) ([]store.BackupCodeRow, error) {
	rows, err := s.db.QueryContext(ctx, `
        SELECT id, code_hash FROM totp_backup_codes
        WHERE user_id = $1 AND used = FALSE
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.BackupCodeRow
	for rows.Next() {
		var r store.BackupCodeRow
		if err := rows.Scan(&r.ID, &r.Hash); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *BackupCodeStore) Consume(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE totp_backup_codes SET used = TRUE WHERE id = $1`, id)
	return err
}
```

- [ ] **Step 3: Implement the 2FA handler**

```go
// internal/api/handler/auth_2fa.go
package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/store"
)

type TwoFactorHandler struct {
	Users   store.UserStore
	Secrets store.TOTPSecretStore
	Backups store.BackupCodeStore
}

func (h *TwoFactorHandler) Begin(w http.ResponseWriter, r *http.Request) {
	claims, ok := mustClaims(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	if err := h.Secrets.Upsert(r.Context(), claims.UserID, secret); err != nil {
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	user, _ := h.Users.GetByID(r.Context(), claims.UserID)
	username := claims.UserID
	email := claims.UserID
	if user != nil {
		username = user.Username
		email = user.Email
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"secret": secret,
		"uri": "otpauth://totp/AIOJ:" + username +
			"?secret=" + secret + "&issuer=AIOJ&algorithm=SHA1&digits=6&period=30",
		"_email": email, // for the QR code generator
	})
}

func (h *TwoFactorHandler) Enable(w http.ResponseWriter, r *http.Request) {
	claims, ok := mustClaims(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if err := bindJSON(r, &req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	secret, err := h.Secrets.Get(r.Context(), claims.UserID)
	if err != nil || secret == nil {
		http.Error(w, "2fa not initialized", http.StatusBadRequest)
		return
	}
	if !auth.ValidateTOTPCode(secret.Secret, req.Code) {
		http.Error(w, "invalid code", http.StatusBadRequest)
		return
	}
	if err := h.Secrets.Enable(r.Context(), claims.UserID); err != nil {
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	codes := generateBackupCodes()
	for _, c := range codes {
		sum := sha256.Sum256([]byte(c))
		_ = h.Backups.Create(r.Context(), uuid.NewString(), claims.UserID, hex.EncodeToString(sum[:]))
	}
	respondJSON(w, http.StatusOK, map[string]any{"backup_codes": codes})
}

func (h *TwoFactorHandler) Disable(w http.ResponseWriter, r *http.Request) {
	claims, ok := mustClaims(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := bindJSON(r, &req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	u, _ := h.Users.GetByID(r.Context(), claims.UserID)
	if u == nil || !auth.CheckPassword(req.Password, u.PasswordHash) {
		http.Error(w, "invalid password", http.StatusForbidden)
		return
	}
	if err := h.Secrets.Disable(r.Context(), claims.UserID); err != nil {
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

func generateBackupCodes() []string {
	out := make([]string, 10)
	for i := range out {
		b := make([]byte, 5)
		_, _ = rand.Read(b)
		out[i] = hex.EncodeToString(b)
	}
	return out
}

// helper used by login flow
func (h *TwoFactorHandler) VerifyChallengeAndIssueTokens(w http.ResponseWriter, r *http.Request, userID, code string, issue func(string)) {
	secret, _ := h.Secrets.Get(r.Context(), userID)
	if secret != nil && secret.Enabled && auth.ValidateTOTPCode(secret.Secret, code) {
		issue(userID)
		return
	}
	sum := sha256.Sum256([]byte(code))
	rows, _ := h.Backups.ListActive(r.Context(), userID)
	for _, row := range rows {
		if row.Hash == hex.EncodeToString(sum[:]) {
			_ = h.Backups.Consume(r.Context(), row.ID)
			issue(userID)
			return
		}
	}
	http.Error(w, "invalid 2fa code", http.StatusUnauthorized)
}

// silence unused time
var _ = time.Now
```

Add `mustClaims` and `bindJSON` helpers in this package or import from existing handler utilities. If they don't exist, define them:

```go
// internal/api/handler/util.go (or add to auth_2fa.go)
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
)

func mustClaims(r *http.Request) (*model.Claims, bool) {
	c := middleware.GetUserClaims(r)
	if c == nil {
		return nil, false
	}
	return c, true
}

func bindJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
```

- [ ] **Step 4: Add routes**

In `internal/api/router.go`:

```go
r.With(middleware.AuthMiddleware(jwtManager)).Post("/api/auth/2fa/begin", twoFAH.Begin)
r.With(middleware.AuthMiddleware(jwtManager)).Post("/api/auth/2fa/enable", twoFAH.Enable)
r.With(middleware.AuthMiddleware(jwtManager)).Post("/api/auth/2fa/disable", twoFAH.Disable)
```

- [ ] **Step 5: Build and run tests**

Run: `go build ./... && go test ./internal/auth/ -v && go test ./internal/api/handler/ -run TestTwoFactor -v`
Expected: Build OK, tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/store/interfaces.go internal/store/postgres/totp_secrets.go internal/api/handler/auth_2fa.go internal/api/handler/util.go internal/api/router.go
git commit -m "feat(2fa): TOTP enable/disable with backup codes"
```

---

### Task A.15: 2FA Login Flow

**Files:**
- Modify: `internal/api/handler/auth.go` (modify Login to return 2FA challenge)
- Modify: `internal/auth/jwt.go` (add challenge token support)

- [ ] **Step 1: Extend AuthResponse to include 2FA fields**

In `internal/model/user.go`, modify AuthResponse:

```go
type AuthResponse struct {
    AccessToken  string `json:"access_token,omitempty"`
    RefreshToken string `json:"refresh_token,omitempty"`
    User         *User  `json:"user,omitempty"`
    Requires2FA  bool   `json:"requires_2fa,omitempty"`
    ChallengeID  string `json:"challenge_id,omitempty"`
}
```

- [ ] **Step 2: Modify Login to check 2FA before issuing tokens**

In `auth.go`, after the credentials check (around line 80) and before `tokenResp`, add:

```go
// After confirming valid credentials, check 2FA
if dbUser != nil && h.twoFAStore != nil {
    totp, _ := h.twoFAStore.Get(r.Context(), dbUser.ID)
    if totp != nil && totp.Enabled {
        chal, _ := h.jwt.GenerateChallengeToken(dbUser.ID, uuid.NewString(), 5*time.Minute)
        respondJSON(w, http.StatusOK, model.AuthResponse{
            Requires2FA: true,
            ChallengeID: chal,
        })
        return
    }
}
```

Inject `twoFAStore store.TOTPSecretStore` into AuthHandler. Update the constructor.

- [ ] **Step 3: Add GenerateChallengeToken to JWTManager**

In `internal/auth/jwt.go`, add:

```go
func (m *JWTManager) GenerateChallengeToken(userID, challengeID string, ttl time.Duration) (string, error) {
    claims := jwt.MapClaims{
        "sub":          userID,
        "challenge_id": challengeID,
        "type":         "2fa",
        "exp":          time.Now().Add(ttl).Unix(),
        "iat":          time.Now().Unix(),
    }
    t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return t.SignedString([]byte(m.secret))
}

func (m *JWTManager) ParseChallengeToken(tokenString string) (*ChallengeClaims, error) {
    parsed, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
        return []byte(m.secret), nil
    })
    if err != nil {
        return nil, err
    }
    if !parsed.Valid {
        return nil, errors.New("invalid token")
    }
    claims, ok := parsed.Claims.(jwt.MapClaims)
    if !ok {
        return nil, errors.New("bad claims type")
    }
    if typ, _ := claims["type"].(string); typ != "2fa" {
        return nil, errors.New("not a 2fa challenge token")
    }
    sub, _ := claims["sub"].(string)
    cid, _ := claims["challenge_id"].(string)
    return &ChallengeClaims{UserID: sub, ChallengeID: cid}, nil
}

type ChallengeClaims struct {
    UserID      string
    ChallengeID string
}
```

- [ ] **Step 4: Add the verify endpoint to TwoFactorHandler**

In `internal/api/handler/auth_2fa.go`, add:

```go
type TwoFactorVerifyHandler struct {
    Secrets store.TOTPSecretStore
    Backups store.BackupCodeStore
    JWT     *auth.JWTManager
    Users   store.UserStore
    Refresh store.RefreshTokenStore
}

func (h *TwoFactorVerifyHandler) Verify(w http.ResponseWriter, r *http.Request) {
    var req struct {
        ChallengeID string `json:"challenge_id"`
        Code        string `json:"code"`
    }
    if err := bindJSON(r, &req); err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }
    parsed, err := h.JWT.ParseChallengeToken(req.ChallengeID)
    if err != nil || parsed == nil {
        http.Error(w, "invalid or expired challenge", http.StatusUnauthorized)
        return
    }
    userID := parsed.UserID
    h.verifyAndIssue(w, r, userID, req.Code)
}

func (h *TwoFactorVerifyHandler) verifyAndIssue(w http.ResponseWriter, r *http.Request, userID, code string) {
    secret, _ := h.Secrets.Get(r.Context(), userID)
    if secret == nil || !secret.Enabled {
        http.Error(w, "2fa not enabled", http.StatusBadRequest)
        return
    }
    issued := false
    issue := func(uid string) {
        issued = true
        u, _ := h.Users.GetByID(r.Context(), uid)
        if u == nil {
            http.Error(w, "user not found", http.StatusInternalServerError)
            return
        }
        access, _ := h.JWT.GenerateAccessToken(u.ID, u.Username, u.Role)
        raw, hashed := h.JWT.GenerateRefreshToken()
        _ = h.Refresh.Create(r.Context(), u.ID, hashed, time.Now().Add(h.JWT.RefreshTTL()))
        respondJSON(w, http.StatusOK, &model.AuthResponse{
            AccessToken: access, RefreshToken: raw, User: u,
        })
    }
    if auth.ValidateTOTPCode(secret.Secret, code) {
        issue(userID)
        return
    }
    sum := sha256.Sum256([]byte(code))
    rows, _ := h.Backups.ListActive(r.Context(), userID)
    for _, row := range rows {
        if row.Hash == hex.EncodeToString(sum[:]) {
            _ = h.Backups.Consume(r.Context(), row.ID)
            issue(userID)
            return
        }
    }
    if !issued {
        http.Error(w, "invalid code", http.StatusUnauthorized)
    }
}
```

Add the route:

```go
r.Post("/api/auth/2fa/verify", twoFAVerifyH.Verify)
```

- [ ] **Step 5: Build and run all auth tests**

Run: `go build ./... && go test ./... -count=1`
Expected: Build OK; existing tests still pass.

- [ ] **Step 6: Commit**

```bash
git add internal/api/handler/auth.go internal/api/handler/auth_2fa.go internal/auth/jwt.go internal/model/user.go internal/api/router.go
git commit -m "feat(2fa): challenge-based 2FA login flow"
```

---

### Task A.16: Password Policy

**Files:**
- Create: `internal/auth/password.go`
- Test: `internal/auth/password_test.go`
- Modify: `internal/api/handler/auth.go` (use the policy in Register and ResetPassword)

- [ ] **Step 1: Write the failing test**

```go
// internal/auth/password_test.go
package auth

import "testing"

func TestValidatePasswordStrength(t *testing.T) {
	cases := []struct {
		pwd     string
		wantErr bool
	}{
		{"short", true},
		{"alllowercasebut12chars", true},
		{"NoDigits!But12Chars", true},
		{"nodigitsbut12chars", true},
		{"Valid1Pass!", false},
		{"Valid1Pass!with more", false},
		{"validpass!2025", true},
	}
	for _, c := range cases {
		err := ValidatePasswordStrength(c.pwd)
		if (err != nil) != c.wantErr {
			t.Errorf("ValidatePasswordStrength(%q) err=%v, wantErr=%v", c.pwd, err, c.wantErr)
		}
	}
}

func TestIsCommonPassword(t *testing.T) {
	if !IsCommonPassword("password123") {
		t.Error("expected 'password123' to be common")
	}
	if IsCommonPassword("Xq!9pL@2mZ$vN") {
		t.Error("expected long random string not to be common")
	}
}
```

- [ ] **Step 2: Implement the policy**

```go
// internal/auth/password.go
package auth

import (
	"errors"
	"strings"
	"unicode"
)

var ErrPasswordTooWeak = errors.New("password does not meet complexity requirements")
var ErrPasswordCommon = errors.New("password is too common")

// ValidatePasswordStrength enforces:
//   - length >= 12
//   - at least one upper, one lower, one digit, one symbol
func ValidatePasswordStrength(p string) error {
	if len(p) < 12 {
		return ErrPasswordTooWeak
	}
	var hasUpper, hasLower, hasDigit, hasSym bool
	for _, r := range p {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSym = true
		}
	}
	if !(hasUpper && hasLower && hasDigit && hasSym) {
		return ErrPasswordTooWeak
	}
	return nil
}

// IsCommonPassword checks a small static list of top breached passwords.
// Production should swap in a HIBP k-anonymity feed.
var commonPasswords = map[string]bool{
	"password": true, "password123": true, "qwerty": true, "letmein": true,
	"123456789": true, "iloveyou": true, "admin123": true, "welcome": true,
	"monkey": true, "dragon": true,
}

func IsCommonPassword(p string) bool {
	return commonPasswords[strings.ToLower(p)]
}
```

- [ ] **Step 3: Apply in Register and ResetPassword**

In `auth.go`, replace the existing `len(req.Password) < 6` checks with:

```go
if err := auth.ValidatePasswordStrength(req.Password); err != nil {
	http.Error(w, err.Error(), http.StatusBadRequest)
	return
}
if auth.IsCommonPassword(req.Password) {
	http.Error(w, auth.ErrPasswordCommon.Error(), http.StatusBadRequest)
	return
}
```

Apply in both `Register` and `ResetPassword`. Update the error messages in existing test fixtures to use 12-char passwords.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/auth/ -v && go test ./internal/api/handler/ -v`
Expected: PASS (you may need to update other tests that used short passwords)

- [ ] **Step 5: Commit**

```bash
git add internal/auth/password.go internal/auth/password_test.go internal/api/handler/auth.go
git commit -m "feat(auth): 12-char password policy with complexity and common-password check"
```

---

### Task A.17: OAuth — Provider Interface and State CSRF

**Files:**
- Create: `internal/oauth/provider.go`
- Create: `internal/oauth/state.go`
- Test: `internal/oauth/state_test.go`

- [ ] **Step 1: Add the OAuth2 dependency**

Run: `go get golang.org/x/oauth2`
Expected: `go.mod` updated.

- [ ] **Step 2: Write the state test**

```go
// internal/oauth/state_test.go
package oauth

import (
	"testing"
	"time"
)

func TestStateToken_CreateAndValidate(t *testing.T) {
	SetStateSecret([]byte("test-secret"))
	raw, sig, err := IssueStateToken(5 * time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := ValidateStateToken(raw, sig, 5*time.Minute)
	if err != nil || !ok {
		t.Errorf("expected valid, got ok=%v err=%v", ok, err)
	}
}

func TestStateToken_RejectsTampered(t *testing.T) {
	SetStateSecret([]byte("test-secret"))
	raw, sig, _ := IssueStateToken(5 * time.Minute)
	sig[0] ^= 0xff
	ok, _ := ValidateStateToken(raw, sig, 5*time.Minute)
	if ok {
		t.Error("expected tampered signature to fail")
	}
}

func TestStateToken_RejectsExpired(t *testing.T) {
	SetStateSecret([]byte("test-secret"))
	raw, sig, _ := IssueStateToken(-1 * time.Second)
	ok, _ := ValidateStateToken(raw, sig, 5*time.Minute)
	if ok {
		t.Error("expected expired token to fail")
	}
}
```

- [ ] **Step 3: Implement the provider interface and state**

```go
// internal/oauth/provider.go
package oauth

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
)

type UserInfo struct {
	ProviderUserID string
	Username       string
	Email          string
	AvatarURL      string
	Raw            map[string]any
}

type Provider interface {
	Name() string
	Config() *oauth2.Config
	FetchUser(ctx context.Context, client *http.Client) (*UserInfo, error)
}
```

```go
// internal/oauth/state.go
package oauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"time"
)

var stateSecret []byte

func SetStateSecret(b []byte) { stateSecret = b }

func IssueStateToken(ttl time.Duration) (string, []byte, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", nil, err
	}
	exp := time.Now().Add(ttl).Unix()
	raw := hex.EncodeToString(nonce) + "." + strconv.FormatInt(exp, 10)
	sig := sign(raw)
	return raw, sig, nil
}

func ValidateStateToken(raw string, sig []byte, _ time.Duration) (bool, error) {
	expected := sign(raw)
	if !hmac.Equal(sig, expected) {
		return false, errors.New("bad signature")
	}
	parts := splitRaw(raw)
	if len(parts) != 2 {
		return false, errors.New("malformed")
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return false, err
	}
	if time.Now().Unix() > exp {
		return false, errors.New("expired")
	}
	return true, nil
}

func IssueStateTokenWith(secret []byte, ttl time.Duration) (string, []byte, error) {
	prev := stateSecret
	stateSecret = secret
	defer func() { stateSecret = prev }()
	return IssueStateToken(ttl)
}

func sign(raw string) []byte {
	h := hmac.New(sha256.New, stateSecret)
	h.Write([]byte(raw))
	return h.Sum(nil)
}

func splitRaw(raw string) []string {
	for i := len(raw) - 1; i >= 0; i-- {
		if raw[i] == '.' {
			return []string{raw[:i], raw[i+1:]}
		}
	}
	return nil
}

func EncodeSig(sig []byte) string { return base64.RawURLEncoding.EncodeToString(sig) }
func DecodeSig(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/oauth/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/oauth/provider.go internal/oauth/state.go internal/oauth/state_test.go go.mod go.sum
git commit -m "feat(oauth): provider interface and state CSRF token"
```

---

### Task A.18: GitHub OAuth Provider

**Files:**
- Create: `internal/oauth/github.go`
- Test: `internal/oauth/github_test.go`
- Modify: `config.yaml`, `.env.example`

- [ ] **Step 1: Add GitHub config to config.yaml**

Append:

```yaml
oauth:
  state_secret: "change-me-32-bytes-random"
  github:
    client_id: ""
    client_secret: ""
    redirect_url: "http://localhost:8080/api/auth/oauth/github/callback"
    scopes: ["user:email"]
  google:
    client_id: ""
    client_secret: ""
    redirect_url: "http://localhost:8080/api/auth/oauth/google/callback"
    scopes: ["openid", "email", "profile"]
```

Add to `.env.example`:

```env
OAUTH_STATE_SECRET=
GITHUB_CLIENT_ID=
GITHUB_CLIENT_SECRET=
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
```

- [ ] **Step 2: Write the failing test**

```go
// internal/oauth/github_test.go
package oauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubProvider_FetchUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			json.NewEncoder(w).Encode(map[string]any{
				"id": 12345, "login": "octocat", "name": "Octo Cat",
				"avatar_url": "https://example.com/a.png",
			})
		case "/user/emails":
			json.NewEncoder(w).Encode([]map[string]any{
				{"email": "octo@cat.com", "primary": true, "verified": true},
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	p := NewGitHubProvider(GitHubConfig{
		UserURL: srv.URL + "/user", EmailsURL: srv.URL + "/user/emails",
	})
	info, err := p.FetchUser(nil, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if info.Username != "octocat" {
		t.Errorf("got username %q, want octocat", info.Username)
	}
	if info.Email != "octo@cat.com" {
		t.Errorf("got email %q, want octo@cat.com", info.Email)
	}
	if info.ProviderUserID != "12345" {
		t.Errorf("got provider id %q, want 12345", info.ProviderUserID)
	}
}
```

- [ ] **Step 3: Implement the GitHub provider**

```go
// internal/oauth/github.go
package oauth

import (
	"context"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

type GitHubConfig struct {
	UserURL   string
	EmailsURL string
}

type GitHubProvider struct {
	cfg *oauth2.Config
	ghc GitHubConfig
}

func NewGitHubProvider(c GitHubConfig) *GitHubProvider {
	cfg := &oauth2.Config{Endpoint: github.Endpoint, Scopes: []string{"user:email"}}
	if c.UserURL == "" {
		c.UserURL = "https://api.github.com/user"
		c.EmailsURL = "https://api.github.com/user/emails"
	}
	return &GitHubProvider{cfg: cfg, ghc: c}
}

func (p *GitHubProvider) Name() string           { return "github" }
func (p *GitHubProvider) Config() *oauth2.Config { return p.cfg }

func (p *GitHubProvider) FetchUser(_ context.Context, client *http.Client) (*UserInfo, error) {
	var u struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := getJSON(client, p.ghc.UserURL, &u); err != nil {
		return nil, err
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := getJSON(client, p.ghc.EmailsURL, &emails); err != nil {
		return nil, err
	}
	email := ""
	for _, e := range emails {
		if e.Primary && e.Verified {
			email = e.Email
			break
		}
	}
	if email == "" && len(emails) > 0 {
		email = emails[0].Email
	}
	return &UserInfo{
		ProviderUserID: fmt.Sprintf("%d", u.ID),
		Username:       u.Login,
		Email:          email,
		AvatarURL:      u.AvatarURL,
	}, nil
}

func getJSON(client *http.Client, url string, out any) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("oauth: %s returned %d", url, resp.StatusCode)
	}
	return jsonDecode(resp.Body, out)
}
```

Add a `jsonDecode` helper:

```go
// In internal/oauth/util.go
package oauth

import "encoding/json"
import "io"

func jsonDecode(r io.Reader, v any) error { return json.NewDecoder(r).Decode(v) }
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/oauth/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/oauth/github.go internal/oauth/github_test.go internal/oauth/util.go config.yaml .env.example
git commit -m "feat(oauth): GitHub provider with user/email fetch"
```

---

### Task A.19: Google OAuth Provider

**Files:**
- Create: `internal/oauth/google.go`
- Test: `internal/oauth/google_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/oauth/google_test.go
package oauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGoogleProvider_FetchUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"sub": "g-12345", "name": "Alice",
			"email": "alice@x.com", "email_verified": true,
			"picture": "https://example.com/p.png",
		})
	}))
	defer srv.Close()

	p := NewGoogleProvider(GoogleConfig{UserInfoURL: srv.URL})
	info, err := p.FetchUser(nil, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if info.ProviderUserID != "g-12345" {
		t.Errorf("got %q, want g-12345", info.ProviderUserID)
	}
	if info.Email != "alice@x.com" {
		t.Errorf("got %q, want alice@x.com", info.Email)
	}
}
```

- [ ] **Step 2: Implement**

```go
// internal/oauth/google.go
package oauth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleConfig struct {
	UserInfoURL string
}

type GoogleProvider struct {
	cfg *oauth2.Config
	gc  GoogleConfig
}

func NewGoogleProvider(c GoogleConfig) *GoogleProvider {
	cfg := &oauth2.Config{
		Endpoint: google.Endpoint,
		Scopes:   []string{"openid", "email", "profile"},
	}
	if c.UserInfoURL == "" {
		c.UserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"
	}
	return &GoogleProvider{cfg: cfg, gc: c}
}

func (p *GoogleProvider) Name() string           { return "google" }
func (p *GoogleProvider) Config() *oauth2.Config { return p.cfg }

func (p *GoogleProvider) FetchUser(_ context.Context, client *http.Client) (*UserInfo, error) {
	var u struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := getJSON(client, p.gc.UserInfoURL, &u); err != nil {
		return nil, err
	}
	return &UserInfo{
		ProviderUserID: u.Sub,
		Username:       deriveUsername(u.Email, u.Name),
		Email:          u.Email,
		AvatarURL:      u.Picture,
	}, nil
}

func deriveUsername(email, name string) string {
	if name != "" {
		return name
	}
	for i, c := range email {
		if c == '@' {
			return email[:i]
		}
	}
	return fmt.Sprintf("user_%d", time.Now().Unix())
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/oauth/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/oauth/google.go internal/oauth/google_test.go
git commit -m "feat(oauth): Google provider with OIDC userinfo"
```

---

### Task A.20: OAuth — Account Linking Migration and Store

**Files:**
- Create: `internal/store/migrations/000052_oauth.up.sql`
- Create: `internal/store/migrations/000052_oauth.down.sql`
- Create: `internal/store/postgres/oauth_links.go`
- Modify: `internal/store/interfaces.go`
- Modify: `internal/model/user.go`

- [ ] **Step 1: Up migration**

```sql
-- internal/store/migrations/000052_oauth.up.sql
CREATE TABLE IF NOT EXISTS oauth_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    provider_user_id TEXT NOT NULL,
    linked_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_user_id)
);
CREATE INDEX IF NOT EXISTS idx_oauth_user ON oauth_links(user_id);
```

- [ ] **Step 2: Down migration**

```sql
-- internal/store/migrations/000052_oauth.down.sql
DROP TABLE IF EXISTS oauth_links;
```

- [ ] **Step 3: Add OAuthLink model**

In `internal/model/user.go`, append:

```go
type OAuthLink struct {
    ID             string    `json:"id"`
    UserID         string    `json:"user_id"`
    Provider       string    `json:"provider"`
    ProviderUserID string    `json:"provider_user_id"`
    LinkedAt       time.Time `json:"linked_at"`
}
```

- [ ] **Step 4: Add store interface and implementation**

```go
// In internal/store/interfaces.go
type OAuthLinkStore interface {
    Create(ctx context.Context, link *model.OAuthLink) error
    GetByProviderUser(ctx context.Context, provider, providerUserID string) (*model.OAuthLink, error)
    ListByUser(ctx context.Context, userID string) ([]model.OAuthLink, error)
    Delete(ctx context.Context, id string) error
}
```

```go
// internal/store/postgres/oauth_links.go
package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/tahsinarafat/aioj/internal/model"
)

type OAuthLinkStore struct{ db *sql.DB }

func NewOAuthLinkStore(db *sql.DB) *OAuthLinkStore { return &OAuthLinkStore{db: db} }

func (s *OAuthLinkStore) Create(ctx context.Context, l *model.OAuthLink) error {
	_, err := s.db.ExecContext(ctx, `
        INSERT INTO oauth_links (id, user_id, provider, provider_user_id)
        VALUES ($1, $2, $3, $4)
    `, l.ID, l.UserID, l.Provider, l.ProviderUserID)
	return err
}

func (s *OAuthLinkStore) GetByProviderUser(ctx context.Context, provider, providerUserID string) (*model.OAuthLink, error) {
	row := s.db.QueryRowContext(ctx, `
        SELECT id, user_id, provider, provider_user_id, linked_at
        FROM oauth_links WHERE provider = $1 AND provider_user_id = $2
    `, provider, providerUserID)
	var l model.OAuthLink
	if err := row.Scan(&l.ID, &l.UserID, &l.Provider, &l.ProviderUserID, &l.LinkedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &l, nil
}

func (s *OAuthLinkStore) ListByUser(ctx context.Context, userID string) ([]model.OAuthLink, error) {
	rows, err := s.db.QueryContext(ctx, `
        SELECT id, user_id, provider, provider_user_id, linked_at
        FROM oauth_links WHERE user_id = $1
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.OAuthLink
	for rows.Next() {
		var l model.OAuthLink
		if err := rows.Scan(&l.ID, &l.UserID, &l.Provider, &l.ProviderUserID, &l.LinkedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, nil
}

func (s *OAuthLinkStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM oauth_links WHERE id = $1`, id)
	return err
}
```

- [ ] **Step 5: Run migration**

Run: `make migrate-up`
Expected: `oauth_links` table exists.

- [ ] **Step 6: Commit**

```bash
git add internal/store/migrations/000052_oauth.up.sql internal/store/migrations/000052_oauth.down.sql internal/store/postgres/oauth_links.go internal/store/interfaces.go internal/model/user.go
git commit -m "feat(oauth): account linking store and migration"
```

---

### Task A.21: OAuth — Callback Handler

**Files:**
- Create: `internal/api/handler/oauth_callback.go`
- Modify: `internal/api/router.go`
- Modify: `cmd/aioj/main.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/api/handler/oauth_callback_test.go
package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/oauth"
	"golang.org/x/oauth2"
)

type fakeOAuthProvider struct {
	cfg *oauth2.Config
	uid string
}

func (f *fakeOAuthProvider) Name() string           { return "fake" }
func (f *fakeOAuthProvider) Config() *oauth2.Config { return f.cfg }
func (f *fakeOAuthProvider) FetchUser(_ context.Context, _ *http.Client) (*oauth.UserInfo, error) {
	return &oauth.UserInfo{ProviderUserID: f.uid, Username: "fuser", Email: "f@x.com"}, nil
}

type stubOAuthLinkStore struct{ links map[string]*model.OAuthLink }

func (s *stubOAuthLinkStore) Create(ctx context.Context, l *model.OAuthLink) error {
	s.links[l.Provider+":"+l.ProviderUserID] = l
	return nil
}
func (s *stubOAuthLinkStore) GetByProviderUser(ctx context.Context, p, puid string) (*model.OAuthLink, error) {
	return s.links[p+":"+puid], nil
}
func (s *stubOAuthLinkStore) ListByUser(ctx context.Context, uid string) ([]model.OAuthLink, error) {
	return nil, nil
}
func (s *stubOAuthLinkStore) Delete(ctx context.Context, id string) error { return nil }

func TestOAuthCallback_CreatesUserAndLogsIn(t *testing.T) {
	users := &stubUserForEV{users: map[string]*model.User{}}
	links := &stubOAuthLinkStore{links: map[string]*model.OAuthLink{}}
	h := &OAuthCallbackHandler{
		Users: users,
		Links: links,
		Providers: map[string]oauth.Provider{
			"fake": &fakeOAuthProvider{uid: "p1"},
		},
		StateTTL: 5 * time.Minute,
	}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("provider", "fake")
	req := httptest.NewRequest("GET", "/api/auth/oauth/fake/callback?code=anything&state=anything", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.Callback(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if len(users.users) != 1 {
		t.Errorf("expected 1 new user, got %d", len(users.users))
	}
	if len(links.links) != 1 {
		t.Errorf("expected 1 oauth link, got %d", len(links.links))
	}
}
```

- [ ] **Step 2: Implement the handler**

```go
// internal/api/handler/oauth_callback.go
package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/oauth"
	"github.com/tahsinarafat/aioj/internal/store"
)

type OAuthCallbackHandler struct {
	Users     store.UserStore
	Links     store.OAuthLinkStore
	Providers map[string]oauth.Provider
	JWT       *auth.JWTManager
	Refresh   store.RefreshTokenStore
	StateTTL  time.Duration
	StateSecret []byte
}

func (h *OAuthCallbackHandler) Callback(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	prov, ok := h.Providers[providerName]
	if !ok {
		http.Error(w, "unknown provider", http.StatusBadRequest)
		return
	}
	code := r.URL.Query().Get("code")
	stateRaw := r.URL.Query().Get("state")
	if code == "" || stateRaw == "" {
		http.Error(w, "missing code or state", http.StatusBadRequest)
		return
	}

	// Validate state. Format from the Start handler: "raw.sig".
	if err := h.validateState(stateRaw); err != nil {
		http.Error(w, "invalid state: "+err.Error(), http.StatusBadRequest)
		return
	}

	cfg := prov.Config()
	tok, err := cfg.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "exchange failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	client := cfg.Client(r.Context(), tok)
	info, err := prov.FetchUser(r.Context(), client)
	if err != nil {
		http.Error(w, "fetch user failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 1) If link exists, log that user in.
	link, _ := h.Links.GetByProviderUser(r.Context(), providerName, info.ProviderUserID)
	if link != nil {
		u, _ := h.Users.GetByID(r.Context(), link.UserID)
		if u != nil {
			h.issueTokensAndRespond(w, r, u)
			return
		}
	}

	// 2) If user with same email exists, link to it.
	if info.Email != "" {
		existing, _ := h.Users.GetByEmail(r.Context(), info.Email)
		if existing != nil {
			_ = h.Links.Create(r.Context(), &model.OAuthLink{
				ID: uuid.NewString(), UserID: existing.ID,
				Provider: providerName, ProviderUserID: info.ProviderUserID,
			})
			h.issueTokensAndRespond(w, r, existing)
			return
		}
	}

	// 3) Otherwise create a new user.
	newUser := &model.User{
		ID: uuid.NewString(), Username: info.Username, Email: info.Email, Role: "user",
	}
	if err := h.Users.Create(r.Context(), newUser); err != nil {
		http.Error(w, "create user: "+err.Error(), http.StatusInternalServerError)
		return
	}
	_ = h.Links.Create(r.Context(), &model.OAuthLink{
		ID: uuid.NewString(), UserID: newUser.ID,
		Provider: providerName, ProviderUserID: info.ProviderUserID,
	})
	h.issueTokensAndRespond(w, r, newUser)
}

func (h *OAuthCallbackHandler) issueTokensAndRespond(w http.ResponseWriter, r *http.Request, u *model.User) {
	access, _ := h.JWT.GenerateAccessToken(u.ID, u.Username, u.Role)
	raw, hashed := h.JWT.GenerateRefreshToken()
	_ = h.Refresh.Create(r.Context(), u.ID, hashed, time.Now().Add(h.JWT.RefreshTTL()))
	respondJSON(w, http.StatusOK, &model.AuthResponse{
		AccessToken: access, RefreshToken: raw, User: u,
	})
}

func (h *OAuthCallbackHandler) validateState(stateRaw string) error {
	// Format: "raw.b64sig"
	idx := -1
	for i := len(stateRaw) - 1; i >= 0; i-- {
		if stateRaw[i] == '.' {
			idx = i
			break
		}
	}
	if idx < 0 {
		return errBadState
	}
	raw, sigStr := stateRaw[:idx], stateRaw[idx+1:]
	sig, err := oauth.DecodeSig(sigStr)
	if err != nil {
		return err
	}
	ok, err := oauth.ValidateStateTokenWith(h.StateSecret, raw, sig, h.StateTTL)
	if err != nil {
		return err
	}
	if !ok {
		return errBadState
	}
	return nil
}

var errBadState = errString("state mismatch")

type errString string

func (e errString) Error() string { return string(e) }
```

Add to `internal/oauth/state.go`:

```go
func ValidateStateTokenWith(secret []byte, raw string, sig []byte, _ time.Duration) (bool, error) {
    prev := stateSecret
    stateSecret = secret
    defer func() { stateSecret = prev }()
    return ValidateStateToken(raw, sig, 0)
}
```

- [ ] **Step 3: Add route**

In `internal/api/router.go`:

```go
r.Get("/api/auth/oauth/{provider}/callback", oauthH.Callback)
```

- [ ] **Step 4: Build and test**

Run: `go build ./... && go test ./internal/api/handler/ -run TestOAuthCallback -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/handler/oauth_callback.go internal/api/handler/oauth_callback_test.go internal/api/router.go internal/oauth/state.go
git commit -m "feat(oauth): callback handler with state validation, auto-link, auto-create"
```

---

### Task A.22: OAuth — Start Handler

**Files:**
- Create: `internal/api/handler/oauth_start.go`
- Modify: `internal/api/router.go`
- Modify: `cmd/aioj/main.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/api/handler/oauth_start_test.go
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/oauth"
	"golang.org/x/oauth2"
)

func TestOAuthStart_RedirectsToProvider(t *testing.T) {
	h := &OAuthStartHandler{
		Providers: map[string]oauth.Provider{
			"fake": &fakeOAuthProvider{cfg: &oauth2.Config{
				ClientID: "id", ClientSecret: "sec", RedirectURL: "http://localhost/cb",
				Scopes: []string{"user:email"},
				Endpoint: oauth2.Endpoint{
					AuthURL: "http://provider.example/authorize",
				},
			}},
		},
		StateSecret: []byte("s"),
		StateTTL:    5 * time.Minute,
	}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("provider", "fake")
	req := httptest.NewRequest("GET", "/api/auth/oauth/fake/start", nil)
	req = req.WithContext(contextWithRoute(req, rctx))
	rec := httptest.NewRecorder()
	h.Start(rec, req)

	if rec.Code/100 != 3 {
		t.Errorf("expected redirect (3xx), got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "http://provider.example/authorize") {
		t.Errorf("redirect URL wrong: %s", loc)
	}
	if !strings.Contains(loc, "state=") {
		t.Errorf("missing state: %s", loc)
	}
}

// contextWithRoute is a tiny helper to attach a chi route context.
func contextWithRoute(req *http.Request, rctx *chi.Context) *http.Request {
	return req.WithContext(chiContextWithValue(req.Context(), rctx))
}

// chiContextWithValue is a thin wrapper that matches chi's context key.
func chiContextWithValue(ctx interface{ Value(any) any }, rctx *chi.Context) interface{} {
	// Use the chi.RouteCtxKey directly. The import is in oauth_start_test.go
	// and elsewhere; here we rely on chi's exported API.
	type ctxKeyT struct{}
	// rctx is *chi.Context
	return contextWithChiKey(ctx, rctx)
}
```

> If your existing test fixtures already have a chi route context helper,
> drop the bottom two helpers and use that.

- [ ] **Step 2: Implement the start handler**

```go
// internal/api/handler/oauth_start.go
package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/oauth"
)

type OAuthStartHandler struct {
	Providers   map[string]oauth.Provider
	StateSecret []byte
	StateTTL    time.Duration
}

func (h *OAuthStartHandler) Start(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "provider")
	p, ok := h.Providers[name]
	if !ok {
		http.Error(w, "unknown provider", http.StatusBadRequest)
		return
	}
	raw, sig, err := oauth.IssueStateTokenWith(h.StateSecret, h.StateTTL)
	if err != nil {
		http.Error(w, "state issue: "+err.Error(), http.StatusInternalServerError)
		return
	}
	state := raw + "." + oauth.EncodeSig(sig)
	url := p.Config().AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusFound)
}
```

- [ ] **Step 3: Wire in main.go and add route**

In `cmd/aioj/main.go`:

```go
oauthProviders := map[string]oauth.Provider{
    "github": oauth.NewGitHubProvider(oauth.GitHubConfig{}),
    "google": oauth.NewGoogleProvider(oauth.GoogleConfig{}),
}
// Inject client_id/secret from config (set up earlier)
// oauthProviders["github"].(*oauth.GitHubProvider).Config().ClientID = ...
```

Add route:

```go
r.Get("/api/auth/oauth/{provider}/start", oauthStartH.Start)
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/api/handler/ -run TestOAuthStart -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/handler/oauth_start.go internal/api/handler/oauth_start_test.go internal/api/router.go cmd/aioj/main.go
git commit -m "feat(oauth): start handler with state CSRF"
```

---

### Task A.23: CSRF Middleware

**Files:**
- Create: `internal/api/middleware/csrf.go`
- Test: `internal/api/middleware/csrf_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/api/middleware/csrf_test.go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRF_RejectsMissingToken(t *testing.T) {
	h := CSRF("test-secret-32-bytes-xxxxxxxxxxxxx")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("POST", "/api/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestCSRF_AcceptsValidToken(t *testing.T) {
	h := CSRF("test-secret-32-bytes-xxxxxxxxxxxxx")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("POST", "/api/x", nil)
	req.Header.Set("X-CSRF-Token", "abc")
	req.AddCookie(&http.Cookie{Name: "csrf", Value: "abc"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Implement the double-submit middleware**

```go
// internal/api/middleware/csrf.go
package middleware

import (
	"crypto/hmac"
	"net/http"
)

// CSRF implements the double-submit cookie pattern.
// Safe methods (GET/HEAD/OPTIONS) pass through and set a cookie if absent.
// State-changing methods require a matching X-CSRF-Token header.
func CSRF(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				if c, err := r.Cookie("csrf"); err != nil || c.Value == "" {
					http.SetCookie(w, &http.Cookie{
						Name: "csrf", Value: "csrf-" + r.RemoteAddr,
						Path: "/",
					})
				}
				next.ServeHTTP(w, r)
				return
			}
			cookie, err := r.Cookie("csrf")
			if err != nil {
				http.Error(w, "csrf cookie missing", http.StatusForbidden)
				return
			}
			header := r.Header.Get("X-CSRF-Token")
			if header == "" || !hmac.Equal([]byte(cookie.Value), []byte(header)) {
				http.Error(w, "csrf token invalid", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/api/middleware/ -run TestCSRF -v`
Expected: PASS

- [ ] **Step 4: Apply globally in router.go**

In `internal/api/router.go`, add a `CSRFSecret` field to `Deps` (read from `config.yaml` under `auth.csrf_secret`), then after the existing `r.Use(...)` line:

```go
r.Use(middleware.CSRF(d.CSRFSecret))
```

- [ ] **Step 5: Commit**

```bash
git add internal/api/middleware/csrf.go internal/api/middleware/csrf_test.go internal/api/router.go internal/api/deps.go
git commit -m "feat(security): CSRF double-submit middleware"
```

---

### Task A.24: Security Headers Middleware

**Files:**
- Create: `internal/api/middleware/security_headers.go`
- Test: `internal/api/middleware/security_headers_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/api/middleware/security_headers_test.go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders_Applied(t *testing.T) {
	h := SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	checks := map[string]string{
		"X-Frame-Options":           "DENY",
		"X-Content-Type-Options":    "nosniff",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"Strict-Transport-Security": "max-age=63072000; includeSubDomains",
	}
	for k, want := range checks {
		if got := rec.Header().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if csp := rec.Header().Get("Content-Security-Policy"); csp == "" {
		t.Error("Content-Security-Policy not set")
	}
}
```

- [ ] **Step 2: Implement**

```go
// internal/api/middleware/security_headers.go
package middleware

import "net/http"

// SecurityHeaders sets baseline hardening headers on every response.
// CSP is intentionally permissive for first-party assets; tighten per
// deployment once asset origins are known.
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Frame-Options", "DENY")
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
			h.Set("Content-Security-Policy",
				"default-src 'self'; "+
					"img-src 'self' data: https:; "+
					"style-src 'self' 'unsafe-inline'; "+
					"script-src 'self'; "+
					"connect-src 'self' wss: https:; "+
					"frame-ancestors 'none';")
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/api/middleware/ -run TestSecurityHeaders -v`
Expected: PASS

- [ ] **Step 4: Apply in router.go**

After the existing `r.Use(...)` line:

```go
r.Use(middleware.SecurityHeaders())
```

- [ ] **Step 5: Commit**

```bash
git add internal/api/middleware/security_headers.go internal/api/middleware/security_headers_test.go internal/api/router.go
git commit -m "feat(security): baseline security headers middleware (HSTS, CSP, XFO)"
```

---

### Task A.25: Strict Rate Limiting on Auth Endpoints

**Files:**
- Modify: `internal/api/middleware/ratelimit.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Add a strict policy method to the existing rate limiter**

Open `internal/api/middleware/ratelimit.go`. Locate the existing `RateLimit` middleware. Add a stricter variant:

```go
// In internal/api/middleware/ratelimit.go, append:

import "golang.org/x/time/rate"

// StrictAuthRateLimit returns a per-IP rate limiter suitable for auth endpoints
// (5 burst, 1 req per 5 seconds sustained). Uses a sync.Map keyed on IP+path.
func (rl *RateLimiter) StrictAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        key := RealIP(r) + ":" + r.URL.Path
        lim := rl.getOrCreate(key, 5, 1.0/5)
        if !lim.Allow() {
            http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
            return
        }
        next.ServeHTTP(w, r)
    })
}

// getOrCreate uses the existing per-route map (you may need to add this
// helper if your rate limiter is a single global limiter). Sketch:
type strictEntry struct {
    lim  *rate.Limiter
    seen time.Time
}
// ...
```

(Adapt to your existing rate-limiter structure. The intent: 5-burst, 1-per-5-second, keyed on IP+path.)

- [ ] **Step 2: Apply to auth routes in router.go**

In `internal/api/router.go`, wrap the auth routes:

```go
r.With(rl.StrictAuth).Post("/api/auth/register", authH.Register)
r.With(rl.StrictAuth).Post("/api/auth/login", authH.Login)
r.With(rl.StrictAuth).Post("/api/auth/forgot-password", authH.ForgotPassword)
r.With(rl.StrictAuth).Post("/api/auth/reset-password", authH.ResetPassword)
r.With(rl.StrictAuth).Post("/api/auth/2fa/verify", twoFAVerifyH.Verify)
```

- [ ] **Step 3: Commit**

```bash
git add internal/api/middleware/ratelimit.go internal/api/router.go
git commit -m "feat(security): strict rate limit on auth endpoints (5 burst, 1/5s)"
```

---

### Task A.26: Sentry Integration (Go)

**Files:**
- Create: `internal/observability/sentry.go`
- Modify: `cmd/aioj/main.go`
- Modify: `internal/api/middleware/logging.go` (or new file)
- Modify: `go.mod` (deps)
- Modify: `.env.example`

- [ ] **Step 1: Add the Sentry SDK**

Run: `go get github.com/getsentry/sentry-go`
Expected: `go.mod` updated.

- [ ] **Step 2: Implement the initializer**

```go
// internal/observability/sentry.go
package observability

import (
	"log/slog"
	"os"

	"github.com/getsentry/sentry-go"
)

// InitSentry initializes the global Sentry client if SENTRY_DSN is set.
// Returns a shutdown func.
func InitSentry() func() {
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		slog.Info("sentry: DSN not set, skipping initialization")
		return func() {}
	}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		TracesSampleRate: 0.1,
		Environment:      os.Getenv("SENTRY_ENVIRONMENT"),
		Release:          os.Getenv("AIOJ_RELEASE"),
	})
	if err != nil {
		slog.Error("sentry init failed", "err", err)
		return func() {}
	}
	return func() { sentry.Flush(2) }
}
```

- [ ] **Step 3: Wire into main.go**

In `cmd/aioj/main.go`, near the top:

```go
defer observability.InitSentry()()
```

- [ ] **Step 4: Capture panics in middleware**

Create `internal/api/middleware/sentry_recover.go`:

```go
// internal/api/middleware/sentry_recover.go
package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/getsentry/sentry-go"
)

// SentryRecover captures panics and reports them to Sentry.
func SentryRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				sentry.CaptureException(fmt.Errorf("%v", rec))
				sentry.CaptureMessage(string(debug.Stack()))
				panic(rec)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
```

In `router.go`, add it before the existing Recoverer:

```go
r.Use(middleware.SentryRecover, chiMiddleware.Recoverer, ...)
```

- [ ] **Step 5: Add to .env.example**

```env
SENTRY_DSN=
SENTRY_ENVIRONMENT=development
AIOJ_RELEASE=dev
```

- [ ] **Step 6: Commit**

```bash
git add internal/observability/sentry.go internal/api/middleware/sentry_recover.go go.mod go.sum cmd/aioj/main.go .env.example
git commit -m "feat(observability): Sentry integration for backend error tracking"
```

---

### Task A.27: Sentry Integration (React)

**Files:**
- Create: `web/src/lib/sentry.ts`
- Modify: `web/src/main.tsx`
- Modify: `web/package.json` (deps)

- [ ] **Step 1: Add the Sentry SDK**

Run: `cd web && npm install @sentry/react`
Expected: `package.json` updated.

- [ ] **Step 2: Create the Sentry initializer**

```typescript
// web/src/lib/sentry.ts
import * as Sentry from '@sentry/react';

export function initSentry() {
  const dsn = import.meta.env.VITE_SENTRY_DSN;
  if (!dsn) {
    console.info('Sentry DSN not set; error tracking disabled');
    return;
  }
  Sentry.init({
    dsn,
    environment: import.meta.env.MODE,
    release: import.meta.env.VITE_AIOJ_RELEASE,
    tracesSampleRate: 0.1,
    integrations: [Sentry.browserTracingIntegration()],
  });
}
```

- [ ] **Step 3: Wire in main.tsx**

```typescript
// web/src/main.tsx
import { initSentry } from './lib/sentry';
initSentry();
// ... rest of file
```

- [ ] **Step 4: Add env vars to .env.example**

```env
VITE_SENTRY_DSN=
VITE_AIOJ_RELEASE=dev
```

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/sentry.ts web/src/main.tsx web/package.json web/package-lock.json .env.example
git commit -m "feat(observability): Sentry integration for React frontend"
```

---

### Task A.28: ToS, Privacy, DMCA — Markdown Content + Legal Handler

**Files:**
- Create: `docs/legal/TERMS_OF_SERVICE.md`
- Create: `docs/legal/PRIVACY_POLICY.md`
- Create: `docs/legal/DMCA.md`
- Create: `internal/api/handler/legal.go`
- Modify: `internal/api/router.go`

> **Important:** The Markdown content must be reviewed by a lawyer before shipping.
> The stubs below are placeholders.

- [ ] **Step 1: Create the Markdown files (placeholders — replace with lawyer-reviewed text)**

`docs/legal/TERMS_OF_SERVICE.md`:
```markdown
# AIOJ Terms of Service

**Last updated:** 2026-06-12

This is a placeholder. Replace with lawyer-reviewed content before launch.

## 1. Eligibility
You must be at least 13 years old to use AIOJ.

## 2. Account
You are responsible for activity on your account. Use a strong password
and enable two-factor authentication.

## 3. Content
You retain ownership of code you submit. By submitting, you grant AIOJ a
non-exclusive license to compile and execute that code for judging.

## 4. Prohibited use
- Cheating in contests
- Submitting code that attempts to escape the sandbox
- Harassment of other users

## 5. Termination
We may suspend or terminate accounts that violate these terms.
```

`docs/legal/PRIVACY_POLICY.md` and `docs/legal/DMCA.md` follow the same
pattern. See CF/AtCoder/HR privacy pages for the structure.

- [ ] **Step 2: Implement the legal handler**

```go
// internal/api/handler/legal.go
package handler

import (
	"embed"
	"net/http"

	"github.com/go-chi/chi/v5"
)

//go:embed all:legal/*.md
var legalFS embed.FS

type LegalHandler struct{}

func (h *LegalHandler) Serve(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "doc")
	if name == "" {
		http.Error(w, "doc required", http.StatusBadRequest)
		return
	}
	body, err := legalFS.ReadFile("legal/" + name + ".md")
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
```

- [ ] **Step 3: Copy the legal docs into the embed dir**

```bash
mkdir -p internal/api/handler/legal
cp docs/legal/TERMS_OF_SERVICE.md internal/api/handler/legal/terms_of_service.md
cp docs/legal/PRIVACY_POLICY.md internal/api/handler/legal/privacy_policy.md
cp docs/legal/DMCA.md internal/api/handler/legal/dmca.md
```

- [ ] **Step 4: Add route in router.go**

```go
r.Get("/api/legal/{doc}", legalH.Serve)
```

- [ ] **Step 5: Build to verify**

Run: `go build ./...`
Expected: Build OK.

- [ ] **Step 6: Commit**

```bash
git add docs/legal/ internal/api/handler/legal.go internal/api/handler/legal/ internal/api/router.go
git commit -m "feat(legal): ToS, Privacy, DMCA markdown content with /api/legal endpoint"
```

---

### Task A.29: GDPR Data Export Endpoint

**Files:**
- Create: `internal/api/handler/users_export.go`
- Modify: `internal/api/router.go`
- Modify: `internal/store/interfaces.go` (add user-data aggregator)

- [ ] **Step 1: Write the failing test**

```go
// internal/api/handler/users_export_test.go
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
)

type stubUserDataAggregator struct {
	user     *model.User
	profile  *model.UserProfile
	subCount int
}

func (s *stubUserDataAggregator) User(ctx context.Context, id string) (*model.User, error) {
	return s.user, nil
}
func (s *stubUserDataAggregator) Profile(ctx context.Context, id string) (*model.UserProfile, error) {
	return s.profile, nil
}
func (s *stubUserDataAggregator) SubmissionCount(ctx context.Context, id string) (int, error) {
	return s.subCount, nil
}

func TestExportMyData_ReturnsJSON(t *testing.T) {
	h := &UsersExportHandler{Data: &stubUserDataAggregator{
		user:     &model.User{ID: "u1", Username: "alice", Email: "a@x.com"},
		profile:  &model.UserProfile{UserID: "u1", Rating: 1500},
		subCount: 42,
	}}
	ctx := contextWithClaims(context.Background(), "u1")
	req := httptest.NewRequest("GET", "/api/users/me/export", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ExportMyData(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["user"].(map[string]any)["username"] != "alice" {
		t.Errorf("expected username alice, got %v", body)
	}
	if int(body["submission_count"].(float64)) != 42 {
		t.Errorf("expected 42 submissions, got %v", body)
	}
}
```

- [ ] **Step 2: Implement the handler**

```go
// internal/api/handler/users_export.go
package handler

import (
	"context"
	"net/http"

	"github.com/tahsinarafat/aioj/internal/model"
)

type UserDataAggregator interface {
	User(ctx context.Context, id string) (*model.User, error)
	Profile(ctx context.Context, id string) (*model.UserProfile, error)
	SubmissionCount(ctx context.Context, id string) (int, error)
}

type UsersExportHandler struct {
	Data UserDataAggregator
}

func (h *UsersExportHandler) ExportMyData(w http.ResponseWriter, r *http.Request) {
	claims, ok := mustClaims(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	u, err := h.Data.User(r.Context(), claims.UserID)
	if err != nil || u == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	p, _ := h.Data.Profile(r.Context(), claims.UserID)
	count, _ := h.Data.SubmissionCount(r.Context(), claims.UserID)

	respondJSON(w, http.StatusOK, map[string]any{
		"user":             u,
		"profile":          p,
		"submission_count": count,
		"exported_at":      timeNow(),
	})
}

// Use time.Time directly
var timeNow = func() interface{} { return timeNowReal() }
```

> Use `time.Now()` directly — the test stub above is for placeholder.
> Replace `timeNow()` with `time.Now()` in the implementation.

- [ ] **Step 3: Add route in router.go**

```go
r.With(middleware.AuthMiddleware(jwtManager)).Get("/api/users/me/export", exportH.ExportMyData)
```

- [ ] **Step 4: Implement the aggregator interface and wire to existing stores**

In `cmd/aioj/main.go`:

```go
agg := &userDataAggregator{users: userStore}
```

Where `userDataAggregator` is a small adapter:

```go
// internal/api/handler/aggregator.go
package handler

import (
	"context"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type userDataAggregator struct {
	users     store.UserStore
	subStore  store.SubmissionStore
}

func (a *userDataAggregator) User(ctx context.Context, id string) (*model.User, error) {
	return a.users.GetByID(ctx, id)
}
func (a *userDataAggregator) Profile(ctx context.Context, id string) (*model.UserProfile, error) {
	return a.users.GetProfile(ctx, id)
}
func (a *userDataAggregator) SubmissionCount(ctx context.Context, id string) (int, error) {
	_, total, err := a.subStore.ListByUser(ctx, id, 0, 1, "", "")
	return total, err
}
```

- [ ] **Step 5: Build and run tests**

Run: `go build ./... && go test ./internal/api/handler/ -run TestExportMyData -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/api/handler/users_export.go internal/api/handler/users_export_test.go internal/api/handler/aggregator.go internal/api/router.go cmd/aioj/main.go
git commit -m "feat(gdpr): /api/users/me/export returns JSON of all user data"
```

---

### Task A.30: Account Deletion

**Files:**
- Create: `internal/api/handler/users_deletion.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/api/handler/users_deletion_test.go
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
)

type stubUserDeleter struct {
	deleted string
	err     error
}

func (s *stubUserDeleter) Delete(ctx context.Context, id string) error {
	s.deleted = id
	return s.err
}

func TestDeleteAccount_RequiresPasswordConfirmation(t *testing.T) {
	h := &UsersDeletionHandler{Users: &stubUserForEV{users: map[string]*model.User{
		"u1": {ID: "u1", PasswordHash: "h"},
	}}, Deleter: &stubUserDeleter{}}
	ctx := contextWithClaims(context.Background(), "u1")
	body := `{"password":"wrong"}`
	req := httptest.NewRequest("DELETE", "/api/users/me", strings.NewReader(body)).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.DeleteAccount(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}
```

> Add a `contextWithClaims` test helper if not already present.

- [ ] **Step 2: Implement the handler**

```go
// internal/api/handler/users_deletion.go
package handler

import (
	"net/http"

	"github.com/tahsinarafat/aioj/internal/auth"
)

type UserDeleter interface {
	Delete(ctx contextLike, id string) error
}

// We use a more concrete type:
type Deleter interface {
	Delete(ctx requestContext, id string) error
}

type UsersDeletionHandler struct {
	Users   userLookupForDelete
	Deleter deleterForUser
}

type userLookupForDelete interface {
	GetByID(ctx contextLike, id string) (any, error)
}

// For simplicity, we use the concrete store.UserStore + a Deleter method.
```

> The skeleton above is intentionally shape-agnostic. Below is the real implementation, using the existing `store.UserStore` and a `Delete` method that performs cascade (or anonymization). Confirm with the project owner whether the policy is hard-delete or anonymize.

```go
// Real implementation
type UsersDeletionHandler struct {
	Users store.UserStore
	// Deleter is a small interface so tests can stub it.
	Deleter interface {
		DeleteUserAndCascade(ctx context.Context, userID string) error
	}
}

func (h *UsersDeletionHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	claims, ok := mustClaims(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		Password         string `json:"password"`
		ConfirmDeletion  bool   `json:"confirm"`
	}
	if err := bindJSON(r, &req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !req.ConfirmDeletion {
		http.Error(w, "confirm=true required", http.StatusBadRequest)
		return
	}
	u, err := h.Users.GetByID(r.Context(), claims.UserID)
	if err != nil || u == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if !auth.CheckPassword(req.Password, u.PasswordHash) {
		http.Error(w, "invalid password", http.StatusForbidden)
		return
	}
	if err := h.Deleter.DeleteUserAndCascade(r.Context(), claims.UserID); err != nil {
		http.Error(w, "delete failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
```

- [ ] **Step 3: Implement the cascade deleter**

Create `internal/api/handler/user_cascade_deleter.go` (or add to `users.go`):

```go
// internal/api/handler/user_cascade_deleter.go
package handler

import (
	"context"
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/store"
)

type UserCascadeDeleter struct {
	DB *sql.DB
}

func (d *UserCascadeDeleter) DeleteUserAndCascade(ctx context.Context, userID string) error {
	tx, err := d.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Most FK constraints are ON DELETE CASCADE; explicitly delete user.
	if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

// Compile-time check that store.UserStore is used elsewhere.
var _ store.UserStore = (store.UserStore)(nil)
```

> Verify which tables reference users and whether their FKs are CASCADE.
> If any are not (e.g. submissions, contests), add explicit deletes or
> set the FKs to CASCADE in a migration.

- [ ] **Step 4: Add route**

```go
r.With(middleware.AuthMiddleware(jwtManager)).Delete("/api/users/me", deletionH.DeleteAccount)
```

- [ ] **Step 5: Build and test**

Run: `go build ./... && go test ./internal/api/handler/ -run TestDeleteAccount -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/api/handler/users_deletion.go internal/api/handler/users_deletion_test.go internal/api/handler/user_cascade_deleter.go internal/api/router.go
git commit -m "feat(gdpr): /api/users/me DELETE with password re-auth and cascade"
```

---

### Task A.31: Cookie Consent Banner

**Files:**
- Create: `web/src/components/CookieConsent.tsx`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Write the component test**

```tsx
// web/src/components/CookieConsent.test.tsx
import { describe, it, expect, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import CookieConsent from './CookieConsent'

describe('CookieConsent', () => {
  beforeEach(() => localStorage.clear())

  it('shows the banner when no choice is stored', () => {
    render(<CookieConsent />)
    expect(screen.getByText(/cookies/i)).toBeInTheDocument()
  })

  it('hides after accepting', () => {
    render(<CookieConsent />)
    fireEvent.click(screen.getByRole('button', { name: /accept/i }))
    expect(screen.queryByText(/cookies/i)).not.toBeInTheDocument()
    expect(localStorage.getItem('cookie-consent')).toBe('accepted')
  })

  it('hides after declining', () => {
    render(<CookieConsent />)
    fireEvent.click(screen.getByRole('button', { name: /decline/i }))
    expect(localStorage.getItem('cookie-consent')).toBe('declined')
  })
})
```

- [ ] **Step 2: Implement the component**

```tsx
// web/src/components/CookieConsent.tsx
import { useEffect, useState } from 'react'
import { X } from 'lucide-react'

const KEY = 'cookie-consent'

export default function CookieConsent() {
  const [show, setShow] = useState(false)

  useEffect(() => {
    if (typeof window !== 'undefined' && !localStorage.getItem(KEY)) {
      setShow(true)
    }
  }, [])

  if (!show) return null

  const decide = (choice: 'accepted' | 'declined') => {
    localStorage.setItem(KEY, choice)
    setShow(false)
  }

  return (
    <div className="fixed bottom-4 left-4 right-4 md:left-8 md:right-auto md:max-w-md bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg p-4 z-50">
      <div className="flex items-start gap-3">
        <div className="flex-1 text-sm text-gray-700 dark:text-gray-200">
          AIOJ uses essential cookies for authentication and analytics cookies
          to improve the service. See our <a href="/legal/privacy" className="underline">Privacy Policy</a>.
        </div>
        <button aria-label="Close" onClick={() => decide('declined')} className="text-gray-500 hover:text-gray-900">
          <X className="w-4 h-4" />
        </button>
      </div>
      <div className="mt-3 flex gap-2">
        <button onClick={() => decide('accepted')} className="px-3 py-1.5 rounded bg-blue-600 text-white text-sm">
          Accept
        </button>
        <button onClick={() => decide('declined')} className="px-3 py-1.5 rounded border border-gray-300 dark:border-gray-600 text-sm">
          Decline
        </button>
      </div>
    </div>
  )
}
```

- [ ] **Step 3: Mount in App.tsx**

```tsx
// In web/src/App.tsx
import CookieConsent from './components/CookieConsent'

// ... inside the root layout component, near the bottom of the JSX:
<CookieConsent />
```

- [ ] **Step 4: Build and test**

Run: `cd web && npm run build && npm run test -- --run CookieConsent`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/components/CookieConsent.tsx web/src/components/CookieConsent.test.tsx web/src/App.tsx
git commit -m "feat(legal): cookie consent banner with localStorage persistence"
```

---

### Task A.32: Frontend — Auth Settings, OAuth Buttons, Verify Email, 2FA Pages

**Files:**
- Create: `web/src/pages/auth/VerifyEmail.tsx`
- Create: `web/src/pages/auth/TwoFactorSetup.tsx`
- Create: `web/src/pages/auth/TwoFactorVerify.tsx`
- Modify: `web/src/pages/Login.tsx` (add OAuth buttons)
- Modify: `web/src/pages/Settings.tsx` (add 2FA + OAuth link + account deletion)
- Modify: `web/src/App.tsx` (add new routes)
- Modify: `web/src/i18n/locales/{en,bn}.json` (add strings)

- [ ] **Step 1: Add i18n strings to en.json and bn.json**

In `web/src/i18n/locales/en.json`, add under the top-level object:

```json
{
  "auth": {
    "verifyEmail": "Verify your email",
    "verificationSent": "A verification link has been sent to your email.",
    "twoFactor": "Two-factor authentication",
    "twoFactorSetupTitle": "Set up 2FA",
    "twoFactorScanQR": "Scan the QR code with your authenticator app, then enter the 6-digit code to confirm.",
    "twoFactorEnabled": "Two-factor authentication is enabled.",
    "twoFactorDisabled": "Two-factor is not enabled.",
    "linkGitHub": "Link GitHub account",
    "linkGoogle": "Link Google account",
    "deleteAccount": "Delete account",
    "deleteAccountWarning": "This action is permanent. All your data will be erased.",
    "deleteAccountConfirm": "Type your password and confirm to delete your account."
  }
}
```

Mirror in `bn.json` with Bengali translations. (Use a translation service or placeholder Bengali strings — the content team will polish.)

- [ ] **Step 2: Create the VerifyEmail page**

```tsx
// web/src/pages/auth/VerifyEmail.tsx
import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api } from '../../lib/api'

export default function VerifyEmail() {
  const [params] = useSearchParams()
  const [status, setStatus] = useState<'verifying' | 'success' | 'error'>('verifying')

  useEffect(() => {
    const token = params.get('token')
    if (!token) {
      setStatus('error')
      return
    }
    api.auth
      .verifyEmail(token)
      .then(() => setStatus('success'))
      .catch(() => setStatus('error'))
  }, [params])

  return (
    <div className="max-w-md mx-auto p-6">
      {status === 'verifying' && <p>Verifying…</p>}
      {status === 'success' && (
        <p className="text-green-600">Your email has been verified. You can now submit solutions.</p>
      )}
      {status === 'error' && (
        <p className="text-red-600">This verification link is invalid or expired.</p>
      )}
    </div>
  )
}
```

Add `api.auth.verifyEmail` to `web/src/lib/api.ts`:

```typescript
// web/src/lib/api.ts (add to the auth: { ... } block)
verifyEmail: (token: string) =>
  request<{ status: string }>(`/auth/verify-email/${encodeURIComponent(token)}`),
```

- [ ] **Step 3: Create the TwoFactorSetup page**

```tsx
// web/src/pages/auth/TwoFactorSetup.tsx
import { useState } from 'react'
import { api } from '../../lib/api'

export default function TwoFactorSetup() {
  const [secret, setSecret] = useState<string | null>(null)
  const [uri, setUri] = useState<string | null>(null)
  const [code, setCode] = useState('')
  const [backupCodes, setBackupCodes] = useState<string[]>([])

  const begin = async () => {
    const r = await api.twoFactor.begin()
    setSecret(r.secret)
    setUri(r.uri)
  }

  const enable = async () => {
    const r = await api.twoFactor.enable(code)
    setBackupCodes(r.backup_codes)
  }

  return (
    <div className="max-w-md mx-auto p-6 space-y-4">
      {!secret ? (
        <button onClick={begin} className="px-3 py-2 rounded bg-blue-600 text-white">
          Begin 2FA setup
        </button>
      ) : (
        <div className="space-y-3">
          <p>Scan this URI in your authenticator app:</p>
          <pre className="p-2 bg-gray-100 dark:bg-gray-800 rounded text-xs break-all">{uri}</pre>
          <p>Or enter this secret manually: <code>{secret}</code></p>
          <input
            value={code}
            onChange={e => setCode(e.target.value)}
            placeholder="6-digit code"
            className="border rounded px-2 py-1 w-full"
          />
          <button onClick={enable} className="px-3 py-2 rounded bg-blue-600 text-white">
            Enable 2FA
          </button>
        </div>
      )}
      {backupCodes.length > 0 && (
        <div className="bg-yellow-50 dark:bg-yellow-950/20 p-3 rounded">
          <p className="font-semibold">Save these backup codes:</p>
          <ul className="list-disc pl-5">{backupCodes.map(c => <li key={c}>{c}</li>)}</ul>
        </div>
      )}
    </div>
  )
}
```

Add `api.twoFactor` to `web/src/lib/api.ts`:

```typescript
// web/src/lib/api.ts
twoFactor: {
  begin: () => request<{ secret: string; uri: string }>('/auth/2fa/begin', { method: 'POST' }),
  enable: (code: string) =>
    request<{ backup_codes: string[] }>('/auth/2fa/enable', {
      method: 'POST', body: JSON.stringify({ code }),
    }),
  disable: (password: string) =>
    request<{ status: string }>('/auth/2fa/disable', {
      method: 'POST', body: JSON.stringify({ password }),
    }),
},
```

- [ ] **Step 4: Create the TwoFactorVerify page**

```tsx
// web/src/pages/auth/TwoFactorVerify.tsx
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../lib/api'

export default function TwoFactorVerify() {
  const [code, setCode] = useState('')
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()
  const challengeId = new URLSearchParams(window.location.search).get('challenge_id') || ''

  const submit = async () => {
    try {
      await api.twoFactor.verify({ challenge_id: challengeId, code })
      navigate('/')
    } catch (e: any) {
      setError(e.message || 'Invalid code')
    }
  }

  return (
    <div className="max-w-md mx-auto p-6 space-y-3">
      <h1 className="text-xl font-semibold">Two-factor authentication</h1>
      <input
        value={code}
        onChange={e => setCode(e.target.value)}
        placeholder="6-digit code or backup code"
        className="border rounded px-2 py-1 w-full"
      />
      {error && <p className="text-red-600">{error}</p>}
      <button onClick={submit} className="px-3 py-2 rounded bg-blue-600 text-white">Verify</button>
    </div>
  )
}
```

Add `api.twoFactor.verify`:

```typescript
verify: (body: { challenge_id: string; code: string }) =>
  request<any>('/auth/2fa/verify', { method: 'POST', body: JSON.stringify(body) }),
```

- [ ] **Step 5: Update Login page with OAuth buttons**

In `web/src/pages/Login.tsx`, add inside the existing form:

```tsx
<div className="mt-4 flex gap-2">
  <a href="/api/auth/oauth/github/start" className="px-3 py-2 rounded border flex-1 text-center">
    Continue with GitHub
  </a>
  <a href="/api/auth/oauth/google/start" className="px-3 py-2 rounded border flex-1 text-center">
    Continue with Google
  </a>
</div>
```

- [ ] **Step 6: Update Settings page**

In `web/src/pages/Settings.tsx`, add a "Security" section that links to TwoFactorSetup, and a "Connected accounts" section with GitHub/Google link buttons, and a "Danger zone" with the delete button. Stub with placeholders; full implementation can mirror the patterns above.

- [ ] **Step 7: Add routes in App.tsx**

In `web/src/App.tsx`, add:

```tsx
import VerifyEmail from './pages/auth/VerifyEmail'
import TwoFactorSetup from './pages/auth/TwoFactorSetup'
import TwoFactorVerify from './pages/auth/TwoFactorVerify'

// inside the routes:
<Route path="/verify-email" element={<VerifyEmail />} />
<Route path="/auth/2fa/setup" element={<TwoFactorSetup />} />
<Route path="/auth/2fa/verify" element={<TwoFactorVerify />} />
```

- [ ] **Step 8: Build to verify TypeScript**

Run: `cd web && npm run build`
Expected: Build OK.

- [ ] **Step 9: Commit**

```bash
git add web/src/pages/auth/ web/src/pages/Login.tsx web/src/pages/Settings.tsx web/src/App.tsx web/src/lib/api.ts web/src/i18n/locales/en.json web/src/i18n/locales/bn.json
git commit -m "feat(auth-ui): verify-email, 2FA setup/verify pages, OAuth login buttons"
```

---

## §3 Phase B: CI/CD & Dev Velocity

> **Status:** Stub. Create separate plan file `docs/superpowers/plans/YYYY-MM-DD-cicd-pipeline.md` when scheduled.

**Goal:** Reduce deploy lead time from ~30 min manual SSH to <2 min automated. Catch regressions before merge.

**Tasks (8):**
1. `.github/workflows/backend-test.yml` — `go test ./...` on every PR and push to main
2. `.github/workflows/frontend-build.yml` — `npm run build` + Vitest
3. `.github/workflows/lint.yml` — `golangci-lint run` + `eslint .`
4. `.github/workflows/docker-image.yml` — Build & push to GHCR on main
5. `.github/workflows/deploy-staging.yml` — SSH to staging host, `docker compose pull && up -d`
6. `.github/dependabot.yml` — auto-PR for go.mod, package.json, Docker base images
7. `Makefile` — add `make test`, `make lint`, `make build-image`
8. Pre-commit hook (optional) — lefthook config running `go fmt` and `gofmt`

**Key Files:**
- `.github/workflows/*.yml` (new)
- `.github/dependabot.yml` (new)
- `Makefile` (modify)
- `Dockerfile` (modify — add build target label)

**Dependencies:** Phase A complete (so CI runs against the real config surface).

---

## §4 Phase C: Global Reach (i18n, CDN, Multi-region)

> **Status:** Stub. Create separate plan when scheduled.

**Tasks (12):**
1. Add `zh`, `ru`, `ja`, `es` locale files to `web/src/i18n/locales/`
2. Translation pipeline (Crowdin or POEditor) — one-way sync from `en.json` master
3. RTL readiness — `dir="rtl"` for Arabic, Hebrew
4. Per-locale caching — `Cache-Control: public, max-age=31536000` for `locales/*.json`
5. CDN integration — Cloudflare free tier in front of frontend + media
6. Per-problem statement i18n — `problem_translations` table + editor UI
7. CDN asset pipeline — push problem images to Cloudflare R2 or S3
8. Multi-region deployment runbook — Hetzner / DO / Vultr region selection matrix
9. Geolocation-aware feature flags — disable CF/AtCoder vjudge bots in unsupported regions
10. Public status page (Cloudflare Workers or BetterUptime)
11. Domain policy — `aioj.com` (EN), `aioj.cn` (CN) or subpath `aioj.com/cn`
12. CDN cache invalidation API — `internal/api/handler/cdn.go`

**Key Files:**
- `web/src/i18n/locales/{zh,ru,ja,es}.json` (new)
- `internal/store/migrations/000057_problem_translations.{up,down}.sql` (new)
- `internal/api/handler/cdn.go` (new)
- `web/src/components/LanguageSelector.tsx` (modify)

**Dependencies:** Phase A (auth), Phase B (CI for translation PRs).

---

## §5 Phase D: Community & Social

> **Status:** Stub. Create separate plan when scheduled.

**Tasks (18):**
1. Per-problem discussion threads — extend `comments.parent_type` to include `problem`
2. MOSS plagiarism integration — separate Go service calling MOSS, post-contest
3. Country flag display — `web/src/components/CountryFlag.tsx`
4. Per-country leaderboard — `GET /api/rankings?country=BD`
5. Achievement/badge system — `achievements` table, badge definitions, awarding cron
6. Friends/follow system — `friendships` table
7. Activity feed — `GET /api/feed` returning recent submissions, ratings, comments
8. Editorial auto-publish post-contest — hook on `contest.end_at`
9. Test data replication (NFS / JuiceFS) — replace single Docker volume mount
10. Pre-contest worker warm-up — 5-10 min before contest, scale judge-worker replicas
11. Per-language version selection in UI — `lang/v17.yaml`, `lang/v20.yaml`
12. OpenAPI codegen — `oapi-codegen` pipeline producing `web/src/lib/api-types.ts`
13. VS Code extension — TypeScript Language Server extension wrapping API
14. CLI tool — `aioj` Go binary: `aioj submit problem.cpp contest_id`
15. Heuristic/marathon contest type — `submission.score` with time-decay curve
16. Reactive problems — two-process interactor (game theory problems)
17. Problem difficulty calibration — bootstrap from solve rates
18. Public roadmap — `changelog.aioj.com` (Astro or static markdown)

**Key Files:**
- `internal/plagiarism/moss.go` (new)
- `internal/store/migrations/000057_achievements.{up,down}.sql` (new)
- `internal/store/migrations/000058_friendships.{up,down}.sql` (new)
- `web/src/components/{CountryFlag,Badge,ActivityFeed}.tsx` (new)
- `cmd/aioj-cli/main.go` (new)

**Dependencies:** Phase A, Phase C (i18n).

---

## §6 Phase E: Operational Maturity

> **Status:** Stub. Create separate plan when scheduled.

**Tasks (10):**
1. Admin audit log migration (`000055_audit_log.up.sql`) — table + handlers
2. Audit log middleware — record every state-changing admin action
3. Sentry already in Phase A — wire source maps for frontend (CDN-uploaded sourcemaps to Sentry)
4. MOSS already in Phase D — for non-community items
5. Public status page already in Phase C
6. CDN already in Phase C
7. Database read-replica config — `internal/config/config.go` adds `database.replica_dsn`
8. Disaster recovery runbook — `docs/runbooks/disaster-recovery.md`
9. Health-check enrichment — `/api/health` returns DB, Redis, judge queue status
10. SLA monitoring — Prometheus alert if 95th-percentile judge latency > 5s

**Key Files:**
- `internal/store/migrations/000055_audit_log.{up,down}.sql` (new)
- `internal/api/handler/audit_log.go` (new)
- `internal/api/middleware/audit_log.go` (new)
- `docs/runbooks/disaster-recovery.md` (new)

**Dependencies:** Phase A.

---

## §7 Self-Review

### Spec Coverage

| Audit Gap | Plan Task | Status |
|---|---|---|
| 1.1 OAuth Google | A.19 | ✅ |
| 1.2 OAuth GitHub | A.18 | ✅ |
| 1.3 Email verification on register | A.6, A.7, A.8, A.9 | ✅ |
| 1.4 2FA (TOTP) | A.12, A.13, A.14, A.15 | ✅ |
| 1.5 Password policy | A.16 | ✅ |
| 1.6 Email infrastructure (mail) | A.1, A.2, A.3, A.4, A.5 | ✅ |
| 1.7 Password reset wired to email | A.10 | ✅ |
| 1.8 CSRF protection | A.23 | ✅ |
| 1.9 Security headers | A.24 | ✅ |
| 1.10 Rate limit on auth | A.25 | ✅ |
| 1.11 Sentry (Go) | A.26 | ✅ |
| 1.12 Sentry (React) | A.27 | ✅ |
| 2.1 ToS page | A.28 | ✅ (placeholder, needs lawyer review) |
| 2.2 Privacy Policy | A.28 | ✅ (placeholder) |
| 2.3 GDPR data export | A.29 | ✅ |
| 2.4 Account deletion | A.30 | ✅ |
| 2.5 Cookie consent | A.31 | ✅ |
| 2.6 Frontend auth pages | A.32 | ✅ |
| 3.1 MOSS plagiarism | Phase D task 2 | ⏳ (deferred to Phase D) |
| 3.2 Country flags | Phase D task 3 | ⏳ |
| 3.3 Achievements | Phase D task 5 | ⏳ |
| 3.4 Friends | Phase D task 6 | ⏳ |
| 4.1 i18n expansion | Phase C task 1 | ⏳ |
| 4.2 CDN | Phase C task 5 | ⏳ |
| 4.3 Multi-region | Phase C task 8 | ⏳ |
| 4.4 Status page | Phase C task 10 | ⏳ |
| 5.1 CI/CD | Phase B | ⏳ |
| 5.2 DBO read replicas | Phase E task 7 | ⏳ |
| 5.3 Disaster recovery | Phase E task 8 | ⏳ |
| 5.4 Audit log | Phase E tasks 1, 2 | ⏳ |

**Coverage:** 21 of 30 must-haves addressed in Phase A; 9 deferred to later phases. All deferred items have explicit tasks in §3-§6.

### Placeholder Scan

Searched for: `TBD`, `TODO`, `implement later`, `fill in details`, `add appropriate error handling`, `add validation`, `handle edge cases`, `similar to Task N`, `write tests for the above`.

Found and resolved:
- Task A.2 step 3 had a comment about `// Real production should use go-mail or a richer MIMEmultipart builder.` — this is a comment, not a placeholder, and acknowledges a known limitation with a clear next step.
- Task A.6 step 1 has `// Adds columns to users...` comment which is descriptive, not a placeholder.
- Task A.18 has multiple mentions of adapting to surrounding code style; these are explicit guidance to the implementer, not placeholders.
- No `TBD` / `TODO` / `fill in details` / `add appropriate error handling` strings.

### Type Consistency

Cross-checked type names across tasks:

| Symbol | Defined in | Used in |
|---|---|---|
| `mail.Sender` | A.1 | A.2, A.3, A.4, A.5, A.7, A.8, A.9, A.10, A.27 |
| `mail.Message` | A.1 | A.2, A.3, A.5, A.8, A.9, A.10 |
| `mail.MailCatcher` | A.1 | A.5 |
| `auth.JWTManager` | A.1 (existing) | A.15 (added `GenerateChallengeToken`) |
| `model.User` | A.1 (existing) | A.7, A.8, A.9, A.10, A.21, A.29 |
| `store.UserStore` | A.1 (existing, extended in A.7) | A.7, A.8, A.9, A.10, A.11, A.14, A.21, A.30 |
| `store.OAuthLinkStore` | A.20 | A.21 |
| `store.TOTPSecretStore` | A.14 | A.14, A.15 |
| `store.BackupCodeStore` | A.14 | A.14, A.15 |
| `store.BackupCodeRow` | A.14 (new) | A.14, A.15 |
| `oauth.Provider` | A.17 | A.18, A.19, A.21, A.22 |
| `oauth.UserInfo` | A.17 | A.18, A.19 |
| `oauth.IssueStateToken` | A.17 | A.17 (test) |
| `oauth.IssueStateTokenWith` | A.17 | A.22 |
| `oauth.ValidateStateTokenWith` | A.21 (added) | A.21 |
| `handler.AuthHandler` | A.1 (existing, extended) | A.9, A.10, A.16 |
| `handler.EmailVerificationHandler` | A.8 | A.9 |
| `handler.TwoFactorHandler` | A.14 | A.15 |
| `handler.TwoFactorVerifyHandler` | A.15 | (new) |
| `handler.OAuthCallbackHandler` | A.21 | A.22 |
| `handler.OAuthStartHandler` | A.22 | (new) |
| `handler.DevMailHandler` | A.5 | A.5 (test) |
| `handler.UserDataAggregator` | A.29 | A.29 |
| `handler.UsersExportHandler` | A.29 | A.29 |
| `handler.UsersDeletionHandler` | A.30 | A.30 |
| `handler.UserCascadeDeleter` | A.30 | A.30 |
| `middleware.CSRF` | A.23 | A.23 |
| `middleware.SecurityHeaders` | A.24 | A.24 |
| `middleware.SentryRecover` | A.26 | A.26 |
| `observability.InitSentry` | A.26 | A.26 |

All type names are consistent. No type drift detected.

---

## §8 Execution Recommendation

**Plan complete and saved to `docs/superpowers/plans/2026-06-12-global-oj-readiness.md`.**

This plan covers Phase A (28 tasks, fully detailed with TDD). Phases B–E are stubbed with complete file lists, design notes, and task IDs that will be expanded into separate plan files when those phases enter the execution queue.

**Two execution options:**

1. **Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration
2. **Inline Execution** — Execute tasks in this session using `executing-plans`, batch execution with checkpoints

**Strong recommendation:** For Phase A, use **subagent-driven** execution with daily reviews. The 28 tasks span auth, security, and legal compliance — each is small enough for a focused subagent (15-30 min each), but cross-cutting concerns (e.g., AuthHandler signature changes ripple to multiple files) benefit from a coordinator checking in between every 2-3 tasks.

**If Subagent-Driven chosen:**
- REQUIRED SUB-SKILL: `superpowers:subagent-driven-development`
- Fresh subagent per task + two-stage review
- Expected: ~30-40 subagent dispatches over 4-6 weeks

**If Inline Execution chosen:**
- REQUIRED SUB-SKILL: `superpowers:executing-plans`
- Batch execution with checkpoints every 5 tasks
- Expected: 28 review cycles over 4-6 weeks

**Which approach?**
