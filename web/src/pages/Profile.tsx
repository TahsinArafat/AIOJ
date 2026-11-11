import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { Link, useNavigate, useLocation } from 'react-router-dom'
import type { TeamInvite, GroupInvite } from '../types/community'
import type { UserProfile } from '../types/api'
import {
    AlertCircle, Award, Building2, CalendarDays, Download, ExternalLink, KeyRound,
    Mail, MapPin, RotateCcw, ShieldAlert, Trash2, UserRound, Users, type LucideIcon,
} from 'lucide-react'
import { api, getAccessToken, clearTokens, signOut } from '../lib/api'
import { loginHref } from '../lib/authRedirect'
import { errorMessage } from '../lib/errors'
import { getRatingColor, getRatingTitle } from '../lib/rating'
import RatingBadge from '../components/RatingBadge'
import SetterApplication from '../components/SetterApplication'
import CountryFlag, { COUNTRY_NAMES, flagEmoji } from '../components/CountryFlag'
import { EmptyState } from '../components/EmptyState'
import { useToast } from '../components/Toast'
import { useTheme } from '../context/ThemeContext'
import { Badge, type BadgeTone } from '../components/ui/Badge'
import { Button } from '../components/ui/Button'
import { buttonClass } from '../components/ui/buttonStyles'
import { Card, CardBody, CardHeader } from '../components/ui/Card'
import { Field, Input, Select } from '../components/ui/Field'
import { PageHeader, Skeleton } from '../components/ui/PageHeader'

// ── Token ────────────────────────────────────────────────────────────────────

/**
 * The access token is the only place the handle and role live client-side.
 * It carries no email or rating, which is why the old page rendered an
 * "Email —" row and "Unrated" for everyone: both were read off the token and
 * both were always undefined. Anything profile-shaped now comes from
 * `GET /users/profile/edit`.
 */
interface TokenUser {
    uid: string
    uname: string
    role?: string
}

function decodeUser(): TokenUser | null {
    const token = getAccessToken()
    if (!token) return null
    try {
        return JSON.parse(atob(token.split('.')[1])) as TokenUser
    } catch {
        return null
    }
}

// ── Form model ───────────────────────────────────────────────────────────────

const BIO_MAX = 500
const SHORT_TEXT_MAX = 64

interface Draft {
    first_name: string
    last_name: string
    country: string
    city: string
    organization: string
    github_url: string
    bio: string
    avatar_url: string
    show_email: boolean
    show_tags: boolean
}

function toDraft(p: UserProfile): Draft {
    return {
        first_name: p.first_name ?? '',
        last_name: p.last_name ?? '',
        country: p.country ?? '',
        city: p.city ?? '',
        organization: p.organization ?? '',
        github_url: p.github_url ?? '',
        bio: p.bio ?? '',
        avatar_url: p.avatar_url ?? '',
        show_email: Boolean(p.show_email),
        show_tags: p.show_tags ?? true,
    }
}

/** Empty is fine; anything else must be an absolute http(s) URL. */
function urlError(value: string, label: string): string {
    const v = value.trim()
    if (!v) return ''
    try {
        const parsed = new URL(v)
        if (parsed.protocol === 'http:' || parsed.protocol === 'https:') return ''
        return `${label} must start with http:// or https://`
    } catch {
        return `${label} must be a full URL, e.g. https://example.com`
    }
}

function fullName(d: { first_name?: string; last_name?: string }): string {
    return [d.first_name?.trim(), d.last_name?.trim()].filter(Boolean).join(' ')
}

function formatDate(iso?: string): string | null {
    if (!iso) return null
    const d = new Date(iso)
    if (Number.isNaN(d.getTime()) || d.getFullYear() < 1970) return null
    return d.toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' })
}

const ROLE_TONE: Record<string, BadgeTone> = { admin: 'accent', setter: 'warn' }

// ── Small shared pieces ──────────────────────────────────────────────────────

/**
 * Avatar with an initial fallback.
 *
 * `failedSrc` (rather than a boolean) keeps the fallback correct when the URL
 * changes — a plain flag would stay broken for the next URL without an effect
 * to reset it.
 */
function Avatar({ src, name, size = 80, ring }: { src?: string | null; name: string; size?: number; ring?: string }) {
    const [failedSrc, setFailedSrc] = useState<string | null>(null)
    const url = (src ?? '').trim()
    const showImage = url !== '' && failedSrc !== url
    const initial = (name.trim().charAt(0) || '?').toUpperCase()

    return (
        <div
            className="flex shrink-0 items-center justify-center overflow-hidden rounded-full bg-brand-soft font-bold text-brand"
            style={{
                width: size,
                height: size,
                fontSize: Math.round(size * 0.38),
                boxShadow: ring ? `0 0 0 3px ${ring}45` : undefined,
            }}
        >
            {showImage ? (
                // Decorative: the handle next to it already names the person.
                <img src={url} alt="" className="h-full w-full object-cover" onError={() => setFailedSrc(url)} />
            ) : (
                <span aria-hidden="true">{initial}</span>
            )}
        </div>
    )
}

