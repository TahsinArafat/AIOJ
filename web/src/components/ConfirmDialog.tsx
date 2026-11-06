import { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { AlertTriangle } from 'lucide-react'

// ── Types ────────────────────────────────────────────────────────────────────

export interface ConfirmOptions {
    /** Heading text. Defaults to a generic confirmation heading. */
    title?: string
    /** Body copy. Strings are rendered as-is; nodes are allowed. */
    message: string
    /** Label for the affirmative button. */
    confirmLabel?: string
    /** Label for the cancel button. */
    cancelLabel?: string
    /** `danger` renders the affirmative button in red. Use for destructive actions. */
    variant?: 'default' | 'danger'
}

type ConfirmFn = (options: ConfirmOptions) => Promise<boolean>

// ── Context ──────────────────────────────────────────────────────────────────

const ConfirmContext = createContext<ConfirmFn | null>(null)

/** Returns a promise-based confirm. Resolves `true` on confirm, `false` on cancel. */
export function useConfirm(): ConfirmFn { // eslint-disable-line react-refresh/only-export-components -- hook stays co-located; moving it would churn imports across session-owned files
    const ctx = useContext(ConfirmContext)
    if (!ctx) throw new Error('useConfirm must be used within a ConfirmProvider')
    return ctx
}

interface PendingState {
    options: ConfirmOptions
    resolve: (result: boolean) => void
}

// ── Provider ─────────────────────────────────────────────────────────────────

export function ConfirmProvider({ children }: { children: React.ReactNode }) {
    const [pending, setPending] = useState<PendingState | null>(null)
    const confirmButtonRef = useRef<HTMLButtonElement>(null)
    const restoreFocusRef = useRef<HTMLElement | null>(null)

    const confirm = useCallback<ConfirmFn>((options) => {
        // A second confirm while one is open resolves the first as cancelled.
        setPending((prev) => {
            prev?.resolve(false)
            return { options, resolve: () => {} }
        })
        return new Promise<boolean>((resolve) => {
            setPending({ options, resolve })
        })
    }, [])

    const settle = useCallback((result: boolean) => {
        setPending((prev) => {
            prev?.resolve(result)
            return null
        })
    }, [])

    // Move focus into the dialog when it opens, and back to the trigger on close.
    useEffect(() => {
        if (!pending) return
        restoreFocusRef.current = document.activeElement as HTMLElement | null
        confirmButtonRef.current?.focus()
        return () => restoreFocusRef.current?.focus?.()
    }, [pending])

    // Escape cancels. Bound on document so it works regardless of focus position.
    useEffect(() => {
        if (!pending) return
        const onKeyDown = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                e.preventDefault()
                settle(false)
            }
        }
        document.addEventListener('keydown', onKeyDown)
        return () => document.removeEventListener('keydown', onKeyDown)
    }, [pending, settle])

    const { title = 'Are you sure?', message, confirmLabel = 'Confirm', cancelLabel = 'Cancel', variant = 'default' } = pending?.options ?? {}

    return (
        <ConfirmContext.Provider value={confirm}>
            {children}
            {pending &&
                createPortal(
                    <div
                        className="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-black/50"
                        onClick={e => { if (e.target === e.currentTarget) settle(false) }}
                    >
                        <div
                            role="dialog"
                            aria-modal="true"
                            aria-labelledby="confirm-dialog-title"
                            aria-describedby="confirm-dialog-message"
                            className="relative w-full max-w-md rounded-xl border border-hairline bg-surface p-6 shadow-xl"
                        >
                            <div className="flex gap-3">
                                {variant === 'danger' && (
                                    <div className="shrink-0 w-10 h-10 rounded-full bg-bad/12 flex items-center justify-center">
                                        <AlertTriangle className="w-5 h-5 text-bad" />
                                    </div>
                                )}
                                <div className="min-w-0">
                                    <h3 id="confirm-dialog-title" className="text-lg font-semibold text-ink">
                                        {title}
                                    </h3>
                                    <p id="confirm-dialog-message" className="mt-2 text-sm text-muted break-words">
                                        {message}
                                    </p>
                                </div>
                            </div>
                            <div className="mt-6 flex justify-end gap-2">
                                <button
                                    type="button"
                                    onClick={() => settle(false)}
                                    className="px-4 py-2 text-sm font-medium text-ink-soft border border-hairline-strong rounded hover:bg-surface-2 transition-colors"
                                >
                                    {cancelLabel}
                                </button>
                                <button
                                    type="button"
                                    ref={confirmButtonRef}
                                    onClick={() => settle(true)}
                                    // A destructive action needs a filled red in both themes.
                                    // `bg-bad` alone would not work: in dark mode it is the light
                                    // red used for text, so white-on-it is ~1.9:1.
                                    className={`px-4 py-2 text-sm font-medium rounded transition-colors ${variant === 'danger'
                                        ? 'bg-danger-fill text-danger-fill-ink hover:bg-danger-fill-hover'
                                        : 'bg-brand text-on-brand hover:bg-brand-hover'
                                        }`}
                                >
                                    {confirmLabel}
                                </button>
                            </div>
                        </div>
                    </div>,
                    document.body
                )}
        </ConfirmContext.Provider>
    )
}
