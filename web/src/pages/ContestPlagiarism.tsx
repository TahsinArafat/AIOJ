import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'

export default function ContestPlagiarism() {
	const { id } = useParams<{ id: string }>()
	const [report, setReport] = useState<any>(null)
	const [pairs, setPairs] = useState<any[]>([])
	const [loading, setLoading] = useState(true)
	const [checking, setChecking] = useState(false)
	const [isAdmin, setIsAdmin] = useState(false)

	useEffect(() => {
		const token = getAccessToken()
		if (token) {
			try {
				const payload = JSON.parse(atob(token.split('.')[1]))
				setIsAdmin(payload.role === 'admin' || payload.role === 'teacher')
			} catch {}
		}
		fetchReport()
		const interval = setInterval(fetchReport, 5000)
		return () => clearInterval(interval)
	}, [id])

	const fetchReport = () => {
		if (!id) return
		api.plagiarism.getReport(id).then(r => {
			setReport(r || null)
			if (r?.id) {
				api.plagiarism.listPairs(id, r.id, 0, 50).then(d => {
					setPairs(d.data || [])
				})
			}
			setLoading(false)
		}).catch(() => setLoading(false))
	}

	const handleRunCheck = async () => {
		if (!id) return
		setChecking(true)
		try {
			await api.plagiarism.runCheck(id, 0.70)
			fetchReport()
		} catch (e: any) { alert(e.message) }
		finally { setChecking(false) }
	}

	const handleFlag = async (pairId: string) => {
		if (!id) return
		await api.plagiarism.updatePair(id, pairId, 'flagged')
		fetchReport()
	}

	const handleIgnore = async (pairId: string) => {
		if (!id) return
		await api.plagiarism.updatePair(id, pairId, 'ignored')
		fetchReport()
	}

	const similarityColor = (sim: number) => {
		if (sim >= 0.85) return 'bg-red-500'
		if (sim >= 0.60) return 'bg-orange-500'
		if (sim >= 0.35) return 'bg-yellow-500'
		return 'bg-green-500'
	}

	if (loading) return <div className="text-center py-20 text-gray-400">Loading plagiarism report...</div>

	return (
		<div className="space-y-6">
			<div className="flex items-center justify-between">
				<div>
					<h1 className="text-2xl font-bold">Plagiarism Report</h1>
					<p className="text-gray-500 text-sm">Review suspicious submission pairs and flag cases.</p>
				</div>
				{isAdmin && (
					<button onClick={handleRunCheck} disabled={checking || report?.status === 'running'}
						className="bg-blue-600 text-white px-4 py-2 rounded text-sm font-semibold hover:bg-blue-700 disabled:opacity-50 transition-colors">
						{checking || report?.status === 'running' ? 'Checking...' : 'Run Plagiarism Check'}
					</button>
				)}
			</div>

			{report && (
				<div className="grid grid-cols-4 gap-4">
					{[
						{ label: 'Status', value: report.status?.toUpperCase(), cls: report.status === 'completed' ? 'text-green-700' : 'text-blue-700' },
						{ label: 'Threshold', value: `${(report.threshold * 100).toFixed(0)}%` },
						{ label: 'Total Pairs', value: report.total_pairs },
						{ label: 'Flagged Pairs', value: pairs.filter((p: any) => p.status === 'flagged').length },
					].map(s => (
						<div key={s.label} className="bg-white border border-gray-200 rounded-lg px-4 py-3 text-center">
							<div className="text-xs text-gray-500 uppercase mb-1">{s.label}</div>
							<div className={`font-bold text-lg ${s.cls || 'text-gray-900'}`}>{s.value}</div>
						</div>
					))}
				</div>
			)}

			<div className="bg-white border border-gray-200 rounded-lg overflow-hidden">
				<table className="w-full text-sm">
					<thead className="bg-gray-50 text-gray-500 text-xs font-medium border-b">
						<tr>
							<th className="px-4 py-3 text-left">Users</th>
							<th className="px-4 py-3 text-left">Problem</th>
							<th className="px-4 py-3 text-center">Similarity</th>
							<th className="px-4 py-3 text-center">Matched</th>
							<th className="px-4 py-3 text-center">Status</th>
							<th className="px-4 py-3 text-center">Actions</th>
						</tr>
					</thead>
					<tbody className="divide-y divide-gray-100">
						{pairs.map((p: any) => (
							<tr key={p.id} className="hover:bg-gray-50">
								<td className="px-4 py-3 font-mono text-xs">
									<span className="font-semibold text-gray-900">{p.user_a_username}</span>
									<span className="text-gray-400 mx-1">↔</span>
									<span className="font-semibold text-gray-900">{p.user_b_username}</span>
									<div className="text-gray-400 mt-0.5">{p.submission_a_lang} / {p.submission_b_lang}</div>
								</td>
								<td className="px-4 py-3 text-xs text-gray-700">{p.problem_title}</td>
								<td className="px-4 py-3 text-center">
									<div className="flex items-center gap-2 justify-center">
										<div className="w-20 bg-gray-200 rounded-full h-2">
											<div className={`h-2 rounded-full ${similarityColor(p.similarity)}`} style={{ width: `${p.similarity * 100}%` }}></div>
										</div>
										<span className="text-xs font-semibold">{(p.similarity * 100).toFixed(0)}%</span>
									</div>
								</td>
								<td className="px-4 py-3 text-center text-gray-500 text-xs">{p.matched_lines}</td>
								<td className="px-4 py-3 text-center">
									<span className={`px-2 py-0.5 rounded text-xs font-medium ${
										p.status === 'flagged' ? 'bg-red-100 text-red-700' : 
										p.status === 'ignored' ? 'bg-gray-100 text-gray-500' : 'bg-yellow-100 text-yellow-700'
									}`}>
										{p.status}
									</span>
								</td>
								<td className="px-4 py-3 text-center flex gap-2 justify-center">
									{p.status === 'pending' && (
										<>
											<button onClick={() => handleFlag(p.id)} className="text-xs bg-red-50 text-red-600 px-2 py-1 rounded hover:bg-red-100 border border-red-100 font-medium">
												Flag
											</button>
											<button onClick={() => handleIgnore(p.id)} className="text-xs bg-gray-50 text-gray-600 px-2 py-1 rounded hover:bg-gray-100 border border-gray-200 font-medium">
												Ignore
											</button>
										</>
									)}
								</td>
							</tr>
						))}
						{pairs.length === 0 && (
							<tr>
								<td colSpan={6} className="px-6 py-16 text-center text-gray-400">No suspicious pairs found.</td>
							</tr>
						)}
					</tbody>
				</table>
			</div>
		</div>
	)
}
