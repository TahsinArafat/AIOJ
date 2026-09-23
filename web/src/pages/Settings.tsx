import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'

/**
 * Auth settings hub: security (2FA), connected OAuth accounts, and pointers
 * into Profile for password / danger-zone flows.
 */
export default function Settings() {
    const { t } = useTranslation()

    return (
        <div className="max-w-2xl mx-auto space-y-6">
            <h1 className="text-2xl font-bold">Settings</h1>

            <section className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-6">
                <h2 className="text-lg font-semibold mb-2">{t('auth.twoFactor')}</h2>
                <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
                    {t('auth.twoFactorScanQR')}
                </p>
                <div className="flex flex-wrap gap-3">
                    <Link
                        to="/auth/2fa/setup"
                        className="px-3 py-2 rounded bg-blue-600 text-white text-sm hover:bg-blue-700"
                    >
                        {t('auth.twoFactorSetupTitle')}
                    </Link>
                    <Link
                        to="/profile"
                        className="px-3 py-2 rounded border border-gray-300 dark:border-gray-600 text-sm hover:bg-gray-50 dark:hover:bg-gray-900"
                    >
                        Change password
                    </Link>
                </div>
            </section>

            <section className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-6">
                <h2 className="text-lg font-semibold mb-4">Connected accounts</h2>
                <div className="flex flex-col sm:flex-row gap-3">
                    <a
                        href="/api/auth/oauth/github/start"
                        className="px-3 py-2 rounded border border-gray-300 dark:border-gray-600 text-sm text-center hover:bg-gray-50 dark:hover:bg-gray-900"
                    >
                        {t('auth.linkGitHub')}
                    </a>
                    <a
                        href="/api/auth/oauth/google/start"
                        className="px-3 py-2 rounded border border-gray-300 dark:border-gray-600 text-sm text-center hover:bg-gray-50 dark:hover:bg-gray-900"
                    >
                        {t('auth.linkGoogle')}
                    </a>
                </div>
            </section>

            <section className="bg-white dark:bg-gray-800 border border-red-200 dark:border-red-900 rounded-lg p-6">
                <h2 className="text-lg font-semibold text-red-700 dark:text-red-400 mb-2">
                    Danger Zone
                </h2>
                <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
                    {t('auth.deleteAccountWarning')}
                </p>
                <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
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
