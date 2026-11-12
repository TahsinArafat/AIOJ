import React from 'react'
import { useState, useEffect, useCallback, useRef } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'
import { errorMessage } from '../lib/errors'
import { contestVisibilityPayload } from '../lib/contestVisibility'
import { PageHeader } from '../components/ui/PageHeader'
import { useTheme } from '../context/ThemeContext'
import {
    FileText, FileDown, MessageSquare, Users, FileCode, Printer,
    Trophy, BarChart3, Search, CircleDot, Megaphone, Shield, Settings,
    Loader2, Play, Check, EyeOff, UserPlus, ArrowDown, ArrowUp, Scale
} from 'lucide-react'
import { useConfirm } from '../components/ConfirmDialog'
import { useToast } from '../components/Toast'
import { EmptyState } from '../components/EmptyState'
import { isoToLocalInput, localInputToIso } from '../lib/datetime'
import { DIVISIONS } from '../lib/divisions'
import { useClarifications } from '../hooks/useClarifications'
import { ClarificationsPanel } from '../components/ClarificationsPanel'
import { ContestAdjustmentsPanel } from './admin/ContestAdjustmentsPanel'

function indexLabel(i: number): string {
    let s = ''
    let n = i
    do {
        s = String.fromCharCode(65 + (n % 26)) + s
        n = Math.floor(n / 26) - 1
    } while (n >= 0)
    return s
}

const TABS = [
    { key: 'challenges', label: 'Challenges', Icon: FileText },
    { key: 'booklet', label: 'Booklet', Icon: FileDown },
    { key: 'clarifications', label: 'Clarifications', Icon: MessageSquare },
    { key: 'participants', label: 'Participants', Icon: Users },
    { key: 'submissions', label: 'Submissions', Icon: FileCode },
    { key: 'prints', label: 'Prints', Icon: Printer },
    { key: 'standings', label: 'Standings', Icon: Trophy },
    { key: 'adjustments', label: 'Adjustments', Icon: Scale },
    { key: 'statistics', label: 'Statistics', Icon: BarChart3 },
    { key: 'plagiarisms', label: 'Plagiarisms', Icon: Search },
    { key: 'balloons', label: 'Balloons', Icon: CircleDot },
    { key: 'announcements', label: 'Announcements', Icon: Megaphone },
    { key: 'moderators', label: 'Moderators', Icon: Shield },
    { key: 'onsite-teams', label: 'Onsite Teams', Icon: UserPlus },
    { key: 'settings', label: 'Settings', Icon: Settings },
] as const

type TabKey = typeof TABS[number]['key']

// Local row shapes for tab data — kept page-local on purpose: this page only
// needs the fields it renders, and shared API model types are in flux.
interface ManagedContest { title?: string }
interface ChallengeRow { problem_id: string; index?: string; score?: number; sort_order?: number; title?: string; slug?: string }
interface ProblemHit { id: string; title?: string; difficulty?: string }
interface RegistrationList { data?: ParticipantRow[]; count?: number }
interface ParticipantRow { user_id?: string; username?: string; registered_at: string }
interface SubmissionRow { id?: string; username?: string; user_id?: string; problem_index?: string; problem_id?: string; language?: string; status: string; time_ms?: number; created_at?: string }
interface PrintRow { id: string; filename?: string; status: string; content?: string; username?: string; created_at?: string }
interface ContestStats {
    total_participants?: number
    total_submissions?: number
    accepted_submissions?: number
    problems?: { problem_id?: string; index?: string; solve_rate: number; accepted?: number; total_submissions?: number }[]
    languages?: Record<string, number>
    verdicts?: Record<string, number>
}
interface BalloonRow { id: string; dispatched?: boolean; problem_index?: string; username?: string; team_name?: string; institution?: string; color?: string; created_at: string }
interface NoticeRow { id: string; content?: string; created_at: string }
interface ModeratorRow { user_id: string; username?: string; access_level: string }
interface OnsiteTeamRow { id: string; team_name?: string; institution?: string; username?: string; password?: string; is_used?: boolean }
interface GroupOption { id: string; name?: string }

export default function ContestManage() {
    const { id } = useParams()
    const nav = useNavigate()
    const [contest, setContest] = useState<ManagedContest | null>(null)
    const [problems, setProblems] = useState<ChallengeRow[]>([])
    const [tab, setTab] = useState<TabKey>(() => {
        const hash = window.location.hash.replace('#', '') as TabKey
        const validTabs: TabKey[] = [
            'challenges', 'booklet', 'clarifications', 'participants', 'submissions',
            'prints', 'standings', 'adjustments', 'statistics', 'plagiarisms', 'balloons',
            'announcements', 'moderators', 'onsite-teams', 'settings'
        ]
        return validTabs.includes(hash) ? hash : 'challenges'
    })
    const [loading, setLoading] = useState(true)

    // Two-way hash sync. The initializer above reads the hash once, so without
    // a hashchange listener a deep link to #balloons (or a Back/Forward step)
    // changed the URL but left the sidebar on Challenges. replaceState keeps
    // tab clicks from piling entries on the history stack.
    useEffect(() => {
        if (window.location.hash.replace('#', '') !== tab) {
            window.history.replaceState(null, '', `#${tab}`)
        }
        const onHashChange = () => {
            const next = window.location.hash.replace('#', '') as TabKey
            const valid: TabKey[] = [
                'challenges', 'booklet', 'clarifications', 'participants', 'submissions',
                'prints', 'standings', 'adjustments', 'statistics', 'plagiarisms', 'balloons',
                'announcements', 'moderators', 'onsite-teams', 'settings'
            ]
            if (valid.includes(next)) setTab(next)
        }
        window.addEventListener('hashchange', onHashChange)
        return () => window.removeEventListener('hashchange', onHashChange)
    }, [tab])

    useEffect(() => {
        if (!id) return
        api.contests.get(id).then(d => {
            setContest(d.contest)
            setProblems(d.problems || [])
        }).catch(() => nav('/contests')).finally(() => setLoading(false))
    }, [id]) // eslint-disable-line react-hooks/exhaustive-deps

    if (loading) return <div className="flex items-center justify-center h-screen"><div className="animate-spin rounded-full h-8 w-8 border-b-2 border-brand" /></div>
    if (!contest) return null

    return (
        <div>
            <PageHeader
                eyebrow={<Link to={`/contests/${id}`} className="hover:text-ink">← {contest.title}</Link>}
                title="Contest management"
                description="Problems, participants, judging and the arena for this contest."
            />

            {/* Stack under lg: the 14rem tab rail sat beside the panel on a 390px
                phone, squeezing the panel to ~150px. On mobile the rail scrolls
                horizontally so all 14 tabs stay reachable. */}
            <div className="flex flex-col lg:flex-row gap-6">
                {/* Sidebar */}
                <aside className="w-full lg:w-56 lg:shrink-0">
                    <nav className="flex lg:block overflow-x-auto lg:overflow-hidden bg-surface border border-hairline rounded-xl lg:sticky lg:top-4">
                        {TABS.map(t => (
                            <button
                                key={t.key}
                                onClick={() => setTab(t.key)}
                                className={`w-auto lg:w-full shrink-0 whitespace-nowrap flex items-center gap-2.5 px-4 py-2.5 text-sm transition-colors text-left ${tab === t.key
                                    ? 'bg-brand-soft text-brand font-medium border-l-3 border-brand'
                                    : 'text-muted hover:bg-surface-2 border-l-3 border-transparent'
                                    }`}
                            >
                                <t.Icon className="w-4 h-4" />
                                {t.label}
                            </button>
                        ))}
                        <Link
                            to={`/contests/${id}/problem/${problems[0]?.index || 'A'}`}
                            className="w-auto lg:w-full shrink-0 whitespace-nowrap flex items-center gap-2.5 px-4 py-2.5 text-sm text-muted hover:bg-ok/10 hover:text-ok transition-colors border-t lg:border-t-0 border-l-3 border-transparent"
                        >
                            <span className="text-base"><Play className="w-4 h-4" /></span>
                            Arena
                        </Link>
                    </nav>
                </aside>

                {/* Content */}
                <main className="flex-1 min-w-0">
                    {tab === 'challenges' && <ChallengesTab contestId={id!} />}
                    {tab === 'booklet' && <BookletTab contestId={id!} />}
                    {tab === 'clarifications' && <ClarificationsTab contestId={id!} />}
                    {tab === 'participants' && <ParticipantsTab contestId={id!} />}
                    {tab === 'submissions' && <SubmissionsTab contestId={id!} />}
                    {tab === 'prints' && <PrintsTab contestId={id!} />}
                    {tab === 'standings' && <StandingsTab contestId={id!} />}
                    {tab === 'adjustments' && <ContestAdjustmentsPanel contestId={id!} />}
                    {tab === 'statistics' && <StatisticsTab contestId={id!} />}
                    {tab === 'plagiarisms' && <PlagiarismsTab contestId={id!} />}
                    {tab === 'balloons' && <BalloonsTab contestId={id!} />}
                    {tab === 'announcements' && <AnnouncementsTab contestId={id!} />}
                    {tab === 'moderators' && <ModeratorsTab contestId={id!} />}
                    {tab === 'onsite-teams' && <OnsiteTeamsTab contestId={id!} />}
                    {tab === 'settings' && <SettingsTab contestId={id!} />}
                </main>
            </div>
        </div>
    )
}

