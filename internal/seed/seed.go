package seed

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"

	// Registers the "postgres" driver for sql.Open below. The product imports
	// this in internal/store/postgres/conn.go, but cmd/seed is a separate
	// binary that does not go through that package — without this the seeder
	// dies with `unknown driver "postgres"`.
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// Config is every knob the sim needs. Every field is optional; fillDefaults
// never returns an empty one.
type Config struct {
	BackendURL  string // http://backend:8080
	AdminUser   string // "ai"
	AdminPass   string // "Aioj-Sim-Admin-2026!"
	ProblemSlug string // "hello"

	// AllowDefaultPass permits falling back to the documented demo password
	// when AdminPass is empty. Set it only for a development/simulation
	// environment: in production an explicit password is required, because the
	// default is published in this repository.
	AllowDefaultPass bool

	// ResetPassword makes the seeder authoritative for the admin's password:
	// when the account already exists but the configured password does not
	// match it, the stored hash is replaced (and must_change_password cleared)
	// instead of failing with 401.
	//
	// The zero value is false so a caller that constructs Config directly (the
	// unit tests, any future embedder) never gets an implicit password write.
	// Compose turns it on: docker-compose.dokploy.yml defaults
	// SEED_ADMIN_RESET_PASSWORD to true, making the env password authoritative
	// for the bootstrap admin. A matching password is still a no-op, so the
	// write only happens when the stored hash is stale. Set it to false to make
	// a password changed in the UI survive deploys instead.
	ResetPassword bool

	// DSN is used for exactly one write: promoting the first user to admin.
	// There is no bootstrap-admin endpoint in the product (PUT
	// /api/admin/users/{id}/role needs an existing admin), so the seeder does
	// this promotion directly. Every later promotion goes through the API.
	DSN string
}

// Result records what the seeder actually did, so a caller can assert idempotence.
type Result struct {
	AdminCreated bool `json:"admin_created"`
	// AdminPasswordReset reports that an existing admin's hash was replaced
	// with SEED_ADMIN_PASSWORD. Only ever true with ResetPassword enabled, and
	// reported separately from AdminCreated because "the account already
	// existed but its password changed" is exactly the case an operator needs
	// to see in the deploy log.
	AdminPasswordReset bool     `json:"admin_password_reset"`
	AdminPromoted      bool     `json:"admin_promoted"`
	ProblemCreated     bool     `json:"problem_created"`
	TestcaseCount      int      `json:"testcase_count"`
	Errors             []string `json:"errors,omitempty"`
}

const (
	defaultBackendURL  = "http://backend:8080"
	defaultAdminUser   = "ai"
	defaultAdminPass   = "Aioj-Sim-Admin-2026!"
	defaultProblemSlug = "hello"

	helloExpected = "Hello, AIOJ!"
)

type apiError struct {
	status int
	body   string
}

func (e apiError) Error() string {
	return fmt.Sprintf("api error %d: %s", e.status, e.body)
}

func fillDefaults(cfg Config) Config {
	if cfg.BackendURL == "" {
		cfg.BackendURL = defaultBackendURL
	}
	cfg.BackendURL = strings.TrimRight(cfg.BackendURL, "/")
	if cfg.AdminUser == "" {
		cfg.AdminUser = defaultAdminUser
	}
	if cfg.AdminPass == "" && cfg.AllowDefaultPass {
		cfg.AdminPass = defaultAdminPass
	}
	if cfg.ProblemSlug == "" {
		cfg.ProblemSlug = defaultProblemSlug
	}
	return cfg
}

// Seed is idempotent: running it against an already-seeded instance is a
// no-op that returns AdminCreated=false, ProblemCreated=false, exit 0.
func Seed(ctx context.Context, cfg Config) (Result, error) {
	cfg = fillDefaults(cfg)
	res := Result{}

	// The demo password is published in this repository, so it may only be used
	// where a public credential is acceptable. Anything else must supply one.
	if cfg.AdminPass == "" {
		return res, fmt.Errorf("seed: SEED_ADMIN_PASSWORD is required outside a " +
			"development environment (the built-in demo password is public)")
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return res, fmt.Errorf("seed: create cookie jar: %w", err)
	}
	client := &http.Client{Timeout: 30 * time.Second, Jar: jar}
	base := cfg.BackendURL

	// Prime the double-submit CSRF cookie before the first unauthenticated POST.
	// The API deliberately rejects writes without the cookie/header pair.
	if err := primeCSRF(ctx, client, base); err != nil {
		return res, err
	}

	// 1. Register the admin (or log back in if it already exists — the
	// idempotent path, not an error). With ResetPassword set, a mismatched
	// password on an existing account is repaired rather than reported as 401.
	token, created, reset, err := registerOrLogin(ctx, client, base, cfg)
	if err != nil {
		return res, err
	}
	res.AdminCreated = created
	res.AdminPasswordReset = reset

	// 2. Promote to admin directly in Postgres (the one DB write in this
	// harness), then re-login so the JWT carries role=admin.
	promoted, err := promoteAdmin(ctx, cfg)
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
		return res, err
	}
	res.AdminPromoted = promoted
	token, err = login(ctx, client, base, cfg.AdminUser, cfg.AdminPass)
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
		return res, err
	}

	// 3. Create the Hello problem.
	createdProblem, err := createProblem(ctx, client, base, token, cfg.ProblemSlug)
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
		return res, err
	}
	res.ProblemCreated = createdProblem

	// 4. Upload testcases (a zip with 1.in empty, 1.out "Hello, AIOJ!").
	n, err := uploadTestcases(ctx, client, base, token, cfg.ProblemSlug)
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
		return res, err
	}
	res.TestcaseCount = n

	return res, nil
}

