import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'
import { EmptyState } from '../components/EmptyState'
import { useToast } from '../components/Toast'
import { errorMessage } from '../lib/errors'

interface PlagiarismReportInfo { status?: string; threshold?: number; total_pairs?: number }
interface PlagiarismPairRow { id: string; matched_lines?: number; problem_title?: string; similarity?: number; status?: string; submission_a_lang?: string; submission_b_lang?: string; user_a_username?: string; user_b_username?: string }

export default function ContestPlagiarism() {
	const toast = useToast()
	const { id } = useParams<{ id: string }>()
	const [report, setReport] = useState<PlagiarismReportInfo | null>(null)
	const [pairs, setPairs] = useState<PlagiarismPairRow[]>([])
	const [loading, setLoading] = useState(true)
	const [checking, setChecking] = useState(false)
	const [isAdmin] = useState(() => {
		const token = getAccessToken()
		if (!token) return false
		try {
			const payload = JSON.parse(atob(token.split('.')[1]))
			return payload.role === 'admin' || payload.role === 'setter'
		} catch { /* malformed JWT — keep default role */ return false }
	})

	useEffect(() => {
		fetchReport()
		const interval = setInterval(fetchReport, 5000)
		return () => clearInterval(interval)
	}, [id]) // eslint-disable-line react-hooks/exhaustive-deps

	function fetchReport() {
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
		} catch (e) { toast.error(errorMessage(e)) }
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

	if (loading) return <div className="text-center py-20 text-muted-soft">Loading plagiarism report...</div>

	return (
		<div className="space-y-6">
			<div className="flex items-center justify-between">
				<div>
					<h1 className="text-2xl font-bold">Plagiarism Report</h1>
					<p className="text-muted text-sm">Review suspicious submission pairs and flag cases.</p>
				</div>
				{isAdmin && (
					<button onClick={handleRunCheck} disabled={checking || report?.status === 'running'}
						className="bg-brand text-on-brand px-4 py-2 rounded text-sm font-semibold hover:bg-brand-hover disabled:opacity-50 transition-colors">
						{checking || report?.status === 'running' ? 'Checking...' : 'Run Plagiarism Check'}
					</button>
				)}
			</div>

			{report && (
				<div className="grid grid-cols-4 gap-4">
					{[
						{ label: 'Status', value: report.status?.toUpperCase(), cls: report.status === 'completed' ? 'text-green-700' : 'text-blue-700' },
						{ label: 'Threshold', value: `${((report.threshold ?? 0) * 100).toFixed(0)}%` },
						{ label: 'Total Pairs', value: report.total_pairs },
						{ label: 'Flagged Pairs', value: pairs.filter((p) => p.status === 'flagged').length },
					].map(s => (
						<div key={s.label} className="bg-surface border border-hairline rounded-lg px-4 py-3 text-center">
							<div className="text-xs text-muted uppercase mb-1">{s.label}</div>
							<div className={`font-bold text-lg ${s.cls || 'text-ink'}`}>{s.value}</div>
						</div>
					))}
				</div>
			)}

			<div className="bg-surface border border-hairline rounded-lg overflow-hidden">
				<table className="w-full text-sm">
					<thead className="bg-surface-2 text-muted text-xs font-medium border-b">
						<tr>
							<th className="px-4 py-3 text-left">Users</th>
							<th className="px-4 py-3 text-left">Problem</th>
							<th className="px-4 py-3 text-center">Similarity</th>
							<th className="px-4 py-3 text-center">Matched</th>
							<th className="px-4 py-3 text-center">Status</th>
							<th className="px-4 py-3 text-center">Actions</th>
						</tr>
					</thead>
					<tbody className="divide-y divide-hairline">
						{pairs.map((p) => (
							<tr key={p.id} className="hover:bg-surface-2">
								<td className="px-4 py-3 font-mono text-xs">
									<span className="font-semibold text-ink">{p.user_a_username}</span>
									<span className="text-muted-soft mx-1">↔</span>
									<span className="font-semibold text-ink">{p.user_b_username}</span>
									<div className="text-muted-soft mt-0.5">{p.submission_a_lang} / {p.submission_b_lang}</div>
								</td>
								<td className="px-4 py-3 text-xs text-ink-soft">{p.problem_title}</td>
								<td className="px-4 py-3 text-center">
									<div className="flex items-center gap-2 justify-center">
										<div className="w-20 bg-surface-3 rounded-full h-2">
											<div className={`h-2 rounded-full ${similarityColor(p.similarity ?? 0)}`} style={{ width: `${(p.similarity ?? 0) * 100}%` }}></div>
										</div>
										<span className="text-xs font-semibold">{((p.similarity ?? 0) * 100).toFixed(0)}%</span>
									</div>
								</td>
								<td className="px-4 py-3 text-center text-muted text-xs">{p.matched_lines}</td>
								<td className="px-4 py-3 text-center">
									<span className={`px-2 py-0.5 rounded text-xs font-medium ${p.status === 'flagged' ? 'bg-red-100 text-red-700' :
										p.status === 'ignored' ? 'bg-surface-3 text-muted' : 'bg-yellow-100 text-yellow-700'
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
											<button onClick={() => handleIgnore(p.id)} className="text-xs bg-surface-2 text-muted px-2 py-1 rounded hover:bg-surface-2 border border-hairline font-medium">
												Ignore
											</button>
										</>
									)}
								</td>
							</tr>
						))}
						{pairs.length === 0 && (
							<tr>
								<td colSpan={6} className="px-6 py-6">
									<EmptyState text="No suspicious pairs found" description="Run a plagiarism check — flagged submission pairs will appear here." />
								</td>
							</tr>
						)}
					</tbody>
				</table>
			</div>
		</div>
	)
}
