import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import { RefreshCw, RotateCcw } from 'lucide-react'

interface PendingSub {
    id: string
    remote_id: string
    bot_id: string
    bot_slug: string
    status: string
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
                    <p className="text-sm text-gray-500 mt-1">
                        Submissions waiting for Codeforces verdict. Auto-polls every 5s.
                    </p>
                </div>
                <button onClick={loadSubs} className="flex items-center gap-1.5 bg-gray-100 text-gray-700 px-3 py-1.5 rounded text-sm hover:bg-gray-200">
                    <RefreshCw className="w-4 h-4" /> Refresh
                </button>
            </div>

            {loading ? (
                <div className="text-center py-8 text-gray-400">Loading...</div>
            ) : (
                <div className="border border-gray-200 rounded-lg overflow-hidden">
                    <table className="w-full text-sm">
                        <thead className="bg-gray-50 text-gray-500 text-xs uppercase">
                            <tr>
                                <th className="px-4 py-3 text-left">Submission</th>
                                <th className="px-4 py-3 text-left">Remote ID</th>
                                <th className="px-4 py-3 text-left">Bot</th>
                                <th className="px-4 py-3 text-left">Status</th>
                                <th className="px-4 py-3 text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-gray-100">
                            {subs.map(s => (
                                <tr key={s.id}>
                                    <td className="px-4 py-3 font-mono text-xs">{s.id.slice(0, 8)}...</td>
                                    <td className="px-4 py-3 font-mono text-xs">
                                        <a href={`https://codeforces.com/submissions/${s.bot_slug}?submissionId=${s.remote_id}`}
                                           target="_blank" rel="noopener noreferrer"
                                           className="text-blue-600 hover:underline">
                                            {s.remote_id}
                                        </a>
                                    </td>
                                    <td className="px-4 py-3">{s.bot_slug || '-'}</td>
                                    <td className="px-4 py-3">
                                        <span className="px-2 py-0.5 rounded text-xs font-medium bg-yellow-100 text-yellow-700">
                                            {s.status}
                                        </span>
                                    </td>
                                    <td className="px-4 py-3 text-right">
                                        <div className="flex justify-end gap-2">
                                            <button onClick={() => refresh(s.id)}
                                                disabled={refreshing.has(s.id)}
                                                className="text-blue-600 hover:text-blue-800 disabled:opacity-50"
                                                title="Force poll verdict now">
                                                <RefreshCw className={`w-4 h-4 ${refreshing.has(s.id) ? 'animate-spin' : ''}`} />
                                            </button>
                                            <button onClick={() => rejudge(s.id)}
                                                className="text-orange-600 hover:text-orange-800"
                                                title="Re-submit to Codeforces">
                                                <RotateCcw className="w-4 h-4" />
                                            </button>
                                        </div>
                                    </td>
                                </tr>
                            ))}
                            {subs.length === 0 && (
                                <tr><td colSpan={5} className="px-4 py-8 text-center text-gray-400">No pending remote submissions</td></tr>
                            )}
                        </tbody>
                    </table>
                </div>
            )}
        </div>
    )
}
