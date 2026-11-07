import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { expect, test, vi, beforeEach } from 'vitest'
import Navbar from './Navbar'
import { FeaturesProvider } from '../context/FeaturesContext'
import { api } from '../lib/api'
import type { ResolvedFeature } from '../types/api'

vi.mock('../lib/api', () => ({
    api: {
        notifications: { unreadCount: vi.fn().mockResolvedValue({ count: 0 }) },
        features: { list: vi.fn() },
    },
    getAccessToken: vi.fn(() => 'mock-token'),
    setTokens: vi.fn(),
    clearTokens: vi.fn(),
    AUTH_CHANGE_EVENT: 'aioj:auth-change',
}))

function feature(key: string, allowed: boolean): ResolvedFeature {
    return {
        key,
        label: key,
        description: '',
        category: 'core',
        default_policy: { enabled: true },
        requires_login: false,
        api_enforced: true,
        policy: { enabled: allowed },
        allowed,
        reason: allowed ? undefined : 'disabled',
    }
}

function renderNavbar() {
    return render(
        <MemoryRouter>
            <FeaturesProvider>
                <Navbar />
            </FeaturesProvider>
        </MemoryRouter>
    )
}

beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.features.list).mockResolvedValue({ data: [] })
})

test('renders brand logo and primary navigation links', async () => {
    renderNavbar()

    // Verify brand link exists
    const brandElement = screen.getByText('AIOJ')
    expect(brandElement).toBeInTheDocument()
    expect(brandElement.getAttribute('href')).toBe('/')

    // Verify key navigation paths exist
    await waitFor(() => expect(screen.getByText('Problems')).toBeInTheDocument())
    expect(screen.getByText('Compete')).toBeInTheDocument()
    expect(screen.getByText('Community')).toBeInTheDocument()
})

test('mounts exactly one notification bell for a signed-in user', async () => {
    // Regression: the bell used to render in both the desktop and mobile
    // clusters, so every 30s poll fired twice and two simultaneous 401s
    // triggered two competing token refreshes.
    renderNavbar()
    // Let the features fetch settle so the assertion below does not race a
    // state update outside act().
    await waitFor(() => expect(api.features.list).toHaveBeenCalled())

    const bells = screen.getAllByRole('button', { name: /Notifications/ })
    expect(bells).toHaveLength(1)
})

test('gives the account dropdown an accessible name', async () => {
    renderNavbar()
    await waitFor(() => expect(api.features.list).toHaveBeenCalled())

    const account = screen.getByRole('button', { name: 'Account menu' })
    expect(account).toHaveAttribute('aria-expanded', 'false')
})

test('hides navigation for a feature the admin has disabled', async () => {
    vi.mocked(api.features.list).mockResolvedValue({
        data: [feature('ide', false), feature('problems', false)],
    })

    renderNavbar()

    // Both are links in the desktop nav, so they must disappear once the gate
    // reports them as denied.
    await waitFor(() => expect(screen.queryByText('IDE')).toBeNull())
    expect(screen.queryByText('Problems')).toBeNull()
    // Untouched features stay visible.
    expect(screen.getByText('Compete')).toBeInTheDocument()
})

test('drops a dropdown when every entry inside it is gated away', async () => {
    vi.mocked(api.features.list).mockResolvedValue({
        data: [
            feature('organizations', false),
            feature('groups', false),
            feature('teams', false),
            feature('blog', false),
            feature('training', false),
            feature('rankings', false),
        ],
    })

    renderNavbar()

    await waitFor(() => expect(screen.queryByText('Community')).toBeNull())
    expect(screen.getByText('Compete')).toBeInTheDocument()
})
