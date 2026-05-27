import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'

const difficultyColor: Record<string, string> = {
    easy: 'text-green-600 bg-green-50',
    medium: 'text-yellow-700 bg-yellow-50',
    hard: 'text-red-600 bg-red-50',
}

export default function ProblemList() {
    const [problems, setProblems] = useState<any[]>([])
    const [total, setTotal] = useState(0)
    const [offset, setOffset] = useState(0)
    const limit = 20

    useEffect(() => {
        api.problems.list(offset, limit).then(d => {
            setProblems(d.data)
            setTotal(d.total)
        }).catch(() => {})
    }, [offset])

    return (
        <div>
            <h1 className="text-2xl font-bold mb-4">Problems</h1>
            <div className="border border-gray-200 rounded-lg overflow-hidden">
                <table className="w-full text-sm">
                    <thead className="bg-gray-50 text-gray-500 text-xs uppercase">
                        <tr>
                            <th className="px-4 py-3 text-left">Title</th>
                            <th className="px-4 py-3 text-left">Difficulty</th>
                            <th className="px-4 py-3 text-left">Tags</th>
                            <th className="px-4 py-3 text-right">Accepted</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-100">
                        {problems.map(p => (
                            <tr key={p.slug} className="hover:bg-gray-50 transition-colors">
                                <td className="px-4 py-3">
                                    <Link to={`/problems/${p.slug}`} className="font-medium text-blue-600 hover:underline">
                                        {p.title}
                                    </Link>
                                </td>
                                <td className="px-4 py-3">
                                    <span className={`px-2 py-0.5 rounded text-xs font-medium ${difficultyColor[p.difficulty] || ''}`}>
                                        {p.difficulty}
                                    </span>
                                </td>
                                <td className="px-4 py-3 text-gray-500">
                                    {p.tags?.join(', ') || '—'}
                                </td>
                                <td className="px-4 py-3 text-right text-gray-500">
                                    {p.accepted_count}/{p.submission_count}
                                </td>
                            </tr>
                        ))}
                        {problems.length === 0 && (
                            <tr><td colSpan={4} className="px-4 py-8 text-center text-gray-400">No problems yet.</td></tr>
                        )}
                    </tbody>
                </table>
            </div>
            <div className="flex justify-between mt-4 text-sm text-gray-500">
                <span>{total} problems</span>
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