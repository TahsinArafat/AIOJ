import { useEffect, useState } from 'react'
import { api } from '../lib/api'

interface ProblemStatsProps {
    problemId: string
}

export default function ProblemStats({ problemId }: ProblemStatsProps) {
    const [stats, setStats] = useState<any>(null)
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        api.stats.getProblemStats(problemId)
            .then(setStats)
            .catch(console.error)
            .finally(() => setLoading(false))
    }, [problemId])

    if (loading) return <div className="text-sm text-gray-500">Loading statistics...</div>
    if (!stats) return <div className="text-sm text-gray-400">No statistics available.</div>

    const total = Object.values(stats.language_distribution || {}).reduce((a: any, b: any) => a + b, 0) as number

    return (
        <div className="space-y-6">
            <div className="grid grid-cols-2 gap-4">
                <div className="bg-gray-50 p-4 rounded-lg">
                    <p className="text-sm text-gray-500">Submissions</p>
                    <p className="text-2xl font-bold mt-1">{stats.total_submissions}</p>
                </div>
                <div className="bg-gray-50 p-4 rounded-lg">
                    <p className="text-sm text-gray-500">Accepted</p>
                    <p className="text-2xl font-bold mt-1 text-green-600">{stats.accepted_submissions}</p>
                </div>
                <div className="bg-gray-50 p-4 rounded-lg">
                    <p className="text-sm text-gray-500">Acceptance Rate</p>
                    <p className="text-2xl font-bold mt-1 text-blue-600">
                        {stats.acceptance_rate ? stats.acceptance_rate.toFixed(1) : '0.0'}%
                    </p>
                </div>
                <div className="bg-gray-50 p-4 rounded-lg">
                    <p className="text-sm text-gray-500">Avg Attempts (AC)</p>
                    <p className="text-2xl font-bold mt-1">
                        {stats.average_attempts ? stats.average_attempts.toFixed(1) : '0.0'}
                    </p>
                </div>
            </div>

            <div>
                <h3 className="font-semibold text-sm text-gray-700 uppercase tracking-wide mb-3">Languages Used</h3>
                {total === 0 ? (
                    <p className="text-sm text-gray-400">No language data yet.</p>
                ) : (
                    <div className="space-y-2">
                        {Object.entries(stats.language_distribution || {}).map(([lang, count]: any) => (
                            <div key={lang} className="flex items-center text-sm">
                                <span className="w-24 text-gray-600 font-medium truncate">{lang}</span>
                                <div className="flex-1 bg-gray-100 h-4 rounded-full overflow-hidden mx-3">
                                    <div className="bg-blue-600 h-full rounded-full" style={{ width: `${(count / total) * 100}%` }} />
                                </div>
                                <span className="text-gray-500 w-12 text-right">{count} ({((count / total) * 100).toFixed(0)}%)</span>
                            </div>
                        ))}
                    </div>
                )}
            </div>
        </div>
    )
}
