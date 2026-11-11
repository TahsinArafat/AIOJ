import { useEffect, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'
import { loginHref } from '../lib/authRedirect'
import { Button } from '../components/ui/Button'

interface PracticeProblem { id: string; slug: string; title: string; difficulty?: string; tags?: string[]; submission_count: number; accepted_count: number }
interface Recommendations { hybrid?: PracticeProblem[]; progression?: PracticeProblem[]; weak_tags?: { tags?: string[]; problems?: PracticeProblem[] } }

export default function Practice() {
    const location = useLocation()
    const [rec, setRec] = useState<Recommendations | null>(null)
    const [loading, setLoading] = useState(() => !getAccessToken())
    const [error, setError] = useState('')
    const [activeTab, setActiveTab] = useState<'hybrid' | 'progression' | 'weak'>('hybrid')

    useEffect(() => {
        if (!getAccessToken()) return
        api.recommendations.get()
            .then(data => {
                setRec(data)
                setLoading(false)
            })
            .catch(err => {
                setError(err.message || 'Failed to load recommendations')
                setLoading(false)
            })
    }, [])

    if (loading) return <div className="text-center py-12 text-muted">Loading recommendations...</div>
    // The error state had no page heading and no way to retry.
    if (error) {
        return (
            <div className="mx-auto max-w-xl py-12">
                <h1 className="text-2xl font-semibold tracking-tight text-ink">Personalized Practice</h1>
                <div role="alert" className="mt-4 rounded-lg border border-bad/30 bg-bad/10 p-4 text-sm text-bad">
                    {error}
                </div>
                <Button variant="secondary" className="mt-4" onClick={() => window.location.reload()}>
                    Try again
                </Button>
            </div>
        )
    }

    if (!getAccessToken()) {
        return (
            <div className="max-w-4xl mx-auto text-center py-20">
                <h1 className="text-3xl font-extrabold tracking-tight text-ink mb-4">Personalized Practice</h1>
                <p className="text-muted mb-6">Log in to get smart problem recommendations tailored to your rating and weak areas.</p>
                <a href={loginHref(location)} className="bg-brand text-on-brand px-6 py-3 rounded-lg font-medium hover:bg-brand-hover transition-colors">
                    Log In to Practice
                </a>
            </div>
        )
    }

    const problems = activeTab === 'hybrid' ? rec?.hybrid : activeTab === 'progression' ? rec?.progression : rec?.weak_tags?.problems

    return (
        <div className="max-w-4xl mx-auto">
            <header className="mb-8">
                <h1 className="text-3xl font-extrabold tracking-tight text-ink">Personalized Practice</h1>
            </header>

            {/* Recommendation summary: counts drawn from the actual payload */}
            <dl className="grid grid-cols-3 gap-3 mb-8">
                {[
                    { label: 'Daily picks', value: rec?.hybrid?.length ?? 0 },
                    { label: 'Progression', value: rec?.progression?.length ?? 0 },
                    { label: 'Weak tags', value: rec?.weak_tags?.tags?.length ?? 0 },
                ].map(s => (
                    <div key={s.label} className="border border-hairline rounded-lg p-4 bg-surface">
                        <dt className="text-xs uppercase tracking-wide text-muted">{s.label}</dt>
                        <dd className="mt-1 text-2xl font-bold text-ink">{s.value}</dd>
                    </div>
                ))}
            </dl>

            {/* Tabs selector */}
            <div className="border-b border-hairline mb-6">
                <nav className="flex gap-6">
                    <button onClick={() => setActiveTab('hybrid')}
                        className={`pb-4 text-sm font-medium ${activeTab === 'hybrid' ? 'border-b-2 border-brand text-brand font-semibold' : 'text-muted hover:text-ink'}`}>
                        Daily Diet (Hybrid)
                    </button>
                    <button onClick={() => setActiveTab('progression')}
                        className={`pb-4 text-sm font-medium ${activeTab === 'progression' ? 'border-b-2 border-brand text-brand font-semibold' : 'text-muted hover:text-ink'}`}>
                        Progression Ladder
                    </button>
                    <button onClick={() => setActiveTab('weak')}
                        className={`pb-4 text-sm font-medium ${activeTab === 'weak' ? 'border-b-2 border-brand text-brand font-semibold' : 'text-muted hover:text-ink'}`}>
                        Targeted Tags Practice
                    </button>
                </nav>
            </div>

            {/* Recommendations Content */}
            <div className="space-y-4">
                {problems && problems.length > 0 ? (
                    problems.map(p => (
                        <div key={p.id} className="bg-surface border border-hairline rounded-lg p-5 hover:border-blue-300 transition-colors shadow-sm flex items-center justify-between">
                            <div className="space-y-1">
                                <Link to={`/problems/${p.slug}`} className="text-lg font-bold text-ink hover:text-brand transition-colors">
                                    {p.title}
                                </Link>
                                <div className="flex flex-wrap gap-2 pt-1 items-center">
                                    <span className={`px-2 py-0.5 rounded text-xs font-semibold uppercase ${p.difficulty === 'easy' ? 'bg-ok/10 text-ok border border-green-100 dark:border-green-800' :
                                        p.difficulty === 'medium' ? 'bg-warn/10 text-warn border border-yellow-100 dark:border-yellow-800' :
                                            'bg-bad/10 text-bad border border-red-100 dark:border-red-800'
                                        }`}>
                                        {p.difficulty}
                                    </span>
                                    {p.tags && p.tags.map((t: string) => (
                                        <span key={t} className="bg-surface-2 border border-hairline text-muted px-2 py-0.5 rounded text-xs">
                                            {t}
                                        </span>
                                    ))}
                                </div>
                            </div>
                            <div className="text-right">
                                <span className="text-sm font-medium text-muted">Acceptance Rate</span>
                                <p className="text-sm font-bold text-ink">
                                    {p.submission_count > 0 ? `${Math.round((p.accepted_count / p.submission_count) * 100)}%` : '0%'}
                                </p>
                            </div>
                        </div>
                    ))
                ) : (
                    <div className="text-center py-12 text-muted-soft bg-surface-2 border border-dashed rounded-lg">
                        {activeTab === 'weak' ? 'Keep submitting! Submit more problems to see your weak topic areas analyzed.' : 'Excellent work! You have cleared all recommended problems in this level.'}
                    </div>
                )}

                {activeTab === 'weak' && rec?.weak_tags?.tags && rec.weak_tags.tags.length > 0 && (
                    <div className="bg-warn/10 border border-warn/30 rounded-lg p-4 text-sm text-warn mt-6">
                        🎯 Recommending problems on your weakest tags: <span className="font-bold">{rec.weak_tags.tags.join(', ')}</span>
                    </div>
                )}
            </div>
        </div>
    )
}
