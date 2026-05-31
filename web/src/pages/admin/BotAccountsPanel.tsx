import { useEffect, useState, useRef } from 'react'
import { api } from '../../lib/api'
import { Plus, Pencil, Trash2, X } from 'lucide-react'

interface BotAccount {
    id: string
    user_id: string
    platform: string
    platform_user: string
    platform_pass: string
    api_key: string
    api_secret: string
    session_data?: Record<string, string>
    status: string
    rate_limit_rps: number
    last_used_at: string | null
    created_at: string
}

interface BotForm {
    user_id: string
    platform: string
    platform_user: string
    platform_pass: string
    api_key: string
    api_secret: string
    rate_limit_rps: string
    session_data: string
}

const emptyForm: BotForm = { user_id: '', platform: 'codeforces', platform_user: '', platform_pass: '', api_key: '', api_secret: '', rate_limit_rps: '1.0', session_data: '' }

const PLATFORMS = [
    { value: 'codeforces', label: 'Codeforces', hint: 'Username + Password for web auth, API Key + Secret for verdict polling' },
    { value: 'atcoder', label: 'AtCoder', hint: 'Username + Password' },
    { value: 'cses', label: 'CSES', hint: 'Username + Password' },
    { value: 'toph', label: 'Toph', hint: 'Username + Password' },
    { value: 'qoj', label: 'QOJ', hint: 'Username + Password' },
]

