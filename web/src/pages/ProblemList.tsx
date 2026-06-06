import { useEffect, useState, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'

const difficultyColor: Record<string, string> = {
    easy: 'text-green-600 dark:text-green-400 bg-green-50 dark:bg-green-900/20',
    medium: 'text-yellow-700 dark:text-yellow-300 bg-yellow-50 dark:bg-yellow-900/20',
    hard: 'text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20',
}

const platforms = [
    { value: '', label: 'All Platforms' },
    { value: 'local', label: 'AIOJ' },
    { value: 'codeforces', label: 'Codeforces' },
    { value: 'atcoder', label: 'AtCoder' },
    { value: 'cses', label: 'CSES' },
    { value: 'toph', label: 'Toph' },
    { value: 'qoj', label: 'QOJ' },
]

const difficulties = [
    { value: '', label: 'All Difficulties' },
    { value: 'easy', label: 'Easy', color: 'text-green-600 dark:text-green-400' },
    { value: 'medium', label: 'Medium', color: 'text-yellow-600 dark:text-yellow-400' },
    { value: 'hard', label: 'Hard', color: 'text-red-600 dark:text-red-400' },
]

const ratings = [
    { value: '', label: 'All Ratings' },
    { value: '800-1199', label: '800-1199 (Novice-Apprentice)' },
    { value: '1200-1599', label: '1200-1599 (Adept-Elite)' },
    { value: '1600-1899', label: '1600-1899 (Elite-Champion)' },
    { value: '1900-2099', label: '1900-2099 (Master)' },
    { value: '2100-2399', label: '2100-2399 (Grandmaster-Titan)' },
    { value: '2400+', label: '2400+ (Immortal+)' },
]

const sortOptions = [
    { value: 'newest', label: 'Most Recent' },
    { value: 'oldest', label: 'Oldest' },
    { value: 'most_solved', label: 'Most Solved' },
    { value: 'least_solved', label: 'Least Solved' },
    { value: 'title_asc', label: 'Title A-Z' },
]

export default function ProblemList() {
    const [problems, setProblems] = useState<any[]>([])
    const [total, setTotal] = useState(0)
    const [offset, setOffset] = useState(0)
    const [difficulty, setDifficulty] = useState('')
    const [selectedTags, setSelectedTags] = useState<string[]>([])
    const [search, setSearch] = useState('')
    const [source, setSource] = useState('')
    const [rating, setRating] = useState('')
    const [sortBy, setSortBy] = useState('newest')
    const [availableTags, setAvailableTags] = useState<string[]>([])
    const [showMobileFilters, setShowMobileFilters] = useState(false)
    const limit = 20

    useEffect(() => {
        api.problems.listTags().then(d => setAvailableTags(d.data || [])).catch(() => {})
    }, [])

    const fetchProblems = useCallback(() => {
        const filters: any = {}
        if (difficulty) filters.difficulty = difficulty
        if (selectedTags.length > 0) filters.tags = selectedTags
        if (search) filters.search = search
        if (source) filters.source = source
        if (rating) filters.rating = rating
        filters.sort = sortBy
        
        api.problems.list(offset, limit, filters).then(d => {
            setProblems(d.data || [])
            setTotal(d.total || 0)
        }).catch(() => {})
    }, [offset, difficulty, selectedTags, search, source, rating, sortBy])

    useEffect(() => {
        setOffset(0)
        fetchProblems()
    }, [difficulty, selectedTags, search, source, rating, sortBy])

    useEffect(() => {
        fetchProblems()
    }, [offset])

    const toggleTag = (tag: string) => {
        setSelectedTags(prev => prev.includes(tag) ? prev.filter(t => t !== tag) : [...prev, tag])
    }

    const clearFilters = () => {
        setDifficulty('')
        setSelectedTags([])
        setSearch('')
        setSource('')
        setRating('')
        setSortBy('newest')
    }

    const hasActiveFilters = difficulty || selectedTags.length > 0 || search || source || rating || sortBy !== 'newest'

    const FilterSidebar = () => (
        <div className="w-64 flex-shrink-0 space-y-6">
            {/* Search */}
            <div>
                <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2">Search</h3>
                <input type="text" value={search} onChange={e => setSearch(e.target.value)}
                    placeholder="Search problems..." className="w-full border rounded-lg px-3 py-2 text-sm" />
            </div>

            {/* Platform */}
            <div>
                <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2">Platform</h3>
                <div className="space-y-1">
                    {platforms.map(p => (
                        <button key={p.value} onClick={() => setSource(p.value)}
                            className={`w-full text-left px-3 py-1.5 rounded text-sm ${source === p.value ? 'bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300 font-medium' : 'text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700'}`}>
                            {p.label}
                        </button>
                    ))}
                </div>
            </div>

            {/* Difficulty */}
            <div>
                <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2">Difficulty</h3>
                <div className="space-y-1">
                    {difficulties.map(d => (
                        <button key={d.value} onClick={() => setDifficulty(d.value)}
                            className={`w-full text-left px-3 py-1.5 rounded text-sm ${d.color || ''} ${difficulty === d.value ? 'bg-blue-50 dark:bg-blue-900/20 font-medium' : 'hover:bg-gray-50 dark:hover:bg-gray-700'}`}>
                            {d.label}
                        </button>
                    ))}
                </div>
            </div>

            {/* Rating */}
            <div>
                <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2">Rating</h3>
                <div className="space-y-1">
                    {ratings.map(r => (
                        <button key={r.value} onClick={() => setRating(r.value)}
                            className={`w-full text-left px-3 py-1.5 rounded text-sm ${rating === r.value ? 'bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300 font-medium' : 'text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700'}`}>
                            {r.label}
                        </button>
                    ))}
                </div>
            </div>

            {/* Tags */}
            {availableTags.length > 0 && (
                <div>
                    <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2">Tags</h3>
                    <div className="flex flex-wrap gap-1.5">
                        {availableTags.map(tag => (
                            <button key={tag} onClick={() => toggleTag(tag)}
                                className={`px-2 py-1 rounded text-xs ${selectedTags.includes(tag) ? 'bg-blue-600 text-white' : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-600'}`}>
                                {tag}
                            </button>
                        ))}
                    </div>
                </div>
            )}

            {/* Sort */}
            <div>
                <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2">Sort By</h3>
                <select value={sortBy} onChange={e => setSortBy(e.target.value)}
                    className="w-full border rounded-lg px-3 py-2 text-sm bg-white dark:bg-gray-800">
                    {sortOptions.map(s => (
                        <option key={s.value} value={s.value}>{s.label}</option>
                    ))}
                </select>
            </div>

            {/* Clear Filters */}
            {hasActiveFilters && (
                <button onClick={clearFilters}
                    className="w-full text-sm text-red-600 dark:text-red-400 hover:text-red-800 dark:hover:text-red-300 px-3 py-2 border border-red-200 dark:border-red-800 rounded-lg hover:bg-red-50 dark:hover:bg-red-900/20">
                    Clear All Filters
                </button>
            )}
        </div>
    )

    return (
        <div>
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-6">
                <div>
                    <h1 className="text-2xl font-bold">Problems</h1>
                    <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">{total} problems found</p>
                </div>
                <button onClick={() => setShowMobileFilters(!showMobileFilters)}
                    className="md:hidden inline-flex items-center px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700">
                    Filters
                </button>
            </div>

            {/* Active filter chips */}
            {hasActiveFilters && (
                <div className="flex flex-wrap gap-2 mb-4">
                    {source && (
                        <span className="inline-flex items-center gap-1 px-2 py-1 bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300 rounded text-xs">
                            {platforms.find(p => p.value === source)?.label}
                            <button onClick={() => setSource('')} className="text-blue-500 dark:text-blue-400 hover:text-blue-700">×</button>
                        </span>
                    )}
                    {difficulty && (
                        <span className="inline-flex items-center gap-1 px-2 py-1 bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300 rounded text-xs">
                            {difficulties.find(d => d.value === difficulty)?.label}
                            <button onClick={() => setDifficulty('')} className="text-blue-500 dark:text-blue-400 hover:text-blue-700">×</button>
                        </span>
                    )}
                    {rating && (
                        <span className="inline-flex items-center gap-1 px-2 py-1 bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300 rounded text-xs">
                            Rating: {rating}
                            <button onClick={() => setRating('')} className="text-blue-500 dark:text-blue-400 hover:text-blue-700">×</button>
                        </span>
                    )}
                    {search && (
                        <span className="inline-flex items-center gap-1 px-2 py-1 bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300 rounded text-xs">
                            "{search}"
                            <button onClick={() => setSearch('')} className="text-blue-500 dark:text-blue-400 hover:text-blue-700">×</button>
                        </span>
                    )}
                    {selectedTags.map(tag => (
                        <span key={tag} className="inline-flex items-center gap-1 px-2 py-1 bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300 rounded text-xs">
                            {tag}
                            <button onClick={() => toggleTag(tag)} className="text-blue-500 dark:text-blue-400 hover:text-blue-700">×</button>
                        </span>
                    ))}
                    <button onClick={clearFilters} className="text-xs text-red-600 dark:text-red-400 hover:text-red-800 dark:hover:text-red-300">Clear all</button>
                </div>
            )}

            <div className="flex gap-6">
                {/* Sidebar */}
                <div className="hidden md:block">
                    <FilterSidebar />
                </div>

                {/* Mobile filter overlay */}
                {showMobileFilters && (
                    <div className="md:hidden fixed inset-0 z-50 bg-black/50" onClick={() => setShowMobileFilters(false)}>
                        <div className="absolute right-0 top-0 h-full w-80 bg-white dark:bg-gray-800 p-4 overflow-y-auto" onClick={e => e.stopPropagation()}>
                            <div className="flex justify-between items-center mb-4">
                                <h2 className="font-semibold">Filters</h2>
                                <button onClick={() => setShowMobileFilters(false)} className="text-gray-500 dark:text-gray-400">✕</button>
                            </div>
                            <FilterSidebar />
                        </div>
                    </div>
                )}

                {/* Problem table - Desktop */}
                <div className="flex-1 min-w-0">
                    <div className="hidden md:block border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
                        <table className="w-full text-sm">
                            <thead className="bg-gray-50 dark:bg-gray-800 text-gray-500 dark:text-gray-400 text-xs uppercase">
                                <tr>
                                    <th className="px-4 py-3 text-left">Title</th>
                                    <th className="px-4 py-3 text-left">Platform</th>
                                    <th className="px-4 py-3 text-left">Difficulty</th>
                                    <th className="px-4 py-3 text-left">Tags</th>
                                    <th className="px-4 py-3 text-right">Solved</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-gray-100 dark:divide-gray-700">
                                {problems.map(p => (
                                    <tr key={p.slug} className="hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors">
                                        <td className="px-4 py-3">
                                            <Link to={`/problems/${p.slug}`} className="font-medium text-blue-600 dark:text-blue-400 hover:underline">
                                                {p.title}
                                            </Link>
                                        </td>
                                        <td className="px-4 py-3">
                                            <span className={`px-2 py-0.5 rounded text-xs font-medium ${
                                                p.source === 'codeforces' ? 'bg-orange-50 dark:bg-orange-900/20 text-orange-700 dark:text-orange-300' :
                                                p.source === 'atcoder' ? 'bg-purple-50 dark:bg-purple-900/20 text-purple-700 dark:text-purple-300' :
                                                p.source === 'cses' ? 'bg-cyan-50 text-cyan-700' :
                                                p.source === 'toph' ? 'bg-pink-50 dark:bg-pink-900/20 text-pink-700' :
                                                p.source === 'qoj' ? 'bg-gray-50 dark:bg-gray-800 text-gray-700 dark:text-gray-300' :
                                                'bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300'
                                            }`}>
                                                {p.source === 'local' || !p.source ? 'AIOJ' : p.source?.charAt(0).toUpperCase() + p.source?.slice(1)}
                                            </span>
                                        </td>
                                        <td className="px-4 py-3">
                                            <span className={`px-2 py-0.5 rounded text-xs font-medium ${difficultyColor[p.difficulty] || ''}`}>
                                                {p.difficulty}
                                            </span>
                                        </td>
                                        <td className="px-4 py-3 text-gray-500 dark:text-gray-400 text-xs max-w-xs truncate">
                                            {p.tags?.join(', ') || '—'}
                                        </td>
                                        <td className="px-4 py-3 text-right text-gray-500 dark:text-gray-400 text-xs">
                                            {p.accepted_count}/{p.submission_count}
                                        </td>
                                    </tr>
                                ))}
                                {problems.length === 0 && (
                                    <tr><td colSpan={5} className="px-4 py-8 text-center text-gray-400 dark:text-gray-500">No problems found.</td></tr>
                                )}
                            </tbody>
                        </table>
                    </div>

                    {/* Problem cards - Mobile */}
                    <div className="md:hidden space-y-3">
                        {problems.map(p => (
                            <div key={p.slug} className="border border-gray-200 dark:border-gray-700 rounded-lg p-4 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors">
                                <Link to={`/problems/${p.slug}`} className="block">
                                    <h3 className="font-medium text-blue-600 dark:text-blue-400 hover:underline mb-2">
                                        {p.title}
                                    </h3>
                                </Link>
                                <div className="flex flex-wrap items-center gap-2 mb-2">
                                    <span className={`px-2 py-0.5 rounded text-xs font-medium ${
                                        p.source === 'codeforces' ? 'bg-orange-50 dark:bg-orange-900/20 text-orange-700 dark:text-orange-300' :
                                        p.source === 'atcoder' ? 'bg-purple-50 dark:bg-purple-900/20 text-purple-700 dark:text-purple-300' :
                                        p.source === 'cses' ? 'bg-cyan-50 text-cyan-700' :
                                        p.source === 'toph' ? 'bg-pink-50 dark:bg-pink-900/20 text-pink-700' :
                                        p.source === 'qoj' ? 'bg-gray-50 dark:bg-gray-800 text-gray-700 dark:text-gray-300' :
                                        'bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300'
                                    }`}>
                                        {p.source === 'local' || !p.source ? 'AIOJ' : p.source?.charAt(0).toUpperCase() + p.source?.slice(1)}
                                    </span>
                                    <span className={`px-2 py-0.5 rounded text-xs font-medium ${difficultyColor[p.difficulty] || ''}`}>
                                        {p.difficulty}
                                    </span>
                                    <span className="text-xs text-gray-500 dark:text-gray-400 ml-auto">
                                        {p.accepted_count}/{p.submission_count} solved
                                    </span>
                                </div>
                                {p.tags && p.tags.length > 0 && (
                                    <div className="flex flex-wrap gap-1.5">
                                        {p.tags.slice(0, 3).map((tag: string) => (
                                            <span key={tag} className="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 rounded text-xs">
                                                {tag}
                                            </span>
                                        ))}
                                        {p.tags.length > 3 && (
                                            <span className="px-1.5 py-0.5 text-gray-400 dark:text-gray-500 text-xs">
                                                +{p.tags.length - 3} more
                                            </span>
                                        )}
                                    </div>
                                )}
                            </div>
                        ))}
                        {problems.length === 0 && (
                            <div className="px-4 py-8 text-center text-gray-400 dark:text-gray-500">No problems found.</div>
                        )}
                    </div>

                    {/* Pagination */}
                    <div className="flex items-center justify-between mt-4 text-sm text-gray-500 dark:text-gray-400">
                        <span>{total} problems</span>
                        <div className="flex gap-2">
                            <button onClick={() => setOffset(Math.max(0, offset - limit))} disabled={offset === 0}
                                className="px-3 py-1 border rounded disabled:opacity-40 hover:bg-gray-50 dark:hover:bg-gray-700">Prev</button>
                            <button onClick={() => setOffset(offset + limit)} disabled={offset + limit >= total}
                                className="px-3 py-1 border rounded disabled:opacity-40 hover:bg-gray-50 dark:hover:bg-gray-700">Next</button>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    )
}
