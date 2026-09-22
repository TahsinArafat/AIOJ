import * as Sentry from '@sentry/react'

export function initSentry() {
    const dsn = import.meta.env.VITE_SENTRY_DSN
    if (!dsn) {
        // Quiet no-op when DSN is unset (dev default).
        return
    }
    Sentry.init({
        dsn,
        environment: import.meta.env.MODE,
        release: import.meta.env.VITE_AIOJ_RELEASE,
        tracesSampleRate: 0.1,
        integrations: [Sentry.browserTracingIntegration()],
    })
}

export { Sentry }
