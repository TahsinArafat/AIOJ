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
    ac: 'text-green-700 dark:text-green-300 bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-800',
    wa: 'text-red-700 dark:text-red-300 bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-800', tle: 'text-yellow-700 dark:text-yellow-300 bg-yellow-50 dark:bg-yellow-900/20 border-yellow-200 dark:border-yellow-700',
    mle: 'text-orange-700 dark:text-orange-300 bg-orange-50 dark:bg-orange-900/20 border-orange-200 dark:border-orange-700', re: 'text-red-700 dark:text-red-300 bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-800',
    ce: 'text-purple-700 dark:text-purple-300 bg-purple-50 dark:bg-purple-900/20 border-purple-200 dark:border-purple-700',
    pending: 'text-blue-700 dark:text-blue-300 bg-blue-50 dark:bg-blue-900/20 border-blue-200 dark:border-blue-800', judging: 'text-blue-700 dark:text-blue-300 bg-blue-50 dark:bg-blue-900/20 border-blue-200 dark:border-blue-800',
    se: 'text-gray-700 dark:text-gray-300 bg-gray-50 dark:bg-gray-800 border-gray-200 dark:border-gray-700',
}

const CASE_COLOR: Record<string, string> = {
    ac: 'text-green-600 dark:text-green-400', wa: 'text-red-600 dark:text-red-400', tle: 'text-yellow-600 dark:text-yellow-400',
    mle: 'text-orange-600 dark:text-orange-400', re: 'text-red-600 dark:text-red-400', se: 'text-gray-600 dark:text-gray-400',
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
        } catch (err: any) {
            alert('Failed to retry: ' + err.message)
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
        } catch (err: any) {
            alert('Failed to recheck: ' + err.message)
        } finally {
            setActionLoading(false)
        }
    }

    if (loading) return <div className="text-center py-20 text-gray-400 dark:text-gray-500">Loading submission...</div>
    if (!sub) return <div className="text-center py-20 text-gray-400 dark:text-gray-500">Submission not found.</div>

    const judgeResult = sub.judge_result || []
    const totalCases = judgeResult.length
    const passedCases = judgeResult.filter((r: any) => r.status === 'ac').length

    return (
        <div className="max-w-4xl mx-auto space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-2xl font-bold">Submission {sub.id.substring(0, 8)}</h1>
                    <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                        <Link to={problemSlug ? `/problems/${problemSlug}` : '#'} className="text-blue-600 dark:text-blue-400 hover:underline">
                            Problem {problemTitle || sub.problem_id?.substring(0, 8)}
                        </Link>
                        {' · '}{sub.language}
                        {' · '}{new Date(sub.created_at).toLocaleString()}
                    </p>
                    {sub.remote_id && (
                        <p className="text-sm text-gray-600 dark:text-gray-400 mt-2 flex items-center gap-1.5">
                            <span className="font-semibold text-gray-700 dark:text-gray-300">Remote OJ ID:</span>
                            {sub.remote_url ? (
                                <a href={sub.remote_url} target="_blank" rel="noopener noreferrer" className="text-blue-600 dark:text-blue-400 hover:underline font-mono font-medium">
                                    {sub.remote_id}
                                </a>
                            ) : (
                                <span className="font-mono text-gray-700 dark:text-gray-300">{sub.remote_id}</span>
                            )}
                        </p>
                    )}
                </div>
                <div className="flex flex-col items-end gap-2">
                    <span className={`px-4 py-2 rounded-lg text-sm font-bold border ${STATUS_COLOR[sub.status] || ''}`}>
                        {STATUS_LABEL[sub.status] || sub.status}
                    </span>
                    {sub.is_remote && (
                        <div className="flex gap-2">
                            {sub.status === 'se' && (
                                <button
                                    onClick={handleRetryRemote}
                                    disabled={actionLoading}
                                    className="bg-blue-600 hover:bg-blue-700 text-white px-3 py-1.5 rounded text-xs font-semibold disabled:opacity-50 transition-colors"
                                >
                                    Retry Remote Submission
                                </button>
                            )}
                            {sub.remote_id && (
                                <button
                                    onClick={handleRecheckRemote}
                                    disabled={actionLoading}
                                    className="bg-gray-600 hover:bg-gray-700 text-white px-3 py-1.5 rounded text-xs font-semibold disabled:opacity-50 transition-colors"
                                >
                                    Recheck Status
                                </button>
                            )}
                        </div>
                    )}
                </div>
            </div>

            {sub.status === 'ce' && sub.compile_output && (
                <div className="bg-purple-50 dark:bg-purple-900/20 border border-purple-200 dark:border-purple-700 rounded-lg p-4">
                    <h3 className="font-semibold text-purple-800 dark:text-purple-300 mb-2">Compile Error</h3>
                    <pre className="text-sm text-purple-900 whitespace-pre-wrap font-mono bg-purple-100 dark:bg-purple-900/30/50 p-3 rounded overflow-x-auto max-h-64">
                        {sub.compile_output}
                    </pre>
                </div>
            )}

            <div className="flex border-b border-gray-200 dark:border-gray-700">
                {['overview', 'code', 'tests'].map(tab => (
                    <button key={tab} onClick={() => setActiveTab(tab as any)}
                        className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px capitalize ${activeTab === tab ? 'border-blue-600 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'}`}>
                        {tab} {tab === 'tests' && totalCases > 0 && `(${passedCases}/${totalCases})`}
                    </button>
                ))}
            </div>

            {activeTab === 'overview' && (
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                    <div className="bg-gray-50 dark:bg-gray-800 p-4 rounded-lg">
                        <p className="text-sm text-gray-500 dark:text-gray-400">Time Used</p>
                        <p className="text-xl font-bold mt-1">
                            {sub.status !== "pending" && sub.status !== "judging" && sub.status !== "ce" && sub.status !== "se" ? `${sub.time_used}ms` : "—"}
                        </p>
                    </div>
                    <div className="bg-gray-50 dark:bg-gray-800 p-4 rounded-lg">
                        <p className="text-sm text-gray-500 dark:text-gray-400">Memory Used</p>
                        <p className="text-xl font-bold mt-1">
                            {sub.status !== "pending" && sub.status !== "judging" && sub.status !== "ce" && sub.status !== "se" ? `${Math.round(sub.memory_used / 1024)}MB` : "—"}
                        </p>
                    </div>
                    <div className="bg-gray-50 dark:bg-gray-800 p-4 rounded-lg">
                        <p className="text-sm text-gray-500 dark:text-gray-400">Score</p>
                        <p className="text-xl font-bold mt-1">{sub.score}</p>
                    </div>
                    <div className="bg-gray-50 dark:bg-gray-800 p-4 rounded-lg">
                        <p className="text-sm text-gray-500 dark:text-gray-400">Code Size</p>
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
                            <p className="text-gray-400 dark:text-gray-500 text-center py-8">
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
                                    <div key={id} className={`rounded-lg border p-4 ${allPassed ? 'border-green-200 dark:border-green-800 bg-green-50 dark:bg-green-900/20/30' : 'border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900/20/30'}`}>
                                        <div className="flex justify-between items-center mb-3">
                                            <span className="font-semibold text-sm text-gray-700 dark:text-gray-300">Subtask {id}</span>
                                            <span className={`text-xs font-bold px-2 py-0.5 rounded ${allPassed ? 'text-green-700 dark:text-green-300 bg-green-100 dark:bg-green-900/30' : 'text-red-700 dark:text-red-300 bg-red-100 dark:bg-red-900/30'}`}>
                                                {earnedScore}/{maxScore} pts
                                            </span>
                                        </div>
                                        <div className="space-y-2">
                                            {cases.map((c, i) => (
                                                <div key={i} className={`border rounded p-3 ${c.status === 'ac' ? 'bg-green-50 dark:bg-green-900/20/50 border-green-100 dark:border-green-800' : 'bg-red-50 dark:bg-red-900/20/50 border-red-100 dark:border-red-800'}`}>
                                                    <div className="flex items-center justify-between mb-1">
                                                        <span className="text-xs font-medium text-gray-600 dark:text-gray-400">{c.case_name || `Case ${i + 1}`}</span>
                                                        <span className={`text-xs font-bold px-1.5 py-0.5 rounded ${CASE_COLOR[c.status] || ''} bg-white dark:bg-gray-800`}>
                                                            {STATUS_LABEL[c.status] || c.status}
                                                        </span>
                                                    </div>
                                                    <div className="flex gap-4 text-xs text-gray-500 dark:text-gray-400">
                                                        {c.time !== undefined && c.time !== null && <span>Time: {c.time}ms</span>}
                                                        {c.memory !== undefined && c.memory !== null && <span>Memory: {Math.round(c.memory / 1024)}MB</span>}
                                                        {(c.score ?? 0) > 0 && <span>Score: {c.score}pts</span>}
                                                    </div>
                                                    {c.detail && (
                                                        <div className="mt-1.5 text-xs text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20 p-1.5 rounded">
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
                            <div key={i} className={`border rounded-lg p-4 ${r.status === 'ac' ? 'bg-green-50 dark:bg-green-900/20/30 border-green-200 dark:border-green-800' : 'bg-red-50 dark:bg-red-900/20/30 border-red-200 dark:border-red-800'}`}>
                                <div className="flex items-center justify-between mb-2">
                                    <span className="font-semibold text-sm">Test Case {i + 1}: {r.case_name || `#${i + 1}`}</span>
                                    <span className={`text-xs font-bold px-2 py-0.5 rounded ${CASE_COLOR[r.status] || ''} bg-white dark:bg-gray-800`}>
                                        {STATUS_LABEL[r.status] || r.status}
                                    </span>
                                </div>
                                <div className="grid grid-cols-3 gap-4 text-xs text-gray-500 dark:text-gray-400 mb-2">
                                    <span>Time: {r.time !== undefined && r.time !== null ? `${r.time}ms` : "—"}</span>
                                    <span>Memory: {r.memory !== undefined && r.memory !== null ? `${Math.round(r.memory / 1024)}MB` : "—"}</span>
                                    <span>Score: {r.score}</span>
                                </div>
                                {r.detail && (
                                    <div className="mt-2 text-xs text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20 p-2 rounded">
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
