import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { useToast } from '../components/Toast'

export default function GroupCreate() {
    const toast = useToast()
    const navigate = useNavigate()
    const [name, setName] = useState('')
    const [description, setDescription] = useState('')
    const [isPublic, setIsPublic] = useState(true)
    const [joinPolicy, setJoinPolicy] = useState('auto_approve')
    const [submitting, setSubmitting] = useState(false)

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        if (!name.trim()) return
        setSubmitting(true)
        try {
            const group = await api.groups.create({ name, description, is_public: isPublic, join_policy: joinPolicy })
            navigate(`/groups/${group.id}`)
        } catch (e) {
            toast.error('Failed to create group: ' + (e instanceof Error ? e.message : String(e)))
        } finally {
            setSubmitting(false)
        }
    }

    return (
        <div className="max-w-md mx-auto">
            <h1 className="text-2xl font-bold mb-6">Create Group</h1>
            <form onSubmit={handleSubmit} className="space-y-4 border p-6 rounded-lg bg-surface">
                <div>
                    <label htmlFor="group-name" className="block text-sm font-semibold text-ink-soft mb-1">Group Name</label>
                    <input id="group-name" type="text" value={name} onChange={e => setName(e.target.value)} required
                        className="w-full border rounded px-3 py-2 text-sm" placeholder="e.g. ACM Study Group" />
                </div>
                <div>
                    <label htmlFor="group-description" className="block text-sm font-semibold text-ink-soft mb-1">Description</label>
                    <textarea id="group-description" value={description} onChange={e => setDescription(e.target.value)} rows={3}
                        className="w-full border rounded px-3 py-2 text-sm" placeholder="Describe the purpose of this group..." />
                </div>
                <div className="flex items-center gap-2">
                    <input type="checkbox" id="public" checked={isPublic} onChange={e => setIsPublic(e.target.checked)}
                        className="rounded border-hairline-strong text-brand focus-visible:outline-[var(--ring)]" />
                    <label htmlFor="public" className="text-sm font-medium text-ink-soft">Public Group (Anyone can join)</label>
                </div>
                <div>
                    <label htmlFor="group-join-policy" className="block text-sm font-semibold text-ink-soft mb-1">Join Policy</label>
                    <select id="group-join-policy" value={joinPolicy} onChange={e => setJoinPolicy(e.target.value)}
                        className="w-full border rounded px-3 py-2 text-sm bg-surface text-ink-soft">
                        <option value="auto_approve">Auto-Approve (Open via link)</option>
                        <option value="manual_approve">Manual Approve (Requires confirmation)</option>
                    </select>
                </div>
                <button type="submit" disabled={submitting || !name.trim()}
                    className="w-full bg-brand text-on-brand py-2 rounded text-sm hover:bg-brand-hover font-medium disabled:opacity-50">
                    {submitting ? 'Creating...' : 'Create Group'}
                </button>
            </form>
        </div>
    )
}
