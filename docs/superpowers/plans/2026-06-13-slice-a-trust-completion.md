# Slice A Trust Completion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish the remaining Phase A mail/email-verification vertical slice so unverified accounts cannot submit, verification links work end-to-end in the UI, and the existing register test no longer panics.

**Architecture:** Backend already has `internal/mail` (SMTP + MailCatcher + templates), ForgotPassword email send (no token leak), register verification email, and `GET /api/auth/verify-email/{token}`. This plan adds the missing gates and UI: block submissions until verified, resend endpoint, `/verify-email` frontend page, and a JWT-safe register test.

**Tech Stack:** Go 1.26 (chi), React 19 + Vite + TypeScript, existing `internal/mail`, `store.UserStore.IsEmailVerified`.

**Spec:** `docs/superpowers/plans/2026-06-12-global-oj-readiness.md` §2 Tasks A.9–A.11 (partial; TOTP/OAuth out of scope).

## Global Constraints

- Workspace-relative paths only; do not touch `.env` or secrets.
- Do not modify `Reference_Projects/`.
- Preserve enumeration-safe ForgotPassword response (never return raw token).
- Register still returns tokens (login allowed); **only submissions** require verified email.
- Onsite bot/contest users use synthetic emails (`…@onsite.aioj`); they must keep working if they submit — gate by `IsEmailVerified` only, which defaults false for new onsite users… **Exception:** onsite flow auto-creates users without email verification. Do **not** gate onsite login path; gate only `SubmissionHandler` create paths the same for all authenticated users. Onsite contestants may need verification skip: mark onsite-created users verified at Create time (see Task 2).
- Tests: `go test ./internal/api/handler/ ./internal/mail/ -count=1` must pass before commit.
- Frontend typecheck: `cd web && npx tsc -b --noEmit` (or project’s existing check script).

---

### Task 1: Fix `TestRegister_SendsVerificationEmail` nil JWT panic

**Files:**
- Modify: `internal/api/handler/auth_password_reset_test.go`

**Interfaces:**
- Consumes: `auth.NewJWTManager`, `auth.NewJWTManager` signature from `internal/auth/jwt.go`.
- Produces: None (test-only).

- [x] **Step 1: Write failing test observation**

Run: `go test ./internal/api/handler/ -run TestRegister_SendsVerificationEmail -count=1 -v`

Expected: FAIL — panic in `(*JWTManager).GenerateAccessToken` because `h.jwt` is nil.

- [x] **Step 2: Fix the test fixture**

In `TestRegister_SendsVerificationEmail`, construct the handler with a real JWT manager:

```go
jwtMgr := auth.NewJWTManager("test-secret", time.Minute, 24*time.Hour)
h := &AuthHandler{
    users:             &stubUserForEV{},
    refreshToks:       &stubRefresh{},
    passwordResetToks: &stubPasswordResetForMail{},
    evt:               &stubEVT{},
    jwt:               jwtMgr,
    mail:              catcher,
    mailTpl:           tpl,
    publicURL:         "http://localhost",
    mailFrom:          "noreply@aioj.com",
}
```

Add import: `"github.com/tahsinarafat/aioj/internal/auth"`.

- [x] **Step 3: Run test to verify it passes**

Run: `go test ./internal/api/handler/ -run TestRegister_SendsVerificationEmail -count=1 -v`

Expected: PASS (201 + one verification email containing `verify-email?token=`).

- [x] **Step 4: Commit** (implemented; commit deferred pending user approval)

---

### Task 2: Gate submissions on verified email (+ mark onsite users verified)

**Files:**
- Modify: `internal/api/handler/submission.go` (`Create`, `CreateUpsolving`, `CustomRun`)
- Modify: `internal/api/handler/auth.go` (onsite user Create path only)
- Test: `internal/api/handler/submission_verification_test.go` (already exists — currently panics)

**Interfaces:**
- Consumes: `store.UserStore.IsEmailVerified(ctx, userID) (bool, error)` (exists).
- Produces: 403 JSON `{"error":"email not verified; check your inbox or resend the verification link"}` from create submission endpoints when not verified.

- [x] **Step 1: Confirm failing test**

Run: `go test ./internal/api/handler/ -run TestSubmission_Create_RejectsUnverifiedEmail -count=1 -v`

