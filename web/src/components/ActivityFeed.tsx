import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'

interface ActivityItem {
  id: string
  type: 'ac' | 'rating' | 'comment'
  username: string
  title: string
  summary?: string
  link?: string
  created_at: string
}

const TYPE_BADGE: Record<string, { label: string; cls: string }> = {
  ac: { label: 'AC', cls: 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-400' },
  rating: { label: 'ELO', cls: 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-400' },
  comment: { label: '💬', cls: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-400' },
}

function timeAgo(iso: string): string {
  const s = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000)
  if (s < 60) return 'just now'
  if (s < 3600) return `${Math.floor(s / 60)}m ago`
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`
  return `${Math.floor(s / 86400)}d ago`
}

/**
 * Community activity feed — recent AC, rating changes, and comments.
 * Fetches GET /api/activity. Failures render empty (home should not break).
 */
export default function ActivityFeed({ limit = 20 }: { limit?: number }) {
  const [items, setItems] = useState<ActivityItem[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    api.feed
      .activity(limit)
      .then(d => {
        if (!cancelled) setItems(d.data || [])
      })
      .catch(() => {
        if (!cancelled) setItems([])
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [limit])

  return (
    <section className="space-y-4" data-testid="activity-feed">
      <div className="flex justify-between items-center">
        <h2 className="text-xl font-bold">Community Activity</h2>
      </div>
      {loading ? (
        <div className="text-center py-6 text-gray-400 dark:text-gray-500">Loading...</div>
      ) : items.length === 0 ? (
        <div className="text-center py-6 text-gray-400 dark:text-gray-500 text-sm">No activity yet.</div>
      ) : (
        <div className="border border-gray-200 dark:border-gray-700 rounded-lg divide-y divide-gray-100 dark:divide-gray-750 bg-white dark:bg-gray-800 shadow-sm">
          {items.map(item => {
            const badge = TYPE_BADGE[item.type] || TYPE_BADGE.comment
            return (
              <div key={item.id} className="px-4 py-3 flex items-start gap-3 text-sm">
                <span className={`text-[10px] font-bold px-1.5 py-0.5 rounded shrink-0 ${badge.cls}`}>
                  {badge.label}
                </span>
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap gap-x-2 items-baseline">
                    <Link
                      to={`/user/${item.username}`}
                      className="font-semibold text-blue-600 dark:text-blue-400 hover:underline"
                    >
                      {item.username}
                    </Link>
                    <span className="text-gray-700 dark:text-gray-300 truncate">{item.title}</span>
                    <span className="text-xs text-gray-400 ml-auto shrink-0">{timeAgo(item.created_at)}</span>
                  </div>
                  {item.summary && (
                    <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5 line-clamp-2">
                      {item.summary}
                    </p>
                  )}
                </div>
              </div>
            )
          })}
        </div>
      )}
    </section>
  )
}
