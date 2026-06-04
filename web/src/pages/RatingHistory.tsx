import { useEffect, useState } from 'react'
import { api, getAccessToken } from '../lib/api'
import RatingBadge from '../components/RatingBadge'

interface RatingEntry {
  id: string
  user_id: string
  contest_id: string
  old_rating: number
  new_rating: number
  rank: number
  rating_change: number
  created_at: string
}

function decodeUser() {
  const token = getAccessToken()
  if (!token) return null
  try {
    return JSON.parse(atob(token.split('.')[1]))
  } catch {
    return null
  }
}

function formatDate(dateStr: string) {
  const d = new Date(dateStr)
  return d.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}

export default function RatingHistory() {
  const user = decodeUser()
  const [history, setHistory] = useState<RatingEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!user?.uid) {
      setLoading(false)
      return
    }

    api.ratings
      .getByUser(user.uid)
      .then((d) => {
        const data = Array.isArray(d) ? d : d?.data ?? []
        setHistory(data.sort((a: RatingEntry, b: RatingEntry) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime()))
      })
      .catch((e) => setError(e.message || 'Failed to load rating history'))
      .finally(() => setLoading(false))
  }, [user?.uid])

  if (!user) {
    return (
      <div className="text-center py-20 text-gray-400 dark:text-gray-500">
        Please log in to view your rating history.
      </div>
    )
  }

  if (loading) {
    return (
      <div className="max-w-2xl mx-auto py-20 text-center text-gray-400 dark:text-gray-500">
        Loading rating history...
      </div>
    )
  }

  if (error) {
    return (
      <div className="max-w-2xl mx-auto py-20 text-center text-red-500 dark:text-red-400">
        {error}
      </div>
    )
  }

  const currentRating = history.length > 0 ? history[history.length - 1].new_rating : user.rating || 0
  const maxRating = history.length > 0 ? Math.max(...history.map((h) => h.new_rating)) : currentRating
  const contestCount = history.length

  return (
    <div className="max-w-2xl mx-auto">
      <h1 className="text-2xl font-bold mb-6">Rating History</h1>

      {/* Summary Card */}
      <div className="grid grid-cols-3 gap-4 mb-8">
        <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-4 text-center shadow-sm">
          <p className="text-xs text-gray-500 dark:text-gray-400 mb-1">Current Rating</p>
          <RatingBadge rating={currentRating} size="lg" />
        </div>
        <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-4 text-center shadow-sm">
          <p className="text-xs text-gray-500 dark:text-gray-400 mb-1">Max Rating</p>
          <RatingBadge rating={maxRating} size="lg" />
        </div>
        <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-4 text-center shadow-sm">
          <p className="text-xs text-gray-500 dark:text-gray-400 mb-1">Contests</p>
          <p className="text-2xl font-semibold">{contestCount}</p>
        </div>
      </div>

      {/* Rating History List */}
      {history.length === 0 ? (
        <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-8 text-center text-gray-400 dark:text-gray-500 shadow-sm">
          No rated contests yet. Participate in contests to see your rating history!
        </div>
      ) : (
        <div className="space-y-3">
          {history.map((entry) => {
            const delta = entry.rating_change
            const deltaColor = delta > 0 ? 'text-green-600 dark:text-green-400' : delta < 0 ? 'text-red-600 dark:text-red-400' : 'text-gray-500 dark:text-gray-400'
            const deltaSign = delta > 0 ? '+' : ''

            return (
              <div
                key={entry.id}
                className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-4 shadow-sm hover:shadow-md transition-shadow"
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="flex items-center gap-2 min-w-[140px]">
                      <span className="text-sm font-medium text-gray-900 dark:text-gray-100">
                        Contest #{entry.contest_id}
                      </span>
                    </div>

                    <div className="flex items-center gap-2">
                      <RatingBadge rating={entry.old_rating} size="sm" />
                      <span className="text-gray-400 dark:text-gray-500">→</span>
                      <RatingBadge rating={entry.new_rating} size="sm" />
                    </div>

                    <span className={`text-sm font-semibold ${deltaColor} min-w-[60px] text-right`}>
                      {deltaSign}{delta}
                    </span>
                  </div>

                  <div className="flex items-center gap-4 text-sm text-gray-500 dark:text-gray-400">
                    <span>Rank #{entry.rank}</span>
                    <span>{formatDate(entry.created_at)}</span>
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
