# ESLint Clean-Sweep (clean files) Implementation Plan

> **For agentic workers:** Steps use checkbox (`- [ ]`) syntax for tracking. Execute tasks in order within their batch.

**Goal:** Drive `eslint .` to zero problems in every file NOT owned by the parallel session's in-flight change (55 files, 205 problems: 162 `@typescript-eslint/no-explicit-any` + 43 other errors), without behavior changes and without touching their 25 dirty files.

**Architecture:** Four disjoint worker batches on clean files only; the parent session owns verification gates (eslint recount, `tsc -p tsconfig.app.json`, vitest, vite build), commits, and push. Fixes are type-level only: precise annotations, local interfaces, `unknown` + narrowing, `instanceof Error` catches. No `any` may remain or be introduced.

**Tech Stack:** ESLint 9 flat config (`typescript-eslint` recommended + react-hooks + react-refresh), TypeScript (non-strict, `noUnusedLocals`/`noUnusedParameters` ON), React 19, Vitest.

**Spec:** This plan (scope = lint hygiene; no feature spec).

## Global Constraints

- NEVER edit these parallel-session files (they hold 335 of the 540 problems): `src/lib/api.ts`, `pages/{ClassDetail,ContestList,ContestManage,ContestPlagiarism,ContestProblem,ContestScoreboard,EditorialList,GroupDetail,GroupList,GymList,OrganizationDetail,OrganizationList,ProblemList,Profile,RatingHistory,SetterPanel,SetterProblemWorkspace,TeamDetail,TeamList,TrainingPlanDetail,TrainingPlanList,VirtualContest}.tsx`, `pages/admin/{BotAccountsPanel,RemoteLanguagesPanel}.tsx`, and untracked `src/lib/errors.ts`, `src/types/community.ts`.
- Each fix must preserve runtime behavior exactly (no effect-dep changes unless proven safe; mount-only effects may use the established `// eslint-disable-next-line react-hooks/exhaustive-deps` pattern already used in `CodeEditor.tsx`/`UserPublicProfile.tsx`).
- Never write `any`, `@ts-ignore`, `@ts-expect-error`, or eslint-disable for `no-explicit-any`.
- Only annotate inside your file: `const d: MyType = await api…` is legal because `any` is assignable. Types that don't exist yet are declared locally in your file (or in an existing shared type file you already own in this batch) — do NOT create/edit files outside your batch list.
- Baseline `tsc` errors (owned by parallel session, tolerated): `ClassDetail.tsx(99,79)`, `OrganizationDetail.tsx(180,79)`, `TrainingPlanDetail.tsx(78,13)`, `TrainingPlanDetail.tsx(78,71)` — no others may exist after your batch.
- Verification per batch: `cd web && npx eslint <your files>` → `0 problems`. Do not run tests/commits (parent gates those).

## Canonical fix patterns

- **State/callback any:** `useState<any[]>([])` → `useState<StandingsRow[]>([])` with a local `interface StandingsRow { … }` matching field usage in the file.
- **Catch any:** `catch (e: any) { toast.error(e.message) }` → `catch (e) { toast.error(e instanceof Error ? e.message : String(e)) }`.
- **Cast any:** `(window as any).grecaptcha` → declare `declare global { interface Window { grecaptcha?: … } }` or `const recaptcha = (window as unknown as { grecaptcha?: Recaptcha })…`.
- **Test mock any:** `(api.x.y as Mock).mockResolvedValue(...)` stays if `Mock` is imported from `vitest`; if written as `as any`, switch to `as Mock`.
- **Unused vars (noUnusedLocals):** delete the binding (never underscore-ignore via any).
- **no-empty blocks:** give the block a comment (`// intentionally empty`) or restructure to `?.` so the block disappears.
- **react-refresh/only-export-components:** move non-component exports to a sibling file only if trivial; otherwise ensure component + non-component exports are separated per the plugin's inline allow constant pattern used by the repo — if unclear, keep exports and report.

### Task 1: Heavy pages batch

**Files (3):** `web/src/pages/ContestDetail.tsx` (33 any + 4 other), `web/src/pages/ProblemDetail.tsx` (19+5), `web/src/pages/admin/LanguagesPanel.tsx` (17+4)

- [ ] Read each file, inventory every eslint finding (`npx eslint <file>`).
- [ ] Apply canonical patterns until `npx eslint` reports 0 problems for all three files.
- [ ] Self-check: no new cross-file edits; report any finding you could not fix without another file.

### Task 2: Mid pages/components batch

**Files (11):** `APISettings.tsx` 7, `SubmissionDetail.tsx` 7, `admin/SystemSettingsPanel.tsx` 5+1, `App.tsx` 4+1, `TrainingPlanCreate.tsx` 5, `admin/AIModelsPanel.tsx` 4+1, `admin/BackupsPanel.tsx` 4+1, `components/CommentSection.tsx` 3+1, `components/ProblemStats.tsx` 4, `pages/BlogDetail.tsx` 4, `pages/ContestCreate.tsx` 3+1

- [ ] Same loop per file → all clean.

### Task 3: Mid-small batch

**Files (18):** `components/SetterWorkspace/EditorialTab.tsx` 1+1, `components/Toast.tsx` 0+3, `pages/GroupJoin.tsx` 2+1, `pages/HackPanel.tsx` 3, `pages/Practice.tsx` 2+1, `pages/admin/SetterAppsPanel.tsx` 2+1, `pages/admin/SubmissionsPanel.tsx` 2+1, `pages/admin/UsersPanel.tsx` 2+1, `components/ActivityFeed.test.tsx` 2, `components/GlobalSearch.tsx` 0+2, `components/Navbar.tsx` 2, `components/SetterWorkspace/TranslationsTab.tsx` 0+2, `components/VisualEditor/MathDialog.tsx` 1+1, `pages/GymDetail.tsx` 2, `pages/Rankings.tsx` 1+1, `pages/UserPublicProfile.tsx` 0+2, `pages/VerifyEmail.tsx` 1+1, `pages/auth/TwoFactorSetup.tsx` 2

- [ ] Same loop per file → all clean.

### Task 4: Tail batch

**Files (23):** `components/{ConfirmDialog,CookieConsent,CountryFlag,EditorialForm,NotificationBell,SetterApplication}.tsx`, `components/VisualEditor/MathExtension.tsx`, `context/ThemeContext.tsx`, `lib/api.refresh.test.ts`, `pages/{BlogCreate,BlogList,EditorialDetail,ForgotPassword,GenerateProblem,GroupCreate,IDE,NotificationPreferences,OrganizationCreate,ProblemCreate,ResetPassword,Submissions,TeamCreate}.tsx`, `pages/auth/TwoFactorVerify.tsx` — each 0–1 problem.

- [ ] Same loop per file → all clean.

### Task 5: Parent gates

- [ ] `cd web && npx eslint .` → only parallel-session files remain (expect ~335).
- [ ] `npx tsc -p tsconfig.app.json` → only the four baseline errors.
- [ ] `npm test -- --run` → all green.
- [ ] `npx vite build` → succeeds.
- [ ] Commit in logical chunks, push, update memory.
