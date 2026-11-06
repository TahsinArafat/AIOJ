import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { api } from '../../lib/api'

export default function TwoFactorSetup() {
    const { t } = useTranslation()
    const [secret, setSecret] = useState<string | null>(null)
    const [uri, setUri] = useState<string | null>(null)
    const [code, setCode] = useState('')
    const [backupCodes, setBackupCodes] = useState<string[]>([])
    const [error, setError] = useState('')
    const [busy, setBusy] = useState(false)

    const begin = async () => {
        setError('')
        setBusy(true)
        try {
            const r = await api.twoFactor.begin()
            setSecret(r.secret)
            setUri(r.uri)
        } catch (e) {
            setError((e instanceof Error ? e.message : '') || 'Failed to start 2FA setup')
        } finally {
            setBusy(false)
        }
    }

    const enable = async () => {
        setError('')
        setBusy(true)
        try {
            const r = await api.twoFactor.enable(code)
            setBackupCodes(r.backup_codes)
        } catch (e) {
            setError((e instanceof Error ? e.message : '') || 'Invalid code')
        } finally {
            setBusy(false)
        }
    }

    return (
        <div className="max-w-md mx-auto p-6 space-y-4">
            <h1 className="text-xl font-semibold">{t('auth.twoFactorSetupTitle')}</h1>
            <p className="text-sm text-muted">{t('auth.twoFactorScanQR')}</p>
            {error && (
                <div className="bg-bad/10 border border-bad/30 text-bad px-4 py-2 rounded text-sm">
                    {error}
                </div>
            )}
            {backupCodes.length > 0 ? (
                <div className="bg-yellow-50 dark:bg-yellow-950/20 border border-warn/30 dark:border-yellow-900 p-3 rounded space-y-2">
                    <p className="font-semibold text-sm text-yellow-900 dark:text-yellow-100">
                        Save these backup codes:
                    </p>
                    <ul className="list-disc pl-5 text-sm font-mono break-all">
                        {backupCodes.map(c => (
                            <li key={c}>{c}</li>
                        ))}
                    </ul>
                    <p className="text-xs text-yellow-800 dark:text-yellow-200">
                        Each code can be used once if you lose your authenticator device.
                    </p>
                    <Link
                        to="/profile"
                        className="inline-block px-3 py-1.5 rounded bg-brand text-on-brand text-sm hover:bg-brand-hover"
                    >
                        Done
                    </Link>
                </div>
            ) : !secret ? (
                <button
                    type="button"
                    onClick={begin}
                    disabled={busy}
                    className="px-3 py-2 rounded bg-brand text-on-brand text-sm hover:bg-brand-hover disabled:opacity-50"
                >
                    {busy ? '…' : 'Begin 2FA setup'}
                </button>
            ) : (
                <div className="space-y-3">
                    <p className="text-sm">Scan this URI in your authenticator app:</p>
                    <pre className="p-2 bg-surface-3 rounded text-xs break-all border border-hairline">
                        {uri}
                    </pre>
                    <p className="text-sm">
                        Or enter this secret manually: <code className="font-mono">{secret}</code>
                    </p>
                    <input
                        value={code}
                        onChange={e => setCode(e.target.value)}
                        placeholder="6-digit code"
                        inputMode="numeric"
                        autoComplete="one-time-code"
                        className="border border-hairline-strong rounded px-2 py-1 w-full text-sm dark:bg-gray-900"
                    />
                    <button
                        type="button"
                        onClick={enable}
                        disabled={busy || code.length < 6}
                        className="px-3 py-2 rounded bg-brand text-on-brand text-sm hover:bg-brand-hover disabled:opacity-50"
                    >
                        {busy ? '…' : 'Enable 2FA'}
                    </button>
                </div>
            )}
        </div>
    )
}
