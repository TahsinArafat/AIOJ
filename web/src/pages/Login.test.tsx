import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, expect, test, vi } from 'vitest'
import Login from './Login'
import { api } from '../lib/api'

vi.mock('../lib/api', () => ({
    api: {
        auth: {
            login: vi.fn(),
            verify2FA: vi.fn(),
        },
    },
    setTokens: vi.fn(),
}))

beforeEach(() => {
    vi.clearAllMocks()
})

function renderPage() {
    return render(
        <MemoryRouter>
            <Login />
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

test('surfaces a login error and keeps the form on screen', async () => {
    vi.mocked(api.auth.login).mockRejectedValue(new Error('invalid credentials'))

    renderPage()

    fireEvent.change(screen.getByLabelText('Username'), { target: { value: 'alice' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'wrong' } })
    fireEvent.click(screen.getByRole('button', { name: 'Login' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('invalid credentials')
    expect(screen.getByRole('button', { name: 'Login' })).toBeInTheDocument()
})
