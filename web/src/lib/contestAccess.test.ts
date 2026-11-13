import { describe, it, expect } from 'vitest'
import {
    SURFACES,
    surfaceAudience,
    isInherited,
    setSurface,
    clearSurface,
    publicSurfaces,
    sanitizeAccessConfig,
} from './contestAccess'
import type { AccessConfig } from './contestAccess'

/**
 * The client mirror of the server's inheritance rule.
 *
 * This is worth testing rather than trusting because the failure is invisible: if
 * this file and `model.AccessConfig.Audience` disagree, the manage screen shows a
 * surface as closed while the round serves it open (or the reverse). Nothing
 * errors, no request fails, and the setter has no way to tell which of the two
 * they are looking at. So the rules are pinned here in the same order the server
 * applies them.
 */
describe('surfaceAudience', () => {
    it('resolves every surface to open for a round with no config', () => {
        // open, not participants. This is the compatibility guarantee: every round
        // that predates the vocabulary has an empty map, and empty has to mean what
        // those rounds actually did — the password and group rules and nothing
        // more. Answering participants here would show every existing round as
        // entrants-only while the server served it to everyone.
        for (const s of SURFACES) {
            expect(surfaceAudience(undefined, s.key)).toBe('open')
            expect(surfaceAudience({}, s.key)).toBe('open')
        }
    })

    it('lets an explicit entry win', () => {
        expect(surfaceAudience({ standings: 'public' }, 'standings')).toBe('public')
        expect(surfaceAudience({ statement: 'staff' }, 'statement')).toBe('staff')
    })

    it('inherits the base surface when a surface has no entry', () => {
        expect(surfaceAudience({ participate: 'public' }, 'standings')).toBe('public')
        expect(surfaceAudience({ participate: 'public' }, 'statement')).toBe('public')
    })

    it('does not let a public surface widen its neighbours', () => {
        // The rule that keeps "I published the scoreboard" from publishing the
        // statements. This is the client half of a security invariant.
        const config: AccessConfig = { standings: 'public' }
        expect(surfaceAudience(config, 'statement')).toBe('open')
        // submit reads as open even though the base would say participants — the
        // coercion, not a second default.
        expect(surfaceAudience(config, 'submit')).toBe('open')
        expect(surfaceAudience(config, 'pdf')).toBe('open')
        expect(surfaceAudience(config, 'problem_list')).toBe('open')
        expect(surfaceAudience(config, 'participate')).toBe('open')
    })

    it('lets an explicit entry override an inherited base in both directions', () => {
        // Closed despite a public base.
        expect(surfaceAudience({ participate: 'public', statement: 'staff' }, 'statement')).toBe('staff')
        // Open despite a participants base.
        expect(surfaceAudience({ statement: 'public' }, 'statement')).toBe('public')
    })

    it('coerces participants on submit, by inheritance as well as directly', () => {
        // The deadlock the server refuses with a 400: requiring an entrant in order
        // to submit blocks the first submission of every round, because submitting
        // is one of the two ways to become one. Direct first...
        expect(surfaceAudience({ submit: 'participants' }, 'submit')).toBe('open')
        // ...then the subtle half: nobody wrote anything on submit here, it arrives
        // from the base. The form must not show submit as restricted while the
        // server treats it as open.
        expect(surfaceAudience({ participate: 'participants' }, 'submit')).toBe('open')
        // And a genuine participants restriction elsewhere is untouched.
        expect(surfaceAudience({ participate: 'participants' }, 'statement')).toBe('participants')
    })
})

describe('isInherited', () => {
    it('reports a surface with no entry of its own as inherited', () => {
        expect(isInherited({}, 'standings')).toBe(true)
        expect(isInherited({ participate: 'public' }, 'standings')).toBe(true)
    })

    it('reports a surface with its own entry as not inherited', () => {
        expect(isInherited({ standings: 'public' }, 'standings')).toBe(false)
        // Even when the stored value matches what it would have inherited: the
        // row has been deliberately set, and the form must not label it inherited.
        expect(isInherited({ participate: 'public', standings: 'public' }, 'standings')).toBe(false)
    })

    it('treats the base surface itself as inherited only when unset', () => {
        expect(isInherited({}, 'participate')).toBe(true)
        expect(isInherited({ participate: 'participants' }, 'participate')).toBe(false)
    })
})

