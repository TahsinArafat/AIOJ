import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'

function decodeRole(): string | null {
    const token = localStorage.getItem('access_token')
    if (!token) return null
    try {
        const payload = JSON.parse(atob(token.split('.')[1]))
        return payload.role ?? null
    } catch {
        return null
    }
}

export default function SetterPanel() {
    const [problems, setProblems] = useState<any[]>([])
    const [contests, setContests] = useState<any[]>([])
    const [activeTab, setActiveTab] = useState<'my-problems' | 'my-contests' | 'all-problems' | 'import'>('my-problems')
    const [contestId, setContestId] = useState('')
    const [problemIndex, setProblemIndex] = useState('')
    const [csesProblemId, setCSESProblemId] = useState('')
    const [atcoderContestId, setAtcoderContestId] = useState('')
    const [atcoderProblemId, setAtcoderProblemId] = useState('')
    const [tophProblemId, setTophProblemId] = useState('')
    const [qojProblemId, setQojProblemId] = useState('')
    const [importing, setImporting] = useState(false)
    const [importResults, setImportResults] = useState<{ success: string[]; failed: string[] } | null>(null)

    const loadData = () => {
        if (activeTab === 'my-problems') {
            api.problems.listMy(0, 50).then(d => setProblems(d.data || [])).catch(console.error)
        } else if (activeTab === 'all-problems') {
            api.problems.list(0, 50).then(d => setProblems(d.data || [])).catch(console.error)
        } else if (activeTab === 'my-contests') {
            api.contests.listMy(0, 50).then(d => setContests(d.data || [])).catch(console.error)
        }
    }

    useEffect(() => { loadData() }, [activeTab])

    const handleImport = async () => {
        if (!contestId || !problemIndex) return alert('Contest ID and Problem Index are required')
        setImporting(true)
        setImportResults(null)
        
        // Handle bulk problem index split by comma or whitespace
        const indices = problemIndex.split(/[\s,]+/).filter(Boolean)
        const success: string[] = []
        const failed: string[] = []

        await Promise.all(
            indices.map(async (idx) => {
                try {
                    const result = await api.problems.importCodeforces(contestId.trim(), idx)
                    success.push(`Codeforces ${contestId.trim()}${idx} (${result.slug})`)
                } catch (e: any) {
                    failed.push(`Codeforces ${contestId.trim()}${idx}: ${e.message}`)
                }
            })
        )

        setImportResults({ success, failed })
        setContestId('')
        setProblemIndex('')
        loadData()
        setImporting(false)
    }

    const handleCSESImport = async () => {
        if (!csesProblemId) return alert('Problem ID is required')
        setImporting(true)
        setImportResults(null)

        const ids = csesProblemId.split(/[\s,]+/).filter(Boolean)
        const success: string[] = []
        const failed: string[] = []

        await Promise.all(
            ids.map(async (id) => {
                try {
                    const result = await api.problems.importCSES(id)
                    success.push(`CSES ${id} (${result.slug})`)
                } catch (e: any) {
                    failed.push(`CSES ${id}: ${e.message}`)
                }
            })
        )

        setImportResults({ success, failed })
        setCSESProblemId('')
        loadData()
        setImporting(false)
    }

    const handleAtCoderImport = async () => {
        if (!atcoderContestId || !atcoderProblemId) return alert('Contest ID and Problem ID are required')
        setImporting(true)
        setImportResults(null)

        const ids = atcoderProblemId.split(/[\s,]+/).filter(Boolean)
        const success: string[] = []
        const failed: string[] = []

        await Promise.all(
            ids.map(async (id) => {
                try {
                    const result = await api.problems.importAtCoder(atcoderContestId.trim(), id)
                    success.push(`AtCoder ${atcoderContestId.trim()}_${id} (${result.slug})`)
                } catch (e: any) {
                    failed.push(`AtCoder ${atcoderContestId.trim()}_${id}: ${e.message}`)
                }
            })
        )

        setImportResults({ success, failed })
        setAtcoderContestId('')
        setAtcoderProblemId('')
        loadData()
        setImporting(false)
    }

    const handleTophImport = async () => {
        if (!tophProblemId) return alert('Problem ID is required')
        setImporting(true)
        setImportResults(null)

        const ids = tophProblemId.split(/[\s,]+/).filter(Boolean)
        const success: string[] = []
        const failed: string[] = []

        await Promise.all(
            ids.map(async (id) => {
                try {
                    const result = await api.problems.importToph(id)
                    success.push(`Toph ${id} (${result.slug})`)
                } catch (e: any) {
                    failed.push(`Toph ${id}: ${e.message}`)
                }
            })
        )

        setImportResults({ success, failed })
        setTophProblemId('')
        loadData()
        setImporting(false)
    }

    const handleQOJImport = async () => {
        if (!qojProblemId) return alert('Problem ID is required')
        setImporting(true)
        setImportResults(null)

        const ids = qojProblemId.split(/[\s,]+/).filter(Boolean)
        const success: string[] = []
        const failed: string[] = []

        await Promise.all(
            ids.map(async (id) => {
                try {
                    const result = await api.problems.importQOJ(id)
                    success.push(`QOJ ${id} (${result.slug})`)
                } catch (e: any) {
                    failed.push(`QOJ ${id}: ${e.message}`)
                }
            })
        )

        setImportResults({ success, failed })
        setQojProblemId('')
        loadData()
        setImporting(false)
    }

    const role = decodeRole()
    const isAdmin = role === 'admin'

    return (
        <div>
            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold">Problem Setter Workspace</h1>
                <div className="flex gap-2">
                    <Link to="/setter/contest/create" className="bg-green-600 text-white px-4 py-2 rounded text-sm hover:bg-green-700 transition-colors">+ Create Contest</Link>
                    <Link to="/setter/create" className="bg-blue-600 text-white px-4 py-2 rounded text-sm hover:bg-blue-700 transition-colors">+ Create Problem</Link>
                </div>
            </div>

            <div className="border-b border-gray-200 dark:border-gray-800 flex gap-4 mb-6">
                <button
                    onClick={() => setActiveTab('my-problems')}
                    className={`pb-2 text-sm font-medium border-b-2 cursor-pointer ${activeTab === 'my-problems' ? 'border-blue-600 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 hover:text-gray-700'}`}
                >
                    My Problems
                </button>
                <button
                    onClick={() => setActiveTab('my-contests')}
                    className={`pb-2 text-sm font-medium border-b-2 cursor-pointer ${activeTab === 'my-contests' ? 'border-blue-600 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 hover:text-gray-700'}`}
                >
                    My Contests
                </button>
                <button
                    onClick={() => setActiveTab('all-problems')}
                    className={`pb-2 text-sm font-medium border-b-2 cursor-pointer ${activeTab === 'all-problems' ? 'border-blue-600 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 hover:text-gray-700'}`}
                >
                    All Problems
                </button>
                {isAdmin && (
                    <button
                        onClick={() => setActiveTab('import')}
                        className={`pb-2 text-sm font-medium border-b-2 cursor-pointer ${activeTab === 'import' ? 'border-blue-600 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 hover:text-gray-700'}`}
                    >
                        Import Problems
                    </button>
                )}
            </div>

            {activeTab === 'import' ? (
                <section className="space-y-6">
                    <h2 className="text-lg font-semibold mb-3">Import Remote Problems</h2>
                    
                    {importResults && (
                        <div className="border rounded-lg p-4 bg-gray-50 dark:bg-gray-800 border-gray-200 dark:border-gray-700 space-y-3">
                            <h3 className="font-semibold text-sm">Import Results</h3>
                            {importResults.success.length > 0 && (
                                <div className="space-y-1">
                                    <h4 className="text-xs font-semibold text-green-600 uppercase">Successfully Imported ({importResults.success.length}):</h4>
                                    <ul className="list-disc pl-5 text-sm text-green-700 dark:text-green-400">
                                        {importResults.success.map((s, idx) => <li key={idx}>{s}</li>)}
                                    </ul>
                                </div>
                            )}
                            {importResults.failed.length > 0 && (
                                <div className="space-y-1">
                                    <h4 className="text-xs font-semibold text-red-600 uppercase">Failed Imports ({importResults.failed.length}):</h4>
                                    <ul className="list-disc pl-5 text-sm text-red-700 dark:text-red-400">
                                        {importResults.failed.map((f, idx) => <li key={idx}>{f}</li>)}
                                    </ul>
                                </div>
                            )}
                        </div>
                    )}

                    <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                        {/* Codeforces */}
                        <div className="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg p-5 shadow-sm">
                            <h3 className="font-semibold text-sm mb-3">Import from Codeforces</h3>
                            <div className="space-y-4">
                                <div className="grid grid-cols-2 gap-4">
                                    <div>
                                        <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Contest ID</label>
                                        <input type="text" value={contestId} onChange={e => setContestId(e.target.value)}
                                            placeholder="e.g. 1800" className="w-full border rounded px-3 py-2 text-sm border-gray-300 dark:border-gray-700 bg-transparent text-gray-800 dark:text-gray-200" />
                                    </div>
                                    <div>
                                        <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Problem Indices (space/comma separated for bulk)</label>
                                        <input type="text" value={problemIndex} onChange={e => setProblemIndex(e.target.value)}
                                            placeholder="e.g. A, B, C" className="w-full border rounded px-3 py-2 text-sm border-gray-300 dark:border-gray-700 bg-transparent text-gray-800 dark:text-gray-200" />
                                    </div>
                                </div>
                                <button onClick={handleImport} disabled={importing}
                                    className="w-full bg-purple-600 text-white px-4 py-2 rounded text-sm hover:bg-purple-700 disabled:opacity-50 transition-colors cursor-pointer font-medium">
                                    {importing ? 'Importing...' : 'Import Problem(s)'}
                                </button>
                            </div>
                        </div>

                        {/* CSES */}
                        <div className="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg p-5 shadow-sm">
                            <h3 className="font-semibold text-sm mb-3">Import from CSES</h3>
                            <div className="space-y-4">
                                <div>
                                    <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Problem ID(s) (space/comma separated for bulk)</label>
                                    <input type="text" value={csesProblemId} onChange={e => setCSESProblemId(e.target.value)}
                                        placeholder="e.g. 1068, 1069" className="w-full border rounded px-3 py-2 text-sm border-gray-300 dark:border-gray-700 bg-transparent text-gray-800 dark:text-gray-200" />
                                </div>
                                <button onClick={handleCSESImport} disabled={importing}
                                    className="w-full bg-teal-600 text-white px-4 py-2 rounded text-sm hover:bg-teal-700 disabled:opacity-50 transition-colors cursor-pointer font-medium">
                                    {importing ? 'Importing...' : 'Import Problem(s)'}
                                </button>
                            </div>
                        </div>

                        {/* AtCoder */}
                        <div className="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg p-5 shadow-sm">
                            <h3 className="font-semibold text-sm mb-3">Import from AtCoder</h3>
                            <div className="space-y-4">
                                <div className="grid grid-cols-2 gap-4">
                                    <div>
                                        <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Contest ID</label>
                                        <input type="text" value={atcoderContestId} onChange={e => setAtcoderContestId(e.target.value)}
                                            placeholder="e.g. abc300" className="w-full border rounded px-3 py-2 text-sm border-gray-300 dark:border-gray-700 bg-transparent text-gray-800 dark:text-gray-200" />
                                    </div>
                                    <div>
                                        <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Problem ID(s) (space/comma separated for bulk)</label>
                                        <input type="text" value={atcoderProblemId} onChange={e => setAtcoderProblemId(e.target.value)}
                                            placeholder="e.g. abc300_a, abc300_b" className="w-full border rounded px-3 py-2 text-sm border-gray-300 dark:border-gray-700 bg-transparent text-gray-800 dark:text-gray-200" />
                                    </div>
                                </div>
                                <button onClick={handleAtCoderImport} disabled={importing}
                                    className="w-full bg-red-600 text-white px-4 py-2 rounded text-sm hover:bg-red-700 disabled:opacity-50 transition-colors cursor-pointer font-medium">
                                    {importing ? 'Importing...' : 'Import Problem(s)'}
                                </button>
                            </div>
                        </div>

                        {/* Toph */}
                        <div className="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg p-5 shadow-sm">
                            <h3 className="font-semibold text-sm mb-3">Import from Toph</h3>
                            <div className="space-y-4">
                                <div>
                                    <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Problem ID(s) (space/comma separated for bulk)</label>
                                    <input type="text" value={tophProblemId} onChange={e => setTophProblemId(e.target.value)}
                                        placeholder="e.g. copycat, formatted-numbers" className="w-full border rounded px-3 py-2 text-sm border-gray-300 dark:border-gray-700 bg-transparent text-gray-800 dark:text-gray-200" />
                                </div>
                                <button onClick={handleTophImport} disabled={importing}
                                    className="w-full bg-orange-600 text-white px-4 py-2 rounded text-sm hover:bg-orange-700 disabled:opacity-50 transition-colors cursor-pointer font-medium">
                                    {importing ? 'Importing...' : 'Import Problem(s)'}
                                </button>
                            </div>
                        </div>

                        {/* QOJ */}
                        <div className="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg p-5 shadow-sm">
                            <h3 className="font-semibold text-sm mb-3">Import from QOJ</h3>
                            <div className="space-y-4">
                                <div>
                                    <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Problem ID(s) (space/comma separated for bulk)</label>
                                    <input type="text" value={qojProblemId} onChange={e => setQojProblemId(e.target.value)}
                                        placeholder="e.g. 1000, 1001" className="w-full border rounded px-3 py-2 text-sm border-gray-300 dark:border-gray-700 bg-transparent text-gray-800 dark:text-gray-200" />
                                </div>
                                <button onClick={handleQOJImport} disabled={importing}
                                    className="w-full bg-blue-600 text-white px-4 py-2 rounded text-sm hover:bg-blue-700 disabled:opacity-50 transition-colors cursor-pointer font-medium">
                                    {importing ? 'Importing...' : 'Import Problem(s)'}
                                </button>
                            </div>
                        </div>
                    </div>
                </section>
            ) : activeTab === 'my-contests' ? (
                <section>
                    <h2 className="text-lg font-semibold mb-3">My Contests</h2>
                    <div className="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden bg-white dark:bg-gray-900 shadow-sm">
                        <table className="w-full text-sm">
                            <thead className="bg-gray-50 dark:bg-gray-800 text-gray-500 dark:text-gray-400 text-xs uppercase">
                                <tr>
                                    <th className="px-4 py-3 text-left">Title</th>
                                    <th className="px-4 py-3 text-left">Start Time</th>
                                    <th className="px-4 py-3 text-left">Format</th>
                                    <th className="px-4 py-3 text-left">Visibility</th>
                                    <th className="px-4 py-3 text-right">Actions</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-gray-100 dark:divide-gray-700">
                                {contests.map(c => (
                                    <tr key={c.id}>
                                        <td className="px-4 py-3 font-medium">{c.title}</td>
                                        <td className="px-4 py-3 text-xs text-gray-500 dark:text-gray-400">{new Date(c.start_time).toLocaleString()}</td>
                                        <td className="px-4 py-3 text-xs capitalize">{c.format}</td>
                                        <td className="px-4 py-3">
                                            <span className={`px-2 py-0.5 rounded text-xs font-medium ${c.visible ? 'bg-green-100 text-green-800 dark:bg-green-900/20 dark:text-green-300' : 'bg-red-100 text-red-800 dark:bg-red-900/20 dark:text-red-300'}`}>
                                                {c.visible ? 'Public' : 'Hidden'}
                                            </span>
                                        </td>
                                        <td className="px-4 py-3 text-right flex gap-2 justify-end items-center">
                                            <Link to={`/contests/${c.id}`} className="text-blue-600 dark:text-blue-400 hover:underline text-xs">View</Link>
                                            <Link to={`/setter/contest/${c.id}/edit`} className="bg-orange-50 dark:bg-orange-900/20 hover:bg-orange-100 border border-orange-200 text-orange-700 dark:text-orange-300 px-2.5 py-1 rounded text-xs">Edit</Link>
                                            <Link to={`/setter/contest/${c.id}/manage`} className="bg-blue-50 dark:bg-blue-900/20 hover:bg-blue-100 border border-blue-200 text-blue-700 dark:text-blue-300 px-2.5 py-1 rounded text-xs">Manage</Link>
                                        </td>
                                    </tr>
                                ))}
                                {contests.length === 0 && (
                                    <tr><td colSpan={5} className="px-4 py-8 text-center text-gray-400 dark:text-gray-500">No contests managed by you yet.</td></tr>
                                )}
                            </tbody>
                        </table>
                    </div>
                </section>
            ) : (
                <section>
                    <h2 className="text-lg font-semibold mb-3">
                        {activeTab === 'my-problems' ? 'My Problems' : 'All Problems'}
                    </h2>
                    <div className="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden bg-white dark:bg-gray-900 shadow-sm">
                        <table className="w-full text-sm">
                            <thead className="bg-gray-50 dark:bg-gray-800 text-gray-500 dark:text-gray-400 text-xs uppercase">
                                <tr>
                                    <th className="px-4 py-3 text-left">Title</th>
                                    <th className="px-4 py-3 text-left">Source</th>
                                    <th className="px-4 py-3 text-left">Difficulty</th>
                                    <th className="px-4 py-3 text-right">Actions</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-gray-100 dark:divide-gray-700">
                                {problems.map(p => (
                                    <tr key={p.id}>
                                        <td className="px-4 py-3 font-medium">{p.title}</td>
                                        <td className="px-4 py-3 text-xs text-gray-500 dark:text-gray-400">{p.source || 'local'}</td>
                                        <td className="px-4 py-3">
                                            <span className={`px-2 py-1 rounded text-xs font-medium ${
                                                p.difficulty === 'easy' ? 'bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300' :
                                                p.difficulty === 'medium' ? 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300' : 'bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300'
                                            }`}>{p.difficulty}</span>
                                        </td>
                                        <td className="px-4 py-3 text-right flex gap-2 justify-end items-center">
                                            <Link to={`/problems/${p.slug}`} className="text-blue-600 dark:text-blue-400 hover:underline text-xs">View</Link>
                                            <Link to={`/setter/${p.slug}`} className="bg-orange-50 dark:bg-orange-900/20 hover:bg-orange-100 border border-orange-200 text-orange-700 dark:text-orange-300 px-2.5 py-1 rounded text-xs">Edit</Link>
                                        </td>
                                    </tr>
                                ))}
                                {problems.length === 0 && (
                                    <tr><td colSpan={4} className="px-4 py-8 text-center text-gray-400 dark:text-gray-500">No problems found.</td></tr>
                                )}
                            </tbody>
                        </table>
                    </div>
                </section>
            )}
        </div>
    )
}