function Stat({ label, value, hint }: { label: string; value: ReactNode; hint?: string }) {
    return (
        <div className="px-4 py-3">
            <p className="text-[11px] font-medium uppercase tracking-wider text-muted">{label}</p>
            <p className="mt-1 text-lg font-semibold leading-tight text-ink">{value}</p>
            {hint && <p className="text-[11px] text-muted-soft">{hint}</p>}
        </div>
    )
}

function FormSection({ title, description, children }: { title: string; description?: string; children: ReactNode }) {
    return (
        <section className="border-t border-hairline pt-5 first:border-0 first:pt-0">
            <h3 className="text-sm font-semibold text-ink">{title}</h3>
            {description && <p className="mt-0.5 text-xs text-muted">{description}</p>}
            <div className="mt-3 space-y-4">{children}</div>
        </section>
    )
}

/** A privacy switch rendered as a described row instead of a bare checkbox. */
function ToggleRow({
    id,
    label,
    description,
    checked,
    onChange,
}: {
    id: string
    label: string
    description: string
    checked: boolean
    onChange: (next: boolean) => void
}) {
    return (
        <label
            htmlFor={id}
            className="flex cursor-pointer items-start gap-3 rounded-lg border border-hairline px-3 py-2.5 transition-colors hover:bg-surface-2"
        >
            <input
                id={id}
                type="checkbox"
                checked={checked}
                onChange={e => onChange(e.target.checked)}
                className="mt-0.5 h-4 w-4 shrink-0 rounded border-hairline-strong text-brand focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--ring)]"
            />
            <span className="min-w-0">
                <span className="block text-sm font-medium text-ink">{label}</span>
                <span className="block text-xs text-muted">{description}</span>
            </span>
        </label>
    )
}

// ── Identity header ──────────────────────────────────────────────────────────

function IdentityCard({
    username,
    role,
    profile,
    loading,
}: {
    username: string
    role?: string
    profile: UserProfile | null
    loading: boolean
}) {
    const { theme } = useTheme()
    const rating = profile?.rating ?? 0
    const colors = getRatingColor(rating)
    // Same light/dark split RatingBadge uses, so the handle and its pill agree.
    const rankColor = theme === 'dark' && colors.darkText ? colors.darkText : colors.text
    const rankTitle = rating > 0 ? getRatingTitle(rating) : 'Unrated'
    const name = profile ? fullName(profile) : ''
    const joined = formatDate(profile?.created_at)

    return (
        <Card className="mb-6 overflow-hidden">
            <div className="flex flex-col gap-5 p-5 sm:flex-row sm:items-start">
                {loading ? (
                    <Skeleton className="h-20 w-20 shrink-0 rounded-full" />
                ) : (
                    <Avatar src={profile?.avatar_url} name={username} size={80} ring={rating > 0 ? colors.hex : undefined} />
                )}

                <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                        <h2
                            className="text-2xl font-bold tracking-tight"
                            style={{ color: rating > 0 ? rankColor : undefined }}
                        >
                            {username}
                        </h2>
                        {rating > 0 && (
                            <span className="text-sm font-semibold" style={{ color: rankColor }}>
                                {rankTitle}
                            </span>
                        )}
                        <Badge tone={ROLE_TONE[role ?? ''] ?? 'neutral'}>
                            {(role ?? 'user').replace(/^./, c => c.toUpperCase())}
                        </Badge>
                    </div>

                    {loading ? (
                        <div className="mt-3 space-y-2">
                            <Skeleton className="h-3.5 w-56" />
                            <Skeleton className="h-3.5 w-40" />
                        </div>
                    ) : (
                        <>
                            <p className="mt-1 text-sm text-ink-soft">
                                {name || <span className="text-muted-soft">No name set yet — add one below.</span>}
                            </p>
                            <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted">
                                {(profile?.country || profile?.city) && (
                                    <span className="inline-flex items-center gap-1.5">
                                        <MapPin className="h-3.5 w-3.5 text-muted-soft" aria-hidden="true" />
                                        {profile?.country
                                            ? <CountryFlag country={profile.country} withName />
                                            : null}
                                        {profile?.country && profile?.city ? '·' : null}
                                        {profile?.city}
                                    </span>
                                )}
                                {profile?.organization && (
                                    <span className="inline-flex items-center gap-1.5">
                                        <Building2 className="h-3.5 w-3.5 text-muted-soft" aria-hidden="true" />
                                        {profile.organization}
                                    </span>
                                )}
                                {joined && (
                                    <span className="inline-flex items-center gap-1.5">
                                        <CalendarDays className="h-3.5 w-3.5 text-muted-soft" aria-hidden="true" />
                                        Member since {joined}
                                    </span>
                                )}
                                {profile?.email && (
                                    <span className="inline-flex items-center gap-1.5">
                                        <Mail className="h-3.5 w-3.5 text-muted-soft" aria-hidden="true" />
                                        {profile.email}
                                        <Badge tone={profile.show_email ? 'ok' : 'neutral'}>
                                            {profile.show_email ? 'public' : 'private'}
                                        </Badge>
                                    </span>
                                )}
                            </div>
                        </>
                    )}
                </div>

                <div className="flex shrink-0 flex-wrap gap-2">
                    <Link
                        to={`/user/${username}`}
                        className={buttonClass({ variant: 'secondary', size: 'sm' })}
                    >
                        Public profile
                        <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
                    </Link>
                </div>
            </div>

            {/* Rank/activity strip — the numbers a competitive programmer opens
                a profile for, all from the profile endpoint. */}
            <div className="grid grid-cols-2 divide-x divide-y divide-hairline border-t border-hairline sm:grid-cols-3 sm:divide-y-0 lg:grid-cols-5">
                <Stat
                    label="Rating"
                    value={rating > 0 ? <RatingBadge rating={rating} size="md" /> : <span className="text-muted-soft">—</span>}
                    hint={rating > 0 ? rankTitle : 'Unrated'}
                />
                <Stat
                    label="Max rating"
                    value={profile?.max_rating ? <RatingBadge rating={profile.max_rating} size="md" /> : <span className="text-muted-soft">—</span>}
                />
                <Stat label="Contests" value={profile?.contest_count ?? '—'} />
                <Stat label="Solved" value={profile?.problems_solved ?? '—'} hint="distinct problems" />
                <Stat label="Submissions" value={profile?.submissions ?? '—'} />
            </div>
        </Card>
    )
}

