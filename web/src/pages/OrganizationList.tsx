import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'
import { EmptyState } from '../components/EmptyState'

interface OrgRow { id: string; name?: string; description?: string | null; member_count?: number; created_at: string }

export default function OrganizationList() {
	const [orgs, setOrgs] = useState<OrgRow[]>([])
	const [loading, setLoading] = useState(true)
	const [isAdmin] = useState(() => {
		const token = getAccessToken()
		if (!token) return false
		try {
			const payload = JSON.parse(atob(token.split('.')[1]))
			return payload.role === 'admin' || payload.role === 'setter'
		} catch { /* malformed JWT — keep default role */ return false }
	})

	useEffect(() => {
		api.organizations.list(0, 50)
			.then(d => setOrgs(d.data || []))
			.catch(() => { })
			.finally(() => setLoading(false))
	}, [])

	if (loading) return <div className="text-center py-20 text-muted-soft">Loading organizations...</div>

	return (
		<div className="space-y-6">
			<div className="flex items-center justify-between">
				<div>
					<h1 className="text-2xl font-bold">Organizations</h1>
					<p className="text-muted text-sm">Join an organization or class to participate in private training curricula.</p>
				</div>
				{isAdmin && (
					<Link to="/organizations/create" className="bg-brand text-on-brand px-4 py-2 rounded text-sm font-semibold hover:bg-brand-hover transition-colors">
						Create Organization
					</Link>
				)}
			</div>

			<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
				{orgs.map(o => (
					<div key={o.id} className="border border-hairline rounded-lg p-5 bg-surface hover:shadow-sm transition-shadow space-y-3">
						<div>
							<h3 className="font-bold text-lg text-ink">
								<Link to={`/organizations/${o.id}`} className="hover:underline text-brand">{o.name}</Link>
							</h3>
							<p className="text-muted text-sm mt-1 line-clamp-2">{o.description || 'No description provided.'}</p>
						</div>
						<div className="flex justify-between items-center text-xs text-muted-soft border-t border-hairline pt-3">
							<span>{o.member_count} Members</span>
							<span>Created {new Date(o.created_at).toLocaleDateString()}</span>
						</div>
					</div>
				))}
				{orgs.length === 0 && (
					<EmptyState text="No organizations found" description="Organizations you belong to will appear here." />
				)}
			</div>
		</div>
	)
}
