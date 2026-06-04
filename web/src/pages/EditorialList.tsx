import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'

export default function EditorialList() {
    const [editorials, setEditorials] = useState<any[]>([])
    const [total, setTotal] = useState(0)

    useEffect(() => {
        api.editorials.list().then(d => {
            setEditorials(d.data || [])
            setTotal(d.total || 0)
        }).catch(console.error)
    }, [])

    return (
        <div>
            <div className="flex justify-between items-center mb-6">
                <div>
                    <h1 className="text-2xl font-bold">Editorials</h1>
                    <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">Problem solutions and explanations.</p>
                </div>
            </div>

            <div className="space-y-3">
                {editorials.map(e => (
                    <Link key={e.id} to={`/editorials/${e.id}`} className="block border rounded-lg p-4 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors">
                        <div className="flex items-center gap-2 mb-1">
                            {e.is_official && (
                                <span className="text-xs bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300 px-2 py-0.5 rounded font-medium">Official</span>
                            )}
                            <h3 className="font-semibold">{e.title}</h3>
                        </div>
                        <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">Problem: {e.problem_title}</p>
                        <div className="flex gap-4 mt-2 text-xs text-gray-400 dark:text-gray-500">
                            <span>{e.username}</span>
                            <span>{e.upvotes} upvotes</span>
                            <span>{new Date(e.created_at).toLocaleDateString()}</span>
                        </div>
                    </Link>
                ))}
                {editorials.length === 0 && (
                    <div className="text-center py-16 text-gray-400 dark:text-gray-500">No editorials found.</div>
                )}
            </div>
            {total > 0 && <p className="text-sm text-gray-400 dark:text-gray-500 mt-4">{total} editorials</p>}
        </div>
    )
}
