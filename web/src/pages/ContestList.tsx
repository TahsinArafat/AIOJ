import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'

export default function ContestList() {
    const [contests, setContests] = useState<any[]>([])
    const [total, setTotal] = useState(0)

    useEffect(() => {
        api.contests.list().then(d => {
            setContests(d.data || [])
            setTotal(d.total || 0)
        }).catch(() => {})
    }, [])

    const statusLabel = (c: any) => {
        const now = Date.now()
        const start = new Date(c.start_time).getTime()
        const end = new Date(c.end_time).getTime()
        if (now < start) return { text: 'Upcoming', cls: 'text-blue-600 bg-blue-50' }
        if (now < end) return { text: 'Running', cls: 'text-green-600 bg-green-50' }
        return { text: 'Ended', cls: 'text-gray-500 bg-gray-100' }
    }

    return (
        <div>
            <h1 className="text-2xl font-bold mb-4">Contests</h1>
            <div className="space-y-2">
                {contests.map(c => {
                    const status = statusLabel(c)
                    return (
                        <Link key={c.id} to={`/contests/${c.id}`}
                            className="block border border-gray-200 rounded-lg p-4 hover:bg-gray-50 transition-colors">
                            <div className="flex items-center justify-between">
                                <span className="font-medium">{c.title}</span>
                                <span className={`text-xs px-2 py-0.5 rounded font-medium ${status.cls}`}>
                                    {status.text}
                                </span>
                            </div>
                            <div className="text-xs text-gray-400 mt-1 flex gap-4">
                                <span>{new Date(c.start_time).toLocaleString()} — {new Date(c.end_time).toLocaleString()}</span>
                                <span className="uppercase font-medium">{c.type}</span>
                            </div>
                        </Link>
                    )
                })}
                {contests.length === 0 && (
                    <div className="text-center py-16 text-gray-400">No contests yet.</div>
                )}
            </div>
            {total > 0 && (
                <p className="text-sm text-gray-400 mt-3">{total} contests total</p>
            )}
        </div>
    )
}
