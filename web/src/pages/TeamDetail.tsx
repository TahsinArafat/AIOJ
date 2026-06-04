import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'
import RatingBadge from '../components/RatingBadge'

export default function TeamDetail() {
    const { id } = useParams<{ id: string }>()
    const [team, setTeam] = useState<any>(null)
    const [members, setMembers] = useState<any[]>([])
    const [isMember, setIsMember] = useState(false)
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        if (!id) return
        Promise.all([
            api.teams.get(id),
            api.teams.members(id)
        ]).then(([t, m]) => {
            setTeam(t)
            setMembers(m.data || [])
            setIsMember(m.data?.some((member: any) => member.user_id === 'current') || false) // simple check
        }).catch(console.error).finally(() => setLoading(false))
    }, [id])

    const handleJoin = async () => {
        if (!id) return
        try {
            await api.teams.join(id)
            setIsMember(true)
            window.location.reload()
        } catch (e: any) { alert('Failed: ' + e.message) }
    }

    const handleLeave = async () => {
        if (!id) return
        try {
            await api.teams.leave(id)
            setIsMember(false)
            window.location.reload()
        } catch (e: any) { alert('Failed: ' + e.message) }
    }

    if (loading) return <div className="text-center py-20 text-gray-400 dark:text-gray-500">Loading team details...</div>
    if (!team) return <div className="text-center py-20 text-gray-400 dark:text-gray-500">Team not found</div>

    return (
        <div className="space-y-6">
            <div className="flex justify-between items-start">
                <div>
                    <h1 className="text-2xl font-bold">{team.name}</h1>
                    <p className="text-gray-600 dark:text-gray-400 mt-2">{team.description || 'No description provided.'}</p>
                </div>
                <div className="flex items-center gap-4">
                    <RatingBadge rating={team.rating ?? 1500} showTitle />
                    {getAccessToken() && (
                        <button onClick={isMember ? handleLeave : handleJoin}
                            className={`px-4 py-2 rounded text-sm font-medium transition-colors ${
                                isMember ? 'bg-red-50 dark:bg-red-900/20 text-red-600 dark:text-red-400 hover:bg-red-100' : 'bg-blue-600 text-white hover:bg-blue-700'
                            }`}>
                            {isMember ? 'Leave Team' : 'Join Team'}
                        </button>
                    )}
                </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
                <div className="bg-gray-50 dark:bg-gray-800 p-4 rounded-lg">
                    <p className="text-sm text-gray-500 dark:text-gray-400">Max Rating</p>
                    <p className="text-xl font-bold mt-1">{team.max_rating ?? 1500}</p>
                </div>
                <div className="bg-gray-50 dark:bg-gray-800 p-4 rounded-lg">
                    <p className="text-sm text-gray-500 dark:text-gray-400">Contest Count</p>
                    <p className="text-xl font-bold mt-1">{team.contest_count ?? 0}</p>
                </div>
            </div>

            <div>
                <h2 className="text-lg font-semibold mb-3">Members ({members.length})</h2>
                <div className="border rounded-lg divide-y bg-white dark:bg-gray-800">
                    {members.map((m, i) => (
                        <div key={i} className="px-4 py-3 flex justify-between items-center text-sm">
                            <span className="font-medium text-gray-800 dark:text-gray-200">{m.username || m.user_id}</span>
                            <span className="text-xs text-gray-400 dark:text-gray-500 uppercase font-medium">{m.role}</span>
                        </div>
                    ))}
                </div>
            </div>
        </div>
    )
}
