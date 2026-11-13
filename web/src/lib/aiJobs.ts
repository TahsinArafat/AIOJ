import type { AIGenerationJob } from '../types/api'
import type { BadgeTone } from '../components/ui/Badge'

/**
 * The one reading of an AI generation job's state.
 *
 * There were four copies of this before — AIStudioTab, AIJobsPage, AIJobDetailPage
 * and the admin tab — each with its own status tones, elapsed formatting and
 * liveness test. They had already drifted: the table said "1000ms" where the detail
 * page said "1000 ms", and one copy coloured a cancelled job neutral while another
 * had no case for it at all. None of that is a bug on its own; what makes it worth
 * fixing is that the shapes a setter acts on — is this job still running, may I
 * retry it, did it fail validation — were answered in four places and agreed by
 * luck rather than by construction.
 *
 * Pure logic, no JSX: this file is imported by components but exports none, which
 * is what keeps the react-refresh boundary in web/src/lib honest.
 */

/** Role and user id from the access token, or null when there is none. */
export function decodeToken(): { role: string | null; userID: string | null } {
    const token = localStorage.getItem('access_token')
    if (!token) return { role: null, userID: null }
    try {
        const payload = JSON.parse(atob(token.split('.')[1]))
        return { role: payload.role ?? null, userID: payload.uid ?? null }
    } catch {
        return { role: null, userID: null }
    }
}

/** Admin or setter — the only two roles any AI surface admits. */
export function canUseAIStudio(role: string | null): boolean {
    return role === 'admin' || role === 'setter'
}

const STATUS_TONES: Record<string, BadgeTone> = {
    queued: 'neutral',
    running: 'brand',
    ready: 'ok',
    validation_failed: 'warn',
    failed: 'bad',
    cancelled: 'neutral',
    applied: 'ok',
    discarded: 'neutral',
}

export function statusTone(status: string): BadgeTone {
    return STATUS_TONES[status] ?? 'neutral'
}

export function statusLabel(status: string): string {
    return status.replaceAll('_', ' ')
}

/**
 * The statuses in the order a setter cares about them: the two that mean "wait",
 * then the outcomes. Drives the filter dropdown so its order is not a third thing
 * to keep in sync.
 */
export const JOB_STATUSES = [
    'queued',
    'running',
    'ready',
    'validation_failed',
    'failed',
    'cancelled',
    'applied',
    'discarded',
] as const

/**
 * Whether a job is still doing something. The single answer to the question the
 * poll loops, the progress bar and the cancel button all ask — three places that
 * used to spell the pair out themselves.
 */
export function isJobActive(job: Pick<AIGenerationJob, 'status'> | null | undefined): boolean {
    return !!job && (job.status === 'queued' || job.status === 'running')
}

/** A finished job the operator may start again. */
export function canRetryJob(job: Pick<AIGenerationJob, 'status'>): boolean {
    return job.status === 'failed' || job.status === 'validation_failed' || job.status === 'cancelled'
}

/** A proposal that can be reviewed and applied. */
export function isJobReviewable(job: Pick<AIGenerationJob, 'status'>): boolean {
    return job.status === 'ready'
}

/**
 * How long the run took, or has been taking.
 *
 * Uses started_at/finished_at when both are present, falls back to the recorded
 * latency, and reports the live elapsed time for a job that is still going. One
 * unit spelling throughout ("1.2s", not "1.2 s" in one place and "1.2s" in
 * another) because these numbers sit in the same table.
 */
export function elapsed(job: AIGenerationJob): string {
    const ms = elapsedMs(job)
    if (ms === null) return '—'
    return ms < 1000 ? `${ms}ms` : `${(ms / 1000).toFixed(1)}s`
}

function elapsedMs(job: AIGenerationJob): number | null {
    if (job.finished_at && job.started_at) {
        return new Date(job.finished_at).getTime() - new Date(job.started_at).getTime()
    }
    if (job.started_at && isJobActive(job)) {
        return Date.now() - new Date(job.started_at).getTime()
    }
    if (job.latency_ms > 0) return job.latency_ms
    return null
}

/** Prompt plus completion tokens, or '—' before any have been spent. */
export function totalTokens(job: Pick<AIGenerationJob, 'prompt_tokens' | 'completion_tokens'>): string {
    const total = job.prompt_tokens + job.completion_tokens
    return total > 0 ? total.toLocaleString() : '—'
}

/**
 * Which apply sections an artifact can produce.
 *
 * The reason this is data rather than four inline conditionals: the server decides
 * what a job produced, and the review UI has to offer exactly that. A section
 * offered for an artifact that cannot produce it is an apply request for empty
 * content, which the server accepts and writes.
 */
export const ARTIFACT_SECTIONS: Record<string, string[]> = {
    full_problem: ['statement', 'solution', 'tests'],
    statement: ['statement'],
    tests: ['tests'],
    solution: ['solution'],
    editorial: ['editorial'],
}

/** Human-readable labels for the apply checkboxes. */
export const SECTION_LABELS: Record<string, string> = {
    statement: 'Statement & samples',
    solution: 'Reference solution',
    tests: 'Test data & generator',
    editorial: 'Editorial',
}

/** Whether a proposal actually carries content for one section. */
export function sectionHasContent(section: string, proposal: {
    title?: string
    solution_code?: string
    generator_code?: string
    editorial_title?: string
    editorial_content?: string
}): boolean {
    switch (section) {
        case 'statement':
            return !!proposal.title
        case 'solution':
            return !!proposal.solution_code
        case 'tests':
            return !!proposal.generator_code
        case 'editorial':
            return !!proposal.editorial_title || !!proposal.editorial_content
        default:
            return false
    }
}
