import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { expect, test, vi, beforeEach } from 'vitest'
import { FeatureGate } from './FeatureGate'
import { FeaturesProvider } from '../context/FeaturesContext'
import { api } from '../lib/api'
import type { ResolvedFeature } from '../types/api'

vi.mock('../lib/api', async (importOriginal) => {
    const actual = await importOriginal<typeof import('../lib/api')>()
    return {
        ...actual,
        api: {
            ...actual.api,
            features: { list: vi.fn() },
        },
    }
})

function feature(overrides: Partial<ResolvedFeature>): ResolvedFeature {
    return {
        key: 'gym',
        label: 'Gym',
        description: 'Practice contests',
        category: 'practice',
        default_policy: { enabled: true },
        requires_login: false,
        api_enforced: true,
        policy: { enabled: true },
        allowed: true,
        ...overrides,
    }
}

function renderGate(children: React.ReactNode) {
    return render(
        <MemoryRouter>
            <FeaturesProvider>
                <FeatureGate feature="gym">{children}</FeatureGate>
            </FeaturesProvider>
        </MemoryRouter>
    )
}

beforeEach(() => {
    vi.mocked(api.features.list).mockReset()
})

test('renders the children when the feature is allowed', async () => {
    vi.mocked(api.features.list).mockResolvedValue({ data: [feature({})] })

    renderGate(<p>gym page</p>)

    expect(await screen.findByText('gym page')).toBeTruthy()
})

test('replaces the page with an explanation when the feature is disabled', async () => {
    vi.mocked(api.features.list).mockResolvedValue({
        data: [feature({
            allowed: false,
            reason: 'disabled',
            policy: { enabled: false, message: 'Back after the round on Sunday.' },
            message: 'Back after the round on Sunday.',
        })],
    })

    renderGate(<p>gym page</p>)

    expect(await screen.findByText('Temporarily unavailable')).toBeTruthy()
    // The admin's own explanation is the useful part; a bare "unavailable"
    // leaves people guessing.
    expect(screen.getByText('Back after the round on Sunday.')).toBeTruthy()
    expect(screen.queryByText('gym page')).toBeNull()
})

test('asks an anonymous visitor to sign in when the gate needs an account', async () => {
    vi.mocked(api.features.list).mockResolvedValue({
        data: [feature({ allowed: false, reason: 'login_required', policy: { enabled: true, roles: ['user'] } })],
    })

    renderGate(<p>gym page</p>)

    expect(await screen.findByText('Sign in to continue')).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Sign in' })).toBeTruthy()
})

test('spells out the rating requirement', async () => {
    vi.mocked(api.features.list).mockResolvedValue({
        data: [feature({
            allowed: false,
            reason: 'rating',
            policy: { enabled: true, min_rating: 1600, max_rating: 0 },
        })],
    })

    renderGate(<p>gym page</p>)

    expect(await screen.findByText('Rating requirement not met')).toBeTruthy()
    expect(screen.getByText('Requires a rating of at least 1600.')).toBeTruthy()
})

test('fails open when the feature list cannot be loaded', async () => {
    // The API still enforces the policy; hiding the whole page because one
    // optional endpoint failed would be worse than a clear 403 later.
    vi.mocked(api.features.list).mockRejectedValue(new Error('network down'))

    renderGate(<p>gym page</p>)

    await waitFor(() => expect(screen.getByText('gym page')).toBeTruthy())
})
