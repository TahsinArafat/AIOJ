import { useEffect, useState } from 'react'
import { Plus, Pencil, Trash2, Zap } from 'lucide-react'
import { api } from '../../lib/api'

interface AIModel {
    id: string
    name: string
    endpoint: string
    api_key?: string
    model_name: string
    enabled: boolean
    priority: number
    description: string
    created_at: string
    updated_at: string
}

const emptyForm = {
    name: '',
    endpoint: '',
    api_key: '',
    model_name: '',
    enabled: true,
    priority: 0,
    description: '',
}

type TestState = 'idle' | 'testing' | 'ok' | 'fail'

export default function AIModelsPanel() {
    const [models, setModels] = useState<AIModel[]>([])
    const [loading, setLoading] = useState(true)
    const [showForm, setShowForm] = useState(false)
    const [editingId, setEditingId] = useState<string | null>(null)
    const [form, setForm] = useState({ ...emptyForm })
    const [saving, setSaving] = useState(false)
    const [testState, setTestState] = useState<TestState>('idle')
    const [testError, setTestError] = useState('')

    const loadModels = () => {
        setLoading(true)
        api.admin.aiModels.list()
            .then(d => setModels(d.data || []))
            .catch(console.error)
            .finally(() => setLoading(false))
    }

    useEffect(() => { loadModels() }, [])

    const openCreate = () => {
        setEditingId(null)
        setForm({ ...emptyForm })
        setTestState('idle')
        setTestError('')
        setShowForm(true)
    }

    const openEdit = async (m: AIModel) => {
        // Fetch full record (with api_key) before populating form
        const full = await api.admin.aiModels.getById(m.id).catch(() => m)
        setEditingId(m.id)
        setForm({
            name: full.name,
            endpoint: full.endpoint,
            api_key: '', // leave blank; server preserves existing on empty
            model_name: full.model_name,
            enabled: full.enabled,
            priority: full.priority,
            description: full.description,
        })
        setTestState('idle')
        setTestError('')
        setShowForm(true)
    }

    const handleCancel = () => {
        setShowForm(false)
        setEditingId(null)
        setForm({ ...emptyForm })
        setTestState('idle')
        setTestError('')
    }

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        if (!form.name || !form.endpoint || !form.model_name) return
        setSaving(true)
        try {
            if (editingId) {
                await api.admin.aiModels.update(editingId, form)
            } else {
                await api.admin.aiModels.create(form)
            }
            handleCancel()
            loadModels()
        } catch (err: any) {
            alert('Save failed: ' + (err.message || err))
        } finally {
            setSaving(false)
        }
    }

    const handleToggle = async (m: AIModel) => {
        try {
            await api.admin.aiModels.toggle(m.id)
            loadModels()
        } catch (err: any) {
            alert('Toggle failed: ' + (err.message || err))
        }
    }

    const handleDelete = async (m: AIModel) => {
        if (!confirm(`Delete AI model "${m.name}"? This cannot be undone.`)) return
        try {
            await api.admin.aiModels.delete(m.id)
            loadModels()
        } catch (err: any) {
            alert('Delete failed: ' + (err.message || err))
        }
    }

    const handleTestConnection = async () => {
        if (!form.endpoint || !form.model_name) {
            alert('Fill in Endpoint and Model Name before testing.')
            return
        }
        setTestState('testing')
        setTestError('')
        try {
            const res = await api.admin.aiModels.testConnection({
                endpoint: form.endpoint,
                api_key: form.api_key,
                model_name: form.model_name,
            })
            if (res.ok) {
                setTestState('ok')
            } else {
                setTestState('fail')
                setTestError(res.error || 'Unknown error')
            }
        } catch (err: any) {
            setTestState('fail')
            setTestError(err.message || 'Request failed')
        }
    }

    const field = (
        label: string,
        key: keyof typeof form,
        opts?: {
            type?: string
            required?: boolean
            placeholder?: string
            hint?: string
        }
    ) => (
        <div>
            <label className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">
                {label} {opts?.required && <span className="text-red-500">*</span>}
            </label>
            <input
                type={opts?.type || 'text'}
                value={form[key] as string}
                onChange={e => setForm(f => ({ ...f, [key]: e.target.value }))}
                placeholder={opts?.placeholder}
                required={opts?.required}
                className="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            {opts?.hint && (
                <p className="text-xs text-gray-400 dark:text-gray-500 mt-1">{opts.hint}</p>
            )}
        </div>
    )

    return (
        <div>
            {/* Header */}
            <div className="flex items-start justify-between mb-6">
                <div>
                    <h2 className="text-lg font-semibold text-gray-900 dark:text-gray-100">AI Models</h2>
                    <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                        Manage AI endpoints for problem generation. The highest-priority enabled model is used automatically.
                    </p>
                </div>
                <button
                    onClick={openCreate}
                    className="flex items-center gap-1.5 bg-blue-600 hover:bg-blue-700 text-white px-3 py-1.5 rounded-lg text-sm font-medium transition-colors cursor-pointer"
                >
                    <Plus className="w-4 h-4" /> Add Model
                </button>
            </div>

            {/* Add / Edit form */}
            {showForm && (
                <div className="bg-white dark:bg-gray-800 border border-blue-200 dark:border-blue-800 rounded-xl p-5 mb-6 shadow-sm">
                    <h3 className="font-semibold text-sm mb-4 text-gray-800 dark:text-gray-200">
                        {editingId ? 'Edit AI Model' : 'Add New AI Model'}
                    </h3>
                    <form onSubmit={handleSubmit} className="space-y-4">
                        {/* Basic info */}
                        <div className="grid grid-cols-2 gap-4">
                            {field('Display Name', 'name', { required: true, placeholder: 'My Fine-Tuned Model' })}
                            {field('Priority', 'priority', {
                                type: 'number',
                                hint: 'Higher = preferred when multiple models are enabled.',
                                placeholder: '0',
                            })}
                        </div>

                        {/* Endpoint section */}
                        <div className="border-t border-gray-100 dark:border-gray-700 pt-4">
                            <h4 className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase mb-3">Endpoint & Credentials</h4>
                            <div className="space-y-3">
                                {field('Endpoint URL', 'endpoint', {
                                    required: true,
                                    placeholder: 'http://localhost:8080/v1',
                                    hint: 'OpenAI-compatible base URL. Must serve POST /chat/completions.',
                                })}
                                <div className="grid grid-cols-2 gap-4">
                                    {field('API Key', 'api_key', {
                                        type: 'password',
                                        placeholder: editingId ? 'Leave blank to keep existing' : 'sk-… or empty for self-hosted',
                                    })}
                                    {field('Model Name', 'model_name', {
                                        required: true,
                                        placeholder: 'codeforces-generator-v1',
                                        hint: 'Exact model ID sent in each API request.',
                                    })}
                                </div>
                            </div>
                        </div>

                        {/* Config */}
                        <div className="border-t border-gray-100 dark:border-gray-700 pt-4">
                            <h4 className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase mb-3">Configuration</h4>
                            <div className="space-y-3">
                                <div>
                                    <label className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">Description</label>
                                    <textarea
                                        value={form.description}
                                        onChange={e => setForm(f => ({ ...f, description: e.target.value }))}
                                        placeholder="Optional notes about this model's strengths, training data, etc."
                                        rows={2}
                                        className="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none"
                                    />
                                </div>
                                <label className="flex items-center gap-3 cursor-pointer">
                                    <input
                                        type="checkbox"
                                        checked={form.enabled}
                                        onChange={e => setForm(f => ({ ...f, enabled: e.target.checked }))}
                                        className="accent-blue-600 w-4 h-4"
                                    />
                                    <span className="text-sm text-gray-700 dark:text-gray-300">Enabled (available for problem generation)</span>
                                </label>
                            </div>
                        </div>

                        {/* Test connection */}
                        <div className="border-t border-gray-100 dark:border-gray-700 pt-4 flex items-center gap-3">
                            <button
                                type="button"
                                onClick={handleTestConnection}
                                disabled={testState === 'testing'}
                                className="flex items-center gap-1.5 px-3 py-1.5 border border-gray-300 dark:border-gray-600 rounded-lg text-sm hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors cursor-pointer disabled:opacity-50"
                            >
                                <Zap className="w-3.5 h-3.5" />
                                {testState === 'testing' ? 'Testing…' : 'Test Connection'}
                            </button>
                            {testState === 'ok' && (
                                <span className="text-sm text-green-600 dark:text-green-400 font-medium">✓ Connection OK</span>
                            )}
                            {testState === 'fail' && (
                                <span className="text-sm text-red-500 dark:text-red-400">✗ {testError || 'Connection failed'}</span>
                            )}
                        </div>

                        {/* Actions */}
                        <div className="flex gap-2 pt-2">
                            <button
                                type="submit"
                                disabled={saving}
                                className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50 cursor-pointer"
                            >
                                {saving ? 'Saving…' : editingId ? 'Update Model' : 'Add Model'}
                            </button>
                            <button
                                type="button"
                                onClick={handleCancel}
                                className="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg text-sm hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors cursor-pointer"
                            >
                                Cancel
                            </button>
                        </div>
                    </form>
                </div>
            )}

            {/* Table */}
            {loading ? (
                <div className="text-center py-12 text-gray-400 dark:text-gray-500 text-sm">Loading…</div>
            ) : models.length === 0 ? (
                <div className="text-center py-16 text-gray-400 dark:text-gray-500">
                    <p className="text-sm">No AI models configured yet.</p>
                    <p className="text-xs mt-1">Add an OpenAI-compatible endpoint to start generating problems.</p>
                </div>
            ) : (
                <div className="border border-gray-200 dark:border-gray-700 rounded-xl overflow-hidden">
                    <table className="w-full text-sm">
                        <thead className="bg-gray-50 dark:bg-gray-800 text-gray-500 dark:text-gray-400 text-xs uppercase">
                            <tr>
                                <th className="px-4 py-3 text-left">Name</th>
                                <th className="px-4 py-3 text-left">Endpoint</th>
                                <th className="px-4 py-3 text-left">Model ID</th>
                                <th className="px-4 py-3 text-center">Priority</th>
                                <th className="px-4 py-3 text-center">Status</th>
                                <th className="px-4 py-3 text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-gray-100 dark:divide-gray-700">
                            {models.map(m => (
                                <tr key={m.id} className="hover:bg-gray-50 dark:hover:bg-gray-800/50">
                                    <td className="px-4 py-3">
                                        <p className="font-medium text-gray-900 dark:text-gray-100">{m.name}</p>
                                        {m.description && (
                                            <p className="text-xs text-gray-400 dark:text-gray-500 mt-0.5 max-w-xs truncate">{m.description}</p>
                                        )}
                                    </td>
                                    <td className="px-4 py-3 font-mono text-xs text-gray-600 dark:text-gray-300 max-w-[200px] truncate">
                                        {m.endpoint}
                                    </td>
                                    <td className="px-4 py-3 font-mono text-xs text-gray-600 dark:text-gray-300">
                                        {m.model_name}
                                    </td>
                                    <td className="px-4 py-3 text-center text-gray-600 dark:text-gray-300">
                                        {m.priority}
                                    </td>
                                    <td className="px-4 py-3 text-center">
                                        <button
                                            onClick={() => handleToggle(m)}
                                            className={`px-2.5 py-1 rounded-full text-xs font-medium cursor-pointer transition-colors ${
                                                m.enabled
                                                    ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 hover:bg-green-200 dark:hover:bg-green-900/50'
                                                    : 'bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-600'
                                            }`}
                                        >
                                            {m.enabled ? 'Enabled' : 'Disabled'}
                                        </button>
                                    </td>
                                    <td className="px-4 py-3">
                                        <div className="flex items-center justify-end gap-2">
                                            <button
                                                onClick={() => openEdit(m)}
                                                title="Edit"
                                                className="p-1.5 rounded-lg text-gray-400 hover:text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-colors cursor-pointer"
                                            >
                                                <Pencil className="w-4 h-4" />
                                            </button>
                                            <button
                                                onClick={() => handleDelete(m)}
                                                title="Delete"
                                                className="p-1.5 rounded-lg text-gray-400 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors cursor-pointer"
                                            >
                                                <Trash2 className="w-4 h-4" />
                                            </button>
                                        </div>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}

            <p className="text-xs text-gray-400 dark:text-gray-500 mt-4">
                The model with the <strong>highest priority</strong> that is <strong>enabled</strong> is automatically selected for each generation request.
                API keys are stored securely and never shown in the list view.
            </p>
        </div>
    )
}
