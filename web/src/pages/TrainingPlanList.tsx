import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'

export default function TrainingPlanList() {
	const [plans, setPlans] = useState<any[]>([])
	const [loading, setLoading] = useState(true)
	const [activeTab, setActiveTab] = useState<'public' | 'org'>('public')
	const [isAdmin, setIsAdmin] = useState(false)
	const [myOrgs, setMyOrgs] = useState<any[]>([])
	const [selectedOrg, setSelectedOrg] = useState<string>('')

	useEffect(() => {
		const token = getAccessToken()
		if (token) {
			try {
				const payload = JSON.parse(atob(token.split('.')[1]))
				setIsAdmin(payload.role === 'admin' || payload.role === 'teacher')
			} catch {}
			api.organizations.my().then(d => {
				setMyOrgs(d.data || [])
				if (d.data?.length > 0) {
					setSelectedOrg(d.data[0].id)
				}
			}).catch(() => {})
		}
	}, [])

	useEffect(() => {
		setLoading(true)
		const opts: any = {}
		if (activeTab === 'org') {
			if (!selectedOrg) {
				setPlans([])
				setLoading(false)
				return
			}
			opts.orgId = selectedOrg
		} else {
			opts.public = true
		}

		api.training.list(0, 50, opts)
			.then(d => setPlans(d.data || []))
			.catch(() => {})
			.finally(() => setLoading(false))
	}, [activeTab, selectedOrg])

	if (loading) return <div className="text-center py-20 text-gray-400">Loading training plans...</div>

	return (
		<div className="space-y-6">
			<div className="flex items-center justify-between">
				<div>
					<h1 className="text-2xl font-bold">Training Plans</h1>
					<p className="text-gray-500 text-sm">Curated lists of problems organized into sections to improve your algorithmic skills.</p>
				</div>
				{isAdmin && (
					<Link to="/training/create" className="bg-blue-600 text-white px-4 py-2 rounded text-sm font-semibold hover:bg-blue-700 transition-colors">
						Create Training Plan
					</Link>
				)}
			</div>

			<div className="border-b border-gray-200 mb-6 flex items-center justify-between">
				<nav className="flex gap-6">
					<button onClick={() => setActiveTab('public')}
						className={`pb-4 text-sm font-medium ${activeTab === 'public' ? 'border-b-2 border-blue-600 text-blue-600 font-semibold' : 'text-gray-500 hover:text-gray-700'}`}>
						Public Plans
					</button>
					{getAccessToken() && (
						<button onClick={() => setActiveTab('org')}
							className={`pb-4 text-sm font-medium ${activeTab === 'org' ? 'border-b-2 border-blue-600 text-blue-600 font-semibold' : 'text-gray-500 hover:text-gray-700'}`}>
							Organization Plans
						</button>
					)}
				</nav>

				{activeTab === 'org' && myOrgs.length > 0 && (
					<select value={selectedOrg} onChange={e => setSelectedOrg(e.target.value)}
						className="border border-gray-300 rounded-md px-3 py-1 text-sm focus:outline-none bg-white">
						{myOrgs.map(o => (
							<option key={o.id} value={o.id}>{o.name}</option>
						))}
					</select>
				)}
			</div>

			<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
				{plans.map(p => (
					<div key={p.id} className="border border-gray-200 rounded-lg p-5 bg-white hover:shadow-sm transition-shadow space-y-3">
						<div>
							<h3 className="font-bold text-lg text-gray-900">
								<Link to={`/training/${p.id}`} className="hover:underline text-blue-600">{p.title}</Link>
							</h3>
							<p className="text-gray-600 text-sm mt-1 line-clamp-2">{p.description || 'No description provided.'}</p>
						</div>
						<div className="flex justify-between items-center text-xs text-gray-400 border-t border-gray-100 pt-3">
							<span className="flex gap-2">
								<span>{p.section_count} Sections</span>
								<span>•</span>
								<span>{p.problem_count} Problems</span>
							</span>
							<span>{p.enrolled_count} Enrolled</span>
						</div>
					</div>
				))}
				{plans.length === 0 && (
					<p className="text-gray-400 text-center py-10 w-full col-span-2">No training plans found.</p>
				)}
			</div>
		</div>
	)
}
