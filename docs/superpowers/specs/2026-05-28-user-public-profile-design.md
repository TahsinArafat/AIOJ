# User Public Profile Page Design

**Date**: 2026-05-28  
**Status**: Draft  
**Scope**: New public page at `/user/:username` showing user profile, stats, and activity

## Overview

Create a public-facing user profile page that displays user information, rating history, recent submissions, and solved problems. The page requires no authentication and follows existing AIOJ frontend patterns.

## Goals

- Display public user information (username, rating, member since)
- Show user statistics (problems solved, contests played)
- Present rating history with simple visualization
- List recent submissions with status colors
- Show solved problems with clickable links
- Handle loading and 404 states gracefully

## Non-Goals

- User authentication or login requirements
- Complex chart libraries (use CSS-only visualization)
- Editing capabilities (read-only public view)
- Sensitive information exposure

## Route

```
/user/:username
```

- Public route, no auth required
- Username extracted from URL parameter
- 404 page if user not found

## Page Structure

### Layout

Single-column layout, max-width 2xl, centered (matches `RatingHistory.tsx` pattern).

```
┌─────────────────────────────────────────────┐
│ Header Card                                  │
│ [Avatar] Username                           │
│          Rating Badge                       │
│          Member since date                  │
├─────────────────────────────────────────────┤
│ Stats Grid (3 columns)                      │
│ ┌─────────┐ ┌─────────┐ ┌─────────┐       │
│ │ Problems│ │ Contests│ │ Rating  │       │
│ │ Solved  │ │ Played  │ │ Current │       │
│ └─────────┘ └─────────┘ └─────────┘       │
├─────────────────────────────────────────────┤
│ Rating History Chart                        │
│ [Simple bar chart - CSS only]              │
│ [Link to /rating-history]                  │
├─────────────────────────────────────────────┤
│ Recent Submissions (last 10)               │
│ ┌─────────────────────────────────────────┐│
│ │ Problem | Status | Language | Time      ││
│ │ ...     | ...    | ...      | ...       ││
│ └─────────────────────────────────────────┘│
├─────────────────────────────────────────────┤
│ Solved Problems                             │
│ [Problem 1] [Problem 2] [Problem 3] ...   │
└─────────────────────────────────────────────┘
```

### Section 1: Header Card

- **Avatar**: Initials-based placeholder (first letter of username)
- **Username**: Display name
- **Rating Badge**: Uses existing `RatingBadge` component with `showTitle` and `size="lg"`
- **Member Since**: Formatted date from `user.created_at`

### Section 2: Stats Grid

Three stat cards in a grid layout:

| Stat | Source | Display |
|------|--------|---------|
| Problems Solved | `stats.solved_count` | Number |
| Contests Played | `stats.contest_count` | Number |
| Current Rating | `user.rating` | RatingBadge or "Unrated" |

### Section 3: Rating History Chart

Simple CSS-only bar chart:
- Last 10 rating entries
- Bars represent `new_rating` values
- Height proportional to max rating
- Link to `/rating-history` for full history
- Empty state: "No rated contests yet"

### Section 4: Recent Submissions

Table showing last 10 submissions:

| Column | Content | Styling |
|--------|---------|---------|
| Problem | Link to `/problems/:slug` | Blue link |
| Status | Status badge | Green (accepted), Red (wrong), Yellow (other) |
| Language | Language name | Gray text |
| Time | Formatted date | Gray text |

Empty state: "No submissions yet"

### Section 5: Solved Problems

List of solved problems as clickable tags:
- Each tag links to `/problems/:slug`
- Flex-wrap layout for responsive display
- Hover effect (gray-200 background)
- Empty state: "No problems solved yet"

## API Endpoints

### New Endpoints Required

```typescript
// Add to api.ts
users: {
  getByUsername: (username: string) => request<UserProfile>(`/users/${username}`),
},
stats: {
  getUserStats: (userId: string) => request<UserStats>(`/stats/user/${userId}`),
}
```

