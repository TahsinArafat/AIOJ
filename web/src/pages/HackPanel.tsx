import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { useToast } from '../components/Toast'

interface HackSubmission { id: string; language: string; user_id: string }
interface HackResult { success?: boolean; status?: string; actual_output?: string; expected_output?: string }

export default function HackPanel() {
    const toast = useToast()
    const { contestId, problemId } = useParams<{ contestId: string; problemId: string }>()
    const navigate = useNavigate()
    const [submissions, setSubmissions] = useState<HackSubmission[]>([])
    const [selectedSub, setSelectedSub] = useState('')
    const [testInput, setTestInput] = useState('')
    const [loading, setLoading] = useState(true)
    const [result, setResult] = useState<HackResult | null>(null)
    const [submitting, setSubmitting] = useState(false)

    useEffect(() => {
        if (!contestId || !problemId) return
        api.hacks.listHackable(contestId, problemId).then(d => {
            setSubmissions(d.data || [])
        }).catch(console.error).finally(() => setLoading(false))
    }, [contestId, problemId])

    const handleHack = async () => {
        if (!contestId || !problemId || !selectedSub || !testInput.trim()) return
        setSubmitting(true)
        setResult(null)
        try {
            const res = await api.hacks.submit({
                contest_id: contestId,
                problem_id: problemId,
                submission_id: selectedSub,
                test_input: testInput,
            })
            setResult(res)
        } catch (e) {
            toast.error('Hack failed: ' + (e instanceof Error ? e.message : String(e)))
        } finally {
            setSubmitting(false)
        }
    }

    if (loading) return <div className="text-center py-20 text-muted-soft">Loading hackable submissions...</div>

    return (
        <div className="max-w-2xl mx-auto space-y-6">
            <div>
                <h1 className="text-2xl font-bold">Hack Contest Solution</h1>
                <p className="text-sm text-muted mt-1">Provide a counter-test case to challenge an accepted submission.</p>
            </div>

            <div className="border border-hairline rounded-lg p-6 bg-surface space-y-4">
                <div>
                    <label className="block text-sm font-semibold text-ink-soft mb-1">Select Target Submission</label>
                    <select aria-label="Select Target Submission" value={selectedSub} onChange={e => setSelectedSub(e.target.value)} className="w-full border rounded px-3 py-2 text-sm">
                        <option value="">-- Select Submission --</option>
                        {submissions.map(s => (
                            <option key={s.id} value={s.id}>
                                Submission {s.id.substring(0, 8)}... ({s.language}) — User {s.user_id.substring(0, 8)}...
                            </option>
                        ))}
                    </select>
                </div>

                <div>
                    <label className="block text-sm font-semibold text-ink-soft mb-1">Counter-Test Input</label>
                    <textarea aria-label="Counter-Test Input" value={testInput} onChange={e => setTestInput(e.target.value)} rows={6}
                        placeholder="Enter the test input to break the solution..." className="w-full border rounded px-3 py-2 text-sm font-mono" />
                </div>

                <div className="flex gap-4">
                    <button onClick={handleHack} disabled={submitting || !selectedSub || !testInput.trim()}
                        className="bg-danger-fill text-white px-6 py-2 rounded text-sm hover:bg-danger-fill-hover font-medium disabled:opacity-50">
                        {submitting ? 'Testing Hack...' : 'Submit Hack'}
                    </button>
                    <button onClick={() => navigate(-1)} className="border px-6 py-2 rounded text-sm hover:bg-surface-2">
                        Cancel
                    </button>
                </div>
            </div>

            {result && (
                <div className={`border rounded-lg p-6 ${result.success ? 'bg-ok/10 border-ok/30' : 'bg-bad/10 border-bad/30'}`}>
                    <h3 className={`font-semibold text-lg ${result.success ? 'text-ok' : 'text-bad'}`}>
                        {result.success ? '✓ Hack Successful!' : '✗ Hack Failed'}
                    </h3>
                    <p className="text-sm mt-2 text-ink-soft">Verdict: {result.status}</p>
                    {result.success && (
                        <div className="mt-4 grid grid-cols-2 gap-4 text-xs font-mono">
                            <div>
                                <p className="text-muted mb-1">Expected Output (Jury):</p>
                                <pre className="bg-surface p-3 border rounded overflow-x-auto">{result.expected_output}</pre>
                            </div>
                            <div>
                                <p className="text-muted mb-1">Actual Output (Defender):</p>
                                <pre className="bg-surface p-3 border rounded overflow-x-auto">{result.actual_output}</pre>
                            </div>
                        </div>
                    )}
                </div>
            )}
        </div>
    )
}
