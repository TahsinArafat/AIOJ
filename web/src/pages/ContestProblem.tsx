import { useEffect, useState, useRef, useCallback } from 'react'
import { useParams, Link } from 'react-router-dom'
import ReactMarkdown from 'react-markdown'
import remarkMath from 'remark-math'
import rehypeKatex from 'rehype-katex'
import 'katex/dist/katex.min.css'
import { api, getAccessToken } from '../lib/api'
import CodeEditor from '../components/CodeEditor'
import { Copy, Check, Lightbulb, ClipboardList, BarChart3, FileText, Download } from 'lucide-react'

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

const LANGS = [
    { value: 'cpp-gpp-64', label: 'GNU G++17 7.3.0 (64 bit)' },
    { value: 'cpp-gpp-32', label: 'GNU G++14 6.4.0 (32 bit)' },
    { value: 'c-gcc-64', label: 'GNU GCC C11 9.2.0 (64 bit)' },
    { value: 'python', label: 'Python 3.8.10' },
    { value: 'java', label: 'Java 11.0.6' },
    { value: 'rust', label: 'Rust 1.75.0' },
    { value: 'nodejs', label: 'Node.js 18.16.1' },
    { value: 'csharp', label: 'Mono C# 6.12.0' },
]

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
    ac: 'bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300 border-green-300 dark:border-green-700',
    success: 'bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300 border-green-300 dark:border-green-700',
    wa: 'bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300 border-red-300 dark:border-red-700',
    tle: 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300 border-yellow-300 dark:border-yellow-700',
    TLE: 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300 border-yellow-300 dark:border-yellow-700',
    mle: 'bg-orange-100 dark:bg-orange-900/30 text-orange-800 dark:text-orange-300 border-orange-300 dark:border-orange-700',
    MLE: 'bg-orange-100 dark:bg-orange-900/30 text-orange-800 dark:text-orange-300 border-orange-300 dark:border-orange-700',
    re: 'bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300 border-red-300 dark:border-red-700',
    RE: 'bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300 border-red-300 dark:border-red-700',
    ce: 'bg-purple-100 dark:bg-purple-900/30 text-purple-800 dark:text-purple-300 border-purple-300 dark:border-purple-700',
    CE: 'bg-purple-100 dark:bg-purple-900/30 text-purple-800 dark:text-purple-300 border-purple-300 dark:border-purple-700',
    pending: 'bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300 border-blue-300 dark:border-blue-700',
    judging: 'bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300 border-blue-300 dark:border-blue-700',
    se: 'bg-gray-100 dark:bg-gray-900/30 text-gray-800 dark:text-gray-300 border-gray-300 dark:border-gray-700',
    SE: 'bg-gray-100 dark:bg-gray-900/30 text-gray-800 dark:text-gray-300 border-gray-300 dark:border-gray-700',
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
            className="text-xs text-gray-500 hover:text-gray-700 transition-colors px-2 py-1 rounded hover:bg-gray-200"
            title="Copy to clipboard"
        >
            {copied ? <><Check className="w-3.5 h-3.5" /> Copied</> : <><Copy className="w-3.5 h-3.5" /> Copy</>}
        </button>
    )
}

