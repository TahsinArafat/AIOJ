import { useEffect, useState } from 'react'
import { api } from '../lib/api'

export default function APISettings() {
    const [keys, setKeys] = useState<any[]>([])
    const [name, setName] = useState('')
    const [newSecret, setNewSecret] = useState('')
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        api.apiKeys.list().then(d => setKeys(d.data || [])).catch(console.error).finally(() => setLoading(false))
    }, [])

    const handleCreate = async () => {
        if (!name.trim()) return
        try {
            const result = await api.apiKeys.create({ name })
            setNewSecret(result.secret)
            setName('')
            window.location.reload()
        } catch (e: any) { alert('Failed: ' + e.message) }
    }

    const handleDelete = async (id: string) => {
        if (!confirm('Delete this API key?')) return
        try {
            await api.apiKeys.delete(id)
            setKeys(prev => prev.filter(k => k.id !== id))
        } catch (e: any) { alert('Failed: ' + e.message) }
    }

    if (loading) return <div className="text-center py-20 text-gray-400">Loading...</div>

    return (
        <div className="max-w-2xl mx-auto">
            <h1 className="text-2xl font-bold mb-6">API Keys</h1>

            <div className="border rounded-lg p-6 mb-6 bg-white">
                <h2 className="font-semibold mb-3">Create New Key</h2>
                <div className="flex gap-2">
                    <input type="text" value={name} onChange={e => setName(e.target.value)}
                        placeholder="Key name" className="flex-1 border rounded px-3 py-2 text-sm" />
                    <button onClick={handleCreate} disabled={!name.trim()}
                        className="bg-blue-600 text-white px-4 py-2 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50">
                        Create
                    </button>
                </div>
                {newSecret && (
                    <div className="mt-4 p-3 bg-yellow-50 border border-yellow-200 rounded">
                        <p className="text-xs font-bold text-yellow-800 mb-2 uppercase">Save this secret - it won't be shown again!</p>
                        <code className="block bg-white p-2 rounded text-xs font-mono break-all border">{newSecret}</code>
                    </div>
                )}
            </div>

            <div>
                <h2 className="font-semibold mb-3">Your API Keys</h2>
                {keys.length === 0 ? (
                    <p className="text-sm text-gray-400">No API keys created yet.</p>
                ) : (
                    <div className="space-y-2">
                        {keys.map(k => (
                            <div key={k.id} className="flex items-center justify-between border rounded p-3 bg-white">
                                <div>
                                    <p className="font-medium text-sm">{k.name || 'Unnamed'}</p>
                                    <p className="text-xs text-gray-400 font-mono">{k.key_preview}</p>
                                </div>
                                <button onClick={() => handleDelete(k.id)}
                                    className="text-red-600 text-xs hover:underline font-medium">Delete</button>
                            </div>
                        ))}
                    </div>
                )}
            </div>
        </div>
    )
}
