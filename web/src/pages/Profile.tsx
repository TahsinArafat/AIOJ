import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'
import RatingBadge from '../components/RatingBadge'

function decodeUser() {
    const token = getAccessToken()
    if (!token) return null
    try {
        return JSON.parse(atob(token.split('.')[1]))
    } catch { return null }
}

function SetterApplication() {
    const [status, setStatus] = useState<string | null>(null)
    const [reason, setReason] = useState('')
    const [submitted, setSubmitted] = useState(false)
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        api.setter.status().then(d => setStatus(d?.status || null)).catch(() => {}).finally(() => setLoading(false))
    }, [])

    const handleApply = async () => {
        try {
            await api.setter.apply(reason)
            setSubmitted(true)
            setStatus('pending')
        } catch (e: any) {
            alert('Failed: ' + e.message)
        }
    }

    if (loading) return <p className="text-sm text-gray-400">Loading...</p>
    if (status === 'approved') return <p className="text-green-600 text-sm">You are a problem setter!</p>
    if (status === 'pending') return <p className="text-yellow-600 text-sm">Your application is pending review.</p>

    return (
        <div>
            {status === 'rejected' && <p className="text-red-600 text-sm mb-2">Your previous application was rejected. You can re-apply.</p>}
            {!submitted ? (
                <div>
                    <textarea rows={3} value={reason} onChange={e => setReason(e.target.value)}
                        placeholder="Why do you want to become a setter?"
                        className="w-full border rounded px-3 py-2 text-sm mb-2 focus:outline-none focus:ring-2 focus:ring-blue-500" />
                    <button onClick={handleApply} disabled={!reason.trim()}
                        className="bg-blue-600 text-white px-4 py-2 rounded text-sm hover:bg-blue-700 disabled:opacity-50 transition-colors">
                        Apply as Problem Setter
                    </button>
                </div>
            ) : (
                <p className="text-green-600 text-sm">Application submitted!</p>
            )}
        </div>
    )
}

export default function Profile() {
    const user = decodeUser()
    if (!user) return <div className="text-center py-20 text-gray-400">Please log in to view your profile.</div>

    return (
        <div className="max-w-md mx-auto">
            <h1 className="text-2xl font-bold mb-6">Profile Settings</h1>

            <div className="space-y-4 bg-white border border-gray-200 rounded-lg p-6 mb-8">
                <div>
                    <label className="block text-sm text-gray-500 mb-1">Username</label>
                    <p className="font-medium">{user.uname || '—'}</p>
                </div>
                <div>
                    <label className="block text-sm text-gray-500 mb-1">Role</label>
                    <span className={`px-2 py-1 rounded text-xs font-medium ${
                        user.role === 'admin' ? 'bg-purple-100 text-purple-800' :
                        user.role === 'teacher' ? 'bg-orange-100 text-orange-800' : 'bg-gray-100 text-gray-800'
                    }`}>{user.role || 'user'}</span>
                </div>
                <div>
                    <label className="block text-sm text-gray-500 mb-1">Rating</label>
                    {user.rating ? (
                        <RatingBadge rating={user.rating} showTitle />
                    ) : (
                        <span className="text-gray-400">Unrated</span>
                    )}
                </div>
                {user.rating && (
                    <div>
                        <Link to="/rating-history" className="text-sm text-blue-600 hover:underline">
                            View Rating History →
                        </Link>
                    </div>
                )}
                <div>
                    <label className="block text-sm text-gray-500 mb-1">User ID</label>
                    <p className="font-mono text-xs text-gray-400">{user.uid || '—'}</p>
                </div>
            </div>

            <div className="flex gap-4 mb-8">
                <Link to="/settings/notifications" className="text-sm text-blue-600 hover:underline">
                    Notification Preferences →
                </Link>
                <Link to="/settings/api" className="text-sm text-blue-600 hover:underline">
                    API Keys →
                </Link>
            </div>

            {user.role === 'user' && (
                <section>
                    <h2 className="text-lg font-semibold mb-4">Become a Problem Setter</h2>
                    <SetterApplication />
                </section>
            )}
        </div>
    )
}