function SampleCase({ sample, index }: { sample: any; index: number }) {
    return (
        <div className="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
            <div className="bg-gray-50 dark:bg-gray-900/20 px-4 py-2 border-b border-gray-200 dark:border-gray-700">
                <span className="font-semibold text-sm text-gray-700 dark:text-gray-300">Sample {index + 1}</span>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 divide-y md:divide-y-0 md:divide-x divide-gray-200">
                <div className="p-4">
                    <div className="flex items-center justify-between mb-2">
                        <span className="text-xs font-semibold text-gray-500 uppercase tracking-wide">Input</span>
                        <CopyButton text={sample.input} />
                    </div>
                    <pre className="bg-gray-50 dark:bg-gray-900 text-gray-800 dark:text-gray-200 border border-gray-200 dark:border-gray-700 text-sm p-3 rounded-md overflow-x-auto font-mono whitespace-pre-wrap">
                        {sample.input}
                    </pre>
                </div>
                <div className="p-4">
                    <div className="flex items-center justify-between mb-2">
                        <span className="text-xs font-semibold text-gray-500 uppercase tracking-wide">Output</span>
                        <CopyButton text={sample.output} />
                    </div>
                    <pre className="bg-gray-50 dark:bg-gray-900 text-gray-800 dark:text-gray-200 border border-gray-200 dark:border-gray-700 text-sm p-3 rounded-md overflow-x-auto font-mono whitespace-pre-wrap">
                        {sample.output}
                    </pre>
                </div>
            </div>
            {sample.explanation && (
                <div className="border-t border-gray-200 dark:border-gray-700 bg-blue-50 dark:bg-blue-900/20 px-4 py-3">
                    <div className="text-xs font-semibold text-blue-700 dark:text-blue-300 uppercase tracking-wide mb-1">Explanation</div>
                    <div className="prose prose-sm max-w-none text-gray-700 dark:text-gray-300">
                        <ReactMarkdown remarkPlugins={[remarkMath]} rehypePlugins={[rehypeKatex]}>
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
        <div className="border border-amber-200 dark:border-amber-700 rounded-lg overflow-hidden">
            <button
                onClick={() => setExpanded(!expanded)}
                className="w-full flex items-center justify-between px-4 py-3 bg-amber-50 dark:bg-amber-900/20 hover:bg-amber-100 transition-colors text-left"
            >
                <span className="text-sm font-semibold text-amber-800 dark:text-amber-300 flex items-center gap-1.5"><Lightbulb className="w-4 h-4" /> Hint</span>
                <span className="text-amber-600 dark:text-amber-400 text-sm">{expanded ? '▲ Hide' : '▼ Show'}</span>
            </button>
            {expanded && (
                <div className="px-4 py-3 bg-white dark:bg-gray-800 border-t border-amber-200 dark:border-amber-700">
                    <div className="prose prose-sm max-w-none text-gray-700 dark:text-gray-300">
                        <ReactMarkdown remarkPlugins={[remarkMath]} rehypePlugins={[rehypeKatex]}>
                            {hint}
                        </ReactMarkdown>
                    </div>
                </div>
            )}
        </div>
    )
}

function SubmissionResult({ result }: { result: any }) {
    if (!result) return null

    if (result.error) {
        return (
            <div className="p-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-700 rounded-lg">
                <div className="flex items-center gap-2">
                    <span className="text-red-600 dark:text-red-400 font-semibold text-sm">Error</span>
                </div>
                <p className="text-red-700 dark:text-red-300 text-sm mt-1">{result.error}</p>
            </div>
        )
    }

    const isPending = result.status === 'pending' || result.status === 'judging'
    const statusKey = (result.status || result.verdict || '').toLowerCase()
    const label = STATUS_LABELS[result.status] || STATUS_LABELS[statusKey] || result.status || 'Submitted'
    const colorClass = STATUS_COLORS[result.status] || STATUS_COLORS[statusKey] || 'bg-gray-100 dark:bg-gray-900/30 text-gray-800 dark:text-gray-300 border-gray-300 dark:border-gray-700'

    return (
        <div className="p-4 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg space-y-3">
            <div className="flex items-center justify-between">
                <span className={`inline-flex items-center px-3 py-1 rounded-full text-xs font-bold border ${colorClass}`}>
                    {isPending && (
                        <svg className="animate-spin -ml-1 mr-2 h-3 w-3" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
                        </svg>
                    )}
                    {label}
                </span>
                {result.id && (
                    <Link
                        to={`/submissions/${result.id}`}
                        className="text-xs text-blue-600 dark:text-blue-400 hover:text-blue-800 hover:underline font-medium"
                    >
                        View Details →
                    </Link>
                )}
            </div>

            {result.id && (
                <div className="text-xs text-gray-500">
                    <span className="font-medium">Submission ID:</span>{' '}
                    <Link to={`/submissions/${result.id}`} className="font-mono text-blue-600 dark:text-blue-400 hover:underline">
                        {result.id.substring(0, 12)}
                    </Link>
                </div>
            )}

            <div className="flex flex-wrap gap-4 text-xs text-gray-600 dark:text-gray-400">
                {result.time_used != null && (
                    <div>
                        <span className="font-medium text-gray-700 dark:text-gray-300">Time:</span>{' '}
                        {typeof result.time_used === 'number' ? `${result.time_used} ms` : result.time_used}
                    </div>
                )}
                {result.memory_used != null && (
                    <div>
                        <span className="font-medium text-gray-700 dark:text-gray-300">Memory:</span>{' '}
                        {typeof result.memory_used === 'number' ? `${Math.round(result.memory_used / 1024)} MB` : result.memory_used}
                    </div>
                )}
                {result.score != null && (
                    <div>
                        <span className="font-medium text-gray-700 dark:text-gray-300">Score:</span> {result.score}
                    </div>
                )}
                {result.language && (
                    <div>
                        <span className="font-medium text-gray-700 dark:text-gray-300">Language:</span> {result.language}
                    </div>
                )}
            </div>

            {result.status === 'ce' && result.compile_output && (
                <div className="mt-2">
                    <div className="text-xs font-semibold text-gray-600 dark:text-gray-400 mb-1">Compile Output:</div>
                    <pre className="bg-gray-900 text-red-400 text-xs p-3 rounded-md overflow-x-auto font-mono max-h-40">
                        {result.compile_output}
                    </pre>
                </div>
            )}
        </div>
    )
}

function MySubmissionsTab({ problemId, contestId }: { problemId: string; contestId?: string }) {
    const [subs, setSubs] = useState<any[]>([])
    const [loading, setLoading] = useState(true)
    const isMounted = useRef(true)

    useEffect(() => {
        isMounted.current = true
        return () => { isMounted.current = false }
    }, [])

    useEffect(() => {
        if (!problemId) return
        setLoading(true)
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
            <div className="py-8 text-center text-gray-500 text-sm">Loading submissions...</div>
        )
    }

    if (subs.length === 0) {
        return (
            <div className="py-8 text-center text-gray-400 text-sm">No submissions yet.</div>
        )
    }

    return (
        <div className="overflow-x-auto">
            <table className="min-w-full text-sm">
                <thead>
                    <tr className="border-b border-gray-200 dark:border-gray-700">
                        <th className="text-left py-2 px-3 text-gray-600 dark:text-gray-400 font-semibold">When</th>
                        <th className="text-left py-2 px-3 text-gray-600 dark:text-gray-400 font-semibold">Verdict</th>
                        <th className="text-left py-2 px-3 text-gray-600 dark:text-gray-400 font-semibold">Time</th>
                        <th className="text-left py-2 px-3 text-gray-600 dark:text-gray-400 font-semibold">Memory</th>
                        <th className="text-left py-2 px-3 text-gray-600 dark:text-gray-400 font-semibold">Lang</th>
                        <th className="text-left py-2 px-3 text-gray-600 dark:text-gray-400 font-semibold"></th>
                    </tr>
                </thead>
                <tbody>
                    {subs.map((s: any) => {
                        const statusKey = (s.status || '').toLowerCase()
                        const label = STATUS_LABELS[s.status] || s.status
                        const colorClass = STATUS_COLORS[s.status] || STATUS_COLORS[statusKey] || 'bg-gray-100 dark:bg-gray-900/30 text-gray-800 dark:text-gray-300 border-gray-300 dark:border-gray-700'
                        return (
                            <tr key={s.id} className="border-b border-gray-100 dark:border-gray-800 hover:bg-gray-50 transition-colors">
                                <td className="py-2.5 px-3 text-gray-500 whitespace-nowrap">
                                    {s.created_at ? new Date(s.created_at).toLocaleString() : '—'}
                                </td>
                                <td className="py-2.5 px-3">
                                    <span className={`inline-block px-2 py-0.5 rounded-full text-xs font-bold border ${colorClass}`}>
                                        {label}
                                    </span>
                                </td>
                                <td className="py-2.5 px-3 text-gray-600 dark:text-gray-400 font-mono whitespace-nowrap">
                                    {s.time_used != null ? `${s.time_used} ms` : '—'}
                                </td>
                                <td className="py-2.5 px-3 text-gray-600 dark:text-gray-400 font-mono whitespace-nowrap">
                                    {s.memory_used != null ? `${Math.round(s.memory_used / 1024)} MB` : '—'}
                                </td>
                                <td className="py-2.5 px-3 text-gray-600 dark:text-gray-400 whitespace-nowrap">
                                    {s.language || '—'}
                                </td>
                                <td className="py-2.5 px-3">
                                    <Link
                                        to={`/submissions/${s.id}`}
                                        className="text-blue-600 dark:text-blue-400 hover:underline text-xs font-medium"
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
    const { contestId, index } = useParams<{ contestId: string; index: string }>()
    const [problem, setProblem] = useState<any>(null)
    const [contest, setContest] = useState<any>(null)
    const [canSubmit, setCanSubmit] = useState(true)
    const [upsolvingDisabled, setUpsolvingDisabled] = useState(false)
    const [statementHidden, setStatementHidden] = useState(false)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)

    const [tab, setTab] = useState<'statement' | 'submissions'>('statement')

    const [customInput, setCustomInput] = useState('')
    const [customOutput, setCustomOutput] = useState<any>(null)
    const [runningCustom, setRunningCustom] = useState(false)
    const [sampleResults, setSampleResults] = useState<any[]>([])
    const [runningSamples, setRunningSamples] = useState(false)
    const [lastTestTime, setLastTestTime] = useState(0)

    const [language, setLanguage] = useState('cpp-gpp-64')
    const [code, setCode] = useState('')
    const [submitting, setSubmitting] = useState(false)
    const [result, setResult] = useState<any>(null)
    const [lastSubmitTime, setLastSubmitTime] = useState(0)
    const isMounted = useRef(true)

    const [splitPos, setSplitPos] = useState(() => {
        const saved = localStorage.getItem('aioj_contest_split_pos')
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
    const status = contest ? getStatus(contest.start_time, contest.end_time, contest.freeze_time) : 'ended'
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
        } catch (err: any) {
            if (isMounted.current) {
                setError(err.message || 'Failed to load problem')
            }
        } finally {
            if (isMounted.current) {
                setLoading(false)
            }
        }
    }, [contestId, index])

    useEffect(() => {
        loadProblem()
    }, [loadProblem])

    // Load saved draft or template on language change
    useEffect(() => {
        if (!problem?.id) return
        const key = `aioj_contest_draft_${problem.id}_${language}`
        const saved = localStorage.getItem(key)
        setCode(saved || TEMPLATE_CODE[language] || '')
    }, [problem?.id, language])

    const handleCodeChange = (newCode: string) => {
        setCode(newCode)
        if (problem?.id) {
            const key = `aioj_contest_draft_${problem.id}_${language}`
            localStorage.setItem(key, newCode)
        }
    }

    const runCustomCode = async () => {
        if (!code.trim()) { alert('Please write some code'); return }
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
        } catch (e: any) {
            alert('Custom run failed: ' + e.message)
        } finally {
            if (isMounted.current) {
                setRunningCustom(false)
            }
        }
    }

    const testWithSamples = async () => {
        if (!getAccessToken()) { alert('Please login first'); return }
        if (!code.trim()) { alert('Please write some code'); return }
        if (!problem?.sample_cases?.length) { alert('No sample cases available'); return }
        const now = Date.now()
        if (now - lastTestTime < 5000) { alert('Please wait 5 seconds between Test Samples runs'); return }
        setLastTestTime(now)
        setRunningSamples(true)
        setSampleResults([])
        const results: any[] = []
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
            } catch (e: any) {
                results.push({
                    index: i + 1,
                    input: sc.input,
                    expected: sc.output,
                    actual: '',
                    passed: false,
                    status: 'error',
                    error: e.message,
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
        if (!getAccessToken()) { alert('Please login first'); return }
        if (!code.trim()) { alert('Please write some code'); return }
        const now = Date.now()
        if (now - lastSubmitTime < 5000) { alert('Please wait 5 seconds between submissions'); return }
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
            setResult(res)

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
                    }
                } catch {
                    if (retries < maxRetries && isMounted.current) {
                        retries++
                        poll()
                    }
                }
            }
            poll()
        } catch (e: any) {
            if (isMounted.current) {
                setResult({ error: e.message })
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
                <div className="flex items-center gap-3 text-gray-500">
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
                <div className="text-red-500 text-lg font-medium">{error}</div>
                <Link to={`/contests/${contestId}`} className="text-blue-600 dark:text-blue-400 hover:underline text-sm">
                    ← Back to Contest
                </Link>
            </div>
        )
    }

    // Not found state
    if (!problem) {
        return (
            <div className="max-w-2xl mx-auto py-20 text-center space-y-4">
                <div className="text-gray-500 text-lg">Problem not found</div>
                <Link to={`/contests/${contestId}`} className="text-blue-600 dark:text-blue-400 hover:underline text-sm">
                    ← Back to Contest
                </Link>
            </div>
        )
    }

    return (
        <div className="max-w-7xl mx-auto px-4 py-6">
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

            {/* Notices */}
            {upsolvingDisabled && (
                <div className="mb-6 p-4 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-300 dark:border-yellow-700 rounded-lg flex items-start gap-3">
                    <span className="text-yellow-600 dark:text-yellow-400 text-lg mt-0.5">⚠</span>
                    <div>
                        <p className="text-yellow-800 dark:text-yellow-300 font-medium text-sm">Upsolving is disabled</p>
                        <p className="text-yellow-700 dark:text-yellow-300 text-sm mt-1">
                            You can view the problem but cannot submit new solutions.
                        </p>
                    </div>
                </div>
            )}

            {statementHidden && (
                <div className="mb-6 p-4 bg-blue-50 dark:bg-blue-900/20 border border-blue-300 dark:border-blue-700 rounded-lg flex items-start gap-3">
                    <span className="text-blue-600 dark:text-blue-400 mt-0.5"><ClipboardList className="w-5 h-5" /></span>
                    <div>
                        <p className="text-blue-800 dark:text-blue-300 font-medium text-sm">Problem statement is hidden</p>
                        <p className="text-blue-700 dark:text-blue-300 text-sm mt-1">
                            The full problem statement is only available in printed form. Please refer to your printed problem set.
                            Sample cases are shown below for reference.
                        </p>
                    </div>
                </div>
            )}

            {!canSubmit && !upsolvingDisabled && isLoggedIn && (
                <div className="mb-6 p-4 bg-gray-50 dark:bg-gray-900/20 border border-gray-300 dark:border-gray-700 rounded-lg flex items-start gap-3">
                    <span className="text-gray-500 text-lg mt-0.5">🚫</span>
                    <div>
                        <p className="text-gray-700 dark:text-gray-300 font-medium text-sm">Submissions are closed</p>
                        <p className="text-gray-600 dark:text-gray-400 text-sm mt-1">You cannot submit to this problem at this time.</p>
                    </div>
                </div>
            )}

            {/* Main split-pane layout */}
            <div ref={containerRef} className="flex h-full select-none" style={{ cursor: dragging ? 'col-resize' : undefined }}>
                {/* LEFT: Problem content (splitPos% width) */}
                <div className="space-y-4 overflow-y-auto pr-1" style={{ width: `${splitPos}%`, minWidth: '20%' }}>
                    {/* Back link */}
                    <Link
                        to={`/contests/${contestId}`}
                        className="text-blue-600 dark:text-blue-400 hover:text-blue-800 hover:underline text-sm font-medium transition-colors"
                    >
                        ← {contest?.title || 'Contest'}
                    </Link>
                    {/* Title with problem letter badge */}
                    <div className="flex items-center gap-3 mt-3">
                        <span className="inline-flex items-center justify-center w-10 h-10 rounded-lg bg-blue-600 text-white font-bold text-lg flex-shrink-0">
                            {index?.toUpperCase()}
                        </span>
                        <h1 className="text-2xl font-bold text-gray-900 dark:text-gray-100 leading-tight">{problem.title}</h1>
                    </div>
                    {/* Metadata bar */}
                    <div className="flex flex-wrap items-center gap-4 mt-3 text-sm text-gray-600 dark:text-gray-400">
                        <span className="flex items-center gap-1.5">
                            <span className="font-semibold text-gray-700 dark:text-gray-300">Time Limit:</span> {problem.time_limit} ms
                        </span>
                        <span className="text-gray-300">|</span>
                        <span className="flex items-center gap-1.5">
                            <span className="font-semibold text-gray-700 dark:text-gray-300">Memory Limit:</span> {Math.round(problem.memory_limit / 1024)} MB
                        </span>
                        {problem.interactive && (
                            <>
                                <span className="text-gray-300">|</span>
                                <span className="inline-block px-2 py-0.5 rounded text-xs font-semibold bg-purple-100 dark:bg-purple-900/30 text-purple-800 dark:text-purple-300">
                                    Interactive
                                </span>
                            </>
                        )}
                        {problem.problem_type === 'output' && (
                            <>
                                <span className="text-gray-300">|</span>
                                <span className="inline-block px-2 py-0.5 rounded text-xs font-semibold bg-amber-100 dark:bg-amber-900/30 text-amber-800 dark:text-amber-300">
                                    Output-Only
                                </span>
                            </>
                        )}
                        {problem.scoring_mode === 'partial' && (
                            <>
                                <span className="text-gray-300">|</span>
                                <span className="inline-block px-2 py-0.5 rounded text-xs font-semibold bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300">
                                    Partial Scoring
                                </span>
                            </>
                        )}
                    </div>
                    {/* Tabs */}
                    <div className="flex border-b border-gray-200 dark:border-gray-700 mb-6">
                        <button
                            onClick={() => setTab('statement')}
                            className={`px-4 py-2.5 text-sm font-medium border-b-2 -mb-px transition-colors ${
                                tab === 'statement'
                                    ? 'border-blue-600 text-blue-600 dark:text-blue-400'
                                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                            }`}
                        >
                            Statement
                        </button>
                        {isLoggedIn && (
                            <button
                                onClick={() => setTab('submissions')}
                                className={`px-4 py-2.5 text-sm font-medium border-b-2 -mb-px transition-colors ${
                                    tab === 'submissions'
                                         ? 'border-blue-600 text-blue-600 dark:text-blue-400'
                                         : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
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
                                            <h3 className="font-semibold text-sm text-gray-700 dark:text-gray-300 uppercase tracking-wide">PDF Problem Statement</h3>
                                            <a
                                                href={problem.description}
                                                target="_blank"
                                                rel="noopener noreferrer"
                                                className="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-900/30 border border-blue-200 dark:border-blue-800 rounded-lg hover:bg-blue-100 dark:hover:bg-blue-900/50 transition-colors"
                                            >
                                                <Download className="w-4 h-4" />
                                                Download PDF
                                            </a>
                                            <iframe
                                                src={problem.description}
                                                className="w-full h-[800px] border-0 rounded-lg shadow-sm bg-white"
                                                title="Problem Statement"
                                            />
                                        </div>
                                    ) : (
                                        <div className="prose prose-sm max-w-none text-gray-800 dark:text-gray-300 leading-relaxed">
                                            <ReactMarkdown remarkPlugins={[remarkMath]} rehypePlugins={[rehypeKatex]}>
                                                {problem.description}
                                            </ReactMarkdown>
                                        </div>
                                    )}

                                    {problem.input_format && (
                                        <div>
                                            <h3 className="font-semibold text-sm text-gray-700 dark:text-gray-300 uppercase tracking-wide mb-2">Input Format</h3>
                                            <div className="prose prose-sm max-w-none text-gray-700 dark:text-gray-300 bg-gray-50 dark:bg-gray-900/20 rounded-lg p-4 border border-gray-200 dark:border-gray-700">
                                                <ReactMarkdown remarkPlugins={[remarkMath]} rehypePlugins={[rehypeKatex]}>
                                                    {problem.input_format}
                                                </ReactMarkdown>
                                            </div>
                                        </div>
                                    )}

                                    {problem.output_format && (
                                        <div>
                                            <h3 className="font-semibold text-sm text-gray-700 dark:text-gray-300 uppercase tracking-wide mb-2">Output Format</h3>
                                            <div className="prose prose-sm max-w-none text-gray-700 dark:text-gray-300 bg-gray-50 dark:bg-gray-900/20 rounded-lg p-4 border border-gray-200 dark:border-gray-700">
                                                <ReactMarkdown remarkPlugins={[remarkMath]} rehypePlugins={[rehypeKatex]}>
                                                    {problem.output_format}
                                                </ReactMarkdown>
                                            </div>
                                        </div>
                                    )}
                                </>
                            )}

                            {/* Sample cases - always show even with hidden statement */}
                            {problem.sample_cases && problem.sample_cases.length > 0 && (
                                <div>
                                    <h3 className="font-semibold text-sm text-gray-700 dark:text-gray-300 uppercase tracking-wide mb-3">Sample Cases</h3>
                                    <div className="space-y-4">
                                        {problem.sample_cases.map((sample: any, i: number) => (
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

                {/* DIVIDER */}
                <div
                    onMouseDown={handleMouseDown}
                    className={`w-3 flex-shrink-0 cursor-col-resize group transition-colors relative ${dragging ? 'bg-blue-500' : 'bg-gray-200 dark:bg-gray-700 hover:bg-blue-500'}`}
                >
                    {/* Grip dots indicator */}
                    <div className="absolute inset-0 flex flex-col items-center justify-center gap-1">
                        <span className="w-1 h-1 rounded-full bg-gray-400 dark:bg-gray-500 group-hover:bg-white/70 transition-colors" />
                        <span className="w-1 h-1 rounded-full bg-gray-400 dark:bg-gray-500 group-hover:bg-white/70 transition-colors" />
                        <span className="w-1 h-1 rounded-full bg-gray-400 dark:bg-gray-500 group-hover:bg-white/70 transition-colors" />
                    </div>
                </div>

                {/* RIGHT: Submit panel (100-splitPos% width) */}
                <div className="flex flex-col gap-3 overflow-y-auto pl-1" style={{ width: `${100 - splitPos}%`, minWidth: '20%' }}>
                    {showSubmit ? (
                        <>
                            <div className="flex items-center justify-between">
                                <select
                                    value={language}
                                    onChange={e => setLanguage(e.target.value)}
                                    className="border border-gray-300 dark:border-gray-600 rounded px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
                                >
                                    {LANGS.map(l => <option key={l.value} value={l.value}>{l.label}</option>)}
                                </select>
                                <div className="flex gap-2">
                                    <button
                                        onClick={testWithSamples}
                                        disabled={runningSamples || !code.trim() || problem?.source !== 'local'}
                                        title={problem?.source !== 'local' ? 'Test Samples is only available for local problems' : undefined}
                                        className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50 transition-colors cursor-pointer"
                                    >
                                        {runningSamples ? 'Testing...' : 'Test Samples'}
                                    </button>
                                    <button
                                        onClick={handleSubmit}
                                        disabled={submitting || !code.trim()}
                                        className="bg-green-600 text-white px-4 py-1.5 rounded text-sm font-medium hover:bg-green-700 disabled:opacity-50 transition-colors cursor-pointer"
                                    >
                                        {submitting ? 'Submitting...' : 'Submit'}
                                    </button>
                                </div>
                            </div>
                            <CodeEditor
                                language={language}
                                value={code}
                                onChange={handleCodeChange}
                                height="400px"
                            />

                            {result && <SubmissionResult result={result} />}

                            {sampleResults.length > 0 && (
                                <div className="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden bg-white dark:bg-gray-800 shadow-sm">
                                    <div className="bg-gray-50 dark:bg-gray-855 border-b border-gray-200 dark:border-gray-700 px-4 py-3 flex items-center justify-between">
                                        <span className="font-semibold text-sm text-gray-700 dark:text-gray-300">Sample Test Results</span>
                                        <span className={`text-xs font-medium px-2 py-0.5 rounded ${
                                            sampleResults.every(r => r.passed) ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300' :
                                            sampleResults.some(r => r.passed) ? 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-300' :
                                            'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300'
                                        }`}>
                                            {sampleResults.filter(r => r.passed).length}/{sampleResults.length} Passed
                                        </span>
                                    </div>
                                    <div className="divide-y divide-gray-100 dark:divide-gray-700">
                                        {sampleResults.map((r) => (
                                            <div key={r.index} className="p-4">
                                                <div className="flex items-center justify-between mb-2">
                                                    <span className="text-sm font-medium text-gray-700 dark:text-gray-300">Sample {r.index}</span>
                                                    <div className="flex items-center gap-2">
                                                        {r.time > 0 && (
                                                            <span className="text-xs text-gray-500 dark:text-gray-400">{r.time}ms</span>
                                                        )}
                                                        <span className={`text-xs font-medium px-2 py-0.5 rounded ${
                                                            r.passed ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300' : 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300'
                                                        }`}>
                                                            {r.passed ? 'Passed' : 'Failed'}
                                                        </span>
                                                    </div>
                                                </div>
                                                <div className="text-xs text-gray-500 dark:text-gray-400 mb-2 font-mono bg-gray-50 dark:bg-gray-800 rounded p-2 overflow-x-auto">
                                                    Input: {r.input.substring(0, 100)}{r.input.length > 100 ? '...' : ''}
                                                </div>
                                                {!r.passed && (
                                                    <div className="grid grid-cols-2 gap-2">
                                                        <div>
                                                            <span className="block text-xs font-semibold text-gray-500 dark:text-gray-400 mb-1">Expected</span>
                                                            <pre className="bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-300 font-mono text-xs p-2 rounded overflow-x-auto max-h-24 border border-green-100">{r.expected || '(empty)'}</pre>
                                                        </div>
                                                        <div>
                                                            <span className="block text-xs font-semibold text-gray-500 dark:text-gray-400 mb-1">Actual</span>
                                                            <pre className="bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300 font-mono text-xs p-2 rounded overflow-x-auto max-h-24 border border-red-100">{r.actual || '(empty)'}</pre>
                                                        </div>
                                                    </div>
                                                )}
                                                {r.compile_output && (
                                                    <pre className="mt-2 text-xs text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20 rounded p-2 overflow-x-auto">{r.compile_output}</pre>
                                                )}
                                                {r.stderr && (
                                                    <pre className="mt-2 text-xs text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20 rounded p-2 overflow-x-auto">{r.stderr}</pre>
                                                )}
                                            </div>
                                        ))}
                                    </div>
                                </div>
                            )}

                            <div className="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden bg-white dark:bg-gray-800 shadow-sm">
                                <div className="bg-gray-50 dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 px-4 py-3 flex items-center justify-between">
                                    <span className="font-semibold text-sm text-gray-700 dark:text-gray-300">Custom Stdin / Scratchpad</span>
                                    <button
                                        onClick={runCustomCode}
                                        disabled={runningCustom || !code.trim() || problem?.source !== 'local'}
                                        title={problem?.source !== 'local' ? 'Custom run is only available for local problems' : undefined}
                                        className="bg-blue-600 hover:bg-blue-700 text-white text-xs font-semibold px-3 py-1.5 rounded disabled:opacity-50 transition-colors cursor-pointer"
                                    >
                                        {runningCustom ? 'Running...' : 'Run Code'}
                                    </button>
                                </div>
                                <div className="p-4 space-y-4">
                                    <div>
                                        <label className="block text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider mb-1.5">Custom Input (Stdin)</label>
                                        <textarea
                                            value={customInput}
                                            onChange={(e) => setCustomInput(e.target.value)}
                                            rows={4}
                                            placeholder="Enter input values here..."
                                            className="w-full font-mono text-xs border border-gray-300 dark:border-gray-600 rounded p-2.5 bg-gray-50 dark:bg-gray-800 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400 transition-colors"
                                        />
                                    </div>

                                    {customOutput && (
                                        <div className="space-y-3 pt-2 border-t border-gray-100 dark:border-gray-700">
                                            <div className="flex items-center justify-between text-xs">
                                                <span className={`font-semibold uppercase tracking-wider ${
                                                    customOutput.status === 'success' ? 'text-green-600 dark:text-green-400' :
                                                    customOutput.status === 'ce' ? 'text-purple-600 dark:text-purple-400' : 'text-red-600 dark:text-red-400'
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
                                                    <span className="text-gray-500 dark:text-gray-400 font-mono">
                                                        {customOutput.time_used}ms / {Math.round(customOutput.memory_used / 1024)}MB
                                                    </span>
                                                )}
                                            </div>

                                            {customOutput.compile_output && (
                                                <div>
                                                    <span className="block text-xs font-semibold text-red-500 dark:text-red-400 mb-1">Compiler Output</span>
                                                    <pre className="bg-red-50 dark:bg-red-900/20 text-red-600 dark:text-red-400 font-mono text-xs p-3 rounded overflow-x-auto max-h-40 border border-red-100">{customOutput.compile_output}</pre>
                                                </div>
                                            )}

                                            {customOutput.status !== 'ce' && (
                                                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                                                    <div>
                                                        <span className="block text-xs font-semibold text-gray-500 dark:text-gray-400 mb-1">Standard Output (Stdout)</span>
                                                        <pre className="bg-gray-900 text-gray-100 font-mono text-xs p-3 rounded overflow-x-auto max-h-40">{customOutput.stdout || <span className="text-gray-500 dark:text-gray-400 italic">No output</span>}</pre>
                                                    </div>
                                                    <div>
                                                        <span className="block text-xs font-semibold text-gray-500 dark:text-gray-400 mb-1">Standard Error (Stderr)</span>
                                                        <pre className="bg-gray-950 text-red-400 font-mono text-xs p-3 rounded overflow-x-auto max-h-40">{customOutput.stderr || <span className="text-gray-600 dark:text-gray-400 italic">No stderr</span>}</pre>
                                                    </div>
                                                </div>
                                            )}
                                        </div>
                                    )}
                                </div>
                            </div>
                        </>
                    ) : !isLoggedIn ? (
                        <div className="p-5 bg-gray-50 dark:bg-gray-900/20 border border-gray-200 dark:border-gray-700 rounded-lg text-center">
                            <h3 className="font-semibold text-gray-800 dark:text-gray-300 mb-2">Login Required</h3>
                            <p className="text-gray-600 dark:text-gray-400 text-sm mb-3">
                                Please login to submit a solution.
                            </p>
                            <Link
                                to="/login"
                                className="inline-block px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 transition-colors"
                            >
                                Login
                            </Link>
                        </div>
                    ) : !canSubmit ? (
                        <div className="p-5 bg-gray-50 dark:bg-gray-900/20 border border-gray-200 dark:border-gray-700 rounded-lg text-center">
                            <h3 className="font-semibold text-gray-800 dark:text-gray-300 mb-2">Submissions Closed</h3>
                            <p className="text-gray-600 dark:text-gray-400 text-sm">
                                {upsolvingDisabled
                                    ? 'Upsolving is disabled for this contest.'
                                    : 'Submissions are not allowed for this problem.'}
                            </p>
                        </div>
                    ) : null}

                    {/* Quick links */}
                    <div className="p-4 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg">
                        <h4 className="text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase tracking-wide mb-2">Quick Links</h4>
                        <div className="space-y-1.5">
                            <Link
                                to={`/contests/${contestId}`}
                                className="block text-sm text-blue-600 dark:text-blue-400 hover:underline"
                            >
                                ← Contest Dashboard
                            </Link>
                            <Link
                                to={`/contests/${contestId}/scoreboard`}
                                className="block text-sm text-blue-600 dark:text-blue-400 hover:underline"
                            >
                                <BarChart3 className="w-3.5 h-3.5" /> Scoreboard
                            </Link>
                            {isLoggedIn && (
                                <Link
                                    to={`/submissions?problem_id=${problem.id}&contest_id=${contestId}`}
                                    className="block text-sm text-blue-600 dark:text-blue-400 hover:underline"
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