// ═══════════════════════════════════════════════
// CHALLENGES TAB
// ═══════════════════════════════════════════════
// Exported for focused tests; the default export remains the routed page.
export function ChallengesTab({ contestId }: { contestId: string }) {
    const confirm = useConfirm()
    const toast = useToast()
    const [problems, setProblems] = useState<ChallengeRow[]>([])
    const [allProblems, setAllProblems] = useState<ProblemHit[]>([])
    const [showAdd, setShowAdd] = useState(false)
    const [search, setSearch] = useState('')
    const [loading, setLoading] = useState(true)
    const [reordering, setReordering] = useState(false)
    const reorderInFlight = useRef(false)

    const load = useCallback(() => {
        return api.contests.get(contestId).then(d => {
            setProblems(d.problems || [])
            setLoading(false)
        })
    }, [contestId])

    useEffect(() => { load() }, [load])

    const searchProblems = async () => {
        if (!search.trim()) return
        const res = await api.problems.listSelectable(search.trim(), 0, 20)
        setAllProblems(res.data || [])
    }

    const addProblem = async (problemId: string) => {
        const idx = indexLabel(problems.length)
        await api.contests.addProblem(contestId, { problem_id: problemId, index: idx })
        load()
        setShowAdd(false)
    }

    const removeProblem = async (problemId: string) => {
        if (!(await confirm({ message: 'Remove this problem?', variant: 'danger' }))) return
        await api.contests.removeProblem(contestId, problemId)
        load()
    }

    const moveProblem = async (idx: number, dir: -1 | 1) => {
        if (reorderInFlight.current) return
        const target = idx + dir
        if (target < 0 || target >= problems.length) return
        const next = [...problems]
            ;[next[idx], next[target]] = [next[target], next[idx]]

        const updates = next.map((p, i) => ({
            problem_id: p.problem_id,
            index: indexLabel(i),
            score: p.score || 100,
            sort_order: i,
        }))

        reorderInFlight.current = true
        setReordering(true)
        try {
            const results = await Promise.allSettled(updates.map(update =>
                api.contests.updateProblem(contestId, update.problem_id, {
                    index: update.index,
                    score: update.score,
                    sort_order: update.sort_order,
                })
            ))
            const failed = results.find(result => result.status === 'rejected')
            if (failed?.status === 'rejected') {
                throw failed.reason
            }
            setProblems(next.map((p, i) => ({
                ...p,
                index: updates[i].index,
                sort_order: updates[i].sort_order,
            })))
            toast.success('Challenge order updated')
        } catch (e) {
            toast.error('Failed to update challenge order: ' + (errorMessage(e) || 'Please try again'))
            try {
                await load()
            } catch {
                // Keep the last known-good order visible if reconciliation also fails.
            }
        } finally {
            reorderInFlight.current = false
            setReordering(false)
        }
    }

    if (loading) return <TabLoading />

    return (
        <TabShell title="Challenges" action={
            <button onClick={() => setShowAdd(!showAdd)} className="px-3 py-1.5 bg-brand text-on-brand text-sm rounded-lg hover:bg-brand-hover">
                + Add Problem
            </button>
        }>
            {showAdd && (
                <div className="bg-brand-soft border border-brand/30 rounded-lg p-4 mb-4">
                    <div className="flex gap-2 mb-3">
                        <input value={search} onChange={e => setSearch(e.target.value)} onKeyDown={e => e.key === 'Enter' && searchProblems()}
                            placeholder="Search problems..." className="flex-1 border rounded-lg px-3 py-1.5 text-sm" />
                        <button onClick={searchProblems} className="px-3 py-1.5 bg-brand text-on-brand text-sm rounded-lg">Search</button>
                    </div>
                    {allProblems.map((p) => (
                        <div key={p.id} className="flex items-center justify-between py-1.5 text-sm">
                            <span>{p.title} <span className="text-muted-soft">({p.difficulty})</span></span>
                            <button onClick={() => addProblem(p.id)} className="text-brand hover:underline text-xs font-semibold">Add</button>
                        </div>
                    ))}
                </div>
            )}
            <table className="w-full text-sm">
                <thead><tr className="border-b border-hairline text-left text-xs text-muted uppercase">
                    <th className="py-2 pr-4 w-12">#</th><th className="py-2 pr-4">Problem</th><th className="py-2 pr-4 w-20">Score</th><th className="py-2 w-40 text-right">Actions</th>
                </tr></thead>
                <tbody className="divide-y divide-hairline">
                    {problems.map((p, idx) => (
                        <tr key={p.problem_id} className="hover:bg-surface-2/30">
                            <td className="py-2.5 pr-4 font-mono font-bold text-brand">{indexLabel(idx)}</td>
                            <td className="py-2.5 pr-4">
                                <div className="flex flex-col">
                                    {p.slug ? (
                                        <Link to={`/problems/${p.slug}`} className="font-semibold text-brand hover:underline">{p.title || p.problem_id}</Link>
                                    ) : (
                                        <span className="font-semibold text-ink">{p.title || p.problem_id}</span>
                                    )}
                                    <span className="text-xs text-muted-soft font-mono">{p.problem_id}</span>
                                </div>
                            </td>
                            <td className="py-2.5 pr-4">{p.score ?? 100}</td>
                            <td className="py-2.5 text-right">
                                <div className="flex items-center justify-end gap-2">
                                    <div className="flex items-center gap-1" role="group" aria-label={`Reorder ${p.title || p.problem_id}`}>
                                        <button
                                            type="button"
                                            onClick={() => moveProblem(idx, -1)}
                                            disabled={reordering || idx === 0}
                                            aria-label={`Move ${p.title || p.problem_id} up`}
                                            aria-busy={reordering}
                                            title="Move up"
                                            className="h-10 w-10 inline-flex items-center justify-center rounded-lg border border-hairline text-muted transition-colors hover:bg-surface-2 hover:text-ink focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-1 disabled:cursor-not-allowed disabled:opacity-30 dark:border-hairline-strong dark:hover:bg-surface-3"
                                        >
                                            {reordering ? <Loader2 className="h-4 w-4 animate-spin" /> : <ArrowUp className="h-4 w-4" />}
                                        </button>
                                        <button
                                            type="button"
                                            onClick={() => moveProblem(idx, 1)}
                                            disabled={reordering || idx === problems.length - 1}
                                            aria-label={`Move ${p.title || p.problem_id} down`}
                                            aria-busy={reordering}
                                            title="Move down"
                                            className="h-10 w-10 inline-flex items-center justify-center rounded-lg border border-hairline text-muted transition-colors hover:bg-surface-2 hover:text-ink focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-1 disabled:cursor-not-allowed disabled:opacity-30 dark:border-hairline-strong dark:hover:bg-surface-3"
                                        >
                                            {reordering ? <Loader2 className="h-4 w-4 animate-spin" /> : <ArrowDown className="h-4 w-4" />}
                                        </button>
                                    </div>
                                    {p.slug && (
                                        <Link to={`/setter/${p.slug}`} className="px-2 py-1 bg-warn/10 text-warn hover:bg-warn/15 border border-warn/30 text-xs rounded">
                                            Edit
                                        </Link>
                                    )}
                                    <button onClick={() => removeProblem(p.problem_id)} className="px-2 py-1 bg-bad/10 text-bad hover:bg-bad/12 border border-bad/30 text-xs rounded">
                                        Remove
                                    </button>
                                </div>
                            </td>
                        </tr>
                    ))}
                </tbody>
            </table>
            {problems.length === 0 && <EmptyState
                icon={<FileText className="w-5 h-5 text-muted-soft" />}
                text="No problems added yet"
                description="Use Add Problem to build the contest challenge list."
            />}
        </TabShell>
    )
}

