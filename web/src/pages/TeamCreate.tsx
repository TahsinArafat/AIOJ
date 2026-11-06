import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { useToast } from '../components/Toast'

export default function TeamCreate() {
    const toast = useToast()
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
        } catch (e) {
            toast.error('Failed to create team: ' + (e instanceof Error ? e.message : String(e)))
        } finally {
            setSubmitting(false)
        }
    }

    return (
        <div className="max-w-md mx-auto">
            <h1 className="text-2xl font-bold mb-6">Create Team</h1>
            <form onSubmit={handleSubmit} className="space-y-4 border p-6 rounded-lg bg-surface">
                <div>
                    <label htmlFor="team-name" className="block text-sm font-semibold text-ink-soft mb-1">Team Name</label>
                    <input id="team-name" type="text" value={name} onChange={e => setName(e.target.value)} required
                        className="w-full border rounded px-3 py-2 text-sm" placeholder="e.g. Code Gladiators" />
                </div>
                <div>
                    <label htmlFor="team-description" className="block text-sm font-semibold text-ink-soft mb-1">Description</label>
                    <textarea id="team-description" value={description} onChange={e => setDescription(e.target.value)} rows={3}
                        className="w-full border rounded px-3 py-2 text-sm" placeholder="Describe your team (optional)..." />
                </div>
                <div className="flex items-center gap-2">
                    <input type="checkbox" id="isPublic" checked={isPublic}
                        onChange={e => setIsPublic(e.target.checked)}
                        className="rounded border-hairline-strong text-brand focus-visible:outline-[var(--ring)]" />
                    <label htmlFor="isPublic" className="text-sm font-medium text-ink-soft">Public Team</label>
                </div>
                <button type="submit" disabled={submitting || !name.trim()}
                    className="w-full bg-brand text-on-brand py-2 rounded text-sm hover:bg-brand-hover font-medium disabled:opacity-50">
                    {submitting ? 'Creating...' : 'Create Team'}
                </button>
            </form>
        </div>
    )
}
