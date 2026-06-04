import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'

export default function Practice() {
    const [rec, setRec] = useState<any>(null)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')
    const [activeTab, setActiveTab] = useState<'hybrid' | 'progression' | 'weak'>('hybrid')

    useEffect(() => {
        if (!getAccessToken()) {
            setLoading(false)
            return
        }
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

    if (loading) return <div className="text-center py-12 text-gray-500 dark:text-gray-400">Loading recommendations...</div>
    if (error) return <div className="max-w-xl mx-auto bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300 p-4 rounded-md border border-red-100 dark:border-red-800 my-6">{error}</div>

    if (!getAccessToken()) {
        return (
            <div className="max-w-4xl mx-auto text-center py-20">
                <h1 className="text-3xl font-extrabold tracking-tight text-gray-900 dark:text-gray-100 mb-4">Personalized Practice</h1>
                <p className="text-gray-600 dark:text-gray-400 mb-6">Log in to get smart problem recommendations tailored to your rating and weak areas.</p>
                <a href="/login" className="bg-blue-600 text-white px-6 py-3 rounded-lg font-medium hover:bg-blue-700 transition-colors">
                    Log In to Practice
                </a>
            </div>
        )
    }

    const problems = activeTab === 'hybrid' ? rec?.hybrid : activeTab === 'progression' ? rec?.progression : rec?.weak_tags?.problems

    return (
        <div className="max-w-4xl mx-auto">
            <header className="mb-8">
                <h1 className="text-3xl font-extrabold tracking-tight text-gray-900 dark:text-gray-100">Personalized Practice</h1>
                <p className="mt-2 text-gray-600 dark:text-gray-400">Smart problem recommendations tailored to your rating and weak areas.</p>
            </header>

            {/* Profile summary banner */}
            <div className="bg-gradient-to-r from-blue-50 to-indigo-50 rounded-xl p-6 border border-blue-100 dark:border-blue-800 mb-8 flex justify-between items-center">
                <div>
                    <h3 className="font-semibold text-blue-900 text-lg">Practice Mode Active</h3>
                    <p className="text-sm text-blue-700 dark:text-blue-300 mt-1">Analyzing your contest ratings and submissions to offer smart challenges.</p>
                </div>
                <div className="bg-blue-600 text-white font-mono px-4 py-2 rounded-lg text-sm font-bold shadow-sm">
                    Level Up
                </div>
            </div>

            {/* Tabs selector */}
            <div className="border-b border-gray-200 dark:border-gray-700 mb-6">
                <nav className="flex gap-6">
                    <button onClick={() => setActiveTab('hybrid')}
                        className={`pb-4 text-sm font-medium ${activeTab === 'hybrid' ? 'border-b-2 border-blue-600 text-blue-600 dark:text-blue-400 font-semibold' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'}`}>
                        Daily Diet (Hybrid)
                    </button>
                    <button onClick={() => setActiveTab('progression')}
                        className={`pb-4 text-sm font-medium ${activeTab === 'progression' ? 'border-b-2 border-blue-600 text-blue-600 dark:text-blue-400 font-semibold' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'}`}>
                        Progression Ladder
                    </button>
                    <button onClick={() => setActiveTab('weak')}
                        className={`pb-4 text-sm font-medium ${activeTab === 'weak' ? 'border-b-2 border-blue-600 text-blue-600 dark:text-blue-400 font-semibold' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'}`}>
                        Targeted Tags Practice
                    </button>
                </nav>
            </div>

            {/* Recommendations Content */}
            <div className="space-y-4">
                {problems && problems.length > 0 ? (
                    problems.map((p: any) => (
                        <div key={p.id} className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-5 hover:border-blue-300 transition-colors shadow-sm flex items-center justify-between">
                            <div className="space-y-1">
                                <Link to={`/problems/${p.slug}`} className="text-lg font-bold text-gray-900 dark:text-gray-100 hover:text-blue-600 transition-colors">
                                    {p.title}
                                </Link>
                                <div className="flex flex-wrap gap-2 pt-1 items-center">
                                    <span className={`px-2 py-0.5 rounded text-xs font-semibold uppercase ${
                                        p.difficulty === 'easy' ? 'bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-300 border border-green-100 dark:border-green-800' :
                                        p.difficulty === 'medium' ? 'bg-yellow-50 dark:bg-yellow-900/20 text-yellow-700 dark:text-yellow-300 border border-yellow-100 dark:border-yellow-800' :
                                        'bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300 border border-red-100 dark:border-red-800'
                                    }`}>
                                        {p.difficulty}
                                    </span>
                                    {p.tags && p.tags.map((t: string) => (
                                        <span key={t} className="bg-gray-50 dark:bg-gray-800 border border-gray-100 dark:border-gray-700 text-gray-600 dark:text-gray-400 px-2 py-0.5 rounded text-xs">
                                            {t}
                                        </span>
                                    ))}
                                </div>
                            </div>
                            <div className="text-right">
                                <span className="text-sm font-medium text-gray-500 dark:text-gray-400">Acceptance Rate</span>
                                <p className="text-sm font-bold text-gray-900 dark:text-gray-100">
                                    {p.submission_count > 0 ? `${Math.round((p.accepted_count / p.submission_count) * 100)}%` : '0%'}
                                </p>
                            </div>
                        </div>
                    ))
                ) : (
                    <div className="text-center py-12 text-gray-400 dark:text-gray-500 bg-gray-50 dark:bg-gray-800 border border-dashed rounded-lg">
                        {activeTab === 'weak' ? 'Keep submitting! Submit more problems to see your weak topic areas analyzed.' : 'Excellent work! You have cleared all recommended problems in this level.'}
                    </div>
                )}

                {activeTab === 'weak' && rec?.weak_tags?.tags?.length > 0 && (
                    <div className="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-700 rounded-lg p-4 text-sm text-amber-800 dark:text-amber-300 mt-6">
                        🎯 Recommending problems on your weakest tags: <span className="font-bold">{rec.weak_tags.tags.join(', ')}</span>
                    </div>
                )}
            </div>
        </div>
    )
}
