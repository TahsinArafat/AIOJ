import { useEffect, useState, useMemo, useCallback, useRef } from 'react'
import { Link } from 'react-router-dom'
import { api, contestSlug } from '../lib/api'
import DivisionBadge from '../components/DivisionBadge'
import { useTheme } from '../context/ThemeContext'
import { DIVISIONS, type Division } from '../lib/divisions'

/* ───────────────────── helpers ───────────────────── */

const FORMAT_BADGES: Record<string, {
    label: string;
    color: string;
    bg: string;
    darkColor: string;
    darkBg: string;
}> = {
    acm:    { label: 'ACM', color: '#6B21A8', bg: '#F3E8FF', darkColor: '#D8B4FE', darkBg: '#4C1D9530' },
    oi:     { label: 'OI',  color: '#0C4A6E', bg: '#E0F2FE', darkColor: '#7DD3FC', darkBg: '#0C4A6E40' },
    ioi:    { label: 'IOI', color: '#065F46', bg: '#D1FAE5', darkColor: '#6EE7B7', darkBg: '#065F4640' },
    cf:     { label: 'CF',  color: '#9A3412', bg: '#FFF7ED', darkColor: '#FDBA74', darkBg: '#9A341240' },
    atcoder:{ label: 'ATC', color: '#4338CA', bg: '#EDE9FE', darkColor: '#A5B4FC', darkBg: '#4338CA40' },
}

function formatDate(iso: string): string {
    const d = new Date(iso)
    const months = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec']
    const mm = months[d.getMonth()]
    const dd = String(d.getDate()).padStart(2, '0')
    const yyyy = d.getFullYear()
    const hh = String(d.getHours()).padStart(2, '0')
    const mi = String(d.getMinutes()).padStart(2, '0')
    return `${mm} ${dd}, ${yyyy} ${hh}:${mi}`
}

function formatDuration(startIso: string, endIso: string): string {
    const ms = new Date(endIso).getTime() - new Date(startIso).getTime()
    const totalMin = Math.floor(ms / 60000)
    const h = Math.floor(totalMin / 60)
    const m = totalMin % 60
    return `${String(h).padStart(2, '0')}h ${String(m).padStart(2, '0')}m`
}

function pad(n: number): string {
    return String(Math.max(0, Math.floor(n))).padStart(2, '0')
}

function formatCountdown(ms: number): string {
    if (ms <= 0) return '00d 00:00:00'
    const d = Math.floor(ms / 86400000)
    const h = Math.floor((ms % 86400000) / 3600000)
    const m = Math.floor((ms % 3600000) / 60000)
    const s = Math.floor((ms % 60000) / 1000)
    if (d > 0) return `${pad(d)}d ${pad(h)}:${pad(m)}:${pad(s)}`
    return `${pad(h)}:${pad(m)}:${pad(s)}`
}

function formatElapsed(startIso: string): string {
    const ms = Date.now() - new Date(startIso).getTime()
    if (ms <= 0) return '00:00:00'
    const h = Math.floor(ms / 3600000)
    const m = Math.floor((ms % 3600000) / 60000)
    const s = Math.floor((ms % 60000) / 1000)
    return `${pad(h)}:${pad(m)}:${pad(s)}`
}

type ContestStatus = 'upcoming' | 'running' | 'ended'

function getContestStatus(c: any): ContestStatus {
    const now = Date.now()
    const start = new Date(c.start_time).getTime()
    const end = new Date(c.end_time).getTime()
    if (now < start) return 'upcoming'
    if (now < end) return 'running'
    return 'ended'
}

/* ───────────────────── sub-components ───────────────────── */

function FormatBadge({ type }: { type: string }) {
    const { theme } = useTheme()
    const info = FORMAT_BADGES[type?.toLowerCase()]
    if (!info) return <span className="text-xs px-1.5 py-0.5 rounded font-medium bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400 uppercase">{type || 'STD'}</span>
    const textColor = theme === 'dark' ? info.darkColor : info.color
    const bgColor = theme === 'dark' ? info.darkBg : info.bg
    return (
        <span className="text-xs px-1.5 py-0.5 rounded font-medium uppercase" style={{ color: textColor, backgroundColor: bgColor }}>
            {info.label}
        </span>
    )
}

