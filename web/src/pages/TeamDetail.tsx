import { useEffect, useState, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'
import RatingBadge from '../components/RatingBadge'

function getUserId(): string | null {
    const token = getAccessToken()
    if (!token) return null
    try {
        const payload = JSON.parse(atob(token.split('.')[1]))
        return payload.uid ?? null
    } catch { return null }
}

function getRole(): string | null {
    const token = getAccessToken()
    if (!token) return null
    try {
        const payload = JSON.parse(atob(token.split('.')[1]))
        return payload.role ?? null
    } catch { return null }
}

export default function TeamDetail() {
    const { id } = useParams<{ id: string }>()
    const navigate = useNavigate()
    const [team, setTeam] = useState<any>(null)
    const [members, setMembers] = useState<any[]>([])
    const [isMember, setIsMember] = useState(false)
    const [currentUserRole, setCurrentUserRole] = useState<string | null>(null)
    const [loading, setLoading] = useState(true)

    // Edit state
    const [editing, setEditing] = useState(false)
    const [editName, setEditName] = useState('')
    const [editDescription, setEditDescription] = useState('')
    const [editIsPublic, setEditIsPublic] = useState(true)

    // Invite state
    const [inviteUsername, setInviteUsername] = useState('')
    const [inviting, setInviting] = useState(false)

    // Pending members state
    const [pendingMembers, setPendingMembers] = useState<any[]>([])
    const [showPending, setShowPending] = useState(false)
    const [acting, setActing] = useState<string | null>(null)

    const fetchTeamData = useCallback(() => {
        if (!id) return
        const currentUserId = getUserId()
        setLoading(true)
        Promise.all([
            api.teams.get(id),
            api.teams.members(id),
        ]).then(([t, m]) => {
            setTeam(t)
            const memberList = m.data || []
            setMembers(memberList)
            const membership = memberList.find((member: any) => member.user_id === currentUserId)
            setIsMember(!!membership)
            setCurrentUserRole(membership?.role || null)
        }).catch(console.error).finally(() => setLoading(false))
    }, [id])

    useEffect(() => {
        fetchTeamData()
    }, [fetchTeamData])

    const fetchPending = useCallback(() => {
        if (!id) return
        api.teams.pending(id).then(d => {
            setPendingMembers(d.data || [])
        }).catch(() => {})
    }, [id])

    const handleJoin = async () => {
        if (!id) return
        try {
            await api.teams.join(id)
            fetchTeamData()
        } catch (e: any) { alert('Failed: ' + e.message) }
    }

    const handleRequestJoin = async () => {
        if (!id) return
        try {
            await api.teams.requestJoin(id)
            fetchTeamData()
        } catch (e: any) { alert('Failed: ' + e.message) }
    }

    const handleLeave = async () => {
        if (!id) return
        try {
            await api.teams.leave(id)
            fetchTeamData()
        } catch (e: any) { alert('Failed: ' + e.message) }
    }

    const handleStartEdit = () => {
        setEditName(team.name)
        setEditDescription(team.description || '')
        setEditIsPublic(team.is_public)
        setEditing(true)
    }

    const handleSaveEdit = async () => {
        if (!id) return
        try {
            const updated = await api.teams.update(id, {
                name: editName,
                description: editDescription,
                is_public: editIsPublic,
            })
            setTeam(updated)
            setEditing(false)
        } catch (e: any) { alert('Update failed: ' + e.message) }
    }

    const handleDelete = async () => {
        if (!id) return
        if (!confirm('Are you sure you want to delete this team? This cannot be undone.')) return
        try {
            await api.teams.delete(id)
            navigate('/teams')
        } catch (e: any) { alert('Delete failed: ' + e.message) }
    }

    const handleInvite = async () => {
        if (!id || !inviteUsername.trim()) return
        setInviting(true)
        try {
            await api.teams.invite(id, { username: inviteUsername.trim() })
            setInviteUsername('')
            fetchPending()
        } catch (e: any) { alert('Invite failed: ' + e.message) }
        finally { setInviting(false) }
    }

    const handleRespond = async (userId: string, action: string) => {
        if (!id) return
        setActing(`${userId}-${action}`)
        try {
            await api.teams.respond(id, { user_id: userId, action })
            fetchPending()
            fetchTeamData()
        } catch (e: any) { alert('Failed: ' + e.message) }
        finally { setActing(null) }
    }

    if (loading) return <div className="text-center py-20 text-gray-400 dark:text-gray-500">Loading team details...</div>
    if (!team) return <div className="text-center py-20 text-gray-400 dark:text-gray-500">Team not found</div>

    const globalRole = getRole()
    const canManage = currentUserRole === 'owner' || currentUserRole === 'captain' || globalRole === 'admin'
    const canDelete = currentUserRole === 'owner' || globalRole === 'admin'
    const activeMembers = members.filter(m => m.role === 'owner' || m.role === 'captain' || m.role === 'member')

    return (
        <div className="space-y-6">
            {editing ? (
                <div className="space-y-4 bg-white dark:bg-gray-900 p-6 rounded-lg border border-gray-200 dark:border-gray-800">
                    <h2 className="text-xl font-bold">Edit Team</h2>
                    <div>
                        <label className="block text-sm font-medium mb-1">Team Name</label>
                        <input type="text" value={editName} onChange={e => setEditName(e.target.value)}
                            className="w-full border border-gray-300 dark:border-gray-700 bg-transparent rounded p-2 text-sm text-gray-800 dark:text-gray-200" />
                    </div>
                    <div>
                        <label className="block text-sm font-medium mb-1">Description</label>
                        <textarea value={editDescription} onChange={e => setEditDescription(e.target.value)} rows={3}
                            className="w-full border border-gray-300 dark:border-gray-700 bg-transparent rounded p-2 text-sm text-gray-800 dark:text-gray-200" />
                    </div>
                    <div className="flex items-center gap-2">
                        <input type="checkbox" id="editIsPublic" checked={editIsPublic}
                            onChange={e => setEditIsPublic(e.target.checked)} className="rounded" />
                        <label htmlFor="editIsPublic" className="text-sm font-medium">Public Team</label>
                    </div>
                    <div className="flex gap-2 justify-end">
                        <button onClick={() => setEditing(false)}
                            className="px-4 py-2 border border-gray-300 dark:border-gray-700 rounded text-sm hover:bg-gray-100 dark:hover:bg-gray-800 cursor-pointer">Cancel</button>
                        <button onClick={handleSaveEdit}
                            className="px-4 py-2 bg-blue-600 text-white rounded text-sm hover:bg-blue-700 cursor-pointer">Save</button>
                    </div>
                </div>
            ) : (
                <>
                    {/* Header */}
                    <div className="flex justify-between items-start">
                        <div>
                            <div className="flex items-center gap-3">
                                <h1 className="text-2xl font-bold">{team.name}</h1>
                                <span className={`text-xs px-2 py-0.5 rounded font-semibold ${
                                    team.is_public
                                        ? 'bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-300'
                                        : 'bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300'
                                }`}>
                                    {team.is_public ? 'Public' : 'Private'}
                                </span>
                            </div>
                            <p className="text-gray-600 dark:text-gray-400 mt-2">{team.description || 'No description provided.'}</p>
                        </div>
                        <div className="flex items-center gap-4">
                            <RatingBadge rating={team.rating ?? 1500} showTitle />
                            {getAccessToken() && (
                                <>
                                    {canManage && (
                                        <>
                                            <button onClick={handleStartEdit}
                                                className="px-4 py-2 border border-gray-300 dark:border-gray-700 rounded text-sm hover:bg-gray-100 dark:hover:bg-gray-800 cursor-pointer">
                                                Edit
                                            </button>
                                            {canDelete && (
                                                <button onClick={handleDelete}
                                                    className="px-4 py-2 bg-red-600 text-white rounded text-sm hover:bg-red-700 cursor-pointer">
                                                    Delete
                                                </button>
                                            )}
                                        </>
                                    )}
                                    {isMember ? (
                                        <button onClick={handleLeave}
                                            className="px-4 py-2 rounded text-sm font-medium bg-red-50 dark:bg-red-900/20 text-red-600 dark:text-red-400 hover:bg-red-100 transition-colors cursor-pointer">
                                            Leave Team
                                        </button>
                                    ) : team.is_public ? (
                                        <button onClick={handleJoin}
                                            className="px-4 py-2 rounded text-sm font-medium bg-blue-600 text-white hover:bg-blue-700 transition-colors cursor-pointer">
                                            Join Team
                                        </button>
                                    ) : (
                                        <button onClick={handleRequestJoin}
                                            className="px-4 py-2 rounded text-sm font-medium bg-blue-600 text-white hover:bg-blue-700 transition-colors cursor-pointer">
                                            Request to Join
                                        </button>
                                    )}
                                </>
                            )}
                        </div>
                    </div>

                    {/* Stats */}
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

                    {/* Members + Management */}
                    <div>
                        <div className="flex items-center justify-between mb-3">
                            <h2 className="text-lg font-semibold">Members ({activeMembers.length})</h2>
                            {canManage && (
                                <button onClick={() => {
                                    setShowPending(!showPending)
                                    if (!showPending) fetchPending()
                                }}
                                    className="text-sm text-blue-600 dark:text-blue-400 hover:underline cursor-pointer">
                                    {showPending ? 'Hide Requests' : 'Pending Requests'}
                                </button>
                            )}
                        </div>

                        {/* Pending Requests Panel */}
                        {showPending && canManage && (
                            <div className="mb-4 space-y-3 bg-gray-50 dark:bg-gray-800/50 border border-gray-200 dark:border-gray-700 rounded-lg p-4">
                                <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300">Invite Member</h3>
                                <div className="flex gap-2">
                                    <input type="text" placeholder="Username..." value={inviteUsername}
                                        onChange={e => setInviteUsername(e.target.value)}
                                        className="flex-1 border border-gray-300 dark:border-gray-700 bg-transparent rounded p-2 text-sm" />
                                    <button onClick={handleInvite} disabled={inviting || !inviteUsername.trim()}
                                        className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded text-sm font-medium disabled:opacity-50 cursor-pointer">
                                        {inviting ? '...' : 'Invite'}
                                    </button>
                                </div>

                                {pendingMembers.length > 0 && (
                                    <div className="border-t border-gray-200 dark:border-gray-700 pt-3">
                                        <h4 className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase mb-2">Join Requests &amp; Invitations</h4>
                                        <div className="space-y-2">
                                            {pendingMembers.map((pm: any) => (
                                                <div key={pm.user_id} className="flex items-center justify-between bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded px-3 py-2">
                                                    <div className="flex items-center gap-2">
                                                        <span className="text-sm font-medium">{pm.username || pm.user_id}</span>
                                                        <span className={`text-xs px-1.5 py-0.5 rounded ${
                                                            pm.role === 'requested'
                                                                ? 'bg-yellow-50 dark:bg-yellow-900/20 text-yellow-700 dark:text-yellow-300'
                                                                : 'bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300'
                                                        }`}>
                                                            {pm.role === 'requested' ? 'Join Request' : 'Invited'}
                                                        </span>
                                                    </div>
                                                    <div className="flex gap-1.5">
                                                        {pm.role === 'requested' ? (
                                                            <>
                                                                <button onClick={() => handleRespond(pm.user_id, 'approve')}
                                                                    disabled={acting === `${pm.user_id}-approve`}
                                                                    className="bg-green-600 text-white px-2.5 py-1 rounded text-xs hover:bg-green-700 disabled:opacity-50 cursor-pointer">
                                                                    Approve
                                                                </button>
                                                                <button onClick={() => handleRespond(pm.user_id, 'reject')}
                                                                    disabled={acting === `${pm.user_id}-reject`}
                                                                    className="bg-red-600 text-white px-2.5 py-1 rounded text-xs hover:bg-red-700 disabled:opacity-50 cursor-pointer">
                                                                    Reject
                                                                </button>
                                                            </>
                                                        ) : (
                                                            <button onClick={() => handleRespond(pm.user_id, 'decline')}
                                                                disabled={acting === `${pm.user_id}-decline`}
                                                                className="bg-gray-600 text-white px-2.5 py-1 rounded text-xs hover:bg-gray-700 disabled:opacity-50 cursor-pointer">
                                                                Revoke
                                                            </button>
                                                        )}
                                                    </div>
                                                </div>
                                            ))}
                                        </div>
                                    </div>
                                )}
                            </div>
                        )}

                        {/* Members List */}
                        <div className="border rounded-lg divide-y bg-white dark:bg-gray-800 border-gray-200 dark:border-gray-700">
                            {activeMembers.map((m, i) => (
                                <div key={i} className="px-4 py-3 flex justify-between items-center text-sm">
                                    <span className="font-medium text-gray-800 dark:text-gray-200">{m.username || m.user_id}</span>
                                    <span className="text-xs text-gray-400 dark:text-gray-500 uppercase font-medium">{m.role}</span>
                                </div>
                            ))}
                            {activeMembers.length === 0 && (
                                <p className="px-4 py-6 text-sm text-gray-400 dark:text-gray-500 text-center">No members yet.</p>
                            )}
                        </div>
                    </div>
                </>
            )}
        </div>
    )
}
