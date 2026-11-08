import { useEffect, useState, useRef, useCallback } from 'react'
import { useParams, Link } from 'react-router-dom'
import ReactMarkdown from 'react-markdown'
import remarkMath from 'remark-math'
import remarkGfm from 'remark-gfm'
import { EmptyState } from '../components/EmptyState'
import { LANGUAGE_OPTIONS } from '../lib/languages'
import rehypeKatex from 'rehype-katex'
import 'katex/dist/katex.min.css'
import { api, getAccessToken } from '../lib/api'
import { useDocumentTitle } from '../lib/documentTitle'
import { lazy } from 'react'
const CodeEditor = lazy(() => import('../components/CodeEditor'))
import { Copy, Check, Lightbulb, ClipboardList, BarChart3, FileText, Download } from 'lucide-react'
import { useToast } from '../components/Toast'
import { SubmissionStatusPanel } from '../components/SubmissionStatusPanel'
import { errorMessage } from '../lib/errors'

// Derived from the API so these track the real response shapes without
// importing model types (shared type files are in flux).
type LoadedProblem = Awaited<ReturnType<typeof api.contests.getProblemByIndex>>['problem'] & { problem_type?: string }
type LoadedContest = Awaited<ReturnType<typeof api.contests.getProblemByIndex>>['contest']
interface SampleCaseData { input: string; output?: string; explanation?: string }
interface SubmissionView {
    id?: string
    error?: string
    status?: string
    verdict?: string
    time_used?: number | string
    memory_used?: number
    score?: number | string
    language?: string
    compile_output?: string
}
interface SubRow { id: string; status?: string; created_at?: string; time_used?: number; memory_used?: number; language?: string }
interface RunResult { stdout?: string; stderr?: string; expected?: string; passed?: boolean | null; status?: string; time_used?: number; memory_used?: number; compile_output?: string; error?: string }
interface SampleRunResult { index: number; input: string; expected?: string; actual?: string; passed: boolean; status?: string; time?: number; memory?: number; stderr?: string; compile_output?: string; error?: string }

function isPdfUrl(url: string | undefined | null): boolean {
    if (!url) return false
    try {
        const parsed = new URL(url)
        if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return false
    } catch {
        return false
    }
    const lower = url.toLowerCase()
    return lower.endsWith('.pdf') || lower.includes('/problem.pdf')
}


/** True when the split view has to collapse into a single column (matches `lg`). */
function isNarrowViewport(): boolean {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return false
    return window.matchMedia('(max-width: 1023px)').matches
}

const TEMPLATE_CODE: Record<string, string> = {
    'cpp-gpp-64': `#include <iostream>\nusing namespace std;\n\nint main() {\n    // Your code here\n    \n    return 0;\n}`,
    'cpp-gpp-32': `#include <iostream>\nusing namespace std;\n\nint main() {\n    // Your code here\n    \n    return 0;\n}`,
    'c-gcc-64': `#include <stdio.h>\n\nint main() {\n    // Your code here\n    \n    return 0;\n}`,
    'python': `# Your code here\n`,
    'java': `import java.util.Scanner;\n\npublic class Main {\n    public static void main(String[] args) {\n        Scanner sc = new Scanner(System.in);\n        // Your code here\n        \n    }\n}`,
    'rust': `fn main() {\n    // Your code here\n    \n}`,
    'nodejs': `// Your code here\n`,
    'csharp': `using System;\n\nclass Program {\n    static void Main() {\n        // Your code here\n        \n    }\n}`,
}

const STATUS_LABELS: Record<string, string> = {
    ac: 'Accepted',
    success: 'Accepted',
    wa: 'Wrong Answer',
    tle: 'Time Limit Exceeded',
    TLE: 'Time Limit Exceeded',
    mle: 'Memory Limit Exceeded',
    MLE: 'Memory Limit Exceeded',
    re: 'Runtime Error',
    RE: 'Runtime Error',
    ce: 'Compile Error',
    CE: 'Compile Error',
    pending: 'Pending',
    judging: 'Judging...',
    se: 'System Error',
    SE: 'System Error',
}