function StatusBadge({ status }: { status: ContestStatus }) {
    const styles: Record<ContestStatus, { text: string; cls: string }> = {
        upcoming: { text: 'Upcoming', cls: 'bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-900/30 dark:text-blue-300 dark:border-blue-700' },
        running:  { text: 'Running',  cls: 'bg-green-50 text-green-700 border-green-200 dark:bg-green-900/30 dark:text-green-300 dark:border-green-700' },
        ended:    { text: 'Ended',    cls: 'bg-gray-50 text-gray-500 border-gray-200 dark:bg-gray-700 dark:text-gray-400 dark:border-gray-600' },
    }
    const s = styles[status]
    return (
        <span className={`inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded-full font-semibold border ${s.cls}`}>
            {status === 'running' && <span className="w-1.5 h-1.5 rounded-full bg-green-500 animate-pulse" />}
            {s.text}
        </span>
    )
}

function CountdownTimer({ targetIso }: { targetIso: string }) {
    const [remaining, setRemaining] = useState(() => new Date(targetIso).getTime() - Date.now())

    useEffect(() => {
        const id = setInterval(() => setRemaining(new Date(targetIso).getTime() - Date.now()), 1000)
        return () => clearInterval(id)
    }, [targetIso])

    if (remaining <= 0) return <span className="text-xs text-green-600 font-semibold">Starting now</span>

    return (
        <span className="font-mono text-xs text-blue-600 font-semibold tracking-wide">
            {formatCountdown(remaining)}
        </span>
    )
}

function ElapsedTimer({ startIso }: { startIso: string }) {
    const [, setTick] = useState(0)

    useEffect(() => {
        const id = setInterval(() => setTick(t => t + 1), 1000)
        return () => clearInterval(id)
    }, [startIso])

    return (
        <span className="font-mono text-xs text-green-600 font-medium tracking-wide">
            {formatElapsed(startIso)}
        </span>
    )
}

/* ───────────────────── main component ───────────────────── */

const PAGE_SIZE = 30

