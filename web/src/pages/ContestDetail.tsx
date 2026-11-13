import React, { useEffect, useState, useMemo } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api, getAccessToken, currentUser } from '../lib/api'
import { errorMessage } from '../lib/errors'
import { useDocumentTitle } from '../lib/documentTitle'
import DivisionBadge from '../components/DivisionBadge'
import { ContestStatusBadge, ContestFormatBadge, ContestTypeBadge } from '../components/contest/ContestBadges'
import type { Division } from '../lib/divisions'
import { EmptyState } from '../components/EmptyState'
import {
    Trophy, FileText, MessageSquare, Users, Zap, FileDown, Pencil, Gamepad2,
    AlertTriangle, Megaphone, Settings, BookOpen, Info, Clock, Shield, CheckCircle2, X, ExternalLink,
    Send, Filter, Eye, EyeOff, Lock
} from 'lucide-react'
import { useConfirm } from '../components/ConfirmDialog'
import { useToast } from '../components/Toast'
import { ContestCountdown, ContestRunningClock } from '../components/ContestClock'
import { formatDuration } from '../lib/contestTime'
import { problemLetter } from '../lib/contestProblems'
import { useContestAnnouncements } from '../hooks/useContestAnnouncements'
import { useClarifications } from '../hooks/useClarifications'
import { ClarificationsPanel, AskClarificationForm } from '../components/ClarificationsPanel'
import { ContestAnnouncementPopup } from '../components/ContestAnnouncementPopup'

function decodeRole(): string | null {
    return currentUser()?.role ?? null
}

// The JWT claim is `uid`, not `user_id`. Reading the wrong key meant this always
// returned null, so no standings row or submission row ever matched the viewer
// and neither was highlighted or clickable.
function decodeUserId(): string | null {
    return currentUser()?.id ?? null
}

// ─── Countdown / Timer Utilities ───────────────────────────────────────────────

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

// The standings tab is gone: the scoreboard has its own page
// (/contests/{id}/scoreboard), and this page's copy of it was a second, worse
// rendering of the same table. The button in the tab strip opens that page.
type Tab = 'about' | 'problems' | 'submissions' | 'clarifications' | 'announcements'