// ── Sections ─────────────────────────────────────────────────────────────────

function DetailsSection({
    profile,
    username,
    onSaved,
}: {
    profile: UserProfile
    username: string
    onSaved: (p: UserProfile) => void
}) {
    const toast = useToast()
    const [draft, setDraft] = useState<Draft>(() => toDraft(profile))
    const [baseline, setBaseline] = useState<Draft>(() => toDraft(profile))
    const [saving, setSaving] = useState(false)
    const [saveError, setSaveError] = useState('')

    const dirty = useMemo(() => JSON.stringify(draft) !== JSON.stringify(baseline), [draft, baseline])

    const githubError = urlError(draft.github_url, 'GitHub URL')
    const avatarError = urlError(draft.avatar_url, 'Avatar URL')
    const bioError = draft.bio.length > BIO_MAX
        ? `Bio is ${draft.bio.length - BIO_MAX} character${draft.bio.length - BIO_MAX === 1 ? '' : 's'} too long`
        : ''
    const hasErrors = Boolean(githubError || avatarError || bioError)

    const set = <K extends keyof Draft>(key: K, value: Draft[K]) =>
        setDraft(prev => ({ ...prev, [key]: value }))

    // The saved country may predate the picker (or be a free-text entry), so it
    // stays selectable instead of being silently dropped on the next save.
    const countryOptions = useMemo(() => {
        const names = new Set(COUNTRY_NAMES)
        if (draft.country) names.add(draft.country)
        return [...names].sort((a, b) => a.localeCompare(b))
    }, [draft.country])

    const save = async () => {
        if (saving || hasErrors || !dirty) return
        setSaving(true)
        setSaveError('')
        try {
            const updated = await api.users.updateProfile({
                first_name: draft.first_name.trim(),
                last_name: draft.last_name.trim(),
                country: draft.country,
                city: draft.city.trim(),
                organization: draft.organization.trim(),
                github_url: draft.github_url.trim(),
                bio: draft.bio.trim(),
                avatar_url: draft.avatar_url.trim(),
                show_email: draft.show_email,
                show_tags: draft.show_tags,
            })
            const next = toDraft(updated)
            setDraft(next)
            setBaseline(next)
            onSaved(updated)
            toast.success('Profile updated')
        } catch (e) {
            setSaveError(errorMessage(e) || 'Could not save your profile')
        } finally {
            setSaving(false)
        }
    }

    const discard = () => {
        setDraft(baseline)
        setSaveError('')
    }

    return (
        <Card>
            <CardHeader
                title="Profile details"
                description="Everything here appears on your public profile, except what you keep private below."
            />
            <CardBody>
                <form
                    className="space-y-5"
                    onSubmit={e => {
                        e.preventDefault()
                        void save()
                    }}
                >
                    <FormSection title="Avatar" description="Paste a link to an image. Leave empty to use your initial.">
                        <div className="flex items-center gap-4">
                            <Avatar src={draft.avatar_url} name={fullName(draft) || username} size={64} />
                            <div className="min-w-0 flex-1">
                                <Field label="Avatar URL" htmlFor="profile-avatar" error={avatarError}>
                                    <Input
                                        id="profile-avatar"
                                        type="url"
                                        inputMode="url"
                                        placeholder="https://…/me.png"
                                        value={draft.avatar_url}
                                        onChange={e => set('avatar_url', e.target.value)}
                                    />
                                </Field>
                            </div>
                        </div>
                    </FormSection>

                    <FormSection title="Name" description="Your real or preferred name, shown next to your handle.">
                        <div className="grid gap-4 sm:grid-cols-2">
                            <Field label="First name" htmlFor="profile-first-name">
                                <Input
                                    id="profile-first-name"
                                    maxLength={SHORT_TEXT_MAX}
                                    autoComplete="given-name"
                                    value={draft.first_name}
                                    onChange={e => set('first_name', e.target.value)}
                                />
                            </Field>
                            <Field label="Last name" htmlFor="profile-last-name">
                                <Input
                                    id="profile-last-name"
                                    maxLength={SHORT_TEXT_MAX}
                                    autoComplete="family-name"
                                    value={draft.last_name}
                                    onChange={e => set('last_name', e.target.value)}
                                />
                            </Field>
                        </div>
                    </FormSection>

                    <FormSection title="Location" description="Used for the flag on your profile and the rankings page.">
                        <div className="grid gap-4 sm:grid-cols-2">
                            <Field label="Country" htmlFor="profile-country">
                                <Select
                                    id="profile-country"
                                    value={draft.country}
                                    onChange={e => set('country', e.target.value)}
                                >
                                    <option value="">Not set</option>
                                    {countryOptions.map(name => (
                                        <option key={name} value={name}>
                                            {[flagEmoji(name), name].filter(Boolean).join(' ')}
                                        </option>
                                    ))}
                                </Select>
                            </Field>
                            <Field label="City" htmlFor="profile-city">
                                <Input
                                    id="profile-city"
                                    maxLength={SHORT_TEXT_MAX}
                                    value={draft.city}
                                    onChange={e => set('city', e.target.value)}
                                />
                            </Field>
                        </div>
                    </FormSection>

                    <FormSection title="About" description="Where you study or work, and a short introduction.">
                        <Field label="Organization" htmlFor="profile-organization">
                            <Input
                                id="profile-organization"
                                maxLength={SHORT_TEXT_MAX}
                                placeholder="University, company, school…"
                                value={draft.organization}
                                onChange={e => set('organization', e.target.value)}
                            />
                        </Field>
                        <Field
                            label="Bio"
                            htmlFor="profile-bio"
                            error={bioError}
                            hint={`${draft.bio.length}/${BIO_MAX} characters. Markdown is not rendered.`}
                        >
                            <textarea
                                id="profile-bio"
                                rows={4}
                                value={draft.bio}
                                onChange={e => set('bio', e.target.value)}
                                className="w-full rounded-lg border border-hairline-strong bg-surface px-3 py-2 text-sm text-ink transition-colors hover:border-muted-soft focus:border-brand focus:outline-none focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--ring)]"
                            />
                        </Field>
                    </FormSection>

                    <FormSection title="Links">
                        <Field label="GitHub URL" htmlFor="profile-github" error={githubError}>
                            <Input
                                id="profile-github"
                                type="url"
                                inputMode="url"
                                placeholder="https://github.com/you"
                                value={draft.github_url}
                                onChange={e => set('github_url', e.target.value)}
                            />
                        </Field>
                    </FormSection>

                    <FormSection title="Privacy" description="What other people can see on your profile.">
                        <div className="space-y-2">
                            <ToggleRow
                                id="profile-show-email"
                                label="Show my email"
                                description="Off by default. Turning this on publishes your account email on your public profile."
                                checked={draft.show_email}
                                onChange={next => set('show_email', next)}
                            />
                            <ToggleRow
                                id="profile-show-tags"
                                label="Show problem tags"
                                description="Lets others see the topic tags of problems you have solved."
                                checked={draft.show_tags}
                                onChange={next => set('show_tags', next)}
                            />
                        </div>
                    </FormSection>

                    {saveError && (
                        <p role="alert" className="flex items-center gap-2 rounded-lg bg-bad/10 px-3 py-2 text-sm text-bad">
                            <AlertCircle className="h-4 w-4 shrink-0" aria-hidden="true" />
                            {saveError}
                        </p>
                    )}

                    <div className="flex flex-wrap items-center gap-3 border-t border-hairline pt-4">
                        <Button type="submit" disabled={!dirty || hasErrors || saving}>
                            {saving ? 'Saving…' : 'Save changes'}
                        </Button>
                        <Button variant="secondary" onClick={discard} disabled={!dirty || saving}>
                            <RotateCcw className="h-4 w-4" aria-hidden="true" />
                            Discard
                        </Button>
                        {/* Announced, not just coloured: the old page only printed
                            a line of text after a successful save. */}
                        <p aria-live="polite" className="text-xs text-muted">
                            {hasErrors
                                ? 'Fix the highlighted fields before saving.'
                                : dirty
                                    ? 'You have unsaved changes.'
                                    : 'All changes saved.'}
                        </p>
                    </div>
                </form>
            </CardBody>
        </Card>
    )
}

