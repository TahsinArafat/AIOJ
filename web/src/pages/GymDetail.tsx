import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'

export default function GymDetail() {
    const { id } = useParams<{ id: string }>()
    const [gym, setGym] = useState<any>(null)
    const [loading, setLoading] = useState(true)
    const [solved, setSolved] = useState(false)

    useEffect(() => {
        if (!id) return
        api.gym.get(id).then(setGym).catch(console.error).finally(() => setLoading(false))
    }, [id])

    const handleMarkSolved = async () => {
        if (!id) return
        try {
            await api.gym.markSolved(id)
            setSolved(true)
            if (gym) setGym({ ...gym, solve_count: gym.solve_count + 1 })
        } catch (e: any) { alert('Failed: ' + e.message) }
    }

    if (loading) return <div className="text-center py-20 text-gray-400">Loading...</div>
    if (!gym) return <div className="text-center py-20 text-gray-400">Gym contest not found</div>

    return (
        <div className="space-y-6">
            <div>
                <h1 className="text-2xl font-bold">{gym.contest_title}</h1>
                <p className="text-gray-600 mt-2">{gym.description || 'No description provided.'}</p>
            </div>

            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                <div className="bg-gray-50 p-4 rounded-lg">
                    <p className="text-sm text-gray-500">Category</p>
                    <p className="font-medium capitalize mt-1">{gym.category}</p>
                </div>
                {gym.difficulty_rating && (
                    <div className="bg-gray-50 p-4 rounded-lg">
                        <p className="text-sm text-gray-500">Difficulty</p>
                        <p className="font-medium font-mono mt-1">{gym.difficulty_rating}</p>
                    </div>
                )}
                <div className="bg-gray-50 p-4 rounded-lg">
                    <p className="text-sm text-gray-500">Solves</p>
                    <p className="font-medium mt-1">{gym.solve_count} users</p>
                </div>
                {gym.country && (
                    <div className="bg-gray-50 p-4 rounded-lg">
                        <p className="text-sm text-gray-500">Country</p>
                        <p className="font-medium mt-1">{gym.country}</p>
                    </div>
                )}
            </div>

            <div className="flex gap-4">
                <Link to={`/contests/${gym.contest_id}`} className="bg-blue-600 text-white px-6 py-2 rounded text-sm hover:bg-blue-700 font-medium">
                    Enter Contest
                </Link>
                {getAccessToken() && !solved && (
                    <button onClick={handleMarkSolved} className="bg-green-600 text-white px-6 py-2 rounded text-sm hover:bg-green-700 font-medium">
                        Mark as Solved
                    </button>
                )}
                {solved && <span className="text-green-600 font-semibold flex items-center">✓ Marked as Solved</span>}
            </div>
        </div>
    )
}
