import { useState } from 'react'
import { Check, Copy, Download } from 'lucide-react'
import { StatementBody, StatementSection } from './StatementSection'

/**
 * The problem statement, in one place.
 *
 * The practice page and the contest page each rendered their own copy of this:
 * two PDF previews, two sample-case layouts ("Standard Input" vs "Input"), two
 * copy buttons and two hint treatments. That is how the contest version drifted
 * far enough to lose whole sections. There is one renderer now, and the only
 * thing a caller chooses is what the current context is allowed to reveal.
 */

export interface StatementSample {
    input: string
    output: string
    explanation?: string | null
}

export interface StatementProblem {
    description?: string | null
    input_format?: string | null
    output_format?: string | null
    constraints?: string | null
    hint?: string | null
    sample_cases?: StatementSample[] | null
}

function isPdfUrl(url: string | undefined | null): boolean {
    if (!url) return false
    try {
        const parsed = new URL(url)
        if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return false
    } catch {
        return false
    }
    const lower = url.toLowerCase()
    return lower.endsWith('.pdf') || lower.includes('/problem.pdf')
}

function CopyButton({ text }: { text: string }) {
    const [copied, setCopied] = useState(false)

    const markCopied = () => {
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
    }

    const handleCopy = async () => {
        try {
            await navigator.clipboard.writeText(text)
            markCopied()
        } catch {
            // clipboard API needs a secure context; fall back to the old trick
            const ta = document.createElement('textarea')
            ta.value = text
            document.body.appendChild(ta)
            ta.select()
            document.execCommand('copy')
            document.body.removeChild(ta)
            markCopied()
        }
    }

    return (
        <button
            onClick={handleCopy}
            className="text-xs text-muted hover:text-ink transition-colors px-2 py-1 rounded hover:bg-surface-3 cursor-pointer"
            title="Copy to clipboard"
        >
            {copied ? <><Check className="w-3.5 h-3.5" /> Copied</> : <><Copy className="w-3.5 h-3.5" /> Copy</>}
        </button>
    )
}

function PdfStatement({ url }: { url: string }) {
    return (
        <div className="space-y-3">
            <h3 className="text-xs font-semibold uppercase tracking-wider text-muted">PDF Problem Statement</h3>
            <a
                href={url}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-2 rounded-lg border border-brand/30 bg-brand-soft px-4 py-2 text-sm font-medium text-brand transition-colors hover:bg-brand-soft"
            >
                <Download className="h-4 w-4" />
                Download PDF
            </a>
            <iframe
                src={url}
                className="h-[800px] w-full rounded-lg border-0 bg-surface shadow-sm"
                title="Problem Statement"
            />
        </div>
    )
}

function SampleCase({ sample, index }: { sample: StatementSample; index: number }) {
    return (
        <div className="overflow-hidden rounded-md border border-hairline">
            <div className="border-b border-hairline bg-surface-2 px-3 py-2">
                <span className="text-xs font-semibold text-muted">Sample {index + 1}</span>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2">
                <div className="border-b border-hairline md:border-b-0 md:border-r">
                    <div className="flex items-center justify-between border-b border-hairline bg-surface-2 px-3 py-2">
                        <span className="text-xs font-semibold text-muted">Input</span>
                        <CopyButton text={sample.input} />
                    </div>
                    <pre className="whitespace-pre-wrap break-all p-3 font-mono text-[13px] text-ink">{sample.input}</pre>
                </div>
                <div>
                    <div className="flex items-center justify-between border-b border-hairline bg-surface-2 px-3 py-2">
                        <span className="text-xs font-semibold text-muted">Output</span>
                        <CopyButton text={sample.output} />
                    </div>
                    <pre className="whitespace-pre-wrap break-all p-3 font-mono text-[13px] text-ink">{sample.output}</pre>
                </div>
            </div>
            {sample.explanation && (
                <div className="border-t border-hairline bg-brand-soft px-3 py-2">
                    <div className="mb-1 text-xs font-semibold uppercase tracking-wider text-brand">Explanation</div>
                    <StatementBody body={sample.explanation} className="prose prose-sm max-w-none text-xs text-ink-soft" />
                </div>
            )}
        </div>
    )
}

function Notes({ hint, collapsible }: { hint: string; collapsible: boolean }) {
    const [expanded, setExpanded] = useState(!collapsible)

    if (!collapsible) return <StatementSection title="Notes" body={hint} />

    return (
        <div className="overflow-hidden rounded-lg border border-warn/30">
            <button
                onClick={() => setExpanded(v => !v)}
                aria-expanded={expanded}
                className="flex w-full items-center justify-between bg-warn/10 px-4 py-3 text-left transition-colors hover:bg-warn/12"
            >
                <span className="text-sm font-semibold text-warn">Hint</span>
                <span className="text-sm text-warn">{expanded ? '▲ Hide' : '▼ Show'}</span>
            </button>
            {expanded && (
                <div className="border-t border-warn/30 bg-surface px-4 py-3">
                    <StatementBody
                        body={hint}
                        className="prose prose-sm max-w-none text-[14px] leading-[1.65] text-ink-soft"
                    />
                </div>
            )}
        </div>
    )
}

export function ProblemStatement({
    problem,
    hideSpoilers = false,
    collapsibleHint = false,
    showStatement = true,
}: {
    problem: StatementProblem
    /**
     * The problem is still live in a contest. The server withholds these fields
     * already; this keeps the UI honest even if one slips through, and drops the
     * hint, which the server clears for the same reason.
     */
    hideSpoilers?: boolean
    /** Contest UX: fold the notes away so they are not read by accident. */
    collapsibleHint?: boolean
    /**
     * Hide the prose sections but keep the samples. Contests can do this with
     * `statement_hidden`, and samples are deliberately still shown.
     */
    showStatement?: boolean
}) {
    const hint = hideSpoilers ? '' : (problem.hint ?? '')

    return (
        <div className="space-y-6">
            {showStatement && (
                isPdfUrl(problem.description) ? (
                    <PdfStatement url={problem.description as string} />
                ) : (
                    <StatementBody body={problem.description} />
                )
            )}
            {showStatement && problem.input_format && (
                <StatementSection title="Input format" body={problem.input_format} />
            )}
            {showStatement && problem.output_format && (
                <StatementSection title="Output format" body={problem.output_format} />
            )}
            {showStatement && problem.constraints && (
                <StatementSection title="Constraints" body={problem.constraints} />
            )}
            {problem.sample_cases && problem.sample_cases.length > 0 && (
                <div>
                    <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted">Samples</h3>
                    <div className="space-y-4">
                        {problem.sample_cases.map((sample, i) => (
                            <SampleCase key={i} sample={sample} index={i} />
                        ))}
                    </div>
                </div>
            )}
            {hint && <Notes hint={hint} collapsible={collapsibleHint} />}
        </div>
    )
}
