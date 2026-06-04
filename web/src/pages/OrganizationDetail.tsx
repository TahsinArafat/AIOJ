import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'

export default function OrganizationDetail() {
	const { id } = useParams<{ id: string }>()
	const [org, setOrg] = useState<any>(null)
	const [classes, setClasses] = useState<any[]>([])
	const [members, setMembers] = useState<any[]>([])
	const [loading, setLoading] = useState(true)
	const [activeTab, setActiveTab] = useState<'classes' | 'members'>('classes')
	const [isMember, setIsMember] = useState(false)
	const [isOwner, setIsOwner] = useState(false)
	const [joining, setJoining] = useState(false)

	// Class creation state
	const [className, setClassName] = useState('')
	const [classDesc, setClassDesc] = useState('')
	const [classError, setClassError] = useState('')
	const [showAddClass, setShowAddClass] = useState(false)

	useEffect(() => {
		if (!id) return
		Promise.all([
			api.organizations.get(id),
			api.classes.list(id, 0, 50),
			api.organizations.members(id)
		]).then(([orgData, classData, memberData]) => {
			setOrg(orgData)
			setClasses(classData.data || [])
			setMembers(memberData.data || [])

			const token = getAccessToken()
			if (token) {
				try {
					const payload = JSON.parse(atob(token.split('.')[1]))
					const currentUserID = payload.user_id
					const member = memberData.data?.find((m: any) => m.user_id === currentUserID)
					setIsMember(!!member)
					setIsOwner(member?.role === 'owner' || member?.role === 'admin' || payload.role === 'admin')
				} catch {}
			}
		}).catch(() => {}).finally(() => setLoading(false))
	}, [id])

	const handleJoinLeave = async () => {
		if (!id) return
		setJoining(true)
		try {
			if (isMember) {
				await api.organizations.leave(id)
				setIsMember(false)
				setMembers(m => m.filter(x => x.username !== 'Me'))
			} else {
				await api.organizations.join(id)
				setIsMember(true)
				window.location.reload()
			}
		} catch (e: any) { alert(e.message) }
		finally { setJoining(false) }
	}

	const handleCreateClass = async (e: React.FormEvent) => {
		e.preventDefault()
		if (!className.trim()) { setClassError('Class Name is required'); return }
		setClassError('')
		try {
			const c = await api.classes.create(id!, { name: className, description: classDesc })
			setClasses(p => [c, ...p])
			setClassName('')
			setClassDesc('')
			setShowAddClass(false)
		} catch (err: any) {
			setClassError(err.message || 'Failed to create class')
		}
	}

	if (loading) return <div className="text-center py-20 text-gray-400 dark:text-gray-500">Loading organization details...</div>
	if (!org) return <div className="text-center py-20 text-gray-400 dark:text-gray-500">Organization not found.</div>

	return (
		<div className="space-y-6">
			<div className="flex items-start justify-between bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-6 shadow-sm">
				<div className="space-y-2">
					<h1 className="text-3xl font-extrabold tracking-tight text-gray-900 dark:text-gray-100">{org.name}</h1>
					<p className="text-gray-600 dark:text-gray-400">{org.description || 'No description provided.'}</p>
					<div className="text-xs text-gray-400 dark:text-gray-500 pt-1">
						<span>{members.length} Members</span> • <span>Created {new Date(org.created_at).toLocaleDateString()}</span>
					</div>
				</div>
				{getAccessToken() && (
					<button onClick={handleJoinLeave} disabled={joining}
						className={`px-6 py-2 rounded font-semibold text-sm transition-colors ${
							isMember 
								? 'bg-red-50 dark:bg-red-900/20 text-red-600 dark:text-red-400 border border-red-100 hover:bg-red-100'
								: 'bg-blue-600 text-white hover:bg-blue-700'
						}`}>
						{isMember ? 'Leave Organization' : 'Join Organization'}
					</button>
				)}
			</div>

			<div className="border-b border-gray-200 dark:border-gray-700 mb-6 flex items-center justify-between">
				<nav className="flex gap-6">
					<button onClick={() => setActiveTab('classes')}
						className={`pb-4 text-sm font-medium ${activeTab === 'classes' ? 'border-b-2 border-blue-600 text-blue-600 dark:text-blue-400 font-semibold' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'}`}>
						Classes
					</button>
					<button onClick={() => setActiveTab('members')}
						className={`pb-4 text-sm font-medium ${activeTab === 'members' ? 'border-b-2 border-blue-600 text-blue-600 dark:text-blue-400 font-semibold' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'}`}>
						Members
					</button>
				</nav>
				{activeTab === 'classes' && isOwner && (
					<button onClick={() => setShowAddClass(!showAddClass)}
						className="bg-blue-600 text-white px-4 py-1.5 rounded text-xs font-semibold hover:bg-blue-700 mb-2">
						{showAddClass ? 'Cancel' : 'Add Class'}
					</button>
				)}
			</div>

			{showAddClass && (
				<form onSubmit={handleCreateClass} className="bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-5 space-y-3">
					<h3 className="font-semibold text-gray-700 dark:text-gray-300 text-sm">Add New Class</h3>
					{classError && <div className="text-red-600 dark:text-red-400 text-xs">{classError}</div>}
					<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
						<input required value={className} onChange={e => setClassName(e.target.value)}
							placeholder="Class Name (e.g. CS101 Section A)"
							className="border border-gray-300 dark:border-gray-600 rounded-md px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400 bg-white dark:bg-gray-800" />
						<input value={classDesc} onChange={e => setClassDesc(e.target.value)}
							placeholder="Description (optional)"
							className="border border-gray-300 dark:border-gray-600 rounded-md px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400 bg-white dark:bg-gray-800" />
					</div>
					<button type="submit" className="bg-blue-600 text-white px-4 py-1.5 rounded text-xs font-semibold hover:bg-blue-700">
						Create Class
					</button>
				</form>
			)}

			{activeTab === 'classes' ? (
				<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
					{classes.map(c => (
						<div key={c.id} className="border border-gray-200 dark:border-gray-700 rounded-lg p-5 bg-white dark:bg-gray-800 hover:shadow-sm transition-shadow flex justify-between items-center">
							<div className="space-y-1">
								<h3 className="font-bold text-base text-gray-900 dark:text-gray-100">
									<Link to={`/classes/${c.id}`} className="hover:underline text-blue-600 dark:text-blue-400">{c.name}</Link>
								</h3>
								<p className="text-gray-500 dark:text-gray-400 text-sm">{c.description || 'No description.'}</p>
							</div>
							<span className="text-xs font-semibold px-2 py-1 rounded bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300">{c.student_count || 0} Students</span>
						</div>
					))}
					{classes.length === 0 && <p className="text-gray-400 dark:text-gray-500 text-center py-10 w-full col-span-2">No classes yet.</p>}
				</div>
			) : (
				<div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
					<table className="w-full text-sm text-left">
						<thead className="bg-gray-50 dark:bg-gray-800 text-gray-500 dark:text-gray-400 text-xs uppercase font-medium">
							<tr>
								<th className="px-6 py-3">Member</th>
								<th className="px-6 py-3">Role</th>
								<th className="px-6 py-3">Joined</th>
							</tr>
						</thead>
						<tbody className="divide-y divide-gray-100 dark:divide-gray-700 font-mono text-xs">
							{members.map(m => (
								<tr key={m.user_id} className="hover:bg-gray-50 dark:hover:bg-gray-700">
									<td className="px-6 py-3 font-semibold text-gray-900 dark:text-gray-100">{m.username || m.user_id}</td>
									<td className="px-6 py-3 uppercase">{m.role}</td>
									<td className="px-6 py-3 text-gray-500 dark:text-gray-400">{new Date(m.joined_at).toLocaleDateString()}</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			)}
		</div>
	)
}
