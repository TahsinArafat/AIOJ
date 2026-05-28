import { useEffect, useRef, useState } from 'react'
import { useParams, useSearchParams, Link } from 'react-router-dom'
import { EditorView, basicSetup } from 'codemirror'
import { EditorState } from '@codemirror/state'
import { oneDark } from '@codemirror/theme-one-dark'
import { cpp } from '@codemirror/lang-cpp'
import { python } from '@codemirror/lang-python'
import { java } from '@codemirror/lang-java'
import { rust } from '@codemirror/lang-rust'
import { api, getAccessToken } from '../lib/api'
import ProblemStats from '../components/ProblemStats'

const LANGS = [
    { value: 'cpp-gpp-64', label: 'C++ (G++ 64-bit)' },
    { value: 'cpp-gpp-32', label: 'C++ (G++ 32-bit)' },
    { value: 'c-gcc-64', label: 'C (GCC 64-bit)' },
    { value: 'c-gcc-32', label: 'C (GCC 32-bit)' },
    { value: 'cpp-clang', label: 'C++ (Clang)' },
    { value: 'python', label: 'Python 3' },
    { value: 'java', label: 'Java' },
    { value: 'rust', label: 'Rust' },
    { value: 'nodejs', label: 'Node.js' },
    { value: 'csharp', label: 'C# (Mono)' },
]

function getLangExtension(lang: string) {
    if (lang.startsWith('cpp') || lang.startsWith('c-')) return cpp()
    if (lang === 'python' || lang === 'pypy') return python()
    if (lang === 'java') return java()
    if (lang === 'rust') return rust()
    return cpp()
}

const STATUS_COLORS: Record<string, string> = {
    ac: 'text-green-600', wa: 'text-red-600', tle: 'text-yellow-600',
    mle: 'text-orange-600', re: 'text-red-700', ce: 'text-purple-600',
    pending: 'text-blue-500', judging: 'text-blue-600', se: 'text-gray-600',
}

const STATUS_LABELS: Record<string, string> = {
    ac: 'Accepted', wa: 'Wrong Answer', tle: 'Time Limit Exceeded',
    mle: 'Memory Limit Exceeded', re: 'Runtime Error', ce: 'Compile Error',
    pending: 'Pending', judging: 'Judging...', se: 'System Error',
}

