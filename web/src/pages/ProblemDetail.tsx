import { useEffect, useState, useRef } from 'react'
import { useParams, useSearchParams, Link } from 'react-router-dom'
import { ProblemStatement } from '../components/ProblemStatement'
import 'katex/dist/katex.min.css'
import { api, getAccessToken } from '../lib/api'
import ProblemStats from '../components/ProblemStats'
import { lazy } from 'react'
const CodeEditor = lazy(() => import('../components/CodeEditor'))
import AddEditorialModal from '../components/AddEditorialModal'
import CommentSection from '../components/CommentSection'
import { EmptyState } from '../components/EmptyState'
import { SubmissionStatusPanel, type SubmissionStatusData } from '../components/SubmissionStatusPanel'

import { useTranslation } from 'react-i18next'
import { useToast } from '../components/Toast'
import { Badge } from '../components/ui/Badge'
import { ProblemMeta } from '../components/ProblemMeta'
import { DEFAULT_LANGUAGE, LANGUAGE_OPTIONS, languageName, languageTemplate } from '../lib/languages'
import { isRemotePlatform } from '../lib/platforms'
import { Skeleton } from '../components/ui/PageHeader'
import { VerdictBadge } from '../components/ui/VerdictBadge'
import { useDocumentTitle } from '../lib/documentTitle'

function useIsDesktop(breakpoint = 768) {
    const [isDesktop, setIsDesktop] = useState(
        () => typeof window !== 'undefined' ? window.innerWidth >= breakpoint : true
    )
    useEffect(() => {
        const mql = window.matchMedia(`(min-width: ${breakpoint}px)`)
        const handler = (e: MediaQueryListEvent) => setIsDesktop(e.matches)
        mql.addEventListener('change', handler)
        return () => mql.removeEventListener('change', handler)
    }, [breakpoint])
    return isDesktop
}

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

interface ProblemTranslation { language: string; title?: string | null; description?: string | null }
interface SampleCase { input: string; output: string; explanation?: string }
interface ProblemData {
    id: string; slug: string; title?: string; description?: string; hint?: string;
    source?: string; difficulty?: string; rating?: number; problem_type?: string; scoring_mode?: string;
    interactive?: boolean; time_limit: number; memory_limit: number;
    input_format?: string; output_format?: string; constraints?: string; pdf?: string;
    tags?: string[]; sample_cases?: SampleCase[];
}
interface RunResult { status: string; stdout?: string; stderr?: string; compile_output?: string; expected?: string; passed?: boolean | null; time_used: number; memory_used: number }
interface SampleRunResult { index: number; input: string; expected?: string; actual?: string; passed: boolean; status?: string; time: number; memory?: number; stderr?: string; compile_output?: string; error?: string }
interface EditorialItem { id: string; title: string; is_official?: boolean; username?: string; time_complexity?: string; upvotes?: number }
interface MySubmission { id: string; language?: string; status: string; time_used: number; memory_used: number }
interface RemoteLangOption { enabled?: boolean; sort_order?: number; local_id: string; display_name: string }

