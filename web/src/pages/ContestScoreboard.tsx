import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api } from '../lib/api'

export default function ContestScoreboard() {
    const { id } = useParams<{ id: string }>()
    const [data, setData] = useState<any>(null)
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        if (!id) return
        api.contests.scoreboard(id)
            .then(setData).catch(() => {}).finally(() => setLoading(false))
    }, [id])

    if (loading) return <div className="text-center py-20 text-gray-400">Loading scoreboard...</div>
    if (!data) return <div className="text-center py-20 text-gray-400">Failed to load.</div>

    const { entries, problems, frozen, contest } = data

    return (
        <div>
            <div className="flex items-center gap-3 mb-4">
                <Link to={`/contests/${id}`} className="text-sm text-blue-600 hover:underline">← {contest?.title}</Link>
                <h1 className="text-2xl font-bold">Scoreboard</h1>
            </div>

            {frozen && (
                <div className="bg-yellow-50 border border-yellow-200 text-yellow-800 px-4 py-2 rounded-lg mb-4 text-sm">
                    ⚠️ Scoreboard is frozen. Results after freeze time are hidden until contest ends.
                </div>
            )}

            <div className="overflow-x-auto">
                <table className="w-full text-sm border-collapse">
                    <thead>
                        <tr className="bg-gray-100">
                            <th className="border border-gray-200 px-3 py-2 text-left w-12">Rank</th>
                            <th className="border border-gray-200 px-3 py-2 text-left">User</th>
                            <th className="border border-gray-200 px-3 py-2 text-center w-16">Solved</th>
                            <th className="border border-gray-200 px-3 py-2 text-center w-20">Penalty</th>
                            {problems?.map((p: any) => (
                                <th key={p.problem_id} className="border border-gray-200 px-2 py-2 text-center w-12 font-bold text-blue-600">
                                    {p.index}
                                </th>
                            ))}
                        </tr>
                    </thead>
                    <tbody>
                        {entries?.map((e: any, i: number) => (
                            <tr key={e.user_id} className={i % 2 === 0 ? 'bg-white' : 'bg-gray-50'}>
                                <td className="border border-gray-200 px-3 py-2 text-gray-500">{e.rank}</td>
                                <td className="border border-gray-200 px-3 py-2 font-medium">
                                    {e.username || e.user_id?.substring(0, 8)}
                                </td>
                                <td className="border border-gray-200 px-3 py-2 text-center font-semibold">{e.total_solved}</td>
                                <td className="border border-gray-200 px-3 py-2 text-center text-gray-500">{e.total_penalty}</td>
                                {problems?.map((p: any) => {
                                    const pr = e.problems?.[p.index]
                                    if (!pr) return <td key={p.problem_id} className="border border-gray-200 px-2 py-2 text-center text-gray-200">—</td>
                                    if (pr.solved) {
                                        return (
                                            <td key={p.problem_id} className="border border-gray-200 px-2 py-2 text-center text-green-600 font-medium text-xs">
                                                +{pr.attempts > 1 ? pr.attempts - 1 : ''}<br />
                                                <span className="text-gray-400">{pr.time}min</span>
                                            </td>
                                        )
                                    }
                                    if (pr.attempts > 0) {
                                        return <td key={p.problem_id} className="border border-gray-200 px-2 py-2 text-center text-red-500 text-xs">-{pr.attempts}</td>
                                    }
                                    return <td key={p.problem_id} className="border border-gray-200 px-2 py-2 text-center text-gray-200">—</td>
                                })}
                            </tr>
                        ))}
                        {(!entries || entries.length === 0) && (
                            <tr>
                                <td colSpan={(problems?.length || 0) + 4} className="px-4 py-8 text-center text-gray-400">
                                    No submissions yet.
                                </td>
                            </tr>
                        )}
                    </tbody>
                </table>
            </div>
        </div>
    )
}
