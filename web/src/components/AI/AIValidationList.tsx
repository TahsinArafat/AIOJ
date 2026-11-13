import type { AIValidationReport } from '../../types/api'

/**
 * The validation gates, rendered the same wherever a job is shown.
 *
 * Inline rather than in a Card, so the caller decides the frame: the Studio shows
 * it under a running job, the detail panel inside a card. Both need the same two
 * facts — which gate failed and what it said — and the previous copies differed
 * only in whether a passing gate printed its message.
 */
export default function AIValidationList({ report }: { report: AIValidationReport }) {
    if (!report.checks?.length) return null
    return (
        <ul className="space-y-1.5">
            {report.checks.map((c) => (
                <li key={c.name} className={`flex items-start gap-2 text-sm ${c.passed ? 'text-ok' : 'text-bad'}`}>
                    <span className="mt-0.5 shrink-0">{c.passed ? '✓' : '✗'}</span>
                    <span>
                        <span className="font-medium">{c.name}</span>
                        {c.message && <span className="ml-1 text-xs text-muted">— {c.message}</span>}
                    </span>
                </li>
            ))}
        </ul>
    )
}
