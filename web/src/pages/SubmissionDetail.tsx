import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { ExternalLink, RefreshCw } from 'lucide-react'
import { api } from '../lib/api'
import { useDocumentTitle } from '../lib/documentTitle'
import { resolveProblemSlug, resolveProblemTitle } from '../lib/problemSlugResolver'
import { languageName } from '../lib/languages'
import { useToast } from '../components/Toast'
import { Badge } from '../components/ui/Badge'
import { Button } from '../components/ui/Button'
import { Card, CardHeader } from '../components/ui/Card'
import { Skeleton } from '../components/ui/PageHeader'
import { VerdictBadge } from '../components/ui/VerdictBadge'
import { isPendingVerdict, verdictMeta } from '../components/ui/verdict'

function groupBySubtask(results: JudgeCase[]): Map<number, JudgeCase[]> {
    const groups = new Map<number, JudgeCase[]>()
    for (const r of results) {
        const id = r.subtask_id ?? 0
        if (!groups.has(id)) groups.set(id, [])
        groups.get(id)!.push(r)
    }
    return groups
}

function formatMemory(kb?: number | null): string {
    if (kb === undefined || kb === null) return '—'
    return kb >= 1024 ? `${(kb / 1024).toFixed(0)} MB` : `${kb} KB`
}

function formatTime(ms?: number | null): string {
    if (ms === undefined || ms === null) return '—'
    return ms < 1000 ? `${ms} ms` : `${(ms / 1000).toFixed(2)} s`
}

interface JudgeCase { status: string; case_name?: string; subtask_id?: number; time?: number; memory?: number; score?: number; detail?: string }
interface SubmissionRecord {
    id: string; problem_id: string; language: string; status: string;
    score?: number; time_used: number; memory_used: number; code_size?: number;
    created_at: string; source_code?: string; compile_output?: string;
    is_remote?: boolean; remote_id?: string; remote_url?: string;
    judge_result?: JudgeCase[];
}

