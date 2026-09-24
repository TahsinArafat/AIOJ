import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { expect, test, vi, beforeEach } from 'vitest'
import Navbar from './Navbar'

vi.mock('../lib/api', () => ({
    api: {
        notifications: { unreadCount: vi.fn().mockResolvedValue({ count: 0 }) },
    },
    getAccessToken: vi.fn(() => 'mock-token'),
    setTokens: vi.fn(),
    clearTokens: vi.fn(),
}))

beforeEach(() => {
    vi.clearAllMocks()
})

test('renders brand logo and primary navigation links', () => {
    render(
        <MemoryRouter>
            <Navbar />
        </MemoryRouter>
    )

    // Verify brand link exists
    const brandElement = screen.getByText('AIOJ')
    expect(brandElement).toBeInTheDocument()
    expect(brandElement.getAttribute('href')).toBe('/')

    // Verify key navigation paths exist
    expect(screen.getByText('Problems')).toBeInTheDocument()
    expect(screen.getByText('Compete')).toBeInTheDocument()
    expect(screen.getByText('Community')).toBeInTheDocument()
})

test('mounts exactly one notification bell for a signed-in user', () => {
    // Regression: the bell used to render in both the desktop and mobile
    // clusters, so every 30s poll fired twice and two simultaneous 401s
    // triggered two competing token refreshes.
    render(
        <MemoryRouter>
            <Navbar />
        </MemoryRouter>
    )

    const bells = screen.getAllByRole('button', { name: /Notifications/ })
    expect(bells).toHaveLength(1)
})

test('gives the account dropdown an accessible name', () => {
    render(
        <MemoryRouter>
            <Navbar />
        </MemoryRouter>
    )

    const account = screen.getByRole('button', { name: 'Account menu' })
    expect(account).toHaveAttribute('aria-expanded', 'false')
})
