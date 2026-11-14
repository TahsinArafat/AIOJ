import { useCallback, useEffect, useMemo, useState } from 'react'
import { AlertTriangle, History, ShieldX, UserMinus } from 'lucide-react'
import { api } from '../lib/api'
import { errorMessage } from '../lib/errors'
import { useConfirm } from './ConfirmDialog'
import { EmptyState } from './EmptyState'
import type { RatingRunEntry } from '../types/api'

/**
 * The rating ledger, as an operator sees it.
 *
 * A round is paid out once, and after that the honest choices are "recompute" or
 * "withdraw" — so this is not a single button. Everything here writes to the
 * ledger rather than editing a rating, which is why the panel also shows what has
 * already happened: an operator about to withdraw a round needs to see that the
 * round after it will be replayed too.
 */

interface ExcludedEntrant {
  user_id: string
  contest_id: string
  reason: string
  created_at: string
}

interface Props {
  contestId: string
  /** Ratings have been applied at least once. */
  applied: boolean
  /** The round's ratings are currently withdrawn. */
  voided: boolean
  /**
   * The board's participant count, used only to decide whether a fuller list is
   * needed. The dropdown cannot use the page's own entries: the board is
   * paginated, and a disqualified entrant is usually not on page 1.
   */
  participantCount: number
  /** Called after any operation that changes ratings or the field. */
  onChanged: () => void | Promise<void>
}

type Op = 'apply' | 'recalculate' | 'void' | 'unvoid'

const KIND_LABEL: Record<string, string> = {
  apply: 'Rated',
  recalculate: 'Recalculated',
}

function formatWhen(iso: string) {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleString(undefined, {
    year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
  })
}

