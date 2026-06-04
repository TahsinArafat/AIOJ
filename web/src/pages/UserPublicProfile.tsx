import { useEffect, useState, useCallback } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api } from '../lib/api'
import RatingBadge from '../components/RatingBadge'
import RatingGraph from '../components/RatingGraph'
import { getRatingColor, getRatingTitle } from '../lib/rating'

// ─── Types ──────────────────────────────────────────────────────────────────

interface UserProfile {
  id: string
  username: string
  rating: number | null
  contests_played: number
  problems_solved: number
  created_at: string
}

interface RatingEntry {
  id: string
  new_rating: number
  old_rating: number
  rating_change: number
  contest_id: string
  rank: number
  created_at: string
}

interface Submission {
  id: string
  problem_id: string
  language: string
  status: string
  score: number
  time_used: number
  memory_used: number
  created_at: string
  submission_type: string
}

interface BlogPost {
  id: string
  title: string
  tags: string[]
  upvotes: number
  comment_count: number
  created_at: string
}

interface Comment {
  id: string
  parent_type: string
  parent_id: string
  content: string
  upvotes: number
  created_at: string
}

type Tab = 'profile' | 'contests' | 'submissions' | 'blogs' | 'comments'

// ─── Helpers ────────────────────────────────────────────────────────────────

function fmtDate(s: string) {
  return new Date(s).toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' })
}

function fmtDateShort(s: string) {
  return new Date(s).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: '2-digit' })
}

