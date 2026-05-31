import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import ReactMarkdown from 'react-markdown'
import remarkMath from 'remark-math'
import rehypeKatex from 'rehype-katex'
import 'katex/dist/katex.min.css'
import { api, getAccessToken } from '../lib/api'
import CodeEditor from '../components/CodeEditor'

const LANGS = [
    { value: 'cpp-gpp-64', label: 'GNU G++17 7.3.0 (64 bit)' },
    { value: 'cpp-gpp-32', label: 'GNU G++14 6.4.0 (32 bit)' },
    { value: 'c-gcc-64', label: 'GNU GCC C11 9.2.0 (64 bit)' },
    { value: 'python', label: 'Python 3.8.10' },
    { value: 'java', label: 'Java 11.0.6' },
    { value: 'rust', label: 'Rust 1.75.0' },
    { value: 'nodejs', label: 'Node.js 18.16.1' },
]

function SubmitForm({ problemId, contestId }: { problemId: string; contestId?: string }) {
    const [language, setLanguage] = useState('cpp-gpp-64')
    const [code, setCode] = useState('')
    const [submitting, setSubmitting] = useState(false)
    const [result, setResult] = useState<any>(null)

    async function handleSubmit() {
        if (!code.trim()) return
        setSubmitting(true)
        setResult(null)
        try {
            const res = await api.submissions.create({
                problem_id: problemId,
                language,
                source_code: code,
                contest_id: contestId,
            })
            setResult(res)
        } catch (err: any) {
            setResult({ error: err.message })
        } finally {
            setSubmitting(false)
        }
    }

    return (
        <div className="border rounded-lg overflow-hidden">
            <div className="p-4 bg-gray-50 border-b">
                <select
                    value={language}
                    onChange={e => setLanguage(e.target.value)}
                    className="w-full p-2 border rounded"
                >
                    {LANGS.map(l => (
                        <option key={l.value} value={l.value}>{l.label}</option>
                    ))}
                </select>
            </div>
            <CodeEditor
                language={language}
                value={code}
                onChange={setCode}
                height="300px"
            />
            <div className="p-4 bg-gray-50 border-t">
                <button
                    onClick={handleSubmit}
                    disabled={submitting || !code.trim()}
                    className="w-full py-2 bg-blue-500 text-white rounded hover:bg-blue-600 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                    {submitting ? 'Submitting...' : 'Submit'}
                </button>
            </div>
            {result && (
                <div className={`p-4 border-t ${result.error ? 'bg-red-50' : 'bg-green-50'}`}>
                    {result.error ? (
                        <p className="text-red-600">{result.error}</p>
                    ) : (
                        <p className="text-green-600">Solution submitted successfully!</p>
                    )}
                </div>
            )}
        </div>
    )
}

