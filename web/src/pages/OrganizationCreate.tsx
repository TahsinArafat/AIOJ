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
		} catch (err: any) {
			setError(err.message || 'Failed to create organization')
		} finally {
			setSubmitting(false)
		}
	}

	return (
		<div className="max-w-md mx-auto bg-white border border-gray-200 rounded-lg p-6 my-10">
			<h1 className="text-xl font-bold mb-4">Create Organization</h1>
			{error && <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-2 rounded mb-4 text-sm">{error}</div>}
			<form onSubmit={handleSubmit} className="space-y-4">
				<div>
					<label className="block text-sm font-medium text-gray-700 mb-1">Organization Name</label>
					<input required value={name} onChange={e => setName(e.target.value)}
						placeholder="e.g. MIT CS Department"
						className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
				</div>
				<div>
					<label className="block text-sm font-medium text-gray-700 mb-1">Description</label>
					<textarea rows={4} value={desc} onChange={e => setDesc(e.target.value)}
						placeholder="Add a brief description..."
						className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
				</div>
				<button type="submit" disabled={submitting}
					className="w-full bg-blue-600 text-white py-2 rounded font-semibold hover:bg-blue-700 disabled:opacity-50 transition-colors">
					{submitting ? 'Creating...' : 'Create Organization'}
				</button>
			</form>
		</div>
	)
}
