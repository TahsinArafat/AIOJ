import { useEffect, useState } from 'react'
import type { TeamInvite, GroupInvite } from '../types/community'
import { errorMessage } from '../lib/errors'
import { Link, useNavigate } from 'react-router-dom'
import { api, getAccessToken, clearTokens, signOut } from '../lib/api'
import RatingBadge from '../components/RatingBadge'
import SetterApplication from '../components/SetterApplication'
import { useToast } from '../components/Toast'
import { buttonClass } from '../components/ui/buttonStyles'
import { Field, Input } from '../components/ui/Field'

function decodeUser() {
    const token = getAccessToken()
    if (!token) return null
    try {
        return JSON.parse(atob(token.split('.')[1]))
    } catch { return null }
}

const TABS = ['Edit Profile', 'Change Password', 'Pending Invites'] as const
type Tab = (typeof TABS)[number]

function EditProfileTab() {
    interface ProfileData {
    first_name?: string
    last_name?: string
    avatar_url?: string | null
    bio?: string | null
    city?: string | null
    country?: string | null
    organization?: string | null
    github_url?: string | null
    show_email: boolean
    show_tags: boolean
}

const [profile, setProfile] = useState<ProfileData | null>(null)
    const [loading, setLoading] = useState(true)
    const [saving, setSaving] = useState(false)
    const [message, setMessage] = useState('')

    useEffect(() => {
        api.users.getProfile()
            .then(d => setProfile(d))
            .catch(() => setProfile(null))
            .finally(() => setLoading(false))
    }, [])

    const handleChange = (field: string, value: string | boolean) => {
        setProfile(prev => (prev ? { ...prev, [field]: value } : prev))
    }

    const handleSave = async () => {
        if (!profile) return
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
        } catch (e) {
            setMessage('Error: ' + errorMessage(e))
        } finally {
            setSaving(false)
        }
    }

    if (loading) return <p className="text-muted-soft text-sm">Loading...</p>

    const fields = [
        { key: 'first_name', label: 'First Name', type: 'text' },
        { key: 'last_name', label: 'Last Name', type: 'text' },
        { key: 'country', label: 'Country', type: 'text' },
        { key: 'city', label: 'City', type: 'text' },
        { key: 'organization', label: 'Organization', type: 'text' },
        { key: 'github_url', label: 'GitHub URL', type: 'url' },
        { key: 'bio', label: 'Bio', type: 'textarea' },
        { key: 'avatar_url', label: 'Avatar URL', type: 'url' },
    ] as const

    return (
        <div className="space-y-4">
            {fields.map(f => {
                // Each control gets an id so its <label> is actually attached to
                // it: without `htmlFor`, clicking the label did not focus the
                // field and a screen reader announced an unnamed text box.
                const fieldId = `profile-${f.key}`
                return (
                    <Field key={f.key} label={f.label} htmlFor={fieldId}>
                        {f.type === 'textarea' ? (
                            <textarea
                                id={fieldId}
                                rows={3}
                                value={profile?.[f.key] || ''}
                                onChange={e => handleChange(f.key, e.target.value)}
                                className="w-full rounded-lg border border-hairline-strong bg-surface px-3 py-2 text-sm text-ink transition-colors hover:border-muted-soft focus:border-brand focus:outline-none"
                            />
                        ) : (
                            <Input
                                id={fieldId}
                                type={f.type}
                                value={profile?.[f.key] || ''}
                                onChange={e => handleChange(f.key, e.target.value)}
                            />
                        )}
                    </Field>
                )
            })}
            <div className="flex items-center gap-6">
                <label className="flex items-center gap-2 text-sm">
                    <input type="checkbox" checked={profile?.show_email ?? false}
                        onChange={e => handleChange('show_email', e.target.checked)}
                        className="rounded border-hairline-strong text-brand focus-visible:outline-[var(--ring)]" />
                    Show email on profile
                </label>
                <label className="flex items-center gap-2 text-sm">
                    <input type="checkbox" checked={profile?.show_tags ?? true}
                        onChange={e => handleChange('show_tags', e.target.checked)}
                        className="rounded border-hairline-strong text-brand focus-visible:outline-[var(--ring)]" />
                    Show problem tags
                </label>
            </div>
            {message && (
                <p className={`text-sm ${message.startsWith('Error') ? 'text-red-600 dark:text-red-400' : 'text-green-600 dark:text-green-400'}`}>{message}</p>
            )}
            <button onClick={handleSave} disabled={saving}
                className="bg-brand text-on-brand px-5 py-2 rounded text-sm hover:bg-brand-hover disabled:opacity-50 transition-colors">
                {saving ? 'Saving...' : 'Save Changes'}
            </button>
        </div>
    )
}