function SecuritySection() {
    const navigate = useNavigate()
    const location = useLocation()
    const [currentPassword, setCurrentPassword] = useState('')
    const [newPassword, setNewPassword] = useState('')
    const [confirmPassword, setConfirmPassword] = useState('')
    const [saving, setSaving] = useState(false)
    const [message, setMessage] = useState('')

    const mismatch = confirmPassword !== '' && newPassword !== confirmPassword
    const tooShort = newPassword !== '' && newPassword.length < 12
    const blocked = mismatch || tooShort

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        if (mismatch) {
            setMessage('Passwords do not match')
            return
        }
        // Mirrors auth.ValidatePasswordStrength; the server enforces it anyway,
        // but a 6-character minimum here only produced a confusing 400.
        if (newPassword.length < 12) {
            setMessage('New password must be at least 12 characters')
            return
        }
        setSaving(true)
        setMessage('')
        try {
            await api.users.updatePassword({ current_password: currentPassword, new_password: newPassword })
            // The server revoked every refresh token for this account, and the
            // current access token still carries the must-change flag, so the
            // only coherent next step is signing in again.
            setMessage('Password updated! Please sign in again.')
            setCurrentPassword('')
            setNewPassword('')
            setConfirmPassword('')
            await signOut()
            navigate(loginHref(location))
        } catch (e) {
            setMessage('Error: ' + errorMessage(e))
        } finally {
            setSaving(false)
        }
    }

    return (
        <div className="space-y-4">
            <Card>
                <CardHeader
                    title="Password"
                    description="Changing your password signs out every other session."
                />
                <CardBody>
                    <form onSubmit={handleSubmit} className="space-y-4">
                        <Field label="Current password" htmlFor="pw-current">
                            <Input
                                id="pw-current"
                                type="password"
                                autoComplete="current-password"
                                value={currentPassword}
                                onChange={e => setCurrentPassword(e.target.value)}
                                required
                            />
                        </Field>
                        <div className="grid gap-4 sm:grid-cols-2">
                            <Field
                                label="New password"
                                htmlFor="pw-new"
                                error={tooShort ? 'At least 12 characters' : undefined}
                                hint="At least 12 characters."
                            >
                                <Input
                                    id="pw-new"
                                    type="password"
                                    autoComplete="new-password"
                                    value={newPassword}
                                    onChange={e => setNewPassword(e.target.value)}
                                    required
                                />
                            </Field>
                            <Field
                                label="Confirm new password"
                                htmlFor="pw-confirm"
                                error={mismatch ? 'Passwords do not match' : undefined}
                            >
                                <Input
                                    id="pw-confirm"
                                    type="password"
                                    autoComplete="new-password"
                                    value={confirmPassword}
                                    onChange={e => setConfirmPassword(e.target.value)}
                                    required
                                />
                            </Field>
                        </div>
                        {message && (
                            <p
                                aria-live="polite"
                                className={`text-sm ${message.startsWith('Error') || message === 'Passwords do not match'
                                    ? 'text-bad'
                                    : 'text-ok'
                                    }`}
                            >
                                {message}
                            </p>
                        )}
                        <Button type="submit" disabled={saving || blocked || !currentPassword || !newPassword}>
                            {saving ? 'Updating…' : 'Update password'}
                        </Button>
                    </form>
                </CardBody>
            </Card>

            <Card>
                <CardHeader title="Related settings" description="Sign-in methods, API access and notifications." />
                <CardBody>
                    <ul className="divide-y divide-hairline text-sm">
                        {[
                            { to: '/settings', label: 'Account security', hint: 'Two-factor authentication and sessions' },
                            { to: '/settings/notifications', label: 'Notification preferences', hint: 'Email and in-app alerts' },
                            { to: '/settings/api', label: 'API keys', hint: 'Programmatic access to your account' },
                        ].map(item => (
                            <li key={item.to}>
                                <Link
                                    to={item.to}
                                    className="flex items-center justify-between gap-3 py-2.5 text-ink-soft transition-colors hover:text-brand"
                                >
                                    <span>
                                        <span className="block font-medium">{item.label}</span>
                                        <span className="block text-xs text-muted">{item.hint}</span>
                                    </span>
                                    <span aria-hidden="true" className="text-muted">→</span>
                                </Link>
                            </li>
                        ))}
                    </ul>
                </CardBody>
            </Card>
        </div>
    )
}

