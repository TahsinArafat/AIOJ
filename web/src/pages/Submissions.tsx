import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'

const STATUS_COLORS: Record<string, string> = {
    ac: 'text-green-600 dark:text-green-400 bg-green-50 dark:bg-green-900/20', wa: 'text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20', tle: 'text-yellow-600 dark:text-yellow-400 bg-yellow-50 dark:bg-yellow-900/20',
    mle: 'text-orange-600 dark:text-orange-400 bg-orange-50 dark:bg-orange-900/20', re: 'text-red-700 dark:text-red-300 bg-red-50 dark:bg-red-900/20', ce: 'text-purple-600 dark:text-purple-400 bg-purple-50 dark:bg-purple-900/20',
    pending: 'text-blue-500 dark:text-blue-400 bg-blue-50 dark:bg-blue-900/20', judging: 'text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-900/20', se: 'text-gray-600 dark:text-gray-400 bg-gray-50 dark:bg-gray-800',
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
            <div className="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
                <table className="w-full text-sm">
                    <thead className="bg-gray-50 dark:bg-gray-800 text-gray-500 dark:text-gray-400 text-xs uppercase">
                        <tr>
                            <th className="px-4 py-3 text-left">ID</th>
                            <th className="px-4 py-3 text-left">Language</th>
                            <th className="px-4 py-3 text-left">Verdict</th>
                            <th className="px-4 py-3 text-left">Time</th>
                            <th className="px-4 py-3 text-left">Memory</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-100 dark:divide-gray-700">
                        {subs.map(s => (
                            <tr key={s.id} className="hover:bg-gray-50 dark:hover:bg-gray-700">
                                <td className="px-4 py-3 font-mono text-xs">
                                    <Link to={`/submissions/${s.id}`} className="text-blue-600 dark:text-blue-400 hover:underline">
                                        {s.id?.substring(0, 8)}...
                                    </Link>
                                </td>
                                <td className="px-4 py-3 text-gray-500 dark:text-gray-400">{s.language}</td>
                                <td className="px-4 py-3 font-semibold">
                                    <span className={`px-2 py-0.5 rounded text-xs font-medium ${STATUS_COLORS[s.status] || ''}`}>
                                        {STATUS_LABEL[s.status] || s.status}
                                    </span>
                                </td>
                                <td className="px-4 py-3 text-gray-500 dark:text-gray-400">{s.time_used > 0 ? `${s.time_used}ms` : '—'}</td>
                                <td className="px-4 py-3 text-gray-500 dark:text-gray-400">{s.memory_used > 0 ? `${Math.round(s.memory_used / 1024)}MB` : '—'}</td>
                            </tr>
                        ))}
                        {subs.length === 0 && (
                            <tr><td colSpan={5} className="px-4 py-8 text-center text-gray-400 dark:text-gray-500">No submissions yet.</td></tr>
                        )}
                    </tbody>
                </table>
            </div>
            <div className="flex justify-between mt-4 text-sm text-gray-500 dark:text-gray-400">
                <span>{total} submissions</span>
                <div className="flex gap-2">
                    <button onClick={() => setOffset(Math.max(0, offset - limit))} disabled={offset === 0}
                        className="px-3 py-1 border rounded disabled:opacity-40 hover:bg-gray-50 dark:hover:bg-gray-700">Prev</button>
                    <button onClick={() => setOffset(offset + limit)} disabled={offset + limit >= total}
                        className="px-3 py-1 border rounded disabled:opacity-40 hover:bg-gray-50 dark:hover:bg-gray-700">Next</button>
                </div>
            </div>
        </div>
    )
}
