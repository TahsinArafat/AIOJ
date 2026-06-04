import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import { RefreshCw, RotateCcw } from 'lucide-react'

interface PendingSub {
    id: string
    remote_id: string
    bot_id: string
    bot_slug: string
    status: string
    platform: string
}

export default function SubmissionsPanel() {
    const [subs, setSubs] = useState<PendingSub[]>([])
    const [loading, setLoading] = useState(true)
    const [refreshing, setRefreshing] = useState<Set<string>>(new Set())

    const loadSubs = () => {
        setLoading(true)
        api.admin.submissions.pendingRemote()
            .then(d => setSubs(d.data || []))
            .catch(console.error)
            .finally(() => setLoading(false))
    }

    useEffect(() => { loadSubs() }, [])

    const rejudge = async (id: string) => {
        if (!confirm('Rejudge this submission? It will be re-submitted to Codeforces.')) return
        try {
            await api.admin.submissions.rejudge(id)
            loadSubs()
        } catch (err: any) {
            alert(err.message)
        }
    }

    const refresh = async (id: string) => {
        setRefreshing(prev => new Set(prev).add(id))
        try {
            await api.admin.submissions.refresh(id)
            setTimeout(loadSubs, 5000)
        } catch (err: any) {
            alert(err.message)
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
                    <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                        Submissions waiting for remote OJ verdict. Auto-polls every 5s.
                    </p>
                </div>
                <button onClick={loadSubs} className="flex items-center gap-1.5 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 px-3 py-1.5 rounded text-sm hover:bg-gray-200 dark:hover:bg-gray-600">
                    <RefreshCw className="w-4 h-4" /> Refresh
                </button>
            </div>

            {loading ? (
                <div className="text-center py-8 text-gray-400 dark:text-gray-500">Loading...</div>
            ) : (
                <div className="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
                    <table className="w-full text-sm">
                        <thead className="bg-gray-50 dark:bg-gray-800 text-gray-500 dark:text-gray-400 text-xs uppercase">
                            <tr>
                                <th className="px-4 py-3 text-left">Submission</th>
                                <th className="px-4 py-3 text-left">Remote ID</th>
                                <th className="px-4 py-3 text-left">Bot</th>
                                <th className="px-4 py-3 text-left">Status</th>
                                <th className="px-4 py-3 text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-gray-100 dark:divide-gray-700">
                            {subs.map(s => (
                                <tr key={s.id}>
                                    <td className="px-4 py-3 font-mono text-xs">{s.id.slice(0, 8)}...</td>
                                    <td className="px-4 py-3 font-mono text-xs">
                                        {s.platform === 'codeforces' ? (
                                            <a href={`https://codeforces.com/submissions/${s.bot_slug}?submissionId=${s.remote_id}`}
                                               target="_blank" rel="noopener noreferrer"
                                               className="text-blue-600 dark:text-blue-400 hover:underline">
                                                {s.remote_id}
                                            </a>
                                        ) : s.platform === 'atcoder' ? (
                                            <a href={s.remote_id.includes('/') 
                                                ? `https://atcoder.jp/contests/${s.remote_id.split('/')[0]}/submissions/${s.remote_id.split('/')[1]}`
                                                : `https://atcoder.jp/contests/abc300/submissions/me`}
                                               target="_blank" rel="noopener noreferrer"
                                               className="text-blue-600 dark:text-blue-400 hover:underline">
                                                {s.remote_id}
                                            </a>
                                        ) : s.platform === 'cses' ? (
                                            <a href={`https://cses.fi/problemset/result/${s.remote_id}/`}
                                               target="_blank" rel="noopener noreferrer"
                                               className="text-blue-600 dark:text-blue-400 hover:underline">
                                                {s.remote_id}
                                            </a>
                                        ) : s.platform === 'toph' ? (
                                            <a href={`https://toph.co/s/${s.remote_id}`}
                                               target="_blank" rel="noopener noreferrer"
                                               className="text-blue-600 dark:text-blue-400 hover:underline">
                                                {s.remote_id}
                                            </a>
                                        ) : s.platform === 'qoj' ? (
                                            <a href={`https://qoj.ac/submission/${s.remote_id}`}
                                               target="_blank" rel="noopener noreferrer"
                                               className="text-blue-600 dark:text-blue-400 hover:underline">
                                                {s.remote_id}
                                            </a>
                                        ) : (
                                            <span>{s.remote_id}</span>
                                        )}
                                    </td>
                                    <td className="px-4 py-3">{s.bot_slug || '-'}</td>
                                    <td className="px-4 py-3">
                                        <span className="px-2 py-0.5 rounded text-xs font-medium bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-300">
                                            {s.status}
                                        </span>
                                    </td>
                                    <td className="px-4 py-3 text-right">
                                        <div className="flex justify-end gap-2">
                                            <button onClick={() => refresh(s.id)}
                                                disabled={refreshing.has(s.id)}
                                                className="text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 disabled:opacity-50"
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
                                <tr><td colSpan={5} className="px-4 py-8 text-center text-gray-400 dark:text-gray-500">No pending remote submissions</td></tr>
                            )}
                        </tbody>
                    </table>
                </div>
            )}
        </div>
    )
}
