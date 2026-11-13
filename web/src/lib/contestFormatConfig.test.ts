import { describe, it, expect } from 'vitest'
import {
    FORMAT_SPECS,
    formatSpec,
    defaultFormatConfig,
    formFromFormatConfig,
    configFromForm,
} from './contestFormatConfig'

/**
 * The client's format settings have to be the server's format settings.
 *
 * These keys are the JSON tags on each format's Config struct in
 * internal/contest/format/<name>/. If the client sends a key the server does not
 * read, nothing errors: the config is stored, the extra key is ignored, and the
 * round scores with the server's zero value for the setting the setter thought
 * they changed. That is the failure mode worth a test — it is invisible from both
 * ends.
 *
 * The expected keys are written out here rather than read from the Go source,
 * because a test that parses Go is a test that passes when both sides are renamed
 * together. Each entry is a deliberate copy of one struct's JSON tags.
 */
describe('format config keys match the server', () => {
    // Copied from the Config structs. Change these when the Go structs change, and
    // this test is what tells you the client needs to follow.
    const SERVER_KEYS: Record<string, string[]> = {
        acm: ['penalty_per_wrong', 'time_penalty'],
        oi: ['max_score_per_problem'],
        ioi: ['partial_credit', 'subtask_scoring'],
        atcoder: ['penalty_is_time_of_ac', 'no_wrong_attempt_penalty'],
        codeforces: ['initial_scores', 'decay_factor', 'min_score_ratio', 'wrong_submission_penalty'],
    }

    it('declares exactly the keys each server struct reads', () => {
        for (const [id, keys] of Object.entries(SERVER_KEYS)) {
            const spec = formatSpec(id)
            expect(spec, `no client spec for format ${id}`).toBeDefined()
            const mine = spec!.fields.map((f) => f.key).sort()
            expect(mine, `keys for ${id}`).toEqual([...keys].sort())
        }
    })

    it('covers every format the create page used to offer', () => {
        // The create page listed exactly these five. A format that exists on the
        // server but has no spec here would be selectable and untunable.
        expect(FORMAT_SPECS.map((f) => f.id).sort()).toEqual(Object.keys(SERVER_KEYS).sort())
    })
})

describe('defaultFormatConfig', () => {
    it('matches the server defaults', () => {
        // Copied from each format's DefaultConfig().
        expect(defaultFormatConfig('acm')).toEqual({ penalty_per_wrong: 20, time_penalty: true })
        expect(defaultFormatConfig('oi')).toEqual({ max_score_per_problem: 100 })
        expect(defaultFormatConfig('ioi')).toEqual({ partial_credit: true, subtask_scoring: true })
        expect(defaultFormatConfig('atcoder')).toEqual({
            penalty_is_time_of_ac: true,
            no_wrong_attempt_penalty: true,
        })
        expect(defaultFormatConfig('codeforces')).toEqual({
            initial_scores: [500, 1000, 1500, 2000, 2500],
            decay_factor: 250,
            min_score_ratio: 0.3,
            wrong_submission_penalty: 50,
        })
    })

    it('is empty for an unknown format', () => {
        expect(defaultFormatConfig('nonsense')).toEqual({})
    })
})

describe('form round trip', () => {
    it('preserves every value through a render and a save', () => {
        // The path that matters: a stored config is loaded into the form, the setter
        // saves without touching it, and the same config goes back. A lost key here
        // is a setting silently reset by opening the page.
        for (const spec of FORMAT_SPECS) {
            const stored = defaultFormatConfig(spec.id)
            const form = formFromFormatConfig(spec.id, stored)
            expect(configFromForm(spec.id, form), `round trip for ${spec.id}`).toEqual(stored)
        }
    })

    it('keeps codeforces initial_scores as a list', () => {
        const stored = defaultFormatConfig('codeforces')
        const form = formFromFormatConfig('codeforces', stored)
        // Rendered as one comma-separated text value...
        expect(form.initial_scores).toBe('500, 1000, 1500, 2000, 2500')
        // ...and split back into the array the struct expects.
        expect(configFromForm('codeforces', form).initial_scores).toEqual([500, 1000, 1500, 2000, 2500])
    })

    it('tolerates a stored config from an older build', () => {
        // A partial config is normal: an older build may not have written every key,
        // and a hand-edited row may have dropped one. Missing keys must come back as
        // empty form fields, not as undefined or as a fabricated default.
        const form = formFromFormatConfig('acm', { penalty_per_wrong: 30 })
        expect(form.penalty_per_wrong).toBe('30')
        expect(form.time_penalty).toBe('false')
    })

    it('tolerates a null or non-object config', () => {
        // The DTO types format_config as `unknown`, so this really can arrive.
        expect(formFromFormatConfig('acm', null).penalty_per_wrong).toBe('')
        expect(formFromFormatConfig('acm', 'nonsense').penalty_per_wrong).toBe('')
        expect(formFromFormatConfig('acm', undefined).penalty_per_wrong).toBe('')
    })

    it('skips an empty number rather than storing zero', () => {
        // An emptied box means "I have not set this", and storing 0 would be a real
        // value: a zero penalty is a legitimate setting, so the two must not collapse.
        const form = formFromFormatConfig('acm', defaultFormatConfig('acm'))
        form.penalty_per_wrong = ''
        expect(configFromForm('acm', form)).toEqual({ time_penalty: true })
    })

    it('omits an unknown format from the config', () => {
        expect(configFromForm('nonsense', { whatever: '1' })).toEqual({})
    })
})