function InviteRow({
    name,
    to,
    note,
    noteTone = 'muted',
    actions,
}: {
    name: string
    to: string
    note: string
    noteTone?: 'muted' | 'warn'
    actions: ReactNode
}) {
    return (
        <li className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-hairline px-4 py-3">
            <div className="min-w-0">
                <Link to={to} className="text-sm font-medium text-brand hover:underline">
                    {name}
                </Link>
                <p className={`text-xs ${noteTone === 'warn' ? 'text-warn' : 'text-muted'}`}>{note}</p>
            </div>
            <div className="flex shrink-0 gap-2">{actions}</div>
        </li>
    )
}

function InvitesSection({
    userId,
    invites,
    loading,
    reload,
}: {
    userId: string
    invites: { teams: TeamInvite[]; groups: GroupInvite[] }
    loading: boolean
    reload: () => Promise<void>
}) {
    const toast = useToast()
    const [acting, setActing] = useState<string | null>(null)

    const respond = async (type: 'team' | 'group', id: string, action: string) => {
        setActing(`${type}-${id}-${action}`)
        try {
            if (type === 'team') {
                await api.teams.respond(id, { user_id: userId, action })
            } else {
                await api.groups.respond(id, { user_id: userId, action })
            }
            await reload()
        } catch (e) {
            toast.error('Failed: ' + errorMessage(e))
        } finally {
            setActing(null)
        }
    }

    const teamInvites = invites.teams.filter(i => i.role === 'invited')
    const groupInvites = invites.groups.filter(i => i.role === 'invited')
    const teamRequests = invites.teams.filter(i => i.role === 'requested')
    const groupRequests = invites.groups.filter(i => i.role === 'requested')
    const total = teamInvites.length + groupInvites.length + teamRequests.length + groupRequests.length

    const acceptDecline = (type: 'team' | 'group', id: string) => (
        <>
            <Button
                size="sm"
                onClick={() => respond(type, id, 'accept')}
                disabled={acting === `${type}-${id}-accept`}
            >
                Accept
            </Button>
            <Button
                size="sm"
                variant="secondary"
                onClick={() => respond(type, id, 'decline')}
                disabled={acting === `${type}-${id}-decline`}
            >
                Decline
            </Button>
        </>
    )

    return (
        <Card>
            <CardHeader
                title="Invitations"
                description="Team and group invitations waiting for your answer."
                action={total > 0 ? <Badge tone="brand">{total} pending</Badge> : undefined}
            />
            <CardBody>
                {loading ? (
                    <div className="space-y-2">
                        <Skeleton className="h-14 w-full" />
                        <Skeleton className="h-14 w-full" />
                    </div>
                ) : total === 0 ? (
                    <EmptyState
                        text="No pending invites or requests"
                        description="Invitations from teams and groups will show up here."
                    />
                ) : (
                    <div className="space-y-6">
                        {teamInvites.length > 0 && (
                            <section>
                                <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted">
                                    Team invitations
                                </h3>
                                <ul className="space-y-2">
                                    {teamInvites.map(inv => (
                                        <InviteRow
                                            key={`team-inv-${inv.team_id}`}
                                            name={inv.team_name || 'Unnamed team'}
                                            to={`/teams/${inv.team_id}`}
                                            note="Invited to join"
                                            actions={acceptDecline('team', inv.team_id)}
                                        />
                                    ))}
                                </ul>
                            </section>
                        )}
                        {groupInvites.length > 0 && (
                            <section>
                                <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted">
                                    Group invitations
                                </h3>
                                <ul className="space-y-2">
                                    {groupInvites.map(inv => (
                                        <InviteRow
                                            key={`group-inv-${inv.group_id}`}
                                            name={inv.group_name || 'Unnamed group'}
                                            to={`/groups/${inv.group_id}`}
                                            note="Invited to join"
                                            actions={acceptDecline('group', inv.group_id)}
                                        />
                                    ))}
                                </ul>
                            </section>
                        )}
                        {teamRequests.length > 0 && (
                            <section>
                                <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted">
                                    Team join requests
                                </h3>
                                <ul className="space-y-2">
                                    {teamRequests.map(req => (
                                        <InviteRow
                                            key={`team-req-${req.team_id}`}
                                            name={req.team_name || 'Unnamed team'}
                                            to={`/teams/${req.team_id}`}
                                            note="Awaiting approval"
                                            noteTone="warn"
                                            actions={
                                                <Button
                                                    size="sm"
                                                    variant="secondary"
                                                    onClick={() => respond('team', req.team_id, 'decline')}
                                                    disabled={acting === `team-${req.team_id}-decline`}
                                                >
                                                    Cancel request
                                                </Button>
                                            }
                                        />
                                    ))}
                                </ul>
                            </section>
                        )}
                        {groupRequests.length > 0 && (
                            <section>
                                <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted">
                                    Group join requests
                                </h3>
                                <ul className="space-y-2">
                                    {groupRequests.map(req => (
                                        <InviteRow
                                            key={`group-req-${req.group_id}`}
                                            name={req.group_name || 'Unnamed group'}
                                            to={`/groups/${req.group_id}`}
                                            note="Awaiting approval"
                                            noteTone="warn"
                                            actions={
                                                <Button
                                                    size="sm"
                                                    variant="secondary"
                                                    onClick={() => respond('group', req.group_id, 'decline')}
                                                    disabled={acting === `group-${req.group_id}-decline`}
                                                >
                                                    Cancel request
                                                </Button>
                                            }
                                        />
                                    ))}
                                </ul>
                            </section>
                        )}
                    </div>
                )}
            </CardBody>
        </Card>
    )
}

