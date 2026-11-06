import type { ReactNode } from 'react'

/**
 * Card surfaces.
 *
 * Every surface paints from semantic tokens (`bg-surface`, `border-hairline`)
 * rather than `bg-white dark:bg-gray-800`, so light and dark are correct by
 * construction and the dark canvas always stays darker than its cards.
 */
export function Card({ className = '', children }: { className?: string; children: ReactNode }) {
    return (
        <div className={`rounded-xl border border-hairline bg-surface shadow-[var(--shadow-card)] ${className}`}>
            {children}
        </div>
    )
}

export function CardHeader({
    title,
    description,
    action,
    className = '',
}: {
    title: ReactNode
    description?: ReactNode
    action?: ReactNode
    className?: string
}) {
    return (
        <div className={`flex items-start justify-between gap-4 border-b border-hairline px-5 py-3.5 ${className}`}>
            <div className="min-w-0">
                <h2 className="text-sm font-semibold tracking-tight text-ink">{title}</h2>
                {description && <p className="mt-0.5 text-xs text-muted">{description}</p>}
            </div>
            {action && <div className="shrink-0">{action}</div>}
        </div>
    )
}

export function CardBody({ className = '', children }: { className?: string; children: ReactNode }) {
    return <div className={`px-5 py-4 ${className}`}>{children}</div>
}
