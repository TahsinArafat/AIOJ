import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api, getAccessToken, contestSlug } from '../lib/api'
import { useToast } from '../components/Toast'
import { EmptyState } from '../components/EmptyState'
import { AlertTriangle } from 'lucide-react'

interface GymData {
    contest_title: string; description?: string; category?: string;
    difficulty_rating?: number; solve_count: number; country?: string;
    contest_slug?: string; contest_display_id?: number; contest_id: string;
}

export default function GymDetail() {
    const toast = useToast()
    const { id } = useParams<{ id: string }>()
    const [gym, setGym] = useState<GymData | null>(null)
    const [loading, setLoading] = useState(true)
    const [solved, setSolved] = useState(false)

    useEffect(() => {
        if (!id) return
        api.gym.get(id).then(setGym).catch(console.error).finally(() => setLoading(false))
        if (getAccessToken()) {
            api.gym.isSolved(id).then(res => setSolved(res.solved)).catch(console.error)
        }
    }, [id])

    const handleMarkSolved = async () => {
        if (!id) return
        try {
            await api.gym.markSolved(id)
            setSolved(true)
            if (gym) setGym({ ...gym, solve_count: gym.solve_count + 1 })
        } catch (e) { toast.error('Failed: ' + (e instanceof Error ? e.message : String(e))) }
    }

    if (loading) return <div className="text-center py-20 text-muted-soft">Loading...</div>
    if (!gym) {
        return (
            <div className="py-16">
                <EmptyState icon={<AlertTriangle className="w-12 h-12 text-bad" />} text="Gym contest not found" description="This gym contest may have been removed, or the link is incorrect." />
            </div>
        )
    }

    return (
        <div className="space-y-6">
            <div>
                <h1 className="text-2xl font-bold">{gym.contest_title}</h1>
                <p className="text-muted mt-2">{gym.description || 'No description provided.'}</p>
            </div>

            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                <div className="bg-surface-2 p-4 rounded-lg">
                    <p className="text-sm text-muted">Category</p>
                    <p className="font-medium capitalize mt-1">{gym.category}</p>
                </div>
                {gym.difficulty_rating && (
                    <div className="bg-surface-2 p-4 rounded-lg">
                        <p className="text-sm text-muted">Difficulty</p>
                        <p className="font-medium font-mono mt-1">{gym.difficulty_rating}</p>
                    </div>
                )}
                <div className="bg-surface-2 p-4 rounded-lg">
                    <p className="text-sm text-muted">Solves</p>
                    <p className="font-medium mt-1">{gym.solve_count} users</p>
                </div>
                {gym.country && (
                    <div className="bg-surface-2 p-4 rounded-lg">
                        <p className="text-sm text-muted">Country</p>
                        <p className="font-medium mt-1">{gym.country}</p>
                    </div>
                )}
            </div>

            <div className="flex gap-4">
                <Link to={`/contests/${contestSlug({ slug: gym.contest_slug, display_id: gym.contest_display_id, id: gym.contest_id })}`} className="bg-brand text-on-brand px-6 py-2 rounded text-sm hover:bg-brand-hover font-medium">
                    Enter Contest
                </Link>
                {getAccessToken() && !solved && (
                    <button onClick={handleMarkSolved} className="bg-ok-fill text-white px-6 py-2 rounded text-sm hover:bg-ok-fill font-medium">
                        Mark as Solved
                    </button>
                )}
                {solved && <span className="text-ok font-semibold flex items-center">✓ Marked as Solved</span>}
            </div>
        </div>
    )
}
