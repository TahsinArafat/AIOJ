import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'
import RatingBadge from '../components/RatingBadge'

const RANK_STYLES: Record<number, string> = {
    1: 'text-amber-700 bg-amber-50 font-bold',
    2: 'text-gray-600 bg-gray-100 font-bold',
    3: 'text-orange-700 bg-orange-50 font-bold',
}

const MEDALS: Record<number, string> = {
    1: '\u{1F947}',
    2: '\u{1F948}',
    3: '\u{1F949}',
}

export default function Rankings() {
    const [users, setUsers] = useState<any[]>([])
    const [total, setTotal] = useState(0)
    const [offset, setOffset] = useState(0)
    const [loading, setLoading] = useState(false)
    const limit = 50

    const fetchRankings = (off: number, append = false) => {
        setLoading(true)
        api.rankings.list(off, limit).then(d => {
            setUsers(prev => append ? [...prev, ...(d.data || [])] : (d.data || []))
            setTotal(d.total || 0)
        }).catch(() => {}).finally(() => setLoading(false))
    }

    useEffect(() => {
        fetchRankings(0)
    }, [])

    const handleLoadMore = () => {
        const nextOffset = offset + limit
        setOffset(nextOffset)
        fetchRankings(nextOffset, true)
    }

    const hasMore = users.length < total

    return (
        <div>
            <h1 className="text-2xl font-bold mb-4">Rankings</h1>

            <div className="border border-gray-200 rounded-lg overflow-hidden">
                <div className="overflow-x-auto">
                    <table className="w-full text-sm">
                        <thead className="bg-gray-50 text-gray-500 text-xs uppercase">
                            <tr>
                                <th className="px-4 py-3 text-left w-16">Rank</th>
                                <th className="px-4 py-3 text-left">Username</th>
                                <th className="px-4 py-3 text-right">Rating</th>
                                <th className="px-4 py-3 text-right">Change</th>
                                <th className="px-4 py-3 text-right">Contests</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-gray-100">
                            {users.map((u, i) => {
                                const rank = offset + i + 1
                                const rankStyle = RANK_STYLES[rank] || ''
                                return (
                                    <tr key={u.username} className={`hover:bg-gray-50 transition-colors ${rankStyle}`}>
                                        <td className="px-4 py-3 text-center">
                                            {MEDALS[rank] || rank}
                                        </td>
                                        <td className="px-4 py-3">
                                            <Link to={`/user/${u.username}`} className="font-medium text-blue-600 hover:underline">
                                                {u.username}
                                            </Link>
                                        </td>
                                        <td className="px-4 py-3 text-right">
                                            <RatingBadge rating={u.rating} size="sm" />
                                        </td>
                                        <td className="px-4 py-3 text-right">
                                            <span className={u.rating_change >= 0 ? 'text-green-600' : 'text-red-600'}>
                                                {u.rating_change >= 0 ? '+' : ''}{u.rating_change}
                                            </span>
                                        </td>
                                        <td className="px-4 py-3 text-right text-gray-500">
                                            {u.contests_played}
                                        </td>
                                    </tr>
                                )
                            })}
                            {users.length === 0 && !loading && (
                                <tr><td colSpan={5} className="px-4 py-8 text-center text-gray-400">No rankings yet.</td></tr>
                            )}
                        </tbody>
                    </table>
                </div>
            </div>

            {loading && (
                <div className="text-center py-4 text-gray-400">Loading...</div>
            )}

            {hasMore && !loading && (
                <div className="flex justify-center mt-4">
                    <button
                        onClick={handleLoadMore}
                        className="px-6 py-2 border border-gray-300 rounded text-sm hover:bg-gray-50 transition-colors"
                    >
                        Load More
                    </button>
                </div>
            )}

            {total > 0 && (
                <p className="text-sm text-gray-400 mt-3 text-center">{users.length} of {total} users</p>
            )}
        </div>
    )
}
