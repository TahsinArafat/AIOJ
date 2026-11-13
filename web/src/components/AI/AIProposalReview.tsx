import { useEffect, useState } from 'react'
import type { AIProposal } from '../../types/api'
import { ARTIFACT_SECTIONS, SECTION_LABELS, sectionHasContent } from '../../lib/aiJobs'
import { Button } from '../ui/Button'

type Props = {
    proposal: AIProposal
    /** The job's artifact_type — decides which sections may be applied. */
    artifactType: string
    /**
     * statement is forced and non-unselectable for a brand-new problem: the
     * problem's title and description come from it, so an apply without it would
     * create a problem with no statement at all. The server does not enforce this,
     * which is exactly why the form must.
     */
    requireStatement?: boolean
    applyLabel: string
    busy?: boolean
    onApply: (sections: string[]) => void
}

function previewValue(section: string, proposal: AIProposal): string {
    switch (section) {
        case 'statement':
            return JSON.stringify({ title: proposal.title, description: proposal.description }, null, 2)
        case 'solution':
            return proposal.solution_code || '(no solution code)'
        case 'tests':
            return JSON.stringify({ generator: proposal.generator_code, seeds: proposal.seeds }, null, 2)
        case 'editorial':
            return JSON.stringify({ title: proposal.editorial_title, content: proposal.editorial_content }, null, 2)
        default:
            return ''
    }
}

/**
 * Pick which parts of a proposal to write, and apply them.
 *
 * Extracted because the Studio and the job detail page both do this and had
 * already diverged: the detail page only ever offered statement/solution/tests, so
 * an editorial job applied from there had nothing to tick, while the Studio
 * handled it. The section list now comes from the job's artifact in one place.
 *
 * The selection is seeded from what the proposal actually contains, so a setter
 * reviewing a job never has to hunt for the section that has content.
 */
export default function AIProposalReview({
    proposal,
    artifactType,
    requireStatement = false,
    applyLabel,
    busy = false,
    onApply,
}: Props) {
    const candidates = ARTIFACT_SECTIONS[artifactType] ?? []
    const available = candidates.filter((s) => sectionHasContent(s, proposal))
    const [selected, setSelected] = useState<string[]>([])

    // Seeded once per proposal, then owned by the operator. Re-seeding on every
    // render would fight their unticks; deferring a microtask keeps the state
    // write out of the effect's pre-paint pass (react-hooks/set-state-in-effect).
    useEffect(() => {
        const withContent = (ARTIFACT_SECTIONS[artifactType] ?? []).filter((s) => sectionHasContent(s, proposal))
        queueMicrotask(() => setSelected(withContent))
    }, [artifactType, proposal])

    const isForced = (section: string) => requireStatement && section === 'statement'
    // An unselectable section still has to be SENT: it is checked and disabled in
    // the UI, which means it is not in `selected`, so it is added here rather than
    // relying on the operator having ticked a box they cannot untick.
    const effective = requireStatement && !selected.includes('statement')
        ? ['statement', ...selected]
        : selected
    const applicable = available.filter((s) => isForced(s) || effective.includes(s))

    if (available.length === 0) {
        return (
            <p className="text-sm text-muted">
                This proposal produced no content for its artifact type ({artifactType.replaceAll('_', ' ')}).
            </p>
        )
    }

    return (
        <div className="space-y-3">
            {available.map((section) => {
                const forced = isForced(section)
                return (
                    <label key={section} className="block cursor-pointer rounded border border-hairline p-3 hover:bg-surface-2">
                        <span className="flex items-center gap-2 text-sm font-medium">
                            <input
                                type="checkbox"
                                className="h-4 w-4"
                                checked={forced || selected.includes(section)}
                                disabled={forced}
                                onChange={(e) => setSelected(e.target.checked
                                    ? [...selected, section]
                                    : selected.filter((x) => x !== section))}
                            />
                            {SECTION_LABELS[section] || section}
                        </span>
                        <pre className="mt-2 max-h-48 overflow-auto whitespace-pre-wrap rounded bg-code px-3 py-2 font-mono text-xs text-code-ink">
                            {previewValue(section, proposal)}
                        </pre>
                    </label>
                )
            })}

            {proposal.title && (
                <div className="space-y-1">
                    <p className="text-sm font-medium">Tags &amp; difficulty</p>
                    <p className="text-sm text-muted-soft">
                        {proposal.difficulty}
                        {proposal.tags?.length ? ` · ${proposal.tags.join(', ')}` : ''}
                    </p>
                    {proposal.time_limit_ms ? (
                        <p className="text-xs text-muted">
                            {proposal.time_limit_ms} ms · {((proposal.memory_limit_kb ?? 262144) / 1024).toFixed(0)} MB
                        </p>
                    ) : null}
                </div>
            )}

            <Button
                size="sm"
                onClick={() => onApply(effective)}
                disabled={busy || applicable.length === 0}
            >
                {busy ? 'Applying…' : applyLabel}
            </Button>
        </div>
    )
}
