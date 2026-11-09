import { useEffect, useState, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { SlidersHorizontal, X } from 'lucide-react'
import { api } from '../lib/api'
import { EmptyState } from '../components/EmptyState'
import { Badge } from '../components/ui/Badge'
import { Button } from '../components/ui/Button'
import { DIFFICULTY_TONE } from '../lib/difficulty'
import { Card } from '../components/ui/Card'
import { Input, Select } from '../components/ui/Field'
import { PageHeader } from '../components/ui/PageHeader'
import { Table, TableShell, Td, Th, THead, Tr } from '../components/ui/Table'

const platforms = [
    { value: '', label: 'All Platforms' },
    { value: 'local', label: 'AIOJ' },
    { value: 'ai', label: 'AI' },
    { value: 'codeforces', label: 'Codeforces' },
    { value: 'atcoder', label: 'AtCoder' },
    { value: 'cses', label: 'CSES' },
    { value: 'toph', label: 'Toph' },
    { value: 'qoj', label: 'QOJ' },
]

const difficulties = [
    { value: '', label: 'All Difficulties' },
    { value: 'easy', label: 'Easy' },
    { value: 'medium', label: 'Medium' },
    { value: 'hard', label: 'Hard' },
]

const ratings = [
    { value: '', label: 'All Ratings' },
    { value: '800-1199', label: '800–1199' },
    { value: '1200-1599', label: '1200–1599' },
    { value: '1600-1899', label: '1600–1899' },
    { value: '1900-2099', label: '1900–2099' },
    { value: '2100-2399', label: '2100–2399' },
    { value: '2400+', label: '2400+' },
]

const sortOptions = [
    { value: 'newest', label: 'Most Recent' },
    { value: 'oldest', label: 'Oldest' },
    { value: 'most_solved', label: 'Most Solved' },
    { value: 'least_solved', label: 'Least Solved' },
    { value: 'title_asc', label: 'Title A-Z' },
]

const PLATFORM_LABEL: Record<string, string> = {
    local: 'AIOJ',
    codeforces: 'Codeforces',
    atcoder: 'AtCoder',
    cses: 'CSES',
    toph: 'Toph',
    qoj: 'QOJ',
}

interface ProblemRow { title?: string; slug?: string; difficulty?: string; rating?: number; source?: string; tags?: string[]; accepted_count?: number; submission_count?: number }

function acceptance(p: ProblemRow): string {
    const total = p.submission_count || 0
    if (!total) return '—'
    return `${Math.round(((p.accepted_count || 0) / total) * 100)}%`
}

export default function ProblemList() {
    const [problems, setProblems] = useState<ProblemRow[]>([])
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
        api.problems.listTags().then(d => setAvailableTags(d.data || [])).catch(() => { })
    }, [])

    const fetchProblems = useCallback(() => {
        const filters: { difficulty?: string; tags?: string[]; search?: string; source?: string; rating?: string; sort?: string } = {}
        if (difficulty) filters.difficulty = difficulty
        if (selectedTags.length > 0) filters.tags = selectedTags
        if (search) filters.search = search
        if (source) filters.source = source
        if (rating) filters.rating = rating
        filters.sort = sortBy

        api.problems.list(offset, limit, filters).then(d => {
            setProblems(d.data || [])
            setTotal(d.total || 0)
        }).catch(() => { })
    }, [offset, difficulty, selectedTags, search, source, rating, sortBy])

    useEffect(() => {
        queueMicrotask(() => setOffset(0))
        fetchProblems()
    }, [difficulty, selectedTags, search, source, rating, sortBy]) // eslint-disable-line react-hooks/exhaustive-deps

    useEffect(() => {
        fetchProblems()
    }, [offset]) // eslint-disable-line react-hooks/exhaustive-deps

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

    const filterPanel = () => (
        <div className="space-y-5">
            <div>
                <FilterLabel>Search</FilterLabel>
                <Input
                    type="search"
                    value={search}
                    onChange={e => setSearch(e.target.value)}
                    placeholder="Search problems…"
                    aria-label="Search problems"
                />
            </div>

            <div>
                <FilterLabel>Platform</FilterLabel>
                <div className="space-y-0.5">
                    {platforms.map(p => (
                        <FilterOption key={p.value} active={source === p.value} onClick={() => setSource(p.value)}>
                            {p.label}
                        </FilterOption>
                    ))}
                </div>
            </div>

            <div>
                <FilterLabel>Difficulty</FilterLabel>
                <div className="space-y-0.5">
                    {difficulties.map(d => (
                        <FilterOption key={d.value} active={difficulty === d.value} onClick={() => setDifficulty(d.value)}>
                            {d.label}
                        </FilterOption>
                    ))}
                </div>
            </div>

            <div>
                <FilterLabel>Rating</FilterLabel>
                <div className="space-y-0.5">
                    {ratings.map(r => (
                        <FilterOption key={r.value} active={rating === r.value} onClick={() => setRating(r.value)}>
                            {r.label}
                        </FilterOption>
                    ))}
                </div>
            </div>

            {availableTags.length > 0 && (
                <div>
                    <FilterLabel>Tags</FilterLabel>
                    <div className="flex flex-wrap gap-1.5">
                        {availableTags.map(tag => {
                            const active = selectedTags.includes(tag)
                            return (
                                <button
                                    key={tag}
                                    type="button"
                                    onClick={() => toggleTag(tag)}
                                    aria-pressed={active}
                                    className={`rounded-md px-2 py-1 text-xs transition-colors ${active
                                        ? 'bg-brand text-on-brand'
                                        : 'bg-surface-3 text-ink-soft hover:bg-hairline'
                                        }`}
                                >
                                    {tag}
                                </button>
                            )
                        })}
                    </div>
                </div>
            )}

            <div>
                <FilterLabel>Sort by</FilterLabel>
                <Select value={sortBy} onChange={e => setSortBy(e.target.value)} aria-label="Sort problems">
                    {sortOptions.map(s => (
                        <option key={s.value} value={s.value}>{s.label}</option>
                    ))}
                </Select>
            </div>

            {hasActiveFilters && (
                <Button variant="ghost" size="sm" className="w-full text-bad hover:bg-bad/10" onClick={clearFilters}>
                    Clear all filters
                </Button>
            )}
        </div>
    )

    return (
        <div>
            <PageHeader
                title="Problems"
                description={`${total.toLocaleString()} problems across ${platforms.length - 1} platforms`}
                actions={
                    <Button
                        variant="secondary"
                        className="lg:hidden"
                        onClick={() => setShowMobileFilters(true)}
                    >
                        <SlidersHorizontal className="h-4 w-4" />
                        Filters
                    </Button>
                }
            />

            {/* Active filter chips */}
            {hasActiveFilters && (
                <div className="mb-4 flex flex-wrap items-center gap-1.5">
                    {source && <Chip label={PLATFORM_LABEL[source] || source} onClear={() => setSource('')} />}
                    {difficulty && <Chip label={difficulty} onClear={() => setDifficulty('')} />}
                    {rating && <Chip label={`Rating ${rating}`} onClear={() => setRating('')} />}
                    {search && <Chip label={`“${search}”`} onClear={() => setSearch('')} />}
                    {selectedTags.map(tag => <Chip key={tag} label={tag} onClear={() => toggleTag(tag)} />)}
                    <button
                        type="button"
                        onClick={clearFilters}
                        className="ml-1 text-xs font-medium text-muted transition-colors hover:text-ink"
                    >
                        Clear all
                    </button>
                </div>
            )}

            <div className="flex gap-6">
                {/* Sidebar */}
                <aside className="hidden w-56 shrink-0 lg:block">
                    <div className="sticky top-20">
                        <Card className="p-4">{filterPanel()}</Card>
                    </div>
                </aside>

                {/* Mobile filter drawer */}
                {showMobileFilters && (
                    <div
                        className="fixed inset-0 z-50 flex justify-end bg-black/50 lg:hidden"
                        onClick={() => setShowMobileFilters(false)}
                    >
                        <div
                            role="dialog"
                            aria-modal="true"
                            aria-label="Filters"
                            className="h-full w-[min(20rem,88vw)] overflow-y-auto border-l border-hairline bg-surface p-4"
                            onClick={e => e.stopPropagation()}
                        >
                            <div className="mb-4 flex items-center justify-between">
                                <h2 className="text-sm font-semibold text-ink">Filters</h2>
                                <button
                                    type="button"
                                    onClick={() => setShowMobileFilters(false)}
                                    aria-label="Close filters"
                                    className="flex h-9 w-9 items-center justify-center rounded-lg text-muted hover:bg-surface-2 hover:text-ink"
                                >
                                    <X className="h-4 w-4" />
                                </button>
                            </div>
                            {filterPanel()}
                        </div>
                    </div>
                )}

                {/* Results */}
                <div className="min-w-0 flex-1">
                    <div className="hidden md:block">
                        <TableShell>
                            <Table>
                                <THead>
                                    <tr>
                                        <Th>Problem</Th>
                                        <Th className="w-32">Platform</Th>
                                        <Th className="w-28">Difficulty</Th>
                                        <Th>Tags</Th>
                                        <Th className="w-44 text-right">Acceptance</Th>
                                    </tr>
                                </THead>
                                <tbody>
                                    {problems.map(p => (
                                        <Tr key={p.slug}>
                                            <Td>
                                                <Link
                                                    to={`/problems/${p.slug}`}
                                                    className="font-medium text-ink transition-colors hover:text-brand"
                                                >
                                                    {p.title}
                                                </Link>
                                            </Td>
                                            <Td>
                                                <Badge tone="neutral">{PLATFORM_LABEL[p.source || 'local'] || p.source}</Badge>
                                            </Td>
                                            <Td>
                                                <span className="flex items-center gap-1.5">
                                                    {p.difficulty ? (
                                                        <Badge tone={DIFFICULTY_TONE[p.difficulty] || 'neutral'}>{p.difficulty}</Badge>
                                                    ) : (
                                                        <span className="text-muted-soft">—</span>
                                                    )}
                                                    {p.rating ? <span className="tnum text-xs text-muted">{p.rating}</span> : null}
                                                </span>
                                            </Td>
                                            <Td className="max-w-[18rem]">
                                                <span className="line-clamp-1 text-xs text-muted">
                                                    {p.tags?.join(', ') || '—'}
                                                </span>
                                            </Td>
                                            <Td className="whitespace-nowrap text-right">
                                                <span className="tnum text-xs text-muted">
                                                    <span className="font-medium text-ink-soft">{acceptance(p)}</span>
                                                    <span aria-hidden="true" className="mx-1 text-hairline-strong">·</span>
                                                    {p.accepted_count}/{p.submission_count}
                                                </span>
                                            </Td>
                                        </Tr>
                                    ))}
                                    {problems.length === 0 && (
                                        <tr>
                                            <td colSpan={5}>
                                                <EmptyState
                                                    text="No problems found"
                                                    description="Try a different search or filter — problem sets sync in from judges."
                                                />
                                            </td>
                                        </tr>
                                    )}
                                </tbody>
                            </Table>
                        </TableShell>
                    </div>

                    {/* Mobile cards */}
                    <div className="space-y-2.5 md:hidden">
                        {problems.map(p => (
                            <Card key={p.slug} className="p-4">
                                <Link to={`/problems/${p.slug}`} className="block">
                                    <h3 className="text-sm font-medium text-ink">{p.title}</h3>
                                </Link>
                                <div className="mt-2 flex flex-wrap items-center gap-1.5">
                                    <Badge tone="neutral">{PLATFORM_LABEL[p.source || 'local'] || p.source}</Badge>
                                    {p.difficulty && <Badge tone={DIFFICULTY_TONE[p.difficulty] || 'neutral'}>{p.difficulty}</Badge>}
                                    {p.rating ? <span className="tnum text-xs text-muted">{p.rating}</span> : null}
                                    <span className="tnum ml-auto text-xs text-muted">
                                        {p.accepted_count}/{p.submission_count}
                                    </span>
                                </div>
                                {p.tags && p.tags.length > 0 && (
                                    <p className="mt-2 line-clamp-1 text-xs text-muted">{p.tags.join(' · ')}</p>
                                )}
                            </Card>
                        ))}
                        {problems.length === 0 && (
                            <Card>
                                <EmptyState
                                    text="No problems found"
                                    description="Try a different search or filter — problem sets sync in from judges."
                                />
                            </Card>
                        )}
                    </div>

                    {/* Pagination */}
                    <nav aria-label="Pagination" className="mt-4 flex items-center justify-between text-xs text-muted">
                        <span className="tnum">
                            {total === 0
                                ? 'No results'
                                : `${offset + 1}–${Math.min(offset + limit, total)} of ${total.toLocaleString()}`}
                        </span>
                        <div className="flex gap-2">
                            <Button
                                variant="secondary"
                                size="sm"
                                onClick={() => setOffset(Math.max(0, offset - limit))}
                                disabled={offset === 0}
                            >
                                Previous
                            </Button>
                            <Button
                                variant="secondary"
                                size="sm"
                                onClick={() => setOffset(offset + limit)}
                                disabled={offset + limit >= total}
                            >
                                Next
                            </Button>
                        </div>
                    </nav>
                </div>
            </div>
        </div>
    )
}

function FilterLabel({ children }: { children: React.ReactNode }) {
    return <h3 className="mb-1.5 text-[11px] font-semibold uppercase tracking-wider text-muted">{children}</h3>
}

function FilterOption({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
    return (
        <button
            type="button"
            onClick={onClick}
            aria-pressed={active}
            className={`w-full rounded-md px-2 py-1.5 text-left text-sm transition-colors ${active
                ? 'bg-brand-soft font-medium text-brand'
                : 'text-ink-soft hover:bg-surface-2 hover:text-ink'
                }`}
        >
            {children}
        </button>
    )
}

function Chip({ label, onClear }: { label: string; onClear: () => void }) {
    return (
        <span className="inline-flex items-center gap-1 rounded-md bg-brand-soft py-0.5 pl-2 pr-1 text-xs text-brand">
            {label}
            <button
                type="button"
                onClick={onClear}
                aria-label={`Remove ${label} filter`}
                className="rounded p-0.5 transition-colors hover:bg-brand/15"
            >
                <X className="h-3 w-3" />
            </button>
        </span>
    )
}