### Existing Endpoints (Reuse)

```typescript
api.ratings.getByUser(userId, limit)  // GET /api/rating/user/:userId
api.submissions.list(offset, limit)   // GET /api/submissions (with user filter)
```

### Data Shapes

```typescript
interface UserProfile {
  id: string;
  username: string;
  rating: number | null;
  created_at: string;
}

interface UserStats {
  solved_count: number;
  contest_count: number;
  submission_count: number;
}

interface RatingEntry {
  id: string;
  user_id: string;
  contest_id: string;
  old_rating: number;
  new_rating: number;
  rank: number;
  rating_change: number;
  created_at: string;
}

interface Submission {
  id: string;
  problem_id: string;
  problem_slug: string;
  problem_name: string;
  status: string;
  language: string;
  created_at: string;
}

interface Problem {
  id: string;
  slug: string;
  name: string;
}
```

## Component Structure

```tsx
// web/src/pages/UserPublicProfile.tsx
export default function UserPublicProfile() {
  const { username } = useParams<{ username: string }>();
  const [user, setUser] = useState<UserProfile | null>(null);
  const [stats, setStats] = useState<UserStats | null>(null);
  const [ratingHistory, setRatingHistory] = useState<RatingEntry[]>([]);
  const [recentSubmissions, setRecentSubmissions] = useState<Submission[]>([]);
  const [solvedProblems, setSolvedProblems] = useState<Problem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // Fetch user by username
    // Fetch stats, rating history, submissions, solved problems
  }, [username]);

  if (loading) return <LoadingSkeleton />;
  if (error) return <NotFound />;
  
  return (
    <div className="max-w-2xl mx-auto">
      <HeaderCard user={user} />
      <StatsGrid stats={stats} rating={user.rating} />
      <RatingChart history={ratingHistory} />
      <RecentSubmissions submissions={recentSubmissions} />
      <SolvedProblems problems={solvedProblems} />
    </div>
  );
}
```

## States

### Loading State

Skeleton placeholders for all sections:
- Gray rounded rectangles for text
- Animated pulse effect
- Matches existing loading patterns

### Error/404 State

- "User not found" message
- Link back to home page
- Red error styling

### Empty States

- Rating history: "No rated contests yet"
- Submissions: "No submissions yet"
- Solved problems: "No problems solved yet"
- All with gray-400 text, centered

## Responsive Behavior

- **Stats grid**: 3 columns on desktop, 1 column on mobile
- **Submissions table**: Horizontal scroll on mobile
- **Solved problems**: Flex-wrap for tags
- **Header**: Stack avatar and info vertically on mobile

## Design Tokens

Following existing AIOJ design system:

- **Colors**: 
  - Primary: blue-600, blue-700
  - Success: green-100, green-800
  - Error: red-100, red-800
  - Warning: yellow-100, yellow-800
  - Neutral: gray-100, gray-200, gray-400, gray-500, gray-600, gray-800, gray-900
- **Spacing**: Tailwind default (4px base)
- **Typography**: 
  - Headings: text-2xl, text-lg, text-sm
  - Body: text-sm, text-xs
- **Borders**: border-gray-200, rounded-lg
- **Shadows**: shadow-sm, shadow-md (hover)
- **Backgrounds**: bg-white, bg-gray-50 (table headers)

## File Changes

### New Files

1. `web/src/pages/UserPublicProfile.tsx` - Main page component

### Modified Files

1. `web/src/App.tsx` - Add route `/user/:username`
2. `web/src/lib/api.ts` - Add `api.users.getByUsername()` and `api.stats.getUserStats()`

## Testing

- Verify 404 handling for invalid usernames
- Verify loading states
- Verify empty states (new user with no activity)
- Verify responsive layout on mobile/desktop
- Verify links navigate correctly

## Success Criteria

- Page loads without authentication
- All sections display correctly with data
- Loading and 404 states work
- Links navigate to correct pages
- Responsive layout works on mobile
- No sensitive information exposed
