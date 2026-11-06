import { useCallback, useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api } from '../lib/api'
import type { PlagiarismPairDetail, PlagiarismReport } from '../types/api'
import { errorMessage } from '../lib/errors'
import { EmptyState } from '../components/EmptyState'
import { Badge, type BadgeTone } from '../components/ui/Badge'
import { Button } from '../components/ui/Button'
import { PageHeader, Skeleton } from '../components/ui/PageHeader'
import { Table, TableShell, Td, Th, THead, Tr } from '../components/ui/Table'

const PAIR_STATUS: Record<string, { label: string; tone: BadgeTone }> = {
	flagged: { label: 'Flagged', tone: 'bad' },
	ignored: { label: 'Ignored', tone: 'neutral' },
	pending: { label: 'Pending review', tone: 'warn' },
}

/**
 * Similarity is bucketed once, here. The old function had a dead branch
 * (`>= 0.60` and `>= 0.35` both returned the same colour), so a 40% match and
 * a 70% match looked identical.
 */
function similarityTone(ratio: number): { bar: string; label: string } {
	if (ratio >= 0.85) return { bar: 'bg-bad/100', label: 'Very high' }
	if (ratio >= 0.60) return { bar: 'bg-warn-fill', label: 'High' }
	if (ratio >= 0.35) return { bar: 'bg-warn/60', label: 'Moderate' }
	return { bar: 'bg-ok/100', label: 'Low' }
}

