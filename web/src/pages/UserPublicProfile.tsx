import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api } from '../lib/api'
import RatingBadge from '../components/RatingBadge'

interface UserProfile {
    id: string
    username: string
    rating: number | null
    created_at: string
}

interface UserStats {
    solved_count: number
    contest_count: number
    submission_count: number
}

interface RatingEntry {
    id: string
    new_rating: number
    rating_change: number
    contest_id: string
    created_at: string
}

interface Submission {
    id: string
    problem_slug: string
    problem_name: string
    status: string
    language: string
    created_at: string
}

interface SolvedProblem {
    id: string
    slug: string
    name: string
}

function formatDate(dateStr: string) {
    return new Date(dateStr).toLocaleDateString('en-US', {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
    })
}

function LoadingSkeleton() {
    return (
        <div className="max-w-2xl mx-auto space-y-6">
            {/* Header skeleton */}
            <div className="bg-white border border-gray-200 rounded-lg p-6 shadow-sm">
                <div className="flex items-center gap-4">
                    <div className="w-16 h-16 rounded-full bg-gray-200 animate-pulse" />
                    <div className="space-y-2">
                        <div className="h-7 w-40 bg-gray-200 rounded animate-pulse" />
                        <div className="h-5 w-24 bg-gray-200 rounded animate-pulse" />
                        <div className="h-4 w-32 bg-gray-200 rounded animate-pulse" />
                    </div>
                </div>
            </div>
            {/* Stats skeleton */}
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                {[1, 2, 3].map(i => (
                    <div key={i} className="bg-white border border-gray-200 rounded-lg p-4 text-center shadow-sm">
                        <div className="h-4 w-24 bg-gray-200 rounded animate-pulse mx-auto mb-2" />
                        <div className="h-8 w-16 bg-gray-200 rounded animate-pulse mx-auto" />
                    </div>
                ))}
            </div>
            {/* Chart skeleton */}
            <div className="bg-white border border-gray-200 rounded-lg p-6 shadow-sm">
                <div className="h-5 w-32 bg-gray-200 rounded animate-pulse mb-4" />
                <div className="h-32 bg-gray-100 rounded animate-pulse" />
            </div>
            {/* Table skeleton */}
            <div className="bg-white border border-gray-200 rounded-lg shadow-sm">
                <div className="p-4 border-b border-gray-200">
                    <div className="h-5 w-40 bg-gray-200 rounded animate-pulse" />
                </div>
                {[1, 2, 3].map(i => (
                    <div key={i} className="p-4 border-t border-gray-100 flex gap-4">
                        <div className="h-4 flex-1 bg-gray-200 rounded animate-pulse" />
                        <div className="h-4 w-20 bg-gray-200 rounded animate-pulse" />
                        <div className="h-4 w-16 bg-gray-200 rounded animate-pulse" />
                    </div>
                ))}
            </div>
        </div>
    )
}

function NotFound() {
    return (
        <div className="max-w-2xl mx-auto text-center py-20">
            <h1 className="text-2xl font-bold mb-2 text-gray-800">User Not Found</h1>
            <p className="text-gray-500 mb-6">The user you are looking for does not exist.</p>
            <Link to="/" className="text-blue-600 hover:underline">← Back to Home</Link>
        </div>
    )
}

function StatusBadge({ status }: { status: string }) {
    const normalized = status.toLowerCase().replace(/[_\s]/g, '_')
    let colors: string
    let label: string

    if (normalized === 'accepted' || normalized === 'ac') {
        colors = 'bg-green-100 text-green-800'
        label = 'Accepted'
    } else if (normalized === 'wrong_answer' || normalized === 'wa') {
        colors = 'bg-red-100 text-red-800'
        label = 'Wrong Answer'
    } else if (normalized === 'time_limit_exceeded' || normalized === 'tle') {
        colors = 'bg-yellow-100 text-yellow-800'
        label = 'Time Limit'
    } else if (normalized === 'runtime_error' || normalized === 're') {
        colors = 'bg-orange-100 text-orange-800'
        label = 'Runtime Error'
    } else if (normalized === 'compilation_error' || normalized === 'ce') {
        colors = 'bg-gray-100 text-gray-800'
        label = 'Compilation Error'
    } else {
        colors = 'bg-blue-100 text-blue-800'
        label = status.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase())
    }

    return (
        <span className={`px-2 py-1 rounded text-xs font-medium ${colors}`}>
            {label}
        </span>
    )
}

