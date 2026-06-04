import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'
import RatingBadge from '../components/RatingBadge'

function decodeUser() {
    const token = getAccessToken()
    if (!token) return null
    try {
        return JSON.parse(atob(token.split('.')[1]))
    } catch { return null }
}

function SetterApplication() {
    const [status, setStatus] = useState<string | null>(null)
    const [reason, setReason] = useState('')
    const [submitted, setSubmitted] = useState(false)
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        api.setter.status().then(d => setStatus(d?.status || null)).catch(() => {}).finally(() => setLoading(false))
    }, [])

    const handleApply = async () => {
        try {
            await api.setter.apply(reason)
            setSubmitted(true)
            setStatus('pending')
        } catch (e: any) {
            alert('Failed: ' + e.message)
        }
    }

    if (loading) return <p className="text-sm text-gray-400 dark:text-gray-500">Loading...</p>
    if (status === 'approved') return <p className="text-green-600 dark:text-green-400 text-sm">You are a problem setter!</p>
    if (status === 'pending') return <p className="text-yellow-600 dark:text-yellow-400 text-sm">Your application is pending review.</p>

    return (
        <div>
            {status === 'rejected' && <p className="text-red-600 dark:text-red-400 text-sm mb-2">Your previous application was rejected. You can re-apply.</p>}
            {!submitted ? (
                <div>
                    <textarea rows={3} value={reason} onChange={e => setReason(e.target.value)}
                        placeholder="Why do you want to become a setter?"
                        className="w-full border rounded px-3 py-2 text-sm mb-2 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400" />
                    <button onClick={handleApply} disabled={!reason.trim()}
                        className="bg-blue-600 text-white px-4 py-2 rounded text-sm hover:bg-blue-700 disabled:opacity-50 transition-colors">
                        Apply as Problem Setter
                    </button>
                </div>
            ) : (
                <p className="text-green-600 dark:text-green-400 text-sm">Application submitted!</p>
            )}
        </div>
    )
}

const TABS = ['Edit Profile', 'Change Password', 'Pending Invites'] as const
type Tab = (typeof TABS)[number]

function EditProfileTab() {
    const [profile, setProfile] = useState<any>(null)
    const [loading, setLoading] = useState(true)
    const [saving, setSaving] = useState(false)
    const [message, setMessage] = useState('')

    useEffect(() => {
        api.users.getProfile()
            .then(d => setProfile(d))
            .catch(() => setProfile(null))
            .finally(() => setLoading(false))
    }, [])

    const handleChange = (field: string, value: any) => {
        setProfile((prev: any) => ({ ...prev, [field]: value }))
    }

    const handleSave = async () => {
        setSaving(true)
        setMessage('')
        try {
            const result = await api.users.updateProfile({
                first_name: profile.first_name || '',
                last_name: profile.last_name || '',
                country: profile.country || '',
                city: profile.city || '',
                organization: profile.organization || '',
                github_url: profile.github_url || '',
                bio: profile.bio || '',
                avatar_url: profile.avatar_url || '',
                show_email: profile.show_email ?? false,
                show_tags: profile.show_tags ?? true,
            })
            setProfile(result)
            setMessage('Profile saved!')
        } catch (e: any) {
            setMessage('Error: ' + e.message)
        } finally {
            setSaving(false)
        }
    }

    if (loading) return <p className="text-gray-400 dark:text-gray-500 text-sm">Loading...</p>

    const fields = [
        { key: 'first_name', label: 'First Name', type: 'text' },
        { key: 'last_name', label: 'Last Name', type: 'text' },
        { key: 'country', label: 'Country', type: 'text' },
        { key: 'city', label: 'City', type: 'text' },
        { key: 'organization', label: 'Organization', type: 'text' },
        { key: 'github_url', label: 'GitHub URL', type: 'url' },
        { key: 'bio', label: 'Bio', type: 'textarea' },
        { key: 'avatar_url', label: 'Avatar URL', type: 'url' },
    ]

    return (
        <div className="space-y-4">
            {fields.map(f => (
                <div key={f.key}>
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{f.label}</label>
                    {f.type === 'textarea' ? (
                        <textarea rows={3} value={profile?.[f.key] || ''} onChange={e => handleChange(f.key, e.target.value)}
                            className="w-full border rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400 dark:bg-gray-700 dark:border-gray-600" />
                    ) : (
                        <input type={f.type} value={profile?.[f.key] || ''} onChange={e => handleChange(f.key, e.target.value)}
                            className="w-full border rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400 dark:bg-gray-700 dark:border-gray-600" />
                    )}
                </div>
            ))}
            <div className="flex items-center gap-6">
                <label className="flex items-center gap-2 text-sm">
                    <input type="checkbox" checked={profile?.show_email ?? false}
                        onChange={e => handleChange('show_email', e.target.checked)}
                        className="rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500" />
                    Show email on profile
                </label>
                <label className="flex items-center gap-2 text-sm">
                    <input type="checkbox" checked={profile?.show_tags ?? true}
                        onChange={e => handleChange('show_tags', e.target.checked)}
                        className="rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500" />
                    Show problem tags
                </label>
            </div>
            {message && (
                <p className={`text-sm ${message.startsWith('Error') ? 'text-red-600 dark:text-red-400' : 'text-green-600 dark:text-green-400'}`}>{message}</p>
            )}
            <button onClick={handleSave} disabled={saving}
                className="bg-blue-600 text-white px-5 py-2 rounded text-sm hover:bg-blue-700 disabled:opacity-50 transition-colors">
                {saving ? 'Saving...' : 'Save Changes'}
            </button>
        </div>
    )
}

