import React, { useEffect, useState, useCallback, useMemo } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'
import DivisionBadge from '../components/DivisionBadge'
import {
    Trophy, FileText, MessageSquare, Users, Zap, FileDown, Pencil, Gamepad2,
    AlertTriangle, Megaphone, Settings, BookOpen, Info, Clock, Shield, CheckCircle2, X, ExternalLink,
    ChevronLeft, ChevronRight, Send, Filter, Eye, EyeOff
} from 'lucide-react'

function decodeRole(): string | null {
    const token = localStorage.getItem('access_token')
    if (!token) return null
    try {
        const payload = JSON.parse(atob(token.split('.')[1]))
        return payload.role ?? null
    } catch {
        return null
    }
}

function decodeUserId(): string | null {
    const token = localStorage.getItem('access_token')
    if (!token) return null
    try {
        const payload = JSON.parse(atob(token.split('.')[1]))
        return payload.user_id ?? null
    } catch {
        return null
    }
}

// ─── Countdown / Timer Utilities ───────────────────────────────────────────────

function pad2(n: number): string {
    return n < 10 ? `0${n}` : `${n}`
}

function formatCountdown(diffMs: number): string {
    if (diffMs <= 0) return '00:00:00'
    const totalSeconds = Math.floor(diffMs / 1000)
    const days = Math.floor(totalSeconds / 86400)
    const hours = Math.floor((totalSeconds % 86400) / 3600)
    const minutes = Math.floor((totalSeconds % 3600) / 60)
    const seconds = totalSeconds % 60
    if (days > 0) return `${days}d ${pad2(hours)}:${pad2(minutes)}:${pad2(seconds)}`
    return `${pad2(hours)}:${pad2(minutes)}:${pad2(seconds)}`
}

function formatDuration(ms: number): string {
    const totalMinutes = Math.floor(ms / 60000)
    const hours = Math.floor(totalMinutes / 60)
    const minutes = totalMinutes % 60
    if (hours > 0) return `${hours}h ${minutes}m`
    return `${minutes}m`
}

function formatDateTime(iso: string): string {
    const d = new Date(iso)
    return d.toLocaleString(undefined, {
        year: 'numeric', month: 'short', day: 'numeric',
        hour: '2-digit', minute: '2-digit',
    })
}

function timeAgo(dateStr: string): string {
    const now = Date.now()
    const then = new Date(dateStr).getTime()
    const seconds = Math.floor((now - then) / 1000)
    if (seconds < 60) return 'just now'
    const minutes = Math.floor(seconds / 60)
    if (minutes < 60) return `${minutes}m ago`
    const hours = Math.floor(minutes / 60)
    if (hours < 24) return `${hours}h ago`
    const days = Math.floor(hours / 24)
    if (days < 30) return `${days}d ago`
    const months = Math.floor(days / 30)
    if (months < 12) return `${months}mo ago`
    const years = Math.floor(months / 12)
    return `${years}y ago`
}

// ─── Status Badge ──────────────────────────────────────────────────────────────

type ContestStatus = 'upcoming' | 'running' | 'frozen' | 'ended'

function getStatus(start: string, end: string, freeze?: string): ContestStatus {
    const now = Date.now()
    const s = new Date(start).getTime()
    const e = new Date(end).getTime()
    const f = freeze ? new Date(freeze).getTime() : null
    if (now < s) return 'upcoming'
    if (now > e) return 'ended'
    if (f && now >= f) return 'frozen'
    return 'running'
}

function StatusBadge({ status }: { status: ContestStatus }) {
    const map: Record<ContestStatus, { label: string; cls: string; dot: string }> = {
        upcoming: { label: 'Before start', cls: 'bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300 border-blue-200 dark:border-blue-700', dot: 'bg-blue-50 dark:bg-blue-900/200' },
        running: { label: 'Running', cls: 'bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-300 border-green-200 dark:border-green-700', dot: 'bg-green-50 dark:bg-green-900/200 animate-pulse' },
        frozen: { label: 'Running (frozen)', cls: 'bg-amber-50 dark:bg-amber-900/20 text-amber-700 dark:text-amber-300 border-amber-200 dark:border-amber-700', dot: 'bg-amber-50 dark:bg-amber-900/200' },
        ended: { label: 'Finished', cls: 'bg-gray-100 dark:bg-gray-900/30 text-gray-600 dark:text-gray-400 border-gray-200 dark:border-gray-700', dot: 'bg-gray-400' },
    }
    const m = map[status]
    return (
        <span className={`inline-flex items-center gap-1.5 text-xs font-semibold px-2.5 py-1 rounded-full border ${m.cls}`}>
            <span className={`w-1.5 h-1.5 rounded-full ${m.dot}`} />
            {m.label}
        </span>
    )
}

function FormatBadge({ format }: { format?: string }) {
    if (!format) return null
    const cls: Record<string, string> = {
        ioi: 'bg-indigo-50 text-indigo-700 border-indigo-200',
        icpc: 'bg-sky-50 text-sky-700 border-sky-200',
        cf: 'bg-purple-50 dark:bg-purple-900/20 text-purple-700 dark:text-purple-300 border-purple-200 dark:border-purple-700',
    }
    return (
        <span className={`inline-block text-xs font-semibold px-2 py-0.5 rounded border uppercase ${cls[format] || 'bg-gray-50 dark:bg-gray-900/20 text-gray-600 dark:text-gray-400 border-gray-200 dark:border-gray-700'}`}>
            {format}
        </span>
    )
}

function TypeBadge({ type }: { type?: string }) {
    if (!type || type === 'standard' || type === 'acm' || type === 'oi' || type === 'ioi' || type === 'practice') return null
    const cls: Record<string, string> = {
        educational: 'bg-emerald-50 dark:bg-emerald-900/20 text-emerald-700 dark:text-emerald-300 border-emerald-200 dark:border-emerald-700',
        open: 'bg-teal-50 text-teal-700 border-teal-200',
    }
    return (
        <span className={`inline-block text-xs font-semibold px-2 py-0.5 rounded border capitalize ${cls[type] || 'bg-gray-50 dark:bg-gray-900/20 text-gray-600 dark:text-gray-400 border-gray-200 dark:border-gray-700'}`}>
            {type}
        </span>
    )
}

// ─── Timer Displays ────────────────────────────────────────────────────────────

function CountdownTimer({ target, label }: { target: string; label: string }) {
    const [remaining, setRemaining] = useState(() => new Date(target).getTime() - Date.now())

    useEffect(() => {
        const iv = setInterval(() => setRemaining(new Date(target).getTime() - Date.now()), 1000)
        return () => clearInterval(iv)
    }, [target])

    if (remaining <= 0) return null

    return (
        <div className="bg-gradient-to-r from-blue-600 to-indigo-600 text-white rounded-xl px-6 py-4 flex items-center gap-6 shadow-lg">
            <div className="text-sm font-medium opacity-90">{label}</div>
            <div className="font-mono text-3xl font-bold tracking-wider">{formatCountdown(remaining)}</div>
        </div>
    )
}

function RunningTimer({ start, end }: { start: string; end: string }) {
    const [now, setNow] = useState(Date.now())

    useEffect(() => {
        const iv = setInterval(() => setNow(Date.now()), 1000)
        return () => clearInterval(iv)
    }, [])

    const startTime = new Date(start).getTime()
    const endTime = new Date(end).getTime()
    const total = endTime - startTime
    const elapsed = now - startTime
    const remaining = endTime - now
    const progress = Math.min(100, Math.max(0, (elapsed / total) * 100))

    return (
        <div className="bg-gradient-to-r from-emerald-600 to-teal-600 text-white rounded-xl px-6 py-4 shadow-lg">
            <div className="flex items-center justify-between mb-2">
                <div className="flex items-center gap-2">
                    <span className="w-2 h-2 rounded-full bg-white animate-pulse" />
                    <span className="text-sm font-medium opacity-90">Contest is running</span>
                </div>
                <span className="text-sm opacity-75">Elapsed: {formatDuration(elapsed)}</span>
            </div>
            <div className="font-mono text-3xl font-bold tracking-wider mb-3">
                {formatCountdown(remaining)} <span className="text-base font-normal opacity-70">remaining</span>
            </div>
            <div className="w-full bg-white/20 rounded-full h-2">
                <div className="bg-white rounded-full h-2 transition-all duration-1000" style={{ width: `${progress}%` }} />
            </div>
        </div>
    )
}

