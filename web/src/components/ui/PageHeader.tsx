import type { ReactNode } from 'react'

/** Page-level heading: eyebrow, title, one-line description, right-aligned actions. */
export function PageHeader({
    title,
    description,
    eyebrow,
    actions,
    className = '',
}: {
    title: ReactNode
    description?: ReactNode
    eyebrow?: ReactNode
    actions?: ReactNode
    className?: string
}) {
    return (
        <header className={`mb-6 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between ${className}`}>
            <div className="min-w-0">
                {eyebrow && (
                    <p className="mb-1 text-[11px] font-semibold uppercase tracking-[0.08em] text-muted">{eyebrow}</p>
                )}
                <h1 className="text-2xl font-semibold tracking-tight text-ink">{title}</h1>
                {description && <p className="mt-1 text-sm text-muted">{description}</p>}
            </div>
            {actions && <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div>}
        </header>
    )
}

export function Skeleton({ className = '' }: { className?: string }) {
    return <div className={`animate-pulse rounded-md bg-surface-3 ${className}`} aria-hidden="true" />
}
