import { expect, test } from 'vitest'
import { ArtifactScope, offeredForExisting, pruneBrief } from './aiArtifacts'
import type { AIArtifactSpec, AIBrief } from '../types/api'

/**
 * The scope questions the brief form asks, and the pruning that keeps a stored
 * brief honest about what the setter actually asked for.
 *
 * The catalog itself is server policy and is tested in Go
 * (internal/model/ai_artifact_test.go); this asserts the client's reading of it.
 */

const spec = (fields: AIArtifactSpec['fields']): AIArtifactSpec => ({
    id: 'editorial',
    label: 'Editorial',
    description: '',
    offered_for_existing: true,
    fields,
})

const BRIEF: AIBrief = {
    tags: ['dp'],
    rating: 1400,
    rating_min: 1300,
    rating_max: 1500,
    problem_type: 'standard',
    testcase_count: 10,
    has_subtasks: false,
    format_profile: 'codeforces',
    solver_mode: 'solo',
    event_context: 'practice',
    audience_level: 'advanced',
    story_tone: 'neutral',
    forbidden_techniques: ['flows'],
    additional_prompt: 'steer',
}

test('an editorial scope accepts voice but not the problem-defining inputs', () => {
    const s = new ArtifactScope(spec(['audience_level', 'story_tone', 'additional_prompt']))

    expect(s.accepts('audience_level')).toBe(true)
    expect(s.accepts('story_tone')).toBe(true)
    expect(s.accepts('topics')).toBe(false)
    expect(s.accepts('rating')).toBe(false)
    expect(s.accepts('testcase_count')).toBe(false)
})

test('only the artifact that accepts topics requires them', () => {
    expect(new ArtifactScope(spec(['topics', 'rating'])).requires('topics')).toBe(true)
    expect(new ArtifactScope(spec(['audience_level'])).requires('topics')).toBe(false)
    // A field that is accepted but is not topics is never required.
    expect(new ArtifactScope(spec(['topics', 'rating'])).requires('rating')).toBe(false)
})

test('acceptsAny composes a section from its fields', () => {
    const s = new ArtifactScope(spec(['testcase_count', 'has_subtasks']))
    expect(s.acceptsAny('topics', 'testcase_count')).toBe(true)
    expect(s.acceptsAny('topics', 'rating')).toBe(false)
})

// An absent catalog must not hide the form: rendering fewer controls than the
// server expects would silently drop a setter's input. The server is the judge.
test('an unknown scope accepts everything and requires nothing', () => {
    const s = new ArtifactScope(undefined)
    expect(s.accepts('topics')).toBe(true)
    expect(s.accepts('rating')).toBe(true)
    expect(s.requires('topics')).toBe(false)
    expect(s.acceptsAny('anything' as never)).toBe(true)
})

test('pruneBrief keeps only the fields the scope accepts', () => {
    const editorial = new ArtifactScope(spec(['audience_level', 'story_tone', 'additional_prompt']))
    const out = pruneBrief(BRIEF, editorial)

    expect(out.audience_level).toBe('advanced')
    expect(out.story_tone).toBe('neutral')
    expect(out.additional_prompt).toBe('steer')
    // The problem-defining inputs an editorial ignores are gone, so the stored
    // brief cannot claim the setter asked for a topic or a rating.
    expect(out.tags).toBeUndefined()
    expect(out.rating).toBeUndefined()
    expect(out.rating_min).toBeUndefined()
    expect(out.testcase_count).toBeUndefined()
    expect(out.format_profile).toBeUndefined()
})

// rating_min/rating_max map to the single 'rating_range' field, so a scope that
// takes a rating keeps the whole range and one that does not drops all three.
test('pruneBrief treats the rating range as one field', () => {
    const withRating = new ArtifactScope(spec(['rating', 'rating_range']))
    expect(pruneBrief(BRIEF, withRating).rating_min).toBe(1300)
    expect(pruneBrief(BRIEF, withRating).rating_max).toBe(1500)

    const noRating = new ArtifactScope(spec(['additional_prompt']))
    expect(pruneBrief(BRIEF, noRating).rating).toBeUndefined()
    expect(pruneBrief(BRIEF, noRating).rating_min).toBeUndefined()
    expect(pruneBrief(BRIEF, noRating).rating_max).toBeUndefined()
})

test('pruneBrief does not mutate the brief it was given', () => {
    const editorical = new ArtifactScope(spec(['audience_level']))
    pruneBrief(BRIEF, editorical)
    expect(BRIEF.tags).toEqual(['dp'])
    expect(BRIEF.rating).toBe(1400)
})

test('pruneBrief keeps the brief whole when the catalog has not loaded', () => {
    const out = pruneBrief(BRIEF, new ArtifactScope(undefined))
    expect(out).toEqual(BRIEF)
})

// A field with no mapping in the table is kept: a new brief field ships until it
// is deliberately scoped, rather than being silently dropped.
test('pruneBrief keeps a field the mapping does not know', () => {
    const brief = { ...BRIEF, future_field: 'keep me' } as AIBrief
    const out = pruneBrief(brief, new ArtifactScope(spec(['audience_level'])))
    expect((out as unknown as Record<string, unknown>).future_field).toBe('keep me')
})

test('offeredForExisting hides the whole-package scope', () => {
    const specs: AIArtifactSpec[] = [
        { ...spec(['topics']), id: 'full_problem', offered_for_existing: false },
        { ...spec(['topics']), id: 'statement', offered_for_existing: true },
    ]
    expect(offeredForExisting(specs).map(s => s.id)).toEqual(['statement'])
})
