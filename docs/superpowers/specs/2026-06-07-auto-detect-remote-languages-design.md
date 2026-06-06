# Auto-Detect Remote Languages

## Summary

Add an "Auto-Detect" feature to the admin panel that scrapes language options from remote OJ submit pages (AtCoder, Codeforces) and auto-maps them to AIOJ's local languages. Admin reviews and confirms before saving.

## Motivation

Currently, adding remote language mappings requires manually entering each language's `remote_id`, `display_name`, and `local_id`. This is tedious and error-prone, especially when platforms have 50+ languages. The submit services already parse language `<select>` elements for submission — we reuse this infrastructure for detection.

## Scope

**v1**: AtCoder + Codeforces  
**Future**: CSES, Toph, QOJ (framework designed to support all)

## Architecture

```
Admin clicks "Auto-Detect" per platform tab
        │
        ▼
POST /api/admin/remote-languages/detect/{platform}
        │
        ▼
Go handler fetches bot account credentials
        │
        ▼
Go client calls Python service → GET /languages
        │
        ▼
Python service logs in via CloakBrowser,
navigates to submit page, reads <select> options
        │
        ▼
Returns [{id, name}, ...] to Go backend
        │
        ▼
Go backend auto-matches by name to local languages
        │
        ▼
Returns matched results to frontend
        │
        ▼
Admin reviews in modal, toggles/edits, clicks "Save"
        │
        ▼
POST /api/admin/remote-languages/bulk → bulk upsert
```

## Component Design

### 1. Python Service: `GET /languages`

#### atcoder-submit (`deploy/atcoder-submit/server.py`)

New endpoint `GET /languages`:
- Logs in using existing CloakBrowser profile session
- Navigates to `https://atcoder.jp/contests/{test_contest}/submit?taskScreenName=...`
  - Uses a known past contest (e.g., `language-test-202505`) that has the full language list
- Reads `select[name="data.LanguageId"]` options via JS:
  ```javascript
  Array.from(select.options).map(o => ({id: o.value, name: o.textContent.trim()}))
  ```
- Returns `{"languages": [{"id": "6017", "name": "C++23 (GCC 15.2.0)"}, ...]}`
- Logs out

#### cf-submit (`deploy/cf-submit/server.py`)

New endpoint `GET /languages`:
- Logs in using existing CloakBrowser profile session
- Navigates to `https://codeforces.com/problemset/submit`
- Reads `select[name="programTypeId"]` options via JS
- Returns `{"languages": [{"id": "54", "name": "GNU G++17 7.3.0 (64 bit)"}, ...]}`
- Logs out

### 2. Go Clients

#### `internal/vjudge/atcoder_submit.go`

New method:
```go
type LanguageOption struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

func (c *AtCoderSubmitClient) FetchLanguages(ctx context.Context) ([]LanguageOption, error)
```
- `GET {baseURL}/languages`
- Parses `{"languages": [...]}` response

#### `internal/vjudge/cf_submit.go`

New method:
```go
func (c *CFSubmitClient) FetchLanguages(ctx context.Context) ([]LanguageOption, error)
```
- `GET {baseURL}/languages`
- Same pattern

### 3. Handler: `AutoDetect`

#### `internal/api/handler/remote_language.go`

New method `AutoDetect(w, r)`:
1. Extract `platform` from URL param
2. Query `bot_accounts` table for an active bot on this platform
3. If no bot found → return 400 "No active bot account for {platform}"
4. Call the appropriate client's `FetchLanguages(ctx)`
5. If service unavailable → return 502 "{platform} submit service unavailable"
6. Load all local languages from `language_limits` table
7. Auto-match remote → local by name similarity:
   - **Exact match**: normalized name equals normalized local lang name
   - **Contains match**: remote name contains local lang keyword (e.g., "C++" in "GNU G++17")
   - **No match**: `local_id` is null
8. Load existing `remote_languages` for this platform to show current mappings
9. Return:
```json
{
  "matches": [
    {
      "remote_id": "6017",
      "remote_name": "C++23 (GCC 15.2.0)",
      "local_id": "cpp-gpp-64",
      "confidence": "high",
      "existing": false
    }
  ]
}
```

#### Name Matching Algorithm

```
normalize(s) = strings.ToLower(strings.TrimSpace(s))

For each remote language:
  For each local language:
    if normalize(remote_name) == normalize(local_name) → confidence: "exact"
    if strings.Contains(normalize(remote_name), normalize(local_keyword)) → confidence: "high"
    if any keyword in remote_name matches local_keyword → confidence: "medium"
  If no match → confidence: "none", local_id: null

Local language keywords (from language config):
  cpp-gpp-64: ["c++", "g++", "gcc"]
  cpp-gpp-32: ["c++", "g++", "gcc"]
  c-gcc-64: ["c", "gcc"]
  python: ["python", "pypy"]
  java: ["java"]
  rust: ["rust", "rustc"]
  nodejs: ["javascript", "node"]
  csharp: ["c#", "csharp", ".net"]
  go: ["go", "golang"]
  haskell: ["haskell", "ghc"]
  ruby: ["ruby"]
  scala: ["scala"]
  kotlin: ["kotlin"]
  swift: ["swift"]
  php: ["php"]
  perl: ["perl"]
  lua: ["lua"]
```

