import { useEffect, useState, useRef, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'
import { resolveProblemSlug, resolveProblemTitle } from '../lib/problemSlugResolver'

function formatTime(seconds: number): string {
    const h = Math.floor(seconds / 3600)
    const m = Math.floor((seconds % 3600) / 60)
    const s = seconds % 60
    return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

function timerColor(remainingSeconds: number): string {
    if (remainingSeconds > 1800) return 'text-emerald-600'
    if (remainingSeconds > 300) return 'text-amber-500'
    if (remainingSeconds > 60) return 'text-red-500'
    return 'text-red-600 animate-pulse'
}

function timerBg(remainingSeconds: number): string {
    if (remainingSeconds > 1800) return 'bg-emerald-50 border-emerald-200'
    if (remainingSeconds > 300) return 'bg-amber-50 border-amber-200'
    if (remainingSeconds > 60) return 'bg-red-50 border-red-200'
    return 'bg-red-100 border-red-300'
}

function progressBarColor(remainingSeconds: number): string {
    if (remainingSeconds > 1800) return 'bg-emerald-500'
    if (remainingSeconds > 300) return 'bg-amber-400'
    return 'bg-red-500'
}

export default function VirtualContest() {
    const [virtualStatus, setVirtualStatus] = useState<any>(null)
    const [contestData, setContestData] = useState<any>(null)
    const [loading, setLoading] = useState(true)
    const [completed, setCompleted] = useState(false)
    const [problemSlugs, setProblemSlugs] = useState<Map<string, string>>(new Map())
    const [problemTitles, setProblemTitles] = useState<Map<string, string>>(new Map())
    const [remainingSeconds, setRemainingSeconds] = useState(0)
    const [elapsedSeconds, setElapsedSeconds] = useState(0)
    const completingRef = useRef(false)

    const handleComplete = useCallback(async (virtualId: string) => {
        if (completingRef.current) return
        completingRef.current = true
        try {
            await api.virtual.complete(virtualId)
        } catch {
            // contest may have already expired server-side
        }
        setCompleted(true)
    }, [])

    useEffect(() => {
        if (!getAccessToken()) {
            setLoading(false)
            return
        }
        api.virtual.status()
            .then((status) => {
                setVirtualStatus(status)
                if (status.is_active) {
                    const endsAt = new Date(status.ends_at).getTime()
                    const startedAt = new Date(status.started_at).getTime()
                    const now = Date.now()
                    const remaining = Math.max(0, Math.floor((endsAt - now) / 1000))
                    const elapsed = Math.max(0, Math.floor((now - startedAt) / 1000))
                    setRemainingSeconds(remaining)
                    setElapsedSeconds(elapsed)
                    return api.contests.get(status.original_contest_id).then((d) => {
                    setContestData(d)
                    if (d.problems?.length) {
                        Promise.all(d.problems.map(async (p: any) => {
                            const [slug, title] = await Promise.all([
                                resolveProblemSlug(p.problem_id),
                                resolveProblemTitle(p.problem_id),
                            ])
                            return { id: p.problem_id, slug, title }
                        })).then(results => {
                            const slugMap = new Map<string, string>()
                            const titleMap = new Map<string, string>()
                            for (const r of results) {
                                if (r.slug) slugMap.set(r.id, r.slug)
                                if (r.title) titleMap.set(r.id, r.title)
                            }
                            setProblemSlugs(slugMap)
                            setProblemTitles(titleMap)
                        })
                    }
                })
                }
            })
            .catch(() => {})
            .finally(() => setLoading(false))
    }, [])

    useEffect(() => {
        if (!virtualStatus?.is_active || completed) return

        const endsAt = new Date(virtualStatus.ends_at).getTime()
        const startedAt = new Date(virtualStatus.started_at).getTime()

        const interval = setInterval(() => {
            const now = Date.now()
            const remaining = Math.max(0, Math.floor((endsAt - now) / 1000))
            const elapsed = Math.max(0, Math.floor((now - startedAt) / 1000))
            setRemainingSeconds(remaining)
            setElapsedSeconds(elapsed)

            if (remaining <= 0) {
                clearInterval(interval)
                handleComplete(virtualStatus.virtual_id)
            }
        }, 1000)

        return () => clearInterval(interval)
    }, [virtualStatus, completed, handleComplete])

    if (loading) {
        return <div className="text-center py-20 text-gray-400">Loading...</div>
    }

    if (!getAccessToken()) {
        return (
            <div className="text-center py-20">
                <p className="text-gray-500 mb-4">Please log in to participate in virtual contests.</p>
                <Link to="/login" className="text-blue-600 hover:underline text-sm">Go to Login</Link>
            </div>
        )
    }

    if (!virtualStatus?.is_active && !completed) {
        return (
            <div className="text-center py-20">
                <div className="bg-white border border-gray-200 rounded-lg shadow-sm p-8 max-w-md mx-auto">
                    <h2 className="text-xl font-bold text-gray-800 mb-2">No Active Virtual Contest</h2>
                    <p className="text-gray-500 text-sm mb-4">
                        Go to a past contest and click <span className="font-medium text-gray-700">Start Virtual Contest</span> to begin a virtual session.
                    </p>
                    <Link to="/contests" className="text-blue-600 hover:underline text-sm">
                        Browse Contests
                    </Link>
                </div>
            </div>
        )
    }

    if (completed) {
        return (
            <div className="text-center py-16">
                <div className="bg-white border border-gray-200 rounded-lg shadow-sm p-10 max-w-lg mx-auto relative overflow-hidden">
                    <div className="absolute inset-0 pointer-events-none opacity-10"
                        style={{
                            background: 'repeating-linear-gradient(45deg, transparent, transparent 10px, #f0abfc 10px, #f0abfc 20px), repeating-linear-gradient(-45deg, transparent, transparent 10px, #93c5fd 10px, #93c5fd 20px)',
                        }}
                    />
                    <div className="relative">
                        <h2 className="text-2xl font-bold text-gray-800 mb-2">Virtual Contest Complete</h2>
                        <p className="text-gray-500 text-sm mb-6">
                            {contestData?.contest?.title || 'Your virtual contest session has ended.'}
                        </p>
                        <div className="flex justify-center gap-8 mb-6 text-sm">
                            <div>
                                <div className="text-gray-400 uppercase text-xs tracking-wide mb-1">Elapsed</div>
                                <div className="text-xl font-mono font-bold text-gray-700">{formatTime(elapsedSeconds)}</div>
                            </div>
                            <div>
                                <div className="text-gray-400 uppercase text-xs tracking-wide mb-1">Problems</div>
                                <div className="text-xl font-mono font-bold text-gray-700">{contestData?.problems?.length || 0}</div>
                            </div>
                        </div>
                        <div className="flex gap-3 justify-center">
                            {virtualStatus?.original_contest_id && (
                                <Link
                                    to={`/contests/${virtualStatus.original_contest_id}/scoreboard`}
                                    className="text-blue-600 hover:underline text-sm"
                                >
                                    View Scoreboard
                                </Link>
                            )}
                            <Link to="/contests" className="text-gray-500 hover:underline text-sm">
                                Browse Contests
                            </Link>
                        </div>
                    </div>
                </div>
            </div>
        )
    }

    const { contest, problems } = contestData || {}
    const totalDuration = contest
        ? Math.floor((new Date(contest.end_time).getTime() - new Date(contest.start_time).getTime()) / 1000)
        : 1
    const progressPercent = Math.min(100, Math.max(0, ((totalDuration - remainingSeconds) / totalDuration) * 100))

    return (
        <div className="space-y-6">
            {/* Header */}
            <div className="flex items-start justify-between gap-4">
                <div>
                    <h1 className="text-2xl font-bold">Virtual Contest</h1>
                    {contest && (
                        <p className="text-gray-500 text-sm mt-1">{contest.title}</p>
                    )}
                </div>
                <div className={`flex items-center gap-2 px-3 py-1.5 rounded-lg border text-sm font-medium ${timerBg(remainingSeconds)}`}>
                    <span className={timerColor(remainingSeconds)}>{formatTime(remainingSeconds)}</span>
                    <span className="text-gray-400 text-xs">remaining</span>
                </div>
            </div>

            {/* Timer Section */}
            <div className={`border rounded-lg shadow-sm p-6 ${timerBg(remainingSeconds)}`}>
                <div className="text-center mb-4">
                    <div className={`text-4xl font-mono font-bold tracking-wider ${timerColor(remainingSeconds)}`}>
                        {formatTime(remainingSeconds)}
                    </div>
                    <div className="text-gray-400 text-xs uppercase tracking-widest mt-1">Time Remaining</div>
                </div>

                {/* Progress bar */}
                <div className="w-full bg-white/60 rounded-full h-2 mb-3">
                    <div
                        className={`h-2 rounded-full transition-all duration-1000 ${progressBarColor(remainingSeconds)}`}
                        style={{ width: `${progressPercent}%` }}
                    />
                </div>

                <div className="flex justify-between text-xs text-gray-500">
                    <span>Elapsed: {formatTime(elapsedSeconds)}</span>
                    <span>Total: {formatTime(totalDuration)}</span>
                </div>
            </div>

            {/* Problems */}
            <div>
                <h2 className="text-lg font-semibold mb-3">Problems</h2>
                {problems?.length > 0 ? (
                    <div className="bg-white border border-gray-200 rounded-lg shadow-sm overflow-hidden">
                        <table className="w-full text-sm">
                            <thead className="bg-gray-50">
                                <tr>
                                    <th className="px-4 py-2.5 text-left text-gray-500 text-xs uppercase tracking-wide">#</th>
                                    <th className="px-4 py-2.5 text-left text-gray-500 text-xs uppercase tracking-wide">Problem</th>
                                    <th className="px-4 py-2.5 text-right text-gray-500 text-xs uppercase tracking-wide">Action</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-gray-100">
                                {problems.map((p: any) => {
                                    const slug = problemSlugs.get(p.problem_id)
                                    const title = problemTitles.get(p.problem_id)
                                    return (
                                    <tr key={p.problem_id} className="hover:bg-gray-50 transition-colors">
                                        <td className="px-4 py-3 font-bold text-blue-600">{p.index}</td>
                                        <td className="px-4 py-3 text-gray-800 font-medium">{title || p.problem_id}</td>
                                        <td className="px-4 py-3 text-right">
                                            <Link
                                                to={slug ? `/problems/${slug}` : '#'}
                                                className="inline-block text-xs text-white bg-blue-600 hover:bg-blue-700 px-3 py-1.5 rounded font-medium transition-colors"
                                            >
                                                Submit
                                            </Link>
                                        </td>
                                    </tr>
                                    )
                                })}
                            </tbody>
                        </table>
                    </div>
                ) : (
                    <div className="bg-white border border-gray-200 rounded-lg shadow-sm p-6 text-gray-400 text-sm">
                        No problems found for this contest.
                    </div>
                )}
            </div>

            {/* Footer */}
            <div className="bg-white border border-gray-200 rounded-lg shadow-sm p-4 flex items-center justify-between">
                <div className="text-sm text-gray-500">
                    Virtual contest in progress. Submissions will be judged in real-time.
                </div>
                <button
                    onClick={() => handleComplete(virtualStatus.virtual_id)}
                    className="text-sm text-red-600 bg-red-50 hover:bg-red-100 px-4 py-2 rounded font-medium transition-colors"
                >
                    End Contest Early
                </button>
            </div>
        </div>
    )
}
