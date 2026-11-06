import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { expect, test, vi, beforeEach } from 'vitest'
import Profile from './Profile'
import { ToastProvider } from '../components/Toast'
import { api, getAccessToken } from '../lib/api'

vi.mock('../lib/api', async (importOriginal) => {
    const actual = await importOriginal<typeof import('../lib/api')>()
    return {
        ...actual,
        getAccessToken: vi.fn(),
        api: {
            ...actual.api,
            users: {
                ...actual.api.users,
                getPendingInvites: vi.fn(),
            },
            teams: {
                ...actual.api.teams,
                respond: vi.fn(),
            },
            groups: {
                ...actual.api.groups,
                respond: vi.fn(),
            },
        },
    }
})

// A JWT-shaped token whose payload decodes to a user (atob of the middle segment).
function makeToken(payload: Record<string, unknown>): string {
    return `header.${btoa(JSON.stringify(payload))}.signature`
}

function renderProfile() {
    return render(
        <MemoryRouter>
            <ToastProvider>
                <Profile />
            </ToastProvider>
        </MemoryRouter>
    )
}

beforeEach(() => {
    vi.mocked(getAccessToken).mockReturnValue(
        makeToken({ uid: 'u1', uname: 'alice', email: 'alice@example.com', role: 'user' })
    )
    vi.mocked(api.users.getPendingInvites).mockReset()
})

test('shows a login prompt when there is no valid token', () => {
    vi.mocked(getAccessToken).mockReturnValue(null)

    renderProfile()

    // The signed-out state now carries a heading and real actions instead of a
    // dead-end sentence, so assert on what the user can actually do.
    expect(screen.getByRole('heading', { name: 'Profile' })).toBeInTheDocument()
    expect(screen.getByText('You need to be signed in to view your profile.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Sign in' })).toHaveAttribute('href', '/login')
})

test('loads pending invites when the tab is opened', async () => {
    vi.mocked(api.users.getPendingInvites).mockResolvedValue({
        teams: [{ team_id: 't1', team_name: 'Team Alpha', role: 'invited' }],
        groups: [{ group_id: 'g1', group_name: 'Group Beta', role: 'invited' }],
    } as never)

    renderProfile()

    fireEvent.click(screen.getByRole('button', { name: 'Pending Invites' }))

    await waitFor(() => expect(screen.getByText('Team Alpha')).toBeInTheDocument())
    expect(screen.getByText('Group Beta')).toBeInTheDocument()
    expect(api.users.getPendingInvites).toHaveBeenCalled()
})

test('accepting a team invite calls the respond endpoint', async () => {
    vi.mocked(api.users.getPendingInvites).mockResolvedValue({
        teams: [{ team_id: 't1', team_name: 'Team Alpha', role: 'invited' }],
        groups: [],
    } as never)
    vi.mocked(api.teams.respond).mockResolvedValue(undefined as never)

    renderProfile()
    fireEvent.click(screen.getByRole('button', { name: 'Pending Invites' }))

    await waitFor(() => expect(screen.getByText('Team Alpha')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Accept' }))

    await waitFor(() =>
        expect(api.teams.respond).toHaveBeenCalledWith('t1', { user_id: 'u1', action: 'accept' })
    )
})

test('shows the empty state when there are no invites or requests', async () => {
    vi.mocked(api.users.getPendingInvites).mockResolvedValue({ teams: [], groups: [] } as never)

    renderProfile()
    fireEvent.click(screen.getByRole('button', { name: 'Pending Invites' }))

    await waitFor(() => expect(screen.getByText('No pending invites or requests.')).toBeInTheDocument())
})
