import { Link } from 'react-router-dom'

export default function SetterPanel() {
    return (
        <div>
            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold">Problem Setter Workspace</h1>
                <Link
                    to="/setter/create"
                    className="bg-blue-600 text-white px-4 py-2 rounded text-sm hover:bg-blue-700 transition-colors"
                >
                    + Create Problem
                </Link>
            </div>

            {/* My Problems List */}
            <section>
                <h2 className="text-lg font-semibold mb-3">My Problems</h2>
                <div className="border border-gray-200 rounded-lg overflow-hidden">
                    <table className="w-full text-sm">
                        <thead className="bg-gray-50 text-gray-500 text-xs uppercase">
                            <tr>
                                <th className="px-4 py-3 text-left">Title</th>
                                <th className="px-4 py-3 text-left">Difficulty</th>
                                <th className="px-4 py-3 text-left">Status</th>
                                <th className="px-4 py-3 text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-gray-100">
                            <tr>
                                <td colSpan={4} className="px-4 py-8 text-center text-gray-400">
                                    No problems created yet. Click "Create Problem" to get started.
                                </td>
                            </tr>
                        </tbody>
                    </table>
                </div>
            </section>
        </div>
    )
}
