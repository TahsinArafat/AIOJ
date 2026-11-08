import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../../lib/api'
import { RefreshCw, RotateCcw } from 'lucide-react'
import { useConfirm } from '../../components/ConfirmDialog'
import { useToast } from '../../components/Toast'

interface PendingSub {
    id: string
    remote_id: string
    bot_id: string
    bot_slug: string
    status: string
    platform: string
}

interface FailedSub {
    id: string
    username?: string
    problem_slug?: string
    problem_title?: string
    language: string
    judge_error?: string
    created_at: string
    contest_id?: string
}

export default function SubmissionsPanel() {
    const confirm = useConfirm()
    const toast = useToast()
    const [subs, setSubs] = useState<PendingSub[]>([])
    const [loading, setLoading] = useState(true)
    const [refreshing, setRefreshing] = useState<Set<string>>(new Set())
    const [failures, setFailures] = useState<FailedSub[]>([])
    const [failuresLoading, setFailuresLoading] = useState(true)

    const loadSubs = () => {
        setLoading(true)
        api.admin.submissions.pendingRemote()
            .then(d => setSubs(d.data || []))
            .catch(console.error)
            .finally(() => setLoading(false))
    }

    // A system error means the judge itself failed, so the failure text is what
    // explains a "judge system error" report.
    const loadFailures = () => {
        setFailuresLoading(true)
        api.admin.submissions.failed(50)
            .then(d => setFailures(d.data || []))
            .catch(console.error)
            .finally(() => setFailuresLoading(false))
    }

    // deferred one microtask: loadSubs opens with a synchronous setLoading(true),
    // which react-hooks/set-state-in-effect rejects in an effect body
    useEffect(() => {
        queueMicrotask(loadSubs)
        queueMicrotask(loadFailures)
    }, [])

    const rejudge = async (id: string) => {
        if (!(await confirm({ message: 'Rejudge this submission? It will be re-submitted to Codeforces.', variant: 'danger' }))) return
        try {
            await api.admin.submissions.rejudge(id)
            loadSubs()
            loadFailures()
        } catch (err) {
            toast.error(err instanceof Error ? err.message : String(err))
        }
    }

    const refresh = async (id: string) => {
        setRefreshing(prev => new Set(prev).add(id))
        try {
            await api.admin.submissions.refresh(id)
            setTimeout(loadSubs, 5000)
        } catch (err) {
            toast.error(err instanceof Error ? err.message : String(err))
        } finally {
            setRefreshing(prev => {
                const next = new Set(prev)
                next.delete(id)
                return next
            })
        }
    }

    return (
        <div>
            <div className="flex items-center justify-between mb-4">
                <div>
                    <h2 className="text-lg font-semibold">Pending Remote Submissions</h2>
                    <p className="text-sm text-muted mt-1">
                        Submissions waiting for remote OJ verdict. Auto-polls every 5s.
                    </p>
                </div>
                <button onClick={loadSubs} className="flex items-center gap-1.5 bg-surface-3 text-ink-soft px-3 py-1.5 rounded text-sm hover:bg-surface-3">
                    <RefreshCw className="w-4 h-4" /> Refresh
                </button>
            </div>

            {loading ? (
                <div className="text-center py-8 text-muted-soft">Loading...</div>
            ) : (
                <div className="border border-hairline rounded-lg overflow-hidden">
                    <table className="w-full text-sm">
                        <thead className="bg-surface-2 text-muted text-xs uppercase">
                            <tr>
                                <th className="px-4 py-3 text-left">Submission</th>
                                <th className="px-4 py-3 text-left">Remote ID</th>
                                <th className="px-4 py-3 text-left">Bot</th>
                                <th className="px-4 py-3 text-left">Status</th>
                                <th className="px-4 py-3 text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-hairline">
                            {subs.map(s => (
                                <tr key={s.id}>
                                    <td className="px-4 py-3 font-mono text-xs">{s.id.slice(0, 8)}...</td>
                                    <td className="px-4 py-3 font-mono text-xs">
                                        {s.platform === 'codeforces' ? (
                                            <a href={`https://codeforces.com/submissions/${s.bot_slug}?submissionId=${s.remote_id}`}
                                               target="_blank" rel="noopener noreferrer"
                                               className="text-brand hover:underline">
                                                {s.remote_id}
                                            </a>
                                        ) : s.platform === 'atcoder' ? (
                                            <a href={s.remote_id.includes('/') 
                                                ? `https://atcoder.jp/contests/${s.remote_id.split('/')[0]}/submissions/${s.remote_id.split('/')[1]}`
                                                : `https://atcoder.jp/contests/abc300/submissions/me`}
                                               target="_blank" rel="noopener noreferrer"
                                               className="text-brand hover:underline">
                                                {s.remote_id}
                                            </a>
                                        ) : s.platform === 'cses' ? (
                                            <a href={`https://cses.fi/problemset/result/${s.remote_id}/`}
                                               target="_blank" rel="noopener noreferrer"
                                               className="text-brand hover:underline">
                                                {s.remote_id}
                                            </a>
                                        ) : s.platform === 'toph' ? (
                                            <a href={`https://toph.co/s/${s.remote_id}`}
                                               target="_blank" rel="noopener noreferrer"
                                               className="text-brand hover:underline">
                                                {s.remote_id}
                                            </a>
                                        ) : s.platform === 'qoj' ? (
                                            <a href={`https://qoj.ac/submission/${s.remote_id}`}
                                               target="_blank" rel="noopener noreferrer"
                                               className="text-brand hover:underline">
                                                {s.remote_id}
                                            </a>
                                        ) : (
                                            <span>{s.remote_id}</span>
                                        )}
                                    </td>
                                    <td className="px-4 py-3">{s.bot_slug || '-'}</td>
                                    <td className="px-4 py-3">
                                        <span className="px-2 py-0.5 rounded text-xs font-medium bg-warn/12 text-warn">
                                            {s.status}
                                        </span>
                                    </td>
                                    <td className="px-4 py-3 text-right">
                                        <div className="flex justify-end gap-2">
                                            <button onClick={() => refresh(s.id)}
                                                disabled={refreshing.has(s.id)}
                                                className="text-brand hover:text-brand disabled:opacity-50"
                                                title="Force poll verdict now">
                                                <RefreshCw className={`w-4 h-4 ${refreshing.has(s.id) ? 'animate-spin' : ''}`} />
                                            </button>
                                            <button onClick={() => rejudge(s.id)}
                                                className="text-orange-600 dark:text-orange-400 hover:text-orange-800 dark:hover:text-orange-300"
                                                title="Re-submit to remote OJ">
                                                <RotateCcw className="w-4 h-4" />
                                            </button>
                                        </div>
                                    </td>
                                </tr>
                            ))}
                            {subs.length === 0 && (
                                <tr><td colSpan={5} className="px-4 py-8 text-center text-muted-soft">No pending remote submissions</td></tr>
                            )}
                        </tbody>
                    </table>
                </div>
            )}

            <div className="mt-10">
                <div className="flex items-center justify-between mb-4">
                    <div>
                        <h2 className="text-lg font-semibold">Judge failures</h2>
                        <p className="text-sm text-muted mt-1">
                            Submissions that ended in a system error, with the judge's own failure text.
                        </p>
                    </div>
                    <button onClick={loadFailures} className="flex items-center gap-1.5 bg-surface-3 text-ink-soft px-3 py-1.5 rounded text-sm hover:bg-surface-3">
                        <RefreshCw className="w-4 h-4" /> Refresh
                    </button>
                </div>

                {failuresLoading ? (
                    <div className="text-center py-8 text-muted-soft">Loading...</div>
                ) : (
                    <div className="border border-hairline rounded-lg overflow-hidden">
                        <table className="w-full text-sm">
                            <thead className="bg-surface-2 text-muted text-xs uppercase">
                                <tr>
                                    <th className="px-4 py-3 text-left">Submission</th>
                                    <th className="px-4 py-3 text-left">Who</th>
                                    <th className="px-4 py-3 text-left">Problem</th>
                                    <th className="px-4 py-3 text-left">Judge error</th>
                                    <th className="px-4 py-3 text-left">When</th>
                                    <th className="px-4 py-3 text-right">Actions</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-hairline">
                                {failures.map(f => (
                                    <tr key={f.id} className="align-top">
                                        <td className="px-4 py-3 font-mono text-xs">
                                            <Link to={`/submissions/${f.id}`} className="text-brand hover:underline">
                                                {f.id.slice(0, 8)}
                                            </Link>
                                        </td>
                                        <td className="px-4 py-3 text-xs">{f.username || '—'}</td>
                                        <td className="px-4 py-3 text-xs">
                                            {f.problem_slug ? (
                                                <Link to={`/problems/${f.problem_slug}`} className="text-brand hover:underline">
                                                    {f.problem_title || f.problem_slug}
                                                </Link>
                                            ) : '—'}
                                        </td>
                                        <td className="px-4 py-3">
                                            <pre className="max-w-xl overflow-x-auto font-mono text-[11px] leading-relaxed whitespace-pre-wrap text-bad">
                                                {f.judge_error || 'No error text recorded'}
                                            </pre>
                                        </td>
                                        <td className="px-4 py-3 text-xs text-muted-soft whitespace-nowrap">
                                            {new Date(f.created_at).toLocaleString()}
                                        </td>
                                        <td className="px-4 py-3 text-right">
                                            <button onClick={() => rejudge(f.id)}
                                                className="text-orange-600 dark:text-orange-400 hover:text-orange-800 dark:hover:text-orange-300"
                                                title="Rejudge this submission">
                                                <RotateCcw className="w-4 h-4" />
                                            </button>
                                        </td>
                                    </tr>
                                ))}
                                {failures.length === 0 && (
                                    <tr><td colSpan={6} className="px-4 py-8 text-center text-muted-soft">No judge failures recorded</td></tr>
                                )}
                            </tbody>
                        </table>
                    </div>
                )}
            </div>
        </div>
    )
}
