import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { api, setTokens } from '../../lib/api'

/** Standalone 2FA challenge page when login redirects here with ?challenge_id=. */
export default function TwoFactorVerify() {
    const { t } = useTranslation()
    const [params] = useSearchParams()
    const challengeId = params.get('challenge_id') || ''
    const [code, setCode] = useState('')
    const [error, setError] = useState<string | null>(null)
    const [busy, setBusy] = useState(false)
    const navigate = useNavigate()

    const submit = async (e: React.FormEvent) => {
        e.preventDefault()
        if (!challengeId) {
            setError('Missing challenge. Please sign in again.')
            return
        }
        setBusy(true)
        setError(null)
        try {
            const d = await api.twoFactor.verify({ challenge_id: challengeId, code })
            setTokens(d.access_token, d.refresh_token)
            navigate('/')
        } catch (e) {
            setError((e instanceof Error ? e.message : '') || 'Invalid code')
        } finally {
            setBusy(false)
        }
    }

    return (
        <div className="max-w-sm mx-auto mt-20 space-y-3">
            <h1 className="text-xl font-semibold">{t('auth.twoFactor')}</h1>
            <form onSubmit={submit} className="space-y-3">
                <input
                    value={code}
                    onChange={e => setCode(e.target.value)}
                    placeholder="6-digit code or backup code"
                    autoComplete="one-time-code"
                    className="border border-gray-300 dark:border-gray-600 rounded px-2 py-1 w-full text-sm dark:bg-gray-900"
                    required
                />
                {error && <p className="text-red-600 text-sm">{error}</p>}
                <button
                    type="submit"
                    disabled={busy}
                    className="px-3 py-2 rounded bg-blue-600 text-white text-sm hover:bg-blue-700 disabled:opacity-50"
                >
                    {busy ? 'Verifying…' : 'Verify'}
                </button>
            </form>
        </div>
    )
}
