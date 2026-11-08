import { renderHook } from '@testing-library/react'
import { expect, test, beforeEach } from 'vitest'
import { formatTitle, titleForPath, useDocumentTitle } from './documentTitle'

beforeEach(() => {
    document.title = 'AIOJ — Online Judge'
})

test('formatTitle appends the site name', () => {
    expect(formatTitle('Problems')).toBe('Problems — AIOJ')
})

test('formatTitle keeps the fuller home title for the bare site name', () => {
    expect(formatTitle('AIOJ')).toBe('AIOJ — Online Judge')
})

test('formatTitle trims instead of emitting a dangling separator', () => {
    expect(formatTitle('   ')).toBe('AIOJ — Online Judge')
})

// The regression this whole module exists for: every route used to keep the
// static index.html title, so a tab could not tell two pages apart.
test('titleForPath names each top-level page', () => {
    const cases: [string, string][] = [
        ['/', 'Home'],
        ['/problems', 'Problems'],
        ['/contests', 'Contests'],
        ['/blog', 'Blog'],
        ['/rankings', 'Rankings'],
        ['/admin', 'Admin'],
        ['/settings', 'Settings'],
        ['/login', 'Sign in'],
    ]
    for (const [path, want] of cases) {
        expect(titleForPath(path)).toBe(want)
    }
})

test('titleForPath distinguishes detail routes from their collections', () => {
    expect(titleForPath('/problems/two-sum')).toBe('Problem')
    expect(titleForPath('/contests/weekly-42')).toBe('Contest')
    expect(titleForPath('/contests/weekly-42/scoreboard')).toBe('Scoreboard')
    expect(titleForPath('/blog/12')).toBe('Blog post')
    expect(titleForPath('/user/alice')).toBe('User')
})

test('titleForPath prefers the more specific rule', () => {
    // /contests/:id/plagiarism must not fall through to the /contests/:id rule.
    expect(titleForPath('/contests/9/plagiarism')).toBe('Plagiarism')
    // /setter/ai/jobs/:id must not be swallowed by /setter/:slug.
    expect(titleForPath('/setter/ai/jobs/abc')).toBe('AI generation job')
    expect(titleForPath('/setter/ai')).toBe('AI problem generation')
})

test('titleForPath tolerates a trailing slash', () => {
    expect(titleForPath('/problems/')).toBe('Problems')
    expect(titleForPath('/contests/')).toBe('Contests')
})

test('titleForPath reports unknown paths as not found', () => {
    expect(titleForPath('/nope/nope')).toBe('Page not found')
})

test('useDocumentTitle writes the suffixed title', () => {
    renderHook(() => useDocumentTitle('Two Sum'))
    expect(document.title).toBe('Two Sum — AIOJ')
})

// Detail pages call the hook with not-yet-loaded data; blanking the tab during
// the fetch would be worse than briefly showing the route-level title.
test('useDocumentTitle ignores an empty title', () => {
    renderHook(() => useDocumentTitle(undefined))
    expect(document.title).toBe('AIOJ — Online Judge')

    renderHook(() => useDocumentTitle('   '))
    expect(document.title).toBe('AIOJ — Online Judge')
})
