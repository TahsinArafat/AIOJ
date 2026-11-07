## AI generation job monitoring (added)
- **Backend**: `AIGenerationStore` interface + postgres impl now has `ListByActor`, `ListAll`, `AdminRequestCancel`
- **Routes**: `GET /api/ai/jobs` (setter list own jobs), `GET /api/admin/ai/jobs` (admin: all, filterable by actor_id/status), `POST /api/admin/ai/jobs/{id}/cancel`
- **Frontend pages**: `web/src/pages/AIJobsPage.tsx` (list, shared setter+admin), `web/src/pages/AIJobDetailPage.tsx` (detail, polling, cancel/retry/discard/apply)
- **Routes wired**: `/setter/ai/jobs` and `/setter/ai/jobs/:id` in App.tsx
- **Admin dashboard**: added "AI Jobs" tab that embeds AIJobsPage with admin role (uses `adminListJobs`)
- **API helpers**: `api.ai.listJobs`, `api.ai.adminListJobs`, `api.ai.adminCancelJob` in `web/src/lib/api.ts`
- **AIStudioTab**: added "Open full status page →" link on the inline job card

## Project conventions
- Backend: chi router, Go, store interface + postgres impl pattern
- Frontend: React, Vite, TypeScript, Vitest for tests
- UI: `Badge`, `Button`, `Card`, `TableShell/Table/Th/Td/THead`, `PageHeader`, `EmptyState` from ui/ components
- Tr component does NOT accept onClick — use native `<tr>` for clickable rows
- Admin dashboard: tab-based with hash sync; add to AdminTab union type + tabs array + valid array + render block + import
- Role checks: decode JWT from localStorage; `decodeRole()` pattern used consistently
