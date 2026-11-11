import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter, useLocation } from 'react-router-dom'
import { beforeEach, expect, test, vi } from 'vitest'
import { useEffect } from 'react'
import Login from './Login'
import { api } from '../lib/api'

vi.mock('../lib/api', () => ({
    api: {
        auth: {
            login: vi.fn(),
            verify2FA: vi.fn(),
            // The login page asks which sign-in methods this instance offers.
            // Default here: password only, so the OAuth assertions below are
            // about explicit provider switches rather than the fallback.
            methods: vi.fn(),
        },
    },
    setTokens: vi.fn(),
}))

beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.auth.methods).mockResolvedValue({
        data: { password_registration: true, google: false, github: false },
    } as never)
})

// Records the router location so a test can assert where a navigation went.
let lastLocation: { pathname: string; search: string } | null = null
function LocationProbe() {
    const location = useLocation()
    useEffect(() => { lastLocation = location })
    return null
}

function renderPage(initial = '/login') {
    lastLocation = null
    return render(
        <MemoryRouter initialEntries={[initial]}>
            <Login />
            <LocationProbe />
        </MemoryRouter>,
    )
}

test('submits credentials and shows the 2FA step when the server asks for one', async () => {
    vi.mocked(api.auth.login).mockResolvedValue({ requires_2fa: true, challenge_id: 'chal-1' } as never)

    renderPage()

    fireEvent.change(screen.getByLabelText('Username'), { target: { value: 'alice' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'hunter2hunter2' } })
    fireEvent.click(screen.getByRole('button', { name: 'Login' }))

    expect(await screen.findByRole('heading', { name: 'Two-factor authentication' })).toBeInTheDocument()
    expect(api.auth.login).toHaveBeenCalledWith({ username: 'alice', password: 'hunter2hunter2' })

    // The challenge id from the login response must be forwarded, not the code.
    vi.mocked(api.auth.verify2FA).mockResolvedValue({ access_token: 'a', refresh_token: 'r' } as never)
    fireEvent.change(screen.getByLabelText('Authentication code'), { target: { value: '123456' } })
    fireEvent.click(screen.getByRole('button', { name: 'Verify' }))

    await waitFor(() => expect(api.auth.verify2FA).toHaveBeenCalledWith('chal-1', '123456'))
})

test('returns to the page the visitor started from', async () => {
    vi.mocked(api.auth.login).mockResolvedValue({ access_token: 'a', refresh_token: 'r' } as never)

    renderPage('/login?next=%2Fproblems')

    fireEvent.change(screen.getByLabelText('Username'), { target: { value: 'alice' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'hunter2hunter2' } })
    fireEvent.click(screen.getByRole('button', { name: 'Login' }))

    await waitFor(() => expect(lastLocation?.pathname).toBe('/problems'))
})

test('refuses to redirect off-site after login', async () => {
    vi.mocked(api.auth.login).mockResolvedValue({ access_token: 'a', refresh_token: 'r' } as never)

    renderPage('/login?next=https%3A%2F%2Fevil.example')

    fireEvent.change(screen.getByLabelText('Username'), { target: { value: 'alice' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'hunter2hunter2' } })
    fireEvent.click(screen.getByRole('button', { name: 'Login' }))

    await waitFor(() => expect(lastLocation?.pathname).toBe('/'))
})

test('surfaces a login error and keeps the form on screen', async () => {
    vi.mocked(api.auth.login).mockRejectedValue(new Error('invalid credentials'))

    renderPage()

    fireEvent.change(screen.getByLabelText('Username'), { target: { value: 'alice' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'wrong' } })
    fireEvent.click(screen.getByRole('button', { name: 'Login' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('invalid credentials')
    expect(screen.getByRole('button', { name: 'Login' })).toBeInTheDocument()
})

test('hides every OAuth button when no provider is available', async () => {
    renderPage()

    // The divider that separates the form from the provider buttons must go too,
    // or the page ends on a stray rule.
    await waitFor(() => expect(api.auth.methods).toHaveBeenCalled())
    await waitFor(() => {
        expect(screen.queryByRole('link', { name: /Continue with Google/i })).not.toBeInTheDocument()
        expect(screen.queryByRole('link', { name: /Continue with GitHub/i })).not.toBeInTheDocument()
    })
})

test('shows only the providers the instance offers', async () => {
    vi.mocked(api.auth.methods).mockResolvedValue({
        data: { password_registration: true, google: true, github: false },
    } as never)

    renderPage()

    expect(await screen.findByRole('link', { name: /Continue with Google/i })).toHaveAttribute(
        'href',
        '/api/auth/oauth/google/start',
    )
    expect(screen.queryByRole('link', { name: /Continue with GitHub/i })).not.toBeInTheDocument()
})

test('still offers a way in when the methods request fails', async () => {
    // Failing open matters: an instance with password registration closed would
    // otherwise present a login page with no usable control at all.
    vi.mocked(api.auth.methods).mockRejectedValue(new Error('network down'))

    renderPage()

    expect(await screen.findByRole('link', { name: /Continue with Google/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Login' })).toBeInTheDocument()
})
