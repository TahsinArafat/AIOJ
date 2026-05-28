import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'

const STATUS_COLORS: Record<string, string> = {
    ac: 'text-green-600 bg-green-50', wa: 'text-red-600 bg-red-50', tle: 'text-yellow-600 bg-yellow-50',
    mle: 'text-orange-600 bg-orange-50', re: 'text-red-700 bg-red-50', ce: 'text-purple-600 bg-purple-50',
    pending: 'text-blue-500 bg-blue-50', judging: 'text-blue-600 bg-blue-50', se: 'text-gray-600 bg-gray-50',
}

const STATUS_LABEL: Record<string, string> = {
    ac: 'Accepted', wa: 'Wrong Answer', tle: 'Time Limit Exceeded',
    mle: 'Memory Limit Exceeded', re: 'Runtime Error', ce: 'Compile Error',
    pending: 'Pending', judging: 'Judging...', se: 'System Error',
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
                                <td className="px-4 py-3 font-mono text-xs">
                                    <Link to={`/submissions/${s.id}`} className="text-blue-600 hover:underline">
                                        {s.id?.substring(0, 8)}...
                                    </Link>
                                </td>
                                <td className="px-4 py-3 text-gray-500">{s.language}</td>
                                <td className="px-4 py-3 font-semibold">
                                    <span className={`px-2 py-0.5 rounded text-xs font-medium ${STATUS_COLORS[s.status] || ''}`}>
                                        {STATUS_LABEL[s.status] || s.status}
                                    </span>
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
