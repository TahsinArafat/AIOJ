# Sub-Plan 17: Public API

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create a documented, rate-limited public API for third-party integrations.

**Architecture:** Add API key authentication, documentation endpoints, OpenAPI spec generation.

**Tech Stack:** Go, PostgreSQL, OpenAPI/Swagger

---

## File Structure

### Backend Files to Create
- `internal/api/public/` - Public API handlers
- `internal/api/public/router.go` - Public API router
- `internal/api/public/docs.go` - API documentation
- `internal/model/apikey.go` - API key models
- `internal/store/postgres/apikeys.go` - API key store

### Backend Files to Modify
- `cmd/aioj/main.go` - Add public API server
- `internal/store/interfaces.go` - Add APIKeyStore interface

---

## Tasks

### Task 1: Database Migration

**Files:**
- Create: `internal/store/migrations/000014_api_keys.up.sql`
- Create: `internal/store/migrations/000014_api_keys.down.sql`

- [ ] **Step 1: Create migration up file**

```sql
-- internal/store/migrations/000014_api_keys.up.sql

CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(128) NOT NULL DEFAULT 'Default',
    description TEXT NOT NULL DEFAULT '',
    rate_limit INTEGER NOT NULL DEFAULT 100,
    is_active BOOLEAN NOT NULL DEFAULT true,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_api_keys_user ON api_keys(user_id);
CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);

-- Rate limiting table
CREATE TABLE api_rate_limits (
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    window_start TIMESTAMPTZ NOT NULL,
    request_count INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (api_key_id, window_start)
);
```

- [ ] **Step 2: Create migration down file**

```sql
-- internal/store/migrations/000014_api_keys.down.sql

DROP TABLE IF EXISTS api_rate_limits;
DROP TABLE IF EXISTS api_keys;
```

- [ ] **Step 3: Run migration**

Run: `make migrate-up`

- [ ] **Step 4: Commit**

```bash
git add internal/store/migrations/000014_api_keys.*
git commit -m "feat(api): add API keys database migration"
```

---

### Task 2: API Key Models and Store

**Files:**
- Create: `internal/model/apikey.go`
- Create: `internal/store/postgres/apikeys.go`
- Modify: `internal/store/interfaces.go`

- [ ] **Step 1: Create API key models**

```go
// internal/model/apikey.go
package model

import "time"

type APIKey struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	KeyHash     string     `json:"-"`
	KeyPreview  string     `json:"key_preview"` // First 8 chars + "..."
	Name        string     `json:"name"`
	Description string     `json:"description"`
	RateLimit   int        `json:"rate_limit"`
	IsActive    bool       `json:"is_active"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type CreateAPIKeyRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	RateLimit   int    `json:"rate_limit,omitempty"`
}

type CreateAPIKeyResponse struct {
	APIKey *APIKey `json:"api_key"`
	Secret string  `json:"secret"` // Only shown once
}
```

- [ ] **Step 2: Add APIKeyStore interface**

```go
type APIKeyStore interface {
	Create(ctx context.Context, k *model.APIKey) error
	GetByHash(ctx context.Context, keyHash string) (*model.APIKey, error)
	GetByUser(ctx context.Context, userID string) ([]model.APIKey, error)
	Update(ctx context.Context, id string, k *model.APIKey) error
	Delete(ctx context.Context, id string) error
	UpdateLastUsed(ctx context.Context, id string) error
	
	// Rate limiting
	IncrementRequestCount(ctx context.Context, keyID string, windowStart time.Time) error
	GetRequestCount(ctx context.Context, keyID string, windowStart time.Time) (int, error)
}
```

- [ ] **Step 3: Implement API key store**

```go
// internal/store/postgres/apikeys.go
package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
)

type APIKeyStore struct {
	db *sql.DB
}

func NewAPIKeyStore(db *sql.DB) *APIKeyStore {
	return &APIKeyStore{db: db}
}

func GenerateAPIKey() (key, hash string, err error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}
	key = "aioj_" + hex.EncodeToString(bytes)
	h := sha256.Sum256([]byte(key))
	hash = hex.EncodeToString(h[:])
	return key, hash, nil
}

