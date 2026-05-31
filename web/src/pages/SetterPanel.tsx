import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'

export default function SetterPanel() {
    const [problems, setProblems] = useState<any[]>([])
    const [showImport, setShowImport] = useState(false)
    const [showCSESImport, setShowCSESImport] = useState(false)
    const [contestId, setContestId] = useState('')
    const [problemIndex, setProblemIndex] = useState('')
    const [csesProblemId, setCSESProblemId] = useState('')
    const [importing, setImporting] = useState(false)

    const loadData = () => {
        api.problems.list().then(d => setProblems(d.data || [])).catch(console.error)
    }

    useEffect(() => { loadData() }, [])

    const handleImport = async () => {
        if (!contestId || !problemIndex) return alert('Contest ID and Problem Index are required')
        setImporting(true)
        try {
            const result = await api.problems.importCodeforces(contestId, problemIndex)
            alert(`Imported: ${result.slug}`)
            setShowImport(false)
            setContestId('')
            setProblemIndex('')
            loadData()
        } catch (e: any) {
            alert(e.message)
        } finally {
            setImporting(false)
        }
    }

    const handleCSESImport = async () => {
        if (!csesProblemId) return alert('Problem ID is required')
        setImporting(true)
        try {
            const result = await api.problems.importCSES(csesProblemId)
            alert(`Imported: ${result.slug}`)
            setShowCSESImport(false)
            setCSESProblemId('')
            loadData()
        } catch (e: any) {
            alert(e.message)
        } finally {
            setImporting(false)
        }
    }

    return (
        <div>
            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold">Problem Setter Workspace</h1>
                <div className="flex gap-2">
                    <button onClick={() => { setShowImport(!showImport); setShowCSESImport(false) }}
                        className="bg-purple-600 text-white px-4 py-2 rounded text-sm hover:bg-purple-700 transition-colors">
                        Import from CF
                    </button>
                    <button onClick={() => { setShowCSESImport(!showCSESImport); setShowImport(false) }}
                        className="bg-teal-600 text-white px-4 py-2 rounded text-sm hover:bg-teal-700 transition-colors">
                        Import from CSES
                    </button>
                    <Link to="/setter/contest/create" className="bg-green-600 text-white px-4 py-2 rounded text-sm hover:bg-green-700 transition-colors">+ Create Contest</Link>
                    <Link to="/setter/create" className="bg-blue-600 text-white px-4 py-2 rounded text-sm hover:bg-blue-700 transition-colors">+ Create Problem</Link>
                </div>
            </div>

            {showImport && (
                <div className="bg-purple-50 border border-purple-200 rounded-lg p-4 mb-6">
                    <h3 className="font-semibold text-sm text-purple-800 mb-3">Import from Codeforces</h3>
                    <p className="text-xs text-gray-500 mb-3">Enter the Contest ID and Problem Index (e.g. Contest ID: 1, Problem Index: A)</p>
                    <div className="flex gap-3 items-end">
                        <div>
                            <label className="block text-xs font-medium text-gray-500 mb-1">Contest ID</label>
                            <input type="text" value={contestId} onChange={e => setContestId(e.target.value)}
                                placeholder="e.g. 1, 1234" className="border rounded px-3 py-2 text-sm w-32" />
                        </div>
                        <div>
                            <label className="block text-xs font-medium text-gray-500 mb-1">Problem Index</label>
                            <input type="text" value={problemIndex} onChange={e => setProblemIndex(e.target.value)}
                                placeholder="e.g. A, B, C" className="border rounded px-3 py-2 text-sm w-24" />
                        </div>
                        <button onClick={handleImport} disabled={importing}
                            className="bg-purple-600 text-white px-4 py-2 rounded text-sm hover:bg-purple-700 disabled:opacity-50">
                            {importing ? 'Importing...' : 'Import'}
                        </button>
                        <button onClick={() => setShowImport(false)} className="text-gray-500 text-sm px-2">Cancel</button>
                    </div>
                </div>
            )}

            {showCSESImport && (
                <div className="bg-teal-50 border border-teal-200 rounded-lg p-4 mb-6">
                    <h3 className="font-semibold text-sm text-teal-800 mb-3">Import from CSES</h3>
                    <p className="text-xs text-gray-500 mb-3">Enter the CSES Problem ID (e.g. 1068 for Weird Algorithm)</p>
                    <div className="flex gap-3 items-end">
                        <div>
                            <label className="block text-xs font-medium text-gray-500 mb-1">Problem ID</label>
                            <input type="text" value={csesProblemId} onChange={e => setCSESProblemId(e.target.value)}
                                placeholder="e.g. 1068, 1069" className="border rounded px-3 py-2 text-sm w-32" />
                        </div>
                        <button onClick={handleCSESImport} disabled={importing}
                            className="bg-teal-600 text-white px-4 py-2 rounded text-sm hover:bg-teal-700 disabled:opacity-50">
                            {importing ? 'Importing...' : 'Import'}
                        </button>
                        <button onClick={() => setShowCSESImport(false)} className="text-gray-500 text-sm px-2">Cancel</button>
                    </div>
                </div>
            )}

            <section>
                <h2 className="text-lg font-semibold mb-3">Problems (Public List)</h2>
                <div className="border border-gray-200 rounded-lg overflow-hidden">
                    <table className="w-full text-sm">
                        <thead className="bg-gray-50 text-gray-500 text-xs uppercase">
                            <tr>
                                <th className="px-4 py-3 text-left">Title</th>
                                <th className="px-4 py-3 text-left">Source</th>
                                <th className="px-4 py-3 text-left">Difficulty</th>
                                <th className="px-4 py-3 text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-gray-100">
                            {problems.map(p => (
                                <tr key={p.id}>
                                    <td className="px-4 py-3 font-medium">{p.title}</td>
                                    <td className="px-4 py-3 text-xs text-gray-500">{p.source || 'local'}</td>
                                    <td className="px-4 py-3">
                                        <span className={`px-2 py-1 rounded text-xs font-medium ${
                                            p.difficulty === 'easy' ? 'bg-green-100 text-green-800' :
                                            p.difficulty === 'medium' ? 'bg-yellow-100 text-yellow-800' : 'bg-red-100 text-red-800'
                                        }`}>{p.difficulty}</span>
                                    </td>
                                    <td className="px-4 py-3 text-right flex gap-2 justify-end items-center">
                                        <Link to={`/problems/${p.slug}`} className="text-blue-600 hover:underline text-xs">View</Link>
                                        <Link to={`/setter/${p.slug}`} className="bg-orange-50 hover:bg-orange-100 border border-orange-200 text-orange-700 px-2.5 py-1 rounded text-xs">Edit</Link>
                                    </td>
                                </tr>
                            ))}
                            {problems.length === 0 && (
                                <tr><td colSpan={4} className="px-4 py-8 text-center text-gray-400">No problems yet.</td></tr>
                            )}
                        </tbody>
                    </table>
                </div>
            </section>
        </div>
    )
}
