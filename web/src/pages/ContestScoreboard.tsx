import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'
import RatingBadge from '../components/RatingBadge'

export default function ContestScoreboard() {
    const { id } = useParams<{ id: string }>()
    const [data, setData] = useState<any>(null)
    const [loading, setLoading] = useState(true)
    const [ratingChanges, setRatingChanges] = useState<any>(null)
    const [isAdmin, setIsAdmin] = useState(false)
    const [calculating, setCalculating] = useState(false)

    useEffect(() => {
        const token = getAccessToken()
        if (token) {
            try {
                const payload = JSON.parse(atob(token.split('.')[1]))
                setIsAdmin(payload.role === 'admin')
            } catch {}
        }
    }, [])

    useEffect(() => {
        if (!id) return
        api.contests.scoreboard(id)
            .then(setData).catch(() => {}).finally(() => setLoading(false))
    }, [id])

    useEffect(() => {
        if (!id) return
        api.ratings.getByContest(id).then(d => {
            if (d.data && d.data.length > 0) setRatingChanges(d.data)
        }).catch(() => {})
    }, [id])

    const handleCalculateRatings = async () => {
        setCalculating(true)
        try {
            await fetch('/api/contests/' + id + '/calculate-ratings', {
                method: 'POST',
                headers: { 'Authorization': 'Bearer ' + getAccessToken() }
            }).then(r => r.json())
            window.location.reload()
        } catch (e: any) {
            alert('Failed: ' + e.message)
            setCalculating(false)
        }
    }

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

            {isAdmin && !ratingChanges && contest?.is_rated !== false && (
                <div className="bg-blue-50 border border-blue-200 text-blue-800 px-4 py-3 rounded-lg mb-4 flex items-center justify-between text-sm">
                    <span>Contest has ended. Apply rating changes to participants.</span>
                    <button
                        onClick={handleCalculateRatings}
                        disabled={calculating}
                        className="bg-blue-600 text-white px-4 py-1.5 rounded text-xs font-semibold hover:bg-blue-700 disabled:opacity-50 transition-colors cursor-pointer"
                    >
                        {calculating ? 'Calculating...' : 'Rate This Contest'}
                    </button>
                </div>
            )}

            {ratingChanges && (
                <div className="bg-green-50 border border-green-200 text-green-800 px-4 py-3 rounded-lg mb-4 text-sm">
                    ✓ Ratings have been applied for this contest.
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
                            {ratingChanges && <th className="border border-gray-200 px-3 py-2 text-center w-20">Rating</th>}
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
                                {ratingChanges && (
                                    <td className="border border-gray-200 px-3 py-2 text-center">
                                        {(() => {
                                            const rc = ratingChanges.find((r: any) => r.user_id === e.user_id)
                                            if (!rc) return <span className="text-gray-300">—</span>
                                            const delta = rc.rating_change
                                            return (
                                                <span className="text-xs">
                                                    <RatingBadge rating={rc.new_rating} size="sm" />
                                                    <span className={`ml-1 font-semibold ${delta >= 0 ? 'text-green-600' : 'text-red-600'}`}>
                                                        {delta >= 0 ? '+' : ''}{delta}
                                                    </span>
                                                </span>
                                            )
                                        })()}
                                    </td>
                                )}
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
                                <td colSpan={(problems?.length || 0) + (ratingChanges ? 5 : 4)} className="px-4 py-8 text-center text-gray-400">
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
