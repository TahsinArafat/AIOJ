import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'

export default function SetterPanel() {
    const [problems, setProblems] = useState<any[]>([])

    const loadData = () => {
        api.problems.list().then(d => setProblems(d.data || [])).catch(console.error)
    }

    useEffect(() => { loadData() }, [])

    return (
        <div>
            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold">Problem Setter Workspace</h1>
                <div className="flex gap-2">
                    <Link
                        to="/setter/contest/create"
                        className="bg-green-600 text-white px-4 py-2 rounded text-sm hover:bg-green-700 transition-colors"
                    >
                        + Create Contest
                    </Link>
                    <Link
                        to="/setter/create"
                        className="bg-blue-600 text-white px-4 py-2 rounded text-sm hover:bg-blue-700 transition-colors"
                    >
                        + Create Problem
                    </Link>
                </div>
            </div>

            <section>
                <h2 className="text-lg font-semibold mb-3">Problems (Public List)</h2>
                <div className="border border-gray-200 rounded-lg overflow-hidden">
                    <table className="w-full text-sm">
                        <thead className="bg-gray-50 text-gray-500 text-xs uppercase">
                            <tr>
                                <th className="px-4 py-3 text-left">Title</th>
                                <th className="px-4 py-3 text-left">Difficulty</th>
                                <th className="px-4 py-3 text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-gray-100">
                            {problems.map(p => (
                                <tr key={p.id}>
                                    <td className="px-4 py-3 font-medium">{p.title}</td>
                                    <td className="px-4 py-3">
                                        <span className={`px-2 py-1 rounded text-xs font-medium ${
                                            p.difficulty === 'easy' ? 'bg-green-100 text-green-800' :
                                            p.difficulty === 'medium' ? 'bg-yellow-100 text-yellow-800' : 'bg-red-100 text-red-800'
                                        }`}>
                                            {p.difficulty}
                                        </span>
                                    </td>
                                    <td className="px-4 py-3 text-right flex gap-2 justify-end items-center">
                                        <Link to={`/problems/${p.slug}`} className="text-blue-600 hover:underline text-xs">View Public</Link>
                                        <Link to={`/setter/${p.slug}`} className="bg-orange-50 hover:bg-orange-100 border border-orange-200 text-orange-700 font-medium px-2.5 py-1 rounded text-xs transition-colors">Edit Workspace</Link>
                                    </td>
                                </tr>
                            ))}
                            {problems.length === 0 && (
                                <tr>
                                    <td colSpan={3} className="px-4 py-8 text-center text-gray-400">
                                        No problems found. Click "Create Problem" to get started.
                                    </td>
                                </tr>
                            )}
                        </tbody>
                    </table>
                </div>
            </section>
        </div>
    )
}
