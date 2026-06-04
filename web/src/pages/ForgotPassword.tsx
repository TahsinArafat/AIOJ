import { useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'

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
        } catch (e: any) {
            setErr(e.message || 'Something went wrong')
        } finally {
            setLoading(false)
        }
    }

    if (sent) {
        return (
            <div className="max-w-sm mx-auto mt-20 text-center">
                <h1 className="text-2xl font-bold mb-4">Check Your Email</h1>
                <p className="text-gray-600 dark:text-gray-400 mb-6">{msg}</p>
                <Link to="/login" className="text-blue-600 dark:text-blue-400 hover:underline">Back to Login</Link>
            </div>
        )
    }

    return (
        <div className="max-w-sm mx-auto mt-20">
            <h1 className="text-2xl font-bold mb-6">Forgot Password</h1>
            {err && <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 px-4 py-2 rounded mb-4 text-sm">{err}</div>}
            <form onSubmit={handle} className="space-y-4">
                <div>
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Email</label>
                    <input
                        type="email"
                        value={email}
                        onChange={e => setEmail(e.target.value)}
                        className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
                        required
                    />
                </div>
                <button
                    type="submit"
                    disabled={loading}
                    className="w-full bg-blue-600 text-white py-2 rounded-md text-sm font-medium hover:bg-blue-700 disabled:opacity-50 transition-colors"
                >
                    {loading ? 'Sending...' : 'Send Reset Link'}
                </button>
            </form>
            <p className="text-center text-sm text-gray-500 dark:text-gray-400 mt-4">
                Remember your password? <Link to="/login" className="text-blue-600 dark:text-blue-400 hover:underline">Login</Link>
            </p>
        </div>
    )
}
