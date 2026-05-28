# AIOJ → Codeforces Alternative: Master Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform AIOJ from a basic online judge into a full-featured competitive programming platform comparable to Codeforces.

**Architecture:** Extend existing Go monolith with new database tables, store interfaces, handlers, and React frontend pages. Maintain current patterns (Chi router, PostgreSQL, React SPA). Add Redis for caching and rating calculations.

**Tech Stack:** Go 1.21+, PostgreSQL 16+, React 19, TypeScript, Tailwind CSS, Redis (new)

---

## Plan Structure

This master plan contains **7 phases** with **23 sub-plans**. Each sub-plan is a standalone feature that produces working, testable software.

| Phase | Sub-Plans | Duration | Priority |
|-------|-----------|----------|----------|
| **Phase 1: Core Competitive** | Rating System, Contest Registration, Division System, Problem Filtering | 4 weeks | CRITICAL |
| **Phase 2: Contest Features** | Virtual Contests, Gym/Training, Educational Rounds, Upsolving | 4 weeks | HIGH |
| **Phase 3: Engagement** | Hacking System, Problem Statistics, Notifications | 4 weeks | HIGH |
| **Phase 4: Community** | Groups, Teams, Blog/Discussions | 4 weeks | MEDIUM |
| **Phase 5: Content** | Editorials, Problem Recommendations | 2 weeks | MEDIUM |
| **Phase 6: Platform** | Public API, Rate Limiting, Webhooks | 2 weeks | LOW |
| **Phase 7: Polish** | Internationalization, Mobile PWA, Performance | 2 weeks | LOW |

**Total Estimated Duration:** 22 weeks (5.5 months)

---

## Sub-Plan Index

### Phase 1: Core Competitive Features

1. **[Rating System](./sub-plans/01-rating-system.md)** - Elo-based rating, color coding, division eligibility
2. **[Contest Registration](./sub-plans/02-contest-registration.md)** - Register/unregister, deadlines, participant lists
3. **[Division System](./sub-plans/03-division-system.md)** - Div 1/2/3/4, rating-based eligibility
4. **[Problem Filtering](./sub-plans/04-problem-filtering.md)** - Tags, difficulty, solved status, rating range

### Phase 2: Contest Features

5. **[Virtual Contests](./sub-plans/05-virtual-contests.md)** - Past contest simulation, ghost participants
6. **[Gym/Training](./sub-plans/06-gym-training.md)** - Community contests, training filters
7. **[Educational Rounds](./sub-plans/07-educational-rounds.md)** - Learning-focused contests, extended hacking
8. **[Upsolving](./sub-plans/08-upsolving.md)** - Full feedback after contests, solution comparison

### Phase 3: Engagement Features

9. **[Hacking System](./sub-plans/09-hacking-system.md)** - Counter-test cases, hack scoring, hacking phases
10. **[Problem Statistics](./sub-plans/10-problem-statistics.md)** - Language distribution, solver stats, difficulty estimation
11. **[Notifications](./sub-plans/11-notifications.md)** - Real-time alerts, email digests, preferences

### Phase 4: Community Features

12. **[Groups](./sub-plans/12-groups.md)** - Create/join groups, group contests, discussions
13. **[Teams](./sub-plans/13-teams.md)** - Team formation, team contests, team rating
14. **[Blog/Discussions](./sub-plans/14-blog-discussions.md)** - User posts, comments, problem discussions

### Phase 5: Content Features

15. **[Editorials](./sub-plans/15-editorials.md)** - Official solutions, community contributions
16. **[Problem Recommendations](./sub-plans/16-recommendations.md)** - Personalized problem suggestions

### Phase 6: Platform Features

17. **[Public API](./sub-plans/17-public-api.md)** - RESTful API, documentation, SDKs
18. **[Rate Limiting](./sub-plans/18-rate-limiting.md)** - Per-user limits, API keys, abuse prevention
19. **[Webhooks](./sub-plans/19-webhooks.md)** - Event notifications for integrations

### Phase 7: Polish

20. **[Internationalization](./sub-plans/20-internationalization.md)** - Multi-language UI, RTL support
21. **[Mobile PWA](./sub-plans/21-mobile-pwa.md)** - Progressive Web App, offline support
22. **[Performance](./sub-plans/22-performance.md)** - Caching, CDN, query optimization
23. **[Monitoring](./sub-plans/23-monitoring.md)** - Metrics, logging, alerting

---

## Database Schema Changes

### New Tables (Cumulative)

