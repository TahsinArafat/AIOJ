import { useState, useEffect, useCallback, useMemo } from 'react';
import { useParams, Link } from 'react-router-dom';
import { api, getAccessToken } from '../lib/api';
import RatingBadge from '../components/RatingBadge';
import { AlertTriangle, Eye, EyeOff, ChevronLeft, ChevronRight } from 'lucide-react';

// ── Types ────────────────────────────────────────────────────────────────────

interface ProblemCell {
  solved: boolean;
  attempts: number;
  time: number;       // minutes from start
  score: number;
  pending: number;
}

interface ScoreboardEntry {
  rank: number;
  user_id: string;
  username: string;
  rating?: number;
  total_solved: number;
  total_penalty: number;
  total_score: number;
  problems: Record<string, ProblemCell>;
}

interface ProblemMeta {
  problem_id: string;
  index: string;
  score: number;
  title?: string;
}

interface ContestInfo {
  id: string;
  title: string;
  format?: string;
  status?: string;
  start_time?: string;
  end_time?: string;
  is_rated?: boolean;
  rating_calculated?: boolean;
}

interface RatingDelta {
  user_id: string;
  rating_change: number;
  new_rating: number;
}

interface PaginationInfo {
  page: number;
  limit: number;
  total: number;
  total_pages: number;
}

interface ScoreboardData {
  entries: ScoreboardEntry[];
  problems: ProblemMeta[];
  frozen: boolean;
  contest: ContestInfo;
  is_judge: boolean;
  can_see_judge: boolean;
  pagination: PaginationInfo;
}

// ── Helpers ──────────────────────────────────────────────────────────────────

function isOI(format?: string): boolean {
  return format === 'ioi' || format === 'oi';
}

// ── Component ────────────────────────────────────────────────────────────────

