import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Check, Copy, Loader2 } from 'lucide-react'
import { VerdictBadge } from './ui/VerdictBadge'
import { isPendingVerdict } from './ui/verdict'
import { languageName } from '../lib/languages'
import { useLanguages } from '../lib/useLanguages'

/**
 * The submission the user just created — id plus a live verdict.
 *
 * Both submit surfaces (practice and contest) used to render their own small
 * status strip, and the practice one showed nothing but the verdict word: no
 * submission id, so a competitor could not quote it, open it, or tell whether
 * the judge had even picked the run up. This panel is the one place that
 * answers "what did I just submit and what is happening to it", and it is
 * rendered the instant the POST returns (status `pending`), before the first
 * poll lands.
 */
export interface SubmissionStatusData {
    id: string
    status?: string | null
    language?: string | null
    time_used?: number | null
    memory_used?: number | null
    score?: number | null
    compile_output?: string | null
}

/** Short, copyable form of a submission id (UUIDs are too long for a chip). */
function shortId(id: string): string {
    return id.length > 12 ? id.substring(0, 8) : id
}

function formatTime(ms?: number | null): string | null {
    if (ms === null || ms === undefined || ms <= 0) return null
    return ms < 1000 ? `${ms} ms` : `${(ms / 1000).toFixed(2)} s`
}

function formatMemory(kb?: number | null): string | null {
    if (kb === null || kb === undefined || kb <= 0) return null
    return kb >= 1024 ? `${Math.round(kb / 1024)} MB` : `${kb} KB`
}

function Meta({ label, value }: { label: string; value: string }) {
    return (
        <div className="flex items-baseline gap-1.5">
            <span className="text-muted">{label}</span>
            <span className="tnum font-medium text-ink-soft">{value}</span>
        </div>
    )
}

export function SubmissionStatusPanel({
    submission,
    polling = false,
    className = '',
}: {
    submission: SubmissionStatusData
    /** True while the page is still polling this submission for a verdict. */
    polling?: boolean
    className?: string
}) {
    // Names come from the judge's list, so a rename in admin#languages shows here.
    const { languages } = useLanguages()
    const [copied, setCopied] = useState(false)

    const copyId = async () => {
        try {
            await navigator.clipboard.writeText(submission.id)
            setCopied(true)
            setTimeout(() => setCopied(false), 2000)
        } catch {
            // Clipboard blocked (insecure origin / no permission): still let
            // the user select the id from the row.
        }
    }

    const status = submission.status || 'pending'
    const pending = isPendingVerdict(status)
    const time = formatTime(submission.time_used)
    const memory = formatMemory(submission.memory_used)
    const language = submission.language ? languageName(submission.language, languages) : null
    const score = typeof submission.score === 'number' && submission.score > 0 ? String(submission.score) : null
    const showCompileOutput = status === 'ce' && !!submission.compile_output

    return (
        <div
            aria-live="polite"
            className={`rounded-lg border border-hairline bg-surface p-3.5 text-sm space-y-3 ${className}`}
        >
            <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="flex min-w-0 items-center gap-2">
                    <span className="text-xs font-semibold uppercase tracking-wider text-muted">Submission</span>
                    <Link
                        to={`/submissions/${submission.id}`}
                        title={submission.id}
                        className="truncate font-mono text-sm font-semibold text-brand hover:underline"
                    >
                        #{shortId(submission.id)}
                    </Link>
                    <button
                        onClick={copyId}
                        title={copied ? 'Copied' : 'Copy full submission id'}
                        className="rounded p-1 text-muted transition-colors hover:bg-surface-3 hover:text-ink cursor-pointer"
                    >
                        {copied
                            ? <Check className="h-3.5 w-3.5" aria-hidden="true" />
                            : <Copy className="h-3.5 w-3.5" aria-hidden="true" />}
                        <span className="sr-only">{copied ? 'Copied' : 'Copy submission id'}</span>
                    </button>
                </div>
                <div className="flex items-center gap-2">
                    <VerdictBadge status={status} className="px-2.5 py-1 text-xs" />
                    <Link
                        to={`/submissions/${submission.id}`}
                        className="text-xs font-medium text-brand hover:underline"
                    >
                        View details →
                    </Link>
                </div>
            </div>

            {pending && (
                <div className="flex items-center gap-2 text-xs text-muted">
                    <Loader2 className="h-3.5 w-3.5 animate-spin text-brand" aria-hidden="true" />
                    <span>
                        {status === 'judging' ? 'Judging your code…' : 'In the judge queue…'}
                        {' '}
                        {polling ? 'this updates automatically.' : 'open the submission for the latest verdict.'}
                    </span>
                </div>
            )}

            {(time || memory || language || score) && (
                <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs">
                    {language && <Meta label="Language" value={language} />}
                    {time && <Meta label="Time" value={time} />}
                    {memory && <Meta label="Memory" value={memory} />}
                    {score && <Meta label="Score" value={score} />}
                </div>
            )}

            {showCompileOutput && (
                <div>
                    <span className="mb-1 block text-xs font-semibold text-muted">Compiler output</span>
                    <pre className="max-h-40 overflow-x-auto rounded border border-hairline bg-code p-2.5 font-mono text-xs text-bad-on-code">
                        {submission.compile_output}
                    </pre>
                </div>
            )}
        </div>
    )
}
