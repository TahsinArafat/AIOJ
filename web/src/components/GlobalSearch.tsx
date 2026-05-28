import { useState, useEffect, useRef, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'

interface SearchResult {
    id: string
    title: string
    subtitle?: string
    username?: string
}

interface SearchResponse {
    problems: SearchResult[]
    users: SearchResult[]
    contests: SearchResult[]
}

const MAX_RESULTS_PER_GROUP = 5

export default function GlobalSearch() {
    const [query, setQuery] = useState('')
    const [results, setResults] = useState<SearchResponse | null>(null)
    const [isOpen, setIsOpen] = useState(false)
    const [isLoading, setIsLoading] = useState(false)
    const [selectedIndex, setSelectedIndex] = useState(-1)
    const containerRef = useRef<HTMLDivElement>(null)
    const inputRef = useRef<HTMLInputElement>(null)
    const navigate = useNavigate()

    // Flatten results for keyboard navigation
    const flatResults = results
        ? [
            ...results.problems.slice(0, MAX_RESULTS_PER_GROUP).map(r => ({ ...r, type: 'problem' as const })),
            ...results.users.slice(0, MAX_RESULTS_PER_GROUP).map(r => ({ ...r, type: 'user' as const })),
            ...results.contests.slice(0, MAX_RESULTS_PER_GROUP).map(r => ({ ...r, type: 'contest' as const })),
        ]
        : []

    const hasResults = results && (results.problems.length > 0 || results.users.length > 0 || results.contests.length > 0)

    // Debounced search
    useEffect(() => {
        if (!query.trim()) {
            setResults(null)
            setIsOpen(false)
            return
        }

        const timer = setTimeout(async () => {
            setIsLoading(true)
            try {
                const data = await api.search.global(query.trim())
                setResults(data)
                setIsOpen(true)
                setSelectedIndex(-1)
            } catch {
                setResults(null)
            } finally {
                setIsLoading(false)
            }
        }, 300)

        return () => clearTimeout(timer)
    }, [query])

    // Click outside to close
    useEffect(() => {
        const handleClickOutside = (e: MouseEvent) => {
            if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
                setIsOpen(false)
            }
        }
        document.addEventListener('mousedown', handleClickOutside)
        return () => document.removeEventListener('mousedown', handleClickOutside)
    }, [])

    const getResultPath = (result: typeof flatResults[number]) => {
        switch (result.type) {
            case 'problem': return `/problems/${result.id}`
            case 'user': return `/user/${result.username}`
            case 'contest': return `/contests/${result.id}`
        }
    }

    const handleSelect = useCallback((result: typeof flatResults[number]) => {
        navigate(getResultPath(result))
        setQuery('')
        setIsOpen(false)
        inputRef.current?.blur()
    }, [navigate])

    const handleKeyDown = (e: React.KeyboardEvent) => {
        if (!isOpen || !hasResults) {
            if (e.key === 'Escape') {
                inputRef.current?.blur()
            }
            return
        }

        switch (e.key) {
            case 'ArrowDown':
                e.preventDefault()
                setSelectedIndex(prev => (prev < flatResults.length - 1 ? prev + 1 : 0))
                break
            case 'ArrowUp':
                e.preventDefault()
                setSelectedIndex(prev => (prev > 0 ? prev - 1 : flatResults.length - 1))
                break
            case 'Enter':
                e.preventDefault()
                if (selectedIndex >= 0 && selectedIndex < flatResults.length) {
                    handleSelect(flatResults[selectedIndex])
                }
                break
            case 'Escape':
                e.preventDefault()
                setIsOpen(false)
                inputRef.current?.blur()
                break
        }
    }

    const renderGroup = (title: string, items: SearchResult[], type: 'problem' | 'user' | 'contest', startIndex: number) => {
        if (items.length === 0) return null

        return (
            <div key={type}>
                <div className="px-3 py-1.5 text-xs font-semibold text-gray-400 uppercase tracking-wider bg-gray-50">
                    {title}
                </div>
                {items.slice(0, MAX_RESULTS_PER_GROUP).map((item, i) => {
                    const flatIndex = startIndex + i
                    const isSelected = flatIndex === selectedIndex
                    return (
                        <div
                            key={item.id}
                            onClick={() => handleSelect({ ...item, type })}
                            className={`px-3 py-2 text-sm cursor-pointer transition-colors ${
                                isSelected ? 'bg-blue-50 text-blue-700' : 'hover:bg-gray-50 text-gray-700'
                            }`}
                        >
                            <div className="font-medium truncate">{item.title}</div>
                            {item.subtitle && (
                                <div className="text-xs text-gray-400 truncate">{item.subtitle}</div>
                            )}
                        </div>
                    )
                })}
            </div>
        )
    }

    return (
        <div ref={containerRef} className="relative">
            {/* Desktop: Full search input */}
            <div className="hidden md:block relative">
                <div className="absolute inset-y-0 left-0 pl-2.5 flex items-center pointer-events-none">
                    {isLoading ? (
                        <svg className="w-4 h-4 text-gray-400 animate-spin" fill="none" viewBox="0 0 24 24">
                            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                        </svg>
                    ) : (
                        <svg className="w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                        </svg>
                    )}
                </div>
                <input
                    ref={inputRef}
                    type="text"
                    value={query}
                    onChange={e => setQuery(e.target.value)}
                    onFocus={() => { if (hasResults) setIsOpen(true) }}
                    onKeyDown={handleKeyDown}
                    placeholder="Search problems, users, contests..."
                    className="w-56 pl-8 pr-3 py-1.5 text-sm border border-gray-200 rounded-lg bg-gray-50 focus:bg-white focus:border-blue-400 focus:ring-1 focus:ring-blue-400 outline-none transition-all placeholder:text-gray-400"
                />
            </div>

            {/* Mobile: Icon only */}
            <button
                onClick={() => {
                    setIsOpen(true)
                    setTimeout(() => inputRef.current?.focus(), 0)
                }}
                className="md:hidden p-2 text-gray-600 hover:text-gray-800 focus:outline-none transition-colors"
            >
                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                </svg>
            </button>

            {/* Mobile expanded search */}
            {isOpen && (
                <div className="md:hidden fixed inset-x-0 top-0 z-50 bg-white border-b border-gray-200 px-4 py-3 flex items-center gap-2 shadow-sm">
                    <div className="relative flex-1">
                        <div className="absolute inset-y-0 left-0 pl-2.5 flex items-center pointer-events-none">
                            {isLoading ? (
                                <svg className="w-4 h-4 text-gray-400 animate-spin" fill="none" viewBox="0 0 24 24">
                                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                                </svg>
                            ) : (
                                <svg className="w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                                </svg>
                            )}
                        </div>
                        <input
                            type="text"
                            value={query}
                            onChange={e => setQuery(e.target.value)}
                            onKeyDown={handleKeyDown}
                            placeholder="Search..."
                            autoFocus
                            className="w-full pl-8 pr-3 py-2 text-sm border border-gray-200 rounded-lg bg-gray-50 focus:bg-white focus:border-blue-400 focus:ring-1 focus:ring-blue-400 outline-none transition-all"
                        />
                    </div>
                    <button
                        onClick={() => { setIsOpen(false); setQuery('') }}
                        className="text-sm text-gray-500 hover:text-gray-700 px-2 py-1"
                    >
                        Cancel
                    </button>
                </div>
            )}

            {/* Desktop dropdown results */}
            {isOpen && hasResults && (
                <div className="hidden md:block absolute left-0 mt-2 w-80 bg-white rounded-lg shadow-xl border border-gray-200 z-50 overflow-hidden max-h-80 overflow-y-auto">
                    {renderGroup('Problems', results!.problems, 'problem', 0)}
                    {renderGroup('Users', results!.users, 'user', results!.problems.slice(0, MAX_RESULTS_PER_GROUP).length)}
                    {renderGroup('Contests', results!.contests, 'contest', results!.problems.slice(0, MAX_RESULTS_PER_GROUP).length + results!.users.slice(0, MAX_RESULTS_PER_GROUP).length)}
                </div>
            )}

            {/* Desktop empty state */}
            {isOpen && results && !hasResults && query.trim() && (
                <div className="hidden md:block absolute left-0 mt-2 w-80 bg-white rounded-lg shadow-xl border border-gray-200 z-50 overflow-hidden">
                    <div className="px-4 py-6 text-center text-sm text-gray-400">
                        No results found for "{query}"
                    </div>
                </div>
            )}

            {/* Mobile dropdown results */}
            {isOpen && hasResults && (
                <div className="md:hidden fixed inset-x-0 top-14 z-50 bg-white border-b border-gray-200 max-h-[60vh] overflow-y-auto shadow-lg">
                    {renderGroup('Problems', results!.problems, 'problem', 0)}
                    {renderGroup('Users', results!.users, 'user', results!.problems.slice(0, MAX_RESULTS_PER_GROUP).length)}
                    {renderGroup('Contests', results!.contests, 'contest', results!.problems.slice(0, MAX_RESULTS_PER_GROUP).length + results!.users.slice(0, MAX_RESULTS_PER_GROUP).length)}
                </div>
            )}

            {/* Mobile empty state */}
            {isOpen && results && !hasResults && query.trim() && (
                <div className="md:hidden fixed inset-x-0 top-14 z-50 bg-white border-b border-gray-200 shadow-lg">
                    <div className="px-4 py-6 text-center text-sm text-gray-400">
                        No results found for "{query}"
                    </div>
                </div>
            )}
        </div>
    )
}