function DangerSection() {
    const toast = useToast()
    const navigate = useNavigate()
    const [exporting, setExporting] = useState(false)
    const [password, setPassword] = useState('')
    const [confirm, setConfirm] = useState(false)
    const [busy, setBusy] = useState(false)
    const [error, setError] = useState('')

    const exportData = async () => {
        setExporting(true)
        try {
            const data = await api.users.exportMyData()
            const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
            const url = URL.createObjectURL(blob)
            const a = document.createElement('a')
            a.href = url
            a.download = 'aioj-data-export.json'
            a.click()
            URL.revokeObjectURL(url)
            toast.success('Data export downloaded')
        } catch (e) {
            toast.error(errorMessage(e) || 'Export failed')
        } finally {
            setExporting(false)
        }
    }

    const handleDelete = async () => {
        if (!confirm) {
            setError('Confirm the permanent deletion to proceed')
            return
        }
        setBusy(true)
        setError('')
        try {
            await api.users.deleteAccount({ password, confirm: true })
            clearTokens()
            navigate('/')
        } catch (e) {
            setError(errorMessage(e) || 'Delete failed')
        } finally {
            setBusy(false)
        }
    }

    return (
        <Card className="border-danger-fill/30">
            <CardHeader
                title={<span className="text-bad">Danger zone</span>}
                description="Download a copy of your data, or close your account for good."
            />
            <CardBody className="space-y-6">
                <div className="flex flex-wrap items-center justify-between gap-3">
                    <div>
                        <p className="text-sm font-medium text-ink">Export your data</p>
                        <p className="text-xs text-muted">
                            Every profile field, submission and rating entry we hold, as JSON.
                        </p>
                    </div>
                    <Button variant="secondary" onClick={exportData} disabled={exporting}>
                        <Download className="h-4 w-4" aria-hidden="true" />
                        {exporting ? 'Preparing…' : 'Download'}
                    </Button>
                </div>

                <div className="border-t border-hairline pt-5">
                    <p className="text-sm font-medium text-ink">Delete my account</p>
                    <p className="mt-1 text-xs text-muted">
                        Permanently deletes your account and personal data. Shared content you authored is
                        kept with authorship cleared where required. This cannot be undone.
                    </p>
                    <div className="mt-3 max-w-md space-y-3">
                        <Field
                            label="Current password"
                            htmlFor="delete-account-password"
                            hint="Leave blank if you signed up with OAuth only."
                        >
                            <Input
                                id="delete-account-password"
                                type="password"
                                value={password}
                                onChange={e => setPassword(e.target.value)}
                                autoComplete="current-password"
                            />
                        </Field>
                        <label className="flex items-start gap-2 text-sm text-ink-soft">
                            <input
                                type="checkbox"
                                checked={confirm}
                                onChange={e => setConfirm(e.target.checked)}
                                className="mt-0.5 h-4 w-4 shrink-0 rounded border-hairline-strong text-brand"
                            />
                            I understand this action is permanent and cannot be undone.
                        </label>
                        {error && <p role="alert" className="text-sm text-bad">{error}</p>}
                        <Button variant="danger" onClick={handleDelete} disabled={busy || !confirm}>
                            <Trash2 className="h-4 w-4" aria-hidden="true" />
                            {busy ? 'Deleting…' : 'Delete my account'}
                        </Button>
                    </div>
                </div>
            </CardBody>
        </Card>
    )
}

