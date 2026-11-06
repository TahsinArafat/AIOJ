import React, { useEffect, useState, useCallback, useMemo } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'
import DivisionBadge from '../components/DivisionBadge'
import { ContestStatusBadge, ContestFormatBadge, ContestTypeBadge } from '../components/contest/ContestBadges'
import type { Division } from '../lib/divisions'
import { EmptyState } from '../components/EmptyState'
import {
    Trophy, FileText, MessageSquare, Users, Zap, FileDown, Pencil, Gamepad2,
    AlertTriangle, Megaphone, Settings, BookOpen, Info, Clock, Shield, CheckCircle2, X, ExternalLink,
    ChevronLeft, ChevronRight, Send, Filter, Eye, EyeOff
} from 'lucide-react'
import { useConfirm } from '../components/ConfirmDialog'
import { useToast } from '../components/Toast'

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
    const [now, setNow] = useState(() => Date.parse(start))

    useEffect(() => {
        const iv = setInterval(() => setNow(Date.now()), 1000)
        const seed = setTimeout(() => setNow(Date.now()), 0)
        return () => { clearInterval(iv); clearTimeout(seed) }
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
                    <span className="w-2 h-2 rounded-full bg-surface animate-pulse" />
                    <span className="text-sm font-medium opacity-90">Contest is running</span>
                </div>
                <span className="text-sm opacity-75">Elapsed: {formatDuration(elapsed)}</span>
            </div>
            <div className="font-mono text-3xl font-bold tracking-wider mb-3">
                {formatCountdown(remaining)} <span className="text-base font-normal opacity-70">remaining</span>
            </div>
            <div className="w-full bg-white/20 rounded-full h-2">
                <div className="bg-surface rounded-full h-2 transition-all duration-1000" style={{ width: `${progress}%` }} />
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
    const borderCls = accent === 'purple' ? 'border-accent/30' : accent === 'green' ? 'border-ok/30' : accent === 'amber' ? 'border-warn/30' : 'border-hairline'
    const headerCls = accent === 'purple' ? 'bg-violet-500/10' : accent === 'green' ? 'bg-ok/10' : accent === 'amber' ? 'bg-warn/10' : 'bg-surface-2/20'
    const iconNode = SIDEBAR_ICONS[icon]
    return (
        <div className={`bg-surface border ${borderCls} rounded-xl shadow-sm overflow-hidden`}>
            <div className={`px-4 py-2.5 ${headerCls} border-b ${borderCls}`}>
                <h3 className="text-xs font-bold text-muted uppercase tracking-wider flex items-center gap-1.5">
                    {iconNode}{title}
                </h3>
            </div>
            <div className="px-4 py-3">{children}</div>
        </div>
    )
}

// ─── Main Component ────────────────────────────────────────────────────────────

type Tab = 'problems' | 'standings' | 'submissions' | 'clarifications' | 'announcements'

interface ContestInfo {
    title: string
    type: string
    format: string
    division?: number
    start_time: string
    end_time: string
    freeze_time?: string | null
    max_participants?: number | null
    pdf_enabled?: boolean
    registration_required?: boolean
    statement_hidden?: boolean
    upsolving_enabled?: boolean
    virtual_contest_enabled?: boolean
}
interface ProblemRow { problem_id?: string; index?: string; title?: string; score?: number }
interface ClarificationItem { id?: string; created_at: string; question?: string; answer?: string | null; username?: string; is_public?: boolean }
interface AnnouncementItem { id: string; created_at: string; content?: string; username?: string }
interface SubmissionRow { id?: string; created_at: string; status: string; language?: string; problem_id?: string; user_id?: string; username?: string; time_used?: number; memory_used?: number; score?: number }
interface StandingsCell { solved?: boolean; attempts?: number; time?: number; pending?: number }
interface StandingsEntry { user_id?: string; username?: string; rank?: number; total_solved?: number; total_penalty?: number; problems?: Record<string, StandingsCell> }

export default function ContestDetail() {
    const confirm = useConfirm()
    const toast = useToast()
    const { id } = useParams<{ id: string }>()
    const role = decodeRole()
    const userId = decodeUserId()
    const isAdmin = role === 'admin' || role === 'owner'

    const [contest, setContest] = useState<ContestInfo | null>(null)
    const [problems, setProblems] = useState<ProblemRow[]>([])
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
    const [clarifications, setClarifications] = useState<ClarificationItem[]>([])
    const [showForm, setShowForm] = useState(false)
    const [question, setQuestion] = useState('')

    // Standings
    const [standings, setStandings] = useState<StandingsEntry[]>([])
    const [standingsProblems, setStandingsProblems] = useState<ProblemRow[]>([])
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
                if (c?.solved && (c.attempts ?? 0) > 0) {
                    if (!(pidx in bestTime) || (c.time ?? Infinity) < bestTime[pidx]) {
                        bestTime[pidx] = c.time ?? Infinity
                        map[pidx] = row.user_id ?? ''
                    }
                }
            }
        }
        return map
    }, [standings, standingsProblems, problems])
    // Announcements
    const [announcements, setAnnouncements] = useState<AnnouncementItem[]>([])
    const [announceText, setAnnounceText] = useState('')
    const [unreadAnnouncements, setUnreadAnnouncements] = useState(0)
    const [unreadClarifications, setUnreadClarifications] = useState(0)

    // Submissions
    const [submissions, setSubmissions] = useState<SubmissionRow[]>([])
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
            const c = res.contest
            setContest(c)
            setProblems(res.problems || [])
            setRegistered(c.is_registered ?? false)
            setRegistrationCount(c.registration_count ?? 0)
        }).finally(() => setLoading(false))
    }, [id])

    const fetchClarifications = useCallback(() => {
        if (!id) return
        const lastSeen = localStorage.getItem(`contest_${id}_last_clarif`) || '0'
        api.clarifications.list(id).then((res: { data?: ClarificationItem[] }) => {
            const items = res.data || []
            const newCount = items.filter(c => new Date(c.created_at).getTime() > Number(lastSeen)).length
            setUnreadClarifications(newCount)
            setClarifications(items)
        }).catch(() => { })
    }, [id])

    useEffect(() => {
        if (activeTab === 'clarifications') {
            fetchClarifications()
            if (id) localStorage.setItem(`contest_${id}_last_clarif`, String(Date.now()))
        }
        if (activeTab === 'announcements' && id) {
            localStorage.setItem(`contest_${id}_last_notice`, String(Date.now()))
        }
    }, [activeTab, fetchClarifications, id])

    useEffect(() => {
        if (activeTab === 'standings' && id) {
            api.contests.standings(id, standingsPage).then((res: { entries?: StandingsEntry[]; problems?: ProblemRow[]; frozen?: boolean; pagination?: { page: number; total_pages: number; total: number } }) => {
                setStandings(res.entries || [])
                setStandingsProblems(res.problems || [])
                setStandingsFrozen(res.frozen || false)
                setStandingsPagination(res.pagination || null)
            }).catch(() => { })
        }
    }, [activeTab, id, standingsPage])

    useEffect(() => {
        if (activeTab === 'submissions' && id && getAccessToken()) {
            const filters: { mine: boolean; problem_id?: string; language?: string; status?: string } = { mine: submissionsMine }
            if (subFilterProblem) filters.problem_id = subFilterProblem
            if (subFilterLang) filters.language = subFilterLang
            if (subFilterStatus) filters.status = subFilterStatus
            api.contests.submissions(id, submissionsOffset, submissionsLimit, filters).then((res: { data?: SubmissionRow[]; total?: number; is_judge?: boolean }) => {
                setSubmissions(res.data || [])
                setSubmissionsTotal(res.total || 0)
                setIsJudge(res.is_judge || false)
            }).catch(() => { })
        }
    }, [activeTab, id, submissionsOffset, submissionsMine, subFilterProblem, subFilterLang, subFilterStatus])

    const filteredSubmissions = useMemo(() => {
        let result = submissions
        if (subFilterId) {
            result = result.filter(s => s.id?.toLowerCase().includes(subFilterId.toLowerCase()))
        }
        if (contest?.freeze_time) {
            const freezeTime = new Date(contest.freeze_time).getTime()
            const now = Date.now() // eslint-disable-line react-hooks/purity -- freeze window intentionally compared against wall clock
            const endTime = new Date(contest.end_time).getTime()
            if (now >= freezeTime && now < endTime) {
                result = result.filter(s => new Date(s.created_at).getTime() < freezeTime)
            }
        }
        return result
    }, [submissions, subFilterId, contest])

    useEffect(() => {
        if (!id) return
        const pollNotices = () => {
            api.contests.announcements(id).then((res: AnnouncementItem[] | { data?: AnnouncementItem[] }) => {
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
                        toast.error(`New Announcement:\n\n${newest.content}`)
                    }
                }
                setAnnouncements(items)
                // Calculate unread from localStorage
                const lastSeen = localStorage.getItem(`contest_${id}_last_notice`) || '0'
                setUnreadAnnouncements(items.filter(a => new Date(a.created_at).getTime() > Number(lastSeen)).length)
            }).catch(() => { })
        }
        pollNotices()
        const interval = setInterval(pollNotices, 15000)
        return () => clearInterval(interval)
    }, [id]) // eslint-disable-line react-hooks/exhaustive-deps -- notification loop keeps its established stale-closure behavior

    const handleRegister = async () => {
        if (!id) return
        try {
            await api.contests.register(id)
            setRegistered(true)
            setRegistrationCount(c => c + 1)
        } catch (e) { toast.error(e instanceof Error ? e.message : String(e)) }
    }

    const handleUnregister = async () => {
        if (!id) return
        try {
            await api.contests.unregister(id)
            setRegistered(false)
            setRegistrationCount(c => Math.max(0, c - 1))
        } catch (e) { toast.error(e instanceof Error ? e.message : String(e)) }
    }

    const handleAsk = async () => {
        if (!id || !question.trim()) return
        try {
            await api.clarifications.create(id, { question: question.trim() })
            setQuestion('')
            setShowForm(false)
            fetchClarifications()
        } catch (e) { toast.error(e instanceof Error ? e.message : String(e)) }
    }

    const handleAnnounce = async () => {
        if (!id || !announceText.trim()) return
        try {
            await api.contests.postAnnouncement(id, announceText.trim())
            setAnnounceText('')
            api.contests.announcements(id).then((res: AnnouncementItem[] | { data?: AnnouncementItem[] }) => setAnnouncements(Array.isArray(res) ? res : res.data || [])).catch(() => { })
        } catch (e) { toast.error(e instanceof Error ? e.message : String(e)) }
    }

    const handleDeleteAnnouncement = async (announcementId: string) => {
        if (!id || !(await confirm({ message: 'Delete this announcement?', variant: 'danger' }))) return
        try {
            await api.contests.deleteAnnouncement(id, announcementId)
            setAnnouncements(a => a.filter(x => x.id !== announcementId))
        } catch (e) { toast.error(e instanceof Error ? e.message : String(e)) }
    }

    if (loading) return <div className="text-center py-20 text-muted">Loading...</div>
    if (!contest) return <div className="text-center py-20 text-muted">Contest not found</div>

    const status = getStatus(contest.start_time, contest.end_time, contest.freeze_time ?? undefined)
    const isUpcoming = status === 'upcoming'
    const isRunning = status === 'running' || status === 'frozen'
    const isEnded = status === 'ended'
    const totalDuration = new Date(contest.end_time).getTime() - new Date(contest.start_time).getTime()

    return (
        <div>
            {/* ── Hero Header ──────────────────────────────────────────────── */}
            <div className="mb-6">
                {/* flex-wrap: the title plus four badges is 526px of
                    min-content, which overflowed a 390px phone. */}
                <div className="flex flex-wrap items-center gap-3 mb-2">
                    <h1 className="text-3xl font-extrabold text-ink">{contest.title}</h1>
                    <ContestStatusBadge status={status} />
                    {contest.format && <ContestFormatBadge format={contest.format} />}
                    {contest.type && <ContestTypeBadge type={contest.type} />}
                    {contest.division !== undefined && <DivisionBadge division={contest.division as Division} />}
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
            {/* overflow-x-auto: five tabs (with unread badges) are 623px of
                min-content, so without a scroller they pushed the document to
                663px on a 390px phone. */}
            <div className="border-b border-hairline mb-6 overflow-x-auto">
                <nav className="flex gap-6 min-w-max">
                    {([
                        { key: 'problems', label: 'Problems', icon: 'problems' },
                        { key: 'standings', label: 'Standings', icon: 'standings' },
                        { key: 'submissions', label: 'Submissions', icon: 'submissions' },
                        { key: 'clarifications', label: 'Clarifications', icon: 'clarifications', unread: unreadClarifications },
                        { key: 'announcements', label: 'Announcements', icon: 'announcements', unread: unreadAnnouncements },
                    ] as { key: Tab; label: string; icon: string; unread?: number }[]).map(tab => (
                        <button
                            key={tab.key}
                            onClick={() => {
                                if (tab.key !== activeTab) {
                                    setActiveTab(tab.key)
                                    if (tab.key === 'clarifications') setUnreadClarifications(0)
                                    if (tab.key === 'announcements') setUnreadAnnouncements(0)
                                    if (tab.key !== 'standings') setStandingsPage(1)
                                }
                            }}
                            className={`pb-3 text-sm font-semibold transition-colors flex items-center gap-1.5 ${activeTab === tab.key
                                ? 'text-brand border-b-2 border-brand'
                                : 'text-muted hover:text-ink border-b-2 border-transparent'
                                }`}
                        >
                            {tab.icon === 'problems' && <FileText className="w-4 h-4" />}
                            {tab.icon === 'standings' && <Trophy className="w-4 h-4" />}
                            {tab.icon === 'submissions' && <Send className="w-4 h-4" />}
                            {tab.icon === 'clarifications' && <MessageSquare className="w-4 h-4" />}
                            {tab.icon === 'announcements' && <Megaphone className="w-4 h-4" />}
                            {tab.label}
                            {tab.unread !== undefined && tab.unread > 0 ? (
                                <span className="ml-1 min-w-[18px] rounded-full bg-danger-fill px-1.5 py-0.5 text-center text-xs font-bold text-danger-fill-ink">{tab.unread}</span>
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
                            <h2 className="text-xl font-bold text-ink mb-4">Problems</h2>
                            {problems.length === 0 ? (
                                <EmptyState icon={<FileText className="w-12 h-12 text-ink-soft" />} text="No problems added yet" description="Contest organizers haven't published problems for this contest yet." />
                            ) : (
                                <div className="bg-surface border border-hairline rounded-xl shadow-sm overflow-hidden">
                                    <table className="w-full text-sm">
                                        <thead>
                                            <tr className="bg-surface-2/20 border-b border-hairline">
                                                <th className="text-left px-4 py-3 font-semibold text-muted">#</th>
                                                <th className="text-left px-4 py-3 font-semibold text-muted">Problem</th>
                                                <th className="text-right px-4 py-3 font-semibold text-muted">Score</th>
                                                <th className="text-center px-4 py-3 font-semibold text-muted">Action</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            {problems.map((p, i) => (
                                                <tr key={p.problem_id} className="border-b border-hairline last:border-0 hover:bg-surface-2 transition-colors">
                                                    <td className="px-4 py-3 font-mono font-bold text-muted">{p.index || String.fromCharCode(65 + i)}</td>
                                                    <td className="px-4 py-3">
                                                        <Link
                                                            to={`/contests/${id}/problem/${p.index || String.fromCharCode(65 + i)}`}
                                                            className="text-brand hover:text-brand hover:underline font-medium"
                                                        >
                                                            {p.title || `Problem ${p.index || String.fromCharCode(65 + i)}`}
                                                        </Link>
                                                    </td>
                                                    <td className="px-4 py-3 text-right font-mono text-ink-soft">{p.score ?? 100}</td>
                                                    <td className="px-4 py-3 text-center">
                                                        <Link
                                                            to={`/contests/${id}/problem/${p.index || String.fromCharCode(65 + i)}`}
                                                            className="inline-block px-3 py-1 bg-brand text-on-brand text-xs font-semibold rounded-lg hover:bg-brand-hover transition-colors"
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
                                <h2 className="text-xl font-bold text-ink flex items-center gap-2">
                                    <Trophy className="w-5 h-5" /> Standings
                                </h2>
                                <div className="flex items-center gap-2">
                                    {standingsFrozen && (
                                        <span className="px-3 py-1 bg-brand-soft text-brand text-xs font-semibold rounded-full flex items-center gap-1 border border-brand/30">
                                            <Clock className="w-3 h-3" /> Frozen
                                        </span>
                                    )}
                                    <a
                                        href={`/contests/${id}/scoreboard`}
                                        target="_blank"
                                        rel="noopener noreferrer"
                                        className="inline-flex items-center gap-1.5 px-3 py-1 text-xs font-semibold text-muted hover:text-brand border border-hairline rounded-full hover:border-brand/40 dark:hover:border-brand/40 hover:bg-surface-2 transition-colors"
                                    >
                                        <ExternalLink className="w-3 h-3" />
                                        Full Scoreboard
                                    </a>
                                </div>
                            </div>
                            {standings.length === 0 ? (
                                <div className="text-center py-16 bg-surface border border-hairline rounded-xl">
                                    <EmptyState icon={<Trophy className="w-12 h-12 text-muted" />} text="No submissions yet" description="Standings will appear once participants start solving" />
                                </div>
                            ) : (
                                <div className="bg-surface border border-hairline rounded-xl shadow-md overflow-x-auto">
                                    <table className="w-full text-sm">
                                        <thead>
                                            <tr className="bg-surface-2 text-ink border-b border-hairline">
                                                <th className="text-center px-3 py-3 font-bold text-xs uppercase tracking-wider w-16">Rank</th>
                                                <th className="text-left px-4 py-3 font-bold text-xs uppercase tracking-wider min-w-[180px]">Team</th>
                                                <th className="text-center px-3 py-3 font-bold text-xs uppercase tracking-wider w-20">
                                                    <span className="text-ok dark:text-ok">=</span>
                                                </th>
                                                <th className="text-center px-3 py-3 font-bold text-xs uppercase tracking-wider w-20">Penalty</th>
                                                {(standingsProblems.length > 0 ? standingsProblems : problems).map((p, i) => (
                                                    <th key={i} className="text-center px-1 py-3 font-bold text-xs uppercase tracking-wider w-16">
                                                        <div className="flex flex-col items-center">
                                                            <span className="text-base">{String.fromCharCode(65 + i)}</span>
                                                            {p.title && <span className="text-[11px] text-muted font-normal normal-case truncate max-w-[60px]" title={p.title}>{p.title}</span>}
                                                        </div>
                                                    </th>
                                                ))}
                                            </tr>
                                        </thead>
                                        <tbody className="divide-y divide-hairline">
                                            {(standingsProblems.length > 0 ? standingsProblems : problems).length > 0 && standings.map((row, idx) => {
                                                const probList = standingsProblems.length > 0 ? standingsProblems : problems
                                                const isMe = row.user_id === userId
                                                const rank = row.rank || idx + 1
                                                const medal = rank === 1 ? 'gold' : rank === 2 ? 'silver' : rank === 3 ? 'bronze' : null
                                                const sec = row.total_penalty ?? 0
                                                const ph = Math.floor(sec / 3600)
                                                const pm = Math.floor((sec % 3600) / 60)
                                                const ps = sec % 60
                                                const penaltyStr = ph > 0 ? `${ph}:${String(pm).padStart(2, '0')}:${String(ps).padStart(2, '0')}` : `${pm}:${String(ps).padStart(2, '0')}`
                                                return (
                                                    <tr key={row.user_id || idx} className={`transition-colors ${isMe ? 'bg-brand-soft hover:bg-brand/15' : idx % 2 === 0 ? 'bg-surface hover:bg-surface-2' : 'bg-surface-2 hover:bg-surface-2'}`}>
                                                        <td className="text-center px-3 py-2.5">
                                                            {medal ? (
                                                                <span className={`inline-flex items-center justify-center w-8 h-8 rounded-full text-sm font-black border ${medal === 'gold' ? 'bg-warn/12 text-warn dark:text-warn border-warn/40' :
                                                                    medal === 'silver' ? 'bg-surface-3/50 text-ink-soft border-hairline-strong' :
                                                                        'bg-warn/12 text-warn border-warn/40'
                                                                    }`}>{rank}</span>
                                                            ) : (
                                                                <span className="text-sm font-bold text-muted">{rank}</span>
                                                            )}
                                                        </td>
                                                        <td className={`px-4 py-2.5 font-semibold ${isMe ? 'text-brand' : 'text-ink'}`}>
                                                            {row.username}
                                                            {isMe && <span className="ml-2 text-[11px] bg-brand-soft text-brand px-1.5 py-0.5 rounded font-bold border border-brand/30">YOU</span>}
                                                        </td>
                                                        <td className="text-center px-3 py-2.5">
                                                            <span className="text-lg font-black text-ink">{row.total_solved ?? 0}</span>
                                                        </td>
                                                        <td className="text-center px-3 py-2.5 font-mono text-muted text-xs">{penaltyStr}</td>
                                                        {probList.map((p, i) => {
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
                                                                        <div className={`inline-flex flex-col items-center justify-center rounded-md px-2 py-1 min-w-[48px] ${isFirst ? 'bg-ok-fill text-white shadow-sm' : 'bg-ok/12 text-ok dark:text-ok border border-ok/30'}`}>
                                                                            <span className="text-xs font-black leading-none">{attempts <= 1 ? '+' : `+${attempts - 1}`}</span>
                                                                            <span className="text-[10px] font-mono leading-none mt-0.5 opacity-70">{timeMin}m</span>
                                                                        </div>
                                                                    </td>
                                                                )
                                                            }
                                                            if (pending > 0) {
                                                                return (
                                                                    <td key={i} className="text-center px-1 py-2.5">
                                                                        <div className="inline-flex flex-col items-center justify-center bg-brand-soft text-brand dark:text-brand border border-brand/30 rounded-md px-2 py-1 min-w-[48px]">
                                                                            <span className="text-xs font-black leading-none">?</span>
                                                                            <span className="text-[10px] font-mono leading-none mt-0.5">{pending}</span>
                                                                        </div>
                                                                    </td>
                                                                )
                                                            }
                                                            if (attempts > 0) {
                                                                return (
                                                                    <td key={i} className="text-center px-1 py-2.5">
                                                                        <div className="inline-flex items-center justify-center bg-bad/12 dark:bg-bad/12 text-bad dark:text-bad border border-bad/30 dark:border-bad/40 rounded-md px-2 py-1 min-w-[48px]">
                                                                            <span className="text-xs font-black">-{attempts}</span>
                                                                        </div>
                                                                    </td>
                                                                )
                                                            }
                                                            return <td key={i} className="text-center px-1 py-2.5"><span className="text-muted font-medium">-</span></td>
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
                                    <span className="text-xs text-muted">
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
                                                    className="inline-flex items-center gap-1 px-2.5 py-1.5 text-sm font-medium rounded-md border border-hairline bg-surface-2 text-ink-soft hover:bg-surface-3 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
                                                >
                                                    <ChevronLeft className="w-4 h-4" />
                                                    Prev
                                                </button>
                                                {pages.map((p, i) =>
                                                    p === 'ellipsis' ? (
                                                        <span key={`e${i}`} className="px-2 py-1.5 text-sm text-muted">...</span>
                                                    ) : (
                                                        <button
                                                            key={p}
                                                            onClick={() => setStandingsPage(p)}
                                                            className={`min-w-[36px] px-2 py-1.5 text-sm font-medium rounded-md border transition-colors ${p === page
                                                                ? 'bg-brand text-on-brand border-brand shadow-sm'
                                                                : 'bg-surface-2 text-ink-soft border-hairline hover:bg-surface-3'
                                                                }`}
                                                        >
                                                            {p}
                                                        </button>
                                                    )
                                                )}
                                                <button
                                                    onClick={() => setStandingsPage(p => Math.min(total_pages, p + 1))}
                                                    disabled={page >= total_pages}
                                                    className="inline-flex items-center gap-1 px-2.5 py-1.5 text-sm font-medium rounded-md border border-hairline bg-surface-2 text-ink-soft hover:bg-surface-3 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
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
                                <h2 className="text-xl font-bold text-ink flex items-center gap-2">
                                    <Send className="w-5 h-5" /> Submissions
                                </h2>
                                {getAccessToken() && (
                                    <div className="flex items-center bg-surface-3/30 rounded-lg p-0.5">
                                        <button
                                            onClick={() => { setSubmissionsMine(true); setSubmissionsOffset(0) }}
                                            className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded-md transition-colors ${submissionsMine ? 'bg-surface text-ink shadow-sm' : 'text-muted hover:text-ink'}`}
                                        >
                                            <Eye className="w-3.5 h-3.5" /> My Submissions
                                        </button>
                                        <button
                                            onClick={() => { setSubmissionsMine(false); setSubmissionsOffset(0) }}
                                            className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded-md transition-colors ${!submissionsMine ? 'bg-surface text-ink shadow-sm' : 'text-muted hover:text-ink'}`}
                                        >
                                            <EyeOff className="w-3.5 h-3.5" /> All Submissions
                                        </button>
                                    </div>
                                )}
                            </div>

                            {status === 'frozen' && (
                                <div className="mb-4 p-3 bg-brand-soft border border-brand/30 rounded-lg flex items-center gap-2">
                                    <Clock className="w-4 h-4 text-brand" />
                                    <span className="text-sm text-brand">Standings frozen — only submissions before freeze are shown.</span>
                                </div>
                            )}

                            {/* Filter controls */}
                            {getAccessToken() && (
                                <div className="flex flex-wrap items-center gap-3 mb-4 p-3 bg-surface-2/20 border border-hairline rounded-lg">
                                    <Filter className="w-4 h-4 text-muted" />
                                    <select
                                        value={subFilterProblem}
                                        onChange={e => { setSubFilterProblem(e.target.value); setSubmissionsOffset(0) }}
                                        className="text-sm border border-hairline rounded-md px-2.5 py-1.5 bg-surface text-ink-soft focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-transparent"
                                    >
                                        <option value="">All Problems</option>
                                        {problems.map((p) => (
                                            <option key={p.problem_id} value={p.problem_id}>{p.index ? `${p.index} - ` : ''}{p.title}</option>
                                        ))}
                                    </select>
                                    <select
                                        value={subFilterLang}
                                        onChange={e => { setSubFilterLang(e.target.value); setSubmissionsOffset(0) }}
                                        className="text-sm border border-hairline rounded-md px-2.5 py-1.5 bg-surface text-ink-soft focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-transparent"
                                    >
                                        <option value="">All Languages</option>
                                        {Array.from(new Set(submissions.map(s => s.language).filter(Boolean))).sort().map(lang => (
                                            <option key={lang} value={lang}>{lang}</option>
                                        ))}
                                    </select>
                                    <select
                                        value={subFilterStatus}
                                        onChange={e => { setSubFilterStatus(e.target.value); setSubmissionsOffset(0) }}
                                        className="text-sm border border-hairline rounded-md px-2.5 py-1.5 bg-surface text-ink-soft focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-transparent"
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
                                        className="text-sm border border-hairline rounded-md px-2.5 py-1.5 bg-surface text-ink-soft focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-transparent w-44"
                                    />
                                    {(subFilterProblem || subFilterLang || subFilterStatus || subFilterId) && (
                                        <button
                                            onClick={() => { setSubFilterProblem(''); setSubFilterLang(''); setSubFilterStatus(''); setSubFilterId(''); setSubmissionsOffset(0) }}
                                            className="text-xs text-muted hover:text-bad font-medium"
                                        >
                                            Clear Filters
                                        </button>
                                    )}
                                </div>
                            )}

                            {!getAccessToken() ? (
                                <EmptyState icon={<Send className="w-12 h-12 text-ink-soft" />} text="Please log in to view submissions" description="Sign in to follow this contest's submissions in real time." />
                            ) : filteredSubmissions.length === 0 ? (
                                <EmptyState
                                    icon={<Send className="w-12 h-12 text-ink-soft" />}
                                    text="No submissions found"
                                    description={(subFilterProblem || subFilterLang || subFilterStatus || subFilterId) ? 'Try adjusting your filters' : undefined}
                                />
                            ) : (
                                <>
                                    <div className="bg-surface border border-hairline rounded-xl shadow-sm overflow-x-auto">
                                        <table className="w-full text-sm">
                                            <thead>
                                                <tr className="bg-surface-2/20 border-b border-hairline">
                                                    <th className="text-left px-4 py-3 font-semibold text-muted">#</th>
                                                    {(!submissionsMine || isJudge) && <th className="text-left px-4 py-3 font-semibold text-muted">User</th>}
                                                    <th className="text-left px-4 py-3 font-semibold text-muted">Problem</th>
                                                    <th className="text-center px-4 py-3 font-semibold text-muted">Lang</th>
                                                    <th className="text-center px-4 py-3 font-semibold text-muted">Verdict</th>
                                                    <th className="text-center px-4 py-3 font-semibold text-muted">Score</th>
                                                    <th className="text-center px-4 py-3 font-semibold text-muted">Time</th>
                                                    <th className="text-center px-4 py-3 font-semibold text-muted">Memory</th>
                                                    <th className="text-right px-4 py-3 font-semibold text-muted">Submitted</th>
                                                </tr>
                                            </thead>
                                            <tbody>
                                                {filteredSubmissions.map((s, i) => {
                                                    const verdictCls: Record<string, string> = {
                                                        ac: 'bg-ok/12 text-ok border-ok/40',
                                                        wa: 'bg-bad/12 text-bad border-bad/40',
                                                        tle: 'bg-warn/12 text-warn border-warn/40',
                                                        mle: 'bg-warn/12 text-warn border-warn/40',
                                                        re: 'bg-warn/12 text-warn border-warn/40',
                                                        ce: 'bg-surface-3/30 text-muted border-hairline-strong',
                                                        pending: 'bg-brand-soft text-brand border-brand/40',
                                                        judging: 'bg-brand-soft text-brand border-brand/40',
                                                        se: 'bg-bad/12 text-bad border-bad/40',
                                                        ie: 'bg-bad/12 text-bad border-bad/40',
                                                    }
                                                    const verdictLabel: Record<string, string> = {
                                                        ac: 'Accepted', wa: 'Wrong Answer', tle: 'Time Limit',
                                                        mle: 'Memory Limit', re: 'Runtime Error', ce: 'Compile Error',
                                                        pending: 'Pending', judging: 'Judging', se: 'Internal Error', ie: 'Internal Error',
                                                    }
                                                    const problem = problems.find(p => p.problem_id === s.problem_id)
                                                    const problemLabel = problem ? `${problem.index || ''} — ${problem.title}` : s.problem_id?.slice(0, 8)
                                                    const canClick = s.user_id === userId || isJudge
                                                    const rowCls = canClick ? 'cursor-pointer hover:bg-brand-soft' : 'hover:bg-surface-2'
                                                    return (
                                                        <tr
                                                            key={s.id}
                                                            className={`border-b border-hairline last:border-0 ${rowCls} transition-colors`}
                                                            onClick={canClick ? () => window.open(`/submissions/${s.id}`, '_blank') : undefined}
                                                        >
                                                            <td className="px-4 py-2.5 font-mono text-xs text-muted">{submissionsOffset + i + 1}</td>
                                                            {(!submissionsMine || isJudge) && <td className="px-4 py-2.5 font-medium text-ink">{s.username || s.user_id?.slice(0, 8)}</td>}
                                                            <td className="px-4 py-2.5 text-ink-soft">{problemLabel}</td>
                                                            <td className="px-4 py-2.5 text-center">
                                                                <span className="px-2 py-0.5 bg-surface-3/30 text-muted text-xs font-mono rounded">{s.language}</span>
                                                            </td>
                                                            <td className="px-4 py-2.5 text-center">
                                                                <span className={`px-2 py-0.5 text-xs font-semibold rounded border ${verdictCls[s.status] || 'bg-surface-3/30 text-muted border-hairline-strong'}`}>
                                                                    {verdictLabel[s.status] || s.status}
                                                                </span>
                                                            </td>
                                                            <td className="px-4 py-2.5 text-center font-mono text-ink-soft">{s.score ?? '-'}</td>
                                                            <td className="px-4 py-2.5 text-center font-mono text-muted text-xs">{s.time_used ? `${s.time_used}ms` : '-'}</td>
                                                            <td className="px-4 py-2.5 text-center font-mono text-muted text-xs">{s.memory_used ? `${(s.memory_used / 1024).toFixed(1)}MB` : '-'}</td>
                                                            <td className="px-4 py-2.5 text-right text-xs text-muted" title={new Date(s.created_at).toLocaleString()}>{timeAgo(s.created_at)}</td>
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
                                                className="px-3 py-1.5 text-sm font-medium rounded-md border border-hairline bg-surface text-muted hover:bg-surface-2 disabled:opacity-40 disabled:cursor-not-allowed"
                                            >Prev</button>
                                            <span className="text-sm text-muted">
                                                {Math.floor(submissionsOffset / submissionsLimit) + 1} / {Math.ceil(submissionsTotal / submissionsLimit)}
                                            </span>
                                            <button
                                                onClick={() => setSubmissionsOffset(o => o + submissionsLimit)}
                                                disabled={submissionsOffset + submissionsLimit >= submissionsTotal}
                                                className="px-3 py-1.5 text-sm font-medium rounded-md border border-hairline bg-surface text-muted hover:bg-surface-2 disabled:opacity-40 disabled:cursor-not-allowed"
                                            >Next</button>
                                        </div>
                                    )}
                                    <div className="mt-2 text-xs text-muted text-center">{submissionsTotal} submission{submissionsTotal !== 1 ? 's' : ''}</div>
                                </>
                            )}
                        </div>
                    )}

                    {activeTab === 'clarifications' && (
                        <div>
                            <div className="flex items-center justify-between mb-4">
                                <h2 className="text-xl font-bold text-ink">Clarifications</h2>
                                {getAccessToken() && (
                                    <button
                                        onClick={() => setShowForm(!showForm)}
                                        className="px-4 py-2 bg-brand text-on-brand text-sm font-semibold rounded-lg hover:bg-brand-hover transition-colors"
                                    >
                                        Ask Question
                                    </button>
                                )}
                            </div>

                            {showForm && (
                                <div className="bg-surface border border-brand/30 rounded-xl p-4 mb-4 shadow-sm">
                                    <textarea
                                        value={question}
                                        onChange={e => setQuestion(e.target.value)}
                                        placeholder="Type your question..."
                                        className="w-full border border-hairline rounded-lg p-3 text-sm resize-none focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-transparent"
                                        rows={3}
                                    />
                                    <div className="flex gap-2 mt-3">
                                        <button onClick={handleAsk} className="px-4 py-2 bg-brand text-on-brand text-sm font-semibold rounded-lg hover:bg-brand-hover">Submit</button>
                                        <button onClick={() => setShowForm(false)} className="px-4 py-2 bg-surface-3/30 text-muted text-sm font-semibold rounded-lg hover:bg-surface-3">Cancel</button>
                                    </div>
                                </div>
                            )}

                            {clarifications.length === 0 ? (
                                <EmptyState icon={<MessageSquare className="w-12 h-12 text-ink-soft" />} text="No clarifications yet" description="Questions from participants and official responses will appear here." />
                            ) : (
                                <div className="space-y-3">
                                    {clarifications.map(c => (
                                        <div key={c.id} className="bg-surface border border-hairline rounded-xl p-4 shadow-sm">
                                            <div className="flex items-start justify-between mb-2">
                                                <span className="text-xs font-medium text-muted">{c.username || 'Anonymous'}</span>
                                                <span className="text-xs text-muted">{new Date(c.created_at).toLocaleString()}</span>
                                            </div>
                                            <p className="text-sm text-ink mb-2">{c.question}</p>
                                            {c.answer && (
                                                <div className={`mt-2 pl-3 border-l-2 ${c.is_public ? 'border-ok/40 bg-ok/10' : 'border-brand/40 bg-brand-soft'} rounded-r-lg p-3`}>
                                                    <p className="text-sm text-ink-soft">{c.answer}</p>
                                                    {c.is_public && (
                                                        <span className="inline-block text-[10px] text-ok font-medium mt-1">
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
                            <h2 className="text-xl font-bold text-ink mb-4 flex items-center gap-2">
                                <Megaphone className="w-5 h-5" /> Announcements
                            </h2>

                            {/* Admin: Post new announcement */}
                            {isAdmin && (
                                <div className="mb-6 bg-warn/10 border border-warn/30 rounded-lg p-4">
                                    <h3 className="text-sm font-semibold text-warn mb-2">Post Announcement</h3>
                                    <div className="flex gap-2">
                                        <input
                                            type="text"
                                            value={announceText}
                                            onChange={e => setAnnounceText(e.target.value)}
                                            onKeyDown={e => e.key === 'Enter' && handleAnnounce()}
                                            placeholder="Write announcement for all participants..."
                                            className="flex-1 border border-hairline rounded-lg px-3 py-2 text-sm focus:ring-2 focus:ring-amber-500 focus:border-transparent"
                                        />
                                        <button
                                            onClick={handleAnnounce}
                                            className="px-4 py-2 bg-warn/10 dark:bg-warn text-white text-sm font-semibold rounded-lg hover:bg-warn-fill transition-colors"
                                        >
                                            Post
                                        </button>
                                    </div>
                                </div>
                            )}

                            {announcements.length === 0 ? (
                                <EmptyState icon={<Megaphone className="w-12 h-12 text-ink-soft" />} text="No announcements yet" description="Announcements from judges will appear here" />
                            ) : (
                                <div className="space-y-3">
                                    {announcements.map((a, i) => (
                                        <div key={a.id} className="bg-warn/10 border border-warn/30 rounded-lg p-4">
                                            <div className="flex items-start justify-between gap-3">
                                                <div className="flex-1">
                                                    <p className="text-ink text-sm leading-relaxed">{a.content}</p>
                                                    <div className="mt-2 flex items-center gap-2 text-xs text-muted">
                                                        {a.username && <span className="font-medium text-muted">{a.username}</span>}
                                                        <span>{new Date(a.created_at).toLocaleString()}</span>
                                                    </div>
                                                </div>
                                                <div className="flex items-center gap-2 flex-shrink-0">
                                                    <span className="text-xs text-warn font-bold bg-warn/12 px-2 py-0.5 rounded">#{i + 1}</span>
                                                    {isAdmin && (
                                                        <button
                                                            onClick={() => handleDeleteAnnouncement(a.id)}
                                                            className="text-bad hover:text-bad p-1"
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
                                    <dt className="text-muted">Start</dt>
                                    <dd className="text-ink font-medium text-right">{formatDateTime(contest.start_time)}</dd>
                                </div>
                                <div className="flex justify-between">
                                    <dt className="text-muted">End</dt>
                                    <dd className="text-ink font-medium text-right">{formatDateTime(contest.end_time)}</dd>
                                </div>
                                <div className="flex justify-between">
                                    <dt className="text-muted">Duration</dt>
                                    <dd className="text-ink font-medium">{formatDuration(totalDuration)}</dd>
                                </div>
                                {contest.freeze_time && (
                                    <div className="flex justify-between">
                                        <dt className="text-muted">Standings freeze</dt>
                                        <dd className="text-ink font-medium text-right">{formatDateTime(contest.freeze_time)}</dd>
                                    </div>
                                )}
                                <div className="border-t border-hairline pt-2 flex justify-between">
                                    <dt className="text-muted">Format</dt>
                                    <dd className="text-ink font-medium uppercase">{contest.format || 'N/A'}</dd>
                                </div>
                                <div className="flex justify-between">
                                    <dt className="text-muted">Type</dt>
                                    <dd className="text-ink font-medium capitalize">{contest.type || 'Standard'}</dd>
                                </div>
                                <div className="flex justify-between">
                                    <dt className="text-muted">Problems</dt>
                                    <dd className="text-ink font-medium">{problems.length}</dd>
                                </div>
                            </dl>
                        </SidebarBox>

                        {/* Registration */}
                        {contest.registration_required && (
                            <SidebarBox title="Registration" icon="users">
                                <div className="space-y-3">
                                    <div className="flex items-center justify-between text-sm">
                                        <span className="text-muted">Participants</span>
                                        <span className="font-semibold text-ink">
                                            {registrationCount}{contest.max_participants ? ` / ${contest.max_participants}` : ''}
                                        </span>
                                    </div>
                                    {contest.max_participants && (
                                        <div className="w-full bg-surface-3/30 rounded-full h-1.5">
                                            <div
                                                className="bg-brand rounded-full h-1.5 transition-all"
                                                style={{ width: `${Math.min(100, (registrationCount / contest.max_participants) * 100)}%` }}
                                            />
                                        </div>
                                    )}
                                    {registered && (
                                        <div className="flex items-center gap-1.5 text-sm text-ok font-medium">
                                            <CheckCircle2 className="w-4 h-4" />
                                            You are registered
                                        </div>
                                    )}
                                    {isUpcoming && getAccessToken() && (
                                        <button
                                            onClick={() => registered ? handleUnregister() : handleRegister()}
                                            className={`w-full py-2 rounded-lg text-sm font-semibold transition-colors ${registered
                                                ? 'bg-bad/10 text-bad hover:bg-bad/12 border border-bad/30'
                                                : 'bg-brand text-on-brand hover:bg-brand-hover'
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
                                        className="flex items-center gap-2 text-sm text-brand hover:text-brand hover:underline"
                                    >
                                        <Trophy className="w-4 h-4" /> View Full Scoreboard
                                    </Link>
                                    {contest.pdf_enabled && (
                                        <a
                                            href={`/api/contests/${id}/pdf`}
                                            target="_blank"
                                            rel="noopener noreferrer"
                                            className="flex items-center gap-2 text-sm text-brand hover:text-brand hover:underline"
                                        >
                                            <FileDown className="w-4 h-4" /> Download PDF
                                        </a>
                                    )}
                                    {contest.upsolving_enabled && (
                                        <span className="flex items-center gap-2 text-sm text-muted">
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
                                        <p className="text-xs text-warn font-medium flex items-center gap-1">
                                            <AlertTriangle className="w-3 h-3" /> Statement hidden mode is ON
                                        </p>
                                    )}
                                </div>
                            </SidebarBox>
                        )}

                        {/* Educational Round Info */}
                        {contest.type === 'educational' && (
                            <SidebarBox title="Educational Round" icon="book" accent="green">
                                <p className="text-sm text-muted">
                                    This is an educational contest. Problems are designed for learning and practice.
                                </p>
                            </SidebarBox>
                        )}

                        {/* Virtual Contest */}
                        {contest.virtual_contest_enabled && (
                            <SidebarBox title="Virtual Contest" icon="game">
                                <p className="text-sm text-muted">
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
