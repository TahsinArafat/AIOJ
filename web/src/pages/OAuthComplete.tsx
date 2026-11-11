import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api, setTokens } from '../lib/api'
import { nextParam } from '../lib/authRedirect'
import { AuthAlert, AuthLayout } from '../components/AuthLayout'
import { Button } from '../components/ui/Button'
import { Field, Input } from '../components/ui/Field'

/**
 * Landing page for OAuth callbacks: everything arrives in the URL fragment,
 * never in a query string, so the identity and the tokens stay out of proxy
 * and referrer logs.
 *
 * A returning user gets `access_token`/`refresh_token` and is signed straight
 * in. A first-time user gets a `signup_token` instead, because the backend does
 * not create the account until they have picked the username everyone else
 * will see (see OAuthCallbackHandler.CompleteSignup).
 */
const PROVIDER_LABELS: Record<string, string> = { google: 'Google', github: 'GitHub' }

type Pending = { token: string; suggested: string; provider: string }

/** A verdict only counts for the handle it was fetched for. */
type Verdict = { handle: string; free: boolean; message: string }

function fragmentParams(): URLSearchParams {
    return new URLSearchParams(window.location.hash.replace(/^#/, ''))
}

function readPending(): Pending | null {
    const params = fragmentParams()
    const ticket = params.get('signup_token')
    if (!ticket) return null
    return {
        token: ticket,
        suggested: params.get('username') ?? '',
        provider: params.get('provider') ?? '',
    }
}

/** The login page, keeping the destination so "not you?" does not lose it. */
function backToLogin(next: string): string {
    return next ? `/login?${new URLSearchParams({ next }).toString()}` : '/login'
}

export default function OAuthComplete() {
    const nav = useNavigate()
    // The fragment is this page's entire input and cannot change while it is
    // mounted, so it is read once during the first render rather than in an
    // effect (which would cost a second render and, under StrictMode, run
    // twice).
    const [pending] = useState<Pending | null>(readPending)
    // Where the user was trying to get to; carried through the signed OAuth
    // state and handed to the SPA in the fragment.
    const [next] = useState<string>(() => nextParam(fragmentParams()) ?? '')
    const [username, setUsername] = useState(() => pending?.suggested ?? '')
    const [verdict, setVerdict] = useState<Verdict | null>(null)
    const [err, setErr] = useState('')
    const [saving, setSaving] = useState(false)
    const handled = useRef(false)

    useEffect(() => {
        if (handled.current) return
        handled.current = true
        const params = fragmentParams()
        const access = params.get('access_token')
        const refresh = params.get('refresh_token')
        if (access && refresh) {
            setTokens(access, refresh)
            window.history.replaceState(null, '', window.location.pathname)
            nav(next || '/', { replace: true })
            return
        }
        // With a sign-up ticket the page stays put and shows the form.
        if (!pending) nav(backToLogin(next), { replace: true })
    }, [nav, next, pending])

    const ticket = pending?.token

    // Live availability, debounced: the same answer the submit will get, told
    // before the form is sent. The request carries the sign-up ticket, so it
    // is not a free username-enumeration endpoint for anyone else.
    useEffect(() => {
        if (!ticket) return
        const name = username.trim()
        if (!name) return
        let cancelled = false
        const timer = setTimeout(() => {
            api.auth
                .oauthUsernameAvailable(ticket, name)
                .then(res => {
                    if (cancelled) return
                    setVerdict({
                        handle: name,
                        free: res.available,
                        message: res.available
                            ? `${res.username ?? name} is available.`
                            : res.error || 'That username is taken.',
                    })
                })
                // A failed hint must not block the form: the submit re-checks.
                .catch(() => {
                    if (!cancelled) setVerdict(null)
                })
        }, 400)
        return () => {
            cancelled = true
            clearTimeout(timer)
        }
    }, [ticket, username])

    const submit = async (e: React.FormEvent) => {
        e.preventDefault()
        if (!pending) return
        setErr('')
        setSaving(true)
        try {
            const d = await api.auth.completeOAuthSignup({
                signup_token: pending.token,
                username: username.trim(),
            })
            setTokens(d.access_token, d.refresh_token)
            window.history.replaceState(null, '', window.location.pathname)
            nav(next || '/', { replace: true })
        } catch (e: unknown) {
            setErr(e instanceof Error ? e.message : 'Could not create your account')
        } finally {
            setSaving(false)
        }
    }

    if (!pending) {
        return (
            <div className="mx-auto mt-20 max-w-sm text-center text-sm text-muted">
                Completing sign-in…
            </div>
        )
    }

    // Ignore a verdict that arrived for an earlier keystroke.
    const current = verdict && verdict.handle === username.trim() ? verdict : null
    const provider = PROVIDER_LABELS[pending.provider] ?? 'your provider'

    return (
        <AuthLayout
            title="Choose your username"
            subtitle="One last step before your account is created."
            footer={
                <>
                    Not you?{' '}
                    <Link to={backToLogin(next)} className="font-medium text-brand hover:underline">
                        Back to sign in
                    </Link>
                </>
            }
        >
            {err && <AuthAlert>{err}</AuthAlert>}
            <p className="mb-4 text-sm leading-relaxed text-muted">
                You signed in with {provider}. Pick the username that will appear on your profile,
                your submissions and the leaderboard.
            </p>
            <form onSubmit={submit} className="space-y-4">
                <Field
                    label="Username"
                    htmlFor="username"
                    hint="3–32 characters: letters, numbers, dots, dashes and underscores."
                >
                    <Input
                        id="username"
                        name="username"
                        value={username}
                        onChange={e => setUsername(e.target.value)}
                        autoComplete="username"
                        placeholder={pending.suggested || 'your-username'}
                        autoFocus
                        required
                    />
                </Field>
                {current && (
                    <p role="status" className={`text-xs ${current.free ? 'text-ok' : 'text-bad'}`}>
                        {current.message}
                    </p>
                )}
                <Button
                    type="submit"
                    size="lg"
                    className="w-full"
                    disabled={saving || current?.free === false}
                >
                    {saving ? 'Creating account…' : 'Create account'}
                </Button>
            </form>
        </AuthLayout>
    )
}