export default function ContestPlagiarism() {
	const { id } = useParams<{ id: string }>()
	const [report, setReport] = useState<PlagiarismReport | null>(null)
	const [pairs, setPairs] = useState<PlagiarismPairDetail[]>([])
	const [loading, setLoading] = useState(true)
	const [checking, setChecking] = useState(false)
	const [err, setErr] = useState('')
	// A lazy initializer, not an effect: reading the token is not a subscription,
	// and the effect version triggered a cascading render on every mount.
	const [isAdmin] = useState(() => {
		try {
			const token = localStorage.getItem('access_token')
			if (!token) return false
			return JSON.parse(atob(token.split('.')[1])).role === 'admin'
		} catch {
			// A malformed token simply means "not an admin" here.
			return false
		}
	})

	// Two calls, not one: `/plagiarism/report` returns the report object
	// itself (a flat body), and the pairs hang off its id. Reading `.report`
	// and `.data` off a single response silently yielded `undefined` for both.
	const fetchReport = useCallback(async () => {
		if (!id) return
		try {
			const r = await api.plagiarism.getReport(id)
			setReport(r || null)
			if (r?.id) {
				const d = await api.plagiarism.listPairs(id, r.id, 0, 50)
				setPairs(d.data || [])
			} else {
				setPairs([])
			}
			setErr('')
		} catch (e) {
			// A missing report is a normal state ("nothing run yet"), not an error.
			const msg = errorMessage(e)
			if (msg && !/not found|no report/i.test(msg)) {
				setErr(msg)
			}
		} finally {
			setLoading(false)
		}
	}, [id])

	useEffect(() => {
		// deferred a microtask: react-hooks/set-state-in-effect
		queueMicrotask(fetchReport)
	}, [fetchReport])

	// Poll while a check is running so the table fills in without a manual reload.
	useEffect(() => {
		if (report?.status !== 'running') return
		const interval = setInterval(() => { void fetchReport() }, 5000)
		return () => clearInterval(interval)
	}, [report?.status, fetchReport])

	const handleRunCheck = async () => {
		if (!id) return
		setChecking(true)
		setErr('')
		try {
			await api.plagiarism.runCheck(id, 0.70)
			await fetchReport()
		} catch (e) {
			setErr(errorMessage(e) || 'Failed to start the check')
		} finally {
			setChecking(false)
		}
	}

	const handleFlag = async (pairId: string) => {
		if (!id) return
		try {
			await api.plagiarism.updatePair(id, pairId, 'flagged')
			await fetchReport()
		} catch (e) {
			setErr(errorMessage(e) || 'Failed to flag the pair')
		}
	}

	const handleIgnore = async (pairId: string) => {
		if (!id) return
		try {
			await api.plagiarism.updatePair(id, pairId, 'ignored')
			await fetchReport()
		} catch (e) {
			setErr(errorMessage(e) || 'Failed to ignore the pair')
		}
	}

	const flaggedCount = pairs.filter(p => p.status === 'flagged').length
	const running = checking || report?.status === 'running'

	return (
		<div>
			<PageHeader
				eyebrow={
					<Link to={`/contests/${id}`} className="hover:text-ink">
						← Back to contest
					</Link>
				}
				title="Plagiarism report"
				description="Review suspicious submission pairs and flag cases for follow-up."
				actions={
					isAdmin && (
						<Button onClick={handleRunCheck} disabled={running}>
							{running ? 'Checking…' : 'Run plagiarism check'}
						</Button>
					)
				}
			/>

			{err && (
				<div role="alert" className="mb-4 rounded-lg border border-bad/30 bg-bad/10 px-3 py-2 text-sm text-bad">
					{err}
				</div>
			)}

			{report && (
				<dl className="mb-5 grid grid-cols-2 gap-3 sm:grid-cols-4">
					<Stat
						label="Status"
						value={(report.status || '—').toUpperCase()}
						tone={report.status === 'completed' ? 'text-ok' : 'text-brand'}
					/>
					<Stat label="Threshold" value={`${((report.threshold ?? 0) * 100).toFixed(0)}%`} />
					<Stat label="Total pairs" value={report.total_pairs ?? 0} />
					<Stat label="Flagged" value={flaggedCount} tone={flaggedCount > 0 ? 'text-bad' : undefined} />
				</dl>
			)}

			{loading ? (
				<div className="space-y-2">
					{Array.from({ length: 5 }).map((_, i) => <Skeleton key={i} className="h-11 w-full" />)}
				</div>
			) : pairs.length === 0 ? (
				<div className="rounded-xl border border-hairline bg-surface">
					<EmptyState
						text="No suspicious pairs found"
						description="Run a plagiarism check — flagged submission pairs will appear here."
					/>
				</div>
			) : (
				<TableShell>
					<Table>
						<THead>
							<tr>
								<Th>Users</Th>
								<Th className="hidden md:table-cell">Problem</Th>
								<Th className="w-48">Similarity</Th>
								<Th className="hidden text-center sm:table-cell">Matched</Th>
								<Th className="w-36">Status</Th>
								<Th className="w-40 text-right">Actions</Th>
							</tr>
						</THead>
						<tbody>
							{pairs.map(p => {
								const ratio = p.similarity ?? 0
								const tone = similarityTone(ratio)
								const status = PAIR_STATUS[p.status] ?? { label: p.status, tone: 'neutral' as BadgeTone }
								return (
									<Tr key={p.id}>
										<Td>
											<div className="flex flex-wrap items-center gap-1">
												<span className="font-medium text-ink">{p.user_a_username}</span>
												<span aria-hidden="true" className="text-muted-soft">↔</span>
												<span className="font-medium text-ink">{p.user_b_username}</span>
											</div>
											<p className="mt-0.5 text-xs text-muted-soft">
												{p.submission_a_lang} / {p.submission_b_lang}
											</p>
											{/* Problem column is hidden on narrow screens, so keep the
											    title reachable there. */}
											<p className="mt-0.5 text-xs text-muted md:hidden">{p.problem_title}</p>
										</Td>
										<Td className="hidden text-xs text-ink-soft md:table-cell">{p.problem_title}</Td>
										<Td>
											<div className="flex items-center gap-2">
												<div className="h-2 w-full max-w-[6rem] rounded-full bg-surface-3">
													<div
														className={`h-2 rounded-full ${tone.bar}`}
														style={{ width: `${Math.min(100, Math.max(0, ratio * 100))}%` }}
													/>
												</div>
												<span className="tnum shrink-0 text-xs font-medium" title={tone.label}>
													{(ratio * 100).toFixed(0)}%
												</span>
											</div>
										</Td>
										<Td className="tnum hidden text-center text-xs text-muted sm:table-cell">
											{p.matched_lines ?? '—'}
										</Td>
										<Td>
											<Badge tone={status.tone}>{status.label}</Badge>
										</Td>
										<Td className="text-right">
											{p.status === 'pending' && (
												<div className="flex justify-end gap-1.5">
													<Button variant="danger" size="sm" onClick={() => handleFlag(p.id)}>Flag</Button>
													<Button variant="secondary" size="sm" onClick={() => handleIgnore(p.id)}>Ignore</Button>
												</div>
											)}
										</Td>
									</Tr>
								)
							})}
						</tbody>
					</Table>
				</TableShell>
			)}
		</div>
	)
}

function Stat({ label, value, tone }: { label: string; value: string | number; tone?: string }) {
	return (
		<div className="rounded-xl border border-hairline bg-surface px-4 py-3">
			<dt className="text-[11px] font-medium uppercase tracking-wide text-muted">{label}</dt>
			<dd className={`tnum mt-1 text-lg font-semibold ${tone || 'text-ink'}`}>{value}</dd>
		</div>
	)
}