export default function ContestProblem() {
    const { contestId, index } = useParams<{ contestId: string; index: string }>()
    const [problem, setProblem] = useState<any>(null)
    const [contest, setContest] = useState<any>(null)
    const [canSubmit, setCanSubmit] = useState(true)
    const [upsolvingDisabled, setUpsolvingDisabled] = useState(false)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)

    useEffect(() => {
        loadProblem()
    }, [contestId, index])

    async function loadProblem() {
        try {
            setLoading(true)
            setError(null)
            const data = await api.contests.getProblemByIndex(contestId!, index!)
            setProblem(data.problem)
            setContest(data.contest)
            setCanSubmit(data.can_submit ?? true)
            setUpsolvingDisabled(data.upsolving_disabled ?? false)
        } catch (err: any) {
            setError(err.message || 'Failed to load problem')
        } finally {
            setLoading(false)
        }
    }

    if (loading) {
        return (
            <div className="flex items-center justify-center min-h-screen">
                <div className="text-lg">Loading problem...</div>
            </div>
        )
    }

    if (error) {
        return (
            <div className="flex flex-col items-center justify-center min-h-screen gap-4">
                <div className="text-red-500 text-lg">{error}</div>
                <Link to={`/contests/${contestId}`} className="text-blue-500 hover:underline">
                    Back to Contest
                </Link>
            </div>
        )
    }

    if (!problem) {
        return (
            <div className="flex flex-col items-center justify-center min-h-screen gap-4">
                <div className="text-lg">Problem not found</div>
                <Link to={`/contests/${contestId}`} className="text-blue-500 hover:underline">
                    Back to Contest
                </Link>
            </div>
        )
    }

    return (
        <div className="container mx-auto px-4 py-8 max-w-6xl">
            <div className="mb-6">
                <Link to={`/contests/${contestId}`} className="text-blue-500 hover:underline">
                    ← Back to {contest?.title || 'Contest'}
                </Link>
            </div>

            <div className="flex flex-col lg:flex-row gap-8">
                <div className="flex-1">
                    <h1 className="text-2xl font-bold mb-2">
                        {index}. {problem.title}
                    </h1>

                    <div className="flex gap-4 text-sm text-gray-600 mb-6">
                        <span>Time Limit: {problem.time_limit}ms</span>
                        <span>Memory Limit: {Math.round(problem.memory_limit / 1024)}MB</span>
                        {problem.difficulty && (
                            <span className={`px-2 py-0.5 rounded ${
                                problem.difficulty === 'easy' ? 'bg-green-100 text-green-800' :
                                problem.difficulty === 'medium' ? 'bg-yellow-100 text-yellow-800' :
                                'bg-red-100 text-red-800'
                            }`}>
                                {problem.difficulty}
                            </span>
                        )}
                    </div>

                    {upsolvingDisabled && (
                        <div className="mb-6 p-4 bg-yellow-50 border border-yellow-200 rounded">
                            <p className="text-yellow-800">
                                Upsolving is disabled for this contest. You can view the problem but cannot submit new solutions.
                            </p>
                        </div>
                    )}

                    <div className="prose max-w-none">
                        <ReactMarkdown remarkPlugins={[remarkMath]} rehypePlugins={[rehypeKatex]}>
                            {problem.description}
                        </ReactMarkdown>
                    </div>

                    {problem.input_format && (
                        <div className="mt-6">
                            <h3 className="text-lg font-semibold mb-2">Input Format</h3>
                            <div className="prose max-w-none">
                                <ReactMarkdown remarkPlugins={[remarkMath]} rehypePlugins={[rehypeKatex]}>
                                    {problem.input_format}
                                </ReactMarkdown>
                            </div>
                        </div>
                    )}

                    {problem.output_format && (
                        <div className="mt-6">
                            <h3 className="text-lg font-semibold mb-2">Output Format</h3>
                            <div className="prose max-w-none">
                                <ReactMarkdown remarkPlugins={[remarkMath]} rehypePlugins={[rehypeKatex]}>
                                    {problem.output_format}
                                </ReactMarkdown>
                            </div>
                        </div>
                    )}

                    {problem.sample_cases && problem.sample_cases.length > 0 && (
                        <div className="mt-6">
                            <h3 className="text-lg font-semibold mb-4">Sample Cases</h3>
                            {problem.sample_cases.map((sample: any, i: number) => (
                                <div key={i} className="mb-6 p-4 bg-gray-50 rounded-lg">
                                    <div className="mb-3">
                                        <div className="font-semibold text-sm text-gray-600 mb-1">Input:</div>
                                        <pre className="bg-white p-3 rounded border overflow-x-auto">
                                            <code>{sample.input}</code>
                                        </pre>
                                    </div>
                                    <div className="mb-3">
                                        <div className="font-semibold text-sm text-gray-600 mb-1">Output:</div>
                                        <pre className="bg-white p-3 rounded border overflow-x-auto">
                                            <code>{sample.output}</code>
                                        </pre>
                                    </div>
                                    {sample.explanation && (
                                        <div>
                                            <div className="font-semibold text-sm text-gray-600 mb-1">Explanation:</div>
                                            <div className="prose max-w-none text-sm">
                                                <ReactMarkdown remarkPlugins={[remarkMath]} rehypePlugins={[rehypeKatex]}>
                                                    {sample.explanation}
                                                </ReactMarkdown>
                                            </div>
                                        </div>
                                    )}
                                </div>
                            ))}
                        </div>
                    )}

                    {problem.hint && (
                        <div className="mt-6">
                            <h3 className="text-lg font-semibold mb-2">Hint</h3>
                            <div className="prose max-w-none">
                                <ReactMarkdown remarkPlugins={[remarkMath]} rehypePlugins={[rehypeKatex]}>
                                    {problem.hint}
                                </ReactMarkdown>
                            </div>
                        </div>
                    )}
                </div>

                <div className="lg:w-1/2">
                    {canSubmit && getAccessToken() ? (
                        <div className="sticky top-4">
                            <h3 className="text-lg font-semibold mb-4">Submit Solution</h3>
                            <SubmitForm problemId={problem.id} contestId={contestId} />
                        </div>
                    ) : !canSubmit ? (
                        <div className="sticky top-4 p-4 bg-gray-50 rounded-lg">
                            <h3 className="text-lg font-semibold mb-2">Submissions Closed</h3>
                            <p className="text-gray-600">
                                {upsolvingDisabled
                                    ? 'Upsolving is disabled for this contest.'
                                    : 'Submissions are not allowed for this problem.'}
                            </p>
                        </div>
                    ) : (
                        <div className="sticky top-4 p-4 bg-gray-50 rounded-lg">
                            <h3 className="text-lg font-semibold mb-2">Login Required</h3>
                            <p className="text-gray-600">
                                Please <Link to="/login" className="text-blue-500 hover:underline">login</Link> to submit a solution.
                            </p>
                        </div>
                    )}
                </div>
            </div>
        </div>
    )
}
