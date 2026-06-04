import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'

export default function TeamCreate() {
    const navigate = useNavigate()
    const [name, setName] = useState('')
    const [description, setDescription] = useState('')
    const [isPublic, setIsPublic] = useState(true)
    const [submitting, setSubmitting] = useState(false)

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        if (!name.trim()) return
        setSubmitting(true)
        try {
            const team = await api.teams.create({ name, description, is_public: isPublic })
            navigate(`/teams/${team.id}`)
        } catch (e: any) {
            alert('Failed to create team: ' + e.message)
        } finally {
            setSubmitting(false)
        }
    }

    return (
        <div className="max-w-md mx-auto">
            <h1 className="text-2xl font-bold mb-6">Create Team</h1>
            <form onSubmit={handleSubmit} className="space-y-4 border p-6 rounded-lg bg-white dark:bg-gray-800">
                <div>
                    <label className="block text-sm font-semibold text-gray-700 dark:text-gray-300 mb-1">Team Name</label>
                    <input type="text" value={name} onChange={e => setName(e.target.value)} required
                        className="w-full border rounded px-3 py-2 text-sm" placeholder="e.g. Code Gladiators" />
                </div>
                <div>
                    <label className="block text-sm font-semibold text-gray-700 dark:text-gray-300 mb-1">Description</label>
                    <textarea value={description} onChange={e => setDescription(e.target.value)} rows={3}
                        className="w-full border rounded px-3 py-2 text-sm" placeholder="Describe your team (optional)..." />
                </div>
                <div className="flex items-center gap-2">
                    <input type="checkbox" id="isPublic" checked={isPublic}
                        onChange={e => setIsPublic(e.target.checked)}
                        className="rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500" />
                    <label htmlFor="isPublic" className="text-sm font-medium text-gray-700 dark:text-gray-300">Public Team</label>
                </div>
                <button type="submit" disabled={submitting || !name.trim()}
                    className="w-full bg-blue-600 text-white py-2 rounded text-sm hover:bg-blue-700 font-medium disabled:opacity-50">
                    {submitting ? 'Creating...' : 'Create Team'}
                </button>
            </form>
        </div>
    )
}
