import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api } from '../lib/api'

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

export default function SubmissionDetail() {
    const { id } = useParams<{ id: string }>()
    const [sub, setSub] = useState<any>(null)
    const [loading, setLoading] = useState(true)
    const [activeTab, setActiveTab] = useState<'overview' | 'code' | 'tests'>('overview')

    useEffect(() => {
        if (!id) return
        const poll = async () => {
            try {
                const d = await api.submissions.get(id)
                setSub(d)
                setLoading(false)
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
                        <Link to={`/problems/${sub.problem_id}`} className="text-blue-600 hover:underline">
                            Problem {sub.problem_id?.substring(0, 8)}
                        </Link>
                        {' · '}{sub.language}
                        {' · '}{new Date(sub.created_at).toLocaleString()}
                    </p>
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

            {activeTab === 'tests' && (
                <div className="space-y-3">
                    {judgeResult.length === 0 ? (
                        <p className="text-gray-400 text-center py-8">
                            {sub.status === 'pending' || sub.status === 'judging'
                                ? 'Waiting for judging results...'
                                : 'No test case details available.'}
                        </p>
                    ) : (
                        judgeResult.map((r: any, i: number) => (
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
                        ))
                    )}
                </div>
            )}
        </div>
    )
}
