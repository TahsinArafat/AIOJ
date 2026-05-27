import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'

export default function ContestDetail() {
    const { id } = useParams<{ id: string }>()
    const [data, setData] = useState<any>(null)
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        if (!id) return
        fetch(`/api/contests/${id}`).then(r => r.json()).then(setData).catch(() => {}).finally(() => setLoading(false))
    }, [id])

    if (loading) return <div className="text-center py-20 text-gray-400">Loading...</div>
    if (!data) return <div className="text-center py-20 text-gray-400">Contest not found.</div>

    const { contest, problems } = data
    const now = Date.now()
    const start = new Date(contest.start_time).getTime()
    const end = new Date(contest.end_time).getTime()
    const isRunning = now >= start && now <= end
    const isUpcoming = now < start
    const isEnded = now > end

    return (
        <div className="space-y-6">
            <div className="flex items-start justify-between">
                <div>
                    <h1 className="text-2xl font-bold">{contest.title}</h1>
                    {contest.description && <p className="text-gray-600 mt-1">{contest.description}</p>}
                    <div className="mt-3 text-sm text-gray-500 space-y-1">
                        <div>Start: <span className="text-gray-700">{new Date(contest.start_time).toLocaleString()}</span></div>
                        <div>End: <span className="text-gray-700">{new Date(contest.end_time).toLocaleString()}</span></div>
                        {contest.freeze_time && (
                            <div>Freeze: <span className="text-gray-700">{new Date(contest.freeze_time).toLocaleString()}</span></div>
                        )}
                        <div>Type: <span className="text-gray-700 uppercase font-medium">{contest.type}</span></div>
                    </div>
                </div>
                <div>
                    {isRunning && <span className="text-green-600 bg-green-50 px-3 py-1 rounded font-medium text-sm">Running</span>}
                    {isUpcoming && <span className="text-blue-600 bg-blue-50 px-3 py-1 rounded font-medium text-sm">Upcoming</span>}
                    {isEnded && <span className="text-gray-500 bg-gray-100 px-3 py-1 rounded font-medium text-sm">Ended</span>}
                </div>
            </div>

            <div>
                <h2 className="text-lg font-semibold mb-3">Problems</h2>
                {problems?.length > 0 ? (
                    <div className="border border-gray-200 rounded-lg overflow-hidden">
                        <table className="w-full text-sm">
                            <thead className="bg-gray-50">
                                <tr>
                                    <th className="px-4 py-2 text-left text-gray-500 text-xs uppercase">#</th>
                                    <th className="px-4 py-2 text-left text-gray-500 text-xs uppercase">Problem</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-gray-100">
                                {problems.map((p: any) => (
                                    <tr key={p.problem_id} className="hover:bg-gray-50">
                                        <td className="px-4 py-2.5 font-bold text-blue-600">{p.index}</td>
                                        <td className="px-4 py-2.5">
                                            <Link to={`/problems/${p.problem_id}`} className="hover:underline text-blue-600">
                                                {p.problem_id}
                                            </Link>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                ) : <p className="text-gray-400">No problems added yet.</p>}
            </div>

            {(isRunning || isEnded) && (
                <div>
                    <Link to={`/contests/${id}/scoreboard`}
                        className="inline-flex items-center gap-1 text-blue-600 hover:underline text-sm">
                        View Scoreboard →
                    </Link>
                </div>
            )}
        </div>
    )
}
