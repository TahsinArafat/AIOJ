# AIOJ → Codeforces Alternative: Implementation Plans

## Overview

This directory contains comprehensive implementation plans to transform AIOJ from a basic online judge into a full-featured competitive programming platform comparable to Codeforces.

**Total Estimated Duration:** 22 weeks (5.5 months)

---

## Plan Index

### Master Plan
- [Master Implementation Plan](./2026-05-28-aioj-codeforces-alternative.md) - Overview, database schema, implementation order

### Phase 1: Core Competitive Features (4 weeks)

| Plan | Description | Duration | Status |
|------|-------------|----------|--------|
| [01 - Rating System](./sub-plans/01-rating-system.md) | Elo-based rating, color coding, division eligibility | 2 weeks | ✅ Ready |
| [02 - Contest Registration](./sub-plans/02-contest-registration.md) | Register/unregister, deadlines, participant lists | 1 week | ✅ Ready |
| [03 - Division System](./sub-plans/03-division-system.md) | Div 1/2/3/4, rating-based eligibility | 1 week | ✅ Ready |
| [04 - Problem Filtering](./sub-plans/04-problem-filtering.md) | Tags, difficulty, solved status, rating range | 1 week | ✅ Ready |

### Phase 2: Contest Features (4 weeks)

| Plan | Description | Duration | Status |
|------|-------------|----------|--------|
| [05 - Virtual Contests](./sub-plans/05-virtual-contests.md) | Past contest simulation, ghost participants | 2 weeks | ✅ Ready |
| [06 - Gym/Training](./sub-plans/06-gym-training.md) | Community contests, training filters | 1 week | ✅ Ready |
| [07 - Educational Rounds](./sub-plans/07-educational-rounds.md) | Learning-focused contests, extended hacking | 1 week | ✅ Ready |
| [08 - Upsolving](./sub-plans/08-upsolving.md) | Full feedback after contests, solution comparison | 1 week | ✅ Ready |

### Phase 3: Engagement Features (4 weeks)

| Plan | Description | Duration | Status |
|------|-------------|----------|--------|
| [09 - Hacking System](./sub-plans/09-hacking-system.md) | Counter-test cases, hack scoring, hacking phases | 2 weeks | ✅ Ready |
| [10 - Problem Statistics](./sub-plans/10-problem-statistics.md) | Language distribution, solver stats, difficulty estimation | 1 week | ✅ Ready |
| [11 - Notifications](./sub-plans/11-notifications.md) | Real-time alerts, email digests, preferences | 1 week | ✅ Ready |

### Phase 4: Community Features (4 weeks)

| Plan | Description | Duration | Status |
|------|-------------|----------|--------|
| [12 - Groups](./sub-plans/12-groups.md) | Create/join groups, group contests, discussions | 2 weeks | ✅ Ready |
| [13 - Teams](./sub-plans/13-teams.md) | Team formation, team contests, team rating | 1 week | ✅ Ready |
| [14 - Blog/Discussions](./sub-plans/14-blog-discussions.md) | User posts, comments, problem discussions | 1 week | ✅ Ready |

### Phase 5: Content Features (2 weeks)

| Plan | Description | Duration | Status |
|------|-------------|----------|--------|
| [15 - Editorials](./sub-plans/15-editorials.md) | Official solutions, community contributions | 1 week | ✅ Ready |
| 16 - Problem Recommendations | Personalized problem suggestions | 1 week | Planned |

### Phase 6: Platform Features (2 weeks)

| Plan | Description | Duration | Status |
|------|-------------|----------|--------|
| [17 - Public API](./sub-plans/17-public-api.md) | RESTful API, documentation, SDKs | 1 week | ✅ Ready |
| [18 - Rate Limiting](./sub-plans/18-rate-limiting.md) | Per-user limits, API keys, abuse prevention | 1 week | ✅ Ready |
| [19 - Webhooks](./sub-plans/19-webhooks.md) | Event notifications for integrations | 1 week | ✅ Ready |

### Phase 7: Polish (2 weeks)