function ChangePasswordTab() {
    const [currentPassword, setCurrentPassword] = useState('')
    const [newPassword, setNewPassword] = useState('')
    const [confirmPassword, setConfirmPassword] = useState('')
    const [saving, setSaving] = useState(false)
    const [message, setMessage] = useState('')

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        if (newPassword !== confirmPassword) {
            setMessage('Passwords do not match')
            return
        }
        if (newPassword.length < 6) {
            setMessage('New password must be at least 6 characters')
            return
        }
        setSaving(true)
        setMessage('')
        try {
            await api.users.updatePassword({ current_password: currentPassword, new_password: newPassword })
            setMessage('Password updated!')
            setCurrentPassword('')
            setNewPassword('')
            setConfirmPassword('')
        } catch (e: any) {
            setMessage('Error: ' + e.message)
        } finally {
            setSaving(false)
        }
    }

    return (
        <form onSubmit={handleSubmit} className="space-y-4">
            <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Current Password</label>
                <input type="password" value={currentPassword} onChange={e => setCurrentPassword(e.target.value)} required
                    className="w-full border rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400 dark:bg-gray-700 dark:border-gray-600" />
            </div>
            <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">New Password</label>
                <input type="password" value={newPassword} onChange={e => setNewPassword(e.target.value)} required
                    className="w-full border rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400 dark:bg-gray-700 dark:border-gray-600" />
            </div>
            <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Confirm New Password</label>
                <input type="password" value={confirmPassword} onChange={e => setConfirmPassword(e.target.value)} required
                    className="w-full border rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400 dark:bg-gray-700 dark:border-gray-600" />
            </div>
            {message && (
                <p className={`text-sm ${message.startsWith('Error') ? 'text-red-600 dark:text-red-400' : 'text-green-600 dark:text-green-400'}`}>{message}</p>
            )}
            <button type="submit" disabled={saving || !currentPassword || !newPassword}
                className="bg-blue-600 text-white px-5 py-2 rounded text-sm hover:bg-blue-700 disabled:opacity-50 transition-colors">
                {saving ? 'Updating...' : 'Update Password'}
            </button>
        </form>
    )
}