func (s *APIKeyStore) Create(ctx context.Context, k *model.APIKey) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO api_keys (user_id, key_hash, name, description, rate_limit) 
		 VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`,
		k.UserID, k.KeyHash, k.Name, k.Description, k.RateLimit,
	).Scan(&k.ID, &k.CreatedAt)
}

func (s *APIKeyStore) GetByHash(ctx context.Context, keyHash string) (*model.APIKey, error) {
	var k model.APIKey
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, key_hash, name, description, rate_limit, is_active, last_used_at, created_at
		 FROM api_keys WHERE key_hash = $1 AND is_active = true`,
		keyHash).Scan(&k.ID, &k.UserID, &k.KeyHash, &k.Name, &k.Description,
		&k.RateLimit, &k.IsActive, &k.LastUsedAt, &k.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

func (s *APIKeyStore) GetByUser(ctx context.Context, userID string) ([]model.APIKey, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, key_hash, name, description, rate_limit, is_active, last_used_at, created_at
		 FROM api_keys WHERE user_id = $1 ORDER BY created_at`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var keys []model.APIKey
	for rows.Next() {
		var k model.APIKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.KeyHash, &k.Name, &k.Description,
			&k.RateLimit, &k.IsActive, &k.LastUsedAt, &k.CreatedAt); err != nil {
			return nil, err
		}
		k.KeyPreview = k.KeyHash[:8] + "..."
		keys = append(keys, k)
	}
	if keys == nil {
		keys = []model.APIKey{}
	}
	return keys, nil
}

func (s *APIKeyStore) IncrementRequestCount(ctx context.Context, keyID string, windowStart time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO api_rate_limits (api_key_id, window_start, request_count) 
		 VALUES ($1, $2, 1) ON CONFLICT (api_key_id, window_start) 
		 DO UPDATE SET request_count = request_count + 1`,
		keyID, windowStart)
	return err
}

func (s *APIKeyStore) GetRequestCount(ctx context.Context, keyID string, windowStart time.Time) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(request_count), 0) FROM api_rate_limits WHERE api_key_id = $1 AND window_start = $2",
		keyID, windowStart).Scan(&count)
	return count, err
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/model/apikey.go internal/store/interfaces.go internal/store/postgres/apikeys.go
git commit -m "feat(api): add API key models and store"
```

---

### Task 3: Public API Router

**Files:**
- Create: `internal/api/public/router.go`
- Create: `internal/api/public/handlers.go`
- Modify: `cmd/aioj/main.go`

- [ ] **Step 1: Create public API router**

```go
// internal/api/public/router.go
package public

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/tahsinarafat/aioj/internal/store"
)

type PublicRouter struct {
	problemStore   store.ProblemStore
	submissionStore store.SubmissionStore
	contestStore   store.ContestStore
	apiKeyStore    store.APIKeyStore
}

func NewRouter(ps store.ProblemStore, ss store.SubmissionStore, cs store.ContestStore, ks store.APIKeyStore) http.Handler {
	pr := &PublicRouter{
		problemStore:   ps,
		submissionStore: ss,
		contestStore:   cs,
		apiKeyStore:    ks,
	}
	
	r := chi.NewRouter()
	r.Use(chiMiddleware.RequestID, chiMiddleware.RealIP, chiMiddleware.Logger, chiMiddleware.Recoverer)
	r.Use(corsMiddleware)
	
	// Health check
	r.Get("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, 200, map[string]string{"status": "ok"})
	})
	
	// Public endpoints (no auth required)
	r.Route("/api/v1", func(r chi.Router) {
		// Problems
		r.Get("/problems", pr.ListProblems)
		r.Get("/problems/{slug}", pr.GetProblem)
		
		// Contests
		r.Get("/contests", pr.ListContests)
		r.Get("/contests/{id}", pr.GetContest)
		r.Get("/contests/{id}/standings", pr.GetContestStandings)
		
		// Users
		r.Get("/users/{username}", pr.GetUser)
		r.Get("/users/{username}/rating", pr.GetUserRating)
	})
	
	// Authenticated endpoints
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(pr.apiKeyAuth)
		
		// Submissions
		r.Post("/submissions", pr.SubmitSolution)
		r.Get("/submissions/{id}", pr.GetSubmission)
	})
	
	return r
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 2: Create public API handlers**

