import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'

export default function TrainingPlanCreate() {
	const nav = useNavigate()
	const [title, setTitle] = useState('')
	const [desc, setDesc] = useState('')
	const [orgId, setOrgId] = useState('')
	const [myOrgs, setMyOrgs] = useState<any[]>([])
	const [sections, setSections] = useState<any[]>([
		{ title: 'Section 1', description: '', problems: [{ problem_id: '', points: 100 }] }
	])
	const [error, setError] = useState('')
	const [submitting, setSubmitting] = useState(false)

	useEffect(() => {
		if (!getAccessToken()) nav('/login')
		api.organizations.my().then(d => setMyOrgs(d.data || [])).catch(() => {})
	}, [nav])

	const handleAddSection = () => {
		setSections(p => [...p, {
			title: `Section ${p.length + 1}`,
			description: '',
			problems: [{ problem_id: '', points: 100 }]
		}])
	}

	const handleAddProblem = (sIdx: number) => {
		setSections(p => {
			const next = [...p]
			next[sIdx].problems.push({ problem_id: '', points: 100 })
			return next
		})
	}

	const handleSectionChange = (sIdx: number, field: string, val: string) => {
		setSections(p => {
			const next = [...p]
			next[sIdx][field] = val
			return next
		})
	}

	const handleProblemChange = (sIdx: number, pIdx: number, field: string, val: string) => {
		setSections(p => {
			const next = [...p]
			next[sIdx].problems[pIdx][field] = field === 'points' ? Number(val) : val
			return next
		})
	}

	const handleSubmit = async (e: React.FormEvent) => {
		e.preventDefault()
		if (!title.trim()) { setError('Title is required'); return }
		setError('')
		setSubmitting(true)
		try {
			await api.training.create({
				title,
				description: desc,
				organization_id: orgId || undefined,
				sections: sections.map(s => ({
					title: s.title,
					description: s.description,
					problems: s.problems.filter((x: any) => x.problem_id.trim())
				}))
			})
			nav('/training')
		} catch (err: any) {
			setError(err.message || 'Failed to create training plan')
		} finally {
			setSubmitting(false)
		}
	}

	return (
		<div className="max-w-2xl mx-auto space-y-6 my-10">
			<h1 className="text-2xl font-bold text-gray-900">Create Training Plan</h1>
			{error && <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-2 rounded text-sm">{error}</div>}
			<form onSubmit={handleSubmit} className="space-y-6 bg-white border border-gray-200 rounded-lg p-6 shadow-sm">
				<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
					<div>
						<label className="block text-sm font-medium text-gray-700 mb-1">Title</label>
						<input required value={title} onChange={e => setTitle(e.target.value)}
							placeholder="e.g. Master DP in 30 Days"
							className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white" />
					</div>
					<div>
						<label className="block text-sm font-medium text-gray-700 mb-1">Organization Scope <span className="text-gray-400 font-normal">(optional)</span></label>
						<select value={orgId} onChange={e => setOrgId(e.target.value)}
							className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white">
							<option value="">Public (Open to All)</option>
							{myOrgs.map(o => (
								<option key={o.id} value={o.id}>{o.name}</option>
							))}
						</select>
					</div>
				</div>
				<div>
					<label className="block text-sm font-medium text-gray-700 mb-1">Description</label>
					<textarea rows={3} value={desc} onChange={e => setDesc(e.target.value)}
						placeholder="Add curriculum outline or guidelines..."
						className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white" />
				</div>

				<div className="border-t border-gray-200 pt-6 space-y-4">
					<div className="flex justify-between items-center">
						<h2 className="text-lg font-bold text-gray-800">Sections & Curriculum</h2>
						<button type="button" onClick={handleAddSection}
							className="text-xs bg-blue-50 hover:bg-blue-100 text-blue-600 px-3 py-1.5 rounded font-semibold border border-blue-100">
							Add Section
						</button>
					</div>

					{sections.map((s, sIdx) => (
						<div key={sIdx} className="border border-gray-200 rounded-lg p-5 bg-gray-50 space-y-3">
							<div className="flex justify-between items-center">
								<input required value={s.title} onChange={e => handleSectionChange(sIdx, 'title', e.target.value)}
									placeholder="Section Title"
									className="border-b border-gray-300 font-semibold text-gray-800 focus:outline-none bg-transparent" />
								<button type="button" onClick={() => handleAddProblem(sIdx)}
									className="text-xs bg-blue-600 hover:bg-blue-700 text-white px-2 py-1 rounded">
									+ Add Problem
								</button>
							</div>
							<input value={s.description} onChange={e => handleSectionChange(sIdx, 'description', e.target.value)}
								placeholder="Section description (optional)"
								className="w-full border border-gray-200 rounded-md px-3 py-1.5 text-xs bg-white focus:outline-none" />

							<div className="space-y-2">
								{s.problems.map((p: any, pIdx: number) => (
									<div key={pIdx} className="flex gap-3 items-center">
										<input required value={p.problem_id} onChange={e => handleProblemChange(sIdx, pIdx, 'problem_id', e.target.value)}
											placeholder="Problem ID (e.g. p1)"
											className="border border-gray-300 rounded-md px-3 py-1.5 text-xs bg-white focus:outline-none flex-1" />
										<input type="number" value={p.points} onChange={e => handleProblemChange(sIdx, pIdx, 'points', e.target.value)}
											placeholder="Points"
											className="border border-gray-300 rounded-md px-3 py-1.5 text-xs bg-white focus:outline-none w-20" />
									</div>
								))}
							</div>
						</div>
					))}
				</div>

				<button type="submit" disabled={submitting}
					className="w-full bg-blue-600 text-white py-2.5 rounded-md font-bold hover:bg-blue-700 disabled:opacity-50 transition-colors">
					{submitting ? 'Creating plan...' : 'Create Training Plan'}
				</button>
			</form>
		</div>
	)
}
