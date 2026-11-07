import { useCallback, useEffect, useMemo, useState } from 'react'
import { AlertTriangle, Check, RotateCcw, ShieldCheck } from 'lucide-react'
import { api } from '../../lib/api'
import type { AdminFeature, FeaturePolicy } from '../../types/api'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Card, CardHeader } from '../../components/ui/Card'
import { Field, Input } from '../../components/ui/Field'
import { useToast } from '../../components/Toast'
import { errorMessage } from '../../lib/errors'

const ROLE_LABELS: { role: string; label: string }[] = [
    { role: 'admin', label: 'Admin' },
    { role: 'setter', label: 'Setter' },
    { role: 'user', label: 'User' },
    { role: 'bot', label: 'Bot' },
]

const CATEGORY_LABELS: Record<string, string> = {
    core: 'Core',
    practice: 'Practice',
    community: 'Community',
    competition: 'Competition',
    platform: 'Platform',
    authoring: 'Authoring',
}

/** A policy reduced to the fields the editor owns, for dirty-checking. */
function draftOf(policy: FeaturePolicy): FeaturePolicy {
    return {
        enabled: policy.enabled,
        roles: [...(policy.roles ?? [])].sort(),
        min_rating: policy.min_rating || 0,
        max_rating: policy.max_rating || 0,
        message: policy.message ?? '',
    }
}

function sameDraft(a: FeaturePolicy, b: FeaturePolicy): boolean {
    return JSON.stringify(draftOf(a)) === JSON.stringify(draftOf(b))
}

/**
 * Admin panel for the feature gates.
 *
 * Each row edits one policy and saves it on its own, so a half-finished edit
 * elsewhere cannot be published by accident. The registry (labels, defaults,
 * whether the API actually enforces the gate) comes from the server, so this
 * panel never drifts from internal/feature.
 */
