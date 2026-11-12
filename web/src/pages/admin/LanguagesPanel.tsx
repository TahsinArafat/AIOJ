import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import { Save, RefreshCw, Play, Trash2, Plus, X, Code, Cpu, Zap, RotateCcw } from 'lucide-react'
import { invalidateLanguages } from '../../lib/useLanguages'
import { useConfirm } from '../../components/ConfirmDialog'
import { useToast } from '../../components/Toast'

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

interface DetectedTool { name: string; path: string; version?: string }
interface TestResult { status?: string; key?: string; message?: string }

export default function LanguagesPanel() {
    const confirm = useConfirm()
    const toast = useToast()
    const [langs, setLangs] = useState<LanguageConfig[]>([])
    const [loading, setLoading] = useState(true)
    const [editKey, setEditKey] = useState<string | null>(null)
    const [form, setForm] = useState<LanguageConfig>(emptyLang)
    const [rawYaml, setRawYaml] = useState('')
    const [useRawEditor, setUseRawEditor] = useState(false)
    const [saving, setSaving] = useState(false)
    const [testing, setTesting] = useState<string | null>(null)
    const [testResult, setTestResult] = useState<TestResult | null>(null)
    const [showCreate, setShowCreate] = useState(false)
    const [extInput, setExtInput] = useState('')
    const [detected, setDetected] = useState<{ compilers: DetectedTool[]; interpreters: DetectedTool[] } | null>(null)
    const [templates, setTemplates] = useState<LanguageConfig[]>([])
    const [showDetect, setShowDetect] = useState(false)
    const [detecting, setDetecting] = useState(false)
    const [showTemplates, setShowTemplates] = useState(false)

    const loadLangs = () => {
        setLoading(true)
        api.admin.languages.list()
            .then(d => setLangs(d.data || []))
            .catch(console.error)
            .finally(() => setLoading(false))
    }

    // deferred one microtask: loadLangs opens with a synchronous setLoading(true),
    // which react-hooks/set-state-in-effect rejects in an effect body
    useEffect(() => { queueMicrotask(loadLangs) }, [])

    const handleEdit = async (key: string) => {
        setEditKey(key)
        setTestResult(null)
        setUseRawEditor(false)
        try {
            const lang = await api.admin.languages.get(key)
            setForm(lang)
            const raw = await api.admin.languages.getRaw(key)
            setRawYaml(typeof raw === 'string' ? raw : JSON.stringify(raw, null, 2))
        } catch (e) {
            toast.error('Failed to load language: ' + (e instanceof Error ? e.message : String(e)))
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
            invalidateLanguages()
            setEditKey(null)
            loadLangs()
        } catch (e) {
            toast.error('Save failed: ' + (e instanceof Error ? e.message : String(e)))
        } finally {
            setSaving(false)
        }
    }

    const handleCreate = async () => {
        if (!form.key || !form.name) { toast.info('Key and Name are required'); return }
        setSaving(true)
        try {
            await api.admin.languages.create(form)
            invalidateLanguages()
            setShowCreate(false)
            setForm(emptyLang)
            loadLangs()
        } catch (e) {
            toast.error('Create failed: ' + (e instanceof Error ? e.message : String(e)))
        } finally {
            setSaving(false)
        }
    }

    const handleDelete = async (key: string) => {
        if (!(await confirm({ message: `Delete language "${key}"? This cannot be undone.`, variant: 'danger' }))) return
        try {
            await api.admin.languages.delete(key)
            invalidateLanguages()
            loadLangs()
        } catch (e) {
            toast.error('Delete failed: ' + (e instanceof Error ? e.message : String(e)))
        }
    }

    // Restores lang/<key>.yaml to the config that ships with the repo. The
    // panel edits those files in place, so a bad save (or a hand edit) could
    // otherwise only be undone by a redeploy.
    const handleReset = async (key: string) => {
        if (!(await confirm({
            message: `Reset "${key}" to the configuration shipped with the app? Local edits to this language will be lost.`,
            variant: 'danger',
        }))) return
        setSaving(true)
        try {
            await api.admin.languages.reset(key)
            invalidateLanguages()
            await loadLangs()
        } catch (e) {
            // A language an operator created has no shipped default; the server
            // answers 404 with that explanation, which is worth showing as-is.
            toast.error('Reset failed: ' + (e instanceof Error ? e.message : String(e)))
        } finally {
            setSaving(false)
        }
    }

    const handleTest = async (key: string) => {
        setTesting(key)
        setTestResult(null)
        try {
            const result = await api.admin.languages.test(key)
            setTestResult(result)
        } catch (e) {
            setTestResult({ status: 'error', message: e instanceof Error ? e.message : String(e) })
        } finally {
            setTesting(null)
        }
    }

    const handleDetect = async () => {
        setShowDetect(true)
        setShowTemplates(false)
        setDetecting(true)
        try {
            const result = await api.admin.languages.detect()
            // Normalise before rendering: the panel calls .map() over both lists,
            // so a response missing a key (or carrying JSON null) blanked the
            // whole admin page. An absent list here means "none found".
            setDetected({
                compilers: Array.isArray(result?.compilers) ? result.compilers : [],
                interpreters: Array.isArray(result?.interpreters) ? result.interpreters : [],
            })
        } catch (e) {
            setShowDetect(false)
            toast.error('Detection failed: ' + (e instanceof Error ? e.message : String(e)))
        } finally {
            setDetecting(false)
        }
    }

    const handleLoadTemplates = async () => {
        setShowTemplates(true)
        setShowDetect(false)
        try {
            const result = await api.admin.languages.templates()
            setTemplates(result.data || [])
        } catch (e) {
            toast.error('Failed to load templates: ' + (e instanceof Error ? e.message : String(e)))
        }
    }

    const applyDetectedCompiler = (tool: DetectedTool) => {
        setForm({ ...form, compile: `${tool.path} -O2 -o {{exe}} {{src}}` })
        setShowCreate(true)
        setEditKey(null)
    }

    const applyDetectedInterpreter = (tool: DetectedTool) => {
        setForm({ ...form, runtime: tool.path })
        setShowCreate(true)
        setEditKey(null)
    }

    const applyTemplate = (tmpl: LanguageConfig) => {
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
        return <div className="text-center py-8 text-muted-soft">Loading languages...</div>
    }

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h2 className="text-lg font-semibold text-ink">Judge Languages</h2>
                    <p className="text-sm text-muted mt-1">Configure compile commands, runtime paths, and resource multipliers for each language.</p>
                </div>
                <div className="flex gap-2">
                    <button onClick={loadLangs} className="p-2 text-muted hover:text-ink hover:bg-surface-2 rounded cursor-pointer" title="Refresh">
                        <RefreshCw size={16} />
                    </button>
                    <button onClick={handleDetect}
                        className="flex items-center gap-1.5 px-3 py-1.5 bg-ok-fill text-white text-sm rounded hover:bg-green-700 cursor-pointer">
                        <Cpu size={14} /> Detect Tools
                    </button>
                    <button onClick={handleLoadTemplates}
                        className="flex items-center gap-1.5 px-3 py-1.5 bg-purple-600 text-white text-sm rounded hover:bg-purple-700 cursor-pointer">
                        <Zap size={14} /> Templates
                    </button>
                    <button onClick={() => { setForm(emptyLang); setShowCreate(true); setEditKey(null); setShowDetect(false); setShowTemplates(false) }}
                        className="flex items-center gap-1.5 px-3 py-1.5 bg-brand text-on-brand text-sm rounded hover:bg-brand-hover cursor-pointer">
                        <Plus size={14} /> Add Language
                    </button>
                </div>
            </div>

            {/* Edit/Create Modal */}
            {(editKey || showCreate) && (
                <div className="border border-blue-200 dark:border-blue-800 rounded-lg bg-brand-soft/30 p-6 space-y-4">
                    <div className="flex items-center justify-between">
                        <h3 className="font-semibold text-ink">{showCreate ? 'Add New Language' : `Edit: ${editKey}`}</h3>
                        <div className="flex items-center gap-2">
                            {!showCreate && (
                                <button onClick={() => setUseRawEditor(!useRawEditor)}
                                    className="text-xs px-2 py-1 border border-hairline-strong rounded hover:bg-surface-2 cursor-pointer">
                                    {useRawEditor ? 'Form Editor' : 'Raw YAML'}
                                </button>
                            )}
                            <button onClick={() => { setEditKey(null); setShowCreate(false) }} className="text-muted-soft hover:text-ink cursor-pointer"><X size={16} /></button>
                        </div>
                    </div>

                    {useRawEditor && !showCreate ? (
                        <div>
                            <label className="block text-xs font-medium text-muted uppercase mb-1">YAML Content</label>
                            <textarea aria-label="YAML Content" value={rawYaml} onChange={e => setRawYaml(e.target.value)} rows={16}
                                className="w-full font-mono text-xs border border-hairline-strong rounded p-3 bg-surface focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]" />
                        </div>
                    ) : (
                        <div className="grid grid-cols-2 gap-4">
                            <div>
                                <label className="block text-xs font-medium text-muted uppercase mb-1">Key *</label>
                                <input aria-label="Key" value={form.key} onChange={e => setForm({ ...form, key: e.target.value })} disabled={!!editKey}
                                    placeholder="cpp-gpp-64" className="w-full border border-hairline-strong rounded px-3 py-2 text-sm disabled:bg-gray-100 focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]" />
                            </div>
                            <div>
                                <label className="block text-xs font-medium text-muted uppercase mb-1">Display Name *</label>
                                <input aria-label="Display Name" value={form.name} onChange={e => setForm({ ...form, name: e.target.value })}
                                    placeholder="C++ (G++ 64-bit)" className="w-full border border-hairline-strong rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]" />
                            </div>
                            <div className="col-span-2">
                                <label className="block text-xs font-medium text-muted uppercase mb-1">Compile Command</label>
                                <div className="flex gap-2">
                                    <input value={form.compile} onChange={e => setForm({ ...form, compile: e.target.value })}
                                        placeholder="/usr/bin/g++ -O2 -std=c++17 -o {{exe}} {{src}}" className="flex-1 font-mono text-xs border border-hairline-strong rounded px-3 py-2 focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]" />
                                </div>
                                <p className="text-xs text-muted-soft mt-1">{'Use {{exe}} for output binary, {{src}} for source file, {{dir}} for working directory.'}</p>
                            </div>
                            <div className="col-span-2">
                                <label className="block text-xs font-medium text-muted uppercase mb-1">Runtime Command</label>
                                <div className="flex gap-2">
                                    <input value={form.runtime} onChange={e => setForm({ ...form, runtime: e.target.value })}
                                        placeholder="/usr/bin/python3" className="flex-1 font-mono text-xs border border-hairline-strong rounded px-3 py-2 focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]" />
                                </div>
                                <p className="text-xs text-muted-soft mt-1">Leave empty for compiled languages. For interpreted languages, set the interpreter path.</p>
                            </div>
                            <div>
                                <label className="block text-xs font-medium text-muted uppercase mb-1">Time Limit Multiplier</label>
                                <input aria-label="Time Limit Multiplier" type="number" step="0.1" min="0.1" value={form.time_limit_multiplier}
                                    onChange={e => setForm({ ...form, time_limit_multiplier: parseFloat(e.target.value) || 1.0 })}
                                    className="w-full border border-hairline-strong rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]" />
                                <p className="text-xs text-muted-soft mt-1">Python=3.0, Java=2.0, C++=1.0</p>
                            </div>
                            <div>
                                <label className="block text-xs font-medium text-muted uppercase mb-1">Memory Limit Multiplier</label>
                                <input aria-label="Memory Limit Multiplier" type="number" step="0.1" min="0.1" value={form.memory_limit_multiplier}
                                    onChange={e => setForm({ ...form, memory_limit_multiplier: parseFloat(e.target.value) || 1.0 })}
                                    className="w-full border border-hairline-strong rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]" />
                                <p className="text-xs text-muted-soft mt-1">Python=2.0, Java=2.0, C++=1.0</p>
                            </div>
                            <div>
                                <label className="block text-xs font-medium text-muted uppercase mb-1">Seccomp Rule</label>
                                <select aria-label="Seccomp Rule" value={form.seccomp_rule} onChange={e => setForm({ ...form, seccomp_rule: e.target.value })}
                                    className="w-full border border-hairline-strong rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]">
                                    {seccompOptions.map(s => <option key={s} value={s}>{s}</option>)}
                                </select>
                            </div>
                            <div>
                                <label className="flex items-center gap-2 text-sm mt-5">
                                    <input type="checkbox" checked={form.mono} onChange={e => setForm({ ...form, mono: e.target.checked })}
                                        className="rounded" />
                                    <span className="text-ink-soft">Mono (C#)</span>
                                </label>
                            </div>
                            <div className="col-span-2">
                                <label className="block text-xs font-medium text-muted uppercase mb-1">File Extensions</label>
                                <div className="flex gap-2 mb-2">
                                    <input value={extInput} onChange={e => setExtInput(e.target.value)} onKeyDown={e => e.key === 'Enter' && (e.preventDefault(), addExtension())}
                                        placeholder=".cpp" className="flex-1 border border-hairline-strong rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]" />
                                    <button onClick={addExtension} className="px-3 py-2 bg-surface-3 rounded text-sm hover:bg-surface-3 cursor-pointer">Add</button>
                                </div>
                                <div className="flex flex-wrap gap-1">
                                    {form.extensions.map(ext => (
                                        <span key={ext} className="inline-flex items-center gap-1 bg-surface-3 text-ink-soft text-xs px-2 py-1 rounded">
                                            {ext}
                                            <button onClick={() => removeExtension(ext)} className="text-muted-soft hover:text-red-500 cursor-pointer"><X size={10} /></button>
                                        </span>
                                    ))}
                                </div>
                            </div>
                        </div>
                    )}

                    <div className="flex justify-end gap-2 pt-2">
                        {/* Only for shipped languages: one an operator created has
                            no built-in default, and the server answers 404. */}
                        {!showCreate && editKey && (
                            <button onClick={() => handleReset(editKey)} disabled={saving}
                                className="mr-auto flex items-center gap-1.5 px-3 py-2 text-sm text-warn border border-warn/40 rounded hover:bg-warn/10 disabled:opacity-50 cursor-pointer"
                                title="Restore the configuration shipped with the app">
                                <RotateCcw size={14} /> Reset to Default
                            </button>
                        )}
                        <button onClick={() => { setEditKey(null); setShowCreate(false) }}
                            className="px-4 py-2 text-sm border border-hairline-strong rounded hover:bg-surface-2 cursor-pointer">Cancel</button>
                        <button onClick={showCreate ? handleCreate : handleSave} disabled={saving}
                            className="flex items-center gap-1.5 px-4 py-2 text-sm bg-brand text-on-brand rounded hover:bg-brand-hover disabled:opacity-50 cursor-pointer">
                            <Save size={14} /> {saving ? 'Saving...' : 'Save'}
                        </button>
                    </div>
                </div>
            )}

            {/* Detected Tools Panel */}
            {showDetect && (
                <div className="rounded-lg border border-ok/30 bg-ok/8 p-6 space-y-4">
                    <div className="flex items-center justify-between">
                        <h3 className="font-semibold text-ink flex items-center gap-2"><Cpu size={16} /> Detected Compilers & Interpreters</h3>
                        <button onClick={() => setShowDetect(false)} className="text-muted-soft hover:text-ink cursor-pointer"><X size={16} /></button>
                    </div>
                    {detecting ? (
                        <p className="text-sm text-muted">Probing the judge sandbox for toolchains...</p>
                    ) : detected && (
                        <>
                            <p className="text-sm text-muted">Tools found on the judge server. Click "Use" to auto-fill the compile/runtime field in the form.</p>

                            <div className="grid grid-cols-2 gap-4">
                                <div>
                                    <h4 className="font-medium text-sm text-ink-soft mb-2">Compilers</h4>
                                    <div className="space-y-2 max-h-48 overflow-y-auto">
                                        {detected.compilers.map((c, i) => (
                                            <div key={i} className="flex items-center justify-between bg-surface border border-hairline rounded px-3 py-2">
                                                <div>
                                                    <div className="text-sm font-medium">{c.name}</div>
                                                    <div className="text-xs text-muted font-mono">{c.path}</div>
                                                    {c.version && <div className="text-xs text-muted-soft mt-0.5">{c.version}</div>}
                                                </div>
                                                <button onClick={() => applyDetectedCompiler(c)}
                                                    className="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900/30 text-brand rounded hover:bg-blue-200 cursor-pointer">Use</button>
                                            </div>
                                        ))}
                                        {detected.compilers.length === 0 && <p className="text-xs text-muted-soft">No compilers detected</p>}
                                    </div>
                                </div>
                                <div>
                                    <h4 className="font-medium text-sm text-ink-soft mb-2">Interpreters</h4>
                                    <div className="space-y-2 max-h-48 overflow-y-auto">
                                        {detected.interpreters.map((i, idx) => (
                                            <div key={idx} className="flex items-center justify-between bg-surface border border-hairline rounded px-3 py-2">
                                                <div>
                                                    <div className="text-sm font-medium">{i.name}</div>
                                                    <div className="text-xs text-muted font-mono">{i.path}</div>
                                                    {i.version && <div className="text-xs text-muted-soft mt-0.5">{i.version}</div>}
                                                </div>
                                                <button onClick={() => applyDetectedInterpreter(i)}
                                                    className="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900/30 text-brand rounded hover:bg-blue-200 cursor-pointer">Use</button>
                                            </div>
                                        ))}
                                        {detected.interpreters.length === 0 && <p className="text-xs text-muted-soft">No interpreters detected</p>}
                                    </div>
                                </div>
                            </div>
                        </>
                    )}
                </div>
            )}

            {/* Templates Panel */}
            {showTemplates && (
                <div className="rounded-lg border border-violet-500/30 bg-violet-500/8 p-6 space-y-4">
                    <div className="flex items-center justify-between">
                        <h3 className="font-semibold text-ink flex items-center gap-2"><Zap size={16} /> Language Templates</h3>
                        <button onClick={() => setShowTemplates(false)} className="text-muted-soft hover:text-ink cursor-pointer"><X size={16} /></button>
                    </div>
                    <p className="text-sm text-muted">Quick-start templates for common languages. Click "Use" to pre-fill the create form.</p>

                    <div className="grid grid-cols-3 gap-3">
                        {templates.map(t => (
                            <div key={t.key} className="bg-surface border border-hairline rounded-lg p-3 hover:border-purple-300 transition-colors">
                                <div className="font-medium text-sm">{t.name}</div>
                                <div className="text-xs text-muted font-mono mt-1">{t.key}</div>
                                <div className="text-xs text-muted-soft mt-1">
                                    {t.compile ? `compile: ${t.compile.substring(0, 30)}...` : `runtime: ${t.runtime}`}
                                </div>
                                <button onClick={() => applyTemplate(t)}
                                    className="mt-2 text-xs px-2 py-1 bg-purple-100 text-accent rounded hover:bg-purple-200 cursor-pointer w-full">Use Template</button>
                            </div>
                        ))}
                    </div>
                </div>
            )}

            {/* Test Result */}
            {testResult && (
                <div className={`border rounded-lg p-4 ${testResult.status === 'error' ? 'border-bad/30 bg-bad/10' : testResult.status === 'warning' ? 'border-warn/30 bg-warn/10' : 'border-ok/30 bg-ok/10'}`}>
                    <div className="flex items-center justify-between mb-2">
                        <span className="font-medium text-sm">{testResult.key}</span>
                        <button onClick={() => setTestResult(null)} className="text-muted-soft hover:text-ink cursor-pointer"><X size={14} /></button>
                    </div>
                    <pre className="text-xs font-mono text-ink-soft whitespace-pre-wrap">{JSON.stringify(testResult, null, 2)}</pre>
                </div>
            )}

            {/* Language List */}
            <div className="border border-hairline rounded-lg overflow-hidden">
                <table className="w-full text-sm">
                    <thead className="bg-surface-2 text-muted text-xs uppercase">
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
                    <tbody className="divide-y divide-hairline">
                        {langs.map(lang => (
                            <tr key={lang.key} className="hover:bg-surface-2">
                                <td className="px-4 py-2 font-medium">{lang.name}</td>
                                <td className="px-4 py-2 font-mono text-xs text-muted">{lang.key}</td>
                                <td className="px-4 py-2 font-mono text-xs text-muted max-w-xs truncate">{lang.compile || '—'}</td>
                                <td className="px-4 py-2 font-mono text-xs text-muted max-w-xs truncate">{lang.runtime || '—'}</td>
                                <td className="px-4 py-2 text-center text-xs">{lang.time_limit_multiplier}×</td>
                                <td className="px-4 py-2 text-center text-xs">{lang.memory_limit_multiplier}×</td>
                                <td className="px-4 py-2 text-right">
                                    <div className="flex items-center justify-end gap-1">
                                        <button onClick={() => handleReset(lang.key)} disabled={saving}
                                            className="p-1.5 text-muted-soft hover:text-warn hover:bg-warn/10 rounded cursor-pointer disabled:opacity-50"
                                            title="Reset to Default">
                                            <RotateCcw size={14} />
                                        </button>
                                        <button onClick={() => handleTest(lang.key)} disabled={testing === lang.key}
                                            className="p-1.5 text-muted-soft hover:text-green-600 hover:bg-green-50 dark:hover:bg-green-900/20 rounded cursor-pointer disabled:opacity-50" title="Test Config">
                                            <Play size={14} />
                                        </button>
                                        <button onClick={() => handleEdit(lang.key)}
                                            className="p-1.5 text-muted-soft hover:text-brand hover:bg-brand-soft rounded cursor-pointer" title="Edit">
                                            <Code size={14} />
                                        </button>
                                        <button onClick={() => handleDelete(lang.key)}
                                            className="p-1.5 text-muted-soft hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 rounded cursor-pointer" title="Delete">
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
