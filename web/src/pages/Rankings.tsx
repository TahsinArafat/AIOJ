import { useEffect, useState, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'
import RatingBadge from '../components/RatingBadge'
import CountryFlag from '../components/CountryFlag'
import { EmptyState } from '../components/EmptyState'
import { Button } from '../components/ui/Button'
import { Input, Select } from '../components/ui/Field'
import { PageHeader, Skeleton } from '../components/ui/PageHeader'
import { Table, TableShell, Td, Th, THead, Tr } from '../components/ui/Table'

const MEDALS: Record<number, string> = {
    1: '\u{1F947}',
    2: '\u{1F948}',
    3: '\u{1F949}',
}

const COUNTRIES = [
    'All Countries',
    'Afghanistan', 'Albania', 'Algeria', 'Angola', 'Argentina', 'Armenia', 'Australia', 'Austria', 'Azerbaijan',
    'Bahrain', 'Bangladesh', 'Belarus', 'Belgium', 'Bolivia', 'Bosnia and Herzegovina', 'Brazil', 'Brunei', 'Bulgaria',
    'Cambodia', 'Cameroon', 'Canada', 'Chile', 'China', 'Colombia', 'Costa Rica', 'Croatia', 'Cuba', 'Cyprus', 'Czech Republic',
    'Denmark', 'Dominican Republic',
    'Ecuador', 'Egypt', 'El Salvador', 'Estonia', 'Ethiopia',
    'Finland', 'France',
    'Georgia', 'Germany', 'Ghana', 'Greece', 'Guatemala',
    'Honduras', 'Hong Kong', 'Hungary',
    'Iceland', 'India', 'Indonesia', 'Iran', 'Iraq', 'Ireland', 'Israel', 'Italy',
    'Jamaica', 'Japan', 'Jordan',
    'Kazakhstan', 'Kenya', 'Kuwait', 'Kyrgyzstan',
    'Laos', 'Latvia', 'Lebanon', 'Libya', 'Lithuania', 'Luxembourg',
    'Macau', 'Madagascar', 'Malaysia', 'Maldives', 'Mali', 'Malta', 'Mexico', 'Moldova', 'Mongolia', 'Montenegro', 'Morocco', 'Myanmar',
    'Nepal', 'Netherlands', 'New Zealand', 'Nicaragua', 'Nigeria', 'North Korea', 'North Macedonia', 'Norway',
    'Oman',
    'Pakistan', 'Palestine', 'Panama', 'Paraguay', 'Peru', 'Philippines', 'Poland', 'Portugal', 'Puerto Rico',
    'Qatar',
    'Romania', 'Russia', 'Rwanda',
    'Saudi Arabia', 'Senegal', 'Serbia', 'Singapore', 'Slovakia', 'Slovenia', 'South Africa', 'South Korea', 'Spain', 'Sri Lanka', 'Sudan', 'Sweden', 'Switzerland', 'Syria',
    'Taiwan', 'Tajikistan', 'Tanzania', 'Thailand', 'Trinidad and Tobago', 'Tunisia', 'Turkey', 'Turkmenistan',
    'Uganda', 'Ukraine', 'United Arab Emirates', 'United Kingdom', 'United States', 'Uruguay', 'Uzbekistan',
    'Venezuela', 'Vietnam',
    'Yemen',
    'Zambia', 'Zimbabwe',
]

interface RankingUser { username: string; country?: string; rating: number; rating_change: number; contests_played?: number }

export default function Rankings() {
    const [users, setUsers] = useState<RankingUser[]>([])
    const [total, setTotal] = useState(0)
    const [offset, setOffset] = useState(0)
    const [loading, setLoading] = useState(false)
    const [country, setCountry] = useState('')
    const [organization, setOrganization] = useState('')
    const limit = 50

    const fetchRankings = useCallback((off: number, append = false, co?: string, org?: string) => {
        setLoading(true)
        api.rankings.list(off, limit, co || undefined, org || undefined).then(d => {
            setUsers(prev => append ? [...prev, ...(d.data || [])] : (d.data || []))
            setTotal(d.total || 0)
        }).catch(() => { }).finally(() => setLoading(false))
    }, [])

    useEffect(() => {
        // deferred a microtask: loader's sync setLoading violates react-hooks/set-state-in-effect
        queueMicrotask(() => fetchRankings(0, false, country, organization))
    }, [country, organization, fetchRankings])

    const handleCountryChange = (val: string) => {
        setCountry(val === 'All Countries' ? '' : val)
        setOffset(0)
    }

    const handleOrgChange = (val: string) => {
        setOrganization(val)
        setOffset(0)
    }

    const handleLoadMore = () => {
        const nextOffset = offset + limit
        setOffset(nextOffset)
        fetchRankings(nextOffset, true, country, organization)
    }

    const hasMore = users.length < total
    const filtered = Boolean(country || organization)
    const initialLoading = loading && users.length === 0

    return (
        <div>
            <PageHeader
                title="Rankings"
                description={total > 0 ? `${total.toLocaleString()} rated users` : 'Standings by rating'}
            />

            {/* Filter Bar */}
            <div className="mb-4 flex flex-wrap items-center gap-2">
                <Select
                    value={country || 'All Countries'}
                    onChange={e => handleCountryChange(e.target.value)}
                    aria-label="Filter by country"
                    className="w-auto min-w-[11rem]"
                >
                    {COUNTRIES.map(c => (
                        <option key={c} value={c}>{c}</option>
                    ))}
                </Select>
                <Input
                    type="search"
                    placeholder="Filter by organization…"
                    value={organization}
                    onChange={e => handleOrgChange(e.target.value)}
                    aria-label="Filter by organization"
                    className="w-auto min-w-[12rem]"
                />
                {filtered && (
                    <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => { setCountry(''); setOrganization(''); setOffset(0); }}
                    >
                        Clear filters
                    </Button>
                )}
            </div>

            {initialLoading ? (
                <div className="space-y-2">
                    {Array.from({ length: 8 }).map((_, i) => <Skeleton key={i} className="h-10 w-full" />)}
                </div>
            ) : users.length === 0 ? (
                <div className="rounded-xl border border-hairline bg-surface">
                    <EmptyState
                        text="No rankings yet"
                        description="Standings fill in as users solve problems and compete in contests."
                    />
                </div>
            ) : (
                <TableShell>
                    <Table>
                        <THead>
                            <tr>
                                <Th className="w-20">Rank</Th>
                                <Th>User</Th>
                                <Th className="text-right">Rating</Th>
                                <Th className="text-right">Change</Th>
                                <Th className="hidden text-right sm:table-cell">Contests</Th>
                            </tr>
                        </THead>
                        <tbody>
                            {users.map((u, i) => {
                                const rank = offset + i + 1
                                const medal = MEDALS[rank]
                                return (
                                    <Tr key={u.username}>
                                        <Td>
                                            {medal ? (
                                                <span className="text-base" aria-label={`Rank ${rank}`}>{medal}</span>
                                            ) : (
                                                <span className="tnum text-xs text-muted">{rank}</span>
                                            )}
                                        </Td>
                                        <Td>
                                            <div className="flex items-center gap-2">
                                                <Link
                                                    to={`/user/${u.username}`}
                                                    className="font-medium text-ink transition-colors hover:text-brand"
                                                >
                                                    {u.username}
                                                </Link>
                                                {u.country ? <CountryFlag country={u.country} /> : null}
                                            </div>
                                        </Td>
                                        <Td className="text-right">
                                            <RatingBadge rating={u.rating} size="sm" />
                                        </Td>
                                        <Td className="text-right">
                                            <span className={`tnum text-xs font-medium ${u.rating_change >= 0 ? 'text-ok' : 'text-bad'}`}>
                                                {u.rating_change >= 0 ? '+' : ''}{u.rating_change}
                                            </span>
                                        </Td>
                                        <Td className="tnum hidden text-right text-xs text-muted sm:table-cell">
                                            {u.contests_played ?? '—'}
                                        </Td>
                                    </Tr>
                                )
                            })}
                        </tbody>
                    </Table>
                </TableShell>
            )}

            {hasMore && !loading && (
                <div className="mt-4 flex justify-center">
                    <Button variant="secondary" onClick={handleLoadMore}>Load more</Button>
                </div>
            )}

            {loading && users.length > 0 && (
                <p className="mt-3 text-center text-xs text-muted">Loading…</p>
            )}

            {total > 0 && (
                <p className="tnum mt-3 text-center text-xs text-muted-soft">{users.length} of {total} users</p>
            )}
        </div>
    )
}