// ═══════════════════════════════════════════════
// BOOKLET TAB
// ═══════════════════════════════════════════════
function BookletTab({ contestId }: { contestId: string }) {
    // A plain link is correct here because the endpoint accepts ?token=: a
    // browser navigation cannot set an Authorization header, and the booklet is
    // public for a published round but staff-only for a draft. The server-side
    // facade (middleware.OptionalAuthMiddleware with a query-token prefix) is what
    // makes one URL work for both callers, instead of the client fetching a blob.
    const token = getAccessToken()
    const href = `/api/contests/${contestId}/pdf${token ? `?token=${encodeURIComponent(token)}` : ''}`
    return (
        <TabShell title="Booklet">
            <div className="text-center py-10">
                <FileDown className="w-16 h-16 text-muted mx-auto mb-4" />
                <p className="text-muted mb-4">Generate and download the contest problem booklet for printing</p>
                <a href={href} target="_blank" rel="noopener noreferrer"
                    className="inline-flex items-center gap-2 px-5 py-2.5 bg-brand text-on-brand rounded-lg hover:bg-brand-hover font-medium">
                    Open Booklet (Print to Save)
                </a>
                <p className="text-xs text-muted-soft mt-3">Opens in a new tab with the print dialog for PDF export</p>
            </div>
        </TabShell>
    )
}

// ═══════════════════════════════════════════════
// CLARIFICATIONS TAB
// ═══════════════════════════════════════════════
function ClarificationsTab({ contestId }: { contestId: string }) {
    const toast = useToast()
    // One implementation for the jury's queue, shared with the contestant's
    // view (see ClarificationsPanel). This tab used to render a flat list with
    // per-row inputs: no way to filter to what was still unanswered, no way to
    // answer a run of questions with the same ruling, and no live updates while
    // a round was in progress.
    const [problems, setProblems] = useState<{ problem_id: string; index: string; title: string }[]>([])
    const clarifications = useClarifications(contestId, {}, {
        enabled: Boolean(contestId),
        pollMs: 8_000,
    })

    useEffect(() => {
        api.contests.get(contestId)
            .then(res => {
                const rows = (res.problems || []) as {
                    problem_id: string; index: string; title: string
                }[]
                // The round is read in index order, so the problem filter is too.
                setProblems([...rows].sort((a, b) => a.index.localeCompare(b.index)))
            })
            .catch(() => { /* the queue still renders without the labels */ })
    }, [contestId])

    const handleAnswer = async (id: string, answer: string, isPublic: boolean) => {
        const updated = await api.clarifications.answer(contestId, id, { answer, is_public: isPublic })
        clarifications.applyOne(updated)
    }

    const handleAnswerMany = async (ids: string[], answer: string, isPublic: boolean) => {
        const { answered } = await api.clarifications.answerMany(contestId, { ids, answer, is_public: isPublic })
        clarifications.applyMany(ids)
        toast.success(`Answered ${answered} question${answered === 1 ? '' : 's'}`)
    }

    return (
        <TabShell
            title="Clarifications"
            subtitle={clarifications.items.filter(c => !c.answer).length > 0
                ? `${clarifications.items.filter(c => !c.answer).length} unanswered`
                : undefined}
        >
            <ClarificationsPanel
                items={clarifications.items}
                loading={clarifications.loading}
                problems={problems}
                isJudge={clarifications.isJudge}
                newIds={clarifications.newIds}
                onAnswer={handleAnswer}
                onAnswerMany={handleAnswerMany}
            />
        </TabShell>
    )
}

