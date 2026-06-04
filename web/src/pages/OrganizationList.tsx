import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'

export default function OrganizationList() {
	const [orgs, setOrgs] = useState<any[]>([])
	const [loading, setLoading] = useState(true)
	const [isAdmin, setIsAdmin] = useState(false)

	useEffect(() => {
		const token = getAccessToken()
		if (token) {
			try {
				const payload = JSON.parse(atob(token.split('.')[1]))
				setIsAdmin(payload.role === 'admin' || payload.role === 'teacher')
			} catch {}
		}
		api.organizations.list(0, 50)
			.then(d => setOrgs(d.data || []))
			.catch(() => {})
			.finally(() => setLoading(false))
	}, [])

	if (loading) return <div className="text-center py-20 text-gray-400 dark:text-gray-500">Loading organizations...</div>

	return (
		<div className="space-y-6">
			<div className="flex items-center justify-between">
				<div>
					<h1 className="text-2xl font-bold">Organizations</h1>
					<p className="text-gray-500 dark:text-gray-400 text-sm">Join an organization or class to participate in private training curricula.</p>
				</div>
				{isAdmin && (
					<Link to="/organizations/create" className="bg-blue-600 text-white px-4 py-2 rounded text-sm font-semibold hover:bg-blue-700 transition-colors">
						Create Organization
					</Link>
				)}
			</div>

			<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
				{orgs.map(o => (
					<div key={o.id} className="border border-gray-200 dark:border-gray-700 rounded-lg p-5 bg-white dark:bg-gray-800 hover:shadow-sm transition-shadow space-y-3">
						<div>
							<h3 className="font-bold text-lg text-gray-900 dark:text-gray-100">
								<Link to={`/organizations/${o.id}`} className="hover:underline text-blue-600 dark:text-blue-400">{o.name}</Link>
							</h3>
							<p className="text-gray-600 dark:text-gray-400 text-sm mt-1 line-clamp-2">{o.description || 'No description provided.'}</p>
						</div>
						<div className="flex justify-between items-center text-xs text-gray-400 dark:text-gray-500 border-t border-gray-100 dark:border-gray-700 pt-3">
							<span>{o.member_count} Members</span>
							<span>Created {new Date(o.created_at).toLocaleDateString()}</span>
						</div>
					</div>
				))}
				{orgs.length === 0 && (
					<div className="col-span-2 text-center py-12 text-gray-400 dark:text-gray-500">No organizations found.</div>
				)}
			</div>
		</div>
	)
}
