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
    it('defaults every surface to participants for a round with no config', () => {
        for (const s of SURFACES) {
            expect(surfaceAudience(undefined, s.key)).toBe('participants')
            expect(surfaceAudience({}, s.key)).toBe('participants')
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
        expect(surfaceAudience(config, 'statement')).toBe('participants')
        expect(surfaceAudience(config, 'submit')).toBe('participants')
        expect(surfaceAudience(config, 'pdf')).toBe('participants')
        expect(surfaceAudience(config, 'problem_list')).toBe('participants')
        expect(surfaceAudience(config, 'participate')).toBe('participants')
    })

    it('lets an explicit entry override an inherited base in both directions', () => {
        // Closed despite a public base.
        expect(surfaceAudience({ participate: 'public', statement: 'staff' }, 'statement')).toBe('staff')
        // Open despite a participants base.
        expect(surfaceAudience({ statement: 'public' }, 'statement')).toBe('public')
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

    it('returns a fresh object so a caller cannot mutate the source', () => {
        const config: AccessConfig = { standings: 'public' }
        const clean = sanitizeAccessConfig(config)
        expect(clean).toEqual(config)
        expect(clean).not.toBe(config)
    })
})