export default function UserPublicProfile() {
    const { username } = useParams<{ username: string }>()
    const [user, setUser] = useState<UserProfile | null>(null)
    const [stats, setStats] = useState<UserStats | null>(null)
    const [ratingHistory, setRatingHistory] = useState<RatingEntry[]>([])
    const [recentSubmissions, setRecentSubmissions] = useState<Submission[]>([])
    const [solvedProblems, setSolvedProblems] = useState<SolvedProblem[]>([])
    const [loading, setLoading] = useState(true)
    const [notFound, setNotFound] = useState(false)

    useEffect(() => {
        if (!username) return

        setLoading(true)
        setNotFound(false)

        api.users.getByUsername(username)
            .then((userData) => {
                setUser(userData)

                // Fetch all data in parallel
                const userId = userData.id
                return Promise.allSettled([
                    api.stats.getUserStats(userId),
                    api.ratings.getByUser(userId, 10),
                    api.submissions.list(0, 10),
                    api.problems.list(0, 100, { search: '' }),
                ])
            })
            .then((results) => {
                if (!results) return

                const [statsResult, ratingsResult, submissionsResult] = results

                if (statsResult.status === 'fulfilled') {
                    const s = statsResult.value
                    setStats({
                        solved_count: s?.solved_count ?? s?.solved ?? 0,
                        contest_count: s?.contest_count ?? s?.contests ?? 0,
                        submission_count: s?.submission_count ?? s?.submissions ?? 0,
                    })
                }

                if (ratingsResult.status === 'fulfilled') {
                    const data = Array.isArray(ratingsResult.value)
                        ? ratingsResult.value
                        : ratingsResult.value?.data ?? []
                    setRatingHistory(
                        data.sort(
                            (a: RatingEntry, b: RatingEntry) =>
                                new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
                        )
                    )
                }

                if (submissionsResult.status === 'fulfilled') {
                    const d = submissionsResult.value
                    const subs: Submission[] = Array.isArray(d) ? d : d?.data ?? []
                    setRecentSubmissions(subs.slice(0, 10))

                    // Extract unique solved problems from accepted submissions
                    const acceptedMap = new Map<string, SolvedProblem>()
                    for (const sub of subs) {
                        if ((sub.status === 'accepted' || sub.status === 'AC') && !acceptedMap.has(sub.problem_slug)) {
                            acceptedMap.set(sub.problem_slug, {
                                id: sub.id,
                                slug: sub.problem_slug,
                                name: sub.problem_name || sub.problem_slug,
                            })
                        }
                    }
                    setSolvedProblems(Array.from(acceptedMap.values()))
                }
            })
            .catch(() => {
                setNotFound(true)
            })
            .finally(() => {
                setLoading(false)
            })
    }, [username])

    if (loading) return <LoadingSkeleton />
    if (notFound || !user) return <NotFound />

    const currentRating = ratingHistory.length > 0
        ? ratingHistory[ratingHistory.length - 1].new_rating
        : user.rating || 0
    const maxRating = ratingHistory.length > 0
        ? Math.max(...ratingHistory.map(h => h.new_rating))
        : currentRating

    return (
        <div className="max-w-2xl mx-auto space-y-6">
            {/* Header Card */}
            <div className="bg-white border border-gray-200 rounded-lg p-6 shadow-sm">
                <div className="flex items-center gap-4">
                    <div className="w-16 h-16 rounded-full bg-gray-100 flex items-center justify-center text-2xl font-bold text-gray-600">
                        {username?.charAt(0).toUpperCase()}
                    </div>
                    <div>
                        <h1 className="text-2xl font-bold text-gray-900">{user.username}</h1>
                        {user.rating ? (
                            <RatingBadge rating={user.rating} showTitle size="lg" />
                        ) : (
                            <span className="text-gray-400 text-sm">Unrated</span>
                        )}
                        <p className="text-sm text-gray-500 mt-1">
                            Member since {formatDate(user.created_at)}
                        </p>
                    </div>
                </div>
            </div>

            {/* Stats Grid */}
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                <div className="bg-white border border-gray-200 rounded-lg p-4 text-center shadow-sm">
                    <p className="text-xs text-gray-500 mb-1">Problems Solved</p>
                    <p className="text-2xl font-semibold">{stats?.solved_count ?? 0}</p>
                </div>
                <div className="bg-white border border-gray-200 rounded-lg p-4 text-center shadow-sm">
                    <p className="text-xs text-gray-500 mb-1">Contests Played</p>
                    <p className="text-2xl font-semibold">{stats?.contest_count ?? 0}</p>
                </div>
                <div className="bg-white border border-gray-200 rounded-lg p-4 text-center shadow-sm">
                    <p className="text-xs text-gray-500 mb-1">Current Rating</p>
                    {user.rating ? (
                        <RatingBadge rating={currentRating} size="lg" />
                    ) : (
                        <p className="text-2xl font-semibold text-gray-400">—</p>
                    )}
                </div>
            </div>

            {/* Rating History Chart */}
            <div className="bg-white border border-gray-200 rounded-lg p-6 shadow-sm">
                <div className="flex items-center justify-between mb-4">
                    <h2 className="text-lg font-semibold">Rating History</h2>
                    {ratingHistory.length > 0 && (
                        <Link to="/rating-history" className="text-sm text-blue-600 hover:underline">
                            View Full History →
                        </Link>
                    )}
                </div>
                {ratingHistory.length === 0 ? (
                    <p className="text-gray-400 text-sm text-center py-8">No rated contests yet</p>
                ) : (
                    <div className="flex items-end gap-1 h-32">
                        {ratingHistory.slice(-10).map((entry, i) => {
                            const height = maxRating > 0
                                ? Math.max(8, (entry.new_rating / maxRating) * 100)
                                : 50
                            const delta = entry.rating_change
                            const color = delta > 0 ? 'bg-green-500' : delta < 0 ? 'bg-red-500' : 'bg-gray-400'
                            return (
                                <div
                                    key={entry.id || i}
                                    className={`flex-1 rounded-t ${color} transition-all`}
                                    style={{ height: `${height}%` }}
                                    title={`${entry.new_rating} (${delta > 0 ? '+' : ''}${delta})`}
                                />
                            )
                        })}
                    </div>
                )}
            </div>

            {/* Recent Submissions */}
            <div className="bg-white border border-gray-200 rounded-lg shadow-sm overflow-hidden">
                <div className="p-4 border-b border-gray-200">
                    <h2 className="text-lg font-semibold">Recent Submissions</h2>
                </div>
                {recentSubmissions.length === 0 ? (
                    <p className="text-gray-400 text-sm text-center py-8">No submissions yet</p>
                ) : (
                    <div className="overflow-x-auto">
                        <table className="w-full">
                            <thead className="bg-gray-50">
                                <tr>
                                    <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Problem</th>
                                    <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
                                    <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Language</th>
                                    <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Time</th>
                                </tr>
                            </thead>
                            <tbody>
                                {recentSubmissions.map((sub) => (
                                    <tr key={sub.id} className="border-t border-gray-100 hover:bg-gray-50">
                                        <td className="px-4 py-3">
                                            <Link
                                                to={`/problems/${sub.problem_slug}`}
                                                className="text-blue-600 hover:underline text-sm"
                                            >
                                                {sub.problem_name || sub.problem_slug}
                                            </Link>
                                        </td>
                                        <td className="px-4 py-3">
                                            <StatusBadge status={sub.status} />
                                        </td>
                                        <td className="px-4 py-3 text-sm text-gray-500">{sub.language}</td>
                                        <td className="px-4 py-3 text-sm text-gray-500">{formatDate(sub.created_at)}</td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                )}
            </div>

            {/* Solved Problems */}
            <div className="bg-white border border-gray-200 rounded-lg p-6 shadow-sm">
                <h2 className="text-lg font-semibold mb-4">Solved Problems</h2>
                {solvedProblems.length === 0 ? (
                    <p className="text-gray-400 text-sm text-center py-4">No problems solved yet</p>
                ) : (
                    <div className="flex flex-wrap gap-2">
                        {solvedProblems.map((p) => (
                            <Link
                                key={p.id}
                                to={`/problems/${p.slug}`}
                                className="px-3 py-1.5 bg-gray-100 hover:bg-gray-200 rounded text-sm transition-colors"
                            >
                                {p.name}
                            </Link>
                        ))}
                    </div>
                )}
            </div>
        </div>
    )
}