export default function FeaturesPanel() {
    const toast = useToast()
    const [features, setFeatures] = useState<AdminFeature[]>([])
    const [drafts, setDrafts] = useState<Record<string, FeaturePolicy>>({})
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)
    const [busy, setBusy] = useState<string | null>(null)

    const load = useCallback(async () => {
        setLoading(true)
        try {
            const res = await api.admin.features.list()
            const list = res?.data ?? []
            setFeatures(list)
            setDrafts(Object.fromEntries(list.map(f => [f.key, draftOf(f.policy)])))
            setError(null)
        } catch (err) {
            setError(errorMessage(err) || 'Failed to load feature gates')
        } finally {
            setLoading(false)
        }
    }, [])

    // Deferred one microtask so the initial `setLoading(true)` does not run
    // synchronously inside the effect body (react-hooks/set-state-in-effect).
    useEffect(() => { queueMicrotask(load) }, [load])

    const grouped = useMemo(() => {
        const out = new Map<string, AdminFeature[]>()
        for (const f of features) {
            const bucket = out.get(f.category) ?? []
            bucket.push(f)
            out.set(f.category, bucket)
        }
        return [...out.entries()]
    }, [features])

    const patch = (key: string, next: Partial<FeaturePolicy>) => {
        setDrafts(prev => ({ ...prev, [key]: { ...prev[key], ...next } }))
    }

    const toggleRole = (key: string, role: string) => {
        const current = drafts[key]?.roles ?? []
        const next = current.includes(role) ? current.filter(r => r !== role) : [...current, role]
        patch(key, { roles: next })
    }

    const save = async (f: AdminFeature) => {
        const draft = drafts[f.key]
        if (!draft) return
        setBusy(f.key)
        try {
            await api.admin.features.set(f.key, {
                enabled: draft.enabled,
                roles: draft.roles ?? [],
                min_rating: Number(draft.min_rating) || 0,
                max_rating: Number(draft.max_rating) || 0,
                message: draft.message ?? '',
            })
            toast.success(`${f.label} updated`)
            await load()
        } catch (err) {
            toast.error(errorMessage(err) || 'Failed to save the feature gate')
        } finally {
            setBusy(null)
        }
    }

    const reset = async (f: AdminFeature) => {
        setBusy(f.key)
        try {
            await api.admin.features.reset(f.key)
            toast.success(`${f.label} reset to the default`)
            await load()
        } catch (err) {
            toast.error(errorMessage(err) || 'Failed to reset the feature gate')
        } finally {
            setBusy(null)
        }
    }

    if (loading && features.length === 0) {
        return <p className="text-sm text-muted">Loading feature gates…</p>
    }

    if (error) {
        return (
            <div role="alert" className="flex items-start gap-2 rounded-lg border border-bad/30 bg-bad/10 px-4 py-3 text-sm text-bad">
                <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                <span>{error}</span>
            </div>
        )
    }

    return (
        <div className="space-y-6">
            <div className="rounded-lg border border-hairline bg-surface-2 px-4 py-3 text-sm text-muted">
                <p className="font-medium text-ink">Every optional surface can be switched off, limited to roles, or limited to a rating range.</p>
                <p className="mt-1">
                    A disabled feature is off for everyone, including admins. Role and rating limits exempt admins, so you can always
                    reach the surface you are configuring. Gates marked <span className="font-medium text-ink">API</span> reject
                    requests on the server; the rest hide the UI only.
                </p>
            </div>

            {grouped.map(([category, items]) => (
                <section key={category} className="space-y-3">
                    <h3 className="text-xs font-semibold uppercase tracking-wide text-muted">
                        {CATEGORY_LABELS[category] ?? category}
                    </h3>

                    {items.map(f => {
                        const draft = drafts[f.key] ?? draftOf(f.policy)
                        const dirty = !sameDraft(draft, f.policy)
                        const restricted =
                            !draft.enabled || (draft.roles?.length ?? 0) > 0 || (draft.min_rating ?? 0) > 0 || (draft.max_rating ?? 0) > 0

                        return (
                            <Card key={f.key}>
                                <CardHeader
                                    title={
                                        <span className="flex flex-wrap items-center gap-2">
                                            {f.label}
                                            <code className="rounded bg-surface-3 px-1.5 py-0.5 text-[11px] font-normal text-muted">{f.key}</code>
                                            {f.customized && <Badge tone="brand">custom</Badge>}
                                            {!restricted && <Badge tone="ok">open</Badge>}
                                            {f.api_enforced
                                                ? <Badge tone="neutral">API</Badge>
                                                : <Badge tone="warn">UI only</Badge>}
                                            {f.requires_login && <Badge tone="neutral">login</Badge>}
                                        </span>
                                    }
                                    description={f.description}
                                    action={
                                        <label className="flex cursor-pointer items-center gap-2 text-sm text-ink-soft">
                                            <input
                                                type="checkbox"
                                                role="switch"
                                                aria-label={`Enable ${f.label}`}
                                                checked={draft.enabled}
                                                onChange={e => patch(f.key, { enabled: e.target.checked })}
                                                className="h-4 w-4 rounded border-hairline-strong text-brand focus-visible:outline-[var(--ring)]"
                                            />
                                            {draft.enabled ? 'Enabled' : 'Disabled'}
                                        </label>
                                    }
                                />

                                <div className="space-y-4 px-5 py-4">
                                    <div>
                                        <p className="mb-1.5 text-xs font-medium text-ink-soft">Allowed roles</p>
                                        <div className="flex flex-wrap gap-2">
                                            {ROLE_LABELS.map(r => {
                                                const active = (draft.roles ?? []).includes(r.role)
                                                return (
                                                    <button
                                                        key={r.role}
                                                        type="button"
                                                        aria-pressed={active}
                                                        onClick={() => toggleRole(f.key, r.role)}
                                                        className={`rounded-lg border px-2.5 py-1 text-xs font-medium transition-colors ${active
                                                            ? 'border-brand bg-brand-soft text-brand'
                                                            : 'border-hairline-strong bg-surface text-muted hover:bg-surface-2'
                                                            }`}
                                                    >
                                                        {r.label}
                                                    </button>
                                                )
                                            })}
                                            <span className="self-center text-xs text-muted">
                                                {(draft.roles?.length ?? 0) === 0 ? 'Everyone' : 'Only the selected roles'}
                                            </span>
                                        </div>
                                    </div>

                                    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                                        <Field label="Minimum rating" hint="0 means no minimum." htmlFor={`${f.key}-min`}>
                                            <Input
                                                id={`${f.key}-min`}
                                                type="number"
                                                min={0}
                                                max={5000}
                                                step={50}
                                                value={draft.min_rating ?? 0}
                                                onChange={e => patch(f.key, { min_rating: Number(e.target.value) })}
                                            />
                                        </Field>
                                        <Field label="Maximum rating" hint="0 means no maximum." htmlFor={`${f.key}-max`}>
                                            <Input
                                                id={`${f.key}-max`}
                                                type="number"
                                                min={0}
                                                max={5000}
                                                step={50}
                                                value={draft.max_rating ?? 0}
                                                onChange={e => patch(f.key, { max_rating: Number(e.target.value) })}
                                            />
                                        </Field>
                                    </div>

                                    <Field
                                        label="Message shown to a blocked user"
                                        hint="Optional. Explain why the feature is unavailable, e.g. a reopen date."
                                        htmlFor={`${f.key}-message`}
                                    >
                                        <Input
                                            id={`${f.key}-message`}
                                            value={draft.message ?? ''}
                                            maxLength={280}
                                            placeholder="e.g. Back after the round on Sunday."
                                            onChange={e => patch(f.key, { message: e.target.value })}
                                        />
                                    </Field>

                                    <div className="flex flex-wrap items-center gap-2">
                                        <Button size="sm" onClick={() => save(f)} disabled={!dirty || busy === f.key}>
                                            <Check className="h-3.5 w-3.5" />
                                            {busy === f.key ? 'Saving…' : 'Save'}
                                        </Button>
                                        <Button
                                            size="sm"
                                            variant="secondary"
                                            onClick={() => reset(f)}
                                            disabled={!f.customized || busy === f.key}
                                        >
                                            <RotateCcw className="h-3.5 w-3.5" />
                                            Reset to default
                                        </Button>
                                        {dirty && <span className="text-xs text-warn">Unsaved changes</span>}
                                        {!f.customized && !dirty && (
                                            <span className="flex items-center gap-1 text-xs text-muted">
                                                <ShieldCheck className="h-3.5 w-3.5" />
                                                Using the built-in default
                                            </span>
                                        )}
                                    </div>
                                </div>
                            </Card>
                        )
                    })}
                </section>
            ))}
        </div>
    )
}
