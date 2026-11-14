import { useState, useEffect, useCallback, useMemo } from 'react';
import { useParams, Link } from 'react-router-dom';
import { api, getAccessToken } from '../lib/api';
import { errorMessage } from '../lib/errors';
import RatingBadge from '../components/RatingBadge';
import { AlertTriangle, Eye, EyeOff, ChevronLeft, ChevronRight } from 'lucide-react';
import { EmptyState } from '../components/EmptyState';
import { formatPenaltyMinutes } from '../lib/contestTime';

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
  /** Set for a generated onsite account: the team it was minted for. */
  team_name?: string;
  institution?: string;
  rating?: number;
  total_solved: number;
  total_penalty: number;
  total_score: number;
  problems: Record<string, ProblemCell>;
}

/**
 * One column header on the board: the letter and its point value.
 *
 * The API deliberately does NOT send the problem's title or slug. Both are the
 * problem's name — the slug is generated from the title — and the standings are
 * watchable by people who are not in the round, so the name is the one fact that
 * would give the statement away. The board identifies problems by letter.
 */
interface ProblemMeta {
  problem_id: string;
  index: string;
  score: number;
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
  is_team_contest?: boolean;
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

/**
 * Whether the round's clock has run out.
 *
 * `contest.status` is the review state — draft, pending, published, rejected — so
 * it is never 'ended'. Gating the admin rating button on `status === 'ended'` made
 * that button unreachable for every round, including finished ones.
 */
function hasEnded(endTime?: string): boolean {
  if (!endTime) return false;
  return new Date(endTime).getTime() < Date.now();
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
    queueMicrotask(() => setCurrentPage(1));
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
    } catch (e) {
      setError(errorMessage(e) || 'Failed to load scoreboard');
    } finally {
      setLoading(false);
    }
  }, [id]);

  const fetchRatings = useCallback(async () => {
    if (!id) return;
    try {
      const res = await api.ratings.getByContest(id);
      const map = new Map<string, RatingDelta>();
      (res.data || []).forEach((r) => {
        map.set(r.user_id, { user_id: r.user_id, rating_change: r.rating_change, new_rating: r.new_rating });
      });
      setRatings(map);
    } catch {
      // ratings may not exist yet — that's fine
    }
  }, [id]);

  useEffect(() => {
    queueMicrotask(() => {
      fetchScoreboard(viewMode, currentPage);
      fetchRatings();
    });
  }, [fetchScoreboard, fetchRatings, viewMode, currentPage]);

  // Auto-refresh every 30s while the round is still running
  useEffect(() => {
    if (!data?.contest || hasEnded(data.contest.end_time)) return;
    const interval = setInterval(() => {
      fetchScoreboard(viewMode, currentPage);
    }, 30_000);
    return () => clearInterval(interval);
  }, [data?.contest?.end_time, fetchScoreboard, viewMode, currentPage]); // eslint-disable-line react-hooks/exhaustive-deps

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
        // The endpoint answers plain text, not JSON: the status line is the only
        // place the reason appears, so read it instead of reporting "HTTP 409".
        const text = await res.text().catch(() => '');
        throw new Error(text.trim() || `HTTP ${res.status}`);
      }
      // The counts are the operator's only evidence that the rating window did
      // something: "rated 22 of 25" is the distribution, "done" is not.
      const body = (await res.json().catch(() => null)) as { participants?: number; rated?: number } | null;
      setRatingMsg(
        body && typeof body.rated === 'number' && typeof body.participants === 'number'
          ? `Rated ${body.rated} of ${body.participants} entrants.`
          : 'Ratings applied.',
      );
      await Promise.all([fetchScoreboard(viewMode, currentPage), fetchRatings()]);
    } catch (e) {
      setRatingMsg(errorMessage(e) || 'Failed to trigger rating');
    } finally {
      setRatingLoading(false);
    }
  };

  // ── Render ───────────────────────────────────────────────────────────────

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-muted text-lg">Loading scoreboard...</div>
      </div>
    );
  }

  if (error || !data) {
    // A board that is not published to spectators answers 403, and a bare
    // "forbidden" is the wrong thing to show someone who followed a link to watch
    // a round. Say what is actually true: this board is for the people in the
    // round. The scoreboard has its own visibility setting, so a private round
    // whose setter kept the default lands here rather than looking broken.
    const locked = /forbidden|403/i.test(error);
    return (
      <div className="max-w-2xl mx-auto mt-16 text-center">
        {locked ? (
          <>
            <h1 className="text-xl font-bold text-ink mb-2">This scoreboard is not public</h1>
            <p className="text-sm text-muted-soft mb-4">
              The setter has kept standings visible to participants only. If you have the contest
              password, open the contest page to unlock it.
            </p>
          </>
        ) : (
          <div className="text-bad text-lg mb-4">{error || 'Scoreboard not found'}</div>
        )}
        <Link to={`/contests/${id}`} className="text-brand hover:underline">
          &larr; Back to contest
        </Link>
      </div>
    );
  }

  const { entries, problems, frozen, contest, pagination } = data;
  const oi = isOI(contest.format);
  const ended = hasEnded(contest.end_time);
  const problemIndices = problems.map((p) => p.index).sort();

  return (
    <div>
      {/* Back link */}
      <div className="mb-3 flex items-center justify-between">
        <div className="min-w-0">
          <Link to={`/contests/${id}`} className="text-xs font-medium text-brand hover:underline">
            &larr; {contest.title || 'Back to contest'}
          </Link>
          {/* The page had no heading at all — it opened straight into the
              standings table, so there was nothing for a screen reader to
              announce and nothing to orient a returning visitor. */}
          <h1 className="mt-1 text-xl font-semibold tracking-tight text-ink">Standings</h1>
        </div>

        {/* Judge/Public view toggle — only visible to admins/judges */}
        {canSeeJudge && (
          <div className="inline-flex items-center rounded-lg border border-hairline bg-surface shadow-sm overflow-hidden text-sm">
            <button
              onClick={() => setViewMode('judge')}
              className={`flex items-center gap-1.5 px-3 py-1.5 font-medium transition-colors ${viewMode === 'judge'
                ? 'bg-brand text-on-brand'
                : 'text-muted hover:bg-surface-2'
                }`}
            >
              <Eye className="w-3.5 h-3.5" />
              Judge View
            </button>
            <button
              onClick={() => setViewMode('public')}
              className={`flex items-center gap-1.5 px-3 py-1.5 font-medium transition-colors ${viewMode === 'public'
                ? 'bg-brand text-on-brand'
                : 'text-muted hover:bg-surface-2'
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
        <div className="mb-3 px-4 py-2.5 rounded border text-sm font-medium bg-warn/10 border-warn/40 text-warn flex items-center gap-2">
          <AlertTriangle className="w-4 h-4 flex-shrink-0" />
          Viewing public standings — frozen submissions are hidden. Switch to Judge View to see real-time data.
        </div>
      )}
      {frozen && !canSeeJudge && (
        <div className="mb-3 px-4 py-2.5 rounded border text-sm font-medium bg-warn/10 border-warn/40 text-warn flex items-center gap-2">
          <AlertTriangle className="w-4 h-4 flex-shrink-0" />
          The scoreboard is frozen. New submissions will not be visible until the contest ends.
        </div>
      )}
      {viewMode === 'judge' && canSeeJudge && (
        <div className="mb-3 px-4 py-2.5 rounded border text-sm font-medium bg-brand-soft border-brand/40 text-brand flex items-center gap-2">
          <Eye className="w-4 h-4 flex-shrink-0" />
          Judge view — you are seeing real-time data including pending submissions.
          {frozen && ' The scoreboard is frozen for regular users.'}
        </div>
      )}

      {/* Admin rating controls. Hidden once the ratings are applied: the endpoint
          refuses a second run, so offering "Recalculate" would only produce an
          error for a button that looks like it should work. */}
      {isAdmin && contest.is_rated && ended && (
        <div className="mb-3 flex items-center gap-3">
          {contest.rating_calculated ? (
            <span className="text-sm text-muted">Ratings have been applied for this round.</span>
          ) : (
            <button
              onClick={handleRateContest}
              disabled={ratingLoading}
              className="px-4 py-1.5 text-sm font-medium rounded bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50 transition-colors"
            >
              {ratingLoading ? 'Applying...' : 'Rate This Contest'}
            </button>
          )}
          {ratingMsg && (
            <span className="text-sm text-muted">{ratingMsg}</span>
          )}
        </div>
      )}

      {/* Scoreboard table */}
      <div className="overflow-x-auto bg-surface border border-hairline-strong rounded-lg shadow-md">
        <table className="w-full border-collapse text-sm" style={{ minWidth: 600 + problemIndices.length * 72 }}>
          <thead>
            <tr className="bg-code text-code-ink">
              <th className="px-3 py-2.5 text-center font-bold w-10">#</th>
              <th className="px-4 py-2.5 text-left font-bold min-w-[140px]">{data?.contest.is_team_contest ? 'Team' : 'Participant'}</th>
              <th className="px-3 py-2.5 text-center font-bold w-12">Solved</th>
              <th className="px-3 py-2.5 text-center font-bold w-20">Penalty</th>
              {ratings.size > 0 && (
                <th className="px-3 py-2.5 text-center font-bold w-28">Rating</th>
              )}
              {problemIndices.map((idx) => {
                return (
                  <th key={idx} className="px-2 py-2.5 text-center font-bold w-[72px]">
                    <div className="text-sm">{idx}</div>
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody className="divide-y divide-hairline">
            {entries.length === 0 ? (
              <tr>
                <td
                  colSpan={4 + problemIndices.length + (ratings.size > 0 ? 1 : 0)}
                  className="px-4 py-4"
                >
                  <EmptyState text="No participants yet" description="Registered participants will appear on the board once the contest starts." />
                </td>
              </tr>
            ) : (
              entries.map((entry) => {
                const isMe = entry.user_id === userId
                const medal = entry.rank === 1 ? 'gold' : entry.rank === 2 ? 'silver' : entry.rank === 3 ? 'bronze' : null
                const ratingDelta = ratings.get(entry.user_id);

                // The API sends the penalty in minutes, not seconds.
                const penaltyStr = formatPenaltyMinutes(entry.total_penalty ?? 0)

                return (
                  <tr
                    key={entry.user_id}
                    className={`transition-colors ${isMe ? 'bg-brand-soft' : ''}`}
                  >
                    {/* Rank */}
                    <td className="px-3 py-2.5 text-center">
                      {medal ? (
                        <span className={`inline-flex items-center justify-center w-8 h-8 rounded-full text-sm font-black ${medal === 'gold' ? 'bg-amber-400 text-amber-950' :
                          medal === 'silver' ? 'bg-hairline-strong text-ink' :
                            'bg-warn-fill text-white'
                          }`}>{entry.rank}</span>
                      ) : (
                        <span className="text-sm font-bold text-muted">{entry.rank}</span>
                      )}
                    </td>

                    {/* Username */}
                    <td className={`px-4 py-2.5 font-semibold ${isMe ? 'text-brand' : 'text-ink'}`}>
                      <div className="flex items-center gap-1.5 min-w-0">
                        {/* A generated onsite account has no profile page worth
                            opening and a handle nobody recognises, so the team
                            it was minted for is shown unlinked. */}
                        {entry.team_name ? (
                          <span className="font-medium truncate" title={entry.username}>
                            {entry.team_name}
                          </span>
                        ) : (
                          <Link
                            to={`/user/${entry.username}`}
                            className="font-medium truncate hover:underline"
                          >
                            {entry.username}
                          </Link>
                        )}
                        {entry.institution && (
                          <span className="text-[11px] font-normal text-muted truncate">{entry.institution}</span>
                        )}
                        {entry.rating != null && (
                          <span className="flex-shrink-0">
                            <RatingBadge rating={entry.rating} size="sm" />
                          </span>
                        )}
                        {isMe && <span className="text-[11px] bg-brand text-on-brand px-1.5 py-0.5 rounded font-bold flex-shrink-0">YOU</span>}
                      </div>
                    </td>

                    {/* Solved */}
                    <td className="px-3 py-2.5 text-center">
                      <span className="text-lg font-black text-ink">{oi ? entry.total_score : entry.total_solved}</span>
                    </td>

                    {/* Penalty */}
                    <td className="px-3 py-2.5 text-center font-mono text-muted text-xs">
                      {penaltyStr}
                    </td>

                    {/* Rating delta */}
                    {ratings.size > 0 && (
                      <td className="px-3 py-2.5 text-center">
                        {ratingDelta ? (
                          <span className="inline-flex items-center gap-1 text-xs font-medium">
                            <span className="text-muted">{ratingDelta.new_rating}</span>
                            <span className={ratingDelta.rating_change > 0 ? 'text-ok' : ratingDelta.rating_change < 0 ? 'text-bad' : 'text-muted-soft'}>
                              {ratingDelta.rating_change > 0 ? '+' : ''}{ratingDelta.rating_change}
                            </span>
                          </span>
                        ) : (
                          <span className="text-muted-soft text-xs font-medium">—</span>
                        )}
                      </td>
                    )}

                    {/* Per-problem cells */}
                    {problemIndices.map((idx) => {
                      const cell = entry.problems[idx];
                      if (!cell) {
                        return <td key={idx} className="px-1 py-2.5 text-center"><span className="text-muted-soft font-medium">—</span></td>;
                      }

                      const isFirst = firstSolves.has(`${entry.user_id}::${idx}`);
                      const timeMin = Math.floor(cell.time || 0)

                      if (cell.solved) {
                        const attemptStr = cell.attempts <= 1 ? '+' : `+${cell.attempts - 1}`
                        return (
                          <td key={idx} className="px-1 py-2.5 text-center" title={`Solved in ${timeMin}m, ${cell.attempts} attempt(s)`}>
                            <div className={`inline-flex flex-col items-center justify-center rounded-md px-2 py-1 min-w-[48px] ${isFirst ? 'bg-ok-fill text-white shadow-sm' : 'bg-ok/12 text-ok border border-ok/30'}`}>
                              <span className="text-xs font-black leading-none">{oi ? cell.score : attemptStr}</span>
                              <span className={`text-[10px] font-mono leading-none mt-0.5 ${isFirst ? 'text-white/70' : 'text-ok/70'}`}>{timeMin}m</span>
                            </div>
                          </td>
                        );
                      }

                      if (cell.pending > 0) {
                        return (
                          <td key={idx} className="px-1 py-2.5 text-center" title={`${cell.pending} pending submission(s)`}>
                            <div className="inline-flex flex-col items-center justify-center bg-brand-soft text-brand border border-brand/30 rounded-md px-2 py-1 min-w-[48px]">
                              <span className="text-xs font-black leading-none">?</span>
                              <span className="text-[10px] font-mono leading-none mt-0.5">{cell.pending}</span>
                            </div>
                          </td>
                        );
                      }

                      if (cell.attempts > 0) {
                        return (
                          <td key={idx} className="px-1 py-2.5 text-center" title={`${cell.attempts} failed attempt(s)`}>
                            <div className="inline-flex items-center justify-center bg-bad/12 text-bad border border-bad/30 rounded-md px-2 py-1 min-w-[48px]">
                              <span className="text-xs font-black">-{cell.attempts}</span>
                            </div>
                          </td>
                        );
                      }

                      return <td key={idx} className="px-1 py-2.5 text-center"><span className="text-muted-soft font-medium">—</span></td>;
                    })}
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>

      {/* Footer info */}
      <div className="mt-3 flex flex-wrap items-center gap-4 text-xs text-muted">
        <span>
          {pagination?.total ?? entries.length} participant{(pagination?.total ?? entries.length) !== 1 ? 's' : ''}
        </span>
        <span className="flex items-center gap-1">
          <span className="inline-block w-3 h-3 rounded-sm bg-ok-fill"></span>
          First solver
        </span>
        <span className="flex items-center gap-1">
          <span className="inline-block w-3 h-3 rounded-sm bg-ok/12 border border-ok/30"></span>
          Solved
        </span>
        <span className="flex items-center gap-1">
          <span className="inline-block w-3 h-3 rounded-sm bg-bad/12 border border-bad/30"></span>
          Failed
        </span>
        <span className="flex items-center gap-1">
          <span className="inline-block w-3 h-3 rounded-sm bg-brand-soft border border-brand/30"></span>
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
              className="inline-flex items-center gap-1 px-2.5 py-1.5 text-sm font-medium rounded-md border border-hairline bg-surface text-muted hover:bg-surface-2 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              <ChevronLeft className="w-4 h-4" />
              Prev
            </button>

            {/* Page numbers */}
            {pages.map((p, i) =>
              p === 'ellipsis' ? (
                <span key={`e${i}`} className="px-2 py-1.5 text-sm text-muted-soft">...</span>
              ) : (
                <button
                  key={p}
                  onClick={() => setCurrentPage(p)}
                  className={`min-w-[36px] px-2 py-1.5 text-sm font-medium rounded-md border transition-colors ${p === page
                    ? 'bg-brand text-on-brand border-brand shadow-sm'
                    : 'bg-surface text-muted border-hairline hover:bg-surface-2'
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
              className="inline-flex items-center gap-1 px-2.5 py-1.5 text-sm font-medium rounded-md border border-hairline bg-surface text-muted hover:bg-surface-2 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
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
