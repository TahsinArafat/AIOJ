import { useEffect, useState } from 'react'
import { api } from '../lib/api'

export default function APISettings() {
    const [keys, setKeys] = useState<any[]>([])
    const [name, setName] = useState('')
    const [newSecret, setNewSecret] = useState('')
    const [loading, setLoading] = useState(true)
    const [activeTab, setActiveTab] = useState<'keys' | 'reference'>('keys')
    const [openapi, setOpenapi] = useState<any>(null)

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
        } catch (e: any) { alert('Failed: ' + e.message) }
    }

    const handleDelete = async (id: string) => {
        if (!confirm('Delete this API key?')) return
        try {
            await api.apiKeys.delete(id)
            setKeys(prev => prev.filter(k => k.id !== id))
        } catch (e: any) { alert('Failed: ' + e.message) }
    }

    if (loading) return <div className="text-center py-20 text-gray-400 dark:text-gray-500">Loading...</div>

    return (
        <div className="max-w-3xl mx-auto">
            <div className="flex justify-between items-center mb-6">
                <h1 className="text-2xl font-bold">API Settings</h1>
            </div>

            <div className="border-b border-gray-200 dark:border-gray-800 flex gap-4 mb-6">
                <button
                    onClick={() => setActiveTab('keys')}
                    className={`pb-2 text-sm font-medium border-b-2 cursor-pointer ${activeTab === 'keys' ? 'border-blue-600 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 hover:text-gray-700'}`}
                >
                    API Keys
                </button>
                <button
                    onClick={() => setActiveTab('reference')}
                    className={`pb-2 text-sm font-medium border-b-2 cursor-pointer ${activeTab === 'reference' ? 'border-blue-600 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 hover:text-gray-700'}`}
                >
                    API Reference (Docs)
                </button>
            </div>

            {activeTab === 'keys' ? (
                <div className="max-w-2xl">
                    <div className="border rounded-lg p-6 mb-6 bg-white dark:bg-gray-800 border-gray-200 dark:border-gray-700">
                        <h2 className="font-semibold mb-3">Create New Key</h2>
                        <div className="flex gap-2">
                            <input type="text" value={name} onChange={e => setName(e.target.value)}
                                placeholder="Key name" className="flex-1 border rounded px-3 py-2 text-sm border-gray-300 dark:border-gray-700 bg-transparent text-gray-800 dark:text-gray-200" />
                            <button onClick={handleCreate} disabled={!name.trim()}
                                className="bg-blue-600 text-white px-4 py-2 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50 cursor-pointer">
                                Create
                            </button>
                        </div>
                        {newSecret && (
                            <div className="mt-4 p-3 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-900/30 rounded animate-pulse">
                                <p className="text-xs font-bold text-yellow-800 dark:text-yellow-300 mb-2 uppercase">Save this secret - it won't be shown again!</p>
                                <code className="block bg-white dark:bg-gray-800 p-2 rounded text-xs font-mono break-all border border-gray-300 dark:border-gray-700">{newSecret}</code>
                            </div>
                        )}
                    </div>

                    <div>
                        <h2 className="font-semibold mb-3">Your API Keys</h2>
                        {keys.length === 0 ? (
                            <p className="text-sm text-gray-400 dark:text-gray-500">No API keys created yet.</p>
                        ) : (
                            <div className="space-y-2">
                                {keys.map(k => (
                                    <div key={k.id} className="flex items-center justify-between border rounded p-3 bg-white dark:bg-gray-800 border-gray-200 dark:border-gray-700">
                                        <div>
                                            <p className="font-medium text-sm">{k.name || 'Unnamed'}</p>
                                            <p className="text-xs text-gray-400 dark:text-gray-500 font-mono mt-1">{k.key_preview}</p>
                                        </div>
                                        <button onClick={() => handleDelete(k.id)}
                                            className="text-red-600 dark:text-red-400 text-xs hover:underline font-medium cursor-pointer">Delete</button>
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
                                <p className="text-sm text-gray-600 dark:text-gray-400">{openapi.info?.description}</p>
                            </div>

                            <div className="space-y-4">
                                {Object.entries(openapi.paths || {}).map(([path, methods]: [string, any]) => (
                                    <div key={path} className="border border-gray-200 dark:border-gray-800 rounded-lg overflow-hidden bg-white dark:bg-gray-900 shadow-sm">
                                        {Object.entries(methods).map(([method, details]: [string, any]) => {
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
                                                            <span className="text-xs font-medium text-amber-600 dark:text-amber-400 border border-amber-300 dark:border-amber-700/50 px-1.5 py-0.5 rounded font-mono">🔑 Auth Required</span>
                                                        )}
                                                    </div>
                                                    <p className="text-sm font-medium">{details.summary}</p>
                                                    {details.parameters && details.parameters.length > 0 && (
                                                        <div className="space-y-1">
                                                            <p className="text-xs font-bold text-gray-500 uppercase tracking-wider">Query Parameters:</p>
                                                            <ul className="text-xs font-mono space-y-1 pl-4 list-disc text-gray-600 dark:text-gray-400">
                                                                {details.parameters.map((param: any) => (
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
                        <div className="text-center py-10 text-gray-400 dark:text-gray-500">Loading documentation schema...</div>
                    )}
                </div>
            )}
        </div>
    )
}
