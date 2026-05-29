import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import { Save, RefreshCw, Play, Trash2, Plus, X, Code, Cpu, Zap } from 'lucide-react'

interface LanguageConfig {
    name: string
    key: string
    compile: string
    runtime: string
    copy_in: Record<string, string>
    time_limit_multiplier: number
    memory_limit_multiplier: number
    seccomp_rule: string
    extensions: string[]
    mono: boolean
    file_path: string
}

const emptyLang: LanguageConfig = {
    name: '',
    key: '',
    compile: '',
    runtime: '',
    copy_in: {},
    time_limit_multiplier: 1.0,
    memory_limit_multiplier: 1.0,
    seccomp_rule: 'general',
    extensions: [],
    mono: false,
    file_path: '',
}

const seccompOptions = ['general', 'c_cpp', 'java', 'node', 'none']

export default function LanguagesPanel() {
    const [langs, setLangs] = useState<LanguageConfig[]>([])
    const [loading, setLoading] = useState(true)
    const [editKey, setEditKey] = useState<string | null>(null)
    const [form, setForm] = useState<LanguageConfig>(emptyLang)
    const [rawYaml, setRawYaml] = useState('')
    const [useRawEditor, setUseRawEditor] = useState(false)
    const [saving, setSaving] = useState(false)
    const [testing, setTesting] = useState<string | null>(null)
    const [testResult, setTestResult] = useState<any>(null)
    const [showCreate, setShowCreate] = useState(false)
    const [extInput, setExtInput] = useState('')
    const [detected, setDetected] = useState<{ compilers: any[]; interpreters: any[] } | null>(null)
    const [templates, setTemplates] = useState<any[]>([])
    const [showDetect, setShowDetect] = useState(false)
    const [showTemplates, setShowTemplates] = useState(false)

    const loadLangs = () => {
        setLoading(true)
        api.admin.languages.list()
            .then(d => setLangs(d.data || []))
            .catch(console.error)
            .finally(() => setLoading(false))
    }

    useEffect(() => { loadLangs() }, [])

    const handleEdit = async (key: string) => {
        setEditKey(key)
        setTestResult(null)
        setUseRawEditor(false)
        try {
            const lang = await api.admin.languages.get(key)
            setForm(lang)
            const raw = await api.admin.languages.getRaw(key)
            setRawYaml(typeof raw === 'string' ? raw : JSON.stringify(raw, null, 2))
        } catch (e: any) {
            alert('Failed to load language: ' + e.message)
        }
    }

    const handleSave = async () => {
        if (!editKey) return
        setSaving(true)
        try {
            if (useRawEditor) {
                await api.admin.languages.updateRaw(editKey, rawYaml)
            } else {
                await api.admin.languages.update(editKey, form)
            }
            setEditKey(null)
            loadLangs()
        } catch (e: any) {
            alert('Save failed: ' + e.message)
        } finally {
            setSaving(false)
        }
    }

    const handleCreate = async () => {
        if (!form.key || !form.name) { alert('Key and Name are required'); return }
        setSaving(true)
        try {
            await api.admin.languages.create(form)
            setShowCreate(false)
            setForm(emptyLang)
            loadLangs()
        } catch (e: any) {
            alert('Create failed: ' + e.message)
        } finally {
            setSaving(false)
        }
    }

    const handleDelete = async (key: string) => {
        if (!confirm(`Delete language "${key}"? This cannot be undone.`)) return
        try {
            await api.admin.languages.delete(key)
            loadLangs()
        } catch (e: any) {
            alert('Delete failed: ' + e.message)
        }
    }

    const handleTest = async (key: string) => {
        setTesting(key)
        setTestResult(null)
        try {
            const result = await api.admin.languages.test(key)
            setTestResult(result)
        } catch (e: any) {
            setTestResult({ status: 'error', message: e.message })
        } finally {
            setTesting(null)
        }
    }

    const handleDetect = async () => {
        setShowDetect(true)
        setShowTemplates(false)
        try {
            const result = await api.admin.languages.detect()
            setDetected(result)
        } catch (e: any) {
            alert('Detection failed: ' + e.message)
        }
    }

    const handleLoadTemplates = async () => {
        setShowTemplates(true)
        setShowDetect(false)
        try {
            const result = await api.admin.languages.templates()
            setTemplates(result.data || [])
        } catch (e: any) {
            alert('Failed to load templates: ' + e.message)
        }
    }

    const useDetectedCompiler = (tool: any) => {
        setForm({ ...form, compile: `${tool.path} -O2 -o {{exe}} {{src}}` })
        setShowCreate(true)
        setEditKey(null)
    }

    const useDetectedInterpreter = (tool: any) => {
        setForm({ ...form, runtime: tool.path })
        setShowCreate(true)
        setEditKey(null)
    }

    const useTemplate = (tmpl: any) => {
        setForm(tmpl)
        setShowCreate(true)
        setShowTemplates(false)
        setEditKey(null)
    }

    const addExtension = () => {
        if (extInput && !form.extensions.includes(extInput)) {
            setForm({ ...form, extensions: [...form.extensions, extInput] })
            setExtInput('')
        }
    }

    const removeExtension = (ext: string) => {
        setForm({ ...form, extensions: form.extensions.filter(e => e !== ext) })
    }

    if (loading) {
        return <div className="text-center py-8 text-gray-400">Loading languages...</div>
    }

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h2 className="text-lg font-semibold text-gray-900">Judge Languages</h2>
                    <p className="text-sm text-gray-500 mt-1">Configure compile commands, runtime paths, and resource multipliers for each language.</p>
                </div>
                <div className="flex gap-2">
                    <button onClick={loadLangs} className="p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 rounded cursor-pointer" title="Refresh">
                        <RefreshCw size={16} />
                    </button>
                    <button onClick={handleDetect}
                        className="flex items-center gap-1.5 px-3 py-1.5 bg-green-600 text-white text-sm rounded hover:bg-green-700 cursor-pointer">
                        <Cpu size={14} /> Detect Tools
                    </button>
                    <button onClick={handleLoadTemplates}
                        className="flex items-center gap-1.5 px-3 py-1.5 bg-purple-600 text-white text-sm rounded hover:bg-purple-700 cursor-pointer">
                        <Zap size={14} /> Templates
                    </button>
                    <button onClick={() => { setForm(emptyLang); setShowCreate(true); setEditKey(null); setShowDetect(false); setShowTemplates(false) }}
                        className="flex items-center gap-1.5 px-3 py-1.5 bg-blue-600 text-white text-sm rounded hover:bg-blue-700 cursor-pointer">
                        <Plus size={14} /> Add Language
                    </button>
                </div>
            </div>

            {/* Edit/Create Modal */}
            {(editKey || showCreate) && (
                <div className="border border-blue-200 rounded-lg bg-blue-50/30 p-6 space-y-4">
                    <div className="flex items-center justify-between">
                        <h3 className="font-semibold text-gray-900">{showCreate ? 'Add New Language' : `Edit: ${editKey}`}</h3>
                        <div className="flex items-center gap-2">
                            {!showCreate && (
                                <button onClick={() => setUseRawEditor(!useRawEditor)}
                                    className="text-xs px-2 py-1 border border-gray-300 rounded hover:bg-gray-100 cursor-pointer">
                                    {useRawEditor ? 'Form Editor' : 'Raw YAML'}
                                </button>
                            )}
                            <button onClick={() => { setEditKey(null); setShowCreate(false) }} className="text-gray-400 hover:text-gray-600 cursor-pointer"><X size={16} /></button>
                        </div>
                    </div>

                    {useRawEditor && !showCreate ? (
                        <div>
                            <label className="block text-xs font-medium text-gray-500 uppercase mb-1">YAML Content</label>
                            <textarea value={rawYaml} onChange={e => setRawYaml(e.target.value)} rows={16}
                                className="w-full font-mono text-xs border border-gray-300 rounded p-3 bg-white focus:outline-none focus:ring-2 focus:ring-blue-500" />
                        </div>
                    ) : (
                        <div className="grid grid-cols-2 gap-4">
                            <div>
                                <label className="block text-xs font-medium text-gray-500 uppercase mb-1">Key *</label>
                                <input value={form.key} onChange={e => setForm({ ...form, key: e.target.value })} disabled={!!editKey}
                                    placeholder="cpp-gpp-64" className="w-full border border-gray-300 rounded px-3 py-2 text-sm disabled:bg-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500" />
                            </div>
                            <div>
                                <label className="block text-xs font-medium text-gray-500 uppercase mb-1">Display Name *</label>
                                <input value={form.name} onChange={e => setForm({ ...form, name: e.target.value })}
                                    placeholder="C++ (G++ 64-bit)" className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
                            </div>
                            <div className="col-span-2">
                                <label className="block text-xs font-medium text-gray-500 uppercase mb-1">Compile Command</label>
                                <div className="flex gap-2">
                                    <input value={form.compile} onChange={e => setForm({ ...form, compile: e.target.value })}
                                        placeholder="/usr/bin/g++ -O2 -std=c++17 -o {{exe}} {{src}}" className="flex-1 font-mono text-xs border border-gray-300 rounded px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500" />
                                </div>
                                <p className="text-xs text-gray-400 mt-1">{'Use {{exe}} for output binary, {{src}} for source file, {{dir}} for working directory.'}</p>
                            </div>
                            <div className="col-span-2">
                                <label className="block text-xs font-medium text-gray-500 uppercase mb-1">Runtime Command</label>
                                <div className="flex gap-2">
                                    <input value={form.runtime} onChange={e => setForm({ ...form, runtime: e.target.value })}
                                        placeholder="/usr/bin/python3" className="flex-1 font-mono text-xs border border-gray-300 rounded px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500" />
                                </div>
                                <p className="text-xs text-gray-400 mt-1">Leave empty for compiled languages. For interpreted languages, set the interpreter path.</p>
                            </div>
                            <div>
                                <label className="block text-xs font-medium text-gray-500 uppercase mb-1">Time Limit Multiplier</label>
                                <input type="number" step="0.1" min="0.1" value={form.time_limit_multiplier}
                                    onChange={e => setForm({ ...form, time_limit_multiplier: parseFloat(e.target.value) || 1.0 })}
                                    className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
                                <p className="text-xs text-gray-400 mt-1">Python=3.0, Java=2.0, C++=1.0</p>
                            </div>
                            <div>
                                <label className="block text-xs font-medium text-gray-500 uppercase mb-1">Memory Limit Multiplier</label>
                                <input type="number" step="0.1" min="0.1" value={form.memory_limit_multiplier}
                                    onChange={e => setForm({ ...form, memory_limit_multiplier: parseFloat(e.target.value) || 1.0 })}
                                    className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
                                <p className="text-xs text-gray-400 mt-1">Python=2.0, Java=2.0, C++=1.0</p>
                            </div>
                            <div>
                                <label className="block text-xs font-medium text-gray-500 uppercase mb-1">Seccomp Rule</label>
                                <select value={form.seccomp_rule} onChange={e => setForm({ ...form, seccomp_rule: e.target.value })}
                                    className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500">
                                    {seccompOptions.map(s => <option key={s} value={s}>{s}</option>)}
                                </select>
                            </div>
                            <div>
                                <label className="flex items-center gap-2 text-sm mt-5">
                                    <input type="checkbox" checked={form.mono} onChange={e => setForm({ ...form, mono: e.target.checked })}
                                        className="rounded" />
                                    <span className="text-gray-700">Mono (C#)</span>
                                </label>
                            </div>
                            <div className="col-span-2">
                                <label className="block text-xs font-medium text-gray-500 uppercase mb-1">File Extensions</label>
                                <div className="flex gap-2 mb-2">
                                    <input value={extInput} onChange={e => setExtInput(e.target.value)} onKeyDown={e => e.key === 'Enter' && (e.preventDefault(), addExtension())}
                                        placeholder=".cpp" className="flex-1 border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
                                    <button onClick={addExtension} className="px-3 py-2 bg-gray-100 rounded text-sm hover:bg-gray-200 cursor-pointer">Add</button>
                                </div>
                                <div className="flex flex-wrap gap-1">
                                    {form.extensions.map(ext => (
                                        <span key={ext} className="inline-flex items-center gap-1 bg-gray-100 text-gray-700 text-xs px-2 py-1 rounded">
                                            {ext}
                                            <button onClick={() => removeExtension(ext)} className="text-gray-400 hover:text-red-500 cursor-pointer"><X size={10} /></button>
                                        </span>
                                    ))}
                                </div>
                            </div>
                        </div>
                    )}

                    <div className="flex justify-end gap-2 pt-2">
                        <button onClick={() => { setEditKey(null); setShowCreate(false) }}
                            className="px-4 py-2 text-sm border border-gray-300 rounded hover:bg-gray-50 cursor-pointer">Cancel</button>
                        <button onClick={showCreate ? handleCreate : handleSave} disabled={saving}
                            className="flex items-center gap-1.5 px-4 py-2 text-sm bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50 cursor-pointer">
                            <Save size={14} /> {saving ? 'Saving...' : 'Save'}
                        </button>
                    </div>
                </div>
            )}

            {/* Detected Tools Panel */}
            {showDetect && detected && (
                <div className="border border-green-200 rounded-lg bg-green-50/30 p-6 space-y-4">
                    <div className="flex items-center justify-between">
                        <h3 className="font-semibold text-gray-900 flex items-center gap-2"><Cpu size={16} /> Detected Compilers & Interpreters</h3>
                        <button onClick={() => setShowDetect(false)} className="text-gray-400 hover:text-gray-600 cursor-pointer"><X size={16} /></button>
                    </div>
                    <p className="text-sm text-gray-500">Tools found on the server. Click "Use" to auto-fill the compile/runtime field in the form.</p>

                    <div className="grid grid-cols-2 gap-4">
                        <div>
                            <h4 className="font-medium text-sm text-gray-700 mb-2">Compilers</h4>
                            <div className="space-y-2 max-h-48 overflow-y-auto">
                                {detected.compilers.map((c: any, i: number) => (
                                    <div key={i} className="flex items-center justify-between bg-white border border-gray-200 rounded px-3 py-2">
                                        <div>
                                            <div className="text-sm font-medium">{c.name}</div>
                                            <div className="text-xs text-gray-500 font-mono">{c.path}</div>
                                            {c.version && <div className="text-xs text-gray-400 mt-0.5">{c.version}</div>}
                                        </div>
                                        <button onClick={() => useDetectedCompiler(c)}
                                            className="text-xs px-2 py-1 bg-blue-100 text-blue-700 rounded hover:bg-blue-200 cursor-pointer">Use</button>
                                    </div>
                                ))}
                                {detected.compilers.length === 0 && <p className="text-xs text-gray-400">No compilers detected</p>}
                            </div>
                        </div>
                        <div>
                            <h4 className="font-medium text-sm text-gray-700 mb-2">Interpreters</h4>
                            <div className="space-y-2 max-h-48 overflow-y-auto">
                                {detected.interpreters.map((i: any, idx: number) => (
                                    <div key={idx} className="flex items-center justify-between bg-white border border-gray-200 rounded px-3 py-2">
                                        <div>
                                            <div className="text-sm font-medium">{i.name}</div>
                                            <div className="text-xs text-gray-500 font-mono">{i.path}</div>
                                            {i.version && <div className="text-xs text-gray-400 mt-0.5">{i.version}</div>}
                                        </div>
                                        <button onClick={() => useDetectedInterpreter(i)}
                                            className="text-xs px-2 py-1 bg-blue-100 text-blue-700 rounded hover:bg-blue-200 cursor-pointer">Use</button>
                                    </div>
                                ))}
                                {detected.interpreters.length === 0 && <p className="text-xs text-gray-400">No interpreters detected</p>}
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {/* Templates Panel */}
            {showTemplates && (
                <div className="border border-purple-200 rounded-lg bg-purple-50/30 p-6 space-y-4">
                    <div className="flex items-center justify-between">
                        <h3 className="font-semibold text-gray-900 flex items-center gap-2"><Zap size={16} /> Language Templates</h3>
                        <button onClick={() => setShowTemplates(false)} className="text-gray-400 hover:text-gray-600 cursor-pointer"><X size={16} /></button>
                    </div>
                    <p className="text-sm text-gray-500">Quick-start templates for common languages. Click "Use" to pre-fill the create form.</p>

                    <div className="grid grid-cols-3 gap-3">
                        {templates.map((t: any) => (
                            <div key={t.key} className="bg-white border border-gray-200 rounded-lg p-3 hover:border-purple-300 transition-colors">
                                <div className="font-medium text-sm">{t.name}</div>
                                <div className="text-xs text-gray-500 font-mono mt-1">{t.key}</div>
                                <div className="text-xs text-gray-400 mt-1">
                                    {t.compile ? `compile: ${t.compile.substring(0, 30)}...` : `runtime: ${t.runtime}`}
                                </div>
                                <button onClick={() => useTemplate(t)}
                                    className="mt-2 text-xs px-2 py-1 bg-purple-100 text-purple-700 rounded hover:bg-purple-200 cursor-pointer w-full">Use Template</button>
                            </div>
                        ))}
                    </div>
                </div>
            )}

            {/* Test Result */}
            {testResult && (
                <div className={`border rounded-lg p-4 ${testResult.status === 'error' ? 'border-red-200 bg-red-50' : testResult.status === 'warning' ? 'border-yellow-200 bg-yellow-50' : 'border-green-200 bg-green-50'}`}>
                    <div className="flex items-center justify-between mb-2">
                        <span className="font-medium text-sm">{testResult.key}</span>
                        <button onClick={() => setTestResult(null)} className="text-gray-400 hover:text-gray-600 cursor-pointer"><X size={14} /></button>
                    </div>
                    <pre className="text-xs font-mono text-gray-700 whitespace-pre-wrap">{JSON.stringify(testResult, null, 2)}</pre>
                </div>
            )}

            {/* Language List */}
            <div className="border border-gray-200 rounded-lg overflow-hidden">
                <table className="w-full text-sm">
                    <thead className="bg-gray-50 text-gray-500 text-xs uppercase">
                        <tr>
                            <th className="px-4 py-2 text-left">Language</th>
                            <th className="px-4 py-2 text-left">Key</th>
                            <th className="px-4 py-2 text-left">Compile Command</th>
                            <th className="px-4 py-2 text-left">Runtime</th>
                            <th className="px-4 py-2 text-center">Time ×</th>
                            <th className="px-4 py-2 text-center">Memory ×</th>
                            <th className="px-4 py-2 text-right">Actions</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-100">
                        {langs.map(lang => (
                            <tr key={lang.key} className="hover:bg-gray-50">
                                <td className="px-4 py-2 font-medium">{lang.name}</td>
                                <td className="px-4 py-2 font-mono text-xs text-gray-600">{lang.key}</td>
                                <td className="px-4 py-2 font-mono text-xs text-gray-500 max-w-xs truncate">{lang.compile || '—'}</td>
                                <td className="px-4 py-2 font-mono text-xs text-gray-500 max-w-xs truncate">{lang.runtime || '—'}</td>
                                <td className="px-4 py-2 text-center text-xs">{lang.time_limit_multiplier}×</td>
                                <td className="px-4 py-2 text-center text-xs">{lang.memory_limit_multiplier}×</td>
                                <td className="px-4 py-2 text-right">
                                    <div className="flex items-center justify-end gap-1">
                                        <button onClick={() => handleTest(lang.key)} disabled={testing === lang.key}
                                            className="p-1.5 text-gray-400 hover:text-green-600 hover:bg-green-50 rounded cursor-pointer disabled:opacity-50" title="Test Config">
                                            <Play size={14} />
                                        </button>
                                        <button onClick={() => handleEdit(lang.key)}
                                            className="p-1.5 text-gray-400 hover:text-blue-600 hover:bg-blue-50 rounded cursor-pointer" title="Edit">
                                            <Code size={14} />
                                        </button>
                                        <button onClick={() => handleDelete(lang.key)}
                                            className="p-1.5 text-gray-400 hover:text-red-600 hover:bg-red-50 rounded cursor-pointer" title="Delete">
                                            <Trash2 size={14} />
                                        </button>
                                    </div>
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>
        </div>
    )
}
