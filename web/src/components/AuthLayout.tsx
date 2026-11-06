import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { Code2 } from 'lucide-react'
import { Card } from './ui/Card'

/**
 * Shared shell for every unauthenticated screen (login, register, password
 * reset, 2FA). One centred card, one brand mark, one place to change it —
 * previously each page repeated `max-w-sm mx-auto mt-20` with slightly
 * different spacing and no surface at all.
 */
export function AuthLayout({
    title,
    subtitle,
    children,
    footer,
}: {
    title: ReactNode
    subtitle?: ReactNode
    children: ReactNode
    footer?: ReactNode
}) {
    return (
        <div className="mx-auto flex w-full max-w-[26rem] flex-1 flex-col justify-center py-6 sm:py-12">
            <div className="mb-6 text-center">
                <Link to="/" className="inline-flex items-center gap-2 text-sm font-semibold tracking-tight text-ink">
                    <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-brand text-on-brand">
                        <Code2 className="h-4 w-4" />
                    </span>
                    AIOJ
                </Link>
                <h1 className="mt-5 text-2xl font-semibold tracking-tight text-ink">{title}</h1>
                {subtitle && <p className="mt-1.5 text-sm text-muted">{subtitle}</p>}
            </div>
            <Card className="p-6">{children}</Card>
            {footer && <div className="mt-5 text-center text-sm text-muted">{footer}</div>}
        </div>
    )
}

export function AuthAlert({ children }: { children: ReactNode }) {
    return (
        <div
            role="alert"
            className="mb-4 rounded-lg border border-bad/30 bg-bad/10 px-3 py-2 text-sm leading-relaxed text-bad"
        >
            {children}
        </div>
    )
}

export function AuthNotice({ children }: { children: ReactNode }) {
    return (
        <div className="mb-4 rounded-lg border border-ok/30 bg-ok/10 px-3 py-2 text-sm leading-relaxed text-ok">
            {children}
        </div>
    )
}

export function AuthDivider({ label = 'or' }: { label?: string }) {
    return (
        <div className="my-5 flex items-center gap-3">
            <span className="h-px flex-1 bg-hairline" />
            <span className="text-[11px] font-medium uppercase tracking-wider text-muted-soft">{label}</span>
            <span className="h-px flex-1 bg-hairline" />
        </div>
    )
}