describe('setSurface and clearSurface', () => {
    it('does not mutate the input', () => {
        const config: AccessConfig = { standings: 'public' }
        const set = setSurface(config, 'statement', 'public')
        const cleared = clearSurface(config, 'standings')

        expect(config).toEqual({ standings: 'public' })
        expect(set).toEqual({ standings: 'public', statement: 'public' })
        expect(cleared).toEqual({})
    })

    it('clears by removing the key rather than storing the inherited value', () => {
        // Storing "participants" would look identical in the UI but would stop the
        // surface following a later change to the base — which is the whole point
        // of an override being absent rather than equal.
        const cleared = clearSurface({ participate: 'public', standings: 'participants' }, 'standings')
        expect(cleared).toEqual({ participate: 'public' })
        expect('standings' in cleared).toBe(false)
        // And it now follows the base, which is public.
        expect(surfaceAudience(cleared, 'standings')).toBe('public')
    })

    it('builds from undefined', () => {
        expect(setSurface(undefined, 'standings', 'public')).toEqual({ standings: 'public' })
        expect(clearSurface(undefined, 'standings')).toEqual({})
    })
})

describe('publicSurfaces', () => {
    it('is empty for a round that publishes nothing', () => {
        expect(publicSurfaces(undefined)).toEqual([])
        expect(publicSurfaces({})).toEqual([])
    })

    it('lists the surfaces anyone can reach, excluding the base', () => {
        const config: AccessConfig = { standings: 'public', pdf: 'public' }
        expect(publicSurfaces(config).map((s) => s.key)).toEqual(['pdf', 'standings'])
    })

    it('follows inheritance from a public base', () => {
        // A public base opens every surface that has no override, so they are all
        // public surfaces — except any the setter explicitly closed.
        const config: AccessConfig = { participate: 'public', statement: 'staff' }
        const keys = publicSurfaces(config).map((s) => s.key)
        expect(keys).not.toContain('statement')
        expect(keys).toContain('standings')
        expect(keys).toContain('submit')
        // The base itself is never listed: it is the round's gate, not a surface a
        // stranger is told about.
        expect(keys).not.toContain('participate')
    })
})

describe('sanitizeAccessConfig', () => {
    it('drops unknown audiences and leaves valid ones', () => {
        const dirty = {
            standings: 'public',
            statement: 'everyone', // not an audience
        } as unknown as AccessConfig
        expect(sanitizeAccessConfig(dirty)).toEqual({ standings: 'public' })
    })

    it('keeps open', () => {
        // Not dropped as redundant. open is the only way to opt one surface out of
        // a participants base, so dropping it would silently re-restrict the
        // surface the setter was opting out of.
        const config: AccessConfig = { participate: 'participants', statement: 'open' }
        expect(sanitizeAccessConfig(config)).toEqual({ participate: 'participants', statement: 'open' })
    })

    it('coerces participants on submit to open', () => {
        // Mirroring the server's only coercion, so the form cannot send a
        // combination validateAccessConfig answers with a 400.
        expect(sanitizeAccessConfig({ submit: 'participants' })).toEqual({ submit: 'open' })
    })

    it('returns a fresh object so a caller cannot mutate the source', () => {
        const config: AccessConfig = { standings: 'public' }
        const clean = sanitizeAccessConfig(config)
        expect(clean).toEqual(config)
        expect(clean).not.toBe(config)
    })
})

describe('offerable audiences', () => {
    it('does not offer participants on submit', () => {
        // The form must not offer a value the server refuses: requiring an entrant
        // to submit deadlocks the first submission of every round.
        const submit = SURFACES.find((s) => s.key === 'submit')
        expect(submit?.allowsParticipants).toBe(false)
    })

    it('offers participants everywhere it is coherent', () => {
        for (const s of SURFACES) {
            if (s.key === 'submit') continue
            expect(s.allowsParticipants).toBe(true)
        }
    })
})
