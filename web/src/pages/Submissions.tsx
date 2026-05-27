import { useEffect, useState } from 'react'
import { api } from '../lib/api'

const STATUS_COLORS: Record<string, string> = {
    ac: 'text-green-600', wa: 'text-red-600', tle: 'text-yellow-600',
    mle: 'text-orange-600', re: 'text-red-700', ce: 'text-purple-600',
    pending: 'text-blue-500', judging: 'text-blue-600', se: 'text-gray-600',
}

export default function Submissions() {
    const [subs, setSubs] = useState<any[]>([])
    const [total, setTotal] = useState(0)
    const [offset, setOffset] = useState(0)
    const limit = 20

    useEffect(() => {
        api.submissions.list(offset, limit).then(d => {
            setSubs(d.data || [])
            setTotal(d.total || 0)
        }).catch(console.error)
    }, [offset])

    return (
        <div>
            <h1 className="text-2xl font-bold mb-4">My Submissions</h1>
            <div className="border border-gray-200 rounded-lg overflow-hidden">
                <table className="w-full text-sm">
                    <thead className="bg-gray-50 text-gray-500 text-xs uppercase">
                        <tr>
                            <th className="px-4 py-3 text-left">ID</th>
                            <th className="px-4 py-3 text-left">Language</th>
                            <th className="px-4 py-3 text-left">Verdict</th>
                            <th className="px-4 py-3 text-left">Time</th>
                            <th className="px-4 py-3 text-left">Memory</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-100">
                        {subs.map(s => (
                            <tr key={s.id} className="hover:bg-gray-50">
                                <td className="px-4 py-3 font-mono text-xs">{s.id?.substring(0, 8)}...</td>
                                <td className="px-4 py-3 text-gray-500">{s.language}</td>
                                <td className="px-4 py-3 font-semibold">
                                    <span className={STATUS_COLORS[s.status] || ''}>{s.status}</span>
                                </td>
                                <td className="px-4 py-3 text-gray-500">{s.time_used > 0 ? `${s.time_used}ms` : '—'}</td>
                                <td className="px-4 py-3 text-gray-500">{s.memory_used > 0 ? `${Math.round(s.memory_used / 1024)}MB` : '—'}</td>
                            </tr>
                        ))}
                        {subs.length === 0 && (
                            <tr><td colSpan={5} className="px-4 py-8 text-center text-gray-400">No submissions yet.</td></tr>
                        )}
                    </tbody>
                </table>
            </div>
            <div className="flex justify-between mt-4 text-sm text-gray-500">
                <span>{total} submissions</span>
                <div className="flex gap-2">
                    <button onClick={() => setOffset(Math.max(0, offset - limit))} disabled={offset === 0}
                        className="px-3 py-1 border rounded disabled:opacity-40 hover:bg-gray-50">Prev</button>
                    <button onClick={() => setOffset(offset + limit)} disabled={offset + limit >= total}
                        className="px-3 py-1 border rounded disabled:opacity-40 hover:bg-gray-50">Next</button>
                </div>
            </div>
        </div>
    )
}