// --- auth ----------------------------------------------------------------

func registerOrLogin(ctx context.Context, c *http.Client, base string, cfg Config) (token string, created, reset bool, err error) {
	user, pass := cfg.AdminUser, cfg.AdminPass

	// Try login first. This keeps seeding idempotent when the account already
	// exists (including databases created before the password policy changed),
	// and avoids making registration the first request on every rerun.
	if token, err = login(ctx, c, base, user, pass); err == nil {
		return token, false, false, nil
	}

	body := map[string]string{
		"username": user,
		"email":    user + "@aioj.test",
		"password": pass,
	}
	resp, err := postJSON(ctx, c, base+"/api/auth/register", body)
	if err != nil {
		return "", false, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		t, err := tokenFrom(resp)
		return t, resp.StatusCode == http.StatusCreated, false, err
	}
	if resp.StatusCode == http.StatusConflict {
		// The username is taken but the configured password did not work.
		// Repairing it requires an explicit opt-in; otherwise re-running the
		// seeder with a different SEED_ADMIN_PASSWORD would quietly rewrite
		// an operator's password on every deploy.
		if !cfg.ResetPassword {
			_, loginErr := login(ctx, c, base, user, pass)
			return "", false, false, fmt.Errorf("seed: user %q already exists with a different "+
				"password — set SEED_ADMIN_RESET_PASSWORD=true once to overwrite it "+
				"(got: %v)", user, loginErr)
		}
		if rerr := resetPassword(ctx, cfg); rerr != nil {
			return "", false, false, rerr
		}
		t, lerr := login(ctx, c, base, user, pass)
		return t, false, true, lerr
	}
	b, _ := io.ReadAll(resp.Body)
	return "", false, false, apiError{resp.StatusCode, string(b)}
}

func login(ctx context.Context, c *http.Client, base, user, pass string) (string, error) {
	body := map[string]string{"username": user, "password": pass}
	resp, err := postJSON(ctx, c, base+"/api/auth/login", body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", apiError{resp.StatusCode, string(b)}
	}
	return tokenFrom(resp)
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	User        struct {
		ID string `json:"id"`
	} `json:"user"`
}

func tokenFrom(resp *http.Response) (string, error) {
	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", err
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("no access_token in auth response")
	}
	return tr.AccessToken, nil
}

// --- admin promotion / password repair (the DB writes) ---------------------

// openDB connects using Config.DSN when set, otherwise the same DB_* env the
// server uses. Returns nil (no error) in the unit-test path where there is
// nothing to connect to.
func openDB(cfg Config) (*sql.DB, error) {
	dsn := cfg.DSN
	if dsn == "" {
		dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			envOr("DB_HOST", "postgres"), envOr("DB_PORT", "5432"),
			envOr("DB_USER", "aioj"), envOr("DB_PASSWORD", "aioj_secret"),
			envOr("DB_NAME", "aioj"))
	}
	return sql.Open("postgres", dsn)
}

// bcryptCost mirrors internal/auth.bcryptCost. A hash written here must be
// indistinguishable from one the API produced, otherwise a seed-created admin
// silently costs a different amount of time to verify than a registered user.
const bcryptCost = 12