// ── Page ─────────────────────────────────────────────────────────────────────

type SectionKey = 'details' | 'security' | 'invites' | 'setter' | 'danger'

export default function Profile() {
    const location = useLocation()
    const [user] = useState(decodeUser)
    const [section, setSection] = useState<SectionKey>('details')
    const [profile, setProfile] = useState<UserProfile | null>(null)
    const [loading, setLoading] = useState(true)
    const [loadError, setLoadError] = useState('')
    const [invites, setInvites] = useState<{ teams: TeamInvite[]; groups: GroupInvite[] }>({ teams: [], groups: [] })
    const [invitesLoading, setInvitesLoading] = useState(true)

    const loadProfile = useCallback(async () => {
        // No setState before the first await: this runs from an effect, and a
        // synchronous state write there would cascade renders.
        try {
            const p = await api.users.getProfile()
            setProfile(p)
            setLoadError('')
        } catch (e) {
            setLoadError(errorMessage(e) || 'Could not load your profile')
        } finally {
            setLoading(false)
        }
    }, [])

    const loadInvites = useCallback(async () => {
        try {
            const d = await api.users.getPendingInvites()
            setInvites({ teams: d.teams || [], groups: d.groups || [] })
        } catch {
            // A failed invite fetch must not take the settings page with it.
        } finally {
            setInvitesLoading(false)
        }
    }, [])

    // Invites load with the page rather than on tab click, so the nav can show
    // how many are waiting — an invitation nobody opens is an invitation missed.
    useEffect(() => {
        if (!user) return
        // Deferred a microtask so the loads still land pre-paint without the
        // effect body itself setting state (react-hooks/set-state-in-effect).
        queueMicrotask(() => {
            void loadProfile()
            void loadInvites()
        })
    }, [user, loadProfile, loadInvites])

    // Signed-out state: it previously said "please log in" with no way to do
    // so, and had no page heading at all.
    if (!user) {
        return (
            <div className="mx-auto max-w-md py-20 text-center">
                <h1 className="text-2xl font-semibold tracking-tight text-ink">Profile</h1>
                <p className="mt-2 text-sm text-muted">You need to be signed in to view your profile.</p>
                <div className="mt-5 flex justify-center gap-2">
                    <Link to={loginHref(location)} className={buttonClass()}>Sign in</Link>
                    <Link to="/register" className={buttonClass({ variant: 'secondary' })}>Create an account</Link>
                </div>
            </div>
        )
    }

    const pendingInvites = invites.teams.length + invites.groups.length

    const sections: { key: SectionKey; label: string; icon: LucideIcon; count?: number }[] = [
        { key: 'details', label: 'Profile details', icon: UserRound },
        { key: 'security', label: 'Password & security', icon: KeyRound },
        { key: 'invites', label: 'Invitations', icon: Users, count: pendingInvites },
        ...(user.role === 'user'
            ? [{ key: 'setter' as SectionKey, label: 'Setter application', icon: Award }]
            : []),
        { key: 'danger', label: 'Danger zone', icon: ShieldAlert },
    ]

    const activeLabel = sections.find(s => s.key === section)?.label ?? 'Profile'

    const renderSection = () => {
        switch (section) {
            case 'details':
                if (loading) {
                    return (
                        <Card>
                            <CardHeader title="Profile details" />
                            <CardBody className="space-y-4">
                                <Skeleton className="h-9 w-full" />
                                <Skeleton className="h-9 w-full" />
                                <Skeleton className="h-24 w-full" />
                            </CardBody>
                        </Card>
                    )
                }
                if (!profile) {
                    return (
                        <Card>
                            <CardBody className="space-y-3">
                                <p role="alert" className="flex items-center gap-2 text-sm text-bad">
                                    <AlertCircle className="h-4 w-4" aria-hidden="true" />
                                    {loadError || 'Your profile could not be loaded.'}
                                </p>
                                <Button variant="secondary" onClick={() => { setLoading(true); void loadProfile() }}>
                                    Try again
                                </Button>
                            </CardBody>
                        </Card>
                    )
                }
                return <DetailsSection profile={profile} username={user.uname} onSaved={setProfile} />
            case 'security':
                return <SecuritySection />
            case 'invites':
                return (
                    <InvitesSection
                        userId={user.uid}
                        invites={invites}
                        loading={invitesLoading}
                        reload={loadInvites}
                    />
                )
            case 'setter':
                return (
                    <Card>
                        <CardHeader
                            title="Become a problem setter"
                            description="Write problems, add editorials and earn setter tools."
                        />
                        <CardBody>
                            <SetterApplication />
                        </CardBody>
                    </Card>
                )
            case 'danger':
                return <DangerSection />
        }
    }

    return (
        <div className="mx-auto max-w-5xl">
            <PageHeader
                eyebrow="Account"
                title="Profile"
                description="How you appear to everyone else on AIOJ."
                actions={
                    // "Public profile" lives on the identity card itself, next to
                    // the handle it refers to — one link, not two.
                    <Link
                        to="/settings/notifications"
                        className={buttonClass({ variant: 'secondary', size: 'sm' })}
                    >
                        Notification settings
                    </Link>
                }
            />

            <IdentityCard
                username={user.uname}
                role={user.role}
                profile={profile}
                loading={loading}
            />

            <div className="grid gap-6 lg:grid-cols-[14rem_minmax(0,1fr)]">
                {/* Section nav: a column on desktop, a scroll strip on mobile.
                    `min-w-0` matters: without it the nowrap strip becomes the
                    grid track's max-content width and pushes the form off the
                    right edge of a phone. */}
                <nav aria-label="Profile sections" className="min-w-0 lg:sticky lg:top-4 lg:self-start">
                    {/* Wraps rather than scrolls on a phone: a section hidden
                        past the right edge is a section nobody finds. */}
                    <ul className="flex flex-wrap gap-1 lg:flex-col lg:flex-nowrap">
                        {sections.map(item => {
                            const Icon = item.icon
                            const active = section === item.key
                            return (
                                <li key={item.key} className="lg:w-full">
                                    <button
                                        type="button"
                                        onClick={() => setSection(item.key)}
                                        aria-current={active ? 'page' : undefined}
                                        className={`flex items-center gap-2 whitespace-nowrap rounded-lg px-3 py-2 text-sm font-medium transition-colors lg:w-full ${
                                            active
                                                ? 'bg-brand-soft text-brand'
                                                : 'text-muted hover:bg-surface-2 hover:text-ink'
                                        }`}
                                    >
                                        <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
                                        {item.label}
                                        {item.count ? (
                                            <Badge tone="brand" className="lg:ml-auto">
                                                {item.count}
                                            </Badge>
                                        ) : null}
                                    </button>
                                </li>
                            )
                        })}
                    </ul>
                </nav>

                <section aria-label={activeLabel} className="min-w-0">
                    {renderSection()}
                </section>
            </div>
        </div>
    )
}
