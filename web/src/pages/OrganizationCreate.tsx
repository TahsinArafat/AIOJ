import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'

export default function OrganizationCreate() {
	const nav = useNavigate()
	const [name, setName] = useState('')
	const [desc, setDesc] = useState('')
	const [error, setError] = useState('')
	const [submitting, setSubmitting] = useState(false)

	const handleSubmit = async (e: React.FormEvent) => {
		e.preventDefault()
		if (!name.trim()) {
			setError('Name is required')
			return
		}
		setError('')
		setSubmitting(true)
		try {
			const res = await api.organizations.create({ name, description: desc })
			nav(`/organizations/${res.id}`)
		} catch (err) {
			setError((err instanceof Error ? err.message : '') || 'Failed to create organization')
		} finally {
			setSubmitting(false)
		}
	}

	return (
		<div className="max-w-md mx-auto bg-surface border border-hairline rounded-lg p-6 my-10">
			<h1 className="text-xl font-bold mb-4">Create Organization</h1>
			{error && <div className="bg-bad/10 border border-bad/30 text-bad px-4 py-2 rounded mb-4 text-sm">{error}</div>}
			<form onSubmit={handleSubmit} className="space-y-4">
				<div>
					<label className="block text-sm font-medium text-ink-soft mb-1">Organization Name</label>
					<input required value={name} onChange={e => setName(e.target.value)}
						placeholder="e.g. MIT CS Department"
						className="w-full border border-hairline-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]" />
				</div>
				<div>
					<label className="block text-sm font-medium text-ink-soft mb-1">Description</label>
					<textarea rows={4} value={desc} onChange={e => setDesc(e.target.value)}
						placeholder="Add a brief description..."
						className="w-full border border-hairline-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]" />
				</div>
				<button type="submit" disabled={submitting}
					className="w-full bg-brand text-on-brand py-2 rounded font-semibold hover:bg-brand-hover disabled:opacity-50 transition-colors">
					{submitting ? 'Creating...' : 'Create Organization'}
				</button>
			</form>
		</div>
	)
}
