import { useEffect, useState, useRef, useCallback } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { api, getAccessToken, contestSlug } from '../lib/api'
import { loginHref } from '../lib/authRedirect'
import { resolveProblemSlug, resolveProblemTitle } from '../lib/problemSlugResolver'
import { useTheme } from '../context/ThemeContext'
import { EmptyState } from '../components/EmptyState'

function formatTime(seconds: number): string {
    const h = Math.floor(seconds / 3600)
    const m = Math.floor((seconds % 3600) / 60)
    const s = seconds % 60
    return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

function timerColor(remainingSeconds: number): string {
    if (remainingSeconds > 1800) return 'text-ok'
    if (remainingSeconds > 300) return 'text-amber-500 dark:text-amber-400'
    if (remainingSeconds > 60) return 'text-bad dark:text-bad'
    return 'text-bad animate-pulse'
}

function timerBg(remainingSeconds: number): string {
    if (remainingSeconds > 1800) return 'bg-emerald-50 border-emerald-200 dark:bg-emerald-900/30 dark:border-emerald-700'
    if (remainingSeconds > 300) return 'bg-warn/10 border-amber-200 dark:bg-amber-900/30 dark:border-amber-700'
    if (remainingSeconds > 60) return 'bg-bad/10 border-bad/30 dark:bg-red-900/30 dark:border-red-700'
    return 'bg-bad/12 border-bad/40 dark:bg-red-900/50 dark:border-red-700'
}

function progressBarColor(remainingSeconds: number): string {
    if (remainingSeconds > 1800) return 'bg-ok-fill'
    if (remainingSeconds > 300) return 'bg-amber-400'
    return 'bg-bad/100'
}

interface VirtualStatusState { is_active?: boolean; ends_at: string; started_at: string; original_contest_id: string; virtual_id?: string }
type ContestProblemRef = { problem_id: string; index?: string }
interface ContestDataShape { contest?: { id: string; slug?: string; display_id?: number; title?: string; start_time: string; end_time: string }; problems?: ContestProblemRef[] }

export default function VirtualContest() {
    const location = useLocation()
    const { theme } = useTheme()
    const [virtualStatus, setVirtualStatus] = useState<VirtualStatusState | null>(null)
    const [contestData, setContestData] = useState<ContestDataShape | null>(null)
    const { contest, problems } = (contestData ?? {}) as ContestDataShape
    const [loading, setLoading] = useState(() => !getAccessToken())
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
        if (!getAccessToken()) return
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
                            Promise.all(d.problems.map(async (p: ContestProblemRef) => {
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
            .catch(() => { })
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
                handleComplete(virtualStatus.virtual_id ?? '')
            }
        }, 1000)

        return () => clearInterval(interval)
    }, [virtualStatus, completed, handleComplete])

    if (loading) {
        return <div className="text-center py-20 text-muted-soft">Loading...</div>
    }

    if (!getAccessToken()) {
        return (
            <div className="text-center py-20">
                <p className="text-muted mb-4">Please log in to participate in virtual contests.</p>
                <Link to={loginHref(location)} className="text-brand hover:underline text-sm">Go to Login</Link>
            </div>
        )
    }

    if (!virtualStatus?.is_active && !completed) {
        return (
            <div className="text-center py-20">
                <div className="bg-surface border border-hairline rounded-lg shadow-sm p-8 max-w-md mx-auto">
                    <h2 className="text-xl font-bold text-ink mb-2">No Active Virtual Contest</h2>
                    <p className="text-muted text-sm mb-4">
                        Go to a past contest and click <span className="font-medium text-ink-soft">Start Virtual Contest</span> to begin a virtual session.
                    </p>
                    <Link to="/contests" className="text-brand hover:underline text-sm">
                        Browse Contests
                    </Link>
                </div>
            </div>
        )
    }

    if (completed) {
        return (
            <div className="text-center py-16">
                <div className="bg-surface border border-hairline rounded-lg shadow-sm p-10 max-w-lg mx-auto relative overflow-hidden">
                    <div className="absolute inset-0 pointer-events-none opacity-10"
                        style={{
                            background: theme === 'dark'
                                ? 'repeating-linear-gradient(45deg, transparent, transparent 10px, #a855f7 10px, #a855f7 20px), repeating-linear-gradient(-45deg, transparent, transparent 10px, #3b82f6 10px, #3b82f6 20px)'
                                : 'repeating-linear-gradient(45deg, transparent, transparent 10px, #f0abfc 10px, #f0abfc 20px), repeating-linear-gradient(-45deg, transparent, transparent 10px, #93c5fd 10px, #93c5fd 20px)',
                        }}
                    />
                    <div className="relative">
                        <h2 className="text-2xl font-bold text-ink mb-2">Virtual Contest Complete</h2>
                        <p className="text-muted text-sm mb-6">
                            {contestData?.contest?.title || 'Your virtual contest session has ended.'}
                        </p>
                        <div className="flex justify-center gap-8 mb-6 text-sm">
                            <div>
                                <div className="text-muted-soft uppercase text-xs tracking-wide mb-1">Elapsed</div>
                                <div className="text-xl font-mono font-bold text-ink-soft">{formatTime(elapsedSeconds)}</div>
                            </div>
                            <div>
                                <div className="text-muted-soft uppercase text-xs tracking-wide mb-1">Problems</div>
                                <div className="text-xl font-mono font-bold text-ink-soft">{contestData?.problems?.length || 0}</div>
                            </div>
                        </div>
                        <div className="flex gap-3 justify-center">
                            {contest && (
                                <Link
                                    to={`/contests/${contestSlug(contest)}/scoreboard`}
                                    className="text-brand hover:underline text-sm"
                                >
                                    View Scoreboard
                                </Link>
                            )}
                            <Link to="/contests" className="text-muted hover:underline text-sm">
                                Browse Contests
                            </Link>
                        </div>
                    </div>
                </div>
            </div>
        )
    }

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
                        <p className="text-muted text-sm mt-1">{contest.title}</p>
                    )}
                </div>
                <div className={`flex items-center gap-2 px-3 py-1.5 rounded-lg border text-sm font-medium ${timerBg(remainingSeconds)}`}>
                    <span className={timerColor(remainingSeconds)}>{formatTime(remainingSeconds)}</span>
                    <span className="text-muted-soft text-xs">remaining</span>
                </div>
            </div>

            {/* Timer Section */}
            <div className={`border rounded-lg shadow-sm p-6 ${timerBg(remainingSeconds)}`}>
                <div className="text-center mb-4">
                    <div className={`text-4xl font-mono font-bold tracking-wider ${timerColor(remainingSeconds)}`}>
                        {formatTime(remainingSeconds)}
                    </div>
                    <div className="text-muted-soft text-xs uppercase tracking-widest mt-1">Time Remaining</div>
                </div>

                {/* Progress bar */}
                <div className="w-full bg-white/60 rounded-full h-2 mb-3">
                    <div
                        className={`h-2 rounded-full transition-all duration-1000 ${progressBarColor(remainingSeconds)}`}
                        style={{ width: `${progressPercent}%` }}
                    />
                </div>

                <div className="flex justify-between text-xs text-muted">
                    <span>Elapsed: {formatTime(elapsedSeconds)}</span>
                    <span>Total: {formatTime(totalDuration)}</span>
                </div>
            </div>

            {/* Problems */}
            <div>
                <h2 className="text-lg font-semibold mb-3">Problems</h2>
                {(problems?.length ?? 0) > 0 ? (
                    <div className="bg-surface border border-hairline rounded-lg shadow-sm overflow-hidden">
                        <table className="w-full text-sm">
                            <thead className="bg-surface-2">
                                <tr>
                                    <th className="px-4 py-2.5 text-left text-muted text-xs uppercase tracking-wide">#</th>
                                    <th className="px-4 py-2.5 text-left text-muted text-xs uppercase tracking-wide">Problem</th>
                                    <th className="px-4 py-2.5 text-right text-muted text-xs uppercase tracking-wide">Action</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-hairline">
                                {problems?.map((p) => {
                                    const slug = problemSlugs.get(p.problem_id)
                                    const title = problemTitles.get(p.problem_id)
                                    return (
                                        <tr key={p.problem_id} className="hover:bg-surface-2 transition-colors">
                                            <td className="px-4 py-3 font-bold text-brand">{p.index}</td>
                                            <td className="px-4 py-3 text-ink font-medium">{title || p.problem_id}</td>
                                            <td className="px-4 py-3 text-right">
                                                <Link
                                                    to={slug ? `/problems/${slug}` : '#'}
                                                    className="inline-block text-xs text-on-brand bg-brand hover:bg-brand-hover px-3 py-1.5 rounded font-medium transition-colors"
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
                    <div className="bg-surface border border-hairline rounded-lg shadow-sm p-6">
                        <EmptyState text="No problems found for this contest" description="Add problems from the problem pool to populate this virtual contest." />
                    </div>
                )}
            </div>

            {/* Footer */}
            <div className="bg-surface border border-hairline rounded-lg shadow-sm p-4 flex items-center justify-between">
                <div className="text-sm text-muted">
                    Virtual contest in progress. Submissions will be judged in real-time.
                </div>
                <button
                    onClick={() => handleComplete(virtualStatus!.virtual_id ?? '')}
                    className="text-sm text-bad bg-bad/10 hover:bg-bad/12 px-4 py-2 rounded font-medium transition-colors"
                >
                    End Contest Early
                </button>
            </div>
        </div>
    )
}
