import { useCallback, useEffect, useMemo, useState } from 'react'
import { AlertTriangle, Check, RotateCcw, ShieldCheck } from 'lucide-react'
import { api } from '../../lib/api'
import type { AdminAuthMethods } from '../../types/api'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Card, CardHeader } from '../../components/ui/Card'
import { useToast } from '../../components/Toast'
import { errorMessage } from '../../lib/errors'

type SwitchKey = keyof AdminAuthMethods['settings']

interface MethodRow {
    key: SwitchKey
    label: string
    description: string
    /** Which provider must be configured, or null for the built-in method. */
    provider: 'google' | 'github' | null
    /** What to set when the provider is enabled but has no credentials. */
    configHint?: string
}

const METHODS: MethodRow[] = [
    {
        key: 'password_registration',
        label: 'Email and password registration',
        description:
            'Lets a visitor create an account by entering a username, email address and password. Turning this off does not affect existing accounts: they can still sign in with their password.',
        provider: null,
    },
    {
        key: 'google',
        label: 'Google sign-in',
        description:
            'Shows “Continue with Google” on the login and register pages. Users can also link Google from Settings.',
        provider: 'google',
        configHint: 'Set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET to make this available.',
    },
    {
        key: 'github',
        label: 'GitHub sign-in',
        description:
            'Shows “Continue with GitHub” on the login and register pages. Users can also link GitHub from Settings.',
        provider: 'github',
        configHint: 'Set GITHUB_CLIENT_ID and GITHUB_CLIENT_SECRET to make this available.',
    },
]

function draftOf(view: AdminAuthMethods): AdminAuthMethods['settings'] {
    return {
        password_registration: view.settings.password_registration,
        google: view.settings.google,
        github: view.settings.github,
    }
}

/**
 * Admin panel for the sign-in methods.
 *
 * Two facts decide whether a button appears on the login page: the switch here,
 * and whether the deployment has credentials. This panel shows both, because a
 * switch that is on but has no credentials behind it looks like a bug from the
 * outside — the provider simply never appears.
 */