// resetPassword makes the configured password authoritative for the admin.
// It only runs when ResetPassword is enabled and the stored hash did not match
// the configured password, so a matching password never triggers a write.
// It also clears must_change_password and sets role/email_verified, because a
// repaired bootstrap account must be usable immediately and must not be pinned
// to the profile page by the migration-000073 flag.
func resetPassword(ctx context.Context, cfg Config) error {
	db, err := openDB(cfg)
	if err != nil {
		return fmt.Errorf("seed: open db: %w", err)
	}
	defer db.Close()

	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPass), bcryptCost)
	if err != nil {
		return fmt.Errorf("seed: hash password: %w", err)
	}
	res, err := db.ExecContext(ctx,
		`UPDATE users
		    SET password_hash = $1,
		        role = 'admin',
		        must_change_password = FALSE,
		        email_verified = TRUE,
		        email_verified_at = COALESCE(email_verified_at, NOW())
		  WHERE username = $2`, string(hash), cfg.AdminUser)
	if err != nil {
		return fmt.Errorf("seed: reset password: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("seed: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("seed: reset password: user %q not found", cfg.AdminUser)
	}
	return nil
}

func promoteAdmin(ctx context.Context, cfg Config) (bool, error) {
	if cfg.DSN == "" && envOr("DB_HOST", "") == "" {
		// Nothing to connect to (unit-test path): assume already admin.
		return false, nil
	}
	db, err := openDB(cfg)
	if err != nil {
		return false, fmt.Errorf("seed: open db: %w", err)
	}
	defer db.Close()

	res, err := db.ExecContext(ctx,
		`UPDATE users
		    SET role = 'admin',
		        email_verified = TRUE,
		        email_verified_at = COALESCE(email_verified_at, NOW())
		  WHERE username = $1`, cfg.AdminUser)
	if err != nil {
		return false, fmt.Errorf("seed: promote admin: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("seed: rows affected: %w", err)
	}
	return n > 0, nil
}

// --- problem + testcases ---------------------------------------------------

func createProblem(ctx context.Context, c *http.Client, base, token, slug string) (bool, error) {
	body := map[string]interface{}{
		"slug":          slug,
		"title":         "Hello, AIOJ!",
		"description":   "Print `" + helloExpected + "`.",
		"input_format":  "No input.",
		"output_format": "The string `" + helloExpected + "`.",
		"time_limit":    1000,
		"memory_limit":  262144,
		"difficulty":    "easy",
		"checker_type":  "exact",
		"visible":       true,
		"sample_cases":  []map[string]string{{"input": "", "output": helloExpected}},
	}
	resp, err := postJSON(ctx, c, base+"/api/problems", body, withToken(token))
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusCreated:
		return true, nil
	case resp.StatusCode == http.StatusConflict:
		return false, nil // already exists — the idempotent path
	}
	b, _ := io.ReadAll(resp.Body)
	return false, apiError{resp.StatusCode, string(b)}
}

func uploadTestcases(ctx context.Context, c *http.Client, base, token, slug string) (int, error) {
	zipBytes, err := buildHelloTestcases()
	if err != nil {
		return 0, err
	}

	// The handler reads the testcases from multipart field "file" and extracts
	// a .zip payload into the problem's testdata dir.
	var form bytes.Buffer
	mw := multipart.NewWriter(&form)
	fw, err := mw.CreateFormFile("file", "testcases.zip")
	if err != nil {
		return 0, err
	}
	if _, err := fw.Write(zipBytes); err != nil {
		return 0, err
	}
	if err := mw.Close(); err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		base+"/api/problems/"+slug+"/testcases", &form)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := c.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return 0, apiError{resp.StatusCode, string(b)}
	}
	return 1, nil
}

// buildHelloTestcases builds the testcase zip in memory: 1.in (empty) and
// 1.out ("Hello, AIOJ!"). Pure function — no network, no filesystem.
func buildHelloTestcases() ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for name, content := range map[string]string{"1.in": "", "1.out": helloExpected} {
		w, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(content)); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// envOr reads an environment variable or returns the fallback.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// --- http helpers ---------------------------------------------------------

type requestOption func(*http.Request)

func withToken(token string) requestOption {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token) }
}

func primeCSRF(ctx context.Context, c *http.Client, base string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/auth/csrf", nil)
	if err != nil {
		return fmt.Errorf("seed: build csrf request: %w", err)
	}
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("seed: prime csrf: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("seed: prime csrf: api error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func postJSON(ctx context.Context, c *http.Client, u string, body interface{}, opts ...requestOption) (*http.Response, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for _, opt := range opts {
		opt(req)
	}
	// Unauthenticated API writes require the double-submit pair. The cookie is
	// retained by the client jar; echo its value in the header.
	if req.Header.Get("Authorization") == "" && c.Jar != nil {
		if u, err := url.Parse(req.URL.String()); err == nil {
			for _, cookie := range c.Jar.Cookies(u) {
				if cookie.Name == "csrf" && cookie.Value != "" {
					req.Header.Set("X-CSRF-Token", cookie.Value)
					break
				}
			}
		}
	}
	return c.Do(req)
}
