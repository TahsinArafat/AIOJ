import type { ReactNode } from 'react'

/**
 * Shared empty-state block: a short headline plus optional actionable copy
 * explaining what will show up here or what the visitor can do next.
 *
 * The icon is optional — table cells and compact panels use it text-only,
 * while feature tabs pass a lucide glyph inside the `text-4xl` slot.
 */
export function EmptyState({ icon, text, description }: {
    icon?: ReactNode
    text: string
    description?: string
}) {
    return (
        <div className="mx-auto flex max-w-md flex-col items-center px-4 py-10 text-center">
            {icon && <div className="mb-2 text-3xl text-muted-soft">{icon}</div>}
            <p className="text-sm font-medium text-ink">{text}</p>
            {description && <p className="mt-1 text-sm leading-relaxed text-muted">{description}</p>}
        </div>
    )
}

export default EmptyState
