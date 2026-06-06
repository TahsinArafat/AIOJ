# Auto-Detect Remote Languages Implementation Plan

## Overview
Add an "Auto-Detect" feature to the admin remote languages panel. When triggered, it scrapes language options from the remote OJ's submit page, auto-maps them to AIOJ's local languages, and presents results for admin review before bulk saving.

## Architecture

### Scraping Strategy
Each Python submit service (atcoder-submit, cf-submit) gets a new `GET /languages` endpoint that:
1. Loads the submit page using the existing CloakBrowser session
2. Extracts all `<option>` values from the language `<select>` element via JS evaluation
3. Returns JSON array of `{id, name}` pairs

Go clients (`atcoder_submit.go`, `cf_submit.go`) get `FetchLanguages(ctx) ([]RemoteLanguage, error)` methods that call the Python endpoint.

### Auto-Match Algorithm
Normalize remote language names to lowercase, then for each local language:
1. **Exact match**: remote name == local display_name (case-insensitive)
2. **Contains match**: local keywords appear in remote name (e.g., "c++" in "GNU G++17 7.3.0")
3. **No match**: local_id = null (admin assigns manually)

### Bulk Save
- Frontend shows review modal with all detected languages
- Admin confirms/unchecks languages
- Calls `POST /api/admin/remote-languages/bulk` with array of `{platform, local_id, remote_id, display_name}`
- Backend does `INSERT ... ON CONFLICT (platform, remote_id) DO UPDATE` (upsert by remote_id)
- Languages without a match (local_id = null) are excluded unless admin manually assigns

## Implementation Steps

### Phase 1: Python Service Endpoints

#### Step 1: Add `GET /languages` to atcoder-submit
- File: `deploy/atcoder-submit/server.py`
- Add new route that reuses CloakBrowser login, loads `https://atcoder.jp/contests/abc422/tasks/abc422_a` (any task page), extracts `select[name="data.LanguageId"]` options via JS eval (existing extraction logic at lines 263-270)
- Return `[{id: "6017", name: "C++23 (GCC 15.2.0)"}, ...]`

#### Step 2: Add `GET /languages` to cf-submit
- File: `deploy/cf-submit/server.py`
- Same pattern: load any CF problem submit page, extract `select[name="programTypeId"]` options
- Return `[{id: "48", name: "GNU G++17 7.3.0"}, ...]`

### Phase 2: Go Client Methods

#### Step 3: Add `FetchLanguages()` to atcoder-submit client
- File: `internal/vjudge/atcoder_submit.go`
- HTTP GET to `{baseURL}/languages`, parse JSON response
- Return `[]model.RemoteLanguage` with `RemoteID` and `DisplayName` populated

#### Step 4: Add `FetchLanguages()` to cf-submit client
- File: `internal/vjudge/cf_submit.go`
- Same pattern as Step 3

#### Step 5: Add `FetchLanguages()` to VJudge Service
- File: `internal/vjudge/service.go`
- Public method that delegates to the appropriate bot's submit client
- Signature: `FetchLanguages(ctx, platform string) ([]model.RemoteLanguage, error)`

### Phase 3: Backend Handler

#### Step 6: Add `AutoDetect` handler
- File: `internal/api/handler/remote_language.go`
- `POST /api/admin/remote-languages/detect/{platform}` — calls `service.FetchLanguages(platform)`, returns raw detected languages

#### Step 7: Add `BulkUpsert` handler
- File: `internal/api/handler/remote_language.go`
- `POST /api/admin/remote-languages/bulk` — accepts array of `{platform, local_id, remote_id, display_name}`, does upsert by `(platform, remote_id)`

#### Step 8: Add `BulkUpsert` store method
- File: `internal/store/postgres/remote_language_store.go`
- SQL: `INSERT INTO remote_languages (platform, local_id, remote_id, display_name, enabled, sort_order) VALUES ... ON CONFLICT (platform, remote_id) DO UPDATE SET local_id = EXCLUDED.local_id, display_name = EXCLUDED.display_name`

#### Step 9: Register new routes
- File: `internal/api/router.go`
- Add inside admin group: `r.Post("/remote-languages/detect/{platform}", remoteLangH.AutoDetect)` and `r.Post("/remote-languages/bulk", remoteLangH.BulkUpsert)`

### Phase 4: Frontend

#### Step 10: Add API client methods
- File: `web/src/lib/api.ts`
- `admin.remoteLanguages.detect(platform)` — POST, returns detected languages
- `admin.remoteLanguages.bulk(items)` — POST, saves bulk

#### Step 11: Update RemoteLanguagesPanel
- File: `web/src/pages/admin/RemoteLanguagesPanel.tsx`
- Add "Auto-Detect" button at top of language list
- On click: show loading spinner, call detect endpoint
- Display results in review table with checkboxes
- "Save" button calls bulk endpoint
- Languages with matches show green checkmark, unmatched show warning

### Phase 5: Verification

#### Step 12: End-to-end test
- Start atcoder-submit on host (port 8004)
- Click "Auto-Detect" for AtCoder platform
- Verify detected languages match AtCoder's 90+ languages
- Verify auto-match finds existing `c_cpp` → `C++23 (GCC 15.2.0)`
- Save and verify DB updated

## Risk Mitigation
- **CloakBrowser session expired**: Detect and re-login on language fetch
- **Submit page changed**: JS extraction fails gracefully with error message
- **Database constraint missing**: Add unique index on `(platform, remote_id)` if needed
- **Large language list**: AtCoder has 90+ languages — UI must handle scroll/search

## Success Criteria
1. Auto-Detect button fetches real language options from live remote OJ
2. Auto-match correctly maps C/C++/Java/Python variants
3. Admin can review and selectively save languages
4. Bulk save upserts correctly (no duplicates, updates existing)
5. Frontend problem pages show updated language list immediately after save
