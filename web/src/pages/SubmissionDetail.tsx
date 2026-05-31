import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api, type TestCaseResult } from '../lib/api'
import { resolveProblemSlug, resolveProblemTitle } from '../lib/problemSlugResolver'

const STATUS_LABEL: Record<string, string> = {
    ac: 'Accepted', wa: 'Wrong Answer', tle: 'Time Limit Exceeded',
    mle: 'Memory Limit Exceeded', re: 'Runtime Error', ce: 'Compile Error',
    pending: 'Pending', judging: 'Judging...', se: 'System Error',
}

const STATUS_COLOR: Record<string, string> = {
    ac: 'text-green-700 bg-green-50 border-green-200',
    wa: 'text-red-700 bg-red-50 border-red-200', tle: 'text-yellow-700 bg-yellow-50 border-yellow-200',
    mle: 'text-orange-700 bg-orange-50 border-orange-200', re: 'text-red-700 bg-red-50 border-red-200',
    ce: 'text-purple-700 bg-purple-50 border-purple-200',
    pending: 'text-blue-700 bg-blue-50 border-blue-200', judging: 'text-blue-700 bg-blue-50 border-blue-200',
    se: 'text-gray-700 bg-gray-50 border-gray-200',
}

const CASE_COLOR: Record<string, string> = {
    ac: 'text-green-600', wa: 'text-red-600', tle: 'text-yellow-600',
    mle: 'text-orange-600', re: 'text-red-600', se: 'text-gray-600',
}

function groupBySubtask(results: TestCaseResult[]): Map<number, TestCaseResult[]> {
    const groups = new Map<number, TestCaseResult[]>()
    for (const r of results) {
        const id = r.subtask_id ?? 0
        if (!groups.has(id)) groups.set(id, [])
        groups.get(id)!.push(r)
    }
    return groups
}