// ─── SidebarBox ────────────────────────────────────────────────────────────────

const SIDEBAR_ICONS: Record<string, React.ReactNode> = {
    info: <Info className="w-4 h-4" />,
    users: <Users className="w-4 h-4" />,
    zap: <Zap className="w-4 h-4" />,
    settings: <Settings className="w-4 h-4" />,
    book: <BookOpen className="w-4 h-4" />,
    game: <Gamepad2 className="w-4 h-4" />,
    shield: <Shield className="w-4 h-4" />,
    clock: <Clock className="w-4 h-4" />,
}

function SidebarBox({ title, icon, children, accent }: {
    title: string; icon: string; children: React.ReactNode; accent?: string
}) {
    const borderCls = accent === 'purple' ? 'border-purple-200 dark:border-purple-700' : accent === 'green' ? 'border-green-200 dark:border-green-700' : accent === 'amber' ? 'border-amber-200 dark:border-amber-700' : 'border-gray-200 dark:border-gray-700'
    const headerCls = accent === 'purple' ? 'bg-purple-50 dark:bg-purple-900/20' : accent === 'green' ? 'bg-green-50 dark:bg-green-900/20' : accent === 'amber' ? 'bg-amber-50 dark:bg-amber-900/20' : 'bg-gray-50 dark:bg-gray-900/20'
    const iconNode = SIDEBAR_ICONS[icon]
    return (
        <div className={`bg-white dark:bg-gray-800 border ${borderCls} rounded-xl shadow-sm overflow-hidden`}>
            <div className={`px-4 py-2.5 ${headerCls} border-b ${borderCls}`}>
                <h3 className="text-xs font-bold text-gray-600 dark:text-gray-400 uppercase tracking-wider flex items-center gap-1.5">
                    {iconNode}{title}
                </h3>
            </div>
            <div className="px-4 py-3">{children}</div>
        </div>
    )
}

// ─── Main Component ────────────────────────────────────────────────────────────

type Tab = 'problems' | 'standings' | 'submissions' | 'clarifications' | 'announcements'

