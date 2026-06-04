import { useEffect, useState, useRef } from 'react'
import { useParams, useSearchParams, Link } from 'react-router-dom'
import ReactMarkdown from 'react-markdown'
import remarkMath from 'remark-math'
import rehypeKatex from 'rehype-katex'
import 'katex/dist/katex.min.css'
import { api, getAccessToken } from '../lib/api'
import ProblemStats from '../components/ProblemStats'
import CodeEditor from '../components/CodeEditor'
import AddEditorialModal from '../components/AddEditorialModal'
import { Download } from 'lucide-react'

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

const LANGS = [
    { value: 'cpp-gpp-64', label: 'GNU G++17 7.3.0 (64 bit)' },
    { value: 'cpp-gpp-32', label: 'GNU G++14 6.4.0 (32 bit)' },
    { value: 'c-gcc-64', label: 'GNU GCC C11 9.2.0 (64 bit)' },
    { value: 'c-gcc-32', label: 'GNU GCC C11 9.2.0 (32 bit)' },
    { value: 'cpp-clang', label: 'Clang++17' },
    { value: 'python', label: 'Python 3.8.10' },
    { value: 'java', label: 'Java 11.0.6' },
    { value: 'rust', label: 'Rust 1.75.0' },
    { value: 'nodejs', label: 'Node.js 18.16.1' },
    { value: 'csharp', label: 'Mono C# 6.12.0' },
]

const TEMPLATE_CODE: Record<string, string> = {
    'cpp-gpp-64': `#include <iostream>
using namespace std;

int main() {
    // Your code here
    
    return 0;
}`,
    'cpp-gpp-32': `#include <iostream>
using namespace std;

int main() {
    // Your code here
    
    return 0;
}`,
    'c-gcc-64': `#include <stdio.h>

int main() {
    // Your code here
    
    return 0;
}`,
    'c-gcc-32': `#include <stdio.h>

int main() {
    // Your code here
    
    return 0;
}`,
    'cpp-clang': `#include <iostream>
using namespace std;

int main() {
    // Your code here
    
    return 0;
}`,
    'python': `# Your code here
`,
    'java': `import java.util.Scanner;

public class Main {
    public static void main(String[] args) {
        Scanner sc = new Scanner(System.in);
        // Your code here
        
    }
}`,
    'rust': `fn main() {
    // Your code here
    
}`,
    'nodejs': `// Your code here
`,
    'csharp': `using System;

class Program {
    static void Main() {
        // Your code here
        
    }
}`,
}

const STATUS_COLORS: Record<string, string> = {
    ac: 'text-green-600 dark:text-green-400', success: 'text-green-600 dark:text-green-400',
    wa: 'text-red-600 dark:text-red-400',
    tle: 'text-yellow-600 dark:text-yellow-400', TLE: 'text-yellow-600 dark:text-yellow-400',
    mle: 'text-orange-600 dark:text-orange-400', MLE: 'text-orange-600 dark:text-orange-400',
    re: 'text-red-700 dark:text-red-300', RE: 'text-red-700 dark:text-red-300',
    ce: 'text-purple-600 dark:text-purple-400', CE: 'text-purple-600 dark:text-purple-400',
    pending: 'text-blue-500 dark:text-blue-400', judging: 'text-blue-600 dark:text-blue-400',
    se: 'text-gray-600 dark:text-gray-400', SE: 'text-gray-600 dark:text-gray-400',
}

const STATUS_LABELS: Record<string, string> = {
    ac: 'Accepted', success: 'Accepted',
    wa: 'Wrong Answer',
    tle: 'Time Limit Exceeded', TLE: 'Time Limit Exceeded',
    mle: 'Memory Limit Exceeded', MLE: 'Memory Limit Exceeded',
    re: 'Runtime Error', RE: 'Runtime Error',
    ce: 'Compile Error', CE: 'Compile Error',
    pending: 'Pending', judging: 'Judging...',
    se: 'System Error', SE: 'System Error',
}