export default function SubmissionDetail() {
    const toast = useToast()
    const { id } = useParams<{ id: string }>()
    const [sub, setSub] = useState<SubmissionRecord | null>(null)
    const [loading, setLoading] = useState(true)
    const [problemSlug, setProblemSlug] = useState<string | null>(null)
    const [problemTitle, setProblemTitle] = useState<string | null>(null)
    // Prefer the problem name (what a reader recognises) over the submission id.
    useDocumentTitle(problemTitle)
    const [activeTab, setActiveTab] = useState<'overview' | 'code' | 'tests'>('overview')
    const [actionLoading, setActionLoading] = useState(false)
    const [pollTrigger, setPollTrigger] = useState(0)

    useEffect(() => {
        if (!id) return
        let active = true
        const poll = async () => {
            try {
                const d = await api.submissions.get(id)
                if (!active) return
                setSub(d)
                setLoading(false)
                if (d.problem_id) {
                    resolveProblemSlug(d.problem_id).then(setProblemSlug)
                    resolveProblemTitle(d.problem_id).then(setProblemTitle)
                }
                if (d.status === 'pending' || d.status === 'judging') {
                    setTimeout(() => {
                        if (active) poll()
                    }, 1500)
                }
            } catch {
                if (active) setLoading(false)
            }
        }
        poll()
        return () => {
            active = false
        }
    }, [id, pollTrigger])

    const handleRetryRemote = async () => {
        if (!id) return
        setActionLoading(true)
        try {
            await api.submissions.retryRemote(id)
            setPollTrigger(prev => prev + 1)
        } catch (err) {
            toast.error('Failed to retry: ' + (err instanceof Error ? err.message : String(err)))
        } finally {
            setActionLoading(false)
        }
    }

    const handleRecheckRemote = async () => {
        if (!id) return
        setActionLoading(true)
        try {
            await api.submissions.recheckRemote(id)
            setPollTrigger(prev => prev + 1)
        } catch (err) {
            toast.error('Failed to recheck: ' + (err instanceof Error ? err.message : String(err)))
        } finally {
            setActionLoading(false)
        }
    }

    if (loading) {
        return (
            <div className="mx-auto w-full max-w-4xl space-y-4">
                <Skeleton className="h-8 w-56" />
                <Skeleton className="h-4 w-80" />
                <Skeleton className="h-64 w-full" />
            </div>
        )
    }

    if (!sub) {
        return (
            <div className="py-20 text-center">
                <p className="text-sm text-muted">Submission not found.</p>
                <Link to="/submissions" className="mt-2 inline-block text-sm text-brand hover:underline">
                    Back to my submissions
                </Link>
            </div>
        )
    }

    const judgeResult = sub.judge_result || []
    const totalCases = judgeResult.length
    const passedCases = judgeResult.filter(r => r.status === 'ac').length
    const pending = isPendingVerdict(sub.status)
    const meta = verdictMeta(sub.status)

    const tabs = [
        { key: 'overview' as const, label: 'Overview' },
        { key: 'code' as const, label: 'Source' },
        { key: 'tests' as const, label: totalCases > 0 ? `Test cases (${passedCases}/${totalCases})` : 'Test cases' },
    ]

    return (
        <div className="mx-auto w-full max-w-4xl space-y-5">
            {/* ── Header ── */}
            <header>
                <nav aria-label="Breadcrumb" className="mb-2 flex items-center gap-1.5 text-xs text-muted">
                    <Link to="/submissions" className="hover:text-ink">My submissions</Link>
                    <span aria-hidden="true" className="text-hairline-strong">/</span>
                    <span className="tnum font-mono text-muted-soft">{sub.id.slice(0, 8)}</span>
                </nav>

                <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                    <div className="min-w-0">
                        <h1 className="flex flex-wrap items-center gap-2.5 text-2xl font-semibold tracking-tight text-ink">
                            Submission
                            <span className="tnum font-mono text-lg text-muted">{sub.id.slice(0, 8)}</span>
                        </h1>
                        <p className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-muted">
                            {problemSlug ? (
                                <Link to={`/problems/${problemSlug}`} className="font-medium text-brand hover:underline">
                                    {problemTitle || `Problem ${sub.problem_id?.slice(0, 8)}`}
                                </Link>
                            ) : (
                                <span>{problemTitle || `Problem ${sub.problem_id?.slice(0, 8)}`}</span>
                            )}
                            <span aria-hidden="true">·</span>
                            <span>{languageName(sub.language)}</span>
                            <span aria-hidden="true">·</span>
                            <time dateTime={sub.created_at}>{new Date(sub.created_at).toLocaleString()}</time>
                        </p>
                        {sub.remote_id && (
                            <p className="mt-2 flex items-center gap-1.5 text-xs text-muted">
                                <span className="font-medium text-ink-soft">Remote OJ id</span>
                                {sub.remote_url ? (
                                    <a
                                        href={sub.remote_url}
                                        target="_blank"
                                        rel="noopener noreferrer"
                                        className="inline-flex items-center gap-1 font-mono text-brand hover:underline"
                                    >
                                        {sub.remote_id}
                                        <ExternalLink className="h-3 w-3" />
                                    </a>
                                ) : (
                                    <span className="font-mono text-ink-soft">{sub.remote_id}</span>
                                )}
                            </p>
                        )}
                    </div>

                    <div className="flex shrink-0 flex-col items-start gap-2 sm:items-end">
                        <VerdictBadge status={sub.status} className="px-3 py-1 text-xs" />
                        {sub.is_remote && (
                            <div className="flex flex-wrap gap-2">
                                {sub.status === 'se' && (
                                    <Button size="sm" onClick={handleRetryRemote} disabled={actionLoading}>
                                        <RefreshCw className="h-3.5 w-3.5" />
                                        Retry
                                    </Button>
                                )}
                                {sub.remote_id && (
                                    <Button variant="secondary" size="sm" onClick={handleRecheckRemote} disabled={actionLoading}>
                                        Recheck status
                                    </Button>
                                )}
                            </div>
                        )}
                    </div>
                </div>
            </header>

            {sub.status === 'ce' && sub.compile_output && (
                <Card className="border-bad/30">
                    <CardHeader title="Compile error" description="The compiler rejected this source." />
                    <pre className="max-h-64 overflow-auto bg-code px-5 py-4 font-mono text-xs leading-relaxed text-code-ink whitespace-pre-wrap">
                        {sub.compile_output}
                    </pre>
                </Card>
            )}

            {/* ── Tabs ── */}
            <div role="tablist" aria-label="Submission detail" className="flex gap-1 border-b border-hairline">
                {tabs.map(t => (
                    <button
                        key={t.key}
                        type="button"
                        role="tab"
                        aria-selected={activeTab === t.key}
                        onClick={() => setActiveTab(t.key)}
                        className={`-mb-px border-b-2 px-3 py-2 text-sm font-medium transition-colors ${activeTab === t.key
                            ? 'border-brand text-brand'
                            : 'border-transparent text-muted hover:text-ink'
                            }`}
                    >
                        {t.label}
                    </button>
                ))}
            </div>

            {activeTab === 'overview' && (
                <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
                    <Stat label="Time" value={pending || sub.status === 'ce' || sub.status === 'se' ? '—' : formatTime(sub.time_used)} />
                    <Stat label="Memory" value={pending || sub.status === 'ce' || sub.status === 'se' ? '—' : formatMemory(sub.memory_used)} />
                    <Stat label="Score" value={sub.score ?? '—'} />
                    <Stat label="Code size" value={sub.code_size ? `${sub.code_size.toLocaleString()} B` : '—'} />
                </div>
            )}

            {activeTab === 'code' && (
                <div className="overflow-hidden rounded-xl border border-code-border">
                    <div className="flex items-center justify-between border-b border-code-border bg-code px-4 py-2">
                        <span className="text-xs font-medium text-code-ink/70">
                            {languageName(sub.language)}
                        </span>
                        {sub.code_size ? (
                            <span className="tnum text-[11px] text-code-ink/50">{sub.code_size.toLocaleString()} bytes</span>
                        ) : null}
                    </div>
                    <pre className="max-h-[32rem] overflow-auto bg-code px-4 py-3 font-mono text-xs leading-relaxed text-code-ink">
                        {sub.source_code || 'Source code not available.'}
                    </pre>
                </div>
            )}

            {activeTab === 'tests' && (
                <TestCases judgeResult={judgeResult} status={sub.status} />
            )}

            {meta.tone === 'neutral' && sub.status === 'se' && (
                <p className="text-xs text-muted">
                    A system error means the judge itself failed, not your solution. Re-submitting usually resolves it.
                </p>
            )}
        </div>
    )
}

