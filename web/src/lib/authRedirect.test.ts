import { expect, test } from 'vitest'
import { loginHref, nextParam, nextPath } from './authRedirect'

test('loginHref carries the current path and search', () => {
    expect(loginHref({ pathname: '/problems', search: '?difficulty=hard' }))
        .toBe('/login?next=%2Fproblems%3Fdifficulty%3Dhard')
})

test('loginHref omits next on the home page', () => {
    expect(loginHref({ pathname: '/', search: '' })).toBe('/login')
})

test('loginHref never targets an auth page', () => {
    for (const pathname of [
        '/login',
        '/register',
        '/forgot-password',
        '/reset-password',
        '/verify-email',
        '/oauth/complete',
    ]) {
        expect(loginHref({ pathname, search: '' })).toBe('/login')
    }
})

test('nextParam reads a valid same-origin path', () => {
    expect(nextParam(new URLSearchParams('next=%2Fproblems'))).toBe('/problems')
    expect(nextParam(new URLSearchParams('next=%2Fproblems%3Fx%3D1'))).toBe('/problems?x=1')
})

test('nextParam refuses an open redirect', () => {
    for (const bad of ['https://evil.example', '//evil.example', '/\\evil.example', 'evil.example']) {
        expect(nextParam(new URLSearchParams(`next=${encodeURIComponent(bad)}`))).toBeNull()
    }
    expect(nextParam(new URLSearchParams())).toBeNull()
})

test('nextParam refuses an auth page that would loop', () => {
    for (const bad of ['/login', '/register', '/oauth/complete']) {
        expect(nextParam(new URLSearchParams(`next=${encodeURIComponent(bad)}`))).toBeNull()
    }
    expect(nextParam(new URLSearchParams('next=%2Flogin%3Fx%3D1'))).toBeNull()
})

test('nextPath falls back to the home page', () => {
    expect(nextPath(new URLSearchParams())).toBe('/')
    expect(nextPath(new URLSearchParams('next=https%3A%2F%2Fevil.example'))).toBe('/')
})
