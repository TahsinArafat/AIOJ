import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'
import { Card, CardHeader } from './ui/Card'
import { Badge, type BadgeTone } from './ui/Badge'
import { Skeleton } from './ui/PageHeader'

interface ActivityItem {
  id: string
  type: 'ac' | 'rating' | 'comment'
  username: string
  title: string
  summary?: string
  link?: string
  created_at: string
}

const TYPE_BADGE: Record<string, { label: string; tone: BadgeTone }> = {
  ac: { label: 'AC', tone: 'ok' },
  rating: { label: 'ELO', tone: 'brand' },
  comment: { label: 'RE', tone: 'neutral' },
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
    <div data-testid="activity-feed">
      <Card>
        <CardHeader title="Community Activity" description="Accepted solutions, rating moves and discussion." />
        {loading ? (
          <div className="space-y-3 px-5 py-4">
            <Skeleton className="h-3.5 w-2/3" />
            <Skeleton className="h-3.5 w-1/2" />
            <Skeleton className="h-3.5 w-3/5" />
          </div>
        ) : items.length === 0 ? (
          <p className="px-5 py-8 text-center text-sm text-muted">No activity yet.</p>
        ) : (
          <ul className="divide-y divide-hairline">
            {items.map(item => {
              const badge = TYPE_BADGE[item.type] || TYPE_BADGE.comment
              return (
                <li key={item.id} className="flex items-start gap-3 px-5 py-3">
                  <Badge tone={badge.tone} className="mt-0.5 shrink-0">{badge.label}</Badge>
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
                      <Link to={`/user/${item.username}`} className="text-sm font-medium text-ink hover:text-brand">
                        {item.username}
                      </Link>
                      <span className="truncate text-sm text-ink-soft">{item.title}</span>
                      <time dateTime={item.created_at} className="ml-auto shrink-0 text-[11px] text-muted-soft">
                        {timeAgo(item.created_at)}
                      </time>
                    </div>
                    {item.summary && (
                      <p className="mt-0.5 line-clamp-2 text-xs leading-relaxed text-muted">{item.summary}</p>
                    )}
                  </div>
                </li>
              )
            })}
          </ul>
        )}
      </Card>
    </div>
  )
}