export default function SubmissionDetail() {
    const { id } = useParams<{ id: string }>()
    const [sub, setSub] = useState<any>(null)
    const [loading, setLoading] = useState(true)
    const [problemSlug, setProblemSlug] = useState<string | null>(null)
    const [problemTitle, setProblemTitle] = useState<string | null>(null)
    const [activeTab, setActiveTab] = useState<'overview' | 'code' | 'tests'>('overview')

    useEffect(() => {
        if (!id) return
        const poll = async () => {
            try {
                const d = await api.submissions.get(id)
                setSub(d)
                setLoading(false)
                if (d.problem_id) {
                    resolveProblemSlug(d.problem_id).then(setProblemSlug)
                    resolveProblemTitle(d.problem_id).then(setProblemTitle)
                }
                if (d.status === 'pending' || d.status === 'judging') {
                    setTimeout(poll, 1500)
                }
            } catch { setLoading(false) }
        }
        poll()
    }, [id])

    if (loading) return <div className="text-center py-20 text-gray-400">Loading submission...</div>
    if (!sub) return <div className="text-center py-20 text-gray-400">Submission not found.</div>

    const judgeResult = sub.judge_result || []
    const totalCases = judgeResult.length
    const passedCases = judgeResult.filter((r: any) => r.status === 'ac').length

    return (
        <div className="max-w-4xl mx-auto space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-2xl font-bold">Submission {sub.id.substring(0, 8)}</h1>
                    <p className="text-sm text-gray-500 mt-1">
                        <Link to={problemSlug ? `/problems/${problemSlug}` : '#'} className="text-blue-600 hover:underline">
                            Problem {problemTitle || sub.problem_id?.substring(0, 8)}
                        </Link>
                        {' · '}{sub.language}
                        {' · '}{new Date(sub.created_at).toLocaleString()}
                    </p>
                    {sub.remote_id && (
                        <p className="text-sm text-gray-600 mt-2 flex items-center gap-1.5">
                            <span className="font-semibold text-gray-700">Remote OJ ID:</span>
                            {sub.remote_url ? (
                                <a href={sub.remote_url} target="_blank" rel="noopener noreferrer" className="text-blue-600 hover:underline font-mono font-medium">
                                    {sub.remote_id}
                                </a>
                            ) : (
                                <span className="font-mono text-gray-700">{sub.remote_id}</span>
                            )}
                        </p>
                    )}
                </div>
                <span className={`px-4 py-2 rounded-lg text-sm font-bold border ${STATUS_COLOR[sub.status] || ''}`}>
                    {STATUS_LABEL[sub.status] || sub.status}
                </span>
            </div>

            {sub.status === 'ce' && sub.compile_output && (
                <div className="bg-purple-50 border border-purple-200 rounded-lg p-4">
                    <h3 className="font-semibold text-purple-800 mb-2">Compile Error</h3>
                    <pre className="text-sm text-purple-900 whitespace-pre-wrap font-mono bg-purple-100/50 p-3 rounded overflow-x-auto max-h-64">
                        {sub.compile_output}
                    </pre>
                </div>
            )}

            <div className="flex border-b border-gray-200">
                {['overview', 'code', 'tests'].map(tab => (
                    <button key={tab} onClick={() => setActiveTab(tab as any)}
                        className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px capitalize ${activeTab === tab ? 'border-blue-600 text-blue-600' : 'border-transparent text-gray-500 hover:text-gray-700'}`}>
                        {tab} {tab === 'tests' && totalCases > 0 && `(${passedCases}/${totalCases})`}
                    </button>
                ))}
            </div>

            {activeTab === 'overview' && (
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                    <div className="bg-gray-50 p-4 rounded-lg">
                        <p className="text-sm text-gray-500">Time Used</p>
                        <p className="text-xl font-bold mt-1">{sub.time_used > 0 ? `${sub.time_used}ms` : '—'}</p>
                    </div>
                    <div className="bg-gray-50 p-4 rounded-lg">
                        <p className="text-sm text-gray-500">Memory Used</p>
                        <p className="text-xl font-bold mt-1">{sub.memory_used > 0 ? `${Math.round(sub.memory_used / 1024)}MB` : '—'}</p>
                    </div>
                    <div className="bg-gray-50 p-4 rounded-lg">
                        <p className="text-sm text-gray-500">Score</p>
                        <p className="text-xl font-bold mt-1">{sub.score}</p>
                    </div>
                    <div className="bg-gray-50 p-4 rounded-lg">
                        <p className="text-sm text-gray-500">Code Size</p>
                        <p className="text-xl font-bold mt-1">{sub.code_size}B</p>
                    </div>
                </div>
            )}

            {activeTab === 'code' && (
                <div className="bg-gray-900 rounded-lg p-4 overflow-x-auto">
                    <pre className="text-green-400 text-sm font-mono whitespace-pre-wrap">
                        {sub.source_code || 'Source code not available.'}
                    </pre>
                </div>
            )}

            {activeTab === 'tests' && (() => {
                if (judgeResult.length === 0) {
                    return (
                        <div className="space-y-3">
                            <p className="text-gray-400 text-center py-8">
                                {sub.status === 'pending' || sub.status === 'judging'
                                    ? 'Waiting for judging results...'
                                    : 'No test case details available.'}
                            </p>
                        </div>
                    )
                }

                const hasSubtasks = judgeResult.some((r: any) => (r.subtask_id ?? 0) > 0)

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
                                    <div key={id} className={`rounded-lg border p-4 ${allPassed ? 'border-green-200 bg-green-50/30' : 'border-red-200 bg-red-50/30'}`}>
                                        <div className="flex justify-between items-center mb-3">
                                            <span className="font-semibold text-sm text-gray-700">Subtask {id}</span>
                                            <span className={`text-xs font-bold px-2 py-0.5 rounded ${allPassed ? 'text-green-700 bg-green-100' : 'text-red-700 bg-red-100'}`}>
                                                {earnedScore}/{maxScore} pts
                                            </span>
                                        </div>
                                        <div className="space-y-2">
                                            {cases.map((c, i) => (
                                                <div key={i} className={`border rounded p-3 ${c.status === 'ac' ? 'bg-green-50/50 border-green-100' : 'bg-red-50/50 border-red-100'}`}>
                                                    <div className="flex items-center justify-between mb-1">
                                                        <span className="text-xs font-medium text-gray-600">{c.case_name || `Case ${i + 1}`}</span>
                                                        <span className={`text-xs font-bold px-1.5 py-0.5 rounded ${CASE_COLOR[c.status] || ''} bg-white`}>
                                                            {STATUS_LABEL[c.status] || c.status}
                                                        </span>
                                                    </div>
                                                    <div className="flex gap-4 text-xs text-gray-500">
                                                        {c.time !== undefined && <span>Time: {c.time > 0 ? `${c.time}ms` : '—'}</span>}
                                                        {c.memory !== undefined && <span>Memory: {c.memory > 0 ? `${Math.round(c.memory / 1024)}MB` : '—'}</span>}
                                                        {(c.score ?? 0) > 0 && <span>Score: {c.score}pts</span>}
                                                    </div>
                                                    {c.detail && (
                                                        <div className="mt-1.5 text-xs text-red-600 bg-red-50 p-1.5 rounded">
                                                            {c.detail}
                                                        </div>
                                                    )}
                                                </div>
                                            ))}
                                        </div>
                                    </div>
                                )
                            })}
                        </div>
                    )
                }

                // Fallback: non-subtask rendering
                return (
                    <div className="space-y-3">
                        {judgeResult.map((r: any, i: number) => (
                            <div key={i} className={`border rounded-lg p-4 ${r.status === 'ac' ? 'bg-green-50/30 border-green-200' : 'bg-red-50/30 border-red-200'}`}>
                                <div className="flex items-center justify-between mb-2">
                                    <span className="font-semibold text-sm">Test Case {i + 1}: {r.case_name || `#${i + 1}`}</span>
                                    <span className={`text-xs font-bold px-2 py-0.5 rounded ${CASE_COLOR[r.status] || ''} bg-white`}>
                                        {STATUS_LABEL[r.status] || r.status}
                                    </span>
                                </div>
                                <div className="grid grid-cols-3 gap-4 text-xs text-gray-500 mb-2">
                                    <span>Time: {r.time > 0 ? `${r.time}ms` : '—'}</span>
                                    <span>Memory: {r.memory > 0 ? `${Math.round(r.memory / 1024)}MB` : '—'}</span>
                                    <span>Score: {r.score}</span>
                                </div>
                                {r.detail && (
                                    <div className="mt-2 text-xs text-red-600 bg-red-50 p-2 rounded">
                                        {r.detail}
                                    </div>
                                )}
                            </div>
                        ))}
                    </div>
                )
            })()}
        </div>
    )
}
