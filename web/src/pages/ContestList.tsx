import { useEffect, useState, useMemo, useCallback, useRef } from 'react'
import { Link } from 'react-router-dom'
import { Search, Trophy } from 'lucide-react'
import { api, contestSlug } from '../lib/api'
import { formatCountdown, formatDuration } from '../lib/contestTime'
import DivisionBadge from '../components/DivisionBadge'
import { DIVISIONS, type Division } from '../lib/divisions'
import { Badge, type BadgeTone } from '../components/ui/Badge'
import { Button } from '../components/ui/Button'
import { buttonClass } from '../components/ui/buttonStyles'
import { Input } from '../components/ui/Field'
import { PageHeader, Skeleton } from '../components/ui/PageHeader'
import { Table, TableShell, Td, Th, THead, Tr } from '../components/ui/Table'

/* ───────────────────── helpers ───────────────────── */

const FORMAT_LABEL: Record<string, string> = {
    acm: 'ACM',
    oi: 'OI',
    ioi: 'IOI',
    cf: 'CF',
    atcoder: 'ATC',
}

const FORMAT_TONE: Record<string, BadgeTone> = {
    acm: 'brand',
    oi: 'neutral',
    ioi: 'ok',
    cf: 'warn',
    atcoder: 'brand',
}

function formatDate(iso: string): string {
    const d = new Date(iso)
    const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
    const mm = months[d.getMonth()]
    const dd = String(d.getDate()).padStart(2, '0')
    const yyyy = d.getFullYear()
    const hh = String(d.getHours()).padStart(2, '0')
    const mi = String(d.getMinutes()).padStart(2, '0')
    return `${mm} ${dd}, ${yyyy} ${hh}:${mi}`
}

// formatDuration/formatCountdown are the shared ones (see ContestClock): this
// file used to carry copies that printed "00d 00:00:00" where the cards printed
// "00:00:00" for the same instant.
function listDuration(startIso: string, endIso: string): string {
    return formatDuration(new Date(endIso).getTime() - new Date(startIso).getTime())
}

function formatElapsed(startIso: string): string {
    return formatCountdown(Date.now() - new Date(startIso).getTime())
}

type ContestStatus = 'upcoming' | 'running' | 'ended'

interface ContestSummary {
    id: string
    title: string
    division?: number
    type: string
    start_time: string
    end_time: string
}

function getContestStatus(c: { start_time: string; end_time: string }): ContestStatus {
    const now = Date.now()
    const start = new Date(c.start_time).getTime()
    const end = new Date(c.end_time).getTime()
    if (now < start) return 'upcoming'
    if (now < end) return 'running'
    return 'ended'
}

/* ───────────────────── sub-components ───────────────────── */

function FormatBadge({ type }: { type: string }) {
    const key = type?.toLowerCase()
    return (
        <Badge tone={FORMAT_TONE[key] || 'neutral'}>
            {FORMAT_LABEL[key] || type?.toUpperCase() || 'STD'}
        </Badge>
    )
}

function StatusBadge({ status }: { status: ContestStatus }) {
    const map: Record<ContestStatus, { text: string; tone: BadgeTone }> = {
        upcoming: { text: 'Upcoming', tone: 'brand' },
        running: { text: 'Running', tone: 'ok' },
        ended: { text: 'Ended', tone: 'neutral' },
    }
    const s = map[status]
    return <Badge tone={s.tone} dot={status === 'running'}>{s.text}</Badge>
}

function CountdownTimer({ targetIso }: { targetIso: string }) {
    const [remaining, setRemaining] = useState(() => new Date(targetIso).getTime() - Date.now())

    useEffect(() => {
        const id = setInterval(() => setRemaining(new Date(targetIso).getTime() - Date.now()), 1000)
        return () => clearInterval(id)
    }, [targetIso])

    if (remaining <= 0) return <span className="text-xs font-medium text-ok">Starting now</span>

    return <span className="tnum text-xs font-medium text-brand">{formatCountdown(remaining)}</span>
}

function ElapsedTimer({ startIso }: { startIso: string }) {
    const [, setTick] = useState(0)

    useEffect(() => {
        const id = setInterval(() => setTick(t => t + 1), 1000)
        return () => clearInterval(id)
    }, [startIso])

    return <span className="tnum text-xs font-medium text-ok">{formatElapsed(startIso)}</span>
}

function SectionHeading({ label, count }: { label: string; count?: number }) {
    return (
        <div className="mb-2.5 flex items-center gap-2">
            <h2 className="text-sm font-semibold tracking-tight text-ink">{label}</h2>
            {count !== undefined && count > 0 && <Badge tone="neutral">{count}</Badge>}
        </div>
    )
}