/** One entry on the round's About tab: who runs it, by username. */
interface JudgeRow { user_id?: string; username?: string; access_level?: string }

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
    description?: string
    /** Present when the caller may read the round as content; absent otherwise. */
    judges?: JudgeRow[]
}
interface ProblemRow {
    problem_id?: string; index?: string; title?: string; score?: number;
    /** Round-wide solve count, always present. */
    solved_count?: number;
    /** Per-viewer progress; absent when signed out. */
    my_solved?: boolean; my_attempted?: boolean; my_submissions?: number;
}
interface SubmissionRow { id?: string; created_at: string; status: string; language?: string; problem_id?: string; user_id?: string; username?: string; time_used?: number; memory_used?: number; score?: number }
export default function ContestDetail() {
    const confirm = useConfirm()
    const toast = useToast()
    const { id } = useParams<{ id: string }>()
    const role = decodeRole()
    const userId = decodeUserId()
    const isAdmin = role === 'admin' || role === 'owner'

    const [contest, setContest] = useState<ContestInfo | null>(null)
    // Why the contest failed to load, when it did, tagged with the id it belongs
    // to. Kept separate from `contest` because "we could not ask" and "it is not
    // there" are different messages and used to be the same one; tagged because
    // this component is reused when the route id changes, and an untagged message
    // would follow the viewer from one contest to the next.
    // code is kept because "password_required" needs a prompt, not an apology: a
    // contestant who simply has not unlocked the round must be able to type the
    // password, and everything else stays a plain error.
    const [loadError, setLoadError] = useState<{ id: string; message: string; code?: string; scoreboardPublic?: boolean } | null>(null)
    const [unlockPassword, setUnlockPassword] = useState('')
    const [unlocking, setUnlocking] = useState(false)
    const [unlockError, setUnlockError] = useState('')
    // reloadKey lets the unlock flow re-run the fetch effect below without
    // duplicating its body.
    const [reloadKey, setReloadKey] = useState(0)
    // The route map only knows it is a contest; the real title arrives with
    // the fetch and replaces "Contest" in the tab.
    useDocumentTitle(contest?.title)
    const [problems, setProblems] = useState<ProblemRow[]>([])
    const [loading, setLoading] = useState(true)
    const [activeTab, setActiveTab] = useState<Tab>(() => {
        const hash = window.location.hash.replace('#', '') as Tab
        // '#standings' is deliberately absent. The hash used to name a tab
        // here, so an old bookmark produces an unknown value; it falls through
        // to Problems, which is the sensible landing place.
        const validTabs: Tab[] = ['about', 'problems', 'submissions', 'clarifications', 'announcements']
        return validTabs.includes(hash) ? hash : 'problems'
    })

    // Sync URL hash when tab changes
    useEffect(() => {
        window.location.hash = activeTab
    }, [activeTab])

    // Registration
    const [registered, setRegistered] = useState(false)
    const [registrationCount, setRegistrationCount] = useState(0)

    // Clarifications. The queue is polled (see the hook) so a jury reply lands
    // on the list without a reload, and the contestant's own question appears
    // when they ask rather than when the tab next loads.
    const clarificationsApi = useClarifications(id, {}, {
        enabled: Boolean(id) && activeTab === 'clarifications',
    })
    // The problem picker for a question, in index order (the order the round is
    // read in), which is not the order the API returns them.
    const clarificationProblems = useMemo(() => [...problems]
        .filter(p => p.problem_id && p.index)
        .sort((a, b) => String(a.index).localeCompare(String(b.index)))
        .map(p => ({ problem_id: p.problem_id as string, index: String(p.index), title: p.title || '' })), [problems])

    // Announcements. Polling and the new-announcement popup live in one hook so
    // every contest page shares the same behaviour — a contestant on a problem
    // statement used to be the one place an announcement could not reach them.
    // `enabled` keeps this page's list authoritative while it is mounted.
    const announcementsApi = useContestAnnouncements(id, {
        enabled: Boolean(id),
        pollMs: 15_000,
    })
    const [announceText, setAnnounceText] = useState('')
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
    // Whether this contest considers the viewer its staff (admin, creator,
    // manager, judge or tester). The page cannot derive this from the JWT: a
    // judge of one round is an ordinary user everywhere else, so the answer has
    // to come from the contest itself. Without it the management entry point was
    // gated on the admin role alone and did not appear for the people running
    // the round.
    const [isStaff, setIsStaff] = useState(false)

    useEffect(() => {
        if (!id) return
        // The rejection is captured rather than swallowed. This fetch used to have
        // no .catch at all, so a 403 (a private round), a 500 and a genuinely
        // missing contest all rendered the same "Contest not found" — which is how
        // an upcoming contest, refused by the server, came to look nonexistent.
        api.contests.get(id).then(res => {
            const c = res.contest
            setContest(c)
            setProblems(res.problems || [])
            setRegistered(c.is_registered ?? false)
            setRegistrationCount(c.registration_count ?? 0)
            setIsStaff(res.is_staff ?? false)
        }).catch((e: unknown) => {
            // setState from a promise callback, not the effect body: this is the
            // async result of an external request, which is what an effect is for.
            const code = (e as { code?: string } | null)?.code
            // A round may publish its board and keep its problems locked. That
            // round is unlisted, so this screen — the one a shared link lands on —
            // is the only place a spectator can be told the board is watchable.
            const scoreboardPublic = (e as { scoreboardPublic?: boolean } | null)?.scoreboardPublic
            setLoadError({ id, message: errorMessage(e), code, scoreboardPublic })
        }).finally(() => setLoading(false))
    }, [id, reloadKey])

    const submitUnlock = async () => {
        if (!id || !unlockPassword) return
        setUnlocking(true)
        setUnlockError('')
        try {
            await api.contests.unlock(id, unlockPassword)
            setUnlockPassword('')
            // Re-fetch: the round is now readable, so the page loads normally.
            setLoading(true)
            setReloadKey(k => k + 1)
        } catch (e) {
            setUnlockError(errorMessage(e) || 'Incorrect password')
        } finally {
            setUnlocking(false)
        }
    }

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

    // Opening the announcements tab is the same as dismissing the popup: they
    // have been seen. This also clears the unread badge on the tab.
    const markAnnouncementsRead = announcementsApi.markAllRead
    useEffect(() => {
        if (activeTab === 'announcements') markAnnouncementsRead()
    }, [activeTab, markAnnouncementsRead])

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

    const handleAsk = async (problemId: string | undefined, text: string) => {
        if (!id) return
        // Rethrow so the form can show its own inline error instead of the page
        // losing the text the person just typed.
        try {
            await api.clarifications.create(id, { problem_id: problemId, question: text })
            await clarificationsApi.refresh()
        } catch (e) {
            toast.error(e instanceof Error ? e.message : String(e))
            throw e
        }
    }

    const handleAnswer = async (clarificationId: string, answer: string, isPublic: boolean) => {
        if (!id) return
        try {
            const updated = await api.clarifications.answer(id, clarificationId, { answer, is_public: isPublic })
            clarificationsApi.applyOne(updated)
        } catch (e) { toast.error(e instanceof Error ? e.message : String(e)) }
    }

    const handleAnswerMany = async (ids: string[], answer: string, isPublic: boolean) => {
        if (!id) return
        try {
            const { answered } = await api.clarifications.answerMany(id, { ids, answer, is_public: isPublic })
            clarificationsApi.applyMany(ids)
            clarificationsApi.clearNew()
            toast.success(`Answered ${answered} question${answered === 1 ? '' : 's'}`)
        } catch (e) { toast.error(e instanceof Error ? e.message : String(e)) }
    }

    const handleAnnounce = async () => {
        if (!id || !announceText.trim()) return
        try {
            await api.contests.postAnnouncement(id, announceText.trim())
            setAnnounceText('')
            // Re-poll rather than pushing it into local state: the post is the
            // authoritative record, and the list the popup reads from has to
            // match it. The organizer's own announcement is not shown as a
            // popup to them — that would be noise on the person who just typed
            // it — so it is marked seen immediately.
            await announcementsApi.refresh()
            announcementsApi.markAllRead()
        } catch (e) { toast.error(e instanceof Error ? e.message : String(e)) }
    }

    const handleDeleteAnnouncement = async (announcementId: string) => {
        if (!id || !(await confirm({ message: 'Delete this announcement?', variant: 'danger' }))) return
        try {
            await api.contests.deleteAnnouncement(id, announcementId)
            announcementsApi.markAllRead()
            await announcementsApi.refresh()
        } catch (e) { toast.error(e instanceof Error ? e.message : String(e)) }
    }

    if (loading) return <div className="text-center py-20 text-muted">Loading...</div>
    // A failure now carries its own reason, so a locked, private or briefly
    // unreachable round no longer reads as "this contest does not exist".
    if (!contest) {
        // Only this contest's own failure explains its absence; a message left
        // over from a previously viewed round would be about a different contest.
        const reason = loadError && loadError.id === id ? loadError.message : ''
        const locked = Boolean(loadError && loadError.id === id && loadError.code === 'password_required')
        const scoreboardPublic = Boolean(loadError && loadError.id === id && loadError.scoreboardPublic)

        if (locked) {
            return (
                <div className="max-w-md mx-auto py-20">
                    <div className="text-center mb-6">
                        <Lock className="w-12 h-12 text-ink-soft mx-auto mb-3" />
                        <h1 className="text-xl font-bold text-ink">This contest is private</h1>
                        <p className="text-sm text-muted-soft mt-2">
                            Enter the contest password to open the problems and submit.
                        </p>
                    </div>
                    <form
                        onSubmit={e => { e.preventDefault(); void submitUnlock() }}
                        className="space-y-3"
                    >
                        <input
                            type="password"
                            aria-label="Contest password"
                            autoFocus
                            value={unlockPassword}
                            onChange={e => setUnlockPassword(e.target.value)}
                            placeholder="Contest password"
                            className="w-full border border-hairline-strong rounded-lg px-3 py-2 text-sm focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-brand"
                        />
                        {unlockError && <p className="text-sm text-danger">{unlockError}</p>}
                        <button
                            type="submit"
                            disabled={unlocking || !unlockPassword}
                            className="w-full bg-brand text-on-brand rounded-lg px-4 py-2 text-sm font-medium disabled:opacity-50"
                        >
                            {unlocking ? 'Unlocking...' : 'Unlock contest'}
                        </button>
                    </form>
                    {/* A published board is unlisted, so this is the only route a
                        spectator has. Only rendered when the server says the round
                        actually has one, so the link cannot dangle. */}
                    {scoreboardPublic && (
                        <p className="text-center text-sm text-muted-soft mt-4">
                            Can't unlock it?{' '}
                            <Link to={`/contests/${id}/scoreboard`} className="text-brand hover:underline">
                                Watch the scoreboard
                            </Link>
                        </p>
                    )}
                </div>
            )
        }

        return (
            <EmptyState
                icon={<AlertTriangle className="w-12 h-12 text-ink-soft" />}
                text="Contest not found"
                description={reason || 'This contest does not exist, or you do not have access to it.'}
            />
        )
    }

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

            {/* ── Clock ────────────────────────────────────────────────────── */}
            {isUpcoming && (
                <div className="mb-6">
                    <ContestCountdown target={contest.start_time} label="Starts in" />
                </div>
            )}
            {isRunning && (
                <div className="mb-6">
                    <ContestRunningClock start={contest.start_time} end={contest.end_time} />
                </div>
            )}

            {/* ── Tab Navigation ───────────────────────────────────────────── */}
            {/* overflow-x-auto: the tabs (with unread badges) are 623px of
                min-content, so without a scroller they pushed the document to
                663px on a 390px phone. */}
            <div className="border-b border-hairline mb-6 overflow-x-auto">
                <nav className="flex gap-6 min-w-max">
                    {([
                        { key: 'about', label: 'About', icon: 'about' },
                        { key: 'problems', label: 'Problems', icon: 'problems' },
                        { key: 'submissions', label: 'Submissions', icon: 'submissions' },
                        { key: 'clarifications', label: 'Clarifications', icon: 'clarifications', unread: unreadClarifications },
                        { key: 'announcements', label: 'Announcements', icon: 'announcements', unread: announcementsApi.unreadCount },
                    ] as { key: Tab; label: string; icon: string; unread?: number }[]).map(tab => (
                        <button
                            key={tab.key}
                            onClick={() => {
                                if (tab.key !== activeTab) {
                                    setActiveTab(tab.key)
                                    if (tab.key === 'clarifications') setUnreadClarifications(0)
                                }
                            }}
                            className={`pb-3 text-sm font-semibold transition-colors flex items-center gap-1.5 ${activeTab === tab.key
                                ? 'text-brand border-b-2 border-brand'
                                : 'text-muted hover:text-ink border-b-2 border-transparent'
                                }`}
                        >
                            {tab.icon === 'about' && <Info className="w-4 h-4" />}
                            {tab.icon === 'problems' && <FileText className="w-4 h-4" />}
                            {tab.icon === 'submissions' && <Send className="w-4 h-4" />}
                            {tab.icon === 'clarifications' && <MessageSquare className="w-4 h-4" />}
                            {tab.icon === 'announcements' && <Megaphone className="w-4 h-4" />}
                            {tab.label}
                            {tab.unread !== undefined && tab.unread > 0 ? (
                                <span className="ml-1 min-w-[18px] rounded-full bg-danger-fill px-1.5 py-0.5 text-center text-xs font-bold text-danger-fill-ink">{tab.unread}</span>
                            ) : null}
                        </button>
                    ))}
                    {/* Standings used to be a tab here as well as a page of its
                        own, which meant two renderings of the same table that
                        could disagree. It is now only the page, opened in a new
                        tab: a standings table is compared against the problems
                        while reading them, so opening it alongside this page is
                        what the link is for. */}
                    <a
                        href={`/contests/${id}/scoreboard`}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="pb-3 text-sm font-semibold transition-colors flex items-center gap-1.5 text-muted hover:text-ink border-b-2 border-transparent"
                    >
                        <Trophy className="w-4 h-4" />
                        Open Standings
                        <ExternalLink className="w-3.5 h-3.5" />
                    </a>
                </nav>
            </div>

            {/* ── Main Layout: Content + Sidebar ───────────────────────────── */}
            <div className={`items-start ${activeTab === 'submissions' ? '' : 'flex gap-6'}`}>
                {/* Left: Main Content */}
                <div className={activeTab === 'submissions' ? '' : 'flex-1 min-w-0'}>
                    {/* About. Everything here is already on the page or in the
                        payload: the round's description, its format and schedule, and
                        who runs it. It exists because those facts were spread across
                        the sidebar and a badge strip, and the one thing a newcomer
                        wants — what is this round, and who is running it — had no
                        single place to be read. */}
                    {activeTab === 'about' && (
                        <div className="space-y-6">
                            <div>
                                <h2 className="text-xl font-bold text-ink mb-4">About this round</h2>
                                {contest.description?.trim() ? (
                                    <p className="text-sm text-ink-soft whitespace-pre-wrap leading-relaxed">
                                        {contest.description}
                                    </p>
                                ) : (
                                    <p className="text-sm text-muted-soft">
                                        The organizer has not added a description.
                                    </p>
                                )}
                            </div>

                            {/* Who runs it. Absent from the payload entirely when the
                                caller may not read the round as content, which is the
                                server's call — the page does not decide who may see
                                the roster. */}
                            {contest.judges !== undefined && (
                                <div>
                                    <h3 className="text-sm font-semibold text-ink uppercase tracking-wide mb-3">
                                        Organizers
                                    </h3>
                                    {contest.judges.length === 0 ? (
                                        <p className="text-sm text-muted-soft">No organizers are listed.</p>
                                    ) : (
                                        <ul className="flex flex-wrap gap-2">
                                            {contest.judges.map(j => (
                                                <li
                                                    key={j.user_id || j.username}
                                                    className="flex items-center gap-2 rounded-lg border border-hairline bg-surface px-3 py-1.5"
                                                >
                                                    <Shield className="w-3.5 h-3.5 text-muted-soft" />
                                                    {j.username ? (
                                                        <Link
                                                            to={`/user/${j.username}`}
                                                            className="text-sm font-medium text-brand hover:underline"
                                                        >
                                                            {j.username}
                                                        </Link>
                                                    ) : (
                                                        <span className="text-sm text-ink-soft">unknown</span>
                                                    )}
                                                    <span className="text-[11px] uppercase tracking-wide text-muted-soft">
                                                        {j.access_level === 'manager' ? 'manager' : 'judge'}
                                                    </span>
                                                </li>
                                            ))}
                                        </ul>
                                    )}
                                </div>
                            )}

                            {/* What the round offers. Not a repeat of the sidebar's
                                Contest Info box — that one carries the schedule, format
                                and problem count, and this one names the capabilities,
                                which had no home anywhere. Only the features that are
                                ON are listed: a row reading "upsolving disabled" is a
                                list of everything that is not there. */}
                            <div>
                                <h3 className="text-sm font-semibold text-ink uppercase tracking-wide mb-3">
                                    Features
                                </h3>
                                <ul className="flex flex-wrap gap-2">
                                    {[
                                        { on: contest.pdf_enabled !== false, label: 'Printable booklet' },
                                        { on: contest.statement_hidden === true, label: 'Statements hidden (onsite)' },
                                        { on: contest.upsolving_enabled === true, label: 'Upsolving' },
                                        { on: contest.virtual_contest_enabled === true, label: 'Virtual participation' },
                                    ].filter(f => f.on).map(f => (
                                        <li
                                            key={f.label}
                                            className="rounded-full border border-hairline bg-surface px-3 py-1 text-xs text-ink-soft"
                                        >
                                            {f.label}
                                        </li>
                                    ))}
                                    {contest.registration_required && (
                                        <li className="rounded-full border border-hairline bg-surface px-3 py-1 text-xs text-ink-soft">
                                            Registration required
                                        </li>
                                    )}
                                </ul>
                            </div>
                        </div>
                    )}

                    {activeTab === 'problems' && (
                        <div>
                            <h2 className="text-xl font-bold text-ink mb-4">Problems</h2>
                            {problems.length === 0 ? (
                                // Before the start the list is deliberately empty,
                                // not unpopulated. Saying "no problems yet" there
                                // would be a lie the round then contradicts.
                                isUpcoming ? (
                                    <EmptyState
                                        icon={<Clock className="w-12 h-12 text-ink-soft" />}
                                        text="Problems are revealed when the round starts"
                                        description={`The problem set opens at ${formatDateTime(contest.start_time)}.`}
                                    />
                                ) : (
                                    <EmptyState icon={<FileText className="w-12 h-12 text-ink-soft" />} text="No problems added yet" description="Contest organizers haven't published problems for this contest yet." />
                                )
                            ) : (
                                <div className="bg-surface border border-hairline rounded-xl shadow-sm overflow-hidden">
                                    <table className="w-full text-sm">
                                        <thead>
                                            <tr className="bg-surface-2/20 border-b border-hairline">
                                                <th className="text-left px-4 py-3 font-semibold text-muted">#</th>
                                                <th className="text-left px-4 py-3 font-semibold text-muted">Problem</th>
                                                <th className="text-center px-4 py-3 font-semibold text-muted">Solved by</th>
                                                <th className="text-right px-4 py-3 font-semibold text-muted">Score</th>
                                                <th className="text-center px-4 py-3 font-semibold text-muted">Action</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            {problems.map((p, i) => {
                                                const letter = problemLetter(p, i)
                                                // A green tick means solved. An amber dot
                                                // means tried but not yet solved, so a
                                                // half-finished problem is not mistaken
                                                // for an untouched one.
                                                return (
                                                    <tr key={p.problem_id} className="border-b border-hairline last:border-0 hover:bg-surface-2 transition-colors">
                                                        <td className="px-4 py-3 font-mono font-bold text-muted">
                                                            <span className="inline-flex items-center gap-1.5">
                                                                {p.my_solved && (
                                                                    <CheckCircle2 className="w-4 h-4 text-ok flex-shrink-0" aria-label="Solved" />
                                                                )}
                                                                {letter}
                                                            </span>
                                                        </td>
                                                        <td className="px-4 py-3">
                                                            <Link
                                                                to={`/contests/${id}/problem/${letter}`}
                                                                className="text-brand hover:text-brand hover:underline font-medium"
                                                            >
                                                                {p.title || `Problem ${letter}`}
                                                            </Link>
                                                            {p.my_solved && (
                                                                <span className="ml-2 text-[11px] font-semibold uppercase tracking-wide text-ok">Solved</span>
                                                            )}
                                                        </td>
                                                        <td className="px-4 py-3 text-center font-mono text-xs">
                                                            {typeof p.solved_count === 'number' ? (
                                                                <span className="inline-flex items-center gap-1 text-muted" title={`${p.solved_count} participant${p.solved_count === 1 ? '' : 's'} solved this`}>
                                                                    <Users className="w-3 h-3" />
                                                                    {p.solved_count}
                                                                </span>
                                                            ) : <span className="text-muted-soft">—</span>}
                                                        </td>
                                                        <td className="px-4 py-3 text-right font-mono text-ink-soft">{p.score ?? 100}</td>
                                                        <td className="px-4 py-3 text-center">
                                                            {p.my_solved ? (
                                                                <Link
                                                                    to={`/contests/${id}/problem/${letter}`}
                                                                    className="inline-block px-3 py-1 bg-ok/12 text-ok border border-ok/30 text-xs font-semibold rounded-lg hover:bg-ok/20 transition-colors"
                                                                >
                                                                    Review
                                                                </Link>
                                                            ) : (
                                                                <>
                                                                    {p.my_attempted && (
                                                                        <span className="mr-2 text-[11px] font-medium text-warn" title={`${p.my_submissions ?? 0} attempt${p.my_submissions === 1 ? '' : 's'}, not solved`}>
                                                                            Tried
                                                                        </span>
                                                                    )}
                                                                    <Link
                                                                        to={`/contests/${id}/problem/${letter}`}
                                                                        className="inline-block px-3 py-1 bg-brand text-on-brand text-xs font-semibold rounded-lg hover:bg-brand-hover transition-colors"
                                                                    >
                                                                        Solve
                                                                    </Link>
                                                                </>
                                                            )}
                                                        </td>
                                                    </tr>
                                                )
                                            })}
                                        </tbody>
                                    </table>
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
                            <div className="mb-4 flex items-center justify-between gap-3">
                                <div>
                                    <h2 className="text-xl font-bold text-ink">Clarifications</h2>
                                    <p className="text-xs text-muted">
                                        Questions from contestants, and the organizers' answers.
                                    </p>
                                </div>
                            </div>

                            {getAccessToken() && (
                                <div className="mb-4">
                                    <AskClarificationForm
                                        problems={clarificationProblems}
                                        onSubmit={handleAsk}
                                    />
                                </div>
                            )}

                            <ClarificationsPanel
                                items={clarificationsApi.items}
                                loading={clarificationsApi.loading}
                                problems={clarificationProblems}
                                isJudge={clarificationsApi.isJudge}
                                newIds={clarificationsApi.newIds}
                                onAnswer={clarificationsApi.isJudge ? handleAnswer : undefined}
                                onAnswerMany={clarificationsApi.isJudge ? handleAnswerMany : undefined}
                            />
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

                            {announcementsApi.announcements.length === 0 ? (
                                <EmptyState icon={<Megaphone className="w-12 h-12 text-ink-soft" />} text="No announcements yet" description="Announcements from judges will appear here" />
                            ) : (
                                <div className="space-y-3">
                                    {announcementsApi.announcements.map((a, i) => (
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
                {activeTab !== 'submissions' && (
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

                        {/* Judge Panel. Gated on staff, not on the admin role: a
                            manager, judge or tester of this round runs it and needs
                            the entry point even though their session role says
                            "user". */}
                        {isStaff && (
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

            {/* Staff shortcut, fixed to the bottom-right corner.
                The sidebar above is `hidden lg:block`, and it is also dropped
                entirely on the standings and submissions tabs — so on a phone, or
                while reading those two tables, a judge had no visible way into the
                panel. This stays reachable from every tab and every viewport. */}
            {isStaff && (
                <Link
                    to={`/setter/contest/${id}/manage`}
                    aria-label="Manage contest"
                    className="fixed bottom-5 right-5 z-40 flex items-center gap-2 px-4 py-3 bg-purple-600 text-white text-sm font-semibold rounded-full shadow-lg hover:bg-purple-700 transition-colors"
                >
                    <Settings className="w-4 h-4" />
                    <span className="hidden sm:inline">Manage Contest</span>
                </Link>
            )}

            {/* A judge's announcement is the one message that has to be seen the
                moment it is posted, whichever tab the competitor is on. */}
            {announcementsApi.popup && (
                <ContestAnnouncementPopup
                    content={announcementsApi.popup.content}
                    author={announcementsApi.popup.username}
                    onDismiss={announcementsApi.dismissPopup}
                    onViewAll={() => {
                        setActiveTab('announcements')
                        announcementsApi.markAllRead()
                    }}
                />
            )}
        </div>
    )
}
