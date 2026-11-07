import { Link, useNavigate } from 'react-router-dom'
import { PageHeader } from '../components/ui/PageHeader'
import AIStudioTab from '../components/SetterWorkspace/AIStudioTab'
import { buttonClass } from '../components/ui/buttonStyles'

function decodeRole(): string | null {
    const token = localStorage.getItem('access_token')
    if (!token) return null
    try {
        const payload = JSON.parse(atob(token.split('.')[1]))
        return payload.role ?? null
    } catch {
        return null
    }
}

/**
 * Author a brand-new problem with the AI harness.
 *
 * The harness previously only ran against an existing problem because AI Studio
 * lives inside /setter/:slug. Nothing on the backend required that: the job API
 * takes an optional problem_id and ApplyService creates the problem when it is
 * absent. This page is the missing entry point, and it replaces the legacy
 * synchronous generator, which was removed.
 */
export default function AIProblemCreate() {
    const navigate = useNavigate()
    const role = decodeRole()
    const allowed = role === 'admin' || role === 'setter'

    if (!allowed) {
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
                title="New problem with AI"
                description="The harness drafts the statement, writes a generator and reference solution, materialises test data, and runs the validation gates. You review the result before anything is published."
                actions={
                    <Link to="/setter/ai/jobs" className={buttonClass({ variant: 'secondary', size: 'sm' })}>
                        Job history
                    </Link>
                }
            />
            <AIStudioTab onCreated={slug => navigate(`/setter/${slug}`)} />
            <p className="mt-6 space-y-1 border-t border-hairline pt-4 text-xs text-muted-soft">
                <span className="block">• The problem is created as a <strong>draft</strong> — not visible to contestants until you publish it.</span>
                <span className="block">• Every gate (coverage, determinism, samples, limits) must pass before the result can be applied.</span>
                <span className="block">• You can edit every field in the Setter Workspace afterwards.</span>
            </p>
        </div>
    )
}
