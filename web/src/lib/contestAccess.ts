/**
 * Per-surface access, mirroring `model.AccessConfig` on the server.
 *
 * A round is not one audience. An onsite final keeps its statements in the room
 * and puts its board on a projector; a hiring round may publish the problem list
 * as a teaser and hold the statements back; a school round may hand out a
 * printable booklet at the door while the statements stay closed in the browser.
 * Each of those is a real combination and none is expressible as one boolean,
 * which is why the setting is a map rather than a flag.
 *
 * The server is the authority. This file exists so the manage screen and the
 * anywhere-else-that-asks share one reading of the map: a second copy of the
 * inheritance rule is how "inherit" and "participants" quietly become different
 * things, and the resulting bug is a surface that looks closed in the form and is
 * open in the round.
 */

export type { AccessAudience, AccessSurface, AccessConfig } from '../types/api'
import type { AccessAudience, AccessSurface, AccessConfig } from '../types/api'

export interface SurfaceSpec {
    key: AccessSurface
    /** Row label in the manage screen's table. */
    label: string
    /** One line under the label: what this surface actually covers. */
    hint: string
    /**
     * Whether "staff" is a sensible choice for this surface. False for the two
     * that are part of competing — a round whose problems are visible only to its
     * organizers is a round nobody can enter, so offering the option would be
     * offering a footgun. The server would honour it; the form should not suggest
     * it.
     */
    allowsStaff: boolean
}

/**
 * The surfaces, in the order the manage screen lists them.
 *
 * `participate` is first because it is the one the others inherit from, and a
 * setter reading the table top to bottom meets the base rule before its
 * overrides. It is also the only row where "public" opens more than itself.
 */
export const SURFACES: SurfaceSpec[] = [
    {
        key: 'participate',
        label: 'Take part',
        hint: 'The round itself. Every surface below inherits this unless it has its own setting.',
        allowsStaff: false,
    },
    {
        key: 'problem_list',
        label: 'Problem list',
        hint: 'The list of problems, their letters, titles and solve counts.',
        allowsStaff: false,
    },
    {
        key: 'statement',
        label: 'Problem statements',
        hint: 'Reading an individual problem. This is what a competitor needs to solve it.',
        allowsStaff: false,
    },
    {
        key: 'pdf',
        label: 'Printable booklet',
        hint: 'The PDF of every problem. Useful for a printed, on-site round.',
        allowsStaff: true,
    },
    {
        key: 'submit',
        label: 'Submit solutions',
        hint: 'Sending an answer. Independent of reading, so a practice round can accept code from anyone.',
        allowsStaff: false,
    },
    {
        key: 'standings',
        label: 'Standings',
        hint: 'The scoreboard. Often published to spectators while the problems stay private.',
        allowsStaff: false,
    },
]

export const AUDIENCE_LABELS: Record<AccessAudience, string> = {
    participants: 'Participants',
    public: 'Anyone',
    staff: 'Organizers only',
}

/**
 * The effective audience for one surface — the client mirror of
 * `AccessConfig.Audience` on the server.
 *
 * An explicit entry wins; otherwise the surface inherits `participate`; otherwise
 * participants. One-way only: a public surface never widens its neighbours, which
 * is the rule that keeps "I published the scoreboard" from publishing the
 * statements.
 */
export function surfaceAudience(config: AccessConfig | undefined, surface: AccessSurface): AccessAudience {
    const explicit = config?.[surface]
    if (explicit) return explicit
    const base = config?.participate
    if (base) return base
    return 'participants'
}

/**
 * Whether a surface is showing an inherited value rather than one set on this
 * round. The form uses it to label the row "inherited" instead of pretending the
 * stored map names it — an inherited value that reads as a stored one is how a
 * setter ends up unable to tell which row they actually changed.
 */
export function isInherited(config: AccessConfig | undefined, surface: AccessSurface): boolean {
    if (surface === 'participate') return config?.participate == null
    return config?.[surface] == null
}

/** Drop an entry, which is how the form expresses "back to inheriting". */
export function clearSurface(config: AccessConfig | undefined, surface: AccessSurface): AccessConfig {
    const next: AccessConfig = { ...(config ?? {}) }
    delete next[surface]
    return next
}

/** Set one surface's audience. */
export function setSurface(
    config: AccessConfig | undefined,
    surface: AccessSurface,
    audience: AccessAudience,
): AccessConfig {
    return { ...(config ?? {}), [surface]: audience }
}

/**
 * The surfaces anyone may reach. Used to tell a setter, in one line, what this
 * round publishes — which is the question they are actually answering with this
 * table, and is not obvious from six independent rows.
 */
export function publicSurfaces(config: AccessConfig | undefined): SurfaceSpec[] {
    return SURFACES.filter((s) => s.key !== 'participate' && surfaceAudience(config, s.key) === 'public')
}

/** A copy with every invalid entry dropped, harmless to send back. */
export function sanitizeAccessConfig(config: AccessConfig | undefined): AccessConfig {
    const out: AccessConfig = {}
    for (const surface of SURFACES) {
        const audience = config?.[surface.key]
        if (audience === 'participants' || audience === 'public' || audience === 'staff') {
            out[surface.key] = audience
        }
    }
    return out
}