| Plan | Description | Duration | Status |
|------|-------------|----------|--------|
| [20 - Internationalization](./sub-plans/20-internationalization.md) | Multi-language UI, RTL support | 1 week | ✅ Ready |
| [21 - Mobile PWA](./sub-plans/21-mobile-pwa.md) | Progressive Web App, offline support | 1 week | ✅ Ready |
| [22 - Performance](./sub-plans/22-performance.md) | Caching, CDN, query optimization | 1 week | ✅ Ready |
| [23 - Monitoring](./sub-plans/23-monitoring.md) | Metrics, logging, alerting | 1 week | ✅ Ready |

---

## Quick Start

### For Implementers

1. **Read the master plan** first to understand the overall architecture
2. **Start with Phase 1** - Rating System is the foundation
3. **Follow TDD** - Write failing tests first, then implement
4. **Commit frequently** - One commit per completed step

### For Reviewers

1. Each sub-plan has a **Verification Checklist** at the end
2. Run `make migrate-up` to apply database migrations
3. Run `go test ./...` to verify backend tests
4. Run `cd web && npm run build` to verify frontend builds

---

## Database Migrations

All migrations are in `internal/store/migrations/`:

| Migration | Description | Phase |
|-----------|-------------|-------|
| 000001_init | Initial schema (users, problems, contests, submissions) | - |
| 000002_setter_collaboration | Permission system for problems/contests | - |
| 000003_rating_system | Rating history, contest divisions | Phase 1 |
| 000004_contest_registration | Registration table, registration fields | Phase 1 |
| 000005_virtual_contests | Virtual contest sessions | Phase 2 |
| 000006_hacking_system | Hacks table, hack phase fields | Phase 3 |
| 000007_groups | Groups, group members, group contests | Phase 4 |
| 000008_notifications | Notifications, preferences | Phase 3 |
| 000009_gym | Gym contests, solves | Phase 2 |
| 000010_educational | Educational rounds, editorials | Phase 2 |
| 000011_teams | Teams, team members, team contests | Phase 4 |
| 000012_blog | Blog posts, comments, votes | Phase 4 |
| 000013_editorials | Problem editorials | Phase 5 |
| 000014_api_keys | API keys for public API | Phase 6 |
| 000015_webhooks | Webhooks for integrations | Phase 6 |
| 000016_performance_indexes | Database performance indexes | Phase 7 |

---

## Key Architecture Decisions

### Backend (Go)
- **Monolithic**: Single binary for simplicity
- **Chi Router**: HTTP routing (existing)
- **PostgreSQL**: Primary database (existing)
- **Redis**: Caching and rate limiting (new)

### Frontend (React)
- **React 19**: UI framework (existing)
- **TypeScript**: Type safety (existing)
- **Tailwind CSS**: Styling (existing)
- **CodeMirror 6**: Code editor (existing)
- **react-i18next**: Internationalization (new)

### Testing
- **TDD**: Test-driven development for all new code
- **Table-driven tests**: Go convention
- **Integration tests**: Database tests with test containers

---

## Success Criteria

### Phase 1 Complete
- [ ] Users have colored ratings that change after contests
- [ ] Users can register for contests
- [ ] Contests have divisions (1/2/3/4)
- [ ] Problems can be filtered by tags, difficulty, solved status

### Phase 2 Complete
- [ ] Users can participate in virtual contests
- [ ] Gym has community contests with training filters
- [ ] Educational rounds have extended hacking phases
- [ ] Full test feedback available for upsolving

### Phase 3 Complete
- [ ] Users can hack solutions during/after contests
- [ ] Problems show detailed statistics
- [ ] Users receive real-time notifications

### Phase 4 Complete
- [ ] Users can create/join groups
- [ ] Teams can participate in contests
- [ ] Users can write blog posts and comment

### Phase 5 Complete
- [ ] Editorials published for problems
- [ ] Users get personalized recommendations

### Phase 6 Complete
- [ ] Public API documented and rate-limited
- [ ] Webhooks available for integrations

### Phase 7 Complete
- [ ] UI available in multiple languages
- [ ] PWA works offline
- [ ] Performance optimized for scale
- [ ] Monitoring and alerting in place

### All Phases Complete
- [ ] AIOJ has feature parity with Codeforces core features
- [ ] Platform can handle 10,000+ concurrent users
- [ ] Public API available for third-party integrations
- [ ] Mobile-friendly PWA available

---

## File Structure Summary

