# Remove Hashed Slugs and Homepage Restructure Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove UUID/hashed slugs from all contest links sidewide and restructure the homepage to Codeforces split column format.

**Architecture:** Update Gym models and query to select slug and display_id so the frontend can generate non-hashed links on the Gym page. Restructure Home component in web/src/App.tsx using a standard 4-column layout split grid, moving widgets like Top Users and Contests to the sidebar, and putting blog/announcements in the main column.

**Tech Stack:** Go (Backend/Chi/PostgreSQL), React + Vite + Tailwind CSS (Frontend)

---

### Task 1: Backend Gym Contest Model and Store Update

**Files:**
- Modify: `internal/model/gym.go`
- Modify: `internal/store/postgres/gym.go`

- [ ] **Step 1: Update model struct**
Add `ContestSlug` and `ContestDisplayID` to `model.GymContest` struct:
```go
type GymContest struct {
	ID               string    `json:"id"`
	ContestID        string    `json:"contest_id"`
	ContestTitle     string    `json:"contest_title,omitempty"`
	ContestSlug      string    `json:"contest_slug,omitempty"`
	ContestDisplayID int       `json:"contest_display_id,omitempty"`
	DifficultyRating *int      `json:"difficulty_rating,omitempty"`
	Category         string    `json:"category"`
	Country          string    `json:"country,omitempty"`
	Season           string    `json:"season,omitempty"`
	Description      string    `json:"description"`
	IsPublic         bool      `json:"is_public"`
	SolveCount       int       `json:"solve_count"`
	CreatedBy        string    `json:"created_by"`
	CreatorName      string    `json:"creator_name,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}
```

- [ ] **Step 2: Update store GetByID and List queries**
Modify `postgres.GymStore.GetByID` to select `c.slug` and `c.display_id` and scan them:
```go
func (s *GymStore) GetByID(ctx context.Context, id string) (*model.GymContest, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, nil
	}
	var g model.GymContest
	var slug sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT g.id, g.contest_id, c.title, c.slug, c.display_id, g.difficulty_rating, g.category, g.country, g.season,
		        g.description, g.is_public, g.solve_count, g.created_by, u.username, g.created_at
		 FROM gym_contests g
		 JOIN contests c ON g.contest_id = c.id
		 JOIN users u ON g.created_by = u.id
		 WHERE g.id = $1`,
		id).Scan(&g.ID, &g.ContestID, &g.ContestTitle, &slug, &g.ContestDisplayID, &g.DifficultyRating, &g.Category,
		&g.Country, &g.Season, &g.Description, &g.IsPublic, &g.SolveCount,
		&g.CreatedBy, &g.CreatorName, &g.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if slug.Valid {
		g.ContestSlug = slug.String
	}
	return &g, nil
}
```

And modify `postgres.GymStore.List` to do the same:
```go
	query := fmt.Sprintf(`SELECT g.id, g.contest_id, c.title, c.slug, c.display_id, g.difficulty_rating, g.category, g.country,
	                              g.season, g.description, g.is_public, g.solve_count, g.created_by, u.username, g.created_at
	                      FROM gym_contests g
	                      JOIN contests c ON g.contest_id = c.id
	                      JOIN users u ON g.created_by = u.id
	                      WHERE %s
	                      ORDER BY g.created_at DESC OFFSET $%d LIMIT $%d`,
		whereClause, argIdx, argIdx+1)
	args = append(args, offset, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.GymContest
	for rows.Next() {
		var g model.GymContest
		var slug sql.NullString
		if err := rows.Scan(&g.ID, &g.ContestID, &g.ContestTitle, &slug, &g.ContestDisplayID, &g.DifficultyRating, &g.Category,
			&g.Country, &g.Season, &g.Description, &g.IsPublic, &g.SolveCount,
			&g.CreatedBy, &g.CreatorName, &g.CreatedAt); err != nil {
			return nil, 0, err
		}
		if slug.Valid {
			g.ContestSlug = slug.String
		}
		items = append(items, g)
	}
```

- [ ] **Step 3: Run backend build and verify**
Run: `go build ./...`
Expected: Success.

- [ ] **Step 4: Commit**
```bash
git add internal/model/gym.go internal/store/postgres/gym.go
git commit -m "backend: add slug and display_id to GymContest"
```

---

