export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger'
export type ButtonSize = 'sm' | 'md' | 'lg'

const BASE =
    'inline-flex items-center justify-center gap-2 rounded-lg font-medium whitespace-nowrap ' +
    'transition-colors duration-150 select-none disabled:cursor-not-allowed disabled:opacity-50 ' +
    'focus-visible:outline-2 focus-visible:outline-offset-2'

const SIZES: Record<ButtonSize, string> = {
    sm: 'h-8 px-3 text-xs',
    md: 'h-9 px-4 text-sm',
    lg: 'h-11 px-5 text-sm',
}

const VARIANTS: Record<ButtonVariant, string> = {
    primary: 'bg-brand text-on-brand hover:bg-brand-hover shadow-sm',
    secondary: 'border border-hairline-strong bg-surface text-ink hover:bg-surface-2',
    ghost: 'text-muted hover:bg-surface-2 hover:text-ink',
    danger: 'border border-danger-fill/40 bg-bad/10 text-bad hover:bg-bad/15',
}

/**
 * Shared button styling.
 *
 * Lives in its own module (not next to the `Button` component) so links that
 * must *look* like buttons — `<Link>` and the OAuth `<a>` redirects — can
 * share the exact same surface without copy-pasting a class string that drifts
 * from the real button.
 */
export function buttonClass(opts: {
    variant?: ButtonVariant
    size?: ButtonSize
    className?: string
} = {}): string {
    const { variant = 'primary', size = 'md', className = '' } = opts
    return [BASE, SIZES[size], VARIANTS[variant], className].filter(Boolean).join(' ')
}
