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
        <div className="text-center py-10">
            {icon && <div className="text-4xl mb-2">{icon}</div>}
            <p className="text-gray-500 dark:text-gray-400 text-sm font-medium">{text}</p>
            {description && <p className="text-gray-400 dark:text-gray-500 text-sm mt-1 max-w-md mx-auto">{description}</p>}
        </div>
    )
}

export default EmptyState