```go
// internal/api/public/handlers.go
package public

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

func (pr *PublicRouter) ListProblems(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	
	problems, total, _ := pr.problemStore.List(r.Context(), offset, limit)
	respondJSON(w, 200, map[string]interface{}{
		"problems": problems,
		"total":    total,
		"offset":   offset,
		"limit":    limit,
	})
}

func (pr *PublicRouter) GetProblem(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	problem, err := pr.problemStore.GetBySlug(r.Context(), slug)
	if err != nil || problem == nil {
		http.Error(w, "not found", 404)
		return
	}
	
	// Return public problem data (without test cases)
	respondJSON(w, 200, map[string]interface{}{
		"id":           problem.ID,
		"slug":         problem.Slug,
		"title":        problem.Title,
		"description":  problem.Description,
		"input_format": problem.InputFormat,
		"output_format": problem.OutputFormat,
		"time_limit":   problem.TimeLimit,
		"memory_limit": problem.MemoryLimit,
		"difficulty":   problem.Difficulty,
		"tags":         problem.Tags,
		"source":       problem.Source,
	})
}

func (pr *PublicRouter) GetUser(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	user, err := pr.userStore.GetByUsername(r.Context(), username)
	if err != nil || user == nil {
		http.Error(w, "not found", 404)
		return
	}
	
	// Return public user data
	respondJSON(w, 200, map[string]interface{}{
		"username":   user.Username,
		"created_at": user.CreatedAt,
	})
}

func (pr *PublicRouter) SubmitSolution(w http.ResponseWriter, r *http.Request) {
	// Get API key from context
	apiKey := getAPIKeyFromContext(r.Context())
	if apiKey == nil {
		http.Error(w, "unauthorized", 401)
		return
	}
	
	var req struct {
		ProblemSlug string `json:"problem_slug"`
		Language    string `json:"language"`
		SourceCode  string `json:"source_code"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", 400)
		return
	}
	
	// Create submission
	// ... implementation ...
	
	respondJSON(w, 201, map[string]string{"status": "queued"})
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
```

- [ ] **Step 3: Add public API server to main**

Add to `cmd/aioj/main.go`:

```go
// Start public API server on separate port
if os.Getenv("PUBLIC_API_PORT") != "" {
    publicRouter := public.NewRouter(problemStore, submissionStore, contestStore, apiKeyStore)
    go func() {
        port := os.Getenv("PUBLIC_API_PORT")
        log.Printf("Public API listening on :%s", port)
        http.ListenAndServe(":"+port, publicRouter)
    }()
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/api/public/ cmd/aioj/main.go
git commit -m "feat(api): add public API router and handlers"
```

---

### Task 4: API Documentation

**Files:**
- Create: `docs/api/openapi.yaml`

- [ ] **Step 1: Create OpenAPI spec**

```yaml
# docs/api/openapi.yaml
openapi: 3.0.0
info:
  title: AIOJ Public API
  description: API for competitive programming platform
  version: 1.0.0
  contact:
    name: AIOJ Support
    email: support@aioj.net

servers:
  - url: http://localhost:8081/api/v1
    description: Local development

paths:
  /health:
    get:
      summary: Health check
      responses:
        '200':
          description: Service is healthy
          
  /problems:
    get:
      summary: List problems
      parameters:
        - name: offset
          in: query
          schema:
            type: integer
            default: 0
        - name: limit
          in: query
          schema:
            type: integer
            default: 20
            maximum: 100
      responses:
        '200':
          description: List of problems
          
  /problems/{slug}:
    get:
      summary: Get problem by slug
      parameters:
        - name: slug
          in: path
          required: true
          schema:
            type: string
      responses:
        '200':
          description: Problem details
        '404':
          description: Problem not found

components:
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: header
      name: X-API-Key
```

- [ ] **Step 2: Add Swagger UI endpoint**

Add to public router:

```go
r.Get("/api/v1/docs", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte(swaggerUIHTML))
})
```

- [ ] **Step 3: Commit**

```bash
git add docs/api/openapi.yaml
git commit -m "feat(api): add OpenAPI documentation"
```

---

### Task 5: Frontend API Key Management

**Files:**
- Create: `web/src/pages/APISettings.tsx`
- Modify: `web/src/App.tsx`
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Add API key management API calls**

```typescript
apiKeys: {
    list: () => request<{ data: any[] }>('/api-keys'),
    create: (data: { name: string; description?: string }) =>
        request<{ api_key: any; secret: string }>('/api-keys', { method: 'POST', body: JSON.stringify(data) }),
    delete: (id: string) => request(`/api-keys/${id}`, { method: 'DELETE' }),
},
```

- [ ] **Step 2: Create API settings page**

```tsx
// web/src/pages/APISettings.tsx
import { useState, useEffect } from 'react';
import { api } from '../lib/api';

