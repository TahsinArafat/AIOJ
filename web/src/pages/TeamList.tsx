import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'
import RatingBadge from '../components/RatingBadge'
import { EmptyState } from '../components/EmptyState'

interface TeamListRow { id: string; name?: string; member_count?: number; rating?: number }

export default function TeamList() {
    const [teams, setTeams] = useState<TeamListRow[]>([])
    const [total, setTotal] = useState(0)

    useEffect(() => {
        api.teams.list().then(d => {
            setTeams(d.data || [])
            setTotal(d.total || 0)
        }).catch(console.error)
    }, [])

    return (
        <div>
            <div className="flex justify-between items-center mb-6">
                <div>
                    <h1 className="text-2xl font-bold">Teams</h1>
                    <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">Form teams to compete in ICPC-style contests.</p>
                </div>
                {getAccessToken() && (
                    <Link to="/teams/create" className="bg-blue-600 text-white px-4 py-2 rounded text-sm hover:bg-blue-700 font-medium">
                        Create Team
                    </Link>
                )}
            </div>

            <div className="space-y-3">
                {teams.map(t => (
                    <Link key={t.id} to={`/teams/${t.id}`} className="block border rounded-lg p-4 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors">
                        <div className="flex justify-between items-center">
                            <div>
                                <h3 className="font-semibold text-lg">{t.name}</h3>
                                <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">{t.member_count} members</p>
                            </div>
                            <RatingBadge rating={t.rating ?? 1500} size="sm" />
                        </div>
                    </Link>
                ))}
                {teams.length === 0 && (
                    <EmptyState text="No teams found" description="Teams you create or join will show up here." />
                )}
            </div>
            {total > 0 && <p className="text-sm text-gray-400 dark:text-gray-500 mt-4">{total} teams total</p>}
        </div>
    )
}