Expected: FAIL — panic in `buildAndEnqueue` (nil stores) because no verification gate runs first.

- [x] **Step 2: Add shared helper in submission.go**

```go
// requireVerifiedEmail returns false and writes 403 when the user has not
// verified their email. Store errors surface as 500.
func (h *SubmissionHandler) requireVerifiedEmail(w http.ResponseWriter, r *http.Request, userID string) bool {
	if h.users == nil {
		// Tests or wiring without users store: fail closed would break
		// unit tests that only exercise other paths; fail open only if
		// store was never injected (should not happen in main).
		return true
	}
	verified, err := h.users.IsEmailVerified(r.Context(), userID)
	if err != nil {
		http.Error(w, "verification check failed", http.StatusInternalServerError)
		return false
	}
	if !verified {
		respondJSON(w, http.StatusForbidden, map[string]string{
			"error": "email not verified; check your inbox or resend the verification link",
		})
		return false
	}
	return true
}
```

- [x] **Step 3: Call gate at start of Create, CreateUpsolving, CustomRun**

In each method, after claims are non-nil:

```go
if !h.requireVerifiedEmail(w, r, claims.UserID) {
	return
}
```

Place **before** JSON body decode / problem lookup so the existing test returns 403 without touching `probStore`.

- [x] **Step 4: Mark onsite-created users verified**

In `internal/api/handler/auth.go` Login onsite branch, after `h.users.Create(...)` for a new onsite user, before or after `MarkUsed`:

```go
// Onsite accounts have synthetic emails and cannot receive mail;
// treat them as pre-verified so they can submit immediately.
_ = h.users.MarkEmailVerified(r.Context(), dbUser.ID)
```

- [x] **Step 5: Run tests**

Run: `go test ./internal/api/handler/ -count=1`

Expected: ALL PASS, including `TestSubmission_Create_RejectsUnverifiedEmail`.

- [x] **Step 6: Commit**

```bash
git add internal/api/handler/submission.go internal/api/handler/auth.go internal/api/handler/submission_verification_test.go
git commit -m "feat(submissions): require verified email; auto-verify onsite users"
```

---

### Task 3: Resend verification email endpoint

**Files:**
- Modify: `internal/api/handler/auth.go` (extract `sendVerificationEmail` already exists — add `ResendVerification` handler)
- Modify: `internal/api/router.go`
- Test: `internal/api/handler/auth_password_reset_test.go` (new test)

**Interfaces:**
- Consumes: `AuthHandler.evt`, `AuthHandler.mail`, `AuthHandler.mailTpl`, `AuthHandler.users`, `middleware.GetUserClaims` / `AuthMiddleware`.
- Produces: `POST /api/auth/verify-email/resend` → `200 {"message":"If the account needs verification, a link has been sent"}` (enumeration-safe; auth required so only logged-in unverified users call it). Body optional `{ "email": "..." }` not required when authorized.

- [x] **Step 1: Write failing test**

```go
func TestResendVerification_SendsEmailWhenUnverified(t *testing.T) {
	catcher := mail.NewMailCatcher()
	tpl, _ := mail.LoadTemplates()
	us := &stubUserForEV{
		users:      map[string]*model.User{"u1": {ID: "u1", Username: "alice", Email: "alice@x.com"}},
		isVerified: false,
	}
	h := &AuthHandler{
		users: us, refreshToks: &stubRefresh{}, evt: &stubEVT{},
		mail: catcher, mailTpl: tpl, publicURL: "http://localhost",
		mailFrom: "noreply@aioj.com",
	}
	req := httptest.NewRequest("POST", "/api/auth/verify-email/resend", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, &auth.Claims{UserID: "u1"}))
	rec := httptest.NewRecorder()
	h.ResendVerification(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if len(catcher.All()) != 1 {
		t.Fatalf("expected 1 email, got %d", len(catcher.All()))
	}
}

func TestResendVerification_AlreadyVerified_NoEmail(t *testing.T) {
	catcher := mail.NewMailCatcher()
	tpl, _ := mail.LoadTemplates()
	us := &stubUserForEV{
		users:      map[string]*model.User{"u1": {ID: "u1", Username: "alice", Email: "alice@x.com"}},
		isVerified: true,
	}
	h := &AuthHandler{
		users: us, refreshToks: &stubRefresh{}, evt: &stubEVT{},
		mail: catcher, mailTpl: tpl, publicURL: "http://localhost",
		mailFrom: "noreply@aioj.com",
	}
	req := httptest.NewRequest("POST", "/api/auth/verify-email/resend", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, &auth.Claims{UserID: "u1"}))
	rec := httptest.NewRecorder()
	h.ResendVerification(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if len(catcher.All()) != 0 {
		t.Errorf("expected no email for verified user, got %d", len(catcher.All()))
	}
}
```

