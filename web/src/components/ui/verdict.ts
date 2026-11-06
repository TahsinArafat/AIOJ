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
