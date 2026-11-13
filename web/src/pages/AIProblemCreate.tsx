import { Link, useNavigate } from 'react-router-dom'
import { PageHeader } from '../components/ui/PageHeader'
import AIStudioTab from '../components/SetterWorkspace/AIStudioTab'
import { canUseAIStudio, decodeToken } from '../lib/aiJobs'

/**
 * The AI Studio — the single page for AI problem generation.
 *
 * This replaces three surfaces that had grown into four components and four
 * copies of the same role check, status map and poll loop:
 *
 *   /setter/ai        AIProblemCreate  — Studio in new-problem mode
 *   /setter/ai/jobs   AIJobsPage       — a job table with its own filters/polling
 *   /setter/ai/jobs/:id AIJobDetailPage — one job, again with its own polling
 *   the admin tab's   AIJobsPage       — the same table, admin-scoped
 *
 * All of them answered one question: "what is the harness doing, and what do I do
 * about it". The job history is now a disclosure inside the Studio rather than a
 * page you navigate away to, because leaving the Studio to check on a job meant
 * losing the brief you were writing — and the two surfaces were polling the same
 * endpoint independently.
 *
 * The old routes redirect here (see App.tsx), so existing links and bookmarks do
 * not 404.
 */
export default function AIProblemCreate() {
    const navigate = useNavigate()
    const { role } = decodeToken()

    if (!canUseAIStudio(role)) {
        return (
            <div className="mx-auto mt-20 max-w-md text-center text-muted">
                <p className="mb-2 text-lg font-semibold">Access restricted</p>
                <p className="text-sm">Only admins and setters can generate problems.</p>
            </div>
        )
    }

    return (
        <div className="max-w-3xl">
            <PageHeader
                eyebrow={<Link to="/setter" className="hover:text-ink">← Setter workspace</Link>}
                title="AI Studio"
                description="Describe a new problem, or open one and improve part of it. The harness drafts the statement, writes a generator and reference solution, materialises test data, and runs the validation gates. You review the result before anything is published."
            />
            <AIStudioTab onCreated={slug => navigate(`/setter/${slug}`)} showHistory isAdmin={role === 'admin'} />
            <p className="mt-6 space-y-1 border-t border-hairline pt-4 text-xs text-muted-soft">
                <span className="block">• The problem is created as a <strong>draft</strong> — not visible to contestants until you publish it.</span>
                <span className="block">• Every gate (coverage, determinism, samples, limits) must pass before the result can be applied.</span>
                <span className="block">• You can edit every field in the Setter Workspace afterwards.</span>
            </p>
        </div>
    )
}
