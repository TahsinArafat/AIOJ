import { useEffect, useState } from 'react'
import { EmptyState } from '../components/EmptyState'
import { api } from '../lib/api'
import { useConfirm } from '../components/ConfirmDialog'
import { useToast } from '../components/Toast'

interface ApiKeyItem { id: string; name: string; key_preview?: string }
interface OpenApiParameter { name?: string; required?: boolean; schema?: { type?: string } }
interface OpenApiOperation { summary?: string; security?: object[]; parameters?: OpenApiParameter[] }
interface OpenApiDoc { info?: { title?: string; description?: string }; paths?: Record<string, Record<string, OpenApiOperation>> }

export default function APISettings() {
    const confirm = useConfirm()
    const toast = useToast()
    const [keys, setKeys] = useState<ApiKeyItem[]>([])
    const [name, setName] = useState('')
    const [newSecret, setNewSecret] = useState('')
    const [loading, setLoading] = useState(true)
    const [activeTab, setActiveTab] = useState<'keys' | 'reference'>('keys')
    const [openapi, setOpenapi] = useState<OpenApiDoc | null>(null)

    useEffect(() => {
        api.apiKeys.list().then(d => setKeys(d.data || [])).catch(console.error).finally(() => setLoading(false))
    }, [])

    useEffect(() => {
        if (activeTab === 'reference' && !openapi) {
            fetch('/openapi.json')
                .then(res => res.json())
                .then(setOpenapi)
                .catch(console.error)
        }
    }, [activeTab, openapi])

    const handleCreate = async () => {
        if (!name.trim()) return
        try {
            const result = await api.apiKeys.create({ name })
            setNewSecret(result.secret)
            setName('')
            api.apiKeys.list().then(d => setKeys(d.data || []))
        } catch (e) { toast.error('Failed: ' + (e instanceof Error ? e.message : String(e))) }
    }

    const handleDelete = async (id: string) => {
        if (!(await confirm({ message: 'Delete this API key?', variant: 'danger' }))) return
        try {
            await api.apiKeys.delete(id)
            setKeys(prev => prev.filter(k => k.id !== id))
        } catch (e) { toast.error('Failed: ' + (e instanceof Error ? e.message : String(e))) }
    }

    if (loading) return <div className="text-center py-20 text-muted-soft">Loading...</div>

    return (
        <div className="max-w-3xl mx-auto">
            <div className="flex justify-between items-center mb-6">
                <h1 className="text-2xl font-bold">API Settings</h1>
            </div>

            <div className="border-b border-hairline flex gap-4 mb-6">
                <button
                    onClick={() => setActiveTab('keys')}
                    className={`pb-2 text-sm font-medium border-b-2 cursor-pointer ${activeTab === 'keys' ? 'border-brand text-brand' : 'border-transparent text-muted hover:text-gray-700'}`}
                >
                    API Keys
                </button>
                <button
                    onClick={() => setActiveTab('reference')}
                    className={`pb-2 text-sm font-medium border-b-2 cursor-pointer ${activeTab === 'reference' ? 'border-brand text-brand' : 'border-transparent text-muted hover:text-gray-700'}`}
                >
                    API Reference (Docs)
                </button>
            </div>

            {activeTab === 'keys' ? (
                <div className="max-w-2xl">
                    <div className="border rounded-lg p-6 mb-6 bg-surface border-hairline">
                        <h2 className="font-semibold mb-3">Create New Key</h2>
                        <div className="flex gap-2">
                            <input id="api-key-name" type="text" value={name} onChange={e => setName(e.target.value)}
                                aria-label="Key name"
                                placeholder="Key name" className="h-9 flex-1 rounded-lg border border-hairline-strong bg-surface px-3 text-sm text-ink placeholder:text-muted-soft" />
                            <button onClick={handleCreate} disabled={!name.trim()}
                                className="bg-brand text-on-brand px-4 py-2 rounded text-sm font-medium hover:bg-brand-hover disabled:opacity-50 cursor-pointer">
                                Create
                            </button>
                        </div>
                        {newSecret && (
                            <div className="mt-4 p-3 bg-warn/10 border border-warn/30 dark:border-yellow-900/30 rounded animate-pulse">
                                <p className="text-xs font-bold text-warn mb-2 uppercase">Save this secret - it won't be shown again!</p>
                                <code className="block bg-surface p-2 rounded text-xs font-mono break-all border border-hairline-strong">{newSecret}</code>
                            </div>
                        )}
                    </div>

                    <div>
                        <h2 className="font-semibold mb-3">Your API Keys</h2>
                        {keys.length === 0 ? (
                            <EmptyState text="No API keys yet" description="Create one above to authenticate your scripts and tools." />
                        ) : (
                            <div className="space-y-2">
                                {keys.map(k => (
                                    <div key={k.id} className="flex items-center justify-between border rounded p-3 bg-surface border-hairline">
                                        <div>
                                            <p className="font-medium text-sm">{k.name || 'Unnamed'}</p>
                                            <p className="text-xs text-muted-soft font-mono mt-1">{k.key_preview}</p>
                                        </div>
                                        <button onClick={() => handleDelete(k.id)}
                                            className="text-bad text-xs hover:underline font-medium cursor-pointer">Delete</button>
                                    </div>
                                ))}
                            </div>
                        )}
                    </div>
                </div>
            ) : (
                <div className="space-y-6">
                    {openapi ? (
                        <>
                            <div>
                                <h2 className="text-xl font-semibold mb-2">{openapi.info?.title}</h2>
                                <p className="text-sm text-muted">{openapi.info?.description}</p>
                            </div>

                            <div className="space-y-4">
                                {Object.entries(openapi.paths || {}).map(([path, methods]: [string, Record<string, OpenApiOperation>]) => (
                                    <div key={path} className="border border-hairline rounded-lg overflow-hidden bg-surface shadow-sm">
                                        {Object.entries(methods).map(([method, details]: [string, OpenApiOperation]) => {
                                            const colorClass = 
                                                method === 'get' ? 'bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300' :
                                                method === 'post' ? 'bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-300' :
                                                method === 'put' ? 'bg-orange-50 text-orange-700 dark:bg-orange-900/20 dark:text-orange-300' :
                                                'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-300';
                                            return (
                                                <div key={method} className="p-4 space-y-3">
                                                    <div className="flex items-center gap-3">
                                                        <span className={`text-xs font-bold uppercase px-2.5 py-1 rounded-md font-mono ${colorClass}`}>
                                                            {method}
                                                        </span>
                                                        <span className="font-mono text-sm font-semibold">{path}</span>
                                                        {details.security && (
                                                            <span className="text-xs font-medium text-warn border border-warn/40/50 px-1.5 py-0.5 rounded font-mono">🔑 Auth Required</span>
                                                        )}
                                                    </div>
                                                    <p className="text-sm font-medium">{details.summary}</p>
                                                    {details.parameters && details.parameters.length > 0 && (
                                                        <div className="space-y-1">
                                                            <p className="text-xs font-bold text-muted uppercase tracking-wider">Query Parameters:</p>
                                                            <ul className="text-xs font-mono space-y-1 pl-4 list-disc text-muted">
                                                                {details.parameters.map(param => (
                                                                    <li key={param.name}>
                                                                        <span className="font-semibold">{param.name}</span> ({param.schema?.type || 'string'}) {param.required ? '<required>' : ''}
                                                                    </li>
                                                                ))}
                                                            </ul>
                                                        </div>
                                                    )}
                                                </div>
                                            )
                                        })}
                                    </div>
                                ))}
                            </div>
                        </>
                    ) : (
                        <div className="text-center py-10 text-muted-soft">Loading documentation schema...</div>
                    )}
                </div>
            )}
        </div>
    )
}
