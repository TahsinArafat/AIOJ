import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'
import { EmptyState } from '../components/EmptyState'

interface TrainingPlanRow { id: string; title?: string; description?: string | null; section_count?: number; problem_count?: number; enrolled_count?: number }
interface OrgOption { id: string; name?: string }

export default function TrainingPlanList() {
	const [plans, setPlans] = useState<TrainingPlanRow[]>([])
	const [loading, setLoading] = useState(true)
	const [activeTab, setActiveTab] = useState<'public' | 'org'>('public')
	const [isAdmin] = useState(() => {
		const token = getAccessToken()
		if (!token) return false
		try {
			const payload = JSON.parse(atob(token.split('.')[1]))
			return payload.role === 'admin' || payload.role === 'setter'
		} catch { /* malformed JWT — keep default role */ return false }
	})
	const [myOrgs, setMyOrgs] = useState<OrgOption[]>([])
	const [selectedOrg, setSelectedOrg] = useState<string>('')

	useEffect(() => {
		const token = getAccessToken()
		if (token) {
			api.organizations.my().then(d => {
				setMyOrgs(d.data || [])
				if (d.data?.length > 0) {
					setSelectedOrg(d.data[0].id)
				}
			}).catch(() => { })
		}
	}, [])

	useEffect(() => {
		const opts: { orgId?: string; public?: boolean } = {}
		if (activeTab === 'org') {
			if (!selectedOrg) {
				queueMicrotask(() => { setPlans([]); setLoading(false) })
				return
			}
			opts.orgId = selectedOrg
		} else {
			opts.public = true
		}
		queueMicrotask(() => setLoading(true))
		api.training.list(0, 50, opts)
			.then(d => setPlans(d.data || []))
			.catch(() => { })
			.finally(() => setLoading(false))
	}, [activeTab, selectedOrg])

	if (loading) return <div className="text-center py-20 text-gray-400 dark:text-gray-500">Loading training plans...</div>

	return (
		<div className="space-y-6">
			<div className="flex items-center justify-between">
				<div>
					<h1 className="text-2xl font-bold">Training Plans</h1>
					<p className="text-gray-500 dark:text-gray-400 text-sm">Curated lists of problems organized into sections to improve your algorithmic skills.</p>
				</div>
				{isAdmin && (
					<Link to="/training/create" className="bg-blue-600 text-white px-4 py-2 rounded text-sm font-semibold hover:bg-blue-700 transition-colors">
						Create Training Plan
					</Link>
				)}
			</div>

			<div className="border-b border-gray-200 dark:border-gray-700 mb-6 flex items-center justify-between">
				<nav className="flex gap-6">
					<button onClick={() => setActiveTab('public')}
						className={`pb-4 text-sm font-medium ${activeTab === 'public' ? 'border-b-2 border-blue-600 text-blue-600 dark:text-blue-400 font-semibold' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'}`}>
						Public Plans
					</button>
					{getAccessToken() && (
						<button onClick={() => setActiveTab('org')}
							className={`pb-4 text-sm font-medium ${activeTab === 'org' ? 'border-b-2 border-blue-600 text-blue-600 dark:text-blue-400 font-semibold' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'}`}>
							Organization Plans
						</button>
					)}
				</nav>

				{activeTab === 'org' && myOrgs.length > 0 && (
					<select value={selectedOrg} onChange={e => setSelectedOrg(e.target.value)}
						className="border border-gray-300 dark:border-gray-600 rounded-md px-3 py-1 text-sm focus:outline-none bg-white dark:bg-gray-800">
						{myOrgs.map(o => (
							<option key={o.id} value={o.id}>{o.name}</option>
						))}
					</select>
				)}
			</div>

			<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
				{plans.map(p => (
					<div key={p.id} className="border border-gray-200 dark:border-gray-700 rounded-lg p-5 bg-white dark:bg-gray-800 hover:shadow-sm transition-shadow space-y-3">
						<div>
							<h3 className="font-bold text-lg text-gray-900 dark:text-gray-100">
								<Link to={`/training/${p.id}`} className="hover:underline text-blue-600 dark:text-blue-400">{p.title}</Link>
							</h3>
							<p className="text-gray-600 dark:text-gray-400 text-sm mt-1 line-clamp-2">{p.description || 'No description provided.'}</p>
						</div>
						<div className="flex justify-between items-center text-xs text-gray-400 dark:text-gray-500 border-t border-gray-100 dark:border-gray-700 pt-3">
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
					<div className="w-full col-span-2"><EmptyState text="No training plans found" description="Training plans you create or follow will appear here." /></div>
				)}
			</div>
		</div>
	)
}
