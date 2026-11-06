import { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { api, setTokens } from '../lib/api'
import { AuthAlert, AuthDivider, AuthLayout } from '../components/AuthLayout'
import { Button } from '../components/ui/Button'
import { buttonClass } from '../components/ui/buttonStyles'
import { Field, Input } from '../components/ui/Field'

/* Brand marks are inline because lucide-react v1 dropped brand icons. */
function GitHubMark() {
    return (
        <svg viewBox="0 0 16 16" aria-hidden="true" className="h-4 w-4 fill-current">
            <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82a7.4 7.4 0 0 1 2-.27c.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A7.995 7.995 0 0 0 16 8c0-4.42-3.58-8-8-8Z" />
        </svg>
    )
}

function GoogleMark() {
    return (
        <svg viewBox="0 0 24 24" aria-hidden="true" className="h-4 w-4">
            <path fill="#4285F4" d="M23.5 12.27c0-.85-.08-1.67-.22-2.45H12v4.64h6.45a5.52 5.52 0 0 1-2.39 3.62v3.01h3.86c2.26-2.08 3.56-5.15 3.56-8.82Z" />
            <path fill="#34A853" d="M12 24c3.24 0 5.96-1.08 7.94-2.91l-3.86-3.01c-1.07.72-2.44 1.15-4.08 1.15-3.13 0-5.78-2.11-6.73-4.96H1.28v3.09A12 12 0 0 0 12 24Z" />
            <path fill="#FBBC05" d="M5.27 14.27a7.2 7.2 0 0 1 0-4.54V6.64H1.28a12 12 0 0 0 0 10.72l3.99-3.09Z" />
            <path fill="#EA4335" d="M12 4.77c1.76 0 3.35.61 4.6 1.8l3.42-3.42C17.95 1.19 15.24 0 12 0A12 12 0 0 0 1.28 6.64l3.99 3.09C6.22 6.88 8.87 4.77 12 4.77Z" />
        </svg>
    )
}

export default function Login() {
    const [form, setForm] = useState({ username: '', password: '' })
    const [err, setErr] = useState('')
    const [loading, setLoading] = useState(false)
    const [challenge, setChallenge] = useState('')
    const [code, setCode] = useState('')
    const [verifying, setVerifying] = useState(false)
    const nav = useNavigate()

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
            // profile endpoints until it is changed; sending it to "/" would
            // only produce 403s.
            nav(d.user?.must_change_password ? '/profile' : '/')
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
            nav(d.user?.must_change_password ? '/profile' : '/')
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
                footer={<Link to="/login" className="text-brand hover:underline">Back to sign in</Link>}
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
            title="Sign in"
            subtitle="Continue to your AIOJ workspace."
            footer={
                <>
                    Don&apos;t have an account?{' '}
                    <Link to="/register" className="font-medium text-brand hover:underline">Create one</Link>
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
                    {loading ? 'Signing in…' : 'Sign in'}
                </Button>
            </form>

            <AuthDivider />

            <div className="space-y-2">
                <a
                    href="/api/auth/oauth/github/start"
                    className={buttonClass({ variant: 'secondary', size: 'md', className: 'w-full' })}
                >
                    <GitHubMark />
                    Continue with GitHub
                </a>
                <a
                    href="/api/auth/oauth/google/start"
                    className={buttonClass({ variant: 'secondary', size: 'md', className: 'w-full' })}
                >
                    <GoogleMark />
                    Continue with Google
                </a>
            </div>
        </AuthLayout>
    )
}
