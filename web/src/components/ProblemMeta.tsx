import { Gauge, MemoryStick, Timer, type LucideIcon } from 'lucide-react'
import { Badge } from './ui/Badge'
import { DIFFICULTY_TONE } from '../lib/difficulty'

/**
 * The limits-and-labels strip under a problem title.
 *
 * The practice page and the contest page each had their own: one showed
 * "1.5 s" with icon chips plus difficulty, rating and tags, the other showed
 * "1500 ms" separated by pipes and nothing else. Both render this now.
 */

/** A limit chip: icon, label and value on one line. */
function Limit({ icon: Icon, label, value }: { icon: LucideIcon; label: string; value: string }) {
    return (
        <span className="flex items-center gap-1.5">
            <Icon className="h-3.5 w-3.5 text-muted-soft" aria-hidden="true" />
            <span className="text-muted">{label}</span>
            <span className="tnum font-medium text-ink">{value}</span>
        </span>
    )
}

export function ProblemMeta({
    timeLimit,
    memoryLimit,
    difficulty,
    rating,
    tags,
    interactive = false,
    hideSpoilers = false,
    className = '',
}: {
    /** Milliseconds, as stored. */
    timeLimit: number
    /** Kilobytes, as stored. */
    memoryLimit: number
    difficulty?: string
    /** Codeforces-scale rating; 0 or undefined means unrated. */
    rating?: number
    tags?: string[]
    interactive?: boolean
    /**
     * The problem is live in a contest. Difficulty, rating and tags point at the
     * intended solution, so they stay off the page until the round is over — the
     * same rule Codeforces applies. The server withholds them already; this is
     * the second half of one policy, not a separate decision.
     */
    hideSpoilers?: boolean
    className?: string
}) {
    const showDifficulty = hideSpoilers ? '' : difficulty
    const showRating = hideSpoilers ? 0 : rating
    const showTags = hideSpoilers ? [] : (tags ?? [])

    return (
        <div className={`flex flex-wrap items-center gap-x-3 gap-y-2 text-xs ${className}`}>
            <Limit icon={Timer} label="Time" value={`${(timeLimit / 1000).toFixed(1)} s`} />
            <Limit icon={MemoryStick} label="Memory" value={`${Math.round(memoryLimit / 1024)} MB`} />
            {interactive && <Badge tone="accent">Interactive</Badge>}
            {showDifficulty && (
                <span className="flex items-center gap-1.5">
                    <span className="text-muted">Difficulty</span>
                    <Badge tone={DIFFICULTY_TONE[showDifficulty] || 'neutral'} className="uppercase tracking-wider">
                        {showDifficulty}
                    </Badge>
                </span>
            )}
            {showRating ? <Limit icon={Gauge} label="Rating" value={String(showRating)} /> : null}
            {showTags.length > 0 && (
                <span className="flex flex-wrap items-center gap-1.5">
                    <span className="text-muted">Tags</span>
                    {showTags.map(tag => (
                        <Badge key={tag} tone="neutral">{tag}</Badge>
                    ))}
                </span>
            )}
        </div>
    )
}
