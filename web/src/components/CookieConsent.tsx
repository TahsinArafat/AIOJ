import { useState } from 'react'
import { X } from 'lucide-react'

const KEY = 'cookie-consent'

export default function CookieConsent() {
    const [show, setShow] = useState(() => typeof window !== 'undefined' && !localStorage.getItem(KEY))

    if (!show) return null

    const decide = (choice: 'accepted' | 'declined') => {
        localStorage.setItem(KEY, choice)
        setShow(false)
    }

    return (
        <div
            role="dialog"
            aria-label="Cookie consent"
            className="fixed bottom-4 left-4 right-4 max-w-[calc(100vw-2rem)] md:left-8 md:right-auto md:max-w-md bg-surface border border-hairline rounded-lg shadow-lg p-4 z-50"
        >
            <div className="flex items-start gap-3">
                <div className="flex-1 text-sm text-ink-soft">
                    AIOJ uses the cookies and browser storage needed to keep you signed in
                    and protect your session. There are no advertising or cross-site tracking
                    cookies. Optional analytics runs only after you accept. See our{' '}
                    <a href="/legal/privacy" className="underline">
                        Privacy Policy
                    </a>
                    .
                </div>
                <button
                    type="button"
                    aria-label="Close"
                    onClick={() => decide('declined')}
                    className="text-muted hover:text-ink"
                >
                    <X className="w-4 h-4" />
                </button>
            </div>
            <div className="mt-3 flex gap-2">
                <button
                    type="button"
                    onClick={() => decide('accepted')}
                    className="px-3 py-1.5 rounded bg-brand hover:bg-brand-hover text-on-brand text-sm"
                >
                    Accept
                </button>
                <button
                    type="button"
                    onClick={() => decide('declined')}
                    className="px-3 py-1.5 rounded border border-hairline-strong text-sm hover:bg-surface-2"
                >
                    Decline
                </button>
            </div>
        </div>
    )
}
