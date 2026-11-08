import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { expect, test, vi, beforeEach } from 'vitest'
import Profile from './Profile'
import { ToastProvider } from '../components/Toast'
import { api, getAccessToken } from '../lib/api'
import type { UserProfile } from '../types/api'

vi.mock('../lib/api', async (importOriginal) => {
    const actual = await importOriginal<typeof import('../lib/api')>()
    return {
        ...actual,
        getAccessToken: vi.fn(),
        signOut: vi.fn(),
        clearTokens: vi.fn(),
        api: {
            ...actual.api,
            users: {
                ...actual.api.users,
                getProfile: vi.fn(),
                updateProfile: vi.fn(),
                getPendingInvites: vi.fn(),
                exportMyData: vi.fn(),
                deleteAccount: vi.fn(),
            },
            teams: {
                ...actual.api.teams,
                respond: vi.fn(),
            },
            groups: {
                ...actual.api.groups,
                respond: vi.fn(),
            },
            setter: {
                ...actual.api.setter,
                status: vi.fn().mockResolvedValue({ status: 'none' }),
            },
        },
    }
})

// A JWT-shaped token whose payload decodes to a user (atob of the middle segment).
function makeToken(payload: Record<string, unknown>): string {
    return `header.${btoa(JSON.stringify(payload))}.signature`
}

const PROFILE: UserProfile = {
    user_id: 'u1',
    email: 'alice@example.com',
    created_at: '2024-03-04T10:00:00Z',
    rating: 1500,
    max_rating: 1610,
    contest_count: 7,
    problems_solved: 123,
    submissions: 456,
    bio: 'Competitive programmer.',
    avatar_url: '',
    first_name: 'Alice',
    last_name: 'Nguyen',
    country: 'Bangladesh',
    city: 'Dhaka',
    organization: 'AIOJ University',
    github_url: '',
    show_email: false,
    show_tags: true,
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
        makeToken({ uid: 'u1', uname: 'alice', role: 'user' })
    )
    vi.mocked(api.users.getProfile).mockReset().mockResolvedValue(PROFILE)
    vi.mocked(api.users.updateProfile).mockReset().mockResolvedValue(PROFILE)
    vi.mocked(api.users.getPendingInvites).mockReset().mockResolvedValue({ teams: [], groups: [] } as never)
    vi.mocked(api.teams.respond).mockReset().mockResolvedValue(undefined as never)
    vi.mocked(api.groups.respond).mockReset().mockResolvedValue(undefined as never)
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

test('identity header shows the handle, rank and real stats', async () => {
    renderProfile()

    // Handle and rank title come from the profile endpoint, not the token —
    // the token carries neither, which is why the old header showed "Unrated"
    // and an empty email for everyone.
    await waitFor(() => expect(screen.getByText('Alice Nguyen')).toBeInTheDocument())
    expect(screen.getByText('alice@example.com')).toBeInTheDocument()
    expect(screen.getAllByText('Adept').length).toBeGreaterThan(0) // 1500 → Adept
    expect(screen.getByText('Contests')).toBeInTheDocument()
    expect(screen.getByText('7')).toBeInTheDocument()
    expect(screen.getByText('123')).toBeInTheDocument()
    expect(screen.getByText('456')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Public profile/ })).toHaveAttribute('href', '/user/alice')
})

test('save stays disabled until something changes, then discards cleanly', async () => {
    renderProfile()

    const save = await screen.findByRole('button', { name: 'Save changes' })
    expect(save).toBeDisabled()
    expect(screen.getByText('All changes saved.')).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('First name'), { target: { value: 'Alicia' } })

    expect(save).toBeEnabled()
    expect(screen.getByText('You have unsaved changes.')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /Discard/ }))

    expect(save).toBeDisabled()
    expect(screen.getByLabelText('First name')).toHaveValue('Alice')
})

test('saving sends trimmed values and reports success', async () => {
    renderProfile()

    fireEvent.change(await screen.findByLabelText('City'), { target: { value: '  Chattogram  ' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))

    await waitFor(() =>
        expect(api.users.updateProfile).toHaveBeenCalledWith(
            expect.objectContaining({ city: 'Chattogram', first_name: 'Alice', country: 'Bangladesh' })
        )
    )
    expect(await screen.findByText('Profile updated')).toBeInTheDocument()
    expect(screen.getByText('All changes saved.')).toBeInTheDocument()
})

test('an invalid avatar URL blocks the save with a field error', async () => {
    renderProfile()

    const avatar = await screen.findByLabelText('Avatar URL')
    fireEvent.change(avatar, { target: { value: 'not-a-url' } })

    expect(screen.getByText('Avatar URL must be a full URL, e.g. https://example.com')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeDisabled()
    expect(screen.getByText('Fix the highlighted fields before saving.')).toBeInTheDocument()
})

test('invitations load with the page so the nav can show a count', async () => {
    vi.mocked(api.users.getPendingInvites).mockResolvedValue({
        teams: [{ team_id: 't1', team_name: 'Team Alpha', role: 'invited' }],
        groups: [{ group_id: 'g1', group_name: 'Group Beta', role: 'invited' }],
    } as never)

    renderProfile()

    // Loaded on mount, not on tab click: an invitation nobody opens is missed.
    await waitFor(() => expect(api.users.getPendingInvites).toHaveBeenCalled())
    const navItem = await screen.findByRole('button', { name: /Invitations/ })
    await waitFor(() => expect(navItem).toHaveTextContent('2'))

    fireEvent.click(navItem)

    expect(await screen.findByText('Team Alpha')).toBeInTheDocument()
    expect(screen.getByText('Group Beta')).toBeInTheDocument()
})

test('accepting a team invite calls the respond endpoint', async () => {
    vi.mocked(api.users.getPendingInvites).mockResolvedValue({
        teams: [{ team_id: 't1', team_name: 'Team Alpha', role: 'invited' }],
        groups: [],
    } as never)

    renderProfile()

    fireEvent.click(await screen.findByRole('button', { name: /Invitations/ }))
    await waitFor(() => expect(screen.getByText('Team Alpha')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Accept' }))

    await waitFor(() =>
        expect(api.teams.respond).toHaveBeenCalledWith('t1', { user_id: 'u1', action: 'accept' })
    )
})

test('shows the empty state when there are no invites or requests', async () => {
    renderProfile()

    fireEvent.click(await screen.findByRole('button', { name: /Invitations/ }))

    await waitFor(() =>
        expect(screen.getByText('No pending invites or requests')).toBeInTheDocument()
    )
})

test('danger zone requires an explicit confirmation before deleting', async () => {
    renderProfile()

    fireEvent.click(await screen.findByRole('button', { name: /Danger zone/ }))

    const del = screen.getByRole('button', { name: /Delete my account/ })
    expect(del).toBeDisabled()

    fireEvent.click(screen.getByRole('checkbox', { name: /permanent and cannot be undone/ }))
    expect(del).toBeEnabled()
    expect(api.users.deleteAccount).not.toHaveBeenCalled()
})
