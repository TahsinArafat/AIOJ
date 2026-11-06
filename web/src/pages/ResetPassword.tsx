import { useState } from 'react'
import { useSearchParams, Link } from 'react-router-dom'
import { api } from '../lib/api'
import { AuthAlert, AuthLayout } from '../components/AuthLayout'
import { Button } from '../components/ui/Button'
import { Field, Input } from '../components/ui/Field'

export default function ResetPassword() {
    const [searchParams] = useSearchParams()
    const token = searchParams.get('token') || ''

    const [password, setPassword] = useState('')
    const [confirm, setConfirm] = useState('')
    const [err, setErr] = useState('')
    const [loading, setLoading] = useState(false)
    const [done, setDone] = useState(false)

    const handle = async (e: React.FormEvent) => {
        e.preventDefault()
        setErr('')
        if (password !== confirm) {
            setErr('Passwords do not match')
            return
        }
        if (password.length < 6) {
            setErr('Password too short (min 6)')
            return
        }
        setLoading(true)
        try {
            await api.auth.resetPassword({ token, new_password: password })
            setDone(true)
        } catch (e) {
            setErr((e instanceof Error ? e.message : '') || 'Reset failed')
        } finally {
            setLoading(false)
        }
    }

    if (done) {
        return (
            <AuthLayout
                title="Password reset"
                subtitle="Your password has been updated."
                footer={<Link to="/login" className="font-medium text-brand hover:underline">Sign in with your new password</Link>}
            >
                <Button size="lg" className="w-full" onClick={() => { window.location.href = '/login' }}>
                    Go to sign in
                </Button>
            </AuthLayout>
        )
    }

    if (!token) {
        return (
            <AuthLayout
                title="Invalid link"
                subtitle="This reset link is missing its token."
                footer={<Link to="/forgot-password" className="font-medium text-brand hover:underline">Request a new reset link</Link>}
            >
                <p className="text-sm leading-relaxed text-muted">
                    Reset links expire after a short time and can only be used once. Request a fresh one to continue.
                </p>
            </AuthLayout>
        )
    }

    return (
        <AuthLayout
            title="Choose a new password"
            subtitle="Pick something you haven't used before."
        >
            {err && <AuthAlert>{err}</AuthAlert>}
            <form onSubmit={handle} className="space-y-4">
                <Field label="New password" htmlFor="password">
                    <Input
                        id="password"
                        type="password"
                        value={password}
                        onChange={e => setPassword(e.target.value)}
                        autoComplete="new-password"
                        required
                        minLength={6}
                    />
                </Field>
                <Field label="Confirm password" htmlFor="confirm">
                    <Input
                        id="confirm"
                        type="password"
                        value={confirm}
                        onChange={e => setConfirm(e.target.value)}
                        autoComplete="new-password"
                        required
                        minLength={6}
                    />
                </Field>
                <Button type="submit" size="lg" className="w-full" disabled={loading}>
                    {loading ? 'Resetting…' : 'Reset password'}
                </Button>
            </form>
        </AuthLayout>
    )
}
