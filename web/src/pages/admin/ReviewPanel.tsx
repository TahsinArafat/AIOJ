import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../../lib/api'
import type { Contest, ProblemListItem } from '../../types/api'
import { useConfirm } from '../../components/ConfirmDialog'
import { useToast } from '../../components/Toast'

/**
 * The admin side of "setters propose, admins approve".
 *
 * Before this, a setter published straight to the archive and announced rated
 * contests with no review at all. This panel is the only place either becomes
 * public, so it is deliberately one screen: what is waiting, and the decisions
 * that matter — including whether a round counts for ratings, which is the
 * author's proposal until an admin settles it here.
 */
export default function ReviewPanel() {
    const confirm = useConfirm()
    const toast = useToast()
    const [problems, setProblems] = useState<ProblemListItem[]>([])
    const [contests, setContests] = useState<Contest[]>([])
    const [loading, setLoading] = useState(true)

    const load = () => {
        setLoading(true)
        Promise.all([
            api.admin.problemReview.queue('pending').then(d => setProblems(d.data || [])),
            api.admin.contestReview.queue('pending').then(d => setContests(d.data || [])),
        ])
            .catch(e => toast.error(e instanceof Error ? e.message : String(e)))
            .finally(() => setLoading(false))
    }

    // deferred one microtask: load() opens with a synchronous setLoading(true),
    // which react-hooks/set-state-in-effect rejects in an effect body
    useEffect(() => { queueMicrotask(load) }, []) // eslint-disable-line react-hooks/exhaustive-deps

    const decideProblem = async (slug: string, status: string, label: string) => {
        if (!(await confirm({ message: `${label} this problem?`, variant: status === 'rejected' ? 'danger' : 'default' }))) return
        try {
            await api.admin.problemReview.review(slug, status)
            toast.success(`Problem ${status}`)
            load()
        } catch (e) {
            toast.error(e instanceof Error ? e.message : String(e))
        }
    }

    const decideContest = async (id: string, status: string, label: string, isRated?: boolean) => {
        if (!(await confirm({ message: `${label} this contest?`, variant: status === 'rejected' ? 'danger' : 'default' }))) return
        try {
            await api.admin.contestReview.review(id, status, '', isRated)
            toast.success(`Contest ${status}`)
            load()
        } catch (e) {
            toast.error(e instanceof Error ? e.message : String(e))
        }
    }

    if (loading) return <div className="text-center py-8 text-muted-soft">Loading...</div>

    const empty = problems.length === 0 && contests.length === 0

    return (
        <div className="space-y-8">
            <div>
                <h2 className="text-lg font-semibold mb-1">Authoring review</h2>
                <p className="text-sm text-muted mb-4">
                    Problems and contests stay private until approved here. Rejecting sends them back to the author
                    with no note shown publicly; they can fix it and resubmit.
                </p>
            </div>

            {empty && (
                <div className="text-center py-10 text-muted-soft border border-hairline rounded-lg">
                    Nothing is waiting for review.
                </div>
            )}

            {problems.length > 0 && (
                <section>
                    <h3 className="text-sm font-semibold uppercase text-muted mb-2">Problems ({problems.length})</h3>
                    <div className="border border-hairline rounded-lg overflow-hidden">
                        <table className="w-full text-sm">
                            <thead className="bg-surface-2 text-muted text-xs uppercase">
                                <tr>
                                    <th className="px-4 py-3 text-left">Problem</th>
                                    <th className="px-4 py-3 text-left">Difficulty</th>
                                    <th className="px-4 py-3 text-right">Actions</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-hairline">
                                {problems.map(p => (
                                    <tr key={p.id}>
                                        <td className="px-4 py-3">
                                            <Link to={`/problems/${p.slug}`} className="font-medium text-brand hover:underline">
                                                {p.title}
                                            </Link>
                                            <div className="text-xs text-muted-soft font-mono">{p.slug}</div>
                                        </td>
                                        <td className="px-4 py-3 text-xs text-muted">{p.difficulty || '—'}</td>
                                        <td className="px-4 py-3 text-right whitespace-nowrap">
                                            <button
                                                type="button"
                                                onClick={() => decideProblem(p.slug, 'published', 'Publish')}
                                                className="px-3 py-1.5 bg-ok/12 text-ok rounded-lg text-xs font-semibold hover:bg-ok/20"
                                            >
                                                Publish
                                            </button>
                                            <button
                                                type="button"
                                                onClick={() => decideProblem(p.slug, 'rejected', 'Reject')}
                                                className="ml-2 px-3 py-1.5 bg-bad/12 text-bad rounded-lg text-xs font-semibold hover:bg-bad/20"
                                            >
                                                Reject
                                            </button>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                </section>
            )}

            {contests.length > 0 && (
                <section>
                    <h3 className="text-sm font-semibold uppercase text-muted mb-2">Contests ({contests.length})</h3>
                    <div className="border border-hairline rounded-lg overflow-hidden">
                        <table className="w-full text-sm">
                            <thead className="bg-surface-2 text-muted text-xs uppercase">
                                <tr>
                                    <th className="px-4 py-3 text-left">Contest</th>
                                    <th className="px-4 py-3 text-left">Starts</th>
                                    <th className="px-4 py-3 text-left">Rating</th>
                                    <th className="px-4 py-3 text-right">Actions</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-hairline">
                                {contests.map(c => (
                                    <tr key={c.id}>
                                        <td className="px-4 py-3">
                                            <Link to={`/contests/${c.id}`} className="font-medium text-brand hover:underline">
                                                {c.title}
                                            </Link>
                                        </td>
                                        <td className="px-4 py-3 text-xs text-muted">
                                            {c.start_time ? new Date(c.start_time).toLocaleString() : '—'}
                                        </td>
                                        <td className="px-4 py-3 text-xs text-muted">
                                            {/* The proposal, not the current value: is_rated is
                                                what the last verdict left behind, and showing it
                                                here would tell the reviewer a round is rated when
                                                the author asked for the opposite. */}
                                            {c.proposed_rated === false ? 'Unrated' : 'Rated'}
                                            <div className="text-muted-soft">
                                                proposed by author
                                            </div>
                                        </td>
                                        <td className="px-4 py-3 text-right whitespace-nowrap">
                                            <button
                                                type="button"
                                                onClick={() => decideContest(c.id, 'published', 'Publish')}
                                                className="px-3 py-1.5 bg-ok/12 text-ok rounded-lg text-xs font-semibold hover:bg-ok/20"
                                            >
                                                Publish
                                            </button>
                                            {/* Publish as unrated is its own button rather than a
                                                toggle, because the common verdict accepts the
                                                proposal and a checkbox would have to be read before
                                                every click to be trusted. */}
                                            <button
                                                type="button"
                                                onClick={() => decideContest(c.id, 'published', 'Publish as unrated', false)}
                                                className="ml-2 px-3 py-1.5 bg-surface-3 text-muted rounded-lg text-xs font-semibold hover:bg-surface-2"
                                            >
                                                Publish unrated
                                            </button>
                                            <button
                                                type="button"
                                                onClick={() => decideContest(c.id, 'rejected', 'Reject')}
                                                className="ml-2 px-3 py-1.5 bg-bad/12 text-bad rounded-lg text-xs font-semibold hover:bg-bad/20"
                                            >
                                                Reject
                                            </button>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                </section>
            )}
        </div>
    )
}
