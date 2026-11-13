import { useState } from 'react'
import AIJobList from '../../components/AI/AIJobList'
import AIJobDetail from '../../components/AI/AIJobDetail'
import { PageHeader } from '../../components/ui/PageHeader'

/**
 * Every setter's AI generation jobs, for admins.
 *
 * This is the cross-round view a per-problem Studio cannot provide — which is why
 * an admin still gets a table rather than being sent to /setter/ai. What it does
 * NOT do any more is carry its own copy of the table, filters, status tones and
 * poll loop: AIJobList is the one implementation, shared with the Studio's job
 * history, and the only difference between the two callers is `isAdmin`.
 */
export default function AIJobsPanel() {
    const [selectedId, setSelectedId] = useState<string | null>(null)

    return (
        <div>
            <PageHeader
                title="AI Generation Jobs"
                description="All AI problem generation jobs across all setters. Admins can force-cancel any running or queued job."
            />
            <AIJobList
                isAdmin
                selectedId={selectedId}
                onSelect={setSelectedId}
                renderDetail={(id) => (
                    <div className="rounded-xl border border-hairline bg-surface p-4">
                        <AIJobDetail
                            jobId={id}
                            onDiscarded={() => setSelectedId(null)}
                            onRetried={setSelectedId}
                            onApplied={() => setSelectedId(null)}
                        />
                    </div>
                )}
            />
        </div>
    )
}