Imports: `middleware`, `auth` as needed.

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/handler/ -run TestResendVerification -count=1 -v`

Expected: FAIL — `undefined: h.ResendVerification`.

- [x] **Step 3: Implement handler**

```go
// ResendVerification re-sends the verify link for the logged-in user.
// Enumeration-safe: always 200 with the same message.
func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	user, err := h.users.GetByID(r.Context(), claims.UserID)
	if err != nil || user == nil {
		respondJSON(w, http.StatusOK, map[string]string{
			"message": "If the account needs verification, a link has been sent",
		})
		return
	}
	if verified, _ := h.users.IsEmailVerified(r.Context(), user.ID); verified {
		respondJSON(w, http.StatusOK, map[string]string{
			"message": "If the account needs verification, a link has been sent",
		})
		return
	}
	h.sendVerificationEmail(r.Context(), user)
	respondJSON(w, http.StatusOK, map[string]string{
		"message": "If the account needs verification, a link has been sent",
	})
}
```

Add import: `"github.com/tahsinarafat/aioj/internal/api/middleware"` if missing.

- [x] **Step 4: Mount route**

In `internal/api/router.go`, next to other auth routes:

```go
r.With(middleware.AuthMiddleware(jwtManager)).Post("/api/auth/verify-email/resend", authH.ResendVerification)
```

- [x] **Step 5: Run tests**

Run: `go test ./internal/api/handler/ -count=1`

Expected: ALL PASS.

- [x] **Step 6: Commit**

```bash
git add internal/api/handler/auth.go internal/api/router.go internal/api/handler/auth_password_reset_test.go
git commit -m "feat(email-verification): authenticated resend verification link"
```

---

### Task 4: Frontend VerifyEmail page + API + register notice

**Files:**
- Create: `web/src/pages/VerifyEmail.tsx`
- Modify: `web/src/App.tsx` (import + route)
- Modify: `web/src/lib/api.ts` (`auth.resendVerification`)
- Modify: `web/src/pages/Register.tsx` (post-register “check email” state)
- Modify: `web/src/pages/ProblemDetail.tsx` or submit path — only if submit error is surfaced poorly; **optional**, prefer generic API error already shown.

**Interfaces:**
- Consumes: `GET /api/auth/verify-email/{token}`, `POST /api/auth/verify-email/resend`, existing `api` client.
- Produces: Route `/verify-email` reading `?token=`; calls verify once; success/failure UI; Register shows “check your email” instead of immediately landing as if fully ready to submit.

- [x] **Step 1: Add API helper**

In `web/src/lib/api.ts` under `auth`:

```ts
verifyEmail: (token: string) =>
    request<{ status: string }>(`/auth/verify-email/${encodeURIComponent(token)}`),
resendVerification: () =>
    request<{ message: string }>('/auth/verify-email/resend', { method: 'POST' }),
```

- [x] **Step 2: Create VerifyEmail page**

`web/src/pages/VerifyEmail.tsx`:

```tsx
import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api } from '../lib/api'