export default function ProblemDetail() {
    const { slug } = useParams<{ slug: string }>()
    const [searchParams] = useSearchParams()
    const isUpsolving = searchParams.get('upsolving') === 'true'
    const contestId = searchParams.get('contest')
    const [problem, setProblem] = useState<any>(null)
    const [lang, setLang] = useState('cpp-gpp-64')
    const [code, setCode] = useState('')
    const [result, setResult] = useState<any>(null)
    const [submitting, setSubmitting] = useState(false)
    const [customInput, setCustomInput] = useState('')
    const [customOutput, setCustomOutput] = useState<any>(null)
    const [runningCustom, setRunningCustom] = useState(false)
    const [sampleResults, setSampleResults] = useState<any[]>([])
    const [runningSamples, setRunningSamples] = useState(false)
    const [lastSubmitTime, setLastSubmitTime] = useState(0)
    const [lastTestTime, setLastTestTime] = useState(0)
    const [tab, setTab] = useState<'statement' | 'stats' | 'editorials' | 'submissions' | 'more'>('statement')
    const [mySubs, setMySubs] = useState<any[]>([])
    const [loadingSubs, setLoadingSubs] = useState(false)
    const [editorials, setEditorials] = useState<any[]>([])
    const [isAddModalOpen, setIsAddModalOpen] = useState(false)
    const isMountedRef = useRef(true)

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
        if (slug) api.problems.get(slug).then(setProblem).catch(() => {})
    }, [slug])

    useEffect(() => {
        if (problem?.id) api.editorials.getByProblem(problem.id).then(d => setEditorials(d.data || [])).catch(() => {})
    }, [problem?.id])

    useEffect(() => {
        if (tab === 'submissions' && problem?.id) {
            setLoadingSubs(true)
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
            setCode(saved || TEMPLATE_CODE[lang] || '')
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
        if (!code.trim()) { alert('Please write some code'); return }
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
        } catch (e: any) {
            alert('Custom run failed: ' + e.message)
        } finally {
            if (isMountedRef.current) {
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
            if (isMountedRef.current) {
                setSampleResults([...results])
            }
        }
        if (isMountedRef.current) {
            setRunningSamples(false)
        }
    }

    const submit = async () => {
        if (!getAccessToken()) { alert('Please login first'); return }
        if (!code.trim()) { alert('Please write some code'); return }
        const now = Date.now()
        if (now - lastSubmitTime < 5000) { alert('Please wait 5 seconds between submissions'); return }
        setLastSubmitTime(now)
        setSubmitting(true)
        setResult(null)
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
            if (isMountedRef.current) {
                setResult(res)
            }
            
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
                    if ((updated.status === 'pending' || updated.status === 'judging') && retries < maxRetries) {
                        retries++
                        poll()
                    }
                } catch {
                    if ((retries < maxRetries) && isMountedRef.current) {
                        retries++
                        poll()
                    }
                }
            }
            poll()
        } catch (e: any) {
            alert('Submit failed: ' + e.message)
        } finally {
            if (isMountedRef.current) {
                setSubmitting(false)
            }
        }
    }

    const role = decodeRole()
    const canExport = role === 'admin' || role === 'teacher'

    if (!problem) {
        return <div className="text-center py-20 text-gray-400 dark:text-gray-500">Loading...</div>
    }

    return (
        <div ref={containerRef} className="flex h-full select-none" style={{ cursor: dragging ? 'col-resize' : undefined }}>
            {/* Problem Statement */}
            <div className="space-y-4 overflow-y-auto pr-1" style={{ width: `${splitPos}%`, minWidth: '20%' }}>
                <div>
                    <h1 className="text-2xl font-bold">
                        {problem.title}
                        {problem.interactive && (
                            <span className="inline-flex items-center ml-2 px-2 py-0.5 text-xs font-semibold bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200 rounded align-middle">
                                Interactive
                            </span>
                        )}
                        {problem.problem_type === 'output' && (
                            <span className="inline-flex items-center ml-2 px-2 py-0.5 text-xs font-semibold bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200 rounded align-middle">
                                Output-Only
                            </span>
                        )}
                        {problem.scoring_mode === 'partial' && (
                            <span className="inline-flex items-center ml-2 px-2 py-0.5 text-xs font-semibold bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:bg-blue-900 dark:text-blue-200 rounded align-middle">
                                Partial Scoring
                            </span>
                        )}
                    </h1>
                    <div className="mt-3 text-sm text-gray-600 dark:text-gray-400 flex flex-wrap items-center gap-4">
                        <span className="flex items-center gap-1.5"><span className="font-semibold text-gray-700 dark:text-gray-300">Time Limit:</span> {problem.time_limit} ms</span>
                        <span className="flex items-center gap-1.5"><span className="font-semibold text-gray-700 dark:text-gray-300">Memory Limit:</span> {Math.round(problem.memory_limit / 1024)} MB</span>
                        <span className="flex items-center gap-1.5"><span className="font-semibold text-gray-700 dark:text-gray-300">Difficulty:</span> <span className={`inline-block px-2 py-0.5 rounded text-xs font-semibold uppercase tracking-wider ${
                            problem.difficulty === 'easy' ? 'bg-green-100 dark:bg-green-900/30 text-green-800' :
                            problem.difficulty === 'hard' ? 'bg-red-100 dark:bg-red-900/30 text-red-800' :
                            'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800'
                        }`}>{problem.difficulty}</span></span>
                    </div>
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
                                    } catch (e: any) {
                                        alert('Export failed: ' + e.message)
                                    }
                                }}
                                className="inline-flex items-center px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700 cursor-pointer"
                            >
                                Export (FPS ZIP)
                            </button>
                        </div>
                    )}
                </div>

                <div className="flex border-b border-gray-200 dark:border-gray-700">
                    <button onClick={() => setTab('statement')} className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px ${tab === 'statement' ? 'border-blue-600 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'}`}>
                        Statement
                    </button>
                    <button onClick={() => setTab('stats')} className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px ${tab === 'stats' ? 'border-blue-600 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'}`}>
                        Statistics
                    </button>
                    <button onClick={() => setTab('editorials')} className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px ${tab === 'editorials' ? 'border-blue-600 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'}`}>
                        Editorials
                    </button>
                    {getAccessToken() && (
                        <button onClick={() => setTab('submissions')} className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px ${tab === 'submissions' ? 'border-blue-600 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'}`}>
                            My Submissions
                        </button>
                    )}
                    <button onClick={() => setTab('more')} className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px ${tab === 'more' ? 'border-blue-600 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'}`}>
                        More
                    </button>
                </div>

                {tab === 'statement' ? (
                    <div className="space-y-5 pb-8">
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
                            <div className="prose prose-sm max-w-none text-gray-800 dark:text-gray-200 leading-relaxed">
                                <ReactMarkdown remarkPlugins={[remarkMath]} rehypePlugins={[rehypeKatex]}>
                                    {problem.description}
                                </ReactMarkdown>
                            </div>
                        )}
                        {problem.input_format && (
                            <div>
                                <h3 className="font-semibold text-sm text-gray-700 dark:text-gray-300 uppercase tracking-wide mb-1">Input Format</h3>
                                <div className="prose prose-sm max-w-none text-gray-700 dark:text-gray-300">
                                    <ReactMarkdown remarkPlugins={[remarkMath]} rehypePlugins={[rehypeKatex]}>
                                        {problem.input_format}
                                    </ReactMarkdown>
                                </div>
                            </div>
                        )}
                        {problem.output_format && (
                            <div>
                                <h3 className="font-semibold text-sm text-gray-700 dark:text-gray-300 uppercase tracking-wide mb-1">Output Format</h3>
                                <div className="prose prose-sm max-w-none text-gray-700 dark:text-gray-300">
                                    <ReactMarkdown remarkPlugins={[remarkMath]} rehypePlugins={[rehypeKatex]}>
                                        {problem.output_format}
                                    </ReactMarkdown>
                                </div>
                            </div>
                        )}
                        {problem.hint && (
                            <div>
                                <h3 className="font-semibold text-sm text-gray-700 dark:text-gray-300 uppercase tracking-wide mb-1">Constraints</h3>
                                <div className="prose prose-sm max-w-none text-gray-700 dark:text-gray-300">
                                    <ReactMarkdown remarkPlugins={[remarkMath]} rehypePlugins={[rehypeKatex]}>
                                        {problem.hint}
                                    </ReactMarkdown>
                                </div>
                            </div>
                        )}
                        {problem.sample_cases?.length > 0 && problem.sample_cases.map((sc: any, i: number) => (
                            <div key={i} className="space-y-1">
                                <h3 className="font-semibold text-sm text-gray-700 dark:text-gray-300 uppercase tracking-wide">Sample {i + 1}</h3>
                                <div className="grid grid-cols-2 gap-2">
                                    <div>
                                        <div className="text-xs text-gray-400 dark:text-gray-500 mb-1">Input</div>
                                        <pre className="bg-gray-100 dark:bg-gray-700 rounded p-2 text-xs overflow-x-auto select-all">{sc.input}</pre>
                                    </div>
                                    <div>
                                        <div className="text-xs text-gray-400 dark:text-gray-500 mb-1">Output</div>
                                        <pre className="bg-gray-100 dark:bg-gray-700 rounded p-2 text-xs overflow-x-auto select-all">{sc.output}</pre>
                                    </div>
                                </div>
                                {sc.explanation && <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">{sc.explanation}</p>}
                            </div>
                        ))}
                    </div>
                ) : tab === 'more' ? (
                    <div className="space-y-6 max-w-2xl py-2">
                        {problem.interactive && (
                            <div className="bg-purple-50 dark:bg-purple-950 border border-purple-200 dark:border-purple-800 rounded-lg p-4">
                                <h3 className="font-semibold text-purple-800 dark:text-purple-300 mb-1">Interactive Problem</h3>
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
                                <h3 className="font-semibold text-gray-800 dark:text-gray-200 mb-3">Tags</h3>
                                <div className="flex flex-wrap gap-2">
                                    {problem.tags.map((tag: string) => (
                                        <Link 
                                            key={tag} 
                                            to={`/problems?tag=${tag}`} 
                                            className="bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 border border-gray-200 dark:border-gray-700 text-sm text-gray-700 dark:text-gray-300 px-3 py-1.5 rounded transition-colors"
                                        >
                                            {tag}
                                        </Link>
                                    ))}
                                </div>
                            </div>
                        )}
                        {problem.source && (
                            <div>
                                <h3 className="font-semibold text-gray-800 dark:text-gray-200 mb-2">Source / Author</h3>
                                <div className="text-sm text-gray-700 dark:text-gray-300 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 px-4 py-2 rounded-lg inline-block">
                                    {problem.source}
                                </div>
                            </div>
                        )}
                        {(!problem.tags?.length && !problem.source && !problem.interactive && problem.scoring_mode !== 'partial') && (
                            <p className="text-gray-500 dark:text-gray-400 text-sm py-4">No additional information available for this problem.</p>
                        )}
                    </div>
                ) : tab === 'stats' ? (
                    <ProblemStats problemId={problem.id} />
                ) : tab === 'editorials' ? (
                    <div className="space-y-4">
                        {(() => {
                            const role = decodeRole()
                            const isPrivileged = !!getAccessToken() && (role === 'admin' || role === 'teacher')
                            if (isPrivileged) {
                                return (
                                    <div className="flex justify-end">
                                        <button
                                            onClick={() => setIsAddModalOpen(true)}
                                            className="bg-blue-600 hover:bg-blue-700 text-white px-3 py-1.5 rounded text-sm font-medium"
                                        >
                                            Add Editorial
                                        </button>
                                    </div>
                                )
                            }
                            return null
                        })()}
                        {editorials.length === 0 ? (
                            <p className="text-gray-400 dark:text-gray-500 text-sm">No editorials yet for this problem.</p>
                        ) : (
                            editorials.map(e => (
                                <Link key={e.id} to={`/editorials/${e.id}`} className="block border rounded p-4 hover:bg-gray-50 dark:hover:bg-gray-700">
                                    <div className="flex items-center gap-2 mb-1">
                                        {e.is_official && <span className="text-xs bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300 px-2 py-0.5 rounded font-medium">Official</span>}
                                        <h4 className="font-medium">{e.title}</h4>
                                    </div>
                                    <div className="flex gap-4 text-xs text-gray-400 dark:text-gray-500">
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
                            <div className="text-center py-8 text-gray-400 dark:text-gray-500">Loading submissions...</div>
                        ) : mySubs.length === 0 ? (
                            <p className="text-gray-400 dark:text-gray-500 text-sm text-center py-8">No submissions yet for this problem.</p>
                        ) : (
                            <div className="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
                                <table className="w-full text-sm">
                                    <thead className="bg-gray-50 dark:bg-gray-800 text-gray-500 dark:text-gray-400 text-xs uppercase">
                                        <tr>
                                            <th className="px-4 py-2 text-left">ID</th>
                                            <th className="px-4 py-2 text-left">Language</th>
                                            <th className="px-4 py-2 text-left">Verdict</th>
                                            <th className="px-4 py-2 text-left">Time</th>
                                            <th className="px-4 py-2 text-left">Memory</th>
                                        </tr>
                                    </thead>
                                    <tbody className="divide-y divide-gray-100 dark:divide-gray-700">
                                        {mySubs.map((s: any) => (
                                            <tr key={s.id} className="hover:bg-gray-50 dark:hover:bg-gray-700">
                                                <td className="px-4 py-2 font-mono text-xs">
                                                    <Link to={`/submissions/${s.id}`} className="text-blue-600 dark:text-blue-400 hover:underline">
                                                        {s.id?.substring(0, 8)}...
                                                    </Link>
                                                </td>
                                                <td className="px-4 py-2 text-gray-500 dark:text-gray-400">{s.language}</td>
                                                <td className="px-4 py-2 font-semibold">
                                                    <span className={`px-2 py-0.5 rounded text-xs font-medium ${
                                                        s.status === 'ac' ? 'text-green-600 dark:text-green-400 bg-green-50 dark:bg-green-900/20' : 
                                                        s.status === 'wa' ? 'text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20' : 
                                                        s.status === 'tle' ? 'text-yellow-600 dark:text-yellow-400 bg-yellow-50 dark:bg-yellow-900/20' :
                                                        s.status === 'mle' ? 'text-orange-600 dark:text-orange-400 bg-orange-50 dark:bg-orange-900/20' :
                                                        s.status === 're' ? 'text-red-700 dark:text-red-300 bg-red-50 dark:bg-red-900/20' :
                                                        s.status === 'ce' ? 'text-purple-600 dark:text-purple-400 bg-purple-50 dark:bg-purple-900/20' :
                                                        s.status === 'pending' ? 'text-blue-500 dark:text-blue-400 bg-blue-50 dark:bg-blue-900/20' :
                                                        s.status === 'judging' ? 'text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-900/20' :
                                                        'text-gray-600 dark:text-gray-400 bg-gray-50 dark:bg-gray-800'
                                                    }`}>
                                                        {s.status === 'ac' ? 'Accepted' : 
                                                         s.status === 'wa' ? 'Wrong Answer' : 
                                                         s.status === 'tle' ? 'Time Limit Exceeded' :
                                                         s.status === 'mle' ? 'Memory Limit Exceeded' :
                                                         s.status === 're' ? 'Runtime Error' :
                                                         s.status === 'ce' ? 'Compile Error' :
                                                         s.status === 'pending' ? 'Pending' :
                                                         s.status === 'judging' ? 'Judging...' :
                                                         s.status}
                                                    </span>
                                                </td>
                                                <td className="px-4 py-2 text-gray-500 dark:text-gray-400">{s.time_used > 0 ? `${s.time_used}ms` : '—'}</td>
                                                <td className="px-4 py-2 text-gray-500 dark:text-gray-400">{s.memory_used > 0 ? `${Math.round(s.memory_used / 1024)}MB` : '—'}</td>
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
                className={`w-1.5 flex-shrink-0 cursor-col-resize group hover:bg-blue-500 transition-colors ${dragging ? 'bg-blue-500' : 'bg-gray-200 dark:bg-gray-700'}`}
            >
                <div className="w-0.5 h-full mx-auto group-hover:bg-blue-400" />
            </div>

            {/* IDE */}
            <div className="flex flex-col gap-3 overflow-y-auto pl-1" style={{ width: `${100 - splitPos}%`, minWidth: '20%' }}>
                <div className="flex items-center justify-between">
                    <select
                        value={lang}
                        onChange={e => setLang(e.target.value)}
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
                            onClick={submit}
                            disabled={submitting}
                            className="bg-green-600 text-white px-4 py-1.5 rounded text-sm font-medium hover:bg-green-700 disabled:opacity-50 transition-colors cursor-pointer"
                        >
                            {submitting ? 'Submitting...' : 'Submit'}
                        </button>
                    </div>
                </div>
                <CodeEditor
                    language={lang}
                    value={code}
                    onChange={handleCodeChange}
                    height="400px"
                />
                {result && (
                    <div className="border border-gray-200 dark:border-gray-700 rounded p-3 text-sm">
                        <div className="flex justify-between items-center">
                            <span className={`font-semibold ${STATUS_COLORS[result.status] || ''}`}>
                                {STATUS_LABELS[result.status] || result.status}
                            </span>
                            {result.time_used > 0 && (
                                <span className="text-gray-500 dark:text-gray-400 text-xs">
                                    {result.time_used}ms / {Math.round(result.memory_used / 1024)}MB
                                </span>
                            )}
                        </div>
                        {result.compile_output && (
                            <pre className="mt-2 text-xs text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20 rounded p-2 overflow-x-auto">{result.compile_output}</pre>
                        )}
                    </div>
                )}

                {/* Sample Test Results */}
                {sampleResults.length > 0 && (
                    <div className="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden bg-white dark:bg-gray-800 shadow-sm">
                        <div className="bg-gray-50 dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 px-4 py-3 flex items-center justify-between">
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

                {/* Custom Scratchpad */}
                <div className="mt-4 border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden bg-white dark:bg-gray-800 shadow-sm">
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
            </div>
        </div>
    )
}