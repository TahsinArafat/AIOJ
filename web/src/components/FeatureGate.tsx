import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { Lock, LogIn, ShieldAlert, Star } from 'lucide-react'
import { useFeature } from '../context/FeaturesContext'
import { buttonClass } from './ui/buttonStyles'
import type { ResolvedFeature } from '../types/api'

/**
 * Renders `children` unless the admin has gated this feature away from the
 * current caller.
 *
 * This is a UI convenience, not the security boundary: the matching API routes
 * are gated server-side (see internal/feature). Hiding the entry point keeps
 * navigation honest and gives the user a reason instead of an API error.
 */
export function FeatureGate({ feature, children, fallback }: {
    feature: string
    children: ReactNode
    fallback?: ReactNode
}) {
    const { allowed, feature: resolved } = useFeature(feature)

    if (allowed) return <>{children}</>
    if (fallback) return <>{fallback}</>
    return <FeatureUnavailable feature={resolved} featureKey={feature} />
}

const REASON_COPY: Record<string, { title: string; body: (label: string) => string; icon: typeof Lock }> = {
    disabled: {
        title: 'Temporarily unavailable',
        body: label => `${label} is switched off by the site administrators right now.`,
        icon: Lock,
    },
    login_required: {
        title: 'Sign in to continue',
        body: label => `${label} is only available to signed-in accounts.`,
        icon: LogIn,
    },
    role: {
        title: 'Not available for your account',
        body: label => `Your account does not have access to ${label.toLowerCase()}.`,
        icon: ShieldAlert,
    },
    rating: {
        title: 'Rating requirement not met',
        body: label => `${label} is limited to a rating range your account is outside of.`,
        icon: Star,
    },
}

/**
 * The page shown in place of a gated route.
 *
 * Every message names the feature and, when the admin wrote one, shows their
 * explanation — a bare "403" leaves people guessing whether the site is broken.
 */
export function FeatureUnavailable({ feature, featureKey }: { feature?: ResolvedFeature; featureKey: string }) {
    const reason = feature?.reason ?? 'disabled'
    const copy = REASON_COPY[reason] ?? REASON_COPY.disabled
    const label = feature?.label ?? featureKey
    const Icon = copy.icon

    const policy = feature?.policy
    const ratingHint =
        reason === 'rating' && policy
            ? policy.min_rating && policy.max_rating
                ? `Requires a rating between ${policy.min_rating} and ${policy.max_rating}.`
                : policy.min_rating
                    ? `Requires a rating of at least ${policy.min_rating}.`
                    : policy.max_rating
                        ? `Available below a rating of ${policy.max_rating}.`
                        : undefined
            : undefined

    return (
        <div className="mx-auto flex max-w-md flex-col items-center px-4 py-16 text-center">
            <span className="mb-3 flex h-11 w-11 items-center justify-center rounded-xl bg-surface-2 text-muted">
                <Icon className="h-5 w-5" />
            </span>
            <h1 className="text-lg font-semibold text-ink">{copy.title}</h1>
            <p className="mt-2 text-sm leading-relaxed text-muted">{copy.body(label)}</p>

            {ratingHint && <p className="mt-1 text-sm text-muted">{ratingHint}</p>}

            {/* The admin's own explanation, when they configured one. */}
            {feature?.message && (
                <p className="mt-3 rounded-lg border border-hairline bg-surface-2 px-3 py-2 text-sm text-ink-soft">
                    {feature.message}
                </p>
            )}

            <div className="mt-6 flex flex-wrap items-center justify-center gap-2">
                {reason === 'login_required' ? (
                    <Link to="/login" className={buttonClass({ size: 'sm' })}>Sign in</Link>
                ) : null}
                <Link to="/" className={buttonClass({ variant: 'secondary', size: 'sm' })}>Go home</Link>
                <Link to="/problems" className={buttonClass({ variant: 'secondary', size: 'sm' })}>Browse problems</Link>
            </div>
        </div>
    )
}

export default FeatureGate