const STATUS_COLORS: Record<string, string> = {
    ac: 'bg-ok/12 text-ok border-ok/40',
    success: 'bg-ok/12 text-ok border-ok/40',
    wa: 'bg-bad/12 text-bad border-bad/40',
    tle: 'bg-warn/12 text-warn border-warn/40',
    TLE: 'bg-warn/12 text-warn border-warn/40',
    mle: 'bg-warn/12 text-warn border-warn/40',
    MLE: 'bg-warn/12 text-warn border-warn/40',
    re: 'bg-bad/12 text-bad border-bad/40',
    RE: 'bg-bad/12 text-bad border-bad/40',
    ce: 'bg-violet-500/12 text-accent border-purple-300 dark:border-purple-700',
    CE: 'bg-violet-500/12 text-accent border-purple-300 dark:border-purple-700',
    pending: 'bg-brand-soft text-brand border-brand/40',
    judging: 'bg-brand-soft text-brand border-brand/40',
    se: 'bg-surface-3/30 text-ink border-hairline-strong',
    SE: 'bg-surface-3/30 text-ink border-hairline-strong',
}

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
    const [now, setNow] = useState(() => Date.now())

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

function CopyButton({ text }: { text: string }) {
    const [copied, setCopied] = useState(false)

    const handleCopy = async () => {
        try {
            await navigator.clipboard.writeText(text)
            setCopied(true)
            setTimeout(() => setCopied(false), 2000)
        } catch {
            const ta = document.createElement('textarea')
            ta.value = text
            document.body.appendChild(ta)
            ta.select()
            document.execCommand('copy')
            document.body.removeChild(ta)
            setCopied(true)
            setTimeout(() => setCopied(false), 2000)
        }
    }

    return (
        <button
            onClick={handleCopy}
            className="text-xs text-muted hover:text-ink transition-colors px-2 py-1 rounded hover:bg-surface-3"
            title="Copy to clipboard"
        >
            {copied ? <><Check className="w-3.5 h-3.5" /> Copied</> : <><Copy className="w-3.5 h-3.5" /> Copy</>}
        </button>
    )
}

function SampleCase({ sample, index }: { sample: SampleCaseData; index: number }) {
    return (
        <div className="border border-hairline rounded-lg overflow-hidden">
            <div className="bg-surface-2/20 px-4 py-2 border-b border-hairline">
                <span className="font-semibold text-sm text-ink-soft">Sample {index + 1}</span>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 divide-y md:divide-y-0 md:divide-x divide-hairline">
                <div className="p-4">
                    <div className="flex items-center justify-between mb-2">
                        <span className="text-xs font-semibold text-muted uppercase tracking-wide">Input</span>
                        <CopyButton text={sample.input} />
                    </div>
                    <pre className="bg-surface-2 text-ink border border-hairline text-sm p-3 rounded-md overflow-x-auto font-mono whitespace-pre-wrap">
                        {sample.input}
                    </pre>
                </div>
                <div className="p-4">
                    <div className="flex items-center justify-between mb-2">
                        <span className="text-xs font-semibold text-muted uppercase tracking-wide">Output</span>
                        <CopyButton text={sample.output || ''} />
                    </div>
                    <pre className="bg-surface-2 text-ink border border-hairline text-sm p-3 rounded-md overflow-x-auto font-mono whitespace-pre-wrap">
                        {sample.output}
                    </pre>
                </div>
            </div>
            {sample.explanation && (
                <div className="border-t border-hairline bg-brand-soft px-4 py-3">
                    <div className="text-xs font-semibold text-brand uppercase tracking-wide mb-1">Explanation</div>
                    <div className="prose prose-sm max-w-none text-ink-soft">
                        <ReactMarkdown remarkPlugins={[remarkGfm, remarkMath]} rehypePlugins={[rehypeKatex]}>
                            {sample.explanation}
                        </ReactMarkdown>
                    </div>
                </div>
            )}
        </div>
    )
}

function HintSection({ hint }: { hint: string }) {
    const [expanded, setExpanded] = useState(false)

    return (
        <div className="border border-warn/30 rounded-lg overflow-hidden">
            <button
                onClick={() => setExpanded(!expanded)}
                className="w-full flex items-center justify-between px-4 py-3 bg-warn/10 hover:bg-warn/12 transition-colors text-left"
            >
                <span className="text-sm font-semibold text-warn flex items-center gap-1.5"><Lightbulb className="w-4 h-4" /> Hint</span>
                <span className="text-warn text-sm">{expanded ? '▲ Hide' : '▼ Show'}</span>
            </button>
            {expanded && (
                <div className="px-4 py-3 bg-surface border-t border-warn/30">
                    <div className="prose prose-sm max-w-none text-ink-soft">
                        <ReactMarkdown remarkPlugins={[remarkGfm, remarkMath]} rehypePlugins={[rehypeKatex]}>
                            {hint}
                        </ReactMarkdown>
                    </div>
                </div>
            )}
        </div>
    )
}

/**
 * Receipt for the submission made on this page. The id and the live verdict
 * come from the shared panel so a contest submission and a practice one look
 * and behave the same.
 */
