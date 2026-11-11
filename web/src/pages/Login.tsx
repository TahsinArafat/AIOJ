import { useState } from 'react'
import { useNavigate, Link, useSearchParams } from 'react-router-dom'
import { api, setTokens } from '../lib/api'
import { nextPath } from '../lib/authRedirect'
import { AuthAlert, AuthDivider, AuthLayout } from '../components/AuthLayout'
import { Button } from '../components/ui/Button'
import { Field, Input } from '../components/ui/Field'
import { OAuthButtons } from '../components/OAuthButtons'
import { hasOAuthProvider, useAuthMethods } from '../lib/authMethods'

export default function Login() {
    const [form, setForm] = useState({ username: '', password: '' })
    const [err, setErr] = useState('')
    const [loading, setLoading] = useState(false)
    const [challenge, setChallenge] = useState('')
    const [code, setCode] = useState('')
    const [verifying, setVerifying] = useState(false)
    const nav = useNavigate()
    const [params] = useSearchParams()
    // Where to go once the session is established. Read from the URL rather
    // than component state so a reload of /login?next=… keeps the destination.
    const dest = nextPath(params)
    const { methods, loading: methodsLoading } = useAuthMethods()

    const handle = async (e: React.FormEvent) => {
        e.preventDefault()
        setErr('')
        setLoading(true)
        try {
            const d = await api.auth.login(form)
            if (d.requires_2fa && d.challenge_id) {
                setChallenge(d.challenge_id)
                return
            }
            if (!d.access_token || !d.refresh_token) {
                setErr('Login failed')
                return
            }
            setTokens(d.access_token, d.refresh_token)
            // The seeded admin (and any admin-issued password) is locked to the
            // profile endpoints until it is changed; sending it to the
            // destination would only produce 403s.
            nav(d.user?.must_change_password ? '/profile' : dest, { replace: true })
        } catch (e: unknown) {
            setErr(e instanceof Error ? e.message : 'Login failed')
        } finally {
            setLoading(false)
        }
    }

    const verify2fa = async (e: React.FormEvent) => {
        e.preventDefault()
        setErr('')
        setVerifying(true)
        try {
            const d = await api.auth.verify2FA(challenge, code)
            setTokens(d.access_token, d.refresh_token)
            nav(d.user?.must_change_password ? '/profile' : dest, { replace: true })
        } catch (e: unknown) {
            setErr(e instanceof Error ? e.message : 'Invalid code')
        } finally {
            setVerifying(false)
        }
    }

    if (challenge) {
        return (
            <AuthLayout
                title="Two-factor authentication"
                subtitle="Enter the 6-digit code from your authenticator app."
                footer={
                    <Link
                        to={params.toString() ? `${'/login'}?${params.toString()}` : '/login'}
                        className="text-brand hover:underline"
                    >
                        Back to sign in
                    </Link>
                }
            >
                {err && <AuthAlert>{err}</AuthAlert>}
                <form onSubmit={verify2fa} className="space-y-4">
                    <Field label="Authentication code" htmlFor="code">
                        <Input
                            id="code"
                            value={code}
                            onChange={e => setCode(e.target.value)}
                            inputMode="numeric"
                            autoComplete="one-time-code"
                            autoFocus
                            className="tnum h-11 text-center text-lg tracking-[0.4em]"
                            required
                        />
                    </Field>
                    <Button type="submit" size="lg" className="w-full" disabled={verifying}>
                        {verifying ? 'Verifying…' : 'Verify'}
                    </Button>
                </form>
            </AuthLayout>
        )
    }

    return (
        <AuthLayout
            title="Login"
            subtitle="Continue to your AIOJ workspace."
            footer={
                <>
                    Don&apos;t have an account?{' '}
                    <Link to="/register" className="font-medium text-brand hover:underline">Register</Link>
                </>
            }
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
                <Field
                    label="Password"
                    htmlFor="password"
                    hint={
                        <Link to="/forgot-password" className="text-brand hover:underline">
                            Forgot your password?
                        </Link>
                    }
                >
                    <Input
                        id="password"
                        name="password"
                        type="password"
                        value={form.password}
                        onChange={e => setForm(p => ({ ...p, password: e.target.value }))}
                        autoComplete="current-password"
                        required
                    />
                </Field>
                <Button type="submit" size="lg" className="w-full" disabled={loading}>
                    {loading ? 'Logging in…' : 'Login'}
                </Button>
            </form>

            {/* The divider and the provider buttons appear together, and only
                when this instance actually offers an OAuth provider. Held back
                until the answer arrives so a provider that is unavailable does
                not flash into view and then vanish. */}
            {!methodsLoading && hasOAuthProvider(methods) && (
                <>
                    <AuthDivider />
                    <OAuthButtons methods={methods} />
                </>
            )}
        </AuthLayout>
    )
}
