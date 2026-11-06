import type { InputHTMLAttributes, ReactNode, SelectHTMLAttributes } from 'react'

const CONTROL =
    'w-full rounded-lg border border-hairline-strong bg-surface px-3 text-sm text-ink ' +
    'placeholder:text-muted-soft transition-colors ' +
    'hover:border-muted-soft focus:border-brand focus:outline-none ' +
    'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--ring)]'

export function Input({ className = '', ...rest }: InputHTMLAttributes<HTMLInputElement>) {
    return <input className={`${CONTROL} h-9 ${className}`} {...rest} />
}

export function Select({ className = '', children, ...rest }: SelectHTMLAttributes<HTMLSelectElement>) {
    return (
        <select className={`${CONTROL} h-9 cursor-pointer pr-8 ${className}`} {...rest}>
            {children}
        </select>
    )
}

/**
 * Label + control + help/error slot.
 *
 * The error is rendered next to the control it belongs to instead of only at
 * the top of the form, so a long form does not make people scroll back up to
 * find out which field failed.
 */
export function Field({
    label,
    htmlFor,
    hint,
    error,
    children,
    className = '',
}: {
    label: ReactNode
    htmlFor?: string
    hint?: ReactNode
    error?: ReactNode
    children: ReactNode
    className?: string
}) {
    return (
        <div className={className}>
            <label htmlFor={htmlFor} className="mb-1.5 block text-xs font-medium text-ink-soft">
                {label}
            </label>
            {children}
            {error ? (
                <p className="mt-1.5 text-xs text-bad">{error}</p>
            ) : hint ? (
                <p className="mt-1.5 text-xs text-muted">{hint}</p>
            ) : null}
        </div>
    )
}
