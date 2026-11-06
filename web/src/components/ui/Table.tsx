import type { ReactNode, ThHTMLAttributes, TdHTMLAttributes } from 'react'

/**
 * Dense data table shell.
 *
 * Tables are the main surface of an online judge (problems, standings,
 * submissions), so they get one consistent treatment: a card border, a sticky-
 * free header band, hairline row dividers, and horizontal scrolling confined
 * to the wrapper instead of leaking into the document.
 */
export function TableShell({ children, className = '' }: { children: ReactNode; className?: string }) {
    return (
        <div className={`overflow-hidden rounded-xl border border-hairline bg-surface shadow-[var(--shadow-card)] ${className}`}>
            <div className="overflow-x-auto">{children}</div>
        </div>
    )
}

export function Table({ children, className = '' }: { children: ReactNode; className?: string }) {
    return <table className={`w-full border-collapse text-sm ${className}`}>{children}</table>
}

export function THead({ children }: { children: ReactNode }) {
    return <thead className="bg-surface-2">{children}</thead>
}

export function Th({ children, className = '', ...rest }: ThHTMLAttributes<HTMLTableCellElement>) {
    return (
        <th
            scope="col"
            className={`border-b border-hairline px-4 py-2.5 text-left text-[11px] font-semibold uppercase tracking-wider text-muted ${className}`}
            {...rest}
        >
            {children}
        </th>
    )
}

export function Td({ children, className = '', ...rest }: TdHTMLAttributes<HTMLTableCellElement>) {
    return (
        <td className={`px-4 py-3 align-middle text-ink-soft ${className}`} {...rest}>
            {children}
        </td>
    )
}

export function Tr({ children, className = '' }: { children: ReactNode; className?: string }) {
    return (
        <tr className={`border-b border-hairline last:border-0 transition-colors hover:bg-surface-2 ${className}`}>
            {children}
        </tr>
    )
}
