import { useState } from 'react'
import { Link } from 'react-router-dom'
import { api, setTokens } from '../lib/api'
import { AuthAlert, AuthDivider, AuthLayout } from '../components/AuthLayout'
import { Button } from '../components/ui/Button'
import { Field, Input } from '../components/ui/Field'
import { OAuthButtons } from '../components/OAuthButtons'
import { hasOAuthProvider, useAuthMethods } from '../lib/authMethods'

export default function Register() {
    const [form, setForm] = useState({ username: '', email: '', password: '' })
    const [err, setErr] = useState('')
    const [loading, setLoading] = useState(false)
    const [sent, setSent] = useState(false)
    const [resent, setResent] = useState(false)
    const { methods, loading: methodsLoading } = useAuthMethods()

    // null (still loading, or the request failed) counts as enabled: the
    // backend rejects a disabled registration with a clear message, and a
    // spinner here would be worse than a form that might refuse.
    const passwordOpen = methods?.password_registration !== false
    const oauthOpen = hasOAuthProvider(methods)

    const handle = async (e: React.FormEvent) => {
        e.preventDefault()
        setErr('')
        if (form.password.length < 12) { setErr('Password must be at least 12 characters'); return }
        setLoading(true)
        try {
            const d = await api.auth.register(form)
            setTokens(d.access_token, d.refresh_token)
            setSent(true)
        } catch (e: unknown) {
            setErr(e instanceof Error ? e.message : 'Registration failed')
        } finally {
            setLoading(false)
        }
    }

    const resend = async () => {
        setErr('')
        try {
            await api.auth.resendVerification()
            setResent(true)
        } catch {
            /* enumeration-safe: ignore errors quietly */
        }
    }

    if (sent) {
        return (
            <AuthLayout
                title="Check your email"
                subtitle={<>We sent a verification link to <strong className="font-medium text-ink">{form.email}</strong>.</>}
                footer={<Link to="/login" className="font-medium text-brand hover:underline">Continue to sign in</Link>}
            >
                <p className="text-sm leading-relaxed text-muted">
                    Verify your email address to submit solutions. You can start browsing problems right away.
                </p>
                {resent && (
                    <p role="status" className="mt-3 text-sm text-ok">Verification email resent.</p>
                )}
                <Button variant="secondary" className="mt-4 w-full" onClick={resend}>
                    Resend verification email
                </Button>
            </AuthLayout>
        )
    }

    const footer = (
        <>
            Already have an account?{' '}
            <Link to="/login" className="font-medium text-brand hover:underline">Login</Link>
        </>
    )

    // An admin can close password registration. When that happens the page must
    // explain why instead of showing a form the API will reject.
    if (!passwordOpen) {
        return (
            <AuthLayout
                title="Registration closed"
                subtitle="This instance does not accept new accounts with an email address and password."
                footer={footer}
            >
                {oauthOpen ? (
                    <>
                        <p className="mb-4 text-sm leading-relaxed text-muted">
                            You can still create an account with one of these providers.
                        </p>
                        <OAuthButtons methods={methods} />
                    </>
                ) : (
                    <p className="text-sm leading-relaxed text-muted">
                        New sign-ups are currently disabled. Please contact the site administrator if you
                        need an account, or <Link to="/login" className="font-medium text-brand hover:underline">sign in</Link> if
                        you already have one.
                    </p>
                )}
            </AuthLayout>
        )
    }

    return (
        <AuthLayout
            title="Register"
            subtitle="Free, self-hosted, and yours to run."
            footer={footer}
        >
            {err && <AuthAlert>{err}</AuthAlert>}
            <form onSubmit={handle} className="space-y-4">
                <Field label="Username" htmlFor="username">
                    <Input
                        id="username"
                        name="username"
                        value={form.username}
                        onChange={e => setForm(p => ({ ...p, username: e.target.value }))}
                        autoComplete="username"
                        required
                    />
                </Field>
                <Field label="Email" htmlFor="email">
                    <Input
                        id="email"
                        name="email"
                        type="email"
                        value={form.email}
                        onChange={e => setForm(p => ({ ...p, email: e.target.value }))}
                        autoComplete="email"
                        required
                    />
                </Field>
                <Field label="Password" htmlFor="password" hint="At least 12 characters.">
                    <Input
                        id="password"
                        name="password"
                        type="password"
                        value={form.password}
                        onChange={e => setForm(p => ({ ...p, password: e.target.value }))}
                        autoComplete="new-password"
                        minLength={12}
                        required
                    />
                </Field>
                <Button type="submit" size="lg" className="w-full" disabled={loading}>
                    {loading ? 'Creating account…' : 'Register'}
                </Button>
            </form>

            {!methodsLoading && oauthOpen && (
                <>
                    <AuthDivider />
                    <OAuthButtons methods={methods} />
                </>
            )}
        </AuthLayout>
    )
}