function StatusBadge({ status }: { status: string }) {
  const s = status.toLowerCase().replace(/[\s-]/g, '_')
  const map: Record<string, { label: string; cls: string }> = {
    accepted:            { label: 'AC',  cls: 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400' },
    ac:                  { label: 'AC',  cls: 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400' },
    wrong_answer:        { label: 'WA',  cls: 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400' },
    wa:                  { label: 'WA',  cls: 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400' },
    time_limit_exceeded: { label: 'TLE', cls: 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-400' },
    tle:                 { label: 'TLE', cls: 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-400' },
    runtime_error:       { label: 'RE',  cls: 'bg-orange-100 dark:bg-orange-900/30 text-orange-700' },
    re:                  { label: 'RE',  cls: 'bg-orange-100 dark:bg-orange-900/30 text-orange-700' },
    compilation_error:   { label: 'CE',  cls: 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300' },
    ce:                  { label: 'CE',  cls: 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300' },
    memory_limit_exceeded: { label: 'MLE', cls: 'bg-purple-100 dark:bg-purple-900/30 text-purple-700' },
    mle:                 { label: 'MLE', cls: 'bg-purple-100 dark:bg-purple-900/30 text-purple-700' },
  }
  const entry = map[s] ?? { label: status.toUpperCase().slice(0, 4), cls: 'bg-blue-100 dark:bg-blue-900/30 text-blue-700' }
  return <span className={`px-2 py-0.5 rounded text-xs font-bold font-mono ${entry.cls}`}>{entry.label}</span>
}

function DeltaBadge({ delta }: { delta: number }) {
  if (delta === 0) return <span className="text-gray-400">±0</span>
  return (
    <span className={delta > 0 ? 'text-green-600 dark:text-green-400 font-semibold' : 'text-red-600 dark:text-red-400 font-semibold'}>
      {delta > 0 ? '+' : ''}{delta}
    </span>
  )
}

function Skeleton() {
  return (
    <div className="animate-pulse space-y-6">
      <div className="h-32 bg-gray-200 dark:bg-gray-700 rounded-lg" />
      <div className="h-10 bg-gray-200 dark:bg-gray-700 rounded" />
      <div className="h-48 bg-gray-200 dark:bg-gray-700 rounded-lg" />
    </div>
  )
}

function Pagination({ page, total, limit, onChange }: { page: number; total: number; limit: number; onChange: (p: number) => void }) {
  const pages = Math.ceil(total / limit)
  if (pages <= 1) return null
  return (
    <div className="flex items-center justify-center gap-2 py-4">
      <button
        disabled={page === 1}
        onClick={() => onChange(page - 1)}
        className="px-3 py-1 text-sm rounded border border-gray-300 dark:border-gray-600 disabled:opacity-40 hover:bg-gray-100 dark:hover:bg-gray-700"
      >
        ‹ Prev
      </button>
      <span className="text-sm text-gray-500 dark:text-gray-400">
        {page} / {pages}
      </span>
      <button
        disabled={page === pages}
        onClick={() => onChange(page + 1)}
        className="px-3 py-1 text-sm rounded border border-gray-300 dark:border-gray-600 disabled:opacity-40 hover:bg-gray-100 dark:hover:bg-gray-700"
      >
        Next ›
      </button>
    </div>
  )
}

// ─── Main component ──────────────────────────────────────────────────────────

const LIMIT = 20

export default function UserPublicProfile() {
  const { username } = useParams<{ username: string }>()
  const [tab, setTab] = useState<Tab>('profile')
  const [user, setUser] = useState<UserProfile | null>(null)
  const [ratingHistory, setRatingHistory] = useState<RatingEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [notFound, setNotFound] = useState(false)

  const [submissions, setSubmissions] = useState<Submission[]>([])
  const [subsTotal, setSubsTotal] = useState(0)
  const [subsPage, setSubsPage] = useState(1)
  const [subsLoading, setSubsLoading] = useState(false)

  const [blogs, setBlogs] = useState<BlogPost[]>([])
  const [blogsTotal, setBlogsTotal] = useState(0)
  const [blogsPage, setBlogsPage] = useState(1)
  const [blogsLoading, setBlogsLoading] = useState(false)

  const [comments, setComments] = useState<Comment[]>([])
  const [commentsTotal, setCommentsTotal] = useState(0)
  const [commentsPage, setCommentsPage] = useState(1)
  const [commentsLoading, setCommentsLoading] = useState(false)

  useEffect(() => {
    if (!username) return
    setLoading(true)
    setNotFound(false)
    api.users.getByUsername(username)
      .then(u => {
        setUser(u)
        return api.ratings.getByUser(u.id, 100)
      })
      .then(d => {
        const data = Array.isArray(d) ? d : d?.data ?? []
        setRatingHistory(
          data.sort((a: RatingEntry, b: RatingEntry) =>
            new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
          )
        )
      })
      .catch(() => setNotFound(true))
      .finally(() => setLoading(false))
  }, [username])

  const loadSubmissions = useCallback((page: number) => {
    if (!username) return
    setSubsLoading(true)
    api.users.getSubmissions(username, (page - 1) * LIMIT, LIMIT)
      .then(d => {
        setSubmissions(d.data ?? [])
        setSubsTotal(d.total ?? 0)
        setSubsPage(page)
      })
      .finally(() => setSubsLoading(false))
  }, [username])

  const loadBlogs = useCallback((page: number) => {
    if (!username) return
    setBlogsLoading(true)
    api.users.getBlogs(username, (page - 1) * LIMIT, LIMIT)
      .then(d => {
        setBlogs(d.data ?? [])
        setBlogsTotal(d.total ?? 0)
        setBlogsPage(page)
      })
      .finally(() => setBlogsLoading(false))
  }, [username])

  const loadComments = useCallback((page: number) => {
    if (!username) return
    setCommentsLoading(true)
    api.users.getComments(username, (page - 1) * LIMIT, LIMIT)
      .then(d => {
        setComments(d.data ?? [])
        setCommentsTotal(d.total ?? 0)
        setCommentsPage(page)
      })
      .finally(() => setCommentsLoading(false))
  }, [username])

  useEffect(() => {
    if (!user) return
    if (tab === 'submissions' && submissions.length === 0 && !subsLoading) loadSubmissions(1)
    if (tab === 'blogs' && blogs.length === 0 && !blogsLoading) loadBlogs(1)
    if (tab === 'comments' && comments.length === 0 && !commentsLoading) loadComments(1)
  }, [tab, user]) // eslint-disable-line react-hooks/exhaustive-deps

  if (loading) return <Skeleton />
  if (notFound || !user) {
    return (
      <div className="max-w-3xl mx-auto text-center py-20">
        <h1 className="text-2xl font-bold mb-2 text-gray-800 dark:text-gray-200">User Not Found</h1>
        <p className="text-gray-500 dark:text-gray-400 mb-6">The user «{username}» does not exist.</p>
        <Link to="/" className="text-blue-600 dark:text-blue-400 hover:underline">← Back to Home</Link>
      </div>
    )
  }

  const currentRating = ratingHistory.length > 0
    ? ratingHistory[ratingHistory.length - 1].new_rating
    : user.rating ?? 0
  const maxRating = ratingHistory.length > 0
    ? Math.max(...ratingHistory.map(h => h.new_rating))
    : currentRating
  const ratingColor = getRatingColor(currentRating)
  const ratingTitle = currentRating > 0 ? getRatingTitle(currentRating) : 'Unrated'

  const TABS: { key: Tab; label: string }[] = [
    { key: 'profile',     label: 'Profile' },
    { key: 'contests',    label: `Contests (${ratingHistory.length})` },
    { key: 'submissions', label: 'Submissions' },
    { key: 'blogs',       label: 'Blogs' },
    { key: 'comments',    label: 'Comments' },
  ]

  return (
    <div className="max-w-4xl mx-auto space-y-4">

      {/* Header */}
      <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-6 shadow-sm">
        <div className="flex items-start gap-5">
          <div
            className="w-20 h-20 rounded-full flex items-center justify-center text-3xl font-bold text-white flex-shrink-0"
            style={{
              background: `linear-gradient(135deg, ${ratingColor.hex}cc, ${ratingColor.hex})`,
              boxShadow: `0 0 0 4px ${ratingColor.hex}50`,
            }}
          >
            {username?.charAt(0).toUpperCase()}
          </div>

          <div className="flex-1 min-w-0">
            <div className="flex items-baseline gap-3 flex-wrap">
              <h1 className="text-2xl font-bold" style={{ color: ratingColor.hex }}>
                {user.username}
              </h1>
              {currentRating > 0 && (
                <span className="text-sm font-medium" style={{ color: ratingColor.hex }}>
                  {ratingTitle}
                </span>
              )}
            </div>

            <div className="flex flex-wrap gap-x-6 gap-y-1 mt-2 text-sm text-gray-600 dark:text-gray-400">
              <span>
                <span className="text-gray-400 dark:text-gray-500 mr-1">Rating:</span>
                {currentRating > 0
                  ? <RatingBadge rating={currentRating} size="md" />
                  : <span className="text-gray-400">Unrated</span>
                }
              </span>
              {maxRating > 0 && maxRating !== currentRating && (
                <span>
                  <span className="text-gray-400 dark:text-gray-500 mr-1">Max:</span>
                  <RatingBadge rating={maxRating} size="md" />
                </span>
              )}
              <span>
                <span className="text-gray-400 dark:text-gray-500 mr-1">Contests:</span>
                <strong className="text-gray-800 dark:text-gray-200">{user.contests_played ?? ratingHistory.length}</strong>
              </span>
              <span>
                <span className="text-gray-400 dark:text-gray-500 mr-1">Solved:</span>
                <strong className="text-gray-800 dark:text-gray-200">{user.problems_solved ?? 0}</strong>
              </span>
            </div>

            <p className="text-xs text-gray-400 dark:text-gray-500 mt-2">
              Member since {fmtDate(user.created_at)}
            </p>
          </div>
        </div>
      </div>

      {/* Tab Bar */}
      <div className="border-b border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 rounded-t-lg overflow-x-auto">
        <nav className="flex gap-0 px-2">
          {TABS.map(t => (
            <button
              key={t.key}
              onClick={() => setTab(t.key)}
              className={`px-4 py-3 text-sm font-medium border-b-2 transition-colors whitespace-nowrap ${
                tab === t.key
                  ? 'border-blue-600 text-blue-600 dark:border-blue-400 dark:text-blue-400'
                  : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'
              }`}
            >
              {t.label}
            </button>
          ))}
        </nav>
      </div>

      {/* Tab Content */}
      <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-b-lg rounded-tr-lg shadow-sm">

        {/* PROFILE TAB */}
        {tab === 'profile' && (
          <div className="p-6 space-y-6">
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
              {[
                { label: 'Current Rating', value: currentRating > 0 ? <RatingBadge rating={currentRating} size="lg" /> : <span className="text-gray-400 text-lg">—</span> },
                { label: 'Max Rating',      value: maxRating > 0 ? <RatingBadge rating={maxRating} size="lg" /> : <span className="text-gray-400 text-lg">—</span> },
                { label: 'Contests',        value: <span className="text-2xl font-bold text-gray-800 dark:text-gray-100">{ratingHistory.length}</span> },
                { label: 'Problems Solved', value: <span className="text-2xl font-bold text-gray-800 dark:text-gray-100">{user.problems_solved ?? 0}</span> },
              ].map(s => (
                <div key={s.label} className="bg-gray-50 dark:bg-gray-700/50 border border-gray-100 dark:border-gray-700 rounded-lg p-4 text-center">
                  <p className="text-xs text-gray-500 dark:text-gray-400 mb-2">{s.label}</p>
                  {s.value}
                </div>
              ))}
            </div>

            <div>
              <h2 className="text-base font-semibold mb-3 text-gray-700 dark:text-gray-300">Rating History</h2>
              <RatingGraph history={ratingHistory} width={700} height={220} />
            </div>
          </div>
        )}

        {/* CONTESTS TAB */}
        {tab === 'contests' && (
          <div>
            {ratingHistory.length === 0 ? (
              <p className="text-center py-12 text-gray-400 dark:text-gray-500 text-sm">No rated contests yet</p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-700/50">
                      <th className="px-4 py-3 text-left font-medium text-gray-500 dark:text-gray-400">#</th>
                      <th className="px-4 py-3 text-left font-medium text-gray-500 dark:text-gray-400">Date</th>
                      <th className="px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400">Rank</th>
                      <th className="px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400">Old</th>
                      <th className="px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400">New</th>
                      <th className="px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400">Change</th>
                    </tr>
                  </thead>
                  <tbody>
                    {[...ratingHistory].reverse().map((entry, i) => (
                      <tr key={entry.id || i} className="border-t border-gray-100 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/40">
                        <td className="px-4 py-2.5 text-gray-400">{ratingHistory.length - i}</td>
                        <td className="px-4 py-2.5 text-gray-600 dark:text-gray-400">{fmtDateShort(entry.created_at)}</td>
                        <td className="px-4 py-2.5 text-right text-gray-700 dark:text-gray-300">
                          {entry.rank ? `#${entry.rank}` : '—'}
                        </td>
                        <td className="px-4 py-2.5 text-right">
                          <RatingBadge rating={entry.old_rating} size="sm" />
                        </td>
                        <td className="px-4 py-2.5 text-right">
                          <RatingBadge rating={entry.new_rating} size="sm" />
                        </td>
                        <td className="px-4 py-2.5 text-right">
                          <DeltaBadge delta={entry.rating_change} />
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        )}

        {/* SUBMISSIONS TAB */}
        {tab === 'submissions' && (
          <div>
            {subsLoading ? (
              <div className="text-center py-12 text-gray-400 text-sm">Loading...</div>
            ) : submissions.length === 0 ? (
              <p className="text-center py-12 text-gray-400 dark:text-gray-500 text-sm">No submissions</p>
            ) : (
              <>
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-700/50">
                        <th className="px-4 py-3 text-left font-medium text-gray-500 dark:text-gray-400">Verdict</th>
                        <th className="px-4 py-3 text-left font-medium text-gray-500 dark:text-gray-400">Language</th>
                        <th className="px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400">Time</th>
                        <th className="px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400">Memory</th>
                        <th className="px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400">Date</th>
                      </tr>
                    </thead>
                    <tbody>
                      {submissions.map(sub => (
                        <tr key={sub.id} className="border-t border-gray-100 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/40">
                          <td className="px-4 py-2.5"><StatusBadge status={sub.status} /></td>
                          <td className="px-4 py-2.5 text-gray-500 dark:text-gray-400 font-mono text-xs">{sub.language}</td>
                          <td className="px-4 py-2.5 text-right text-gray-500 dark:text-gray-400 font-mono text-xs">
                            {sub.time_used > 0 ? `${sub.time_used}ms` : '—'}
                          </td>
                          <td className="px-4 py-2.5 text-right text-gray-500 dark:text-gray-400 font-mono text-xs">
                            {sub.memory_used > 0 ? `${Math.round(sub.memory_used / 1024)}KB` : '—'}
                          </td>
                          <td className="px-4 py-2.5 text-right text-gray-500 dark:text-gray-400">{fmtDateShort(sub.created_at)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <Pagination page={subsPage} total={subsTotal} limit={LIMIT} onChange={p => loadSubmissions(p)} />
              </>
            )}
          </div>
        )}

        {/* BLOGS TAB */}
        {tab === 'blogs' && (
          <div>
            {blogsLoading ? (
              <div className="text-center py-12 text-gray-400 text-sm">Loading...</div>
            ) : blogs.length === 0 ? (
              <p className="text-center py-12 text-gray-400 dark:text-gray-500 text-sm">No blog posts yet</p>
            ) : (
              <>
                <div className="divide-y divide-gray-100 dark:divide-gray-700">
                  {blogs.map(post => (
                    <div key={post.id} className="px-6 py-4 hover:bg-gray-50 dark:hover:bg-gray-700/40">
                      <Link to={`/blog/${post.id}`} className="text-base font-semibold text-blue-600 dark:text-blue-400 hover:underline">
                        {post.title}
                      </Link>
                      <div className="flex gap-4 mt-1 text-xs text-gray-400 dark:text-gray-500">
                        <span>▲ {post.upvotes}</span>
                        <span>💬 {post.comment_count}</span>
                        <span>{fmtDate(post.created_at)}</span>
                      </div>
                      {post.tags?.length > 0 && (
                        <div className="flex gap-1.5 mt-2 flex-wrap">
                          {post.tags.map(tag => (
                            <span key={tag} className="text-xs bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300 px-2 py-0.5 rounded">
                              {tag}
                            </span>
                          ))}
                        </div>
                      )}
                    </div>
                  ))}
                </div>
                <div className="px-6 pb-2">
                  <Pagination page={blogsPage} total={blogsTotal} limit={LIMIT} onChange={p => loadBlogs(p)} />
                </div>
              </>
            )}
          </div>
        )}

        {/* COMMENTS TAB */}
        {tab === 'comments' && (
          <div>
            {commentsLoading ? (
              <div className="text-center py-12 text-gray-400 text-sm">Loading...</div>
            ) : comments.length === 0 ? (
              <p className="text-center py-12 text-gray-400 dark:text-gray-500 text-sm">No comments yet</p>
            ) : (
              <>
                <div className="divide-y divide-gray-100 dark:divide-gray-700">
                  {comments.map(c => (
                    <div key={c.id} className="px-6 py-4">
                      <div className="flex items-center gap-2 mb-1">
                        <span className="text-xs bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400 px-2 py-0.5 rounded capitalize">
                          {c.parent_type}
                        </span>
                        <span className="text-xs text-gray-400">{fmtDate(c.created_at)}</span>
                        {c.upvotes !== 0 && (
                          <span className="text-xs text-gray-400">▲ {c.upvotes}</span>
                        )}
                      </div>
                      <p className="text-sm text-gray-700 dark:text-gray-300 line-clamp-3">{c.content}</p>
                      {c.parent_type === 'blog' && (
                        <Link to={`/blog/${c.parent_id}`} className="text-xs text-blue-500 hover:underline mt-1 inline-block">
                          View post →
                        </Link>
                      )}
                    </div>
                  ))}
                </div>
                <div className="px-6 pb-2">
                  <Pagination page={commentsPage} total={commentsTotal} limit={LIMIT} onChange={p => loadComments(p)} />
                </div>
              </>
            )}
          </div>
        )}

      </div>
    </div>
  )
}