function SubmissionResult({ result, polling }: { result: SubmissionView; polling: boolean }) {
    if (result.error) {
        return (
            <div className="p-4 bg-bad/10 border border-bad/30 rounded-lg">
                <div className="flex items-center gap-2">
                    <span className="text-bad font-semibold text-sm">Error</span>
                </div>
                <p className="text-bad text-sm mt-1">{result.error}</p>
            </div>
        )
    }

    // A result without an id cannot be linked or polled — nothing to show.
    if (!result.id) return null

    return (
        <SubmissionStatusPanel
            polling={polling}
            submission={{
                id: result.id,
                status: result.status || result.verdict || 'pending',
                language: result.language,
                time_used: typeof result.time_used === 'number' ? result.time_used : null,
                memory_used: result.memory_used ?? null,
                score: typeof result.score === 'number' ? result.score : null,
                compile_output: result.compile_output,
            }}
        />
    )
}

function MySubmissionsTab({ problemId, contestId }: { problemId: string; contestId?: string }) {
    const [subs, setSubs] = useState<SubRow[]>([])
    const [loading, setLoading] = useState(true)
    const isMounted = useRef(true)

    useEffect(() => {
        isMounted.current = true
        return () => { isMounted.current = false }
    }, [])

    useEffect(() => {
        if (!problemId) return
        queueMicrotask(() => setLoading(true))
        api.submissions.list(0, 30, problemId, contestId)
            .then(d => {
                if (isMounted.current) setSubs(d.data || [])
            })
            .catch(console.error)
            .finally(() => {
                if (isMounted.current) setLoading(false)
            })
    }, [problemId, contestId])

    if (loading) {
        return (
            <div className="py-8 text-center text-muted text-sm">Loading submissions...</div>
        )
    }

    if (subs.length === 0) {
        return (
            <div className="py-4"><EmptyState text="No submissions yet" description="Submissions for this problem will appear here as participants solve." /></div>
        )
    }

    return (
        <div className="overflow-x-auto">
            <table className="min-w-full text-sm">
                <thead>
                    <tr className="border-b border-hairline">
                        <th className="text-left py-2 px-3 text-muted font-semibold">When</th>
                        <th className="text-left py-2 px-3 text-muted font-semibold">Verdict</th>
                        <th className="text-left py-2 px-3 text-muted font-semibold">Time</th>
                        <th className="text-left py-2 px-3 text-muted font-semibold">Memory</th>
                        <th className="text-left py-2 px-3 text-muted font-semibold">Lang</th>
                        <th className="text-left py-2 px-3 text-muted font-semibold"></th>
                    </tr>
                </thead>
                <tbody>
                    {subs.map((s) => {
                        const statusKey = (s.status || '').toLowerCase()
                        const label = STATUS_LABELS[s.status || ''] || s.status
                        const colorClass = STATUS_COLORS[s.status || ''] || STATUS_COLORS[statusKey] || 'bg-surface-3/30 text-ink border-hairline-strong'
                        return (
                            <tr key={s.id} className="border-b border-hairline hover:bg-surface-2 transition-colors">
                                <td className="py-2.5 px-3 text-muted whitespace-nowrap">
                                    {s.created_at ? new Date(s.created_at).toLocaleString() : '—'}
                                </td>
                                <td className="py-2.5 px-3">
                                    <span className={`inline-block px-2 py-0.5 rounded-full text-xs font-bold border ${colorClass}`}>
                                        {label}
                                    </span>
                                </td>
                                <td className="py-2.5 px-3 text-muted font-mono whitespace-nowrap">
                                    {s.time_used != null ? `${s.time_used} ms` : '—'}
                                </td>
                                <td className="py-2.5 px-3 text-muted font-mono whitespace-nowrap">
                                    {s.memory_used != null ? `${Math.round(s.memory_used / 1024)} MB` : '—'}
                                </td>
                                <td className="py-2.5 px-3 text-muted whitespace-nowrap">
                                    {s.language || '—'}
                                </td>
                                <td className="py-2.5 px-3">
                                    <Link
                                        to={`/submissions/${s.id}`}
                                        className="text-brand hover:underline text-xs font-medium"
                                    >
                                        View
                                    </Link>
                                </td>
                            </tr>
                        )
                    })}
                </tbody>
            </table>
        </div>
    )
}

