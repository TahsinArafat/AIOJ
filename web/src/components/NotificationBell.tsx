import { useEffect, useState, useRef } from 'react'
import { api, getAccessToken } from '../lib/api'

export default function NotificationBell() {
    const [count, setCount] = useState(0)
    const [notifications, setNotifications] = useState<any[]>([])
    const [isOpen, setIsOpen] = useState(false)
    const containerRef = useRef<HTMLDivElement>(null)

    useEffect(() => {
        if (!getAccessToken()) return

        const fetchCount = () => {
            api.notifications.unreadCount().then(d => setCount(d.count)).catch(() => {})
        }

        fetchCount()
        const interval = setInterval(fetchCount, 30000)
        return () => clearInterval(interval)
    }, [])

    useEffect(() => {
        if (isOpen) {
            api.notifications.list(false, 10).then(d => setNotifications(d.data || [])).catch(() => {})
        }
    }, [isOpen])

    useEffect(() => {
        const handleClickOutside = (e: MouseEvent) => {
            if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
                setIsOpen(false)
            }
        }
        document.addEventListener('mousedown', handleClickOutside)
        return () => document.removeEventListener('mousedown', handleClickOutside)
    }, [])

    const handleMarkAllRead = async () => {
        try {
            await api.notifications.markAllAsRead()
            setCount(0)
            setNotifications(prev => prev.map(n => ({ ...n, read: true })))
        } catch (e) { console.error(e) }
    }

    const handleRead = async (id: string) => {
        try {
            await api.notifications.markAsRead(id)
            setCount(c => Math.max(0, c - 1))
            setNotifications(prev => prev.map(n => n.id === id ? { ...n, read: true } : n))
        } catch (e) { console.error(e) }
    }

    return (
        <div ref={containerRef} className="relative">
            <button onClick={() => setIsOpen(!isOpen)} className="relative p-2 text-gray-600 hover:text-gray-800 focus:outline-none transition-colors">
                <svg className="w-5.5 h-5.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
                </svg>
                {count > 0 && (
                    <span className="absolute top-1 right-1 inline-flex items-center justify-center px-1.5 py-0.5 text-3xs font-bold leading-none text-white transform translate-x-1/2 -translate-y-1/2 bg-red-600 rounded-full">
                        {count}
                    </span>
                )}
            </button>

            {isOpen && (
                <div className="absolute right-0 mt-2 w-80 bg-white rounded-lg shadow-xl border border-gray-200 z-50 overflow-hidden">
                    <div className="px-4 py-2.5 border-b border-gray-100 flex justify-between items-center bg-gray-50">
                        <span className="font-semibold text-sm text-gray-700">Notifications</span>
                        {count > 0 && (
                            <button onClick={handleMarkAllRead} className="text-xs text-blue-600 hover:underline font-medium">
                                Mark all as read
                            </button>
                        )}
                    </div>

                    <div className="max-h-64 overflow-y-auto divide-y divide-gray-100">
                        {notifications.map(n => (
                            <div key={n.id} onClick={() => handleRead(n.id)}
                                className={`p-3 text-xs cursor-pointer transition-colors ${n.read ? 'hover:bg-gray-50' : 'bg-blue-50/50 hover:bg-blue-50'}`}>
                                <div className="flex justify-between items-start">
                                    <span className="font-medium text-gray-800">{n.title}</span>
                                    <span className="text-gray-400 scale-90">{new Date(n.created_at).toLocaleDateString()}</span>
                                </div>
                                <p className="text-gray-500 mt-1">{n.content}</p>
                            </div>
                        ))}
                        {notifications.length === 0 && (
                            <div className="px-4 py-8 text-center text-gray-400 text-xs">No notifications yet.</div>
                        )}
                    </div>
                </div>
            )}
        </div>
    )
}
