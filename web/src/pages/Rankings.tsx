import { useEffect, useState, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'
import RatingBadge from '../components/RatingBadge'

const RANK_STYLES: Record<number, string> = {
    1: 'text-amber-700 bg-amber-50 font-bold',
    2: 'text-gray-600 dark:text-gray-400 bg-gray-100 dark:bg-gray-700 font-bold',
    3: 'text-orange-700 dark:text-orange-300 bg-orange-50 dark:bg-orange-900/20 font-bold',
}

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

export default function Rankings() {
    const [users, setUsers] = useState<any[]>([])
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
        }).catch(() => {}).finally(() => setLoading(false))
    }, [])

    useEffect(() => {
        fetchRankings(0, false, country, organization)
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

    return (
        <div>
            <h1 className="text-2xl font-bold mb-4">Rankings</h1>

            {/* Filter Bar */}
            <div className="flex flex-wrap gap-3 mb-4">
                <select
                    value={country || 'All Countries'}
                    onChange={e => handleCountryChange(e.target.value)}
                    className="border border-gray-200 dark:border-gray-600 rounded-lg px-3 py-2 text-sm bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
                >
                    {COUNTRIES.map(c => (
                        <option key={c} value={c}>{c}</option>
                    ))}
                </select>
                <input
                    type="text"
                    placeholder="Filter by organization..."
                    value={organization}
                    onChange={e => handleOrgChange(e.target.value)}
                    className="border border-gray-200 dark:border-gray-600 rounded-lg px-3 py-2 text-sm bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400 min-w-[180px]"
                />
                {(country || organization) && (
                    <button
                        onClick={() => { setCountry(''); setOrganization(''); setOffset(0); }}
                        className="px-3 py-2 text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 transition-colors"
                    >
                        Clear filters
                    </button>
                )}
            </div>

            <div className="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
                <div className="overflow-x-auto">
                    <table className="w-full text-sm">
                        <thead className="bg-gray-50 dark:bg-gray-800 text-gray-500 dark:text-gray-400 text-xs uppercase">
                            <tr>
                                <th className="px-4 py-3 text-left w-16">Rank</th>
                                <th className="px-4 py-3 text-left">Username</th>
                                <th className="px-4 py-3 text-right">Rating</th>
                                <th className="px-4 py-3 text-right">Change</th>
                                <th className="px-4 py-3 text-right">Contests</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-gray-100 dark:divide-gray-700">
                            {users.map((u, i) => {
                                const rank = offset + i + 1
                                const rankStyle = RANK_STYLES[rank] || ''
                                return (
                                    <tr key={u.username} className={`hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors ${rankStyle}`}>
                                        <td className="px-4 py-3 text-center">
                                            {MEDALS[rank] || rank}
                                        </td>
                                        <td className="px-4 py-3">
                                            <Link to={`/user/${u.username}`} className="font-medium text-blue-600 dark:text-blue-400 hover:underline">
                                                {u.username}
                                            </Link>
                                        </td>
                                        <td className="px-4 py-3 text-right">
                                            <RatingBadge rating={u.rating} size="sm" />
                                        </td>
                                        <td className="px-4 py-3 text-right">
                                            <span className={u.rating_change >= 0 ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}>
                                                {u.rating_change >= 0 ? '+' : ''}{u.rating_change}
                                            </span>
                                        </td>
                                        <td className="px-4 py-3 text-right text-gray-500 dark:text-gray-400">
                                            {u.contests_played}
                                        </td>
                                    </tr>
                                )
                            })}
                            {users.length === 0 && !loading && (
                                <tr><td colSpan={5} className="px-4 py-8 text-center text-gray-400 dark:text-gray-500">No rankings yet.</td></tr>
                            )}
                        </tbody>
                    </table>
                </div>
            </div>

            {loading && (
                <div className="text-center py-4 text-gray-400 dark:text-gray-500">Loading...</div>
            )}

            {hasMore && !loading && (
                <div className="flex justify-center mt-4">
                    <button
                        onClick={handleLoadMore}
                        className="px-6 py-2 border border-gray-300 dark:border-gray-600 rounded text-sm hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
                    >
                        Load More
                    </button>
                </div>
            )}

            {total > 0 && (
                <p className="text-sm text-gray-400 dark:text-gray-500 mt-3 text-center">{users.length} of {total} users</p>
            )}
        </div>
    )
}