export default function APISettings() {
  const [keys, setKeys] = useState<any[]>([]);
  const [newKey, setNewKey] = useState<any>(null);
  const [name, setName] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.apiKeys.list().then(d => setKeys(d.data || [])).finally(() => setLoading(false));
  }, []);

  const handleCreate = async () => {
    if (!name.trim()) return;
    const result = await api.apiKeys.create({ name });
    setNewKey(result);
    setKeys([...keys, result.api_key]);
    setName('');
  };

  const handleDelete = async (id: string) => {
    await api.apiKeys.delete(id);
    setKeys(keys.filter(k => k.id !== id));
  };

  return (
    <div className="max-w-2xl mx-auto">
      <h1 className="text-2xl font-bold mb-6">API Keys</h1>

      {/* Create New Key */}
      <div className="border rounded-lg p-4 mb-6">
        <h2 className="font-semibold mb-3">Create New Key</h2>
        <div className="flex gap-2">
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Key name"
            className="flex-1 border rounded px-3 py-2"
          />
          <button
            onClick={handleCreate}
            disabled={!name.trim()}
            className="bg-blue-600 text-white px-4 py-2 rounded disabled:opacity-50"
          >
            Create
          </button>
        </div>

        {newKey && (
          <div className="mt-4 p-3 bg-yellow-50 border border-yellow-200 rounded">
            <p className="text-sm font-medium text-yellow-800 mb-2">
              ⚠️ Save this secret - it won't be shown again!
            </p>
            <code className="block bg-white p-2 rounded text-sm break-all">
              {newKey.secret}
            </code>
          </div>
        )}
      </div>

      {/* Existing Keys */}
      <div>
        <h2 className="font-semibold mb-3">Your API Keys</h2>
        {loading ? (
          <p>Loading...</p>
        ) : keys.length === 0 ? (
          <p className="text-gray-500">No API keys yet.</p>
        ) : (
          <div className="space-y-2">
            {keys.map(k => (
              <div key={k.id} className="flex items-center justify-between border rounded p-3">
                <div>
                  <p className="font-medium">{k.name}</p>
                  <p className="text-sm text-gray-500 font-mono">{k.key_preview}</p>
                </div>
                <button
                  onClick={() => handleDelete(k.id)}
                  className="text-red-600 hover:text-red-800 text-sm"
                >
                  Delete
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Add route**

```tsx
<Route path="/settings/api" element={<APISettings />} />
```

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/APISettings.tsx web/src/App.tsx web/src/lib/api.ts
git commit -m "feat(api): add API key management UI"
```

---

## Verification Checklist

- [ ] API keys can be created
- [ ] API key secret shown once
- [ ] Public endpoints work without auth
- [ ] Authenticated endpoints require API key
- [ ] Rate limiting works
- [ ] OpenAPI docs accessible

---

## Notes

1. **API Key Format**: `aioj_` prefix + 64 hex chars
2. **Rate Limit**: Default 100 requests per hour
3. **Documentation**: OpenAPI 3.0 spec
4. **CORS**: Enabled for all origins (configurable)