export default function ProblemDetail() {
    const { slug } = useParams<{ slug: string }>()
    const [searchParams] = useSearchParams()
    const isUpsolving = searchParams.get('upsolving') === 'true'
    const contestId = searchParams.get('contest')
    const [problem, setProblem] = useState<any>(null)
    const [lang, setLang] = useState('cpp-gpp-64')
    const [result, setResult] = useState<any>(null)
    const [submitting, setSubmitting] = useState(false)
    const [tab, setTab] = useState<'statement' | 'stats' | 'editorials'>('statement')
    const [editorials, setEditorials] = useState<any[]>([])
    const editorRef = useRef<HTMLDivElement>(null)
    const viewRef = useRef<EditorView | null>(null)

    useEffect(() => {
        if (slug) api.problems.get(slug).then(setProblem).catch(() => {})
    }, [slug])

    useEffect(() => {
        if (problem?.id) api.editorials.getByProblem(problem.id).then(d => setEditorials(d.data || [])).catch(() => {})
    }, [problem?.id])

    useEffect(() => {
        if (!editorRef.current) return
        viewRef.current?.destroy()
        const state = EditorState.create({
            doc: '',
            extensions: [basicSetup, oneDark, getLangExtension(lang)],
        })
        viewRef.current = new EditorView({ state, parent: editorRef.current })
        return () => { viewRef.current?.destroy() }
    }, [lang])

    const submit = async () => {
        if (!getAccessToken()) { alert('Please login first'); return }
        if (!viewRef.current) return
        const code = viewRef.current.state.doc.toString()
        if (!code.trim()) { alert('Please write some code'); return }
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
            setResult(res)
            // Poll for result
            const poll = async () => {
                await new Promise(r => setTimeout(r, 2000))
                const updated = await api.submissions.get(res.id)
                setResult(updated)
                if (updated.status === 'pending' || updated.status === 'judging') {
                    poll()
                }
            }
            poll()
        } catch (e: any) {
            alert('Submit failed: ' + e.message)
        } finally {
            setSubmitting(false)
        }
    }

    if (!problem) {
        return <div className="text-center py-20 text-gray-400">Loading...</div>
    }

    return (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 h-full">
            {/* Problem Statement */}
            <div className="space-y-4 overflow-y-auto">
                <div>
                    <h1 className="text-2xl font-bold">{problem.title}</h1>
                    <div className="flex gap-3 mt-1 text-sm text-gray-500">
                        <span>Time: {problem.time_limit}ms</span>
                        <span>Memory: {Math.round(problem.memory_limit / 1024)}MB</span>
                        <span className={`font-medium ${
                            problem.difficulty === 'easy' ? 'text-green-600' :
                            problem.difficulty === 'hard' ? 'text-red-600' : 'text-yellow-600'
                        }`}>{problem.difficulty}</span>
                    </div>
                </div>

                <div className="flex border-b border-gray-200">
                    <button onClick={() => setTab('statement')} className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px ${tab === 'statement' ? 'border-blue-600 text-blue-600' : 'border-transparent text-gray-500 hover:text-gray-700'}`}>
                        Statement
                    </button>
                    <button onClick={() => setTab('stats')} className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px ${tab === 'stats' ? 'border-blue-600 text-blue-600' : 'border-transparent text-gray-500 hover:text-gray-700'}`}>
                        Statistics
                    </button>
                    <button onClick={() => setTab('editorials')} className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px ${tab === 'editorials' ? 'border-blue-600 text-blue-600' : 'border-transparent text-gray-500 hover:text-gray-700'}`}>
                        Editorials
                    </button>
                </div>

                {tab === 'statement' ? (
                    <>
                        <div className="prose prose-sm max-w-none">
                            <p className="whitespace-pre-wrap text-gray-800">{problem.description}</p>
                        </div>
                        {problem.input_format && (
                            <div>
                                <h3 className="font-semibold text-sm text-gray-700 uppercase tracking-wide mb-1">Input Format</h3>
                                <p className="text-sm text-gray-700 whitespace-pre-wrap">{problem.input_format}</p>
                            </div>
                        )}
                        {problem.output_format && (
                            <div>
                                <h3 className="font-semibold text-sm text-gray-700 uppercase tracking-wide mb-1">Output Format</h3>
                                <p className="text-sm text-gray-700 whitespace-pre-wrap">{problem.output_format}</p>
                            </div>
                        )}
                        {problem.sample_cases?.length > 0 && problem.sample_cases.map((sc: any, i: number) => (
                            <div key={i}>
                                <h3 className="font-semibold text-sm text-gray-700 uppercase tracking-wide mb-1">Sample {i + 1}</h3>
                                <div className="grid grid-cols-2 gap-2">
                                    <div>
                                        <div className="text-xs text-gray-400 mb-1">Input</div>
                                        <pre className="bg-gray-100 rounded p-2 text-xs overflow-x-auto">{sc.input}</pre>
                                    </div>
                                    <div>
                                        <div className="text-xs text-gray-400 mb-1">Output</div>
                                        <pre className="bg-gray-100 rounded p-2 text-xs overflow-x-auto">{sc.output}</pre>
                                    </div>
                                </div>
                                {sc.explanation && <p className="text-xs text-gray-500 mt-1">{sc.explanation}</p>}
                            </div>
                        ))}
                    </>
                ) : tab === 'stats' ? (
                    <ProblemStats problemId={problem.id} />
                ) : (
                    <div className="space-y-4">
                        {editorials.length === 0 ? (
                            <p className="text-gray-400 text-sm">No editorials yet for this problem.</p>
                        ) : (
                            editorials.map(e => (
                                <Link key={e.id} to={`/editorials/${e.id}`} className="block border rounded p-4 hover:bg-gray-50">
                                    <div className="flex items-center gap-2 mb-1">
                                        {e.is_official && <span className="text-xs bg-green-100 text-green-700 px-2 py-0.5 rounded font-medium">Official</span>}
                                        <h4 className="font-medium">{e.title}</h4>
                                    </div>
                                    <div className="flex gap-4 text-xs text-gray-400">
                                        <span>{e.username}</span>
                                        {e.time_complexity && <span>Time: {e.time_complexity}</span>}
                                        <span>{e.upvotes} upvotes</span>
                                    </div>
                                </Link>
                            ))
                        )}
                    </div>
                )}
            </div>

            {/* Editor */}
            <div className="flex flex-col gap-3">
                <div className="flex items-center justify-between">
                    <select
                        value={lang}
                        onChange={e => setLang(e.target.value)}
                        className="border border-gray-300 rounded px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                    >
                        {LANGS.map(l => <option key={l.value} value={l.value}>{l.label}</option>)}
                    </select>
                    <button
                        onClick={submit}
                        disabled={submitting}
                        className="bg-green-600 text-white px-4 py-1.5 rounded text-sm font-medium hover:bg-green-700 disabled:opacity-50 transition-colors"
                    >
                        {submitting ? 'Submitting...' : 'Submit'}
                    </button>
                </div>
                <div ref={editorRef} className="border border-gray-200 rounded overflow-hidden flex-1" style={{ minHeight: '400px' }} />
                {result && (
                    <div className="border border-gray-200 rounded p-3 text-sm">
                        <div className="flex justify-between items-center">
                            <span className={`font-semibold ${STATUS_COLORS[result.status] || ''}`}>
                                {STATUS_LABELS[result.status] || result.status}
                            </span>
                            {result.time_used > 0 && (
                                <span className="text-gray-500 text-xs">
                                    {result.time_used}ms / {Math.round(result.memory_used / 1024)}MB
                                </span>
                            )}
                        </div>
                        {result.compile_output && (
                            <pre className="mt-2 text-xs text-red-600 bg-red-50 rounded p-2 overflow-x-auto">{result.compile_output}</pre>
                        )}
                    </div>
                )}
            </div>
        </div>
    )
}