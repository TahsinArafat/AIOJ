import { useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'
import { AuthAlert, AuthLayout } from '../components/AuthLayout'
import { Button } from '../components/ui/Button'
import { Field, Input } from '../components/ui/Field'

export default function ForgotPassword() {
    const [email, setEmail] = useState('')
    const [msg, setMsg] = useState('')
    const [err, setErr] = useState('')
    const [loading, setLoading] = useState(false)
    const [sent, setSent] = useState(false)

    const handle = async (e: React.FormEvent) => {
        e.preventDefault()
        setErr('')
        setMsg('')
        setLoading(true)
        try {
            const d = await api.auth.forgotPassword({ email })
            setMsg(d.message)
            setSent(true)
        } catch (e) {
            setErr((e instanceof Error ? e.message : '') || 'Something went wrong')
        } finally {
            setLoading(false)
        }
    }

    if (sent) {
        return (
            <AuthLayout
                title="Check your email"
                subtitle="If that address belongs to an account, a reset link is on its way."
                footer={<Link to="/login" className="font-medium text-brand hover:underline">Back to sign in</Link>}
            >
                <p className="text-sm leading-relaxed text-muted">{msg}</p>
            </AuthLayout>
        )
    }

    return (
        <AuthLayout
            title="Reset your password"
            subtitle="We'll email you a link to choose a new password."
            footer={<Link to="/login" className="font-medium text-brand hover:underline">Back to sign in</Link>}
        >
            {err && <AuthAlert>{err}</AuthAlert>}
            <form onSubmit={handle} className="space-y-4">
                <Field label="Email" htmlFor="email">
                    <Input
                        id="email"
                        type="email"
                        value={email}
                        onChange={e => setEmail(e.target.value)}
                        autoComplete="email"
                        required
                    />
                </Field>
                <Button type="submit" size="lg" className="w-full" disabled={loading}>
                    {loading ? 'Sending…' : 'Send reset link'}
                </Button>
            </form>
        </AuthLayout>
    )
}
