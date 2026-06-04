import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'

const CATEGORIES = [
    { value: '', label: 'All Categories' },
    { value: 'general', label: 'General' },
    { value: 'icpc', label: 'ICPC' },
    { value: 'ioi', label: 'IOI' },
    { value: 'educational', label: 'Educational' },
    { value: 'regional', label: 'Regional' },
    { value: 'national', label: 'National' },
    { value: 'open', label: 'Open' },
]

export default function GymList() {
    const [gyms, setGyms] = useState<any[]>([])
    const [total, setTotal] = useState(0)
    const [category, setCategory] = useState('')
    const [search, setSearch] = useState('')

    useEffect(() => {
        api.gym.list(0, 20, { category: category || undefined, search: search || undefined }).then(d => {
            setGyms(d.data || [])
            setTotal(d.total || 0)
        }).catch(console.error)
    }, [category, search])

    const getDifficultyColor = (rating?: number) => {
        if (!rating) return 'text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-800'
        if (rating < 1200) return 'text-green-600 dark:text-green-400 bg-green-50 dark:bg-green-900/20'
        if (rating < 1600) return 'text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-900/20'
        if (rating < 2000) return 'text-purple-600 dark:text-purple-400 bg-purple-50 dark:bg-purple-900/20'
        if (rating < 2400) return 'text-orange-600 dark:text-orange-400 bg-orange-50 dark:bg-orange-900/20'
        return 'text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20'
    }

    return (
        <div>
            <div className="flex justify-between items-center mb-6">
                <div>
                    <h1 className="text-2xl font-bold">Gym</h1>
                    <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">Practice on community-curated contests</p>
                </div>
            </div>

            <div className="flex gap-4 mb-6">
                <select value={category} onChange={e => setCategory(e.target.value)} className="border rounded px-3 py-1.5 text-sm">
                    {CATEGORIES.map(c => <option key={c.value} value={c.value}>{c.label}</option>)}
                </select>
                <input type="text" value={search} onChange={e => setSearch(e.target.value)}
                    placeholder="Search gym..." className="border rounded px-3 py-1.5 text-sm flex-1" />
            </div>

            <div className="space-y-3">
                {gyms.map(g => (
                    <Link key={g.id} to={`/gym/${g.id}`} className="block border rounded-lg p-4 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors">
                        <div className="flex justify-between items-start">
                            <div>
                                <h3 className="font-semibold text-lg">{g.contest_title}</h3>
                                <p className="text-sm text-gray-600 dark:text-gray-400 mt-1">{g.description || 'No description provided.'}</p>
                                <div className="flex gap-4 mt-3 text-xs text-gray-400 dark:text-gray-500">
                                    <span className="capitalize font-medium text-gray-500 dark:text-gray-400">{g.category}</span>
                                    {g.country && <span>{g.country}</span>}
                                    {g.season && <span>{g.season}</span>}
                                    <span>{g.solve_count} solves</span>
                                </div>
                            </div>
                            {g.difficulty_rating && (
                                <span className={`text-xs px-2.5 py-1 rounded font-bold font-mono ${getDifficultyColor(g.difficulty_rating)}`}>
                                    {g.difficulty_rating}
                                </span>
                            )}
                        </div>
                    </Link>
                ))}
                {gyms.length === 0 && (
                    <div className="text-center py-16 text-gray-400 dark:text-gray-500">No gym contests found.</div>
                )}
            </div>
            {total > 0 && <p className="text-sm text-gray-400 dark:text-gray-500 mt-4">{total} gym contests total</p>}
        </div>
    )
}