export default function ContestList() {
    const { theme } = useTheme()
    const [contests, setContests] = useState<any[]>([])
    const [total, setTotal] = useState(0)
    const [division, setDivision] = useState<Division | undefined>(undefined)
    const [search, setSearch] = useState('')
    const [searchInput, setSearchInput] = useState('')
    const [offset, setOffset] = useState(0)
    const [loading, setLoading] = useState(false)
    const [hasMore, setHasMore] = useState(false)
    const searchTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

    // debounce search
    const onSearchChange = useCallback((v: string) => {
        setSearchInput(v)
        if (searchTimer.current) clearTimeout(searchTimer.current)
        searchTimer.current = setTimeout(() => setSearch(v), 300)
    }, [])

    // fetch on division / search change (reset)
    useEffect(() => {
        setOffset(0)
        loadContests(0, true)
    }, [division, search])

    const loadContests = useCallback(async (off: number, replace: boolean) => {
        setLoading(true)
        try {
            const d = await api.contests.list(off, PAGE_SIZE, division)
            const items = d.data || []
            setContests(prev => replace ? items : [...prev, ...items])
            setTotal(d.total || 0)
            setHasMore(off + items.length < (d.total || 0))
            setOffset(off + items.length)
        } catch {
            // silent
        } finally {
            setLoading(false)
        }
    }, [division])

    const loadMore = useCallback(() => {
        if (!loading && hasMore) loadContests(offset, false)
    }, [loading, hasMore, offset, loadContests])

    // categorize
    const { upcoming, past } = useMemo(() => {
        const filtered = contests.filter(c => {
            if (!search) return true
            return c.title?.toLowerCase().includes(search.toLowerCase())
        })
        const u: any[] = []
        const p: any[] = []
        for (const c of filtered) {
            const s = getContestStatus(c)
            if (s === 'ended') p.push(c)
            else u.push(c) // upcoming + running
        }
        // sort upcoming by start_time ascending (soonest first)
        u.sort((a, b) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime())
        // sort past by start_time descending (most recent first)
        p.sort((a, b) => new Date(b.start_time).getTime() - new Date(a.start_time).getTime())
        return { upcoming: u, past: p }
    }, [contests, search])

    return (
        <div className="max-w-6xl mx-auto px-4 py-6">
            {/* ── Header ── */}
            <div className="flex items-center justify-between mb-6">
                <div>
                    <h1 className="text-2xl font-bold text-gray-900 dark:text-gray-100">Contests</h1>
                    <p className="text-sm text-gray-500 mt-0.5">{total} contest{total !== 1 ? 's' : ''} available</p>
                </div>
                <Link to="/gym" className="text-sm text-blue-600 hover:text-blue-800 font-medium">
                    Go to Gym →
                </Link>
            </div>

            {/* ── Filters bar ── */}
            <div className="flex flex-col sm:flex-row sm:items-center gap-3 mb-6">
                {/* Division buttons */}
                <div className="flex gap-1.5 flex-wrap">
                    <button
                        onClick={() => setDivision(undefined)}
                        className={`px-3 py-1.5 rounded text-sm font-medium transition-colors ${
                            division === undefined
                                ? 'bg-gray-900 text-white shadow-sm'
                                : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 hover:bg-gray-200'
                        }`}
                    >
                        All
                    </button>
                    {(Object.entries(DIVISIONS) as [string, any][]).map(([key, info]) => {
                        const div = Number(key) as Division
                        const active = division === div
                        const activeBg = theme === 'dark' && info.darkBg ? info.darkBg.replace(/30$|40$/, '60') : info.color
                        return (
                            <button
                                key={key}
                                onClick={() => setDivision(div)}
                                className={`px-3 py-1.5 rounded text-sm font-medium transition-colors ${
                                    active
                                        ? 'text-white shadow-sm'
                                        : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600'
                                }`}
                                style={active ? { backgroundColor: activeBg } : {}}
                            >
                                {info.name}
                            </button>
                        )
                    })}
                </div>

                {/* Search */}
                <div className="relative sm:ml-auto">
                    <input
                        type="text"
                        placeholder="Search contests..."
                        value={searchInput}
                        onChange={e => onSearchChange(e.target.value)}
                        className="w-full sm:w-64 pl-8 pr-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500"
                    />
                    <svg className="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                    </svg>
                </div>
            </div>

            {/* ── Current / Upcoming Contests ── */}
            <div className="mb-10">
                <div className="flex items-center gap-2 mb-3">
                    <div className="w-1 h-5 rounded-full bg-blue-500" />
                    <h2 className="text-lg font-bold text-gray-800 dark:text-gray-200">Current or Upcoming Contests</h2>
                    {upcoming.length > 0 && (
                        <span className="text-xs bg-blue-100 text-blue-700 px-2 py-0.5 rounded-full font-semibold">{upcoming.length}</span>
                    )}
                </div>

                {upcoming.length === 0 && !loading ? (
                    <div className="text-center py-12 text-gray-400 border border-dashed border-gray-200 dark:border-gray-700 rounded-lg">
                        <svg className="w-10 h-10 mx-auto mb-2 text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                        </svg>
                        No upcoming contests at the moment.
                    </div>
                ) : (
                    <div className="overflow-x-auto border border-gray-200 dark:border-gray-700 rounded-lg">
                        <table className="w-full text-sm">
                            <thead>
                                <tr className="bg-gray-50 border-b border-gray-200 dark:border-gray-700">
                                    <th className="text-left px-4 py-2.5 font-semibold text-gray-600 dark:text-gray-400 w-8">#</th>
                                    <th className="text-left px-4 py-2.5 font-semibold text-gray-600 dark:text-gray-400">Contest</th>
                                    <th className="text-left px-4 py-2.5 font-semibold text-gray-600 dark:text-gray-400">Start</th>
                                    <th className="text-left px-4 py-2.5 font-semibold text-gray-600 dark:text-gray-400">Length</th>
                                    <th className="text-left px-4 py-2.5 font-semibold text-gray-600 dark:text-gray-400">Countdown / Elapsed</th>
                                    <th className="text-left px-4 py-2.5 font-semibold text-gray-600 dark:text-gray-400">Status</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-gray-100">
                                {upcoming.map((c, i) => {
                                    const status = getContestStatus(c)
                                    return (
                                        <tr key={c.id} className="hover:bg-blue-50/40 transition-colors">
                                            <td className="px-4 py-2.5 text-gray-400 text-xs font-medium">{i + 1}</td>
                                            <td className="px-4 py-2.5">
                                                <div className="flex items-center gap-2 flex-wrap">
                                                    <Link to={`/contests/${contestSlug(c)}`} className="font-semibold text-gray-900 dark:text-gray-100 hover:text-blue-600 dark:hover:text-blue-400 hover:underline">
                                                        {c.title}
                                                    </Link>
                                                    <DivisionBadge division={c.division ?? 0} size="sm" />
                                                    <FormatBadge type={c.type} />
                                                </div>
                                            </td>
                                            <td className="px-4 py-2.5 text-gray-600 dark:text-gray-400 whitespace-nowrap">{formatDate(c.start_time)}</td>
                                            <td className="px-4 py-2.5 text-gray-600 dark:text-gray-400 whitespace-nowrap font-mono text-xs">{formatDuration(c.start_time, c.end_time)}</td>
                                            <td className="px-4 py-2.5 whitespace-nowrap">
                                                {status === 'upcoming' && <CountdownTimer targetIso={c.start_time} />}
                                                {status === 'running' && (
                                                    <div className="flex flex-col gap-0.5">
                                                        <ElapsedTimer startIso={c.start_time} />
                                                        <span className="text-[10px] text-gray-400">elapsed</span>
                                                    </div>
                                                )}
                                            </td>
                                            <td className="px-4 py-2.5">
                                                <StatusBadge status={status} />
                                            </td>
                                        </tr>
                                    )
                                })}
                            </tbody>
                        </table>
                    </div>
                )}
            </div>

            {/* ── Past Contests ── */}
            <div className="mb-6">
                <div className="flex items-center gap-2 mb-3">
                    <div className="w-1 h-5 rounded-full bg-gray-400" />
                    <h2 className="text-lg font-bold text-gray-800 dark:text-gray-200">Past Contests</h2>
                    {past.length > 0 && (
                        <span className="text-xs bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 px-2 py-0.5 rounded-full font-semibold">{past.length}</span>
                    )}
                </div>

                {past.length === 0 && !loading ? (
                    <div className="text-center py-12 text-gray-400 border border-dashed border-gray-200 dark:border-gray-700 rounded-lg">
                        No past contests found.
                    </div>
                ) : (
                    <div className="overflow-x-auto border border-gray-200 dark:border-gray-700 rounded-lg">
                        <table className="w-full text-sm">
                            <thead>
                                <tr className="bg-gray-50 border-b border-gray-200 dark:border-gray-700">
                                    <th className="text-left px-4 py-2.5 font-semibold text-gray-600 dark:text-gray-400 w-8">#</th>
                                    <th className="text-left px-4 py-2.5 font-semibold text-gray-600 dark:text-gray-400">Contest</th>
                                    <th className="text-left px-4 py-2.5 font-semibold text-gray-600 dark:text-gray-400">Start</th>
                                    <th className="text-left px-4 py-2.5 font-semibold text-gray-600 dark:text-gray-400">Length</th>
                                    <th className="text-left px-4 py-2.5 font-semibold text-gray-600 dark:text-gray-400">Status / Standings</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-gray-100">
                                {past.map((c, i) => (
                                    <tr key={c.id} className="hover:bg-gray-50/60 transition-colors">
                                        <td className="px-4 py-2.5 text-gray-400 text-xs font-medium">{i + 1}</td>
                                        <td className="px-4 py-2.5">
                                            <div className="flex items-center gap-2 flex-wrap">
                                                <Link to={`/contests/${contestSlug(c)}`} className="font-semibold text-gray-900 dark:text-gray-100 hover:text-blue-600 hover:underline">
                                                    {c.title}
                                                </Link>
                                                <DivisionBadge division={c.division ?? 0} size="sm" />
                                                <FormatBadge type={c.type} />
                                            </div>
                                        </td>
                                        <td className="px-4 py-2.5 text-gray-600 dark:text-gray-400 whitespace-nowrap">{formatDate(c.start_time)}</td>
                                        <td className="px-4 py-2.5 text-gray-600 dark:text-gray-400 whitespace-nowrap font-mono text-xs">{formatDuration(c.start_time, c.end_time)}</td>
                                        <td className="px-4 py-2.5">
                                            <div className="flex items-center gap-3">
                                                <StatusBadge status="ended" />
                                                <Link
                                                    to={`/contests/${contestSlug(c)}/scoreboard`}
                                                    className="text-xs text-blue-600 hover:text-blue-800 hover:underline font-medium"
                                                >
                                                    Final standings
                                                </Link>
                                            </div>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                )}
            </div>

            {/* ── Load more ── */}
            {hasMore && (
                <div className="flex justify-center mt-6">
                    <button
                        onClick={loadMore}
                        disabled={loading}
                        className="px-6 py-2 text-sm font-medium rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 transition-colors"
                    >
                        {loading ? (
                            <span className="flex items-center gap-2">
                                <svg className="animate-spin w-4 h-4" viewBox="0 0 24 24" fill="none">
                                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                                </svg>
                                Loading...
                            </span>
                        ) : (
                            `Load more (${offset} of ${total})`
                        )}
                    </button>
                </div>
            )}

            {/* ── Loading indicator ── */}
            {loading && contests.length === 0 && (
                <div className="flex justify-center py-16">
                    <svg className="animate-spin w-8 h-8 text-blue-500" viewBox="0 0 24 24" fill="none">
                        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                    </svg>
                </div>
            )}
        </div>
    )
}