### Task 2: Frontend Pages Updates

**Files:**
- Modify: `web/src/pages/GymDetail.tsx`
- Modify: `web/src/pages/VirtualContest.tsx`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Update GymDetail link**
Change GymDetail to import `contestSlug` and use it:
```tsx
import { api, getAccessToken, contestSlug } from '../lib/api'
// ...
<Link to={`/contests/${contestSlug({ slug: gym.contest_slug, display_id: gym.contest_display_id, id: gym.contest_id })}`} className="bg-blue-600 text-white px-6 py-2 rounded text-sm hover:bg-blue-700 font-medium">
    Enter Contest
</Link>
```

- [ ] **Step 2: Update VirtualContest scoreboard link**
Change VirtualContest to use `contestSlug(contest)`:
```tsx
import { api, getAccessToken, contestSlug } from '../lib/api'
// ...
<Link
    to={`/contests/${contestSlug(contest)}/scoreboard`}
    className="text-blue-600 hover:underline text-sm"
>
    View Scoreboard
</Link>
```

- [ ] **Step 3: Restructure Homepage layout in web/src/App.tsx**
Modify `Home` component in `web/src/App.tsx` to:
1. Import `contestSlug` from `./lib/api`.
2. Fetch top 10 users using `api.rankings.list(0, 10)` in `useEffect` and set them in a new `rankings` state.
3. Change layout inside the return value of `Home` to:
```tsx
import { useState, useEffect } from 'react'
import { BrowserRouter, Routes, Route, Link } from 'react-router-dom'
import { api, contestSlug } from './lib/api'
import { ThemeProvider } from './context/ThemeContext'
// ...
```
And inside `Home`:
```tsx
function Home() {
    const [contests, setContests] = useState<any[]>([])
    const [posts, setPosts] = useState<any[]>([])
    const [stats, setStats] = useState({ problems: 0, users: 0, submissions: 0 })
    const [rankings, setRankings] = useState<any[]>([])
    const [loading, setLoading] = useState(true)
    const token = localStorage.getItem('access_token')
    const username = token ? JSON.parse(atob(token.split('.')[1])).sub : null

    useEffect(() => {
        Promise.all([
            api.contests.list(0, 5),
            api.blog.list(0, 5),
            api.stats.getPlatform(),
            api.rankings.list(0, 10),
        ]).then(([contestData, blogData, statsData, rankData]) => {
            setContests(contestData.data || [])
            setPosts(blogData.data || [])
            setStats(statsData)
            setRankings(rankData.data || [])
        }).catch(() => {}).finally(() => setLoading(false))
    }, [])

    const contestStatus = (c: any) => {
        const now = Date.now()
        const start = new Date(c.start_time).getTime()
        const end = new Date(c.end_time).getTime()
        if (now < start) return { text: 'Upcoming', cls: 'text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-900/20' }
        if (now < end) return { text: 'Running', cls: 'text-green-600 dark:text-green-400 bg-green-50 dark:bg-green-900/20' }
        return { text: 'Ended', cls: 'text-gray-500 dark:text-gray-400 bg-gray-100 dark:bg-gray-700' }
    }

    return (
        <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
            {/* Main Content (Left, spans 3 columns) */}
            <div className="lg:col-span-3 space-y-6">
                {/* Hero */}
                <section className="bg-gradient-to-r from-blue-500 to-blue-700 text-white rounded-lg px-8 py-12">
                    <h1 className="text-3xl font-bold mb-2">Welcome to AIOJ</h1>
                    <p className="text-blue-100 mb-6 max-w-lg">A lightweight online judge for competitive programming. Practice problems, join contests, and improve your skills.</p>
                    <div className="flex gap-3">
                        <Link to="/problems" className="bg-white dark:bg-gray-800 text-blue-700 dark:text-blue-300 px-5 py-2 rounded font-medium hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-colors">Browse Problems</Link>
                        <Link to="/contests" className="border border-white/40 px-5 py-2 rounded font-medium hover:bg-white dark:hover:bg-gray-700/10 transition-colors">View Contests</Link>
                    </div>
                </section>

                {/* Stats */}
                <section className="grid grid-cols-3 gap-4">
                    {[
                        { label: 'Problems', value: stats.problems },
                        { label: 'Users', value: stats.users },
                        { label: 'Submissions', value: stats.submissions },
                    ].map(s => (
                        <div key={s.label} className="border border-gray-200 dark:border-gray-700 rounded-lg p-5 text-center bg-white dark:bg-gray-800">
                            <div className="text-2xl font-bold text-gray-900 dark:text-gray-100">{s.value.toLocaleString()}</div>
                            <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">{s.label}</div>
                        </div>
                    ))}
                </section>

                {/* Recent Blog Posts */}
                <section className="space-y-4">
                    <div className="flex justify-between items-center">
                        <h2 className="text-xl font-bold">Latest Posts</h2>
                        <Link to="/blog" className="text-sm text-blue-600 dark:text-blue-400 hover:underline">View all</Link>
                    </div>
                    {loading ? (
                        <div className="text-center py-8 text-gray-400 dark:text-gray-500">Loading...</div>
                    ) : posts.length === 0 ? (
                        <div className="text-center py-8 text-gray-400 dark:text-gray-500">No posts yet.</div>
                    ) : (
                        <div className="space-y-3">
                            {posts.map(p => (
                                <Link key={p.id} to={`/blog/${p.id}`}
                                    className="block border border-gray-200 dark:border-gray-700 rounded-lg p-5 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors bg-white dark:bg-gray-800 shadow-sm">
                                    <h3 className="text-lg font-semibold text-blue-600 dark:text-blue-400 hover:underline mb-2">{p.title}</h3>
                                    <div className="text-xs text-gray-500 dark:text-gray-450 flex gap-4 items-center">
                                        <span>By <span className="font-semibold text-gray-700 dark:text-gray-300">{p.username}</span></span>
                                        <span>•</span>
                                        <span>{p.upvotes} upvotes</span>
                                        <span>•</span>
                                        <span>{new Date(p.created_at).toLocaleDateString()}</span>
                                    </div>
                                </Link>
                            ))}
                        </div>
                    )}
                </section>
            </div>

            {/* Sidebar (Right, spans 1 column) */}
            <div className="space-y-6">
                {/* User Stats / Login box */}
                <div className="border border-gray-200 dark:border-gray-700 rounded-lg p-5 bg-white dark:bg-gray-800 shadow-sm">
                    {username ? (
                        <div className="text-center space-y-3">
                            <div className="w-16 h-16 bg-blue-100 dark:bg-blue-900/50 rounded-full flex items-center justify-center mx-auto">
                                <span className="text-xl font-bold text-blue-600 dark:text-blue-300 uppercase">{username[0]}</span>
                            </div>
                            <div>
                                <h3 className="font-bold text-gray-950 dark:text-gray-50">{username}</h3>
                                <p className="text-xs text-gray-500">Logged in</p>
                            </div>
                            <Link to="/profile" className="block text-xs bg-blue-600 text-white py-1.5 px-3 rounded hover:bg-blue-700 transition-colors font-medium">
                                View Profile
                            </Link>
                        </div>
                    ) : (
                        <div className="space-y-3 text-center">
                            <h3 className="font-bold text-gray-800 dark:text-gray-200 text-sm">Join the Community</h3>
                            <p className="text-xs text-gray-500">Sign in to solve problems, compete in contests, and read posts.</p>
                            <div className="flex gap-2 justify-center">
                                <Link to="/login" className="text-xs bg-blue-600 text-white py-1.5 px-4 rounded hover:bg-blue-700 transition-colors font-medium">
                                    Login
                                </Link>
                                <Link to="/register" className="text-xs bg-gray-100 hover:bg-gray-200 text-gray-700 py-1.5 px-4 rounded transition-colors font-medium">
                                    Register
                                </Link>
                            </div>
                        </div>
                    )}
                </div>

                {/* Contests Block */}
                <div className="border border-gray-200 dark:border-gray-700 rounded-lg p-5 bg-white dark:bg-gray-800 shadow-sm">
                    <h3 className="font-bold text-sm mb-3 border-b pb-2 border-gray-100 dark:border-gray-700">Recent & Upcoming Contests</h3>
                    {loading ? (
                        <div className="text-xs text-gray-400 py-2">Loading...</div>
                    ) : contests.length === 0 ? (
                        <div className="text-xs text-gray-400 py-2">No contests</div>
                    ) : (
                        <div className="space-y-2">
                            {contests.map(c => {
                                const status = contestStatus(c)
                                return (
                                    <Link key={c.id} to={`/contests/${contestSlug(c)}`} className="block group">
                                        <div className="text-xs font-semibold text-gray-800 dark:text-gray-200 group-hover:text-blue-600 transition-colors line-clamp-1">
                                            {c.title}
                                        </div>
                                        <div className="flex items-center justify-between mt-1 text-[10px] text-gray-450">
                                            <span>{new Date(c.start_time).toLocaleDateString()}</span>
                                            <span className={`px-1.5 rounded-[3px] font-medium scale-90 origin-right ${status.cls}`}>{status.text}</span>
                                        </div>
                                    </Link>
                                )
                            })}
                            <Link to="/contests" className="block text-center text-xs text-blue-600 hover:underline mt-2">
                                View all contests
                            </Link>
                        </div>
                    )}
                </div>

                {/* Top Rated Users rankings widget */}
                <div className="border border-gray-200 dark:border-gray-700 rounded-lg p-5 bg-white dark:bg-gray-800 shadow-sm">
                    <h3 className="font-bold text-sm mb-3 border-b pb-2 border-gray-100 dark:border-gray-700">Top Rated Users</h3>
                    {loading ? (
                        <div className="text-xs text-gray-400 py-2">Loading...</div>
                    ) : rankings.length === 0 ? (
                        <div className="text-xs text-gray-400 py-2">No rankings</div>
                    ) : (
                        <div className="space-y-2">
                            {rankings.map((user, i) => (
                                <div key={user.user_id} className="flex justify-between items-center text-xs">
                                    <div className="flex gap-2 items-center">
                                        <span className="font-mono text-gray-400 w-4">{i + 1}</span>
                                        <Link to={`/user/${user.username}`} className="font-semibold text-blue-600 dark:text-blue-400 hover:underline">
                                            {user.username}
                                        </Link>
                                    </div>
                                    <span className="font-mono font-bold text-gray-700 dark:text-gray-300">{user.rating}</span>
                                </div>
                            ))}
                            <Link to="/rankings" className="block text-center text-xs text-blue-600 hover:underline mt-2">
                                View full standings
                            </Link>
                        </div>
                    )}
                </div>

                {/* Quick Links */}
                <div className="border border-gray-200 dark:border-gray-700 rounded-lg p-5 bg-white dark:bg-gray-800 shadow-sm">
                    <h3 className="font-bold text-sm mb-3 border-b pb-2 border-gray-100 dark:border-gray-700">Quick Links</h3>
                    <div className="grid grid-cols-2 gap-2 text-center">
                        <Link to="/problems" className="bg-gray-50 hover:bg-gray-100 dark:bg-gray-900/50 dark:hover:bg-gray-900 border border-gray-100 dark:border-gray-750 text-xs py-2 rounded text-gray-700 dark:text-gray-300 font-medium transition-colors">Problems</Link>
                        <Link to="/practice" className="bg-gray-50 hover:bg-gray-100 dark:bg-gray-900/50 dark:hover:bg-gray-900 border border-gray-100 dark:border-gray-750 text-xs py-2 rounded text-gray-700 dark:text-gray-300 font-medium transition-colors">Practice</Link>
                        <Link to="/blog" className="bg-gray-50 hover:bg-gray-100 dark:bg-gray-900/50 dark:hover:bg-gray-900 border border-gray-100 dark:border-gray-750 text-xs py-2 rounded text-gray-700 dark:text-gray-300 font-medium transition-colors">Blogs</Link>
                        <Link to="/rankings" className="bg-gray-50 hover:bg-gray-100 dark:bg-gray-900/50 dark:hover:bg-gray-900 border border-gray-100 dark:border-gray-750 text-xs py-2 rounded text-gray-700 dark:text-gray-300 font-medium transition-colors">Rankings</Link>
                    </div>
                </div>
            </div>
        </div>
    )
}
```

- [ ] **Step 4: Run frontend build and verify**
Run: `cd web && npm run build`
Expected: Success.

- [ ] **Step 5: Commit**
```bash
git add web/src/pages/GymDetail.tsx web/src/pages/VirtualContest.tsx web/src/App.tsx
git commit -m "frontend: change links to slug/display_id and restructure homepage to Codeforces format"
```