export default function ProblemDetail() {
    const toast = useToast()
    const { slug } = useParams<{ slug: string }>()
    const [searchParams] = useSearchParams()
    const isUpsolving = searchParams.get('upsolving') === 'true'
    const contestId = searchParams.get('contest')
    const [problem, setProblem] = useState<ProblemData | null>(null)
    // The slug whose load failed outright (a 404 for a problem that is live in a
    // contest, or a deleted one), so the page can say so instead of showing a
    // skeleton that never resolves. Stored as a slug rather than a boolean so
    // navigating to another problem needs no reset.
    const [failedSlug, setFailedSlug] = useState<string | null>(null)
    const { i18n } = useTranslation()
    // Translation for the active UI language, if the setter supplied one.
    const [translation, setTranslation] = useState<ProblemTranslation | null>(null)
    // Tab title carries the problem name once it loads; until then the route
    // map's "Problem" stays, rather than blanking the tab mid-fetch.
    useDocumentTitle(problem?.title)

    // The description to actually show: the translation when the setter supplied
    // one for the active language, otherwise the problem's default. Centralised
    // so the PDF-vs-markdown branch and the markdown body can't disagree.
    const effectiveDescription = translation?.description || problem?.description
    const [lang, setLang] = useState(DEFAULT_LANGUAGE)
    const [languages, setLanguages] = useState(LANGUAGE_OPTIONS)
    const [code, setCode] = useState('')
    const [result, setResult] = useState<SubmissionStatusData | null>(null)
    // True while the poller is still chasing a verdict for the submission above.
    const [pollingResult, setPollingResult] = useState(false)
    const [submitting, setSubmitting] = useState(false)
    const [customInput, setCustomInput] = useState('')
    const [customOutput, setCustomOutput] = useState<RunResult | null>(null)
    const [runningCustom, setRunningCustom] = useState(false)
    const [sampleResults, setSampleResults] = useState<SampleRunResult[]>([])
    const [runningSamples, setRunningSamples] = useState(false)
    const [lastSubmitTime, setLastSubmitTime] = useState(0)
    const [lastTestTime, setLastTestTime] = useState(0)
    const [tab, setTab] = useState<'statement' | 'stats' | 'editorials' | 'submissions' | 'more'>('statement')
    const [mySubs, setMySubs] = useState<MySubmission[]>([])
    const [loadingSubs, setLoadingSubs] = useState(false)
    const [editorials, setEditorials] = useState<EditorialItem[]>([])
    const [isAddModalOpen, setIsAddModalOpen] = useState(false)
    const isMountedRef = useRef(true)
    const isDesktop = useIsDesktop()
    const [mobileTab, setMobileTab] = useState<'problem' | 'code'>('problem')

    const [splitPos, setSplitPos] = useState(() => {
        const saved = localStorage.getItem('aioj_split_pos')
        return saved ? parseFloat(saved) : 50
    })
    const [dragging, setDragging] = useState(false)
    const containerRef = useRef<HTMLDivElement>(null)

    const handleMouseDown = (e: React.MouseEvent) => {
        e.preventDefault()
        setDragging(true)
    }

    useEffect(() => {
        if (!dragging) return
        const handleMouseMove = (e: MouseEvent) => {
            if (!containerRef.current) return
            const rect = containerRef.current.getBoundingClientRect()
            const pct = ((e.clientX - rect.left) / rect.width) * 100
            const clamped = Math.min(Math.max(pct, 20), 80)
            setSplitPos(clamped)
        }
        const handleMouseUp = () => {
            setDragging(false)
            localStorage.setItem('aioj_split_pos', splitPos.toString())
        }
        window.addEventListener('mousemove', handleMouseMove)
        window.addEventListener('mouseup', handleMouseUp)
        return () => {
            window.removeEventListener('mousemove', handleMouseMove)
            window.removeEventListener('mouseup', handleMouseUp)
        }
    }, [dragging, splitPos])

    useEffect(() => {
        isMountedRef.current = true
        return () => {
            isMountedRef.current = false
        }
    }, [])

    useEffect(() => {
        if (slug) {
            api.problems.get(slug).then(p => {
                setProblem(p)
                // local/default problems render the built-in language list; previously
                // this reset was a synchronous setLanguages in the languages effect
                if (!isRemotePlatform(p?.source)) setLanguages(LANGUAGE_OPTIONS)
            }).catch(() => {
                // A problem that is live in a contest answers 404 here: it is not
                // part of the archive until the round ends. Swallowing the error
                // left the skeleton spinning forever.
                if (isMountedRef.current) setFailedSlug(slug)
            })
        }
    }, [slug])

    // Fetch the translation for the active UI language and prefer it over the
    // problem's default title/description. Mirrors the backend's Coalesce
    // fallback: an absent translation simply leaves the defaults in place.
    useEffect(() => {
        if (!problem?.id) return
        let cancelled = false
        api.problems.listI18n(problem.id)
            .then((rows: ProblemTranslation[] | null) => {
                if (cancelled) return
                const match = (rows || []).find(r => r.language === i18n.language)
                setTranslation(match || null)
            })
            .catch(() => { if (!cancelled) setTranslation(null) })
        return () => { cancelled = true }
    }, [problem?.id, i18n.language])

    // Switching language re-runs the effect above (it depends on i18n.language),
    // so no separate languageChanged listener is needed.

    useEffect(() => {
        // Read the source once so the dependency below matches exactly what
        // this effect consumes (a statement edit must not refetch the list).
        const remoteSource = problem?.source
        if (isRemotePlatform(remoteSource)) {
            api.remoteLanguages.list(remoteSource)
                .then(d => {
                    const langs = (d.data || [])
                        .filter((rl: RemoteLangOption) => rl.enabled)
                        .sort((a: RemoteLangOption, b: RemoteLangOption) => (a.sort_order ?? 0) - (b.sort_order ?? 0))
                        .map((rl: RemoteLangOption) => ({ value: rl.local_id, label: rl.display_name }))
                    if (langs.length > 0) {
                        setLanguages(langs)
                        setLang(langs[0].value)
                    }
                })
                .catch(() => { })
        }
    }, [problem?.source])

    useEffect(() => {
        if (problem?.id) api.editorials.getByProblem(problem.id).then(d => setEditorials(d.data || [])).catch(() => { })
    }, [problem?.id])

    useEffect(() => {
        if (tab === 'submissions' && problem?.id) {
            // deferred a microtask: spinner still lands pre-paint, but the setState is
            // no longer synchronous (react-hooks/set-state-in-effect)
            queueMicrotask(() => setLoadingSubs(true))
            api.submissions.list(0, 50, problem.id, contestId || undefined)
                .then(d => {
                    if (isMountedRef.current) {
                        setMySubs(d.data || [])
                    }
                })
                .catch(console.error)
                .finally(() => {
                    if (isMountedRef.current) {
                        setLoadingSubs(false)
                    }
                })
        }
    }, [tab, problem?.id, contestId])

    useEffect(() => {
        if (slug) {
            const key = `aioj_draft_${problem?.id || slug}_${lang}`
            const saved = localStorage.getItem(key)
            // deferred a microtask: draft still lands pre-paint (react-hooks/set-state-in-effect)
            queueMicrotask(() => setCode(saved || languageTemplate(lang)))
        }
    }, [slug, problem?.id, lang])

    const handleCodeChange = (newCode: string) => {
        setCode(newCode)
        if (slug) {
            const key = `aioj_draft_${problem?.id || slug}_${lang}`
            localStorage.setItem(key, newCode)
        }
    }

    const runCustomCode = async () => {
        if (!code.trim()) { toast.info('Please write some code'); return }
        setRunningCustom(true)
        setCustomOutput(null)
        try {
            const res = await api.submissions.run({
                source_code: code,
                language: lang,
                input: customInput,
            })
            if (isMountedRef.current) {
                setCustomOutput(res)
            }
        } catch (e) {
            toast.error('Custom run failed: ' + (e instanceof Error ? e.message : String(e)))
        } finally {
            if (isMountedRef.current) {
                setRunningCustom(false)
            }
        }
    }

    const testWithSamples = async () => {
        if (!getAccessToken()) { toast.info('Please login first'); return }
        if (!code.trim()) { toast.info('Please write some code'); return }
        if (!problem?.sample_cases?.length) { toast.info('No sample cases available'); return }
        const now = Date.now()
        if (now - lastTestTime < 5000) { toast.info('Please wait 5 seconds between Test Samples runs'); return }
        setLastTestTime(now)
        setRunningSamples(true)
        setSampleResults([])
        const results: SampleRunResult[] = []
        for (let i = 0; i < problem.sample_cases.length; i++) {
            const sc = problem.sample_cases[i]
            try {
                const res = await api.submissions.run({
                    source_code: code,
                    language: lang,
                    input: sc.input,
                    expected: sc.output,
                })
                const actual = (res.stdout || '').trim().replace(/\r\n/g, '\n')
                const expected = (res.expected || sc.output || '').trim().replace(/\r\n/g, '\n')
                const passed = res.passed ?? false
                results.push({
                    index: i + 1,
                    input: sc.input,
                    expected,
                    actual,
                    passed,
                    status: res.status,
                    time: res.time_used,
                    memory: res.memory_used,
                    stderr: res.stderr,
                    compile_output: res.compile_output,
                })
            } catch (e) {
                results.push({
                    index: i + 1,
                    input: sc.input,
                    expected: sc.output,
                    actual: '',
                    passed: false,
                    status: 'error',
                    time: 0,
                    error: e instanceof Error ? e.message : String(e),
                })
            }
            if (isMountedRef.current) {
                setSampleResults([...results])
            }
        }
        if (isMountedRef.current) {
            setRunningSamples(false)
        }
    }

    const submit = async () => {
        if (!getAccessToken()) { toast.info('Please login first'); return }
        if (!code.trim()) {
            toast.info(problem?.problem_type === 'output_only'
                ? 'Paste your answer output first'
                : 'Please write some code')
            return
        }
        if (!problem) return
        const now = Date.now()
        if (now - lastSubmitTime < 5000) { toast.info('Please wait 5 seconds between submissions'); return }
        setLastSubmitTime(now)
        setSubmitting(true)
        setResult(null)
        setPollingResult(false)
        try {
            const apiCall = isUpsolving
                ? api.submissions.createUpsolving({
                    problem_id: problem.id,
                    language: lang,
                    source_code: code,
                    contest_id: contestId || undefined,
                })
                : api.submissions.create({
                    problem_id: problem.id,
                    language: lang,
                    source_code: code,
                })
            const res = await apiCall
            if (!isMountedRef.current) return

            // The POST resolves as soon as the row exists (status `pending`), so
            // surface the id and verdict right now instead of waiting for the
            // first poll — the user needs to know which submission they made.
            setResult(res)
            setPollingResult(true)
            setMySubs(prev => [res, ...prev.filter(s => s.id !== res.id)])
            toast.success(`Submitted #${res.id.substring(0, 8)} — judging…`)

            let retries = 0
            const maxRetries = 60
            const poll = async () => {
                if (!isMountedRef.current) return
                await new Promise(r => setTimeout(r, 2000))
                if (!isMountedRef.current) return

                try {
                    const updated = await api.submissions.get(res.id)
                    if (!isMountedRef.current) return
                    setResult(updated)
                    // Keep the list row in step with the live verdict.
                    setMySubs(prev => prev.map(s => s.id === updated.id
                        ? { ...s, status: updated.status, time_used: updated.time_used, memory_used: updated.memory_used }
                        : s))
                    if ((updated.status === 'pending' || updated.status === 'judging') && retries < maxRetries) {
                        retries++
                        poll()
                    } else {
                        setPollingResult(false)
                    }
                } catch {
                    if ((retries < maxRetries) && isMountedRef.current) {
                        retries++
                        poll()
                    } else if (isMountedRef.current) {
                        setPollingResult(false)
                    }
                }
            }
            poll()
        } catch (e) {
            toast.error('Submit failed: ' + (e instanceof Error ? e.message : String(e)))
        } finally {
            if (isMountedRef.current) {
                setSubmitting(false)
            }
        }
    }

    const role = decodeRole()
    const canExport = role === 'admin' || role === 'setter'

    if (!problem && failedSlug === slug) {
        return (
            <EmptyState
                text="Problem not found"
                description="This problem is not in the archive. Problems that are part of a contest appear here once that contest has ended."
            />
        )
    }

    if (!problem) {
        // A skeleton that matches the real layout, so the page does not jump
        // when the statement lands.
        return (
            <div className="space-y-4 py-6">
                <Skeleton className="h-7 w-64" />
                <Skeleton className="h-4 w-96 max-w-full" />
                <Skeleton className="h-64 w-full" />
            </div>
        )
    }

    return (
        <div ref={containerRef} className="flex flex-col h-full select-none md:flex-row" style={{ cursor: dragging ? 'col-resize' : undefined }}>
            {/* Mobile tab bar */}
            <div className="flex md:hidden border-b border-hairline bg-surface flex-shrink-0">
                <button
                    onClick={() => setMobileTab('problem')}
                    className={`flex-1 px-4 py-2.5 text-sm font-medium border-b-2 transition-colors ${mobileTab === 'problem'
                        ? 'border-brand text-brand'
                        : 'border-transparent text-muted hover:text-ink'
                        }`}
                >
                    Problem
                </button>
                <button
                    onClick={() => setMobileTab('code')}
                    className={`flex-1 px-4 py-2.5 text-sm font-medium border-b-2 transition-colors ${mobileTab === 'code'
                        ? 'border-brand text-brand'
                        : 'border-transparent text-muted hover:text-ink'
                        }`}
                >
                    Code
                </button>
            </div>

            {/* Problem Statement */}
            <div
                className={`space-y-4 overflow-y-auto pr-1 ${mobileTab === 'problem' ? '' : 'hidden'} md:block`}
                style={isDesktop ? { width: `${splitPos}%`, minWidth: '20%' } : { flex: 1 }}
            >
                <div>
                    <h1 className="flex flex-wrap items-center gap-2 text-xl font-semibold tracking-tight text-ink sm:text-2xl">
                        {translation?.title || problem.title}
                        {translation?.title && (
                            <Badge tone="ok" label={`Translated to ${i18n.language}`}>
                                {i18n.language.toUpperCase()}
                            </Badge>
                        )}
                        {problem.problem_type === 'output_only' && <Badge tone="warn">Output-only</Badge>}
                        {problem.scoring_mode === 'partial' && <Badge tone="brand">Partial scoring</Badge>}
                    </h1>
                    {/* Limits are the first thing a competitor looks for, so they
                        get a fixed-order strip shared with the contest page. */}
                    <ProblemMeta
                        className="mt-3 border-b border-hairline pb-3"
                        timeLimit={problem.time_limit}
                        memoryLimit={problem.memory_limit}
                        difficulty={problem.difficulty}
                        rating={problem.rating}
                        tags={problem.tags}
                        interactive={problem.interactive}
                    />
                    {canExport && (
                        <div className="mt-2">
                            <button
                                onClick={async () => {
                                    try {
                                        const resp = await fetch(api.problems.exportProblemUrl(problem.slug), {
                                            headers: { 'Authorization': `Bearer ${getAccessToken()}` }
                                        })
                                        if (!resp.ok) throw new Error('Export failed')
                                        const blob = await resp.blob()
                                        const url = URL.createObjectURL(blob)
                                        const a = document.createElement('a')
                                        a.href = url
                                        a.download = `${problem.slug}.zip`
                                        a.click()
                                        URL.revokeObjectURL(url)
                                    } catch (e) {
                                        toast.error('Export failed: ' + (e instanceof Error ? e.message : String(e)))
                                    }
                                }}
                                className="inline-flex items-center px-4 py-2 border border-hairline-strong rounded-md shadow-sm text-sm font-medium text-ink-soft bg-surface hover:bg-surface-2 cursor-pointer"
                            >
                                Export (FPS ZIP)
                            </button>
                        </div>
                    )}
                </div>

                <div
                    role="tablist"
                    aria-label="Problem detail"
                    className="flex shrink-0 overflow-x-auto border-b border-hairline [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]"
                >
                    {([
                        { key: 'statement' as const, label: 'Statement' },
                        { key: 'stats' as const, label: 'Statistics' },
                        { key: 'editorials' as const, label: 'Editorials' },
                        ...(getAccessToken() ? [{ key: 'submissions' as const, label: 'My Submissions' }] : []),
                        { key: 'more' as const, label: 'More' },
                    ]).map(t => (
                        <button
                            key={t.key}
                            type="button"
                            role="tab"
                            aria-selected={tab === t.key}
                            onClick={() => setTab(t.key)}
                            className={`-mb-px whitespace-nowrap border-b-2 px-3.5 py-2 text-sm font-medium transition-colors sm:px-4 ${tab === t.key
                                ? 'border-brand text-brand'
                                : 'border-transparent text-muted hover:text-ink'
                                }`}
                        >
                            {t.label}
                        </button>
                    ))}
                </div>

                {tab === 'statement' ? (
                    <div className="space-y-5 pb-8 px-4 py-6 md:px-8 md:py-8">
                        <ProblemStatement problem={{ ...problem, description: effectiveDescription }} />
                    </div>
                ) : tab === 'more' ? (
                    <div className="space-y-6 max-w-2xl py-2">
                        {problem.interactive && (
                            <div className="bg-purple-50 dark:bg-purple-950 border border-purple-200 dark:border-purple-800 rounded-lg p-4">
                                <h3 className="font-semibold text-accent mb-1">Interactive Problem</h3>
                                <p className="text-sm text-purple-700 dark:text-purple-400 leading-relaxed">
                                    Your program communicates with an interactor via stdin/stdout. Remember to flush your output buffer after each write.
                                </p>
                            </div>
                        )}
                        {problem.scoring_mode === 'partial' && (
                            <div className="bg-blue-50 dark:bg-blue-950 border border-blue-200 dark:border-blue-800 rounded-lg p-4">
                                <h3 className="font-semibold text-blue-800 dark:text-blue-300 mb-1">Partial Scoring</h3>
                                <p className="text-sm text-blue-700 dark:text-blue-400 leading-relaxed">
                                    You earn points for each passing subtask. The final score is the sum of the points from the subtasks your solution passes.
                                </p>
                            </div>
                        )}
                        {problem.tags && problem.tags.length > 0 && (
                            <div>
                                <h3 className="font-semibold text-ink mb-3">Tags</h3>
                                <div className="flex flex-wrap gap-2">
                                    {problem.tags.map((tag: string) => (
                                        <Link
                                            key={tag}
                                            to={`/problems?tag=${tag}`}
                                            className="bg-surface-3 hover:bg-surface-3 border border-hairline text-sm text-ink-soft px-3 py-1.5 rounded transition-colors"
                                        >
                                            {tag}
                                        </Link>
                                    ))}
                                </div>
                            </div>
                        )}
                        {problem.source && (
                            <div>
                                <h3 className="font-semibold text-ink mb-2">Source / Author</h3>
                                <div className="text-sm text-ink-soft bg-surface-2 border border-hairline px-4 py-2 rounded-lg inline-block">
                                    {problem.source}
                                </div>
                            </div>
                        )}
                        {(!problem.tags?.length && !problem.source && !problem.interactive && problem.scoring_mode !== 'partial') && (
                            <p className="text-muted text-sm py-4">No additional information available for this problem.</p>
                        )}
                        <CommentSection parentType="problem" parentId={problem.id} />
                    </div>
                ) : tab === 'stats' ? (
                    <ProblemStats problemId={problem.id} />
                ) : tab === 'editorials' ? (
                    <div className="space-y-4">
                        {(() => {
                            const role = decodeRole()
                            const isPrivileged = !!getAccessToken() && (role === 'admin' || role === 'setter')
                            if (isPrivileged) {
                                return (
                                    <div className="flex justify-end">
                                        <button
                                            onClick={() => setIsAddModalOpen(true)}
                                            className="bg-brand hover:bg-brand-hover text-on-brand px-3 py-1.5 rounded text-sm font-medium"
                                        >
                                            Add Editorial
                                        </button>
                                    </div>
                                )
                            }
                            return null
                        })()}
                        {editorials.length === 0 ? (
                            <EmptyState text="No editorials yet for this problem" description="Published write-ups from other solvers will appear here." />
                        ) : (
                            editorials.map(e => (
                                <Link key={e.id} to={`/editorials/${e.id}`} className="block border rounded p-4 hover:bg-surface-2">
                                    <div className="flex items-center gap-2 mb-1">
                                        {e.is_official && <span className="text-xs bg-ok/12 text-ok px-2 py-0.5 rounded font-medium">Official</span>}
                                        <h4 className="font-medium">{e.title}</h4>
                                    </div>
                                    <div className="flex gap-4 text-xs text-muted-soft">
                                        <span>{e.username}</span>
                                        {e.time_complexity && <span>Time: {e.time_complexity}</span>}
                                        <span>{e.upvotes} upvotes</span>
                                    </div>
                                </Link>
                            ))
                        )}
                        <AddEditorialModal
                            problemId={problem.id}
                            isUserAdmin={decodeRole() === 'admin'}
                            isOpen={isAddModalOpen}
                            onClose={() => setIsAddModalOpen(false)}
                            onSuccess={() => {
                                api.editorials.getByProblem(problem.id)
                                    .then(d => setEditorials(d.data || []))
                                    .catch(console.error)
                            }}
                        />
                    </div>
                ) : (
                    <div className="space-y-4">
                        {loadingSubs ? (
                            <div className="text-center py-8 text-muted-soft">Loading submissions...</div>
                        ) : mySubs.length === 0 ? (
                            <EmptyState text="No submissions yet for this problem" description="Submit a solution to start your attempt record." />
                        ) : (
                            <div className="border border-hairline rounded-lg overflow-x-auto">
                                <table className="w-full text-sm">
                                    <thead className="bg-surface-2 text-muted text-xs uppercase">
                                        <tr>
                                            <th className="px-4 py-2 text-left">ID</th>
                                            <th className="px-4 py-2 text-left">Language</th>
                                            <th className="px-4 py-2 text-left">Verdict</th>
                                            <th className="px-4 py-2 text-left">Time</th>
                                            <th className="px-4 py-2 text-left">Memory</th>
                                        </tr>
                                    </thead>
                                    <tbody className="divide-y divide-hairline">
                                        {mySubs.map(s => (
                                            <tr key={s.id} className="hover:bg-surface-2">
                                                <td className="px-4 py-2 font-mono text-xs">
                                                    <Link to={`/submissions/${s.id}`} className="text-brand hover:underline">
                                                        {s.id?.substring(0, 8)}...
                                                    </Link>
                                                </td>
                                                <td className="px-4 py-2 text-muted">{languageName(s.language)}</td>
                                                <td className="px-4 py-2">
                                                    <VerdictBadge status={s.status} />
                                                </td>
                                                <td className="px-4 py-2 text-muted">{s.time_used > 0 ? `${s.time_used}ms` : '—'}</td>
                                                <td className="px-4 py-2 text-muted">{s.memory_used > 0 ? `${Math.round(s.memory_used / 1024)}MB` : '—'}</td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </table>
                            </div>
                        )}
                    </div>
                )}
            </div>

            {/* Divider */}
            <div
                onMouseDown={handleMouseDown}
                className={`hidden md:flex w-3 flex-shrink-0 cursor-col-resize group transition-colors relative ${dragging ? 'bg-blue-500' : 'bg-surface-3 hover:bg-blue-500'}`}
            >
                {/* Grip dots indicator */}
                <div className="absolute inset-0 flex flex-col items-center justify-center gap-1">
                    <span className="w-1 h-1 rounded-full bg-muted-soft group-hover:bg-white/70 transition-colors" />
                    <span className="w-1 h-1 rounded-full bg-muted-soft group-hover:bg-white/70 transition-colors" />
                    <span className="w-1 h-1 rounded-full bg-muted-soft group-hover:bg-white/70 transition-colors" />
                </div>
            </div>

            {/* IDE */}
            <div
                className={`flex flex-col gap-3 overflow-y-auto pl-1 ${mobileTab === 'code' ? '' : 'hidden'} md:block`}
                style={isDesktop ? { width: `${100 - splitPos}%`, minWidth: '20%' } : { flex: 1 }}
            >
                <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2">
                    <select
                        value={lang}
                        onChange={e => setLang(e.target.value)}
                        className="border border-hairline-strong rounded px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] w-full sm:w-auto"
                    >
                        {languages.map(l => <option key={l.value} value={l.value}>{l.label}</option>)}
                    </select>
                    <div className="flex gap-2 flex-shrink-0">
                        <button
                            onClick={testWithSamples}
                            disabled={runningSamples || !code.trim() || isRemotePlatform(problem?.source)}
                            title={isRemotePlatform(problem?.source) ? 'Test Samples is only available for locally judged problems' : undefined}
                            className="bg-brand text-on-brand px-3 py-1.5 rounded text-sm font-medium hover:bg-brand-hover disabled:opacity-50 transition-colors cursor-pointer"
                        >
                            {runningSamples ? 'Testing...' : 'Test Samples'}
                        </button>
                        <button
                            onClick={submit}
                            disabled={submitting}
                            className="bg-ok-fill text-white px-4 py-1.5 rounded text-sm font-medium hover:bg-green-700 disabled:opacity-50 transition-colors cursor-pointer"
                        >
                            {submitting ? 'Submitting...' : 'Submit'}
                        </button>
                    </div>
                </div>
                {/* Submission receipt. Sits above the editor so the id and the
                    live verdict are on screen the instant Submit is pressed. */}
                {result && <SubmissionStatusPanel submission={result} polling={pollingResult} />}
                <CodeEditor
                    language={lang}
                    value={code}
                    onChange={handleCodeChange}
                    height={isDesktop ? "400px" : "250px"}
                />

                {/* Sample Test Results */}
                {sampleResults.length > 0 && (
                    <div className="border border-hairline rounded-lg overflow-hidden bg-surface shadow-sm">
                        <div className="bg-surface-2 border-b border-hairline px-4 py-3 flex items-center justify-between">
                            <span className="font-semibold text-sm text-ink-soft">Sample Test Results</span>
                            <span className={`text-xs font-medium px-2 py-0.5 rounded ${sampleResults.every(r => r.passed) ? 'bg-ok/12 text-ok' :
                                sampleResults.some(r => r.passed) ? 'bg-warn/12 text-warn' :
                                    'bg-bad/12 text-bad'
                                }`}>
                                {sampleResults.filter(r => r.passed).length}/{sampleResults.length} Passed
                            </span>
                        </div>
                        <div className="divide-y divide-hairline">
                            {sampleResults.map((r) => (
                                <div key={r.index} className="p-4">
                                    <div className="flex items-center justify-between mb-2">
                                        <span className="text-sm font-medium text-ink-soft">Sample {r.index}</span>
                                        <div className="flex items-center gap-2">
                                            {r.time > 0 && (
                                                <span className="text-xs text-muted">{r.time}ms</span>
                                            )}
                                            <span className={`text-xs font-medium px-2 py-0.5 rounded ${r.passed ? 'bg-ok/12 text-ok' : 'bg-bad/12 text-bad'
                                                }`}>
                                                {r.passed ? 'Passed' : 'Failed'}
                                            </span>
                                        </div>
                                    </div>
                                    <div className="text-xs text-muted mb-2 font-mono bg-surface-2 rounded p-2 overflow-x-auto">
                                        Input: {r.input.substring(0, 100)}{r.input.length > 100 ? '...' : ''}
                                    </div>
                                    {!r.passed && (
                                        <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                                            <div>
                                                <span className="block text-xs font-semibold text-muted mb-1">Expected</span>
                                                <pre className="bg-ok/10 text-ok font-mono text-xs p-2 rounded overflow-x-auto max-h-24 border border-green-100">{r.expected || '(empty)'}</pre>
                                            </div>
                                            <div>
                                                <span className="block text-xs font-semibold text-muted mb-1">Actual</span>
                                                <pre className="bg-bad/10 text-bad font-mono text-xs p-2 rounded overflow-x-auto max-h-24 border border-red-100">{r.actual || '(empty)'}</pre>
                                            </div>
                                        </div>
                                    )}
                                    {r.compile_output && (
                                        <pre className="mt-2 text-xs text-bad bg-bad/10 rounded p-2 overflow-x-auto">{r.compile_output}</pre>
                                    )}
                                    {r.stderr && (
                                        <pre className="mt-2 text-xs text-bad bg-bad/10 rounded p-2 overflow-x-auto">{r.stderr}</pre>
                                    )}
                                </div>
                            ))}
                        </div>
                    </div>
                )}

                {/* Custom Scratchpad */}
                <div className="mt-4 border border-hairline rounded-lg overflow-hidden bg-surface shadow-sm">
                    <div className="bg-surface-2 border-b border-hairline px-4 py-3 flex items-center justify-between">
                        <span className="font-semibold text-sm text-ink-soft">Custom Stdin / Scratchpad</span>
                        <button
                            onClick={runCustomCode}
                            disabled={runningCustom || !code.trim() || problem?.source !== 'local'}
                            title={problem?.source !== 'local' ? 'Custom run is only available for local problems' : undefined}
                            className="bg-brand hover:bg-brand-hover text-on-brand text-xs font-semibold px-3 py-1.5 rounded disabled:opacity-50 transition-colors cursor-pointer"
                        >
                            {runningCustom ? 'Running...' : 'Run Code'}
                        </button>
                    </div>
                    <div className="p-4 space-y-4">
                        <div>
                            <label className="block text-xs font-semibold text-muted uppercase tracking-wider mb-1.5">Custom Input (Stdin)</label>
                            <textarea aria-label="Custom Input (Stdin)"
                                value={customInput}
                                onChange={(e) => setCustomInput(e.target.value)}
                                rows={4}
                                placeholder="Enter input values here..."
                                className="w-full font-mono text-xs border border-hairline-strong rounded p-2.5 bg-surface-2 focus:bg-surface focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] transition-colors"
                            />
                        </div>

                        {customOutput && (
                            <div className="space-y-3 pt-2 border-t border-hairline">
                                <div className="flex items-center justify-between text-xs">
                                    <span className={`font-semibold uppercase tracking-wider ${customOutput.status === 'success' ? 'text-ok' :
                                        customOutput.status === 'ce' ? 'text-accent' : 'text-bad'
                                        }`}>
                                        Execution: {
                                            customOutput.status === 'success' ? 'Completed' :
                                                customOutput.status === 'ce' ? 'Compilation Error' :
                                                    customOutput.status === 'tle' ? 'Time Limit Exceeded' :
                                                        customOutput.status === 'mle' ? 'Memory Limit Exceeded' :
                                                            customOutput.status === 're' ? 'Runtime Error' :
                                                                customOutput.status
                                        }
                                    </span>
                                    {customOutput.time_used > 0 && (
                                        <span className="text-muted font-mono">
                                            {customOutput.time_used}ms / {Math.round(customOutput.memory_used / 1024)}MB
                                        </span>
                                    )}
                                </div>

                                {customOutput.compile_output && (
                                    <div>
                                        <span className="block text-xs font-semibold text-bad dark:text-red-400 mb-1">Compiler Output</span>
                                        <pre className="bg-bad/10 text-bad font-mono text-xs p-3 rounded overflow-x-auto max-h-40 border border-red-100">{customOutput.compile_output}</pre>
                                    </div>
                                )}

                                {customOutput.status !== 'ce' && (
                                    <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                                        <div>
                                            <span className="block text-xs font-semibold text-muted mb-1">Standard Output (Stdout)</span>
                                            <pre className="bg-code text-code-ink font-mono text-xs p-3 rounded overflow-x-auto max-h-40">{customOutput.stdout || <span className="text-code-ink/60 italic">No output</span>}</pre>
                                        </div>
                                        <div>
                                            <span className="block text-xs font-semibold text-muted mb-1">Standard Error (Stderr)</span>
                                            <pre className="bg-code text-bad-on-code font-mono text-xs p-3 rounded overflow-x-auto max-h-40">{customOutput.stderr || <span className="text-code-ink/60 italic">No stderr</span>}</pre>
                                        </div>
                                    </div>
                                )}
                            </div>
                        )}
                    </div>
                </div>
            </div>
        </div>
    )
}