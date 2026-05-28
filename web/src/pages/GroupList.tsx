import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'

export default function GroupList() {
    const [groups, setGroups] = useState<any[]>([])
    const [total, setTotal] = useState(0)

    useEffect(() => {
        api.groups.list().then(d => {
            setGroups(d.data || [])
            setTotal(d.total || 0)
        }).catch(console.error)
    }, [])

    return (
        <div>
            <div className="flex justify-between items-center mb-6">
                <div>
                    <h1 className="text-2xl font-bold">Groups</h1>
                    <p className="text-sm text-gray-500 mt-1">Join study groups and training rooms.</p>
                </div>
                {getAccessToken() && (
                    <Link to="/groups/create" className="bg-blue-600 text-white px-4 py-2 rounded text-sm hover:bg-blue-700 font-medium">
                        Create Group
                    </Link>
                )}
            </div>

            <div className="space-y-3">
                {groups.map(g => (
                    <Link key={g.id} to={`/groups/${g.id}`} className="block border rounded-lg p-4 hover:bg-gray-50 transition-colors">
                        <div className="flex justify-between items-center">
                            <div>
                                <h3 className="font-semibold text-lg">{g.name}</h3>
                                <p className="text-sm text-gray-600 mt-1">{g.description || 'No description provided.'}</p>
                                <div className="flex gap-4 mt-3 text-xs text-gray-400">
                                    <span>{g.member_count} members</span>
                                    <span>{g.is_public ? 'Public' : 'Private'}</span>
                                </div>
                            </div>
                        </div>
                    </Link>
                ))}
                {groups.length === 0 && (
                    <div className="text-center py-16 text-gray-400">No groups found.</div>
                )}
            </div>
            {total > 0 && <p className="text-sm text-gray-400 mt-4">{total} groups total</p>}
        </div>
    )
}
