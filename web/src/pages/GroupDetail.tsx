import { useEffect, useState, useCallback } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'

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

export default function GroupDetail() {
    const { id } = useParams<{ id: string }>()
    const navigate = useNavigate()
    const [group, setGroup] = useState<any>(null)
    const [members, setMembers] = useState<any[]>([])
    const [isMember, setIsMember] = useState(false)
    const [currentUserRole, setCurrentUserRole] = useState<string | null>(null)
    const [loading, setLoading] = useState(true)

    const [activeTab, setActiveTab] = useState<'members' | 'contests' | 'pending'>('members')
    const [contests, setContests] = useState<any[]>([])

    const [editing, setEditing] = useState(false)
    const [editName, setEditName] = useState('')
    const [editDescription, setEditDescription] = useState('')
    const [editIsPublic, setEditIsPublic] = useState(true)
    const [editJoinPolicy, setEditJoinPolicy] = useState('auto_approve')

    // Invite state
    const [inviteUsername, setInviteUsername] = useState('')
    const [inviting, setInviting] = useState(false)

    // Pending members
    const [pendingMembers, setPendingMembers] = useState<any[]>([])
    const [acting, setActing] = useState<string | null>(null)

    const [newContestId, setNewContestId] = useState('')

    const fetchGroupData = useCallback(() => {
        if (!id) return
        const currentUserId = getUserId()
        setLoading(true)
        Promise.all([
            api.groups.get(id),
            api.groups.members(id),
            api.groups.getContests(id),
        ]).then(([g, m, c]) => {
            setGroup(g)
            const memberList = m.data || []
            setMembers(memberList)
            setContests(c.data || [])
            const membership = memberList.find((mb: any) => mb.user_id === currentUserId)
            setIsMember(!!membership)
            setCurrentUserRole(membership?.role || null)
        }).catch(console.error).finally(() => setLoading(false))
    }, [id])

    useEffect(() => {
        fetchGroupData()
    }, [fetchGroupData])

    const fetchPending = useCallback(() => {
        if (!id) return
        api.groups.pending(id).then(d => {
            setPendingMembers(d.data || [])
        }).catch(() => {})
    }, [id])

    const handleJoin = async () => {
        if (!id) return
        try {
            await api.groups.join(id)
            fetchGroupData()
        } catch (e: any) { alert('Failed: ' + e.message) }
    }

    const handleLeave = async () => {
        if (!id) return
        try {
            await api.groups.leave(id)
            fetchGroupData()
        } catch (e: any) { alert('Failed: ' + e.message) }
    }

    const handleStartEdit = () => {
        setEditName(group.name)
        setEditDescription(group.description || '')
        setEditIsPublic(group.is_public)
        setEditJoinPolicy(group.join_policy || 'auto_approve')
        setEditing(true)
    }

    const handleSaveEdit = async () => {
        if (!id) return
        try {
            const updated = await api.groups.update(id, {
                name: editName,
                description: editDescription,
                is_public: editIsPublic,
                join_policy: editJoinPolicy,
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

    const handleInvite = async () => {
        if (!id || !inviteUsername.trim()) return
        setInviting(true)
        try {
            await api.groups.invite(id, { username: inviteUsername.trim() })
            setInviteUsername('')
            fetchPending()
        } catch (e: any) { alert('Invite failed: ' + e.message) }
        finally { setInviting(false) }
    }

    const handleRespond = async (userId: string, action: string) => {
        if (!id) return
        setActing(`${userId}-${action}`)
        try {
            await api.groups.respond(id, { user_id: userId, action })
            fetchPending()
            fetchGroupData()
        } catch (e: any) { alert('Failed: ' + e.message) }
        finally { setActing(null) }
    }

    const handleCopyInviteLink = () => {
        if (group?.invite_code) {
            const url = `${window.location.origin}/groups/join?code=${group.invite_code}`
            navigator.clipboard.writeText(url).then(() => {
                alert('Invite link copied to clipboard!')
            }).catch(() => {
                // fallback
                const ta = document.createElement('textarea')
                ta.value = url
                document.body.appendChild(ta)
                ta.select()
                document.execCommand('copy')
                document.body.removeChild(ta)
                alert('Invite link copied!')
            })
        }
    }

    if (loading) return <div className="text-center py-20 text-gray-400 dark:text-gray-500">Loading group details...</div>
    if (!group) return <div className="text-center py-20 text-gray-400 dark:text-gray-500">Group not found</div>

    const globalRole = getRole()
    const canManage = currentUserRole === 'owner' || currentUserRole === 'manager' || globalRole === 'admin'
    const canDelete = currentUserRole === 'owner' || globalRole === 'admin'
    const activeMembers = members.filter(m => m.role === 'owner' || m.role === 'manager' || m.role === 'member')

    const TABS: { key: typeof activeTab; label: string }[] = [
        { key: 'members', label: `Members (${activeMembers.length})` },
        { key: 'contests', label: `Contests (${contests.length})` },
    ]
    if (canManage) {
        TABS.push({ key: 'pending', label: 'Pending' })
    }

    return (
        <div className="space-y-6">
            {editing ? (
                <div className="space-y-4 bg-white dark:bg-gray-900 p-6 rounded-lg border border-gray-200 dark:border-gray-800">
                    <h2 className="text-xl font-bold">Edit Group</h2>
                    <div>
                        <label className="block text-sm font-medium mb-1">Group Name</label>
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
                        <label htmlFor="editIsPublic" className="text-sm font-medium">Public Group</label>
                    </div>
                    <div>
                        <label className="block text-sm font-medium mb-1">Join Policy</label>
                        <select value={editJoinPolicy} onChange={e => setEditJoinPolicy(e.target.value)}
                            className="w-full border border-gray-300 dark:border-gray-700 bg-transparent rounded p-2 text-sm text-gray-800 dark:text-gray-200">
                            <option value="auto_approve">Auto-Approve (Open via link)</option>
                            <option value="manual_approve">Manual Approve (Requires confirmation)</option>
                        </select>
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
                            <div className="flex items-center gap-3 flex-wrap">
                                <h1 className="text-2xl font-bold">{group.name}</h1>
                                <span className={`text-xs px-2 py-0.5 rounded font-semibold ${
                                    group.is_public
                                        ? 'bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-300'
                                        : 'bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300'
                                }`}>
                                    {group.is_public ? 'Public' : 'Private'}
                                </span>
                                <span className={`text-xs px-2 py-0.5 rounded font-semibold ${
                                    group.join_policy === 'auto_approve'
                                        ? 'bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300'
                                        : 'bg-amber-50 dark:bg-amber-900/20 text-amber-700 dark:text-amber-300'
                                }`}>
                                    {group.join_policy === 'auto_approve' ? 'Auto-Join via Link' : 'Approval Required'}
                                </span>
                            </div>
                            <p className="text-gray-600 dark:text-gray-400 mt-2">{group.description || 'No description provided.'}</p>
                        </div>
                        <div className="flex gap-2">
                            {canManage && (
                                <>
                                    <button onClick={handleStartEdit}
                                        className="px-4 py-2 border border-gray-300 dark:border-gray-700 rounded text-sm hover:bg-gray-100 dark:hover:bg-gray-800 cursor-pointer">Edit</button>
                                    {canDelete && (
                                        <button onClick={handleDelete}
                                            className="px-4 py-2 bg-red-600 text-white rounded text-sm hover:bg-red-700 cursor-pointer">Delete</button>
                                    )}
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

                    {/* Tabs */}
                    <div className="border-b border-gray-200 dark:border-gray-800 flex gap-4">
                        {TABS.map(tab => (
                            <button key={tab.key} onClick={() => {
                                setActiveTab(tab.key)
                                if (tab.key === 'pending') fetchPending()
                            }}
                                className={`pb-2 text-sm font-medium border-b-2 cursor-pointer ${
                                    activeTab === tab.key
                                        ? 'border-blue-600 text-blue-600 dark:text-blue-400'
                                        : 'border-transparent text-gray-500 hover:text-gray-700'
                                }`}>
                                {tab.label}
                            </button>
                        ))}
                    </div>

                    {/* Tab: Members */}
                    {activeTab === 'members' && (
                        <div className="border rounded-lg divide-y bg-white dark:bg-gray-800 border-gray-200 dark:border-gray-700">
                            {activeMembers.map((m, i) => (
                                <div key={i} className="px-4 py-3 flex justify-between items-center text-sm">
                                    <span className="font-medium text-gray-800 dark:text-gray-200">{m.username || m.user_id}</span>
                                    <span className="text-xs text-gray-400 dark:text-gray-500 uppercase font-medium">{m.role}</span>
                                </div>
                            ))}
                            {activeMembers.length === 0 && (
                                <p className="p-4 text-sm text-gray-400 dark:text-gray-500 text-center">No members yet.</p>
                            )}
                        </div>
                    )}

                    {/* Tab: Contests */}
                    {activeTab === 'contests' && (
                        <div className="space-y-4">
                            {canManage && (
                                <div className="flex gap-2 max-w-md">
                                    <input type="text" placeholder="Contest ID..." value={newContestId}
                                        onChange={e => setNewContestId(e.target.value)}
                                        className="flex-1 border border-gray-300 dark:border-gray-700 bg-transparent rounded p-2 text-sm text-gray-800 dark:text-gray-200" />
                                    <button onClick={handleAddContest}
                                        className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded text-sm font-medium cursor-pointer">
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

                    {/* Tab: Pending (manager only) */}
                    {activeTab === 'pending' && canManage && (
                        <div className="space-y-4">
                            {/* Invite Code & Link */}
                            <div className="bg-gray-50 dark:bg-gray-800/50 border border-gray-200 dark:border-gray-700 rounded-lg p-4 space-y-3">
                                <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300">Invite Link</h3>
                                {group.invite_code ? (
                                    <div className="flex items-center gap-2">
                                        <code className="flex-1 bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded px-3 py-2 text-sm font-mono">
                                            {`${window.location.origin}/groups/join?code=${group.invite_code}`}
                                        </code>
                                        <button onClick={handleCopyInviteLink}
                                            className="px-3 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded text-sm font-medium cursor-pointer whitespace-nowrap">
                                            Copy Link
                                        </button>
                                    </div>
                                ) : (
                                    <p className="text-sm text-gray-400 dark:text-gray-500">No invite code available.</p>
                                )}
                            </div>

                            {/* Invite Member */}
                            <div className="bg-gray-50 dark:bg-gray-800/50 border border-gray-200 dark:border-gray-700 rounded-lg p-4 space-y-3">
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
                            </div>

                            {/* Pending Requests & Invitations */}
                            {pendingMembers.length > 0 && (
                                <div className="space-y-2">
                                    <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300">
                                        Requests &amp; Invitations ({pendingMembers.length})
                                    </h3>
                                    <div className="border rounded-lg divide-y bg-white dark:bg-gray-800 border-gray-200 dark:border-gray-700">
                                        {pendingMembers.map((pm: any) => (
                                            <div key={pm.user_id} className="px-4 py-3 flex items-center justify-between text-sm">
                                                <div className="flex items-center gap-2">
                                                    <span className="font-medium">{pm.username || pm.user_id}</span>
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

                            {pendingMembers.length === 0 && (
                                <p className="text-sm text-gray-400 dark:text-gray-500 text-center py-8">No pending requests or invitations.</p>
                            )}
                        </div>
                    )}
                </>
            )}
        </div>
    )
}