function PendingInvitesTab({ userId }: { userId: string }) {
    const [invites, setInvites] = useState<{ teams: any[]; groups: any[] }>({ teams: [], groups: [] })
    const [loading, setLoading] = useState(true)
    const [acting, setActing] = useState<string | null>(null)

    const fetchInvites = () => {
        setLoading(true)
        api.users.getPendingInvites()
            .then(d => setInvites({ teams: d.teams || [], groups: d.groups || [] }))
            .catch(() => {})
            .finally(() => setLoading(false))
    }

    useEffect(() => { fetchInvites() }, [])

    const handleRespond = async (type: 'team' | 'group', id: string, userId: string, action: string) => {
        setActing(`${type}-${id}-${action}`)
        try {
            if (type === 'team') {
                await api.teams.respond(id, { user_id: userId, action })
            } else {
                await api.groups.respond(id, { user_id: userId, action })
            }
            fetchInvites()
        } catch (e: any) {
            alert('Failed: ' + e.message)
        } finally {
            setActing(null)
        }
    }

    if (loading) return <p className="text-gray-400 dark:text-gray-500 text-sm">Loading...</p>

    const teamInvites = invites.teams.filter(i => i.role === 'invited')
    const groupInvites = invites.groups.filter(i => i.role === 'invited')
    const teamRequests = invites.teams.filter(i => i.role === 'requested')
    const groupRequests = invites.groups.filter(i => i.role === 'requested')

    return (
        <div className="space-y-6">
            {teamInvites.length === 0 && groupInvites.length === 0 && teamRequests.length === 0 && groupRequests.length === 0 ? (
                <p className="text-gray-400 dark:text-gray-500 text-sm">No pending invites or requests.</p>
            ) : (
                <>
                    {teamInvites.length > 0 && (
                        <div>
                            <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2">Team Invitations</h3>
                            <div className="space-y-2">
                                {teamInvites.map(inv => (
                                    <div key={inv.team_id} className="flex items-center justify-between bg-gray-50 dark:bg-gray-700/50 border border-gray-200 dark:border-gray-600 rounded px-4 py-3">
                                        <div>
                                            <Link to={`/teams/${inv.team_id}`} className="font-medium text-sm text-blue-600 dark:text-blue-400 hover:underline">{inv.team_name}</Link>
                                            <p className="text-xs text-gray-400 dark:text-gray-500">Invited to join</p>
                                        </div>
                                        <div className="flex gap-2">
                                            <button onClick={() => handleRespond('team', inv.team_id, userId, 'accept')}
                                                disabled={acting === `team-${inv.team_id}-accept`}
                                                className="bg-green-600 text-white px-3 py-1.5 rounded text-xs hover:bg-green-700 disabled:opacity-50 transition-colors">
                                                Accept
                                            </button>
                                            <button onClick={() => handleRespond('team', inv.team_id, userId, 'decline')}
                                                disabled={acting === `team-${inv.team_id}-decline`}
                                                className="bg-red-600 text-white px-3 py-1.5 rounded text-xs hover:bg-red-700 disabled:opacity-50 transition-colors">
                                                Decline
                                            </button>
                                        </div>
                                    </div>
                                ))}
                            </div>
                        </div>
                    )}
                    {groupInvites.length > 0 && (
                        <div>
                            <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2">Group Invitations</h3>
                            <div className="space-y-2">
                                {groupInvites.map(inv => (
                                    <div key={inv.group_id} className="flex items-center justify-between bg-gray-50 dark:bg-gray-700/50 border border-gray-200 dark:border-gray-600 rounded px-4 py-3">
                                        <div>
                                            <Link to={`/groups/${inv.group_id}`} className="font-medium text-sm text-blue-600 dark:text-blue-400 hover:underline">{inv.group_name}</Link>
                                            <p className="text-xs text-gray-400 dark:text-gray-500">Invited to join</p>
                                        </div>
                                        <div className="flex gap-2">
                                            <button onClick={() => handleRespond('group', inv.group_id, userId, 'accept')}
                                                disabled={acting === `group-${inv.group_id}-accept`}
                                                className="bg-green-600 text-white px-3 py-1.5 rounded text-xs hover:bg-green-700 disabled:opacity-50 transition-colors">
                                                Accept
                                            </button>
                                            <button onClick={() => handleRespond('group', inv.group_id, userId, 'decline')}
                                                disabled={acting === `group-${inv.group_id}-decline`}
                                                className="bg-red-600 text-white px-3 py-1.5 rounded text-xs hover:bg-red-700 disabled:opacity-50 transition-colors">
                                                Decline
                                            </button>
                                        </div>
                                    </div>
                                ))}
                            </div>
                        </div>
                    )}
                    {teamRequests.length > 0 && (
                        <div>
                            <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2">Pending Team Join Requests</h3>
                            <div className="space-y-2">
                                {teamRequests.map(req => (
                                    <div key={req.team_id} className="flex items-center justify-between bg-gray-50 dark:bg-gray-700/50 border border-gray-200 dark:border-gray-600 rounded px-4 py-3">
                                        <div>
                                            <Link to={`/teams/${req.team_id}`} className="font-medium text-sm text-blue-600 dark:text-blue-400 hover:underline">{req.team_name}</Link>
                                            <p className="text-xs text-yellow-600 dark:text-yellow-400">Awaiting approval</p>
                                        </div>
                                        <button onClick={() => handleRespond('team', req.team_id, userId, 'decline')}
                                            disabled={acting === `team-${req.team_id}-decline`}
                                            className="bg-gray-600 text-white px-3 py-1.5 rounded text-xs hover:bg-gray-700 disabled:opacity-50 transition-colors">
                                            Cancel Request
                                        </button>
                                    </div>
                                ))}
                            </div>
                        </div>
                    )}
                    {groupRequests.length > 0 && (
                        <div>
                            <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2">Pending Group Join Requests</h3>
                            <div className="space-y-2">
                                {groupRequests.map(req => (
                                    <div key={req.group_id} className="flex items-center justify-between bg-gray-50 dark:bg-gray-700/50 border border-gray-200 dark:border-gray-600 rounded px-4 py-3">
                                        <div>
                                            <Link to={`/groups/${req.group_id}`} className="font-medium text-sm text-blue-600 dark:text-blue-400 hover:underline">{req.group_name}</Link>
                                            <p className="text-xs text-yellow-600 dark:text-yellow-400">Awaiting approval</p>
                                        </div>
                                        <button onClick={() => handleRespond('group', req.group_id, userId, 'decline')}
                                            disabled={acting === `group-${req.group_id}-decline`}
                                            className="bg-gray-600 text-white px-3 py-1.5 rounded text-xs hover:bg-gray-700 disabled:opacity-50 transition-colors">
                                            Cancel Request
                                        </button>
                                    </div>
                                ))}
                            </div>
                        </div>
                    )}
                </>
            )}
        </div>
    )
}

