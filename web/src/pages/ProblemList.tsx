import { useEffect, useState, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'

function decodeRole(): string | null {
    const token = localStorage.getItem('access_token')
    if (!token) return null
    try {
        const payload = JSON.parse(atob(token.split('.')[1]))
        return payload.role ?? null
    } catch {
        return null
    }
}

const difficultyColor: Record<string, string> = {
    easy: 'text-green-600 bg-green-50',
    medium: 'text-yellow-700 bg-yellow-50',
    hard: 'text-red-600 bg-red-50',
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
    { value: 'easy', label: 'Easy', color: 'text-green-600' },
    { value: 'medium', label: 'Medium', color: 'text-yellow-600' },
    { value: 'hard', label: 'Hard', color: 'text-red-600' },
]

const ratings = [
    { value: '', label: 'All Ratings' },
    { value: '800-1199', label: '800-1199 (Newbie-Pupil)' },
    { value: '1200-1599', label: '1200-1599 (Specialist-Expert)' },
    { value: '1600-1899', label: '1600-1899 (Expert-CM)' },
    { value: '1900-2099', label: '1900-2099 (Master)' },
    { value: '2100-2399', label: '2100-2399 (IM-GM)' },
    { value: '2400+', label: '2400+ (IGM+)' },
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
    const role = decodeRole()
    const canImport = role === 'admin' || role === 'teacher'

    const handleImport = useCallback(async (e: React.ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0]
        if (!file) return
        try {
            const res = await api.problems.importProblem(file)
            window.location.href = `/problems/${res.slug}`
        } catch (err: any) {
            alert('Failed to import problem: ' + (err.message || err))
        }
    }, [])

    const FilterSidebar = () => (
        <div className="w-64 flex-shrink-0 space-y-6">
            {/* Search */}
            <div>
                <h3 className="text-sm font-semibold text-gray-700 mb-2">Search</h3>
                <input type="text" value={search} onChange={e => setSearch(e.target.value)}
                    placeholder="Search problems..." className="w-full border rounded-lg px-3 py-2 text-sm" />
            </div>

            {/* Platform */}
            <div>
                <h3 className="text-sm font-semibold text-gray-700 mb-2">Platform</h3>
                <div className="space-y-1">
                    {platforms.map(p => (
                        <button key={p.value} onClick={() => setSource(p.value)}
                            className={`w-full text-left px-3 py-1.5 rounded text-sm ${source === p.value ? 'bg-blue-50 text-blue-700 font-medium' : 'text-gray-600 hover:bg-gray-50'}`}>
                            {p.label}
                        </button>
                    ))}
                </div>
            </div>

            {/* Difficulty */}
            <div>
                <h3 className="text-sm font-semibold text-gray-700 mb-2">Difficulty</h3>
                <div className="space-y-1">
                    {difficulties.map(d => (
                        <button key={d.value} onClick={() => setDifficulty(d.value)}
                            className={`w-full text-left px-3 py-1.5 rounded text-sm ${d.color || ''} ${difficulty === d.value ? 'bg-blue-50 font-medium' : 'hover:bg-gray-50'}`}>
                            {d.label}
                        </button>
                    ))}
                </div>
            </div>

            {/* Rating */}
            <div>
                <h3 className="text-sm font-semibold text-gray-700 mb-2">Rating</h3>
                <div className="space-y-1">
                    {ratings.map(r => (
                        <button key={r.value} onClick={() => setRating(r.value)}
                            className={`w-full text-left px-3 py-1.5 rounded text-sm ${rating === r.value ? 'bg-blue-50 text-blue-700 font-medium' : 'text-gray-600 hover:bg-gray-50'}`}>
                            {r.label}
                        </button>
                    ))}
                </div>
            </div>

            {/* Tags */}
            {availableTags.length > 0 && (
                <div>
                    <h3 className="text-sm font-semibold text-gray-700 mb-2">Tags</h3>
                    <div className="flex flex-wrap gap-1.5">
                        {availableTags.map(tag => (
                            <button key={tag} onClick={() => toggleTag(tag)}
                                className={`px-2 py-1 rounded text-xs ${selectedTags.includes(tag) ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200'}`}>
                                {tag}
                            </button>
                        ))}
                    </div>
                </div>
            )}

            {/* Sort */}
            <div>
                <h3 className="text-sm font-semibold text-gray-700 mb-2">Sort By</h3>
                <select value={sortBy} onChange={e => setSortBy(e.target.value)}
                    className="w-full border rounded-lg px-3 py-2 text-sm bg-white">
                    {sortOptions.map(s => (
                        <option key={s.value} value={s.value}>{s.label}</option>
                    ))}
                </select>
            </div>

            {/* Clear Filters */}
            {hasActiveFilters && (
                <button onClick={clearFilters}
                    className="w-full text-sm text-red-600 hover:text-red-800 px-3 py-2 border border-red-200 rounded-lg hover:bg-red-50">
                    Clear All Filters
                </button>
            )}
        </div>
    )

    return (
        <div>
            <div className="flex items-center justify-between mb-6">
                <div>
                    <h1 className="text-2xl font-bold">Problems</h1>
                    <p className="text-sm text-gray-500 mt-1">{total} problems found</p>
                </div>
                <div className="flex gap-2">
                    {canImport && (
                        <label className="inline-flex items-center px-4 py-2 border border-transparent rounded-lg shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 cursor-pointer">
                            Import (XML/ZIP)
                            <input type="file" accept=".xml,.zip" className="hidden" onChange={handleImport} />
                        </label>
                    )}
                    <button onClick={() => setShowMobileFilters(!showMobileFilters)}
                        className="md:hidden inline-flex items-center px-4 py-2 border border-gray-300 rounded-lg text-sm font-medium text-gray-700 hover:bg-gray-50">
                        Filters
                    </button>
                </div>
            </div>

            {/* Active filter chips */}
            {hasActiveFilters && (
                <div className="flex flex-wrap gap-2 mb-4">
                    {source && (
                        <span className="inline-flex items-center gap-1 px-2 py-1 bg-blue-50 text-blue-700 rounded text-xs">
                            {platforms.find(p => p.value === source)?.label}
                            <button onClick={() => setSource('')} className="text-blue-500 hover:text-blue-700">×</button>
                        </span>
                    )}
                    {difficulty && (
                        <span className="inline-flex items-center gap-1 px-2 py-1 bg-blue-50 text-blue-700 rounded text-xs">
                            {difficulties.find(d => d.value === difficulty)?.label}
                            <button onClick={() => setDifficulty('')} className="text-blue-500 hover:text-blue-700">×</button>
                        </span>
                    )}
                    {rating && (
                        <span className="inline-flex items-center gap-1 px-2 py-1 bg-blue-50 text-blue-700 rounded text-xs">
                            Rating: {rating}
                            <button onClick={() => setRating('')} className="text-blue-500 hover:text-blue-700">×</button>
                        </span>
                    )}
                    {search && (
                        <span className="inline-flex items-center gap-1 px-2 py-1 bg-blue-50 text-blue-700 rounded text-xs">
                            "{search}"
                            <button onClick={() => setSearch('')} className="text-blue-500 hover:text-blue-700">×</button>
                        </span>
                    )}
                    {selectedTags.map(tag => (
                        <span key={tag} className="inline-flex items-center gap-1 px-2 py-1 bg-blue-50 text-blue-700 rounded text-xs">
                            {tag}
                            <button onClick={() => toggleTag(tag)} className="text-blue-500 hover:text-blue-700">×</button>
                        </span>
                    ))}
                    <button onClick={clearFilters} className="text-xs text-red-600 hover:text-red-800">Clear all</button>
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
                        <div className="absolute right-0 top-0 h-full w-80 bg-white p-4 overflow-y-auto" onClick={e => e.stopPropagation()}>
                            <div className="flex justify-between items-center mb-4">
                                <h2 className="font-semibold">Filters</h2>
                                <button onClick={() => setShowMobileFilters(false)} className="text-gray-500">✕</button>
                            </div>
                            <FilterSidebar />
                        </div>
                    </div>
                )}

                {/* Problem table */}
                <div className="flex-1 min-w-0">
                    <div className="border border-gray-200 rounded-lg overflow-hidden">
                        <table className="w-full text-sm">
                            <thead className="bg-gray-50 text-gray-500 text-xs uppercase">
                                <tr>
                                    <th className="px-4 py-3 text-left">Title</th>
                                    <th className="px-4 py-3 text-left">Platform</th>
                                    <th className="px-4 py-3 text-left">Difficulty</th>
                                    <th className="px-4 py-3 text-left">Tags</th>
                                    <th className="px-4 py-3 text-right">Solved</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-gray-100">
                                {problems.map(p => (
                                    <tr key={p.slug} className="hover:bg-gray-50 transition-colors">
                                        <td className="px-4 py-3">
                                            <Link to={`/problems/${p.slug}`} className="font-medium text-blue-600 hover:underline">
                                                {p.title}
                                            </Link>
                                        </td>
                                        <td className="px-4 py-3">
                                            <span className={`px-2 py-0.5 rounded text-xs font-medium ${
                                                p.source === 'codeforces' ? 'bg-orange-50 text-orange-700' :
                                                p.source === 'atcoder' ? 'bg-purple-50 text-purple-700' :
                                                p.source === 'cses' ? 'bg-cyan-50 text-cyan-700' :
                                                p.source === 'toph' ? 'bg-pink-50 text-pink-700' :
                                                p.source === 'qoj' ? 'bg-gray-50 text-gray-700' :
                                                'bg-blue-50 text-blue-700'
                                            }`}>
                                                {p.source === 'local' || !p.source ? 'AIOJ' : p.source?.charAt(0).toUpperCase() + p.source?.slice(1)}
                                            </span>
                                        </td>
                                        <td className="px-4 py-3">
                                            <span className={`px-2 py-0.5 rounded text-xs font-medium ${difficultyColor[p.difficulty] || ''}`}>
                                                {p.difficulty}
                                            </span>
                                        </td>
                                        <td className="px-4 py-3 text-gray-500 text-xs max-w-xs truncate">
                                            {p.tags?.join(', ') || '—'}
                                        </td>
                                        <td className="px-4 py-3 text-right text-gray-500 text-xs">
                                            {p.accepted_count}/{p.submission_count}
                                        </td>
                                    </tr>
                                ))}
                                {problems.length === 0 && (
                                    <tr><td colSpan={5} className="px-4 py-8 text-center text-gray-400">No problems found.</td></tr>
                                )}
                            </tbody>
                        </table>
                    </div>
                    <div className="flex justify-between mt-4 text-sm text-gray-500">
                        <span>{total} problems</span>
                        <div className="flex gap-2">
                            <button onClick={() => setOffset(Math.max(0, offset - limit))} disabled={offset === 0}
                                className="px-3 py-1 border rounded disabled:opacity-40 hover:bg-gray-50">Prev</button>
                            <button onClick={() => setOffset(offset + limit)} disabled={offset + limit >= total}
                                className="px-3 py-1 border rounded disabled:opacity-40 hover:bg-gray-50">Next</button>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    )
}
