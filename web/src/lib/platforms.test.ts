import { expect, test } from 'vitest'
import { isRemotePlatform, PLATFORM_LABEL } from './platforms'

/**
 * "Remote" is an allowlist. Deriving it from `source !== 'local'` put AI
 * generated problems (source "ai") into the remote pipeline, where they asked
 * for a platform that does not exist and came back as a System Error.
 */
test('only real remote platforms count as remote', () => {
    for (const p of ['codeforces', 'atcoder', 'cses', 'toph', 'qoj']) {
        expect(isRemotePlatform(p)).toBe(true)
    }
})

test('locally judged sources are not remote', () => {
    for (const p of ['local', 'ai', '', undefined, null]) {
        expect(isRemotePlatform(p)).toBe(false)
    }
})

test('every known source has a label', () => {
    expect(PLATFORM_LABEL.local).toBe('AIOJ')
    expect(PLATFORM_LABEL.ai).toBe('AI')
    expect(PLATFORM_LABEL.codeforces).toBe('Codeforces')
})