export default function AuthMethodsPanel() {
    const toast = useToast()
    const [view, setView] = useState<AdminAuthMethods | null>(null)
    const [draft, setDraft] = useState<AdminAuthMethods['settings'] | null>(null)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)
    const [saving, setSaving] = useState(false)

    const load = useCallback(async () => {
        setLoading(true)
        try {
            const res = await api.admin.authMethods.get()
            const data = res?.data ?? null
            setView(data)
            setDraft(data ? draftOf(data) : null)
            setError(null)
        } catch (err) {
            setError(errorMessage(err) || 'Failed to load sign-in methods')
        } finally {
            setLoading(false)
        }
    }, [])

    // Deferred one microtask so `setLoading(true)` does not run synchronously
    // inside the effect body (react-hooks/set-state-in-effect).
    useEffect(() => { queueMicrotask(load) }, [load])

    const dirty = useMemo(() => {
        if (!view || !draft) return false
        const current = draftOf(view)
        return (Object.keys(current) as SwitchKey[]).some(k => current[k] !== draft[k])
    }, [view, draft])

    const save = async () => {
        if (!draft) return
        setSaving(true)
        try {
            const res = await api.admin.authMethods.set(draft)
            const data = res?.data ?? null
            setView(data)
            setDraft(data ? draftOf(data) : null)
            toast.success('Sign-in methods updated')
        } catch (err) {
            toast.error(errorMessage(err) || 'Failed to save sign-in methods')
        } finally {
            setSaving(false)
        }
    }

    const reset = async () => {
        if (!view) return
        setSaving(true)
        try {
            const res = await api.admin.authMethods.set(view.default)
            const data = res?.data ?? null
            setView(data)
            setDraft(data ? draftOf(data) : null)
            toast.success('Sign-in methods reset to the defaults')
        } catch (err) {
            toast.error(errorMessage(err) || 'Failed to reset sign-in methods')
        } finally {
            setSaving(false)
        }
    }

    if (loading && !view) {
        return <p className="text-sm text-muted">Loading sign-in methods…</p>
    }

    if (error) {
        return (
            <div role="alert" className="flex items-start gap-2 rounded-lg border border-bad/30 bg-bad/10 px-4 py-3 text-sm text-bad">
                <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                <span>{error}</span>
            </div>
        )
    }

    if (!view || !draft) return null

    // Warn about the one configuration that locks everybody out of sign-up.
    const passwordOff = draft.password_registration === false
    const noProviderUsable =
        (!draft.google || !view.configured.google) && (!draft.github || !view.configured.github)
    const registrationClosed = passwordOff && noProviderUsable

    return (
        <div className="space-y-6">
            <div className="rounded-lg border border-hairline bg-surface-2 px-4 py-3 text-sm text-muted">
                <p className="font-medium text-ink">Choose how people sign in to this instance.</p>
                <p className="mt-1">
                    A provider only appears on the login page when the switch is on <em>and</em> this deployment has
                    credentials for it. Signing in with a password is never affected by these switches — only creating a
                    new account with one.
                </p>
            </div>

            {registrationClosed && (
                <div role="alert" className="flex items-start gap-2 rounded-lg border border-warn/40 bg-warn/10 px-4 py-3 text-sm text-ink-soft">
                    <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-warn" />
                    <span>
                        New sign-ups are fully closed: password registration is off and no OAuth provider is usable.
                        Existing accounts can still sign in.
                    </span>
                </div>
            )}

            {METHODS.map(m => {
                const enabled = draft[m.key]
                const configured = m.provider ? view.configured[m.provider] : true
                const unusable = enabled && !configured

                return (
                    <Card key={m.key}>
                        <CardHeader
                            title={
                                <span className="flex flex-wrap items-center gap-2">
                                    {m.label}
                                    {!configured && <Badge tone="warn">not configured</Badge>}
                                    {configured && enabled && <Badge tone="ok">on</Badge>}
                                    {configured && !enabled && <Badge tone="neutral">off</Badge>}
                                </span>
                            }
                            description={m.description}
                            action={
                                <label className="flex cursor-pointer items-center gap-2 text-sm text-ink-soft">
                                    <input
                                        type="checkbox"
                                        role="switch"
                                        aria-label={`Enable ${m.label}`}
                                        checked={enabled}
                                        onChange={e => setDraft(d => (d ? { ...d, [m.key]: e.target.checked } : d))}
                                        className="h-4 w-4 rounded border-hairline-strong text-brand focus-visible:outline-[var(--ring)]"
                                    />
                                    {enabled ? 'Enabled' : 'Disabled'}
                                </label>
                            }
                        />

                        <div className="space-y-3 px-5 py-4">
                            {!configured && (
                                <p className="text-sm text-muted">
                                    <span className="font-medium text-ink">Not available on this deployment.</span>{' '}
                                    {m.configHint}
                                </p>
                            )}
                            {unusable && (
                                <p className="text-sm text-warn">
                                    Switched on, but the login page will not offer it until credentials are set.
                                </p>
                            )}
                            {configured && enabled && (
                                <p className="flex items-center gap-1.5 text-xs text-muted">
                                    <ShieldCheck className="h-3.5 w-3.5" />
                                    Visible on the login and register pages.
                                </p>
                            )}
                        </div>
                    </Card>
                )
            })}

            <div className="flex flex-wrap items-center gap-2">
                <Button onClick={save} disabled={!dirty || saving}>
                    <Check className="h-3.5 w-3.5" />
                    {saving ? 'Saving…' : 'Save changes'}
                </Button>
                <Button variant="secondary" onClick={reset} disabled={!view.customized || saving}>
                    <RotateCcw className="h-3.5 w-3.5" />
                    Reset to defaults
                </Button>
                {dirty && <span className="text-xs text-warn">Unsaved changes</span>}
                {!view.customized && !dirty && (
                    <span className="flex items-center gap-1 text-xs text-muted">
                        <ShieldCheck className="h-3.5 w-3.5" />
                        Using the built-in defaults
                    </span>
                )}
            </div>
        </div>
    )
}
