import { useEffect, useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'

function getUserId(): string | null {
    const token = localStorage.getItem('access_token')
    if (!token) return null
    try {
        const payload = JSON.parse(atob(token.split('.')[1]))
        return payload.uid ?? null
    } catch {
        return null
    }
}

function getRole(): string | null {
    const token = localStorage.getItem('access_token')
    if (!token) return null
    try {
        const payload = JSON.parse(atob(token.split('.')[1]))
        return payload.role ?? null
    } catch {
        return null
    }
}

export default function GroupDetail() {
    const { id } = useParams<{ id: string }>()
    const navigate = useNavigate()
    const [group, setGroup] = useState<any>(null)
    const [members, setMembers] = useState<any[]>([])
    const [isMember, setIsMember] = useState(false)
    const [loading, setLoading] = useState(true)

    const [activeTab, setActiveTab] = useState<'members' | 'contests'>('members')
    const [contests, setContests] = useState<any[]>([])
    const [newContestId, setNewContestId] = useState('')

    const [editing, setEditing] = useState(false)
    const [editName, setEditName] = useState('')
    const [editDescription, setEditDescription] = useState('')
    const [editIsPublic, setEditIsPublic] = useState(true)

    const fetchGroupData = () => {
        if (!id) return
        const currentUserId = getUserId()
        Promise.all([
            api.groups.get(id),
            api.groups.members(id),
            api.groups.getContests(id)
        ]).then(([g, m, c]) => {
            setGroup(g)
            setMembers(m.data || [])
            setContests(c.data || [])
            setIsMember(m.data?.some((member: any) => member.user_id === currentUserId) || false)
        }).catch(console.error).finally(() => setLoading(false))
    }

    useEffect(() => {
        fetchGroupData()
    }, [id])

    const handleJoin = async () => {
        if (!id) return
        try {
            await api.groups.join(id)
            setIsMember(true)
            fetchGroupData()
        } catch (e: any) { alert('Failed: ' + e.message) }
    }

    const handleLeave = async () => {
        if (!id) return
        try {
            await api.groups.leave(id)
            setIsMember(false)
            fetchGroupData()
        } catch (e: any) { alert('Failed: ' + e.message) }
    }

    const handleStartEdit = () => {
        setEditName(group.name)
        setEditDescription(group.description || '')
        setEditIsPublic(group.is_public)
        setEditing(true)
    }

    const handleSaveEdit = async () => {
        if (!id) return
        try {
            const updated = await api.groups.update(id, {
                name: editName,
                description: editDescription,
                is_public: editIsPublic
            })
            setGroup(updated)
            setEditing(false)
        } catch (e: any) { alert('Update failed: ' + e.message) }
    }

    const handleDelete = async () => {
        if (!id) return
        if (!confirm('Are you sure you want to delete this group?')) return
        try {
            await api.groups.delete(id)
            navigate('/groups')
        } catch (e: any) { alert('Delete failed: ' + e.message) }
    }

    const handleAddContest = async () => {
        if (!id || !newContestId.trim()) return
        try {
            await api.groups.addContest(id, newContestId.trim())
            setNewContestId('')
            const c = await api.groups.getContests(id)
            setContests(c.data || [])
        } catch (e: any) { alert('Failed to add contest: ' + e.message) }
    }

    const handleRemoveContest = async (contestId: string) => {
        if (!id) return
        if (!confirm('Remove this contest from the group?')) return
        try {
            await api.groups.removeContest(id, contestId)
            const c = await api.groups.getContests(id)
            setContests(c.data || [])
        } catch (e: any) { alert('Failed to remove contest: ' + e.message) }
    }

    if (loading) return <div className="text-center py-20 text-gray-400 dark:text-gray-500">Loading group details...</div>
    if (!group) return <div className="text-center py-20 text-gray-400 dark:text-gray-500">Group not found</div>

    const currentUserId = getUserId()
    const role = getRole()
    const canManage = group.created_by === currentUserId || role === 'admin'

    return (
        <div className="space-y-6">
            {editing ? (
                <div className="space-y-4 bg-white dark:bg-gray-900 p-6 rounded-lg border border-gray-200 dark:border-gray-800">
                    <h2 className="text-xl font-bold">Edit Group</h2>
                    <div>
                        <label className="block text-sm font-medium mb-1">Group Name</label>
                        <input
                            type="text"
                            value={editName}
                            onChange={e => setEditName(e.target.value)}
                            className="w-full border border-gray-300 dark:border-gray-700 bg-transparent rounded p-2 text-sm text-gray-800 dark:text-gray-200"
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium mb-1">Description</label>
                        <textarea
                            value={editDescription}
                            onChange={e => setEditDescription(e.target.value)}
                            rows={3}
                            className="w-full border border-gray-300 dark:border-gray-700 bg-transparent rounded p-2 text-sm text-gray-800 dark:text-gray-200"
                        />
                    </div>
                    <div className="flex items-center gap-2">
                        <input
                            type="checkbox"
                            id="editIsPublic"
                            checked={editIsPublic}
                            onChange={e => setEditIsPublic(e.target.checked)}
                            className="rounded"
                        />
                        <label htmlFor="editIsPublic" className="text-sm font-medium">Public Group</label>
                    </div>
                    <div className="flex gap-2 justify-end">
                        <button onClick={() => setEditing(false)} className="px-4 py-2 border border-gray-300 dark:border-gray-700 rounded text-sm hover:bg-gray-100 dark:hover:bg-gray-800 cursor-pointer">Cancel</button>
                        <button onClick={handleSaveEdit} className="px-4 py-2 bg-blue-600 text-white rounded text-sm hover:bg-blue-700 cursor-pointer">Save</button>
                    </div>
                </div>
            ) : (
                <>
                    <div className="flex justify-between items-start">
                        <div>
                            <div className="flex items-center gap-3">
                                <h1 className="text-2xl font-bold">{group.name}</h1>
                                <span className={`text-xs px-2 py-0.5 rounded font-semibold ${group.is_public ? 'bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-300' : 'bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300'}`}>
                                    {group.is_public ? 'Public' : 'Private'}
                                </span>
                            </div>
                            <p className="text-gray-600 dark:text-gray-400 mt-2">{group.description || 'No description provided.'}</p>
                        </div>
                        <div className="flex gap-2">
                            {canManage && (
                                <>
                                    <button onClick={handleStartEdit} className="px-4 py-2 border border-gray-300 dark:border-gray-700 rounded text-sm hover:bg-gray-100 dark:hover:bg-gray-800 cursor-pointer">Edit</button>
                                    <button onClick={handleDelete} className="px-4 py-2 bg-red-600 text-white rounded text-sm hover:bg-red-700 cursor-pointer">Delete</button>
                                </>
                            )}
                            {getAccessToken() && (
                                <button onClick={isMember ? handleLeave : handleJoin}
                                    className={`px-4 py-2 rounded text-sm font-medium transition-colors cursor-pointer ${
                                        isMember ? 'bg-red-50 dark:bg-red-900/20 text-red-600 dark:text-red-400 hover:bg-red-100' : 'bg-blue-600 text-white hover:bg-blue-700'
                                    }`}>
                                    {isMember ? 'Leave Group' : 'Join Group'}
                                </button>
                            )}
                        </div>
                    </div>

                    <div className="border-b border-gray-200 dark:border-gray-800 flex gap-4">
                        <button
                            onClick={() => setActiveTab('members')}
                            className={`pb-2 text-sm font-medium border-b-2 cursor-pointer ${activeTab === 'members' ? 'border-blue-600 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 hover:text-gray-700'}`}
                        >
                            Members ({members.length})
                        </button>
                        <button
                            onClick={() => setActiveTab('contests')}
                            className={`pb-2 text-sm font-medium border-b-2 cursor-pointer ${activeTab === 'contests' ? 'border-blue-600 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 hover:text-gray-700'}`}
                        >
                            Group Contests ({contests.length})
                        </button>
                    </div>

                    {activeTab === 'members' ? (
                        <div className="border rounded-lg divide-y bg-white dark:bg-gray-800 border-gray-200 dark:border-gray-700">
                            {members.map((m, i) => (
                                <div key={i} className="px-4 py-3 flex justify-between items-center text-sm">
                                    <span className="font-medium text-gray-800 dark:text-gray-200">{m.username || m.user_id}</span>
                                    <span className="text-xs text-gray-400 dark:text-gray-500 uppercase font-medium">{m.role}</span>
                                </div>
                            ))}
                        </div>
                    ) : (
                        <div className="space-y-4">
                            {canManage && (
                                <div className="flex gap-2 max-w-md">
                                    <input
                                        type="text"
                                        placeholder="Contest ID..."
                                        value={newContestId}
                                        onChange={e => setNewContestId(e.target.value)}
                                        className="flex-1 border border-gray-300 dark:border-gray-700 bg-transparent rounded p-2 text-sm text-gray-800 dark:text-gray-200"
                                    />
                                    <button onClick={handleAddContest} className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded text-sm font-medium cursor-pointer">
                                        Add Contest
                                    </button>
                                </div>
                            )}

                            <div className="border rounded-lg divide-y bg-white dark:bg-gray-800 border-gray-200 dark:border-gray-700">
                                {contests.map((c, i) => (
                                    <div key={i} className="px-4 py-3 flex justify-between items-center text-sm">
                                        <Link to={`/contests/${c.id}`} className="font-medium text-blue-600 dark:text-blue-400 hover:underline">
                                            {c.title}
                                        </Link>
                                        <div className="flex items-center gap-4 text-xs text-gray-400">
                                            <span>Type: {c.type}</span>
                                            {canManage && (
                                                <button onClick={() => handleRemoveContest(c.id)} className="text-red-600 hover:underline cursor-pointer">
                                                    Remove
                                                </button>
                                            )}
                                        </div>
                                    </div>
                                ))}
                                {contests.length === 0 && (
                                    <p className="p-4 text-sm text-gray-400 dark:text-gray-500 text-center">No contests in this group yet.</p>
                                )}
                            </div>
                        </div>
                    )}
                </>
            )}
        </div>
    )
}