function ChangePasswordTab() {
    const navigate = useNavigate()
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
        // Mirrors auth.ValidatePasswordStrength; the server enforces it anyway,
        // but a 6-character minimum here only produced a confusing 400.
        if (newPassword.length < 12) {
            setMessage('New password must be at least 12 characters')
            return
        }
        setSaving(true)
        setMessage('')
        try {
            await api.users.updatePassword({ current_password: currentPassword, new_password: newPassword })
            // The server revoked every refresh token for this account, and the
            // current access token still carries the must-change flag, so the
            // only coherent next step is signing in again.
            setMessage('Password updated! Please sign in again.')
            setCurrentPassword('')
            setNewPassword('')
            setConfirmPassword('')
            await signOut()
            navigate('/login')
        } catch (e) {
            setMessage('Error: ' + errorMessage(e))
        } finally {
            setSaving(false)
        }
    }

    return (
        <form onSubmit={handleSubmit} className="space-y-4">
            <Field label="Current password" htmlFor="pw-current">
                <Input
                    id="pw-current"
                    type="password"
                    autoComplete="current-password"
                    value={currentPassword}
                    onChange={e => setCurrentPassword(e.target.value)}
                    required
                />
            </Field>
            <Field label="New password" htmlFor="pw-new">
                <Input
                    id="pw-new"
                    type="password"
                    autoComplete="new-password"
                    value={newPassword}
                    onChange={e => setNewPassword(e.target.value)}
                    required
                />
            </Field>
            <Field label="Confirm new password" htmlFor="pw-confirm">
                <Input
                    id="pw-confirm"
                    type="password"
                    autoComplete="new-password"
                    value={confirmPassword}
                    onChange={e => setConfirmPassword(e.target.value)}
                    required
                />
            </Field>
            {message && (
                <p className={`text-sm ${message.startsWith('Error') ? 'text-red-600 dark:text-red-400' : 'text-green-600 dark:text-green-400'}`}>{message}</p>
            )}
            <button type="submit" disabled={saving || !currentPassword || !newPassword}
                className="bg-brand text-on-brand px-5 py-2 rounded text-sm hover:bg-brand-hover disabled:opacity-50 transition-colors">
                {saving ? 'Updating...' : 'Update Password'}
            </button>
        </form>
    )
}

