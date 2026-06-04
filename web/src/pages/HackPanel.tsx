import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api } from '../lib/api'

export default function HackPanel() {
    const { contestId, problemId } = useParams<{ contestId: string; problemId: string }>()
    const navigate = useNavigate()
    const [submissions, setSubmissions] = useState<any[]>([])
    const [selectedSub, setSelectedSub] = useState('')
    const [testInput, setTestInput] = useState('')
    const [loading, setLoading] = useState(true)
    const [result, setResult] = useState<any>(null)
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
        } catch (e: any) {
            alert('Hack failed: ' + e.message)
        } finally {
            setSubmitting(false)
        }
    }

    if (loading) return <div className="text-center py-20 text-gray-400 dark:text-gray-500">Loading hackable submissions...</div>

    return (
        <div className="max-w-2xl mx-auto space-y-6">
            <div>
                <h1 className="text-2xl font-bold">Hack Contest Solution</h1>
                <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">Provide a counter-test case to challenge an accepted submission.</p>
            </div>

            <div className="border border-gray-200 dark:border-gray-700 rounded-lg p-6 bg-white dark:bg-gray-800 space-y-4">
                <div>
                    <label className="block text-sm font-semibold text-gray-700 dark:text-gray-300 mb-1">Select Target Submission</label>
                    <select value={selectedSub} onChange={e => setSelectedSub(e.target.value)} className="w-full border rounded px-3 py-2 text-sm">
                        <option value="">-- Select Submission --</option>
                        {submissions.map(s => (
                            <option key={s.id} value={s.id}>
                                Submission {s.id.substring(0, 8)}... ({s.language}) — User {s.user_id.substring(0, 8)}...
                            </option>
                        ))}
                    </select>
                </div>

                <div>
                    <label className="block text-sm font-semibold text-gray-700 dark:text-gray-300 mb-1">Counter-Test Input</label>
                    <textarea value={testInput} onChange={e => setTestInput(e.target.value)} rows={6}
                        placeholder="Enter the test input to break the solution..." className="w-full border rounded px-3 py-2 text-sm font-mono" />
                </div>

                <div className="flex gap-4">
                    <button onClick={handleHack} disabled={submitting || !selectedSub || !testInput.trim()}
                        className="bg-red-600 text-white px-6 py-2 rounded text-sm hover:bg-red-700 font-medium disabled:opacity-50">
                        {submitting ? 'Testing Hack...' : 'Submit Hack'}
                    </button>
                    <button onClick={() => navigate(-1)} className="border px-6 py-2 rounded text-sm hover:bg-gray-50 dark:hover:bg-gray-700">
                        Cancel
                    </button>
                </div>
            </div>

            {result && (
                <div className={`border rounded-lg p-6 ${result.success ? 'bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-800' : 'bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-800'}`}>
                    <h3 className={`font-semibold text-lg ${result.success ? 'text-green-800' : 'text-red-800'}`}>
                        {result.success ? '✓ Hack Successful!' : '✗ Hack Failed'}
                    </h3>
                    <p className="text-sm mt-2 text-gray-700 dark:text-gray-300">Verdict: {result.status}</p>
                    {result.success && (
                        <div className="mt-4 grid grid-cols-2 gap-4 text-xs font-mono">
                            <div>
                                <p className="text-gray-500 dark:text-gray-400 mb-1">Expected Output (Jury):</p>
                                <pre className="bg-white dark:bg-gray-800 p-3 border rounded overflow-x-auto">{result.expected_output}</pre>
                            </div>
                            <div>
                                <p className="text-gray-500 dark:text-gray-400 mb-1">Actual Output (Defender):</p>
                                <pre className="bg-white dark:bg-gray-800 p-3 border rounded overflow-x-auto">{result.actual_output}</pre>
                            </div>
                        </div>
                    )}
                </div>
            )}
        </div>
    )
}
