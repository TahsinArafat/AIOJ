# Custom Rating and Rankings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Modify the rating calculation to show unrated users as 0, calculate their first contest rating using a 1500 baseline, hide admin accounts from rankings lists, and rename rating color tiers to custom, non-Codeforces names.

**Architecture:** 
1. Fix rating initialization in the contest handler so that unrated users start with `0` visible rating, while the calculator uses `1500` internally for their first calculation.
2. Filter rankings queries in PostgreSQL `UserStore` to exclude users with `role = 'admin'`.
3. Modify the profile loading logic to dynamically auto-create user profile rows in the DB to avoid 404 errors.
4. Rename rating tier constants, color strings, helper functions, labels, and graphs in both frontend and backend to use custom names (Novice, Apprentice, Adept, Elite, Champion, Master, Grandmaster, Titan, Immortal, Apex).

**Tech Stack:** Go (monolith, database/sql), React, TypeScript.

---

### Task 1: Rename Rating Color Tiers (Go Backend)

**Files:**
- Modify: `internal/model/rating.go`
- Modify: `internal/model/model_test.go`

- [ ] **Step 1: Rename constants and update GetColor in `internal/model/rating.go`**
  Modify `/Users/tahsinarafat/App_Dev/AIOJ/internal/model/rating.go` to declare custom names:
  ```go
  const (
  	ColorNovice      = "novice"       // < 1200
  	ColorApprentice  = "apprentice"   // 1200-1399
  	ColorAdept       = "adept"        // 1400-1599
  	ColorElite       = "elite"        // 1600-1899
  	ColorChampion    = "champion"     // 1900-2099
  	ColorMaster      = "master"       // 2100-2299
  	ColorGrandmaster = "grandmaster"  // 2300-2399
  	ColorTitan       = "titan"        // 2400-2599
  	ColorImmortal    = "immortal"     // 2600-2899
  	ColorApex        = "apex"         // 2900+
  )
  ```
  Update `GetColor(rating int) string` to use these constants.

- [ ] **Step 2: Update model tests in `internal/model/model_test.go`**
  Modify `/Users/tahsinarafat/App_Dev/AIOJ/internal/model/model_test.go` to match the new constant names and test case expectations.

- [ ] **Step 3: Run model tests to verify they pass**
  Run: `go test -v ./internal/model/...`
  Expected: PASS

- [ ] **Step 4: Commit**
  ```bash
  git add internal/model/rating.go internal/model/model_test.go
  git commit -m "feat: rename rating color tiers to custom names in backend"
  ```

---

### Task 2: Rename Rating Color Tiers (React Frontend)

**Files:**
- Modify: `web/src/lib/rating.ts`
- Modify: `web/src/components/RatingGraph.tsx`
- Modify: `web/src/pages/ProblemList.tsx`

- [ ] **Step 1: Update `web/src/lib/rating.ts`**
  Modify `/Users/tahsinarafat/App_Dev/AIOJ/web/src/lib/rating.ts` to return new tier names:
  - `legendary-grandmaster` -> `apex`
  - `international-grandmaster` -> `immortal`
  - `grandmaster` -> `titan`
  - `international-master` -> `grandmaster`
  - `master` -> `master`
  - `candidate-master` -> `champion`
  - `expert` -> `elite`
  - `specialist` -> `adept`
  - `pupil` -> `apprentice`
  - `newbie` -> `novice`

- [ ] **Step 2: Update `web/src/components/RatingGraph.tsx`**
  Modify `/Users/tahsinarafat/App_Dev/AIOJ/web/src/components/RatingGraph.tsx` to update the labels in `BANDS` array to match the new tier names.

- [ ] **Step 3: Update `web/src/pages/ProblemList.tsx`**
  Modify `/Users/tahsinarafat/App_Dev/AIOJ/web/src/pages/ProblemList.tsx` to update rating range filter labels (e.g. `Newbie-Pupil` -> `Novice-Apprentice`).

- [ ] **Step 4: Commit**
  ```bash
  git add web/src/lib/rating.ts web/src/components/RatingGraph.tsx web/src/pages/ProblemList.tsx
  git commit -m "feat: rename rating color tiers in frontend"
  ```

---

### Task 3: Unrated User Rating Calculations & Persistence

**Files:**
- Modify: `internal/api/handler/contest.go:605-621`
- Modify: `internal/api/router.go:254-257`
- Modify: `internal/rating/service.go:53-70`
- Modify: `internal/store/postgres/users.go:127-161`
- Modify: `internal/store/interfaces.go:21`
- Modify: `internal/model/contest.go:8-40`