export default function Profile() {
    const user = decodeUser()
    const [activeTab, setActiveTab] = useState<Tab>('Edit Profile')

    if (!user) return <div className="text-center py-20 text-gray-400 dark:text-gray-500">Please log in to view your profile.</div>

    const renderTab = () => {
        switch (activeTab) {
            case 'Edit Profile': return <EditProfileTab />
            case 'Change Password': return <ChangePasswordTab />
            case 'Pending Invites': return <PendingInvitesTab userId={user.uid} />
        }
    }

    return (
        <div className="max-w-2xl mx-auto px-4 py-8">
            {/* User Info Card */}
            <div className="space-y-4 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-6 mb-6">
                <h1 className="text-2xl font-bold mb-4">Profile Settings</h1>
                <div className="grid grid-cols-2 gap-4">
                    <div>
                        <label className="block text-sm text-gray-500 dark:text-gray-400 mb-1">Username</label>
                        <p className="font-medium">{user.uname || '—'}</p>
                    </div>
                    <div>
                        <label className="block text-sm text-gray-500 dark:text-gray-400 mb-1">Email</label>
                        <p className="font-medium">{user.email || '—'}</p>
                    </div>
                    <div>
                        <label className="block text-sm text-gray-500 dark:text-gray-400 mb-1">Role</label>
                        <span className={`px-2 py-1 rounded text-xs font-medium ${
                            user.role === 'admin' ? 'bg-purple-100 text-purple-800' :
                            user.role === 'teacher' ? 'bg-orange-100 text-orange-800' : 'bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-200'
                        }`}>{user.role || 'user'}</span>
                    </div>
                    <div>
                        <label className="block text-sm text-gray-500 dark:text-gray-400 mb-1">Rating</label>
                        {user.rating ? (
                            <RatingBadge rating={user.rating} showTitle />
                        ) : (
                            <span className="text-gray-400 dark:text-gray-500">Unrated</span>
                        )}
                    </div>
                </div>
                {user.rating && (
                    <div>
                        <Link to="/rating-history" className="text-sm text-blue-600 dark:text-blue-400 hover:underline">
                            View Rating History →
                        </Link>
                    </div>
                )}
                <div className="flex gap-4">
                    <Link to="/settings/notifications" className="text-sm text-blue-600 dark:text-blue-400 hover:underline">
                        Notification Preferences →
                    </Link>
                    <Link to="/settings/api" className="text-sm text-blue-600 dark:text-blue-400 hover:underline">
                        API Keys →
                    </Link>
                </div>
            </div>

            {/* Tabs */}
            <div className="border-b border-gray-200 dark:border-gray-700 mb-6">
                <nav className="flex gap-6">
                    {TABS.map(tab => (
                        <button key={tab} onClick={() => setActiveTab(tab)}
                            className={`pb-3 text-sm font-medium border-b-2 transition-colors ${
                                activeTab === tab
                                    ? 'border-blue-600 text-blue-600 dark:border-blue-400 dark:text-blue-400'
                                    : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'
                            }`}>
                            {tab}
                        </button>
                    ))}
                </nav>
            </div>

            {/* Tab Content */}
            <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-6 mb-8">
                {renderTab()}
            </div>

            {/* Setter Application */}
            {user.role === 'user' && (
                <section className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-6">
                    <h2 className="text-lg font-semibold mb-4">Become a Problem Setter</h2>
                    <SetterApplication />
                </section>
            )}
        </div>
    )
}
