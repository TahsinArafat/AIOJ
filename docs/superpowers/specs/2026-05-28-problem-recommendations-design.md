# Problem Recommendations System Design

This document details the design for the personalized **Problem Recommendations** feature in AIOJ. This implementation fulfills the requirements of Sub-Plan 16, introducing a complete training companion that helps users progress systematically, practice their weak areas, and receive a curated hybrid practice diet.

---

## 1. Objectives

- Offer **Rating-Based Progression**: Recommend unsolved problems in the user's current rating/difficulty band.
- Offer **Weak Tag practice**: Analyze past submissions to identify tags where the user has high failure rates and recommend matching unsolved practice problems.
- Offer **Hybrid curated recommendations**: A combination of progression problems, weak-tag practice, and a daily challenge.
- Build a responsive frontend dashboard under a new **"Practice"** tab/page.

---

## 2. Architecture & Recommendation Algorithm

### A. Difficulty Classification
We align user ratings to the existing problem difficulty spectrum (`easy`, `medium`, `hard`):
- **User Rating < 1400** (Newbie, Pupil) → `easy` difficulty.
- **1400 <= User Rating < 1900** (Specialist, Expert) → `medium` difficulty.
- **User Rating >= 1900** (Candidate Master+) → `hard` difficulty.
- *Default rating is 1200 if no contest history is present.*

### B. Progression Recommendation
1. Query user's current rating from `user_profiles`.
2. Map rating to target difficulty (`easy`, `medium`, `hard`).
3. Query database for visible problems in this difficulty level that the user has **never solved** (no `status = 'ac'` submission).
4. Order by acceptance rate or popularity, returning up to 5 items.

### C. Weak-Tag Practice
1. Query user's past non-AC submissions.
2. Join with `problems` to extract the tags associated with each problem.
3. Calculate tag error counts (count of submissions where `status != 'ac'`).
4. Select the top 2 tags with the highest failure counts (where the user has not subsequently solved the problem).
5. Query unsolved problems containing these weak tags matching the user's difficulty band.

### D. Hybrid Curriculum
Generate a unified 5-problem list:
- 2 Progression problems.
- 2 Weak-tag reinforcement problems.
- 1 "Daily Challenge" (a random unsolved problem matching their difficulty tier).

---

## 3. Database Schema Changes

No new tables are required. The system fully leverages existing tables:
- `users` / `user_profiles` (for user ratings).
- `problems` (for tag lists, difficulty levels).
- `submissions` (to filter solved problems and compute tag error rates).

We will add a database index to optimize recommendation queries:
```sql
CREATE INDEX IF NOT EXISTS idx_submissions_user_status_problem ON submissions(user_id, status, problem_id);
```

---

## 4. REST API Endpoint

### `GET /api/recommendations`
- **Authentication**: Required (JWT Bearer Token).
- **Request Headers**:
  - `Authorization: Bearer <token>`
- **Response Format (`200 OK`)**:
```json
{
  "progression": [
    {
      "id": "uuid",
      "slug": "problem-slug",
      "title": "Problem Title",
      "difficulty": "easy",
      "tags": ["dp", "math"],
      "submission_count": 42,
      "accepted_count": 12,
      "source": "local"
    }
  ],
  "weak_tags": {
    "tags": ["dp", "graphs"],
    "problems": [
      {
        "id": "uuid",
        "slug": "another-slug",
        "title": "Graphs Practice",
        "difficulty": "easy",
        "tags": ["graphs"],
        "submission_count": 100,
        "accepted_count": 50,
        "source": "local"
      }
    ]
  },
  "hybrid": [
    {
      "id": "uuid",
      "slug": "daily-challenge",
      "title": "Daily Challenge",
      "difficulty": "easy",
      "tags": ["greedy"],
      "submission_count": 20,
      "accepted_count": 15,
      "source": "local"
    }
  ]
}
```

---

## 5. Frontend UI Design

A new dashboard under `/practice` (or linked as "Practice" in the Navbar) featuring:
- **Profile Summary Card**: Shows user's rating, current tier, and overall stats.
- **Three distinct sub-sections / tabs**:
  - **Daily Diet (Hybrid)**: Curated list of 5 problems combining progression, weak tags, and a daily challenge.
  - **Progression Ladder**: Problems in the user's current tier to help boost their rating.
  - **Targeted Practice**: Problems matching their weakest topics (with tags clearly highlighted).
- Responsive grid style using Tailwind CSS matching the classic AIOJ Codeforces aesthetic.