function PendingInvitesTab({ userId }: { userId: string }) {
    const toast = useToast()
    const [invites, setInvites] = useState<{ teams: TeamInvite[]; groups: GroupInvite[] }>({ teams: [], groups: [] })
    const [loading, setLoading] = useState(true)
    const [acting, setActing] = useState<string | null>(null)

    const fetchInvites = () => {
        setLoading(true)
        api.users.getPendingInvites()
            .then(d => setInvites({ teams: d.teams || [], groups: d.groups || [] }))
            .catch(() => { })
            .finally(() => setLoading(false))
    }

    useEffect(() => {
        // deferred a microtask: spinner still lands pre-paint, but the setState is
        // no longer synchronous (react-hooks/set-state-in-effect)
        queueMicrotask(() => fetchInvites())
    }, [])

    const handleRespond = async (type: 'team' | 'group', id: string, userId: string, action: string) => {
        setActing(`${type}-${id}-${action}`)
        try {
            if (type === 'team') {
                await api.teams.respond(id, { user_id: userId, action })
            } else {
                await api.groups.respond(id, { user_id: userId, action })
            }
            fetchInvites()
        } catch (e) {
            toast.error('Failed: ' + errorMessage(e))
        } finally {
            setActing(null)
        }
    }

    if (loading) return <p className="text-muted-soft text-sm">Loading...</p>

    const teamInvites = invites.teams.filter(i => i.role === 'invited')
    const groupInvites = invites.groups.filter(i => i.role === 'invited')
    const teamRequests = invites.teams.filter(i => i.role === 'requested')
    const groupRequests = invites.groups.filter(i => i.role === 'requested')

    return (
        <div className="space-y-6">
            {teamInvites.length === 0 && groupInvites.length === 0 && teamRequests.length === 0 && groupRequests.length === 0 ? (
                <p className="text-muted-soft text-sm">No pending invites or requests.</p>
            ) : (
                <>
                    {teamInvites.length > 0 && (
                        <div>
                            <h3 className="text-sm font-semibold text-ink-soft mb-2">Team Invitations</h3>
                            <div className="space-y-2">
                                {teamInvites.map(inv => (
                                    <div key={inv.team_id} className="flex items-center justify-between bg-surface-2/50 border border-hairline rounded px-4 py-3">
                                        <div>
                                            <Link to={`/teams/${inv.team_id}`} className="font-medium text-sm text-brand hover:underline">{inv.team_name}</Link>
                                            <p className="text-xs text-muted-soft">Invited to join</p>
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
                            <h3 className="text-sm font-semibold text-ink-soft mb-2">Group Invitations</h3>
                            <div className="space-y-2">
                                {groupInvites.map(inv => (
                                    <div key={inv.group_id} className="flex items-center justify-between bg-surface-2/50 border border-hairline rounded px-4 py-3">
                                        <div>
                                            <Link to={`/groups/${inv.group_id}`} className="font-medium text-sm text-brand hover:underline">{inv.group_name}</Link>
                                            <p className="text-xs text-muted-soft">Invited to join</p>
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
                            <h3 className="text-sm font-semibold text-ink-soft mb-2">Pending Team Join Requests</h3>
                            <div className="space-y-2">
                                {teamRequests.map(req => (
                                    <div key={req.team_id} className="flex items-center justify-between bg-surface-2/50 border border-hairline rounded px-4 py-3">
                                        <div>
                                            <Link to={`/teams/${req.team_id}`} className="font-medium text-sm text-brand hover:underline">{req.team_name}</Link>
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
                            <h3 className="text-sm font-semibold text-ink-soft mb-2">Pending Group Join Requests</h3>
                            <div className="space-y-2">
                                {groupRequests.map(req => (
                                    <div key={req.group_id} className="flex items-center justify-between bg-surface-2/50 border border-hairline rounded px-4 py-3">
                                        <div>
                                            <Link to={`/groups/${req.group_id}`} className="font-medium text-sm text-brand hover:underline">{req.group_name}</Link>
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
    const toast = useToast()
    const user = decodeUser()
    const [activeTab, setActiveTab] = useState<Tab>('Edit Profile')

    // Signed-out state: it previously said "please log in" with no way to do
    // so, and had no page heading at all.
    if (!user) {
        return (
            <div className="mx-auto max-w-md py-20 text-center">
                <h1 className="text-2xl font-semibold tracking-tight text-ink">Profile</h1>
                <p className="mt-2 text-sm text-muted">You need to be signed in to view your profile.</p>
                <div className="mt-5 flex justify-center gap-2">
                    <Link to="/login" className={buttonClass()}>Sign in</Link>
                    <Link to="/register" className={buttonClass({ variant: 'secondary' })}>Create an account</Link>
                </div>
            </div>
        )
    }

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
            <div className="space-y-4 bg-surface border border-hairline rounded-lg p-6 mb-6">
                <h1 className="text-2xl font-bold mb-4">Profile Settings</h1>
                <div className="grid grid-cols-2 gap-4">
                    <div>
                        <label className="block text-sm text-muted mb-1">Username</label>
                        <p className="font-medium">{user.uname || '—'}</p>
                    </div>
                    <div>
                        <label className="block text-sm text-muted mb-1">Email</label>
                        <p className="font-medium">{user.email || '—'}</p>
                    </div>
                    <div>
                        <label className="block text-sm text-muted mb-1">Role</label>
                        <span className={`px-2 py-1 rounded text-xs font-medium ${user.role === 'admin' ? 'bg-purple-100 text-purple-800' :
                            user.role === 'setter' ? 'bg-orange-100 text-orange-800' : 'bg-surface-3 text-ink'
                            }`}>{user.role || 'user'}</span>
                    </div>
                    <div>
                        <label className="block text-sm text-muted mb-1">Rating</label>
                        {user.rating ? (
                            <RatingBadge rating={user.rating} showTitle />
                        ) : (
                            <span className="text-muted-soft">Unrated</span>
                        )}
                    </div>
                </div>
                {user.rating && (
                    <div>
                        <Link to="/rating-history" className="text-sm text-brand hover:underline">
                            View Rating History →
                        </Link>
                    </div>
                )}
                <div className="flex gap-4 flex-wrap">
                    <Link to="/settings/notifications" className="text-sm text-brand hover:underline">
                        Notification Preferences →
                    </Link>
                    <Link to="/settings" className="text-sm text-brand hover:underline">
                        Security Settings →
                    </Link>
                    <Link to="/settings/api" className="text-sm text-brand hover:underline">
                        API Keys →
                    </Link>
                    <button
                        type="button"
                        onClick={async () => {
                            try {
                                const data = await api.users.exportMyData()
                                const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
                                const url = URL.createObjectURL(blob)
                                const a = document.createElement('a')
                                a.href = url
                                a.download = 'aioj-data-export.json'
                                a.click()
                                URL.revokeObjectURL(url)
                            } catch (e) {
                                toast.error(errorMessage(e) || 'Export failed')
                            }
                        }}
                        className="text-sm text-brand hover:underline"
                    >
                        Download My Data →
                    </button>
                </div>
            </div>

            {/* Tabs */}
            <div className="border-b border-hairline mb-6">
                <nav className="flex gap-6">
                    {TABS.map(tab => (
                        <button key={tab} onClick={() => setActiveTab(tab)}
                            className={`pb-3 text-sm font-medium border-b-2 transition-colors ${activeTab === tab
                                ? 'border-brand text-brand dark:border-blue-400 dark:text-blue-400'
                                : 'border-transparent text-muted hover:text-ink'
                                }`}>
                            {tab}
                        </button>
                    ))}
                </nav>
            </div>

            {/* Tab Content */}
            <div className="bg-surface border border-hairline rounded-lg p-6 mb-8">
                {renderTab()}
            </div>

            {/* Setter Application */}
            {user.role === 'user' && (
                <section className="bg-surface border border-hairline rounded-lg p-6">
                    <h2 className="text-lg font-semibold mb-4">Become a Problem Setter</h2>
                    <SetterApplication />
                </section>
            )}

            {/* GDPR danger zone */}
            <section className="bg-surface border border-red-200 dark:border-red-900 rounded-lg p-6 mt-6">
                <h2 className="text-lg font-semibold text-red-700 dark:text-red-400 mb-2">Danger Zone</h2>
                <p className="text-sm text-muted mb-4">
                    Permanently delete your account and personal data. Shared content you authored is kept with authorship cleared where required.
                    Download a copy first if you need it.
                </p>
                <DeleteAccountSection />
            </section>
        </div>
    )
}

function DeleteAccountSection() {
    const navigate = useNavigate()
    const [password, setPassword] = useState('')
    const [confirm, setConfirm] = useState(false)
    const [busy, setBusy] = useState(false)
    const [error, setError] = useState('')

    const handleDelete = async () => {
        if (!confirm) {
            setError('Confirm the permanent deletion to proceed')
            return
        }
        setBusy(true)
        setError('')
        try {
            await api.users.deleteAccount({ password, confirm: true })
            clearTokens()
            navigate('/')
        } catch (e) {
            setError(errorMessage(e) || 'Delete failed')
        } finally {
            setBusy(false)
        }
    }

    return (
        <div className="space-y-3 max-w-md">
            <Field
                label="Current password"
                htmlFor="delete-account-password"
                hint="Leave blank if you signed up with OAuth only."
            >
                <Input
                    id="delete-account-password"
                    type="password"
                    value={password}
                    onChange={e => setPassword(e.target.value)}
                    autoComplete="current-password"
                />
            </Field>
            <label className="flex items-start gap-2 text-sm text-ink-soft">
                <input
                    type="checkbox"
                    checked={confirm}
                    onChange={e => setConfirm(e.target.checked)}
                    className="mt-0.5"
                />
                I understand this action is permanent and cannot be undone.
            </label>
            {error && <p className="text-sm text-red-600 dark:text-red-400">{error}</p>}
            <button
                type="button"
                onClick={handleDelete}
                disabled={busy || !confirm}
                className="bg-red-600 hover:bg-red-700 disabled:opacity-50 text-white text-sm font-medium px-4 py-2 rounded"
            >
                {busy ? 'Deleting…' : 'Delete My Account'}
            </button>
        </div>
    )
}