export default function ContestRatingLedger({ contestId, applied, voided, participantCount, onChanged }: Props) {
  const confirm = useConfirm()

  const [busy, setBusy] = useState<Op | 'exclude' | null>(null)
  const [msg, setMsg] = useState('')
  const [error, setError] = useState('')

  const [exclusions, setExclusions] = useState<ExcludedEntrant[]>([])
  const [runs, setRuns] = useState<RatingRunEntry[]>([])
  const [loadingDetail, setLoadingDetail] = useState(false)
  const [showHistory, setShowHistory] = useState(false)

  const [pickUser, setPickUser] = useState('')
  const [reason, setReason] = useState('')

  // The full field, so the dropdown is not limited to whatever page the operator
  // happens to be looking at. 500 is the endpoint's ceiling, which is also its
  // page size ceiling — a round larger than that is not a round this panel can
  // enumerate, and the API is the right place to fix that rather than here.
  const [field, setField] = useState<{ user_id: string; username: string }[]>([])

  const excludedIDs = useMemo(() => new Set(exclusions.map(e => e.user_id)), [exclusions])

  // Who can be disqualified: everyone on the board who is not already excluded.
  // An onsite account is shown by its handle, which is all the API sends here.
  const candidates = useMemo(
    () => field.filter(p => !excludedIDs.has(p.user_id)),
    [field, excludedIDs],
  )

  const nameOf = useCallback(
    (userID: string) => field.find(p => p.user_id === userID)?.username || userID,
    [field],
  )

  const loadDetail = useCallback(async () => {
    setLoadingDetail(true)
    try {
      const [ex, rh, board] = await Promise.all([
        api.contests.ratingExclusions(contestId),
        api.contests.ratingRuns(contestId),
        // One page sized to the whole field: the panel needs every name, not the
        // slice the scoreboard is currently showing.
        api.contests.scoreboard(contestId, 'public', 1, Math.min(Math.max(participantCount, 1), 500)),
      ])
      setExclusions(ex.data || [])
      setRuns(rh.data || [])
      setField((board.entries || []).map(e => ({
        user_id: e.user_id,
        username: e.username || e.team_name || e.user_id,
      })))
    } catch {
      // The panel still works without its detail: the operations are the point,
      // and an empty list is indistinguishable from "nothing recorded yet".
    } finally {
      setLoadingDetail(false)
    }
  }, [contestId, participantCount])

  // Deferred, like the scoreboard's own fetches: loadDetail sets state on entry,
  // and a synchronous set inside an effect body cascades a render.
  useEffect(() => { queueMicrotask(() => void loadDetail()) }, [loadDetail])

  const run = async (op: Op) => {
    const confirmations: Record<Op, { title: string; message: string; label: string } | null> = {
      apply: null,
      recalculate: {
        title: 'Recalculate from this round?',
        message: 'Every rated round after this one is replayed, because each round’s rating is computed on top of the previous one. Ratings that were already correct may shift slightly.',
        label: 'Recalculate',
      },
      void: {
        title: 'Withdraw this round’s ratings?',
        message: 'The standings stay exactly as they are — only the rating effect is withdrawn, and every later round is replayed without it. You can reinstate it afterwards.',
        label: 'Withdraw ratings',
      },
      unvoid: {
        title: 'Reinstate this round’s ratings?',
        message: 'The round counts again and every later round is replayed on top of it.',
        label: 'Reinstate',
      },
    }

    const ask = confirmations[op]
    if (ask && !(await confirm({ title: ask.title, message: ask.message, confirmLabel: ask.label, variant: op === 'void' ? 'danger' : 'default' }))) {
      return
    }

    setBusy(op)
    setError('')
    setMsg('')
    try {
      const body =
        op === 'apply' ? await api.contests.calculateRatings(contestId)
          : op === 'recalculate' ? await api.contests.recalculateRatings(contestId)
            : op === 'void' ? await api.contests.voidRatings(contestId)
              : await api.contests.unvoidRatings(contestId)

      // Both numbers matter and they are different questions: "how many entrants
      // were rated" for a payout, "how many rounds were replayed" for everything
      // else. Reporting one where the other is meant is how an operator concludes
      // nothing happened.
      if (op === 'apply' && typeof body?.rated === 'number' && typeof body?.participants === 'number') {
        setMsg(`Rated ${body.rated} of ${body.participants} entrants.`)
      } else if (typeof body?.contests === 'number') {
        setMsg(`Replayed ${body.contests} round${body.contests === 1 ? '' : 's'}.`)
      } else {
        setMsg('Done.')
      }
      if (body?.team_rating_error) {
        setError(`Team ratings were not updated: ${body.team_rating_error}`)
      }
      await Promise.all([onChanged(), loadDetail()])
    } catch (e) {
      setError(errorMessage(e) || 'Rating operation failed')
    } finally {
      setBusy(null)
    }
  }

  const addExclusion = async () => {
    if (!pickUser) return
    const name = nameOf(pickUser)
    if (!(await confirm({
      title: `Disqualify ${name}?`,
      message: 'They are removed from this round’s rated field and every later round is replayed. The round still counts for everyone else.',
      confirmLabel: 'Disqualify',
      variant: 'danger',
    }))) return

    setBusy('exclude')
    setError('')
    setMsg('')
    try {
      const body = await api.contests.addRatingExclusion(contestId, { user_id: pickUser, reason })
      setMsg(typeof body?.contests === 'number'
        ? `${name} disqualified; replayed ${body.contests} round${body.contests === 1 ? '' : 's'}.`
        : `${name} disqualified.`)
      setPickUser('')
      setReason('')
      await Promise.all([onChanged(), loadDetail()])
    } catch (e) {
      setError(errorMessage(e) || 'Could not disqualify that entrant')
    } finally {
      setBusy(null)
    }
  }

  const removeExclusion = async (userID: string) => {
    const name = nameOf(userID)
    if (!(await confirm({
      title: `Reinstate ${name}?`,
      message: 'Their result in this round counts again and every later round is replayed.',
      confirmLabel: 'Reinstate',
    }))) return

    setBusy('exclude')
    setError('')
    setMsg('')
    try {
      const body = await api.contests.removeRatingExclusion(contestId, userID)
      setMsg(typeof body?.contests === 'number'
        ? `${name} reinstated; replayed ${body.contests} round${body.contests === 1 ? '' : 's'}.`
        : `${name} reinstated.`)
      await Promise.all([onChanged(), loadDetail()])
    } catch (e) {
      setError(errorMessage(e) || 'Could not reinstate that entrant')
    } finally {
      setBusy(null)
    }
  }

  const working = busy !== null

  return (
    <section className="mb-4 rounded-lg border border-hairline-strong bg-surface shadow-sm">
      <header className="flex flex-wrap items-center gap-2 px-4 py-3 border-b border-hairline">
        <ShieldX className="w-4 h-4 text-muted flex-shrink-0" />
        <h2 className="text-sm font-semibold text-ink">Ratings</h2>
        <span className="text-xs text-muted">
          {!applied ? 'Not paid out yet' : voided ? 'Withdrawn' : 'Applied'}
        </span>

        <div className="ml-auto flex flex-wrap items-center gap-2">
          {!applied ? (
            <button
              onClick={() => run('apply')}
              disabled={working}
              className="px-3 py-1.5 text-sm font-medium rounded bg-brand text-on-brand hover:bg-brand-hover disabled:opacity-50 transition-colors"
            >
              {busy === 'apply' ? 'Applying…' : 'Rate this round'}
            </button>
          ) : (
            <>
              <button
                onClick={() => run('recalculate')}
                disabled={working || voided}
                title={voided ? 'Reinstate the ratings first' : 'Replay every rated round from this one onward'}
                className="px-3 py-1.5 text-sm font-medium rounded border border-hairline-strong text-ink hover:bg-surface-2 disabled:opacity-50 transition-colors"
              >
                {busy === 'recalculate' ? 'Replaying…' : 'Recalculate from here'}
              </button>
              <button
                onClick={() => run(voided ? 'unvoid' : 'void')}
                disabled={working}
                className={`px-3 py-1.5 text-sm font-medium rounded border transition-colors disabled:opacity-50 ${voided
                  ? 'border-hairline-strong text-ink hover:bg-surface-2'
                  : 'border-bad/40 text-bad hover:bg-bad/10'
                  }`}
              >
                {busy === 'void' || busy === 'unvoid'
                  ? 'Working…'
                  : voided ? 'Reinstate ratings' : 'Withdraw ratings'}
              </button>
            </>
          )}
        </div>
      </header>

      {(msg || error) && (
        <div className={`px-4 py-2 text-sm border-b border-hairline ${error ? 'text-bad bg-bad/5' : 'text-muted'}`}>
          {error || msg}
        </div>
      )}

      {/* Disqualifications. A disqualified entrant is not a deleted result: the
          round still counts for everyone else, and the field shrinks, which is
          why this re-ranks the round rather than editing one row. */}
      <div className="px-4 py-3 border-b border-hairline">
        <div className="flex items-center gap-2 mb-2">
          <UserMinus className="w-3.5 h-3.5 text-muted" />
          <h3 className="text-xs font-semibold uppercase tracking-wide text-muted">Disqualified entrants</h3>
        </div>

        {exclusions.length === 0 ? (
          <p className="text-xs text-muted-soft">Nobody is disqualified from this round.</p>
        ) : (
          <ul className="space-y-1.5">
            {exclusions.map(e => (
              <li key={e.user_id} className="flex flex-wrap items-center gap-2 text-sm">
                <span className="font-medium text-ink">{nameOf(e.user_id)}</span>
                {e.reason && <span className="text-xs text-muted">— {e.reason}</span>}
                <button
                  onClick={() => removeExclusion(e.user_id)}
                  disabled={working}
                  className="ml-auto text-xs font-medium text-brand hover:underline disabled:opacity-50"
                >
                  Reinstate
                </button>
              </li>
            ))}
          </ul>
        )}

        <div className="mt-2 flex flex-wrap items-center gap-2">
          <select
            aria-label="Entrant to disqualify"
            value={pickUser}
            onChange={e => setPickUser(e.target.value)}
            disabled={working || candidates.length === 0}
            className="min-w-0 flex-1 sm:flex-none sm:w-48 px-2 py-1.5 text-sm rounded border border-hairline-strong bg-surface text-ink disabled:opacity-50"
          >
            <option value="">
              {candidates.length === 0 ? 'No eligible entrants' : 'Select an entrant…'}
            </option>
            {candidates.map(p => (
              <option key={p.user_id} value={p.user_id}>{p.username}</option>
            ))}
          </select>
          <input
            aria-label="Reason for disqualification"
            value={reason}
            onChange={e => setReason(e.target.value)}
            placeholder="Reason (recorded)"
            disabled={working}
            className="min-w-0 flex-1 px-2 py-1.5 text-sm rounded border border-hairline-strong bg-surface text-ink disabled:opacity-50"
          />
          <button
            onClick={addExclusion}
            disabled={working || !pickUser}
            className="px-3 py-1.5 text-sm font-medium rounded border border-bad/40 text-bad hover:bg-bad/10 disabled:opacity-50 transition-colors"
          >
            {busy === 'exclude' ? 'Working…' : 'Disqualify'}
          </button>
        </div>
      </div>

      {/* Run history. This is the question a dispute actually asks, so it is a
          table rather than a log line, and it outlives the session that made the
          change. */}
      <div className="px-4 py-3">
        <button
          onClick={() => setShowHistory(v => !v)}
          aria-expanded={showHistory}
          className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-muted hover:text-ink transition-colors"
        >
          <History className="w-3.5 h-3.5" />
          History {runs.length > 0 && <span className="font-normal normal-case">({runs.length})</span>}
        </button>

        {showHistory && (
          loadingDetail ? (
            <p className="mt-2 text-xs text-muted-soft">Loading…</p>
          ) : runs.length === 0 ? (
            <div className="mt-2">
              <EmptyState text="No rating operations yet" description="Payouts, recalculations and withdrawals are recorded here with the operator who made them." />
            </div>
          ) : (
            <ul className="mt-2 space-y-1.5">
              {runs.map(r => (
                <li key={r.id} className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5 text-xs">
                  <span className="font-medium text-ink">{KIND_LABEL[r.kind] || r.kind}</span>
                  <span className="text-muted-soft">{formatWhen(r.created_at)}</span>
                  {r.actor_name && <span className="text-muted">by {r.actor_name}</span>}
                  {r.reason && <span className="text-muted-soft">— {r.reason}</span>}
                </li>
              ))}
            </ul>
          )
        )}
      </div>

      {applied && !voided && (
        <p className="flex items-start gap-2 px-4 py-2.5 text-xs text-muted border-t border-hairline">
          <AlertTriangle className="w-3.5 h-3.5 flex-shrink-0 mt-0.5" />
          Ratings are never edited in place. Withdrawing or disqualifying supersedes the old
          decision and replays every rated round after this one, so the history above stays readable.
        </p>
      )}
    </section>
  )
}
