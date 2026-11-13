import type { BadgeTone } from './Badge'

/**
 * Judge verdict vocabulary — labels, abbreviations and colour tones.
 *
 * This lived copy-pasted in Submissions, SubmissionDetail, ProblemDetail,
 * ContestProblem and the admin panels, and the copies had already drifted
 * (some knew `TLE`, some did not; `se` was grey in one file and red in
 * another). It sits in its own module so the Fast Refresh rule stays happy
 * about the component file next to it.
 */

export interface VerdictMeta {
    label: string
    short: string
    tone: BadgeTone
    /**
     * A voided submission is struck through. It is a real verdict and it stays
     * visible — deleting the row would hide that something was ruled on — but
     * it must not read as a result the code earned.
     */
    struck?: boolean
}

const VERDICTS: Record<string, VerdictMeta> = {
    ac: { label: 'Accepted', short: 'AC', tone: 'ok' },
    success: { label: 'Accepted', short: 'AC', tone: 'ok' },
    wa: { label: 'Wrong Answer', short: 'WA', tone: 'bad' },
    tle: { label: 'Time Limit Exceeded', short: 'TLE', tone: 'warn' },
    mle: { label: 'Memory Limit Exceeded', short: 'MLE', tone: 'warn' },
    re: { label: 'Runtime Error', short: 'RE', tone: 'bad' },
    ce: { label: 'Compile Error', short: 'CE', tone: 'accent' },
    se: { label: 'System Error', short: 'SE', tone: 'neutral' },
    pending: { label: 'Pending', short: 'PENDING', tone: 'brand' },
    judging: { label: 'Judging', short: 'JUDGING', tone: 'brand' },
    running: { label: 'Running', short: 'RUN', tone: 'brand' },
    error: { label: 'Error', short: 'ERR', tone: 'bad' },
    // An organizer voided this submission (a resubmit-rule breach, a copied
    // solution). It is not a judge verdict: it scores nothing and does not
    // count as an attempt.
    skipped: { label: 'Skipped', short: 'SKIP', tone: 'neutral', struck: true },
}

export function verdictMeta(status: string | undefined | null): VerdictMeta {
    const key = (status || '').toLowerCase()
    return VERDICTS[key] || { label: status || 'Unknown', short: (status || '—').toUpperCase(), tone: 'neutral' }
}

/** A verdict is still in flight when the judge has not produced a result yet. */
export function isPendingVerdict(status: string | undefined | null): boolean {
    const key = (status || '').toLowerCase()
    return key === 'pending' || key === 'judging' || key === 'running'
}

/**
 * Whether a submission was voided by an organizer.
 *
 * Kept next to verdictMeta so a caller that needs to know whether a row counts
 * (a rejudge button, a "solved" tally) asks one place rather than re-spelling
 * the status and drifting from the display side.
 */
export function isSkippedVerdict(status: string | undefined | null): boolean {
    return (status || '').toLowerCase() === 'skipped'
}

/** Every verdict the UI knows, for a filter dropdown. */
export function verdictOptions(): { value: string; label: string }[] {
    return Object.keys(VERDICTS)
        .filter(k => k !== 'success' && k !== 'running' && k !== 'error')
        .map(k => ({ value: k, label: VERDICTS[k].label }))
}

/**
 * Normalise a `/run` status for comparison.
 *
 * `/run` answers in upper case ("CE", "TLE", "success") while stored
 * submissions are lower case. The in-problem IDE compared the raw value
 * against lower case, so a compile error never matched: it rendered the bare
 * string "CE" and kept the empty stdout/stderr panels on screen.
 */
export function runStatus(status: string | undefined | null): string {
    return (status || '').toLowerCase()
}