function Stat({ label, value }: { label: string; value: string | number }) {
    return (
        <div className="rounded-xl border border-hairline bg-surface px-4 py-3">
            <p className="text-[11px] font-medium uppercase tracking-wide text-muted">{label}</p>
            <p className="tnum mt-1 text-lg font-semibold text-ink">{value}</p>
        </div>
    )
}

function TestCases({ judgeResult, status }: { judgeResult: JudgeCase[]; status: string }) {
    if (judgeResult.length === 0) {
        return (
            <div className="rounded-xl border border-hairline bg-surface px-4 py-10 text-center">
                <p className="text-sm text-muted">
                    {isPendingVerdict(status)
                        ? 'Waiting for judging results…'
                        : 'No test case details available.'}
                </p>
            </div>
        )
    }

    const hasSubtasks = judgeResult.some(r => (r.subtask_id ?? 0) > 0)

    if (hasSubtasks) {
        const subtaskMap = groupBySubtask(judgeResult)
        const subtaskIds = [...subtaskMap.keys()].filter(id => id > 0).sort((a, b) => a - b)

        return (
            <div className="space-y-3">
                {subtaskIds.map(id => {
                    const cases = subtaskMap.get(id)!
                    const allPassed = cases.every(c => c.status === 'ac')
                    const earnedScore = cases.reduce((s, c) => s + (c.status === 'ac' ? (c.score ?? 0) : 0), 0)
                    const maxScore = cases.reduce((s, c) => s + (c.score ?? 0), 0)

                    return (
                        <Card key={id}>
                            <CardHeader
                                title={`Subtask ${id}`}
                                action={
                                    <div className="flex items-center gap-2">
                                        <span className="tnum text-xs text-muted">{earnedScore}/{maxScore} pts</span>
                                        <Badge tone={allPassed ? 'ok' : 'bad'}>{allPassed ? 'Passed' : 'Failed'}</Badge>
                                    </div>
                                }
                            />
                            <ul className="divide-y divide-hairline">
                                {cases.map((c, i) => (
                                    <CaseRow key={i} testCase={c} label={c.case_name || `Case ${i + 1}`} />
                                ))}
                            </ul>
                        </Card>
                    )
                })}
            </div>
        )
    }

    return (
        <div className="overflow-hidden rounded-xl border border-hairline bg-surface">
            <ul className="divide-y divide-hairline">
                {judgeResult.map((r, i) => (
                    <CaseRow key={i} testCase={r} label={r.case_name || `Test case ${i + 1}`} showScore />
                ))}
            </ul>
        </div>
    )
}

function CaseRow({ testCase, label, showScore = false }: { testCase: JudgeCase; label: string; showScore?: boolean }) {
    return (
        <li className="px-4 py-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="text-sm font-medium text-ink-soft">{label}</span>
                <VerdictBadge status={testCase.status} />
            </div>
            <div className="tnum mt-1 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted">
                <span>Time {formatTime(testCase.time)}</span>
                <span>Memory {formatMemory(testCase.memory)}</span>
                {showScore && <span>Score {testCase.score ?? 0}</span>}
            </div>
            {testCase.detail && (
                <p className="mt-2 rounded-lg border border-bad/25 bg-bad/10 px-2.5 py-1.5 font-mono text-xs leading-relaxed text-bad">
                    {testCase.detail}
                </p>
            )}
        </li>
    )
}
