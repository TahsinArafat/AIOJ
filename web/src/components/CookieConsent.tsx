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
            className="fixed bottom-4 left-4 right-4 max-w-[calc(100vw-2rem)] md:left-8 md:right-auto md:max-w-md bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg p-4 z-50"
        >
            <div className="flex items-start gap-3">
                <div className="flex-1 text-sm text-gray-700 dark:text-gray-200">
                    AIOJ uses essential cookies for authentication and analytics cookies
                    to improve the service. See our{' '}
                    <a href="/legal/privacy" className="underline">
                        Privacy Policy
                    </a>
                    .
                </div>
                <button
                    type="button"
                    aria-label="Close"
                    onClick={() => decide('declined')}
                    className="text-gray-500 hover:text-gray-900 dark:hover:text-gray-100"
                >
                    <X className="w-4 h-4" />
                </button>
            </div>
            <div className="mt-3 flex gap-2">
                <button
                    type="button"
                    onClick={() => decide('accepted')}
                    className="px-3 py-1.5 rounded bg-blue-600 hover:bg-blue-700 text-white text-sm"
                >
                    Accept
                </button>
                <button
                    type="button"
                    onClick={() => decide('declined')}
                    className="px-3 py-1.5 rounded border border-gray-300 dark:border-gray-600 text-sm hover:bg-gray-50 dark:hover:bg-gray-900"
                >
                    Decline
                </button>
            </div>
        </div>
    )
}
