import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import { EmptyState } from './EmptyState'

interface ProblemStatsProps {
    problemId: string
}

interface ProblemStatsData { total_submissions: number; accepted_submissions: number; acceptance_rate: number; average_attempts: number; language_distribution: Record<string, number> }

export default function ProblemStats({ problemId }: ProblemStatsProps) {
    const [stats, setStats] = useState<ProblemStatsData | null>(null)
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        api.stats.getProblemStats(problemId)
            .then(setStats)
            .catch(console.error)
            .finally(() => setLoading(false))
    }, [problemId])

    if (loading) return <div className="text-sm text-muted">Loading statistics...</div>
    if (!stats) {
        return (
            <EmptyState
                text="No statistics available"
                description="Submission stats will appear after the first solutions to this problem."
            />
        )
    }

    const total = Object.values(stats.language_distribution || {}).reduce((a, b) => a + b, 0) as number

    return (
        <div className="space-y-6">
            <div className="grid grid-cols-2 gap-4">
                <div className="bg-surface-2 p-4 rounded-lg">
                    <p className="text-sm text-muted">Submissions</p>
                    <p className="text-2xl font-bold mt-1">{stats.total_submissions}</p>
                </div>
                <div className="bg-surface-2 p-4 rounded-lg">
                    <p className="text-sm text-muted">Accepted</p>
                    <p className="text-2xl font-bold mt-1 text-ok">{stats.accepted_submissions}</p>
                </div>
                <div className="bg-surface-2 p-4 rounded-lg">
                    <p className="text-sm text-muted">Acceptance Rate</p>
                    <p className="text-2xl font-bold mt-1 text-brand">
                        {stats.acceptance_rate ? stats.acceptance_rate.toFixed(1) : '0.0'}%
                    </p>
                </div>
                <div className="bg-surface-2 p-4 rounded-lg">
                    <p className="text-sm text-muted">Avg Attempts (AC)</p>
                    <p className="text-2xl font-bold mt-1">
                        {stats.average_attempts ? stats.average_attempts.toFixed(1) : '0.0'}
                    </p>
                </div>
            </div>

            <div>
                <h3 className="font-semibold text-sm text-ink-soft uppercase tracking-wide mb-3">Languages Used</h3>
                {total === 0 ? (
                    <EmptyState text="No language data yet" description="The language breakdown fills in as people submit solutions." />
                ) : (
                    <div className="space-y-2">
                        {Object.entries(stats.language_distribution || {}).map(([lang, count]) => (
                            <div key={lang} className="flex items-center text-sm">
                                <span className="w-24 text-muted font-medium truncate">{lang}</span>
                                <div className="flex-1 bg-surface-3 h-4 rounded-full overflow-hidden mx-3">
                                    <div className="bg-brand h-full rounded-full" style={{ width: `${(count / total) * 100}%` }} />
                                </div>
                                <span className="text-muted w-12 text-right">{count} ({((count / total) * 100).toFixed(0)}%)</span>
                            </div>
                        ))}
                    </div>
                )}
            </div>
        </div>
    )
}