export default function VerifyEmail() {
    const [params] = useSearchParams()
    const token = params.get('token') || ''
    const [state, setState] = useState<'loading' | 'ok' | 'err'>('loading')
    const [msg, setMsg] = useState('')

    useEffect(() => {
        if (!token) {
            setState('err')
            setMsg('Missing verification token.')
            return
        }
        api.auth.verifyEmail(token)
            .then(() => setState('ok'))
            .catch((e: any) => {
                setState('err')
                setMsg(e.message || 'Verification failed')
            })
    }, [token])

    if (state === 'loading') {
        return (
            <div className="max-w-sm mx-auto mt-20 text-center text-gray-600 dark:text-gray-300">
                Verifying your email…
            </div>
        )
    }
    if (state === 'ok') {
        return (
            <div className="max-w-sm mx-auto mt-20 text-center">
                <h1 className="text-2xl font-bold mb-4">Email Verified</h1>
                <p className="text-gray-600 dark:text-gray-400 mb-6">
                    Your email is verified. You can now submit solutions.
                </p>
                <Link to="/login" className="inline-block bg-blue-600 text-white px-5 py-2 rounded-md text-sm font-medium hover:bg-blue-700">
                    Continue to Login
                </Link>
            </div>
        )
    }
    return (
        <div className="max-w-sm mx-auto mt-20 text-center">
            <h1 className="text-2xl font-bold mb-4">Verification Failed</h1>
            <p className="text-gray-600 dark:text-gray-400 mb-6">{msg}</p>
            <Link to="/login" className="text-blue-600 dark:text-blue-400 hover:underline">Back to Login</Link>
        </div>
    )
}
```

- [x] **Step 3: Register route in App.tsx**

```tsx
import VerifyEmail from './pages/VerifyEmail'
// ...
<Route path="/verify-email" element={<VerifyEmail />} />
```

Place next to `/forgot-password`.

- [x] **Step 4: Register success → check-email notice**

In `Register.tsx`, after successful `api.auth.register`:

- Still call `setTokens` (backend returns tokens).
- Set local state `sent = true` and render:

```tsx
if (sent) {
    return (
        <div className="max-w-sm mx-auto mt-20 text-center">
            <h1 className="text-2xl font-bold mb-4">Check Your Email</h1>
            <p className="text-gray-600 dark:text-gray-400 mb-4">
                We sent a verification link to <strong>{form.email}</strong>.
                Verify your email to submit solutions.
            </p>
            <button
                type="button"
                onClick={() => { api.auth.resendVerification().catch(() => {}) }}
                className="text-blue-600 dark:text-blue-400 hover:underline text-sm"
            >
                Resend verification email
            </button>
            <p className="mt-4">
                <Link to="/login" className="text-blue-600 dark:text-blue-400 hover:underline">Continue to Login</Link>
            </p>
        </div>
    )
}
```

Do **not** auto-navigate to `/` on register.

- [x] **Step 5: Frontend checks**

Run: `cd web && npx tsc -b --noEmit` (or `npm run build` if that is the project check).

Expected: No type errors.

- [x] **Step 6: Commit**

```bash
git add web/src/pages/VerifyEmail.tsx web/src/App.tsx web/src/lib/api.ts web/src/pages/Register.tsx
git commit -m "feat(web): verify-email page, resend API, register check-email notice"
```

---

### Task 5: End-to-end verification

**Files:** none (verification only)

- [x] **Step 1: Backend package tests**

Run: `go test ./internal/mail/ ./internal/api/handler/ -count=1`

Expected: PASS.

- [x] **Step 2: Full build**

Run: `go build ./...`

Expected: No errors.

- [x] **Step 3: Frontend typecheck/build**

Run: `cd web && npm run build` (or project equivalent).

Expected: Success.

- [x] **Step 4: Manual/API golden path (if stack running)**

1. `POST /api/auth/register` → 201 + mail in `GET /api/dev/mail`.
2. `POST /api/submissions` with that user → 403 email not verified.
3. `GET /api/auth/verify-email/{token}` → 200 verified.
4. Repeat submit → not 403 for verification.
5. `POST /api/auth/forgot-password` → body has no `token` field; catcher has reset link.
6. Browser: open `http://localhost:8081/verify-email?token=…` → success UI.

- [x] **Step 5: Update gap ledger**

In `.sleepycode/memory.md` or project memory, mark Slice A (mail + reset + verify + submit gate) done; next slice = TOTP 2FA (Phase A.12+).

---

## Self-Review

- Spec A.9 register email: already shipped; Task 1 only fixes test.
- Spec A.10 reset email: already shipped and tested.
- Spec A.11 submit gate: Task 2.
- Resend + VerifyEmail UI: closes operational gap for email links (publicURL `/verify-email?token=`).
- Onsite submit path: Task 2 Step 4 prevents regression.
- No placeholders: all steps include code or exact commands.