export default function ContestScoreboard() {
  const { id } = useParams<{ id: string }>();

  const [data, setData] = useState<ScoreboardData | null>(null);
  const [ratings, setRatings] = useState<Map<string, RatingDelta>>(new Map());
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [ratingLoading, setRatingLoading] = useState(false);
  const [ratingMsg, setRatingMsg] = useState('');
  const [viewMode, setViewMode] = useState<'judge' | 'public'>('public');
  const [canSeeJudge, setCanSeeJudge] = useState(false);
  const [currentPage, setCurrentPage] = useState(1);

  // Reset to page 1 when view mode changes
  useEffect(() => {
    setCurrentPage(1);
  }, [viewMode]);

  const isAdmin = useMemo(() => {
    try {
      const token = getAccessToken();
      if (!token) return false;
      const payload = JSON.parse(atob(token.split('.')[1]));
      return payload.role === 'admin';
    } catch {
      return false;
    }
  }, []);

  const userId = useMemo(() => {
    try {
      const token = getAccessToken();
      if (!token) return null;
      const payload = JSON.parse(atob(token.split('.')[1]));
      return payload.uid || null;
    } catch {
      return null;
    }
  }, []);

  const fetchScoreboard = useCallback(async (view?: string, page?: number) => {
    if (!id) return;
    try {
      const res = await api.contests.scoreboard(id, view, page);
      const sbData = res as unknown as ScoreboardData;
      setData(sbData);
      // Capture judge capability from the response (works even in public view)
      if (sbData.can_see_judge) {
        setCanSeeJudge(true);
      }
      setError('');
    } catch (e: any) {
      setError(e?.message || 'Failed to load scoreboard');
    } finally {
      setLoading(false);
    }
  }, [id]);

  const fetchRatings = useCallback(async () => {
    if (!id) return;
    try {
      const res = await api.ratings.getByContest(id);
      const map = new Map<string, RatingDelta>();
      (res.data || []).forEach((r: any) => {
        map.set(r.user_id, { user_id: r.user_id, rating_change: r.rating_change, new_rating: r.new_rating });
      });
      setRatings(map);
    } catch {
      // ratings may not exist yet — that's fine
    }
  }, [id]);

  useEffect(() => {
    fetchScoreboard(viewMode, currentPage);
    fetchRatings();
  }, [fetchScoreboard, fetchRatings, viewMode, currentPage]);

  // Auto-refresh every 30s for running contests
  useEffect(() => {
    if (!data?.contest || data.contest.status === 'ended') return;
    const interval = setInterval(() => {
      fetchScoreboard(viewMode, currentPage);
    }, 30_000);
    return () => clearInterval(interval);
  }, [data?.contest?.status, fetchScoreboard, viewMode, currentPage]);

  // Determine first-solve problem indices
  const firstSolves = useMemo(() => {
    if (!data) return new Set<string>();
    const solved: Record<string, { time: number; user_id: string }> = {};
    for (const entry of data.entries) {
      for (const [idx, cell] of Object.entries(entry.problems)) {
        if (cell.solved && cell.attempts > 0) {
          if (!solved[idx] || cell.time < solved[idx].time) {
            solved[idx] = { time: cell.time, user_id: entry.user_id };
          }
        }
      }
    }
    const set = new Set<string>();
    for (const idx of Object.keys(solved)) {
      set.add(`${solved[idx].user_id}::${idx}`);
    }
    return set;
  }, [data]);

  const handleRateContest = async () => {
    if (!id) return;
    setRatingLoading(true);
    setRatingMsg('');
    try {
      const token = getAccessToken();
      const res = await fetch(`${import.meta.env.VITE_API_URL || ''}/api/contests/${id}/calculate-ratings`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.message || `HTTP ${res.status}`);
      }
      setRatingMsg('Rating calculation started!');
      // Refresh ratings after a delay
      setTimeout(() => fetchRatings(), 3000);
    } catch (e: any) {
      setRatingMsg(e?.message || 'Failed to trigger rating');
    } finally {
      setRatingLoading(false);
    }
  };

  // ── Render ───────────────────────────────────────────────────────────────

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-gray-500 text-lg">Loading scoreboard...</div>
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="max-w-2xl mx-auto mt-16 text-center">
        <div className="text-red-500 text-lg mb-4">{error || 'Scoreboard not found'}</div>
        <Link to={`/contests/${id}`} className="text-blue-600 hover:underline">
          &larr; Back to contest
        </Link>
      </div>
    );
  }

  const { entries, problems, frozen, contest, pagination } = data;
  const oi = isOI(contest.format);
  const problemIndices = problems.map((p) => p.index).sort();

  // Build a map from index to problem meta
  const problemMap = new Map<string, ProblemMeta>();
  problems.forEach((p) => problemMap.set(p.index, p));

  return (
    <div className="max-w-full mx-auto px-2 sm:px-4 py-4">
      {/* Back link */}
      <div className="mb-3 flex items-center justify-between">
        <Link
          to={`/contests/${id}`}
          className="text-blue-600 hover:text-blue-800 hover:underline text-sm font-medium"
        >
          &larr; {contest.title || 'Back to contest'}
        </Link>

        {/* Judge/Public view toggle — only visible to admins/judges */}
        {canSeeJudge && (
          <div className="inline-flex items-center rounded-lg border border-gray-200 bg-white shadow-sm overflow-hidden text-sm">
            <button
              onClick={() => setViewMode('judge')}
              className={`flex items-center gap-1.5 px-3 py-1.5 font-medium transition-colors ${viewMode === 'judge'
                  ? 'bg-blue-600 text-white'
                  : 'text-gray-600 hover:bg-gray-50'
                }`}
            >
              <Eye className="w-3.5 h-3.5" />
              Judge View
            </button>
            <button
              onClick={() => setViewMode('public')}
              className={`flex items-center gap-1.5 px-3 py-1.5 font-medium transition-colors ${viewMode === 'public'
                  ? 'bg-blue-600 text-white'
                  : 'text-gray-600 hover:bg-gray-50'
                }`}
            >
              <EyeOff className="w-3.5 h-3.5" />
              Public
            </button>
          </div>
        )}
      </div>

      {/* Frozen / Judge banners */}
      {frozen && viewMode === 'public' && canSeeJudge && (
        <div className="mb-3 px-4 py-2.5 rounded border text-sm font-medium bg-amber-50 border-amber-300 text-amber-800 flex items-center gap-2">
          <AlertTriangle className="w-4 h-4 flex-shrink-0" />
          Viewing public standings — frozen submissions are hidden. Switch to Judge View to see real-time data.
        </div>
      )}
      {frozen && !canSeeJudge && (
        <div className="mb-3 px-4 py-2.5 rounded border text-sm font-medium bg-amber-50 border-amber-300 text-amber-800 flex items-center gap-2">
          <AlertTriangle className="w-4 h-4 flex-shrink-0" />
          The scoreboard is frozen. New submissions will not be visible until the contest ends.
        </div>
      )}
      {viewMode === 'judge' && canSeeJudge && (
        <div className="mb-3 px-4 py-2.5 rounded border text-sm font-medium bg-blue-50 border-blue-300 text-blue-800 flex items-center gap-2">
          <Eye className="w-4 h-4 flex-shrink-0" />
          Judge view — you are seeing real-time data including pending submissions.
          {frozen && ' The scoreboard is frozen for regular users.'}
        </div>
      )}

      {/* Admin rating button */}
      {isAdmin && contest.status === 'ended' && (
        <div className="mb-3 flex items-center gap-3">
          <button
            onClick={handleRateContest}
            disabled={ratingLoading}
            className="px-4 py-1.5 text-sm font-medium rounded bg-purple-600 text-white hover:bg-purple-700 disabled:opacity-50 transition-colors"
          >
            {ratingLoading ? 'Calculating...' : contest.rating_calculated ? 'Recalculate Ratings' : 'Rate This Contest'}
          </button>
          {ratingMsg && (
            <span className="text-sm text-gray-600">{ratingMsg}</span>
          )}
        </div>
      )}

      {/* Scoreboard table */}
      <div className="overflow-x-auto bg-white border border-gray-300 rounded-lg shadow-md">
        <table className="w-full border-collapse text-sm" style={{ minWidth: 600 + problemIndices.length * 72 }}>
          <thead>
            <tr className="bg-gray-900 text-white">
              <th className="px-3 py-2.5 text-center font-bold w-10">#</th>
              <th className="px-4 py-2.5 text-left font-bold min-w-[140px]">Team</th>
              <th className="px-3 py-2.5 text-center font-bold w-12">Solved</th>
              <th className="px-3 py-2.5 text-center font-bold w-20">Penalty</th>
              {ratings.size > 0 && (
                <th className="px-3 py-2.5 text-center font-bold w-28">Rating</th>
              )}
              {problemIndices.map((idx) => {
                const pm = problemMap.get(idx);
                return (
                  <th key={idx} className="px-2 py-2.5 text-center font-bold w-[72px]">
                    <div className="text-sm">{idx}</div>
                    {pm?.title && (
                      <div className="text-[10px] font-normal text-gray-400 truncate max-w-[68px]" title={pm.title}>{pm.title}</div>
                    )}
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-200">
            {entries.length === 0 ? (
              <tr>
                <td
                  colSpan={4 + problemIndices.length + (ratings.size > 0 ? 1 : 0)}
                  className="px-4 py-8 text-center text-gray-400 italic"
                >
                  No participants yet.
                </td>
              </tr>
            ) : (
              entries.map((entry, idx) => {
                const isMe = entry.user_id === userId
                const medal = entry.rank === 1 ? 'gold' : entry.rank === 2 ? 'silver' : entry.rank === 3 ? 'bronze' : null
                const ratingDelta = ratings.get(entry.user_id);

                // Format penalty as H:MM:SS
                const sec = entry.total_penalty ?? 0
                const ph = Math.floor(sec / 3600)
                const pm = Math.floor((sec % 3600) / 60)
                const ps = sec % 60
                const penaltyStr = ph > 0 ? `${ph}:${String(pm).padStart(2, '0')}:${String(ps).padStart(2, '0')}` : `${pm}:${String(ps).padStart(2, '0')}`

                return (
                  <tr
                    key={entry.user_id}
                    className={`transition-colors ${isMe ? 'bg-blue-50 hover:bg-blue-100' : idx % 2 === 0 ? 'bg-white hover:bg-gray-50' : 'bg-gray-50 hover:bg-gray-100'}`}
                  >
                    {/* Rank */}
                    <td className="px-3 py-2.5 text-center">
                      {medal ? (
                        <span className={`inline-flex items-center justify-center w-8 h-8 rounded-full text-sm font-black ${medal === 'gold' ? 'bg-yellow-400 text-yellow-900' :
                            medal === 'silver' ? 'bg-gray-300 text-gray-800' :
                              'bg-amber-600 text-white'
                          }`}>{entry.rank}</span>
                      ) : (
                        <span className="text-sm font-bold text-gray-600">{entry.rank}</span>
                      )}
                    </td>

                    {/* Username */}
                    <td className={`px-4 py-2.5 font-semibold ${isMe ? 'text-blue-700' : 'text-gray-800'}`}>
                      <div className="flex items-center gap-1.5 min-w-0">
                        <Link
                          to={`/profile/${entry.username}`}
                          className="font-medium truncate hover:underline"
                        >
                          {entry.username}
                        </Link>
                        {entry.rating != null && (
                          <span className="flex-shrink-0">
                            <RatingBadge rating={entry.rating} size="sm" />
                          </span>
                        )}
                        {isMe && <span className="text-[10px] bg-blue-200 text-blue-700 px-1.5 py-0.5 rounded font-bold flex-shrink-0">YOU</span>}
                      </div>
                    </td>

                    {/* Solved */}
                    <td className="px-3 py-2.5 text-center">
                      <span className="text-lg font-black text-gray-900">{oi ? entry.total_score : entry.total_solved}</span>
                    </td>

                    {/* Penalty */}
                    <td className="px-3 py-2.5 text-center font-mono text-gray-500 text-xs">
                      {penaltyStr}
                    </td>

                    {/* Rating delta */}
                    {ratings.size > 0 && (
                      <td className="px-3 py-2.5 text-center">
                        {ratingDelta ? (
                          <span className="inline-flex items-center gap-1 text-xs font-medium">
                            <span className="text-gray-500">{ratingDelta.new_rating}</span>
                            <span className={ratingDelta.rating_change > 0 ? 'text-green-600' : ratingDelta.rating_change < 0 ? 'text-red-600' : 'text-gray-400'}>
                              {ratingDelta.rating_change > 0 ? '+' : ''}{ratingDelta.rating_change}
                            </span>
                          </span>
                        ) : (
                          <span className="text-gray-400 text-xs font-medium">—</span>
                        )}
                      </td>
                    )}

                    {/* Per-problem cells */}
                    {problemIndices.map((idx) => {
                      const cell = entry.problems[idx];
                      if (!cell) {
                        return <td key={idx} className="px-1 py-2.5 text-center"><span className="text-gray-400 font-medium">—</span></td>;
                      }

                      const isFirst = firstSolves.has(`${entry.user_id}::${idx}`);
                      const timeSec = cell.time || 0
                      const timeMin = Math.floor(timeSec / 60)

                      if (cell.solved) {
                        const attemptStr = cell.attempts <= 1 ? '+' : `+${cell.attempts - 1}`
                        return (
                          <td key={idx} className="px-1 py-2.5 text-center" title={`Solved in ${timeMin}m, ${cell.attempts} attempt(s)`}>
                            <div className={`inline-flex flex-col items-center justify-center rounded-md px-2 py-1 min-w-[48px] ${isFirst ? 'bg-emerald-500 text-white shadow-sm' : 'bg-emerald-50 text-emerald-700 border border-emerald-200'}`}>
                              <span className="text-xs font-black leading-none">{oi ? cell.score : attemptStr}</span>
                              <span className={`text-[10px] font-mono leading-none mt-0.5 ${isFirst ? 'text-white/70' : 'text-emerald-600/70'}`}>{timeMin}m</span>
                            </div>
                          </td>
                        );
                      }

                      if (cell.pending > 0) {
                        return (
                          <td key={idx} className="px-1 py-2.5 text-center" title={`${cell.pending} pending submission(s)`}>
                            <div className="inline-flex flex-col items-center justify-center bg-sky-50 text-sky-700 border border-sky-200 rounded-md px-2 py-1 min-w-[48px]">
                              <span className="text-xs font-black leading-none">?</span>
                              <span className="text-[10px] font-mono leading-none mt-0.5">{cell.pending}</span>
                            </div>
                          </td>
                        );
                      }

                      if (cell.attempts > 0) {
                        return (
                          <td key={idx} className="px-1 py-2.5 text-center" title={`${cell.attempts} failed attempt(s)`}>
                            <div className="inline-flex items-center justify-center bg-rose-50 text-rose-600 border border-rose-200 rounded-md px-2 py-1 min-w-[48px]">
                              <span className="text-xs font-black">-{cell.attempts}</span>
                            </div>
                          </td>
                        );
                      }

                      return <td key={idx} className="px-1 py-2.5 text-center"><span className="text-gray-400 font-medium">—</span></td>;
                    })}
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>

      {/* Footer info */}
      <div className="mt-3 flex flex-wrap items-center gap-4 text-xs text-gray-500">
        <span>
          {pagination?.total ?? entries.length} participant{(pagination?.total ?? entries.length) !== 1 ? 's' : ''}
        </span>
        <span className="flex items-center gap-1">
          <span className="inline-block w-3 h-3 rounded-sm bg-green-600"></span>
          First solver
        </span>
        <span className="flex items-center gap-1">
          <span className="inline-block w-3 h-3 rounded-sm bg-green-100 border border-green-300"></span>
          Solved
        </span>
        <span className="flex items-center gap-1">
          <span className="inline-block w-3 h-3 rounded-sm bg-red-100 border border-red-300"></span>
          Failed
        </span>
        <span className="flex items-center gap-1">
          <span className="inline-block w-3 h-3 rounded-sm bg-blue-100 border border-blue-300"></span>
          Pending
        </span>
      </div>

      {/* Pagination */}
      {pagination && pagination.total_pages > 1 && (() => {
        const { page, total_pages } = pagination;
        // Generate page numbers: 1,2,3,4,5,...,19,20
        const pages: (number | 'ellipsis')[] = [];
        if (total_pages <= 7) {
          for (let i = 1; i <= total_pages; i++) pages.push(i);
        } else {
          // Always show first 2 and last 2
          const left = Math.max(2, page - 1);
          const right = Math.min(total_pages - 1, page + 1);
          pages.push(1);
          if (left > 2) pages.push('ellipsis');
          for (let i = left; i <= right; i++) pages.push(i);
          if (right < total_pages - 1) pages.push('ellipsis');
          pages.push(total_pages);
        }

        return (
          <div className="mt-4 flex items-center justify-center gap-1">
            {/* Prev */}
            <button
              onClick={() => setCurrentPage(p => Math.max(1, p - 1))}
              disabled={page <= 1}
              className="inline-flex items-center gap-1 px-2.5 py-1.5 text-sm font-medium rounded-md border border-gray-200 bg-white text-gray-600 hover:bg-gray-50 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              <ChevronLeft className="w-4 h-4" />
              Prev
            </button>

            {/* Page numbers */}
            {pages.map((p, i) =>
              p === 'ellipsis' ? (
                <span key={`e${i}`} className="px-2 py-1.5 text-sm text-gray-400">...</span>
              ) : (
                <button
                  key={p}
                  onClick={() => setCurrentPage(p)}
                  className={`min-w-[36px] px-2 py-1.5 text-sm font-medium rounded-md border transition-colors ${p === page
                      ? 'bg-blue-600 text-white border-blue-600 shadow-sm'
                      : 'bg-white text-gray-600 border-gray-200 hover:bg-gray-50'
                    }`}
                >
                  {p}
                </button>
              )
            )}

            {/* Next */}
            <button
              onClick={() => setCurrentPage(p => Math.min(total_pages, p + 1))}
              disabled={page >= total_pages}
              className="inline-flex items-center gap-1 px-2.5 py-1.5 text-sm font-medium rounded-md border border-gray-200 bg-white text-gray-600 hover:bg-gray-50 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              Next
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>
        );
      })()}
    </div>
  );
}
