import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'

export default function GroupList() {
    const [groups, setGroups] = useState<any[]>([])
    const [total, setTotal] = useState(0)
    const [activeTab, setActiveTab] = useState<'all' | 'my'>('all')
    const [joinCode, setJoinCode] = useState('')
    const [joining, setJoining] = useState(false)
    const [joinMessage, setJoinMessage] = useState('')

    useEffect(() => {
        if (activeTab === 'all') {
            api.groups.list().then(d => {
                setGroups(d.data || [])
                setTotal(d.total || 0)
            }).catch(console.error)
        } else {
            api.groups.my().then(d => {
                setGroups(d.data || [])
                setTotal(d.data?.length || 0)
            }).catch(console.error)
        }
    }, [activeTab])

    const handleJoinByCode = async () => {
        if (!joinCode.trim()) return
        setJoining(true)
        setJoinMessage('')
        try {
            await api.groups.joinByCode(joinCode.trim())
            setJoinMessage('Joined successfully!')
            setJoinCode('')
            if (activeTab === 'my') {
                api.groups.my().then(d => {
                    setGroups(d.data || [])
                    setTotal(d.data?.length || 0)
                }).catch(console.error)
            }
        } catch (e: any) {
            setJoinMessage('Failed: ' + e.message)
        } finally {
            setJoining(false)
        }
    }

    return (
        <div>
            <div className="flex justify-between items-center mb-6">
                <div>
                    <h1 className="text-2xl font-bold">Groups</h1>
                    <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">Join study groups and training rooms.</p>
                </div>
                {getAccessToken() && (
                    <Link to="/groups/create" className="bg-blue-600 text-white px-4 py-2 rounded text-sm hover:bg-blue-700 font-medium">
                        Create Group
                    </Link>
                )}
            </div>

            {getAccessToken() && (
                <div className="border-b border-gray-200 dark:border-gray-800 flex gap-4 mb-6">
                    <button
                        onClick={() => setActiveTab('all')}
                        className={`pb-2 text-sm font-medium border-b-2 cursor-pointer ${activeTab === 'all' ? 'border-blue-600 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 hover:text-gray-700'}`}
                    >
                        All Groups
                    </button>
                    <button
                        onClick={() => setActiveTab('my')}
                        className={`pb-2 text-sm font-medium border-b-2 cursor-pointer ${activeTab === 'my' ? 'border-blue-600 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 hover:text-gray-700'}`}
                    >
                        My Groups
                    </button>
                </div>
            )}

            {/* Join by Code */}
            {getAccessToken() && (
                <div className="flex gap-2 items-center mb-6 p-3 bg-gray-50 dark:bg-gray-800/50 border border-gray-200 dark:border-gray-700 rounded-lg">
                    <label className="text-sm font-medium text-gray-600 dark:text-gray-400 whitespace-nowrap">Join with Code:</label>
                    <input type="text" placeholder="Enter invite code..." value={joinCode}
                        onChange={e => setJoinCode(e.target.value)}
                        className="flex-1 border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-900 rounded px-3 py-2 text-sm"
                        onKeyDown={e => e.key === 'Enter' && handleJoinByCode()} />
                    <button onClick={handleJoinByCode} disabled={joining || !joinCode.trim()}
                        className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded text-sm font-medium disabled:opacity-50 cursor-pointer">
                        {joining ? 'Joining...' : 'Join'}
                    </button>
                    {joinMessage && (
                        <span className={`text-sm ${joinMessage.startsWith('Failed') ? 'text-red-600' : 'text-green-600'}`}>{joinMessage}</span>
                    )}
                </div>
            )}

            <div className="space-y-3">
                {groups.map(g => (
                    <Link key={g.id} to={`/groups/${g.id}`} className="block border rounded-lg p-4 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors">
                        <div className="flex justify-between items-center">
                            <div>
                                <h3 className="font-semibold text-lg">{g.name}</h3>
                                <p className="text-sm text-gray-600 dark:text-gray-400 mt-1">{g.description || 'No description provided.'}</p>
                                <div className="flex gap-4 mt-3 text-xs text-gray-400 dark:text-gray-500">
                                    <span>{g.member_count} members</span>
                                    <span>{g.is_public ? 'Public' : 'Private'}</span>
                                </div>
                            </div>
                        </div>
                    </Link>
                ))}
                {groups.length === 0 && (
                    <div className="text-center py-16 text-gray-400 dark:text-gray-500">No groups found.</div>
                )}
            </div>
            {total > 0 && <p className="text-sm text-gray-400 dark:text-gray-500 mt-4">{total} groups total</p>}
        </div>
    )
}