/* ───────────────────── main component ───────────────────── */

const PAGE_SIZE = 30

export default function ContestList() {
    const [contests, setContests] = useState<ContestSummary[]>([])
    const [total, setTotal] = useState(0)
    const [division, setDivision] = useState<Division | undefined>(undefined)
    const [search, setSearch] = useState('')
    const [searchInput, setSearchInput] = useState('')
    const [offset, setOffset] = useState(0)
    const [loading, setLoading] = useState(false)
    const [hasMore, setHasMore] = useState(false)
    const searchTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

    // debounce search
    const onSearchChange = useCallback((v: string) => {
        setSearchInput(v)
        if (searchTimer.current) clearTimeout(searchTimer.current)
        searchTimer.current = setTimeout(() => setSearch(v), 300)
    }, [])

    const loadContests = useCallback(async (off: number, replace: boolean) => {
        setLoading(true)
        try {
            const d = await api.contests.list(off, PAGE_SIZE, division)
            const items = d.data || []
            setContests(prev => replace ? items : [...prev, ...items])
            setTotal(d.total || 0)
            setHasMore(off + items.length < (d.total || 0))
            setOffset(off + items.length)
        } catch {
            // silent
        } finally {
            setLoading(false)
        }
    }, [division])

    // fetch on division / search change (reset)
    useEffect(() => {
        queueMicrotask(() => {
            setOffset(0)
            loadContests(0, true)
        })
    }, [division, search, loadContests])

    const loadMore = useCallback(() => {
        if (!loading && hasMore) loadContests(offset, false)
    }, [loading, hasMore, offset, loadContests])

    // categorize
    const { upcoming, past } = useMemo(() => {
        const filtered = contests.filter(c => {
            if (!search) return true
            return c.title?.toLowerCase().includes(search.toLowerCase())
        })
        const u: ContestSummary[] = []
        const p: ContestSummary[] = []
        for (const c of filtered) {
            const s = getContestStatus(c)
            if (s === 'ended') p.push(c)
            else u.push(c) // upcoming + running
        }
        // sort upcoming by start_time ascending (soonest first)
        u.sort((a, b) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime())
        // sort past by start_time descending (most recent first)
        p.sort((a, b) => new Date(b.start_time).getTime() - new Date(a.start_time).getTime())
        return { upcoming: u, past: p }
    }, [contests, search])

    const initialLoading = loading && contests.length === 0

    return (
        <div>
            <PageHeader
                title="Contests"
                description={`${total} contest${total !== 1 ? 's' : ''} available`}
                actions={
                    <Link to="/gym" className={buttonClass({ variant: 'secondary', size: 'sm' })}>
                        <Trophy className="h-4 w-4" />
                        Go to Gym
                    </Link>
                }
            />

            {/* ── Filters bar ── */}
            <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center">
                <div role="group" aria-label="Filter by division" className="flex flex-wrap gap-1">
                    <DivisionButton active={division === undefined} onClick={() => setDivision(undefined)}>
                        All
                    </DivisionButton>
                    {Object.entries(DIVISIONS).map(([key, info]) => {
                        const div = Number(key) as Division
                        return (
                            <DivisionButton key={key} active={division === div} onClick={() => setDivision(div)}>
                                {info.name}
                            </DivisionButton>
                        )
                    })}
                </div>

                <div className="relative sm:ml-auto">
                    <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-soft" />
                    <Input
                        type="search"
                        placeholder="Search contests..."
                        value={searchInput}
                        onChange={e => onSearchChange(e.target.value)}
                        aria-label="Search contests"
                        className="w-full pl-8 sm:w-64"
                    />
                </div>
            </div>

            {/* ── Current / Upcoming Contests ── */}
            <section className="mb-10">
                <SectionHeading label="Current or Upcoming Contests" count={upcoming.length} />

                {initialLoading ? (
                    <div className="space-y-2">
                        <Skeleton className="h-10 w-full" />
                        <Skeleton className="h-10 w-full" />
                    </div>
                ) : upcoming.length === 0 ? (
                    <div className="rounded-xl border border-dashed border-hairline-strong px-4 py-12 text-center">
                        <Trophy className="mx-auto mb-2 h-8 w-8 text-muted-soft" />
                        <p className="text-sm text-muted">No upcoming contests at the moment.</p>
                    </div>
                ) : (
                    <TableShell>
                        <Table>
                            <THead>
                                <tr>
                                    <Th className="w-10">#</Th>
                                    <Th>Contest</Th>
                                    <Th className="hidden sm:table-cell">Start</Th>
                                    <Th className="hidden md:table-cell">Length</Th>
                                    <Th>Countdown / Elapsed</Th>
                                    <Th className="w-28">Status</Th>
                                </tr>
                            </THead>
                            <tbody>
                                {upcoming.map((c, i) => {
                                    const status = getContestStatus(c)
                                    return (
                                        <Tr key={c.id}>
                                            <Td className="tnum text-xs text-muted-soft">{i + 1}</Td>
                                            <Td>
                                                <div className="flex flex-wrap items-center gap-1.5">
                                                    <Link
                                                        to={`/contests/${contestSlug(c)}`}
                                                        className="font-medium text-ink transition-colors hover:text-brand"
                                                    >
                                                        {c.title}
                                                    </Link>
                                                    <DivisionBadge division={(c.division ?? 0) as Division} size="sm" />
                                                    <FormatBadge type={c.type} />
                                                </div>
                                            </Td>
                                            <Td className="hidden whitespace-nowrap text-xs text-muted sm:table-cell">
                                                {formatDate(c.start_time)}
                                            </Td>
                                            <Td className="tnum hidden whitespace-nowrap text-xs text-muted md:table-cell">
                                                {listDuration(c.start_time, c.end_time)}
                                            </Td>
                                            <Td className="whitespace-nowrap">
                                                {status === 'upcoming' && <CountdownTimer targetIso={c.start_time} />}
                                                {status === 'running' && <ElapsedTimer startIso={c.start_time} />}
                                                {status === 'ended' && <span className="text-xs text-muted-soft">—</span>}
                                            </Td>
                                            <Td>
                                                <StatusBadge status={status} />
                                            </Td>
                                        </Tr>
                                    )
                                })}
                            </tbody>
                        </Table>
                    </TableShell>
                )}
            </section>

            {/* ── Past Contests ── */}
            <section>
                <SectionHeading label="Past Contests" count={past.length} />

                {initialLoading ? (
                    <Skeleton className="h-10 w-full" />
                ) : past.length === 0 ? (
                    <div className="rounded-xl border border-dashed border-hairline-strong px-4 py-12 text-center">
                        <p className="text-sm text-muted">No past contests found.</p>
                    </div>
                ) : (
                    <TableShell>
                        <Table>
                            <THead>
                                <tr>
                                    <Th className="w-10">#</Th>
                                    <Th>Contest</Th>
                                    <Th className="hidden sm:table-cell">Start</Th>
                                    <Th className="hidden md:table-cell">Length</Th>
                                    <Th className="w-48">Status / Standings</Th>
                                </tr>
                            </THead>
                            <tbody>
                                {past.map((c, i) => (
                                    <Tr key={c.id}>
                                        <Td className="tnum text-xs text-muted-soft">{i + 1}</Td>
                                        <Td>
                                            <div className="flex flex-wrap items-center gap-1.5">
                                                <Link
                                                    to={`/contests/${contestSlug(c)}`}
                                                    className="font-medium text-ink transition-colors hover:text-brand"
                                                >
                                                    {c.title}
                                                </Link>
                                                <DivisionBadge division={(c.division ?? 0) as Division} size="sm" />
                                                <FormatBadge type={c.type} />
                                            </div>
                                        </Td>
                                        <Td className="hidden whitespace-nowrap text-xs text-muted sm:table-cell">
                                            {formatDate(c.start_time)}
                                        </Td>
                                        <Td className="tnum hidden whitespace-nowrap text-xs text-muted md:table-cell">
                                            {listDuration(c.start_time, c.end_time)}
                                        </Td>
                                        <Td>
                                            <div className="flex items-center gap-3">
                                                <StatusBadge status="ended" />
                                                <Link
                                                    to={`/contests/${contestSlug(c)}/scoreboard`}
                                                    className="text-xs font-medium text-brand hover:underline"
                                                >
                                                    Standings
                                                </Link>
                                            </div>
                                        </Td>
                                    </Tr>
                                ))}
                            </tbody>
                        </Table>
                    </TableShell>
                )}
            </section>

            {/* ── Load more ── */}
            {hasMore && (
                <div className="mt-6 flex justify-center">
                    <Button variant="secondary" onClick={loadMore} disabled={loading}>
                        {loading ? 'Loading…' : `Load more (${offset} of ${total})`}
                    </Button>
                </div>
            )}
        </div>
    )
}

function DivisionButton({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
    return (
        <button
            type="button"
            onClick={onClick}
            aria-pressed={active}
            className={`h-8 rounded-lg px-3 text-xs font-medium transition-colors ${active
                ? 'bg-ink text-canvas'
                : 'bg-surface-2 text-muted hover:bg-surface-3 hover:text-ink'
                }`}
        >
            {children}
        </button>
    )
}
