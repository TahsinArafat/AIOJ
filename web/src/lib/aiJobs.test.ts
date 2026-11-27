import { expect, test } from 'vitest'
import { ARTIFACT_SECTIONS } from './aiJobs'

/**
 * Lock ARTIFACT_SECTIONS to the backend apply contract.
 *
 * The section keys here must match the `selected[...]` reads in
 * internal/generate/apply.go and the ws/wsol/we/wt flags in
 * internal/generate/proposal.go. A section offered by the UI that apply.go does
 * not read is silently accepted and written as nothing — the editorial bug. If
 * you change one side, change the other and update this test.
 */
test('artifact sections match the apply contract', () => {
    expect(ARTIFACT_SECTIONS).toEqual({
        full_problem: ['statement', 'solution', 'tests', 'editorial'],
        statement: ['statement'],
        tests: ['tests'],
        solution: ['solution'],
        editorial: ['editorial'],
    })
})