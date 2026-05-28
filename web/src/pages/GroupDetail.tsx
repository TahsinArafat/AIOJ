import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'

export default function GroupDetail() {
    const { id } = useParams<{ id: string }>()
    const [group, setGroup] = useState<any>(null)
    const [members, setMembers] = useState<any[]>([])
    const [isMember, setIsMember] = useState(false)
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        if (!id) return
        Promise.all([
            api.groups.get(id),
            api.groups.members(id)
        ]).then(([g, m]) => {
            setGroup(g)
            setMembers(m.data || [])
            setIsMember(m.data?.some((member: any) => member.user_id === 'current') || false) // simple check
        }).catch(console.error).finally(() => setLoading(false))
    }, [id])

    const handleJoin = async () => {
        if (!id) return
        try {
            await api.groups.join(id)
            setIsMember(true)
            window.location.reload()
        } catch (e: any) { alert('Failed: ' + e.message) }
    }

    const handleLeave = async () => {
        if (!id) return
        try {
            await api.groups.leave(id)
            setIsMember(false)
            window.location.reload()
        } catch (e: any) { alert('Failed: ' + e.message) }
    }

    if (loading) return <div className="text-center py-20 text-gray-400">Loading group details...</div>
    if (!group) return <div className="text-center py-20 text-gray-400">Group not found</div>

    return (
        <div className="space-y-6">
            <div className="flex justify-between items-start">
                <div>
                    <h1 className="text-2xl font-bold">{group.name}</h1>
                    <p className="text-gray-600 mt-2">{group.description || 'No description provided.'}</p>
                </div>
                {getAccessToken() && (
                    <button onClick={isMember ? handleLeave : handleJoin}
                        className={`px-4 py-2 rounded text-sm font-medium transition-colors ${
                            isMember ? 'bg-red-50 text-red-600 hover:bg-red-100' : 'bg-blue-600 text-white hover:bg-blue-700'
                        }`}>
                        {isMember ? 'Leave Group' : 'Join Group'}
                    </button>
                )}
            </div>

            <div>
                <h2 className="text-lg font-semibold mb-3">Members ({members.length})</h2>
                <div className="border rounded-lg divide-y bg-white">
                    {members.map((m, i) => (
                        <div key={i} className="px-4 py-3 flex justify-between items-center text-sm">
                            <span className="font-medium text-gray-800">{m.username || m.user_id}</span>
                            <span className="text-xs text-gray-400 uppercase font-medium">{m.role}</span>
                        </div>
                    ))}
                </div>
            </div>
        </div>
    )
}
