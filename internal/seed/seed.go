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
	"os"
	"strings"
	"time"

	// Registers the "postgres" driver for sql.Open below. The product imports
	// this in internal/store/postgres/conn.go, but cmd/seed is a separate
	// binary that does not go through that package — without this the seeder
	// dies with `unknown driver "postgres"`.
	_ "github.com/lib/pq"
)

// Config is every knob the sim needs. Every field is optional; fillDefaults
// never returns an empty one.
type Config struct {
	BackendURL  string // http://backend:8080
	AdminUser   string // "ai"
	AdminPass   string // "aiseedpass"
	ProblemSlug string // "hello"

	// DSN is used for exactly one write: promoting the first user to admin.
	// There is no bootstrap-admin endpoint in the product (PUT
	// /api/admin/users/{id}/role needs an existing admin), so the seeder does
	// this promotion directly. Every later promotion goes through the API.
	DSN string
}

// Result records what the seeder actually did, so a caller can assert idempotence.
type Result struct {
	AdminCreated   bool     `json:"admin_created"`
	AdminPromoted  bool     `json:"admin_promoted"`
	ProblemCreated bool     `json:"problem_created"`
	TestcaseCount  int      `json:"testcase_count"`
	Errors         []string `json:"errors,omitempty"`
}

const (
	defaultBackendURL  = "http://backend:8080"
	defaultAdminUser   = "ai"
	defaultAdminPass   = "aiseedpass"
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
	if cfg.AdminPass == "" {
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

	client := &http.Client{Timeout: 30 * time.Second}
	base := cfg.BackendURL

	// 1. Register the admin (or log back in if it already exists — the
	// idempotent path, not an error).
	token, created, err := registerOrLogin(ctx, client, base, cfg.AdminUser, cfg.AdminPass)
	if err != nil {
		return res, err
	}
	res.AdminCreated = created

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

func registerOrLogin(ctx context.Context, c *http.Client, base, user, pass string) (token string, created bool, err error) {
	body := map[string]string{
		"username": user,
		"email":    user + "@aioj.test",
		"password": pass,
	}
	resp, err := postJSON(ctx, c, base+"/api/auth/register", body)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		t, err := tokenFrom(resp)
		return t, resp.StatusCode == http.StatusCreated, err
	}
	if resp.StatusCode == http.StatusConflict {
		t, err := login(ctx, c, base, user, pass)
		return t, false, err
	}
	b, _ := io.ReadAll(resp.Body)
	return "", false, apiError{resp.StatusCode, string(b)}
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

// --- admin promotion (the single DB write) --------------------------------

func promoteAdmin(ctx context.Context, cfg Config) (bool, error) {
	dsn := cfg.DSN
	if dsn == "" {
		dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			envOr("DB_HOST", "postgres"), envOr("DB_PORT", "5432"),
			envOr("DB_USER", "aioj"), envOr("DB_PASSWORD", "aioj_secret"),
			envOr("DB_NAME", "aioj"))
	}
	if dsn == "" {
		// Nothing to connect to (unit-test path): assume already admin.
		return false, nil
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return false, fmt.Errorf("seed: open db: %w", err)
	}
	defer db.Close()

	res, err := db.ExecContext(ctx,
		`UPDATE users SET role = 'admin' WHERE username = $1 AND role <> 'admin'`,
		cfg.AdminUser)
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
		"slug":           slug,
		"title":          "Hello, AIOJ!",
		"description":    "Print `" + helloExpected + "`.",
		"input_format":   "No input.",
		"output_format":  "The string `" + helloExpected + "`.",
		"time_limit":     1000,
		"memory_limit":   262144,
		"difficulty":     "easy",
		"checker_type":   "exact",
		"visible":        true,
		"sample_cases":   []map[string]string{{"input": "", "output": helloExpected}},
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
	return c.Do(req)
}
