import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import { Save, RefreshCw } from 'lucide-react'

interface Setting {
    key: string
    value: any
    description: string
    updated_at: string
    updated_by: string | null
}

const settingGroups: Record<string, { label: string; icon: string }> = {
    'vjudge': { label: 'VJudge Configuration', icon: '🤖' },
    'site': { label: 'Site Settings', icon: '⚙️' },
    'captcha': { label: 'CAPTCHA Configuration', icon: '🔐' },
}

const settingMeta: Record<string, { label: string; type: 'text' | 'number' | 'boolean' | 'select'; options?: string[] }> = {
    'vjudge.submit_timeout': { label: 'Submit Timeout (seconds)', type: 'number' },
    'vjudge.poll_interval': { label: 'Poll Interval (seconds)', type: 'number' },
    'vjudge.max_retries': { label: 'Max Retries', type: 'number' },
    'vjudge.rate_limit_rps': { label: 'Default Rate Limit (req/sec)', type: 'number' },
    'captcha.provider': { label: 'CAPTCHA Provider', type: 'select', options: ['none', 'deathbycaptcha', '2captcha', 'flaresolverr'] },
    'captcha.api_key': { label: 'CAPTCHA API Key', type: 'text' },
    'platform.maintenance_mode': { label: 'Maintenance Mode', type: 'boolean' },
    'platform.registration_enabled': { label: 'Allow New Registrations', type: 'boolean' },
}

function getGroup(key: string): string {
    const prefix = key.split('.')[0]
    return settingGroups[prefix]?.label || 'Other'
}

function getGroupIcon(key: string): string {
    const prefix = key.split('.')[0]
    return settingGroups[prefix]?.icon || '⚙️'
}

function getMeta(key: string) {
    return settingMeta[key] || { label: key, type: 'text' }
}

export default function SystemSettingsPanel() {
    const [settings, setSettings] = useState<Setting[]>([])
    const [loading, setLoading] = useState(true)
    const [savingKey, setSavingKey] = useState<string | null>(null)
    const [localValues, setLocalValues] = useState<Record<string, any>>({})

    const loadSettings = () => {
        setLoading(true)
        api.admin.settings.list()
            .then(d => {
                const items = d.data || []
                setSettings(items)
                const vals: Record<string, any> = {}
                items.forEach((s: Setting) => { vals[s.key] = s.value })
                setLocalValues(vals)
            })
            .catch(console.error)
            .finally(() => setLoading(false))
    }

    useEffect(() => { loadSettings() }, [])

    const updateLocal = (key: string, value: any) => {
        setLocalValues(prev => ({ ...prev, [key]: value }))
    }

    const handleSave = async (key: string) => {
        setSavingKey(key)
        try {
            const value = localValues[key]
            await api.admin.settings.update(key, value)
            loadSettings()
        } catch (err: any) {
            alert(err.message)
        } finally {
            setSavingKey(null)
        }
    }

    const groups = new Map<string, Setting[]>()
    settings.forEach(s => {
        const group = getGroup(s.key)
        if (!groups.has(group)) groups.set(group, [])
        groups.get(group)!.push(s)
    })

    if (loading) return <div className="text-center py-8 text-gray-400">Loading settings...</div>

    return (
        <div>
            <div className="flex items-center justify-between mb-6">
                <div>
                    <h2 className="text-lg font-semibold">System Settings</h2>
                    <p className="text-sm text-gray-500 mt-1">Configure platform-wide settings and VJudge behavior</p>
                </div>
                <button onClick={loadSettings} className="flex items-center gap-1.5 text-sm text-gray-600 hover:text-gray-900">
                    <RefreshCw className="w-4 h-4" /> Refresh
                </button>
            </div>

            <div className="space-y-6">
                {[...groups.entries()].map(([groupName, groupSettings]) => {
                    const firstKey = groupSettings[0].key
                    const icon = getGroupIcon(firstKey)
                    return (
                        <div key={groupName} className="bg-white border border-gray-200 rounded-lg overflow-hidden">
                            <div className="bg-gray-50 border-b border-gray-200 px-4 py-3 flex items-center gap-2">
                                <span>{icon}</span>
                                <h3 className="font-semibold text-sm text-gray-700">{groupName}</h3>
                            </div>
                            <div className="divide-y divide-gray-100">
                                {groupSettings.map(s => {
                                    const meta = getMeta(s.key)
                                    const localVal = localValues[s.key] ?? s.value
                                    const isDirty = JSON.stringify(localVal) !== JSON.stringify(s.value)

                                    return (
                                        <div key={s.key} className="px-4 py-3 flex items-center gap-4">
                                            <div className="flex-1 min-w-0">
                                                <div className="text-sm font-medium text-gray-900">{meta.label}</div>
                                                <div className="text-xs text-gray-500 mt-0.5">{s.description}</div>
                                            </div>
                                            <div className="flex items-center gap-3 flex-shrink-0">
                                                {meta.type === 'boolean' ? (
                                                    <button
                                                        onClick={() => updateLocal(s.key, !localVal)}
                                                        className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                                                            localVal ? 'bg-blue-600' : 'bg-gray-300'
                                                        }`}
                                                    >
                                                        <span className={`inline-block h-4 w-4 rounded-full bg-white transition-transform ${
                                                            localVal ? 'translate-x-6' : 'translate-x-1'
                                                        }`} />
                                                    </button>
                                                ) : meta.type === 'select' ? (
                                                    <select
                                                        value={String(localVal)}
                                                        onChange={e => updateLocal(s.key, e.target.value)}
                                                        className="border rounded px-2 py-1.5 text-sm bg-white w-48"
                                                    >
                                                        {meta.options?.map(opt => (
                                                            <option key={opt} value={opt}>{opt}</option>
                                                        ))}
                                                    </select>
                                                ) : meta.type === 'number' ? (
                                                    <input
                                                        type="number"
                                                        value={String(localVal)}
                                                        onChange={e => updateLocal(s.key, Number(e.target.value))}
                                                        className="border rounded px-2 py-1.5 text-sm w-24 text-right"
                                                    />
                                                ) : (
                                                    <input
                                                        type="text"
                                                        value={String(localVal)}
                                                        onChange={e => updateLocal(s.key, e.target.value)}
                                                        className="border rounded px-2 py-1.5 text-sm w-48"
                                                    />
                                                )}
                                                {isDirty && (
                                                    <button
                                                        onClick={() => handleSave(s.key)}
                                                        disabled={savingKey === s.key}
                                                        className="flex items-center gap-1 bg-blue-600 text-white px-3 py-1.5 rounded text-xs hover:bg-blue-700 disabled:opacity-50"
                                                    >
                                                        <Save className="w-3 h-3" />
                                                        {savingKey === s.key ? 'Saving...' : 'Save'}
                                                    </button>
                                                )}
                                            </div>
                                        </div>
                                    )
                                })}
                            </div>
                        </div>
                    )
                })}
                {settings.length === 0 && (
                    <div className="text-center py-8 text-gray-400">No system settings configured.</div>
                )}
            </div>
        </div>
    )
}