```sql
-- Phase 1: Rating System
CREATE TABLE rating_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    contest_id UUID NOT NULL REFERENCES contests(id),
    old_rating INTEGER NOT NULL,
    new_rating INTEGER NOT NULL,
    rank INTEGER NOT NULL,
    rating_change INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Phase 1: Contest Registration
CREATE TABLE contest_registrations (
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id),
    registered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (contest_id, user_id)
);

-- Phase 1: Division System
ALTER TABLE contests ADD COLUMN division INTEGER NOT NULL DEFAULT 0;
ALTER TABLE contests ADD COLUMN rated_for_min INTEGER NOT NULL DEFAULT 0;
ALTER TABLE contests ADD COLUMN rated_for_max INTEGER NOT NULL DEFAULT 9999;

-- Phase 2: Virtual Contests
CREATE TABLE virtual_contests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    original_contest_id UUID NOT NULL REFERENCES contests(id),
    user_id UUID NOT NULL REFERENCES users(id),
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    duration_minutes INTEGER NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active'
);

-- Phase 2: Gym
CREATE TABLE gym_contests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contest_id UUID NOT NULL REFERENCES contests(id),
    difficulty_rating INTEGER,
    category VARCHAR(64),
    country VARCHAR(64),
    season VARCHAR(32),
    is_public BOOLEAN NOT NULL DEFAULT true,
    created_by UUID NOT NULL REFERENCES users(id)
);

-- Phase 3: Hacking
CREATE TABLE hacks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contest_id UUID NOT NULL REFERENCES contests(id),
    problem_id UUID NOT NULL REFERENCES problems(id),
    hacker_id UUID NOT NULL REFERENCES users(id),
    defender_id UUID NOT NULL REFERENCES users(id),
    submission_id UUID NOT NULL REFERENCES submissions(id),
    test_input TEXT NOT NULL,
    expected_output TEXT,
    actual_output TEXT,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    success BOOLEAN,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Phase 3: Notifications
CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    type VARCHAR(32) NOT NULL,
    title VARCHAR(256) NOT NULL,
    content TEXT,
    link VARCHAR(512),
    read BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Phase 4: Groups
CREATE TABLE groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(128) NOT NULL,
    description TEXT,
    is_public BOOLEAN NOT NULL DEFAULT true,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE group_members (
    group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id),
    role VARCHAR(16) NOT NULL DEFAULT 'member',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (group_id, user_id)
);

-- Phase 4: Teams
CREATE TABLE teams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(128) NOT NULL,
    description TEXT,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE team_members (
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id),
    role VARCHAR(16) NOT NULL DEFAULT 'member',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (team_id, user_id)
);

-- Phase 4: Blog
CREATE TABLE blog_posts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    title VARCHAR(256) NOT NULL,
    content TEXT NOT NULL,
    tags TEXT[] NOT NULL DEFAULT '{}',
    upvotes INTEGER NOT NULL DEFAULT 0,
    downvotes INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    parent_type VARCHAR(16) NOT NULL,
    parent_id UUID NOT NULL,
    content TEXT NOT NULL,
    upvotes INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Phase 5: Editorials
CREATE TABLE editorials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    problem_id UUID NOT NULL REFERENCES problems(id),
    user_id UUID NOT NULL REFERENCES users(id),
    title VARCHAR(256) NOT NULL,
    content TEXT NOT NULL,
    is_official BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Phase 6: API Keys
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(128),
    rate_limit INTEGER NOT NULL DEFAULT 100,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ
);
```

---

## Implementation Order

### Recommended Sequence

```
Week 1-2:  Rating System (foundation for everything else)
Week 3:    Contest Registration + Division System
Week 4:    Problem Filtering
Week 5-6:  Virtual Contests
Week 7:    Gym/Training
Week 8:    Educational Rounds
Week 9:    Upsolving
Week 10-11: Hacking System
Week 12:   Problem Statistics
Week 13:   Notifications
Week 14-15: Groups
Week 16:   Teams
Week 17-18: Blog/Discussions
Week 19:   Editorials
Week 20:   Public API + Rate Limiting
Week 21:   Internationalization + PWA
Week 22:   Performance + Monitoring
```

---

## Success Criteria

### Phase 1 Complete When:
- [ ] Users have colored ratings that change after contests
- [ ] Users can register for contests
- [ ] Contests have divisions (1/2/3/4)
- [ ] Problems can be filtered by tags, difficulty, solved status

### Phase 2 Complete When:
- [ ] Users can participate in virtual contests
- [ ] Gym has community contests with training filters
- [ ] Educational rounds have extended hacking phases
- [ ] Full test feedback available for upsolving

### Phase 3 Complete When:
- [ ] Users can hack solutions during/after contests
- [ ] Problems show detailed statistics
- [ ] Users receive real-time notifications

### Phase 4 Complete When:
- [ ] Users can create/join groups
- [ ] Teams can participate in contests
- [ ] Users can write blog posts and comment

### Phase 5 Complete When:
- [ ] Editorials published for problems
- [ ] Users get personalized recommendations

### Phase 6 Complete When:
- [ ] Public API documented and rate-limited
- [ ] Webhooks available for integrations

### Phase 7 Complete When:
- [ ] UI available in multiple languages
- [ ] PWA works offline
- [ ] Performance optimized for scale

---

## Risk Mitigation

### Technical Risks
1. **Rating calculation complexity** → Use proven Elo implementation, test with historical data
2. **Virtual contest state management** → Use Redis for ephemeral state
3. **Hacking system abuse** → Rate limit hacks, require minimum rating
4. **Scale issues** → Add caching layer early, optimize queries

### Schedule Risks
1. **Scope creep** → Each sub-plan is independent, can ship incrementally
2. **Dependencies** → Rating system is foundation, must complete first
3. **Testing** → TDD approach ensures quality, prevents regressions

---

## Notes for Implementers

1. **Follow existing patterns** - Use the same handler/store/model structure
2. **TDD everywhere** - Write failing test first, then implement
3. **Migrations are additive** - Never modify existing migrations
4. **Frontend matches backend** - Each API endpoint needs corresponding UI
5. **Commit frequently** - One commit per completed step
