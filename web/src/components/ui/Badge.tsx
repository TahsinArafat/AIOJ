import type { ReactNode } from 'react'

export type BadgeTone = 'neutral' | 'brand' | 'accent' | 'ok' | 'warn' | 'bad'

const TONES: Record<BadgeTone, string> = {
    neutral: 'bg-surface-3 text-muted',
    brand: 'bg-brand-soft text-brand',
    // `accent` is a dark-safe violet: the old `bg-purple-100 text-purple-800`
    // pair had no dark variant, so it kept a pale lavender fill on the dark
    // canvas.
    accent: 'bg-violet-500/12 text-violet-700 dark:text-violet-300',
    ok: 'bg-ok/12 text-ok',
    warn: 'bg-warn/12 text-warn',
    bad: 'bg-bad/12 text-bad',
}

/**
 * Small status pill. `tone` carries the meaning; the palette resolves through
 * theme tokens so the same badge is legible on light and dark.
 */
export function Badge({
    children,
    tone = 'neutral',
    dot = false,
    className = '',
    label,
}: {
    children: ReactNode
    tone?: BadgeTone
    dot?: boolean
    className?: string
    /** Full text for an abbreviated badge, surfaced as a native tooltip. */
    label?: string
}) {
    return (
        <span
            title={label}
            className={`inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[11px] font-medium leading-5 whitespace-nowrap ${TONES[tone]} ${className}`}
        >
            {dot && <span className="h-1.5 w-1.5 rounded-full bg-current" aria-hidden="true" />}
            {children}
        </span>
    )
}
