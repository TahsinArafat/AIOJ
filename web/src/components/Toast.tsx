import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { CheckCircle2, XCircle, Info, X } from 'lucide-react'

// ── Types ────────────────────────────────────────────────────────────────────

type ToastVariant = 'success' | 'error' | 'info'

interface Toast {
    id: number
    variant: ToastVariant
    message: string
}

interface ToastApi {
    success: (message: string) => void
    error: (message: string) => void
    info: (message: string) => void
}

// ── Context ──────────────────────────────────────────────────────────────────

const ToastContext = createContext<ToastApi | null>(null)

/** Shows transient status messages. Errors surface via `role="alert"`. */
export function useToast(): ToastApi { // eslint-disable-line react-refresh/only-export-components -- hook stays co-located; moving it would churn imports across session-owned files
    const ctx = useContext(ToastContext)
    if (!ctx) throw new Error('useToast must be used within a ToastProvider')
    return ctx
}

const DURATION = 5000

// ── Provider ─────────────────────────────────────────────────────────────────

export function ToastProvider({ children }: { children: React.ReactNode }) {
    const [toasts, setToasts] = useState<Toast[]>([])
    const nextId = useRef(0)
    const timers = useRef(new Map<number, ReturnType<typeof setTimeout>>())

    const dismiss = useCallback((id: number) => {
        setToasts(prev => prev.filter(t => t.id !== id))
        const timer = timers.current.get(id)
        if (timer) {
            clearTimeout(timer)
            timers.current.delete(id)
        }
    }, [])

    const push = useCallback((variant: ToastVariant, message: string) => {
        const id = ++nextId.current
        setToasts(prev => [...prev, { id, variant, message }])
        timers.current.set(id, setTimeout(() => dismiss(id), DURATION))
    }, [dismiss])

    // Clear every pending timer on unmount so no state update lands after teardown.
    useEffect(() => {
        const pending = timers.current
        return () => {
            pending.forEach(t => clearTimeout(t))
            pending.clear()
        }
    }, [])

    const api = useMemo<ToastApi>(() => ({
        success: (m: string) => push('success', m),
        error: (m: string) => push('error', m),
        info: (m: string) => push('info', m),
    }), [push])

    const icons = {
        success: CheckCircle2,
        error: XCircle,
        info: Info,
    }
    const styles = {
        success: 'text-green-600 dark:text-green-400',
        error: 'text-red-600 dark:text-red-400',
        info: 'text-blue-600 dark:text-blue-400',
    }

    return (
        <ToastContext.Provider value={api}>
            {children}
            {toasts.length > 0 &&
                createPortal(
                    <div className="fixed top-4 right-4 z-[110] flex flex-col gap-2 w-[calc(100vw-2rem)] max-w-sm pointer-events-none" aria-live="polite">
                        {toasts.map(t => {
                            const Icon = icons[t.variant]
                            return (
                                <div
                                    key={t.id}
                                    role={t.variant === 'error' ? 'alert' : 'status'}
                                    className="pointer-events-auto flex items-start gap-3 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg p-3"
                                >
                                    <Icon className={`w-5 h-5 shrink-0 mt-0.5 ${styles[t.variant]}`} />
                                    <span className="text-sm text-gray-800 dark:text-gray-200 break-words flex-1">{t.message}</span>
                                    <button
                                        type="button"
                                        onClick={() => dismiss(t.id)}
                                        aria-label="Dismiss notification"
                                        className="shrink-0 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 transition-colors"
                                    >
                                        <X className="w-4 h-4" />
                                    </button>
                                </div>
                            )
                        })}
                    </div>,
                    document.body
                )}
        </ToastContext.Provider>
    )
}