- [ ] **Step 1: Update `internal/model/contest.go`**
  Add `RatingCalculated bool `json:"rating_calculated"`` to the `Contest` struct.

- [ ] **Step 2: Add dynamic rating_calculated checks in HTTP handlers**
  In `/Users/tahsinarafat/App_Dev/AIOJ/internal/api/handler/contest.go`:
  - In `GetByID` (line 186): Fetch rating history to set `c.RatingCalculated = len(ratings) > 0`.
  - In `Scoreboard` (line 505): Fetch rating history to set `contest.RatingCalculated = len(ratings) > 0`.

- [ ] **Step 3: Add `UpdateRating` to `store.UserStore` interface**
  In `/Users/tahsinarafat/App_Dev/AIOJ/internal/store/interfaces.go`, add to `UserStore` interface:
  ```go
  UpdateRating(ctx context.Context, userID string, rating, maxRating, contestCount int) error
  ```

- [ ] **Step 4: Implement `UpdateRating` and fix `GetProfile` in `/Users/tahsinarafat/App_Dev/AIOJ/internal/store/postgres/users.go`**
  - Implement `UpdateRating`:
    ```go
    func (s *UserStore) UpdateRating(ctx context.Context, userID string, rating, maxRating, contestCount int) error {
    	_, err := s.db.ExecContext(ctx,
    		`UPDATE user_profiles SET rating = $1, max_rating = $2, contest_count = $3 WHERE user_id = $4`,
    		rating, maxRating, contestCount, userID)
    	return err
    }
    ```
  - In `GetProfile`, if it returns `sql.ErrNoRows`, execute an insert query to initialize a default profile record dynamically:
    ```go
    if err == sql.ErrNoRows {
    	_, insertErr := s.db.ExecContext(ctx,
    		`INSERT INTO user_profiles (user_id, rating, problems_solved, submissions, bio, max_rating, contest_count)
    		 VALUES ($1, 0, 0, 0, '', 0, 0) ON CONFLICT (user_id) DO NOTHING`,
    		userID)
    	if insertErr != nil {
    		return nil, insertErr
    	}
    	// Re-run the select query
    	err = s.db.QueryRowContext(ctx, ...).Scan(...)
    	if err != nil {
    		return nil, err
    	}
    	return &p, nil
    }
    ```

- [ ] **Step 5: Update `ApplyContestRatings` in `internal/rating/service.go`**
  Update `ApplyContestRatings` to load each participant's current profile, compute their new `max_rating` and incremented `contest_count`, and persist these to `user_profiles` using `s.userStore.UpdateRating`.

- [ ] **Step 6: Update `CalculateRatings` HTTP handler in `internal/api/handler/contest.go`**
  - Change `oldRating := rating.DefaultRating` to `oldRating := 0` when `latest == nil`. This shows unrated users starting from rating `0`.
  - Wire `ApplyContestRatings` to persist calculations upon triggering. Initialize the service passing `h.userStore` instead of `nil`.

- [ ] **Step 7: Wire `/api/rating/calculate/{id}` in Chi router**
  In `/Users/tahsinarafat/App_Dev/AIOJ/internal/api/router.go`, add route `/api/rating/calculate/{id}` to route POSTs to `contestH.CalculateRatings` for frontend compatibility.

- [ ] **Step 8: Run rating and service tests**
  Run: `go test -v ./internal/rating/...`
  Expected: PASS

- [ ] **Step 9: Commit**
  ```bash
  git add internal/api/handler/contest.go internal/api/router.go internal/rating/service.go internal/store/postgres/users.go internal/store/interfaces.go internal/model/contest.go
  git commit -m "feat: show unrated users as 0, calculate with 1500 baseline, and apply ratings on rate contest"
  ```

---

### Task 4: Hide Admin Accounts in Rankings/Homepage Lists

**Files:**
- Modify: `internal/store/postgres/users.go:212-232`

- [ ] **Step 1: Exclude admin users in `ListUsersByRating`**
  Modify `/Users/tahsinarafat/App_Dev/AIOJ/internal/store/postgres/users.go` in `ListUsersByRating`:
  - Update `countQuery` to include `WHERE u.role != 'admin'`.
  - Update the main `query` WHERE clause from `WHERE 1=1%s` to `WHERE u.role != 'admin'%s`.

- [ ] **Step 2: Run backend tests**
  Run: `go test -v ./internal/store/postgres/...`
  Expected: PASS

- [ ] **Step 3: Commit**
  ```bash
  git add internal/store/postgres/users.go
  git commit -m "feat: hide admin accounts from homepage and rankings lists"
  ```
