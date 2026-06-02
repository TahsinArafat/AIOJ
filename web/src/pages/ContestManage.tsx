import React from 'react'
import { useState, useEffect, useCallback } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { useTheme } from '../context/ThemeContext'
import {
    FileText, FileDown, MessageSquare, Users, FileCode, Printer,
    Trophy, BarChart3, Search, CircleDot, Megaphone, Shield, Settings,
    Loader2, Play, Check, EyeOff, UserPlus
} from 'lucide-react'

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
    { key: 'statistics', label: 'Statistics', Icon: BarChart3 },
    { key: 'plagiarisms', label: 'Plagiarisms', Icon: Search },
    { key: 'balloons', label: 'Balloons', Icon: CircleDot },
    { key: 'announcements', label: 'Announcements', Icon: Megaphone },
    { key: 'moderators', label: 'Moderators', Icon: Shield },
    { key: 'onsite-teams', label: 'Onsite Teams', Icon: UserPlus },
    { key: 'settings', label: 'Settings', Icon: Settings },
] as const

type TabKey = typeof TABS[number]['key']

export default function ContestManage() {
    const { id } = useParams()
    const nav = useNavigate()
    const [contest, setContest] = useState<any>(null)
    const [problems, setProblems] = useState<any[]>([])
    const [tab, setTab] = useState<TabKey>(() => {
        const hash = window.location.hash.replace('#', '') as TabKey
        const validTabs: TabKey[] = [
            'challenges', 'booklet', 'clarifications', 'participants', 'submissions',
            'prints', 'standings', 'statistics', 'plagiarisms', 'balloons',
            'announcements', 'moderators', 'onsite-teams', 'settings'
        ]
        return validTabs.includes(hash) ? hash : 'challenges'
    })
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        window.location.hash = tab
    }, [tab])

    useEffect(() => {
        if (!id) return
        api.contests.get(id).then(d => {
            setContest(d.contest || d)
            setProblems(d.problems || [])
        }).catch(() => nav('/contests')).finally(() => setLoading(false))
    }, [id])

    if (loading) return <div className="flex items-center justify-center h-screen"><div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600" /></div>
    if (!contest) return null

    return (
        <div className="max-w-7xl mx-auto px-4 py-6">
            {/* Header */}
            <div className="mb-6">
                <Link to={`/contests/${id}`} className="text-sm text-blue-600 hover:underline">← {contest.title}</Link>
                <h1 className="text-2xl font-bold mt-1">Contest Management</h1>
            </div>

            <div className="flex gap-6">
                {/* Sidebar */}
                <aside className="w-56 shrink-0">
                    <nav className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl overflow-hidden sticky top-4">
                        {TABS.map(t => (
                            <button
                                key={t.key}
                                onClick={() => setTab(t.key)}
                                className={`w-full flex items-center gap-2.5 px-4 py-2.5 text-sm transition-colors text-left ${
                                    tab === t.key
                                        ? 'bg-blue-50 text-blue-700 font-medium border-l-3 border-blue-600'
                                        : 'text-gray-600 hover:bg-gray-50 border-l-3 border-transparent'
                                }`}
                            >
                                <t.Icon className="w-4 h-4" />
                                {t.label}
                            </button>
                        ))}
                        <Link
                            to={`/contests/${id}/problem/${problems[0]?.index || 'A'}`}
                            className="w-full flex items-center gap-2.5 px-4 py-2.5 text-sm text-gray-600 hover:bg-green-50 hover:text-green-700 transition-colors border-t border-gray-100 border-l-3 border-transparent"
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
function ChallengesTab({ contestId }: { contestId: string }) {
    const [problems, setProblems] = useState<any[]>([])
    const [allProblems, setAllProblems] = useState<any[]>([])
    const [showAdd, setShowAdd] = useState(false)
    const [search, setSearch] = useState('')
    const [loading, setLoading] = useState(true)

    const load = useCallback(() => {
        api.contests.get(contestId).then(d => {
            setProblems(d.problems || [])
            setLoading(false)
        })
    }, [contestId])

    useEffect(() => { load() }, [load])

    const searchProblems = async () => {
        if (!search.trim()) return
        const res = await api.problems.list(0, 20, { search })
        setAllProblems(res.data || [])
    }

    const addProblem = async (problemId: string) => {
        const idx = indexLabel(problems.length)
        await api.contests.addProblem(contestId, { problem_id: problemId, index: idx })
        load()
        setShowAdd(false)
    }

    const removeProblem = async (problemId: string) => {
        if (!confirm('Remove this problem?')) return
        await api.contests.removeProblem(contestId, problemId)
        load()
    }

    const moveProblem = async (idx: number, dir: -1 | 1) => {
        const target = idx + dir
        if (target < 0 || target >= problems.length) return
        const next = [...problems]
        ;[next[idx], next[target]] = [next[target], next[idx]]
        
        next.forEach((p, i) => {
            p.index = indexLabel(i)
            p.sort_order = i
        })
        
        try {
            await Promise.all(next.map((p, i) => 
                api.contests.updateProblem(contestId, p.problem_id, {
                    index: p.index,
                    score: p.score || 100,
                    sort_order: i
                })
            ))
            load()
        } catch (e: any) {
            alert('Failed to reorder: ' + e.message)
        }
    }

    if (loading) return <TabLoading />

    return (
        <TabShell title="Challenges" action={
            <button onClick={() => setShowAdd(!showAdd)} className="px-3 py-1.5 bg-blue-600 text-white text-sm rounded-lg hover:bg-blue-700">
                + Add Problem
            </button>
        }>
            {showAdd && (
                <div className="bg-blue-50 border border-blue-200 rounded-lg p-4 mb-4">
                    <div className="flex gap-2 mb-3">
                        <input value={search} onChange={e => setSearch(e.target.value)} onKeyDown={e => e.key === 'Enter' && searchProblems()}
                            placeholder="Search problems..." className="flex-1 border rounded-lg px-3 py-1.5 text-sm" />
                        <button onClick={searchProblems} className="px-3 py-1.5 bg-blue-600 text-white text-sm rounded-lg">Search</button>
                    </div>
                    {allProblems.map((p: any) => (
                        <div key={p.id} className="flex items-center justify-between py-1.5 text-sm">
                            <span>{p.title} <span className="text-gray-400">({p.difficulty})</span></span>
                            <button onClick={() => addProblem(p.id)} className="text-blue-600 hover:underline text-xs font-semibold">Add</button>
                        </div>
                    ))}
                </div>
            )}
            <table className="w-full text-sm">
                <thead><tr className="border-b border-gray-200 dark:border-gray-700 text-left text-xs text-gray-500 uppercase">
                    <th className="py-2 pr-4 w-12">#</th><th className="py-2 pr-4">Problem</th><th className="py-2 pr-4 w-20">Score</th><th className="py-2 w-32 text-right">Actions</th>
                </tr></thead>
                <tbody className="divide-y divide-gray-100">
                    {problems.map((p: any, idx: number) => (
                        <tr key={p.problem_id} className="hover:bg-gray-50 dark:hover:bg-gray-700/30">
                            <td className="py-2.5 pr-4 font-mono font-bold text-blue-600">{indexLabel(idx)}</td>
                            <td className="py-2.5 pr-4">
                                <div className="flex flex-col">
                                    {p.slug ? (
                                        <Link to={`/problems/${p.slug}`} className="font-semibold text-blue-600 hover:underline">{p.title || p.problem_id}</Link>
                                    ) : (
                                        <span className="font-semibold text-gray-800">{p.title || p.problem_id}</span>
                                    )}
                                    <span className="text-xs text-gray-400 font-mono">{p.problem_id}</span>
                                </div>
                            </td>
                            <td className="py-2.5 pr-4">{p.score ?? 100}</td>
                            <td className="py-2.5 text-right">
                                <div className="flex items-center justify-end gap-2">
                                    <button onClick={() => moveProblem(idx, -1)} disabled={idx === 0}
                                        className="p-1 border rounded text-xs text-gray-400 hover:text-gray-700 hover:bg-gray-50 disabled:opacity-30 disabled:cursor-not-allowed">
                                        ▲
                                    </button>
                                    <button onClick={() => moveProblem(idx, 1)} disabled={idx === problems.length - 1}
                                        className="p-1 border rounded text-xs text-gray-400 hover:text-gray-700 hover:bg-gray-50 disabled:opacity-30 disabled:cursor-not-allowed">
                                        ▼
                                    </button>
                                    {p.slug && (
                                        <Link to={`/setter/${p.slug}`} className="px-2 py-1 bg-orange-50 text-orange-700 hover:bg-orange-100 border border-orange-200 text-xs rounded">
                                            Edit
                                        </Link>
                                    )}
                                    <button onClick={() => removeProblem(p.problem_id)} className="px-2 py-1 bg-red-50 text-red-700 hover:bg-red-100 border border-red-200 text-xs rounded">
                                        Remove
                                    </button>
                                </div>
                            </td>
                        </tr>
                    ))}
                </tbody>
            </table>
            {problems.length === 0 && <EmptyState icon={<FileText className="w-5 h-5 text-gray-400" />} text="No problems added yet" />}
        </TabShell>
    )
}

// ═══════════════════════════════════════════════
// BOOKLET TAB
// ═══════════════════════════════════════════════
function BookletTab({ contestId }: { contestId: string }) {
    return (
        <TabShell title="Booklet">
            <div className="text-center py-10">
                <FileDown className="w-16 h-16 text-gray-300 mx-auto mb-4" />
                <p className="text-gray-500 mb-4">Generate and download the contest problem booklet as PDF</p>
                <a href={`/api/contests/${contestId}/pdf`} target="_blank" rel="noopener noreferrer"
                    className="inline-flex items-center gap-2 px-5 py-2.5 bg-blue-600 text-white rounded-lg hover:bg-blue-700 font-medium">
                    Open Booklet (Print to Save)
                </a>
                <p className="text-xs text-gray-400 mt-3">Opens in a new tab with print dialog for PDF export</p>
            </div>
        </TabShell>
    )
}

// ═══════════════════════════════════════════════
// CLARIFICATIONS TAB
// ═══════════════════════════════════════════════
function ClarificationsTab({ contestId }: { contestId: string }) {
    const [items, setItems] = useState<any[]>([])
    const [loading, setLoading] = useState(true)
    const [answer, setAnswer] = useState<Record<string, string>>({})
    const [publicAns, setPublicAns] = useState<Record<string, boolean>>({})

    const load = useCallback(() => {
        api.clarifications.list(contestId).then(d => setItems(d.data || [])).finally(() => setLoading(false))
    }, [contestId])
    useEffect(() => { load() }, [load])

    const handleAnswer = async (id: string) => {
        if (!answer[id]?.trim()) return
        await api.clarifications.answer(contestId, id, { answer: answer[id], is_public: publicAns[id] || false })
        setAnswer(p => ({ ...p, [id]: '' }))
        load()
    }

    if (loading) return <TabLoading />

    return (
        <TabShell title="Clarifications">
            {items.length === 0 ? <EmptyState icon={<MessageSquare className="w-5 h-5 text-gray-400" />} text="No clarifications yet" /> : (
                <div className="space-y-3">
                    {items.map((c: any) => (
                        <div key={c.id} className="border border-gray-200 dark:border-gray-700 rounded-lg p-4">
                            <div className="flex items-start justify-between mb-2">
                                <div>
                                    <span className="text-xs text-gray-400">{new Date(c.created_at).toLocaleString()}</span>
                                    {c.problem_index && <span className="ml-2 text-xs bg-gray-100 px-1.5 py-0.5 rounded font-mono">Problem {c.problem_index}</span>}
                                    {c.username && <span className="ml-2 text-xs text-gray-500">by {c.username}</span>}
                                </div>
                                {c.answered ? <span className="text-xs text-green-600 font-medium">Answered</span> : <span className="text-xs text-orange-500 font-medium">⏳ Pending</span>}
                            </div>
                            <p className="text-sm text-gray-800 mb-2">{c.question}</p>
                            {c.answer && <div className="bg-green-50 border border-green-200 rounded p-2 text-sm text-green-800 mt-2"><strong>Answer:</strong> {c.answer}</div>}
                            {!c.answered && (
                                <div className="mt-3 flex gap-2">
                                    <input value={answer[c.id] || ''} onChange={e => setAnswer(p => ({ ...p, [c.id]: e.target.value }))}
                                        placeholder="Type answer..." className="flex-1 border rounded-lg px-3 py-1.5 text-sm" />
                                    <label className="flex items-center gap-1 text-xs text-gray-500">
                                        <input type="checkbox" checked={publicAns[c.id] || false} onChange={e => setPublicAns(p => ({ ...p, [c.id]: e.target.checked }))} />
                                        Public
                                    </label>
                                    <button onClick={() => handleAnswer(c.id)} className="px-3 py-1.5 bg-green-600 text-white text-sm rounded-lg hover:bg-green-700">Reply</button>
                                </div>
                            )}
                        </div>
                    ))}
                </div>
            )}
        </TabShell>
    )
}

// ═══════════════════════════════════════════════
// PARTICIPANTS TAB
// ═══════════════════════════════════════════════
function ParticipantsTab({ contestId }: { contestId: string }) {
    const [data, setData] = useState<any>(null)
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        api.contests.listRegistrations(contestId).then(setData).finally(() => setLoading(false))
    }, [contestId])

    if (loading) return <TabLoading />
    const participants = data?.data || []

    return (
        <TabShell title="Participants" subtitle={`${data?.count ?? participants.length} registered`}>
            {participants.length === 0 ? <EmptyState icon={<Users className="w-5 h-5 text-gray-400" />} text="No participants yet" /> : (
                <table className="w-full text-sm">
                    <thead><tr className="border-b border-gray-200 dark:border-gray-700 text-left text-xs text-gray-500 uppercase">
                        <th className="py-2 pr-4">#</th><th className="py-2 pr-4">Username</th><th className="py-2">Registered</th>
                    </tr></thead>
                    <tbody className="divide-y divide-gray-100">
                        {participants.map((p: any, i: number) => (
                            <tr key={p.user_id} className="hover:bg-gray-50">
                                <td className="py-2 pr-4 text-gray-400">{i + 1}</td>
                                <td className="py-2 pr-4"><Link to={`/user/${p.username}`} className="text-blue-600 hover:underline">{p.username}</Link></td>
                                <td className="py-2 text-gray-500 text-xs">{new Date(p.registered_at).toLocaleString()}</td>
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
    const [items, setItems] = useState<any[]>([])
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        api.contests.submissions(contestId, 0, 100, { mine: false }).then(d => setItems(d.data || [])).finally(() => setLoading(false))
    }, [contestId])

    if (loading) return <TabLoading />

    const verdictColor = (s: string) => {
        if (s === 'ACCEPTED') return 'text-green-600 bg-green-50'
        if (s?.includes('PENDING') || s === 'JUDGING') return 'text-yellow-600 bg-yellow-50'
        return 'text-red-600 bg-red-50'
    }

    return (
        <TabShell title="Submissions" subtitle={`${items.length} total`}>
            {items.length === 0 ? <EmptyState icon={<FileCode className="w-5 h-5 text-gray-400" />} text="No submissions yet" /> : (
                <div className="overflow-x-auto">
                    <table className="w-full text-sm">
                        <thead><tr className="border-b border-gray-200 dark:border-gray-700 text-left text-xs text-gray-500 uppercase">
                            <th className="py-2 pr-3">ID</th><th className="py-2 pr-3">User</th><th className="py-2 pr-3">Problem</th>
                            <th className="py-2 pr-3">Lang</th><th className="py-2 pr-3">Verdict</th><th className="py-2 pr-3">Time</th><th className="py-2">When</th>
                        </tr></thead>
                        <tbody className="divide-y divide-gray-100">
                            {items.slice(0, 100).map((s: any) => (
                                <tr key={s.id} className="hover:bg-gray-50">
                                    <td className="py-2 pr-3"><Link to={`/submissions/${s.id}`} className="text-blue-600 hover:underline font-mono text-xs">{s.id?.slice(0, 8)}</Link></td>
                                    <td className="py-2 pr-3">{s.username || s.user_id?.slice(0, 8)}</td>
                                    <td className="py-2 pr-3 font-mono text-xs">{s.problem_index || s.problem_id?.slice(0, 8)}</td>
                                    <td className="py-2 pr-3 text-xs">{s.language}</td>
                                    <td className="py-2 pr-3"><span className={`text-xs px-2 py-0.5 rounded-full font-medium ${verdictColor(s.status)}`}>{s.status}</span></td>
                                    <td className="py-2 pr-3 text-xs text-gray-500">{s.time_ms ? `${s.time_ms}ms` : '—'}</td>
                                    <td className="py-2 text-xs text-gray-400">{s.created_at ? new Date(s.created_at).toLocaleString() : '—'}</td>
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
    const [items, setItems] = useState<any[]>([])
    const [loading, setLoading] = useState(true)

    const load = useCallback(() => {
        api.onsite.listPrints(contestId).then(d => setItems(d.data || [])).finally(() => setLoading(false))
    }, [contestId])
    useEffect(() => { load() }, [load])

    const updateStatus = async (printId: string, status: string) => {
        await api.onsite.updatePrintStatus(contestId, printId, status)
        load()
    }

    if (loading) return <TabLoading />

    const statusColor = (s: string) => {
        if (s === 'printed') return 'text-green-700 bg-green-100'
        if (s === 'cancelled') return 'text-red-700 bg-red-100'
        return 'text-yellow-700 bg-yellow-100'
    }

    return (
        <TabShell title="Print Requests" subtitle={`${items.length} requests`}>
            {items.length === 0 ? <EmptyState icon={<Printer className="w-5 h-5 text-gray-400" />} text="No print requests" /> : (
                <div className="space-y-3">
                    {items.map((p: any) => (
                        <div key={p.id} className="border border-gray-200 dark:border-gray-700 rounded-lg p-4">
                            <div className="flex items-center justify-between mb-2">
                                <div className="flex items-center gap-2">
                                    <span className="font-medium text-sm">{p.filename}</span>
                                    <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${statusColor(p.status)}`}>{p.status}</span>
                                </div>
                                <div className="flex gap-2">
                                    {p.status === 'pending' && (
                                        <>
                                            <button onClick={() => updateStatus(p.id, 'printed')} className="text-xs px-2 py-1 bg-green-600 text-white rounded hover:bg-green-700">Mark Printed</button>
                                            <button onClick={() => updateStatus(p.id, 'cancelled')} className="text-xs px-2 py-1 bg-red-100 text-red-700 rounded hover:bg-red-200">Cancel</button>
                                        </>
                                    )}
                                </div>
                            </div>
                            <pre className="bg-gray-50 border rounded p-3 text-xs font-mono overflow-x-auto max-h-40 overflow-y-auto">{p.content}</pre>
                            <div className="mt-2 text-xs text-gray-400">
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
                <a href={`/contests/${contestId}/scoreboard`} className="text-blue-600 hover:underline font-medium">
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
    const [stats, setStats] = useState<any>(null)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')

    useEffect(() => {
        api.contests.stats(contestId)
            .then(setStats)
            .catch(e => setError(e.message))
            .finally(() => setLoading(false))
    }, [contestId])

    if (loading) return <TabLoading />
    if (error) return <TabShell title="Statistics"><div className="text-red-500 text-sm p-4">{error}</div></TabShell>
    if (!stats) return null

    const languagesArray = stats.languages ? Object.entries(stats.languages).map(([language, count]) => ({ language, count: count as number })) : []
    const verdictsArray = stats.verdicts ? Object.entries(stats.verdicts).map(([status, count]) => ({ status, count: count as number })) : []

    return (
        <TabShell title="Statistics">
            {/* Summary Cards */}
            <div className="grid grid-cols-3 gap-4 mb-6">
                <StatCard label="Participants" value={stats.total_participants} icon={<Users className="w-5 h-5 text-gray-400" />} />
                <StatCard label="Submissions" value={stats.total_submissions} icon={<FileCode className="w-5 h-5 text-gray-400" />} />
                <StatCard label="Accepted" value={stats.accepted_submissions} icon={<Check className="w-5 h-5 text-green-500" />} />
            </div>

            {/* Per-problem solve rate */}
            {stats.problems && stats.problems.length > 0 && (
                <div className="mb-6">
                    <h3 className="text-sm font-semibold text-gray-700 mb-3">Problem Solve Rates</h3>
                    <div className="space-y-2">
                        {stats.problems.map((p: any) => (
                            <div key={p.problem_id} className="flex items-center gap-3">
                                <span className="font-mono font-bold text-blue-600 w-8">{p.index}</span>
                                <div className="flex-1 bg-gray-100 rounded-full h-5 overflow-hidden">
                                    <div className="bg-green-500 h-5 rounded-full flex items-center justify-end pr-2 text-xs text-white font-medium transition-all"
                                        style={{ width: `${Math.max(p.solve_rate, 2)}%` }}>
                                        {p.solve_rate > 10 && `${p.solve_rate}%`}
                                    </div>
                                </div>
                                <span className="text-xs text-gray-500 w-24 text-right">{p.accepted}/{p.total_submissions} solved</span>
                            </div>
                        ))}
                    </div>
                </div>
            )}

            {/* Language distribution */}
            {languagesArray.length > 0 && (
                <div className="mb-6">
                    <h3 className="text-sm font-semibold text-gray-700 mb-3">Languages</h3>
                    <div className="flex gap-2 flex-wrap">
                        {languagesArray.map((l: any) => (
                            <span key={l.language} className="text-xs bg-gray-100 px-3 py-1.5 rounded-full">
                                {l.language}: <strong>{l.count}</strong>
                            </span>
                        ))}
                    </div>
                </div>
            )}

            {/* Verdict distribution */}
            {verdictsArray.length > 0 && (
                <div>
                    <h3 className="text-sm font-semibold text-gray-700 mb-3">Verdicts</h3>
                    <div className="flex gap-2 flex-wrap">
                        {verdictsArray.map((v: any) => {
                            const color = v.status === 'ACCEPTED' ? 'bg-green-100 text-green-800' :
                                v.status?.includes('PENDING') ? 'bg-yellow-100 text-yellow-800' : 'bg-red-100 text-red-800'
                            return <span key={v.status} className={`text-xs px-3 py-1.5 rounded-full ${color}`}>{v.status}: <strong>{v.count}</strong></span>
                        })}
                    </div>
                </div>
            )}
        </TabShell>
    )
}

function StatCard({ label, value, icon }: { label: string; value: number | string; icon: React.ReactNode }) {
    return (
        <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl p-4 text-center">
            <div className="text-2xl mb-1">{icon}</div>
            <div className="text-2xl font-bold text-gray-800">{value}</div>
            <div className="text-xs text-gray-500 uppercase tracking-wider mt-1">{label}</div>
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
                <a href={`/contests/${contestId}/plagiarism`} className="text-blue-600 hover:underline font-medium">
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
    const [items, setItems] = useState<any[]>([])
    const [loading, setLoading] = useState(true)
    const [filter, setFilter] = useState<'all' | 'pending' | 'dispatched'>('pending')

    const load = useCallback(() => {
        api.onsite.listBalloons(contestId).then(d => setItems(d.data || [])).finally(() => setLoading(false))
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
                        className={`px-3 py-1.5 text-xs rounded-full font-medium capitalize ${filter === f ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200'}`}>
                        {f} {f === 'pending' ? `(${pending})` : ''}
                    </button>
                ))}
            </div>
            {filtered.length === 0 ? <EmptyState icon={<CircleDot className="w-5 h-5 text-gray-400" />} text="No balloons" /> : (
                <div className="space-y-2">
                    {filtered.map((b: any) => (
                        <div key={b.id} className={`flex items-center justify-between p-3 rounded-lg border ${b.dispatched ? 'bg-gray-50 border-gray-200 dark:border-gray-700' : 'bg-white dark:bg-gray-800 border-blue-200'}`}>
                            <div className="flex items-center gap-3">
                                <CircleDot className="w-5 h-5 text-blue-500" />
                                <div>
                                    <div className="flex items-center gap-2">
                                        <span className="font-mono font-bold text-blue-600">{b.problem_index}</span>
                                        <span className="text-sm font-medium">{b.username}</span>
                                        {b.color && <span className="text-xs px-2 py-0.5 rounded-full" style={{ backgroundColor: theme === 'dark' ? b.color + '40' : b.color + '20', color: b.color }}>{b.color}</span>}
                                    </div>
                                    <span className="text-xs text-gray-400">{new Date(b.created_at).toLocaleTimeString()}</span>
                                </div>
                            </div>
                            {!b.dispatched ? (
                                <button onClick={() => dispatch(b.id)} className="px-3 py-1.5 bg-green-600 text-white text-xs rounded-lg hover:bg-green-700 font-medium">
                                    Dispatch
                                </button>
                            ) : (
                                <span className="text-xs text-green-600 font-medium">Dispatched</span>
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
    const [items, setItems] = useState<any[]>([])
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
                <button onClick={create} className="px-4 py-2 bg-blue-600 text-white text-sm rounded-lg hover:bg-blue-700">Post</button>
            </div>
            {items.length === 0 ? <EmptyState icon={<Megaphone className="w-5 h-5 text-gray-400" />} text="No announcements" /> : (
                <div className="space-y-2">
                    {items.map((n: any) => (
                        <div key={n.id} className="flex items-start justify-between p-3 bg-yellow-50 border border-yellow-200 rounded-lg">
                            <div>
                                <p className="text-sm text-gray-800">{n.content}</p>
                                <span className="text-xs text-gray-400">{new Date(n.created_at).toLocaleString()}</span>
                            </div>
                            <button onClick={() => remove(n.id)} className="text-red-400 hover:text-red-600 text-xs ml-4">✕</button>
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
    const [items, setItems] = useState<any[]>([])
    const [loading, setLoading] = useState(true)
    const [userId, setUserId] = useState('')
    const [level, setLevel] = useState('judge')

    const load = useCallback(() => {
        api.permissions.list(contestId).then(d => setItems(d.data || [])).finally(() => setLoading(false))
    }, [contestId])
    useEffect(() => { load() }, [load])

    const add = async () => {
        if (!userId.trim()) return
        await api.permissions.add(contestId, { user_id: userId, access_level: level })
        setUserId('')
        load()
    }

    const remove = async (uid: string) => {
        await api.permissions.remove(contestId, uid)
        load()
    }

    if (loading) return <TabLoading />

    const levelColor = (l: string) => {
        if (l === 'manager') return 'bg-purple-100 text-purple-700'
        if (l === 'judge') return 'bg-blue-100 text-blue-700'
        return 'bg-gray-100 text-gray-700'
    }

    return (
        <TabShell title="Moderators">
            <div className="flex gap-2 mb-4">
                <input value={userId} onChange={e => setUserId(e.target.value)} placeholder="User ID..." className="flex-1 border rounded-lg px-3 py-2 text-sm" />
                <select value={level} onChange={e => setLevel(e.target.value)} className="border rounded-lg px-3 py-2 text-sm">
                    <option value="judge">Judge</option>
                    <option value="manager">Manager</option>
                    <option value="tester">Tester</option>
                </select>
                <button onClick={add} className="px-4 py-2 bg-blue-600 text-white text-sm rounded-lg hover:bg-blue-700">Add</button>
            </div>
            {items.length === 0 ? <EmptyState icon={<Shield className="w-5 h-5 text-gray-400" />} text="No moderators added" /> : (
                <div className="space-y-2">
                    {items.map((p: any) => (
                        <div key={p.user_id} className="flex items-center justify-between p-3 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg">
                            <div className="flex items-center gap-3">
                                <Link to={`/user/${p.username}`} className="font-medium text-sm text-gray-800 hover:text-blue-600">{p.username || p.user_id}</Link>
                                <span className={`text-xs px-2 py-0.5 rounded-full font-medium capitalize ${levelColor(p.access_level)}`}>{p.access_level}</span>
                            </div>
                            <button onClick={() => remove(p.user_id)} className="text-red-400 hover:text-red-600 text-xs">Remove</button>
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
    const [teams, setTeams] = useState<any[]>([])
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
        } catch (e: any) { alert(e.message) }
        finally { setGenerating(false) }
    }

    const copyAllCredentials = () => {
        const text = teams.map(t => `Team: ${t.team_name}${t.institution ? ' (' + t.institution + ')' : ''}\nUsername: ${t.username}\nPassword: ${t.password || '—'}\n`).join('\n---\n')
        navigator.clipboard.writeText(text).then(() => alert('Credentials copied to clipboard'))
    }

    if (loading) return <TabLoading />

    return (
        <TabShell title="Onsite Teams" subtitle={`${teams.length} generated`}>
            <div className="space-y-4 mb-6 p-4 bg-blue-50 border border-blue-200 rounded-lg">
                <h3 className="text-sm font-semibold text-blue-800">Generate Team Credentials</h3>
                <p className="text-xs text-blue-600">Enter one team per line: <code>TeamName, Institution</code></p>
                <textarea
                    value={teamInput}
                    onChange={e => setTeamInput(e.target.value)}
                    rows={5}
                    placeholder={"Team Alpha, MIT\nTeam Beta, Stanford\nTeam Gamma"}
                    className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm font-mono"
                />
                <button onClick={generateFromText} disabled={generating || !teamInput.trim()}
                    className="px-4 py-2 bg-blue-600 text-white text-sm rounded-lg hover:bg-blue-700 disabled:opacity-50">
                    {generating ? 'Generating...' : 'Generate Credentials'}
                </button>
            </div>

            {teams.length > 0 && (
                <>
                    <div className="flex justify-end mb-3">
                        <button onClick={copyAllCredentials}
                            className="px-3 py-1.5 bg-gray-100 text-gray-700 text-xs rounded-lg hover:bg-gray-200">
                            Copy All to Clipboard
                        </button>
                    </div>
                    <div className="overflow-x-auto">
                        <table className="w-full text-sm">
                            <thead><tr className="border-b border-gray-200 dark:border-gray-700 text-left text-xs text-gray-500 uppercase">
                                <th className="py-2 pr-3">Team Name</th>
                                <th className="py-2 pr-3">Institution</th>
                                <th className="py-2 pr-3">Username</th>
                                <th className="py-2 pr-3">Password</th>
                                <th className="py-2">Used</th>
                            </tr></thead>
                            <tbody className="divide-y divide-gray-100">
                                {teams.map((t: any) => (
                                    <tr key={t.id} className="hover:bg-gray-50">
                                        <td className="py-2 pr-3 font-medium">{t.team_name}</td>
                                        <td className="py-2 pr-3 text-gray-500">{t.institution || '—'}</td>
                                        <td className="py-2 pr-3 font-mono text-xs">{t.username}</td>
                                        <td className="py-2 pr-3 font-mono text-xs">{t.password || '—'}</td>
                                        <td className="py-2">
                                            {t.is_used ? (
                                                <span className="text-xs px-2 py-0.5 bg-green-100 text-green-700 rounded-full">Used</span>
                                            ) : (
                                                <span className="text-xs px-2 py-0.5 bg-yellow-100 text-yellow-700 rounded-full">New</span>
                                            )}
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                </>
            )}
            {teams.length === 0 && <EmptyState icon={<UserPlus className="w-5 h-5 text-gray-400" />} text="No team credentials generated yet" />}
        </TabShell>
    )
}

// ═══════════════════════════════════════════════
// SETTINGS TAB (full edit form)
// ═══════════════════════════════════════════════
function SettingsTab({ contestId }: { contestId: string }) {
    const [form, setForm] = useState({
        title: '', description: '', start_time: '', end_time: '', freeze_time: '',
        password: '', pdf_enabled: true, statement_hidden: false,
        upsolving_enabled: true, virtual_contest_enabled: true,
        visibility: 'public',
        group_id: '',
    })
    const [groups, setGroups] = useState<any[]>([])
    const [loading, setLoading] = useState(true)
    const [saving, setSaving] = useState(false)
    const [error, setError] = useState('')

    useEffect(() => {
        api.contests.get(contestId).then(d => {
            const c = d.contest
            let vis = 'public'
            if (c.group_id) {
                vis = 'group'
            } else if (c.password) {
                vis = 'private'
            }
            setForm({
                title: c.title || '',
                description: c.description || '',
                start_time: c.start_time ? new Date(c.start_time).toISOString().slice(0, 16) : '',
                end_time: c.end_time ? new Date(c.end_time).toISOString().slice(0, 16) : '',
                freeze_time: c.freeze_time ? new Date(c.freeze_time).toISOString().slice(0, 16) : '',
                password: c.password || '',
                pdf_enabled: c.pdf_enabled ?? true,
                statement_hidden: c.statement_hidden ?? false,
                upsolving_enabled: c.upsolving_enabled ?? true,
                virtual_contest_enabled: c.virtual_contest_enabled ?? true,
                visibility: vis,
                group_id: c.group_id || '',
            })
        }).finally(() => setLoading(false))

        api.groups.list(0, 100).then(res => {
            setGroups(res.data || [])
        }).catch(() => {})
    }, [contestId])

    const handleSave = async () => {
        if (!form.title.trim()) { setError('Title is required'); return }
        if (!form.start_time || !form.end_time) { setError('Start and end time required'); return }
        if (new Date(form.end_time) <= new Date(form.start_time)) { setError('End must be after start'); return }
        if (form.visibility === 'private' && !form.password.trim()) { setError('Password is required for Private contests'); return }
        if (form.visibility === 'group' && !form.group_id) { setError('Please select a group'); return }
        setError('')
        setSaving(true)
        try {
            await api.contests.update(contestId, {
                title: form.title,
                description: form.description || undefined,
                start_time: new Date(form.start_time).toISOString(),
                end_time: new Date(form.end_time).toISOString(),
                freeze_time: form.freeze_time ? new Date(form.freeze_time).toISOString() : undefined,
                password: form.visibility === 'private' ? form.password : '',
                group_id: form.visibility === 'group' ? form.group_id : '',
                pdf_enabled: form.pdf_enabled,
                statement_hidden: form.statement_hidden,
                upsolving_enabled: form.upsolving_enabled,
                virtual_contest_enabled: form.virtual_contest_enabled,
            })
        } catch (err: any) {
            setError(err.message || 'Failed to save')
        } finally {
            setSaving(false)
        }
    }

    if (loading) return <TabLoading />

    return (
        <TabShell title="Settings">
            {error && <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-2 rounded mb-4 text-sm">{error}</div>}

            <div className="space-y-5">
                {/* Title */}
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Title</label>
                    <input value={form.title} onChange={e => setForm(p => ({ ...p, title: e.target.value }))}
                        className="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500" />
                </div>

                {/* Description */}
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Description</label>
                    <textarea value={form.description} onChange={e => setForm(p => ({ ...p, description: e.target.value }))}
                        rows={3} className="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500" />
                </div>

                {/* Times */}
                <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-1">Start Time</label>
                        <input type="datetime-local" value={form.start_time} onChange={e => setForm(p => ({ ...p, start_time: e.target.value }))}
                            className="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500" />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-1">End Time</label>
                        <input type="datetime-local" value={form.end_time} onChange={e => setForm(p => ({ ...p, end_time: e.target.value }))}
                            className="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500" />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-1">Freeze Time <span className="text-gray-400">(optional)</span></label>
                        <input type="datetime-local" value={form.freeze_time} onChange={e => setForm(p => ({ ...p, freeze_time: e.target.value }))}
                            className="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500" />
                    </div>
                </div>

                {/* Visibility Settings */}
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Visibility Mode</label>
                    <select value={form.visibility} onChange={e => setForm(p => ({ ...p, visibility: e.target.value }))}
                        className="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500 bg-white">
                        <option value="public">Public (Anyone can join)</option>
                        <option value="private">Private (Password/Invite required)</option>
                        <option value="group">Specific Group</option>
                    </select>
                </div>

                {form.visibility === 'private' && (
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-1">Contest Password</label>
                        <input value={form.password} onChange={e => setForm(p => ({ ...p, password: e.target.value }))}
                            placeholder="Enter password for private access..."
                            className="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500" required />
                    </div>
                )}

                {form.visibility === 'group' && (
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-1">Select Restricted Group</label>
                        <select value={form.group_id} onChange={e => setForm(p => ({ ...p, group_id: e.target.value }))}
                            className="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500 bg-white">
                            <option value="">-- Choose Group --</option>
                            {groups.map(g => (
                                <option key={g.id} value={g.id}>{g.name}</option>
                            ))}
                        </select>
                    </div>
                )}

                {/* Options */}
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">Options</label>
                    <div className="grid grid-cols-2 gap-3">
                        {([
                            ['pdf_enabled', 'PDF Generation', FileText],
                            ['statement_hidden', 'Hide Statements (Onsite)', EyeOff],
                            ['upsolving_enabled', 'Enable Upsolving', Loader2],
                            ['virtual_contest_enabled', 'Virtual Contests', Play],
                        ] as const).map(([key, label, icon]) => (
                            <label key={key} className="flex items-center gap-2 cursor-pointer p-2 rounded-lg hover:bg-gray-50 border border-gray-100">
                                <input type="checkbox" checked={form[key] as boolean}
                                    onChange={e => setForm(p => ({ ...p, [key]: e.target.checked }))}
                                    className="rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500" />
                                <span className="text-sm flex items-center gap-1.5">{typeof icon === "string" ? icon : React.createElement(icon as any, { className: "w-4 h-4" })} {label}</span>
                            </label>
                        ))}
                    </div>
                </div>

                {/* Save */}
                <div className="flex gap-3 pt-2">
                    <button onClick={handleSave} disabled={saving}
                        className="px-6 py-2.5 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 font-medium text-sm transition-colors">
                        {saving ? 'Saving...' : 'Save Changes'}
                    </button>
                    <Link to={`/contests/${contestId}`} className="px-4 py-2.5 bg-gray-100 text-gray-600 rounded-lg hover:bg-gray-200 text-sm">
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
        <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl overflow-hidden">
            <div className="px-5 py-3.5 border-b border-gray-200 dark:border-gray-700 bg-gray-50 flex items-center justify-between">
                <div>
                    <h2 className="font-semibold text-gray-800">{title}</h2>
                    {subtitle && <p className="text-xs text-gray-400 mt-0.5">{subtitle}</p>}
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
            <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-600" />
        </div>
    )
}

function EmptyState({ icon, text }: { icon: React.ReactNode; text: string }) {
    return (
        <div className="text-center py-10">
            <div className="text-4xl mb-2">{icon}</div>
            <p className="text-gray-400 text-sm">{text}</p>
        </div>
    )
}