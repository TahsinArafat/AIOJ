import { useEffect, useState } from 'react'
import { api } from '../lib/api'

export default function SetterApplication() {
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

    if (loading) return <p className="text-sm text-gray-400 dark:text-gray-500">Loading...</p>
    if (status === 'approved') return <p className="text-green-600 dark:text-green-400 text-sm">You are a problem setter!</p>
    if (status === 'pending') return <p className="text-yellow-600 dark:text-yellow-400 text-sm">Your application is pending review.</p>

    return (
        <div>
            {status === 'rejected' && <p className="text-red-600 dark:text-red-400 text-sm mb-2">Your previous application was rejected. You can re-apply.</p>}
            {!submitted ? (
                <div>
                    <textarea rows={3} value={reason} onChange={e => setReason(e.target.value)}
                        placeholder="Why do you want to become a setter?"
                        className="w-full border rounded px-3 py-2 text-sm mb-2 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 border-gray-300 dark:border-gray-700" />
                    <button onClick={handleApply} disabled={!reason.trim()}
                        className="bg-blue-600 text-white px-4 py-2 rounded text-sm hover:bg-blue-700 disabled:opacity-50 transition-colors cursor-pointer">
                        Apply as Problem Setter
                    </button>
                </div>
            ) : (
                <p className="text-green-600 dark:text-green-400 text-sm">Application submitted!</p>
            )}
        </div>
    )
}