### Backend Files to Create
```
internal/
├── cache/
│   ├── cache.go
│   ├── redis.go
│   └── memory.go
├── hack/
│   └── service.go
├── health/
│   └── health.go
├── logging/
│   └── logger.go
├── metrics/
│   └── metrics.go
├── model/
│   ├── apikey.go
│   ├── blog.go
│   ├── editorial.go
│   ├── group.go
│   ├── gym.go
│   ├── hack.go
│   ├── notification.go
│   ├── rating.go
│   ├── team.go
│   ├── virtual.go
│   └── webhook.go
├── notification/
│   └── service.go
├── rating/
│   ├── calculator.go
│   └── service.go
├── ratelimit/
│   ├── limiter.go
│   ├── middleware.go
│   └── store.go
├── store/
│   └── postgres/
│       ├── apikeys.go
│       ├── blog.go
│       ├── editorials.go
│       ├── groups.go
│       ├── gym.go
│       ├── hacks.go
│       ├── notifications.go
│       ├── rating.go
│       ├── registrations.go
│       ├── teams.go
│       └── virtual.go
├── virtual/
│   └── service.go
├── webhook/
│   ├── delivery.go
│   └── events.go
└── api/
    ├── handler/
    │   ├── blog.go
    │   ├── editorial.go
    │   ├── group.go
    │   ├── gym.go
    │   ├── hack.go
    │   ├── notification.go
    │   ├── rating.go
    │   ├── registration.go
    │   ├── stats.go
    │   ├── team.go
    │   ├── virtual.go
    │   └── webhook.go
    └── public/
        ├── router.go
        └── handlers.go
```

### Frontend Files to Create
```
web/src/
├── components/
│   ├── CommentSection.tsx
│   ├── DivisionBadge.tsx
│   ├── HackResult.tsx
│   ├── LanguageChart.tsx
│   ├── LanguageSwitcher.tsx
│   ├── NotificationBell.tsx
│   ├── NotificationItem.tsx
│   ├── NotificationList.tsx
│   ├── OfflineBanner.tsx
│   ├── ProblemFilters.tsx
│   ├── ProblemStats.tsx
│   ├── RatingBadge.tsx
│   ├── TagSelector.tsx
│   └── VirtualTimer.tsx
├── hooks/
│   └── useOnlineStatus.ts
├── i18n/
│   ├── index.ts
│   └── locales/
│       ├── en.json
│       ├── bn.json
│       └── ru.json
├── lib/
│   ├── divisions.ts
│   └── rating.ts
└── pages/
    ├── APISettings.tsx
    ├── BlogCreate.tsx
    ├── BlogDetail.tsx
    ├── BlogList.tsx
    ├── EditorialCreate.tsx
    ├── EditorialDetail.tsx
    ├── EditorialList.tsx
    ├── GroupCreate.tsx
    ├── GroupDetail.tsx
    ├── GroupList.tsx
    ├── GymDetail.tsx
    ├── GymList.tsx
    ├── HackPanel.tsx
    ├── TeamCreate.tsx
    ├── TeamDetail.tsx
    ├── TeamList.tsx
    ├── VirtualContest.tsx
    └── VirtualContest.tsx
```

---

## Contributing

### Adding New Sub-Plans

1. Create file in `sub-plans/` directory
2. Follow the template structure (see existing plans)
3. Include:
   - File structure (files to create/modify)
   - Bite-sized tasks with checkboxes
   - Exact code in every step
   - Verification checklist
4. Update this README with the new plan

### Template Structure

```markdown
# Sub-Plan XX: Feature Name

> **For agentic workers:** REQUIRED SUB-SKILL: ...

**Goal:** One sentence describing what this builds

**Architecture:** 2-3 sentences about approach

**Tech Stack:** Key technologies

---

## File Structure
...

## Tasks
### Task N: Component Name
**Files:**
- Create: `exact/path/to/file`
- Modify: `exact/path/to/existing`

- [ ] **Step 1: Description**
[code]
- [ ] **Step 2: Verify**
Run: `command`
Expected: result
- [ ] **Step 3: Commit**
```bash
git add files
git commit -m "message"
```

---

## Verification Checklist
...
```

---

## Questions?

For questions about these plans, refer to:
- [Gap Analysis](../analysis/codeforces-gap-analysis.md) - Why these features are needed
- [README](../../README.md) - Project overview
- [USER_GUIDE](../../USER_GUIDE.md) - Current feature documentation
