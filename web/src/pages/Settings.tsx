import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { OAuthButtons } from '../components/OAuthButtons'
import { hasOAuthProvider, useAuthMethods } from '../lib/authMethods'

/**
 * Auth settings hub: security (2FA), connected OAuth accounts, and pointers
 * into Profile for password / danger-zone flows.
 */
export default function Settings() {
    const { t } = useTranslation()
    const { methods } = useAuthMethods()

    return (
        <div className="max-w-2xl mx-auto space-y-6">
            <h1 className="text-2xl font-bold">Settings</h1>

            <section className="bg-surface border border-hairline rounded-lg p-6">
                <h2 className="text-lg font-semibold mb-2">{t('auth.twoFactor')}</h2>
                <p className="text-sm text-muted mb-4">
                    {t('auth.twoFactorScanQR')}
                </p>
                <div className="flex flex-wrap gap-3">
                    <Link
                        to="/auth/2fa/setup"
                        className="px-3 py-2 rounded bg-brand text-on-brand text-sm hover:bg-brand-hover"
                    >
                        {t('auth.twoFactorSetupTitle')}
                    </Link>
                    <Link
                        to="/profile"
                        className="px-3 py-2 rounded border border-hairline-strong text-sm hover:bg-surface-2"
                    >
                        Change password
                    </Link>
                </div>
            </section>

            {hasOAuthProvider(methods) && (
                <section className="bg-surface border border-hairline rounded-lg p-6">
                    <h2 className="text-lg font-semibold mb-4">Connected accounts</h2>
                    <OAuthButtons methods={methods} />
                </section>
            )}

            <section className="bg-surface border border-bad/30 dark:border-red-900 rounded-lg p-6">
                <h2 className="text-lg font-semibold text-bad mb-2">
                    Danger Zone
                </h2>
                <p className="text-sm text-muted mb-4">
                    {t('auth.deleteAccountWarning')}
                </p>
                <p className="text-sm text-muted mb-4">
                    {t('auth.deleteAccountConfirm')}
                </p>
                <Link
                    to="/profile"
                    className="inline-block px-3 py-2 rounded border border-red-300 text-red-700 text-sm hover:bg-red-50 dark:hover:bg-red-950/30"
                >
                    {t('auth.deleteAccount')} →
                </Link>
            </section>
        </div>
    )
}