// ═══════════════════════════════════════════════
// PARTICIPANTS TAB
// ═══════════════════════════════════════════════
function ParticipantsTab({ contestId }: { contestId: string }) {
    const [data, setData] = useState<RegistrationList | null>(null)
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        api.contests.listRegistrations(contestId).then(setData).finally(() => setLoading(false))
    }, [contestId])

    if (loading) return <TabLoading />
    const participants = data?.data || []

    return (
        <TabShell title="Participants" subtitle={`${data?.count ?? participants.length} registered`}>
            {participants.length === 0 ? <EmptyState
                icon={<Users className="w-5 h-5 text-muted-soft" />}
                text="No participants yet"
                description="Registered contestants will appear here."
            /> : (
                <table className="w-full text-sm">
                    <thead><tr className="border-b border-hairline text-left text-xs text-muted uppercase">
                        <th className="py-2 pr-4">#</th><th className="py-2 pr-4">Username</th><th className="py-2">Registered</th>
                    </tr></thead>
                    <tbody className="divide-y divide-hairline">
                        {participants.map((p, i) => (
                            <tr key={p.user_id} className="hover:bg-surface-2">
                                <td className="py-2 pr-4 text-muted-soft">{i + 1}</td>
                                <td className="py-2 pr-4"><Link to={`/user/${p.username}`} className="text-brand hover:underline">{p.username}</Link></td>
                                <td className="py-2 text-muted text-xs">{new Date(p.registered_at).toLocaleString()}</td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            )}
        </TabShell>
    )
}

// ═══════════════════════════════════════════════
// SUBMISSIONS TAB
// ═══════════════════════════════════════════════
function SubmissionsTab({ contestId }: { contestId: string }) {
    const [items, setItems] = useState<SubmissionRow[]>([])
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        api.contests.submissions(contestId, 0, 100, { mine: false }).then(d => setItems(d.data || [])).finally(() => setLoading(false))
    }, [contestId])

    if (loading) return <TabLoading />

    const verdictColor = (s: string) => {
        if (s === 'ACCEPTED') return 'text-ok bg-ok/10'
        if (s?.includes('PENDING') || s === 'JUDGING') return 'text-warn bg-warn/10'
        return 'text-bad bg-bad/10'
    }

    return (
        <TabShell title="Submissions" subtitle={`${items.length} total`}>
            {items.length === 0 ? <EmptyState
                icon={<FileCode className="w-5 h-5 text-muted-soft" />}
                text="No submissions yet"
                description="Contest submissions will appear here as participants submit solutions."
            /> : (
                <div className="overflow-x-auto">
                    <table className="w-full text-sm">
                        <thead><tr className="border-b border-hairline text-left text-xs text-muted uppercase">
                            <th className="py-2 pr-3">ID</th><th className="py-2 pr-3">User</th><th className="py-2 pr-3">Problem</th>
                            <th className="py-2 pr-3">Lang</th><th className="py-2 pr-3">Verdict</th><th className="py-2 pr-3">Time</th><th className="py-2">When</th>
                        </tr></thead>
                        <tbody className="divide-y divide-hairline">
                            {items.slice(0, 100).map((s) => (
                                <tr key={s.id} className="hover:bg-surface-2">
                                    <td className="py-2 pr-3"><Link to={`/submissions/${s.id}`} className="text-brand hover:underline font-mono text-xs">{s.id?.slice(0, 8)}</Link></td>
                                    <td className="py-2 pr-3">{s.username || s.user_id?.slice(0, 8)}</td>
                                    <td className="py-2 pr-3 font-mono text-xs">{s.problem_index || s.problem_id?.slice(0, 8)}</td>
                                    <td className="py-2 pr-3 text-xs">{s.language}</td>
                                    <td className="py-2 pr-3"><span className={`text-xs px-2 py-0.5 rounded-full font-medium ${verdictColor(s.status)}`}>{s.status}</span></td>
                                    <td className="py-2 pr-3 text-xs text-muted">{s.time_ms ? `${s.time_ms}ms` : '—'}</td>
                                    <td className="py-2 text-xs text-muted-soft">{s.created_at ? new Date(s.created_at).toLocaleString() : '—'}</td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}
        </TabShell>
    )
}

// ═══════════════════════════════════════════════
// PRINTS TAB
// ═══════════════════════════════════════════════
function PrintsTab({ contestId }: { contestId: string }) {
    const [items, setItems] = useState<PrintRow[]>([])
    const [loading, setLoading] = useState(true)

    const load = useCallback(() => {
        api.onsite.listPrints(contestId).then(d => setItems((d.data || []) as PrintRow[])).finally(() => setLoading(false))
    }, [contestId])
    useEffect(() => { load() }, [load])

    const updateStatus = async (printId: string, status: string) => {
        await api.onsite.updatePrintStatus(contestId, printId, status)
        load()
    }

    if (loading) return <TabLoading />

    const statusColor = (s: string) => {
        if (s === 'printed') return 'text-ok bg-ok/12'
        if (s === 'cancelled') return 'text-bad bg-bad/12'
        return 'text-warn bg-warn/12'
    }

    return (
        <TabShell title="Print Requests" subtitle={`${items.length} requests`}>
            {items.length === 0 ? <EmptyState
                icon={<Printer className="w-5 h-5 text-muted-soft" />}
                text="No print requests"
                description="Onsite print requests will appear here for review."
            /> : (
                <div className="space-y-3">
                    {items.map((p) => (
                        <div key={p.id} className="border border-hairline rounded-lg p-4">
                            <div className="flex items-center justify-between mb-2">
                                <div className="flex items-center gap-2">
                                    <span className="font-medium text-sm">{p.filename}</span>
                                    <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${statusColor(p.status)}`}>{p.status}</span>
                                </div>
                                <div className="flex gap-2">
                                    {p.status === 'pending' && (
                                        <>
                                            <button onClick={() => updateStatus(p.id, 'printed')} className="text-xs px-2 py-1 bg-ok-fill text-white rounded hover:bg-ok-fill">Mark Printed</button>
                                            <button onClick={() => updateStatus(p.id, 'cancelled')} className="text-xs px-2 py-1 bg-bad/12 text-bad rounded hover:bg-bad/20">Cancel</button>
                                        </>
                                    )}
                                </div>
                            </div>
                            <pre className="bg-surface-2 border rounded p-3 text-xs font-mono overflow-x-auto max-h-40 overflow-y-auto">{p.content}</pre>
                            <div className="mt-2 text-xs text-muted-soft">
                                {p.username && <span>By {p.username} · </span>}
                                {p.created_at && new Date(p.created_at).toLocaleString()}
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </TabShell>
    )
}

// ═══════════════════════════════════════════════
// STANDINGS TAB
// ═══════════════════════════════════════════════
function StandingsTab({ contestId }: { contestId: string }) {
    return (
        <TabShell title="Standings">
            <div className="text-center py-6">
                <a href={`/contests/${contestId}/scoreboard`} className="text-brand hover:underline font-medium">
                    Open Full Scoreboard →
                </a>
            </div>
        </TabShell>
    )
}

// ═══════════════════════════════════════════════
// STATISTICS TAB
// ═══════════════════════════════════════════════
function StatisticsTab({ contestId }: { contestId: string }) {
    const [stats, setStats] = useState<ContestStats | null>(null)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')

    useEffect(() => {
        api.contests.stats(contestId)
            .then(r => setStats(r as ContestStats))
            .catch(e => setError(e.message))
            .finally(() => setLoading(false))
    }, [contestId])

    if (loading) return <TabLoading />
    if (error) return <TabShell title="Statistics"><div className="text-bad text-sm p-4">{error}</div></TabShell>
    if (!stats) return <TabShell title="Statistics"><EmptyState text="No statistics yet" description="Statistics appear once participants register and submit solutions." /></TabShell>

    const languagesArray = stats.languages ? Object.entries(stats.languages).map(([language, count]) => ({ language, count: count as number })) : []
    const verdictsArray = stats.verdicts ? Object.entries(stats.verdicts).map(([status, count]) => ({ status, count: count as number })) : []

    return (
        <TabShell title="Statistics">
            {/* Summary Cards */}
            <div className="grid grid-cols-3 gap-4 mb-6">
                <StatCard label="Participants" value={stats.total_participants} icon={<Users className="w-5 h-5 text-muted-soft" />} />
                <StatCard label="Submissions" value={stats.total_submissions} icon={<FileCode className="w-5 h-5 text-muted-soft" />} />
                <StatCard label="Accepted" value={stats.accepted_submissions} icon={<Check className="w-5 h-5 text-ok" />} />
            </div>

            {/* Per-problem solve rate */}
            {stats.problems && stats.problems.length > 0 && (
                <div className="mb-6">
                    <h3 className="text-sm font-semibold text-ink-soft mb-3">Problem Solve Rates</h3>
                    <div className="space-y-2">
                        {stats.problems.map((p) => (
                            <div key={p.problem_id} className="flex items-center gap-3">
                                <span className="font-mono font-bold text-brand w-8">{p.index}</span>
                                <div className="flex-1 bg-surface-3 rounded-full h-5 overflow-hidden">
                                    <div className="bg-ok/100 h-5 rounded-full flex items-center justify-end pr-2 text-xs text-white font-medium transition-all"
                                        style={{ width: `${Math.max(p.solve_rate, 2)}%` }}>
                                        {p.solve_rate > 10 && `${p.solve_rate}%`}
                                    </div>
                                </div>
                                <span className="text-xs text-muted w-24 text-right">{p.accepted}/{p.total_submissions} solved</span>
                            </div>
                        ))}
                    </div>
                </div>
            )}

            {/* Language distribution */}
            {languagesArray.length > 0 && (
                <div className="mb-6">
                    <h3 className="text-sm font-semibold text-ink-soft mb-3">Languages</h3>
                    <div className="flex gap-2 flex-wrap">
                        {languagesArray.map((l) => (
                            <span key={l.language} className="text-xs bg-surface-3 px-3 py-1.5 rounded-full">
                                {l.language}: <strong>{l.count}</strong>
                            </span>
                        ))}
                    </div>
                </div>
            )}

            {/* Verdict distribution */}
            {verdictsArray.length > 0 && (
                <div>
                    <h3 className="text-sm font-semibold text-ink-soft mb-3">Verdicts</h3>
                    <div className="flex gap-2 flex-wrap">
                        {verdictsArray.map((v) => {
                            const color = v.status === 'ACCEPTED' ? 'bg-ok/12 text-ok' :
                                v.status?.includes('PENDING') ? 'bg-warn/12 text-warn' : 'bg-bad/12 text-bad'
                            return <span key={v.status} className={`text-xs px-3 py-1.5 rounded-full ${color}`}>{v.status}: <strong>{v.count}</strong></span>
                        })}
                    </div>
                </div>
            )}
        </TabShell>
    )
}

function StatCard({ label, value, icon }: { label: string; value?: number | string; icon: React.ReactNode }) {
    return (
        <div className="bg-surface border border-hairline rounded-xl p-4 text-center">
            <div className="text-2xl mb-1">{icon}</div>
            <div className="text-2xl font-bold text-ink">{value}</div>
            <div className="text-xs text-muted uppercase tracking-wider mt-1">{label}</div>
        </div>
    )
}

// ═══════════════════════════════════════════════
// PLAGIARISMS TAB
// ═══════════════════════════════════════════════
function PlagiarismsTab({ contestId }: { contestId: string }) {
    return (
        <TabShell title="Plagiarisms">
            <div className="text-center py-6">
                <a href={`/contests/${contestId}/plagiarism`} className="text-brand hover:underline font-medium">
                    Open Plagiarism Checker →
                </a>
            </div>
        </TabShell>
    )
}

// ═══════════════════════════════════════════════
// BALLOONS TAB
// ═══════════════════════════════════════════════
function BalloonsTab({ contestId }: { contestId: string }) {
    const { theme } = useTheme()
    const [items, setItems] = useState<BalloonRow[]>([])
    const [loading, setLoading] = useState(true)
    const [filter, setFilter] = useState<'all' | 'pending' | 'dispatched'>('pending')

    const load = useCallback(() => {
        api.onsite.listBalloons(contestId).then(d => setItems((d.data || []) as BalloonRow[])).finally(() => setLoading(false))
    }, [contestId])
    useEffect(() => { load() }, [load])

    const dispatch = async (id: string) => {
        await api.onsite.dispatchBalloon(contestId, id)
        load()
    }

    if (loading) return <TabLoading />

    const filtered = items.filter(b => {
        if (filter === 'pending') return !b.dispatched
        if (filter === 'dispatched') return b.dispatched
        return true
    })

    const pending = items.filter(b => !b.dispatched).length

    return (
        <TabShell title="Balloons" subtitle={`${pending} pending`}>
            <div className="flex gap-2 mb-4">
                {(['all', 'pending', 'dispatched'] as const).map(f => (
                    <button key={f} onClick={() => setFilter(f)}
                        className={`px-3 py-1.5 text-xs rounded-full font-medium capitalize ${filter === f ? 'bg-brand text-on-brand' : 'bg-surface-3 text-muted hover:bg-surface-3'}`}>
                        {f} {f === 'pending' ? `(${pending})` : ''}
                    </button>
                ))}
            </div>
            {filtered.length === 0 ? <EmptyState
                icon={<CircleDot className="w-5 h-5 text-muted-soft" />}
                text={items.length === 0 ? 'No balloons yet' : `No ${filter} balloons`}
                description={items.length === 0
                    ? 'Balloons will appear here as contestants submit and dispatch achievements.'
                    : filter === 'pending'
                        ? 'Dispatched balloons are hidden by this filter.'
                        : filter === 'dispatched'
                            ? 'Balloons waiting to be dispatched are hidden by this filter.'
                            : 'No balloon records are available for this contest.'}
            /> : (
                <div className="space-y-2">
                    {filtered.map((b) => (
                        <div key={b.id} className={`flex items-center justify-between p-3 rounded-lg border ${b.dispatched ? 'bg-surface-2 border-hairline' : 'bg-surface border-brand/30'}`}>
                            <div className="flex items-center gap-3">
                                <CircleDot className="w-5 h-5 text-brand" />
                                <div>
                                    <div className="flex items-center gap-2">
                                        <span className="font-mono font-bold text-brand">{b.problem_index}</span>
                                        {/* A generated onsite account's username is a
                                            throwaway handle; the runner needs the team
                                            name to know where to deliver. */}
                                        <span className="text-sm font-medium">{b.team_name || b.username}</span>
                                        {b.team_name && b.institution && (
                                            <span className="text-xs text-muted">({b.institution})</span>
                                        )}
                                        {b.color && <span className="text-xs px-2 py-0.5 rounded-full" style={{ backgroundColor: theme === 'dark' ? b.color + '40' : b.color + '20', color: b.color }}>{b.color}</span>}
                                    </div>
                                    <span className="text-xs text-muted-soft">{new Date(b.created_at).toLocaleTimeString()}</span>
                                </div>
                            </div>
                            {!b.dispatched ? (
                                <button onClick={() => dispatch(b.id)} className="px-3 py-1.5 bg-ok-fill text-white text-xs rounded-lg hover:bg-ok-fill font-medium">
                                    Dispatch
                                </button>
                            ) : (
                                <span className="text-xs text-ok font-medium">Dispatched</span>
                            )}
                        </div>
                    ))}
                </div>
            )}
        </TabShell>
    )
}

// ═══════════════════════════════════════════════
// ANNOUNCEMENTS TAB
// ═══════════════════════════════════════════════
function AnnouncementsTab({ contestId }: { contestId: string }) {
    const [items, setItems] = useState<NoticeRow[]>([])
    const [loading, setLoading] = useState(true)
    const [content, setContent] = useState('')

    const load = useCallback(() => {
        api.notices.list(contestId).then(d => setItems(d.data || [])).finally(() => setLoading(false))
    }, [contestId])
    useEffect(() => { load() }, [load])

    const create = async () => {
        if (!content.trim()) return
        await api.notices.create(contestId, { content })
        setContent('')
        load()
    }

    const remove = async (noticeId: string) => {
        await api.notices.delete(contestId, noticeId)
        load()
    }

    if (loading) return <TabLoading />

    return (
        <TabShell title="Announcements">
            <div className="flex gap-2 mb-4">
                <input value={content} onChange={e => setContent(e.target.value)} onKeyDown={e => e.key === 'Enter' && create()}
                    placeholder="Write an announcement..." className="flex-1 border rounded-lg px-3 py-2 text-sm" />
                <button onClick={create} className="px-4 py-2 bg-brand text-on-brand text-sm rounded-lg hover:bg-brand-hover">Post</button>
            </div>
            {items.length === 0 ? <EmptyState
                icon={<Megaphone className="w-5 h-5 text-muted-soft" />}
                text="No announcements yet"
                description="Post a contest update above to notify participants."
            /> : (
                <div className="space-y-2">
                    {items.map((n) => (
                        <div key={n.id} className="flex items-start justify-between p-3 bg-warn/10 border border-warn/30 rounded-lg">
                            <div>
                                <p className="text-sm text-ink">{n.content}</p>
                                <span className="text-xs text-muted-soft">{new Date(n.created_at).toLocaleString()}</span>
                            </div>
                            <button onClick={() => remove(n.id)} className="text-bad hover:text-bad text-xs ml-4">✕</button>
                        </div>
                    ))}
                </div>
            )}
        </TabShell>
    )
}

// ═══════════════════════════════════════════════
// MODERATORS TAB
// ═══════════════════════════════════════════════
function ModeratorsTab({ contestId }: { contestId: string }) {
    const [items, setItems] = useState<ModeratorRow[]>([])
    const [loading, setLoading] = useState(true)
    const [userId, setUserId] = useState('')
    const [level, setLevel] = useState('judge')
    const [adding, setAdding] = useState(false)
    const [error, setError] = useState('')

    const load = useCallback(() => {
        api.permissions.list(contestId).then(d => setItems(d.data || [])).finally(() => setLoading(false))
    }, [contestId])
    useEffect(() => { load() }, [load])

    const add = async () => {
        if (!userId.trim()) return
        // This used to be a bare await: a rejected request left the box filled in
        // and the list unchanged, with nothing on screen to say why. A wrong user
        // is the expected mistake here, so it has to be reported.
        setError('')
        setAdding(true)
        try {
            await api.permissions.add(contestId, { user_id: userId.trim(), access_level: level })
            setUserId('')
            load()
        } catch (err) {
            setError(errorMessage(err) || 'Could not add that user')
        } finally {
            setAdding(false)
        }
    }

    const remove = async (uid: string) => {
        setError('')
        try {
            await api.permissions.remove(contestId, uid)
            load()
        } catch (err) {
            setError(errorMessage(err) || 'Could not remove that user')
        }
    }

    if (loading) return <TabLoading />

    const levelColor = (l: string) => {
        if (l === 'manager') return 'bg-purple-100 text-purple-700'
        if (l === 'judge') return 'bg-brand-soft text-brand'
        return 'bg-surface-3 text-ink-soft'
    }

    return (
        <TabShell title="Moderators">
            {error && <div className="bg-bad/10 border border-bad/30 text-bad px-4 py-2 rounded mb-4 text-sm">{error}</div>}
            <div className="flex gap-2 mb-4">
                <input value={userId} onChange={e => setUserId(e.target.value)} placeholder="Username or user ID..." className="flex-1 border rounded-lg px-3 py-2 text-sm" />
                <select value={level} onChange={e => setLevel(e.target.value)} className="border rounded-lg px-3 py-2 text-sm">
                    <option value="judge">Judge</option>
                    <option value="manager">Manager</option>
                    <option value="tester">Tester</option>
                </select>
                <button onClick={add} disabled={adding} className="px-4 py-2 bg-brand text-on-brand text-sm rounded-lg hover:bg-brand-hover disabled:opacity-50">
                    {adding ? 'Adding...' : 'Add'}
                </button>
            </div>
            {items.length === 0 ? <EmptyState
                icon={<Shield className="w-5 h-5 text-muted-soft" />}
                text="No moderators added"
                description="Use the username and access-level controls above to add one."
            /> : (
                <div className="space-y-2">
                    {items.map((p) => (
                        <div key={p.user_id} className="flex items-center justify-between p-3 bg-surface border border-hairline rounded-lg">
                            <div className="flex items-center gap-3">
                                <Link to={`/user/${p.username}`} className="font-medium text-sm text-ink hover:text-brand">{p.username || p.user_id}</Link>
                                <span className={`text-xs px-2 py-0.5 rounded-full font-medium capitalize ${levelColor(p.access_level)}`}>{p.access_level}</span>
                            </div>
                            <button onClick={() => remove(p.user_id)} className="text-bad hover:text-bad text-xs">Remove</button>
                        </div>
                    ))}
                </div>
            )}
        </TabShell>
    )
}

// ═══════════════════════════════════════════════
// ONSITE TEAMS TAB
// ═══════════════════════════════════════════════
function OnsiteTeamsTab({ contestId }: { contestId: string }) {
    const confirm = useConfirm()
    const toast = useToast()
    const [teams, setTeams] = useState<OnsiteTeamRow[]>([])
    const [loading, setLoading] = useState(true)
    const [generating, setGenerating] = useState(false)
    const [teamInput, setTeamInput] = useState('')

    const load = useCallback(() => {
        api.onsite.listBatch(contestId).then(d => setTeams(d.data || [])).finally(() => setLoading(false))
    }, [contestId])
    useEffect(() => { load() }, [load])

    const generateFromText = async () => {
        if (!teamInput.trim()) return
        const lines = teamInput.trim().split('\n').filter(l => l.trim())
        const teamList = lines.map(line => {
            const parts = line.split(',').map(s => s.trim())
            return { team_name: parts[0], institution: parts[1] || '' }
        })
        if (teamList.length === 0) return
        setGenerating(true)
        try {
            const res = await api.onsite.generateBatch(contestId, teamList)
            const created = res.users || []
            setTeams(prev => [...created, ...prev])
            setTeamInput('')
        } catch (e) { toast.error(errorMessage(e)) }
        finally { setGenerating(false) }
    }

    const copyAllCredentials = () => {
        const text = teams.map(t => `Team: ${t.team_name}${t.institution ? ' (' + t.institution + ')' : ''}\nUsername: ${t.username}\nPassword: ${t.password || '—'}\n`).join('\n---\n')
        navigator.clipboard.writeText(text).then(() => toast.success('Credentials copied to clipboard'))
    }

    const deleteUser = async (userId: string) => {
        if (!(await confirm({ message: 'Delete these credentials?', variant: 'danger' }))) return
        try {
            await api.onsite.deleteUser(contestId, userId)
            setTeams(prev => prev.filter(t => t.id !== userId))
        } catch (e) { toast.error(errorMessage(e)) }
    }

    if (loading) return <TabLoading />

    return (
        <TabShell title="Onsite Teams" subtitle={`${teams.length} generated`}>
            <div className="space-y-4 mb-6 p-4 bg-brand-soft border border-brand/30 rounded-lg">
                <h3 className="text-sm font-semibold text-brand">Generate Team Credentials</h3>
                <p className="text-xs text-brand">Enter one team per line: <code>TeamName, Institution</code></p>
                <textarea
                    value={teamInput}
                    onChange={e => setTeamInput(e.target.value)}
                    rows={5}
                    placeholder={"Team Alpha, MIT\nTeam Beta, Stanford\nTeam Gamma"}
                    className="w-full border border-hairline-strong rounded-lg px-3 py-2 text-sm font-mono"
                />
                <button onClick={generateFromText} disabled={generating || !teamInput.trim()}
                    className="px-4 py-2 bg-brand text-on-brand text-sm rounded-lg hover:bg-brand-hover disabled:opacity-50">
                    {generating ? 'Generating...' : 'Generate Credentials'}
                </button>
            </div>

            {teams.length > 0 && (
                <>
                    <div className="flex justify-end mb-3">
                        <button onClick={copyAllCredentials}
                            className="px-3 py-1.5 bg-surface-3 text-ink-soft text-xs rounded-lg hover:bg-surface-3">
                            Copy All to Clipboard
                        </button>
                    </div>
                    <div className="overflow-x-auto">
                        <table className="w-full text-sm">
                            <thead><tr className="border-b border-hairline text-left text-xs text-muted uppercase">
                                <th className="py-2 pr-3">Team Name</th>
                                <th className="py-2 pr-3">Institution</th>
                                <th className="py-2 pr-3">Username</th>
                                <th className="py-2 pr-3">Password</th>
                                <th className="py-2">Used</th>
                                <th className="py-2 w-16"></th>
                            </tr></thead>
                            <tbody className="divide-y divide-hairline">
                                {teams.map((t) => (
                                    <tr key={t.id} className="hover:bg-surface-2">
                                        <td className="py-2 pr-3 font-medium">{t.team_name}</td>
                                        <td className="py-2 pr-3 text-muted">{t.institution || '—'}</td>
                                        <td className="py-2 pr-3 font-mono text-xs">{t.username}</td>
                                        <td className="py-2 pr-3 font-mono text-xs">{t.password || '—'}</td>
                                        <td className="py-2">
                                            {t.is_used ? (
                                                <span className="text-xs px-2 py-0.5 bg-ok/12 text-ok rounded-full">Used</span>
                                            ) : (
                                                <span className="text-xs px-2 py-0.5 bg-warn/12 text-warn rounded-full">New</span>
                                            )}
                                        </td>
                                        <td className="py-2">
                                            <button onClick={() => deleteUser(t.id)}
                                                className="text-bad hover:text-bad text-xs font-medium">
                                                Del
                                            </button>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                </>
            )}
            {teams.length === 0 && <EmptyState
                icon={<UserPlus className="w-5 h-5 text-muted-soft" />}
                text="No team credentials generated yet"
                description="Enter team names above, then generate credentials for the onsite contest."
            />}
        </TabShell>
    )
}

// ═══════════════════════════════════════════════
// SETTINGS TAB (full edit form)
// ═══════════════════════════════════════════════
function SettingsTab({ contestId }: { contestId: string }) {
    const [form, setForm] = useState({
        title: '', slug: '', description: '', start_time: '', end_time: '', freeze_time: '',
        password: '', has_password: false, pdf_enabled: true, statement_hidden: false,
        upsolving_enabled: true, virtual_contest_enabled: true,
        visibility: 'public',
        // Separate from `visibility` on purpose. A round can keep its problems
        // behind a password and still publish its board — that is what an onsite
        // final or a hiring round is — so this is not a third option in the
        // visibility select, it is its own control.
        scoreboard_visibility: 'participants',
        group_id: '',
        division: 0,
        registration_required: false, registration_deadline: '', max_participants: '',
        is_rated: true, rated_for_min: 0, rated_for_max: 9999,
    })
    const [groups, setGroups] = useState<GroupOption[]>([])
    const [loading, setLoading] = useState(true)
    const [saving, setSaving] = useState(false)
    const [error, setError] = useState('')

    useEffect(() => {
        api.contests.get(contestId).then(d => {
            const c = d.contest
            let vis = 'public'
            if (c.group_id) {
                vis = 'group'
            } else if (c.has_password) {
                vis = 'private'
            }
            setForm({
                title: c.title || '',
                slug: c.slug || '',
                description: c.description || '',
                // Local wall-clock for the input, not a sliced UTC instant.
                start_time: isoToLocalInput(c.start_time),
                end_time: isoToLocalInput(c.end_time),
                freeze_time: isoToLocalInput(c.freeze_time),
                password: '',
                has_password: !!c.has_password,
                pdf_enabled: c.pdf_enabled ?? true,
                statement_hidden: c.statement_hidden ?? false,
                upsolving_enabled: c.upsolving_enabled ?? true,
                virtual_contest_enabled: c.virtual_contest_enabled ?? true,
                visibility: vis,
                // Default rather than a cast: an older API response, or a contest
                // saved before this field existed, means participants-only, and
                // silently rendering it as "public" would be the wrong way to be
                // wrong.
                scoreboard_visibility: c.scoreboard_visibility === 'public' ? 'public' : 'participants',
                group_id: c.group_id || '',
                division: c.division ?? 0,
                registration_required: c.registration_required ?? false,
                registration_deadline: isoToLocalInput(c.registration_deadline),
                // '' not 0: the column is nullable and an empty box means "no cap".
                max_participants: c.max_participants == null ? '' : String(c.max_participants),
                is_rated: c.is_rated ?? true,
                rated_for_min: c.rated_for_min ?? 0,
                rated_for_max: c.rated_for_max ?? 9999,
            })
        }).finally(() => setLoading(false))

        api.groups.list(0, 100).then(res => {
            setGroups(res.data || [])
        }).catch(() => { })
    }, [contestId])

    const handleSave = async () => {
        if (!form.title.trim()) { setError('Title is required'); return }
        if (!form.start_time || !form.end_time) { setError('Start and end time required'); return }
        if (new Date(form.end_time) <= new Date(form.start_time)) { setError('End must be after start'); return }
        if (form.visibility === 'private' && !form.has_password && !form.password.trim()) { setError('Password is required for Private contests'); return }
        if (form.visibility === 'group' && !form.group_id) { setError('Please select a group'); return }
        // The slug is part of the public URL, so a stray space or slash is worth
        // catching here rather than turning into a 500 on a UNIQUE violation.
        const slug = form.slug.trim()
        if (slug && !/^[a-z0-9]+(?:-[a-z0-9]+)*$/i.test(slug)) {
            setError('Slug may contain only letters, numbers and hyphens')
            return
        }
        const maxParticipants = form.max_participants.trim() === '' ? undefined : Number(form.max_participants)
        if (maxParticipants !== undefined && (!Number.isFinite(maxParticipants) || maxParticipants < 0)) {
            setError('Max participants must be a positive number')
            return
        }
        setError('')
        setSaving(true)
        try {
            await api.contests.update(contestId, {
                title: form.title,
                // undefined = leave the stored slug alone; the field is optional
                // and a contest may legitimately have none.
                slug: slug || undefined,
                description: form.description || undefined,
                // The input holds a local wall clock; this is the one place it
                // becomes the UTC instant the API expects.
                start_time: localInputToIso(form.start_time),
                end_time: localInputToIso(form.end_time),
                freeze_time: localInputToIso(form.freeze_time),
                ...contestVisibilityPayload(form),
                scoreboard_visibility: form.scoreboard_visibility,
                pdf_enabled: form.pdf_enabled,
                statement_hidden: form.statement_hidden,
                upsolving_enabled: form.upsolving_enabled,
                virtual_contest_enabled: form.virtual_contest_enabled,
                division: form.division,
                registration_required: form.registration_required,
                // null clears the deadline; undefined would mean "leave it alone",
                // and the API distinguishes the two.
                registration_deadline: form.registration_deadline ? localInputToIso(form.registration_deadline) : null,
                max_participants: maxParticipants,
                is_rated: form.is_rated,
                rated_for_min: form.rated_for_min,
                rated_for_max: form.rated_for_max,
            })
        } catch (err) {
            setError(errorMessage(err) || 'Failed to save')
        } finally {
            setSaving(false)
        }
    }

    if (loading) return <TabLoading />

    return (
        <TabShell title="Settings">
            {error && <div className="bg-bad/10 border border-bad/30 text-bad px-4 py-2 rounded mb-4 text-sm">{error}</div>}

            <div className="space-y-5">
                {/* Title */}
                {/* Title + Slug. The slug is the contest's public URL
                    (/contests/<slug>); without this field a round created with an
                    auto-generated slug had no way to get a readable one. The server
                    has always accepted it on update — the panel just never asked. */}
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div>
                        <label className="block text-sm font-medium text-ink-soft mb-1">Title</label>
                        <input aria-label="Title" value={form.title} onChange={e => setForm(p => ({ ...p, title: e.target.value }))}
                            className="w-full border border-hairline-strong rounded-lg px-3 py-2 text-sm focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-brand" />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-ink-soft mb-1">URL Slug <span className="text-muted-soft">(optional)</span></label>
                        <input aria-label="URL Slug" value={form.slug} onChange={e => setForm(p => ({ ...p, slug: e.target.value }))}
                            placeholder="aioj-round-1"
                            className="w-full border border-hairline-strong rounded-lg px-3 py-2 text-sm focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-brand" />
                        <p className="text-xs text-muted mt-1">Letters, numbers and hyphens. Changing it breaks existing links.</p>
                    </div>
                </div>

                {/* Division */}
                <div>
                    <label className="block text-sm font-medium text-ink-soft mb-1">Division</label>
                    <select aria-label="Division" value={form.division} onChange={e => setForm(p => ({ ...p, division: Number(e.target.value) }))}
                        className="w-full border border-hairline-strong rounded-lg px-3 py-2 text-sm focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-brand bg-surface">
                        {Object.entries(DIVISIONS).map(([value, d]) => (
                            <option key={value} value={value}>{d.name}</option>
                        ))}
                    </select>
                </div>

                {/* Description */}
                <div>
                    <label className="block text-sm font-medium text-ink-soft mb-1">Description</label>
                    <textarea aria-label="Description" value={form.description} onChange={e => setForm(p => ({ ...p, description: e.target.value }))}
                        rows={3} className="w-full border border-hairline-strong rounded-lg px-3 py-2 text-sm focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-brand" />
                </div>

                {/* Times */}
                <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                    <div>
                        <label className="block text-sm font-medium text-ink-soft mb-1">Start Time</label>
                        <input aria-label="Start Time" type="datetime-local" value={form.start_time} onChange={e => setForm(p => ({ ...p, start_time: e.target.value }))}
                            className="w-full border border-hairline-strong rounded-lg px-3 py-2 text-sm focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-brand" />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-ink-soft mb-1">End Time</label>
                        <input aria-label="End Time" type="datetime-local" value={form.end_time} onChange={e => setForm(p => ({ ...p, end_time: e.target.value }))}
                            className="w-full border border-hairline-strong rounded-lg px-3 py-2 text-sm focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-brand" />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-ink-soft mb-1">Freeze Time <span className="text-muted-soft">(optional)</span></label>
                        <input aria-label="Freeze Time (optional)" type="datetime-local" value={form.freeze_time} onChange={e => setForm(p => ({ ...p, freeze_time: e.target.value }))}
                            className="w-full border border-hairline-strong rounded-lg px-3 py-2 text-sm focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-brand" />
                    </div>
                </div>

                {/* Visibility Settings */}
                <div>
                    <label className="block text-sm font-medium text-ink-soft mb-1">Visibility Mode</label>
                    <select aria-label="Visibility Mode" value={form.visibility} onChange={e => setForm(p => ({ ...p, visibility: e.target.value }))}
                        className="w-full border border-hairline-strong rounded-lg px-3 py-2 text-sm focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-brand bg-surface">
                        <option value="public">Public (Anyone can join)</option>
                        <option value="private">Private (Password/Invite required)</option>
                        <option value="group">Specific Group</option>
                    </select>
                </div>

                {/* Scoreboard visibility. A separate axis from the block above,
                    because a round can be private and still publish its board:
                    an onsite final is closed to the room and its scoreboard is
                    on a projector. The two settings do not imply each other, so
                    they are not options in one select. */}
                <div>
                    <label className="block text-sm font-medium text-ink-soft mb-1">Scoreboard Visibility</label>
                    <select aria-label="Scoreboard Visibility" value={form.scoreboard_visibility}
                        onChange={e => setForm(p => ({ ...p, scoreboard_visibility: e.target.value }))}
                        className="w-full border border-hairline-strong rounded-lg px-3 py-2 text-sm focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-brand bg-surface">
                        <option value="participants">Participants only (whoever can open the round)</option>
                        <option value="public">Anyone (watchable without the password)</option>
                    </select>
                    <p className="text-xs text-muted-soft mt-1">
                        {form.scoreboard_visibility === 'public'
                            ? 'Anyone with the link can watch the scoreboard, including people who cannot open the problems. Problem titles are hidden from them. The round stays out of the contest list and search either way.'
                            : 'Only people who can already open the round can see the scoreboard.'}
                    </p>
                </div>

                {form.visibility === 'private' && (
                    <div>
                        <label className="block text-sm font-medium text-ink-soft mb-1">Contest Password</label>
                        <input aria-label="Contest Password" value={form.password} onChange={e => setForm(p => ({ ...p, password: e.target.value }))}
                            placeholder={form.has_password ? "•••••••• (leave blank to keep current)" : "Enter password for private access..."}
                            className="w-full border border-hairline-strong rounded-lg px-3 py-2 text-sm focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-brand" />
                    </div>
                )}

                {form.visibility === 'group' && (
                    <div>
                        <label className="block text-sm font-medium text-ink-soft mb-1">Select Restricted Group</label>
                        <select aria-label="Select Restricted Group" value={form.group_id} onChange={e => setForm(p => ({ ...p, group_id: e.target.value }))}
                            className="w-full border border-hairline-strong rounded-lg px-3 py-2 text-sm focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-brand bg-surface">
                            <option value="">-- Choose Group --</option>
                            {groups.map(g => (
                                <option key={g.id} value={g.id}>{g.name}</option>
                            ))}
                        </select>
                    </div>
                )}

                {/* Registration. The server has enforced registration_required
                    since the field existed — submit answers "you must register"
                    — but no screen ever offered it, so a round could not be made
                    registration-only after it was created. */}
                <div>
                    <label className="block text-sm font-medium text-ink-soft mb-2">Registration</label>
                    <label className="flex items-start gap-2 cursor-pointer p-2 rounded-lg hover:bg-surface-2 border border-hairline">
                        <input type="checkbox" checked={form.registration_required}
                            onChange={e => setForm(p => ({ ...p, registration_required: e.target.checked }))}
                            className="mt-0.5 rounded border-hairline-strong text-brand focus-visible:outline-[var(--ring)]" />
                        <span className="text-sm">
                            Require registration before submitting
                            <span className="block text-xs text-muted mt-0.5">
                                {form.registration_required
                                    ? 'Contestants must register during the window below before they can submit.'
                                    : 'Anyone who may open the round may submit without registering.'}
                            </span>
                        </span>
                    </label>
                    {form.registration_required && (
                        <div className="mt-3 grid grid-cols-1 md:grid-cols-2 gap-4">
                            <div>
                                <label className="block text-xs font-medium text-muted mb-1">Registration deadline <span className="text-muted-soft">(optional)</span></label>
                                <input aria-label="Registration deadline" type="datetime-local" value={form.registration_deadline}
                                    onChange={e => setForm(p => ({ ...p, registration_deadline: e.target.value }))}
                                    className="w-full border border-hairline-strong rounded-lg px-3 py-2 text-sm focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-brand" />
                            </div>
                            <div>
                                <label className="block text-xs font-medium text-muted mb-1">Max participants <span className="text-muted-soft">(optional)</span></label>
                                <input aria-label="Max participants" type="number" min={0} value={form.max_participants}
                                    onChange={e => setForm(p => ({ ...p, max_participants: e.target.value }))}
                                    placeholder="Unlimited"
                                    className="w-full border border-hairline-strong rounded-lg px-3 py-2 text-sm focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-brand" />
                            </div>
                        </div>
                    )}
                </div>

                {/* Options */}
                <div>
                    <label className="block text-sm font-medium text-ink-soft mb-2">Options</label>
                    <div className="grid grid-cols-2 gap-3">
                        {([
                            ['pdf_enabled', 'PDF Generation', FileText],
                            ['statement_hidden', 'Hide Statements (Onsite)', EyeOff],
                            ['upsolving_enabled', 'Enable Upsolving', Loader2],
                            ['virtual_contest_enabled', 'Virtual Contests', Play],
                        ] as const).map(([key, label, icon]) => (
                            <label key={key} className="flex items-center gap-2 cursor-pointer p-2 rounded-lg hover:bg-surface-2 border border-hairline">
                                <input type="checkbox" checked={form[key] as boolean}
                                    onChange={e => setForm(p => ({ ...p, [key]: e.target.checked }))}
                                    className="rounded border-hairline-strong text-brand focus-visible:outline-[var(--ring)]" />
                                <span className="text-sm flex items-center gap-1.5">{typeof icon === "string" ? icon : React.createElement(icon as unknown as React.ComponentType<{ className?: string }>, { className: "w-4 h-4" })} {label}</span>
                            </label>
                        ))}
                    </div>
                </div>

                {/* Rating. is_rated has been in the schema since migration
                    000003 but no screen ever offered it, so every contest was
                    silently rated. An unrated round is a real choice — a
                    practice or onsite round must not move anyone's rating. */}
                <div>
                    <label className="block text-sm font-medium text-ink-soft mb-2">Rating</label>
                    <label className="flex items-start gap-2 cursor-pointer p-2 rounded-lg hover:bg-surface-2 border border-hairline">
                        <input type="checkbox" checked={form.is_rated}
                            onChange={e => setForm(p => ({ ...p, is_rated: e.target.checked }))}
                            className="mt-0.5 rounded border-hairline-strong text-brand focus-visible:outline-[var(--ring)]" />
                        <span className="text-sm">
                            Rated contest
                            <span className="block text-xs text-muted mt-0.5">
                                {form.is_rated
                                    ? 'Rating changes are calculated when the round ends.'
                                    : 'The round will not change anyone’s rating.'}
                            </span>
                        </span>
                    </label>
                    {form.is_rated && (
                        <div className="mt-3 grid grid-cols-2 gap-3">
                            <div>
                                <label className="block text-xs font-medium text-muted mb-1">Rated for rating ≥</label>
                                <input aria-label="Rated from rating" type="number" min={0} value={form.rated_for_min}
                                    onChange={e => setForm(p => ({ ...p, rated_for_min: Number(e.target.value) || 0 }))}
                                    className="w-full border border-hairline-strong rounded-lg px-3 py-2 text-sm focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-brand" />
                            </div>
                            <div>
                                <label className="block text-xs font-medium text-muted mb-1">Rated for rating ≤</label>
                                <input aria-label="Rated to rating" type="number" min={0} value={form.rated_for_max}
                                    onChange={e => setForm(p => ({ ...p, rated_for_max: Number(e.target.value) || 0 }))}
                                    className="w-full border border-hairline-strong rounded-lg px-3 py-2 text-sm focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-brand" />
                            </div>
                        </div>
                    )}
                </div>

                {/* Save */}
                <div className="flex gap-3 pt-2">
                    <button onClick={handleSave} disabled={saving}
                        className="px-6 py-2.5 bg-brand text-on-brand rounded-lg hover:bg-brand-hover disabled:opacity-50 font-medium text-sm transition-colors">
                        {saving ? 'Saving...' : 'Save Changes'}
                    </button>
                    <Link to={`/contests/${contestId}`} className="px-4 py-2.5 bg-surface-3 text-muted rounded-lg hover:bg-surface-3 text-sm">
                        Cancel
                    </Link>
                </div>
            </div>
        </TabShell>
    )
}

// ═══════════════════════════════════════════════
// SHARED COMPONENTS
// ═══════════════════════════════════════════════
function TabShell({ title, subtitle, action, children }: { title: string; subtitle?: string; action?: React.ReactNode; children: React.ReactNode }) {
    return (
        <div className="bg-surface border border-hairline rounded-xl overflow-hidden">
            <div className="px-5 py-3.5 border-b border-hairline bg-surface-2 flex items-center justify-between">
                <div>
                    <h2 className="font-semibold text-ink">{title}</h2>
                    {subtitle && <p className="text-xs text-muted-soft mt-0.5">{subtitle}</p>}
                </div>
                {action}
            </div>
            <div className="p-5">{children}</div>
        </div>
    )
}

function TabLoading() {
    return (
        <div className="flex items-center justify-center py-20">
            <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-brand" />
        </div>
    )
}

