# Phase D — Community & Social

**Status:** Core shipped (2026-06-13)

| Task | Status | Notes |
|------|--------|-------|
| Per-problem discussion | ✅ | `parent_type` already in CHECK; CommentSection on ProblemDetail |
| Country flag display | ✅ | `CountryFlag` + rankings country column |
| Per-country leaderboard | ✅ | rankings `?country=` already existed |
| Achievement/badge system | ✅ | migrations 000063; awards on AC milestones |
| Friends/follow | ✅ | migrations 000064; follow API |
| Activity feed | ✅ | GET /api/activity + ActivityFeed on Home |
| MOSS plagiarism | ✅ | optional MossClient (MOSS_USER/MOSS_URL); local LCS default |
| Editorial auto-publish | ⏳ | |
| Country flags on profile | ✅ | uses same CountryFlag where country set |

Verified: go build/test, vitest 58, tsc, vite build; DB migrate → 64.
