import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'

interface Notification {
    id: string
    title: string
    content: string
    link?: string
    read: boolean
    created_at: string
}

export default function Notifications() {
    const [notifications, setNotifications] = useState<Notification[]>([])
    const [loading, setLoading] = useState(true)
    const navigate = useNavigate()

    useEffect(() => {
        api.notifications.list(false, 100)
            .then(d => setNotifications(d.data || []))
            .catch(console.error)
            .finally(() => setLoading(false))
    }, [])

    const handleMarkAsRead = async (id: string) => {
        try {
            await api.notifications.markAsRead(id)
            setNotifications(prev => prev.map(n => n.id === id ? { ...n, read: true } : n))
        } catch (e) { console.error(e) }
    }

    const handleMarkAllRead = async () => {
        try {
            await api.notifications.markAllAsRead()
            setNotifications(prev => prev.map(n => ({ ...n, read: true })))
        } catch (e) { console.error(e) }
    }

    const unreadCount = notifications.filter(n => !n.read).length

    if (loading) return <div className="text-center py-20 text-gray-400 dark:text-gray-500">Loading...</div>

    return (
        <div className="max-w-2xl mx-auto">
            <div className="flex justify-between items-center mb-6">
                <h1 className="text-2xl font-bold">Notifications</h1>
                {unreadCount > 0 && (
                    <button
                        onClick={handleMarkAllRead}
                        className="text-sm text-blue-600 dark:text-blue-400 hover:underline font-medium"
                    >
                        Mark all as read
                    </button>
                )}
            </div>

            {notifications.length === 0 ? (
                <p className="text-gray-400 dark:text-gray-500 text-center py-8">No notifications yet.</p>
            ) : (
                <div className="space-y-2">
                    {notifications.map(n => (
                        <div
                            key={n.id}
                            onClick={() => {
                                if (!n.read) handleMarkAsRead(n.id)
                                if (n.link) navigate(n.link)
                            }}
                            className={`block border border-gray-200 dark:border-gray-700 rounded-lg p-4 transition-colors ${
                                n.read
                                    ? 'bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700'
                                    : 'bg-blue-50 dark:bg-blue-900/20 hover:bg-blue-50 dark:hover:bg-blue-900/20/80'
                            } ${n.link ? 'cursor-pointer' : ''}`}
                        >
                            <div className="flex justify-between items-start">
                                <span className="font-medium text-gray-800 dark:text-gray-200 text-sm">{n.title}</span>
                                <span className="text-xs text-gray-400 dark:text-gray-500 whitespace-nowrap ml-4">
                                    {new Date(n.created_at).toLocaleString()}
                                </span>
                            </div>
                            <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">{n.content}</p>
                        </div>
                    ))}
                </div>
            )}
        </div>
    )
}