export default function ContestDetail() {
    const { id } = useParams<{ id: string }>()
    const role = decodeRole()
    const userId = decodeUserId()
    const isAdmin = role === 'admin' || role === 'owner'

    const [contest, setContest] = useState<any>(null)
    const [problems, setProblems] = useState<any[]>([])
    const [loading, setLoading] = useState(true)
    const [activeTab, setActiveTab] = useState<Tab>(() => {
        const hash = window.location.hash.replace('#', '') as Tab
        const validTabs: Tab[] = ['problems', 'standings', 'submissions', 'clarifications', 'announcements']
        return validTabs.includes(hash) ? hash : 'problems'
    })

    // Sync URL hash when tab changes
    useEffect(() => {
        window.location.hash = activeTab
    }, [activeTab])

    // Registration
    const [registered, setRegistered] = useState(false)
    const [registrationCount, setRegistrationCount] = useState(0)

    // Clarifications
    const [clarifications, setClarifications] = useState<any[]>([])
    const [showForm, setShowForm] = useState(false)
    const [question, setQuestion] = useState('')

    // Standings
    const [standings, setStandings] = useState<any[]>([])
    const [standingsProblems, setStandingsProblems] = useState<any[]>([])
    const [standingsFrozen, setStandingsFrozen] = useState(false)
    const [standingsPage, setStandingsPage] = useState(1)
    const [standingsPagination, setStandingsPagination] = useState<{ page: number; total_pages: number; total: number } | null>(null)
    // First solver per problem (earliest solve time)
    const firstSolvers = useMemo(() => {
        const probList = standingsProblems.length > 0 ? standingsProblems : problems
        const map: Record<string, string> = {} // problem index -> user_id of first solver
        const bestTime: Record<string, number> = {}
        for (const row of standings) {
            for (let pi = 0; pi < probList.length; pi++) {
                const pidx = probList[pi].index || String.fromCharCode(65 + pi)
                const c = row.problems?.[pidx]
                if (c?.solved && c.attempts > 0) {
                    if (!(pidx in bestTime) || c.time < bestTime[pidx]) {
                        bestTime[pidx] = c.time
                        map[pidx] = row.user_id
                    }
                }
            }
        }
        return map
    }, [standings, standingsProblems, problems])
    // Announcements
    const [announcements, setAnnouncements] = useState<any[]>([])
    const [announceText, setAnnounceText] = useState('')
    const [unreadAnnouncements, setUnreadAnnouncements] = useState(0)
    const [unreadClarifications, setUnreadClarifications] = useState(0)

    // Submissions
    const [submissions, setSubmissions] = useState<any[]>([])
    const [submissionsTotal, setSubmissionsTotal] = useState(0)
    const [submissionsOffset, setSubmissionsOffset] = useState(0)
    const submissionsLimit = 20
    const [submissionsMine, setSubmissionsMine] = useState(true)
    const [subFilterProblem, setSubFilterProblem] = useState('')
    const [subFilterLang, setSubFilterLang] = useState('')
    const [subFilterStatus, setSubFilterStatus] = useState('')
    const [subFilterId, setSubFilterId] = useState('')
    const [isJudge, setIsJudge] = useState(false)

    useEffect(() => {
        if (!id) return
        api.contests.get(id).then(res => {
            setContest(res.contest || res)
            setProblems(res.problems || [])
            setRegistered((res.contest || res).is_registered ?? false)
            setRegistrationCount((res.contest || res).registration_count ?? 0)
        }).finally(() => setLoading(false))
    }, [id])

    const fetchClarifications = useCallback(() => {
        if (!id) return
        const lastSeen = localStorage.getItem(`contest_${id}_last_clarif`) || '0'
        api.clarifications.list(id).then(res => {
            const items = res.data || []
            const newCount = items.filter((c: any) => new Date(c.created_at).getTime() > Number(lastSeen)).length
            setUnreadClarifications(newCount)
            setClarifications(items)
        }).catch(() => {})
    }, [id])

    useEffect(() => {
        if (activeTab === 'clarifications') {
            fetchClarifications()
            if (id) localStorage.setItem(`contest_${id}_last_clarif`, String(Date.now()))
            setUnreadClarifications(0)
        }
        if (activeTab === 'announcements' && id) {
            localStorage.setItem(`contest_${id}_last_notice`, String(Date.now()))
            setUnreadAnnouncements(0)
        }
        // Reset standings page when switching tabs
        if (activeTab !== 'standings') {
            setStandingsPage(1)
        }
    }, [activeTab, fetchClarifications, id])

    useEffect(() => {
        if (activeTab === 'standings' && id) {
            api.contests.standings(id, standingsPage).then((res: any) => {
                setStandings(res.entries || [])
                setStandingsProblems(res.problems || [])
                setStandingsFrozen(res.frozen || false)
                setStandingsPagination(res.pagination || null)
            }).catch(() => {})
        }
    }, [activeTab, id, standingsPage])

    useEffect(() => {
        if (activeTab === 'submissions' && id && getAccessToken()) {
            const filters: any = { mine: submissionsMine }
            if (subFilterProblem) filters.problem_id = subFilterProblem
            if (subFilterLang) filters.language = subFilterLang
            if (subFilterStatus) filters.status = subFilterStatus
            api.contests.submissions(id, submissionsOffset, submissionsLimit, filters).then((res: any) => {
                setSubmissions(res.data || [])
                setSubmissionsTotal(res.total || 0)
                setIsJudge(res.is_judge || false)
            }).catch(() => {})
        }
    }, [activeTab, id, submissionsOffset, submissionsMine, subFilterProblem, subFilterLang, subFilterStatus])

    const filteredSubmissions = useMemo(() => {
        let result = submissions
        if (subFilterId) {
            result = result.filter((s: any) => s.id?.toLowerCase().includes(subFilterId.toLowerCase()))
        }
        if (contest?.freeze_time) {
            const freezeTime = new Date(contest.freeze_time).getTime()
            const now = Date.now()
            const endTime = new Date(contest.end_time).getTime()
            if (now >= freezeTime && now < endTime) {
                result = result.filter((s: any) => new Date(s.created_at).getTime() < freezeTime)
            }
        }
        return result
    }, [submissions, subFilterId, contest])

    useEffect(() => {
        if (!id) return
        const pollNotices = () => {
            api.contests.announcements(id).then((res: any) => {
                const items = Array.isArray(res) ? res : res.data || []
                // Show popup when new announcements arrive (after first load)
                if (announcements.length > 0 && items.length > announcements.length) {
                    const newest = items[items.length - 1]
                    if (newest) {
                        setUnreadAnnouncements(prev => prev + 1)
                        if (Notification.permission === 'granted') {
                            new Notification('New Announcement', { body: newest.content, tag: `contest_${id}_notice` })
                        } else if (Notification.permission !== 'denied') {
                            Notification.requestPermission()
                        }
                        alert(`New Announcement:\n\n${newest.content}`)
                    }
                }
                setAnnouncements(items)
                // Calculate unread from localStorage
                const lastSeen = localStorage.getItem(`contest_${id}_last_notice`) || '0'
                setUnreadAnnouncements(items.filter((a: any) => new Date(a.created_at).getTime() > Number(lastSeen)).length)
            }).catch(() => {})
        }
        pollNotices()
        const interval = setInterval(pollNotices, 15000)
        return () => clearInterval(interval)
    }, [id])

    const handleRegister = async () => {
        if (!id) return
        try {
            await api.contests.register(id)
            setRegistered(true)
            setRegistrationCount(c => c + 1)
        } catch (e: any) { alert(e.message) }
    }

    const handleUnregister = async () => {
        if (!id) return
        try {
            await api.contests.unregister(id)
            setRegistered(false)
            setRegistrationCount(c => Math.max(0, c - 1))
        } catch (e: any) { alert(e.message) }
    }

    const handleAsk = async () => {
        if (!id || !question.trim()) return
        try {
            await api.clarifications.create(id, { question: question.trim() })
            setQuestion('')
            setShowForm(false)
            fetchClarifications()
        } catch (e: any) { alert(e.message) }
    }

    const handleAnnounce = async () => {
        if (!id || !announceText.trim()) return
        try {
            await api.contests.postAnnouncement(id, announceText.trim())
            setAnnounceText('')
            api.contests.announcements(id).then((res: any) => setAnnouncements(Array.isArray(res) ? res : res.data || [])).catch(() => {})
        } catch (e: any) { alert(e.message) }
    }

    const handleDeleteAnnouncement = async (announcementId: string) => {
        if (!id || !confirm('Delete this announcement?')) return
        try {
            await api.contests.deleteAnnouncement(id, announcementId)
            setAnnouncements(a => a.filter((x: any) => x.id !== announcementId))
        } catch (e: any) { alert(e.message) }
    }

    if (loading) return <div className="text-center py-20 text-gray-500 dark:text-gray-400">Loading...</div>
    if (!contest) return <div className="text-center py-20 text-gray-500 dark:text-gray-400">Contest not found</div>

    const status = getStatus(contest.start_time, contest.end_time, contest.freeze_time)
    const isUpcoming = status === 'upcoming'
    const isRunning = status === 'running' || status === 'frozen'
    const isEnded = status === 'ended'
    const totalDuration = new Date(contest.end_time).getTime() - new Date(contest.start_time).getTime()

    return (
        <div className="max-w-7xl mx-auto px-4 py-6">
            {/* ── Hero Header ──────────────────────────────────────────────── */}
            <div className="mb-6">
                <div className="flex items-center gap-3 mb-2">
                    <h1 className="text-3xl font-extrabold text-gray-900 dark:text-gray-100">{contest.title}</h1>
                    <StatusBadge status={status} />
                    {contest.format && <FormatBadge format={contest.format} />}
                    {contest.type && <TypeBadge type={contest.type} />}
                    {contest.division !== undefined && <DivisionBadge division={contest.division} />}
                </div>
            </div>

            {/* ── Timer ────────────────────────────────────────────────────── */}
            {isUpcoming && (
                <div className="mb-6">
                    <CountdownTimer target={contest.start_time} label="Contest starts in" />
                </div>
            )}
            {isRunning && (
                <div className="mb-6">
                    <RunningTimer start={contest.start_time} end={contest.end_time} />
                </div>
            )}

            {/* ── Tab Navigation ───────────────────────────────────────────── */}
            <div className="border-b border-gray-200 dark:border-gray-700 mb-6">
                <nav className="flex gap-6">
                    {([
                        { key: 'problems', label: 'Problems', icon: 'problems' },
                        { key: 'standings', label: 'Standings', icon: 'standings' },
                        { key: 'submissions', label: 'Submissions', icon: 'submissions' },
                        { key: 'clarifications', label: 'Clarifications', icon: 'clarifications', unread: unreadClarifications },
                        { key: 'announcements', label: 'Announcements', icon: 'announcements', unread: unreadAnnouncements },
                    ] as { key: Tab; label: string; icon: string; unread?: number }[]).map(tab => (
                        <button
                            key={tab.key}
                            onClick={() => setActiveTab(tab.key)}
                            className={`pb-3 text-sm font-semibold transition-colors flex items-center gap-1.5 ${
                                activeTab === tab.key
                                    ? 'text-blue-600 dark:text-blue-400 border-b-2 border-blue-600'
                                    : 'text-gray-500 hover:text-gray-700 border-b-2 border-transparent'
                            }`}
                        >
                            {tab.icon === 'problems' && <FileText className="w-4 h-4" />}
                            {tab.icon === 'standings' && <Trophy className="w-4 h-4" />}
                            {tab.icon === 'submissions' && <Send className="w-4 h-4" />}
                            {tab.icon === 'clarifications' && <MessageSquare className="w-4 h-4" />}
                            {tab.icon === 'announcements' && <Megaphone className="w-4 h-4" />}
                            {tab.label}
                            {tab.unread !== undefined && tab.unread > 0 ? (
                                <span className="ml-1 px-1.5 py-0.5 text-xs font-bold bg-red-50 dark:bg-red-900/200 text-white rounded-full min-w-[18px] text-center">{tab.unread}</span>
                            ) : null}
                        </button>
                    ))}
                </nav>
            </div>

            {/* ── Main Layout: Content + Sidebar ───────────────────────────── */}
            <div className={`items-start ${activeTab === 'standings' || activeTab === 'submissions' ? '' : 'flex gap-6'}`}>
                {/* Left: Main Content */}
                <div className={activeTab === 'standings' || activeTab === 'submissions' ? '' : 'flex-1 min-w-0'}>
                    {activeTab === 'problems' && (
                        <div>
                            <h2 className="text-xl font-bold text-gray-900 dark:text-gray-100 mb-4">Problems</h2>
                            {problems.length === 0 ? (
                                <div className="text-center py-16">
                                    <FileText className="w-12 h-12 text-gray-700 dark:text-gray-300 mx-auto mb-3" />
                                    <p className="text-gray-500 dark:text-gray-400">No problems added yet.</p>
                                </div>
                            ) : (
                                <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm overflow-hidden">
                                    <table className="w-full text-sm">
                                        <thead>
                                            <tr className="bg-gray-50 dark:bg-gray-900/20 border-b border-gray-200 dark:border-gray-700">
                                                <th className="text-left px-4 py-3 font-semibold text-gray-600 dark:text-gray-400">#</th>
                                                <th className="text-left px-4 py-3 font-semibold text-gray-600 dark:text-gray-400">Problem</th>
                                                <th className="text-right px-4 py-3 font-semibold text-gray-600 dark:text-gray-400">Score</th>
                                                <th className="text-center px-4 py-3 font-semibold text-gray-600 dark:text-gray-400">Action</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            {problems.map((p: any, i: number) => (
                                                <tr key={p.problem_id} className="border-b border-gray-100 dark:border-gray-800 last:border-0 hover:bg-gray-50 transition-colors">
                                                    <td className="px-4 py-3 font-mono font-bold text-gray-500">{p.index || String.fromCharCode(65 + i)}</td>
                                                    <td className="px-4 py-3">
                                                        <Link
                                                            to={`/contests/${id}/problem/${p.index || String.fromCharCode(65 + i)}`}
                                                            className="text-blue-600 dark:text-blue-400 hover:text-blue-800 hover:underline font-medium"
                                                        >
                                                            {p.title || `Problem ${p.index || String.fromCharCode(65 + i)}`}
                                                        </Link>
                                                    </td>
                                                    <td className="px-4 py-3 text-right font-mono text-gray-700 dark:text-gray-300">{p.score ?? 100}</td>
                                                    <td className="px-4 py-3 text-center">
                                                        <Link
                                                            to={`/contests/${id}/problem/${p.index || String.fromCharCode(65 + i)}`}
                                                            className="inline-block px-3 py-1 bg-blue-600 text-white text-xs font-semibold rounded-lg hover:bg-blue-700 transition-colors"
                                                        >
                                                            Solve
                                                        </Link>
                                                    </td>
                                                </tr>
                                            ))}
                                        </tbody>
                                    </table>
                                </div>
                            )}
                        </div>
                    )}

                    {activeTab === 'standings' && (
                        <div>
                            <div className="flex items-center justify-between mb-4">
                                <h2 className="text-xl font-bold text-gray-900 dark:text-gray-100 flex items-center gap-2">
                                    <Trophy className="w-5 h-5" /> Standings
                                </h2>
                                <div className="flex items-center gap-2">
                                    {standingsFrozen && (
                                        <span className="px-3 py-1 bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400 text-xs font-semibold rounded-full flex items-center gap-1 border border-blue-200 dark:border-blue-800">
                                            <Clock className="w-3 h-3" /> Frozen
                                        </span>
                                    )}
                                    <a
                                        href={`/contests/${id}/scoreboard`}
                                        target="_blank"
                                        rel="noopener noreferrer"
                                        className="inline-flex items-center gap-1.5 px-3 py-1 text-xs font-semibold text-gray-500 dark:text-gray-400 hover:text-blue-600 dark:hover:text-blue-400 border border-gray-200 dark:border-gray-700 rounded-full hover:border-blue-300 dark:hover:border-blue-700 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
                                    >
                                        <ExternalLink className="w-3 h-3" />
                                        Full Scoreboard
                                    </a>
                                </div>
                            </div>
                            {standings.length === 0 ? (
                                <div className="text-center py-16 bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl">
                                    <Trophy className="w-12 h-12 text-gray-600 dark:text-gray-400 mx-auto mb-3" />
                                    <p className="text-gray-500 dark:text-gray-400 font-medium">No submissions yet</p>
                                    <p className="text-gray-500 text-sm mt-1">Standings will appear once participants start solving</p>
                                </div>
                            ) : (
                                <div className="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl shadow-md overflow-x-auto">
                                    <table className="w-full text-sm">
                                        <thead>
                                            <tr className="bg-gray-50 dark:bg-gray-800 text-gray-900 dark:text-gray-100 border-b border-gray-200 dark:border-gray-700">
                                                <th className="text-center px-3 py-3 font-bold text-xs uppercase tracking-wider w-16">Rank</th>
                                                <th className="text-left px-4 py-3 font-bold text-xs uppercase tracking-wider min-w-[180px]">Team</th>
                                                <th className="text-center px-3 py-3 font-bold text-xs uppercase tracking-wider w-20">
                                                    <span className="text-emerald-700 dark:text-emerald-400">=</span>
                                                </th>
                                                <th className="text-center px-3 py-3 font-bold text-xs uppercase tracking-wider w-20">Penalty</th>
                                                {(standingsProblems.length > 0 ? standingsProblems : problems).map((p: any, i: number) => (
                                                    <th key={i} className="text-center px-1 py-3 font-bold text-xs uppercase tracking-wider w-16">
                                                        <div className="flex flex-col items-center">
                                                            <span className="text-base">{String.fromCharCode(65 + i)}</span>
                                                            {p.title && <span className="text-[10px] text-gray-500 dark:text-gray-400 font-normal normal-case truncate max-w-[60px]" title={p.title}>{p.title}</span>}
                                                        </div>
                                                    </th>
                                                ))}
                                            </tr>
                                        </thead>
                                        <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                                            {(standingsProblems.length > 0 ? standingsProblems : problems).length > 0 && standings.map((row: any, idx: number) => {
                                                const probList = standingsProblems.length > 0 ? standingsProblems : problems
                                                const isMe = row.user_id === userId
                                                const rank = row.rank || idx + 1
                                                const medal = rank === 1 ? 'gold' : rank === 2 ? 'silver' : rank === 3 ? 'bronze' : null
                                                const sec = row.total_penalty ?? 0
                                                const ph = Math.floor(sec / 3600)
                                                const pm = Math.floor((sec % 3600) / 60)
                                                const ps = sec % 60
                                                const penaltyStr = ph > 0 ? `${ph}:${String(pm).padStart(2,'0')}:${String(ps).padStart(2,'0')}` : `${pm}:${String(ps).padStart(2,'0')}`
                                                return (
                                                    <tr key={row.user_id || idx} className={`transition-colors ${isMe ? 'bg-blue-50 dark:bg-blue-900/20 hover:bg-blue-100 dark:hover:bg-blue-900/30' : idx % 2 === 0 ? 'bg-white dark:bg-gray-900 hover:bg-gray-100 dark:hover:bg-gray-800' : 'bg-gray-50 dark:bg-gray-800/50 hover:bg-gray-100 dark:hover:bg-gray-800'}`}>
                                                        <td className="text-center px-3 py-2.5">
                                                            {medal ? (
                                                                <span className={`inline-flex items-center justify-center w-8 h-8 rounded-full text-sm font-black border ${
                                                                    medal === 'gold' ? 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-400 border-yellow-300 dark:border-yellow-700' :
                                                                    medal === 'silver' ? 'bg-gray-200 dark:bg-gray-700/50 text-gray-700 dark:text-gray-300 border-gray-300 dark:border-gray-600' :
                                                                    'bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-400 border-orange-300 dark:border-orange-700'
                                                                }`}>{rank}</span>
                                                            ) : (
                                                                <span className="text-sm font-bold text-gray-500 dark:text-gray-400">{rank}</span>
                                                            )}
                                                        </td>
                                                        <td className={`px-4 py-2.5 font-semibold ${isMe ? 'text-blue-700 dark:text-blue-400' : 'text-gray-800 dark:text-gray-200'}`}>
                                                            {row.username}
                                                            {isMe && <span className="ml-2 text-[10px] bg-blue-100 dark:bg-blue-900/40 text-blue-800 dark:text-blue-300 px-1.5 py-0.5 rounded font-bold border border-blue-200 dark:border-blue-800">YOU</span>}
                                                        </td>
                                                        <td className="text-center px-3 py-2.5">
                                                            <span className="text-lg font-black text-gray-900 dark:text-gray-100">{row.total_solved ?? 0}</span>
                                                        </td>
                                                        <td className="text-center px-3 py-2.5 font-mono text-gray-500 dark:text-gray-400 text-xs">{penaltyStr}</td>
                                                        {probList.map((p: any, i: number) => {
                                                            const pidx = p.index || String.fromCharCode(65 + i)
                                                            const cell = row.problems?.[pidx] || {}
                                                            const solved = cell.solved
                                                            const attempts = cell.attempts || 0
                                                            const pending = cell.pending || 0
                                                            const timeSec = cell.time || 0
                                                            const isFirst = solved && firstSolvers[pidx] === row.user_id
                                                            const timeMin = Math.floor(timeSec / 60)
                                                            if (solved) {
                                                                return (
                                                                    <td key={i} className="text-center px-1 py-2.5">
                                                                        <div className={`inline-flex flex-col items-center justify-center rounded-md px-2 py-1 min-w-[48px] ${isFirst ? 'bg-emerald-600 text-white shadow-sm' : 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-800'}`}>
                                                                            <span className="text-xs font-black leading-none">{attempts <= 1 ? '+' : `+${attempts - 1}`}</span>
                                                                            <span className="text-[10px] font-mono leading-none mt-0.5 opacity-70">{timeMin}m</span>
                                                                        </div>
                                                                    </td>
                                                                )
                                                            }
                                                            if (pending > 0) {
                                                                return (
                                                                    <td key={i} className="text-center px-1 py-2.5">
                                                                        <div className="inline-flex flex-col items-center justify-center bg-sky-100 dark:bg-sky-900/30 text-sky-700 dark:text-sky-400 border border-sky-200 dark:border-sky-800 rounded-md px-2 py-1 min-w-[48px]">
                                                                            <span className="text-xs font-black leading-none">?</span>
                                                                            <span className="text-[10px] font-mono leading-none mt-0.5">{pending}</span>
                                                                        </div>
                                                                    </td>
                                                                )
                                                            }
                                                            if (attempts > 0) {
                                                                return (
                                                                    <td key={i} className="text-center px-1 py-2.5">
                                                                        <div className="inline-flex items-center justify-center bg-rose-100 dark:bg-rose-900/30 text-rose-700 dark:text-rose-400 border border-rose-200 dark:border-rose-800 rounded-md px-2 py-1 min-w-[48px]">
                                                                            <span className="text-xs font-black">-{attempts}</span>
                                                                        </div>
                                                                    </td>
                                                                )
                                                            }
                                                            return <td key={i} className="text-center px-1 py-2.5"><span className="text-gray-600 dark:text-gray-400 font-medium">-</span></td>
                                                        })}
                                                    </tr>
                                                )
                                            })}
                                        </tbody>
                                    </table>
                                </div>
                            )}
                            {/* Participant count & Pagination */}
                            {standingsPagination && standingsPagination.total > 0 && (
                                <div className="mt-3 flex flex-col items-center gap-3">
                                    <span className="text-xs text-gray-500 dark:text-gray-400">
                                        {standingsPagination.total} participant{standingsPagination.total !== 1 ? 's' : ''}
                                    </span>
                                    {standingsPagination.total_pages > 1 && (() => {
                                        const { page, total_pages } = standingsPagination;
                                        const pages: (number | 'ellipsis')[] = [];
                                        if (total_pages <= 7) {
                                            for (let i = 1; i <= total_pages; i++) pages.push(i);
                                        } else {
                                            const left = Math.max(2, page - 1);
                                            const right = Math.min(total_pages - 1, page + 1);
                                            pages.push(1);
                                            if (left > 2) pages.push('ellipsis');
                                            for (let i = left; i <= right; i++) pages.push(i);
                                            if (right < total_pages - 1) pages.push('ellipsis');
                                            pages.push(total_pages);
                                        }
                                        return (
                                            <div className="flex items-center justify-center gap-1">
                                                <button
                                                    onClick={() => setStandingsPage(p => Math.max(1, p - 1))}
                                                    disabled={page <= 1}
                                                    className="inline-flex items-center gap-1 px-2.5 py-1.5 text-sm font-medium rounded-md border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 text-gray-700 dark:text-gray-300 hover:bg-gray-700 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
                                                >
                                                    <ChevronLeft className="w-4 h-4" />
                                                    Prev
                                                </button>
                                                {pages.map((p, i) =>
                                                    p === 'ellipsis' ? (
                                                        <span key={`e${i}`} className="px-2 py-1.5 text-sm text-gray-500">...</span>
                                                    ) : (
                                                        <button
                                                            key={p}
                                                            onClick={() => setStandingsPage(p)}
                                                            className={`min-w-[36px] px-2 py-1.5 text-sm font-medium rounded-md border transition-colors ${
                                                                p === page
                                                                    ? 'bg-blue-600 text-white border-blue-600 shadow-sm'
                                                                    : 'bg-gray-50 dark:bg-gray-800 text-gray-700 dark:text-gray-300 border-gray-200 dark:border-gray-700 hover:bg-gray-700'
                                                            }`}
                                                        >
                                                            {p}
                                                        </button>
                                                    )
                                                )}
                                                <button
                                                    onClick={() => setStandingsPage(p => Math.min(total_pages, p + 1))}
                                                    disabled={page >= total_pages}
                                                    className="inline-flex items-center gap-1 px-2.5 py-1.5 text-sm font-medium rounded-md border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 text-gray-700 dark:text-gray-300 hover:bg-gray-700 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
                                                >
                                                    Next
                                                    <ChevronRight className="w-4 h-4" />
                                                </button>
                                            </div>
                                        );
                                    })()}
                                </div>
                            )}
                        </div>
                    )}

                    {activeTab === 'submissions' && (
                        <div>
                            <div className="flex items-center justify-between mb-4">
                                <h2 className="text-xl font-bold text-gray-900 dark:text-gray-100 flex items-center gap-2">
                                    <Send className="w-5 h-5" /> Submissions
                                </h2>
                                {getAccessToken() && (
                                    <div className="flex items-center bg-gray-100 dark:bg-gray-900/30 rounded-lg p-0.5">
                                        <button
                                            onClick={() => { setSubmissionsMine(true); setSubmissionsOffset(0) }}
                                            className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded-md transition-colors ${submissionsMine ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm' : 'text-gray-500 hover:text-gray-700'}`}
                                        >
                                            <Eye className="w-3.5 h-3.5" /> My Submissions
                                        </button>
                                        <button
                                            onClick={() => { setSubmissionsMine(false); setSubmissionsOffset(0) }}
                                            className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded-md transition-colors ${!submissionsMine ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm' : 'text-gray-500 hover:text-gray-700'}`}
                                        >
                                            <EyeOff className="w-3.5 h-3.5" /> All Submissions
                                        </button>
                                    </div>
                                )}
                            </div>

                            {status === 'frozen' && (
                                <div className="mb-4 p-3 bg-blue-50 border border-blue-200 rounded-lg flex items-center gap-2">
                                    <Clock className="w-4 h-4 text-blue-600" />
                                    <span className="text-sm text-blue-700">Standings frozen — only submissions before freeze are shown.</span>
                                </div>
                            )}

                            {/* Filter controls */}
                            {getAccessToken() && (
                                <div className="flex flex-wrap items-center gap-3 mb-4 p-3 bg-gray-50 dark:bg-gray-900/20 border border-gray-200 dark:border-gray-700 rounded-lg">
                                    <Filter className="w-4 h-4 text-gray-500 dark:text-gray-400" />
                                    <select
                                        value={subFilterProblem}
                                        onChange={e => { setSubFilterProblem(e.target.value); setSubmissionsOffset(0) }}
                                        className="text-sm border border-gray-200 dark:border-gray-700 rounded-md px-2.5 py-1.5 bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                                    >
                                        <option value="">All Problems</option>
                                        {problems.map((p: any) => (
                                            <option key={p.problem_id} value={p.problem_id}>{p.index ? `${p.index} - ` : ''}{p.title}</option>
                                        ))}
                                    </select>
                                    <select
                                        value={subFilterLang}
                                        onChange={e => { setSubFilterLang(e.target.value); setSubmissionsOffset(0) }}
                                        className="text-sm border border-gray-200 dark:border-gray-700 rounded-md px-2.5 py-1.5 bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                                    >
                                        <option value="">All Languages</option>
                                        {Array.from(new Set(submissions.map((s: any) => s.language).filter(Boolean))).sort().map((lang: any) => (
                                            <option key={lang} value={lang}>{lang}</option>
                                        ))}
                                    </select>
                                    <select
                                        value={subFilterStatus}
                                        onChange={e => { setSubFilterStatus(e.target.value); setSubmissionsOffset(0) }}
                                        className="text-sm border border-gray-200 dark:border-gray-700 rounded-md px-2.5 py-1.5 bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                                    >
                                        <option value="">All Verdicts</option>
                                        <option value="ac">Accepted</option>
                                        <option value="wa">Wrong Answer</option>
                                        <option value="tle">Time Limit</option>
                                        <option value="mle">Memory Limit</option>
                                        <option value="re">Runtime Error</option>
                                        <option value="ce">Compile Error</option>
                                        <option value="se">Internal Error</option>
                                        <option value="pending">Pending</option>
                                        <option value="judging">Judging</option>
                                    </select>
                                    <input
                                        type="text"
                                        value={subFilterId}
                                        onChange={e => { setSubFilterId(e.target.value); setSubmissionsOffset(0) }}
                                        placeholder="Submission ID..."
                                        className="text-sm border border-gray-200 dark:border-gray-700 rounded-md px-2.5 py-1.5 bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 focus:ring-2 focus:ring-blue-500 focus:border-transparent w-44"
                                    />
                                    {(subFilterProblem || subFilterLang || subFilterStatus || subFilterId) && (
                                        <button
                                            onClick={() => { setSubFilterProblem(''); setSubFilterLang(''); setSubFilterStatus(''); setSubFilterId(''); setSubmissionsOffset(0) }}
                                            className="text-xs text-gray-500 hover:text-red-600 font-medium"
                                        >
                                            Clear Filters
                                        </button>
                                    )}
                                </div>
                            )}

                            {!getAccessToken() ? (
                                <div className="text-center py-16">
                                    <Send className="w-12 h-12 text-gray-700 dark:text-gray-300 mx-auto mb-3" />
                                    <p className="text-gray-500 dark:text-gray-400">Please log in to view submissions.</p>
                                </div>
                            ) : filteredSubmissions.length === 0 ? (
                                <div className="text-center py-16">
                                    <Send className="w-12 h-12 text-gray-700 dark:text-gray-300 mx-auto mb-3" />
                                    <p className="text-gray-500 font-medium">No submissions found</p>
                                    {(subFilterProblem || subFilterLang || subFilterStatus || subFilterId) && (
                                        <p className="text-gray-500 dark:text-gray-400 text-sm mt-1">Try adjusting your filters</p>
                                    )}
                                </div>
                            ) : (
                                <>
                                    <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm overflow-x-auto">
                                        <table className="w-full text-sm">
                                            <thead>
                                                <tr className="bg-gray-50 dark:bg-gray-900/20 border-b border-gray-200 dark:border-gray-700">
                                                    <th className="text-left px-4 py-3 font-semibold text-gray-600 dark:text-gray-400">#</th>
                                                    {(!submissionsMine || isJudge) && <th className="text-left px-4 py-3 font-semibold text-gray-600 dark:text-gray-400">User</th>}
                                                    <th className="text-left px-4 py-3 font-semibold text-gray-600 dark:text-gray-400">Problem</th>
                                                    <th className="text-center px-4 py-3 font-semibold text-gray-600 dark:text-gray-400">Lang</th>
                                                    <th className="text-center px-4 py-3 font-semibold text-gray-600 dark:text-gray-400">Verdict</th>
                                                    <th className="text-center px-4 py-3 font-semibold text-gray-600 dark:text-gray-400">Score</th>
                                                    <th className="text-center px-4 py-3 font-semibold text-gray-600 dark:text-gray-400">Time</th>
                                                    <th className="text-center px-4 py-3 font-semibold text-gray-600 dark:text-gray-400">Memory</th>
                                                    <th className="text-right px-4 py-3 font-semibold text-gray-600 dark:text-gray-400">Submitted</th>
                                                </tr>
                                            </thead>
                                            <tbody>
                                                {filteredSubmissions.map((s: any, i: number) => {
                                                    const verdictCls: Record<string, string> = {
                                                        ac: 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300 border-green-300 dark:border-green-700',
                                                        wa: 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300 border-red-300 dark:border-red-700',
                                                        tle: 'bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-300 border-amber-300 dark:border-amber-700',
                                                        mle: 'bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-300 border-amber-300 dark:border-amber-700',
                                                        re: 'bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-300 border-orange-300 dark:border-orange-700',
                                                        ce: 'bg-gray-100 dark:bg-gray-900/30 text-gray-600 dark:text-gray-400 border-gray-300 dark:border-gray-700',
                                                        pending: 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700',
                                                        judging: 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700',
                                                        se: 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300 border-red-300 dark:border-red-700',
                                                        ie: 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300 border-red-300 dark:border-red-700',
                                                    }
                                                    const verdictLabel: Record<string, string> = {
                                                        ac: 'Accepted', wa: 'Wrong Answer', tle: 'Time Limit',
                                                        mle: 'Memory Limit', re: 'Runtime Error', ce: 'Compile Error',
                                                        pending: 'Pending', judging: 'Judging', se: 'Internal Error', ie: 'Internal Error',
                                                    }
                                                    const problem = problems.find((p: any) => p.problem_id === s.problem_id)
                                                    const problemLabel = problem ? `${problem.index || ''} — ${problem.title}` : s.problem_id?.slice(0, 8)
                                                    const canClick = s.user_id === userId || isJudge
                                                    const rowCls = canClick ? 'cursor-pointer hover:bg-blue-50' : 'hover:bg-gray-50'
                                                    return (
                                                        <tr
                                                            key={s.id}
                                                            className={`border-b border-gray-100 dark:border-gray-800 last:border-0 ${rowCls} transition-colors`}
                                                            onClick={canClick ? () => window.open(`/submissions/${s.id}`, '_blank') : undefined}
                                                        >
                                                            <td className="px-4 py-2.5 font-mono text-xs text-gray-500 dark:text-gray-400">{submissionsOffset + i + 1}</td>
                                                            {(!submissionsMine || isJudge) && <td className="px-4 py-2.5 font-medium text-gray-800 dark:text-gray-300">{s.username || s.user_id?.slice(0, 8)}</td>}
                                                            <td className="px-4 py-2.5 text-gray-700 dark:text-gray-300">{problemLabel}</td>
                                                            <td className="px-4 py-2.5 text-center">
                                                                <span className="px-2 py-0.5 bg-gray-100 dark:bg-gray-900/30 text-gray-600 dark:text-gray-400 text-xs font-mono rounded">{s.language}</span>
                                                            </td>
                                                            <td className="px-4 py-2.5 text-center">
                                                                <span className={`px-2 py-0.5 text-xs font-semibold rounded border ${verdictCls[s.status] || 'bg-gray-100 dark:bg-gray-900/30 text-gray-600 dark:text-gray-400 border-gray-300 dark:border-gray-700'}`}>
                                                                    {verdictLabel[s.status] || s.status}
                                                                </span>
                                                            </td>
                                                            <td className="px-4 py-2.5 text-center font-mono text-gray-700 dark:text-gray-300">{s.score ?? '-'}</td>
                                                            <td className="px-4 py-2.5 text-center font-mono text-gray-500 text-xs">{s.time_used ? `${s.time_used}ms` : '-'}</td>
                                                            <td className="px-4 py-2.5 text-center font-mono text-gray-500 text-xs">{s.memory_used ? `${(s.memory_used / 1024).toFixed(1)}MB` : '-'}</td>
                                                            <td className="px-4 py-2.5 text-right text-xs text-gray-500 dark:text-gray-400" title={new Date(s.created_at).toLocaleString()}>{timeAgo(s.created_at)}</td>
                                                        </tr>
                                                    )
                                                })}
                                            </tbody>
                                        </table>
                                    </div>
                                    {/* Pagination */}
                                    {submissionsTotal > submissionsLimit && (
                                        <div className="mt-3 flex items-center justify-center gap-2">
                                            <button
                                                onClick={() => setSubmissionsOffset(o => Math.max(0, o - submissionsLimit))}
                                                disabled={submissionsOffset === 0}
                                                className="px-3 py-1.5 text-sm font-medium rounded-md border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-600 dark:text-gray-400 hover:bg-gray-50 disabled:opacity-40 disabled:cursor-not-allowed"
                                            >Prev</button>
                                            <span className="text-sm text-gray-500">
                                                {Math.floor(submissionsOffset / submissionsLimit) + 1} / {Math.ceil(submissionsTotal / submissionsLimit)}
                                            </span>
                                            <button
                                                onClick={() => setSubmissionsOffset(o => o + submissionsLimit)}
                                                disabled={submissionsOffset + submissionsLimit >= submissionsTotal}
                                                className="px-3 py-1.5 text-sm font-medium rounded-md border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-600 dark:text-gray-400 hover:bg-gray-50 disabled:opacity-40 disabled:cursor-not-allowed"
                                            >Next</button>
                                        </div>
                                    )}
                                    <div className="mt-2 text-xs text-gray-500 dark:text-gray-400 text-center">{submissionsTotal} submission{submissionsTotal !== 1 ? 's' : ''}</div>
                                </>
                            )}
                        </div>
                    )}

                    {activeTab === 'clarifications' && (
                        <div>
                            <div className="flex items-center justify-between mb-4">
                                <h2 className="text-xl font-bold text-gray-900 dark:text-gray-100">Clarifications</h2>
                                {getAccessToken() && (
                                    <button
                                        onClick={() => setShowForm(!showForm)}
                                        className="px-4 py-2 bg-blue-600 text-white text-sm font-semibold rounded-lg hover:bg-blue-700 transition-colors"
                                    >
                                        Ask Question
                                    </button>
                                )}
                            </div>

                            {showForm && (
                                <div className="bg-white dark:bg-gray-800 border border-blue-200 dark:border-blue-700 rounded-xl p-4 mb-4 shadow-sm">
                                    <textarea
                                        value={question}
                                        onChange={e => setQuestion(e.target.value)}
                                        placeholder="Type your question..."
                                        className="w-full border border-gray-200 dark:border-gray-700 rounded-lg p-3 text-sm resize-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                                        rows={3}
                                    />
                                    <div className="flex gap-2 mt-3">
                                        <button onClick={handleAsk} className="px-4 py-2 bg-blue-600 text-white text-sm font-semibold rounded-lg hover:bg-blue-700">Submit</button>
                                        <button onClick={() => setShowForm(false)} className="px-4 py-2 bg-gray-100 dark:bg-gray-900/30 text-gray-600 dark:text-gray-400 text-sm font-semibold rounded-lg hover:bg-gray-200">Cancel</button>
                                    </div>
                                </div>
                            )}

                            {clarifications.length === 0 ? (
                                <div className="text-center py-16">
                                    <MessageSquare className="w-12 h-12 text-gray-700 dark:text-gray-300 mx-auto mb-3" />
                                    <p className="text-gray-500 dark:text-gray-400">No clarifications yet.</p>
                                </div>
                            ) : (
                                <div className="space-y-3">
                                    {clarifications.map((c: any) => (
                                        <div key={c.id} className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl p-4 shadow-sm">
                                            <div className="flex items-start justify-between mb-2">
                                                <span className="text-xs font-medium text-gray-500">{c.username || 'Anonymous'}</span>
                                                <span className="text-xs text-gray-500 dark:text-gray-400">{new Date(c.created_at).toLocaleString()}</span>
                                            </div>
                                            <p className="text-sm text-gray-800 dark:text-gray-300 mb-2">{c.question}</p>
                                            {c.answer && (
                                                <div className={`mt-2 pl-3 border-l-2 ${c.is_public ? 'border-green-400 bg-green-50 dark:bg-green-900/20' : 'border-blue-400 bg-blue-50 dark:bg-blue-900/20'} rounded-r-lg p-3`}>
                                                    <p className="text-sm text-gray-700 dark:text-gray-300">{c.answer}</p>
                                                    {c.is_public && (
                                                        <span className="inline-block text-[10px] text-green-600 dark:text-green-400 font-medium mt-1">
                                                            <Megaphone className="w-3 h-3 inline mr-0.5" />Public response
                                                        </span>
                                                    )}
                                                </div>
                                            )}
                                        </div>
                                    ))}
                                </div>
                            )}
                        </div>
                    )}

                    {activeTab === 'announcements' && (
                        <div>
                            <h2 className="text-xl font-bold text-gray-900 dark:text-gray-100 mb-4 flex items-center gap-2">
                                <Megaphone className="w-5 h-5" /> Announcements
                            </h2>

                            {/* Admin: Post new announcement */}
                            {isAdmin && (
                                <div className="mb-6 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-700 rounded-lg p-4">
                                    <h3 className="text-sm font-semibold text-amber-800 dark:text-amber-300 mb-2">Post Announcement</h3>
                                    <div className="flex gap-2">
                                        <input
                                            type="text"
                                            value={announceText}
                                            onChange={e => setAnnounceText(e.target.value)}
                                            onKeyDown={e => e.key === 'Enter' && handleAnnounce()}
                                            placeholder="Write announcement for all participants..."
                                            className="flex-1 border border-gray-200 dark:border-gray-700 rounded-lg px-3 py-2 text-sm focus:ring-2 focus:ring-amber-500 focus:border-transparent"
                                        />
                                        <button
                                            onClick={handleAnnounce}
                                            className="px-4 py-2 bg-amber-50 dark:bg-amber-900/200 text-white text-sm font-semibold rounded-lg hover:bg-amber-600 transition-colors"
                                        >
                                            Post
                                        </button>
                                    </div>
                                </div>
                            )}

                            {announcements.length === 0 ? (
                                <div className="text-center py-16">
                                    <Megaphone className="w-12 h-12 text-gray-700 dark:text-gray-300 mx-auto mb-3" />
                                    <p className="text-gray-500 font-medium">No announcements yet</p>
                                    <p className="text-gray-500 dark:text-gray-400 text-sm mt-1">Announcements from judges will appear here</p>
                                </div>
                            ) : (
                                <div className="space-y-3">
                                    {announcements.map((a: any, i: number) => (
                                        <div key={a.id} className="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-700 rounded-lg p-4">
                                            <div className="flex items-start justify-between gap-3">
                                                <div className="flex-1">
                                                    <p className="text-gray-800 dark:text-gray-300 text-sm leading-relaxed">{a.content}</p>
                                                    <div className="mt-2 flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                                                        {a.username && <span className="font-medium text-gray-600 dark:text-gray-400">{a.username}</span>}
                                                        <span>{new Date(a.created_at).toLocaleString()}</span>
                                                    </div>
                                                </div>
                                                <div className="flex items-center gap-2 flex-shrink-0">
                                                    <span className="text-xs text-amber-600 dark:text-amber-400 font-bold bg-amber-100 dark:bg-amber-900/30 px-2 py-0.5 rounded">#{i + 1}</span>
                                                    {isAdmin && (
                                                        <button
                                                            onClick={() => handleDeleteAnnouncement(a.id)}
                                                            className="text-red-400 hover:text-red-600 p-1"
                                                        >
                                                            <X className="w-4 h-4" />
                                                        </button>
                                                    )}
                                                </div>
                                            </div>
                                        </div>
                                    ))}
                                </div>
                            )}
                        </div>
                    )}
                </div>

                {/* Right: Sidebar — hidden on standings/submissions tab to maximize table width */}
                {activeTab !== 'standings' && activeTab !== 'submissions' && (
                <div className="w-80 flex-shrink-0 space-y-4 hidden lg:block">
                    {/* Contest Info */}
                    <SidebarBox title="Contest Info" icon="info">
                        <dl className="space-y-2.5 text-sm">
                            <div className="flex justify-between">
                                <dt className="text-gray-500">Start</dt>
                                <dd className="text-gray-800 dark:text-gray-300 font-medium text-right">{formatDateTime(contest.start_time)}</dd>
                            </div>
                            <div className="flex justify-between">
                                <dt className="text-gray-500">End</dt>
                                <dd className="text-gray-800 dark:text-gray-300 font-medium text-right">{formatDateTime(contest.end_time)}</dd>
                            </div>
                            <div className="flex justify-between">
                                <dt className="text-gray-500">Duration</dt>
                                <dd className="text-gray-800 dark:text-gray-300 font-medium">{formatDuration(totalDuration)}</dd>
                            </div>
                            {contest.freeze_time && (
                                <div className="flex justify-between">
                                    <dt className="text-gray-500">Standings freeze</dt>
                                    <dd className="text-gray-800 dark:text-gray-300 font-medium text-right">{formatDateTime(contest.freeze_time)}</dd>
                                </div>
                            )}
                            <div className="border-t border-gray-100 dark:border-gray-800 pt-2 flex justify-between">
                                <dt className="text-gray-500">Format</dt>
                                <dd className="text-gray-800 dark:text-gray-300 font-medium uppercase">{contest.format || 'N/A'}</dd>
                            </div>
                            <div className="flex justify-between">
                                <dt className="text-gray-500">Type</dt>
                                <dd className="text-gray-800 dark:text-gray-300 font-medium capitalize">{contest.type || 'Standard'}</dd>
                            </div>
                            <div className="flex justify-between">
                                <dt className="text-gray-500">Problems</dt>
                                <dd className="text-gray-800 dark:text-gray-300 font-medium">{problems.length}</dd>
                            </div>
                        </dl>
                    </SidebarBox>

                    {/* Registration */}
                    {contest.registration_required && (
                        <SidebarBox title="Registration" icon="users">
                            <div className="space-y-3">
                                <div className="flex items-center justify-between text-sm">
                                    <span className="text-gray-500">Participants</span>
                                    <span className="font-semibold text-gray-800 dark:text-gray-300">
                                        {registrationCount}{contest.max_participants ? ` / ${contest.max_participants}` : ''}
                                    </span>
                                </div>
                                {contest.max_participants && (
                                    <div className="w-full bg-gray-100 dark:bg-gray-900/30 rounded-full h-1.5">
                                        <div
                                            className="bg-blue-50 dark:bg-blue-900/200 rounded-full h-1.5 transition-all"
                                            style={{ width: `${Math.min(100, (registrationCount / contest.max_participants) * 100)}%` }}
                                        />
                                    </div>
                                )}
                                {registered && (
                                    <div className="flex items-center gap-1.5 text-sm text-green-600 dark:text-green-400 font-medium">
                                        <CheckCircle2 className="w-4 h-4" />
                                        You are registered
                                    </div>
                                )}
                                {isUpcoming && getAccessToken() && (
                                    <button
                                        onClick={() => registered ? handleUnregister() : handleRegister()}
                                        className={`w-full py-2 rounded-lg text-sm font-semibold transition-colors ${
                                            registered
                                                ? 'bg-red-50 dark:bg-red-900/20 text-red-600 dark:text-red-400 hover:bg-red-100 border border-red-200 dark:border-red-700'
                                                : 'bg-blue-600 text-white hover:bg-blue-700'
                                        }`}
                                    >
                                        {registered ? 'Unregister' : 'Register Now'}
                                    </button>
                                )}
                            </div>
                        </SidebarBox>
                    )}

                    {/* Quick Links */}
                    {(isRunning || isEnded) && (
                        <SidebarBox title="Quick Links" icon="zap">
                            <div className="space-y-2">
                                <Link
                                    to={`/contests/${id}/scoreboard`}
                                    className="flex items-center gap-2 text-sm text-blue-600 dark:text-blue-400 hover:text-blue-800 hover:underline"
                                >
                                    <Trophy className="w-4 h-4" /> View Full Scoreboard
                                </Link>
                                {contest.pdf_enabled && (
                                    <a
                                        href={`/api/contests/${id}/pdf`}
                                        target="_blank"
                                        rel="noopener noreferrer"
                                        className="flex items-center gap-2 text-sm text-blue-600 dark:text-blue-400 hover:text-blue-800 hover:underline"
                                    >
                                        <FileDown className="w-4 h-4" /> Download PDF
                                    </a>
                                )}
                                {contest.upsolving_enabled && (
                                    <span className="flex items-center gap-2 text-sm text-gray-500">
                                        <Pencil className="w-4 h-4" /> Upsolving is enabled
                                    </span>
                                )}
                            </div>
                        </SidebarBox>
                    )}

                    {/* Judge Panel */}
                    {isAdmin && (
                        <SidebarBox title="Judge Panel" icon="settings" accent="purple">
                            <div className="space-y-2">
                                <Link
                                    to={`/setter/contest/${id}/manage`}
                                    className="flex items-center gap-2 w-full px-3 py-2 bg-purple-600 text-white text-sm font-semibold rounded-lg hover:bg-purple-700 transition-colors text-center justify-center"
                                >
                                    <Settings className="w-4 h-4" /> Manage Contest
                                </Link>
                                {isRunning && contest.statement_hidden && (
                                    <p className="text-xs text-yellow-700 dark:text-yellow-300 font-medium flex items-center gap-1">
                                        <AlertTriangle className="w-3 h-3" /> Statement hidden mode is ON
                                    </p>
                                )}
                            </div>
                        </SidebarBox>
                    )}

                    {/* Educational Round Info */}
                    {contest.type === 'educational' && (
                        <SidebarBox title="Educational Round" icon="book" accent="green">
                            <p className="text-sm text-gray-600 dark:text-gray-400">
                                This is an educational contest. Problems are designed for learning and practice.
                            </p>
                        </SidebarBox>
                    )}

                    {/* Virtual Contest */}
                    {contest.virtual_contest_enabled && (
                        <SidebarBox title="Virtual Contest" icon="game">
                            <p className="text-sm text-gray-600 dark:text-gray-400">
                                Virtual contests are available. Join anytime to practice with real contest timing.
                            </p>
                        </SidebarBox>
                    )}

                </div>
                )}
            </div>
        </div>
    )
}
