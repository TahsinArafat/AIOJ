import { useEffect, useState } from 'react'
import type { ClassMember, ClassInfo } from '../types/community'
import { errorMessage } from '../lib/errors'
import { useParams, Link } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'
import { useDocumentTitle } from '../lib/documentTitle'

export default function ClassDetail() {
	const { id } = useParams<{ id: string }>()
	const [c, setC] = useState<ClassInfo | null>(null)
	useDocumentTitle(c?.name)
	const [members, setMembers] = useState<ClassMember[]>([])
	const [loading, setLoading] = useState(true)
	const [isTeacher, setIsTeacher] = useState(false)
	const [joiningCode, setJoiningCode] = useState('')
	const [joinError, setJoinError] = useState('')

	useEffect(() => {
		if (!id) return
		Promise.all([
			api.classes.get(id),
			api.classes.members(id)
		]).then(([classData, memberData]) => {
			setC(classData)
			setMembers(memberData.data || [])

			const token = getAccessToken()
			if (token) {
				try {
					const payload = JSON.parse(atob(token.split('.')[1]))
					const currentUserID = payload.user_id
					const member = memberData.data?.find((m: ClassMember) => m.user_id === currentUserID)
					setIsTeacher(member?.role === 'teacher' || payload.role === 'admin')
				} catch { /* malformed JWT — keep default role */ }
			}
		}).catch(() => { }).finally(() => setLoading(false))
	}, [id])

	const handleJoinByCode = async (e: React.FormEvent) => {
		e.preventDefault()
		if (!joiningCode.trim()) return
		setJoinError('')
		try {
			const res = await api.classes.joinByCode(joiningCode)
			window.location.href = `/classes/${res.class_id}`
		} catch (err) {
			setJoinError(errorMessage(err) || 'Invalid invite code')
		}
	}

	if (loading) return <div className="text-center py-20 text-muted-soft">Loading class details...</div>
	if (!c) return (
		<div className="max-w-md mx-auto bg-surface border border-hairline rounded-lg p-6 my-10 space-y-4">
			<h1 className="text-xl font-bold">Class Not Found</h1>
			<p className="text-muted text-sm">You might not be enrolled. Enter class invite code to join:</p>
			{joinError && <div className="text-bad text-xs">{joinError}</div>}
			<form onSubmit={handleJoinByCode} className="flex gap-2">
				<input required value={joiningCode} onChange={e => setJoiningCode(e.target.value)}
					placeholder="8-character code"
					className="border border-hairline-strong rounded-md px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] flex-1" />
				<button type="submit" className="bg-brand text-on-brand px-4 py-1.5 rounded text-sm font-semibold hover:bg-brand-hover">
					Join
				</button>
			</form>
		</div>
	)

	return (
		<div className="space-y-6">
			<div className="bg-surface border border-hairline rounded-lg p-6 shadow-sm space-y-2">
				<div className="flex items-center gap-3">
					<Link to={`/organizations/${c.organization_id}`} className="text-xs text-brand hover:underline">
						← {c.org_name}
					</Link>
				</div>
				<h1 className="text-2xl font-bold text-ink">{c.name}</h1>
				<p className="text-muted text-sm">{c.description || 'No class description.'}</p>
				{isTeacher && (
					<div className="bg-brand-soft border border-blue-100 rounded-md p-3 text-xs text-blue-800 font-mono mt-3 inline-block">
						INVITE CODE: <span className="font-bold text-blue-900 select-all">{c.invite_code}</span>
					</div>
				)}
			</div>

			<div>
				<h2 className="text-lg font-semibold mb-3">Class Members</h2>
				<div className="bg-surface border border-hairline rounded-lg overflow-hidden">
					<table className="w-full text-sm text-left">
						<thead className="bg-surface-2 text-muted text-xs uppercase font-medium">
							<tr>
								<th className="px-6 py-3">Student</th>
								<th className="px-6 py-3">Role</th>
								<th className="px-6 py-3">Joined</th>
							</tr>
						</thead>
						<tbody className="divide-y divide-hairline font-mono text-xs">
							{members.map(m => (
								<tr key={m.user_id} className="hover:bg-surface-2">
									<td className="px-6 py-3 font-semibold text-ink">{m.username || m.user_id}</td>
									<td className="px-6 py-3 uppercase">{m.role}</td>
									<td className="px-6 py-3 text-muted">{new Date(m.joined_at ?? '').toLocaleDateString()}</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			</div>
		</div>
	)
}
