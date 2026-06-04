# Design Spec: Remove Hashed Slugs and Restructure Homepage with Codeforces Format

## 1. Overview
Currently, the homepage and other sections use the internal UUID (`id`) of contests in their links, resulting in URLs like `/contests/a0000001-0000-0000-0000-000000000001`. We want to replace these with either custom slugs or numeric `display_id` values sidewide.
Additionally, we want to restructure the homepage layout to match a Codeforces-style split format: a main content area for announcements and blog posts on the left, and a sidebar on the right containing user info/login, upcoming/recent contests, top-rated users, and quick links.

## 2. Proposed Changes

### 2.1 Backend Changes
To support clean slugs on pages like Gym Detail, we need to pass `c.slug` and `c.display_id` in the Gym model and query.

#### `internal/model/gym.go`
Update `GymContest` struct to include slug and display ID:
```go
type GymContest struct {
    ...
    ContestSlug      string    `json:"contest_slug,omitempty"`
    ContestDisplayID int       `json:"contest_display_id,omitempty"`
    ...
}
```

#### `internal/store/postgres/gym.go`
Update SQL SELECT statements in `GetByID` and `List` to retrieve `c.slug` and `c.display_id` from the `contests` table:
```go
// In GetByID:
`SELECT g.id, g.contest_id, c.title, c.slug, c.display_id, g.difficulty_rating, g.category, g.country, g.season,
        g.description, g.is_public, g.solve_count, g.created_by, u.username, g.created_at
 FROM gym_contests g
 JOIN contests c ON g.contest_id = c.id
 JOIN users u ON g.created_by = u.id
 WHERE g.id = $1`

// In List:
`SELECT g.id, g.contest_id, c.title, c.slug, c.display_id, g.difficulty_rating, g.category, g.country,
        g.season, g.description, g.is_public, g.solve_count, g.created_by, u.username, g.created_at
 FROM gym_contests g
 JOIN contests c ON g.contest_id = c.id
 JOIN users u ON g.created_by = u.id
 ...`
```
Scan the columns into the new struct fields.

---

### 2.2 Frontend Changes

#### `web/src/App.tsx` (Homepage Restructuring)
1. Import `contestSlug` from `lib/api`.
2. Restructure the layout to use a split grid (`grid grid-cols-1 lg:grid-cols-4 gap-6`).
3. **Left Column (`lg:col-span-3` - Main Content)**:
   - Welcome/Hero banner.
   - Latest Posts (Recent Blog Posts) with more details (author, date).
4. **Right Column (`lg:col-span-1` - Sidebar)**:
   - **User Stats/Login Card**: If logged in, show username and profile link. If not, show login/register links.
   - **Contests Card**: "Recent / Upcoming Contests" list. All links will use `contestSlug(c)` instead of `c.id`.
   - **Top Rated Users Widget**: Top 10 users ranked by rating (using `api.rankings.list(0, 10)`).
   - **Quick Links**.

#### `web/src/pages/GymDetail.tsx`
1. Import `contestSlug` from `lib/api`.
2. Update the "Enter Contest" link:
   ```tsx
   <Link to={`/contests/${contestSlug({ slug: gym.contest_slug, display_id: gym.contest_display_id, id: gym.contest_id })}`} ...>
   ```

#### `web/src/pages/VirtualContest.tsx`
1. Import `contestSlug` from `lib/api`.
2. Update the "View Scoreboard" link at page completion:
   ```tsx
   <Link to={`/contests/${contestSlug(contest)}/scoreboard`} ...>
   ```

## 3. Verification & Testing
1. Verify Go compilation succeeds (`go build ./...`).
2. Verify frontend compilation succeeds (`npm run build`).
3. Verify that URLs on the home page recent contests, gym detail contest page, and virtual contest summary page resolve to slugs (e.g. `/contests/my-slug` or `/contests/3`) instead of UUIDs.
