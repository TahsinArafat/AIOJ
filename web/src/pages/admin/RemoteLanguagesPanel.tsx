import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import { Plus, Pencil, Trash2, X, Save, Search, Check, AlertTriangle } from 'lucide-react'

interface RemoteLanguage {
    id: string
    platform: string
    local_id: string
    remote_id: string
    display_name: string
    enabled: boolean
    sort_order: number
    inline_comment_prefix: string
}

const PLATFORMS = [
    { value: 'codeforces', label: 'Codeforces' },
    { value: 'atcoder', label: 'AtCoder' },
    { value: 'cses', label: 'CSES' },
    { value: 'toph', label: 'Toph' },
    { value: 'qoj', label: 'QOJ' },
]

export default function RemoteLanguagesPanel() {
    const [platform, setPlatform] = useState('codeforces')
    const [langs, setLangs] = useState<RemoteLanguage[]>([])
    const [loading, setLoading] = useState(true)
    const [editingId, setEditingId] = useState<string | null>(null)
    const [showForm, setShowForm] = useState(false)
    const [form, setForm] = useState({ local_id: '', remote_id: '', display_name: '', enabled: true, sort_order: 0, inline_comment_prefix: '//' })
    const [detecting, setDetecting] = useState(false)
    const [detectResults, setDetectResults] = useState<{ matched: { remote_id: string; display_name: string; local_id: string }[]; unmatched: { remote_id: string; display_name: string }[] } | null>(null)
    const [showDetectModal, setShowDetectModal] = useState(false)
    const [selectedForSave, setSelectedForSave] = useState<Set<number>>(new Set())

    const loadLangs = () => {
        setLoading(true)
        api.admin.remoteLanguages.list(platform)
            .then(d => setLangs(d.data || []))
            .catch(console.error)
            .finally(() => setLoading(false))
    }

    useEffect(() => { loadLangs() }, [platform])

    const resetForm = () => {
        setForm({ local_id: '', remote_id: '', display_name: '', enabled: true, sort_order: 0, inline_comment_prefix: '//' })
        setEditingId(null)
        setShowForm(false)
    }

    const startEdit = (lang: RemoteLanguage) => {
        setEditingId(lang.id)
        setForm({
            local_id: lang.local_id,
            remote_id: lang.remote_id,
            display_name: lang.display_name,
            enabled: lang.enabled,
            sort_order: lang.sort_order,
            inline_comment_prefix: lang.inline_comment_prefix || '//',
        })
        setShowForm(true)
    }

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        try {
            if (editingId) {
                await api.admin.remoteLanguages.update(editingId, form)
            } else {
                await api.admin.remoteLanguages.create({ platform, ...form })
            }
            resetForm()
            loadLangs()
        } catch (err: any) {
            alert(err.message)
        }
    }

    const handleDelete = async (id: string) => {
        if (!confirm('Delete this language mapping?')) return
        try {
            await api.admin.remoteLanguages.delete(id)
            loadLangs()
        } catch (err: any) {
            alert(err.message)
        }
    }

    const handleToggle = async (lang: RemoteLanguage) => {
        try {
            await api.admin.remoteLanguages.update(lang.id, { enabled: !lang.enabled })
            loadLangs()
        } catch (err: any) {
            alert(err.message)
        }
    }

    const handleAutoDetect = async () => {
        setDetecting(true)
        try {
            const result = await api.admin.remoteLanguages.detect(platform)
            setDetectResults({ matched: result.matched, unmatched: result.unmatched })
            setSelectedForSave(new Set(result.matched.map((_, i) => i)))
            setShowDetectModal(true)
        } catch (err: any) {
            alert(err.message)
        } finally {
            setDetecting(false)
        }
    }

    const handleBulkSave = async () => {
        if (!detectResults) return
        const all = [
            ...detectResults.matched.map(m => ({ platform, local_id: m.local_id, remote_id: m.remote_id, display_name: m.display_name })),
            ...detectResults.unmatched.map(u => ({ platform, local_id: '', remote_id: u.remote_id, display_name: u.display_name })),
        ]
        const selected = all.filter((_, i) => selectedForSave.has(i))
        if (selected.length === 0) return
        try {
            await api.admin.remoteLanguages.bulkUpsert(
                selected.map((s, i) => ({ ...s, enabled: true, sort_order: i }))
            )
            loadLangs()
            setShowDetectModal(false)
            setDetectResults(null)
        } catch (err: any) {
            alert(err.message)
        }
    }

    const totalItems = detectResults ? detectResults.matched.length + detectResults.unmatched.length : 0

    return (
        <div>
            <div className="flex items-center justify-between mb-4">
                <div>
                    <h2 className="text-lg font-semibold">Remote OJ Languages</h2>
                    <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">Configure language mappings for remote OJ platforms</p>
                </div>
                <button onClick={() => { resetForm(); setShowForm(true) }}
                    className="flex items-center gap-1.5 bg-blue-600 text-white px-3 py-1.5 rounded text-sm hover:bg-blue-700 transition-colors">
                    <Plus className="w-4 h-4" /> Add Language
                </button>
                <button onClick={handleAutoDetect} disabled={detecting}
                    className="flex items-center gap-1.5 bg-gray-600 dark:bg-gray-600 text-white px-3 py-1.5 rounded text-sm hover:bg-gray-700 dark:hover:bg-gray-500 transition-colors disabled:opacity-50">
                    <Search className="w-4 h-4" /> {detecting ? 'Detecting...' : 'Auto-Detect'}
                </button>
            </div>

            <div className="flex gap-2 mb-4">
                {PLATFORMS.map(p => (
                    <button key={p.value} onClick={() => setPlatform(p.value)}
                        className={`px-3 py-1.5 rounded text-sm font-medium transition-colors ${platform === p.value ? 'bg-blue-600 text-white' : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'}`}>
                        {p.label}
                    </button>
                ))}
            </div>

            {showForm && (
                <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-5 mb-6">
                    <div className="flex items-center justify-between mb-4">
                        <h3 className="font-semibold text-sm">{editingId ? 'Edit Language' : 'Add Language'}</h3>
                        <button onClick={resetForm} className="text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300"><X className="w-4 h-4" /></button>
                    </div>
                    <form onSubmit={handleSubmit} className="grid grid-cols-6 gap-4 items-end">
                        <div>
                            <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Local ID</label>
                            <input value={form.local_id} onChange={e => setForm({ ...form, local_id: e.target.value })}
                                placeholder="e.g. cpp-gpp-64" required className="w-full border rounded px-3 py-2 text-sm" />
                        </div>
                        <div>
                            <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Remote ID</label>
                            <input value={form.remote_id} onChange={e => setForm({ ...form, remote_id: e.target.value })}
                                placeholder="e.g. 54" required className="w-full border rounded px-3 py-2 text-sm" />
                        </div>
                        <div>
                            <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Display Name</label>
                            <input value={form.display_name} onChange={e => setForm({ ...form, display_name: e.target.value })}
                                placeholder="e.g. GNU G++17" required className="w-full border rounded px-3 py-2 text-sm" />
                        </div>
                        <div>
                            <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Comment Prefix</label>
                            <input value={form.inline_comment_prefix} onChange={e => setForm({ ...form, inline_comment_prefix: e.target.value })}
                                placeholder="e.g. //" className="w-full border rounded px-3 py-2 text-sm font-mono" />
                        </div>
                        <div>
                            <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Sort Order</label>
                            <input type="number" value={form.sort_order} onChange={e => setForm({ ...form, sort_order: parseInt(e.target.value) || 0 })}
                                className="w-full border rounded px-3 py-2 text-sm" />
                        </div>
                        <div className="flex gap-2">
                            <button type="submit" className="bg-blue-600 text-white px-4 py-2 rounded text-sm hover:bg-blue-700">
                                <Save className="w-4 h-4" />
                            </button>
                            <button type="button" onClick={resetForm} className="px-4 py-2 text-sm text-gray-600 dark:text-gray-400 hover:text-black dark:hover:text-white">Cancel</button>
                        </div>
                    </form>
                </div>
            )}

            {loading ? (
                <div className="text-center py-8 text-gray-400 dark:text-gray-500">Loading...</div>
            ) : (
                <div className="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
                    <table className="w-full text-sm">
                        <thead className="bg-gray-50 dark:bg-gray-800 text-gray-500 dark:text-gray-400 text-xs uppercase">
                            <tr>
                                <th className="px-4 py-3 text-left">Local ID</th>
                                <th className="px-4 py-3 text-left">Remote ID</th>
                                <th className="px-4 py-3 text-left">Display Name</th>
                                <th className="px-4 py-3 text-left">Comment</th>
                                <th className="px-4 py-3 text-left">Order</th>
                                <th className="px-4 py-3 text-left">Status</th>
                                <th className="px-4 py-3 text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-gray-100 dark:divide-gray-700">
                            {langs.map(l => (
                                <tr key={l.id} className={!l.enabled ? 'bg-gray-50 dark:bg-gray-800 opacity-60' : ''}>
                                    <td className="px-4 py-3 font-mono text-xs">{l.local_id}</td>
                                    <td className="px-4 py-3 font-mono text-xs">{l.remote_id}</td>
                                    <td className="px-4 py-3">{l.display_name}</td>
                                    <td className="px-4 py-3 font-mono text-xs">{l.inline_comment_prefix || '//'}</td>
                                    <td className="px-4 py-3 text-gray-500 dark:text-gray-400">{l.sort_order}</td>
                                    <td className="px-4 py-3">
                                        <button onClick={() => handleToggle(l)}
                                            className={`px-2 py-0.5 rounded text-xs font-medium cursor-pointer ${l.enabled ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300' : 'bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400'}`}>
                                            {l.enabled ? 'Enabled' : 'Disabled'}
                                        </button>
                                    </td>
                                    <td className="px-4 py-3 text-right">
                                        <div className="flex justify-end gap-2">
                                            <button onClick={() => startEdit(l)} className="text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300"><Pencil className="w-4 h-4" /></button>
                                            <button onClick={() => handleDelete(l.id)} className="text-red-600 dark:text-red-400 hover:text-red-800 dark:hover:text-red-300"><Trash2 className="w-4 h-4" /></button>
                                        </div>
                                    </td>
                                </tr>
                            ))}
                            {langs.length === 0 && (
                                <tr><td colSpan={6} className="px-4 py-8 text-center text-gray-400 dark:text-gray-500">No language mappings configured</td></tr>
                            )}
                        </tbody>
                    </table>
                </div>
            )}

            {showDetectModal && detectResults && (
                <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
                    <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 w-full max-w-2xl max-h-[80vh] flex flex-col">
                        <div className="flex items-center justify-between px-5 py-4 border-b border-gray-200 dark:border-gray-700">
                            <div className="flex items-center gap-2">
                                <h3 className="font-semibold text-sm">Review Detected Languages</h3>
                                <span className="px-2 py-0.5 rounded text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300">
                                    {selectedForSave.size} of {totalItems} selected
                                </span>
                            </div>
                            <button onClick={() => { setShowDetectModal(false); setDetectResults(null) }}
                                className="text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300">
                                <X className="w-4 h-4" />
                            </button>
                        </div>
                        <div className="overflow-auto flex-1 px-5 py-3">
                            {detectResults.matched.length > 0 && (
                                <div className="mb-4">
                                    <h4 className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase mb-2 flex items-center gap-1.5">
                                        <Check className="w-3.5 h-3.5 text-green-500" /> Matched ({detectResults.matched.length})
                                    </h4>
                                    <table className="w-full text-sm">
                                        <thead className="text-xs text-gray-500 dark:text-gray-400 uppercase">
                                            <tr>
                                                <th className="px-3 py-2 text-left w-10"></th>
                                                <th className="px-3 py-2 text-left">Remote Name</th>
                                                <th className="px-3 py-2 text-left">Remote ID</th>
                                                <th className="px-3 py-2 text-left">Matched Local ID</th>
                                            </tr>
                                        </thead>
                                        <tbody className="divide-y divide-gray-100 dark:divide-gray-700">
                                            {detectResults.matched.map((m, i) => (
                                                <tr key={i}>
                                                    <td className="px-3 py-2">
                                                        <input type="checkbox" checked={selectedForSave.has(i)}
                                                            onChange={() => {
                                                                const next = new Set(selectedForSave)
                                                                next.has(i) ? next.delete(i) : next.add(i)
                                                                setSelectedForSave(next)
                                                            }}
                                                            className="rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500" />
                                                    </td>
                                                    <td className="px-3 py-2">{m.display_name}</td>
                                                    <td className="px-3 py-2 font-mono text-xs">{m.remote_id}</td>
                                                    <td className="px-3 py-2 font-mono text-xs text-green-600 dark:text-green-400">{m.local_id}</td>
                                                </tr>
                                            ))}
                                        </tbody>
                                    </table>
                                </div>
                            )}
                            {detectResults.unmatched.length > 0 && (
                                <div>
                                    <h4 className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase mb-2 flex items-center gap-1.5">
                                        <AlertTriangle className="w-3.5 h-3.5 text-amber-500" /> No Match ({detectResults.unmatched.length})
                                    </h4>
                                    <table className="w-full text-sm">
                                        <thead className="text-xs text-gray-500 dark:text-gray-400 uppercase">
                                            <tr>
                                                <th className="px-3 py-2 text-left w-10"></th>
                                                <th className="px-3 py-2 text-left">Remote Name</th>
                                                <th className="px-3 py-2 text-left">Remote ID</th>
                                                <th className="px-3 py-2 text-left">Matched Local ID</th>
                                            </tr>
                                        </thead>
                                        <tbody className="divide-y divide-gray-100 dark:divide-gray-700">
                                            {detectResults.unmatched.map((u, i) => {
                                                const idx = detectResults.matched.length + i
                                                return (
                                                    <tr key={idx}>
                                                        <td className="px-3 py-2">
                                                            <input type="checkbox" checked={selectedForSave.has(idx)}
                                                                onChange={() => {
                                                                    const next = new Set(selectedForSave)
                                                                    next.has(idx) ? next.delete(idx) : next.add(idx)
                                                                    setSelectedForSave(next)
                                                                }}
                                                                className="rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500" />
                                                        </td>
                                                        <td className="px-3 py-2">{u.display_name}</td>
                                                        <td className="px-3 py-2 font-mono text-xs">{u.remote_id}</td>
                                                        <td className="px-3 py-2 font-mono text-xs text-gray-400 dark:text-gray-500">No match</td>
                                                    </tr>
                                                )
                                            })}
                                        </tbody>
                                    </table>
                                </div>
                            )}
                            {detectResults.matched.length === 0 && detectResults.unmatched.length === 0 && (
                                <div className="text-center py-8 text-gray-400 dark:text-gray-500 text-sm">
                                    No languages detected for this platform.
                                </div>
                            )}
                        </div>
                        <div className="flex justify-end gap-2 px-5 py-4 border-t border-gray-200 dark:border-gray-700">
                            <button onClick={() => { setShowDetectModal(false); setDetectResults(null) }}
                                className="px-4 py-2 text-sm text-gray-600 dark:text-gray-400 hover:text-black dark:hover:text-white">
                                Cancel
                            </button>
                            <button onClick={handleBulkSave} disabled={selectedForSave.size === 0}
                                className="flex items-center gap-1.5 bg-green-600 text-white px-4 py-2 rounded text-sm hover:bg-green-700 transition-colors disabled:opacity-50">
                                <Save className="w-4 h-4" /> Save Selected
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}