export default function BotAccountsPanel() {
    const [bots, setBots] = useState<BotAccount[]>([])
    const [loading, setLoading] = useState(true)
    const [showForm, setShowForm] = useState(false)
    const [editingId, setEditingId] = useState<string | null>(null)
    const [form, setForm] = useState<BotForm>(emptyForm)
    const [saving, setSaving] = useState(false)
    const [users, setUsers] = useState<{id: string, username: string}[]>([])
    const [userSearch, setUserSearch] = useState('')
    const [showUserDropdown, setShowUserDropdown] = useState(false)
    const userDropdownRef = useRef<HTMLDivElement>(null)

    const loadBots = () => {
        setLoading(true)
        api.admin.botAccounts.list().then(d => setBots(d.data || [])).catch(console.error).finally(() => setLoading(false))
    }

    useEffect(() => { 
        loadBots()
        api.admin.listUsers(0, 100).then(d => setUsers(d.data || [])).catch(() => {})
    }, [])

    useEffect(() => {
        const handler = (e: MouseEvent) => {
            if (userDropdownRef.current && !userDropdownRef.current.contains(e.target as Node)) {
                setShowUserDropdown(false)
            }
        }
        document.addEventListener('mousedown', handler)
        return () => document.removeEventListener('mousedown', handler)
    }, [])

    const resetForm = () => {
        setForm(emptyForm)
        setEditingId(null)
        setShowForm(false)
    }

    const startEdit = (bot: BotAccount) => {
        setEditingId(bot.id)
        setForm({
            user_id: bot.user_id,
            platform: bot.platform,
            platform_user: bot.platform_user || '',
            platform_pass: '',
            api_key: bot.api_key || '',
            api_secret: '',
            rate_limit_rps: String(bot.rate_limit_rps),
            session_data: bot.session_data ? JSON.stringify(bot.session_data) : '',
        })
        setShowForm(true)
    }

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        setSaving(true)
        try {
            if (editingId) {
                const update: any = {}
                if (form.platform_user) update.platform_user = form.platform_user
                if (form.platform_pass) update.platform_pass = form.platform_pass
                if (form.api_key) update.api_key = form.api_key
                if (form.api_secret) update.api_secret = form.api_secret
                update.rate_limit_rps = parseFloat(form.rate_limit_rps) || 1.0
                if (form.session_data) {
                    try {
                        update.session_data = JSON.parse(form.session_data)
                    } catch {}
                }
                await api.admin.botAccounts.update(editingId, update)
            } else {
                const payload: any = {
                    user_id: form.user_id || 'system',
                    platform: form.platform,
                    platform_user: form.platform_user,
                    platform_pass: form.platform_pass,
                    api_key: form.api_key,
                    api_secret: form.api_secret,
                    rate_limit_rps: parseFloat(form.rate_limit_rps) || 1.0,
                }
                if (form.session_data) {
                    try {
                        payload.session_data = JSON.parse(form.session_data)
                    } catch {}
                }
                await api.admin.botAccounts.create(payload)
            }
            resetForm()
            loadBots()
        } catch (err: any) {
            alert(err.message)
        } finally {
            setSaving(false)
        }
    }

    const handleDelete = async (id: string) => {
        if (!confirm('Delete this bot account?')) return
        try {
            await api.admin.botAccounts.delete(id)
            loadBots()
        } catch (err: any) {
            alert(err.message)
        }
    }

    const handleStatusToggle = async (bot: BotAccount) => {
        const newStatus = bot.status === 'active' ? 'banned' : 'active'
        try {
            await api.admin.botAccounts.update(bot.id, { status: newStatus })
            loadBots()
        } catch (err: any) {
            alert(err.message)
        }
    }

    const statusColor = (s: string) =>
        s === 'active' ? 'bg-green-100 text-green-800' :
        s === 'expired' ? 'bg-yellow-100 text-yellow-800' :
        s === 'error' ? 'bg-red-100 text-red-800' :
        s === 'banned' ? 'bg-red-100 text-red-800' :
        'bg-gray-100 text-gray-800'

    const platformHint = PLATFORMS.find(p => p.value === form.platform)?.hint || ''

    if (loading) return <div className="text-center py-8 text-gray-400">Loading...</div>

    return (
        <div>
            <div className="flex items-center justify-between mb-4">
                <div>
                    <h2 className="text-lg font-semibold">VJudge Bot Accounts</h2>
                    <p className="text-sm text-gray-500 mt-1">Manage credentials for remote OJ platforms (Codeforces, AtCoder, etc.)</p>
                </div>
                <button onClick={() => { resetForm(); setShowForm(true) }}
                    className="flex items-center gap-1.5 bg-blue-600 text-white px-3 py-1.5 rounded text-sm hover:bg-blue-700 transition-colors">
                    <Plus className="w-4 h-4" /> Add Bot Account
                </button>
            </div>

            {showForm && (
                <div className="bg-white border border-gray-200 rounded-lg p-5 mb-6">
                    <div className="flex items-center justify-between mb-4">
                        <h3 className="font-semibold text-sm">{editingId ? 'Edit Bot Account' : 'Add New Bot Account'}</h3>
                        <button onClick={resetForm} className="text-gray-400 hover:text-gray-600"><X className="w-4 h-4" /></button>
                    </div>
                    <form onSubmit={handleSubmit} className="space-y-4">
                        <div className="grid grid-cols-2 gap-4">
                            <div>
                                <label className="block text-xs font-medium text-gray-500 mb-1">Platform</label>
                                <select
                                    required
                                    value={form.platform}
                                    onChange={e => setForm({ ...form, platform: e.target.value })}
                                    className="w-full border rounded px-3 py-2 text-sm"
                                >
                                    {PLATFORMS.map(p => (
                                        <option key={p.value} value={p.value}>{p.label}</option>
                                    ))}
                                </select>
                                {platformHint && <p className="text-xs text-gray-400 mt-1">{platformHint}</p>}
                            </div>
                            <div ref={userDropdownRef}>
                                <label className="block text-xs font-medium text-gray-500 mb-1">Owner</label>
                                <div className="relative">
                                    <input
                                        type="text"
                                        value={userSearch}
                                        onChange={e => {
                                            setUserSearch(e.target.value)
                                            setShowUserDropdown(true)
                                            if (!e.target.value) setForm({ ...form, user_id: '' })
                                        }}
                                        onFocus={() => setShowUserDropdown(true)}
                                        placeholder="Search user..."
                                        className="w-full border rounded px-3 py-2 text-sm"
                                    />
                                    {showUserDropdown && (
                                        <div className="absolute z-10 mt-1 w-full bg-white border border-gray-200 rounded-md shadow-lg max-h-48 overflow-y-auto">
                                            <div
                                                onClick={() => { setForm({ ...form, user_id: '' }); setUserSearch(''); setShowUserDropdown(false) }}
                                                className="px-3 py-2 text-sm text-gray-500 hover:bg-gray-50 cursor-pointer"
                                            >
                                                System-owned (no user)
                                            </div>
                                            {users.filter(u => u.username.toLowerCase().includes(userSearch.toLowerCase())).map(u => (
                                                <div
                                                    key={u.id}
                                                    onClick={() => { setForm({ ...form, user_id: u.id }); setUserSearch(u.username); setShowUserDropdown(false) }}
                                                    className={`px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 ${form.user_id === u.id ? 'bg-blue-50 text-blue-700' : ''}`}
                                                >
                                                    {u.username}
                                                </div>
                                            ))}
                                            {users.filter(u => u.username.toLowerCase().includes(userSearch.toLowerCase())).length === 0 && (
                                                <div className="px-3 py-2 text-sm text-gray-400">No users found</div>
                                            )}
                                        </div>
                                    )}
                                </div>
                            </div>
                        </div>
                        <div className="border-t border-gray-100 pt-4">
                            <h4 className="text-xs font-semibold text-gray-500 uppercase mb-3">Authentication</h4>
                            <div className="grid grid-cols-2 gap-4">
                                <div>
                                    <label className="block text-xs font-medium text-gray-500 mb-1">Username</label>
                                    <input type="text" value={form.platform_user} onChange={e => setForm({ ...form, platform_user: e.target.value })}
                                        placeholder="Remote OJ username" className="w-full border rounded px-3 py-2 text-sm" />
                                </div>
                                <div>
                                    <label className="block text-xs font-medium text-gray-500 mb-1">Password</label>
                                    <input type="password" value={form.platform_pass} onChange={e => setForm({ ...form, platform_pass: e.target.value })}
                                        placeholder="Leave empty to keep existing" className="w-full border rounded px-3 py-2 text-sm" />
                                </div>
                            </div>
                        </div>
                        {form.platform === 'codeforces' && (
                            <div className="border-t border-gray-100 pt-4">
                                <h4 className="text-xs font-semibold text-gray-500 uppercase mb-3">Codeforces API Key (Optional, for verdict polling)</h4>
                                <div className="grid grid-cols-2 gap-4">
                                    <div>
                                        <label className="block text-xs font-medium text-gray-500 mb-1">API Key</label>
                                        <input type="text" value={form.api_key} onChange={e => setForm({ ...form, api_key: e.target.value })}
                                            placeholder="From https://codeforces.com/settings/api" className="w-full border rounded px-3 py-2 text-sm" />
                                    </div>
                                    <div>
                                        <label className="block text-xs font-medium text-gray-500 mb-1">API Secret</label>
                                        <input type="password" value={form.api_secret} onChange={e => setForm({ ...form, api_secret: e.target.value })}
                                            placeholder="Leave empty to keep existing" className="w-full border rounded px-3 py-2 text-sm" />
                                    </div>
                                </div>
                            </div>
                        )}
                        {form.platform === 'codeforces' && (
                            <div className="border-t border-gray-100 pt-4">
                                <h4 className="text-xs font-semibold text-gray-500 uppercase mb-3">Browser Cookies (paste from DevTools)</h4>
                                <div className="space-y-3">
                                    <div>
                                        <label className="block text-xs font-medium text-gray-500 mb-1">JSESSIONID</label>
                                        <input type="text" value={form.session_data ? JSON.parse(form.session_data || '{}').JSESSIONID || '' : ''}
                                            onChange={e => {
                                                try {
                                                    const current = JSON.parse(form.session_data || '{}')
                                                    current.JSESSIONID = e.target.value
                                                    setForm({ ...form, session_data: JSON.stringify(current) })
                                                } catch { setForm({ ...form, session_data: JSON.stringify({ JSESSIONID: e.target.value }) }) }
                                            }}
                                            placeholder="Copy from browser DevTools (F12) → Application → Cookies → JSESSIONID"
                                            className="w-full border rounded px-3 py-2 text-sm font-mono" />
                                    </div>
                                    <div>
                                        <label className="block text-xs font-medium text-gray-500 mb-1">39ce7</label>
                                        <input type="text" value={form.session_data ? JSON.parse(form.session_data || '{}')['39ce7'] || '' : ''}
                                            onChange={e => {
                                                try {
                                                    const current = JSON.parse(form.session_data || '{}')
                                                    current['39ce7'] = e.target.value
                                                    setForm({ ...form, session_data: JSON.stringify(current) })
                                                } catch { setForm({ ...form, session_data: JSON.stringify({ '39ce7': e.target.value }) }) }
                                            }}
                                            placeholder="Copy from browser DevTools (F12) → Application → Cookies → 39ce7"
                                            className="w-full border rounded px-3 py-2 text-sm font-mono" />
                                    </div>
                                    <div>
                                        <label className="block text-xs font-medium text-gray-500 mb-1">cf_clearance (optional, auto-generated by bypass proxy)</label>
                                        <input type="text" value={form.session_data ? JSON.parse(form.session_data || '{}').cf_clearance || '' : ''}
                                            onChange={e => {
                                                try {
                                                    const current = JSON.parse(form.session_data || '{}')
                                                    current.cf_clearance = e.target.value
                                                    setForm({ ...form, session_data: JSON.stringify(current) })
                                                } catch { setForm({ ...form, session_data: JSON.stringify({ cf_clearance: e.target.value }) }) }
                                            }}
                                            placeholder="Auto-filled by bypass proxy, or paste from browser"
                                            className="w-full border rounded px-3 py-2 text-sm font-mono" />
                                    </div>
                                    <p className="text-xs text-gray-400 mt-1">
                                        Get cookies from browser: DevTools (F12) → Application → Cookies → codeforces.com. 
                                        The bypass proxy auto-generates cf_clearance for the server IP.
                                    </p>
                                </div>
                            </div>
                        )}
                        <div className="border-t border-gray-100 pt-4">
                            <div className="grid grid-cols-2 gap-4">
                                <div>
                                    <label className="block text-xs font-medium text-gray-500 mb-1">Rate Limit (requests/sec)</label>
                                    <input type="number" step="0.1" min="0.1" value={form.rate_limit_rps} onChange={e => setForm({ ...form, rate_limit_rps: e.target.value })}
                                        className="w-full border rounded px-3 py-2 text-sm" />
                                </div>
                            </div>
                        </div>
                        <div className="flex justify-end gap-2 pt-2">
                            <button type="button" onClick={resetForm}
                                className="px-4 py-2 text-sm text-gray-600 hover:text-black transition-colors">Cancel</button>
                            <button type="button" onClick={async () => {
                                try {
                                    let sessionData: Record<string, string> | undefined
                                    if (form.session_data) {
                                        try { sessionData = JSON.parse(form.session_data) } catch {}
                                    }
                                    const result = await api.admin.botAccounts.testLogin({
                                        platform: form.platform,
                                        platform_user: form.platform_user,
                                        platform_pass: form.platform_pass,
                                        session_data: sessionData,
                                    })
                                    alert(result.message)
                                } catch (err: any) {
                                    alert('Test failed: ' + err.message)
                                }
                            }}
                                className="px-4 py-2 text-sm text-blue-600 hover:text-blue-800 border border-blue-300 rounded transition-colors">
                                Test Login
                            </button>
                            <button type="submit" disabled={saving}
                                className="bg-blue-600 text-white px-4 py-2 rounded text-sm hover:bg-blue-700 disabled:opacity-50 transition-colors">
                                {saving ? 'Saving...' : editingId ? 'Update Bot Account' : 'Create Bot Account'}
                            </button>
                        </div>
                    </form>
                </div>
            )}

            <div className="border border-gray-200 rounded-lg overflow-hidden">
                <table className="w-full text-sm">
                    <thead className="bg-gray-50 text-gray-500 text-xs uppercase">
                        <tr>
                            <th className="px-4 py-3 text-left">Platform</th>
                            <th className="px-4 py-3 text-left">Username</th>
                            <th className="px-4 py-3 text-left">Status</th>
                            <th className="px-4 py-3 text-left">Rate Limit</th>
                            <th className="px-4 py-3 text-left">Last Used</th>
                            <th className="px-4 py-3 text-right">Actions</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-100">
                        {bots.map(b => (
                            <tr key={b.id}>
                                <td className="px-4 py-3">
                                    <span className="font-medium capitalize">{b.platform}</span>
                                </td>
                                <td className="px-4 py-3 font-mono text-xs">{b.platform_user || '—'}</td>
                                <td className="px-4 py-3">
                                    <button onClick={() => handleStatusToggle(b)}
                                        className={`px-2 py-1 rounded text-xs font-medium cursor-pointer hover:opacity-80 ${statusColor(b.status)}`}>
                                        {b.status}
                                    </button>
                                </td>
                                <td className="px-4 py-3 text-gray-500 text-xs">{b.rate_limit_rps} rps</td>
                                <td className="px-4 py-3 text-gray-500 text-xs">
                                    {b.last_used_at ? new Date(b.last_used_at).toLocaleString() : 'Never'}
                                </td>
                                <td className="px-4 py-3 text-right">
                                    <div className="flex justify-end gap-2">
                                        <button onClick={() => startEdit(b)} className="text-blue-600 hover:text-blue-800" title="Edit">
                                            <Pencil className="w-4 h-4" />
                                        </button>
                                        <button onClick={() => handleDelete(b.id)} className="text-red-600 hover:text-red-800" title="Delete">
                                            <Trash2 className="w-4 h-4" />
                                        </button>
                                    </div>
                                </td>
                            </tr>
                        ))}
                        {bots.length === 0 && (
                            <tr>
                                <td colSpan={6} className="px-4 py-8 text-center text-gray-400">
                                    No bot accounts configured. Click "Add Bot Account" to add one.
                                </td>
                            </tr>
                        )}
                    </tbody>
                </table>
            </div>
        </div>
    )
}