export default function ContestProblem() {
    const toast = useToast()
    const { contestId, index } = useParams<{ contestId: string; index: string }>()
    const [problem, setProblem] = useState<LoadedProblem | null>(null)
    const [contest, setContest] = useState<LoadedContest | null>(null)
    useDocumentTitle(problem?.title)
    const [canSubmit, setCanSubmit] = useState(true)
    const [upsolvingDisabled, setUpsolvingDisabled] = useState(false)
    const [statementHidden, setStatementHidden] = useState(false)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)

    const [tab, setTab] = useState<'statement' | 'submissions'>('statement')

    const [customInput, setCustomInput] = useState('')
    const [customOutput, setCustomOutput] = useState<RunResult | null>(null)
    const [runningCustom, setRunningCustom] = useState(false)
    const [sampleResults, setSampleResults] = useState<SampleRunResult[]>([])
    const [runningSamples, setRunningSamples] = useState(false)
    const [lastTestTime, setLastTestTime] = useState(0)

    const [language, setLanguage] = useState('cpp-gpp-64')
    const [code, setCode] = useState('')
    const [submitting, setSubmitting] = useState(false)
    const [result, setResult] = useState<SubmissionView | null>(null)
    // True while the poller is still chasing a verdict for the submission above.
    const [polling, setPolling] = useState(false)
    const [lastSubmitTime, setLastSubmitTime] = useState(0)
    const isMounted = useRef(true)

    const [splitPos, setSplitPos] = useState(() => {
        const saved = localStorage.getItem('aioj_contest_split_pos')
        return saved ? parseFloat(saved) : 50
    })
    const [dragging, setDragging] = useState(false)
    const containerRef = useRef<HTMLDivElement>(null)

    // The drag divider only makes sense when both panes fit side by side. Below
    // `lg` each pane was ~170px wide at 390px, which pushed the Run/Submit
    // buttons out of the submit pane, so stack the panes instead.
    const [isNarrow, setIsNarrow] = useState(isNarrowViewport)
    useEffect(() => {
        if (typeof window.matchMedia !== 'function') return
        const mq = window.matchMedia('(max-width: 1023px)')
        const onChange = () => setIsNarrow(mq.matches)
        mq.addEventListener('change', onChange)
        return () => mq.removeEventListener('change', onChange)
    }, [])

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
            localStorage.setItem('aioj_contest_split_pos', splitPos.toString())
        }
        window.addEventListener('mousemove', handleMouseMove)
        window.addEventListener('mouseup', handleMouseUp)
        return () => {
            window.removeEventListener('mousemove', handleMouseMove)
            window.removeEventListener('mouseup', handleMouseUp)
        }
    }, [dragging, splitPos])

    const isLoggedIn = !!getAccessToken()
    const status = contest ? getStatus(contest.start_time, contest.end_time, contest.freeze_time ?? undefined) : 'ended'
    const isUpcoming = status === 'upcoming'
    const isRunning = status === 'running' || status === 'frozen'
    const showSubmit = isLoggedIn && canSubmit && !upsolvingDisabled

    useEffect(() => {
        isMounted.current = true
        return () => { isMounted.current = false }
    }, [])

    const loadProblem = useCallback(async () => {
        if (!contestId || !index) return
        try {
            setLoading(true)
            setError(null)
            const data = await api.contests.getProblemByIndex(contestId, index)
            if (!isMounted.current) return
            setProblem(data.problem)
            setContest(data.contest)
            setCanSubmit(data.can_submit ?? true)
            setUpsolvingDisabled(data.upsolving_disabled ?? false)
            setStatementHidden(data.statement_hidden ?? false)
        } catch (err) {
            if (isMounted.current) {
                setError(errorMessage(err) || 'Failed to load problem')
            }
        } finally {
            if (isMounted.current) {
                setLoading(false)
            }
        }
    }, [contestId, index])

    useEffect(() => {
        queueMicrotask(loadProblem)
    }, [loadProblem])

    // Load saved draft or template on language change
    useEffect(() => {
        if (!problem?.id) return
        const key = `aioj_contest_draft_${problem.id}_${language}`
        const saved = localStorage.getItem(key)
        queueMicrotask(() => setCode(saved || TEMPLATE_CODE[language] || ''))
    }, [problem?.id, language])

    const handleCodeChange = (newCode: string) => {
        setCode(newCode)
        if (problem?.id) {
            const key = `aioj_contest_draft_${problem.id}_${language}`
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
                language: language,
                input: customInput,
            })
            if (isMounted.current) {
                setCustomOutput(res)
            }
        } catch (e) {
            toast.error('Custom run failed: ' + errorMessage(e))
        } finally {
            if (isMounted.current) {
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
                    language: language,
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
                    error: errorMessage(e),
                })
            }
            if (isMounted.current) {
                setSampleResults([...results])
            }
        }
        if (isMounted.current) {
            setRunningSamples(false)
        }
    }

    const handleSubmit = async () => {
        if (!getAccessToken()) { toast.info('Please login first'); return }
        if (!code.trim()) { toast.info('Please write some code'); return }
        if (!problem) return
        const now = Date.now()
        if (now - lastSubmitTime < 5000) { toast.info('Please wait 5 seconds between submissions'); return }
        setLastSubmitTime(now)
        setSubmitting(true)
        setResult(null)

        try {
            const res = await api.submissions.create({
                problem_id: problem.id,
                language,
                source_code: code,
                contest_id: contestId,
            })
            if (!isMounted.current) return
            // The POST resolves before the judge has touched the row: show the
            // id and the pending verdict now, then let the poller refine it.
            setResult(res)
            setPolling(true)

            // Poll for verdict
            let retries = 0
            const maxRetries = 90
            const poll = async () => {
                if (!isMounted.current) return
                await new Promise(r => setTimeout(r, 2000))
                if (!isMounted.current) return

                try {
                    const updated = await api.submissions.get(res.id)
                    if (!isMounted.current) return
                    setResult(updated)
                    if ((updated.status === 'pending' || updated.status === 'judging') && retries < maxRetries) {
                        retries++
                        poll()
                    } else {
                        setPolling(false)
                    }
                } catch {
                    if (retries < maxRetries && isMounted.current) {
                        retries++
                        poll()
                    } else if (isMounted.current) {
                        setPolling(false)
                    }
                }
            }
            poll()
        } catch (e) {
            if (isMounted.current) {
                setResult({ error: errorMessage(e) })
            }
        } finally {
            if (isMounted.current) {
                setSubmitting(false)
            }
        }
    }

    // Loading state
    if (loading) {
        return (
            <div className="flex items-center justify-center min-h-[60vh]">
                <div className="flex items-center gap-3 text-muted">
                    <svg className="animate-spin h-5 w-5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
                    </svg>
                    <span>Loading problem...</span>
                </div>
            </div>
        )
    }

    // Error state
    if (error) {
        return (
            <div className="max-w-2xl mx-auto py-20 text-center space-y-4">
                <div className="text-bad text-lg font-medium">{error}</div>
                <Link to={`/contests/${contestId}`} className="text-brand hover:underline text-sm">
                    ← Back to Contest
                </Link>
            </div>
        )
    }

    // Not found state
    if (!problem) {
        return (
            <div className="max-w-2xl mx-auto py-20 text-center space-y-4">
                <div className="text-muted text-lg">Problem not found</div>
                <Link to={`/contests/${contestId}`} className="text-brand hover:underline text-sm">
                    ← Back to Contest
                </Link>
            </div>
        )
    }

    return (
        <div>
            {isUpcoming && (
                <div className="mb-6">
                    <CountdownTimer target={contest?.start_time ?? ''} label="Contest starts in" />
                </div>
            )}
            {isRunning && (
                <div className="mb-6">
                    <RunningTimer start={contest?.start_time ?? ''} end={contest?.end_time ?? ''} />
                </div>
            )}

            {/* Notices */}
            {upsolvingDisabled && (
                <div className="mb-6 p-4 bg-warn/10 border border-warn/40 rounded-lg flex items-start gap-3">
                    <span className="text-warn text-lg mt-0.5">⚠</span>
                    <div>
                        <p className="text-warn font-medium text-sm">Upsolving is disabled</p>
                        <p className="text-warn text-sm mt-1">
                            You can view the problem but cannot submit new solutions.
                        </p>
                    </div>
                </div>
            )}

            {statementHidden && (
                <div className="mb-6 p-4 bg-brand-soft border border-brand/40 rounded-lg flex items-start gap-3">
                    <span className="text-brand mt-0.5"><ClipboardList className="w-5 h-5" /></span>
                    <div>
                        <p className="text-brand font-medium text-sm">Problem statement is hidden</p>
                        <p className="text-brand text-sm mt-1">
                            The full problem statement is only available in printed form. Please refer to your printed problem set.
                            Sample cases are shown below for reference.
                        </p>
                    </div>
                </div>
            )}

            {!canSubmit && !upsolvingDisabled && isLoggedIn && (
                <div className="mb-6 p-4 bg-surface-2/20 border border-hairline-strong rounded-lg flex items-start gap-3">
                    <span className="text-muted text-lg mt-0.5">🚫</span>
                    <div>
                        <p className="text-ink-soft font-medium text-sm">Submissions are closed</p>
                        <p className="text-muted text-sm mt-1">You cannot submit to this problem at this time.</p>
                    </div>
                </div>
            )}

            {/* Main split-pane layout (stacks into one column below `lg`) */}
            <div
                ref={containerRef}
                className={`flex h-full select-none ${isNarrow ? 'flex-col gap-6' : ''}`}
                style={{ cursor: dragging ? 'col-resize' : undefined }}
            >
                {/* LEFT: Problem content (splitPos% width) */}
                <div
                    className="space-y-4 overflow-y-auto pr-1"
                    style={isNarrow ? { width: '100%' } : { width: `${splitPos}%`, minWidth: '20%' }}
                >
                    {/* Back link */}
                    <Link
                        to={`/contests/${contestId}`}
                        className="text-brand hover:text-brand hover:underline text-sm font-medium transition-colors"
                    >
                        ← {contest?.title || 'Contest'}
                    </Link>
                    {/* Title with problem letter badge */}
                    <div className="flex items-center gap-3 mt-3">
                        <span className="inline-flex items-center justify-center w-10 h-10 rounded-lg bg-brand text-on-brand font-bold text-lg flex-shrink-0">
                            {index?.toUpperCase()}
                        </span>
                        <h1 className="text-2xl font-bold text-ink leading-tight">{problem.title}</h1>
                    </div>
                    {/* Metadata bar */}
                    <div className="flex flex-wrap items-center gap-4 mt-3 text-sm text-muted">
                        <span className="flex items-center gap-1.5">
                            <span className="font-semibold text-ink-soft">Time Limit:</span> {problem.time_limit} ms
                        </span>
                        <span className="text-muted">|</span>
                        <span className="flex items-center gap-1.5">
                            <span className="font-semibold text-ink-soft">Memory Limit:</span> {Math.round(problem.memory_limit / 1024)} MB
                        </span>
                        {problem.interactive && (
                            <>
                                <span className="text-muted">|</span>
                                <span className="inline-block px-2 py-0.5 rounded text-xs font-semibold bg-violet-500/12 text-accent">
                                    Interactive
                                </span>
                            </>
                        )}
                        {problem.problem_type === 'output' && (
                            <>
                                <span className="text-muted">|</span>
                                <span className="inline-block px-2 py-0.5 rounded text-xs font-semibold bg-warn/12 text-warn">
                                    Output-Only
                                </span>
                            </>
                        )}
                        {problem.scoring_mode === 'partial' && (
                            <>
                                <span className="text-muted">|</span>
                                <span className="inline-block px-2 py-0.5 rounded text-xs font-semibold bg-brand-soft text-brand">
                                    Partial Scoring
                                </span>
                            </>
                        )}
                    </div>
                    {/* Tabs */}
                    <div className="flex border-b border-hairline mb-6">
                        <button
                            onClick={() => setTab('statement')}
                            className={`px-4 py-2.5 text-sm font-medium border-b-2 -mb-px transition-colors ${tab === 'statement'
                                ? 'border-brand text-brand'
                                : 'border-transparent text-muted hover:text-ink hover:border-hairline-strong'
                                }`}
                        >
                            Statement
                        </button>
                        {isLoggedIn && (
                            <button
                                onClick={() => setTab('submissions')}
                                className={`px-4 py-2.5 text-sm font-medium border-b-2 -mb-px transition-colors ${tab === 'submissions'
                                    ? 'border-brand text-brand'
                                    : 'border-transparent text-muted hover:text-ink hover:border-hairline-strong'
                                    }`}
                            >
                                My Submissions
                            </button>
                        )}
                    </div>

                    {/* Statement Tab */}
                    {tab === 'statement' && (
                        <div className="space-y-6 pb-8">
                            {!statementHidden && (
                                <>
                                    {isPdfUrl(problem.description) ? (
                                        <div className="space-y-3">
                                            <h3 className="font-semibold text-sm text-ink-soft uppercase tracking-wide">PDF Problem Statement</h3>
                                            <a
                                                href={problem.description}
                                                target="_blank"
                                                rel="noopener noreferrer"
                                                className="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-brand bg-brand-soft border border-brand/30 rounded-lg hover:bg-brand-soft transition-colors"
                                            >
                                                <Download className="w-4 h-4" />
                                                Download PDF
                                            </a>
                                            <iframe
                                                src={problem.description}
                                                className="w-full h-[800px] border-0 rounded-lg shadow-sm bg-surface"
                                                title="Problem Statement"
                                            />
                                        </div>
                                    ) : (
                                        <div className="prose prose-sm max-w-none text-ink leading-relaxed">
                                            <ReactMarkdown remarkPlugins={[remarkGfm, remarkMath]} rehypePlugins={[rehypeKatex]}>
                                                {problem.description}
                                            </ReactMarkdown>
                                        </div>
                                    )}

                                    {problem.input_format && (
                                        <div>
                                            <h3 className="font-semibold text-sm text-ink-soft uppercase tracking-wide mb-2">Input Format</h3>
                                            <div className="prose prose-sm max-w-none text-ink-soft bg-surface-2/20 rounded-lg p-4 border border-hairline">
                                                <ReactMarkdown remarkPlugins={[remarkGfm, remarkMath]} rehypePlugins={[rehypeKatex]}>
                                                    {problem.input_format}
                                                </ReactMarkdown>
                                            </div>
                                        </div>
                                    )}

                                    {problem.output_format && (
                                        <div>
                                            <h3 className="font-semibold text-sm text-ink-soft uppercase tracking-wide mb-2">Output Format</h3>
                                            <div className="prose prose-sm max-w-none text-ink-soft bg-surface-2/20 rounded-lg p-4 border border-hairline">
                                                <ReactMarkdown remarkPlugins={[remarkGfm, remarkMath]} rehypePlugins={[rehypeKatex]}>
                                                    {problem.output_format}
                                                </ReactMarkdown>
                                            </div>
                                        </div>
                                    )}

                                    {problem.constraints && (
                                        <div>
                                            <h3 className="font-semibold text-sm text-ink-soft uppercase tracking-wide mb-2">Constraints</h3>
                                            <div className="prose prose-sm max-w-none text-ink-soft bg-surface-2/20 rounded-lg p-4 border border-hairline">
                                                <ReactMarkdown remarkPlugins={[remarkGfm, remarkMath]} rehypePlugins={[rehypeKatex]}>
                                                    {problem.constraints}
                                                </ReactMarkdown>
                                            </div>
                                        </div>
                                    )}
                                </>
                            )}

                            {/* Sample cases - always show even with hidden statement */}
                            {problem.sample_cases && problem.sample_cases.length > 0 && (
                                <div>
                                    <h3 className="font-semibold text-sm text-ink-soft uppercase tracking-wide mb-3">Sample Cases</h3>
                                    <div className="space-y-4">
                                        {problem.sample_cases.map((sample: SampleCaseData, i: number) => (
                                            <SampleCase key={i} sample={sample} index={i} />
                                        ))}
                                    </div>
                                </div>
                            )}

                            {/* Hint - collapsible */}
                            {problem.hint && <HintSection hint={problem.hint} />}
                        </div>
                    )}

                    {/* Submissions Tab */}
                    {tab === 'submissions' && (
                        <MySubmissionsTab problemId={problem.id} contestId={contestId} />
                    )}
                </div>

                {/* DIVIDER (only meaningful when the panes sit side by side) */}
                {!isNarrow && (
                    <div
                        onMouseDown={handleMouseDown}
                        className={`w-3 flex-shrink-0 cursor-col-resize group transition-colors relative ${dragging ? 'bg-brand' : 'bg-surface-3 hover:bg-brand'}`}
                    >
                        {/* Grip dots indicator */}
                        <div className="absolute inset-0 flex flex-col items-center justify-center gap-1">
                            <span className="w-1 h-1 rounded-full bg-muted-soft group-hover:bg-white/70 transition-colors" />
                            <span className="w-1 h-1 rounded-full bg-muted-soft group-hover:bg-white/70 transition-colors" />
                            <span className="w-1 h-1 rounded-full bg-muted-soft group-hover:bg-white/70 transition-colors" />
                        </div>
                    </div>
                )}

                {/* RIGHT: Submit panel (100-splitPos% width) */}
                <div
                    className="flex flex-col gap-3 overflow-y-auto pl-1"
                    style={isNarrow ? { width: '100%' } : { width: `${100 - splitPos}%`, minWidth: '20%' }}
                >
                    {showSubmit ? (
                        <>
                            <div className="flex flex-wrap items-center justify-between gap-2">
                                <select
                                    value={language}
                                    onChange={e => setLanguage(e.target.value)}
                                    aria-label="Solution language"
                                    className="border border-hairline-strong rounded px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]"
                                >
                                    {LANGUAGE_OPTIONS.map(l => <option key={l.value} value={l.value}>{l.label}</option>)}
                                </select>
                                <div className="flex gap-2">
                                    <button
                                        onClick={testWithSamples}
                                        disabled={runningSamples || !code.trim() || problem?.source !== 'local'}
                                        title={problem?.source !== 'local' ? 'Test Samples is only available for local problems' : undefined}
                                        className="bg-brand text-on-brand px-3 py-1.5 rounded text-sm font-medium hover:bg-brand-hover disabled:opacity-50 transition-colors cursor-pointer"
                                    >
                                        {runningSamples ? 'Testing...' : 'Test Samples'}
                                    </button>
                                    <button
                                        onClick={handleSubmit}
                                        disabled={submitting || !code.trim()}
                                        className="bg-ok-fill text-white px-4 py-1.5 rounded text-sm font-medium hover:bg-ok-fill disabled:opacity-50 transition-colors cursor-pointer"
                                    >
                                        {submitting ? 'Submitting...' : 'Submit'}
                                    </button>
                                </div>
                            </div>
                            {/* Above the editor so the submission id and the live
                                verdict are on screen the moment Submit is pressed. */}
                            {result && <SubmissionResult result={result} polling={polling} />}

                            <CodeEditor
                                language={language}
                                value={code}
                                onChange={handleCodeChange}
                                height="400px"
                                ariaLabel="Solution source code"
                            />

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
                                                        {(r.time ?? 0) > 0 && (
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
                                                    <div className="grid grid-cols-2 gap-2">
                                                        <div>
                                                            <span className="block text-xs font-semibold text-muted mb-1">Expected</span>
                                                            <pre className="bg-ok/10 text-ok font-mono text-xs p-2 rounded overflow-x-auto max-h-24 border border-ok/20">{r.expected || '(empty)'}</pre>
                                                        </div>
                                                        <div>
                                                            <span className="block text-xs font-semibold text-muted mb-1">Actual</span>
                                                            <pre className="bg-bad/10 text-bad font-mono text-xs p-2 rounded overflow-x-auto max-h-24 border border-bad/20">{r.actual || '(empty)'}</pre>
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

                            <div className="border border-hairline rounded-lg overflow-hidden bg-surface shadow-sm">
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
                                                {(customOutput.time_used ?? 0) > 0 && (
                                                    <span className="text-muted font-mono">
                                                        {customOutput.time_used}ms / {Math.round((customOutput.memory_used ?? 0) / 1024)}MB
                                                    </span>
                                                )}
                                            </div>

                                            {customOutput.compile_output && (
                                                <div>
                                                    <span className="block text-xs font-semibold text-bad dark:text-bad mb-1">Compiler Output</span>
                                                    <pre className="bg-bad/10 text-bad font-mono text-xs p-3 rounded overflow-x-auto max-h-40 border border-bad/20">{customOutput.compile_output}</pre>
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
                        </>
                    ) : !isLoggedIn ? (
                        <div className="p-5 bg-surface-2/20 border border-hairline rounded-lg text-center">
                            <h3 className="font-semibold text-ink mb-2">Login Required</h3>
                            <p className="text-muted text-sm mb-3">
                                Please login to submit a solution.
                            </p>
                            <Link
                                to="/login"
                                className="inline-block px-4 py-2 bg-brand text-on-brand text-sm font-medium rounded-lg hover:bg-brand-hover transition-colors"
                            >
                                Login
                            </Link>
                        </div>
                    ) : !canSubmit ? (
                        <div className="p-5 bg-surface-2/20 border border-hairline rounded-lg text-center">
                            <h3 className="font-semibold text-ink mb-2">Submissions Closed</h3>
                            <p className="text-muted text-sm">
                                {upsolvingDisabled
                                    ? 'Upsolving is disabled for this contest.'
                                    : 'Submissions are not allowed for this problem.'}
                            </p>
                        </div>
                    ) : null}

                    {/* Quick links */}
                    <div className="p-4 bg-surface border border-hairline rounded-lg">
                        <h4 className="text-xs font-semibold text-muted uppercase tracking-wide mb-2">Quick Links</h4>
                        <div className="space-y-1.5">
                            <Link
                                to={`/contests/${contestId}`}
                                className="block text-sm text-brand hover:underline"
                            >
                                ← Contest Dashboard
                            </Link>
                            <Link
                                to={`/contests/${contestId}/scoreboard`}
                                className="block text-sm text-brand hover:underline"
                            >
                                <BarChart3 className="w-3.5 h-3.5" /> Scoreboard
                            </Link>
                            {isLoggedIn && (
                                <Link
                                    to={`/submissions?problem_id=${problem.id}&contest_id=${contestId}`}
                                    className="block text-sm text-brand hover:underline"
                                >
                                    <FileText className="w-3.5 h-3.5" /> All Submissions for this Problem
                                </Link>
                            )}
                        </div>
                    </div>
                </div>
            </div>
        </div>
    )
}