### 4. Store: `BulkUpsert`

#### `internal/store/postgres/remote_language_store.go`

New method:
```go
func (s *RemoteLanguageStore) BulkUpsert(ctx, platform string, langs []model.RemoteLanguage) error
```

SQL:
```sql
INSERT INTO remote_languages (id, platform, local_id, remote_id, display_name, enabled, sort_order, inline_comment_prefix)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (platform, local_id)
DO UPDATE SET
  remote_id = EXCLUDED.remote_id,
  display_name = EXCLUDED.display_name,
  enabled = EXCLUDED.enabled,
  sort_order = EXCLUDED.sort_order,
  inline_comment_prefix = EXCLUDED.inline_comment_prefix
```

Interface addition in `store/interfaces.go`:
```go
BulkUpsert(ctx context.Context, platform string, langs []model.RemoteLanguage) error
```

### 5. Routes

In `router.go` under admin group:
```go
r.Post("/remote-languages/detect/{platform}", remoteLangH.AutoDetect)
r.Post("/remote-languages/bulk", remoteLangH.BulkUpsert)
```

### 6. Frontend

#### `web/src/lib/api.ts`

Add to `admin.remoteLanguages`:
```typescript
detect: (platform: string) =>
    request<{ matches: any[] }>(`/admin/remote-languages/detect/${platform}`, { method: 'POST' }),
bulkUpsert: (d: { platform: string; languages: { local_id: string; remote_id: string; display_name: string; enabled: boolean; sort_order: number; inline_comment_prefix: string }[] }) =>
    request<any>('/admin/remote-languages/bulk', { method: 'POST', body: JSON.stringify(d) }),
```

#### `web/src/pages/admin/RemoteLanguagesPanel.tsx`

New UI elements:
1. **"Auto-Detect" button** next to platform tabs — shows loading spinner while scraping
2. **Review modal/dialog** showing detected languages in a table:
   - Columns: Remote Name, Remote ID, Matched Local ID (dropdown), Enabled, Sort Order, Comment Prefix
   - Each row has a checkbox to include/exclude
   - Unmatched languages show empty local_id with a dropdown to select manually
3. **"Save All" button** in modal → calls bulk upsert, closes modal, refreshes list

## Error Handling

| Error | Response | UX |
|-------|----------|-----|
| No bot account for platform | 400 "No active bot account" | Toast error |
| Submit service unreachable | 502 "Submit service unavailable" | Toast error with hint to check service |
| Login failed | 502 "Login failed — check bot credentials" | Toast error |
| Submit page not found | 502 "Could not load submit page" | Toast error |
| Partial language load | 200 with partial results | Show what was found, warn about errors |

## Files Changed

### New files
- None

### Modified files
| File | Change |
|------|--------|
| `deploy/atcoder-submit/server.py` | Add `GET /languages` endpoint |
| `deploy/cf-submit/server.py` | Add `GET /languages` endpoint |
| `internal/vjudge/atcoder_submit.go` | Add `FetchLanguages()` method + `LanguageOption` type |
| `internal/vjudge/cf_submit.go` | Add `FetchLanguages()` method |
| `internal/api/handler/remote_language.go` | Add `AutoDetect()` and `BulkUpsert()` handlers + name matching logic |
| `internal/api/router.go` | Add 2 new admin routes |
| `internal/store/interfaces.go` | Add `BulkUpsert` to `RemoteLanguageStore` interface |
| `internal/store/postgres/remote_language_store.go` | Implement `BulkUpsert` |
| `web/src/lib/api.ts` | Add `detect()` and `bulkUpsert()` API methods |
| `web/src/pages/admin/RemoteLanguagesPanel.tsx` | Add Auto-Detect button, review modal, bulk save |

## Testing

1. **Python services**: Test `GET /languages` returns valid JSON with language array
2. **Go clients**: Test `FetchLanguages` parses response correctly
3. **Handler**: Test `AutoDetect` with mocked client returns matched results
4. **Name matching**: Unit tests for normalize + match algorithm
5. **Bulk upsert**: Test INSERT ... ON CONFLICT works correctly
6. **Frontend**: Manual test — click Auto-Detect, review modal shows languages, save works
7. **E2E**: Full flow from button click to `remote_languages` table populated
