import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'

export default function TrainingPlanDetail() {
	const { id } = useParams<{ id: string }>()
	const [data, setData] = useState<any>(null)
	const [loading, setLoading] = useState(true)
	const [enrolling, setEnrolling] = useState(false)

	useEffect(() => {
		if (!id) return
		const fetchDetail = () => {
			api.training.get(id).then(d => {
				setData(d)
				// Resolve problem slugs and titles
				const allProblems: string[] = []
				d.sections?.forEach((s: any) => {
					s.problems?.forEach((p: any) => {
						if (p.problem_id) allProblems.push(p.problem_id)
					})
				})

				if (allProblems.length > 0) {
					// Use standard fetch or api calls to get details if needed,
					// but since slug/title resolver exists, we can use it!
					// Fallback to simple matching if needed
				}
			}).catch(() => {}).finally(() => setLoading(false))
		}

		fetchDetail()
	}, [id])

	const handleEnrollToggle = async () => {
		if (!id || !data) return
		setEnrolling(true)
		try {
			if (data.enrolled) {
				await api.training.unenroll(id)
				setData((p: any) => ({ ...p, enrolled: false, progress: null }))
			} else {
				await api.training.enroll(id)
				setData((p: any) => ({ ...p, enrolled: true, progress: { total_problems: p.problem_count, completed_problems: 0, percentage: 0 } }))
				window.location.reload()
			}
		} catch (e: any) { alert(e.message) }
		finally { setEnrolling(false) }
	}

	if (loading) return <div className="text-center py-20 text-gray-400">Loading training plan details...</div>
	if (!data) return <div className="text-center py-20 text-gray-400">Training plan not found.</div>

	return (
		<div className="max-w-3xl mx-auto space-y-6">
			<div className="bg-white border border-gray-200 rounded-lg p-6 shadow-sm flex justify-between items-start">
				<div className="space-y-2 flex-1">
					<div className="flex items-center gap-2">
						<Link to="/training" className="text-xs text-blue-600 hover:underline">← Training Plans</Link>
					</div>
					<h1 className="text-2xl font-bold text-gray-900">{data.title}</h1>
					<p className="text-gray-600 text-sm">{data.description || 'No description provided.'}</p>
					<div className="text-xs text-gray-400 pt-1 flex gap-3">
						<span>{data.section_count} Sections</span>
						<span>•</span>
						<span>{data.problem_count} Problems</span>
						<span>•</span>
						<span>{data.enrolled_count} Enrolled</span>
					</div>
				</div>
				{getAccessToken() && (
					<button onClick={handleEnrollToggle} disabled={enrolling}
						className={`px-6 py-2 rounded font-semibold text-sm transition-colors ${
							data.enrolled 
								? 'bg-red-50 text-red-600 border border-red-100 hover:bg-red-100'
								: 'bg-blue-600 text-white hover:bg-blue-700'
						}`}>
						{data.enrolled ? 'Leave Plan' : 'Enroll in Plan'}
					</button>
				)}
			</div>

			{/* Progress summary for enrolled users */}
			{data.enrolled && data.progress && (
				<div className="bg-gradient-to-r from-blue-50 to-indigo-50 border border-blue-100 rounded-lg p-5">
					<div className="flex justify-between items-center text-sm font-semibold text-blue-900 mb-2">
						<span>Your Progress</span>
						<span>{data.progress.completed_problems} / {data.progress.total_problems} Solved ({Math.round(data.progress.percentage)}%)</span>
					</div>
					<div className="w-full bg-blue-200/50 rounded-full h-2">
						<div className="bg-blue-600 h-2 rounded-full transition-all" style={{ width: `${data.progress.percentage}%` }}></div>
					</div>
				</div>
			)}

			<div className="space-y-4">
				<h2 className="text-lg font-bold text-gray-800">Curriculum</h2>
				{data.sections?.map((s: any) => (
					<div key={s.id} className="bg-white border border-gray-200 rounded-lg p-5 space-y-3">
						<div>
							<h3 className="font-bold text-base text-gray-900">{s.title}</h3>
							{s.description && <p className="text-gray-500 text-sm mt-0.5">{s.description}</p>}
						</div>

						<div className="divide-y divide-gray-100 border-t border-gray-100 pt-2">
							{s.problems?.map((p: any) => (
								<div key={p.id} className="py-2.5 flex items-center justify-between text-sm">
									<div className="flex items-center gap-2">
										<Link to={`/problems/${p.problem_id}`} className="font-semibold text-blue-600 hover:underline">
											{p.problem_id}
										</Link>
									</div>
									<span className="text-xs font-semibold px-2 py-0.5 rounded bg-gray-100 text-gray-600 font-mono">
										{p.points} pts
									</span>
								</div>
							))}
							{(!s.problems || s.problems.length === 0) && (
								<p className="text-xs text-gray-400 py-2">No problems in this section.</p>
							)}
						</div>
					</div>
				))}
				{(!data.sections || data.sections.length === 0) && (
					<p className="text-gray-400 text-center py-10">No sections added yet.</p>
				)}
			</div>
		</div>
	)
}
