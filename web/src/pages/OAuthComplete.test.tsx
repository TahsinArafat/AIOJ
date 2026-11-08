import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, expect, test, vi } from 'vitest'
import OAuthComplete from './OAuthComplete'
import { api, setTokens } from '../lib/api'

vi.mock('../lib/api', () => ({
    api: {
        auth: {
            completeOAuthSignup: vi.fn(),
            oauthUsernameAvailable: vi.fn(),
        },
    },
    setTokens: vi.fn(),
}))

function withFragment(fragment: string) {
    window.location.hash = fragment
    return render(
        <MemoryRouter initialEntries={['/oauth/complete']}>
            <OAuthComplete />
        </MemoryRouter>,
    )
}

function free(handle: string) {
    vi.mocked(api.auth.oauthUsernameAvailable).mockResolvedValue({ available: true, username: handle } as never)
}

function taken(message: string) {
    vi.mocked(api.auth.oauthUsernameAvailable).mockResolvedValue({ available: false, error: message } as never)
}

beforeEach(() => {
    vi.clearAllMocks()
    window.history.replaceState(null, '', '/oauth/complete')
    free('fuser')
})

test('signs a returning user straight in when the fragment holds tokens', async () => {
    withFragment('#access_token=at&refresh_token=rt')

    await waitFor(() => expect(setTokens).toHaveBeenCalledWith('at', 'rt'))
    // No username prompt: the account already exists.
    expect(screen.queryByRole('heading', { name: 'Choose your username' })).not.toBeInTheDocument()
})

test('asks a first-time user to confirm the suggested username', async () => {
    withFragment('#signup_token=ticket-1&username=fuser&provider=google')

    expect(await screen.findByRole('heading', { name: 'Choose your username' })).toBeInTheDocument()
    const input = screen.getByLabelText('Username') as HTMLInputElement
    expect(input.value).toBe('fuser')
    expect(screen.getByText(/You signed in with Google/)).toBeInTheDocument()
    // The suggestion is checked for availability without any typing.
    expect(await screen.findByText(/fuser is available/)).toBeInTheDocument()
    expect(vi.mocked(api.auth.oauthUsernameAvailable)).toHaveBeenCalledWith('ticket-1', 'fuser')
    expect(setTokens).not.toHaveBeenCalled()
})

test('blocks submission while the handle is taken', async () => {
    taken('username taken')
    withFragment('#signup_token=ticket-1&username=fuser&provider=github')

    expect(await screen.findByText('username taken')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Create account' })).toBeDisabled()
})

test('creates the account with the username the person typed', async () => {
    vi.mocked(api.auth.completeOAuthSignup).mockResolvedValue({
        access_token: 'a2',
        refresh_token: 'r2',
    } as never)
    withFragment('#signup_token=ticket-1&username=fuser&provider=google')

    fireEvent.change(screen.getByLabelText('Username'), { target: { value: 'my_handle' } })
    await waitFor(() =>
        expect(api.auth.oauthUsernameAvailable).toHaveBeenLastCalledWith('ticket-1', 'my_handle'),
    )
    fireEvent.click(screen.getByRole('button', { name: 'Create account' }))

    await waitFor(() =>
        expect(api.auth.completeOAuthSignup).toHaveBeenCalledWith({
            signup_token: 'ticket-1',
            username: 'my_handle',
        }),
    )
    await waitFor(() => expect(setTokens).toHaveBeenCalledWith('a2', 'r2'))
})

test('shows the server error when the handle is refused at submit time', async () => {
    vi.mocked(api.auth.completeOAuthSignup).mockRejectedValue(new Error('username taken'))
    // The hint says it is free, so submit is enabled and the server disagrees.
    free('fuser')

    withFragment('#signup_token=ticket-1&username=fuser')

    fireEvent.click(await screen.findByRole('button', { name: 'Create account' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('username taken')
    expect(setTokens).not.toHaveBeenCalled()
})

test('falls back to the sign-in page without a fragment', async () => {
    withFragment('')

    expect(await screen.findByText('Completing sign-in…')).toBeInTheDocument()
    expect(api.auth.completeOAuthSignup).not.toHaveBeenCalled()
})
